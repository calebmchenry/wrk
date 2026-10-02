#!/usr/bin/env python3
"""Validate all packaged assets and embedded Go metadata; execute the native asset.

Usage: python3 scripts/verify-release.py dist
Requires Go for portable build-info inspection. Cross-platform inspection is not
runtime evidence; the reusable CI matrix runs real replacements on all platforms.
"""
import hashlib
import json
import pathlib
import platform
import re
import subprocess
import sys
import tarfile
import tempfile

dist = pathlib.Path(sys.argv[1] if len(sys.argv) > 1 else "dist")
manifests = list(dist.glob("wrk_*_checksums.txt"))
assert len(manifests) == 1, "expected exactly one checksum manifest"
version = manifests[0].name.removeprefix("wrk_").removesuffix("_checksums.txt")
kind = "snapshot" if version.endswith("-snapshot") else "release"
if kind == "release":
    assert re.fullmatch(r"(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)", version)
checksums = {}
for line in manifests[0].read_text().splitlines():
    digest, name = line.split()
    assert name not in checksums and re.fullmatch(r"[0-9a-f]{64}", digest)
    checksums[name] = digest
expected = {f"wrk_{version}_{os}_{arch}.tar.gz" for os in ("darwin", "linux") for arch in ("amd64", "arm64")}
assert set(checksums) == expected, "checksum manifest does not match the platform matrix"
assert {p.name for p in dist.glob("*.tar.gz")} == expected
native = (platform.system().lower(), {"x86_64": "amd64", "aarch64": "arm64", "arm64": "arm64"}.get(platform.machine()))
commit = subprocess.check_output(["git", "rev-parse", "HEAD"], text=True).strip()
for name in sorted(expected):
    archive = dist / name
    assert hashlib.sha256(archive.read_bytes()).hexdigest() == checksums[name], name
    os_name, arch = name.removesuffix(".tar.gz").rsplit("_", 2)[1:]
    with tarfile.open(archive) as tar, tempfile.TemporaryDirectory() as tmp:
        members = tar.getmembers()
        assert len(members) == 1 and members[0].name == "wrk" and members[0].isfile(), name
        assert members[0].mode & 0o111 and members[0].size <= 128 << 20, name
        executable = pathlib.Path(tmp) / "wrk"
        executable.write_bytes(tar.extractfile(members[0]).read())
        executable.chmod(0o755)
        metadata = subprocess.check_output(["go", "version", "-m", str(executable)], text=True)
        for value in (f"GOOS={os_name}", f"GOARCH={arch}", "CGO_ENABLED=0", f"vcs.revision={commit}"):
            assert value in metadata, (name, value, metadata)
        # -trimpath omits ldflags from Go build info; inspect injected strings
        # here and assert their exact JSON fields when executing each native asset.
        payload = executable.read_bytes()
        for value in (version, commit, kind):
            assert value.encode() + b"\0" in payload, (name, value)
        if (os_name, arch) == native:
            envelope = json.loads(subprocess.check_output([str(executable), "version", "--json"], text=True))
            info = envelope["result"]
            assert envelope["ok"] and envelope["project_root"] is None
            assert (info["version"], info["commit"], info["build_kind"], info["goos"], info["goarch"]) == (version, commit, kind, os_name, arch)
            print(f"{name}: checksum, contents, metadata, native execution OK")
        else:
            print(f"{name}: checksum, contents, metadata OK (cross-built; not executed here)")
