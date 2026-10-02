# Sprint 001 Draft: First Runnable wrk CLI

## Overview

Deliver the first runnable `wrk` CLI so this repository can manage its own backlog using the documented `.wrk/` format. The sprint scope is the child milestone `wrk-682f60c7`: implement `init`, `new`, `list`, `show`, `update` for title/status only, and `validate`, with readable output and `--json` for every command.

Repository evidence:

- The repository is documentation-only today. There is no source tree, package manifest, test harness, or committed history beyond untracked local files.
- `.wrk/config.yaml` is version `1`, prefix `wrk`, default priority `normal`, empty default labels, and no configured custom fields.
- The existing backlog contains one completed prerequisite ticket, one open parent CLI ticket, and one open sprint ticket. The sprint ticket depends on the completed format ticket and is a child of the broader minimal-CLI parent.
- `docs/ticket-format.md` and `docs/configuration.md` are the normative contracts for storage, validation, project discovery, and direct-edit boundaries.

Requirements for this sprint:

- Existing ticket files and config must be read without migration.
- Metadata changes must preserve the Markdown body byte-for-byte and preserve unrelated metadata, including unconfigured `fields` values.
- Failed validation must leave files unchanged.
- The CLI must read the current file immediately before update and reject stale writes according to an explicit implementation contract.
- Dependency blockers are reported, but only `done` satisfies a prerequisite and blockers do not prevent explicit status updates.

Assumptions for this draft:

- Implement in Go to produce a straightforward local binary and avoid runtime dependency setup for dogfooding.
- Target macOS and Linux for local verification.
- Use a conservative MVP command surface plus one small addition, `wrk new --parent <id>`, because the sprint acceptance criteria require creating follow-up tickets under the parent after metadata updates work. All other parent, dependency, priority, label, and custom-field mutation flags stay deferred.

## Use Cases

1. A contributor runs `wrk` or `wrk --help` in a fresh checkout and sees command help with no interactive prompts.
2. A contributor runs `wrk init` in an empty project and gets a `.wrk/config.yaml` matching the documented initial configuration, without overwriting any existing `.wrk` directory or config.
3. A contributor runs `wrk new "Title"` and receives a new unique ticket ID plus path. The ticket has required frontmatter, configured defaults, status `todo`, and an optional body from `--body-file <path>` or `--body-file -`.
4. An agent runs `wrk list` from any subdirectory and sees active tickets sorted deterministically.
5. An agent runs `wrk list --ready` and sees only `todo` tickets whose dependencies are all `done`.
6. An agent runs `wrk show wrk-682f60c7` and sees the complete Markdown ticket plus derived children and dependency blockers.
7. An agent runs `wrk update <id> --title "New title" --status in-progress` and gets one validated metadata update that preserves the body exactly.
8. An agent directly edits a ticket body in Markdown, then runs `wrk validate` and confirms the repository is still structurally valid.
9. A contributor encounters invalid config, malformed YAML, duplicate keys, missing references, graph cycles, ID mismatches, or stale updates and receives a nonzero exit plus an actionable error with no partial writes.
10. After implementation, this repository dogfoods the CLI by showing the sprint ticket, marking it `in-progress`, creating deferred follow-up tickets under the parent, and marking the sprint ticket `done` only when the acceptance criteria are met.

## Architecture

Use a small Go module with clear internal package boundaries. Keep storage file-based and deterministic; do not introduce a database, daemon, network service, editor integration, or interactive UI.

Proposed source layout:

```text
cmd/wrk/main.go
internal/cli/
internal/config/
internal/ticket/
internal/store/
internal/validate/
internal/output/
testdata/
```

Contracts:

- `cmd/wrk` only wires command parsing, process exit codes, stdin/stdout/stderr, and command dispatch.
- `internal/config` discovers the nearest `.wrk`, parses `.wrk/config.yaml`, rejects duplicate YAML keys and unknown top-level settings, validates version and defaults, and never falls through to an outer project after finding an inner `.wrk`.
- `internal/ticket` owns the frontmatter schema, ticket ID format, status enum, relationship model, body splitting, and exact body preservation.
- `internal/store` owns filesystem reads/writes, project scans, ID generation, collision checks, atomic temp-file writes, lock acquisition, and stale-change detection.
- `internal/validate` validates one loaded project: config, ticket filenames, IDs, unique IDs, required metadata, custom field value types, parent graph, dependency graph, missing references, self-references, and duplicate dependencies.
- `internal/output` renders stable human output and JSON output. Command logic should build typed result structs and let output formatting stay centralized.

YAML parsing requirements:

- Use a parser that exposes mapping nodes so duplicate keys can be rejected before unmarshalling.
- Preserve unknown ticket frontmatter fields only if they are nested under `fields`; reject unknown built-in top-level ticket fields during validation.
- Preserve all existing ticket frontmatter values not modified by the command, including optional fields and unconfigured custom values.

Filesystem write contract:

- For `new`, generate IDs with the configured prefix plus eight random lowercase hex characters, then create the ticket with exclusive create semantics. If the file exists, generate another ID. Never truncate or overwrite.
- For `update`, read the ticket bytes, validate the entire project state, compute the new metadata, serialize the new frontmatter plus the original body bytes, acquire a per-project lock, reread the target file, and compare the reread bytes to the originally validated bytes. If they differ, fail with a stale-update error and leave the file unchanged.
- Write updates to a temporary file in `.wrk/`, fsync the file when supported, rename over the target, and clean up temporary files on failure. This is a CLI-level stale-write guard, not protection against arbitrary editors that ignore the lock.

Output contracts:

- Human output is stable and readable, with errors on stderr and no progress noise on stdout.
- `--json` emits a single JSON object for each command result.
- Mutating JSON results include at least `id`, `path`, and changed fields where applicable.
- List output sorts by status group, then priority, then ID unless implementation finds a simpler documented ordering; JSON must use the same deterministic ordering.
- Show output includes the full ticket content plus derived `children`, `depends_on`, and `blockers`.

Exit code contract:

- `0` success.
- `1` validation or user input error.
- `2` project discovery/configuration error.
- `3` stale update or filesystem conflict.

## Implementation

Phase 1: Bootstrap the executable and tests

- Create the Go module and `cmd/wrk`.
- Add command parsing for the required commands and global `--json`.
- Make bare `wrk` and `wrk help` print help.
- Add a smoke test that runs the compiled command in a temporary directory.
- Document the chosen local run command early, for example `go run ./cmd/wrk --help`.

Phase 2: Project discovery and configuration

- Implement upward discovery of the nearest `.wrk`.
- Implement `wrk init` for a specified directory or working directory.
- Parse config with duplicate-key detection, unknown-setting rejection, version checks, prefix validation, defaults validation, and custom-field definition validation.
- Verify invalid inner `.wrk/config.yaml` prevents falling through to an outer project.

Phase 3: Ticket parsing and validation

- Implement ticket file discovery for `.wrk/<id>.md`, excluding `config.yaml` and `.wrk/AGENTS.md`.
- Parse frontmatter and body while preserving original body bytes.
- Validate required fields, statuses, ID/filename match, optional priority/labels/parent/depends_on/fields types, and unknown top-level metadata.
- Validate configured custom fields while allowing unconfigured custom fields.
- Build derived children and dependency blocker data.
- Validate parent and dependency graphs separately, including missing references, self-references, duplicate dependencies, and cycles.

Phase 4: Read-only commands

- Implement `list`, `list --all`, `list --ready`, and contradictory flag rejection.
- Implement `show <id>` with full body, children, dependencies, and blockers.
- Implement `validate` as a read-only full-project check.
- Add JSON output for all read-only commands and assert deterministic ordering.

Phase 5: Ticket creation

- Implement `new "Title"`, configured defaults, random ID generation, collision retry, and exclusive create.
- Implement `--body-file <path>` and `--body-file -`.
- Implement the minimal `--parent <id>` addition for follow-up-ticket dogfooding. Validate the parent exists and does not create a parent cycle.
- Preserve scope by not adding dependency, priority, label, or custom-field mutation flags unless explicitly reauthorized.

Phase 6: Title/status update

- Implement `update <id> --title ...`, `--status ...`, and combined title/status updates.
- Reject an update with no change flags or invalid status/title input.
- Validate the proposed full project before writing.
- Preserve the Markdown body byte-for-byte and preserve unrelated metadata/custom values semantically.
- Add lock, reread, byte-compare, temp-file, rename, and stale-conflict behavior.
- Confirm blockers are reported but do not prohibit explicit status changes.

Phase 7: Documentation and dogfooding

- Update README with build/install/local-run instructions and working command examples.
- Update agent docs to state which metadata operations the CLI supports now and which remain deferred.
- Use the CLI to show `wrk-682f60c7`, mark it `in-progress`, create follow-up tickets under `wrk-3f8a21b7` for deferred metadata operations, and mark `wrk-682f60c7` `done` after all criteria pass.
- Keep the parent `wrk-3f8a21b7` open because broader metadata operations remain out of scope.

Verification sequencing:

- Unit tests start with parser/config behavior before command behavior.
- Integration tests use temporary project directories and subprocess execution.
- Compatibility tests copy this repository's current `.wrk` fixtures into temp directories; never mutate the live backlog during tests.
- Mutation tests assert failed operations leave target bytes unchanged.
- Body preservation tests compare exact bytes for UTF-8 content, CRLF, missing terminal newline, YAML-delimiter-like body text, and stdin body input.
- Conflict tests deterministically modify the target between initial read and guarded write through an injected hook rather than relying on timing.

## Files Summary

Files to create:

- `go.mod`: Go module definition.
- `cmd/wrk/main.go`: executable entry point.
- `internal/cli/*`: command parser and dispatch.
- `internal/config/*`: project discovery and config parsing/validation.
- `internal/ticket/*`: ticket schema, parsing, serialization, ID/status helpers.
- `internal/store/*`: project scan, locking, atomic writes, stale detection, ID creation.
- `internal/validate/*`: repository validation and graph checks.
- `internal/output/*`: human and JSON renderers.
- `testdata/wrk-project/*`: copied fixture data for compatibility tests.

Files to modify during implementation:

- `README.md`: local run/install instructions and command examples.
- `docs/index.md`: agent navigation to implemented CLI docs.
- `docs/ticket-format.md`: only if implementation clarifies error/output behavior without changing the agreed format.
- `docs/configuration.md`: only if implementation clarifies discovery or validation behavior without changing the agreed format.
- `.wrk/*.md`: only through the completed CLI dogfooding steps authorized by `wrk-682f60c7`.

Files that should not be introduced in this sprint:

- Databases, lock servers, editor integrations, credentials, remote service clients, assignment systems, or generated vendored dependencies.

## Definition of Done

- A fresh checkout can run or install `wrk` using documented commands.
- `wrk` with no arguments displays help.
- `init`, `new`, `list`, `show`, `update`, and `validate` match the ticket command contract.
- `new --body-file -` works with stdin.
- `list --all`, `list --ready`, and contradictory flag errors are covered.
- `update` accepts combined title/status changes and rejects unsupported metadata mutation flags for this sprint.
- Every command has readable default output and deterministic `--json`.
- Nearest-project discovery works from subdirectories; invalid inner config does not fall through.
- Existing repository tickets and config validate without migration.
- Invalid config, duplicate YAML keys, unknown config settings, unsupported version, invalid defaults, invalid custom-field definitions, malformed ticket metadata, ID/filename mismatch, duplicate IDs, missing references, self-references, duplicate dependencies, and parent/dependency graph cycles are diagnosed.
- Creation never clobbers an existing ticket.
- Title/status updates preserve body bytes and unrelated metadata/custom fields.
- Failed operations and stale updates leave existing files byte-identical.
- Automated tests cover parser/config validation, graph validation, output JSON, project discovery, creation, updates, body preservation, failure atomicity, and a subprocess-level workflow.
- README and agent docs accurately distinguish implemented title/status updates from deferred metadata operations.
- The CLI is used to show, start, create deferred follow-up tickets for, and complete `wrk-682f60c7` as authorized by that ticket.

## Risks

- YAML preservation can become fragile if implementation unmarshals into structs too early. Mitigate by parsing to nodes, validating duplicate keys, and serializing only the metadata map plus untouched body.
- Atomic rename does not by itself prevent lost updates. Mitigate with lock plus reread byte comparison and document the actual guarantee.
- Validation scope can block useful read commands if every malformed unrelated ticket aborts every command. For this sprint, require full validation before mutations and `validate`; allow `list` and `show` to report project validation errors clearly rather than silently ignoring invalid files.
- Adding `new --parent` slightly expands the command contract. Mitigate by keeping it creation-only and documenting it as necessary for the ticket's required follow-up dogfooding.
- Go YAML libraries differ in duplicate-key handling and comments/ordering preservation. Choose and test the parser behavior before building command logic on top.
- The repo is currently untracked. Implementation should avoid assuming committed baseline state and should not initialize git or rewrite unrelated files.

## Security

- Treat all ticket and config content as local repository data. Do not execute content from Markdown, YAML values, titles, labels, or custom fields.
- Do not add credentials or personal settings to `.wrk/config.yaml`.
- Sanitize paths for `--body-file`: read the user-supplied file path, but never write outside the discovered `.wrk` directory for ticket operations.
- Use exclusive file creation for new tickets and temp files to avoid clobbering.
- Keep JSON output properly escaped through the standard encoder.
- Avoid leaking unrelated file contents in errors; report paths and validation messages, not full raw ticket bodies unless the command is `show`.

## Dependencies

- Go toolchain available locally for development and tests.
- A YAML package that exposes node-level mappings and supports strict duplicate-key detection. The implementation should pin it in `go.mod`.
- Standard library filesystem primitives for exclusive create, rename, and locking support. If cross-platform file locking requires a small dependency, isolate it in `internal/store`.
- Existing docs and fixtures: `docs/ticket-format.md`, `docs/configuration.md`, `.wrk/config.yaml`, `.wrk/wrk-9d42c6a1.md`, `.wrk/wrk-3f8a21b7.md`, and `.wrk/wrk-682f60c7.md`.
- No network service, external database, or SaaS dependency.

## Open Questions

1. Should `new --parent <id>` be accepted as the minimal interface addition for required follow-up-ticket dogfooding, or should follow-up ticket creation wait for the broader parent milestone?
2. Should `list` and `show` abort on any unrelated invalid ticket, or should they render valid data while surfacing validation warnings? Mutations and `validate` should always require a valid project.
3. What exact JSON envelope should be considered stable for downstream agents: bare result objects, or `{ "ok": true, "result": ... }` plus `{ "ok": false, "error": ... }` for errors?
4. Are the proposed exit codes sufficient, or should the first release only guarantee nonzero failures?
5. Should update serialization preserve frontmatter key order exactly, or is semantic preservation of unrelated metadata sufficient as long as the body bytes are exact?
6. Is macOS plus Linux enough for this sprint, with Windows deferred until there is explicit demand?
