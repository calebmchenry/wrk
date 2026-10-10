---
id: wrk-52fbccc3
title: Create and edit items in the browser without losing agent changes
status: done
parent: wrk-a1561423
depends_on:
  - wrk-27b71a32
  - wrk-ce7350ff
priority: normal
labels:
  - web
---

## Outcome

Perform the requested basic item workflow in the browser while agents continue using the same project safely.

## Acceptance criteria

- [x] Create an item with only a title required; support optional Markdown body, labels, parent, and dependencies, honoring current creation defaults.
- [x] Edit title/body, all statuses, labels, parent, and dependencies; offer suggestions from existing labels/items. Preserve priority/custom fields when not edited.
- [x] Reuse shared validated store mutations for HTTP writes and attach the revision originally loaded by the browser to all existing-item edits, including quick status actions.
- [x] Use explicit save/cancel for multi-field/body drafts, retain unsaved input across live refresh, indicate external changes, and offer reload/review on conflict without silent overwrite or automatic destructive retry.
- [x] Do not silently replace the form's base revision when live data arrives. Prevent or clearly warn about navigating away from an unsaved draft.
- [x] Show successful saved state only after publication is confirmed; display validation, BUSY, conflict, and committed-with-error outcomes accurately and resync ambiguous results.
- [x] Enforce server-side host/origin/mutation protections and supported request content types. A different website must not be able to submit local mutations; GET remains read-only.
- [x] Bound inputs, validate referenced IDs against the selected project, and reuse shared schema/cycle checks with actionable field errors.
- [x] Verify persistence by reading browser changes through the CLI, and test an agent edit after a browser draft opened followed by a rejected stale save with draft preserved.
- [x] Test accessible forms, status/labels/relationship actions, empty body handling, canceled/reopened items, invalid cycles, and external edits during interaction.

## Scope

One item type and explicit transitions; no assignment/workflow configuration, bulk operations, deletion UI, or custom-field form builder. The related-link ticket extends the same relationship UI afterward.

## Implementation decisions (2026-10-09)

- Add same-origin JSON POST creation and PATCH single-item edits through the shared store. Every PATCH requires the browser's original expected revision. Keep priority/custom values outside the form and preserve omitted fields.
- Keep forms in the persistent draft mount with explicit save/cancel, item/label suggestions, and navigation protection. Polling only updates surrounding data and external-change notices. Conflict review is explicit; no automatic retry or base-revision advancement.
- Distinguish unpublished validation/BUSY/conflict errors, confirmed publication, and committed/transport ambiguity. Retain drafts and resynchronize before further action on ambiguous outcomes.
- Add API and real Chromium/CLI coverage for mutation security, persistence, statuses/relationships, stale saves, draft retention, and publication error handling.

Implementation and verification are complete; no remaining work in this ticket.

## Implementation and verification (2026-10-09)

- Added strict same-origin JSON POST creation and PATCH revision-checked edits.
  Requests reject unsupported methods/content types, missing/foreign Origins,
  wrong hosts, malformed/duplicate/unknown fields, nulls, oversized input, and
  project/path selection. GET/HEAD remain read-only. Shared store code owns
  schema/reference/cycle validation, locking, preservation, and publication.
- Added bounded service entry points to the shared store. Locked loading,
  creation collision reloads, candidate file/total size checks, and final snapshot
  comparison honor service limits and cancellation. Ordinary CLI behavior remains
  unbounded. Resource failures cannot masquerade as stale missing-target results.
- Added accessible create/edit forms, label/item suggestions, explicit Save/Cancel,
  all statuses, quick done/reopen actions, labels, parenting, and dependencies.
  Omitted metadata and untouched CRLF bodies remain exact; edited textarea bodies
  use LF. Creation uses current defaults, with an explicit label-default choice.
- Forms retain the original revision/input across polling, internal navigation,
  filters, deletion, and reconnects. Dirty/in-flight/uncertain drafts warn before
  full-page departure. Explicit conflict review can reload or keep changed values
  against the reviewed revision; collection edits retain unrelated agent additions.
- Publication-confirmed success/no-op, validation/BUSY/conflict, committed errors,
  and lost-response ambiguity have distinct UI states. Ambiguous saves retain the
  draft, disable Save, and resynchronize; another mutation requires explicit review.
  Lost creation responses never automatically create a duplicate.
- Updated README, browser/API/storage documentation, and documentation index links.
  Existing external-editor/Git publication boundaries remain in force; browser
  drafts are in-memory and are lost if the user accepts departure or the browser
  crashes. Related-item links remain in wrk-95118e5e.

### Passing checks

- macOS arm64, Go 1.25.4: clean formatting; unfiltered `go test -count=1 ./...`
  and `go test -race -count=1 ./...`; `go vet ./...`; executable build.
- Linux arm64, Go 1.25.4 (`golang:1.25.4-bookworm`): the same normal/race suites,
  formatting, vet, build, and project validation pass on the container filesystem
  after copying source from a read-only host mount. This is runtime verification.
- `npm ci`, Chromium installation, and all 20 Playwright scenarios pass. The eight
  new editing scenarios use the real loopback server/current compiled CLI and cover
  defaults, persistence, every status, custom metadata, exact omitted body bytes,
  empty bodies, labels/relationships, cycles, stale forms/quick actions, explicit
  review, navigation/deletion, cancel/reopen, and uncertain save/create outcomes.
  Four focused scenarios also pass after the final collection-review refinement.
- All eight Node model/poller/draft tests and all six Python burn-runner tests pass.
- Visually inspected desktop and 390-pixel editing screenshots; controls, focus,
  draft notices, and form content fit without horizontal overflow.
- API tests cover writer BUSY, selected-project references, stale no-ops,
  malformed/deleted targets, security/JSON/input limits, cancellation, committed
  result envelopes, and unchanged ticket bytes after rejected writes. Store tests
  exercise locked-load/candidate limits and cancellation/growth before publication.
- Packaged executable verification includes the embedded editor module.
  `git diff --check` is clean.
