---
id: wrk-551ff6c2
title: Publish versioned wrk binaries on GitHub Releases
status: in-progress
priority: normal
labels: []
---
## Outcome

Users can download a versioned wrk executable from GitHub Releases and run it without installing Go.

## Proposed approach

Use GitHub Actions plus GoReleaser OSS. A pushed semantic-version tag (initially v0.1.0) runs verification, builds ./cmd/wrk, and publishes archives, SHA-256 checksums, and release notes. Target darwin/linux on amd64/arm64 with CGO_ENABLED=0. Keep Windows outside this release because filesystem/locking implementations currently only support macOS and Linux.

Add .goreleaser.yaml, a CI workflow, a tag-triggered release workflow, version reporting, and README download/install instructions. Give only the release job contents: write and use the built-in GITHUB_TOKEN. Fetch full Git history for GoReleaser. Verify snapshot packaging locally before the first published release. Homebrew distribution can follow later.

## Acceptance criteria

- [ ] Establish the GitHub owner/repository and intended visibility, make the initial source commit, and configure/push the remote.
- [ ] Add automated formatting, test, race, vet, and project validation checks on supported platforms; gate releases on passing checks.
- [ ] Produce macOS and Linux archives for amd64 and arm64, with checksums and identifiable version/commit metadata.
- [ ] Verify local snapshot packaging and smoke-test installation/execution on supported platforms; distinguish cross-compilation from runtime verification.
- [ ] Publish the first versioned GitHub Release with downloadable assets and release notes.
- [ ] Document download, checksum verification, installation, and maintainer release steps; link the maintainer documentation from docs/index.md.

## Findings and handoff

2026-10-02: Reviewed docs/index.md, README.md, go.mod, platform constraints, existing tickets, and official GitHub/GoReleaser documentation. This checkout has no Git commits or remote, no .github workflows, and no GoReleaser configuration. The current module path is wrk. A canonical github.com/<owner>/<repo> module path plus updated internal imports is needed if we also offer remote go install; it is not required for downloadable binaries. Prior execution evidence covers macOS arm64 and Linux arm64; amd64 runtime verification is still outstanding.

The user authorized creating the initial commit and pushing main to the existing repository at https://github.com/calebmchenry/wrk.git. The origin remote is configured and currently has no branches. Preserve the existing repository visibility. Initial source publication is in progress; release automation and binary publication remain future work.

Pre-commit verification: go test ./... passed, and go run ./cmd/wrk validate passed with 8 tickets. Runtime locks, build output, and sprint logs are covered by the existing ignore rules. Record the push result here after publishing the initial commit.

## References

- [GitHub Releases](https://docs.github.com/en/repositories/releasing-projects-on-github/about-releases)
- [GoReleaser with GitHub Actions](https://goreleaser.com/customization/ci/actions/)
- [GoReleaser quick start and snapshot builds](https://goreleaser.com/getting-started/quick-start/)
- [GoReleaser checksums](https://goreleaser.com/customization/package/checksum/)
