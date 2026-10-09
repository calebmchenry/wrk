# CLI contract

## Commands

| Command | Behavior |
| --- | --- |
| `wrk`, `wrk --help`, `wrk help [command]` | Help, exit 0, no project discovery. |
| `wrk version`, `wrk --version` | Version, commit, build kind, and platform; offline, no project discovery. |
| `wrk upgrade --check` | Check the latest stable release without local writes. |
| `wrk upgrade` | Verify and atomically install a newer standalone release binary. |
| `wrk init [directory]` | Existing target directory, default cwd. Exclusively create `.wrk` and the [default config](configuration.md). Refuse any existing entry. |
| `wrk serve [--port 0..65535] [--open]` | Serve one existing project on `127.0.0.1`, default port 7331. Bundled page and strict read APIs; Ctrl-C stops it. |
| `wrk new "Title"` | Create a `todo` ticket with configured prefix, priority, and labels; print ID/path. |
| `wrk new "Title" --body-file path` | Exact UTF-8 body bytes; `-` reads stdin through EOF before locking. |
| `wrk new "Title" --parent id` | Create a child of an existing project ticket. |
| `wrk new "Title" --depends-on id --field estimate=3` | Repeatable prerequisites and typed custom values at creation. |
| `wrk new "Title" --priority high --label cli --label storage` | Explicit overrides; supplied labels replace the entire configured default list. |
| `wrk new "Title" --no-labels` | Explicit empty labels; conflicts with `--label`. |
| `wrk list` | `todo`, `in-progress`, and manually `blocked` tickets, including unfinished dependency blockers. |
| `wrk list --all` | All five statuses. |
| `wrk list --ready` | `todo` with all dependencies `done`. |
| `wrk list --label burn --label backend` | Require every supplied label (AND); exact, case-sensitive matching. |
| `wrk list --under id` | Descendants at every depth, excluding the root. Intersects label and status/readiness filters. |
| `wrk show id` | Full source plus derived children and unfinished dependency blockers. |
| `wrk update id --title "Title" --status in-progress` | Either or both flags, one validated mutation. No-op reports `changed: false`, without rewriting. |
| `wrk update id --body-file path` | Replace the exact UTF-8 body; `-` reads stdin through EOF before locking. Empty input clears it. |
| `wrk update id --add-label burn --remove-label triage` | Repeatable, idempotent label edits; may combine with title/status/priority. |
| `wrk update id --priority high --label cli --label storage` | Set priority and replace the entire label list. |
| `wrk update id --no-labels` | Clear labels; absent/already-empty labels are a no-op. |
| `wrk update id --parent other-id` | Set or change a parent; `--no-parent` removes it. |
| `wrk update id --add-dependency other-id --remove-dependency old-id` | Repeatable, idempotent dependency edits. |
| `wrk update id --field estimate=5 --remove-field obsolete` | Set typed YAML values or remove custom fields by name. |
| `wrk update id --add-label burn --recursive` | Labels only; root plus all descendants, regardless of status. Per-ticket publication results. |
| `wrk validate` | Aggregate determinable errors without writes. |

Every command accepts `--json` and `--help`. Flags may precede or follow positional
arguments after the command; `--json`, `--help`, and the project selectors below
also precede the command. Support
`--flag=value` and `--` to end option parsing. A value beginning with `--` must use
`--flag=value`. Unknown/repeated scalar flags, surplus arguments, empty updates,
`list --all --ready`, and `--label ... --no-labels` on new/update are usage errors.

## Project selection

`new`, `list`, `show`, `update`, `validate`, and `serve` accept either `--project directory`
or `--config directory/.wrk/config.yaml`, before or after the command:

```sh
wrk --project '/path/to/my project' list --ready --json
wrk show <id> --config '/path/to/my project/.wrk/config.yaml'
wrk --project ../other-project update <id> --body-file description.md
```

An explicit selector overrides discovery from cwd. `--project` names the exact
directory containing `.wrk`, not a descendant from which to search upward.
`--config` names that project's existing `.wrk/config.yaml` and selects all its
tickets and settings. Alternate config names, separate data/config layouts, and
applying another project's settings to cwd are unsupported. Missing or invalid
targets fail without fallback or initialization.

Relative selectors and relative `--body-file` paths both resolve against the
invocation directory. Selection never changes the process working directory.
Paths are made absolute and lexically cleaned (`.`/`..` collapsed); directory
symlinks are followed by filesystem access but are not expanded in the reported
root. The result is not a physical canonical path: two directory aliases may
report different roots for the same project. `.wrk` itself must be a real
directory, and config/ticket/lock files must be regular files, never symlinks.
Do not change directory aliases during a command; the existing
[external-editor boundary](storage.md) still applies.

Selectors are scalar and mutually exclusive, including when they name the same
project. Repeated selectors, empty/missing values, conflicts, and selectors on
`init`, `help`, `version`/`--version`, or `upgrade` are `USAGE` errors (exit 2).
`init` retains its positional target directory. Help for a supported data command
(such as `list --project missing --help`) returns without resolving the selector.
Help/version/upgrade otherwise remain independent of project discovery.

JSON `project_root` reports the selected absolute root, including failures once
that root is identified; usage errors and unsupported config path shapes leave it
null. Human output and project-relative ticket/diagnostic paths retain their
existing format. Invalid selected roots/boundaries report `INVALID_PROJECT`;
config shape/content errors report `INVALID_CONFIG` (exit 1).

## Local server

```sh
wrk serve                               # http://127.0.0.1:7331/
wrk serve --port 0 --open                # OS-allocated port; opt-in browser launch
wrk --project '/path/to/my project' serve --port 8080
wrk serve --config '/path/to/my project/.wrk/config.yaml' --json
```

`serve` resolves an existing project once, validates it before listening, and
keeps that absolute root for its lifetime. The ordinary selection/discovery and
symlink rules apply. A new nearer `.wrk` directory cannot change the selection.
No initialization, database, cache, writer lock, or per-request subprocess is used.
CLI and agent writes remain available while the server runs.

Bind only IPv4 `127.0.0.1`. Port defaults to 7331; `--port` accepts decimal integers
0 through 65535. Zero requests an OS-assigned port; every other value is used
exactly. Invalid values are usage errors (exit 2); occupied/unavailable ports
produce `LISTEN` (exit 1), with no automatic fallback. The actual URL and absolute
project root are printed after a successful listen.

The executable embeds the HTML, CSS, and JavaScript; it needs no CDN, Node,
frontend server, external fonts, or separate asset installation. The current
shell identifies the project, shows its validated item count, and offers manual
reload with diagnostics. Full item browsing, automatic live refresh, and editing
belong to the following web tickets and are not implemented yet.

`--open` runs `open <url>` on macOS or `xdg-open <url>` on Linux, after listening,
without a shell. It is opt-in and has a three-second timeout. A launch failure
prints `OPEN_BROWSER` with the URL and leaves the server running. Open that URL
manually. The launcher returning successfully does not prove a browser displayed it.

Ctrl-C and SIGTERM cancel handlers and stop accepting connections. Shutdown allows
up to three seconds to drain and then closes remaining connections. A normal stop
exits 0; a listen/server/shutdown/output error exits 1. SIGKILL or failed output
cannot guarantee a terminal event. No stream endpoint is implemented yet; future
streams must observe the request context and avoid hijacking connections.

### Serve output

Human mode writes root/URL/start instructions and the final stop message to
stdout, with diagnostics and browser-launch warnings on stderr. Terminal escaping
is unchanged. HTTP access logs are not mixed into command output.

`serve --json` is the long-running exception to the single-envelope CLI contract:
it emits newline-delimited schema-version-1 envelopes with `command: "serve"`,
the selected `project_root`, `ok`, `result`, and `errors`. Each line is complete
and immediately written; do not wait for process exit to read startup information.

| Phase | Envelope |
| --- | --- |
| Usage, resolution, validation, or listen failure | One ordinary failure envelope, `ok: false`, `result: null`; no started event. |
| Listening | `ok: true`, `result: {"event":"started","url":"http://127.0.0.1:<port>/"}`, empty errors. |
| Browser launch fails | Nonfatal `ok: true`, `result` has `event: "warning"`, `url`, and a diagnostic-shaped `warning` with code `OPEN_BROWSER`; empty errors. |
| Normal shutdown | `ok: true`, `result` has `event: "stopped"` and `url`; empty errors. |
| Server/shutdown failure after startup | `ok: false`, `result` has `event: "error"` and `url`; errors include `SERVE`; exit 1. |

JSON mode emits no routine stderr output. `serve --help --json` remains an
ordinary single successful help envelope without resolving a project.

### Read API

Only the printed authority `127.0.0.1:<actual-port>` is accepted. `localhost`,
alternate IP spellings, other ports/hosts, proxy-form requests, foreign/null
Origins, and cross-site or same-site Fetch Metadata requests receive 403.
Browser requests must be same-origin; direct navigation (`Sec-Fetch-Site: none`)
and local clients without Origin/Fetch Metadata are supported for reads.
No CORS permissions are returned. Security rejections omit the project root.
Loopback is a local-user trust boundary, not authentication against local software.

All routes accept GET/HEAD only. Read requests must have no body. The common guard
already requires an exact same-origin `Origin` and `Content-Type: application/json`
for unsafe methods; passing it currently returns 405. Future mutation endpoints
must retain this guard and body limit, validate JSON, and call shared store services
with the expected item revision. No cross-origin or missing-Origin write exception
is allowed. OPTIONS/preflight is not enabled.

| Route | Successful `result` |
| --- | --- |
| `/api/project` | `ticket_count`, `config` (`version`, `prefix`, `defaults.priority`, `defaults.labels`, `fields` definitions), ordered `statuses` and `priorities`. Each field definition has `type`, `description`, `options`. |
| `/api/items` | `tickets`, using the CLI summary contract including source revision, parent, dependencies, labels, effective priority, and derived blockers. Defaults to active items. |
| `/api/items/<id>` | `ticket` summary, exact UTF-8 `body` and full `source`, and direct `children` summaries. |

List parameters are `all=true|false`, `ready=true|false`, repeatable nonempty
`label`, and scalar `under=<id>`. They use the CLI's intersections, readiness,
and descendant semantics. All/ready cannot both be true. Unknown parameters,
repeated scalars, malformed encoding, and invalid values return 400; missing
items/scope roots return 404. IDs must match the existing ID grammar. Roots,
config paths, and arbitrary file paths cannot be supplied in requests.
Project and item routes accept no query parameters.

HTTP JSON uses `{schema_version: 1, ok, project_root, result, errors}`. Successful
errors arrays are empty; failed results are null with actionable shared diagnostics
(code/message and path/field/line/IDs when available). Each read freshly loads and
validates the whole project. Any invalid file/config/reference makes the read fail
with 503, even when the requested item is valid. No partial success or stale healthy
snapshot is returned. After external repair, another request/reload can succeed.
Reads are independent filesystem snapshots, with the same external-editor/Git
boundary as CLI reads; multiple requests are not a transaction. Item revisions
cover exact source bytes as described in [storage](storage.md#item-revisions-and-stale-edits).
Arbitrary custom YAML values remain in `source`, without lossy JSON coercion.

Only `/`, `/app.js`, and `/style.css` serve assets, from the embedded filesystem.
No directory listing, project-file serving, clean-path redirects, or SPA fallback
is provided. Assets and APIs use `Cache-Control: no-store`, `nosniff`, no-referrer,
frame denial, and a self-only Content Security Policy without inline script access.
Project content is inserted as text, never executable HTML.

### Resource limits and recovery

The HTTP server sets a 16 KiB header limit, five-second header timeout,
15-second request-read timeout, 20-second response-write timeout, and 30-second
idle timeout. URLs are limited to 4096 bytes (414); bodies over 1 MiB receive 413.
At most four project reads run concurrently; excess reads receive 503 `BUSY`
with `Retry-After: 1`.

Startup and each project read have a 15-second context deadline, at most 2 MiB per
config/ticket file, 32 MiB total input, and 20,000 direct `.wrk` directory entries
(including ignored entries). Hitting a project read limit produces `RESOURCE_LIMIT`
and no healthy partial result. Use the CLI for larger projects or reduce input sizes.
These are server resource limits, not changes to the file format or ordinary CLI.
Cancellation is checked during reads, inventory scanning, and between validation
phases; individual YAML parsing/graph operations and filesystem syscalls are not
preemptible. Avoid unstable directory aliases or nonlocal filesystems.

For runtime diagnostics, inspect the named file and `wrk validate`; restore a
known-good file or obtain authorized repair, then reload. For 403, use the exact
printed URL. For `LISTEN`, stop the conflicting process or choose another port.
Automatic recovery/reconnect display will be expanded with live updates.

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

All operational failures exit 1 and usage errors exit 2. Version/upgrade JSON
remains a single envelope on stdout with no routine stderr messages.

Titles are nonblank single-line strings; supplied spacing is retained. Priorities
are `low`, `normal`, `high`, `urgent`; statuses are `todo`, `in-progress`, `blocked`,
`done`, `canceled`. Duplicate labels remain allowed. Only `done` satisfies a dependency.
Explicit changes on blocked tickets and reopening are allowed. Nothing cascades.
Relationship and custom-field edits can combine with other single-ticket metadata changes.

## Body updates and revisions

`update --body-file path|-` replaces everything after the closing frontmatter
delimiter line ending. Omission preserves the existing body byte-for-byte; an
empty file or empty stdin clears it. LF/CRLF, blank lines, Unicode, and a missing
final newline are retained exactly. The input is Markdown body content, not a
complete ticket file; it cannot replace frontmatter.

Read the entire input before project discovery or locking. Relative paths resolve
against the invocation directory, including when running from a nested directory
or selecting a different project with `--project` or `--config`.
Unreadable inputs return `IO`; invalid UTF-8 returns `INVALID_BODY` (exit 1), with
no mutation. The flag is scalar and may appear before or after the ticket ID.
Repeated/missing flags and combination with `--recursive` are usage errors.

Body replacement can combine with any supported single-item metadata changes in
one validated mutation. An identical body with otherwise unchanged values is a
no-op and retains source bytes and file identity. Frontmatter may be reformatted
on a real body edit, while unrelated YAML values and file permissions survive.

All JSON ticket summaries include an opaque `revision`, derived from the exact
source bytes. It is additive in JSON schema version 1 and is not stored in ticket
frontmatter. Consumers should retain the revision from the same read as their
draft. The shared Go store accepts a single-item expected revision for future
browser edits; this CLI batch does not expose a revision-precondition flag.
See [the revision contract](storage.md#item-revisions-and-stale-edits) for scope,
conflicts, reload/retry behavior, and remaining concurrency limits.

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
updates preserve exact bodies unless `--body-file` replaces them. Unrelated YAML
semantics, omitted fields, and permission bits are preserved; no-ops preserve
source bytes and file identity. Project creation defaults never influence
updates or change existing tickets.

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
status, priority, body, or other metadata edits. It supports replacement, clearing,
and incremental label modes with the same conflicts described above. Recursive
replacement/clearing changes each target's entire label list; use add/remove to
retain unrelated labels. Relationship and custom-field edits are single-ticket only.

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

## Relationship updates

`update --parent id` sets or changes the parent; `--no-parent` removes its key.
Setting the same parent or clearing an absent parent is a no-op. The two flags
conflict, and each may appear only once. Reparenting changes derived children and
`list --under` immediately; the ID and filename stay fixed.

`new --depends-on id` is repeatable. It preserves argument order and rejects
duplicate edges. On updates, `--add-dependency id` and `--remove-dependency id`
are repeatable and idempotent. Existing edges keep their order; new edges append
once in argument order. Removing an absent edge is a no-op, including a valid ID
that is not in the project. A removal argument must have ticket-ID syntax.
Removing the last edge leaves `depends_on: []`; no-op removal leaves an omitted
field omitted. Adding and removing the same ID is `USAGE` (exit 2), regardless
of argument order or current data.

All resulting parent/dependency references must exist in the project. Invalid
candidate values, missing references, self-references, duplicate edges, and cycles
fail validation (exit 1) without publishing any part of a combined update.
Parent and dependency cycles are checked separately: a parent may depend on its
children. Explicit status changes remain allowed despite unfinished dependencies;
reopening is allowed and changes never cascade. Only `done` satisfies an edge.

## Custom-field input and updates

`new` and `update` accept repeated `--field 'name=YAML'`; `update` also accepts
repeated `--remove-field name`. Set flags change only the named values; they do
not replace the entire `fields` map. Removal deletes the named key. Removing the
last value leaves `fields: {}`; removing absent names never inserts a map.

Each value is exactly one independent YAML document, parsed as nodes without
Go-value or JSON conversion. Nested collections, custom tags, recursive aliases,
and exact numeric scalar text are supported. Anchors are local to each argument;
an alias cannot refer to another argument or the existing ticket. Duplicate YAML
keys, malformed YAML, empty values, and extra documents are usage errors. Use
explicit `null` to store a null, or YAML `''` for an empty string. Shell quoting
protects the argument; inner YAML quotes determine its type:

```sh
wrk new "Estimate work" --field 'estimate=3.5' --field 'needs_review=true'
wrk update <id> --field 'customer="123"' --field 'area=cli'
wrk update <id> --field 'precise=!!int 123456789012345678901234567890'
wrk update <id> --field 'opaque=!tag {steps: [one, two]}' --remove-field obsolete
```

Field names are nonempty UTF-8 strings and remain literal: dots do not select
nested paths. The first unescaped `=` separates name and value. In the name only,
write `\=` for a literal equals sign and `\\` for a literal backslash, for example
`--field 'a\=b=3'` sets the name `a=b`. `--remove-field` takes the literal name
without this escaping, for example `--remove-field 'a=b'`.

Setting the same name more than once or setting and removing the same name is
`USAGE`, regardless of order. Repeated removals are harmless. Setting an identical
resolved YAML graph is a no-op; tags, exact scalar text, and collection order
matter, while comments, styles, and anchor names do not. Thus `1` and `1.0` are
separate source values, and quoting `"true"` selects a string instead of a boolean.
A no-op preserves all source bytes and file identity, even for omitted fields.

Configured string, number, boolean, and enum definitions are enforced without
coercion (exit 1, `INVALID_TICKET`). Unconfigured values may be arbitrary YAML,
including null. Definitions and all fields remain optional. Existing invalid
projects are rejected before mutation; removing a bad field is not a repair bypass.

Edits preserve exact bodies unless explicitly replaced. Permission bits, unrelated
built-ins, and every unrelated custom value are preserved. Aliases of
replaced/removed definitions retain their old values; anchors may be renamed or
relocated. The output is reparsed and checked
before publication. If unrelated values cannot be preserved (for example an
untouched alias of the entire changing frontmatter), the update fails unchanged
with `PRESERVATION_UNSUPPORTED`. See the [storage contract](storage.md).

## Discovery and validation

Without an explicit selector, walk upward to the nearest entry named `.wrk`.
A file, symlink, missing config,
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
`update`. This does not grant a manual-frontmatter editing exception.
Read commands create no locks, caches, or other files.

## Human and JSON output

Human lists show ID, status, priority, title, and blockers. `show` separates derived
information from source. Terminal controls are escaped in human output. JSON uses
standard escaping and retains source content exactly.

Except for [serve lifecycle events](#serve-output), JSON is exactly one object
plus newline on stdout, including ordinary failures;
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
      "revision": "<opaque source revision>",
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

`project_root` is absolute, or null before discovery/selection identifies a root.
It retains directory aliases as described in [project selection](#project-selection).
Project paths are relative,
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
| `serve` | Lifecycle `event` and `url`; see [serve output](#serve-output). |
| Help | `usage` string |

`show.source` includes every custom value and the body; no lossy custom YAML-to-JSON
projection is offered. Every ticket summary also includes `revision`, including
children and mutation results. Errors contain `code`, `message`, and applicable
`path`, `field`, `line`, `column`, and involved `ids`.

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
