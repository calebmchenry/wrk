---
id: wrk-6f798aa0
title: Verify and document the complete local browser and agent workflow
status: todo
parent: wrk-a1561423
depends_on:
  - wrk-47b4e027
  - wrk-27b71a32
  - wrk-6387cfe7
  - wrk-fa8c7f1f
  - wrk-ce7350ff
  - wrk-52fbccc3
  - wrk-95118e5e
priority: normal
labels:
  - web
---

## Outcome

Demonstrate that a built wrk binary supplies a reliable local browser workspace alongside agent/CLI edits, and publish accurate user/agent documentation.

## Acceptance criteria

- [ ] Exercise a packaged executable with embedded assets outside the source tree on supported macOS and Linux local filesystems; record versions/platforms and distinguish actual runtime checks from cross-compilation.
- [ ] Verify default/nested-directory discovery, explicit project/config selection from elsewhere, relative-path semantics, port selection/occupation/0, --open behavior, and graceful shutdown.
- [ ] Complete browser create/read/edit/status/labels/parent/dependency/related workflows and independently inspect their persisted files with the current CLI.
- [ ] Observe external CLI edits live within the documented bound, plus direct body edits, atomic replacements, config changes, deletion, invalid data/repair, reconnect, and sleep/resume behavior where feasible.
- [ ] Demonstrate draft preservation and stale-save rejection after agent edits, plus actionable validation, lock contention, and committed-error recovery.
- [ ] Verify loopback-only serving, local host/origin enforcement, no cross-site mutation, safe Markdown, and no unintended remote asset/network requirements.
- [ ] Run appropriate uncached Go unit/integration tests, race tests, vet, build and frontend/browser checks for the implemented stack; follow README's integration-cache guidance.
- [ ] Update README, indexed agent docs, CLI/config/storage/ticket contracts, and release verification/package checks for embedded UI assets. Document that the server stays foreground/local and no agent runner or Git sync is included.
- [ ] Run go run ./cmd/wrk validate and record concrete evidence plus any remaining limitations; close this ticket only when all child feature criteria are met.

## Completion boundary

This ticket verifies and documents the feature batch. Publishing a release/deploying anything is outside the current request. The parent milestone can be completed after this evidence is accepted and every required child is done.
