# Local browser and agent workflow verification

This records the original workspace/package acceptance. For the subsequent compact
table, hierarchy, modal, direct-edit and inline-child UX, see the
[compact workflow acceptance record](compact-workspace-verification.md).

Ticket: [wrk-6f798aa0](../.wrk/wrk-6f798aa0.md). Date: 2026-10-09.
Source: the working tree based on `f2183ba2406ab509cc1b84f6ea71bcd910c7ccd3`,
including the completed editing/related-link changes and this acceptance work.
This is dirty-tree snapshot evidence, not a published release or clean-commit
attestation. All seven prerequisite feature tickets are done.

## Platforms and tools

| Environment | Runtime coverage |
| --- | --- |
| macOS 15.7.7 (24G720), Darwin 24.6.0, arm64, APFS | Full uncached Go/race suites, vet/build, native extracted snapshot/API smoke, all 23 Chromium scenarios against that package |
| Linux arm64, Debian 12, kernel 5.10.124-linuxkit, overlayfs in `golang:1.25.4-bookworm` | Full uncached Go/race suites, vet/build, native extracted snapshot/API smoke, all 23 Chromium scenarios against that package |
| macOS amd64 and Linux amd64 | Archive/checksum/build metadata checked; cross-built only in this session |

Go 1.25.4, Node 22.11.0, Playwright 1.64.0 with Chromium 156.0.8078.4,
Python 3.13.3 on macOS / 3.11.2 on Linux, GoReleaser OSS 2.18.2,
actionlint 1.7.12. Linux checks run as unprivileged UID 1000 (`wrkcheck`), so
the unwritable-directory check is exercised rather than skipped for root.
The four-platform hosted Go/native-package matrix remains in CI; the added
macOS/Linux browser jobs have been linted locally, not run on hosted runners
as part of this ticket.

The two native snapshot archives used for runtime checks have these SHA-256
digests (the verifier also checks the complete four-archive manifest):

```text
0d9852aa40d21dce35e80d41a3e409b7c010d2c81adde81705bab4efb0cb3c0c  wrk_0.1.0-snapshot_darwin_arm64.tar.gz
0bfef0067d55429dd3846d50730fe5cc8d345e90cdad170e450f464563832ace  wrk_0.1.0-snapshot_linux_arm64.tar.gz
```

## Reproduction

From the checkout, after installing the pinned development tools:

```sh
test -z "$(gofmt -l cmd internal test)"
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
go build -o ./bin/wrk ./cmd/wrk
python3 scripts/verify-workspace.py ./bin/wrk
python3 -B -m unittest discover -s scripts -p 'test_ticket_burn.py' -v
npm ci
npx playwright install chromium
npm test
goreleaser check
goreleaser release --snapshot --clean
python3 scripts/verify-release.py dist
wrk_browser_package="$(mktemp -d)"
tar -xzf dist/wrk_0.1.0-snapshot_darwin_arm64.tar.gz -C "$wrk_browser_package"
WRK_BROWSER_BINARY="$wrk_browser_package/wrk" npm run test:browser
go run ./cmd/wrk validate
```

Select the matching native archive on Linux or another architecture. Run browser
timing checks after CPU-heavy builds finish. `-count=1` is necessary because the
integration package builds a CLI at runtime; the Go cache cannot track its source
dependencies. The browser suite runs that exact binary for both HTTP serving and
independent CLI inspection, with temporary project files outside the checkout.

For Linux on a Mac, copy source and archives from a read-only mount into the
container's local filesystem before running checks, following
[local packaging guidance](releases.md#verification-and-local-packaging).
Install Node and Playwright's Chromium/dependencies in the disposable container;
never count writes to the host bind mount as Linux storage evidence.
Use Docker `--init` or wrap subprocess checks with `tini -s --` so orphaned
children are reaped. The initial Linux Python interruption run observed dead
grandchildren as zombies under PID 1 `sleep`; all six tests passed under `tini`.

## Evidence by requirement

| Requirement | Concrete checks |
| --- | --- |
| Embedded, standalone executable | GoReleaser produced four single-executable archives. `verify-release.py` checks names/checksums/platform/commit/version/kind, then runs `verify-workspace.py` on the native extracted asset. Its temporary cwd has no asset tree; the binary has an empty PATH. HTML/CSS, app/model/live/editor modules, safe Markdown, API creation/editing, reciprocal links, CLI file inspection, stale-save rejection, and SIGTERM pass. |
| Project selection and lifecycle | `TestServeSelectionAndLifecycle` covers root/nested discovery, selectors from another project, both relative selectors, immutable server selection, and Ctrl-C/SIGTERM. `TestExplicitProjectWorkflow` and `TestExplicitSelectionNeverFallsBack` cover invocation-relative body input, spaces, invalid boundaries and selector isolation. `TestServeDefaultAndRequestedPorts` checks 7331 and an explicit port, occupied-port rejection without fallback, then exact binding. Port 0 and invalid values also pass. |
| Browser launcher | `TestServeOpenExecutable` runs the platform launcher command via controlled `open`/`xdg-open` scripts on PATH, verifies exactly one actual-URL argument, success, nonfatal failure warning, usable HTTP, and clean shutdown. The CLI unit test verifies HTTP is usable before the launcher callback. |
| Browser writes and persisted files | Chromium creates and edits titles/bodies, all five statuses, labels, parents, dependencies and reciprocal related links. Separate CLI `show --json` reads source/relationships; assertions check exact untouched CRLF body bytes, explicit empty bodies, custom YAML/priority preservation, defaults, done/reopen and cycle errors. |
| Live external changes | Separate CLI processes create/rename/status/label/reparent/link/change descriptions with two-second UI assertions. Tests cover direct body edits, atomic CLI replacement, configuration changes, deletion, malformed YAML/repair, selected-item/filter context, and changes during initial loading. Go tests include same-size/mtime-preserving edits and repair to identical bytes. |
| Drafts and recovery | Real stale source and incoming-link revisions reject saves while retaining input; explicit review reconciles draft edits. A separate Python process holds the actual advisory flock: browser Save reports BUSY, CLI reads still work, bytes stay unchanged, and manual retry after release succeeds. Tests cover invalid references/cycles, deleted targets, navigation/departure warnings and lost creation responses without duplicate retries. |
| Committed and uncertain saves | Browser tests forward a real write, then inject a committed-error envelope or drop the response. They verify publication via a separate CLI, retained input, disabled Save, resynchronization, explicit review, and no automatic retry. Store/API tests inject post-publication failures, including partial related-link batches, to verify truthful per-file publication results. |
| Local request and content protections | Real packaged HTTP rejects foreign Origin and wrong Host without writes. Go handler tests cover Host/Origin/Fetch Metadata, forms, preflight, invalid JSON/paths, method and resource bounds. Chromium verifies raw HTML/dangerous links are inert and tracks requests to prove no automatic external asset/image loads. The server binds IPv4 loopback only. |
| Reconnect and resume | Chromium toggles offline mode, drops a request, and dispatches pagehide/pageshow to verify full-snapshot recovery. Deterministic Node tests advance a late timer and simulate cancellation/resume without requiring physical system sleep. |

The packaged browser runs passed all 23 scenarios in 29.0 seconds on macOS and
31.5 seconds on Linux. The nine Node model/poller/draft tests and six Python
runner tests passed on both platforms, as did uncached Go normal/race suites,
vet, build and formatting. The lifecycle tests also passed with verbose output
on both platforms, including default port 7331 (no skip). Desktop 1440×1000 and
390-pixel editing screenshots from both runs were visually inspected; controls,
links, chips and notices fit without horizontal overflow. GoReleaser check,
snapshot packaging, actionlint and `git diff --check` passed. Final repository
validation reported all 26 tickets valid.

## Boundaries and limitations

- The first browser run had one two-second reconnect assertion fail while
  concurrent Go tests/cross-builds were active (22/23 passed). The same scenario
  passed three isolated repetitions, followed by the complete packaged run.
  No timeout was relaxed and no product change was needed. This supports the
  documented normal-operation bound, not a latency guarantee under CPU starvation.
- Sleep/resume uses browser events and a deterministic late-timer test; the host
  was not physically suspended. Committed/dropped outcomes are injected failures,
  not evidence of a real disk fault or power-loss durability.
- `--open` verifies launcher invocation and failure handling with controlled
  executables. Chromium independently verifies actual rendering; the user's
  default desktop browser was not launched by the tests.
- Only Chromium is exercised. Other browser engines, network filesystems,
  remote hosting and amd64 runtime checks are outside this local evidence.
- The server stays foreground/local; agent actions use the separate `wrk run` command
  (including explicit `--stream`). The server provides no Git synchronization,
  automatic commits or daemon installation. Direct body/config edits and Git
  operations must stay between both CLI and browser mutations. Drafts remain
  in tab memory; accepting departure or a browser crash loses unsaved input.
- No release was published. The parent milestone's
  [acceptance review](../.wrk/wrk-a1561423.md#acceptance-review-2026-10-09)
  explicitly accepts this evidence; completing the verification ticket alone
  did not automatically change the parent's status.
