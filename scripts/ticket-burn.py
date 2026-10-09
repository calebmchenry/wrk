#!/usr/bin/env python3
"""Implement one selected wrk ticket at a time with fresh Codex sessions."""

import argparse
from datetime import datetime
import json
import os
from pathlib import Path
import re
import shlex
import signal
import subprocess
import sys
import time


class BurnError(Exception):
    pass


def positive_int(value):
    number = int(value)
    if number <= 0:
        raise argparse.ArgumentTypeError("must be greater than zero")
    return number


def command(value):
    try:
        parts = shlex.split(value)
    except ValueError as exc:
        raise argparse.ArgumentTypeError(str(exc)) from exc
    if not parts:
        raise argparse.ArgumentTypeError("command must not be empty")
    return parts


def parse_args():
    checkout = Path(__file__).resolve().parent.parent
    default_wrk = "go run ./cmd/wrk" if Path.cwd().resolve() == checkout else "wrk"
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--list-command", required=True, type=command,
                        help="command returning a wrk list --json envelope (re-run each pass)")
    parser.add_argument("--wrk-command", type=command, default=default_wrk,
                        help="wrk command for completion checks (default: %(default)s)")
    parser.add_argument("--codex-command", type=command,
                        default="codex exec --dangerously-bypass-approvals-and-sandbox",
                        help="Codex command; the prompt is appended (default: %(default)s)")
    parser.add_argument("--max-tickets", type=positive_int,
                        help="stop after this many completed tickets")
    parser.add_argument("--log-dir", type=Path, default=Path(".wrk-burn"),
                        help="parent directory for run logs (default: %(default)s)")
    parser.add_argument("--heartbeat", type=positive_int, default=60,
                        help="seconds between still-running messages (default: %(default)s)")
    return parser.parse_args()


def stop_child(child):
    # The child has its own session, so stop its tools as well as its launcher.
    try:
        try:
            os.killpg(child.pid, signal.SIGTERM)
        except ProcessLookupError:
            pass
        try:
            child.wait(timeout=5)
        except subprocess.TimeoutExpired:
            pass
    finally:
        try:
            os.killpg(child.pid, signal.SIGKILL)
        except ProcessLookupError:
            pass
        child.wait()


def run_child(argv, cwd, stdout, stderr, heartbeat, emit, label):
    started = time.monotonic()
    with subprocess.Popen(argv, cwd=cwd, stdin=subprocess.DEVNULL,
                          stdout=stdout, stderr=stderr, text=True, encoding="utf-8",
                          start_new_session=True) as child:
        try:
            while True:
                try:
                    output, _ = child.communicate(timeout=heartbeat)
                    return child.returncode, output, int(time.monotonic() - started)
                except subprocess.TimeoutExpired:
                    emit(f"{label}: still running ({int(time.monotonic() - started)}s)")
        except BaseException:
            stop_child(child)
            raise


def read_wrk(argv, expected, cwd, log, heartbeat, emit):
    log.write(f"\n$ {shlex.join(argv)}\n")
    code, output, _ = run_child(argv, cwd, subprocess.PIPE, log,
                               heartbeat, emit, expected)
    log.write(output + "\n")
    if code:
        raise BurnError(f"{expected} command exited {code}; see run.log")
    try:
        data = json.loads(output)
    except ValueError as exc:
        raise BurnError(f"{expected} command returned invalid JSON; see run.log") from exc
    if not isinstance(data, dict) or data.get("ok") is not True:
        raise BurnError(f"{expected} command did not report success; see run.log")
    if data.get("command") != expected or not isinstance(data.get("result"), dict):
        raise BurnError(f"expected a wrk {expected} JSON envelope; see run.log")
    root = data.get("project_root")
    if not isinstance(root, str) or not Path(root).is_absolute() or not Path(root).is_dir():
        raise BurnError(f"{expected} returned an invalid project_root; see run.log")
    return Path(root).resolve(), data["result"]


def burn(args, run_dir, log):
    def emit(message):
        line = f"[{datetime.now():%H:%M:%S}] {message}"
        log.write(line + "\n")
        print(line, flush=True)

    completed = 0
    started = time.monotonic()
    try:
        emit(f"Logs: {run_dir}")
        cwd = Path.cwd()
        project = None
        attempted = set()
        while args.max_tickets is None or completed < args.max_tickets:
            emit(f"Selecting ticket ({completed} completed)")
            root, result = read_wrk(args.list_command, "list", cwd, log, args.heartbeat, emit)
            if project is not None and root != project:
                raise BurnError("selection command changed projects")
            project = root
            tickets = result.get("tickets")
            if not isinstance(tickets, list):
                raise BurnError("list result must contain a tickets array")
            if not tickets:
                emit("No matching tickets")
                return 0
            ticket = tickets[0]
            ticket_id = ticket.get("id") if isinstance(ticket, dict) else None
            if not isinstance(ticket_id, str) or not re.fullmatch(r"[a-z][a-z0-9]{0,15}-[0-9a-f]{8}", ticket_id):
                raise BurnError("first list result has an invalid ticket ID")
            if ticket_id in attempted:
                raise BurnError(f"selection returned {ticket_id} again; check the filter")
            if ticket.get("status") not in ("todo", "in-progress"):
                raise BurnError(f"selected {ticket_id} is {ticket.get('status')!r}; check the filter")
            attempted.add(ticket_id)
            child_log = run_dir / f"{completed + 1:03d}-{ticket_id}.log"
            emit(f"#{completed + 1} {ticket_id}: {ticket.get('title', '')}")
            emit(f"Starting Codex; log: {child_log}")
            argv = args.codex_command + [f"Implement {ticket_id}"]
            with child_log.open("w", encoding="utf-8", buffering=1) as output:
                output.write(f"# cwd: {project}\n# command: {shlex.join(argv)}\n\n")
                code, _, elapsed = run_child(argv, project, output, subprocess.STDOUT,
                                             args.heartbeat, emit, ticket_id)
                output.write(f"\n# exit: {code}; elapsed: {elapsed}s\n")
            emit(f"{ticket_id}: Codex exited {code} after {elapsed}s")
            if code:
                raise BurnError(f"Codex failed for {ticket_id}; see {child_log}")
            root, result = read_wrk(
                args.wrk_command + ["show", ticket_id, "--project", str(project), "--json"],
                "show", cwd, log, args.heartbeat, emit)
            updated = result.get("ticket")
            if root != project or not isinstance(updated, dict) or updated.get("id") != ticket_id:
                raise BurnError("completion check returned the wrong project or ticket")
            if updated.get("status") != "done":
                raise BurnError(f"{ticket_id} remains {updated.get('status')!r}; stopping for review")
            completed += 1
            emit(f"{ticket_id}: done")
        emit(f"Reached --max-tickets {args.max_tickets}")
        return 0
    except KeyboardInterrupt:
        emit("Interrupted; stopped the active command")
        return 130
    except (BurnError, OSError, UnicodeError) as exc:
        emit(f"Stopped: {exc}")
        return 1
    finally:
        emit(f"Finished: {completed} completed in {int(time.monotonic() - started)}s; logs: {run_dir}")


def interrupt(_signum, _frame):
    raise KeyboardInterrupt


def main():
    args = parse_args()
    signal.signal(signal.SIGTERM, interrupt)
    try:
        run_dir = args.log_dir.resolve() / datetime.now().strftime("%Y%m%d-%H%M%S-%f")
        run_dir.mkdir(parents=True)
        with (run_dir / "run.log").open("w", encoding="utf-8", buffering=1) as log:
            return burn(args, run_dir, log)
    except KeyboardInterrupt:
        return 130
    except OSError as exc:
        print(f"ticket-burn: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
