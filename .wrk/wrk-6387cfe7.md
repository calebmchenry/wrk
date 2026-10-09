---
id: wrk-6387cfe7
title: Serve one local project with bundled UI assets and read APIs
status: todo
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

- [ ] Reuse project selection/discovery and fix the selected project for the process lifetime. Do not initialize missing projects or accept request-supplied filesystem roots.
- [ ] Bind only to 127.0.0.1. Use default port 7331, allow --port 1..65535 and --port 0, reject invalid values, and report port conflicts without silently switching.
- [ ] Print the actual usable URL and absolute project path. Implement opt-in --open after successful listening; browser-launch failure leaves the server running with a usable URL.
- [ ] Bundle a minimal UI shell and all runtime assets into the binary, with no external asset fetches or separate runtime installation.
- [ ] Provide structured project/list/item reads with IDs, item revisions, config-driven information, derived hierarchy/blockers, and actionable diagnostics using shared loading/validation.
- [ ] Match existing strict validation: do not present a partially invalid project as a healthy result. Surface startup failures clearly; runtime diagnostic/recovery display is completed with live updates.
- [ ] Establish HTTP request/resource limits and cancellation; graceful Ctrl-C shutdown closes listeners and active streams without holding a writer lock for server lifetime.
- [ ] Restrict Host/origin handling to the local app and define same-origin mutation protection for the editing endpoints. Do not expose arbitrary filesystem content or permissive cross-origin APIs.
- [ ] Define and document serve's long-running --json/startup/error/shutdown output behavior explicitly while retaining existing command output contracts.
- [ ] Test binding/ports, unrelated-cwd selection, graceful shutdown, request isolation/invalid IDs, invalid projects, host/origin rejection, and a packaged-binary asset smoke.

## Implementation context

Call Go project/store services directly; do not shell out per HTTP request. The UI ticket owns complete browsing views; the live-update ticket owns refresh delivery. CLI flags and docs should make local-only operation clear.
