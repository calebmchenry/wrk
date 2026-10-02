---
id: wrk-3f8a21b7
title: Build the minimal CLI
status: todo
depends_on:
  - wrk-9d42c6a1
---

## Outcome

Implement the first usable wrk CLI against the agreed [ticket format](../docs/ticket-format.md) and [project configuration](../docs/configuration.md). Use it to manage this project's own backlog.

## Acceptance criteria

- [x] Discover and read `.wrk/config.yaml`, including prefix, creation defaults, format version, and custom-field definitions.
- [x] Initialize a `.wrk/` directory and configuration without overwriting existing files.
- [x] Create, list, and show tickets, including child relationships and dependency blockers.
- [ ] Change titles, statuses, parents, dependencies, priorities, labels, and custom-field values through CLI commands.
- [x] Reject invalid metadata, missing references, self-references, and cycles without modifying files.
- [x] Preserve the Markdown body and unrelated custom-field values during metadata changes, and avoid silently overwriting concurrent edits.
- [x] Provide `wrk validate` for read-only checks of configuration, ticket files, IDs, and relationship integrity.
- [x] Keep ticket data in `.wrk/` with no external service required.
- [x] Verify creation, metadata changes, body preservation, validation failures, and configuration behavior with automated tests.
- [x] Document installation and working command examples in the README; update agent docs to remove the bootstrap limitation.
- [ ] Use the CLI to manage this project's tickets, including marking this ticket complete.

## Notes

Migrated from bootstrap ticket `002-build-minimal-cli.md` during initial format setup. Record implementation progress here until commands are available for metadata changes.

Start with [wrk-682f60c7: Ship the first runnable CLI for dogfooding](wrk-682f60c7.md). That child delivers the initial command loop with title and status updates; the broader metadata operations above remain follow-up work under this ticket.

Sprint 001 delivers the initial runnable CLI and verified title/status workflow. The broader metadata criterion and parent completion remain open. Deferred work is tracked in these CLI-created children:

- [wrk-1a09af55: Support parent and dependency updates](wrk-1a09af55.md)
- [wrk-e146d171: Support priority and label updates](wrk-e146d171.md)
- [wrk-c12ff4c6: Support custom-field creation and updates](wrk-c12ff4c6.md)

See [Sprint 001 execution evidence](../docs/sprints/SPRINT-001-EXECUTION.md) for both-platform validation and the accepted external-editor boundary.
