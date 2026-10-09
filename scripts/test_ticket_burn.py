"""Exercise the runner using disposable projects and fake wrk/Codex processes."""

import json
import os
from pathlib import Path
import shlex
import signal
import subprocess
import sys
import tempfile
import time
import unittest


SCRIPT = Path(__file__).resolve().with_name("ticket-burn.py")
IDS = ["wrk-11111111", "wrk-22222222"]
FAKE = r'''
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import time

role, directory = sys.argv[1:3]
base = Path(directory)
state = json.loads((base / "state.json").read_text())
scenario = state["scenario"]
root = state["project"]
ids = ["wrk-11111111", "wrk-22222222"]
with (base / "calls.jsonl").open("a") as calls:
    calls.write(json.dumps({"role": role, "argv": sys.argv[3:], "cwd": os.getcwd()}) + "\n")

def save():
    (base / "state.json").write_text(json.dumps(state))

if scenario == "interrupt_" + role:
    (base / "child.pid").write_text(str(os.getpid()))
    subprocess.Popen([sys.executable, "-c", "import signal,time; signal.signal(signal.SIGTERM, signal.SIG_IGN); from pathlib import Path; import os; Path(" + repr(str(base / "grandchild.pid")) + ").write_text(str(os.getpid())); time.sleep(60)"])
    time.sleep(60)

if role == "codex":
    print("full Codex stdout", flush=True)
    print("full Codex stderr", file=sys.stderr, flush=True)
    if scenario == "codex_failure":
        sys.exit(7)
    if scenario == "heartbeat":
        time.sleep(1.2)
    ticket_id = sys.argv[3].removeprefix("Implement ")
    state["statuses"][ticket_id] = "in-progress" if scenario == "unfinished" else "done"
    save()
    sys.exit(0)

command = "list" if role == "list" else "show"
if scenario == role + "_failure":
    print("diagnostic from " + role, file=sys.stderr)
    sys.exit(4)
if scenario == "bad_json" and role == "list":
    print("not JSON")
    sys.exit(0)
if scenario == "bad_utf8" and role == "list":
    sys.stdout.buffer.write(b"\xff")
    sys.exit(0)

def ticket(ticket_id):
    return {"id": ticket_id, "title": "Example " + ticket_id,
            "status": state["statuses"].get(ticket_id, "todo")}

if role == "list":
    remaining = [ticket(i) for i in ids if i not in state["statuses"]]
    result = {"tickets": remaining[:1]}
    if scenario == "empty":
        result["tickets"] = []
    elif scenario == "malformed_tickets":
        result["tickets"] = None
    elif scenario == "invalid_id":
        result["tickets"] = [{"id": "../../oops"}]
    elif scenario == "blocked":
        result["tickets"][0]["status"] = "blocked"
    elif scenario == "malformed_status":
        result["tickets"][0]["status"] = []
    elif scenario == "stale_query":
        result["tickets"] = [ticket(ids[0])]
    elif scenario == "project_change" and state["statuses"]:
        root = str(base)
else:
    assert sys.argv[3] == "show"
    assert sys.argv[5:] == ["--project", root, "--json"], sys.argv
    result = {"ticket": ticket(sys.argv[4])}
    if scenario == "wrong_ticket":
        result["ticket"]["id"] = ids[1]

print(json.dumps({"ok": scenario != "envelope_failure", "command": command,
                  "project_root": root, "result": result}))
'''


class TicketBurnTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.base = Path(self.temp.name).resolve()
        self.project = self.base / "project with spaces"
        self.project.mkdir()
        self.fake = self.base / "fake commands.py"
        self.fake.write_text(FAKE)
        self.configure("normal")

    def configure(self, scenario):
        (self.base / "state.json").write_text(json.dumps({
            "scenario": scenario, "project": str(self.project), "statuses": {}}))
        (self.base / "calls.jsonl").write_text("")

    def argv(self, *extra):
        def cmd(role):
            return shlex.join([sys.executable, str(self.fake), role, str(self.base)])
        return [sys.executable, str(SCRIPT), "--list-command", cmd("list"),
                "--wrk-command", cmd("show"), "--codex-command", cmd("codex"),
                "--log-dir", str(self.base / "logs"), *extra]

    def run_burn(self, *extra):
        return subprocess.run(self.argv(*extra), cwd=self.base, text=True,
                              stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=15)

    def calls(self):
        return [json.loads(line) for line in (self.base / "calls.jsonl").read_text().splitlines()]

    def test_requeries_and_runs_exact_prompt_in_selected_project(self):
        result = self.run_burn()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        calls = self.calls()
        self.assertEqual([c["role"] for c in calls],
                         ["list", "codex", "show", "list", "codex", "show", "list"])
        children = [c for c in calls if c["role"] == "codex"]
        self.assertEqual([c["argv"] for c in children], [[f"Implement {i}"] for i in IDS])
        self.assertTrue(all(c["cwd"] == str(self.project) for c in children))
        self.assertTrue(all(c["cwd"] == str(self.base) for c in calls if c["role"] != "codex"))
        self.assertIn("No matching tickets", result.stdout)
        self.assertIn("2 completed", result.stdout)
        run_dir, = (self.base / "logs").iterdir()
        progress = (run_dir / "run.log").read_text()
        self.assertIn("Selecting ticket", progress)
        self.assertIn("Finished: 2 completed", progress)
        for index, ticket_id in enumerate(IDS, 1):
            content = (run_dir / f"{index:03d}-{ticket_id}.log").read_text()
            self.assertIn("full Codex stdout", content)
            self.assertIn("full Codex stderr", content)
            self.assertIn("# exit: 0", content)

    def test_empty_query_never_starts_codex(self):
        self.configure("empty")
        result = self.run_burn()
        self.assertEqual(result.returncode, 0, result.stdout)
        self.assertEqual([c["role"] for c in self.calls()], ["list"])
        self.assertIn("No matching tickets", result.stdout)
        self.assertIn("0 completed", result.stdout)

    def test_limit_and_heartbeat(self):
        self.configure("heartbeat")
        result = self.run_burn("--max-tickets", "1", "--heartbeat", "1")
        self.assertEqual(result.returncode, 0, result.stdout)
        self.assertIn("wrk-11111111: still running", result.stdout)
        self.assertIn("Reached --max-tickets 1", result.stdout)
        self.assertEqual([c["role"] for c in self.calls()], ["list", "codex", "show"])

    def test_failures_stop_without_retrying(self):
        scenarios = {
            "list_failure": ("list command exited 4", ["list"]),
            "bad_json": ("invalid JSON", ["list"]),
            "bad_utf8": ("codec can't decode", ["list"]),
            "envelope_failure": ("did not report success", ["list"]),
            "malformed_tickets": ("tickets array", ["list"]),
            "invalid_id": ("invalid ticket ID", ["list"]),
            "blocked": ("check the filter", ["list"]),
            "malformed_status": ("check the filter", ["list"]),
            "codex_failure": ("Codex failed", ["list", "codex"]),
            "unfinished": ("remains 'in-progress'", ["list", "codex", "show"]),
            "show_failure": ("show command exited 4", ["list", "codex", "show"]),
            "wrong_ticket": ("wrong project or ticket", ["list", "codex", "show"]),
            "stale_query": ("again; check the filter", ["list", "codex", "show", "list"]),
            "project_change": ("changed projects", ["list", "codex", "show", "list"]),
        }
        for scenario, (message, roles) in scenarios.items():
            with self.subTest(scenario=scenario):
                self.configure(scenario)
                result = self.run_burn()
                self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
                self.assertIn(message, result.stdout)
                self.assertNotIn("Traceback", result.stderr)
                self.assertEqual([c["role"] for c in self.calls()], roles)

    def test_invalid_arguments_and_missing_command(self):
        for extra in [("--max-tickets", "0"), ("--heartbeat", "-1"),
                      ("--codex-command", ""), ("--list-command", "'")]:
            with self.subTest(extra=extra):
                self.assertEqual(self.run_burn(*extra).returncode, 2)
        result = self.run_burn("--codex-command", str(self.base / "missing"))
        self.assertEqual(result.returncode, 1)
        self.assertIn("No such file", result.stdout)

    def test_interrupt_stops_command_and_descendants(self):
        for role, sig in [("list", signal.SIGINT), ("codex", signal.SIGTERM)]:
            with self.subTest(role=role):
                self.configure("interrupt_" + role)
                for name in ("child.pid", "grandchild.pid"):
                    (self.base / name).unlink(missing_ok=True)
                process = subprocess.Popen(self.argv(), cwd=self.base, text=True,
                                           stdout=subprocess.PIPE, stderr=subprocess.PIPE)
                try:
                    deadline = time.monotonic() + 5
                    while not (self.base / "grandchild.pid").exists():
                        self.assertIsNone(process.poll())
                        self.assertLess(time.monotonic(), deadline, "fake command did not start")
                        time.sleep(0.02)
                    process.send_signal(sig)
                    stdout, stderr = process.communicate(timeout=10)
                    self.assertEqual(process.returncode, 130, stdout + stderr)
                    self.assertIn("Interrupted; stopped the active command", stdout)
                    self.assertNotIn("show", [c["role"] for c in self.calls()])
                    for name in ("child.pid", "grandchild.pid"):
                        pid = int((self.base / name).read_text())
                        deadline = time.monotonic() + 3
                        while time.monotonic() < deadline:
                            try:
                                os.kill(pid, 0)
                            except ProcessLookupError:
                                break
                            time.sleep(0.02)
                        else:
                            self.fail(f"{name} still alive after interruption")
                finally:
                    if process.poll() is None:
                        process.kill()
                    process.communicate(timeout=5)
                    for name in ("child.pid", "grandchild.pid"):
                        if (self.base / name).exists():
                            try:
                                os.kill(int((self.base / name).read_text()), signal.SIGKILL)
                            except ProcessLookupError:
                                pass


if __name__ == "__main__":
    unittest.main()
