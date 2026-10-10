# v0.1.0 verification evidence

This records the published v0.1.0 only. The later local browser workspace has
separate [snapshot/runtime verification evidence](workspace-verification.md);
those checks do not imply a new published release.

Source: `14242c8c700c7af77cd0d0c9a1c48f892d6ee7c7`.
Tag: `v0.1.0`. Verification date: 2026-10-02.

## Source and controlled upgrades

The [complete hosted verification run](https://github.com/calebmchenry/wrk/actions/runs/37075705677)
passed formatting, uncached tests, uncached race tests, vet, project validation,
and development-binary execution on every supported platform:

| Platform | Native hosted runner | Real two-version replacement |
| --- | --- | --- |
| Linux amd64 | `ubuntu-24.04` | Passed |
| Linux arm64 | `ubuntu-24.04-arm` | Passed |
| macOS amd64 | `macos-15-intel` | Passed |
| macOS arm64 | `macos-15` | Passed |

`TestReleaseUpgradeEndToEnd` builds versions 1.0.0 and 1.1.0 as disposable,
identifiable release fixtures. It checks availability, downloads over controlled
HTTPS, verifies checksums, extracts/probes/stages/replaces the actual running
binary, re-executes version 1.1.0, and confirms a subsequent no-op. It runs both
outside a project and inside an unsupported-format project. It also covers
truncated downloads, checksum mismatch, concurrent invocations, a killed process,
and a directory-sync error after publication with truthful committed output.
No artificial stable releases are published for this test.

Package-level tests cover strict version ordering, all four asset names,
malformed/draft/prerelease metadata, unsupported platforms, missing/ambiguous
assets, rate limits, network timeout/offline responses, bounds and HTTPS redirects,
archive traversal/link/duplicate/size/CRC rejection, candidate mismatch, permissions,
symlinks, changed targets, lock contention, no-ops, and cleanup. CLI tests cover
human/JSON/help/error contracts and project-independent dispatch.

The same complete test/race/vet/validation checks also passed locally on macOS
arm64 (Go 1.25.4) and Linux arm64 (Go 1.25.4 in Docker, source copied onto the
container's local filesystem). The Linux Docker root run skips the unwritable
directory unit test; the unprivileged hosted runners and local Mac exercise it.

## Packaging

GoReleaser OSS v2.18.2 configuration validation and local snapshot packaging passed.
`scripts/verify-release.py` verified all four archive names, each archive's sole
regular executable entry, SHA-256 checksums, platform/build provenance, and native
macOS arm64 execution. Cross-built artifacts were explicitly reported as such;
they were not counted as native runtime evidence.

The hosted snapshot package check passed as part of the linked verification run.
Workflow validation passed with actionlint v1.7.12.

## Published release and live checks

The [release workflow](https://github.com/calebmchenry/wrk/actions/runs/37076069709)
passed the complete tagged-source matrix, release packaging, checksum/metadata
validation, and native smoke execution of the exact release archives on all four
platforms. The publish job then uploaded those tested assets into a draft and
published [v0.1.0](https://github.com/calebmchenry/wrk/releases/tag/v0.1.0) as a stable
release. No repository visibility changes or developer installation replacements
were performed.

All five published assets were downloaded afresh and verified against the manifest.
`scripts/verify-release.py` confirmed the exact archive names, contents, platform
metadata, release version 0.1.0, and commit `14242c8c700c7af77cd0d0c9a1c48f892d6ee7c7`.
The downloaded binaries were extracted into disposable directories and executed:

| Live published-binary check | macOS arm64 | Linux arm64 |
| --- | --- | --- |
| `version --json` reports 0.1.0 / release / expected commit | Passed | Passed |
| `upgrade --check --json` against public GitHub | Passed | Passed |
| `upgrade --json` returns `changed: false` | Passed | Passed |
| `current_version` / `latest_version` both 0.1.0 | Passed | Passed |
| `available: false`, `project_root: null`, exit 0 | Passed | Passed |

The macOS live check ran inside a synthetic unsupported-format project; the Linux
live check ran with PATH set to an unavailable directory, demonstrating execution
without Go or helper tools on PATH. Hosted native asset smoke tests provide the
amd64 runtime evidence. The first release includes the upgrade command; older
binaries without it require the documented one-time manual bootstrap.
