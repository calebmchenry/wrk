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

- [ ] Discover and read `.wrk/config.yaml`, including prefix, creation defaults, format version, and custom-field definitions.
- [ ] Initialize a `.wrk/` directory and configuration without overwriting existing files.
- [ ] Create, list, and show tickets, including child relationships and dependency blockers.
- [ ] Change titles, statuses, parents, dependencies, priorities, labels, and custom-field values through CLI commands.
- [ ] Reject invalid metadata, missing references, self-references, and cycles without modifying files.
- [ ] Preserve the Markdown body and unrelated custom-field values during metadata changes, and avoid silently overwriting concurrent edits.
- [ ] Provide `wrk validate` for read-only checks of configuration, ticket files, IDs, and relationship integrity.
- [ ] Keep ticket data in `.wrk/` with no external service required.
- [ ] Verify creation, metadata changes, body preservation, validation failures, and configuration behavior with automated tests.
- [ ] Document installation and working command examples in the README; update agent docs to remove the bootstrap limitation.
- [ ] Use the CLI to manage this project's tickets, including marking this ticket complete.

## Notes

Migrated from bootstrap ticket `002-build-minimal-cli.md` during initial format setup. Record implementation progress here until commands are available for metadata changes.

Start with [wrk-682f60c7: Ship the first runnable CLI for dogfooding](wrk-682f60c7.md). That child delivers the initial command loop with title and status updates; the broader metadata operations above remain follow-up work under this ticket.
