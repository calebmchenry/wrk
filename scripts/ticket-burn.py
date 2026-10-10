#!/usr/bin/env python3
"""Run fresh Codex sessions through wrk run, requiring persisted done status."""

import argparse
import os
from pathlib import Path
import shlex
import sys


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
    parser = argparse.ArgumentParser(
        allow_abbrev=False,
        description=__doc__,
        epilog="Uses native wrk filters; --list-command is retired. "
               "Use --heartbeat-interval 60s instead of --heartbeat 60. "
               "Logs default to the selected project's .wrk-runs/. See docs/burns.md.")
    parser.add_argument("--wrk-command", type=command, default=default_wrk,
                        help="wrk executable and prefix arguments (default: %(default)s)")
    parser.add_argument("--codex-command", type=command,
                        default="codex exec --dangerously-bypass-approvals-and-sandbox",
                        help="Codex command; appends Implement {id} (default: %(default)s)")
    for name in ("ready", "all", "stream", "verbose", "json"):
        parser.add_argument("--" + name, action="store_true", help="forward to wrk run")
    parser.add_argument("--label", action="append", default=[], help="require every label")
    for name in ("under", "ticket", "project", "config", "max-tickets",
                 "poll-interval", "heartbeat-interval", "log-dir"):
        parser.add_argument("--" + name, help="forward to wrk run")
    args = parser.parse_args()
    if not (args.ready or args.all or args.label or args.under or args.ticket):
        parser.error("select work with --ready, --all, --label, --under, or --ticket")
    return args


def main():
    args = parse_args()
    argv = args.wrk_command + ["run"]
    for name, value in vars(args).items():
        if name in ("wrk_command", "codex_command"):
            continue
        flag = "--" + name.replace("_", "-")
        if isinstance(value, bool):
            if value:
                argv.append(flag)
        elif isinstance(value, list):
            argv.extend(flag + "=" + item for item in value)
        elif value is not None:
            argv.append(flag + "=" + value)
    argv += ["--expect-status=done", "--", *args.codex_command, "Implement {id}"]
    # Replace this process: wrk owns selection, checks, logs, signals, and exit codes.
    try:
        os.execvp(argv[0], argv)
    except OSError as exc:
        print(f"ticket-burn: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
