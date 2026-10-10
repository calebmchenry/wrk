# Storage contract

Supported environments are macOS and Linux local filesystems with advisory flock,
hard links, atomic same-directory rename, and directory sync. Unsupported primitives
fail without a weaker fallback. Ticket operations require no network. Explicit
`upgrade` commands use HTTPS and a separate installation lock; see the
[upgrade contract](cli.md#version-and-upgrade).

Writers hold the persistent `.wrk/.lock` inode exclusively and nonblocking. BUSY
means another writer holds it. The file is never removed on release; process exit
releases the OS lock. Symlinks and nonregular lock files are refused. Read commands
create no lock or other files.

After locking, load and validate all inputs. Stage complete candidate bytes in an
exclusive same-directory `.wrk-stage-*` file, sync and close, compare the config,
candidate inventory, identities and bytes again, and then publish. Creation uses
an atomic no-replace hard link. Updates use atomic rename and preserve permission
bits. New files honor umask. Flush the directory after publication.

CLI and browser writers share the same lock and publication path. External changes visible at the final comparison
are rejected with CONFLICT, including changes to unrelated validation inputs.
**The comparison and replacement are separate operations. An editor or Git change
between them may be lost or invalidate the candidate.** Direct body/config edits
and Git operations must occur outside a CLI mutation. This accepted boundary is
not filesystem compare-and-swap or protection against hostile ancestor changes.
The same external-editor boundary applies during browser mutations: keep direct
body/config edits and Git operations between all saves. An idle server or a read
poll holds no writer lock and does not prevent those external operations.

Pre-publication failures leave preexisting ticket/config data unchanged. Successful
link/rename is the publication point: later errors return affected ID/path and
`publication: "committed"` with `ok: false`. Never blindly retry or roll back;
inspect the affected ticket. Directory-sync errors mean durability is uncertain;
cleanup errors can leave operation-owned staging names. An undeliverable stdout
or killed process cannot guarantee an envelope, so inspect before retrying.

Ignore `**/.wrk/.lock` and `**/.wrk/.wrk-stage-*` in Git (plus `/bin/` here).
Interrupted operations may leave staging files. Reads ignore them and never clean
another operation's artifacts. A leftover unlocked `.lock` is harmless. Remove
only known abandoned staging files while no CLI writer is active.

Initialization exclusively creates `.wrk`, publishes config without replacement,
and refuses any existing `.wrk` entry. Handled failures clean only files owned by
that operation and remove its directory only if empty; never recursively delete.
After interrupted init, inspect the incomplete directory and explicitly resolve it;
`init` never adopts existing data.

YAML updates clone nodes and replace mapping edges without changing old value
nodes. Before encoding, traverse the reachable graph once, relocate definitions
whose original occurrence was removed, and generate unique anchors for shared or
recursive values. This retains aliases of edited scalars, sequences, mappings,
and custom values without expansion, including aliases used as mapping keys.
Independent custom-field inputs cannot collide with existing anchor names.
Reparse output and compare requested values, every unrelated value, and exact
body bytes (the existing body when omitted, or the explicit UTF-8 replacement).
Creation also verifies that encoding preserves the supplied values.
Unknown custom tags and exact numeric scalar text survive; comments, spacing,
and anchor names are not byte guarantees. Unsupported preservation fails unchanged
with PRESERVATION_UNSUPPORTED.

## Item revisions and stale edits

`ticket.Revision(source)` returns an opaque token for the exact file bytes read,
including frontmatter delimiters, comments, formatting, custom values, and body.
The current encoding is `sha256:` plus the lowercase hexadecimal SHA-256 digest;
callers should round-trip the token without interpreting it. `Snapshot.Summary`
includes this revision, so CLI list/show/mutation summaries and HTTP reads
use the same definition. Compute it from the source in the read snapshot, never
from a separately reread file. No revision is stored on disk.

The revision excludes project config, unrelated tickets, derived children/blockers,
file mode, inode identity, and timestamps. An unrelated edit or a same-byte atomic
replacement made before a request does not invalidate the draft. Restoring the
exact original bytes restores its revision: this is a content precondition, not
an event counter or a record of intervening edits. Config and whole-project
relationships are still loaded and validated for every write, including no-ops.

`store.UpdateWithOptions` accepts `ExpectedRevision *string` for a single item.
Nil leaves the existing unconditional update behavior; a supplied empty string
does not match. After acquiring the writer lock and loading current inputs, check
the original draft's revision before candidate creation, staging, or no-op return.
A changed or unreadable target returns `CONFLICT`; an absent target returns
`NOT_FOUND`. These checks precede general project diagnostics so a deleted target
with dangling references or a malformed external rewrite still reports a
recognizable stale-edit failure. A matching revision never bypasses project or
candidate validation. Recursive label updates reject a revision precondition;
one root token cannot protect every selected descendant.

On conflict, retain the caller's draft, reload current data, review the differences,
and explicitly retry using the newly read revision. Never silently replace the
draft's base token on refresh or automatically retry a stale save. A request that
already matches the current values still rejects an obsolete revision. Successful
no-ops keep the same source, inode, and revision. A committed mutation result
describes its published bytes, including when later sync/cleanup reports an error;
inspect/resynchronize before retrying an ambiguous result.

After the precondition passes, the existing final snapshot comparison still
checks **all** validation inputs, including bytes, inventory, identities, and
modes, before publication. A change during the mutation is rejected even if it
is unrelated to the draft. Revisions protect the time before the request but do
not close the accepted external-editor/Git comparison/rename race documented
above. Reads remain lock-free snapshots and are not multi-file transactions.


## Recursive label publication

A recursive label update holds one exclusive project lock for the entire batch,
including staging, publication, and cleanup. After loading/validating the whole
project, select the root plus all parent descendants regardless of status and sort
by ID. Compute and validate every candidate before staging anything; stage, sync,
and close all changed files before the first rename. No-ops retain their original
bytes and inode. A validation, preservation, or staging failure commits nothing.

Publish each changed file with the ordinary atomic rename and directory-sync
protocol. Before **each** rename, compare all config/ticket validation inputs,
including inventory, bytes, inode identity, and mode. After a successful rename,
advance the expected snapshot for only that file using its staged bytes and
identity captured before publication. Never reload the project between writes:
that would silently accept external changes. A detected change to a previously
committed ticket, a pending/no-op ticket, or any unrelated input stops the batch.
The accepted external-editor comparison/rename race still applies to each file.

This is per-ticket atomicity, not a multi-file transaction. Readers do not take
locks and can observe a partially applied label batch. On the first handled error,
stop and clean only operation-owned staging files. Before the first commit,
preexisting data is unchanged and the error result is null. After any commit,
report the selected batch with per-ticket committed/unchanged/pending states and
an overall committed marker, even if the root itself was not published. Never
roll back committed files. Directory-sync failures retain the committed state and
report uncertain durability. Cleanup failures identify remaining staging names.

After an error, interrupt, killed process, or failed output, inspect the root and
all descendants (`show <root>` and `list --all --under <root>`). Read ticket source
as needed, resolve the reported cause, then rerun the same label operation.
All label modes (replacement, clear, add/remove) are idempotent: already-applied
changes become no-ops and pending changes
can finish. A retry selects a **fresh** descendant snapshot, so it also includes
children added since the original invocation. Confirm the scope if relationships
changed. No automatic inheritance, transaction journal, or rollback is provided.
Abandoned stages are ignored and only removed manually when no CLI writer is
active; an interrupted process releases its OS lock.

## Related-link publication

Each unordered pair has one stored edge on the endpoint that first added it.
Creation writes only the new item. Updates plan ordinary changes on the addressed
item plus removal of incoming edges on their owner files. Reverse adds are no-ops;
we never copy or move an existing edge to another owner. Every individual candidate
and every published prefix is a valid graph. The store validates all candidates
and stages all changed files before publishing any, using the same lock,
preservation checks, comparison, sorted ID order, cleanup, and per-file result
states as recursive labeling. The addressed item is included even if unchanged.
Only its requested ordinary fields may change; other owners receive link removals.

This is **not** a multi-file transaction. Before publication a failure changes no
ticket data. After a successful rename, a later failure reports overall
`publication: "committed"` and entries marked `committed`, `unchanged`, or `pending`.
A target can be pending/unchanged while some owner removals have committed. Reads
can observe a valid partial link set. Do not roll back or blindly retry. Inspect
all entries, resynchronize, resolve the failure, and explicitly retry the intended
operation. A CLI retry finds the remaining owners under a fresh lock; already
removed edges become no-ops. Retrying clear also clears links newly added since
the first request. Browser retries require explicit review and new preconditions,
and reconcile individual add/remove intent against the reviewed set.

`related_revision` is a separate opaque content token for the sorted reciprocal
ID set and addressed ID, derived from the same snapshot as `revision`. It excludes
owner orientation, other items' bodies/metadata, timestamps, and config. An
incoming add/remove changes it even when the addressed file's source revision
is identical. Restoring the same link set restores the token. The store's optional
`ExpectedRelatedRevision` checks this token under the writer lock, before staging
or a no-op; stale sets return `CONFLICT`. Source preconditions retain their existing
behavior. Invalid project data is still rejected, and final comparison checks all
validation inputs before each publication. Neither token bypasses preservation,
validation, or the accepted external-editor race boundary. Recursive label updates
reject either precondition. HTTP link edits require both tokens; the browser sends
both for every edit and never advances either while polling.

## Bounded browser mutations

The HTTP service calls `store.CreateContext` / `store.UpdateContext`, retaining the
same locking, candidate validation, preservation, and publication path as the CLI.
The request context and read limits apply inside the writer lock (and creation
collision reloads), not only to a preliminary read. Snapshots retain those limits
for candidate file/total byte checks and final comparison. Comparison reads need
only the original byte length plus one; changed size is a conflict. Cancellation
or resource-limit failures during initial loading are reported before revision
checks so a partially loaded snapshot cannot masquerade as a deleted target.
Ordinary CLI calls retain unlimited loading. These bounds do not make individual
YAML/graph operations or filesystem syscalls preemptible, or change the accepted
external-editor publication boundary.
