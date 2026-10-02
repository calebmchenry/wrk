---
id: wrk-551ff6c2
title: Publish versioned wrk binaries on GitHub Releases
status: done
priority: normal
labels:
  - upgrade
---
## Outcome

Users can download a versioned wrk executable from GitHub Releases and run it without installing Go. Stable releases expose the version metadata, platform archives, and checksums needed by [wrk upgrade](wrk-8d5b1b84.md).

## Proposed approach

Use GitHub Actions plus GoReleaser OSS. A pushed semantic-version tag (initially v0.1.0) runs verification, builds ./cmd/wrk, and publishes archives, SHA-256 checksums, and release notes. Target darwin/linux on amd64/arm64 with CGO_ENABLED=0. Keep Windows outside this release because filesystem/locking implementations currently only support macOS and Linux.

Add .goreleaser.yaml, a CI workflow, a tag-triggered release workflow, version reporting, and README download/install instructions. Give only the release job contents: write and use the built-in GITHUB_TOKEN. Fetch full Git history for GoReleaser. Verify snapshot packaging locally before the first published release. Homebrew distribution can follow later.

## Acceptance criteria

- [x] Publish the initial source commit to calebmchenry/wrk, preserving its existing visibility, and configure main to track origin/main.
- [x] Complete [version reporting and the shared release asset contract](wrk-bab18df9.md), including project-independent version commands and identifiable release/development builds.
- [x] Complete [automated release packaging](wrk-f28fed55.md): formatting, test, race, vet, and project validation gates; macOS/Linux archives for amd64/arm64; SHA-256 checksums; and version/commit metadata.
- [x] Verify local snapshot packaging and smoke-test installation/execution on supported platforms; distinguish cross-compilation from runtime verification.
- [x] Publish the first versioned GitHub Release with downloadable assets and release notes.
- [x] Coordinate the first upgrade-capable stable release with [the upgrade deliverable](wrk-8d5b1b84.md) after its command implementation and controlled end-to-end verification are complete. Prefer including the command in the first release; if a release predates it, document the one-time manual bootstrap installation.
- [x] Verify published asset names/checksums against the shared contract, download/install a published binary, confirm its version, and smoke-test its live upgrade check without replacing a developer's working installation.
- [x] Document download, checksum verification, installation, and maintainer release steps; link the maintainer documentation from docs/index.md.

## Findings and handoff

2026-10-02: Reviewed docs/index.md, README.md, go.mod, platform constraints, existing tickets, and official GitHub/GoReleaser documentation. At initial review, this checkout had no Git commits or remote. There are still no .github workflows or GoReleaser configuration. The current module path is wrk. A canonical github.com/<owner>/<repo> module path plus updated internal imports is needed if we also offer remote go install; it is not required for downloadable binaries. Prior execution evidence covers macOS arm64 and Linux arm64; amd64 runtime verification is still outstanding.

The user authorized creating the initial commit and pushing main to the existing repository at https://github.com/calebmchenry/wrk.git. Initial commit d43b393 (Initial commit) was pushed successfully; main now tracks origin/main. Repository visibility was unchanged. Release automation and binary publication remain future work, so this ticket remains in progress.

Verification: go test ./... and git diff --cached --check passed, and go run ./cmd/wrk validate passed with 8 tickets. Runtime locks, build output, and sprint logs are covered by the existing ignore rules.

## Implementation work items and release coordination

2026-10-02: The user requested amendment of the related work items and a complete implementation list. The earlier feasibility notes are now captured as actionable tickets. `gh repo view calebmchenry/wrk --json isPrivate,url,latestRelease` confirmed a public repository with no latest release. Preserve that visibility; anonymous access is sufficient for the initial updater scope.

This release ticket retains ownership of publication, live installation evidence, and release documentation. Its implementation children are:

1. [wrk-bab18df9: Version reporting and release asset contract](wrk-bab18df9.md).
2. [wrk-f28fed55: Verified multi-platform release packaging](wrk-f28fed55.md), after the contract is defined.

The companion [wrk-8d5b1b84: Upgrade deliverable](wrk-8d5b1b84.md) owns [stable release checking](wrk-2c22eb33.md), [safe installation](wrk-f1f845bd.md), and [end-to-end verification and upgrade documentation](wrk-af501027.md). Packaging and upgrade implementation may proceed after the common contract exists. Controlled tests use release fixtures before publication; no live release is needed to implement the commands. Publish the first upgrade-capable release after these checks pass, then record live download/install/version/check evidence here. Do not create artificial stable releases just for a two-version test.

All seven related tickets carry the `upgrade` label: `go run ./cmd/wrk list --all --label upgrade`. Prerequisites are explicit in bodies because the current CLI cannot create/update dependency edges; do not infer ordering from `list --ready` alone or hand-edit frontmatter. The existing [relationship update ticket](wrk-1a09af55.md) tracks that CLI gap and is not a prerequisite for implementing upgrades.

Remaining work is implementation of the listed tickets and the final release steps. This backlog amendment does not mark any implementation complete; the release parent remains in progress. The local module path remains `wrk`; remote Go-module installation, Homebrew distribution, Windows, private release authentication, and prerelease/downgrade channels are outside this batch.

Backlog verification: `go run ./cmd/wrk validate` passed with 15 tickets. CLI `show --json` confirmed the seven batch members' statuses, parent relationships, and `upgrade` labels; all local links in their bodies resolve. `git diff --check` passed for the amended tracked files. No feature implementation or publication was performed in this refinement.

## References

- [GitHub Releases](https://docs.github.com/en/repositories/releasing-projects-on-github/about-releases)
- [GoReleaser with GitHub Actions](https://goreleaser.com/customization/ci/actions/)
- [GoReleaser quick start and snapshot builds](https://goreleaser.com/getting-started/quick-start/)
- [GoReleaser checksums](https://goreleaser.com/customization/package/checksum/)

## Implementation progress — 2026-10-02

Implementation started for the authorized upgrade batch. Preserve the existing uncommitted metadata work. Use one strict stable version/asset contract, a fixed anonymous HTTPS release source, fixture-driven verification, and same-directory atomic executable replacement. Verify on macOS arm64 and Linux via Docker; record architecture runtime coverage separately. Publication follows implementation and all release gates.

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
