# wrk

wrk is a local CLI for project tasks. Tickets live as Markdown files in `.wrk/`
alongside your code and travel through Git branches, commits, and reviews. Agents
can read the work and edit descriptions directly; the CLI validates metadata and
relationships before writing.

## Install a release

Download a standalone binary from [GitHub Releases](https://github.com/calebmchenry/wrk/releases).
Go is not required. macOS (`darwin`) and Linux are supported on `amd64` (Intel/x86-64)
and `arm64` (Apple Silicon/AArch64). For example, to install v0.1.0 on Apple Silicon:

```sh
(
set -eu
wrk_download_dir="$(mktemp -d)"
cd "$wrk_download_dir"
curl -fLO https://github.com/calebmchenry/wrk/releases/download/v0.1.0/wrk_0.1.0_darwin_arm64.tar.gz
curl -fLO https://github.com/calebmchenry/wrk/releases/download/v0.1.0/wrk_0.1.0_checksums.txt
awk '$2 == "wrk_0.1.0_darwin_arm64.tar.gz"' wrk_0.1.0_checksums.txt > selected-checksum.txt
test -s selected-checksum.txt
shasum -a 256 -c selected-checksum.txt
tar -xzf wrk_0.1.0_darwin_arm64.tar.gz
mkdir -p "$HOME/.local/bin"
install -m 755 wrk "$HOME/.local/bin/wrk"
"$HOME/.local/bin/wrk" version
)
```

Substitute the platform/architecture in both archive filenames and the checksum
selection. On Linux, `sha256sum -c selected-checksum.txt` also works. Add
`$HOME/.local/bin` to PATH. The checksum verifies the archive against the trusted
GitHub release; it is not an independent cryptographic signature.

```sh
wrk version                    # also: wrk --version
wrk upgrade --check            # inspect without writing files
wrk upgrade                    # install a newer stable release
wrk upgrade --check --json
```

These commands work outside a project and inside an invalid/newer-format project.
Only `upgrade` contacts the network. Checks exit 0 whether a newer version exists
or not; failures exit 1 and invalid arguments exit 2. Upgrades never downgrade.
An unknown/development version reports availability as unknown; development,
snapshot, and `go run` builds must be installed manually. A binary predating the
upgrade command needs one manual installation of an upgrade-capable release.

Self-upgrade supports user-owned standalone binaries in writable directories.
It follows standalone symlinks and replaces their resolved target. Recognized
package-manager paths receive guidance to use that manager. It never invokes
sudo or chooses a different executable from PATH. `wrk update` still edits tickets.
For connectivity, rate-limit, checksum, permission, and interrupted-upgrade
troubleshooting, see the [upgrade contract](docs/cli.md#version-and-upgrade).

## Build and install

Use Go 1.25 or newer on macOS or Linux. The verified development toolchain is
Go 1.25.4 on arm64 for both platforms. From this repository:

```sh
go run ./cmd/wrk --help
go build -o ./bin/wrk ./cmd/wrk
./bin/wrk validate
go install ./cmd/wrk
```

`go install` writes to `GOBIN` when set, otherwise the `bin` directory in `GOPATH`
(normally `$HOME/go/bin`). For the default destination, add this to your shell:

```sh
export PATH="$(go env GOPATH)/bin:$PATH"
wrk --help
```

If you set `GOBIN`, put that directory on PATH instead. The module name is `wrk`
for local development; there is no published module or package-manager install.
Dependency downloads are needed at build time. Normal CLI commands work offline.

## Try the workflow

After installation, create a disposable project:

```sh
wrk_demo_dir="$(mktemp -d)"
cd "$wrk_demo_dir"
wrk init
wrk new "Try wrk" --priority high --label demo
wrk list --ready
```

`new` prints the ticket's ID and relative path. Substitute its ID for `<id>` below:

```sh
wrk show <id>
wrk update <id> --status in-progress
wrk update <id> --title "Tried wrk" --status done
wrk list --all
wrk validate
```

From another directory, select the existing project with `--project directory`
or `--config directory/.wrk/config.yaml` on `new`, `list`, `show`, `update`, and
`validate`. Selectors work before or after the command:

```sh
wrk --project '/path/to/my project' list --ready
wrk show <id> --config '/path/to/my project/.wrk/config.yaml'
```

Explicit selection overrides cwd discovery and requires that exact project; it
never searches ancestors or initializes missing data. Relative selectors and
`--body-file` paths use the invocation directory. See the
[project selection contract](docs/cli.md#project-selection) for path and symlink behavior.

Use `update --body-file path|-` to replace a description, optionally with metadata
changes in the same operation. Empty input clears it; omitted input preserves it.
Input must be UTF-8 and is read before locking. Repeating the existing body is a
no-op. Body edits cannot use `--recursive`.

Between CLI updates, you may also edit the Markdown body in `.wrk/<id>.md`
directly. Everything through the closing `---` is metadata and must be changed
through supported CLI commands. Metadata updates retain the body byte-for-byte,
including whitespace and missing final newlines.

```sh
printf 'Description from stdin\n' | wrk new "Another task" --body-file - --no-labels
printf 'Updated description\n' | wrk update <id> --body-file - --status in-progress
wrk new "Task from a file" --body-file .wrk/config.yaml
wrk list --json
```

Use `new --parent <id>` for children. Creation supports `--priority`, repeated
`--label` (replaces configured labels), and `--no-labels`. Updates support
`--title`, `--status`, `--priority`, repeated `--label` to replace the entire label
list, `--no-labels` to clear it, and repeated `--add-label` / `--remove-label` for
incremental edits. Parent/dependency and custom-field edits are supported too:

```sh
wrk update <id> --priority urgent --label backend --label cli
wrk update <id> --no-labels
wrk update <id> --priority normal --add-label reviewed --remove-label needs-triage
wrk update <id> --parent <parent-id>
wrk update <id> --no-parent --add-dependency <prerequisite-id>
wrk update <id> --remove-dependency <prerequisite-id>
wrk new "Estimate work" --depends-on <prerequisite-id> --field 'estimate=3.5'
wrk update <id> --field 'customer="123"' --field 'needs_review=true'
wrk update <id> --remove-field estimate
```

Priority is `low`, `normal`, `high`, or `urgent`. Replacement/clearing conflicts
with add/remove flags, and `--label` conflicts with `--no-labels`. Any one label
mode can combine with other metadata changes in a single validated update.
Replacement preserves supplied order and duplicates in source; JSON summaries
sort labels. Unchanged values are no-ops, including clearing absent labels or
setting an omitted priority to its effective value `normal`. Creation defaults
never change existing tickets or influence updates.

Parent set/clear flags conflict. Dependency add/remove flags are repeatable and
idempotent, but adding and removing the same ID conflicts. Creation accepts
repeated `--depends-on` and rejects duplicate edges. Relationships must reference
existing tickets and cannot form cycles within either graph.

Repeated `--field 'name=YAML'` sets typed custom values on creation or update;
`--remove-field name` removes them. Shell quotes preserve the argument, while
inner YAML quotes select strings such as `"123"` or `"true"`. Values retain YAML
tags, nested collections, aliases, and exact numeric precision. Configured types
are checked without coercion. Repeated assignments or setting/removing the same
field conflict. Removing absent fields and setting identical values are no-ops.
See [relationship edits](docs/cli.md#relationship-updates) and
[custom-field input](docs/cli.md#custom-field-input-and-updates) for full semantics.

Bare `wrk` prints help. Flags may come before or after positional arguments;
`--flag=value` and `--` are supported. Every command supports `--json`, including
errors. See [the CLI contract](docs/cli.md) for output schemas and exit codes.

## Scoped agent work sessions

Select an arbitrary batch with labels, or list a deliverable's descendants:

```sh
wrk update <id> --add-label burn-tonight --remove-label needs-triage
wrk update <parent-id> --add-label burn-tonight --recursive
wrk list --label burn-tonight
wrk list --ready --label burn-tonight
wrk list --ready --under <parent-id>
wrk update <id> --status blocked
wrk update <id> --status todo
```

Add/remove flags repeat and preserve unrelated labels; adding an existing label
or removing an absent one is a no-op. Adding and removing the same label conflicts.
Recursive updates permit labels only and include the root plus all descendants,
even done/canceled tickets. Replacement (`--label`) and clearing (`--no-labels`)
also work recursively and affect each target's entire label list; use add/remove
to preserve unrelated labels. Title, status, and priority cannot be recursive. Labels are a snapshot, not inheritance: supply
`new --parent <id> --label burn-tonight` for later subtasks. `list --under` excludes
the root. Repeated label filters require **all** labels; label, descendant, and
status filters intersect. Dependencies outside the scope still affect readiness.

An agent should read scope instructions and durable notes, resume eligible
in-progress work, then pick a ready ticket, mark it in-progress, implement and
verify it, record results, and mark it done. When work needs external input, record
the reason, unblocking condition, and resume notes in its body, mark it blocked,
and continue with other eligible work. Blocked stays visible in active lists and
never becomes ready automatically; unblock explicitly. Only done satisfies a
dependency. Status changes never cascade.

Agree on scope, permissions, required verification, and stopping limits before a
session. Stop when the batch is complete or nothing can proceed, and report
unfinished work and blockers. An empty ready list alone does **not** mean complete:
inspect in-progress/blocked work, unmet dependencies, and the root separately.
Keep handoffs in ticket bodies. Follow the [full agent burn loop](docs/burns.md)
for safe resumption and stop rules. These are CLI primitives and a workflow;
there is no built-in AI runner or scheduler.

Recursive labeling publishes one file at a time under one writer lock. On a
partial failure, output identifies committed, unchanged, and pending tickets.
Inspect the entire scope and resolve the error before retrying the same idempotent
operation; retries use a fresh descendant snapshot. See
[recursive publication and recovery](docs/storage.md#recursive-label-publication).

## Safe local use

The nearest `.wrk` entry is the project boundary, even if invalid. `init` requires
an existing target directory and refuses any existing `.wrk` entry. Any invalid
ticket makes ordinary data commands fail; use read-only `wrk validate` for sorted,
actionable diagnostics. Existing version-1 tickets need no migration. Older
clients reject `blocked` status and may refuse all data commands once it appears; upgrade all clients
before using it.

CLI writers use a persistent advisory lock and check all validation inputs for
changes before atomic publication. **Do direct body/config edits and Git operations
outside CLI mutations.** A non-cooperating editor can still save between the final
comparison and replacement; that race is not protected. If an error reports
`publication: "committed"`, inspect the affected ticket before retrying.

Ignore `**/.wrk/.lock` and `**/.wrk/.wrk-stage-*` in other projects' `.gitignore`.
These runtime files are ignored by ticket scanning. See [storage and recovery](docs/storage.md)
for filesystem support, concurrency limits, permissions, and interruption handling.

## Developing wrk with wrk

Use wrk for this repository's own work. Follow the [daily workflow](docs/index.md#daily-workflow)
to find or create a ticket, mark it in progress, record results in its body, and
complete it through the CLI. Run the current checkout from the repository root:

```sh
go run ./cmd/wrk validate
go run ./cmd/wrk list
```

No initialization is needed here: the backlog already lives in [`.wrk/`](.wrk/).
Prefer `go run ./cmd/wrk` over an installed binary that may be stale, or rebuild
`./bin/wrk` after CLI changes. Read the [ticket editing contract](docs/ticket-format.md#editing-contract)
before editing tickets.

For code changes, run the development checks:

```sh
gofmt -l cmd internal test
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
go build -o ./bin/wrk ./cmd/wrk
```

Formatting must report no files. Use `-count=1` so the integration package reruns
its compiled CLI: it builds the binary in `TestMain`, and Go's test cache does not
track that runtime build as a source dependency. Run the checks on both macOS and
Linux local filesystems; cross-compilation alone does not exercise storage.
Tests use disposable projects and retain the
immutable original data in `testdata/compat/.wrk/`. [Sprint 001 evidence](docs/sprints/SPRINT-001-EXECUTION.md)
records macOS/Linux verification and clean-source build/install runs.
