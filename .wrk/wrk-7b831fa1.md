---
id: wrk-7b831fa1
title: Explain run selection, progress, waiting, and failures
status: done
parent: wrk-0a233d21
depends_on:
  - wrk-8e95508a
priority: normal
labels:
  - automation
  - run-cli
---
## Outcome

Users can tell what wrk run is selecting, why a ticket was chosen, whether the child is alive, and why the run is waiting or stopped. Full child output remains available in logs; machine consumers receive structured lifecycle events.

Depends on [wrk-8e95508a](wrk-8e95508a.md), which transitively provides the core action engine. Read the shared contract in [wrk-0a233d21](wrk-0a233d21.md).

## Human output

- Print project/cwd, effective selection, ordering, mode/idle interval, action template, configured success condition, success-count limit if any, and run-log location once at startup.
- Derive readable filter descriptions from the actual parsed selection: ready means todo with all dependencies done; repeated labels mean all supplied labels; descendants include the parent's title/ID and exclude the parent itself. Preserve intersection semantics instead of loosely paraphrasing the query.
- At selection, show a concise filter description and meaningful candidate counts. Distinguish unprocessed matches from already processed matches; do not imply a fixed total queue because new work can arrive.
- At action start, show sequence number, ticket ID/title/priority, and child-log path. Show the ordering rule so the user understands why this candidate is first.
- Keep permanent start/finish/error milestones. In an interactive terminal update one running line in place; redirected output/logs must remain readable append-only text without cursor-control sequences. Retain configurable approximately 60-second heartbeats, readable elapsed durations, and actual output recency where available.
- Do not infer implementation/test phases, percent complete, ETA, or a hang from silence. With no child output, say so instead of inventing output recency.
- Entering idle should explain no unprocessed matches and show useful unfinished-work counts within the same label/descendant scope. Distinguish in-progress, manually blocked, and todo waiting on dependencies. Readiness-filter removal is for diagnostics only and never expands execution eligibility. Use consistent snapshot counts and avoid double counting.
- All-processed matches need their own explanation; successful actions do not necessarily imply tickets done. Print idle explanations when entering idle or the explanation/counts change, rather than every poll.
- Report process failure separately from exit-0/required-status mismatch. Include observed status when available, the ticket/log, a show command, and a shell-quoted explicit-ticket rerun command preserving the selected project, action arguments, and success requirement.
- On a handled stop, report successful-action count, elapsed time, actual stopping reason, failed/current ticket where relevant, and log location. Call actions successful/processed unless done was actually verified. Empty-query exit does not claim all scoped work complete.

## Logs and structured output

- Create timestamped run directories and numbered per-action stdout/stderr logs; record selected project, command, start/end, child exit, elapsed time, and completion-check result. Stream output to files without retaining an unbounded in-memory transcript.
- Use the parent's initial .wrk-runs location/override rule, expose it early, and add appropriate ignore guidance. Do not dump the inherited environment or make logs a workflow checkpoint/resume database.
- Offer explicit live child-output verbosity for troubleshooting while retaining logs. Define its interaction with terminal updating and JSON rather than interleaving raw child bytes into structured stdout.
- Use a documented newline-delimited JSON event contract consistent with wrk's versioned envelope conventions. Include project, event, ticket/attempt identity when relevant, timestamps/durations, selection/counts, child exit, required/observed status, diagnostics, log paths, and final reason as applicable.
- Flush events promptly. Human text, terminal controls, and raw child output must not corrupt JSON stdout. Follow existing help/pre-start-error conventions; do not promise a terminal event after SIGKILL or failed output.
- Logging/output errors must be reported and terminate active execution cleanly rather than silently running additional tickets without the promised record.

## Acceptance criteria

- [x] Implement the startup, selection, action, heartbeat, idle, failure/retry, and final output using the shared execution facts.
- [x] Implement full child/run logs, path overrides, terminal versus redirected rendering, and explicit child-output verbosity.
- [x] Define and document the JSON event schema and stdout/stderr behavior; use one execution path for human and JSON modes.
- [x] Test precise combined filter descriptions, root exclusion, parent titles, raw/processed/candidate counts, and empty-queue diagnostics without eligibility expansion.
- [x] Test exit failure versus completion mismatch, successful unchanged tickets, retry argument quoting/project selection, changing idle state, readable redirected output, and no invented progress.
- [x] Verify JSON parseability/order/prompt flushing, noisy child output isolation, full logs, log/output failure handling, and interruption summaries where output remains writable.
- [x] Inspect an actual terminal run plus captured plain/JSON output using fake commands; no real backlog burn is needed.

## Scope

Keep a readable terminal transcript, not a full-screen TUI. No AI-generated log summaries, semantic interpretation of arbitrary child output, progress percentages, or persistent workflow state.

## Implementation completed (2026-10-10)

- Extended the shared engine with startup and heartbeat events, action sequence,
  timestamps/durations, completion-check outcomes, and measured stdout/stderr byte
  activity. Process failures retain their identity while reporting persisted status
  when available. Selection/idle diagnostics remain on the same execution path;
  parent title changes also refresh the idle explanation.
- Added one CLI recorder for version-1 NDJSON envelopes and human output. Startup
  explains the exact query, project/cwd, command, ordering, modes/limits, and logs.
  Current raw/processed/candidate counts and scoped state counts remain distinct.
  Terminal heartbeats replace one line; redirected progress stays append-only.
- Added timestamped `.wrk-runs/` directories, numbered full stdout/stderr logs,
  `--log-dir` (invocation-relative), `--heartbeat-interval` (default 60s), and
  `--verbose` (live raw bytes on stderr, disables terminal replacement). Files
  stream directly and sync/close at completion; no in-memory child transcript,
  environment dump, persistent processed set, or workflow checkpoint was added.
- Failure output distinguishes process failures from completion mismatches and
  includes project-pinned show/retry commands, log paths, and final cause/count/time.
  Write/close errors stop further work. Actual SIGPIPE is observed so broken output
  triggers process-group cleanup instead of abruptly abandoning the active child.
- Documented all flags, stream schemas, stdout/stderr behavior, failure contracts,
  log permissions/retention, ignore guidance, and the provisional JSON-format
  migration. Added indexed [requirement-level evidence](../docs/run-output-verification.md).

## Verification

- Full uncached `go test -count=1 ./...` and `go test -race -count=1 ./...` pass on
  macOS 15.7.7 arm64 and Debian 12.12/Linux arm64 with Go 1.25.4. Vet/build and
  formatting checks pass. Linux uses Docker `--init`, UID 1000, a local `/work/wrk`
  copy, and temporary projects outside any repository ancestor.
- CLI/integration checks rerun after tightening an existing interruption fixture's
  marker-content synchronization and the final stop-summary wording. The fixture
  had signaled on file existence before the helper finished writing; it now waits
  for complete contents. No product mutation guarantee was changed.
- Added precise selection/idle tests, real shell retry round-trips from another
  project, gated live-JSON delivery tests, separate 8 MiB binary stdout/stderr log
  checks, observer/log creation/write/close failure injection, and a real closed
  JSON pipe test verifying active child and descendant cleanup.
- Inspected an actual macOS PTY run and captured plain/JSON output using fake shell
  commands. Verified initial silence/later actual output recency, permanent
  milestones, no controls in redirected text, unchanged successful todo actions,
  process failure versus completion mismatch, suppressed unchanged idle polls,
  and an idle SIGINT summary. Captures live under ignored
  `.wrk-runs/output-verification-20261010/`.
- All 6 burn-script tests and all 15 browser model tests pass on both platforms.
  Final CLI/integration normal/race reruns, vet/build, formatting/diff checks, and
  project validation pass. The project contains 40 valid tickets.

## Handoff

Implementation and acceptance are complete. `run --json` now emits lifecycle
NDJSON; consumers use the `finished` event's `outcome` for the final summary. No
work remains in this ticket. The Codex burn migration and integrated workflow
verification remain in wrk-ef04f589; the parent milestone is still open.
