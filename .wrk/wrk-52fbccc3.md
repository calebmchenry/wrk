---
id: wrk-52fbccc3
title: Create and edit items in the browser without losing agent changes
status: todo
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

- [ ] Create an item with only a title required; support optional Markdown body, labels, parent, and dependencies, honoring current creation defaults.
- [ ] Edit title/body, all statuses, labels, parent, and dependencies; offer suggestions from existing labels/items. Preserve priority/custom fields when not edited.
- [ ] Reuse shared validated store mutations for HTTP writes and attach the revision originally loaded by the browser to all existing-item edits, including quick status actions.
- [ ] Use explicit save/cancel for multi-field/body drafts, retain unsaved input across live refresh, indicate external changes, and offer reload/review on conflict without silent overwrite or automatic destructive retry.
- [ ] Do not silently replace the form's base revision when live data arrives. Prevent or clearly warn about navigating away from an unsaved draft.
- [ ] Show successful saved state only after publication is confirmed; display validation, BUSY, conflict, and committed-with-error outcomes accurately and resync ambiguous results.
- [ ] Enforce server-side host/origin/mutation protections and supported request content types. A different website must not be able to submit local mutations; GET remains read-only.
- [ ] Bound inputs, validate referenced IDs against the selected project, and reuse shared schema/cycle checks with actionable field errors.
- [ ] Verify persistence by reading browser changes through the CLI, and test an agent edit after a browser draft opened followed by a rejected stale save with draft preserved.
- [ ] Test accessible forms, status/labels/relationship actions, empty body handling, canceled/reopened items, invalid cycles, and external edits during interaction.

## Scope

One item type and explicit transitions; no assignment/workflow configuration, bulk operations, deletion UI, or custom-field form builder. The related-link ticket extends the same relationship UI afterward.
