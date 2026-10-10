# Scoped agent burns

A burn is an extended, explicitly scoped work session using wrk commands and durable
ticket bodies. `wrk run` executes a supplied command sequentially; it has no built-in
AI provider, scheduler, or parallel ticket claiming. Follow the
[daily workflow](index.md#daily-workflow). Commands below abbreviate this checkout's
`go run ./cmd/wrk` to `wrk` unless shown in full.

## Minimal Codex runner

From this checkout's root, the ordinary burn is now a native invocation:

```sh
go run ./cmd/wrk run --ready --label web --expect-status done -- \
  codex exec --dangerously-bypass-approvals-and-sandbox 'Implement {id}'
```

Each selected ticket gets one fresh Codex call with exactly `Implement <ticket-id>`
as one argument. The supplied Codex flags preserve full local access without approval
prompts. Configure Codex for noninteractive use beforehand. Repository instructions
and the ticket define implementation and verification. Exit 0 alone is insufficient
for a burn: `--expect-status done` checks the persisted ticket in the selected project
before counting success and selecting again. Newly unblocked work can run next.

Selection uses native `list` semantics and ascending ID order, not priority order.
`--ready` means todo with every dependency done and excludes in-progress work.
Repeated labels require all values; `--under` selects descendants but excludes the
parent itself. Combine them or bound a trial to one verified success:

```sh
wrk run --ready --under <parent-id> --label web --label backend \
  --max-tickets 1 --expect-status done -- \
  codex exec --dangerously-bypass-approvals-and-sandbox 'Implement {id}'
```

Ordinary runs exit when no unprocessed matches remain. An empty ready queue does
not establish project completion. Explicit continuous mode waits for previously
unprocessed tickets to become eligible, polling only while idle:

```sh
wrk run --ready --label maintenance --stream --poll-interval 5s \
  --max-tickets 3 --expect-status done -- \
  codex exec --dangerously-bypass-approvals-and-sandbox 'Implement {id}'
```

`--max-tickets` counts successful actions; it is a count of done tickets only when
done was required and verified. It is not a wall-clock deadline: streaming can wait
indefinitely without reaching the limit. Ctrl-C stops an idle wait or the active
process group. One sequential runner does not claim tickets against another worker;
coordinate workers externally and inspect existing in-progress work before a burn.

For an unchanged-status review, omit the done requirement:

```sh
wrk run --all --under <parent-id> -- ./review-ticket '{id}'
```

Here success is exit 0 with a valid existing ticket. Each successful ID is processed
once per invocation even if it still matches, leaves and re-enters the query, or is
reopened. Streaming also skips those IDs across polls. A fresh invocation can visit
them again. Failures stop immediately; no mode automatically retries.

### Thin Python convenience wrapper

[`scripts/ticket-burn.py`](../scripts/ticket-burn.py) is retained as an exec-only
wrapper around `wrk run`. It requires Python 3.9+ and macOS/Linux. It adds the
Codex command, the exact prompt, and the required done check; wrk owns all selection,
process management, completion checks, logs, and output:

```sh
python3 scripts/ticket-burn.py --ready --label web --max-tickets 1
python3 /path/to/wrk/scripts/ticket-burn.py --ready --label maintenance --stream
```

Pass at least one selector (`--ready`, `--all`, `--label`, `--under`, or `--ticket`).
The wrapper forwards native filters, project/config selection, streaming, intervals,
limits, `--log-dir`, `--verbose`, and `--json`. `--wrk-command` defaults to
`go run ./cmd/wrk` when launched at this checkout's root and `wrk` elsewhere. An
explicit command replaces that prefix. It now supplies the entire `run` invocation,
not just a completion-check executable.

`--codex-command 'codex exec --dangerously-bypass-approvals-and-sandbox --model MODEL'`
customizes Codex options. It replaces the entire default, including permission flags;
the wrapper appends `Implement {id}`. These two command strings support shell quoting
to preserve argument boundaries, but never shell expansion. Use direct `wrk run` for
other actions or success conditions; the wrapper always requires done.

### Migration from the old script

| Former interface/behavior | Current equivalent/change |
| --- | --- |
| `--list-command 'go run ./cmd/wrk list --ready --label web --json'` | Pass `--ready --label web` to the wrapper or the direct native invocation above. `--list-command` is rejected. |
| Arbitrary external selection wrappers/custom ordering | No automatic mapping: choose supported native filters and ID ordering, or keep custom selection outside this workflow. wrk does not execute external list/show commands. |
| `--wrk-command` for `show` only | Executable/prefix for the whole `wrk run`; use native `--project`/`--config` to pin the project. |
| `--heartbeat 60` | `--heartbeat-interval 60s`; positive Go durations replace integer seconds. The old flag is rejected. |
| `.wrk-burn/<timestamp>/` in launch cwd | `.wrk-runs/<timestamp>/` in the selected project; `--log-dir PATH` overrides the base relative to launch cwd. Existing logs are left in place. |
| Combined `001-<id>.log` and list/show `run.log` | Separate numbered stdout/stderr files plus lifecycle `run.jsonl`; see the [log contract](cli.md#run-output-and-logs). |
| Repeated selection was an error | Successful IDs are skipped for this invocation; unchanged-status reviews can drain normally. |
| Empty selection always stopped | Ordinary mode still stops; explicit `--stream` waits for unprocessed eligible work. |
| Script-specific status restrictions | Native query rules apply. Use `--ready` for implementation queues; explicit `--ticket` retries can select in-progress work. |

Add `/.wrk-runs/` to the selected project's `.gitignore`. Keep `/.wrk-burn/` ignored
if retaining old logs. No log format is a workflow checkpoint or resume database.

### Arguments, execution, and output

Direct `wrk run` requires `--` before the executable. Everything after it belongs
to the child, including flags such as `--json`. Quote `'Implement {id}'` to preserve
the prompt as one argument. Every literal `{id}` in child arguments is substituted;
the executable itself is never substituted. There is no implicit shell, expansion,
pipeline, redirection, dotenv loading, or interactive prompting. Exported environment
and PATH are inherited, and child stdin is closed. A directly executed script needs
executable permission and a valid interpreter/shebang; otherwise invoke the
interpreter explicitly, for example `-- python3 'scripts/action with spaces.py' '{id}'`.

Children run in the selected project root, so relative script paths, PATH entries,
and child file arguments resolve there. CLI `--project`, `--config`, and `--log-dir`
paths resolve from the invocation directory. From another directory, use a freshly
built absolute wrk executable and `--project /path/to/project` (or its config).
The runner holds no lifetime writer lock: children can invoke wrk mutations.
Direct body/config edits and Git operations remain outside those mutations.

Human progress reports selection, starts/finishes, elapsed time, output recency, idle
reasons, and failure/retry guidance. Default child output is captured in full logs.
`--verbose` mirrors child bytes to stderr and disables terminal line replacement.
`--json` emits version-1 lifecycle NDJSON to stdout, including a final `finished`
event with `outcome`; even with verbose, raw child output stays off JSON stdout.
See [output fields, log paths, and failure details](cli.md#run-output-and-logs).

SIGINT/SIGTERM stops the active process group, escalating to SIGKILL within two
seconds for an unresponsive leader and cleaning up remaining group descendants.
Partial changes remain. Exit codes are 0 for a drained query, success limit, or
successful explicit ticket; 1 for execution/completion failures; 2 for usage errors;
and 130 for interruption. For supervision that signals a single PID, use a compiled
wrk binary via `--wrk-command /absolute/path/to/wrk` rather than the `go run` launcher.
Use the compiled binary when exact exit codes matter too: `go run` can report a
nonzero child status on stderr while itself exiting 1 (for example, usage status 2).

### Inspection and explicit retry

Exit 0 means the Codex session ended normally; it does not establish acceptance.
If the required done check fails, read the numbered stdout/stderr logs and ticket
handoff. The former workspace sandbox sometimes prevented required verification;
full local access remains the supplied command's default, not a generic wrk setting.

After inspecting and repairing partial work, retry the unfinished ticket explicitly:

```sh
wrk show <failed-id>
wrk run --ticket <failed-id> --expect-status done -- \
  codex exec --dangerously-bypass-approvals-and-sandbox 'Implement {id}'
# Equivalent convenience wrapper:
python3 scripts/ticket-burn.py --ticket <failed-id>
```

`--ticket` bypasses readiness without resetting status, and conflicts with query
filters and streaming. A ready queue will not resume in-progress work automatically.
Restart the queue after resolving the failed ticket and its verification.

A multi-step action follows the same rule:

```sh
wrk run --ready --label maintenance --expect-status done -- \
  /bin/sh 'scripts/implement and verify.sh' '{id}'
# Inspect logs and durable results, repair the failure, then restart the WHOLE script:
wrk run --ticket <failed-id> --expect-status done -- \
  /bin/sh 'scripts/implement and verify.sh' '{id}'
```

The script must keep the ticket unfinished until all required steps succeed, check
step failures, and mark done only at the end. Its author owns safe repetition of
partial effects: for example, verify and reuse an existing first-step result before
trying the second step again. Agent calls are not inherently idempotent. wrk provides
no named steps, checkpoints, automatic retries, rollback, or automatic Git operations.

[Integrated migration verification](burn-verification.md) maps disposable-project
and fake-command checks to the workflow. No real Codex session is needed to run them:

```sh
go test -count=1 ./internal/runner ./internal/cli ./test/integration
python3 -B -m unittest discover -s scripts -p 'test_ticket_burn.py' -v
```

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
For bounded/manual burns, stop rather than polling indefinitely. Explicitly requested
`wrk run --stream` may keep waiting within its fixed scope until interrupted, failed,
or limited. Never silently expand the batch or mark unfinished work done.

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
