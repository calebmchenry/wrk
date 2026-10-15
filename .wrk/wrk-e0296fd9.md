---
id: wrk-e0296fd9
title: Make wrk run output concise and readable with multiline commands
status: done
related:
  - wrk-7b831fa1
  - wrk-b92b4796
priority: normal
labels:
  - automation
  - run-cli
---
## Outcome

Make the default human output of `wrk run` easy to scan during long agent actions and after a failure, including when command arguments contain a large multiline prompt. Users should immediately see what is running, the outcome, and how to inspect or recover.

## Reported example

On 2026-10-15, the user supplied output from `./scripts/run-queued.sh` in `/Users/calebmchenry/code/mmo-prototyping`. It selected ready tickets labeled `queued` and ran `codex exec` with a multi-paragraph implementation prompt and an expected persisted status of `done`.

The transcript shows:

- The entire prompt printed three times: startup action template, expanded command at action start, and explicit retry command after failure.
- Text running together at a boundary: `commit hash.'Success: exit 0 and valid existing ticket; persisted status done`.
- Large, irregular leading spaces in prompt paragraphs and before the show command, plus long log paths and metadata crowding the output.
- Repeated selection/order descriptions and failure details obscuring the useful result.
- Ticket `wrk-8f9b4f7e` ("Prototype orthographic follow camera with 45-degree rotation and zoom") finished after `15m54s` with child exit 0 but persisted status `in-progress`. The runner correctly rejected completion because `done` was required, reporting zero successful actions and `RUN_COMPLETION: expected status done; observed in-progress`.

These are observations from the pasted transcript, not a verified diagnosis of its terminal rendering. The terminal configuration, wrapper contents, wrk version, and raw capture were not supplied. Investigate newline handling, wrapped progress lines, and stdout/stderr interactions before attributing a root cause. The transcript does not establish why the child left the ticket unfinished.

The user also finds the wall of white text numbing and wants heartbeats to be easier to parse at a glance. Add restrained color and visual hierarchy so the current activity and changing values stand out from supporting details.

## Scope and acceptance criteria

- [x] Use a compact default startup summary and clear per-ticket start, progress, finish, and final-result records. Keep project, effective selection, mode, success condition, ticket identity/title, elapsed time, and relevant counts understandable without repeatedly printing unchanged configuration.
- [x] Give heartbeats a consistent, compact layout with a recognizable running label, ticket ID, elapsed time, and last observed child-output age (or explicit no-output text). Emphasize the state and changing values so they are easy to locate without rereading a sentence; keep ancillary details visually secondary. For example: `RUNNING  wrk-8f9b4f7e  |  elapsed 08m12s  |  last output 4s ago` (illustrative, not a fixed format). Silence alone must not be styled or labeled as a warning, failure, or hang.
- [x] Use restrained semantic color and emphasis for human terminal output: distinguish running, waiting, successful completion, and failure consistently across heartbeats and milestones. Keep explicit text labels so color is never the only signal, and check readability on light and dark terminal backgrounds. Honor `NO_COLOR` and disable styling for `TERM=dumb`, redirected destinations, and JSON output. Decide color support per output destination; wrk styling must not leak into command arguments, structured logs, or raw child log bytes, or bleed into subsequent output.
- [x] Avoid dumping the full multiline prompt at both startup and each action start. Use a concise command description by default and make the complete template and expanded argv discoverable in the existing logs or an explicit detail view. Display shortening must never change the executed arguments.
- [x] Render multiline content and long paths with consistent line boundaries and deliberate spacing. Reproduce and address any merged records or staircase indentation in supported terminal conditions. Heartbeat updates must not leave stale wrapped lines or overwrite permanent milestones at narrow terminal widths; resizing must remain readable.
- [x] Present an exit-0/status-mismatch failure plainly: child exit 0, expected `done`, observed `in-progress`, ticket ID, elapsed time, and zero successful actions in this example. Distinguish it from process failure, and avoid redundant prose obscuring the cause or suggesting the ticket completed successfully.
- [x] Group actionable recovery information clearly: labeled run/stdout/stderr log locations, a project-pinned show command, and access to an exact copyable explicit-ticket retry. Keep large retry commands separate from the concise result; explain where to obtain them if omitted from the default display. Preserve shell quoting, multiline arguments, `{id}`, project selection, and required status. Retain the instruction to inspect partial changes before retrying from the beginning.
- [x] Preserve full child logs, exact command data, JSON event compatibility, and output-error handling. Redirected human output and `TERM=dumb` remain readable append-only text without terminal controls; `--verbose` continues to work without corrupting progress or JSON stdout.
- [x] Add focused regression coverage and manually review a real PTY plus redirected output using disposable projects and fake children. Cover a long multiline argument, long paths, narrow/wide terminal widths and resize, repeated heartbeats, successful completion, nonzero child exit, and exit-0/status mismatch. Review heartbeat scanability with color enabled and disabled, light/dark backgrounds, `NO_COLOR`, `TERM=dumb`, and mixed terminal/redirected stdout and stderr. Verify exact retry argv round-trips and JSON parseability. Update the output contract and verification notes with representative before/after examples.

## Implementation starting points

- `internal/cli/run_output.go`: startup/action/final human rendering, shell-quoted commands, terminal heartbeat replacement, and output destinations.
- `internal/cli/run_output_test.go` and existing CLI/integration coverage: renderer behavior, retry fidelity, and stream isolation.
- [Output contract](../docs/cli.md#run-output-and-logs) and [prior verification](../docs/run-output-verification.md).
- Completed output work: [wrk-7b831fa1](wrk-7b831fa1.md). Recovery investigation: [wrk-b92b4796](wrk-b92b4796.md).

Keep this focused on presentation. Selection, completion checks, ticket status, and retry policy stay as defined today; no automatic retries, status resets, inferred agent phases, or full-screen TUI are needed.

## Implementation and verification (2026-10-15)

Implemented compact human rendering in `internal/cli/run_human.go` with append-only
RUN/START/RUNNING/WAITING/OK/FAILED/DONE records. Status labels use restrained ANSI
color, changing values use bold, and supporting text keeps the default foreground.
Styling is decided per destination and disabled for NO_COLOR, TERM=dumb, redirected
streams, and JSON. Every styled span resets. No silence warnings or inferred phases
were added.

Startup describes the executable and argument count once; exact command templates
and expanded argv remain in run.jsonl. Failure recovery groups a shared directory
and labeled log filenames, a project-pinned show command, and a cat command for a
private, synced retry.sh. The file preserves exact shell quoting, multiline arguments,
{id}, project selection, and required status, and is never automatically executed.
Existing JSON fields/events remain intact, with an additive retry_file field.

Reproduced three prompt copies, bare-LF staircase indentation with OPOST disabled,
a merged partial verbose child-output/finish boundary, and single-row heartbeat
clearing at 36 columns using the prior renderer from 6e7182b. This does not establish
the original reported terminal's configuration. Append-only output avoids stale rows
and milestone overwrites on resize. Explicit terminal line endings and verbose
boundaries keep wrk records aligned; raw child logs and JSON live bytes are unchanged.

Verification:

- Full uncached Go tests, race tests, vet/build, formatting, four burn-wrapper tests,
  and seventeen browser model tests passed on macOS 15.8.1 arm64 and Debian 12.12
  Linux arm64 with Go 1.25.4. Linux used a local source copy, --init, and UID 1000.
- Final CLI/integration normal/race reruns cover the final verbose-newline adjustment.
  Regression coverage includes exact retry-file shell execution from another project,
  long multiline prompts, mismatch vs process failure, escaped values/style resets,
  stream isolation, and retry-artifact creation failure. Existing binary-log and
  broken-pipe/output-error regressions pass.
- `scripts/verify-run-output.py` passes twelve PTY/redirected scenarios on both
  platforms: long arguments/paths; 36/100 columns and 100→36→120→52 resizing;
  repeated silent/noisy heartbeats; success, exit 7, and exit-0/done/in-progress
  mismatch; raw-terminal partial/newline child output; NO_COLOR, TERM=dumb,
  both mixed destination directions, JSON+verbose, and idle SIGINT.
- Manually reviewed actual captures and light/dark theme renderings, including
  narrow wrapping and no-color text. Raw logs and executed argv remain exact;
  JSON parses and matches run.jsonl. No real agent was invoked.
- Final checkout validation passed with 46 tickets; formatting and diff checks are clean.
- Updated output contract, development instructions, and before/after verification
  evidence in docs/run-output-verification.md, linked from docs/index.md.

## Handoff

Acceptance is complete; no implementation work remains. Existing unrelated changes
to wrk-ef04f589 and the pre-existing wrk-b92b4796 ticket were preserved. Diagnostic
captures are ignored under `.wrk-runs/compact-output-accepted-20261015/`; baseline
reproductions are under `.wrk-runs/compact-output-final-20261015/`.
