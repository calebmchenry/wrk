# CLI contract

## Commands

| Command | Behavior |
| --- | --- |
| `wrk`, `wrk --help`, `wrk help [command]` | Help, exit 0, no project discovery. |
| `wrk version`, `wrk --version` | Version, commit, build kind, and platform; offline, no project discovery. |
| `wrk upgrade --check` | Check the latest stable release without local writes. |
| `wrk upgrade` | Verify and atomically install a newer standalone release binary. |
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
| `wrk update id --add-label burn --remove-label triage` | Repeatable, idempotent label edits; may combine with title/status/priority. |
| `wrk update id --priority high --label cli --label storage` | Set priority and replace the entire label list. |
| `wrk update id --no-labels` | Clear labels; absent/already-empty labels are a no-op. |
| `wrk update id --add-label burn --recursive` | Labels only; root plus all descendants, regardless of status. Per-ticket publication results. |
| `wrk validate` | Aggregate determinable errors without writes. |

Every command accepts `--json` and `--help`. Flags may precede or follow positional
arguments after the command; global `--json` also precedes the command. Support
`--flag=value` and `--` to end option parsing. A value beginning with `--` must use
`--flag=value`. Unknown/repeated scalar flags, surplus arguments, empty updates,
`list --all --ready`, and `--label ... --no-labels` on new/update are usage errors.

## Version and upgrade

`version` and `upgrade` dispatch before cwd/project discovery and validation, so
invalid projects or unsupported project formats do not prevent an upgrade.
`--version` aliases `version` and supports `--json`. `upgrade` only accepts
`--check`, `--json`, and `--help`; pinned versions, force, downgrade, prerelease,
private-repository authentication, and custom release sources are not supported.
Ordinary ticket commands and version/help never contact the network.

Both commands use the existing envelope with `project_root: null`. Version's
result contains `version`, `commit`, `build_kind`, `goos`, and `goarch`.
Plain builds report `dev`/`development`; snapshot packaging reports `snapshot`.
Stable release builds have a canonical `X.Y.Z` version, a full Git commit, and
`build_kind: "release"`. See the [shared release contract](releases.md).

Upgrade results contain `current_version`, `latest_version`, `available`, and
`changed`. `available` is boolean for comparable stable releases, or null with
an explanatory `reason` for development/unknown metadata. `--check` creates no
stages, locks, caches, or project files. Successful checks exit 0 regardless of
availability. Installation adds `destination`, and sets `changed: true` and
`publication: "committed"` after replacement. Same/newer versions return a no-op.

The client uses anonymous HTTPS to the fixed public `calebmchenry/wrk` latest
release endpoint. Drafts, prereleases, malformed versions, missing/duplicate
assets, and URLs outside the selected repository/tag/asset contract are rejected.
Redirects must stay HTTPS (at most five); requests time out after 30 seconds.
Responses are bounded to 2 MiB metadata, 1 MiB manifest, and 64 MiB archive.
The expanded executable is limited to 128 MiB with 1 MiB archive overhead.
SHA-256 is verified before extraction/execution. Only a regular root `wrk` entry
is accepted; paths, links, extra entries, sparse data, and corrupt gzip trailers
are rejected. The staged executable must report the selected version/platform
within five seconds. Checksums trust GitHub's release source; they are not signatures.

Installation resolves the running executable, follows standalone symlinks, and
preserves its permission bits. It requires a user-owned executable and writable
directory; setuid/setgid and recognized Homebrew/MacPorts/Nix/Snap/system-package
paths are refused. No sudo, interactive prompt, or alternate PATH installation
is used. Other package managers may require manual identification by the user.

An independent persistent `.<executable-name>.upgrade.lock` beside the resolved
binary serializes installers. Do not remove that lock while upgrades may run.
The updater probes the installed version under the lock to reject a stale running
process, stages in the same directory, syncs, compares target bytes/identity/mode
and symlink resolution, atomically renames, and syncs the directory. As with ticket
updates, the comparison/rename boundary cannot protect against a non-cooperating
editor or hostile directory changes. Avoid manually replacing the binary during
an upgrade. Other hard links to the old inode remain unchanged.

Before replacement, failure returns `result: null` and preserves the installed
binary. Handled failures remove owned stages. After replacement, errors retain
`changed: true`, `destination`, and `publication: "committed"` with `ok: false`.
Inspect `<destination> version` before retrying; no automatic rollback is performed.
Killed processes release the OS lock but may leave `.wrk-upgrade-*` stage files.
Only remove known abandoned stages when no upgrader is running. A killed process
or failed stdout may not emit a report, so inspect the installed version first.

| Diagnostic | Action |
| --- | --- |
| `NETWORK` | Check connectivity/proxy/TLS settings and retry; incomplete downloads are never installed. |
| `RATE_LIMIT` | Wait for GitHub's anonymous rate limit to reset, or download/verify manually. |
| `RELEASE_NOT_FOUND` | No stable release or required download is currently available; check Releases. |
| `RELEASE_INVALID`, `INTEGRITY`, `CANDIDATE_INVALID` | Stop and inspect the release assets/checksums; the prior installation remains intact. |
| `INSTALL_METHOD`, `UNSUPPORTED_PLATFORM` | Use your package manager, rebuild development code, or manually install a supported standalone release. |
| `PERMISSION`, `IO` | Inspect the named operation and installation directory; prefer a user-owned standalone location. |
| `BUSY`, `CONFLICT` | Wait for other work to finish, inspect the installed version, then rerun that binary. |
| `DURABILITY_UNCERTAIN`, `CLEANUP_FAILED` | Inspect the publication marker and installed version; resolve directory/stage issues before retrying. |

All operational failures exit 1 and usage errors exit 2. JSON remains a single
envelope on stdout with no routine stderr messages.

Titles are nonblank single-line strings; supplied spacing is retained. Priorities
are `low`, `normal`, `high`, `urgent`; statuses are `todo`, `in-progress`, `blocked`,
`done`, `canceled`. Duplicate labels remain allowed. Only `done` satisfies a dependency.
Explicit changes on blocked tickets and reopening are allowed. Nothing cascades.
General relationship and custom-field update flags are deferred.

## Priority and replacement updates

`update --priority` accepts `low`, `normal`, `high`, or `urgent`. An omitted source
priority has the fixed effective value `normal`; setting it to normal is a no-op
and leaves the field omitted, even alongside other changes. Setting a different
priority inserts the field. Invalid priority values fail candidate validation
(exit 1, `INVALID_TICKET`) without publishing any part of a combined update.
Repeated scalar flags such as `--priority` are usage errors (exit 2).

Repeated `update --label value` **replaces the entire label list** with the values
in argument order, including duplicates. An identical ordered list is a no-op;
reordering or changing duplicate counts is a source change even though summaries
sort labels. `update --no-labels` clears the list. Clearing absent or already-empty
labels is a no-op; clearing nonempty labels writes `labels: []`.

The label modes are mutually exclusive:

| Mode | Flags | Conflicts |
| --- | --- | --- |
| Replacement | One or more `--label` | `--no-labels`, any add/remove flag |
| Clear | `--no-labels` | `--label`, any add/remove flag |
| Incremental | Repeated `--add-label` and/or `--remove-label` | Replacement/clear; adding and removing the same value |

Conflicts are `USAGE` regardless of flag order or the current labels. Empty or
non-UTF-8 label arguments are also usage errors. Priority, title, and status can
combine with any one label mode in one validated single-ticket mutation. All
updates preserve exact bodies, unrelated YAML semantics, omitted fields, and
permission bits; no-ops preserve source bytes and file identity. Project creation
defaults never influence updates or change existing tickets.

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
status, priority, or other metadata edits. It supports replacement, clearing, and
incremental label modes with the same conflicts described above. Recursive
replacement/clearing changes each target's entire label list; use add/remove to
retain unrelated labels. Relationship and custom-field update flags remain
unsupported.

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
