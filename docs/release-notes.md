Standalone wrk binaries for macOS and Linux on amd64 and arm64. Go is not required.

## New in v0.2.0

- Open a bundled local browser workspace with `wrk serve --open`. Search and filter
  tickets, expand parent/child rows, create tickets and inline children, and edit
  details directly. Agent and filesystem changes appear automatically, with
  conflict detection to protect unsaved drafts.
- Run a command for a ticket or filtered scope with `wrk run`. Continuous polling,
  expected-status checks, per-ticket logs, progress output, and JSON events support
  agent workflows. The Codex ticket-burn wrapper now uses this runner.
- Create and update parent relationships, dependencies, related-ticket links,
  priorities, labels, and typed custom fields through the CLI.
- Replace ticket bodies with `--body-file` and select projects explicitly with
  `--project` or `--config`.

## Install or upgrade

Existing standalone release installations can run `wrk upgrade --check` to check
availability and `wrk upgrade` to install the latest stable release with checksum
verification and atomic replacement. Run `wrk version` to inspect the installed
version and commit.

Verify your platform archive against the accompanying SHA-256 manifest before
manual installation. Binaries predating the upgrade command need one manual install.
See [installation instructions](https://github.com/calebmchenry/wrk#install-a-release)
and the [upgrade contract](https://github.com/calebmchenry/wrk/blob/main/docs/cli.md#version-and-upgrade).

Self-upgrade supports user-owned standalone release binaries. Development builds
and package-manager installations need their corresponding install method.
Releases are stable-only; the updater does not downgrade an installed version.
