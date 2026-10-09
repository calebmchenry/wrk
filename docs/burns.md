# Scoped agent burns

A burn is an extended, explicitly scoped work session using ordinary wrk commands
and durable ticket bodies. The CLI does not run AI, schedule sessions, grant
permissions, or provide parallel task claiming. Use the [daily workflow](index.md#daily-workflow)
and run this checkout's CLI from the repository root as `go run ./cmd/wrk`.
Commands below abbreviate it to `wrk`.

## Minimal Codex runner

[`scripts/ticket-burn.py`](../scripts/ticket-burn.py) runs one fresh Codex session
per ticket. It requires Python 3.9+, the Codex CLI configured for noninteractive
use, and macOS or Linux. From this checkout's root:

```sh
python3 scripts/ticket-burn.py \
  --list-command 'go run ./cmd/wrk list --ready --label web --json'
```

Use `--under <parent-id>` instead of, or alongside, `--label` to select a hierarchy.
Add `--max-tickets 1` for a single-ticket run. Selection is required; the script
does not choose a scope for you.

Each pass runs the selection command, takes the first result, and starts
`codex exec --dangerously-bypass-approvals-and-sandbox` with exactly
`Implement <ticket-id>`. By default, child sessions have full local access with
no Codex sandbox or approval prompts; no extra invocation flags are needed.
Repository instructions and the ticket supply the implementation and verification
workflow. The runner checks `wrk show <id> --json` for `done` before querying
again, so newly unblocked tickets can appear on the next pass.

The command must return the full successful `wrk list --json` envelope, including
`project_root` and `result.tickets`. Order is preserved; ordinary wrk lists use
ID order, not priority order. Selection and completion-check commands run in the
directory where you launched the script. Codex runs in the envelope's project
root, and every completion check explicitly selects that project. A query that
switches projects stops the run.

Commands are split into arguments with shell quoting supported; shell expansion,
redirection, and pipelines are not implicit. For custom filtering, use a wrapper
command that preserves the envelope. `--wrk-command` selects the executable and
any prefix arguments used for `show`; it defaults to `go run ./cmd/wrk` when
launched at this checkout's root and `wrk` elsewhere. For example, from another
project with wrk installed:

```sh
python3 /path/to/wrk/scripts/ticket-burn.py \
  --list-command 'wrk list --ready --label maintenance --json' \
  --max-tickets 3
```

Use `--codex-command 'codex exec --dangerously-bypass-approvals-and-sandbox --model MODEL'` to
customize Codex options. The runner appends the implementation prompt as a single
argument and closes child stdin. Authentication and settings come from the
existing Codex installation. An explicit `--codex-command` replaces the entire
default command, including its permission flags.

Console progress includes each ticket's ID/title, start, exit code, elapsed time,
and a heartbeat every 60 seconds (`--heartbeat N` changes the interval). Logs go
in `.wrk-burn/<timestamp>/` under the launch directory; `--log-dir PATH` changes
the parent directory. `run.log` records progress plus list/show commands and
outputs. `001-<ticket-id>.log`, etc. contain the full Codex stdout/stderr, command,
and working directory. This repository ignores `.wrk-burn/`.

The runner stops at the first command/JSON error, failed Codex invocation,
unfinished ticket, repeated selection, or selected ticket outside `todo` and
`in-progress`. It leaves the ticket's status to Codex and does not retry. Ctrl-C
or SIGTERM stops the active command and its process group. The exit code is 0
for an empty selection or the ticket limit, 1 for failure, 2 for invalid arguments,
and 130 for interruption.

An empty result prints **No matching tickets** and exits; it does not wait for
future arrivals or imply all work is complete. In particular, `--ready` excludes
in-progress tickets. After an interrupted or unfinished attempt, inspect and
resume that ticket before running the ready queue again. Run one worker for a
scope: this script does not claim tickets or coordinate concurrent agents.

Test the runner without invoking real Codex or changing the backlog:

```sh
python3 -B -m unittest discover -s scripts -p 'test_ticket_burn.py' -v
```

### When Codex exits 0 but the ticket stays in progress

Exit 0 means the Codex session ended normally, not that the ticket met its
acceptance criteria. Read the final response in the numbered ticket log and
the ticket's handoff notes. The runner deliberately checks the persisted status.

The former default `--sandbox workspace-write` could prevent verification needing
local listeners, browser IPC, Docker sockets, or files outside its writable
directories. A run on 2026-10-08 implemented `wrk-ce7350ff` but could not verify
it: loopback binds returned `operation not permitted`, Chromium IPC was denied,
and Docker was inaccessible. The child correctly left verification unfinished.

The runner now defaults to full local access at the user's request; see the
[official noninteractive permissions documentation](https://learn.chatgpt.com/docs/non-interactive-mode#permissions-and-safety).
Resume a previously unfinished ticket directly with
`codex exec --dangerously-bypass-approvals-and-sandbox 'Implement <ticket-id>'`.
Once it is verified and marked done, restart the ready queue using the ordinary
runner command. `--ready` will not resume it automatically, and unfinished
dependencies may leave no ready tickets at all.

## Establish the session

Read repository instructions, the user's scope instructions, relevant tickets,
and existing handoffs. Record the session contract in the selected deliverable's
body or an existing coordination ticket: scope selector, intended outcome,
permissions already granted, actions needing separate approval, required checks,
and time/budget/other stopping limits. Labels identify work; they do not authorize
deployment, messages, unrelated changes, or wider work. Do not expand scope merely
to keep the session busy. If a prerequisite outside scope is unfinished, report
it; work on it only if the user's authorization includes it.

Choose a label for an arbitrary batch or a parent ID for a hierarchy. To label a
hierarchy once, use `wrk update <root> --add-label <batch> --recursive`. This includes
the root and every descendant regardless of status. Parent relationships and
dependencies stay unchanged. `--recursive` permits label edits only, and a label
cannot be added and removed in the same invocation. Inspect the reported targets.
A partial error requires [inspection and recovery](storage.md#recursive-label-publication)
before retrying.

For each pass, validate and inspect both active and complete scoped lists:

```sh
wrk validate
wrk list --label <batch>
wrk list --all --label <batch>
wrk list --ready --label <batch>
```

For a parent scope, substitute `--under <root>` for the label filter and read
`wrk show <root>` separately. `--under` includes all descendants but excludes the
root; explicitly state whether completing that root is also part of the session.
Label and parent selectors can be combined by intersection; repeated labels
require all values. Do not infer readiness from an incomplete local list:
dependencies are checked across the whole project.

## Work and resume loop

1. Read existing in-progress tickets in scope before starting another. Use `show`
   to inspect full source, dependency blockers, and children. Reconcile the recorded
   next step with the working tree and verification evidence. Resume authorized
   work that can proceed; do not assume another active worker's ticket is free.
   A writer lock protects individual CLI mutations, not an entire coding session.
2. If nothing in progress can proceed, select a ready ticket within scope. `ready`
   means todo with every dependency done. Read its acceptance criteria and body,
   then mark it in-progress through `wrk update <id> --status in-progress` before
   implementation. Selection never overrides permissions or stopping limits.
3. Implement and run the required verification. Update the body with decisions,
   files/artifacts, commands and results, remaining work, and the exact next step.
   Make body/config edits and Git operations **between**, never during, CLI
   mutations. For subtasks, use `wrk new "Title" --parent <id> --label <batch>`
   with every applicable batch label explicitly supplied. New children do not
   inherit labels from earlier recursive updates; explicit labels also replace
   configured creation defaults.
4. If external input or an unavailable prerequisite prevents progress, first
   record the reason, who/what can unblock it, the observable unblocking condition,
   work already done, verification state, and a concrete resume step. Then mark
   the ticket `blocked` and continue with other eligible scoped work. Do not use
   blocked merely for a session time limit or work that is still actionable.
5. When acceptance criteria are met and checks pass, record the evidence and mark
   the ticket done. Completing dependencies does not clear manual blocked status,
   and completing children does not close their parent. Mark a parent done only
   when its own criteria are satisfied. Canceled means explicitly retired work;
   it never satisfies another ticket's dependency.
6. Re-read the scope and repeat. If a blocked ticket's condition is now met, review
   the body and explicitly set todo for reselection or in-progress when resuming
   immediately. Retain the old reason and note how it was resolved. Never clear
   blocked merely because dependency blockers are now empty.

For example, a durable handoff can be a body section like this:

```markdown
## Session handoff

Scope: label burn-tonight; finish API tickets; publishing requires separate approval.
Completed: input parser and fixture updates in internal/cli.
Verification: go test ./internal/cli passed; full race suite still required.
Blocked reason: expected response format is undecided.
Unblocking condition: user supplies the required response fields.
Resume: update the response fixture, finish output code, run full checks.
Remaining: output formatting and acceptance verification; ticket is not done.
```

## Stop and report

An empty ready list is a signal to inspect, not evidence of completion. Inspect
in-progress and blocked tickets, todos with unfinished dependencies, external
prerequisites, and the selected root. Stop when all scoped work meets its criteria,
when no authorized work can proceed, or when an explicit session limit is reached.
Do not poll indefinitely, silently expand the batch, or mark unfinished work done.

On a limit, leave actionable work in-progress with a durable next step. If nothing
can proceed, identify the blockers and their unblocking conditions. Run
`wrk validate` before handing off. Report completed ticket IDs and verification,
remaining ticket IDs/statuses (including any canceled work), blockers or permission
gaps, the reason for stopping, and where the next session should resume. A label
can be removed later with `--remove-label`; do not remove it just to make the
unfinished batch disappear from reports.

`blocked` extends the version-1 status vocabulary. Older clients may reject every
data command in a project containing that value. Upgrade all clients before using
it. Existing tickets and config need no migration. See [CLI scope semantics](cli.md#scopes-and-label-mutations)
for the exact flags and [ticket body conventions](ticket-format.md#body-conventions)
for the durable record.
