---
id: wrk-14ac1a9c
title: Capture compact web UX direction and recommendations
status: done
priority: normal
labels:
  - web
---
## Outcome

Capture the user's compact web UX direction, identify complementary behaviors, and organize the accepted direction into an implementation backlog. This ticket covers discovery and organization; product implementation is tracked separately.

## User direction (2026-10-09)

- Simplicity and efficient vertical space are primary constraints.
- Compact query controls above the table, corresponding to CLI filters such as descendants under a parent and labels/tags; fuzzy title search.
- Hierarchical table with indented children and expand/collapse controls on parents. Rows show title and tags; ID visibility remains a design question.
- Clicking a row opens a detail panel while leaving the table visible.
- New-item modal with a top-right close icon, borderless title and description inputs, useful placeholders, tag chips and an optional parent picker. Only title is required. Use a plain textarea for now.
- Add-child row action, potentially in an overflow menu, inserts an autofocused inline title input. Enter or an inline confirmation creates the child and starts another blank sibling. Cancel removes the draft row.
- Details should feel like an editable document rather than a form, with all relevant information and a plain description textarea.

## Existing behavior inspected

Read docs/index.md, docs/browser.md, relevant CLI filter contracts, the completed web discovery/milestone tickets, and the current HTML/CSS. Existing browser behavior includes live updates, URL/history state, narrow-screen detail navigation, revision-checked saves, draft retention, and conflict recovery. Preserve those capabilities in a redesign.

CLI --under excludes the root; current browser parent focus includes it. Repeated label filters use AND semantics. Current search is substring matching across ID/title/body; fuzzy title search is a deliberate behavior change.

## Recommendations accepted on 2026-10-09

The recommendations were initially proposed for discussion. The user subsequently accepted them and requested work items under one parent with dependencies, delegating the detailed write-up.

- Keep a compact status control in each row, with title taking most width, capped tag chips, a muted ID, and an overflow action menu. Do not introduce extra rows for metadata.
- Use one compact query toolbar when space allows. Default to active work, retain All/Ready options, removable filter chips and clear filters. Preserve CLI filter semantics and choose one visible term for tags/labels.
- During filtering/search, show muted ancestor context and automatically reveal matching descendants; restore manual collapse state when clearing search. Context ancestors do not count as matching results.
- Keep quick child creation under the chosen parent; create another blank row only after confirmed success. Escape/cancel exits. Pin the active creation row against filtering/live refresh, and retain typed titles on errors.
- Open a right detail pane only when selected; preserve table position and give it independent scrolling, a close control, copy ID/link, and full-width treatment on narrow screens.
- Use immediate saves for discrete metadata changes; consider a small Save/Cancel affordance shown only while title/description edits are dirty. Preserve drafts and surface agent conflicts close to the edited content.
- Keep secondary relationships/metadata behind compact disclosures when empty or uncommon. Ensure borderless fields remain keyboard accessible with accessible names and visible focus.
- Avoid adding persistent banners for normal live/save state; show compact status and expand only when an actionable issue occurs.

## Verification and scope

Initial `go run ./cmd/wrk validate` passed with 26 tickets; active and ready lists were empty. Existing web tickets are complete and cover the original implementation, so this new discussion has its own record. Existing unrelated working-tree changes remain untouched. No product code was changed or browser behavior tested for this discussion.

## Completion criteria

- [x] Inspect current product and filter contracts.
- [x] Record the user's requested direction separately from proposed additions.
- [x] Identify a minimal set of recommendations and behavioral edge cases for the conversation.
- [x] Validate ticket records after this update: 27 valid tickets.

## Organized implementation backlog (2026-10-09)

Created [wrk-d70e8056: Deliver a compact table-first web workspace](wrk-d70e8056.md) with six direct children:

1. [wrk-9be86c55](wrk-9be86c55.md): compact table/detail layout and navigation.
2. [wrk-bd701745](wrk-bd701745.md): hierarchy, compact filters and fuzzy title search.
3. [wrk-87f1ef2a](wrk-87f1ef2a.md): minimal creation modal and reusable tag/item pickers.
4. [wrk-83357d9a](wrk-83357d9a.md): document-style detail editing and revision-aware immediate saves.
5. [wrk-824a1e93](wrk-824a1e93.md): row status actions and repeated inline child creation.
6. [wrk-183a9f94](wrk-183a9f94.md): integrated browser/CLI verification, visual review and documentation.

The parent owns the shared UX contract and dependency rationale. It is related to this completed discovery ticket rather than parenting it. Implementation tickets remain todo; this organization does not claim product implementation or verification.

Write-up defaults resolve the user's accepted direction: Tag maps to existing labels; Under excludes the root while allowing muted root context; fuzzy search matches titles; priority/custom metadata remain visible and preserved; empty related/dependency sections retain an add action. Discrete saves coordinate with retained text drafts through one revision-aware path. The single active draft and uncertain-publication protections remain in force.

Only the layout should be ready initially. The parent waits on final verification; that transitively waits for all feature work. The parent must receive its own acceptance review. Updated docs/index.md with discovery and milestone links plus scoped CLI commands.

### Organization completion criteria

- [x] Record user acceptance separately from the original proposal.
- [x] Create one parent and six actionable children with criteria, scope and verification expectations.
- [x] Set real prerequisite edges and explain the sequence in the parent.
- [x] Link discovery and agent documentation without changing product code or unrelated working-tree edits.
- [x] Verify the final hierarchy, ready set and project validation.

Organization verification passed: `go run ./cmd/wrk validate` reports 34 valid tickets. The scoped JSON list contains exactly six todo children with the intended parent, labels and dependency edges; the scoped ready list contains only wrk-9be86c55. Parent `show` confirms its final-verification prerequisite and discovery relationship. `git diff --check` passed. No product code changed and no product tests were needed for this backlog-only task.
