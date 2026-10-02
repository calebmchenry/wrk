# Sprint 001 execution

The user's request to execute Sprint 001 authorizes implementation. The user selected Strategy A (orchestrated). No commits or publication are in scope.

Preconditions verified: the version-1 format prerequisite is done; Go 1.25.4 darwin/arm64 is installed; Docker provides an actual Linux aarch64 execution environment. Commands are `go test ./...`, `go test -race ./...`, `go vet ./...`, `gofmt -l cmd internal test`, and `go build -o bin/wrk ./cmd/wrk`. There is no Git history or commit convention to apply.

The September 28 weather report selects gpt-6-astra for CS/math implementation work. No delegated model run is needed for Strategy A. Dependencies are pinned to go.yaml.in/yaml/v3 v3.0.5 (Go 1.16 minimum) and golang.org/x/sys v0.37.0 (Go 1.24 minimum), compatible with the Go 1.25 baseline.

## Working checklist

- [x] Phase 1: executable, argument/output contracts, YAML and storage feasibility.
- [x] Phase 2: strict project validation and read commands.
- [x] Phase 3: safe updates and first live dogfooding.
- [x] Phase 4: initialization and creation.
- [x] Phase 5: end-to-end, both-platform and clean-source verification, documentation.
- [x] Phase 6: follow-ups and milestone completion.

Original configuration and all three ticket files were copied byte-for-byte to `testdata/compat/.wrk/` before any live mutation. Keep these fixtures unchanged.

Phase 1 gate: `go test ./...`, `go vet ./...`, build, and JSON help passed on Go 1.25.4 macOS arm64. YAML fixtures prove preservation of anchored mutable scalars, arbitrary-precision explicit integers, opaque tags, recursive aliases, and CRLF/Unicode body bytes. Storage fixtures cover stable lock identity, complete no-replace publication, and injected pre/post-publication failures. Small shared `internal/schema` and `internal/diagnostic` packages avoid coupling config validation to ticket internals.

Phase 2 gate: tests, vet, and build passed on macOS; live read-only validation returned three valid tickets. Fixtures test malformed config/tickets/custom schemas, both cycle classes, deep hierarchy, valid parent-depends-on-child, old-prefix IDs, read-only tree equality, and invalid inner boundaries.

Phase 3 gate: tests, vet, build passed. Two actual child processes raced first lock-file creation: one held the lock and one received BUSY; replacing config did not bypass it; killing the holder released it without changing the inode. Mutation tests reject detected target/config/inventory/inode changes, retain body and permission bits, and distinguish committed directory-sync failure. The built CLI validated/showed live wrk-682f60c7 and marked it in-progress; evidence was appended to its body separately.

Phase 4 gate: tests, vet, and build passed on macOS. Tests cover exact initial config, existing file/directory/symlink refusal, foreign-file-safe init cleanup, defaults/overrides, old-prefix reads, exact Unicode/CRLF body bytes, duplicate labels, invalid creation, 128-attempt exhaustion, and a competing creation immediately before publication.

## Phase 5 verification evidence

All commands below actually passed on 2026-10-02. Both environments used Go 1.25.4 and the same pinned dependency versions. There is no required Git commit or remote.

| Environment | Filesystem | Result |
| --- | --- | --- |
| macOS arm64, Darwin 24.6.0 | Local APFS (`/System/Volumes/Data`); clean-copy temp directories on the same local volume | Format, unit/integration tests, race, vet, build, run, install, and outside-source workflow passed. |
| Linux arm64, kernel 5.10.124-linuxkit | Local container overlayfs; source was copied from a read-only host mount into `/work` before testing | Format, unit/integration tests, race, vet, build, run, install, and outside-source workflow passed. |

Executed on each clean source copy:

```sh
go version
uname -a
gofmt -l cmd internal test  # no output
go test ./...
go test -race ./...
go vet ./...
go run ./cmd/wrk --help
go build -o bin/wrk ./cmd/wrk
# GOBIN is a task-owned temporary directory, not a global install.
GOBIN=<temporary-installed-directory> go install ./cmd/wrk
```

The macOS clean source directory was created with Python `TemporaryDirectory` and contained only `cmd`, `internal`, `test`, `testdata`, `go.mod`, and `go.sum`; there was no `.git` or prebuilt `bin`. Its installed binary ran bare help, init, new, ready/list/show, start, direct body editing, combined title/status completion, list/all, stdin/file creation, and final validation from a separate temporary project outside the source tree. This executed the README examples with the real generated ID substituted for `<id>`.

Linux runner:

```sh
docker run --rm   --mount type=bind,source=/Users/calebmchenry/code/wrk,target=/source,readonly   golang:1.25.4-bookworm sh -eu -c '
    mkdir /work
    cp -R /source/cmd /source/internal /source/test /source/testdata /source/go.mod /source/go.sum /work/
    cd /work
    go version
    uname -a
    stat -f -c "%T" /work
    test -z "$(gofmt -l cmd internal test)"
    go test ./...
    go test -race ./...
    go vet ./...
    go run ./cmd/wrk --help
    go build -o bin/wrk ./cmd/wrk
    mkdir /installed
    GOBIN=/installed go install ./cmd/wrk
    cd /tmp
    /installed/wrk
    mkdir /tmp/wrk-smoke
    cd /tmp/wrk-smoke
    /installed/wrk init --json
    /installed/wrk new "Linux install smoke" --json
    /installed/wrk list --ready --json
    /installed/wrk validate --json
  '
```

The image digest was `sha256:e17419604b6d1f9bc245694425f0ec9b1b53685c80850900a376fb10cb0f70cb`. This was actual Linux execution, not cross-compilation. Raw local session logs were captured at `/tmp/wrk-sprint001-macos-verification.log` and `/tmp/wrk-sprint001-linux-verification.log`; this document retains the commands/results independent of those temporary logs.

### Definition of Done evidence map

| Boundary | Executed evidence |
| --- | --- |
| Arguments and JSON | `internal/cli` tests plus compiled `test/integration`: interspersed/equal/end-of-options flags, repeated/conflicting flags, errors for every data command, help outside a project, full-source round-trip, empty arrays, relative paths, human control escaping. |
| Config/discovery/compatibility | `internal/project`: original fixture read-only tree equality, nested invalid boundaries (including symlinks/files), duplicate/unknown/wrong-typed config settings, old-prefix IDs, custom types and opaque values, missing optionals. |
| Body/YAML | `internal/ticket` and `internal/schema`: LF/CRLF/Unicode/empty/no-newline bodies, later delimiters, opaque tags/nested collections/large integer precision, recursive and alias-heavy structures without expansion, aliases of edited fields and complex keys, no-op bytes. |
| Graphs/workflow | Separate parent/dependency cycles, missing/self/duplicate edges, valid parent-depends-on-child, 150-level hierarchy, canceled blockers, all statuses/reopening, explicit blocked updates, no cascades. |
| Creation/init | `internal/store`: cryptographic default RNG, forced and late collisions, 128-attempt exhaustion, complete hard-link publication, configured defaults/overrides/empty labels, missing parents, existing-entry refusal, foreign-file-safe init cleanup. |
| Snapshots/concurrency | Whole-input byte/inode/mode/inventory comparison, same-size and timestamp-preserving edits, editor rename, target/config/unrelated ticket changes; two real processes race first lock creation, config replacement cannot bypass the lock, inode persists, process kill releases it. |
| Publication/failures | Injected write/sync/close/link/rename/cleanup/directory-sync faults; real process kills before/after link/rename; pre-publication original-byte equality, permission/umask checks, ignored residual staging, committed-error JSON, failed stdout does not roll back. |
| Accepted limitation | `TestAcceptedEditorRace` explicitly characterizes the unsupported editor save between final comparison and replacement. README, agent guidance, ticket editing contract, and storage docs explain it. |
| Delivery | macOS/Linux format/tests/race/vet/build/install, clean-source copy and installed workflow outside source, README commands, exact runtime ignore patterns (tickets remain unignored). |

### Immutable fixture hashes (SHA-256)

```text
60d49381fb06bdabf3425ed9c1c3343853cb0c149f09f70a288325f690ce2905  config.yaml
46358cc3c708e5181b1adce3666adc0c531971481f40221075a1cd3203c17802  wrk-3f8a21b7.md
d611c280d8cbeda5b4b4a61e2ebccf4250970ad931456fb401ce2c4a1cc92862  wrk-682f60c7.md
54685fe34aad84793095887b43c7a529ca31cdae514c6894f419551c21ecd60d  wrk-9d42c6a1.md
```

No product-scope deviations were needed. Validators live alongside snapshots/config/graphs rather than a separate `validate.go`; shared schema/diagnostic helpers keep the packages acyclic. Linux uses the available local Docker runner. Frontmatter may be reformatted as agreed; bodies and unrelated semantic values are preserved. Follow-up scope remains relationship, priority/label, and custom-field operations.

## Phase 6 live dogfooding

`./bin/wrk list --all --json` found only the three original tickets, so there were no equivalent follow-ups. Created through the CLI under wrk-3f8a21b7:

- [wrk-1a09af55: Support parent and dependency updates](../../.wrk/wrk-1a09af55.md)
- [wrk-e146d171: Support priority and label updates](../../.wrk/wrk-e146d171.md)
- [wrk-c12ff4c6: Support custom-field creation and updates](../../.wrk/wrk-c12ff4c6.md)

Ticket body acceptance/evidence notes were edited directly between CLI operations. No ticket frontmatter was edited directly. The original compatibility fixtures remain unchanged.

Final live sequence passed: validate (six tickets), `update wrk-682f60c7 --status done --json`, validate, and show. Child status is done; parent wrk-3f8a21b7 remains todo. All milestone body acceptance criteria are checked. Compatibility SHA-256 hashes were rechecked unchanged. All six phases and every Definition of Done criterion are satisfied. No commits or pushes were performed.
