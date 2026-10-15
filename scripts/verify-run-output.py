#!/usr/bin/env python3
"""Exercise run rendering on real PTYs; retain captures for visual review.

Usage: python3 scripts/verify-run-output.py ./bin/wrk [capture-directory]
Uses only the standard library on macOS/Linux, disposable tickets, and fake children.
"""

import fcntl
import json
import os
from pathlib import Path
import pty
import re
import select
import signal
import struct
import subprocess
import sys
import tempfile
import termios
import time


binary = str(Path(sys.argv[1]).resolve())
captures = Path(sys.argv[2] if len(sys.argv) > 2 else tempfile.mkdtemp(prefix="wrk-output-")).resolve()
captures.mkdir(parents=True, exist_ok=True)
project = Path(tempfile.mkdtemp(prefix="wrk-pty-")) / ("project with spaces and a quote's " + "long-" * 15)
project.mkdir()


def cli(*args):
    return subprocess.check_output([binary, *args], cwd=project, text=True)


cli("init")
ticket = json.loads(cli("new", "Multiline command review", "--json"))["result"]["ticket"]["id"]
cli("update", ticket, "--status", "in-progress")
child = project / "fake child.py"
child.write_text("""import json, os, sys, time
from pathlib import Path
Path('argv.json').write_text(json.dumps(sys.argv[1:]))
time.sleep(.4)
ending = chr(10) if os.environ.get('WRK_FAKE_NEWLINE') else ''
sys.stdout.write('out bytes' + ending); sys.stdout.flush()
time.sleep(.35)
sys.stderr.write('err bytes' + ending); sys.stderr.flush()
time.sleep(.4)
sys.exit(int(sys.argv[1]))
""")
prompt = "Implement {id}.\n\n    " + "Keep 'quotes', $variables, and spaces. " * 100 + "\nReport the commit hash."
sgr = re.compile(rb"\x1b\[[0-9;]*m")


def review(name, *, width=100, resize=False, raw=False, color=True, dumb=False,
           stdout_tty=True, stderr_tty=True, verbose=False, json_mode=False,
           exit_code=0, mismatch=False, idle=False, child_newline=False):
    master, slave = pty.openpty()
    fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", 35, width, 0, 0))
    if raw:
        settings = termios.tcgetattr(slave)
        settings[1] &= ~termios.OPOST
        termios.tcsetattr(slave, termios.TCSANOW, settings)
    env = os.environ.copy()
    env["TERM"] = "dumb" if dumb else "xterm-256color"
    env.pop("NO_COLOR", None)
    env.pop("WRK_FAKE_NEWLINE", None)
    if child_newline:
        env["WRK_FAKE_NEWLINE"] = "1"
    if not color:
        env["NO_COLOR"] = "1"
    log_dir = captures / name / ("long log path " + "segment-" * 14)
    log_dir.mkdir(parents=True)
    argv = [binary, "run", "--log-dir", str(log_dir), "--heartbeat-interval=150ms"]
    argv += ["--stream", "--ready", "--poll-interval=50ms"] if idle else ["--ticket", ticket, "--expect-status", "done" if mismatch else "in-progress"]
    if verbose:
        argv.append("--verbose")
    if json_mode:
        argv.append("--json")
    argv += ["--", sys.executable, str(child), str(exit_code), prompt, "", "{id}"]
    with tempfile.TemporaryFile() as out, tempfile.TemporaryFile() as err:
        process = subprocess.Popen(argv, cwd=project, env=env, stdin=subprocess.DEVNULL,
                                   stdout=slave if stdout_tty else out,
                                   stderr=slave if stderr_tty else err)
        data = bytearray()
        start = time.monotonic()
        resized = set()
        interrupted = False
        try:
            while True:
                elapsed = time.monotonic() - start
                if elapsed > 10:
                    raise AssertionError(f"{name}: timed out")
                if resize:
                    for threshold, columns in ((.3, 36), (.7, 120), (.9, 52)):
                        if elapsed >= threshold and columns not in resized:
                            fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", 35, columns, 0, 0))
                            process.send_signal(signal.SIGWINCH)
                            resized.add(columns)
                if idle and elapsed >= .5 and not interrupted:
                    process.send_signal(signal.SIGINT)
                    interrupted = True
                ready, _, _ = select.select([master], [], [], .03)
                if ready:
                    data.extend(os.read(master, 65536))
                elif process.poll() is not None:
                    break
            expected = 130 if idle else (1 if exit_code or mismatch else 0)
            assert process.wait() == expected, name
        finally:
            if process.poll() is None:
                process.kill()
                process.wait()
            os.close(slave)
            os.close(master)
        out.seek(0)
        err.seek(0)
        redirected_out, redirected_err = out.read(), err.read()
    data = bytes(data)
    for suffix, content in (("terminal", data), ("stdout", redirected_out), ("stderr", redirected_err)):
        (captures / name / (suffix + ".txt")).write_bytes(content)
    assert b"\x1b" not in redirected_out + redirected_err, name
    styled = color and not dumb and not json_mode and (stdout_tty or stderr_tty)
    assert (b"\x1b[" in data) == styled, name
    plain = sgr.sub(b"", data)
    assert b"\x1b" not in plain and b"\r" not in re.sub(rb"\r*\n\r*", b"\n", plain), name
    if raw and not dumb and not json_mode:
        human = data.replace(b"out bytes\n", b"").replace(b"err bytes\n", b"") if child_newline else data
        assert not re.search(rb"(?<!\r)\n", human), "raw tty staircase: " + name
        if child_newline:
            assert b"out bytes\n\r" in data and b"err bytes\n\r" in data, name
    records = [json.loads(line) for line in next(log_dir.glob("*/run.jsonl")).read_text().splitlines()]
    assert all(record["result"]["command_template"][-3] == prompt for record in records), name
    if json_mode:
        assert not stdout_tty, "JSON stdout must be captured separately for parse review"
        assert [json.loads(line) for line in redirected_out.splitlines()] == records, name
    else:
        combined = sgr.sub(b"", data + redirected_out + redirected_err)
        assert b"Report the commit hash" not in combined, name
        if idle:
            assert combined.count(b"WAITING  ") == 1, name
        else:
            assert combined.count(b"RUNNING  ") >= 3 and b"no output yet" in combined and b"last output" in combined, name
            if verbose:
                assert re.search(rb"\n(?:RUNNING|OK|FAILED)  ", combined), name
    if not idle:
        action = records[-1]["result"]["outcome"]["last_action"]
        ending = b"\n" if child_newline else b""
        assert Path(action["stdout_log"]).read_bytes() == b"out bytes" + ending, name
        assert Path(action["stderr_log"]).read_bytes() == b"err bytes" + ending, name
        assert json.loads((project / "argv.json").read_text()) == [str(exit_code), prompt.replace("{id}", ticket), "", ticket], name
        if exit_code or mismatch:
            final = records[-1]["result"]
            assert Path(final["retry_file"]).read_text().endswith(final["retry_command"] + "\n"), name
    print(name + ": OK")


review("wide-success")
review("narrow-mismatch", width=36, mismatch=True)
review("resize-process-failure", resize=True, exit_code=7)
review("raw-tty-verbose", raw=True, verbose=True)
review("raw-tty-child-newlines", raw=True, verbose=True, child_newline=True)
review("no-color", color=False, mismatch=True)
review("dumb", dumb=True)
review("redirected", stdout_tty=False, stderr_tty=False, verbose=True, mismatch=True)
review("terminal-progress", stdout_tty=False, exit_code=7)
review("terminal-summary", stderr_tty=False, mismatch=True)
review("json-verbose", stdout_tty=False, verbose=True, json_mode=True, mismatch=True)
review("waiting", idle=True)
print(f"Captures: {captures}\nDisposable project: {project}")
