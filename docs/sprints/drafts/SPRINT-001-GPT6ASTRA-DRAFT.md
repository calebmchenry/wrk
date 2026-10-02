# Sprint 001: First Runnable wrk CLI — Independent GPT6Astra Draft

## Overview

Deliver the complete local ticket workflow in `wrk-682f60c7`: initialize a project, create work, find ready work, inspect it, start it, edit its Markdown description directly, finish it, and validate the repository. The CLI must work with the existing `.wrk` files without migration. The broader metadata-management ticket, `wrk-3f8a21b7`, remains open.

The scope authority is [the runnable milestone](../../../.wrk/wrk-682f60c7.md). [Ticket format](../../ticket-format.md) and [configuration](../../configuration.md) define correctness. [Sprint intent](SPRINT-001-INTENT.md) identifies unresolved design decisions. This document is an independent proposal, not authorization to implement or change ticket metadata.

### Repository evidence

- Read the requested intent, both `AGENTS.md` files, documentation index, ticket format, configuration specification, and milestone. Also inspected `README.md`, `.wrk/config.yaml`, and the other two real tickets.
- The repository contains documentation and three tickets; there are no source modules, tests, package manifests, or existing CLI to extend. All implementation paths below are proposed additions.
- Configuration is version `1`, prefix `wrk`, normal default priority, no default labels, and no configured custom fields.
- The format ticket is `done`; the parent and runnable milestone are `todo`. The milestone has a parent and depends on the completed format ticket. These are useful compatibility fixtures, but additional fixtures must exercise richer metadata.
- Git has no commits or remote, and the existing project files are untracked. Implementation must preserve them. A fresh-checkout verification can initially use a clean source copy; creating a commit or remote is not a prerequisite.
- The inspected machine has Go `1.25.4` on Darwin ARM64. That establishes local availability, not an approved minimum version, supported toolchain policy, or Linux verification.

### Requirements and proposed decisions

Requirements include the seed commands, exact body preservation, semantic preservation of unrelated metadata, strict validation, noninteractive execution, readable and JSON output, no-clobber creation, and useful failures. Requirements also include explicit status transitions, independent relationship graphs, and blockers that inform rather than prohibit status changes.

Proposed decisions are Go, macOS/Linux on local filesystems, strict project-wide validation, a small extension to creation flags, the JSON contract below, and a bounded concurrency contract. These decisions need ratification when the sprint is selected. In particular, accepting bounded editor-race detection is a clarification of the concurrency requirement, not a guarantee already present in the repository.

Exclude general metadata updates, dependency-editing commands, a body editor, interactive prompts, assignments, external services, a database, Git automation, and release distribution. Do not add abstractions for those features before they are needed.

## Use Cases

| User outcome | Supported interaction | Acceptance detail |
| --- | --- | --- |
| Run from a fresh source tree | Build or install locally; run `wrk` | Bare invocation displays help and exits successfully without requiring a project. |
| Start tracking a directory | `wrk init [directory]` | Defaults to the working directory; never modifies an existing project. |
| Create a small task | `wrk new "Title"` | Status is `todo`; configured prefix, priority, and labels apply. Output identifies the ticket and path. |
| Supply a description | `wrk new "Title" --body-file notes.md` or `--body-file -` | Read UTF-8 input before publishing; preserve its exact body bytes, including an absent final newline. |
| Create milestone follow-ups | `wrk new "Add parent updates" --parent wrk-3f8a21b7` | Proposed minimal parenting addition; validate the existing parent in this project. |
| Override creation defaults | `wrk new "Title" --priority high --label cli` or `--no-labels` | Proposed flags distinguish omitted labels from an explicitly empty list. |
| Find current work | `wrk list`, `wrk list --all`, `wrk list --ready` | Default includes `todo` and `in-progress`; ready means `todo` with every dependency `done`. |
| Inspect context | `wrk show <id>` | Display full ticket source, derived children, and unfinished dependency blockers. |
| Start or finish work | `wrk update <id> --status in-progress`, then `--status done` | Accept all four statuses; neither blockers nor child status trigger implicit changes. |
| Rename and transition together | `wrk update <id> --title "Revised title" --status todo` | Validate one candidate and publish at most one replacement. |
| Edit the explanation | Edit only the Markdown body using an editor | A later CLI update retains that body byte-for-byte. Concurrent editing follows the explicit storage contract below. |
| Audit integrity | `wrk validate` | Diagnose configuration, ticket, identity, and relationship errors without creating or modifying files. |
| Automate any command | Add `--json` | One documented JSON result on stdout, including failures; no prompts or progress noise. |

`list --all --ready`, unknown flags, duplicate scalar flags, an empty update, invalid statuses, and surplus positional arguments are usage errors. Repeated `--label` is deliberate; repeated `--title`, `--status`, or `--parent` is not. Support flags before or after positional arguments, as the milestone examples require, plus `--` for a title beginning with a dash. Accept `--json` before or after the subcommand.

The proposed creation extensions are limited to `--parent`, `--priority`, repeatable `--label`, and `--no-labels`. Specifying any labels replaces configured labels; it does not append to them. `--no-labels` conflicts with `--label`. Unspecified flags use defaults. Do not add creation-time status, dependency, or custom-field flags in this sprint. Without `new --parent`, the required parented follow-up tickets cannot be created through the scoped CLI; omitting it requires an explicit scope decision, not a manual frontmatter workaround.

## Architecture

### Language, packaging, and boundaries

Use a small Go module with `cmd/wrk` and internal packages. Go offers a direct local build and a standalone executable; the cost is more explicit argument parsing and filesystem code than a short scripting implementation. A published module path, package manager formula, and installer are unnecessary for this milestone. Use provisional module path `wrk` until a canonical remote exists.

The documented local procedure, once implementation exists, is `go mod download`, `go build -o ./bin/wrk ./cmd/wrk`, and `./bin/wrk`. Offer `go install ./cmd/wrk` as the optional installation path, explaining how to locate the Go binary directory and add it to PATH. Verify both procedures without inventing a remote-based install command.

Keep only two runtime dependencies: a YAML node parser and Unix locking support. Prefer `go.yaml.in/yaml/v3`, pinned to a tested release, because its node representation supports inspecting tags and retaining arbitrary YAML subtrees. Strict application validation remains our responsibility; decoding into a struct alone is insufficient. The package documents the node and decoding APIs needed for this approach. [YAML package documentation](https://pkg.go.dev/go.yaml.in/yaml/v3).

Use the standard library for JSON, random IDs, filesystem access, testing, and most command dispatch. A small explicit argument parser is acceptable for this surface, provided subprocess tests cover argument order, repeated flags, and `--`. Do not let a parser that stops at the first positional argument invalidate the documented syntax.

| Package | Responsibility | Must not do |
| --- | --- | --- |
| `internal/ticket` | Split frontmatter/body, inspect YAML nodes, validate fields, prepare metadata changes | Discover projects or write files |
| `internal/project` | Discover the boundary, validate config, load snapshots, index identities and graphs | Format CLI output or publish changes |
| `internal/store` | Serialize cooperating writers, compare snapshots, stage and publish complete files | Decide workflow or rewrite relationships |
| `internal/cli` | Parse arguments, orchestrate operations, render text/JSON and diagnostics | Duplicate domain validation |
| `cmd/wrk` | Connect process arguments and streams to the CLI; return an exit code | Contain business logic |

Keep test seams concrete and narrow: inject ID generation and hooks around storage comparison/publication. Do not introduce a generic storage backend, event system, or plugin interface.

### Project discovery and validation policy

Resolve the working directory and walk upward to the nearest entry named `.wrk`. Once found, it is authoritative. A missing config, invalid config, non-directory boundary, or forbidden symlink is an error; never continue to an outer project. `init [directory]` targets the supplied existing directory directly and may intentionally create a nested project.

Read configuration on every data command. Validate exact setting names, duplicate keys at every mapping level, version as integer `1`, prefix grammar, defaults, and custom definitions. Validate existing IDs against `^[a-z][a-z0-9]{0,15}-[0-9a-f]{8}$`, independently of the current configured prefix.

Only direct child filenames matching that ID pattern plus `.md` are tickets. Ignore supporting files, temporary files, and unrelated Markdown; do not recurse. Collect embedded IDs before building an ID-keyed map, so mismatched filenames and duplicate embedded IDs remain visible rather than overwriting each other in memory. Report both paths for a duplicate identity.

Proposed policy: all normal reads and mutations require a valid full project. `validate` aggregates all determinable diagnostics, sorted by path, field, and code. A malformed config prevents reliable schema validation; do not fabricate downstream type errors. Normal commands may return the same diagnostics without partial success. This trades permissive inspection for one consistent meaning of “valid project” and prevents misleading ready lists.

Allow one bounded repair: when a target has parseable frontmatter and valid identity, `update` may replace invalid or missing title/status fields that the supplied flags actually repair. Validate the entire resulting candidate project before publishing. Unrelated errors, malformed YAML, unknown keys, or an unrepaired target error still block it. No repair mode or graph recovery command is added. Document restoring known-good files and using the diagnostic paths; do not instruct agents to bypass the metadata editing contract.

### Ticket parsing and preservation

Treat a ticket as raw UTF-8 bytes plus a parsed frontmatter node. Require an opening `---` delimiter at the beginning and a closing delimiter on its own line; accept LF and CRLF. The body begins immediately after the closing delimiter's line ending. A closing delimiter at EOF represents an empty body. Never split on later delimiter-like body text.

Keep three representations: original file bytes for stale-write detection, original body bytes for exact preservation, and YAML nodes for field validation and controlled changes. A metadata update may reformat frontmatter, but may not change the body or the meaning of unrelated fields. Do not decode unknown custom values through `map[string]string` or JSON.

Require one YAML mapping document and reject unknown top-level ticket keys. Validate required keys and types, nonblank single-line titles, the four statuses, optional priorities, list types, relationship IDs, nonempty string labels, and a mapping for `fields`. Omission has the documented default meaning; explicit `null` is not equivalent to a missing typed field. Do not silently normalize stored strings or remove unrelated empty collections. The schema does not prohibit duplicate labels, so do not invent that restriction.

Configured custom values must match YAML-resolved types without coercion: strings remain strings, booleans remain booleans, numeric values are numeric scalars, and enums are strings matching an option. All custom values are optional. Unconfigured values retain their tagged scalar, mapping, sequence, or null representation, including nested values and numeric precision. Do not impose a JSON-only schema on them.

For an update, clone the parsed tree, replace only the supplied title/status entries, serialize frontmatter, and append the untouched body. Reparse the complete candidate and compare all unrelated metadata structurally, including scalar tags and values. Never publish if preservation cannot be demonstrated. YAML anchors and aliases need an explicit feasibility test: changing an anchored title must not change an unrelated alias's effective value. Materialize affected aliases from the old value when safely representable; otherwise fail unchanged with `PRESERVATION_UNSUPPORTED`. Do not claim universal YAML round-tripping until the tests establish the supported boundary. Any necessary format restriction is a decision to document, not an implicit migration.

### Relationships and derived state

Build separate parent and dependency adjacency lists. Validate existence, self-references, duplicate dependencies, and cycles with separate traversals and useful cycle paths. A parent depending on a child is valid when neither individual graph has a cycle. Derive children from `parent`; never persist reciprocal lists.

A blocker is a dependency whose status is anything other than `done`; `canceled` therefore blocks. `ready` is exactly `status == todo && blockers.length == 0`. Compute these values without changing statuses. Completing, canceling, or reopening a ticket never changes another ticket, and updating a blocked ticket is permitted.

### Output contract

Use the same domain result for text and JSON. Text list rows show ID, status, priority, title, and blocker IDs. Sort tickets and children lexicographically by ID, and blocker arrays by dependency ID. `show` prints full ticket source followed by clearly separated derived sections. Escape terminal control characters in human output; JSON retains the actual source through standard JSON escaping.

Every command supports a versioned envelope:

```json
{
  "schema_version": 1,
  "command": "update",
  "ok": true,
  "project_root": "/absolute/project",
  "result": {
    "ticket": {
      "id": "wrk-a7f39c21",
      "path": ".wrk/wrk-a7f39c21.md",
      "title": "Example",
      "status": "in-progress",
      "parent": null,
      "depends_on": [],
      "priority": "normal",
      "labels": [],
      "blockers": []
    },
    "changed": true
  },
  "errors": []
}
```

Paths are slash-separated and relative to `project_root`; the root is absolute and invariant when invoked from different subdirectories. Before discovery, `project_root` is `null`. Runtime output has no timestamps or random request IDs. Deterministic output means the same snapshot and operation produce the same ordering and values, apart from intentional randomness when creating a ticket.

| Command | `result` on success |
| --- | --- |
| `init` | `created: true`, `path: ".wrk"`, `config_path: ".wrk/config.yaml"` |
| `new` | Common ticket summary, `created: true` |
| `list` | `tickets` array of common summaries, including blocker ID/status records |
| `show` | Common ticket summary, exact `source` string, derived `children` summaries |
| `update` | Common ticket summary, `changed` boolean; an effective no-op does not rewrite the file |
| `validate` | `valid: true`, `ticket_count` |
| Bare invocation or help | `usage` string when JSON was requested; readable help otherwise |

`source` is the complete UTF-8 ticket, including custom fields and body. This deliberately avoids a lossy YAML-to-JSON conversion for arbitrary custom values. A typed custom-field JSON projection can be designed with the later custom-field editing work.

On ordinary failure, set `ok: false`, `result: null`, and include ordered errors with `code`, `message`, and applicable `path`, `field`, and involved IDs. `validate` uses this shape for invalid repositories. Stable exit categories: `0` success/help, `2` usage, `3` invalid project or proposed data, `4` missing project/ticket or existing init target, `5` conflict/busy, `1` I/O or unexpected failure. Codes such as `INVALID_CONFIG`, `MISSING_REFERENCE`, `CONFLICT`, and `PRESERVATION_UNSUPPORTED` give more precision than the exit category.

JSON mode emits exactly one object plus a newline on stdout for commands that can report a result, with no ordinary diagnostic duplicate on stderr. Human-mode errors go to stderr. Broken output pipes and process termination cannot guarantee delivery of a JSON error. Serialize the success result before publication where possible; failure to deliver stdout after a successful mutation must not trigger a filesystem rollback.

### Storage and concurrency contract

The literal requirement to never overwrite any concurrent edit cannot be guaranteed by a portable check-then-rename operation against an editor that ignores locks. Atomic rename prevents torn publication; it is not compare-and-swap. This sprint must explicitly choose its supported concurrency boundary before declaring that criterion complete.

Recommended contract:

1. Cooperating CLI writers serialize through an advisory project lock held from snapshot load through publication. Acquire it without interactive waiting; contention returns `BUSY`. Use a persistent `.wrk/.lock` inode and never unlink it on release, avoiding competing locks on different inodes. Add it to generated/local ignore rules. The descriptor releases the lock on process exit; a surviving file does not indicate a stale held lock.
2. After locking, load configuration and all ticket bytes, validate, and prepare the candidate. Before publication, recheck the ticket filename set and exact contents of config and tickets. Detect a changed body, config, relationship, deletion, or atomic editor replacement even when timestamps and lengths match. Return `CONFLICT` without publishing and without automatic retries.
3. Stage the complete candidate in a unique temporary file in `.wrk`, check writes and close/sync results, retain appropriate file permissions, and repeat the comparison immediately before publication.
4. For creation, generate four cryptographically random bytes and encode eight lowercase hex characters. Check both loaded identities and destination paths. Publish using a same-filesystem no-replace operation, such as linking the complete staged file into its final pathname. A destination collision retries with a new ID, at most 32 candidates; never use a plain overwriting rename for creation. After a publication collision, reload and validate the snapshot before retrying; only the newly occupied ID may be incorporated, and any other snapshot change returns `CONFLICT`. Fail safely if the filesystem cannot support the selected publication primitive.
5. For updates, atomically replace the target pathname only after successful comparison. Preserve all body bytes and unrelated metadata from the loaded target. Flush the directory where supported and clean up only this operation's temporary files.

This prevents lost updates among CLI writers and rejects external edits visible at the final comparison. It cannot prevent an editor from writing between that comparison and replacement, or a multi-file snapshot from changing after its final check. The recommended operating condition is no simultaneous noncooperating edits during a metadata command. Sequential direct body edits remain fully supported. A stronger guarantee requires editor participation or a platform-specific storage design and is a scope decision, not a late implementation detail.

Read commands and `validate` do not create locks or temporary files. They load and recheck snapshots, returning conflict on observable changes; they do not claim a transactionally consistent view during arbitrary external writes. Mutation commands revalidate after obtaining their exclusive lock even if data was read earlier.

Separate pre-publication failure from post-publication uncertainty. Validation failures, conflicts, staging failures, and failed publication leave existing ticket/config bytes unchanged. A directory-sync failure or termination after publication may leave a complete committed change without acknowledged success. If reportable, return an I/O error with `published: true`, affected ID/path, and an instruction to inspect before retrying; do not promise rollback. A failure to remove a temporary hard link after successful publication is cleanup failure, not evidence that ticket creation failed.

`init` uses exclusive directory creation and refuses any preexisting `.wrk`, including an empty or invalid one. Write and publish config without replacement; do not reuse an outer project's settings. On pre-publication failure, remove only files created by this invocation and remove its directory only if empty. Never recursively remove an existing directory. A crash may leave an incomplete boundary; document inspection and recovery. It must never leave a partially written config presented as complete.

Lock/temp artifacts are implementation details, excluded from ticket scanning and Git. `init` may create a small `.wrk/.gitignore` for these artifacts only in the new directory; it must not generate or overwrite agent instructions. Existing projects receive equivalent ignore rules through implementation documentation/repository setup.

## Implementation

### Phase 1 — Establish executable and settle risky contracts

- Add `go.mod`, pinned dependencies, `cmd/wrk/main.go`, command dispatch, help, and structured diagnostics. Select and record a supported Go toolchain; verify dependency compatibility instead of treating the locally installed version as the release baseline.
- Confirm Go and macOS/Linux scope, creation extensions, strict validation behavior, and the bounded concurrency decision. Record accepted contracts in `docs/cli.md` and storage limitations in `docs/storage.md` during implementation.
- Run focused feasibility tests for YAML typed/nested value preservation, anchors affecting an updated field, no-replace creation, advisory locks, and replacement behavior on the supported systems. These are the highest-risk dependencies of later phases.
- Implement test hooks for deterministic collisions and edits between snapshot, staging, comparison, and publication. Keep hooks internal to tests; do not expose fault injection through CLI flags or environment variables.

**Exit gate:** a clean source copy builds, bare invocation/help works, both output modes have an envelope, and storage/YAML limitations are explicit. If the stronger editor guarantee remains required, revise storage scope before proceeding; a hash-plus-rename implementation cannot discharge it.

### Phase 2 — Read and validate the existing project

- Implement nearest-boundary discovery and strict config parsing, including configured custom types and defaults.
- Implement byte-based frontmatter/body separation, node-level schema checks, identity indexing, relationship validation, and derived children/blockers.
- Implement `validate`, `list` and its filters, and `show` using the common result model.
- Copy the three real tickets and config into fixtures without altering the live backlog. Add synthetic fixtures for optional metadata and edge cases absent from the current project.

**Exit gate:** the real fixture validates; default/ready/all selections are correct; `show` includes complete source and accurate derived context; invalid inner projects never fall through. No read operation changes the fixture tree.

### Phase 3 — Publish safe updates and begin dogfooding

- Implement lock acquisition, snapshot comparison, staging, atomic replacement, permission preservation, and cleanup/failure classification.
- Implement combined title/status changes, the bounded repair rule, candidate-project validation, and no-op handling. Reparse and compare candidate metadata before writing.
- Test exact body preservation and unrelated metadata semantics, including custom values and default omission. Exercise stale-edit detection through deterministic hooks, without relying on sleep-based races.
- After temporary-project tests pass, use the new CLI to show `wrk-682f60c7` and set it to `in-progress`. This is the first live metadata mutation; record evidence in its body only after the command succeeds.

**Exit gate:** update behavior, JSON results, preservation checks, and pre-publication failure assertions pass. Status changes touch only the target and do not enforce blockers or cascade.

### Phase 4 — Initialize and create tickets

- Implement `init [directory]`, version-1 default configuration, exclusive publication, and safe cleanup of newly created artifacts.
- Implement `new`, cryptographic IDs, bounded collision retry, body-file/stdin ingestion, and validated `todo` creation. An omitted body file creates an empty body; resolve an explicit file path relative to the invoking working directory, not the project root.
- Implement the agreed parent and default-override flags. Validate parent references against the locked current snapshot. Read and validate body input before taking the mutation lock so waiting on stdin does not hold the project lock.
- Verify that prefix changes affect only new IDs, existing IDs remain resolvable, and defaults apply at creation only.

**Exit gate:** creation cannot overwrite a preexisting destination; stdin and file bodies are exact; failed creates publish no partial ticket; follow-up tickets can be created under the existing parent without broader metadata updates.

### Phase 5 — Verify the complete workflow and document it

- Build the binary and run subprocess tests in disposable projects, including invocation from nested directories and both output modes for every command.
- Test the full loop: init, new, list/ready, show, start, direct body edit, combined title/status update, finish, list/all, validate. Include a blocked ticket whose explicit status change succeeds.
- Run `go test ./...`, `go test -race ./...`, `go vet ./...`, and `go build -o ./bin/wrk ./cmd/wrk` on supported environments. Check formatting. Run actual macOS and Linux tests; an authored CI workflow or cross-compilation alone does not establish Linux filesystem behavior.
- Verify the documented fresh-source procedure using a clean temporary source copy with no prebuilt binary or repository-specific environment assumptions. Dependency download is allowed for building; normal CLI operation must be offline.
- Update README with local build/install commands, PATH guidance, working examples, and direct body-edit guidance. Update agent docs to enumerate supported operations and deferred metadata commands; remove the blanket “CLI has not been implemented” statement only when true.
- Document JSON schemas, error/exit semantics, path/sort conventions, preservation boundaries, the invalid-project policy, lock behavior, crash recovery, and the editor-race limitation. Link new documentation from `docs/index.md`.

**Exit gate:** recorded macOS/Linux test and clean-source evidence, readable examples that actually run, and documentation matching the shipped interface.

### Phase 6 — Complete the milestone through the CLI

- Use `new --parent wrk-3f8a21b7` to create follow-up tickets for parent/dependency changes and priority/label/custom-field updates. Capture acceptance criteria and validation/preservation expectations in their bodies. Check for equivalent tickets before creating duplicates.
- Retain the broader parent's current status unless explicitly transitioning it for ongoing work; never mark it done as a consequence of this sprint.
- Review the milestone criteria and evidence, update its body checklist and handoff notes, run `validate`, then use `update wrk-682f60c7 --status done` only after all gates pass. Validate again and inspect the final JSON/text results.

**Exit gate:** follow-ups exist under the parent, the milestone is complete through the CLI, the parent remains open, and the live backlog validates. Do not introduce Git commits, a remote, or publication as hidden completion requirements.

### Verification matrix

| Boundary | Meaningful automated coverage |
| --- | --- |
| Config/discovery | Nested discovery; missing/invalid inner config; unknown settings; duplicate nested keys; wrong version/type; invalid prefix/defaults/enum definitions |
| Parser/preservation | UTF-8, LF, CRLF, empty body, no terminal newline, later `---` lines, malformed delimiters, nested custom values, scalar types, anchored values, semantic no-op |
| Identity | Old prefix after config change; filename mismatch; duplicate embedded IDs; forced random collision; destination created immediately before no-replace publish |
| Graphs/workflow | Missing refs, self edges, duplicate dependencies, independent cycles, valid parent-depends-on-child, canceled blocker, all statuses, no cascade |
| CLI surface | Flag ordering, `--`, stdin, default overrides, explicit empty labels, combined update, contradictory flags, empty update, no project help |
| JSON | Every command success; usage/project/conflict failures; valid JSON on stdout alone; stable arrays/paths; exact source round-trip; mutation IDs and paths |
| Failure safety | Lock contention; same-size/same-timestamp edits; atomic editor replacement; changed config/other ticket/set; write/sync/publish errors; no-clobber; unchanged existing bytes before publication |
| Crash/publication | Subprocess stopped before publication leaves old files; after publication leaves a complete new file; residual temp files are ignored; post-publication errors identify uncertainty |
| Compatibility | Original three-ticket fixture works without migration; live docs and bodies are not rewritten by validation or ordinary reads |

Inject an edit before the final comparison and require conflict without overwrite. Separately exercise an edit after that comparison to characterize the unsupported race; do not present the earlier conflict test as proof that the later window is closed. Filesystem assertions compare bytes and directory entries, not just command exit codes.

## Files Summary

All implementation changes below belong to future execution. Producing this draft writes only `docs/sprints/drafts/SPRINT-001-GPT6ASTRA-DRAFT.md`.

| Path | Planned change |
| --- | --- |
| `go.mod`, `go.sum` | New local module and reproducibly pinned dependencies |
| `cmd/wrk/main.go` | Thin executable entry point |
| `internal/cli/{args,run,output,errors}.go` | Command parsing, orchestration, output contracts |
| `internal/ticket/{parse,validate,update}.go` | Byte-preserving ticket representation and candidate validation |
| `internal/project/{discover,config,snapshot,validate,graph}.go` | Project boundary, config, indexes, integrity and derived state |
| `internal/store/{lock_unix,write,init}.go` | Supported-platform locking, comparison, staged publication |
| `internal/**/*_test.go`, `test/integration/cli_test.go` | Focused unit, filesystem, and subprocess tests |
| `testdata/compat/`, `testdata/invalid/` | Read-only copies of current data and focused edge fixtures |
| `.gitignore`, `.wrk/.gitignore` | Ignore local binary and exact runtime artifact patterns |
| `.github/workflows/test.yml` | macOS/Linux verification if hosted CI is adopted; no implied remote creation |
| `README.md` | Verified local build/install and command examples |
| `docs/cli.md`, `docs/storage.md` | Interface, JSON, diagnostics, persistence guarantees and recovery |
| `docs/index.md`, `.wrk/AGENTS.md` | Navigate new docs and distinguish supported/deferred metadata operations |
| `docs/ticket-format.md`, `docs/configuration.md` | Clarify accepted edge semantics only; preserve format version and established contracts |
| `.wrk/wrk-682f60c7.md`, new follow-up ticket files | During dogfooding only: CLI metadata changes and directly edited evidence/checklists |

No migration, database, reciprocal relationship fields, or rewrite of the broader parent ticket is planned. Keep the original compatibility fixture immutable even after live dogfooding changes the milestone.

## Definition of Done

- [ ] Accepted decisions explicitly cover language/platforms, creation flags, invalid-project handling, YAML preservation boundaries, and the concurrency guarantee.
- [ ] A clean source copy builds or installs through the README procedure and bare `wrk` displays help.
- [ ] Every seed command works noninteractively, including stdin bodies, combined updates, ready/all filters, all statuses, and per-command JSON.
- [ ] Existing tickets/config need no migration; old-prefix IDs and deferred metadata fields remain valid and preserved.
- [ ] Configuration, duplicate keys, IDs, types, references, and both graphs are validated with useful diagnostics and no writes from `validate`.
- [ ] Creation uses cryptographic local IDs and atomic no-clobber publication; forced collisions never overwrite tickets.
- [ ] Updates retain exact body bytes and unrelated metadata semantics. Failed validation and detected conflicts leave existing ticket/config bytes unchanged.
- [ ] The agreed CLI/editor concurrency boundary is implemented, deterministically tested, and documented without claiming rename is compare-and-swap.
- [ ] Post-publication uncertainty is distinguishable from pre-publication rejection; no blind rollback can overwrite subsequent edits.
- [ ] Blockers, canceled prerequisites, explicit transitions, separate graphs, and no-cascade behavior match the format specification.
- [ ] JSON envelopes, paths, ordering, source representation, mutation results, and error categories are documented and subprocess-tested.
- [ ] Core tests, race checks, vet, build, actual macOS/Linux verification, and clean-source execution evidence pass and are recorded.
- [ ] README and agent docs accurately describe the available CLI and remaining metadata scope.
- [ ] CLI-created follow-ups belong to the parent; the milestone is shown, started, and finished through the CLI only after its acceptance criteria pass.
- [ ] Final live validation succeeds and `wrk-3f8a21b7` remains open.

Do not mark the sprint complete merely because sequential workflow tests pass while concurrency acceptance, Linux verification, or YAML preservation scope remains unresolved.

## Risks

| Risk | Consequence | Mitigation / decision |
| --- | --- | --- |
| Treating a final hash check as compare-and-swap | False safety claim and a possible overwritten editor save | Ratify the bounded contract before storage implementation; require a different design if all editor races must be prevented. |
| YAML round-trip changes unrelated values | Corrupted custom data or changed types despite an unchanged body | Node representation, reparse/semantic comparison, anchor tests, and fail-closed preservation checks. Resolve unsupported valid constructs explicitly. |
| Strict validation prevents ordinary reads | One damaged file makes list/show unavailable | Aggregate precise diagnostics; retain direct file readability; allow only targeted title/status repair. Defer broad recovery rather than quietly weakening consistency. |
| Safety primitives differ across filesystems/platforms | No-clobber or locking assumptions fail outside tested environments | Scope to tested local filesystems; fail safely on unsupported primitives; obtain real macOS/Linux test evidence. |
| Broadening creation becomes general metadata CRUD | First runnable milestone stalls behind the parent scope | Add only parenting and default overrides needed for the documented workflow; create follow-up tickets for everything else. |
| Early JSON design constrains later custom fields | Lossy values or a breaking machine interface | Version the envelope and expose exact source now; design a richer projection when custom-field operations exist. |
| An error follows successful publication | A retry creates duplicate work or masks a completed update | Report affected IDs/paths and publication state when possible; document inspection before retrying, especially after interrupted `new`. |
| Live dogfooding replaces untracked user work | Irrecoverable loss without Git history | Establish safety tests on copies first; limit live changes to CLI-authorized ticket operations; never bootstrap by replacing the existing `.wrk`. |

Prioritize storage and YAML feasibility before completing the command surface. Those choices constrain every mutation, output guarantee, and completion test; postponing them would create expensive cross-phase rework.

## Security

- Treat repository YAML, Markdown, filenames, and command arguments as data. No template evaluation, shell execution, dynamic imports, network access, or Git subprocesses are needed during normal operation.
- Accept IDs only through the documented grammar before constructing ticket paths. Read and publish only direct children of the discovered `.wrk`; reject symlinked config/ticket files and a symlinked boundary with actionable diagnostics. Ordinary user-selected body-file paths are inputs, not destinations.
- Do not claim protection against a hostile process replacing ancestor directories or bypassing filesystem permissions. The supported model is a locally owned project with cooperative CLI writers; the editor race limitation remains explicit.
- Use node parsing without application-object construction. Test pathological aliases and nesting; apply explicit, documented resource limits if needed, returning errors before writing. Such limits must not silently discard custom values.
- Escape control sequences in human terminal output. Preserve content in JSON using standard escaping so source data is not executed or confused with terminal controls.
- Create unique staging files without following preexisting links, preserve restrictive permissions when replacing existing tickets, and never broaden them merely to match a default mode. Clean up only artifacts owned by the operation.
- Store no credentials in config, tickets, or runtime artifacts. Validation diagnostics should identify file/field locations without dumping entire custom values unnecessarily.

## Dependencies

- A ratified scope and concurrency contract. The editor guarantee is the principal product dependency, not just an implementation-library choice.
- A supported Go toolchain and module downloads for initial build. Pin exact dependency versions and checksums during implementation; do not rely on whatever release happens to be latest on a developer's machine.
- `go.yaml.in/yaml/v3` or an equivalent node-capable parser that passes the preservation feasibility tests. Prefer changing the parser to silently narrowing the documented data model.
- `golang.org/x/sys/unix` for platform locking primitives if the standard-library approach would obscure platform behavior. Keep its use inside the store package. [Unix package documentation](https://pkg.go.dev/golang.org/x/sys/unix).
- macOS and Linux execution environments with tested local filesystem semantics. Windows, network filesystems, and cloud-synchronized directories are deferred support targets.
- Existing version-1 docs and real tickets as compatibility inputs. No remote, external service, database, distribution channel, or new Git history is required.

Implementation dependency order is executable/contracts → parser/config/graphs → safe updates → creation/init → end-to-end verification/docs → live completion. Command implementations must share the same validator, snapshot model, and renderer throughout that sequence.

## Open Questions

| Question | Recommended decision | When it must be resolved |
| --- | --- | --- |
| Is Go with macOS/Linux the intended first target? | Adopt it; select a supported tested toolchain and document the minimum. | Before package/dependency setup. |
| Does the concurrency criterion require protection against an arbitrary editor saving in the final check-to-replace window? | Accept serialized CLI writers plus deterministic stale-file detection under a documented no-simultaneous-editor condition. If unacceptable, expand storage/editor coordination scope explicitly. | Before committing to the storage protocol; blocks final acceptance. |
| May `new` accept parent and default-override flags? | Add `--parent`, `--priority`, `--label`, and `--no-labels`; keep all general metadata updates deferred. | Before final command tests and follow-up-ticket creation. |
| Should malformed unrelated tickets block list/show as well as mutations? | Yes for this milestone, with precise validation diagnostics and bounded title/status repair. | Before snapshot/command orchestration is finalized. |
| How broad is the supported YAML preservation boundary? | Preserve all ordinary tagged scalar and nested values; prove anchor handling early. Any remaining unsupported valid construct must be named and accepted, never silently coerced. | Phase 1 feasibility and before preservation is declared complete. |
| Is exact ticket source sufficient for custom values in JSON v1? | Yes; provide normalized built-ins plus full source, avoiding an accidental JSON-only custom schema. | Before output contract tests. |
| Is refusing every existing `.wrk` on `init` acceptable? | Yes; deterministic no-overwrite behavior is simpler than implicit repair. Document incomplete-init recovery separately. | Before init implementation. |
| Where will actual Linux and fresh-source evidence be recorded before a remote exists? | Use available local/container Linux execution or later hosted CI, and record concrete commands/results in the milestone body. A clean source copy is acceptable until a real checkout exists. | Before the completion gate; do not invent evidence. |

These are explicit decision points for selecting the sprint. The draft assumes the recommended answers for implementation sequencing, but does not convert those assumptions into already-approved requirements.
