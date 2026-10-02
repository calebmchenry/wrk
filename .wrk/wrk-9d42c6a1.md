---
id: wrk-9d42c6a1
title: Define the ticket format
status: done
---

## Outcome

Define how wrk stores tickets in `.wrk/` so people and agents can read them directly, edit descriptions freely, and manage structured metadata through the CLI, with changes tracked by Git.

## Acceptance criteria

- [x] Document the file format, required fields, ticket IDs, filenames, and supported statuses in `docs/`.
- [x] Keep task descriptions, context, and acceptance criteria readable without the CLI.
- [x] Define parenting, dependencies, custom fields, and the boundary between direct edits and CLI changes.
- [x] Define project configuration for prefixes, creation defaults, and custom fields.
- [x] Update this project's tickets to follow the chosen format and link the documentation from `docs/index.md`.

## Notes

Migrated from bootstrap ticket `001-define-ticket-format.md` during the initial format setup, before the CLI existed. The format is documented in [ticket-format.md](../docs/ticket-format.md) and [configuration.md](../docs/configuration.md). Command implementation and enforcement remain in [wrk-3f8a21b7](wrk-3f8a21b7.md).
