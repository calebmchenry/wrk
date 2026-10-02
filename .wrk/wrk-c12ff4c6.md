---
id: wrk-c12ff4c6
title: Support custom-field creation and updates
status: todo
parent: wrk-3f8a21b7
priority: normal
labels: []
---

## Outcome

Provide CLI operations to set and remove custom-field values on existing tickets and supply them during creation.

## Scope

Design explicit input syntax that retains YAML types and numeric precision. Apply configured string/number/boolean/enum checks without coercion. Unconfigured custom values remain arbitrary YAML nodes, including nested collections and tags; do not silently convert existing data through JSON.

## Acceptance criteria

- [ ] Define and document creation/update/remove flags and their conflict/no-op semantics.
- [ ] Support creation-time custom values and existing-ticket custom-field operations.
- [ ] Enforce configured definitions while preserving unconfigured values, aliases, and exact scalar precision.
- [ ] Preserve exact bodies and all unrelated metadata; reject unsupported preservation unchanged.
- [ ] Reuse complete-project validation, stable locking, stale-input checks, and publication-aware error handling.
- [ ] Add meaningful parser/storage/CLI tests on supported platforms and update indexed docs.

## Context

Sprint 001 already reads, validates, and preserves custom fields but intentionally exposes no mutation flags for them. Follow [configuration](../docs/configuration.md), [ticket format](../docs/ticket-format.md), and [storage](../docs/storage.md).
