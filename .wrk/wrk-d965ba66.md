---
id: wrk-d965ba66
title: Support scoped AI task burns with labels and blocked status
status: done
priority: normal
labels: []
---
## Outcome

An AI agent can work through a selected batch of tickets for an extended session, resume from durable ticket notes, and skip work that needs external input. Users can select an arbitrary batch with labels or a deliverable's descendants with a parent scope.

## CLI scope

Implemented commands (examples):

```sh
wrk update <id> --add-label burn-tonight
wrk update <id> --remove-label burn-tonight
wrk update <id> --add-label backend --remove-label needs-triage
wrk update <parent-id> --add-label burn-tonight --recursive
wrk update <parent-id> --remove-label burn-tonight --recursive
wrk list --ready --label burn-tonight
wrk list --ready --under <parent-id>
wrk update <id> --status blocked
wrk update <id> --status todo
```

## Behavior

- Labels select batches without changing existing parent relationships. Parent scopes traverse descendants at any depth; dependencies still determine readiness across the whole project.
- Label add/remove flags are repeatable and preserve unrelated labels. Adding an existing label or removing an absent one is a successful no-op. Do not invalidate existing tickets that contain duplicate labels.
- Recursive labeling includes the selected ticket and every descendant, follows parent relationships rather than dependencies, and applies regardless of ticket status. Initially permit only label changes with recursion; reject combinations with title/status or other metadata edits.
- Recursive labeling is a snapshot, not inheritance. Later children receive batch labels only when explicitly supplied, including by the agent when creating subtasks during a burn.
- Add blocked as a built-in status alongside todo, in-progress, done, and canceled. Record the reason, unblocking condition, and resume notes in the ticket body.
- Ready still means todo with every dependency done. Explicitly blocked tickets remain visible in ordinary active listings but are excluded from ready listings. A canceled or blocked dependency does not satisfy readiness.
- Unblocking is explicit. Completing dependencies never clears manual blocked status. Parent completion and other status changes do not cascade.

## Agent workflow

Document a loop that reads scope instructions and existing in-progress work, selects a ready ticket, marks it in-progress, implements and verifies it, records results, and marks it done. If it needs external input, record why and mark it blocked, then continue with other eligible work.

Keep scope, permissions, verification, and stopping rules explicit. Stop when the batch is complete or no work can proceed, and report unfinished work and blockers. An empty ready list must not imply completion. Keep durable handoffs in ticket bodies so another session can resume.

## Acceptance criteria

- [x] Implement label mutations, recursive labeling, label filtering, and descendant-scoped listing with human and JSON output.
- [x] Add blocked status consistently to validation, updates, active listings, readiness, help, and documentation.
- [x] Define flag conflicts, repeated-label filter semantics, combined scope filters, and whether --under includes the root before implementation.
- [x] Define recursive update locking, stale-input checks, interruption/partial-failure reporting, and safe retry behavior; preserve bodies, unrelated metadata, and existing publication guarantees.
- [x] Test nested descendants, unrelated tickets, no-ops, invalid flags, cross-scope dependencies, blocked/unblocked transitions, and preservation/failure behavior.
- [x] Document the burn loop and new CLI behavior in README and indexed agent docs, including compatibility implications for older clients that reject blocked status.
- [x] Run the required supported-platform checks and wrk validate.

## Related work and boundaries

Reuse [wrk-e146d171: Support priority and label updates](wrk-e146d171.md) for shared single-ticket label mutation work; avoid implementing that twice. Its priority and replacement/clear-label scope remains there. Parent/dependency mutation remains tracked in [wrk-1a09af55](wrk-1a09af55.md).

This work provides CLI primitives and a documented agent workflow. A built-in AI runner, autonomous scheduling, parallel task claiming, and automatic label inheritance are outside scope. Implementation is complete; shared single-ticket add/remove work is recorded in wrk-e146d171.

## Implementation decisions (before implementation)

- Repeated `list --label` filters use AND (all requested labels, exact and case-sensitive); duplicate filters are harmless. Label, descendant, and status/readiness filters intersect. `--under` excludes the root and includes descendants at every depth; an unknown root is NOT_FOUND, even for an otherwise empty list.
- Repeated add/remove flags are idempotent. Reject adding and removing the same label in one update as USAGE. Reject empty/non-UTF-8 label arguments. Preserve order and duplicates of unrelated existing labels; remove all occurrences of a removed label, append each new label once. Absent labels stay omitted for no-ops. Single-ticket label edits may combine with title/status.
- `--recursive` requires label changes and rejects title/status. It includes the root and all descendants regardless of status, ordered by ID. It never follows dependency edges or adds inheritance.
- Hold one project lock from load through cleanup. Prepare/validate/stage all changed tickets before the first rename. Compare all validation inputs before each rename, advancing expected bytes/inode/mode only for this operation's committed files using staging identities. Preserve per-file atomic publication, permission bits, exact bodies and unrelated YAML values. No multi-file atomicity or rollback is promised.
- On handled failure before any rename, retain result:null and unchanged preexisting data. After any commit, report the complete selected batch with per-ticket committed/unchanged/pending publication states and the top-level committed marker; stop on the first error and clean operation-owned stages. Kills/stdout failure may prevent reporting; inspect the scope and retry the same idempotent operation once the cause is resolved. A retry takes a fresh descendant snapshot. External edits and Git operations remain outside CLI mutations.
- Add blocked to the version-1 vocabulary without rewriting existing tickets/config. Older clients that reject blocked cannot operate on a project containing it; upgrade all writers/readers before using it. Manual blocked status never clears automatically.

## Implementation and verification

Completed on 2026-10-02 with Go 1.25.4 on macOS arm64 (Darwin 24.6.0, local filesystem) and actual Linux arm64 (kernel 5.10.124-linuxkit, Docker `golang:1.25.4-bookworm`, source copied into container-local `/work`).

- Shared `ticket.Changes` / `PatchChanges` and `store.UpdateWithOptions` provide single-ticket and recursive label changes without a second implementation. YAML aliases to edited scalars, label lists, and label elements retain their old semantic values in unrelated fields; unsupported whole-ticket recursive aliases fail before any publication.
- Scoped lists use exact AND label filters and descendant/status intersections. Manual blocked is validated, updated, listed as active, and excluded from ready; dependency readiness still uses the entire project and only done satisfies it.
- Recursive mutations validate all candidates, stage all changed files, then publish in ID order under one lock. Each rename compares the complete expected snapshot; committed file identities advance from staged identities. Human/JSON output identifies committed, unchanged, and pending targets on handled partial failure. Tests cover all statuses, unrelated tickets, later children without inheritance, and safe retries.
- Added `internal/ticket/labels_test.go`, `internal/cli/scopes_test.go`, `internal/project/scopes_test.go`, `internal/store/recursive_test.go`, recursive subprocess-kill cases in `interruption_test.go`, and compiled `test/integration/burn_test.go`. Coverage includes 200-level scopes, root exclusion/inclusion, combined filters, duplicates/no-ops, invalid flags, cross-scope canceled/blocked dependencies, explicit unblock/no cascade, exact bodies/omitted metadata/permissions/aliases, full-batch preservation failure, write/sync/close/rename/directory-sync/cleanup failures, stale config/inventory/inode/mode/committed/pending/unrelated inputs, lock retention, real kills before and after publication, and retry behavior.
- Both platforms passed formatting (`gofmt -l cmd internal test`, empty), `go test -count=1 ./...`, `go test -race -count=1 ./...`, `go vet ./...`, and `go build -o bin/wrk ./cmd/wrk`. Linux additionally passed install into `/installed` and installed help/init/new/label-filtered ready/validate smoke commands outside the source directory. Integration runs rebuild and execute the CLI on each platform.
- README and indexed `docs/burns.md` document scope, authorization, in-progress resumption, verification, durable blocked handoffs, explicit unblocking, subtask labels, and stopping/reporting rules. CLI/storage/ticket docs define filters, per-file publication, retries and older-client incompatibility. README now records why verification uses `-count=1`: Go's integration-test cache does not track a CLI built at runtime in TestMain.
- Live `go run ./cmd/wrk validate` passed (9 tickets) after implementation; final completion uses the CLI and repeats validation. `git diff --check` passed. Original compatibility fixtures remain unchanged.

## Remaining work

None for this ticket. Priority and label replacement/clear operations remain in wrk-e146d171; parent/dependency mutation remains in wrk-1a09af55. No built-in runner, scheduling, parallel claiming, or inheritance was added. Recursive publication remains per-file rather than all-or-nothing, as specified above.
