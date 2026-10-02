---
id: wrk-1a09af55
title: Support parent and dependency updates
status: todo
parent: wrk-3f8a21b7
priority: normal
labels: []
---

## Outcome

Extend `wrk update` with parent and dependency operations, preserving the validated transactional workflow established in Sprint 001.

## Scope

Support setting/clearing a parent and adding/removing dependencies. Carry creation-time dependency options into this work; `new --parent` already exists. Choose and document the exact flag conflict and no-op semantics before implementation. Keep IDs immutable and derive children from parent fields.

## Acceptance criteria

- [ ] Support parent changes and clearing through the CLI without direct frontmatter edits.
- [ ] Support dependency additions/removals and creation-time dependencies.
- [ ] Validate complete candidate projects, including references, self/duplicate edges, and separate graph cycles.
- [ ] Preserve exact body bytes, unrelated metadata/custom values, permissions, and publication/error guarantees.
- [ ] Verify blocked explicit updates, reopening, no cascades, no-op behavior, and strict-project rejection.
- [ ] Add meaningful CLI/storage tests on supported platforms and update indexed docs.

## Context

Follow [the ticket format](../docs/ticket-format.md), [CLI contract](../docs/cli.md), and [storage boundary](../docs/storage.md). Sprint 001 deliberately deferred these update operations; title/status updates and creation-time parenting already work.

2026-10-02: The release/upgrade batch (`list --all --label upgrade`, rooted in [wrk-551ff6c2](wrk-551ff6c2.md) and [wrk-8d5b1b84](wrk-8d5b1b84.md)) is a concrete consumer of dependency creation/update. Its prerequisites are currently body links because the supported CLI cannot create those edges. Once these commands exist, encode the still-relevant implementation prerequisites through the CLI and verify readiness. This metadata improvement does not block the upgrade implementation and must not be worked around by editing frontmatter.
