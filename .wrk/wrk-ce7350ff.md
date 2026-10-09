---
id: wrk-ce7350ff
title: Reflect agent and filesystem changes live in the browser
status: todo
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

- [ ] Detect ticket creation/update/deletion and .wrk/config.yaml changes made outside the server, including CLI atomic replacement and direct body edits; update visible views within two seconds under normal local operation.
- [ ] Choose and document a modest polling or filesystem-notification approach with bounded resources. Coalesce rapid changes and ignore lock/staging/runtime files; no per-file event history is promised.
- [ ] Refresh list membership, status, labels, selected details, hierarchy, blockers, progress, and config-derived display from current validated project state.
- [ ] Close the initial-load/subscription race and resynchronize from a full current snapshot after reconnect, sleep/resume, or dropped notifications. Show connection/reconnecting state.
- [ ] Preserve filter/search/selection/scroll context. Define a dirty-draft interface that refreshes surrounding data but never overwrites unsaved form content; editing wires it into actual forms.
- [ ] When a selected item is removed or no longer matches filters, show an understandable state without silently discarding a draft.
- [ ] Surface invalid/intermediate file states clearly, label any retained last-valid snapshot as stale, avoid claiming health while invalid, and recover automatically after repair.
- [ ] Do not take a writer lock for watching, mutate files while reading, or imply consistency beyond the existing per-item publication boundary.
- [ ] Add deterministic detection/reconnect/recovery tests and a real browser-plus-separate-CLI scenario covering create, status, label, parent, dependency, and body changes.

## Verification example

Start serve, open a selected item, then use the checkout CLI in another process to move it todo -> in-progress -> done and change labels/relationships. Confirm updates appear within the normal-operation bound without a page reload. Also test atomic replacement, malformed YAML followed by repair, reconnection, and ignored staging churn.
