---
id: wrk-3f8a21b7
title: Build the minimal CLI
status: done
depends_on:
  - wrk-9d42c6a1
---

## Outcome

Implement the first usable wrk CLI against the agreed [ticket format](../docs/ticket-format.md) and [project configuration](../docs/configuration.md). Use it to manage this project's own backlog.

## Acceptance criteria

- [x] Discover and read `.wrk/config.yaml`, including prefix, creation defaults, format version, and custom-field definitions.
- [x] Initialize a `.wrk/` directory and configuration without overwriting existing files.
- [x] Create, list, and show tickets, including child relationships and dependency blockers.
- [x] Change titles, statuses, parents, dependencies, priorities, labels, and custom-field values through CLI commands.
- [x] Reject invalid metadata, missing references, self-references, and cycles without modifying files.
- [x] Preserve the Markdown body and unrelated custom-field values during metadata changes, and avoid silently overwriting concurrent edits.
- [x] Provide `wrk validate` for read-only checks of configuration, ticket files, IDs, and relationship integrity.
- [x] Keep ticket data in `.wrk/` with no external service required.
- [x] Verify creation, metadata changes, body preservation, validation failures, and configuration behavior with automated tests.
- [x] Document installation and working command examples in the README; update agent docs to remove the bootstrap limitation.
- [x] Use the CLI to manage this project's tickets, including marking this ticket complete.

## Notes

Migrated from bootstrap ticket `002-build-minimal-cli.md` during initial format setup. Record implementation progress here until commands are available for metadata changes.

Initial delivery started with [wrk-682f60c7: Ship the first runnable CLI for dogfooding](wrk-682f60c7.md). That child delivered the initial command loop with title and status updates; the broader metadata operations were follow-up work under this ticket and are now implemented.

Sprint 001 delivered the initial runnable CLI and verified title/status workflow. The remaining metadata operations were tracked in these CLI-created children, all now complete:

- [wrk-1a09af55: Support parent and dependency updates](wrk-1a09af55.md)
- [wrk-e146d171: Support priority and label updates](wrk-e146d171.md)
- [wrk-c12ff4c6: Support custom-field creation and updates](wrk-c12ff4c6.md)

See [Sprint 001 execution evidence](../docs/sprints/SPRINT-001-EXECUTION.md) for both-platform validation and the accepted external-editor boundary.

## Completion work (2026-10-02)

The user authorized implementation of both remaining children and completion of this parent. Relationship and custom-field operations are now in progress under their existing tickets. Close this parent only after both children satisfy their criteria and the full development checks pass.

## Completion evidence (2026-10-02)

- Full uncached tests (`go test -count=1 ./...`), race tests (`go test -race -count=1 ./...`), `go vet ./...`, formatting, build, and project validation passed on macOS arm64 and Linux arm64 with Go 1.25.4. Linux ran as an unprivileged user in `golang:1.25.4-bookworm`, with source copied onto the container's local filesystem; this is runtime verification, not cross-compilation.
- New parser, YAML-preservation, storage, and compiled-CLI integration tests cover typed input, relationships, exact bodies/modes, no-ops, conflicts, rejected combined updates, strict invalid-project handling, and publication failures. Existing regression tests also pass.
- README, indexed CLI/configuration/ticket/storage docs, and `.wrk/AGENTS.md` now document the complete command surface.
- Both remaining feature children are implemented and verified; the earlier priority/label and initial CLI children were already complete. All acceptance criteria for the minimal CLI are satisfied.
- Used the current checkout CLI to start and finish these tickets and to encode existing release prerequisites. No metadata was hand-edited.
- Remaining work: none.

## Source handoff (2026-10-02)

The user authorized committing and pushing the completed metadata work to `origin/main`. The implementation, tests, docs, completed tickets, and migrated release dependency edges are included. Local macOS/Linux checks above passed; `git diff --check` and current-checkout project validation passed again before handoff. A main-branch push starts the Verify matrix and snapshot packaging. Stable asset publication requires a separate canonical version tag through the documented Release workflow.
