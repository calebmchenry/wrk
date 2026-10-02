# Sprint 001 Intent: First Runnable wrk CLI

## Seed

Plan the initial CLI milestone in [wrk-682f60c7](../../../.wrk/wrk-682f60c7.md), requested with the `sprint-plan-codex-only` skill. Produce an implementation-ready sprint, not implementation code. The ticket's command contract and acceptance criteria are the scope authority; [ticket format](../../ticket-format.md) and [configuration](../../configuration.md) define correctness. The parent [wrk-3f8a21b7](../../../.wrk/wrk-3f8a21b7.md) retains broader metadata operations.

## Context

- This is a documentation-only repository: README, agent conventions, configuration, and three real tickets; no source code, build configuration, tests, remote, or commits exist yet.
- The completed format milestone established YAML frontmatter plus a directly editable UTF-8 Markdown body, local random IDs, separate parent/dependency graphs, and versioned project configuration.
- The first runnable milestone provides `init`, `new`, `list`, `show`, `update` (title/status), and `validate`; all are noninteractive and have readable plus JSON output.
- Metadata edits must preserve the body exactly and unrelated metadata/custom values, validate before writing, and avoid silently overwriting concurrent edits. The nearest `.wrk` is an authoritative boundary even when invalid.
- Agent documentation is maintained in `docs/` with navigation in `docs/index.md`; ticket bodies can record planning progress, but metadata changes must wait for the CLI.

## Recent Sprint Context

No existing sprints or git history. The current backlog records a completed format/bootstrap migration, a broad minimal-CLI parent, and this narrower runnable milestone. Existing files are untracked user work; do not replace them or initialize commits as part of planning.

## Relevant Codebase Areas

| Existing path | Role |
| --- | --- |
| `AGENTS.md`, `.wrk/AGENTS.md` | Agent rules and ticket editing constraints |
| `README.md`, `docs/index.md` | Local execution instructions and agent navigation |
| `docs/ticket-format.md` | Identity, fields, graphs, editing contract |
| `docs/configuration.md` | Discovery, version, prefix, defaults, custom schema |
| `.wrk/config.yaml`, `.wrk/wrk-*.md` | Real compatibility fixtures and eventual dogfooding data |

All implementation modules and test locations are new and depend on the chosen language. Propose concrete paths and a small architecture rather than assuming existing packages.

## Constraints

- Plan only; do not implement commands, edit ticket frontmatter, or mark work started/completed now.
- Keep the exact seed command surface, including combined title/status updates, `--body-file -`, `list --all`, `list --ready`, useful errors, help on bare invocation, and per-command `--json` results with IDs/paths for mutations.
- Do not pull the parent's general metadata CRUD, editor, UI, assignments, network integrations, or database into this milestone.
- Existing metadata and configurations must work without migration. Unconfigured custom values survive updates. Duplicate YAML keys, unknown top-level settings, unsupported versions, wrong types, filename/ID mismatch, and invalid graphs must be diagnosed.
- Only `done` satisfies a prerequisite; parent/dependency graphs are validated separately. Blockers do not prevent explicit status changes or cascade changes.
- Clarify storage safety honestly: atomic rename alone is not a compare-and-swap; CLI locks do not automatically coordinate editors. Define a realistic mechanism and deterministic verification for stale changes without overstating guarantees.
- The ticket requires follow-up tickets under the parent, but it defers `update --parent` and does not explicitly expose creation-time parent/default override flags. Resolve that tension with the smallest justified interface decision, or clearly identify it for interview.
- User confirmed Go for the implementation and macOS plus Linux for the first milestone. Windows support and public release packaging are deferred.

## Success Criteria

1. A fresh checkout can build/install or run the CLI through a short documented procedure and display help.
2. The full ticket workflow succeeds: create, discover/list/ready, show, start, directly edit description, finish, validate.
3. All preexisting tickets/config are understood without migration, including relationships and all deferred metadata fields.
4. Invalid input produces useful nonzero failures with unchanged existing files. Creation never clobbers a ticket, and stale updates are rejected to the agreed concurrency contract.
5. JSON results are deterministic, parseable, and documented; list/show expose children/blockers as appropriate, with no progress noise on stdout.
6. Automated tests exercise spec boundaries and a subprocess-level workflow; local install verification and eventual CLI dogfooding are explicit completion gates.
7. README and agent docs distinguish implemented and deferred operations; follow-up tickets capture remaining metadata scope, and the parent stays open.

## Verification Strategy

- Reference: no implementation exists; the seed ticket and format/config documents are normative.
- Use table-driven parser/config/graph tests, isolated filesystem integration tests, and CLI subprocess tests against disposable projects.
- Copy the real `.wrk` data into test fixtures without mutating the live backlog; validate compatibility and semantic preservation.
- Compare exact body bytes across updates (UTF-8, CRLF, missing terminal newline, delimiter-like body text). Compare unrelated metadata semantically, including nested unconfigured custom values and YAML scalar types.
- Test nearest-project boundaries, invalid inner config, prefix changes, explicit versus configured defaults, collisions/no-clobber, malformed YAML/duplicate keys, separate graph cycles, canceled blockers, and contradictory flags.
- Exercise deterministic conflict/failure injection instead of timing-dependent races; assert unsuccessful writes leave existing files byte-identical and do not publish partial tickets.
- Document JSON shape and failure semantics, stable sort/path conventions, supported OS/filesystem assumptions, and exact build/test commands.
- After implementation, use the CLI to show/start/finish the milestone and create deferred-work tickets as authorized by the seed; never mark the broader parent done prematurely.

## Uncertainty Assessment

- Correctness uncertainty: **Medium** — the domain and file schema are defined, but YAML strictness, preservation, filesystem races, and JSON errors need explicit contracts.
- Scope uncertainty: **Medium** — milestone commands are precise; creation-time overrides/parenting and recovery on malformed repositories need resolution.
- Architecture uncertainty: **High** — greenfield language/packaging, parser choice, storage commit protocol, and platform support remain design choices.

## Open Questions

1. Language/platform decision resolved by user: Go, macOS and Linux. Choose the smallest suitable local build/install approach.
2. What concurrency guarantee is feasible for CLI writers and directly editing agents/editors? What happens on conflict or interruption?
3. Should `new --parent` be the one minimal addition needed for follow-up-ticket dogfooding? Which creation overrides are needed now versus deferred?
4. Should invalid unrelated tickets block every command, only mutations/validation, or selected reads? Can `update` repair the target's invalid title/status?
5. What are the smallest useful JSON envelopes, error categories/exit codes, deterministic ordering, and path conventions?
6. Is local automated verification on macOS plus Linux CI sufficient, and how should fresh-checkout/install and live dogfooding evidence be recorded?

## Planning Workflow

### Interview decisions received during draft collection

- Go; macOS and Linux.
- Serialize CLI writers and reject detected stale snapshots. Direct body edits occur outside CLI updates; the final comparison-to-replacement race with a non-cooperating editor is an accepted and documented limitation.
- Include creation-time `--parent`, `--priority`, repeatable `--label`, and `--no-labels`. General metadata updates remain follow-up work.
- Strict failure for normal commands when any existing ticket is invalid; `validate` reports all determinable diagnostics. Do not silently return partial list/show results.
- Completion gate confirmed: automated parser/storage/CLI tests on macOS and Linux, a clean-source build/install smoke test (source copy acceptable while Git history is absent), and live milestone dogfooding.

- [x] Resolve command set and orient.
- [x] Write shared intent.
- [x] Collect available independent drafts (all three lanes completed successfully).
- [x] Collect available cross-critiques (all three lanes completed successfully).
- [x] Interview user (six explicit answers received).
- [x] Merge final sprint and notes.
- [x] Sync ledger and verify documents.
- [x] Present final sprint for approval; approval remains pending.

Resolved CLI executable: `/Users/calebmchenry/.nvm/versions/node/v22.11.0/bin/codex`. The interactive shell aliases `codex` to `codex --dangerously-bypass-approvals-and-sandbox`; this installed CLI does not list `--full-auto`. Preserve the user's existing startup behavior through the explicit executable plus `exec --dangerously-bypass-approvals-and-sandbox -m MODEL -c 'model_reasoning_effort="xhigh"'`, consistently for drafts/critiques. Lanes: `gpt-6-astra`, `gpt-5.6-sol`, `gpt-5.5`. Redirect per-lane stdout/stderr into its matching `.md.log`; artifact files are the review interface.
