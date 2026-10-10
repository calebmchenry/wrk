"""Test only the exec wrapper; selection/process/log tests live in Go suites."""

import json
import os
from pathlib import Path
import shlex
import subprocess
import sys
import tempfile
import unittest


SCRIPT = Path(__file__).resolve().with_name("ticket-burn.py")


class TicketBurnTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.base = Path(self.temp.name).resolve()
        self.fake = self.base / "fake wrk"
        self.fake.write_text(f"#!{sys.executable}\n" + '''
import json, os, sys
print(json.dumps({"args": sys.argv[1:], "cwd": os.getcwd(),
                  "env": os.environ.get("WRK_WRAPPER_TEST"), "pid": os.getpid()}))
sys.exit(int(os.environ.get("WRK_WRAPPER_EXIT", "0")))
''')
        self.fake.chmod(0o755)
        for name in ("wrk", "go"):
            (self.base / name).symlink_to(self.fake)
        self.env = dict(os.environ, PATH=str(self.base) + os.pathsep + os.environ["PATH"],
                        WRK_WRAPPER_TEST="exported value")

    def launch(self, *extra, cwd=None):
        return subprocess.Popen([sys.executable, str(SCRIPT), *extra],
                                cwd=cwd or self.base, env=self.env, text=True,
                                stdout=subprocess.PIPE, stderr=subprocess.PIPE)

    def capture(self, *extra, cwd=None, code=0):
        with self.launch(*extra, cwd=cwd) as proc:
            out, err = proc.communicate(timeout=10)
            self.assertEqual(proc.returncode, code, out + err)
            if code in (1, 2):
                return err
            data = json.loads(out)
            self.assertEqual(data["pid"], proc.pid, "wrapper must exec, not supervise wrk")
            self.assertEqual(data["env"], "exported value")
            return data

    def test_full_access_default_and_checkout_cli(self):
        suffix = ["run", "--ready", "--label=web", "--expect-status=done", "--",
                  "codex", "exec", "--dangerously-bypass-approvals-and-sandbox", "Implement {id}"]
        external = self.capture("--ready", "--label", "web")
        self.assertEqual(external["args"], suffix)
        checkout = self.capture("--ready", "--label", "web", cwd=SCRIPT.parent.parent)
        self.assertEqual(checkout["args"], ["run", "./cmd/wrk", *suffix])
        self.assertEqual(Path(checkout["cwd"]), SCRIPT.parent.parent)

    def test_native_flags_and_quoted_commands(self):
        data = self.capture(
            "--wrk-command", shlex.join([sys.executable, str(self.fake), "prefix"]),
            "--codex-command", "custom-codex exec --model 'two words'",
            "--ready", "--label=web", "--label=backend", "--under=wrk-11111111",
            "--project=../project with spaces", "--stream", "--poll-interval=20ms",
            "--max-tickets=3", "--heartbeat-interval=1s", "--log-dir=logs with spaces",
            "--json", "--verbose")
        args = data["args"]
        self.assertEqual(args[:2], ["prefix", "run"])
        self.assertEqual(args[-7:], ["--expect-status=done", "--", "custom-codex", "exec",
                                    "--model", "two words", "Implement {id}"])
        for flag in ("--ready", "--label=web", "--label=backend", "--under=wrk-11111111",
                     "--project=../project with spaces", "--stream", "--poll-interval=20ms",
                     "--max-tickets=3", "--heartbeat-interval=1s", "--log-dir=logs with spaces",
                     "--json", "--verbose"):
            self.assertIn(flag, args[2:-7])

    def test_explicit_retry_and_exit_codes(self):
        for code in (0, 7, 130):
            self.env["WRK_WRAPPER_EXIT"] = str(code)
            data = self.capture("--ticket=wrk-11111111", "--config=../project/.wrk/config.yaml",
                                code=code)
            self.assertIn("--ticket=wrk-11111111", data["args"])
            self.assertIn("--config=../project/.wrk/config.yaml", data["args"])

    def test_rejected_legacy_flags_and_command_override(self):
        for flags in ([], ["--ready", "--list-command", "wrk list --json"],
                      ["--ready", "--heartbeat", "1"], ["--ready", "--expect-status", "todo"],
                      ["--ready", "--", "other-command"], ["--ready", "--codex-command", ""],
                      ["--ready", "--wrk-command", "'"]):
            with self.subTest(flags=flags):
                self.assertIn("error:", self.capture(*flags, code=2))
        error = self.capture("--ready", "--wrk-command", str(self.base / "missing"), code=1)
        self.assertIn("ticket-burn:", error)
        self.assertNotIn("Traceback", error)


if __name__ == "__main__":
    unittest.main()
