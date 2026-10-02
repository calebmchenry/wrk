## Working on wrk

Use wrk to track this repository's work. At the start of a task, read
[docs/index.md](docs/index.md) and follow its [daily workflow](docs/index.md#daily-workflow).
Run the current checkout's CLI from the repository root with `go run ./cmd/wrk`;
an installed `wrk` may be stale.

- Use `list` and `show` to find and read relevant tickets. Reuse an existing ticket
  when it covers the work; otherwise create one with `new`.
- Set the ticket to `in-progress` before implementation. Keep its body current with
  decisions, verification, and remaining work; mark it `done` when its criteria are met.
- Create tickets and change supported metadata through the CLI. Edit ticket bodies
  directly, but never hand-edit frontmatter to work around missing commands.
- Run `go run ./cmd/wrk validate` before handing off work. Keep direct body/config
  edits and Git operations outside CLI mutations.

## /docs

`docs/` contains Markdown documentation for agents. Agents are responsible for
maintaining it and may add new content. Consult [the index](docs/index.md) for
existing guidance; when you learn an undocumented workflow, document it and add
an index link.
