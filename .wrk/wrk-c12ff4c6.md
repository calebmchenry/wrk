---
id: wrk-c12ff4c6
title: Support custom-field creation and updates
status: done
parent: wrk-3f8a21b7
priority: normal
labels: []
---

## Outcome

Provide CLI operations to set and remove custom-field values on existing tickets and supply them during creation.

## Scope

Design explicit input syntax that retains YAML types and numeric precision. Apply configured string/number/boolean/enum checks without coercion. Unconfigured custom values remain arbitrary YAML nodes, including nested collections and tags; do not silently convert existing data through JSON.

## Acceptance criteria

- [x] Define and document creation/update/remove flags and their conflict/no-op semantics.
- [x] Support creation-time custom values and existing-ticket custom-field operations.
- [x] Enforce configured definitions while preserving unconfigured values, aliases, and exact scalar precision.
- [x] Preserve exact bodies and all unrelated metadata; reject unsupported preservation unchanged.
- [x] Reuse complete-project validation, stable locking, stale-input checks, and publication-aware error handling.
- [x] Add meaningful parser/storage/CLI tests on supported platforms and update indexed docs.

## Context

Sprint 001 already reads, validates, and preserves custom fields but intentionally exposes no mutation flags for them. Follow [configuration](../docs/configuration.md), [ticket format](../docs/ticket-format.md), and [storage](../docs/storage.md).

## Implementation decisions (2026-10-02)

- Add repeatable `new/update --field name=YAML` and `update --remove-field name`. Split at the first unescaped `=` (escape literal name characters as `\=` or `\\`); values are independent, single YAML documents. Quote YAML string literals when they resemble other types. Explicit `null` is a value; an empty right-hand side is rejected.
- Duplicate assignments and set/remove of the same name are usage errors. Removing absent names and setting identical YAML values are no-ops. Existing unrelated custom values and exact bodies remain unchanged; YAML formatting and anchor names may change.
- Keep nodes rather than decoding through Go/JSON values. Regenerate reachable anchors when edited definitions disappear so aliases, cycles, nested tags, and scalar precision survive. Reparse and verify all intended and unrelated values before candidate validation and publication.
- Verification will cover configured types, arbitrary YAML, anchor collisions, no-ops, failed preservation, storage failures, and native macOS/Linux runs.

## Completion evidence (2026-10-02)

- Full uncached tests (`go test -count=1 ./...`), race tests (`go test -race -count=1 ./...`), `go vet ./...`, formatting, build, and project validation passed on macOS arm64 and Linux arm64 with Go 1.25.4. Linux ran as an unprivileged user in `golang:1.25.4-bookworm`, with source copied onto the container's local filesystem; this is runtime verification, not cross-compilation.
- New parser, YAML-preservation, storage, and compiled-CLI integration tests cover typed input, relationships, exact bodies/modes, no-ops, conflicts, rejected combined updates, strict invalid-project handling, and publication failures. Existing regression tests also pass.
- README, indexed CLI/configuration/ticket/storage docs, and `.wrk/AGENTS.md` now document the complete command surface.
- Implemented repeatable `new/update --field name=YAML` and `update --remove-field name`, including escaped `=` and backslash characters in field names. Input stays as YAML nodes; configured types reject coercion, and aliases/tags/precision survive creation and updates.
- Replaced scalar-only alias detachment with graph-aware anchor relocation. Coverage includes removed anchored collections/keys, self-referential values, anchor collisions, aliases from built-ins into custom data, and unchanged rejection when whole-frontmatter aliases prevent preservation.
- Remaining work: none.
