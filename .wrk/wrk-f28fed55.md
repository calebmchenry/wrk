---
id: wrk-f28fed55
title: Automate verified multi-platform release packaging
status: done
parent: wrk-551ff6c2
priority: normal
labels:
  - upgrade
depends_on:
  - wrk-bab18df9
---
## Outcome

CI verifies the source and produces the exact binary archives and checksums consumed by `wrk upgrade`.

## Acceptance criteria

- [x] Add GitHub Actions verification for formatting, tests, race tests, vet, and `go run ./cmd/wrk validate` on macOS and Linux. Release publication must be gated on successful verification of the tagged source.
- [x] Add GoReleaser OSS configuration and a semantic-version-tag release workflow, using full Git history, the built-in GitHub token, and `contents: write` only for the publishing job.
- [x] Build `./cmd/wrk` with `CGO_ENABLED=0` for darwin/linux on amd64/arm64 and inject the version/commit/build-kind metadata from [wrk-bab18df9](wrk-bab18df9.md).
- [x] Package the four archives and SHA-256 manifest using the agreed names and contents. Publish release notes with the assets; stable tags are eligible for latest-release selection, while snapshots/prereleases are excluded from the stable upgrade path.
- [x] Verify snapshot packaging locally: inspect all four archives, check checksums and metadata, and retain reproducible commands/results. Distinguish cross-compilation from actual runtime coverage.
- [x] Document the maintainer build/tag/publish procedure and required permissions, and link it from `docs/index.md`.

## Prerequisites and handoff

Prerequisite: [wrk-bab18df9: version and asset contract](wrk-bab18df9.md). Actual first-release publication, installation smoke tests, and release bootstrap coordination remain in parent [wrk-551ff6c2](wrk-551ff6c2.md); this child owns automation and snapshot evidence.

2026-10-02: Created during backlog refinement; implementation has not started. The repository is public and `gh repo view` reported no latest release. Dependencies are body links until the CLI supports dependency creation/update.

## Implementation progress — 2026-10-02

Implementation started for the authorized upgrade batch. Preserve the existing uncommitted metadata work. Use one strict stable version/asset contract, a fixed anonymous HTTPS release source, fixture-driven verification, and same-directory atomic executable replacement. Verify on macOS arm64 and Linux via Docker; record architecture runtime coverage separately. Publication follows implementation and all release gates.

### Implemented and verified

Added GoReleaser OSS v2.18.2 configuration, full-history workflows, four-platform verification matrix, snapshot validation, and release artifact smoke gates before draft-to-stable publication. Only the final publication job has contents:write; all artifacts are built once and tested before publishing. Local goreleaser check and snapshot packaging passed: four archives contain exactly one regular root wrk, every checksum matches, all target/provenance metadata is present, and native macOS arm64 execution reports snapshot metadata. Source tests/race/vet/validation pass on macOS and Linux arm64. GitHub runner execution and first publication remain to be recorded.

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
