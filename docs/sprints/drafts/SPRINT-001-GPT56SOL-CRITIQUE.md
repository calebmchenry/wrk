# Combined Critique: Sprint 001 Drafts

## Grounding and resolved decisions

The merge should treat `wrk-682f60c7` as the scope authority and `docs/ticket-format.md` plus `docs/configuration.md` as the correctness contract. The broader metadata CRUD in `wrk-3f8a21b7` remains deferred. Four questions left open in one or both drafts are now settled: implementation is Go on macOS and Linux; CLI writers are serialized and stale snapshots are rejected, while the final comparison-to-replacement race with a non-cooperating editor is accepted and documented; creation includes `--parent`, `--priority`, repeatable `--label`, and `--no-labels`; and every ordinary command fails when any existing project ticket is invalid, while `validate` aggregates all determinable diagnostics. The final sprint must state these as requirements, not open questions.

## Review of the GPT6Astra draft

### Strengths

This is the stronger correctness design. Its raw-file, body-byte, and YAML-node representations cleanly separate three obligations that are easy to conflate: stale detection, exact body preservation, and semantic metadata preservation. It correctly avoids converting arbitrary unconfigured `fields` values through JSON-shaped Go values, validates the parent and dependency graphs independently, preserves old-prefix IDs, and proposes deterministic tests for collisions and concurrent changes rather than timing-based tests.

The storage section directly addresses the hardest requirements. A persistent `.wrk/.lock` inode that is never unlinked avoids the classic split-lock race in which two processes hold locks on different inodes. Holding that lock across the authoritative snapshot, validation, staging, and publication is the right writer model. Staging in `.wrk` and publishing a new ticket with a no-replace primitive avoids both overwrite and cross-filesystem rename hazards. The draft also distinguishes pre-publication rejection from a failure after publication, when rollback would itself be unsafe. Its explicit `published: true` concept and “inspect before retrying” guidance are important for `new`, where a blind retry could create a duplicate ticket.

The verification matrix is unusually complete. It covers same-size edits, atomic editor replacement, destination races, canceled blockers, valid cross-graph relationships, CRLF and missing final newlines, and actual Linux filesystem behavior. The Definition of Done includes the ticket workflow, compatibility, concurrency limitations, JSON determinism, documentation, and live dogfooding without closing the parent.

### Weaknesses and scope creep

The draft frequently turns prudent implementation choices into a larger product surface. A six-category exit-code taxonomy, a versioned JSON protocol with detailed error records, two new documentation files, symlink policy, parser resource limits, anchor materialization, hosted CI, multiple ignore files, and a bounded repair mode collectively risk making the first runnable CLI a storage-platform project. JSON and error behavior must be documented, but a compact stable envelope and a few exit classes are enough for this milestone. CI is useful only if an actual remote is available; the requirement is recorded macOS/Linux execution, not creation of repository infrastructure.

The proposed “bounded repair” exception conflicts with the resolved strict-invalid-project policy. `update` is an ordinary command and must not become a repair path for an invalid target. Recovery remains direct restoration from Git or another known-good copy, followed by `validate`; documentation must never recommend manual frontmatter mutation. Removing repair also simplifies the validator and prevents ambiguity about which invalid states are repairable.

The package plan is plausible but slightly overcommitted for a greenfield repository. The final sprint should describe responsibilities and likely files without requiring every proposed file or package. `internal/project` and `internal/ticket` could otherwise develop circular ownership around parsing, graph construction, and candidate validation. A minimal implementation can begin with `internal/ticket`, `internal/project`, `internal/store`, and `internal/cli`, splitting further only when tests expose a real boundary.

### Gaps in risk analysis and edge cases

The persistent lock creates an unresolved repository side effect. If `.wrk/.lock` is created by the first mutation, an invalid mutation can alter the directory even though failed validation promises no changes; if it is deleted on release, the inode guarantee is broken. The merged plan must choose whether the zero-byte lock is a tracked/pre-provisioned sentinel or an explicitly permitted ignored runtime artifact, and test that repeated acquisitions use the same inode. Existing projects lacking the sentinel need a race-safe one-time creation rule.

The YAML discussion is sophisticated but assumes node re-emission plus semantic comparison is feasible for the entire accepted YAML domain. Anchors, aliases, explicit tags, numeric spellings, merge keys, comments, and flow-style mappings can expose differences between “same parsed value” and “preserved custom value.” Materializing aliases is especially risky scope. The plan needs an early spike comparing two alternatives: node-based re-encoding with tag-aware equivalence, and span-based replacement of only `title`/`status` scalars. The latter preserves unrelated bytes but is harder for flow mappings and aliases. The sprint should select the smallest design that passes representative fixtures and fail closed for demonstrably unsupported constructs rather than silently narrowing the format.

The `source`-based JSON design is a good way to avoid lossy custom-value projection, but it should be stated as an intentional v1 contract: normalized built-ins and derived relationships are projected; the exact ticket source carries arbitrary YAML custom values. Generic YAML-to-JSON conversion must not be introduced accidentally. Also specify YAML scalar resolution used for configured `number` and `boolean` checks.

Finally, whole-project stale rechecks are conservative but potentially complex and linear for every write. That is acceptable now, but the accepted editor race applies to any non-cooperating change after the final check, not only the target file, and documentation/tests should say so. Post-link cleanup failure, directory-sync failure, and process death after publication all need observable tests where the final path contains a complete file and no retry is automatic.

## Review of the GPT-5.5 draft

### Strengths

This draft is easier to execute. Its phases follow a sensible dependency order, its file summary is restrained, and it keeps databases, services, editor integration, and general metadata updates out of scope. It correctly calls for node-level duplicate-key detection, exact body-byte tests, independent graph validation, copied compatibility fixtures, deterministic conflict injection, and subprocess workflow coverage. Its Definition of Done maps well to most of the milestone ticket and correctly keeps the parent open.

The architecture also has understandable ownership boundaries and recognizes that atomic rename alone does not close the editor race. It explicitly retains unrelated metadata and unconfigured custom fields, includes combined title/status updates, and prohibits live fixture mutation during tests.

### Weaknesses and incorrect assumptions

It predates three resolved decisions in material ways. Creation implements only `--parent`, omitting `--priority`, repeatable `--label`, and `--no-labels`; those flags must be present in use cases, parsing rules, implementation, tests, and Definition of Done. Its risk section permits `list` and `show` to surface errors while rendering valid data, which is incompatible with strict failure for all ordinary commands. Its open questions also incorrectly leave Go/platform, parenting, and invalid-project policy unsettled.

The writer protocol is incomplete. It reads and validates before acquiring the lock, then rereads only the target. A config, ticket set, or related ticket can change between validation and publication, invalidating the candidate project. The authoritative full snapshot must be loaded and validated under the project lock, or fully reloaded and revalidated after acquiring it. The draft does not require a persistent lock inode or prohibit unlink-on-release, leaving a split-brain locking hazard.

“Exclusive create semantics” on the final ticket path is not sufficient. Opening the destination with `O_EXCL` and then writing exposes a partial ticket if the process crashes. Creation needs a complete, synced staging file followed by same-directory no-replace publication (for example, a hard link on the supported local filesystems), then directory sync. Update can use atomic replacement because overwriting the validated target is intended. These two publication paths should not be conflated.

### Missing risks, edge cases, and Definition of Done items

The draft does not describe post-publication errors, cleanup failures, or retry ambiguity. It also lacks tests for lock contention, same-size/same-timestamp edits, changes to config or unrelated tickets, final-path collision after staging, process interruption, and the accepted edit after the last comparison. Its YAML plan says to preserve values but does not define whether preservation is byte, node, tag, or Go-value based; nor does it prevent lossy JSON projection of nested or tagged custom values.

The DoD should explicitly require the persistent-lock invariant, no partial final ticket during creation, exact creation-flag semantics (labels replace configured defaults; `--no-labels` conflicts with `--label`), actual macOS and Linux storage tests, post-publication uncertainty reporting, and the ban on manual live ticket metadata edits. The proposed status/priority/ID list sort also adds an unneeded policy; lexicographic ID order is simpler unless the product contract deliberately chooses another ordering.

## Comparison and actionable merge recommendations

Use the GPT-5.5 draft's concise phase structure and the GPT6Astra draft's storage, preservation, and verification model. Merge them with these changes:

1. Convert all resolved decisions into non-negotiable acceptance criteria and delete the corresponding open questions. Remove bounded repair and partial read behavior.
2. Put storage feasibility first: prove persistent-inode advisory locking on macOS/Linux, reload and validate the complete project under lock, stage complete files in `.wrk`, use no-replace publication for `new`, and atomic replacement for `update`. Define lock-sentinel creation without weakening failed-write guarantees.
3. Add a focused YAML spike before command implementation. Preserve body bytes exactly; preserve unrelated metadata semantically including YAML tags and nested custom values; compare candidate metadata before publication. Keep arbitrary custom values out of normalized JSON and expose exact source instead.
4. Define publication states: failures before publication change no existing ticket; after publication, report the affected ID/path and `published: true` when possible, never roll back blindly, and require inspection before retry. Test partial staging, sync errors, cleanup errors, and crashes on both sides of publication.
5. Include all four creation extensions and their conflict/default semantics, while keeping update limited to title/status. Follow-up tickets must be created through the finished CLI; no plan or recovery step may manually mutate live ticket frontmatter.
6. Keep the file layout provisional and small. Require concrete Go files for the executable, domain/project logic, store with Unix-specific lock code, CLI/output, unit tests, and subprocess tests; avoid mandatory CI, extra docs, or package splits unless they directly discharge a ticket criterion.
7. Make the DoD evidence-based: exact commands and results for build, test, race test, vet, clean-source run, macOS/Linux filesystem behavior, JSON output, strict invalid-project failures, no-clobber creation, stale rejection, accepted final editor race documentation, live validation, and dogfooding with the parent still open.

This merge keeps the smaller milestone executable while retaining the safety properties that make dogfooding on the repository's real, manually readable ticket files responsible.
