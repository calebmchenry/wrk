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
| `wrk list` | `todo`, `in-progress`, and manually `blocked` tickets, including unfinished dependency blockers. |
| `wrk list --all` | All five statuses. |
| `wrk list --ready` | `todo` with all dependencies `done`. |
| `wrk list --label burn --label backend` | Require every supplied label (AND); exact, case-sensitive matching. |
| `wrk list --under id` | Descendants at every depth, excluding the root. Intersects label and status/readiness filters. |
| `wrk show id` | Full source plus derived children and unfinished dependency blockers. |
| `wrk update id --title "Title" --status in-progress` | Either or both flags, one validated mutation. No-op reports `changed: false`, without rewriting. |
| `wrk update id --add-label burn --remove-label triage` | Repeatable, idempotent label edits; may combine with title/status. |
| `wrk update id --add-label burn --recursive` | Labels only; root plus all descendants, regardless of status. Per-ticket publication results. |
| `wrk validate` | Aggregate determinable errors without writes. |

Every command accepts `--json` and `--help`. Flags may precede or follow positional
arguments after the command; global `--json` also precedes the command. Support
`--flag=value` and `--` to end option parsing. A value beginning with `--` must use
`--flag=value`. Unknown/repeated scalar flags, surplus arguments, empty updates,
`list --all --ready`, and `new --label ... --no-labels` are usage errors.

Titles are nonblank single-line strings; supplied spacing is retained. Priorities
are `low`, `normal`, `high`, `urgent`; statuses are `todo`, `in-progress`, `blocked`,
`done`, `canceled`. Duplicate labels remain allowed. Only `done` satisfies a dependency.
Explicit changes on blocked tickets and reopening are allowed. Nothing cascades.
General relationship, priority, label replacement/clear, and custom-field update flags are deferred.

## Scopes and label mutations

`--label` is repeatable on `list`: all requested labels must be present. Repeating
the same filter is harmless. Label matching uses exact strings without trimming or
case folding. Label filters, `--under`, and `--all`/`--ready` intersect; readiness
still checks dependencies across the **whole project**. `--under` is a scalar flag,
excludes the selected root, and returns `NOT_FOUND` for a missing/invalid ID even
if another filter matches nothing. An existing leaf produces an empty array.

`--add-label` and `--remove-label` are repeatable on `update`. Adding a present
label or removing an absent label succeeds without rewriting. Removing a label
removes every occurrence; unrelated labels retain their order and duplicates in
source, and new labels are appended once in argument order. Labels must be
nonempty UTF-8 strings. An absent labels field stays omitted on no-op removal;
removing the last label from an existing field leaves `labels: []`. Configured
creation defaults never affect these edits.

Adding and removing the same label in one invocation is `USAGE`, independent of
flag order. `--recursive` requires at least one label mutation and rejects title,
status, or other metadata edits. Unsupported flags (`--label`, `--no-labels`,
priority, relationships, custom fields on update) remain errors. Single-ticket
label edits can combine with title/status as one validated update.

Recursive labeling selects the root **and** all descendants, at every depth and
in every status, following parent edges only. It applies a snapshot, not
inheritance: later children need explicit `new --label` flags. Publication order
and output are sorted by ID, so the root need not appear first. See the
[batch storage contract](storage.md#recursive-label-publication) for failures and
safe retries.

`blocked` is a manual status. Record its reason, unblocking condition, and resume
notes in the body. It remains in active lists but never appears in `--ready`, even
after dependencies finish. Resume with an explicit status update, commonly
`--status todo` (or `in-progress` when resuming authorized work). Only `done`
satisfies dependencies; blocked and canceled prerequisites do not. Parent and
child status changes never cascade. Dependency `blockers` in output remain derived
from dependencies; a manually blocked ticket may have an empty blockers array.

The project format and JSON schema remain version 1. Existing tickets need no
migration, but older clients reject the new blocked value and may refuse **all**
data commands in a project containing it. Upgrade all clients before using it.

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
| `update` (single ticket) | `ticket`, `changed` |
| `update --recursive` | `root_id`, `updates` array of `{ticket, changed, publication}`, overall `changed` |
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

Recursive `updates` includes every selected ticket, including no-ops. Each entry's
`publication` is `committed` (renamed by this invocation), `unchanged` (already had
the requested labels), or `pending` (needed a change but was not published).
`changed` is true only for committed entries; overall `changed` means any commit.
Successful batches contain no pending entries. On handled failure before any
commit, the ordinary `result: null` contract holds. After a commit, return the full
batch result, top-level `publication: "committed"`, `ok: false`, and exit 1. A final
sync/cleanup/lock-release error may occur even when all entries are committed.
Summaries describe the operation's observed state, not a fresh read of external
edits. Human output lists ID, path, and publication state, with an inspection
notice on partial/uncertain publication. Kills or failed stdout may provide no
report; inspect all targets. No committed data is rolled back.
