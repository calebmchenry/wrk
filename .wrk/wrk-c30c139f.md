---
id: wrk-c30c139f
title: Add a minimal command-driven Codex ticket burn script
status: done
priority: normal
labels:
  - automation
---
## Outcome

Provide a small standalone script that repeatedly runs a user-specified wrk list command/filter and invokes a fresh Codex session to implement one selected ticket per iteration. The user emphasized simplicity, progress output, logs, and a short implementation prompt. On 2026-10-08 the user authorized implementation and clarified that the prompt should not explicitly prohibit skills; just avoid copying the reference script's skill invocation.

## Implemented approach

- Add scripts/ticket-burn.py using only the Python standard library.
- Accept a selection command that returns the ordinary wrk list --json envelope; execute it again before each selection so completed dependencies can reveal more work.
- Select the first returned ticket, respecting the command's ordering and scope.
- Run a fresh codex exec with exactly the prompt `Implement <ticket-id>`.
- Let repository instructions and the ticket supply the implementation, verification, and status-update workflow.
- Print ticket number/ID/title, start/end, elapsed time, log location, and periodic still-running messages. Preserve complete child output in per-ticket logs plus a run progress log.
- After Codex exits, use wrk show --json in the same project to verify the ticket is done before advancing. A successful process exit or disappearance from --ready does not establish completion.
- Stop on an empty successful query, command/JSON errors, a failed Codex invocation, or an unfinished ticket. Report an empty query as no matching work, not all project work complete.
- Keep query execution and Codex execution in the intended project. Use go run ./cmd/wrk for this checkout.
- A small optional maximum-ticket count is useful for trying a run. Avoid adding sprint orchestration, manifests, state databases, automatic Git operations, or automatic retries.

## Reference and findings

Reviewed ../build-wars/scripts/ticket-burn.py, especially run_child's timestamped progress, elapsed-time heartbeat, and per-child stdout/stderr log. Its sprint planning/execution architecture is unnecessary here. Existing wrk-d965ba66 delivered filters, blocked status, and the documented manual burn workflow; it explicitly excluded an AI runner.

wrk list --json supplies result.tickets; an empty list is valid. --ready means todo with completed dependencies and excludes in-progress work. Current lists are ordered by ID, not priority. Preserve the supplied selection order for the minimal initial version.

## Acceptance criteria

- [x] A specified JSON selection command is re-run after each completed ticket, and each Codex invocation implements just one ticket with no skill invocation in the prompt.
- [x] Console progress and durable logs expose the current ticket, elapsed time, completion, and stopping reason.
- [x] Errors and unfinished tickets stop clearly without an automatic retry loop; interruption stops the active child cleanly.
- [x] Document invocation, command output contract, log locations, and the empty-query meaning in indexed docs.
- [x] Verify with fake commands, including new work appearing after completion, empty selection, malformed/error output, child failure, unfinished status, and interruption; do not burn the real backlog as a test.
- [x] Run wrk validate before handoff.

## Implementation and verification

Completed on 2026-10-08. `scripts/ticket-burn.py` is a 203-line executable using only the Python standard library. It accepts a required list command and optional wrk/Codex command overrides, ticket limit, log directory, and heartbeat interval. The original default child was `codex exec --sandbox workspace-write`; this was changed at the user's request on 2026-10-09 as recorded below. The prompt is a single argument with exactly `Implement <ticket-id>`.

- Commands use argument splitting with quoted arguments supported, without implicit shell execution. Selection/checks retain the launch cwd; Codex runs in the selected project, and completion checks use an explicit --project selector. The same project is required throughout a run.
- Timestamped run directories contain run.log and numbered per-ticket stdout/stderr logs. The script stops on command/data errors, unfinished tickets, repeated selections, or a selected status outside todo/in-progress. Empty selection reports no matching tickets. SIGINT/SIGTERM stop the active process group, including descendants; interruptions exit 130.
- README, indexed docs/burns.md, and --help describe usage and limits. `.wrk-burn/` is ignored. Added the Python tests to the Verify CI matrix.
- `python3 -B -m unittest discover -s scripts -p 'test_ticket_burn.py' -v` passed on macOS and Linux (Docker golang:1.25.4-bookworm with --init). Six tests include 14 failure subcases, exact prompts/cwd, dependency-style re-querying, logging, heartbeat, limits, invalid arguments, missing commands, and SIGINT/SIGTERM with a descendant that ignores SIGTERM. No real Codex session ran.
- A separate disposable-project smoke used the real current wrk CLI, its default go-run completion command, and a fake Codex process: first ticket completed, second became ready through its dependency, second completed, and the empty selection stopped the loop. The temporary project and logs were removed afterward.
- `gofmt -l cmd internal test` reported no files. `go test -count=1 ./...`, `go test -race -count=1 ./...`, `go vet ./...`, and `go build -o ./bin/wrk ./cmd/wrk` passed on macOS and Linux. The Linux snapshot was copied to container-local storage. `git diff --check` and `go run ./cmd/wrk validate` passed (26 tickets).
- Existing concurrent browser implementation changes, including README/index and .gitignore additions, were preserved. This task does not commit files or run the real backlog.

## Remaining work

None. Real Codex authentication/model execution was not exercised; CLI flags were checked with the installed `codex exec --help`. The runner relies on repository instructions and the child agent for implementation checks and ticket updates; it verifies the persisted done status before proceeding.

## Run investigation (2026-10-09)

Investigated `.wrk-burn/20261008-213550-783709` after the user reported that the loop stopped after its first invocation. This was the completion guard, not a one-ticket limit or timeout. The log records `codex exec --sandbox workspace-write 'Implement wrk-ce7350ff'`, approval policy never, and an 805-second successful session exit. The child's final response explicitly leaves the ticket in-progress because server/browser verification was blocked by sandbox permissions.

Evidence: loopback bind failures (`operation not permitted`, first at ticket log line 4967), Docker socket denial (line 5381), and Chromium MachPortRendezvous IPC denial (line 6996). The ticket's handoff records implementation, passing restricted checks, and the remaining unfiltered Go/race, browser, screenshot, and Linux verification. A zero Codex exit status did not mean those criteria passed.

All remaining todo descendants of wrk-a1561423 depend directly or indirectly on wrk-ce7350ff. The ready query is currently empty; removing the completion guard or merely rerunning the same query would not advance this dependency chain. Recovery is to finish the current in-progress ticket using explicitly chosen permissions that permit the required verification, then restart the ready queue with those same child permissions. No task status was reset or falsely completed, no backlog execution was started, and the runner's permission defaults were not widened as part of this diagnosis.

Added this recovery workflow to the already indexed docs/burns.md. Verified installed Codex help and official noninteractive/sandbox documentation. This follow-up changes documentation only; `go run ./cmd/wrk validate` and `git diff --check` pass.

## Full-access default (2026-10-09)

The user explicitly requested full power by default without having to specify it each run. Changed the script's default child command to `codex exec --dangerously-bypass-approvals-and-sandbox`, which the installed CLI documents as disabling sandboxing and confirmation prompts. Explicit --codex-command overrides still replace the whole command. Updated README and the indexed runner documentation; the short implementation prompt, completion checks, and selection behavior are unchanged.

Verification passed: all six existing fake-command tests, plus a disposable PATH-based fake Codex smoke that omitted --codex-command and asserted both successive child invocations received `exec --dangerously-bypass-approvals-and-sandbox` and the exact ticket prompt. --help reports the new default; `git diff --check` and `go run ./cmd/wrk validate` pass (26 tickets). No real backlog burn was started and no global Codex settings were changed; this preference is the runner's default.
