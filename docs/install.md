# Installing wrk

Install a standalone release binary to use the CLI, browser workspace, and task
runner, or [build from source](../README.md#build-from-source).

## Install a release

Download a standalone binary from [GitHub Releases](https://github.com/calebmchenry/wrk/releases/latest).
Go is not required. macOS (`darwin`) and Linux are supported on `amd64` (Intel/x86-64)
and `arm64` (Apple Silicon/AArch64). To install on Apple Silicon, replace `X.Y.Z`
below with the version shown on the release page, without the leading `v`:

```sh
(
set -eu
wrk_version="X.Y.Z"
wrk_archive="wrk_${wrk_version}_darwin_arm64.tar.gz"
wrk_checksums="wrk_${wrk_version}_checksums.txt"
wrk_release_url="https://github.com/calebmchenry/wrk/releases/download/v${wrk_version}"
wrk_download_dir="$(mktemp -d)"
cd "$wrk_download_dir"
curl -fLO "$wrk_release_url/$wrk_archive"
curl -fLO "$wrk_release_url/$wrk_checksums"
awk -v archive="$wrk_archive" '$2 == archive' "$wrk_checksums" > selected-checksum.txt
test -s selected-checksum.txt
shasum -a 256 -c selected-checksum.txt
tar -xzf "$wrk_archive"
mkdir -p "$HOME/.local/bin"
install -m 755 wrk "$HOME/.local/bin/wrk"
"$HOME/.local/bin/wrk" version
)
```

For another platform, change `darwin_arm64` in `wrk_archive` to `darwin_amd64`,
`linux_amd64`, or `linux_arm64`. On Linux, `sha256sum -c selected-checksum.txt` also
works. Add `$HOME/.local/bin` to PATH. The checksum verifies the archive against
the trusted GitHub release; it is not an independent cryptographic signature.

```sh
wrk version                    # also: wrk --version
wrk upgrade --check            # inspect without writing files
wrk upgrade                    # install a newer stable release
wrk upgrade --check --json
```

These commands work outside a project and inside an invalid/newer-format project.
Only `upgrade` contacts an external service. Checks exit 0 whether a newer version exists
or not; failures exit 1 and invalid arguments exit 2. Upgrades never downgrade.
An unknown/development version reports availability as unknown; development,
snapshot, and `go run` builds must be installed manually. A binary predating the
upgrade command needs one manual installation of an upgrade-capable release.

Self-upgrade supports user-owned standalone binaries in writable directories.
It follows standalone symlinks and replaces their resolved target. Recognized
package-manager paths receive guidance to use that manager. It never invokes
sudo or chooses a different executable from PATH. `wrk update` still edits tickets.
For connectivity, rate-limit, checksum, permission, and interrupted-upgrade
troubleshooting, see the [upgrade contract](cli.md#version-and-upgrade).

## Build from source

Follow the [source installation steps](../README.md#build-from-source) with Go 1.25
or newer. There is no published Go module or package-manager install; run
`go install ./cmd/wrk` from a clone of this repository.

`go install` writes to `GOBIN` when set, otherwise the `bin` directory in `GOPATH`
(normally `$HOME/go/bin`). Put that directory on PATH. Dependency downloads are
needed at build time. The installed CLI and browser workspace work offline;
checking for or installing a release upgrade needs network access.

Source builds report a development version and do not self-upgrade. To update,
bring your checkout up to date and rerun `go install ./cmd/wrk`.

For a binary without installing it, run this from the checkout root:

```sh
go build -o ./bin/wrk ./cmd/wrk
./bin/wrk version
```
