---
id: wrk-ce7350ff
title: Reflect agent and filesystem changes live in the browser
status: done
parent: wrk-a1561423
depends_on:
  - wrk-fa8c7f1f
priority: high
labels:
  - web
---

## Outcome

Keep the browser open while agents work through wrk items, and see their changes automatically without manual refresh.

## Acceptance criteria

- [x] Detect ticket creation/update/deletion and .wrk/config.yaml changes made outside the server, including CLI atomic replacement and direct body edits; update visible views within two seconds under normal local operation.
- [x] Choose and document a modest polling or filesystem-notification approach with bounded resources. Coalesce rapid changes and ignore lock/staging/runtime files; no per-file event history is promised.
- [x] Refresh list membership, status, labels, selected details, hierarchy, blockers, progress, and config-derived display from current validated project state.
- [x] Close the initial-load/subscription race and resynchronize from a full current snapshot after reconnect, sleep/resume, or dropped notifications. Show connection/reconnecting state.
- [x] Preserve filter/search/selection/scroll context. Define a dirty-draft interface that refreshes surrounding data but never overwrites unsaved form content; editing wires it into actual forms.
- [x] When a selected item is removed or no longer matches filters, show an understandable state without silently discarding a draft.
- [x] Surface invalid/intermediate file states clearly, label any retained last-valid snapshot as stale, avoid claiming health while invalid, and recover automatically after repair.
- [x] Do not take a writer lock for watching, mutate files while reading, or imply consistency beyond the existing per-item publication boundary.
- [x] Add deterministic detection/reconnect/recovery tests and a real browser-plus-separate-CLI scenario covering create, status, label, parent, dependency, and body changes.

## Verification example

Start serve, open a selected item, then use the checkout CLI in another process to move it todo -> in-progress -> done and change labels/relationships. Confirm updates appear within the normal-operation bound without a page reload. Also test atomic replacement, malformed YAML followed by repair, reconnection, and ignored staging churn.

## Implementation decisions

- Poll the validated workspace every 750 ms after each completed read, with a single browser request and timer. Content-based conditional reads avoid transmitting/rendering unchanged snapshots; there is no filesystem event backlog or initial subscription race.
- Include the selected detail in the same validated workspace load. Keep the existing item API for other clients. Validate before conditional responses so invalid data can never appear healthy.
- Refresh immediately on selection, manual reload, reconnect, and resume signals; recover with a full snapshot after failures. Retain last-valid data with explicit stale diagnostics.
- Keep URL state and pane context, and define a draft registration interface with a separate persistent form mount. Editing remains in wrk-52fbccc3.
- Verify byte-level detection, invalid/repair behavior, polling lifecycle, draft retention, and real browser updates from a separate CLI process. Existing unrelated checkout changes are outside this ticket.


## Implementation and verification (completed 2026-10-09)

Implementation and all acceptance checks are complete. The earlier session's
loopback/Chromium sandbox restrictions were resolved by verification in the
current environment; the existing implementation needed no further fixes.

- Added 750 ms conditional workspace polling and content ETags covering config,
  ticket paths/bytes, and selected ID. Every conditional read validates first;
  ignored lock/staging/runtime churn cannot trigger a changed snapshot. Readers
  take no writer lock and keep the existing bounded request/resource model.
- Workspace responses now include optional selected detail from the same load.
  All views refresh together, including relationships and derived state. Selected
  deletion produces null detail plus a usable list, without losing the selection.
- Added cancellation/generation protection, automatic full recovery after errors,
  explicit browser resume handlers, and late-timer detection for system sleep
  without a browser event. The header labels disconnected/invalid retained data
  stale. A repaired project recovers automatically, including identical bytes.
- Retain filters, selection, current pane scroll, and detail disclosures. Added
  the documented `drafts.register` interface and persistent `#draft` form mount;
  notifications preserve the original base revision and flag external changes,
  deletion, filter exclusion, and stale data. Actual forms stay in wrk-52fbccc3.
- Added deterministic Go handler and Node lifecycle/draft tests. Extended the
  Playwright suite from 7 to 12 scenarios, including separate CLI changes with
  two-second assertions, direct edits, config/invalid/repair, relationships and
  progress, dirty drafts, deletion, offline/dropped response/BFCache recovery,
  initial-load changes, and ignored churn. Packaged asset checks include live.mjs.
- Updated README, API/browser documentation, and the docs index. Preserved the
  unrelated pre-existing burn runner, workflow, and ticket changes.

### Passing checks

- On macOS arm64 with Go 1.25.4: unfiltered `go test -count=1 ./...` and
  `go test -race -count=1 ./...`, `go vet ./...`, clean
  `gofmt -l cmd internal test`, and `go build -o ./bin/wrk ./cmd/wrk` all pass.
  This includes the five socket-dependent tests previously blocked by the sandbox.
- The same unfiltered normal/race suites, formatting, vet, build, and project
  validation pass on Linux arm64 with Go 1.25.4 in
  `golang:1.25.4-bookworm`. Source was copied from a read-only host mount into
  `/tmp/wrk` on the container's local filesystem, following the
  [documented Linux workflow](../docs/releases.md#verification-and-local-packaging).
  The existing unwritable-directory test skips under container root; macOS
  runs it unprivileged.
- `npm run test:browser`: all 12 Chromium scenarios pass (21.3 seconds), using
  the current compiled CLI and a real loopback server. Separate CLI changes,
  direct edits, deletion, and config updates satisfy two-second assertions.
  Invalid/repair, offline/dropped response/resume, initial-load races, pane
  context, draft retention, and ignored churn scenarios all pass.
- Inspected generated `desktop.png` (1440x1000) and `narrow.png` (390x844)
  under `test-results/`; content, live status, controls, and relationships render
  correctly with no horizontal overflow.
- `npm test`: all eight deterministic Node model/lifecycle/draft tests pass
  on Node 22.11.0. All six Python burn-runner tests and `git diff --check` pass.

### Remaining work

None for this ticket. Actual browser forms and save/navigation protection remain
in wrk-52fbccc3; the parent workspace milestone stays open.
