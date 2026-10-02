# CLI contract

## Commands

| Command | Behavior |
| --- | --- |
| `wrk`, `wrk --help`, `wrk help [command]` | Help, exit 0, no project discovery. |
| `wrk init [directory]` | Existing target directory, default cwd. Exclusively create `.wrk` and the [default config](configuration.md). Refuse any existing entry. |
| `wrk new "Title"` | Create a `todo` ticket with configured prefix, priority, and labels; print ID/path. |
| `wrk new "Title" --body-file path` | Exact UTF-8 body bytes; `-` reads stdin through EOF before locking. |
| `wrk new "Title" --parent id` | Create a child of an existing project ticket. |
| `wrk new "Title" --priority high --label cli --label storage` | Explicit overrides; supplied labels replace the entire configured default list. |
| `wrk new "Title" --no-labels` | Explicit empty labels; conflicts with `--label`. |
| `wrk list` | `todo` and `in-progress`, including blocked tickets. |
| `wrk list --all` | All four statuses. |
| `wrk list --ready` | `todo` with all dependencies `done`. |
| `wrk show id` | Full source plus derived children and unfinished dependency blockers. |
| `wrk update id --title "Title" --status in-progress` | Either or both flags, one validated mutation. No-op reports `changed: false`, without rewriting. |
| `wrk validate` | Aggregate determinable errors without writes. |

Every command accepts `--json` and `--help`. Flags may precede or follow positional
arguments after the command; global `--json` also precedes the command. Support
`--flag=value` and `--` to end option parsing. A value beginning with `--` must use
`--flag=value`. Unknown/repeated scalar flags, surplus arguments, empty updates,
`list --all --ready`, and `new --label ... --no-labels` are usage errors.

Titles are nonblank single-line strings; supplied spacing is retained. Priorities
are `low`, `normal`, `high`, `urgent`; statuses are `todo`, `in-progress`, `done`,
`canceled`. Duplicate labels remain allowed. Only `done` satisfies a dependency.
Explicit changes on blocked tickets and reopening are allowed. Nothing cascades.
General relationship, priority, label, and custom-field update flags are deferred.

## Discovery and validation

Walk upward to the nearest entry named `.wrk`. A file, symlink, missing config,
or malformed inner project is an authoritative failing boundary. `init` targets
its argument directly and can create nested projects. Config, ticket candidates,
and locks must be regular files; project boundaries must be real directories.

Scan only direct filenames matching `[a-z][a-z0-9]{0,15}-[0-9a-f]{8}.md` (anchored,
with a literal dot). Ignore supporting files and runtime artifacts. Embedded IDs
must match filenames and be unique. IDs from older prefixes remain valid.

All data commands require a fully valid project. Config and tickets use strict
YAML nodes: no duplicate keys, unknown built-in fields, implicit typed-value
coercions, or null substitutes. Unknown custom fields retain arbitrary YAML nodes,
tags, collections, and aliases. Parent and dependency graphs are checked separately.

`validate` sorts diagnostics by path, field/location, and code. If malformed inputs
prevent a later check, `CHECK_UNAVAILABLE` says so. Restore a known-good file or
obtain explicitly authorized repair; an invalid project cannot be repaired with
this sprint's `update`. This does not grant a manual-frontmatter editing exception.
Read commands create no locks, caches, or other files.

## Human and JSON output

Human lists show ID, status, priority, title, and blockers. `show` separates derived
information from source. Terminal controls are escaped in human output. JSON uses
standard escaping and retains source content exactly.

JSON is exactly one object plus newline on stdout, including ordinary failures;
there is no duplicate routine stderr diagnostic. Human errors go to stderr.
There are no prompts, progress messages, or incidental timestamps.

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

`project_root` is absolute, or null before discovery. Project paths are relative,
slash-separated, and independent of cwd. Tickets, children, blockers, dependency
IDs, and labels are lexicographically sorted. Empty arrays are `[]`, never null.
Blocker objects contain `id` and `status`. Summary defaults are effective values
and never cause omitted source fields to be inserted during updates.

| Command | Successful `result` |
| --- | --- |
| `init` | `created`, `path: ".wrk"`, `config_path: ".wrk/config.yaml"` |
| `new` | `ticket` summary, `created: true` |
| `list` | `tickets` array of summaries |
| `show` | `ticket`, exact full UTF-8 `source`, `children` summaries |
| `update` | `ticket`, `changed` |
| `validate` | `valid: true`, `ticket_count` |
| Help | `usage` string |

`show.source` includes every custom value and the body; no lossy custom YAML-to-JSON
projection is offered. Errors contain `code`, `message`, and applicable `path`,
`field`, `line`, `column`, and involved `ids`.

Exit 0 means success/help; 2 means usage error; 1 means operational/data error.
Stable error codes include `USAGE`, `NOT_FOUND`, `INVALID_TARGET`, `ALREADY_EXISTS`,
`INVALID_PROJECT`, `INVALID_CONFIG`, `INVALID_TICKET`, `INVALID_BODY`, `ID_MISMATCH`,
`DUPLICATE_ID`, `MISSING_REFERENCE`, `SELF_REFERENCE`, `DUPLICATE_DEPENDENCY`, `CYCLE`,
`CHECK_UNAVAILABLE`, `PROJECT_INVALID`, `BUSY`, `CONFLICT`, `ID_EXHAUSTED`, `IO`,
`PRESERVATION_UNSUPPORTED`, `CLEANUP_FAILED`, and `DURABILITY_UNCERTAIN`.

On pre-publication failure, `ok: false`, `result: null`, and existing ticket/config
bytes remain unchanged. After publication, a later error retains the affected
ID/path in `result`, adds `publication: "committed"`, and reports `ok: false`.
Inspect before retrying. A killed process or failed stdout cannot promise an error
envelope and never causes rollback of committed data. See [storage](storage.md).
