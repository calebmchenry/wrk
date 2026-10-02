---
id: wrk-f1f845bd
title: Implement verified and atomic wrk self-upgrade installation
status: todo
parent: wrk-8d5b1b84
priority: normal
labels:
  - upgrade
---
## Outcome

`wrk upgrade` installs a newer official stable release safely into the standalone executable's existing location.

## Acceptance criteria

- [ ] Reuse [wrk-2c22eb33: release discovery](wrk-2c22eb33.md), skip replacement when already current or newer, and enforce the standalone official-release build policy. Reject replacement for development/`go run` and recognized package-manager builds with appropriate installation guidance.
- [ ] Locate the running executable and resolve supported standalone symlinks without replacing the symlink itself. Detect broken/changed targets and permission problems before publication. Do not invoke sudo or silently select a different PATH installation.
- [ ] Download the exact archive and manifest from the selected release over HTTPS with timeouts and size bounds; verify SHA-256 before extracting or executing the candidate. Missing/malformed checksums and mismatches fail closed. Document that checksums rely on the trusted release source rather than providing an independent signature.
- [ ] Extract only the expected regular `wrk` file, rejecting unsafe archive entries, links, duplicate executable entries, and oversized/decompression-abuse inputs. Verify the candidate reports the expected release version before replacement.
- [ ] Stage on the destination filesystem, set appropriate executable permissions, and atomically publish without truncating the running binary. All pre-publication failures leave the previous installation intact and clean up owned temporary files.
- [ ] Serialize concurrent upgrades for the resolved installation independently of `.wrk` locks, recheck the destination before replacement, and avoid replacing a binary already changed by another process or downgrading after a concurrent upgrade.
- [ ] Report old/new versions, destination, and whether installation changed in human and JSON output. Distinguish pre-publication failure from errors after replacement so a retry cannot falsely assume nothing happened. Preserve the existing 0/1/2 exit-code conventions and do not prompt.
- [ ] Add focused tests for checksum/download/extraction failures, candidate-version mismatch, unwritable directories, symlinks, concurrent replacement, same/newer-version no-ops, cleanup, and publication reporting. Real macOS/Linux replacement verification is owned by the verification child.

## Prerequisites and handoff

Prerequisites: [wrk-bab18df9](wrk-bab18df9.md) and [wrk-2c22eb33](wrk-2c22eb33.md). Use local fixtures during development; no published release or globally installed wrk needs to be overwritten for tests. Release packaging is [wrk-f28fed55](wrk-f28fed55.md).

2026-10-02: Planned; implementation has not started. Prerequisites are body links pending CLI dependency support.
