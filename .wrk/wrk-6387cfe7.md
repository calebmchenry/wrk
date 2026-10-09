---
id: wrk-6387cfe7
title: Serve one local project with bundled UI assets and read APIs
status: done
parent: wrk-a1561423
depends_on:
  - wrk-47b4e027
  - wrk-27b71a32
priority: normal
labels:
  - web
---

## Outcome

wrk serve starts a self-contained local HTTP server for one selected project, ready for browsing and live updates.

## Acceptance criteria

- [x] Reuse project selection/discovery and fix the selected project for the process lifetime. Do not initialize missing projects or accept request-supplied filesystem roots.
- [x] Bind only to 127.0.0.1. Use default port 7331, allow --port 1..65535 and --port 0, reject invalid values, and report port conflicts without silently switching.
- [x] Print the actual usable URL and absolute project path. Implement opt-in --open after successful listening; browser-launch failure leaves the server running with a usable URL.
- [x] Bundle a minimal UI shell and all runtime assets into the binary, with no external asset fetches or separate runtime installation.
- [x] Provide structured project/list/item reads with IDs, item revisions, config-driven information, derived hierarchy/blockers, and actionable diagnostics using shared loading/validation.
- [x] Match existing strict validation: do not present a partially invalid project as a healthy result. Surface startup failures clearly; runtime diagnostic/recovery display is completed with live updates.
- [x] Establish HTTP request/resource limits and cancellation; graceful Ctrl-C shutdown closes listeners and active streams without holding a writer lock for server lifetime.
- [x] Restrict Host/origin handling to the local app and define same-origin mutation protection for the editing endpoints. Do not expose arbitrary filesystem content or permissive cross-origin APIs.
- [x] Define and document serve's long-running --json/startup/error/shutdown output behavior explicitly while retaining existing command output contracts.
- [x] Test binding/ports, unrelated-cwd selection, graceful shutdown, request isolation/invalid IDs, invalid projects, host/origin rejection, and a packaged-binary asset smoke.

## Implementation context

Call Go project/store services directly; do not shell out per HTTP request. The UI ticket owns complete browsing views; the live-update ticket owns refresh delivery. CLI flags and docs should make local-only operation clear.

## Implementation decisions (2026-10-08)

- Reuse project resolution once at startup and shared strict loading for each read. Serve only embedded, explicitly named assets and fixed project/item API routes.
- Accept the exact printed loopback authority and same origin; reject cross-origin/fetch-metadata requests. Reserve a required same-origin Origin plus JSON content type for future mutation handlers.
- Use newline-delimited existing CLI envelopes for serve lifecycle events in JSON mode, with ordinary single-envelope failures before startup. Browser-launch failures are nonfatal warning events.
- Bound HTTP headers, request bodies, concurrent reads, read duration, and project input sizes; cancel reads/shutdown cooperatively without acquiring a store writer lock.
- Implementation and verification are complete. Full browsing, automatic refresh, and editing remain in their existing follow-up tickets.


## Implementation and verification (2026-10-08)

- Added `wrk serve`, default port 7331, strict `--port 0..65535`, opt-in `--open`, reusable project/config selection, printed absolute project root and actual URL, and signal-driven shutdown. JSON lifecycle events are `started`, optional nonfatal `warning`, and `stopped` or `error`; startup failures retain the ordinary single-envelope contract.
- Added `internal/web` with an embedded HTML/CSS/JavaScript shell and project/list/item read APIs. Reads expose shared summaries/revisions, exact body/source, children/blockers, validated defaults, custom-field definitions, and status/priority vocabularies. List filters reuse shared project semantics. Unknown custom YAML remains lossless in source.
- Enforced the exact loopback Host and same-origin request policy, required Origin/JSON protection for future unsafe methods, explicit asset routes, no CORS, CSP/frame/nosniff headers, no filesystem serving, and no request-provided roots. All current endpoints are read-only.
- Added bounded/cancellable loading through the shared project loader while preserving ordinary CLI behavior. HTTP limits, four concurrent reads, input-size/inventory limits, cooperative cancellation, strict runtime diagnostics, and shutdown are documented in `docs/cli.md`. No writer lock is held while serving.
- Focused tests cover default/explicit/allocated ports and conflicts; invalid/repeated flags; nested and unrelated cwd with both selectors; a newly created nearer project after startup; concurrent CLI edits; hierarchy/blockers/revision/body/config data; invalid config/tickets and recovery; malicious paths/IDs/roots; Host/Origin/Fetch Metadata restrictions; future mutation guards; resource limits; cancellation; active-stream shutdown; and nonfatal browser-launch failure while the HTTP URL is already usable.
- Compiled integration tests archive the executable as the sole `wrk` tar.gz entry, extract it away from the checkout, and request its embedded HTML/JS/CSS without runtime asset files. Process tests verify newline-delimited lifecycle output and Ctrl-C listener closure. This is a controlled packaged-binary smoke, not a published release or GoReleaser run.
- macOS arm64, Go 1.25.4: `gofmt -l cmd internal test` clean; `go test -count=1 ./...`, `go test -race -count=1 ./...`, `go vet ./...`, `go build -o ./bin/wrk ./cmd/wrk`, `node --check internal/web/assets/app.js`, and `git diff --check` passed on the final implementation.
- Linux arm64, Go 1.25.4 (`golang:1.25.4-bookworm`): copied cmd/internal/test/testdata/go.mod/go.sum from a read-only source mount into the container's local `/work`; formatting, both uncached full suites, vet, and build passed on the final implementation. This exercised the Linux runtime and filesystem rather than only cross-compilation.
- Manual binary startup on this repository reported the selected root and OS-assigned loopback URL; Ctrl-C emitted `stopped` and exited 0. Browser visual verification was attempted but Chrome blocked the loopback page with `ERR_BLOCKED_BY_CLIENT`; no browser policy was bypassed. HTTP/asset and JavaScript syntax checks passed; visual browser verification remains part of the later full browser workflow ticket.
- Updated README, CLI/configuration contracts, and the docs index. No remaining implementation work in this ticket.
