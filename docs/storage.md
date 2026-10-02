# Storage contract

Supported environments are macOS and Linux local filesystems with advisory flock,
hard links, atomic same-directory rename, and directory sync. Unsupported primitives
fail without a weaker fallback. Runtime use requires no network.

Writers hold the persistent `.wrk/.lock` inode exclusively and nonblocking. BUSY
means another writer holds it. The file is never removed on release; process exit
releases the OS lock. Symlinks and nonregular lock files are refused. Read commands
create no lock or other files.

After locking, load and validate all inputs. Stage complete candidate bytes in an
exclusive same-directory `.wrk-stage-*` file, sync and close, compare the config,
candidate inventory, identities and bytes again, and then publish. Creation uses
an atomic no-replace hard link. Updates use atomic rename and preserve permission
bits. New files honor umask. Flush the directory after publication.

CLI writers are serialized. External changes visible at the final comparison
are rejected with CONFLICT, including changes to unrelated validation inputs.
**The comparison and replacement are separate operations. An editor or Git change
between them may be lost or invalidate the candidate.** Direct body/config edits
and Git operations must occur outside a CLI mutation. This accepted boundary is
not filesystem compare-and-swap or protection against hostile ancestor changes.

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

YAML updates clone nodes, detach aliases of edited scalars, label sequences, and
label elements to retain their old
meaning, reparse output, and compare all unrelated semantic values and exact body
bytes. Unknown custom tags and recursive aliases are retained without expansion.
Frontmatter comments/spacing are not byte guarantees. Unsupported preservation
fails unchanged with PRESERVATION_UNSUPPORTED.


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
