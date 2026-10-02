# Sprint 001 Draft: First Runnable `wrk` CLI

## Overview

### Objective

Deliver the first locally runnable `wrk` CLI and use it to complete the workflow in `wrk-682f60c7`: initialize a project, create tickets, discover work, inspect relationships, update title and status, validate the repository, and dogfood those commands on this repository.

This is an implementation sprint. The command and acceptance contract in `.wrk/wrk-682f60c7.md` is the scope authority. `docs/ticket-format.md` and `docs/configuration.md` are the data and validation authorities. The broader metadata operations in `.wrk/wrk-3f8a21b7.md` remain outside this sprint.

### Repository evidence

- The repository currently contains documentation, `.wrk/config.yaml`, and three tickets. There is no application source, module/build configuration, automated test suite, commit, or configured Git remote.
- All current files are untracked user work. Implementation must add files without replacing or normalizing existing repository content.
- The current `.wrk/config.yaml` uses format version `1`, prefix `wrk`, normal priority, empty labels, and no custom-field definitions.
- The existing tickets exercise an older-prefix-compatible ID model, parenting, dependencies, missing optional fields, and a completed dependency. They are compatibility fixtures, not files to rewrite during parser development.
- The available local Go toolchain is Go 1.25.4 on macOS/arm64. This is evidence about the development machine, not proof of the minimum supported toolchain.

### Requirements, sprint decisions, and assumptions

| Classification | Contract |
| --- | --- |
| Requirement | Implement `init`, `new`, `list`, `show`, `update` for title/status, and `validate`, including bare-command help and noninteractive behavior. |
| Requirement | Support readable output and per-command `--json`; mutation results contain affected IDs and project-relative paths. |
| Requirement | Treat the nearest `.wrk/` as the project boundary even when its configuration is absent or invalid. |
| Requirement | Read existing version-1 config and tickets without migration; validate strict config, ticket identity, field types, and both relationship graphs. |
| Requirement | Preserve body bytes exactly and preserve the semantic values of unrelated metadata and custom fields on update. |
| Requirement | Never clobber a ticket during creation; reject failed or stale mutations without publishing a partial result. |
| Sprint decision | Implement in Go, with a small command parser rather than a CLI framework. Use `go.yaml.in/yaml/v4` for YAML nodes and `golang.org/x/sys/unix` for Unix advisory locking. |
| Sprint decision | Target macOS and Linux local filesystems. Windows and network/distributed filesystem guarantees are deferred. |
| Sprint decision | Add `wrk new "Title" --parent <id>` as the only command-surface addition. It is necessary to create the required follow-up tickets under `wrk-3f8a21b7` without violating the metadata editing contract. It does not add parent updates. |
| Sprint decision | Before any command other than `init` succeeds, load and validate one complete project snapshot. An unrelated invalid ticket therefore blocks reads and mutations; `validate` is the recovery diagnostic and manual file repair is the recovery mechanism. |
| Sprint decision | JSON and diagnostic ordering are stable contracts in version 1; paths in output are slash-separated and relative to the project root. |
| Assumption requiring confirmation | Go 1.25 is an acceptable initial minimum and `module wrk` is acceptable until a canonical remote module path exists. |
| Assumption requiring confirmation | The concurrency acceptance criterion permits an explicitly documented best-effort boundary for non-cooperating editors; ordinary filesystem APIs cannot provide a true compare-and-swap replacement against them. |

### In scope

- A Go module and `cmd/wrk` executable.
- Strict configuration and ticket parsing, project discovery, repository validation, derived children/blocker data, and deterministic diagnostics.
- Atomic/no-clobber storage behavior suitable for APFS and common Linux local filesystems.
- The exact command surface in the seed ticket plus creation-time `--parent`.
- Human and JSON output contracts, stable exit behavior, unit/integration/subprocess tests, README examples, and agent documentation.
- Dogfooding the implemented CLI against the live repository only after the automated tests pass.

### Out of scope

- Updating parents, dependencies, priority, labels, or custom fields after creation.
- Creation-time priority, label, dependency, or custom-field overrides.
- An editor, body update command, interactive prompt, TUI, daemon, database, network service, assignment system, authentication, or external integration.
- Automatic transitions, cascades, a `blocked` status, custom statuses, timestamps, comments, or history outside Git.
- Automatic repair, migration, or canonical rewriting of existing files.
- Windows support, distributed locking, and correctness claims for network filesystems.
- Completing the broader `wrk-3f8a21b7` parent.

## Use Cases

### 1. Initialize a project

`wrk init [directory] [--json]` initializes an existing directory, defaulting to the working directory.

- If `.wrk/` does not exist, create it and `.wrk/config.yaml` with the documented version-1 initial configuration.
- If `.wrk/` exists without a config and contains no ticket files, preserve support files such as `AGENTS.md` and create the config with exclusive creation.
- If a config or any ticket already exists, fail without changing anything. Initialization is not a repair or migration command.
- Do not discover or modify an outer project while initializing an explicit target.
- A nonexistent target directory is an error; `init` creates `.wrk/`, not arbitrary parent directories.
- Bare `wrk` and `wrk help` print top-level help to stdout and exit zero. Command usage mistakes print command help to stderr and exit nonzero.

### 2. Create a ticket

`wrk new "Title" [--body-file <path>|--body-file -] [--parent <id>] [--json]` creates one ticket.

- Require exactly one non-whitespace, single-line title. Preserve the supplied title text rather than trimming or normalizing it.
- Read a body file as exact UTF-8 bytes. `--body-file -` consumes stdin through EOF and never prompts. With no body flag, the logical body is empty.
- Apply the configured prefix, priority default, and label default. A new ticket has status `todo`; custom fields and dependencies are absent.
- If `--parent` is supplied, require an existing ticket and validate the resulting parent graph before publication.
- Generate four cryptographically random bytes and encode them as eight lowercase hexadecimal characters. Retry on collisions.
- Publish the new path with a no-replace primitive. A collision can cause a retry but can never overwrite an existing file.
- Human output prints the ID and `.wrk/<id>.md`. JSON returns those same values in named fields.

### 3. Find active and ready work

- `wrk list` returns `todo` and `in-progress` tickets, including blocked tickets.
- `wrk list --all` returns all four statuses.
- `wrk list --ready` returns only `todo` tickets with no dependencies or with every dependency in `done`.
- `canceled` does not satisfy a dependency.
- `--all --ready` is a usage error. Unknown flags and extra positional arguments are errors.
- Results, children, dependencies, and blockers are ordered lexicographically by ID so output does not depend on directory iteration order.

### 4. Inspect a ticket

`wrk show <id> [--json]` resolves an exact ID in the current project.

- Human output contains the complete stored ticket followed by clearly separated, derived child and blocker summaries. The derived information is never written into the ticket.
- JSON contains all built-in fields, custom fields, the Markdown body as a string, the project-relative path, derived children, and unfinished dependency blockers.
- An absent optional `priority`, `labels`, or `fields` value is exposed with its semantic default (`normal`, `[]`, or `{}`) without rewriting the source file.

### 5. Update title and status without losing a direct body edit

`wrk update <id> [--title <title>] [--status <status>] [--json]` requires at least one update flag and permits both together.

- Accept exactly `todo`, `in-progress`, `done`, and `canceled`; reopening is allowed.
- Report blockers but do not prevent an explicit status change.
- Parse and validate the complete proposed repository before writing.
- Preserve every byte after the closing frontmatter delimiter, including CRLF, a missing terminal newline, and delimiter-like lines in the body.
- Preserve unrelated metadata values, including nested unconfigured custom values and their YAML scalar types. Metadata formatting and comments should be retained where the YAML library permits, but byte-for-byte frontmatter formatting is not a contract.
- If any input snapshot changes before commit, report a conflict and leave the target unchanged. The user can rerun the command against the new state.

### 6. Validate a project

`wrk validate [--json]` performs no writes and reports all independently discoverable diagnostics in deterministic order.

Validation covers:

- configuration syntax, duplicate keys, unknown settings, version, prefix, defaults, and custom-field definitions;
- ticket frontmatter syntax, required and optional field types, duplicate keys, unknown top-level fields, UTF-8, ID shape, filename/ID agreement, and duplicate frontmatter IDs;
- configured custom-field types while allowing unconfigured values in `fields`;
- missing relationships, self-references, duplicate dependencies, parent cycles, and dependency cycles, with each graph checked independently.

The command exits zero only when no error diagnostics exist. It should continue structural ticket checks when a bad configuration makes configured-field checks unavailable, and clearly label skipped checks.

### 7. Use JSON safely from automation

`--json` is a command-local flag and may appear anywhere after the subcommand. `wrk --json list` is not supported in this sprint.

- Successful JSON is the only stdout output; progress and warnings never contaminate stdout.
- JSON failures produce a single error envelope on stderr and leave stdout empty.
- Human failures are concise on stderr and include the path, diagnostic code, and remediation where known.
- Exit codes are: `0` success, `1` I/O or conflict failure, `2` usage failure, and `3` invalid project/config/ticket data.

## Architecture

### Component boundaries

```text
cmd/wrk
  -> internal/cli       argument parsing, help, output selection, exit mapping
  -> internal/app       command orchestration and use-case results
      -> internal/project   discovery and immutable snapshot loading
      -> internal/config    config YAML decoding and validation
      -> internal/ticket    ticket split/decode/encode and field validation
      -> internal/graph     separate parent/dependency graph checks and derivations
      -> internal/storage   locks, fingerprints, temp files, atomic publication
```

Domain and application packages return typed results and typed diagnostics; they do not print or terminate the process. Only `cmd/wrk` maps an application result to human or JSON output and an exit code. This keeps command behavior testable without subprocesses while retaining a small number of end-to-end subprocess tests.

### CLI parsing contract

A small purpose-built parser is preferable to adding a CLI framework for six commands. It must support:

- `--flag value` and `--flag=value`;
- flags before or after positional values within a subcommand, including the required `wrk new "Title" --body-file file` order;
- `--` to end flag parsing;
- duplicate scalar flags as a usage error rather than “last value wins”;
- command-specific usage, unknown-flag rejection, and exact positional arity.

The parser produces typed command requests. It does not read files, discover projects, or perform validation beyond syntax and flag contradictions.

### Project discovery and paths

For every command except `init`:

1. Convert the working directory to an absolute, cleaned path.
2. Walk toward the filesystem root one directory at a time.
3. Stop at the first directory entry named `.wrk` that is a real directory, not a symlink.
4. Treat that directory as authoritative. A missing/unreadable/invalid `config.yaml` is an error; discovery must not continue upward.
5. Retain both the absolute root for I/O and project-relative slash paths for output.

Ticket lookup never constructs a path from unchecked input. Validate an ID against the general existing-ID pattern `[a-z][a-z0-9]{0,15}-[0-9a-f]{8}`, then resolve it through the already loaded snapshot.

### Configuration codec

Decode YAML into `yaml.Node` first, then validate explicitly. Do not decode directly into permissive structs because that can lose duplicate keys and silently accept unknown settings.

- Require exactly one YAML document and recursively reject duplicate mapping keys.
- Reject unknown top-level config settings and unknown keys within `defaults` or field definitions.
- Require `version: 1` as an integer, not a numeric string.
- Validate the prefix and creation defaults exactly as documented.
- Validate field names as nonempty strings. Require supported types and enforce nonempty, distinct string options for enums.
- Treat missing optional `defaults` and `fields` as documented defaults.
- Reject aliases, merge keys, and application-specific YAML tags in version 1. They are outside the documented format and complicate duplicate detection, resource safety, preservation, and JSON conversion.

`init` emits the exact initial configuration documented in `docs/configuration.md` with LF endings. Existing valid config is never rewritten by another command.

### Ticket codec and body boundary

Read a ticket as bytes and split it once into `frontmatter` and `body`:

- Require valid UTF-8 and an opening `---` delimiter on the first line.
- Recognize the first later line consisting only of `---` as the closing delimiter. Accept LF or CRLF around delimiters.
- Define `body` as every byte after the closing delimiter's line ending; if the delimiter ends at EOF, the body is empty.
- Never scan the body for another delimiter.

Decode frontmatter into a YAML node and retain both the node and exact body slice. Recursively detect duplicate keys. Reject unknown top-level fields because the format reserves that namespace for built-ins. Validate configured fields but retain unconfigured entries under `fields`.

Version-1 custom values use the JSON-compatible YAML value domain: string, finite number, boolean, null, sequence, or string-keyed mapping. Preserve scalar tags so strings that resemble booleans or numbers do not change type. Aliases, merge keys, non-string mapping keys, non-finite numbers, timestamps, binary scalars, and custom tags are rejected with a precise diagnostic. This is a proposed clarification of the current documentation and must be confirmed before implementation.

For updates, replace only the `title` and/or `status` scalar nodes, validate the proposed ticket and complete graph, encode the frontmatter, write `---\n`, the encoded mapping, `---\n`, and append the original body slice unchanged. Tests compare body bytes, not strings. Semantic node comparisons prove that unrelated metadata values and types survive even if YAML whitespace changes.

### Repository snapshot and validation

`project.Load` returns an immutable snapshot containing:

- parsed config and its original bytes/fingerprint;
- every ticket candidate's path, original bytes/fingerprint, parsed ticket, and diagnostics;
- an inventory fingerprint covering candidate filenames, so concurrent addition/removal is detectable;
- indexes by filename ID and frontmatter ID.

Only regular files matching the general `<id>.md` pattern are ticket candidates. `config.yaml` and `AGENTS.md` are support files. Other files are ignored unless they collide with a reserved temporary/lock name. Symlinked config and ticket candidates are rejected rather than followed.

Validation runs in layers so `validate` can report more than the first failure:

1. File and UTF-8 checks.
2. YAML structure and duplicate-key checks.
3. Built-in and configured custom-field checks.
4. Identity and uniqueness checks.
5. Reference, duplicate dependency, and self-reference checks.
6. Parent graph cycle detection.
7. Dependency graph cycle detection.

Graph cycles are reported separately with a deterministic representative path beginning at the lexicographically smallest ID in the cycle. A parent may depend on a child because the two graphs are never merged.

All operational commands other than `init` and `validate` require a valid complete snapshot; `validate` loads invalid state specifically to report it and exits nonzero. This policy is intentionally strict and simple: commands never present partial repository state or publish a mutation on top of known corruption. It also means `update` cannot repair an invalid target and `show` cannot bypass an unrelated bad ticket; users repair malformed files directly, consistent with `validate` being diagnostic rather than corrective.

### Derived workflow state

Children are derived by indexing `parent`; no child list is stored. Blockers are dependencies whose status is not `done`, including `canceled`. “Ready” means status `todo` and an empty blocker list. Blockers never change or prohibit status updates.

No command implicitly changes another ticket, closes a parent, starts a dependent ticket, or cascades cancellation.

### Mutation and concurrency contract

The storage layer uses one writer protocol for `new` and `update`:

1. Load and validate a baseline snapshot before preparing output.
2. Open `.wrk/config.yaml` and take an exclusive advisory lock for coordination among `wrk` writers.
3. Reload the complete project and compare the config bytes, ticket inventory, and ticket fingerprints with the baseline. Any difference returns a conflict before a write.
4. Build and validate the proposed snapshot in memory.
5. Write a uniquely named temporary file in `.wrk/`, flush it, and close it. New tickets use the process umask; updates preserve the target's permission bits.
6. For an update, reread and compare the target immediately before an atomic same-directory rename. Flush the `.wrk` directory after publication where the OS supports it.
7. For creation, atomically hard-link the completed temporary inode to the final name. A preexisting destination yields a collision and cannot be replaced. Remove the temporary name after the link succeeds, then flush the directory.
8. Clean up temporary files on handled failures and release the lock. Startup/validation may report stale reserved temporary files left by an interrupted process without treating them as tickets.

Tests receive explicit storage hooks at “after baseline,” “after lock,” “after temp write,” and “before publish” so conflicts and failures are deterministic rather than timing-based.

This protocol guarantees serialization between cooperating `wrk` processes, no-clobber creation, atomic visibility of completed updates, and detection of non-cooperating changes observed at either verification point. It does **not** provide a true compare-and-swap against an editor that replaces or writes the target in the final check-to-rename window; `flock` is advisory and editors do not normally honor it. A stronger guarantee requires a lock-aware editing protocol, daemon, or storage mechanism outside the documented direct-edit model. The README must state this limit rather than claiming that atomic rename alone prevents all lost updates.

### Output contracts

Every JSON result has `schema_version: 1`, `ok`, and `command`. The initial shapes are:

- `init`: `project.path`, `project.config_path`.
- `new`: `ticket.id`, `ticket.path`.
- `list`: `tickets[]` summaries containing `id`, `title`, `status`, `priority`, `path`, and `blockers[]`.
- `show`: `ticket` with all fields and body, plus `children[]` and `blockers[]` summaries.
- `update`: `ticket.id`, `ticket.path`, and `changed[]` in fixed field order.
- `validate`: `valid` and `diagnostics[]`.

Errors use `{schema_version, ok:false, command, error:{code,message,path?,details?}}`. Diagnostics use stable machine codes, severity, project-relative path, optional line/column, and message. Arrays are sorted deterministically. JSON object key order is not a semantic promise, but golden tests keep encoder output stable.

Human output is separately formatted and tested for useful content, not terminal coloring. Version 1 emits no color, progress spinners, or prompts, which keeps redirected output predictable.

## Implementation

### Phase 1: Freeze executable and output contracts

1. Add `go.mod` and establish `cmd/wrk` as the only executable entry point.
2. Record the supported Go version and provisional local module path.
3. Add a command-contract document with syntax, JSON examples, exit codes, path conventions, ordering, and the documented concurrency boundary.
4. Implement typed CLI requests, help, usage errors, and output selection without command behavior.

Exit criteria: bare invocation and every command's `--help` work; parser tests cover flag placement, `--`, duplicate flags, contradictory list flags, and positional arity.

### Phase 2: Implement strict codecs

1. Implement recursive YAML key, alias/tag, and JSON-compatible-value checks.
2. Implement version-1 configuration decode and validation.
3. Implement byte-oriented ticket splitting, node decode, built-in validation, and frontmatter re-encoding.
4. Implement title/status patching that retains the exact body slice and unrelated metadata values.
5. Add table-driven tests for all documented field types and malformed inputs.

Exit criteria: the live config and all three live tickets parse from copied fixtures; malformed YAML and semantic failures produce stable diagnostics; byte-preservation cases pass.

### Phase 3: Implement discovery, snapshots, and graphs

1. Implement nearest-boundary discovery, including nested valid and invalid inner projects.
2. Load candidate files into an immutable snapshot with indexes and fingerprints.
3. Validate identity, filename agreement, duplicates, references, and both graphs.
4. Derive sorted children and blocker lists and ready status.
5. Make `validate` aggregate deterministic diagnostics and make other commands require a clean snapshot.

Exit criteria: discovery and graph tests cover missing references, self-references, duplicate dependencies, separate cycle classes, parent-depends-on-child, and canceled blockers.

### Phase 4: Implement safe storage

1. Implement initialization with exclusive config creation and cleanup limited to artifacts created by the failed invocation.
2. Implement cryptographic ID generation and collision retry.
3. Implement Unix advisory writer locking, full-snapshot revalidation, temporary writes, flushes, update rename, and creation no-replace linking.
4. Add deterministic failure and conflict injection behind internal test interfaces.
5. Verify that every failed mutation leaves all preexisting files byte-identical and publishes no final ticket.

Exit criteria: concurrent cooperating writers serialize; injected non-cooperating changes are rejected at both checkpoints; creation collisions never overwrite; interruption points do not expose partial final files.

### Phase 5: Implement commands and formats

1. Implement `init` and `new`, including stdin body input and creation-time parent.
2. Implement `list`, filtering, ready calculation, and stable ordering.
3. Implement `show` with full body, children, and blockers.
4. Implement combined title/status `update`.
5. Wire human and JSON formatters and exit mapping for every command.

Exit criteria: application tests cover all use cases and output contracts; stdout/stderr separation is exact.

### Phase 6: Verify at subprocess and repository boundaries

1. Add subprocess tests that build a temporary `wrk` binary and exercise init, create, list, show, body edit, combined update, ready filtering, validation, and JSON parsing.
2. Run subprocesses from nested directories and place an invalid inner project inside a valid outer project.
3. Copy the current `.wrk` content into test fixtures; never point mutation tests at the live backlog.
4. Test exact body bytes for LF, CRLF, no terminal newline, Unicode, empty body, and delimiter-like body text.
5. Test semantic preservation of absent optionals and nested custom values with string, number, boolean, null, sequence, and mapping nodes.
6. Run `go test ./...`, `go vet ./...`, and a clean-copy build on macOS; add Linux CI or equivalent Linux verification before completion.

Exit criteria: all automated checks pass without mutating live `.wrk`; a clean directory containing only source/documentation can build and execute help using the documented commands.

### Phase 7: Document and dogfood

1. Update README with prerequisites, `go run`, `go build`, `go install`, and the complete first-workflow examples.
2. Add the command-contract page to `docs/index.md` and update agent guidance to say exactly which frontmatter fields the CLI can mutate now and which remain deferred.
3. On the live repository, run `wrk validate` and `wrk show wrk-682f60c7` before any mutation.
4. Use `wrk update wrk-682f60c7 --status in-progress` once update behavior is verified.
5. Use `wrk new ... --parent wrk-3f8a21b7` to create separate follow-up tickets for relationship updates, priority/label updates, and custom-field updates. Put creation-time default overrides in the appropriate follow-up rather than expanding this sprint.
6. Re-run validation and the complete automated suite.
7. Use the CLI to mark `wrk-682f60c7` done only after every criterion is evidenced. Leave `wrk-3f8a21b7` open.

Exit criteria: documentation commands run as written, live tickets validate, required follow-ups exist under the parent, and the milestone alone is done.

## Files Summary

The exact split may be adjusted to keep packages cohesive, but ownership boundaries should remain as follows.

| Path | Change | Purpose |
| --- | --- | --- |
| `go.mod`, `go.sum` | Add | Go module, supported language version, and pinned YAML/Unix dependencies. |
| `cmd/wrk/main.go` | Add | Thin executable entry point and process exit mapping. |
| `internal/cli/parse.go` | Add | Subcommand/flag parser and typed requests. |
| `internal/cli/help.go` | Add | Top-level and command-specific help. |
| `internal/cli/output.go` | Add | Human/JSON rendering and stdout/stderr policy. |
| `internal/app/app.go` | Add | Command dispatch and shared application result types. |
| `internal/app/init.go`, `new.go`, `list.go`, `show.go`, `update.go`, `validate.go` | Add | Use-case orchestration without presentation concerns. |
| `internal/config/config.go` | Add | Strict version-1 configuration codec and validation. |
| `internal/ticket/ticket.go` | Add | Domain model, effective optional values, and field validation. |
| `internal/ticket/codec.go` | Add | Frontmatter/body split, YAML node preservation, and update encoding. |
| `internal/yamlcheck/yamlcheck.go` | Add | Shared duplicate-key, alias/tag, and value-domain checks. |
| `internal/project/discover.go` | Add | Nearest `.wrk` discovery and path conventions. |
| `internal/project/load.go` | Add | Immutable snapshot loading, indexing, and fingerprints. |
| `internal/graph/graph.go` | Add | Separate cycle checks and derived children/blockers. |
| `internal/diagnostic/diagnostic.go` | Add | Stable diagnostic codes, ordering, and locations. |
| `internal/storage/storage.go` | Add | Initialization, ID generation, temp writes, publish, and conflict checks. |
| `internal/storage/lock_unix.go` | Add | macOS/Linux advisory lock implementation. |
| `internal/testutil/` | Add | Disposable-project and subprocess helpers; no production imports. |
| `internal/**/**/*_test.go` | Add | Unit and integration tests colocated with owned behavior. |
| `testdata/projects/compat/.wrk/` | Add | Read-only copy of current real config/tickets for compatibility tests. |
| `docs/cli.md` | Add | Command, JSON, errors, paths, platform, and concurrency contract. |
| `README.md` | Modify | Fresh-environment prerequisites and runnable workflow examples. |
| `docs/index.md` | Modify | Link CLI docs and replace the bootstrap-only workflow statement. |
| `.wrk/AGENTS.md` | Modify through normal file editing only where it is body/documentation, not ticket metadata | Explain that only title/status metadata updates are currently available and list deferred fields. |
| `.wrk/wrk-682f60c7.md` | Modify via `wrk update` | Dogfood milestone status changes. Body notes may be edited directly for evidence. |
| `.wrk/wrk-3f8a21b7.md` | Body-only update if useful | Record follow-up split without completing the parent. |
| `.wrk/wrk-<new>.md` | Add via `wrk new --parent` | Follow-up tickets under `wrk-3f8a21b7`; never hand-create their frontmatter. |

No implementation step should rewrite `.wrk/config.yaml` or existing tickets merely to canonicalize formatting.

## Definition of Done

- [ ] A documented Go prerequisite and `go run`, `go build`, and `go install` path work from a clean source copy; `wrk` with no arguments displays help.
- [ ] `init`, `new`, `list`, `show`, title/status `update`, and `validate` satisfy the stated command contracts without prompts.
- [ ] `new --body-file -` reads stdin exactly, and `new --parent` creates a valid child without adding parent updates to scope.
- [ ] Every command has deterministic human and JSON output; JSON successes and failures are parseable with uncontaminated stdout.
- [ ] Nearest-project discovery works from nested directories and refuses to fall through an invalid inner `.wrk`.
- [ ] Version-1 config parsing rejects malformed YAML, recursive duplicate keys, unknown settings, unsupported versions, invalid defaults, and invalid custom definitions before writes.
- [ ] Existing IDs validate independently of the current prefix; filename mismatch, duplicate IDs, invalid field types, and unknown top-level ticket fields are diagnosed.
- [ ] Parent and dependency references, self-references, duplicate dependencies, and cycles are validated separately and reported deterministically.
- [ ] Active, all, and ready lists obey status/dependency semantics; `canceled` dependencies remain blockers; contradictory flags fail usefully.
- [ ] `show` exposes the full ticket, sorted children, and sorted unfinished blockers in both output modes.
- [ ] Combined title/status updates preserve exact body bytes and all unrelated metadata/custom values and types.
- [ ] All failed validation, I/O, collision, and injected conflict paths leave preexisting files byte-identical and publish no partial final ticket.
- [ ] Two cooperating `wrk` writers serialize, new-ticket publication cannot replace an existing path, and the non-cooperating-editor limitation is documented accurately.
- [ ] Unit, integration, fixture compatibility, and subprocess workflow tests pass with `go test ./...`; `go vet ./...` passes.
- [ ] Verification runs on macOS and Linux local filesystems, or the missing platform is explicitly removed from the support claim before release.
- [ ] README examples and agent docs distinguish implemented title/status operations from deferred frontmatter operations.
- [ ] The live repository validates without migration; the CLI shows and starts `wrk-682f60c7`, creates follow-up tickets under `wrk-3f8a21b7`, and marks only the milestone done after all other checks pass.
- [ ] No database, network service, interactive UI, metadata migration, parent completion, or deferred metadata mutation enters the sprint.

## Risks

| Risk | Impact | Mitigation or boundary |
| --- | --- | --- |
| The acceptance wording implies perfect stale-write protection against direct editors, which portable local filesystem replacement cannot guarantee. | A body edit in the final verification/rename race can still be lost. | Serialize CLI writers, fingerprint full snapshots, verify twice, inject conflicts in tests, document the remaining race, and require a scope change if a hard guarantee is mandatory. |
| YAML decoding can silently coerce or discard information. | Custom values may change type or duplicate keys may escape notice. | Validate `yaml.Node` trees before typed conversion, reject unsupported YAML features, patch only selected nodes, and compare unrelated nodes semantically in tests. |
| Strict whole-project validity blocks useful reads or targeted repairs. | One malformed ticket can stop `list`, `show`, `new`, or `update`. | Keep the initial behavior consistent and deterministic; provide aggregated `validate` diagnostics and document direct repair. Revisit tolerant reads only as a separately specified feature. |
| Adding `new --parent` expands the literal command table. | Reviewers may view it as scope creep. | Tie it solely to the milestone's required follow-up creation and defer all parent updates. Without it, the dogfooding criterion conflicts with the editing contract. |
| The repository has no commits or remote. | A literal fresh checkout/clone cannot yet be reproduced, and the canonical Go module path is unknown. | Verify a clean source copy now; do not create commits or invent a remote in this sprint. Record the limitation and obtain the canonical module path before public distribution. |
| APFS/ext4 behavior does not imply network-filesystem behavior. | Lock, flush, hard-link, and rename guarantees may differ. | Scope support to macOS/Linux local filesystems and fail safely when no-replace linking is unsupported. |
| JSON becomes an API as soon as agents consume it. | Accidental shape changes break automation. | Version the envelope, document field/ordering semantics, use typed result structs, and keep golden/subprocess tests. |
| Compatibility tests may accidentally mutate live `.wrk`. | User backlog data could be changed during development. | Copy fixtures once, run all mutation tests in temporary directories, and reserve live changes for the final explicit dogfood sequence. |
| All current repository files are untracked. | Cleanup or scaffolding commands could overwrite user work. | Use additive writes and exclusive creation, inspect paths before generation, and avoid repository-wide formatters or cleanup commands. |

## Security

- Treat repository YAML and Markdown as untrusted local input. Parsing must not execute tags, commands, templates, shell expansions, or file references.
- Reject YAML aliases, merge keys, and application-specific tags in version 1 to avoid ambiguous semantics and alias-expansion abuse.
- Reject NUL bytes and invalid UTF-8. Diagnostics should identify locations without echoing entire bodies or potentially sensitive custom values.
- Validate IDs before lookup and resolve tickets from the loaded index, preventing `..`, separators, absolute paths, and option-like IDs from becoming filesystem paths.
- Do not follow symlinked `.wrk` boundaries, config files, ticket files, or mutation targets. Temporary and final files must stay in the discovered `.wrk` directory.
- Use `crypto/rand` for IDs. Randomness is for collision avoidance, not authentication or secrecy.
- Create files subject to the caller's umask; preserve permission bits when replacing a ticket. Do not broaden permissions during an update.
- Use uniquely named same-directory temporary files with exclusive creation. Never use predictable shared `/tmp` paths. Clean only files created by the current invocation.
- Keep configuration credential-free as required. The CLI must not add telemetry, network calls, credential discovery, or environment dumps.
- Avoid shell interpolation in documented examples. Pass titles, IDs, and paths as argument vectors in tests.
- Resource exhaustion remains possible with extremely large local repositories because the initial design loads a complete snapshot. Do not invent undocumented ticket-size limits during implementation; measure fixture behavior and add explicit, documented limits only with product approval.

## Dependencies

### Technical

- Go 1.25 is the proposed minimum; only language/library features available at that version may be used.
- [`go.yaml.in/yaml/v4`](https://github.com/yaml/go-yaml), pinned in `go.mod`/`go.sum`, provides YAML node parsing and encoding. The maintained project recommends v4 for new projects; strictness remains application-owned.
- `golang.org/x/sys/unix`, pinned in `go.mod`/`go.sum`, provides advisory locking and Unix-specific filesystem operations behind a build-tagged package.
- Standard-library `crypto/rand`, `encoding/json`, `io/fs`, and filesystem primitives provide the rest of the implementation. No CLI framework, database, or service dependency is needed.
- APFS on macOS and a common local Linux filesystem with same-directory atomic rename, hard links, advisory locks, and directory flush support are the initial storage environments.

### Project and sequencing

- `wrk-9d42c6a1` is already `done` and its ticket/config documentation is normative.
- `wrk-682f60c7` remains `todo` until the update command is proven, then transitions through the CLI.
- `wrk-3f8a21b7` remains the open parent and owns deferred metadata operations.
- The current `.wrk` files are compatibility inputs. No migration is a prerequisite.
- Linux verification requires an available CI runner or a documented equivalent environment; none exists in current repository evidence.
- A canonical Git remote/module path and actual fresh-clone test are external prerequisites for public distribution, not for a local runnable milestone.

## Open Questions

1. **Concurrency acceptance — blocking for the final claim:** Does “concurrent edits are not silently overwritten” accept the documented advisory-lock plus double-fingerprint boundary? A hard guarantee against a non-cooperating editor in the final race requires a different editing/storage protocol and is not implementable with atomic rename alone.
2. **Go identity — resolve before scaffolding:** Is Go 1.25 the intended minimum, and should the temporary module path be `wrk`, or is there a canonical remote import path not present in repository evidence?
3. **Creation-time parent — recommended answer: include it:** Confirm `new --parent` as the smallest resolution of the requirement to create follow-up tickets under `wrk-3f8a21b7`. Rejecting it requires either deferring that acceptance criterion or authorizing another bootstrap metadata edit.
4. **Invalid unrelated tickets — recommended answer: strict snapshot:** Should a bad unrelated ticket block `list`, `show`, `new`, and `update` as proposed? A tolerant mode needs command-specific partial-validity rules and risks inconsistent output, so it should not be inferred silently.
5. **YAML value subset — recommended answer: JSON-compatible values only:** Confirm rejection of aliases, merge keys, custom tags, timestamps, binary scalars, non-finite numbers, and non-string nested mapping keys. The current docs require type preservation but do not define these YAML features or their JSON representation.
6. **Initialization of an existing empty `.wrk` — recommended answer: allow config creation:** Confirm that `init` may add `config.yaml` when `.wrk/` exists with support files but no config or tickets, while refusing any directory that already contains config or ticket data.
7. **JSON and exits — recommended answer: adopt the proposed version-1 envelope and `0/1/2/3` exit classes:** The ticket requires JSON but does not define success/error schemas, relative paths, or exit categories.
8. **Fresh-checkout evidence — blocking for literal wording:** With no commit or remote, should a clean copied source tree count for this milestone, or will a maintainer establish Git history so a real clone/checkout can be tested? The implementation sprint should not silently create repository history.
9. **Linux completion gate:** What runner or environment will provide Linux filesystem verification? If none is available, support must be documented as macOS-only until that evidence exists.
