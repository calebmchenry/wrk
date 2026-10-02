Standalone wrk binaries for macOS and Linux on amd64 and arm64. Go is not required.

- Track local project work with validated Markdown tickets, scopes, labels, and metadata updates.
- Inspect build provenance with `wrk version` or `wrk --version`.
- Check for a newer stable release with `wrk upgrade --check`.
- Install a verified release with `wrk upgrade`, using checksum verification and atomic replacement.
- Version and upgrade commands work without a project, including inside invalid or newer-format projects.

Verify your platform archive against the accompanying SHA-256 manifest before
manual installation. Binaries predating the upgrade command need one manual install.
See [installation instructions](https://github.com/calebmchenry/wrk#install-a-release)
and the [upgrade contract](https://github.com/calebmchenry/wrk/blob/main/docs/cli.md#version-and-upgrade).

Self-upgrade supports user-owned standalone release binaries. Development builds
and package-manager installations need their corresponding install method.
Releases are stable-only; the updater does not downgrade an installed version.
