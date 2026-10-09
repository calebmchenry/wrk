---
id: wrk-fa8c7f1f
title: Browse and focus wrk items in a searchable local web interface
status: done
parent: wrk-a1561423
depends_on:
  - wrk-6387cfe7
priority: normal
labels:
  - web
---

## Outcome

A list and detail pane make existing .wrk information easy to inspect while preserving the current work context.

## Acceptance criteria

- [x] Show project identity/path and a searchable list with ID, title, status, priority, labels, and blocker information. Search title, ID, and body.
- [x] Default to active work; offer all items plus Ready and manually Blocked views using current CLI semantics. Visually distinguish unfinished dependencies from explicit blocked status.
- [x] Support exact label filters and a parent focus showing the selected parent plus its descendants. Document this UI focus versus CLI --under, which excludes the root.
- [x] Display Markdown descriptions, parent navigation, children, dependencies, and reverse dependent information. Show direct-child completed/total counts without changing parent status.
- [x] Selecting an item opens a detail pane without losing search, filters, or list position; links and browser back/forward preserve usable navigation.
- [x] Render all five statuses; keep done/canceled reachable. Show existing priority/custom values without forcing a custom-field editing system.
- [x] Render untrusted ticket text safely, disable executable/raw unsafe HTML and dangerous link schemes, and avoid automatic remote resource loads.
- [x] Include clear empty, loading, missing-item, and validation-error states, keyboard-accessible controls, visible focus, and sensible narrow-screen layout.
- [x] Verify search/filter intersections, deep hierarchy navigation, missing selection, safe Markdown, and real browser rendering using representative projects.

## Scope

Deliver browsing first. The editing ticket adds forms; the live-update ticket keeps these views synchronized; the related-link ticket adds the new general relationship. A board and saved-view system are deferred.

## Implementation decisions (2026-10-08)

- Extend the embedded vanilla-JavaScript workspace with a searchable list and detail pane. Keep active/all/ready/manually-blocked views and exact AND labels consistent with the CLI; UI parent focus includes its root before intersecting other filters.
- Add one validated workspace read for project identity, summaries, and bodies used by local search. Item reads supply safe rendered Markdown, YAML custom values, and relationship summaries.
- Persist filters/selection in URL fragments, preserving list scroll while navigating. Manual reload remains the refresh mechanism; automatic live updates and editing stay in their existing tickets.
- Verify filter intersections, hierarchy, safe Markdown/resource handling, navigation, invalid/missing/empty states, and desktop/narrow browser rendering before completion.

## Implementation and verification (2026-10-08)

- Added the embedded list/detail workspace: project name/path/prefix, body/title/ID substring search, active/all/ready/manually-blocked views, exact AND label filters, and inclusive parent focus. Dependency blockers remain distinct from explicit blocked status; done/canceled remain reachable through All and relationship links. Child counts include direct children and count only done as complete.
- Selection and filters use URL fragments. Native ticket links, browser Back/Forward, copied/reloaded URLs, independent pane scroll, keyboard focus, and narrow-screen Back to list preserve working context. Missing selections/scopes, empty projects/results, loading, and strict validation/network failures have explicit recovery states. Aborted/stale detail responses cannot replace a newer selection.
- Added `/api/workspace` for one validated project/summary/body snapshot without per-item search fetches. Extended detail reads with safe Markdown, exact original YAML metadata, parent, dependencies, and reverse dependents. Existing list API/CLI semantics are unchanged.
- Pinned Goldmark v1.8.6 for Markdown rendering. Raw HTML stays disabled; link destinations are normalized then allowlisted; local ticket links navigate inside the app; other local paths and executable schemes remain plain text. Images become text placeholders, CSP blocks images, and metadata/source/diagnostics always use text nodes. Arbitrary custom YAML (including recursive aliases) stays exact in the metadata disclosure.
- macOS arm64, Go 1.25.4: uncached full Go suite and race suite, vet, build, clean gofmt, and diff checks passed. Targeted web/integration race tests, vet, and build also passed after final dependency traversal cleanup. The packaged-binary smoke now verifies the embedded ES module outside the checkout.
- Linux arm64, Go 1.25.4 (`golang:1.25.4-bookworm`): copied sources from a read-only mount into the container's local filesystem. Formatting, uncached full Go/race suites, vet, and build passed. Final targeted web/integration race tests, vet, and build also passed after the dependency traversal cleanup.
- Node model tests pass for filter/search intersections, exact/case-sensitive labels, all five statuses, canceled dependency blockers, URL round trips, missing focus, direct-child counts, and a 10,000-level parent chain. JavaScript syntax checks pass.
- All seven Playwright/Chromium scenarios pass against a disposable 40-item project served by the compiled CLI: intersecting filters/body search, parent/child/dependency/reverse links, closed items outside filters, Back/Forward/reload/list scroll, skip-link and keyboard focus, missing/empty/loading/error recovery, safe Markdown/no remote loads/custom YAML, 390px layout, and stale response cancellation. Desktop 1440×1000 and narrow 390×844 screenshots were opened and visually inspected. Node/Playwright are development-only dependencies; runtime assets remain embedded.
- Updated README, CLI/API docs, the docs index, and `docs/browser.md` with browsing semantics, Markdown/resource policy, snapshot/manual-refresh boundaries, and reproducible browser verification. No browser editing or automatic refresh was added; those remain their existing tickets. All acceptance criteria are met.
