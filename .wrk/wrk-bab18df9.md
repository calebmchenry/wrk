---
id: wrk-bab18df9
title: Add version reporting and define the release asset contract
status: done
parent: wrk-551ff6c2
priority: normal
labels:
  - upgrade
---
## Outcome

Release binaries identify their version and build provenance, and packaging and the updater share an explicit asset contract.

## Acceptance criteria

- [x] Add `wrk version` and `wrk --version`, including the existing JSON envelope, without project discovery, ticket validation, or network access. Work outside a project and inside an invalid or newer-format project.
- [x] Embed semantic version, commit, and build kind in release builds; distinguish release, snapshot, and local development builds. Development builds must not masquerade as an installed stable release.
- [x] Document and implement one tag/version normalization policy: tags use `vX.Y.Z`; stable archive names use `wrk_X.Y.Z_<darwin|linux>_<amd64|arm64>.tar.gz`; the checksum manifest is `wrk_X.Y.Z_checksums.txt`.
- [x] Each archive contains one regular executable named `wrk` at its root; the manifest records SHA-256 for every published archive. The packaging configuration and updater must use this same contract.
- [x] Verify version output for local and injected release builds, human/JSON forms, help/invalid arguments, and all four platform asset names. Update the CLI contract and release documentation.

## Prerequisites and handoff

No implementation prerequisites. This is a child of the existing release deliverable. The release automation and upgrade-check tickets consume its contract. Keep the local module path unchanged; remote `go install ...@latest` support is outside this work.

2026-10-02: Created during backlog refinement. Implementation has not started. All related tickets carry the `upgrade` label. Dependency mutation is not supported by the current CLI, so prerequisites are recorded in bodies rather than hand-edited frontmatter.

## Implementation progress — 2026-10-02

Implementation started for the authorized upgrade batch. Preserve the existing uncommitted metadata work. Use one strict stable version/asset contract, a fixed anonymous HTTPS release source, fixture-driven verification, and same-directory atomic executable replacement. Verify on macOS arm64 and Linux via Docker; record architecture runtime coverage separately. Publication follows implementation and all release gates.

### Implemented and verified

Implemented internal/buildinfo and project-independent version/--version in human and JSON output. Stable tags are exactly vX.Y.Z; stable metadata uses X.Y.Z, full commit, and release kind. Development and snapshot builds remain identifiable. All four archive names and manifest contract are shared with the updater and GoReleaser. Tests cover numeric ordering, invalid tags, all platform names, injected release builds, default development builds, parser errors, and invalid/newer-format projects. macOS and Linux test/race/vet gates passed; snapshot archive metadata and native execution verified.
