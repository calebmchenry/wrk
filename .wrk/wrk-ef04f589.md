---
id: wrk-ef04f589
title: Migrate and verify Codex burns through wrk run
status: done
parent: wrk-0a233d21
depends_on:
  - wrk-7b831fa1
related:
  - wrk-c30c139f
priority: normal
labels:
  - automation
  - run-cli
---
## Outcome

The existing Codex burn workflow uses wrk run for orchestration, with a documented migration and end-to-end verification of the integrated runner on supported platforms.

Depends on [wrk-7b831fa1](wrk-7b831fa1.md), transitively covering core execution and streaming. Existing burn behavior/history is [wrk-c30c139f](wrk-c30c139f.md); the agreed scope is [wrk-0a233d21](wrk-0a233d21.md).

## Migration

- Make the ordinary burn a direct documented invocation or a thin convenience wrapper around wrk run. Do not keep two independent selection/process/logging loops.
- Preserve one fresh Codex call per selected ticket, the exact prompt `Implement {id}`, the existing full-access command `codex exec --dangerously-bypass-approvals-and-sandbox`, and required persisted done verification. Codex-specific flags belong to the supplied command/wrapper, not the generic CLI engine.
- Document the native equivalent of the old --list-command usage, for example:

  ```sh
  go run ./cmd/wrk run --ready --label web --expect-status done -- \
    codex exec --dangerously-bypass-approvals-and-sandbox 'Implement {id}'
  ```

- Include scoped --under/combined-label selection, --stream idle polling, success limits, explicit-ticket retries, and an unchanged-status review example that processes each ticket only once per invocation.
- Explain the intentional changes: native filters replace external selection commands; successful IDs are skipped for the invocation; generic success defaults to exit 0; burn success also requires done. Streaming waits for previously unprocessed eligible tickets, while a fresh invocation may revisit them.
- Choose and document whether scripts/ticket-burn.py becomes a thin wrapper or is retired in favor of the direct invocation. Describe any old flag/log-path changes explicitly; do not silently pretend arbitrary former selection wrappers map to native filters.
- Move applicable fake-command coverage into the CLI/runner tests and adjust CI only after equivalent coverage exists. Preserve the full-access-default regression check without invoking a real AI session.

## User and agent documentation

Update README, docs/cli.md, docs/burns.md, and docs/index.md to describe the implemented command rather than the proposed syntax. Update historical no-runner/no-indefinite-polling statements to allow explicit wrk run --stream while retaining bounded/manual burn guidance.

Explain -- argument separation and literal placeholders; exported environment/PATH inheritance; project-root child cwd versus invocation-relative CLI paths; script interpreter/executable requirements; no implicit shell, dotenv, or interactive prompts; process-group interruption; logs and JSON events; default action success versus expected status; and the fact that ready excludes in-progress work.

For multi-step scripts, document restarting the whole action after inspection, keeping tickets unfinished until the entire required workflow succeeds, and author responsibility for safe repetition after partial failure. Do not promise agent calls are inherently idempotent or provide workflow checkpoints, automatic retries, or rollback. Clarify that one sequential runner does not claim tickets against other workers.

## Acceptance criteria

- [x] Replace burn orchestration with a thin use of wrk run and document migration, examples, limits, and recovery.
- [x] Validate the exact Codex command/prompt and done-check configuration using a fake executable, not a real backlog burn.
- [x] Run disposable-project integration cases covering newly unblocked work, unchanged-status actions processed once, empty/all-processed queries, stream arrivals, limits, child failure, exit-0/not-done failure, interruption, logs, and JSON output.
- [x] Demonstrate explicit retry of an in-progress ticket with a fake multi-step action: first step leaves a durable result, second fails, the run stops, and retry restarts the whole action safely and completes. No automatic rollback, retry, or named-step state is added.
- [x] Verify exported env/PATH, a script with a spaced path, explicit project selection from another directory, project-relative execution, and a child invoking wrk mutations without a lifetime writer lock.
- [x] Verify supported macOS/Linux execution and child/descendant termination; use container-local source/filesystem on Linux as documented. Distinguish actual execution evidence from cross-build-only evidence.
- [x] Run formatting, `go test -count=1 ./...`, `go test -race -count=1 ./...`, `go vet ./...`, build/install smoke, remaining wrapper tests if retained, `git diff --check`, and `go run ./cmd/wrk validate`. Record precise evidence and any unresolved verification.
- [x] Review final help/docs against real behavior, including JSON/verbose interaction and the distinction between successful actions and done tickets.
- [x] Leave a handoff for the parent's acceptance review; completion of this ticket does not itself close the parent.

## Scope and status

This ticket owns migration, integrated verification, and final public documentation.
The backlog-creation turn authorized no implementation; the user's explicit
2026-10-10 implementation request superseded that historical boundary. No release,
real-agent burn, Git commit, or push was performed. Existing uncommitted checkout
work was preserved.

## Implementation completed (2026-10-10)

- Retained scripts/ticket-burn.py as a small exec-only wrapper around wrk run. It
  requires a native selector, forwards native run options, and supplies the exact
  full-access Codex default, single `Implement {id}` prompt, and required done check.
  Removed the independent selection/process/logging/completion loop. Custom wrk and
  Codex prefixes remain available; --list-command and --heartbeat are retired.
- Replaced script orchestration tests with four focused wrapper tests, including
  default argv, checkout CLI selection, PID/environment inheritance, quoted prefixes,
  native flags, explicit tickets, exit propagation, and rejected legacy options.
  Added test/integration/codex_burn_test.go for the exact fake Codex action through
  the real CLI, dependency unlocking, both failure modes, and safe whole-script retry
  after durable first-step output and a failing second step. Existing CLI/runner
  suites retain selection, streaming, interruption, logging, and output coverage.
- Updated README, docs/cli.md, docs/burns.md, docs/index.md and historical server/
  container wording. The migration covers scopes, stream/limits, once-per-invocation
  success, explicit retries, old flag/log changes, argv/environment/path/interpreter
  semantics, noninteractive execution, process groups, no lifetime writer lock,
  JSON/verbose, and author responsibility for partial effects. CI retains the wrapper
  check and uses Go tests for native orchestration coverage.
- Added indexed [requirement-level verification](../docs/burn-verification.md) and
  retained local diagnostic captures under .wrk-runs/burn-verification-20261010/.

## Verification

- Go 1.25.4 on macOS 15.7.7 arm64 and Debian 12.12 Linux arm64: formatting,
  `go test -count=1 ./...`, `go test -race -count=1 ./...`, vet, build, isolated
  install/version smoke, all four wrapper tests, embedded workspace smoke, and
  project validation pass. Linux used Docker --init, UID/GID 1000, and a copied
  container-local /work/wrk tree with temporary projects outside a project ancestor.
- Actual SIGINT/SIGTERM tests cover responsive/ignoring child leaders, descendants,
  ordinary/stream modes and retained partial changes. JSON pipe failure also stops
  active process groups. No real Codex was invoked.
- Both installed native binaries passed an additional actual-wrapper -> wrk run ->
  fake Codex smoke, checking exact argv, distinct child PIDs, required done, limit,
  drain/empty, selected cwd from elsewhere, JSON+verbose and spaced relative logs.
- All 15 browser model tests pass on macOS. macOS/Linux amd64 binaries cross-build;
  those architectures were not executed locally and hosted CI was not run here.
- Reviewed real CLI/wrapper help against final docs. Recorded that the go run
  launcher maps usage exit 2 to its own exit 1; compiled wrk is documented for exact
  exit codes and single-PID supervision. `git diff --check` and final validation pass.

## Parent acceptance handoff

All criteria for this ticket are met; no required work remains here. The parent
wrk-0a233d21 remains open for its own outcome/acceptance review. Its reviewer can use
[burn verification](../docs/burn-verification.md) together with
[run output verification](../docs/run-output-verification.md), the four completed
child tickets, and the migration/recovery docs. No real backlog burn or release
publication is needed for that review.
