# Burn migration verification

This records wrk-ef04f589's migration of Codex burns to `wrk run` on 2026-10-10.
All actions used fake executables and disposable projects. No real AI session,
backlog burn, release publication, commit, or push was performed. The subsequent
[parent acceptance review](#parent-acceptance-review) completes wrk-0a233d21.

## Behavior and coverage

| Requirement | Evidence |
| --- | --- |
| One orchestration engine | `scripts/ticket-burn.py` constructs native arguments and uses `os.execvp`; it has no selection, subprocess, completion-check, or logging loop. Python tests assert PID preservation, inherited environment/cwd, checkout versus installed defaults, quoted prefixes, native options, and exit propagation. |
| Exact Codex defaults and done requirement | Python's `test_full_access_default_and_checkout_cli` checks the default `codex exec --dangerously-bypass-approvals-and-sandbox` and single `Implement {id}` argument. `TestRunCodexBurn` executes a fake `codex` from an isolated PATH through the compiled CLI, requires done, and verifies fresh dependency selection plus nonzero/exit-0-not-done failures. |
| Selection, re-query, once per invocation, and limits | `TestRunSelectionAndRequery` covers dependency unlocking, arrivals during actions, AND labels/descendant root exclusion, unchanged statuses, fresh invocations, empty queries, and positive success limits. `TestScopedBurnWorkflow` retains the original real CLI scope/readiness/blocked workflow checks. |
| Streaming and all-processed queues | `TestRunStreamWhileIdle` covers actual arrivals, project pinning, invalid data, and idle SIGINT/SIGTERM. Runner tests `TestStreamEligibilityOnLaterPoll`, `TestStreamProcessedIDsSurvivePollsAndReentry`, `TestStreamScopeChangesAndIdleInterruption`, and `TestStreamLaterFailuresStop` cover eligibility changes, all-processed idle queues, reopened IDs, and failures without retries. |
| Explicit retry after a partial multi-step failure | `TestRunMultiStepExplicitRetry` runs a non-executable `scripts/two steps.sh` using `/bin/sh` from a different launch directory. Step one saves a durable result; step two fails with exit 7, leaving in-progress. The run stops before the next ticket. After external repair, `--ticket` restarts both steps, reuses the first result without rewriting it, finishes step two, and persists done. Ready excludes the failed in-progress ticket. No runner checkpoint, rollback, or automatic retry participates. |
| Arguments, environment, paths, and writes | `TestRunArgumentsEnvironmentAndProject` verifies literal argv/metacharacters, empty args, exported env/PATH, relative PATH from project cwd, ignored `.env`, closed stdin, explicit project/config selection from another project, and pinned completion checks. The new multi-step and fake Codex actions invoke CLI mutations while running, demonstrating the absence of a lifetime writer lock. |
| Errors stop immediately | `TestRunFailureAndRetry` covers missing commands, usage errors, nonzero exits, missing/invalid tickets, status mismatch, invalid projects, and explicit retries. The new fake Codex cases require in-progress to remain unfinished after either failure mode. |
| Logs, heartbeats, and JSON/verbose | `TestRunRecordsAndChildIsolation`, `TestHumanRunningAndIdleOutput`, `TestRunJSONFlushesWhileChildIsAlive`, and `TestRunNoisyChildFullLogs` cover ordered/flushed lifecycle envelopes, human state/counts, actual output recency, JSON stdout isolation with verbose stderr, and separate complete 8 MiB stdout/stderr logs. Existing output-failure tests stop the active action before another starts. |
| Interruption and descendants | `TestRunInterruptProcessGroup` exercises actual SIGINT/SIGTERM with responsive and ignoring leaders, ignoring descendants, ordinary/stream modes, partial changes, and exit 130. `TestRunClosedJSONPipeStopsProcessGroup` verifies cleanup when output disappears. Linux zombie state is distinguished from running processes. |
| Wrapper migration | Four Python tests replace the six old orchestration tests. Applicable selection/process/log behavior now lives in CLI/runner tests above. External list/show envelope validation and project-switching wrappers no longer apply because those commands are not executed; malformed project validation and fixed native project selection remain covered. Retired flags are rejected, including argparse abbreviation of `--heartbeat`. CI retains wrapper tests and runs orchestration coverage in Go normal/race suites. |

## Native execution and build checks

The verification uses Go 1.25.4. Native platforms:

- macOS 15.7.7 arm64, local filesystem workspace and disposable temporary projects.
- Debian 12.12 Linux arm64, `golang:1.25.4-bookworm`, Docker `--init`, UID/GID
  1000:1000. Source was copied from a read-only host mount to `/work/wrk`; tests
  run on the container-local filesystem. `/tmp` has no project ancestor.

Commands:

```sh
test -z "$(gofmt -l cmd internal test)"
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
go build -o ./bin/wrk ./cmd/wrk
# Use an isolated absolute installation directory, not the user's normal install:
GOBIN=/absolute/verification/bin go install ./cmd/wrk
/absolute/verification/bin/wrk version --json
python3 -B -m unittest discover -s scripts -p 'test_ticket_burn.py' -v
python3 scripts/verify-workspace.py ./bin/wrk
go run ./cmd/wrk validate
```

All listed checks pass on both native platforms. macOS integration execution took
33.646s normally and 82.913s with race detection; Linux took 27.205s and 72.069s.
Formatting reports no files, `git diff --check` passes, and final project validation
reports 40 valid tickets. All 15 browser model tests also pass on macOS; no browser
UI code was changed here. No required verification remains unresolved.

Both amd64 builds (`GOOS=darwin` and `GOOS=linux`, `GOARCH=amd64`) compile. This is
cross-build evidence only, not amd64 runtime verification. The existing CI matrix
covers native macOS/Linux amd64/arm64; hosted CI was not run in this local session.

## Integrated wrapper smoke and documentation review

An additional disposable-project smoke runs the installed binary through the actual
Python wrapper and a fake `codex` on an isolated PATH. It verifies two separate
processes, exact argv/prompt, persisted done, one-success limit, subsequent drain,
empty queue, selected project cwd from another launch directory, JSON plus verbose
stderr, and invocation-relative log overrides containing spaces. Captured lifecycle
events are independently decoded; full child logs are retained. This smoke passes
on both macOS and Linux with their native installed binaries.

Local captures and the reproducible smoke driver are under the ignored
`.wrk-runs/burn-verification-20261010/`, including native check output,
`macos-smoke/`, `linux-smoke/`, and CLI/wrapper help. Tests remain in the tracked suites above;
these diagnostic captures are not resumable state.

Reviewed `wrk run --help` and wrapper help against README, docs/cli.md, and
docs/burns.md. The migration documents native filters replacing `--list-command`,
`--heartbeat-interval` replacing integer `--heartbeat`, new log paths/formats,
processed-ID skipping, explicit stream waiting, and default action success versus
required done. It also documents script interpreters, environment/PATH, project cwd,
argument separation, no interactive input/dotenv/shell, partial-failure recovery,
and independent-worker coordination. Server-only historical no-runner wording and
manual no-indefinite-polling guidance now allow separate explicit `wrk run --stream`.

Compiled CLI exit codes are 0/1/2/130. A checked usage failure through `go run`
reports `exit status 2` on stderr while the launcher exits 1; documentation recommends
the compiled binary when supervising one PID or requiring exact exit codes.

## Parent acceptance review

Reviewed [wrk-0a233d21](../.wrk/wrk-0a233d21.md) on 2026-10-10 after all four
children completed. The integrated implementation meets the parent's accepted
behavior and all six criteria. No additional production-code changes were needed.

| Parent criterion | Acceptance evidence |
| --- | --- |
| Native scoped execution, success checks, and explicit retry | Reviewed parser, shared list selection, runner, and process handling. Fresh tests exercise argument boundaries, environment/PATH/cwd, project pinning, immediate re-query, unchanged-status success, persisted status checks, failures, whole-script retry, and SIGINT/SIGTERM process groups. |
| Efficient continuous waiting | Reviewed interruptible idle timers and invocation-local processed IDs. Fresh stream tests cover arrivals, eligibility changes, all-processed queues, reentry/reopening, scoped counts, prompt interruption, and terminal failures. No empty-ready result claims project completion. |
| Accurate human output, logs, and JSON | Reviewed the shared recorder and event schema against the CLI docs. Fresh tests cover lifecycle ordering/flushing, success versus completion, measured output activity, full binary logs, retry quoting, output/log failures, and broken-pipe cleanup. The output child's [terminal/plain/JSON review](run-output-verification.md) remains applicable. |
| Codex burn integration | Both installed native binaries passed the real Python wrapper with an isolated fake Codex: exact command/prompt, fresh processes, required done, limits, drain/empty, selected cwd, JSON plus verbose stderr, and invocation-relative log overrides. |
| Platform checks and indexed documentation | Fresh macOS/Linux checks below pass. Actual CLI/wrapper help matches the [command contract](cli.md#scoped-command-execution) and [migration/recovery guide](burns.md#minimal-codex-runner). |
| Separate parent review | Compared the implementation, tests, documentation, and child handoffs with every parent requirement; reviewed the boundaries as well as the delivered behavior before checking the parent checklist. |

Fresh verification used Go 1.25.4 on macOS 15.7.7 arm64 and Debian 12.12 Linux
arm64. Linux used `golang:1.25.4-bookworm`, Docker `--init`, UID/GID 1000:1000,
and a source copy on the container-local `/work/wrk` filesystem; temporary projects
were outside repository ancestors.

On both platforms, formatting, full uncached normal/race Go suites, vet, build,
isolated install/version checks, all four wrapper tests, workspace smoke, and the
installed-wrapper smoke passed. Integration package times were 34.712s normal /
81.787s race on macOS, and 27.783s normal / 71.871s race on Linux. All 15 browser
model tests and `git diff --check` passed on macOS. Final repository validation
reports 40 valid tickets.

Fresh diagnostic captures are under ignored `.wrk-runs/parent-acceptance-20261010/`,
including normal/race logs, the Linux check driver, help text, and both native
wrapper smoke captures. The smoke reused the driver from the child verification.
The child evidence above supplies amd64 cross-builds; no amd64 runtime, hosted CI,
or real agent session was performed for this review. No required acceptance work
remains.
