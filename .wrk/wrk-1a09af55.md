---
id: wrk-1a09af55
title: Support parent and dependency updates
status: done
parent: wrk-3f8a21b7
priority: normal
labels: []
---

## Outcome

Extend `wrk update` with parent and dependency operations, preserving the validated transactional workflow established in Sprint 001.

## Scope

Support setting/clearing a parent and adding/removing dependencies. Carry creation-time dependency options into this work; `new --parent` already exists. Choose and document the exact flag conflict and no-op semantics before implementation. Keep IDs immutable and derive children from parent fields.

## Acceptance criteria

- [x] Support parent changes and clearing through the CLI without direct frontmatter edits.
- [x] Support dependency additions/removals and creation-time dependencies.
- [x] Validate complete candidate projects, including references, self/duplicate edges, and separate graph cycles.
- [x] Preserve exact body bytes, unrelated metadata/custom values, permissions, and publication/error guarantees.
- [x] Verify blocked explicit updates, reopening, no cascades, no-op behavior, and strict-project rejection.
- [x] Add meaningful CLI/storage tests on supported platforms and update indexed docs.

## Context

Follow [the ticket format](../docs/ticket-format.md), [CLI contract](../docs/cli.md), and [storage boundary](../docs/storage.md). Sprint 001 deliberately deferred these update operations; title/status updates and creation-time parenting already work.

2026-10-02: The release/upgrade batch (`list --all --label upgrade`, rooted in [wrk-551ff6c2](wrk-551ff6c2.md) and [wrk-8d5b1b84](wrk-8d5b1b84.md)) is a concrete consumer of dependency creation/update. Its prerequisites are currently body links because the supported CLI cannot create those edges. Once these commands exist, encode the still-relevant implementation prerequisites through the CLI and verify readiness. This metadata improvement does not block the upgrade implementation and must not be worked around by editing frontmatter.

## Implementation decisions (2026-10-02)

- Add `update --parent id` / `--no-parent`, repeatable `--add-dependency id` / `--remove-dependency id`, and repeatable `new --depends-on id`.
- Parent set/clear conflict. Adding/removing the same dependency conflicts; repeated add/remove requests are idempotent. Creation rejects duplicate dependency edges. Removing an absent dependency is a no-op; clearing a parent removes its key.
- Combined updates validate the complete candidate project, keep parent/dependency graphs separate, and never cascade status. Recursive mutation remains labels only.
- Reuse the existing lock/stage/compare/publication path. Verification will cover relationships, preservation, invalid projects, failures, and native macOS/Linux runs.

## Completion evidence (2026-10-02)

- Full uncached tests (`go test -count=1 ./...`), race tests (`go test -race -count=1 ./...`), `go vet ./...`, formatting, build, and project validation passed on macOS arm64 and Linux arm64 with Go 1.25.4. Linux ran as an unprivileged user in `golang:1.25.4-bookworm`, with source copied onto the container's local filesystem; this is runtime verification, not cross-compilation.
- New parser, YAML-preservation, storage, and compiled-CLI integration tests cover typed input, relationships, exact bodies/modes, no-ops, conflicts, rejected combined updates, strict invalid-project handling, and publication failures. Existing regression tests also pass.
- README, indexed CLI/configuration/ticket/storage docs, and `.wrk/AGENTS.md` now document the complete command surface.
- Implemented `new --depends-on` and `update --parent` / `--no-parent` / `--add-dependency` / `--remove-dependency`. Both graphs validate independently; readiness updates immediately, explicit blocked/reopen transitions work, and no statuses cascade.
- Exercised the new CLI on this backlog: encoded the documented prerequisites for `wrk-f28fed55`, `wrk-2c22eb33`, `wrk-f1f845bd`, and `wrk-af501027`. Their prerequisites are already done; statuses stay done and no readiness blockers are introduced. Body-only sequencing notes are now backed by validated edges.
- Remaining work: none.
