---
id: wrk-f28fed55
title: Automate verified multi-platform release packaging
status: in-progress
parent: wrk-551ff6c2
priority: normal
labels:
  - upgrade
---
## Outcome

CI verifies the source and produces the exact binary archives and checksums consumed by `wrk upgrade`.

## Acceptance criteria

- [ ] Add GitHub Actions verification for formatting, tests, race tests, vet, and `go run ./cmd/wrk validate` on macOS and Linux. Release publication must be gated on successful verification of the tagged source.
- [ ] Add GoReleaser OSS configuration and a semantic-version-tag release workflow, using full Git history, the built-in GitHub token, and `contents: write` only for the publishing job.
- [ ] Build `./cmd/wrk` with `CGO_ENABLED=0` for darwin/linux on amd64/arm64 and inject the version/commit/build-kind metadata from [wrk-bab18df9](wrk-bab18df9.md).
- [ ] Package the four archives and SHA-256 manifest using the agreed names and contents. Publish release notes with the assets; stable tags are eligible for latest-release selection, while snapshots/prereleases are excluded from the stable upgrade path.
- [ ] Verify snapshot packaging locally: inspect all four archives, check checksums and metadata, and retain reproducible commands/results. Distinguish cross-compilation from actual runtime coverage.
- [ ] Document the maintainer build/tag/publish procedure and required permissions, and link it from `docs/index.md`.

## Prerequisites and handoff

Prerequisite: [wrk-bab18df9: version and asset contract](wrk-bab18df9.md). Actual first-release publication, installation smoke tests, and release bootstrap coordination remain in parent [wrk-551ff6c2](wrk-551ff6c2.md); this child owns automation and snapshot evidence.

2026-10-02: Created during backlog refinement; implementation has not started. The repository is public and `gh repo view` reported no latest release. Dependencies are body links until the CLI supports dependency creation/update.

## Implementation progress — 2026-10-02

Implementation started for the authorized upgrade batch. Preserve the existing uncommitted metadata work. Use one strict stable version/asset contract, a fixed anonymous HTTPS release source, fixture-driven verification, and same-directory atomic executable replacement. Verify on macOS arm64 and Linux via Docker; record architecture runtime coverage separately. Publication follows implementation and all release gates.

### Implemented and verified

Added GoReleaser OSS v2.18.2 configuration, full-history workflows, four-platform verification matrix, snapshot validation, and release artifact smoke gates before draft-to-stable publication. Only the final publication job has contents:write; all artifacts are built once and tested before publishing. Local goreleaser check and snapshot packaging passed: four archives contain exactly one regular root wrk, every checksum matches, all target/provenance metadata is present, and native macOS arm64 execution reports snapshot metadata. Source tests/race/vet/validation pass on macOS and Linux arm64. GitHub runner execution and first publication remain to be recorded.
