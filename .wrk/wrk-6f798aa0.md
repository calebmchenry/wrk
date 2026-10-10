---
id: wrk-6f798aa0
title: Verify and document the complete local browser and agent workflow
status: done
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

- [x] Exercise a packaged executable with embedded assets outside the source tree on supported macOS and Linux local filesystems; record versions/platforms and distinguish actual runtime checks from cross-compilation.
- [x] Verify default/nested-directory discovery, explicit project/config selection from elsewhere, relative-path semantics, port selection/occupation/0, --open behavior, and graceful shutdown.
- [x] Complete browser create/read/edit/status/labels/parent/dependency/related workflows and independently inspect their persisted files with the current CLI.
- [x] Observe external CLI edits live within the documented bound, plus direct body edits, atomic replacements, config changes, deletion, invalid data/repair, reconnect, and sleep/resume behavior where feasible.
- [x] Demonstrate draft preservation and stale-save rejection after agent edits, plus actionable validation, lock contention, and committed-error recovery.
- [x] Verify loopback-only serving, local host/origin enforcement, no cross-site mutation, safe Markdown, and no unintended remote asset/network requirements.
- [x] Run appropriate uncached Go unit/integration tests, race tests, vet, build and frontend/browser checks for the implemented stack; follow README's integration-cache guidance.
- [x] Update README, indexed agent docs, CLI/config/storage/ticket contracts, and release verification/package checks for embedded UI assets. Document that the server stays foreground/local and no agent runner or Git sync is included.
- [x] Run go run ./cmd/wrk validate and record concrete evidence plus any remaining limitations; close this ticket only when all child feature criteria are met.

## Implementation notes (2026-10-09)

- All seven prerequisite feature tickets are done; retain their existing uncommitted changes.
- Added a standalone packaged-workspace smoke verifier, now used by the release
  verifier and development-binary CI. It exercises embedded modules, API/CLI
  persistence, related links, stale writes, security, and shutdown with empty PATH.
- Added default/requested/occupied-port, relative-selector, real launcher
  success/failure, and SIGTERM executable tests. Added an actual separate-process
  writer-lock browser scenario and an optional exact packaged-binary browser input.
- Added macOS/Linux browser CI and expanded the user/agent workflow contracts.

## Verification completed (2026-10-09)

- [Complete evidence and reproduction commands](../docs/workspace-verification.md)
  record platforms, tool versions, archive hashes, requirement-level checks and
  limitations. All seven prerequisite feature tickets and their criteria are done.
- macOS 15.7.7 / arm64 / APFS and Debian 12 / Linux arm64 / local overlayfs:
  Go 1.25.4 uncached `go test -count=1 ./...` and `go test -race -count=1 ./...`,
  vet, build and formatting passed. Linux ran as UID 1000, including permission
  checks normally skipped under root. Executable lifecycle tests passed both
  platforms with default port 7331 available, both selectors, launcher stubs,
  exact requested/allocated ports, conflicts, Ctrl-C and SIGTERM.
- GoReleaser 2.18.2 check/snapshot packaging and four-archive checksum/metadata
  verification passed. Native macOS/Linux arm64 archives ran outside the source
  tree with embedded modules and no helper tools on PATH; full browser suites
  used those exact extracted binaries. amd64 assets were cross-built/inspected,
  not executed in this session. No release was published.
- Playwright 1.64.0 / Chromium 156.0.8078.4: 23/23 scenarios passed on macOS
  (29.0 s) and Linux (31.5 s), including real CLI persistence/live updates,
  actual writer contention, stale drafts, related links, and save recovery.
  Desktop and 390px editing screenshots from both systems were visually checked.
- Nine Node model/poller/draft tests and six Python runner tests passed on each
  platform; actionlint 1.7.12 and `git diff --check` passed. Linux interruption
  tests required a child reaper (`tini -s --`); documented Docker `--init` usage.
- An initial macOS reconnect assertion failed during concurrent builds; three
  isolated repetitions and the full packaged suite passed without a relaxed
  timeout or product change. Sleep/resume and committed errors are explicitly
  documented as simulated/injected, not physical sleep or disk-fault evidence.
- `go run ./cmd/wrk validate` reports all 26 tickets valid. No remaining work
  for this ticket; the parent milestone stays open for acceptance of the evidence.

## Completion boundary

This ticket verifies and documents the feature batch. Publishing a release/deploying anything is outside the current request. The parent milestone can be completed after this evidence is accepted and every required child is done.
