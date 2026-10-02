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

YAML updates clone nodes, detach aliases of edited scalars to retain their old
meaning, reparse output, and compare all unrelated semantic values and exact body
bytes. Unknown custom tags and recursive aliases are retained without expansion.
Frontmatter comments/spacing are not byte guarantees. Unsupported preservation
fails unchanged with PRESERVATION_UNSUPPORTED.
