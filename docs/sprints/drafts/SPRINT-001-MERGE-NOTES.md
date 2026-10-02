# Sprint 001 Merge Notes

Status: planning complete; [final sprint](../SPRINT-001.md) awaits user approval. No implementation was performed.

## Local feasibility evidence

- There is no existing implementation to extend, no remote or commit history, and no source/test/build convention to inherit. Existing README, docs, configuration, and ticket files are user-owned untracked work.
- The installed toolchains include Go 1.25.4, Python 3.13.3, Node 22.11.0, and Cargo 1.64.0; `uv` is not present. These are observations, not release-version recommendations.
- `docs/configuration.md` describes explicit creation overrides, including an empty label list, while the seed's command table only specifies title/body creation. The final plan must resolve this rather than promise unimplemented flags.
- The seed requires deferred metadata tickets under `wrk-3f8a21b7`, although general parent mutations are deferred. A narrow creation-time parent option is a candidate resolution.
- Go's [`os.Rename`](https://pkg.go.dev/os#Rename) can replace existing files and does not provide compare-and-swap. A lock plus a final reread detects stale snapshots but cannot exclude a non-cooperating editor between comparison and rename. The final plan must define the supported concurrency boundary explicitly.
- The maintained YAML organization's [`go.yaml.in/yaml/v3`](https://pkg.go.dev/go.yaml.in/yaml/v3) exposes nodes and documents type-resolution compatibility behavior. AST inspection is needed to distinguish strict field types and preserve unknown custom values; a struct-only decoder or generic JSON roundtrip can lose information. Pin dependencies during implementation and verify the selected version against fixtures.
- Unconfigured custom values are allowed by the current spec. A plan that silently restricts all unknown values to JSON primitives, flattens aliases into changed meanings, or rejects every unfamiliar YAML tag would alter the contract. Define a lossless representation or preserve opaque nodes and raw frontmatter in output.
- Exact body preservation requires keeping the bytes after the closing frontmatter delimiter independently of YAML decoding. Include CRLF, delimiter-like body text, Unicode, and absent terminal newline fixtures.
- No GitHub remote exists. A CI workflow may be prepared and Linux checks specified, but a local macOS run cannot be reported as executed Linux CI.

## Draft and critique synthesis

### Independent draft strengths

- **GPT-6 Astra:** strongest end-to-end contract coherence. It resolves creation defaults, preserves arbitrary custom YAML through a source-bearing JSON result, distinguishes publication from durability/output failures, uses a persistent lock inode, and puts storage/preservation feasibility before broad command implementation.
- **GPT-5.6 Sol:** strongest inventory of modules, exact CLI parsing behavior, strict schema layering, whole-project snapshots, and deterministic diagnostics. Its detailed discovery and subprocess cases are useful implementation inputs.
- **GPT-5.5:** most compact delivery outline and clear core workflow. It keeps the broader parent open, ties the parent creation flag to dogfooding, and calls for deterministic conflict tests and copied compatibility fixtures.

### Choices adopted in the final merge

- Adopt four cohesive production areas (CLI, project/config, ticket/YAML, store) rather than the Sol draft's many small packages. Keep pure validation separate from filesystem publication within these areas.
- Do not adopt a new JSON-only custom-value restriction. Prefer normalized built-in fields plus exact source for `show`, with lossless source data as the custom-value representation in JSON v1.
- Do not lock `config.yaml`: an editor can atomically replace its inode, splitting writer coordination. Use a persistent dedicated lock inode and never unlink it during ordinary release.
- Acquire the writer lock before the authoritative snapshot, then compare all validation inputs immediately before publication. CLI contention may fail promptly with a retryable diagnostic rather than queue indefinitely.
- Use complete staging plus no-replace publication for new tickets. An exclusive open directly on the final path prevents clobbering but can expose partial contents.
- Strict user-selected validation wins over Astra's optional invalid-title/status repair exception. General repair is out of scope; do not quietly authorize direct frontmatter repair that contradicts the repository's editing contract.
- Keep the user-approved creation flags even though two drafts defer default overrides. Their omission conflicts with the now-explicit interview answer and the documented creation-default behavior.
- Preserve the distinction between pre-publication rejection and errors after a successful link/rename. A blanket “all nonzero results mean unchanged files” promise is not implementable when directory sync or stdout delivery fails afterward.

### Cross-critique findings accepted

| Finding | Review support | Final treatment |
| --- | --- | --- |
| Mutable config is an unsafe lock inode; deleting a dedicated lock also splits coordination. | All three critiques | Persistent `.wrk/.lock`, no unlink/truncate, race-safe initial creation, real two-process/repeated-inode/config-replacement tests. Read-only commands create nothing. |
| Checking only the target misses changes to validation inputs. | Astra and Sol | Take the authoritative snapshot under lock; compare config, inventory, ticket identities and bytes immediately before publication. Explicitly retain the accepted external-editor limitation. |
| Exclusive open on the final ticket exposes partial data. | All three critiques | Stage complete bytes, then atomically publish without replacement; test collisions, short/write/close failures and interruptions. |
| “Every failure means unchanged files” is false after publication. | All three critiques | Pre-publication rejection leaves existing ticket/config bytes unchanged; later errors carry committed paths/state when possible. Distinguish cleanup and durability errors; never blind rollback/retry. |
| JSON-only custom values would narrow the existing format. | All three critiques | Retain YAML nodes and scalar tags, exact body bytes, semantic candidate comparison, and full `source` in show JSON. No generic YAML-to-JSON rewrite. |
| Alias handling needs early feasibility evidence. | All three critiques | Test aliases of mutable fields; compare scalar-span patching if node re-emission fails. Fail unsupported mutations unchanged rather than coercing values. |
| Bounded update repair conflicts with the strict-invalid-project decision. | Sol and GPT-5.5 critiques | Remove the exception. Ordinary commands fail on invalid project state. |
| Lock/temp artifacts are permitted filesystem side effects, not ticket data. | Sol critique and local review | Document persistent ignored runtime artifacts separately from unchanged ticket/config guarantees; do not clean up other operations' names. |
| Discovery must stop even at an unusable inner `.wrk`. | Astra critique | Encountering a file/symlink/invalid directory is an error boundary; matching nonregular ticket entries are diagnosed. |
| `_unix.go` is not itself a build constraint. | Astra critique | Require explicit `//go:build darwin || linux`. |
| Confirmed creation flags and OS checks cannot be deferred. | All three critiques | Carry four flags through use cases, implementation, tests and DoD; require actual Linux execution without silently dropping support. |
| Real fixtures do not cover every promised case. | Astra critique | Preserve the three real fixtures and add synthetic old-prefix/custom-value/alias/graph/conflict cases. |
| Output/parser details must stop being negotiable during execution. | All three critiques | Stable ID sort, flag placement/arity rules, one versioned envelope, full-source JSON, project-relative paths, and 0/1/2 exit classes. |

### Recommendations rejected or bounded

- **Manual frontmatter repair:** rejected, including the GPT-5.5 critique's isolated recommendation to remove repair mode and then repair directly. It contradicts repo conventions and that critique's own final recommendation. The plan directs restoration from known-good data or an explicitly authorized repair; it does not add an implicit editing exception.
- **JSON-compatible YAML only:** rejected. Output convenience does not authorize a format migration. Rare preservation limitations are explicit mutation failures, not global rejection of otherwise valid custom values.
- **Support-file-only initialization:** Astra's critique favors adopting an existing `.wrk` that lacks config/tickets. The final plan instead refuses any existing `.wrk`, as Astra's draft and GPT-5.5's critique recommend. Both can satisfy no-overwrite requirements; refusal is simpler and there is no concrete adoption need in this milestone.
- **Six-category exit codes:** reduced to success `0`, operational/data failure `1`, usage `2`; stable error codes carry detail. This accepts Sol's scope concern without weakening machine-readable diagnostics.
- **Only command-local JSON:** not adopted. Support `--json` before or after the subcommand as a simple global formatting option; the user-facing contract is more consistent and parser tests keep the small addition bounded.
- **Many separate application/config/graph/diagnostic/output packages:** consolidated into four cohesive areas plus a thin entry point. File names are implementation guidance, not a requirement to create empty abstractions.
- **Mandatory hosted CI/public module identity:** rejected as prerequisites. Actual macOS/Linux evidence is required, but a remote, hosted runner, published package, and Git history are not. The user explicitly accepted a clean source copy for installation evidence.
- **General-purpose anchor rewriting/resource policy:** bounded to preservation feasibility and explicit fail-closed behavior. Do not build a general YAML migration engine or impose undocumented format limits.

### Acceptance mapping checked against the final merge

| Seed requirement | Planned execution gate |
| --- | --- |
| Fresh checkout/local run and bare help | Scaffold phase plus clean-source verification in delivery phase |
| Six command contract and noninteractive behavior | Parser contract; read commands phase; update phase; init/new phase; subprocess workflow |
| Per-command readable/JSON results including IDs and paths | Shared renderer/envelope frozen early; success/error subprocess matrix |
| Nearest boundary and strict config/custom definitions | Project/config phase and invalid nested-project fixtures |
| Existing tickets without migration | Immutable copies of all three tickets/config; real-project validation before live mutations |
| Stable unique local IDs and no overwrite | Crypto IDs, complete staging, no-replace publication, deterministic collision fixtures |
| Exact body and unrelated metadata/custom-value preservation | Byte split plus YAML nodes; semantic reparse comparison and adversarial fixtures |
| Validate before writing; stale change protection | Locked authoritative snapshot, complete candidate validation, full-input recheck, conflict/fault tests |
| Metadata/identity/reference/graph diagnostics | Layered validator with separate graphs and deterministic diagnostics |
| Blockers without forbidden transitions/cascades | Derived readiness/blockers plus blocked-status and no-cascade tests |
| Meaningful automated tests | Unit, filesystem, compatibility and compiled-CLI tests on both supported OSes |
| README and agent docs | Verified examples; supported/deferred metadata list; index links |
| Show/start/follow-ups/done through CLI | Start after safe update phase; create parented follow-ups after new; finish only after every gate |

### Final phase sequence

1. Scaffold executable and output contracts; prove storage/YAML primitives.
2. Parse/load/validate project; implement read commands.
3. Implement safe title/status updates; begin live dogfooding only after safety tests.
4. Implement init/new, exact bodies, user-approved creation flags, and collisions.
5. Complete subprocess/OS/clean-source verification and documentation.
6. Create deferred-work tickets, record evidence, and complete only the child milestone through the CLI.

The final document contains explicit checkboxes and exit gates, concrete module paths, a bounded storage contract, and a verification matrix. Resolved alternatives are removed from its Open Questions section; dependency compatibility, YAML feasibility, and Linux execution remain explicit implementation gates.

## Interview refinements

The user explicitly selected Go and macOS plus Linux. Apply these to the merge even if a draft was written before the answers arrived.

The user also explicitly selected serialized CLI writers with stale-file checks and the documented editor-race boundary; all four proposed creation flags (`--parent`, `--priority`, `--label`, `--no-labels`); and strict failure for normal commands on an invalid project. These answers supersede contrary draft assumptions.

The sixth answer explicitly accepted automated parser/storage/CLI verification on both macOS and Linux, a clean-source build/install smoke test using a source copy while Git history is absent, and live milestone dogfooding. No interview defaults remain unanswered.

## Review provenance

All three requested CLI lanes completed draft and critique passes successfully with `xhigh` reasoning. No model was dropped, retried at a different effort, or substituted. Reviewer diversity was not reduced. Each critique reviewed only the other two drafts; no fourth critique artifact was created. Local file-level feasibility judgment is recorded in these merge notes.

| Lane | Draft | Combined critique of other drafts |
| --- | --- | --- |
| `gpt-6-astra` | [Draft](SPRINT-001-GPT6ASTRA-DRAFT.md) | [Critique](SPRINT-001-GPT6ASTRA-CRITIQUE.md) |
| `gpt-5.6-sol` | [Draft](SPRINT-001-GPT56SOL-DRAFT.md) | [Critique](SPRINT-001-GPT56SOL-CRITIQUE.md) |
| `gpt-5.5` | [Draft](SPRINT-001-GPT55-DRAFT.md) | [Critique](SPRINT-001-GPT55-CRITIQUE.md) |

Resolved command family for both phases: `/Users/calebmchenry/.nvm/versions/node/v22.11.0/bin/codex exec --dangerously-bypass-approvals-and-sandbox -m MODEL -c 'model_reasoning_effort="xhigh"' PROMPT > ARTIFACT.log 2>&1`. The explicit startup flag matches the user's existing interactive alias; the installed CLI does not expose the skill's older `--full-auto` flag. Per-lane logs remain local and are ignored by `drafts/.gitignore`.

Language/platform questions were asked early while drafts ran; remaining scope and verification questions were resolved during drafting/review. All six answers were explicit. Later review findings introduced no unresolved product decision.

## Planning completion checklist

- [x] Resolve commands and orient to project conventions/backlog.
- [x] Write shared intent and orientation summary.
- [x] Receive all three independent drafts.
- [x] Receive all three combined cross-critiques.
- [x] Conduct six-question interview and apply answers.
- [x] Write final sprint and accepted/rejected merge reasoning.
- [x] Sync ledger and verify document links/metadata preservation.
- [ ] User approves the final sprint.

The final approval step comes from the explicitly invoked [sprint-plan-codex-only skill](/Users/calebmchenry/.codex/skills/sprint-plan-codex-only/SKILL.md): “Present to the user for approval.” The ledger's `planned` state records planning, not approval or execution.

Verification: all local links in 13 Markdown files resolved; six review artifacts and all required final-sprint sections were present; the ledger contains Sprint 001 as `planned`; per-lane logs are ignored. SHA-256 comparisons confirmed the original config, agent guidance, parent, and completed format ticket remained unchanged, and the milestone retained its original contents plus only an appended planning note. All existing ticket frontmatter is unchanged. No application tests were run because this task produced planning documents, not an implementation.
