Standalone wrk binaries for macOS and Linux on amd64 and arm64. Go is not required.

## New in v0.3.0

- New tickets receive an immutable UTC `created_at` timestamp through both
  `wrk new` and the browser. CLI lists and JSON/API responses expose it, and the
  browser shows a read-only creation date. Existing undated tickets remain undated.
- `wrk run` now uses compact status records, readable heartbeats, and terminal
  colors that respect `NO_COLOR`, `TERM=dumb`, and redirected output. Multiline
  prompts stay in the logs, and failures provide an exact `retry.sh` file to inspect
  before rerunning. Completion checks and explicit retry behavior are unchanged.
- Browser rows show clearer hierarchy guides across four levels, with **See more**
  scopes for deeper branches and more consistent expansion and filtering.

## Compatibility

Upgrade all CLI clients and restart running `wrk serve` instances with v0.3.0
before creating new tickets. Older strict clients reject the new `created_at`
frontmatter field and may refuse project data commands once it is present.
Existing tickets need no migration or date backfill.

Human `new` output includes the assigned date, human lists add `CREATED_AT`, and
JSON/API ticket summaries add `created_at` (a timestamp string or `null`). Run JSON
events retain existing fields and add `retry_file` for failed/current actions.

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
