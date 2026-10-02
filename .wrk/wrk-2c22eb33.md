---
id: wrk-2c22eb33
title: Implement stable release discovery and wrk upgrade --check
status: done
parent: wrk-8d5b1b84
priority: normal
labels:
  - upgrade
---
## Outcome

Users can inspect whether a newer stable wrk release is available without modifying their installation or project.

## Acceptance criteria

- [x] Add `wrk upgrade --check` and help/parser support, dispatching before project discovery and validation. Keep `wrk update` behavior unchanged.
- [x] Query the latest stable release of the fixed public `calebmchenry/wrk` repository over HTTPS with bounded response sizes, request timeouts, and clear diagnostics for unavailable releases, malformed responses, HTTP errors/rate limits, and offline operation.
- [x] Apply the semantic version/tag policy from [wrk-bab18df9](wrk-bab18df9.md); reject draft/prerelease candidates and malformed versions. Compare numerically, and never offer an implicit downgrade when the installed version is newer.
- [x] Resolve exactly one supported platform archive and its checksum manifest from the same release using the shared naming contract. Missing or ambiguous assets are explicit errors.
- [x] Report current version, latest version, and availability in human and JSON output. A successful check exits 0 whether an update exists or not; operational errors exit 1 and usage errors exit 2. For unknown/development versions, report the limitation without inventing an ordering.
- [x] `--check` performs no executable replacement, staging, local caching, or lock-file writes. All other ordinary commands remain offline.
- [x] Use injected HTTP fixtures to cover older/equal/newer installed versions, semantic-version ordering, unsupported platforms, missing releases/assets, malformed metadata, timeout/rate-limit responses, unknown build metadata, JSON/help, and execution outside or inside an invalid project. Tests must not depend on live GitHub availability.

## Prerequisites and handoff

Prerequisite: [wrk-bab18df9: version and asset contract](wrk-bab18df9.md). Packaging [wrk-f28fed55](wrk-f28fed55.md) can proceed alongside implementation after that contract exists. Public release publication is needed for the final live smoke check, not unit/integration development. Share release-resolution logic with the installer child of [wrk-8d5b1b84](wrk-8d5b1b84.md).

2026-10-02: Planned; implementation has not started. Prerequisites are body links pending CLI dependency support.

## Implementation progress — 2026-10-02

Implementation started for the authorized upgrade batch. Preserve the existing uncommitted metadata work. Use one strict stable version/asset contract, a fixed anonymous HTTPS release source, fixture-driven verification, and same-directory atomic executable replacement. Verify on macOS arm64 and Linux via Docker; record architecture runtime coverage separately. Publication follows implementation and all release gates.

### Implemented and verified

Implemented fixed-source HTTPS stable discovery and upgrade --check before project discovery. Enforced request deadlines, bounded metadata/download sizes, HTTPS redirects, stable release/tag validation, exact asset cardinality, and same repository/tag/filename URLs. Unknown builds report available:null with guidance. Checks make no filesystem changes; ordinary commands stay offline. Fixture tests cover version ordering/no downgrade, unknown metadata, unsupported platforms, missing/duplicate/invalid assets, HTTP errors/rate limits/offline/timeout, malformed/oversized responses, JSON/help, and invalid projects. macOS and Linux test/race/vet checks passed.
