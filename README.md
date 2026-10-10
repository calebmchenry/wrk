# wrk

**Project tasks that live with your code.**

wrk is a local task tracker for developers and coding agents. Each ticket is a
Markdown file in `.wrk/`, with a description, status, and relationships to other
tickets. Manage work from the terminal or a local browser workspace, and commit
your plans, decisions, and progress alongside the code they describe.

No account, hosted service, or separate database is required. Ticket management
works offline on macOS and Linux.

[Get started](#quick-start) · [Install](#build-and-install) · [Browser workspace](#local-browser-workspace) · [Documentation](#documentation-and-help)

## Why use wrk?

wrk fits projects where the people and agents doing the work already share a
repository and want the backlog to travel with it.

- **Keep context with the code.** Tickets move through Git branches, commits, and
  reviews with the implementation. Anyone with the checkout can read them.
- **Pick up where you left off.** Record acceptance criteria, decisions, and
  handoff notes in Markdown so the next person or agent can continue the work.
- **See what can happen next.** Break work into parent/child tasks, express
  prerequisites, and use `wrk list --ready` to find tasks whose dependencies are done.
- **Use the interface that suits the work.** Browse and edit visually, use the CLI
  from your terminal, or consume JSON in a script. They all use the same files.

wrk leaves commits and synchronization to your normal Git workflow. Its browser
workspace runs on your own machine.

## Build and install

### Install a release

Download the latest release from [GitHub Releases](https://github.com/calebmchenry/wrk/releases/latest).
Standalone binaries are available for macOS and Linux on Intel/x86-64 and ARM64,
with the CLI and browser workspace included. No Go installation is required.
See the [installation guide](docs/install.md) for archive selection, checksum
verification, and installation commands.

Check for and install newer stable releases with:

```sh
wrk upgrade --check
wrk upgrade
```

### Build from source

If you prefer to build it yourself, use **Go 1.25 or newer** and Git:

```sh
git clone https://github.com/calebmchenry/wrk.git
cd wrk
go install ./cmd/wrk
export PATH="$(go env GOPATH)/bin:$PATH"
wrk version
```

If you have set `GOBIN`, add that directory to PATH instead. Add the PATH setting
to your shell configuration to keep it for future terminals. Once installed,
`wrk` runs from any project directory; Go is only needed to build it.

Source builds are updated by bringing the checkout up to date and rerunning
`go install ./cmd/wrk`.

## Quick start

Try wrk in a disposable directory after installing it:

```sh
wrk_demo_dir="$(mktemp -d)"
cd "$wrk_demo_dir"
wrk init
wrk new "Ship a getting-started guide" --priority high --label docs
wrk list --ready
```

`init` creates `.wrk/config.yaml`; `new` creates a ticket and prints its ID and
file path. Copy that ID in place of `<id>` for the rest of the workflow:

```sh
wrk show <id>
wrk update <id> --status in-progress
# Write the guide, verify it, and record the result in the ticket.
wrk update <id> --status done
wrk list --all
wrk validate
```

`list` shows active work. `list --ready` shows `todo` tickets with every dependency
`done`; `list --all` includes completed and canceled work. Marking work `blocked`
keeps it visible but excludes it from the ready list until you explicitly change
its status.

To use wrk in your own project, run `wrk init` from that project's root instead.
Initialize each project once; an existing `.wrk/` directory is never overwritten.
Commands also find the project from subdirectories, or you can select it explicitly:

```sh
wrk --project /path/to/your/project list --ready
```

## Keep your work in Git

Commit `.wrk/config.yaml` and `.wrk/*.md` with your project. Ticket descriptions
are ordinary Markdown: use them for the outcome, acceptance criteria, decisions,
and notes for whoever takes over next.

You can edit the body of `.wrk/<id>.md` in your editor. Use `wrk new`, `wrk update`,
or the browser for ticket metadata—the YAML frontmatter at the top of the file.
Keep direct file edits and Git operations between CLI or browser saves.
Run `wrk validate` after manual edits or merges to check tickets and relationships.

Add these runtime files to your project's `.gitignore`:

```gitignore
**/.wrk/.lock
**/.wrk/.wrk-stage-*
/.wrk-runs/
```

See the [ticket format](docs/ticket-format.md) for an example ticket and
[configuration](docs/configuration.md) to customize the ID prefix, default labels,
and custom fields.

## Organize a larger project

Once you have tickets, use their IDs to connect and filter the work. Replace the
placeholder IDs below with IDs from `wrk list` or `wrk new`.

| When you want to… | Command |
| --- | --- |
| Break a task into smaller pieces | `wrk new "Write the examples" --parent <parent-id>` |
| Wait for a prerequisite | `wrk new "Publish the guide" --depends-on <draft-id>` |
| Add a task to a batch | `wrk update <id> --add-label launch` |
| Find ready work in that batch | `wrk list --ready --label launch` |
| Find ready descendants of a task | `wrk list --ready --under <parent-id>` |
| Record a blocker | `wrk update <id> --status blocked` |
| Resume a blocked task | `wrk update <id> --status todo` |
| Link useful context | `wrk update <id> --add-related <other-id>` |

Parents group work; dependencies determine readiness; related links provide
context. Completing a child does not automatically complete its parent.
For blocked work, record the reason and what will unblock it in the description.

See the [CLI reference](docs/cli.md) for priorities, custom fields, recursive
labeling, and all command options.

## Local browser workspace

From an initialized project, run:

```sh
wrk serve --open
```

The workspace opens at `http://127.0.0.1:7331`. Keep the command running while you
work; Ctrl-C stops it. If that port is in use, `wrk serve --port 0 --open` chooses
an available one and opens the printed URL.

Search titles, filter by status or tags, expand parent/child tasks, and open a
ticket to read its description. Create tasks, add children, and edit titles,
descriptions, status, tags, dependencies, and related links. Browser tags are the
same as CLI labels.

Changes from another terminal or an agent appear automatically, normally within
two seconds. If a ticket changes while you are editing it, the workspace keeps
your draft and asks you to review the current version before saving over it.
The UI is bundled in the executable; there is no separate frontend install.

See the [browser walkthrough](docs/browser.md#browser-and-agent-walkthrough) for
working in the browser and CLI together.

## Work with agents and scripts

An agent that can run shell commands and read project files can use wrk. Give it
a ticket or a scope, and keep its findings in ticket descriptions so future
sessions have the context they need. A useful instruction for your project's
agent guidance is:

> Use wrk to track work. Read the ticket with `wrk show <id>`, mark it
> `in-progress`, implement and verify the change, and record decisions and results
> in its body. Mark it `done` when its acceptance criteria are met. If blocked,
> record why and set `blocked`. Run `wrk validate` before handing off.

For scripts, commands support `--json`:

```sh
wrk list --ready --label docs --json
```

`wrk run` applies a command to matching tickets one at a time, substituting each
ID for `{id}`. For example, inspect every ready documentation task:

```sh
wrk run --ready --label docs --verbose -- wrk show '{id}'
```

`--verbose` displays the command's output as it runs. Supply your own script or
agent command to do the work. Add `--expect-status done`
to require that the command leaves each ticket completed, or `--max-tickets 1`
to try one ticket first. Runs save output in `.wrk-runs/` and stop on failure;
retries are explicit. wrk does not include an AI provider.

Read the [runner reference](docs/cli.md#scoped-command-execution) and
[agent work-session guide](docs/burns.md) for scope selection, waiting for new work,
completion checks, and recovery.

## Documentation and help

- [CLI reference](docs/cli.md): commands, filters, JSON output, and troubleshooting.
- [Browser guide](docs/browser.md): navigation, editing, and handling conflicts.
- [Installation](docs/install.md): binaries, checksums, source builds, and upgrades.
- [Ticket format](docs/ticket-format.md) and [configuration](docs/configuration.md):
  how your work is stored and customized.
- [Agent work sessions](docs/burns.md): scoped execution and durable handoffs.
- [Full documentation index](docs/index.md): workflow, storage, and development details.

Run `wrk --help` for command syntax. For bugs, questions, or feature requests,
[open an issue](https://github.com/calebmchenry/wrk/issues). Include the command,
expected behavior, actual output, and `wrk version` when reporting a bug.

## Developing wrk with wrk

Contributions are welcome. This repository tracks its own work in [`.wrk/`](.wrk/).
Follow the [daily workflow](docs/index.md#daily-workflow) and use
`go run ./cmd/wrk` from the repository root to exercise the current checkout.
See the [development guide](docs/development.md) for setup, required checks,
browser tests, and packaging verification.
