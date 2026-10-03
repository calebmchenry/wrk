# Working with wrk tickets

Read [the ticket format](../docs/ticket-format.md), [configuration](../docs/configuration.md),
and [CLI contract](../docs/cli.md) before changing tickets.

Follow the [daily workflow](../docs/index.md#daily-workflow) to find or create a
ticket, start work, record results, and complete it. In this repository, run each
`wrk` command below as `go run ./cmd/wrk` from the repository root to use the current
checkout, or use a freshly rebuilt `./bin/wrk`.

- Edit Markdown bodies directly for descriptions, acceptance criteria, and handoff notes.
- Use `wrk new` to create tickets. Creation supports `--body-file` (including stdin),
  `--parent`, `--priority`, repeated `--label`, `--no-labels`, repeated `--depends-on`,
  and repeated `--field name=YAML`.
- Use `wrk update <id>` for title/status/priority, labels, parent/dependency, and
  custom-field changes. Clear parenting with `--no-parent`, edit prerequisites with
  `--add-dependency` / `--remove-dependency`, and set/remove custom values with
  `--field name=YAML` / `--remove-field name`. Only labels support `--recursive`.
  Never change an existing ticket's ID. See the CLI contract for conflicts and quoting.
- `.wrk/config.yaml` may be edited directly. Defaults affect only new tickets.
- Run `wrk validate` to check the whole project. Normal data commands fail if any
  ticket is invalid; restore known-good data or obtain explicit repair authorization.
- Do direct body/config edits and Git operations outside CLI mutations. The lock
  serializes CLI writers and the final comparison detects visible external changes,
  but a save between comparison and replacement is not protected. See [storage](../docs/storage.md).
- Ignore `.lock` and `.wrk-stage-*` runtime artifacts; do not delete a live lock inode.
  If publication committed before an error/interruption, inspect before retrying.
