# Releases and upgrades

## Shared contract

Stable tags use exactly `vX.Y.Z`, without leading zeroes, suffixes, or build metadata.
Embedded release versions omit the `v`. Numeric comparisons have no integer overflow
limit. The public source is `calebmchenry/wrk`; keep published versions/assets immutable.
Correct a bad release by publishing a new higher version, not by replacing its assets
or moving its tag. The updater never downgrades a newer installed version.

Each release contains these four archives and one SHA-256 manifest:

```text
wrk_X.Y.Z_darwin_amd64.tar.gz
wrk_X.Y.Z_darwin_arm64.tar.gz
wrk_X.Y.Z_linux_amd64.tar.gz
wrk_X.Y.Z_linux_arm64.tar.gz
wrk_X.Y.Z_checksums.txt
```

Every archive contains exactly one regular executable, `wrk`, at the root, with
executable permissions. The manifest contains each archive's SHA-256 and exact
filename. `.goreleaser.yaml`, `internal/buildinfo`, the updater, and
`scripts/verify-release.py` enforce this contract. Changes to it need an explicit
upgrade compatibility plan before release.

Release builds use `CGO_ENABLED=0`, `-trimpath`, and linker metadata:

```text
-X wrk/internal/buildinfo.Version=X.Y.Z
-X wrk/internal/buildinfo.Commit=<full Git commit>
-X wrk/internal/buildinfo.Kind=release
```

Plain `go build`, `go install`, and `go run` remain development builds. Snapshots
have a `-snapshot` version and `snapshot` kind and are never published by the
stable workflow. The workflow rejects noncanonical tags; any separately managed
prerelease must remain marked prerelease and outside GitHub's latest stable release.
The updater also checks these conditions instead of trusting the endpoint alone.

## Verification and local packaging

Use Go 1.25+ and GoReleaser OSS v2.18.2 (also pinned in CI). From the repo root:

```sh
test -z "$(gofmt -l cmd internal test)"
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
go run ./cmd/wrk validate
goreleaser check
goreleaser release --snapshot --clean
python3 scripts/verify-release.py dist
```

Snapshot packaging may use a dirty tree; it is inspection evidence, not publishable
release evidence. The verifier checks all archive names, checksums, entries, platform
and build metadata, then executes the native asset's version and workspace smoke
checks. `scripts/verify-workspace.py` starts that extracted executable with an
empty PATH in a disposable project outside the checkout, fetches the HTML/CSS and
all imported modules, checks host/origin restrictions, creates and edits through
HTTP and the CLI, checks reciprocal links and stale-save rejection, and verifies
SIGTERM shutdown. The archive still contains only `wrk`; all UI assets are embedded.
Cross-compilation/metadata
inspection does not prove runtime correctness on another platform.
When verifying downloaded published artifacts with this script, check out their
release tag first: the provenance check compares each binary's commit to HEAD.

The reusable `Verify` workflow runs formatting, tests, race tests, vet, project
validation, and executable smoke tests on Linux amd64/arm64 and macOS amd64/arm64.
It also packages and verifies a snapshot. Integration tests build two identifiable
release CLIs and serve controlled HTTPS fixtures through a test-only Go build overlay.
They exercise the real parser/download/checksum/archive/probe/rename path, including
re-execution and no-op, invalid project formats, truncated downloads, integrity errors,
concurrent processes, interruption, and accurate post-publication error reporting.
Production binaries contain no fixture transport or alternate release endpoint.

The `browser` job runs the Node model tests and Chromium workflow suite on macOS
and Linux, retaining screenshots and failure traces. To run the same suite against
an extracted native archive locally, use
`WRK_BROWSER_BINARY=/absolute/path/to/extracted/wrk npm run test:browser` after
installing the locked development dependencies and Playwright Chromium.
See [workspace verification evidence](workspace-verification.md) for current
platform coverage and the distinction between real and injected failure checks.

For a local Linux check on a Mac with Docker, copy the source into the container's
local filesystem (do not treat a host bind mount as Linux filesystem evidence):

```sh
docker run --rm --init -v "$PWD:/source:ro" golang:1.25.4-bookworm \
  bash -c 'cp -a /source /tmp/wrk && cd /tmp/wrk && go test -count=1 ./... && go test -race -count=1 ./... && go vet ./... && go run ./cmd/wrk validate'
```

Use `--init` when testing subprocess interruption: a container whose PID 1 does
not reap orphaned children can leave dead grandchildren as zombies, making the
subprocess liveness checks misleading. For an already-running container, execute
the suite under `tini -s --` (a child subreaper). Run permission checks as an
unprivileged user; root bypasses the unwritable-directory test. Browser timing
assertions should run after concurrent builds finish, on the local filesystem.

## Publish a stable release

1. Finish implementation and controlled verification, update tickets/evidence, and
   review the source to publish. The release should include the upgrade command.
2. Commit and push the intended source. Confirm the complete `Verify` matrix passes.
   Ensure the proposed version is higher than the published latest stable version.
3. From a clean checkout, create and push the next canonical tag, for example:

   ```sh
   git tag -a v0.1.0 -m "wrk v0.1.0"
   git push origin v0.1.0
   ```

4. `Release` verifies the tagged source through the full reusable workflow. Only
   its publish job receives `contents: write`; it uses the built-in `GITHUB_TOKEN`
   and full Git history. No personal token or repository-visibility change is needed.
   GoReleaser builds with `--skip=publish`. The workflow verifies the bundle and
   runs each exact packaged native asset on all four runners. Only then does `gh`
   upload those verified artifacts to a draft and publish it as latest, combining
   `docs/release-notes.md` with GitHub-generated commit notes. Update that notes file
   for each new release. Failed uploads leave a draft outside the stable channel.
5. Inspect the workflow, release notes, five assets, and checksums. Download into a
   disposable directory, verify and run the binary's `version` and `upgrade --check`.
   Record actual runtime coverage separately from build coverage. Never replace a
   developer's installation merely to gather evidence.

If publication fails, inspect the draft/release and logs before retrying. Do not
delete or overwrite an already published release. Resolve an unpublished partial
draft deliberately. Preserve the release's tag/commit relationship and checksums.

No artificial stable versions are needed for the two-version test: it uses fixtures.
Binaries that predate `upgrade` need one manual installation from Releases.
Authentication for private releases, Windows, Homebrew distribution, pinned installs,
prerelease channels, and remote Go-module installation remain outside this release.

## References

- [GoReleaser build configuration](https://goreleaser.com/customization/builds/builders/go/)
- [Archive configuration](https://goreleaser.com/customization/package/archives/)
- [GoReleaser in GitHub Actions](https://goreleaser.com/customization/ci/actions/)
- [GitHub latest-release API](https://docs.github.com/en/rest/releases/releases#get-the-latest-release)
- [GitHub runner matrix](https://docs.github.com/en/actions/reference/runners/github-hosted-runners)
