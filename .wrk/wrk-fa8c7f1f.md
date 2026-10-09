---
id: wrk-fa8c7f1f
title: Browse and focus wrk items in a searchable local web interface
status: todo
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

- [ ] Show project identity/path and a searchable list with ID, title, status, priority, labels, and blocker information. Search title, ID, and body.
- [ ] Default to active work; offer all items plus Ready and manually Blocked views using current CLI semantics. Visually distinguish unfinished dependencies from explicit blocked status.
- [ ] Support exact label filters and a parent focus showing the selected parent plus its descendants. Document this UI focus versus CLI --under, which excludes the root.
- [ ] Display Markdown descriptions, parent navigation, children, dependencies, and reverse dependent information. Show direct-child completed/total counts without changing parent status.
- [ ] Selecting an item opens a detail pane without losing search, filters, or list position; links and browser back/forward preserve usable navigation.
- [ ] Render all five statuses; keep done/canceled reachable. Show existing priority/custom values without forcing a custom-field editing system.
- [ ] Render untrusted ticket text safely, disable executable/raw unsafe HTML and dangerous link schemes, and avoid automatic remote resource loads.
- [ ] Include clear empty, loading, missing-item, and validation-error states, keyboard-accessible controls, visible focus, and sensible narrow-screen layout.
- [ ] Verify search/filter intersections, deep hierarchy navigation, missing selection, safe Markdown, and real browser rendering using representative projects.

## Scope

Deliver browsing first. The editing ticket adds forms; the live-update ticket keeps these views synchronized; the related-link ticket adds the new general relationship. A board and saved-view system are deferred.
