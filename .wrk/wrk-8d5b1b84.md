---
id: wrk-8d5b1b84
title: Support upgrading wrk to the latest stable release
status: done
priority: normal
labels:
  - upgrade
---
## Outcome

Users with a standalone release binary can run `wrk upgrade --check` to inspect the latest stable release and `wrk upgrade` to install it without Go or a wrk project.

## Scope and decisions

Target macOS and Linux on amd64/arm64, matching [wrk-551ff6c2: GitHub Releases](wrk-551ff6c2.md). Use the public `calebmchenry/wrk` release source and anonymous HTTPS requests. Keep `wrk update` for ticket edits. Support human and JSON output without interactive prompts, and keep ordinary ticket commands offline.

The first implementation covers standalone official release binaries. Development/`go run` builds and recognized package-manager installations receive actionable guidance. Windows, private-repository authentication, background checks, prerelease channels, pinned-version installation, downgrades/force flags, Homebrew distribution, and remote Go-module installation are deferred.

## Acceptance criteria

- [x] Version metadata and release asset names are shared with [wrk-bab18df9](wrk-bab18df9.md).
- [x] `wrk upgrade --check` correctly reports stable-release availability without writing local files.
- [x] `wrk upgrade` downloads and verifies the correct asset, safely replaces the installed executable, and is a no-op when already current or newer.
- [x] Permission, installation-method, network, integrity, and concurrent-upgrade failures have deterministic diagnostics and preserve the existing binary before publication.
- [x] Upgrade/version commands work independently of project discovery, including inside invalid or unsupported-format projects, with the established JSON and exit-code conventions.
- [x] Automated failure coverage, macOS/Linux runtime replacement evidence, user documentation, and release bootstrap instructions are complete.
- [x] An upgrade-capable stable release is downloadable, and a controlled two-version upgrade has been verified. Do not close this parent solely because its implementation children are done.

## Implementation children

1. [wrk-2c22eb33: Stable release discovery and upgrade check](wrk-2c22eb33.md), after the shared version/asset contract.
2. [wrk-f1f845bd: Verified and atomic installation](wrk-f1f845bd.md), after release checking.
3. [wrk-af501027: End-to-end verification and documentation](wrk-af501027.md), after the commands and packaging are available.

The two release-foundation tickets remain children of [wrk-551ff6c2](wrk-551ff6c2.md). List this entire cross-parent batch with `go run ./cmd/wrk list --all --label upgrade`.

## Prerequisites and sequencing

Release metadata comes from [wrk-bab18df9](wrk-bab18df9.md); packaging comes from [wrk-f28fed55](wrk-f28fed55.md). Upgrade implementation can use local release fixtures before publication. Final delivery also requires [wrk-551ff6c2](wrk-551ff6c2.md). Publication is not a prerequisite for implementing or testing the command, avoiding a circular gate.

2026-10-02: User requested amended work items and an implementation list. This is a planned deliverable; implementation has not started. Prerequisites are recorded in ticket bodies because the current CLI has no dependency mutation command. `list --ready` does not enforce these body links; read them before selection.

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
