# wrk

wrk is a local CLI for project tasks. Tickets live as Markdown files in `.wrk/`
alongside your code and travel through Git branches, commits, and reviews. Agents
can read the work and edit descriptions directly; the CLI validates metadata and
relationships before writing.

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

Between CLI updates, edit the Markdown body in `.wrk/<id>.md` directly. Everything
through the closing `---` is metadata and must be changed through supported CLI
commands. Title/status updates retain the body byte-for-byte, including whitespace
and missing final newlines.

```sh
printf 'Description from stdin\n' | wrk new "Another task" --body-file - --no-labels
wrk new "Task from a file" --body-file .wrk/config.yaml
wrk list --json
```

Use `new --parent <id>` for children. Creation supports `--priority`, repeated
`--label` (replaces configured labels), and `--no-labels`. Updates currently support
only `--title` and `--status`. General parent/dependency, priority/label, and custom
field updates are tracked as follow-up work.

Bare `wrk` prints help. Flags may come before or after positional arguments;
`--flag=value` and `--` are supported. Every command supports `--json`, including
errors. See [the CLI contract](docs/cli.md) for output schemas and exit codes.

## Safe local use

The nearest `.wrk` entry is the project boundary, even if invalid. `init` requires
an existing target directory and refuses any existing `.wrk` entry. Any invalid
ticket makes ordinary data commands fail; use read-only `wrk validate` for sorted,
actionable diagnostics. Existing version-1 tickets need no migration.

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
before editing tickets; updates currently support title and status.

For code changes, run the development checks:

```sh
gofmt -l cmd internal test
go test ./...
go test -race ./...
go vet ./...
go build -o ./bin/wrk ./cmd/wrk
```

Formatting must report no files. Tests use disposable projects and retain the
immutable original data in `testdata/compat/.wrk/`. [Sprint 001 evidence](docs/sprints/SPRINT-001-EXECUTION.md)
records macOS/Linux verification and clean-source build/install runs.
