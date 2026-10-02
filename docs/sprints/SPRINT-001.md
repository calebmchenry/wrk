# Sprint 001: First Runnable wrk CLI

Status: completed. All six phases and the Definition of Done passed, including actual macOS/Linux verification and live CLI dogfooding. See [execution evidence](SPRINT-001-EXECUTION.md).

Source ticket: [wrk-682f60c7](../../.wrk/wrk-682f60c7.md). Parent: [wrk-3f8a21b7](../../.wrk/wrk-3f8a21b7.md).

## Overview

Deliver a locally runnable Go CLI so we can use wrk to build wrk: initialize a project, create a ticket, find ready work, inspect it, start it, edit its Markdown body directly, and finish it. Existing configuration and tickets must work without migration. The [ticket format](../ticket-format.md) and [project configuration](../configuration.md) remain the data contract.

The repository currently has documentation and three tickets, with no implementation, tests, commits, or remote. This sprint establishes the executable, strict validation, safe file publication, readable and JSON output, and meaningful automated verification. The broader parent remains open for metadata updates beyond title and status.

### Confirmed decisions

| Decision | Agreed scope |
| --- | --- |
| Language and platforms | Go; verify on macOS and Linux local filesystems. |
| Concurrent changes | Serialize CLI writers and reject detected stale snapshots. Direct edits happen outside CLI updates; the final comparison-to-replacement race with a non-cooperating editor is explicitly accepted and documented. |
| Creation flags | Include `--parent`, `--priority`, repeatable `--label`, and `--no-labels`, in addition to title/body input. |
| Invalid project behavior | Normal data commands fail if any ticket is invalid. `validate` aggregates determinable diagnostics. No partial list/show success or repair exception. |
| Verification | Automated parser/storage/CLI tests on both platforms, clean-source build/install smoke test, and live dogfooding. A clean source copy is acceptable while Git history is absent. |

Defer general parent/dependency/priority/label/custom-field updates, creation-time dependency/custom-field flags, interactive editing, automatic workflow transitions, assignments, integrations, databases, Windows, and public distribution. Creation-time overrides do not bring those update operations into scope.

## Use Cases

| Command | Observable behavior |
| --- | --- |
| `wrk`, `wrk --help`, `wrk help` | Print help and exit zero without discovering a project. |
| `wrk init [directory]` | Initialize the existing target directory, defaulting to cwd. Refuse any existing `.wrk` entry; never overwrite or adopt existing data. |
| `wrk new "Title"` | Create a `todo` ticket with configured prefix, priority, and labels; print ID and path. |
| `wrk new "Title" --body-file <path>` | Use exact UTF-8 body bytes from the file; `-` reads stdin through EOF. |
| `wrk new "Title" --parent <id>` | Create a child of an existing ticket in the discovered project. |
| `wrk new "Title" --priority high --label cli` | Explicit priority and labels replace configured defaults. Repeated `--label` supplies the whole label list. |
| `wrk new "Title" --no-labels` | Explicitly use an empty label list. Conflicts with `--label`. |
| `wrk list` | Return `todo` and `in-progress` tickets, including blocked tickets. |
| `wrk list --all` | Include all four statuses. |
| `wrk list --ready` | Return only `todo` tickets whose dependencies are all `done`. |
| `wrk show <id>` | Show full ticket source, derived children, and unfinished dependency blockers. |
| `wrk update <id> --title "New title" --status in-progress` | Apply either or both flags in one validated update; accept all four statuses. |
| `wrk validate` | Check configuration, tickets, identity, and relationships without writing anything. |

Every command supports `--json`. Flags may follow positional arguments, as in the source ticket, or precede them; `--json` also works before the subcommand. Support `--flag=value` and `--` to end option parsing. Reject unknown flags, repeated scalar flags, surplus arguments, empty updates, and `list --all --ready` with useful nonzero errors. Repeated `--label` is intentional.

Titles are nonblank single-line strings; preserve their supplied text rather than trimming it. Missing optional fields retain their documented meaning without being inserted into existing files. New tickets always start at `todo`; changing configuration never changes existing ticket values. A no-op update succeeds with `changed: false` and does not rewrite the ticket.

## Architecture

### Components and packaging

Use `module wrk` for local development until a canonical remote exists. Set the initial language baseline to Go 1.25, verify dependencies against it, and record the supported development toolchain during execution. Provide these local paths:

```sh
go run ./cmd/wrk --help
go build -o ./bin/wrk ./cmd/wrk
go install ./cmd/wrk
```

Document where Go installs executables and the required PATH adjustment. No published module, package-manager formula, remote, or Git commit is required. Go's [build/install documentation](https://go.dev/doc/tutorial/compile-install) describes these two executable workflows.

| Area | Ownership |
| --- | --- |
| `cmd/wrk/main.go` | Wire arguments/streams to the CLI and exit with its returned code. |
| `internal/cli/` | Typed requests, command orchestration, help, human/JSON rendering, diagnostic-to-exit mapping. |
| `internal/project/` | Boundary discovery, config, immutable snapshots, identity indexes, repository validation, separate graphs and derived state. |
| `internal/ticket/` | Byte-oriented ticket parsing, YAML node/schema checks, candidate title/status patching, semantic preservation comparison. |
| `internal/store/` | Writer lock, complete staging, snapshot comparison, no-clobber creation, atomic replacement, initialization and cleanup. |

Pure validators return diagnostics and never print or write. Store operations receive validated candidate bytes and the original snapshot. Keep fault hooks narrow and internal to tests; no generic storage backend or plugin architecture.

Use the maintained YAML organization's [`go.yaml.in/yaml/v3`](https://pkg.go.dev/go.yaml.in/yaml/v3) node API, pinned to a tested release in `go.mod`/`go.sum`. Use `golang.org/x/sys/unix` for isolated macOS/Linux locking. Put an explicit `//go:build darwin || linux` constraint on platform-specific files; `_unix.go` alone is not a Go platform constraint. The standard library covers JSON, randomness, filesystem access, and tests. A small table-driven argument parser is sufficient, but its interspersed-flag behavior must be tested; a default parser that stops at the first positional argument does not satisfy the command contract.

### Discovery and strict snapshots

Walk upward from cwd and stop at the nearest entry named `.wrk`. It is an authoritative boundary even if it is a file, forbidden symlink, or directory with missing/invalid config; report the error and never fall through. `init` addresses its explicit target directly and can create a nested project. It requires an existing target directory.

Load config for every data command. Require one YAML document, integer version `1`, valid prefix, valid defaults, and valid optional custom definitions. Reject duplicate keys recursively, unknown settings at every defined schema level, unsupported versions, invalid types, and invalid enum options. Missing optional config entries use documented defaults; explicit null does not stand in for a typed value.

Scan only direct ticket candidates matching `^[a-z][a-z0-9]{0,15}-[0-9a-f]{8}\.md$`. Existing IDs use this general grammar independently of the current prefix. Ignore supporting files and unrelated names; matching entries that are symlinks or nonregular files produce diagnostics. Index embedded IDs before reducing to a map so duplicate IDs are not hidden by overwrite. Detect filename/ID mismatch and duplicate embedded IDs independently.

Each snapshot contains config bytes, candidate filenames, every ticket's bytes and file identity, parsed nodes, indexes, and diagnostics. Normal commands require a fully valid snapshot. `validate` reports all independent errors it can determine; if config/schema or parsing errors prevent later checks, say which checks were unavailable instead of inventing cascading errors. Sort diagnostics by path, location/field, and code.

An invalid project cannot be repaired through this sprint's `update` command. Report precise locations and recommend restoring a known-good file or obtaining an explicitly authorized repair; do not silently introduce a manual-frontmatter exception to the repository rules.

### Ticket bytes, YAML values, and preservation

Require UTF-8 and an opening `---` line at the beginning. The first subsequent delimiter line closes frontmatter. Accept LF and CRLF; keep every byte after that closing line's line ending as the body. A closing delimiter at EOF means an empty body. Never trim body whitespace, add a terminal newline, or interpret later body delimiters.

Retain original file bytes, exact body bytes, and YAML nodes separately. Validate required/optional built-in keys, strict types, statuses, priority, relationship IDs, labels, and configured custom values. Reject unknown top-level ticket fields; custom data belongs under `fields`. Enforce configured types without coercion and preserve omitted fields. Duplicate labels are not forbidden by the current schema; do not invent a new restriction.

Configured strings/enums require resolved string nodes; booleans require boolean nodes (`true`/`false`), and numbers require integer/float nodes. Quoted numeric/boolean lookalikes do not satisfy numeric/boolean definitions. Resolve aliases safely before type checks and retain numeric precision rather than converting arbitrary integers through `float64`. The YAML library's compatibility coercions do not override the project's strict-type contract.

Unknown custom values remain YAML nodes, including nested collections and scalar tags. Do not round-trip them through JSON or restrict all custom values to JSON primitives. For title/status updates, clone the tree, patch only requested values, serialize frontmatter, append the untouched body, reparse, and verify that all unrelated semantic values remain equal. Frontmatter spacing/comments are not byte-level guarantees.

Prove alias behavior early: changing an anchored title/status must not change an unrelated custom value that aliases the old value. Preserve its previous meaning, for example by materializing the affected old alias value. Avoid unbounded alias expansion. If an opaque YAML structure cannot be preserved, fail the mutation unchanged with `PRESERVATION_UNSUPPORTED`; do not silently coerce it or redefine otherwise valid files as invalid. Reads and validation can still operate on an otherwise valid source. Any general format restriction requires a separate decision, not an incidental parser shortcut.

### Relationships and workflow

Build and validate parent and dependency graphs separately. Detect missing references, self-references, duplicate dependencies, and cycles with useful involved IDs/paths. A parent depending on its child is valid when neither separate graph cycles. Derive children from child `parent` fields; never store reciprocal lists or impose a fixed hierarchy depth.

Only `done` satisfies a dependency. `canceled`, `todo`, and `in-progress` prerequisites remain blockers. Readiness means `todo` with no blockers. Explicit updates are allowed on blocked tickets, reopening is allowed, and no operation automatically transitions parents, children, or dependents.

### Output and errors

Human lists show ID, status, priority, title, and blocker IDs. Sort tickets, children, and blocker summaries lexicographically by ID. `show` prints source and then clearly separated derived information. Escape terminal control sequences in human output; JSON uses standard escaping and retains source content.

Use one JSON envelope for successes and ordinary failures:

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

`project_root` is absolute, or null before discovery. Other paths are slash-separated relative to it and do not change with invocation subdirectory. Arrays have deterministic ordering; there are no incidental timestamps. Errors have stable `code`, `message`, and applicable `path`, `field`, location, and involved IDs.

| Command | Successful `result` |
| --- | --- |
| `init` | `created`, `path: ".wrk"`, `config_path: ".wrk/config.yaml"` |
| `new` | Common `ticket` summary, `created: true` |
| `list` | `tickets` array of common summaries |
| `show` | Common `ticket` summary, exact full UTF-8 `source`, derived `children` summaries |
| `update` | Common `ticket` summary, `changed` boolean |
| `validate` | `valid: true`, `ticket_count` |
| Help requested with JSON | `usage` string |

The full `source` in `show` includes all custom values and body without a lossy YAML-to-JSON projection. Built-in defaults in summaries are effective values, not instructions to materialize absent keys. Blockers contain dependency IDs and statuses; all arrays are empty arrays rather than null when empty.

Exit `0` means success/help, `2` means usage error, and `1` means an operational or data error. Machine error codes distinguish invalid config/ticket/graphs, not-found, busy/conflict, I/O, and preservation failures. JSON mode emits exactly one object plus newline on stdout, including ordinary failures, with no duplicated routine stderr message. Human errors go to stderr. No prompts or progress output occur.

On pre-publication failure, `ok` is false, `result` is null, and no existing ticket/config data changed. If publication succeeded before a later cleanup/sync failure, return an explicit committed result with affected ID/path and `publication: "committed"`, plus `ok: false` and a diagnostic. An undeliverable stdout or killed process cannot guarantee an error envelope; never roll back committed data merely because output failed.

### Storage and supported concurrency

Atomic replacement prevents torn publication; it is not compare-and-swap. Go's [`os.Rename`](https://pkg.go.dev/os#Rename) replaces an existing destination and has platform-specific atomicity limits. The agreed guarantee is serialization among cooperating CLI writers plus detection of external changes visible at the final comparison, on tested macOS/Linux local filesystems.

1. Read body-file/stdin input before acquiring the mutation lock. Race-safely open/create the persistent `.wrk/.lock` using non-truncating create semantics, no symlink following, and a regular-file check. Concurrent first users must open the same inode. Acquire an advisory exclusive lock nonblocking; contention returns `BUSY`. Never unlink the lock file on release and never use editable `config.yaml` as the lock inode. A leftover unlocked file is harmless; process exit releases the lock.
2. Load the authoritative full snapshot after locking. Validate it, build the proposed change, and validate the complete candidate project before publishing anything.
3. Stage complete candidate bytes in a unique same-directory temporary file with a non-ticket name. Check write/sync/close errors; preserve target permission bits for updates, and apply the caller's umask for new files. Never widen existing permissions.
4. Immediately before publication, compare config bytes, ticket inventory, ticket identities, and ticket bytes to the snapshot. Detect same-size edits, preserved timestamps, deletion, replacement, and unrelated validation-input changes. On mismatch, return `CONFLICT` without retrying or overwriting the new state.
5. For `new`, generate four cryptographically random bytes as eight lowercase hex characters. Check IDs and paths, and publish the complete staged inode with an atomic no-replace operation such as a same-filesystem hard link. Retry destination collisions with a new ID, up to 128 attempts. Revalidate/recheck before each publication attempt. Never write partial contents directly to the final ticket path or use overwriting rename for creation.
6. For `update`, atomically rename the staged file over the target after the successful comparison. This is the publication point. Flush the directory where supported; clean up only operation-owned staging names and release the lock.

The final comparison and replacement are separate operations. An editor that saves between them can still lose its change, and any other validation input can also change after its comparison. Direct body/config edits and Git operations must therefore occur outside a CLI mutation. Sequential direct body editing remains supported. Document this accepted boundary in `docs/storage.md`, README/agent guidance, and the editing-contract clarification; do not claim protection from every arbitrary editor race.

Read commands, including `validate`, create no locks, caches, or files. Mutations may leave the persistent lock file and, after interruption, ignored staging files. These are permitted runtime artifacts, not ticket data; no command treats their presence as an invalid ticket or automatically deletes another operation's files. Add exact ignore patterns in this repo and document them for other initialized projects. Validation/conflict/pre-publication I/O failures leave all preexisting ticket/config bytes unchanged. A successful link/rename followed by another error is a committed operation, not a rejection; distinguish cleanup errors from durability uncertainty, report affected paths, and avoid blind rollback or automatic retry.

For `init`, fail on any existing `.wrk` entry. Create the directory exclusively, stage the default config, and publish it without replacement. On handled failure, remove only artifacts created by this invocation and remove its directory only if still empty. Never recursively delete a directory that another process may have populated. Interrupted initialization may leave an incomplete directory; subsequent commands fail clearly, and `init` never silently adopts it. Unsupported filesystem primitives fail safely rather than falling back to weaker publication.

## Implementation

### Phase 1: Executable, contracts, and feasibility (~10%)

**Files:** `go.mod`, `go.sum`, `cmd/wrk/main.go`, `internal/cli/{args,run,output,errors}.go`, initial ticket/store tests, `docs/cli.md`, `docs/storage.md`, `.gitignore`.

- [x] Establish the local module, Go baseline, pinned compatible dependencies, binary entry point, and narrow package boundaries.
- [x] Implement help, typed command requests, flag placement/arity/conflict rules, output envelopes, and exit mapping.
- [x] Record the confirmed scope and output/storage contracts before implementing command behavior.
- [x] Prove YAML node preservation (including aliases of mutable fields), complete no-replace creation, persistent locking, and replacement/failure behavior with focused test seams. If node re-emission fails representative preservation fixtures, compare a narrow scalar-span patch before expanding scope; do not silently narrow the format.
- [x] Ignore `bin/` and exact runtime lock/staging patterns; do not ignore ticket Markdown.

**Exit gate:** help builds and works without a project; parser/output checks pass; the key YAML and storage primitives have an implementable tested design. Resolve failed feasibility cases before building mutations on top of them.

### Phase 2: Project validation and read workflow (~25%)

**Files:** `internal/project/{discover,config,snapshot,validate,graph}.go`, `internal/ticket/{parse,validate}.go`, read-command orchestration, colocated tests, `testdata/compat/.wrk/`.

- [x] Copy the original three tickets and config into immutable compatibility fixtures before any live mutation.
- [x] Implement nearest-boundary discovery, strict config and field-definition validation, and byte-oriented ticket parsing.
- [x] Preserve nodes/body bytes; validate identities, duplicates, all documented metadata, and configured custom fields.
- [x] Implement separate graph validation, children, blockers, and readiness; aggregate deterministic diagnostics.
- [x] Implement `validate`, `list`/`--all`/`--ready`, and `show` in both formats; enforce the strict-project policy.

**Exit gate:** original fixtures work without migration; negative fixtures produce useful diagnostics; read commands leave the whole fixture tree unchanged; invalid inner projects never fall through.

### Phase 3: Safe title/status updates and first dogfooding (~25%)

**Files:** `internal/store/{lock_unix,write}.go`, `internal/ticket/update.go`, update-command orchestration and preservation/conflict tests.

- [x] Implement the persistent lock, authoritative snapshot, staging, full-input comparison, atomic replacement, permission preservation, and publication-aware errors.
- [x] Patch either/both title and status in one candidate; validate and reparse before publishing, comparing unrelated semantic values and exact body bytes.
- [x] Implement unchanged no-op results without rewriting, and useful busy/conflict/preservation errors.
- [x] Add deterministic hooks to change files or inject failures at defined write checkpoints. Verify no cascades and permitted explicit updates on blocked tickets.
- [x] After safety tests pass, run the CLI to validate and show live `wrk-682f60c7`, then mark it `in-progress`. Record evidence in its body through direct body editing only.

**Exit gate:** valid title/status updates preserve data, pre-publication failures reject cleanly, contention/stale snapshots are tested, and the first live metadata transition is performed through the CLI.

### Phase 4: Initialization and creation (~15%)

**Files:** `internal/store/{init,create}.go`, init/new orchestration, ID/default/body/parent tests.

- [x] Implement non-overwriting `init [directory]` and exact default configuration.
- [x] Implement `new`, cryptographic IDs, bounded collision handling, complete staged no-replace publication, and ID/path results.
- [x] Read exact UTF-8 body-file/stdin bytes before locking and preserve them in the new ticket.
- [x] Implement `--parent`, `--priority`, repeated `--label`, and `--no-labels`; distinguish omission from explicit empty labels and validate the resulting ticket/project.
- [x] Verify that configured defaults affect only new tickets and that old-prefix IDs remain valid after a prefix change.

**Exit gate:** disposable projects can be initialized and populated through the command contract; collision/failure tests never overwrite existing data or expose partial final tickets; parented follow-ups are possible without general relationship updates.

### Phase 5: End-to-end verification and documentation (~20%)

**Files:** `test/integration/cli_test.go`, targeted fixtures/tests, `README.md`, `docs/{cli,storage,index,ticket-format,configuration}.md`, `.wrk/AGENTS.md`; optionally `.github/workflows/test.yml`.

- [x] Exercise the full compiled-CLI loop in disposable projects: init, new, list/ready, show, start, direct body edit, combined update, finish, list/all, validate.
- [x] Cover each command's JSON success and representative errors, nested discovery, configured defaults/custom schemas, and the failure matrix below.
- [x] Run formatting, tests, race checks, vet, and build on macOS and Linux local filesystems. Record actual commands/results; cross-compilation or an unexecuted CI file is not Linux test evidence.
- [x] Verify build and install from a clean source copy with no prebuilt binary and a temporary install destination. Confirm bare help and a disposable ticket workflow. Module downloads at build time are allowed; normal CLI use is offline.
- [x] Add tested README commands and PATH guidance; document JSON/error/path/sort contracts, init behavior, strict validation, preservation, interruption recovery, and the concurrency boundary.
- [x] Update agent docs to enumerate supported creation/title/status operations and deferred metadata updates; remove the blanket pre-CLI limitation only once true. Link new docs from `docs/index.md`.

**Exit gate:** both-platform test evidence and clean-source build/install evidence exist, all examples execute as documented, and docs accurately describe the delivered guarantees.

### Phase 6: Follow-ups and milestone completion (~5%)

**Files:** live `.wrk/wrk-682f60c7.md` and CLI-created follow-up tickets; sprint execution notes/ledger when executed.

- [x] Check for existing equivalent follow-ups, then create tickets through `new --parent wrk-3f8a21b7 --body-file ...` for relationship updates, priority/label updates, and custom-field operations. Carry creation-time dependency/custom-field options into those follow-ups if needed.
- [x] Record acceptance evidence and check off the milestone's completed body criteria. Keep the immutable compatibility fixture unchanged.
- [x] Review every completion gate, validate the live project, mark `wrk-682f60c7` `done` through the CLI, and validate/show again.
- [x] Leave `wrk-3f8a21b7` open. Do not infer parent completion from its child's status or create Git history as a hidden delivery step.

**Exit gate:** required follow-ups exist under the parent, the child alone has completed through the CLI, and the live backlog validates.

### Verification matrix

| Boundary | Required evidence |
| --- | --- |
| CLI syntax | Flags before/after positionals, `--flag=value`, `--`, duplicate scalar flags, empty updates, contradictory flags, bare/command help. |
| Discovery/config | Nested projects; invalid/missing/non-directory/symlink inner boundary; duplicate keys; unknown settings; wrong versions/types; invalid prefix/defaults/enums. |
| Compatibility | Original config and all three tickets; absent optionals; richer synthetic metadata/custom values; no migration or read-time rewrite. |
| Body and YAML | LF/CRLF, Unicode, empty body, no terminal newline, later delimiters, file/stdin bodies, nested/custom scalar types and precision, aliases affected by title/status edits, unchanged unrelated semantics. |
| Identity/creation | Old prefix, filename mismatch, duplicate embedded IDs, forced RNG/path collisions, collision immediately before publication, bounded retries, no partial final files. |
| Graphs/workflow | Missing/self/duplicate edges, each cycle class, valid parent-depends-on-child, deep hierarchy, canceled blockers, all statuses/reopening, no cascades. |
| Mutations | Exact target and unrelated-file comparisons on validation/conflict/failure; preserved permissions; no-op bytes unchanged; explicit defaults override and empty labels. |
| Concurrency | Two real processes contend on the stable lock inode, including simultaneous first creation and config replacement while locked; repeated acquisitions retain its inode; lock releases on exit; target/config/other-ticket/inventory changes trigger conflict; same-size/timestamp-preserving and editor-rename changes are detected before publication. |
| Publication failures | Inject write/sync/close/link/rename failures; interrupt before/after publication; distinguish rejected from committed-with-error; ignore residual staging files and never blindly roll back. |
| JSON | Every command, success and failure, one stdout object, version/paths/IDs, stable arrays, full source round-trip, no hidden diagnostics on stdout. |
| Delivery | Actual macOS and Linux checks, clean-source build/install, working README examples, live show/start/follow-up/done sequence. |

Deterministic hooks replace timing-sensitive race tests. An edit before the final comparison must be rejected; an edit after it characterizes the documented unsupported editor race, not a promised guarantee. Tests use disposable directories and never mutate the live backlog.

Expected verification commands (after implementation):

```sh
gofmt -l cmd internal test
go test ./...
go test -race ./...
go vet ./...
go build -o ./bin/wrk ./cmd/wrk
```

`gofmt -l` must report no files. The clean-copy install smoke uses a task-specific temporary `GOBIN`, runs `go install ./cmd/wrk`, and invokes that installed binary from outside the source tree against a disposable project. Record toolchain and OS with the evidence; do not label planned tests as executed.

## Files Summary

| Path | Action | Purpose |
| --- | --- | --- |
| `go.mod`, `go.sum`, `cmd/wrk/main.go` | Create | Local Go module, pinned dependencies, executable. |
| `internal/cli/*.go` | Create | Requests, six commands, help, human/JSON results and exits. |
| `internal/project/*.go` | Create | Discovery/config/snapshots/identity/graphs. |
| `internal/ticket/*.go` | Create | Strict nodes/schema, byte split, preservation and patches. |
| `internal/store/*.go` | Create | Stable locking, staging, initialization and publication. |
| Colocated `*_test.go`, `test/integration/cli_test.go` | Create | Meaningful domain, filesystem, and subprocess verification. |
| `testdata/compat/.wrk/`, targeted fixture directories | Create | Immutable original data and cases missing from it. |
| `.gitignore`; optional `.github/workflows/test.yml` | Create | Exclude binary/runtime artifacts; run macOS/Linux checks if hosted CI is available. |
| `docs/cli.md`, `docs/storage.md` | Create | Exact interface, JSON, failure and storage contracts. |
| `README.md`, `docs/index.md`, `.wrk/AGENTS.md` | Modify | Verified local usage and accurate agent workflow. |
| `docs/ticket-format.md`, `docs/configuration.md` | Clarify | Accepted concurrency/preservation/default behavior without a migration or new format version. |
| `.wrk/wrk-682f60c7.md`, new follow-up files | CLI mutations/body notes during execution | Dogfood only after safety checks; preserve parent scope. |

Existing files are untracked user work. Do not replace scaffolding over them, run destructive cleanup, normalize all tickets, or commit/publish without a separate instruction.

## Definition of Done

- [x] A clean source copy builds and installs through the documented Go workflow; bare `wrk` displays help.
- [x] All seed commands and the four confirmed creation flags work noninteractively, including stdin bodies, combined updates, and contradictory-flag errors.
- [x] Every command has readable and JSON results, mutation IDs/paths, deterministic ordering, and useful nonzero failures.
- [x] Nearest-boundary discovery and strict config/custom-definition checks work without falling through invalid inner projects.
- [x] Existing tickets/config need no migration; prefix changes affect creation only; optional/deferred metadata is understood and preserved.
- [x] ID generation/collision handling never clobbers another ticket or publishes partial contents.
- [x] Updates preserve exact body bytes and unrelated semantic metadata/custom values; no-op updates do not rewrite.
- [x] Validation covers malformed config/metadata, duplicate keys/IDs, filename mismatch, references, self-edges, duplicate dependencies, and separate graph cycles without writes.
- [x] Invalid projects fail normal commands; diagnostics are actionable and do not introduce unauthorized metadata repair.
- [x] CLI writer serialization and stale-input checks match the accepted, documented external-editor boundary.
- [x] Pre-publication failures preserve existing ticket/config data; post-publication failures report committed state and never cause blind rollback.
- [x] Blockers/readiness, canceled prerequisites, explicit status changes/reopening, and no-cascade behavior match the schema.
- [x] Parser, storage, CLI, compatibility, and failure tests pass on macOS and Linux; build/install/race/vet/format evidence is recorded.
- [x] README and indexed agent docs contain verified examples and correctly distinguish supported operations from deferred work.
- [x] The live CLI shows/starts the child, creates parented follow-ups, and finishes the child only after all gates pass; final validation passes and the parent stays open.


## Risks & Mitigations

| Risk | Likelihood / impact | Mitigation |
| --- | --- | --- |
| YAML mutation changes an unrelated value | Medium / High | Keep nodes and exact body bytes; test scalar tags/aliases; reparse and compare; fail unchanged when preservation cannot be established. |
| Lock or rename is mistaken for universal editor safety | Medium / High | Stable lock inode, full-input final comparison, deterministic tests, and the explicitly accepted no-simultaneous-editor condition. |
| Failure after publication prompts a duplicate retry | Medium / High | Distinguish committed results, retain ID/path evidence, and advise inspection after interruption. |
| Strict validity blocks useful inspection/repair | Medium / Medium | Aggregate diagnostics; source files remain readable directly; restoration/authorized repair is explicit follow-up scope. |
| Platform/filesystem assumptions are unverified | Medium / High | Run on both supported OSes; fail safely on unavailable primitives; do not infer Linux semantics from macOS or a cross-build. |
| Initial milestone grows into the parent scope | Medium / Medium | Limit updates to title/status; creation additions are enumerated; capture deferred commands in parented tickets. |
| Live dogfooding damages the untracked baseline | Low / High | Preserve original fixtures; prove mutations on copies first; use the CLI only for scoped live changes. |

## Security Considerations

Treat YAML, Markdown, and arguments as data; never execute tags, templates, commands, or body contents. Validate IDs before path construction, resolve ticket references through the snapshot, and reject symlinked project boundaries/config/ticket candidates. Keep temporary and final writes inside the discovered directory, with exclusive staging and operation-owned cleanup. Escape terminal control sequences in human output and avoid dumping unrelated values in error messages.

No runtime network service, credentials, telemetry, database, or Git subprocess is needed. This is a locally owned project model, not protection against a hostile process replacing ancestor directories. Handle alias recursion without unbounded expansion and make any resource bounds explicit; never silently discard custom data.

## Dependencies

- Existing version-1 format/config documentation and original tickets; the format prerequisite is already `done`.
- Go toolchain and pinned YAML/Unix packages. Dependency download is a build-time need; operation is local/offline.
- Actual macOS and Linux execution environments with tested local filesystem semantics. Choose the Linux runner during execution; no runner or remote currently exists in the repo.
- Accepted scope decisions above. No migration, service, remote/module publication, or Git history is a prerequisite.

## Open Questions

No unresolved product questions block planning. Phase 1 must select compatible dependency versions and verify the YAML preservation cases; Phase 5 must obtain and record actual Linux execution. These are execution gates, not permission to lower the agreed support or preservation criteria.

The user authorized implementation with the sprint-execute request. Planning provenance and accepted/rejected review findings are recorded in [merge notes](drafts/SPRINT-001-MERGE-NOTES.md).
