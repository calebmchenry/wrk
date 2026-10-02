# Combined Critique: GPT6Astra and GPT56Sol Sprint 001 Drafts

## Grounding

This critique is grounded in `SPRINT-001-INTENT.md`, `docs/ticket-format.md`, `docs/configuration.md`, and the live scope ticket `wrk-682f60c7`. The controlling constraints are now resolved: the implementation is Go; the first supported platforms are macOS and Linux local filesystems; ordinary commands must fail strictly on any invalid project state; `new` must include `--parent`, `--priority`, repeatable `--label`, and `--no-labels`; cooperating CLI writers are serialized and stale snapshots are rejected, with the final non-cooperating editor race explicitly documented as an accepted limitation. The final sprint should not carry these as open questions.

Both reviewed drafts correctly understand the main product boundary: deliver the first runnable local CLI, preserve existing ticket data without migration, keep broader metadata mutation work under the parent, and dogfood the milestone only after tests prove the CLI. The merge should preserve that discipline while sharpening a few risky edges: file-level storage feasibility, the lock inode, partial-file behavior, YAML preservation versus JSON projection, post-publication failures, and the ban on manual ticket metadata mutation.

## GPT6Astra Draft

### Strengths

The GPT6Astra draft is the stronger draft on sequencing and acceptance risk. Its phase order puts executable contracts first, then parsing/validation, then safe updates, then creation/init, then full verification and live completion. That order is sensible because update safety and metadata preservation are the highest-risk operations, and because the first live dogfood mutation should be the milestone status transition after update has been tested on disposable fixtures.

Its storage section is unusually careful. The persistent `.wrk/.lock` inode is the right shape for serialized CLI writers because it avoids the split-lock problem that appears when a lock file is deleted and recreated. It also correctly distinguishes advisory locking from editor coordination, rejects observable stale changes, and explicitly names the final check-to-replace race instead of overstating atomic rename. The no-replace creation plan using a staged complete file plus same-filesystem link is also well aligned with the ticket-format rule that creation must never overwrite an existing ticket.

The verification matrix is broad and useful. It covers stale writes, same-size/same-timestamp edits, atomic editor replacement, forced ID collisions, old-prefix IDs, invalid inner projects, separate graph cycles, canceled blockers, body delimiter edge cases, exact source in JSON, and crash/publication states. Its Definition of Done also does a good job requiring macOS/Linux evidence, clean-source build evidence, JSON/exit documentation, and final live validation.

### Weaknesses

The main weakness is that the draft still presents several now-resolved choices as proposed or open. The final sprint must collapse those into requirements: Go, macOS/Linux, strict invalid-project failure, resolved creation flags, and the bounded editor-race contract.

It also proposes a bounded `update` repair mode for invalid or missing title/status on the target. That conflicts with the now-confirmed strict failure policy for ordinary commands on invalid projects. Even if the repair is attractive, it introduces a partial-validity exception that complicates validation, race checks, and user mental model. Remove it from the merge and leave recovery to direct file repair followed by `validate`.

The YAML strategy is preservation-forward, which is good, but the anchor/alias handling is perhaps too ambitious for Sprint 001. “Materialize affected aliases” could turn into a format migration by accident. The merged sprint should either reject unsupported YAML constructs before mutation with a clear diagnostic, or prove a narrow preservation boundary through feasibility tests before committing to support. In no case should it silently coerce or JSON-normalize unconfigured custom values.

The file summary is broadly feasible but somewhat coarse. It should be merged with Sol’s more explicit package split while retaining Astra’s storage invariants. The docs changes also need tighter wording: clarifying `docs/ticket-format.md` and `docs/configuration.md` is acceptable only when the implementation discovers an ambiguity, not as a way to narrow established preservation obligations after the fact.

### Risk and Edge-Case Gaps

Astra covers most key storage risks, including post-publication uncertainty, but the final sprint should make the partial-file story even more operational: temporary files must use reserved names that cannot be scanned as tickets, be created with exclusive creation, survive crashes harmlessly, and be ignored or diagnosed without blocking valid tickets. It should also call out lock-file behavior in validation: the persistent lock inode is not a ticket and its presence does not mean a stale held lock.

The draft should add a sharper file-level feasibility gate for Unix behavior: hard-link publication support, directory sync behavior, permission preservation, no symlink following for mutation targets, and how unsupported filesystems fail safely. These are not implementation trivia; they decide whether macOS/Linux support is real.

### Definition of Done Completeness

Astra’s DoD is close to complete. Required changes are: remove open-question framing for resolved decisions, remove the repair exception, explicitly include the resolved `new` flags, require partial-file and stale-temp verification, require a persistent lock inode test, and require documentation of the final editor race as a limitation rather than a failure of the sprint.

## GPT56Sol Draft

### Strengths

The GPT56Sol draft is the stronger draft on implementation shape. Its package boundaries are concrete and easy to execute: `internal/cli`, `internal/app`, `internal/project`, `internal/config`, `internal/ticket`, `internal/graph`, `internal/storage`, and `internal/diagnostic`. The layered validation model is also clear, and it correctly adopts strict whole-project validity for ordinary commands. That aligns with the resolved decision and avoids partial list/show semantics.

Its use cases are readable and implementation-oriented. The parser contract is explicit about `--flag value`, `--flag=value`, `--`, duplicate scalar flags, and exact positional arity. The repository snapshot model is also good: config bytes, ticket bytes, inventory fingerprint, identity indexes, and deterministic graph validation. Its subprocess verification phase correctly protects the live `.wrk` backlog by copying fixtures and reserving live mutation for final dogfooding.

The draft is also strong on the manual metadata mutation boundary. The live milestone status changes happen through `wrk update`, follow-up tickets are created through `wrk new`, and body notes may be edited directly. That matches the ticket-format contract that frontmatter must go through the CLI after bootstrap.

### Weaknesses

The largest issue is scope drift in the opposite direction: it omits the now-resolved creation-time `--priority`, repeatable `--label`, and `--no-labels` flags, and explicitly says creation-time priority/label overrides are out of scope. That is no longer acceptable. Its Phase 7 instruction to put creation-time default overrides into a follow-up should be removed; these flags belong in Sprint 001.

The storage lock choice is weaker than Astra’s. Locking `.wrk/config.yaml` creates an inode-stability problem because config is directly editable by people and agents. If config is replaced by an editor, different processes may lock different inodes. The merge should use Astra’s persistent `.wrk/.lock` file and never unlink it on release.

Sol’s YAML value strategy is too restrictive as written. It proposes a JSON-compatible YAML domain and rejects aliases, timestamps, binary scalars, non-string nested mapping keys, and custom tags as a “clarification.” The normative docs require preservation of unconfigured custom values when unrelated metadata changes; they do not say custom values are JSON-only. The final sprint can define unsupported YAML constructs only after a feasibility decision, but the default posture should be preservation-first and fail-closed, not projection-first.

The JSON contract is also less safe for arbitrary custom values. `show` JSON containing `custom fields` and `body` as normalized values risks creating an accidental lossy projection for unknown YAML. Astra’s exact `source` string is safer for v1. The merge can still expose normalized built-ins and configured simple fields, but arbitrary unconfigured custom values should remain available through exact source unless a tested, lossless representation is specified.

### Risk and Edge-Case Gaps

Sol’s mutation protocol handles staging, hard-link creation, and deterministic hooks, but it does not sufficiently separate pre-publication rejection from post-publication uncertainty. The final sprint needs Astra’s `published: true` style distinction for directory-sync failures, cleanup failures, or termination after publication. A retry after an acknowledged failure must not create duplicate work or roll back a successful mutation over a later edit.

Partial-file creation is mostly addressed, but not enough for crash recovery. The merged sprint should require temp names outside the ticket filename pattern, close/fsync error checks, directory fsync where supported, and tests for interruption before and after publication. It should also define whether stale temp files are ignored, warned by `validate`, or cleaned only by the owning operation.

Sol’s `init` behavior allows adding config to an existing `.wrk` with support files, while Astra refuses any existing `.wrk`. The normative config doc only says init must not overwrite existing configuration or tickets, so either policy can be defended, but the merge must choose one. Given the user’s emphasis on no clobber and strictness, refusing preexisting `.wrk` is simpler unless there is a concrete bootstrap need for the support-files exception.

### Definition of Done Completeness

Sol’s DoD is good on command behavior, strict validation, graph correctness, live dogfooding, and docs. It needs additions for resolved creation flags, race testing (`go test -race ./...`), persistent lock inode behavior, post-publication uncertainty, partial-file/crash assertions, exact YAML custom-value preservation, and documented final editor race. It should also stop listing resolved items as open questions.

## Comparison and Merge Recommendations

Use Astra as the backbone for storage guarantees, verification strategy, and Definition of Done. Use Sol for package decomposition, parser specifics, validation layering, and the concise use-case structure. The merged sprint should make decisions declarative, not tentative.

Actionable merge changes:

- Replace all open questions about language, platforms, concurrency, creation flags, and invalid-project behavior with accepted requirements.
- Include `new --parent`, `--priority`, repeatable `--label`, and `--no-labels`; keep post-creation metadata updates deferred.
- Adopt strict ordinary-command failure on any invalid project state; remove Astra’s bounded `update` repair exception.
- Use a persistent `.wrk/.lock` inode for writer serialization, not `config.yaml`; never unlink the lock file on release.
- Define creation as staged complete file plus atomic no-replace publish, with reserved temp names, no partial final files, forced collision tests, stale-temp handling, and crash/interruption tests.
- Preserve arbitrary unconfigured custom YAML values semantically on metadata updates. Avoid JSON-only projection for unknown custom values; expose exact ticket source in JSON v1 unless a lossless representation is proven.
- Add explicit post-publication failure semantics: distinguish rejected mutations from committed-but-unacknowledged mutations, and prohibit blind rollback.
- Keep manual ticket metadata mutation banned. Dogfood status changes and follow-up ticket creation only through the new CLI; body evidence may be edited directly.
- Require macOS and Linux local-filesystem evidence, `go test ./...`, `go test -race ./...`, `go vet ./...`, clean-source build/install/help verification, and subprocess JSON/human workflow tests.

The final sprint should be stricter than either draft about resolved decisions and slightly more conservative than both about claiming filesystem guarantees. The acceptance bar is not merely “the sequential workflow works”; it is that the CLI can mutate ticket metadata without body loss, metadata drift, partial files, or false concurrency claims.
