---
id: wrk-9be86c55
title: Build the compact table and detail-panel layout
status: done
parent: wrk-d70e8056
priority: high
labels:
  - web
  - web-ux
---
## Outcome

Replace the vertically stacked header, filter sidebar and card-like list with a compact workspace shell that gives rows most of the screen and opens details only when selected. Follow the parent's shared product contract.

## Acceptance criteria

- [x] Keep project identity, compact connection state, query-control space and New action in a small header/toolbar area. Move full project path and occasional actions such as reload into a compact disclosure/menu. Show no dedicated normal-state connection/save banner or empty detail placeholder.
- [x] Render a one-line table-row structure with slots for chevron, status, title, tags, muted ID and overflow actions. Title receives most width; cap visible tag chips with +N overflow and make full title/tags/ID available accessibly. Secondary metadata must not increase row height.
- [x] Use a real usable table/list structure with accessible item links and keyboard-operable controls; row action clicks do not also select/navigate the row. Provide visible hover, selection and keyboard-focus states.
- [x] Opening a row displays a right-side detail pane while leaving useful table width; no selection returns all available width to the table. Add close, copy ID and copy link controls and independent scrolling. Copy failures have useful feedback.
- [x] Preserve filters, URL fragments, browser Back/Forward, selection and table scroll through pane open/close and relationship navigation. Selected items outside filters and missing items have compact recovery states.
- [x] On narrow screens, use a full-width detail view and Back to table, preserve scroll/context and avoid horizontal page overflow. Hide/truncate secondary row content before sacrificing title and status usability.
- [x] Provide compact loading, empty, stale/reconnecting and validation-error treatments. A problem may expand for actionable detail; normal operation stays compact.
- [x] Verify basic navigation/focus in the real browser and visually inspect representative desktop and narrow layouts; keep existing read/live/navigation behavior working during the change.

## Scope and handoff

This ticket establishes DOM/layout/navigation and common visual primitives. Keep existing functional create/edit entry points usable until their replacements land. Hierarchy/filter behavior is owned by the browsing child; the modal, detail editor and row mutation behavior have separate children. Avoid coupling unrelated changes into this foundation.

## Starting points

internal/web/assets/index.html, style.css, app.js; docs/browser.md navigation/live contracts; test/browser/workspace.spec.mjs.

## Verification and remaining work

Completed 2026-10-09.

### Implementation and handoff

- Replaced the tall header/sidebar/card layout with a compact identity/connection header, query toolbar, and 40-pixel native list rows. Rows have separate chevron/status/title/tag/ID/action slots, two visible tag chips plus +N, full accessible descriptions, and visible hover/selection/focus. Row actions expose full title/tags/ID and copy controls without selecting the row.
- The unselected table fills the workspace. Selection opens a right pane; screens at 760 pixels or narrower use a full-width detail view and Back to table. Close/back restores focus and table position. URL filters, selection, relationship navigation, history, independent scrolling, and reload restoration remain intact, including when the narrow table is hidden.
- Added detail copy controls with successful-copy feedback and a selectable manual-copy fallback when clipboard access is denied. Kept existing create/edit/quick-completion entry points and revision-aware draft/conflict recovery intact.
- Moved project path/config summary/reload into a disclosure. Connection state stays in the header; save/copy confirmations are temporary notices. Errors expand to show diagnostics while retaining marked stale data. Empty/missing/outside-filter states have compact recovery guidance.
- Preserved list DOM for selection-only refreshes, while derived progress still updates when a filtered-out child changes. Positioned hidden accessible descriptions within rows so they cannot add page overflow.
- Updated docs/browser.md and its docs/index.md link. Tags is the new filter wording; substring search, inclusive parent focus, flat ordering, and the legacy editor remain for their owning children to replace. No server/storage contract changed. Existing unrelated working-tree changes were preserved.

### Verification

- macOS arm64: clean gofmt output; uncached `go test -count=1 ./...` and `go test -race -count=1 ./...`; `go vet ./...`; current binary build; all 6 ticket-burn Python tests; all 9 Node model tests passed.
- `npm ci` and `npx playwright install chromium` succeeded. All 27 browser scenarios passed after the final layout correction. A final focused run passed relationship/history, narrow-pane context, and the additional filtered-child-progress/reload-scroll scenario (28 distinct scenarios covered overall).
- Browser coverage includes real keyboard navigation/focus, row-action isolation, clipboard success/denial, capped/long tags and titles, pane sizing, independent scroll, Back/Forward/reload, narrow relationship navigation, hidden-table polling, disclosure bounds, actionable validation recovery, and existing read/live/edit/security regressions.
- Visually inspected 1440×1000 desktop table/detail, 390×844 narrow table/detail, and 320×700 expanded validation-error screenshots. Corrected excess page height and cramped narrow controls found in the first visual pass. Automated bounds checks also cover 760- and 800-pixel widths.
- Screenshot evidence (ignored local artifacts): `test-results/workspace-compact-rows-kee-83e01-es-with-usable-copy-actions/{compact-table,compact-detail}.png`; `test-results/workspace-narrow-panes-ret-9ba47-s-polling-close-and-history/{compact-narrow-table,compact-narrow-detail}.png`; `test-results/workspace-compact-disclosu-3e9c9-pose-actionable-diagnostics/compact-narrow-error.png`. The final focused navigation run is under `test-results/final-navigation/`.
- Linux arm64: copied this checkout into the disposable container's local filesystem using `golang:1.25.4-bookworm`; clean formatting, uncached Go/race suites, vet, and native build passed. All 6 Python tests passed with Docker `--init`; the first run without init hit the already-documented zombie-reaping issue in interruption assertions. Chromium visual verification was on macOS.
- `git diff --check` passed. Final project validation/status verification is recorded by the completion CLI commands.

No work remains in this ticket. The browsing and creation-modal children can start; this does not complete the parent milestone.
