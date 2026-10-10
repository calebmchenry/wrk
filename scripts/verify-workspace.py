#!/usr/bin/env python3
"""Smoke-test a standalone native wrk executable outside its source tree.

Usage: python3 scripts/verify-workspace.py /absolute/path/to/wrk
Uses only Python's standard library; the executable runs with an empty PATH.
The full Chromium suite separately verifies rendering and browser interactions.
"""
import http.client
import json
import os
import pathlib
import re
import selectors
import signal
import socket
import subprocess
import sys
import tempfile
from urllib.parse import urljoin, urlsplit


def verify(executable):
    executable = pathlib.Path(executable).resolve(strict=True)
    with tempfile.TemporaryDirectory(prefix="wrk-workspace-") as tmp:
        cwd = pathlib.Path(tmp)
        env = dict(os.environ, PATH=str(cwd / "no-tools"))

        def cli(*args):
            run = subprocess.run([str(executable), *args, "--json"], cwd=cwd,
                                 env=env, capture_output=True, text=True, timeout=10)
            assert run.returncode == 0 and not run.stderr, (args, run.stdout, run.stderr)
            data = json.loads(run.stdout)
            assert data["ok"], data
            return data["result"]

        cli("init")
        context = cli("new", "Packaged context")["ticket"]["id"]
        with tempfile.TemporaryFile(mode="w+") as stderr:
            process = subprocess.Popen(
                [str(executable), "serve", "--port=0", "--json"], cwd=cwd, env=env,
                stdin=subprocess.DEVNULL, stdout=subprocess.PIPE, stderr=stderr,
                text=True,
            )
            try:
                def event(want):
                    with selectors.DefaultSelector() as selector:
                        selector.register(process.stdout, selectors.EVENT_READ)
                        assert selector.select(10), f"timed out waiting for {want}"
                    data = json.loads(process.stdout.readline())
                    assert data["ok"] and data["errors"] == [], data
                    assert data["result"]["event"] == want, data
                    assert pathlib.Path(data["project_root"]).resolve() == cwd.resolve(), data
                    return data["result"]

                base = event("started")["url"]
                url = urlsplit(base)
                assert url.scheme == "http" and url.hostname == "127.0.0.1" and url.port, base

                def request(path, status=200, method="GET", body=None, headers=None):
                    connection = http.client.HTTPConnection(url.hostname, url.port, timeout=5)
                    try:
                        connection.request(method, path, body=body, headers=headers or {})
                        response = connection.getresponse()
                        payload = response.read()
                        assert response.status == status, (path, response.status, payload)
                        assert response.getheader("Access-Control-Allow-Origin") is None
                        assert response.getheader("Cache-Control") == "no-store"
                        assert response.getheader("X-Content-Type-Options") == "nosniff"
                        return payload
                    finally:
                        connection.close()

                def api(path, status=200, method="GET", body=None, origin=None):
                    headers = {"Content-Type": "application/json", "Origin": origin or base.rstrip("/")}
                    payload = request(path, status, method,
                                      None if body is None else json.dumps(body), headers)
                    data = json.loads(payload)
                    assert data["ok"] == (status < 400), data
                    return data

                # Check the actual embedded entry point and every current module.
                html = request("/").decode()
                assert "Local workspace" in html
                paths = set(re.findall(r'(?:<script[^>]*src|<link[^>]*href)="([^"]+)"', html))
                assert paths == {"/style.css", "/app.js"}, paths
                visited = set()
                while paths:
                    path = paths.pop()
                    assert path.startswith("/") and not path.startswith("//"), path
                    asset = request(path).decode()
                    assert asset.strip(), path
                    visited.add(path)
                    # Static ES module imports used by the bundled UI.
                    for dependency in re.findall(r'\bfrom\s+[\'"]([^\'"]+)[\'"]', asset):
                        resolved = urlsplit(urljoin(base.rstrip("/") + path, dependency))
                        assert (resolved.scheme, resolved.netloc) == (url.scheme, url.netloc), dependency
                        if resolved.path not in visited:
                            paths.add(resolved.path)
                assert visited == {"/style.css", "/app.js", "/model.mjs", "/live.mjs", "/editor.mjs", "/pickers.mjs", "/detail.mjs", "/inline.mjs"}, visited
                request("/.wrk/config.yaml", 404)
                request("/api/project", 403, headers={"Host": f"localhost:{url.port}"})
                api("/api/items", 403, "POST", {"title": "Rejected cross-site write"}, "https://example.invalid")
                assert api("/api/project")["result"]["ticket_count"] == 1

                created = api("/api/items", 201, "POST", {
                    "title": "Packaged browser write", "body": "Embedded **Markdown** body\n",
                    "labels": ["smoke"], "related": [context],
                })["result"]["ticket"]
                item_id = created["id"]
                saved = cli("show", item_id)
                assert saved["ticket"]["labels"] == ["smoke"] and saved["ticket"]["related"] == [context], saved
                assert saved["source"].endswith("\n---\nEmbedded **Markdown** body\n"), saved
                assert (cwd / saved["ticket"]["path"]).read_text() == saved["source"]
                assert cli("show", context)["ticket"]["related"] == [item_id]
                detail = api(f"/api/items/{item_id}")["result"]
                assert "<strong>Markdown</strong>" in detail["body_html"]
                cli("update", item_id, "--status=in-progress", "--title=Agent changed the item")
                stale = api(f"/api/items/{item_id}", 409, "PATCH", {
                    "expected_revision": created["revision"], "title": "Stale overwrite",
                })
                assert stale["errors"][0]["code"] == "CONFLICT", stale
                workspace = api(f"/api/workspace?selected={item_id}")["result"]
                current = workspace["detail"]["ticket"]
                assert current["title"] == "Agent changed the item" and current["status"] == "in-progress", current
                api(f"/api/items/{item_id}", 200, "PATCH", {
                    "expected_revision": current["revision"], "status": "done",
                })
                assert cli("show", item_id)["ticket"]["status"] == "done"
                cli("validate")
                process.send_signal(signal.SIGTERM)
                assert event("stopped")["url"] == base
                assert process.wait(timeout=5) == 0
                assert not process.stdout.read(), "unexpected lifecycle output"
                stderr.seek(0)
                assert not stderr.read(), "unexpected stderr"
                with socket.socket() as probe:
                    probe.settimeout(1)
                    assert probe.connect_ex((url.hostname, url.port)) != 0, "listener survived shutdown"
            finally:
                if process.poll() is None:
                    process.kill()
                    process.wait(timeout=5)
                process.stdout.close()
    print("Workspace OK: embedded assets, local API/CLI persistence, related links, stale writes, SIGTERM; empty PATH")


if __name__ == "__main__":
    verify(sys.argv[1])
