---
id: wrk-2c22eb33
title: Implement stable release discovery and wrk upgrade --check
status: done
parent: wrk-8d5b1b84
priority: normal
labels:
  - upgrade
depends_on:
  - wrk-bab18df9
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

### Completion evidence — 2026-10-02

Completed this item as part of the seven-ticket upgrade batch. Source commit
`14242c8c700c7af77cd0d0c9a1c48f892d6ee7c7` is published as stable
[v0.1.0](https://github.com/calebmchenry/wrk/releases/tag/v0.1.0).
[Source verification](https://github.com/calebmchenry/wrk/actions/runs/37075705677)
and [tagged release gates](https://github.com/calebmchenry/wrk/actions/runs/37076069709)
passed on native macOS/Linux amd64/arm64 runners, including the controlled
two-version executable replacement tests. The exact four packaged release assets
were checksum/metadata verified and natively executed on all four platforms before
publication. Freshly downloaded assets passed checksum/contract verification;
published macOS arm64 and Linux arm64 binaries reported the expected version/commit,
queried live GitHub successfully, and returned an unchanged upgrade no-op. No
developer installation was replaced and no artificial stable test releases were
published. Repository visibility is unchanged.

See [durable verification evidence](../docs/release-verification.md),
[maintainer procedure](../docs/releases.md), and the
[CLI contract](../docs/cli.md#version-and-upgrade). Local formatting, uncached tests,
race tests, vet, build, actionlint v1.7.12, GoReleaser v2.18.2 validation/snapshot
packaging, and project validation passed. Remaining work for this item: none.

2026-10-02 relationship follow-up: the prerequisites listed above are now encoded through `wrk update --add-dependency`, delivered by [wrk-1a09af55](wrk-1a09af55.md). All referenced prerequisites are done; this completed ticket has no dependency blockers. Earlier body-only dependency notes describe the pre-implementation state.
