---
id: wrk-f28fed55
title: Automate verified multi-platform release packaging
status: todo
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
