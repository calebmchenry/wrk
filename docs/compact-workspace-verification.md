# Compact workspace acceptance

Ticket: [wrk-183a9f94](../.wrk/wrk-183a9f94.md). Date: 2026-10-09.
Source: working tree based on `f2183ba2406ab509cc1b84f6ea71bcd910c7ccd3`,
including all five completed compact UX feature children. This records the current
checkout, including pre-existing uncommitted work, rather than a released commit.
The [original workspace record](workspace-verification.md) retains historical
package certification; this pass verifies the final integrated UX and standalone
native binaries without repeating release publication or upgrade certification.

## Environments and checks

| Environment | Checks |
| --- | --- |
| macOS 15.7.7 (24G720), Darwin 24.6.0, arm64, APFS | Formatting, uncached Go normal/race suites, vet, native build, standalone workspace smoke, 15 Node tests, 6 Python tests and all 51 Chromium scenarios passed (1.1 minutes). |
| Debian 12.12, Linux 5.10.124-linuxkit, arm64, overlayfs | Same development/model/smoke checks and all 51 Chromium scenarios passed (1.2 minutes). |

Both use Go 1.25.4, Node 22.11.0, Playwright 1.64.0 and Chromium
156.0.8078.4. Python is 3.13.3 on macOS and 3.11.2 on Linux; npm is 11.10.1
and 10.9.0 respectively. Linux uses `golang:1.25.4-bookworm`, Docker `--init`,
and unprivileged UID 1000 (`wrkcheck`). Source was copied from a read-only host
mount into `/work/wrk`; temporary projects and all verification writes use the
container filesystem, outside any project ancestor.

Commands on each platform, from the copied/current repository root:

```sh
test -z "$(gofmt -l cmd internal test)"
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
go build -o ./bin/wrk ./cmd/wrk
python3 scripts/verify-workspace.py ./bin/wrk
python3 -B -m unittest discover -s scripts -p 'test_ticket_burn.py' -v
npm ci
npm test
npm run test:browser -- --output test-results/compact-macos
go run ./cmd/wrk validate
```

`git diff --check` and relative documentation file-link checks also pass in the
host checkout. The disposable Linux source copy intentionally omits `.git`.
At verification-child completion, current-checkout `wrk validate` reported 34
valid tickets and the parent remained open after its acceptance review. The direct
parent request is completed separately below.

Use `compact-linux` for the Linux browser output. Install Chromium with
`npx playwright install chromium` (plus `--with-deps` for a fresh Linux image).
For this Linux run it was installed into `/opt/ms-playwright`, with
`PLAYWRIGHT_BROWSERS_PATH=/opt/ms-playwright` for the test user. Run browser
timing checks after CPU-heavy builds. The browser suite builds the current CLI
and runs both the HTTP server and independent CLI reads against disposable projects.
No installed `wrk` is used. See [container isolation](browser.md#linux-verification-isolation).

## Evidence by requirement

| Requirement | Evidence in `test/browser/workspace.spec.mjs` and other checks |
| --- | --- |
| Query, hierarchy and honest counts | `views, exact tags, fuzzy title search...`, `collapse keeps selected details...`, `live closed ancestors...`, and `Under and exact AND tags...` exercise Active/All/Ready/Blocked, case-sensitive AND tags, global prerequisites, canceled blockers, descendant-only Under, ordered title subsequences, negative body/ID matches, context-only closed ancestors, restoration of manual collapse, missing scopes and selected items outside filters. The model suite also covers Unicode, repeated characters and a 10,000-level chain. |
| Modal creation and picking | `browser creation honors defaults...`, `creation modal traps focus...`, `creation uses publication defaults...` and both `keyboard-only compact workflow...` cases cover title-only writes, description, existing/new tags, closed parents, effective defaults, explicit empty tags, pending picker input, modal focus wrapping, discard and focus return. Separate CLI `show --json` confirms saved metadata and body. |
| Repeated children and row status | `inline repeated children...`, `inline draft survives live movement...`, row-status recovery and both keyboard walkthroughs verify explicit parent IDs, normal defaults without inheritance, filtered/collapsed parent context, out-of-query created markers without inflated counts, two siblings, blank/double Enter, restored focus, and all five statuses without navigation. CLI reads confirm every created sibling. |
| Document editing and own/external revision coordination | `confirmed metadata and row writes...`, `cancel keeps confirmed metadata...`, `editing relationships preserves exact untouched body...`, `live CLI edits preserve real document...`, and keyboard walkthroughs verify immediate status/tags/parent/relationships, dirty title/body Save/Cancel, input during a pending save, metadata confirmation followed by another agent edit, exact untouched CRLF/custom YAML/priority, empty body, and stale-save rejection. |
| Live input and navigation | The real-document scenario preserves focused description text, a nonempty selection range, textarea scroll, table/panel scroll, selected ID and collapsed branches while separate CLI processes update the selected and neighboring items. It verifies `CONFLICT` and independently confirms the agent's bytes remain saved. Inline, picker and narrow-pane scenarios preserve focus/caret, query, pins, Back/Forward/reload, filters, copy links and relationship context. |
| Keyboard and accessible controls | Two integrated cases at 1440 and 390 pixels use only Tab/Shift+Tab, keyboard text entry, native status typeahead, Enter, Space and Escape for UI actions. They open/close details, Save/Cancel, toggle hierarchy, open/close/trap the modal, select tags/parent, change row status, add repeated children and retain inline caret/focus after an agent update. Separate tests assert accessible names/descriptions, visible focus, inner-picker Escape, focus return and independent row controls. Native confirmation dialogs are accepted by the test driver. |
| Failure and uncertainty recovery | Existing validation, actual external advisory writer lock/BUSY, deletion, malformed YAML/repair, offline/reconnect, obsolete reads and resume checks remain. Committed/partial/unknown saves and lost modal/inline create responses retain drafts, require explicit review and never automatically retry. Store/API tests cover actual publication boundaries; response failures are injected in browser tests. |
| Safe content and embedded runtime | Unsafe HTML/links remain inert, images make no external requests, and source/custom metadata remains readable and unchanged. `verify-workspace.py` walks every static import and verifies HTML/CSS plus app/model/live/editor/pickers/detail/inline modules, API/CLI writes, related links, stale revisions and SIGTERM from a temporary cwd with an empty executable PATH. Go integration checks also execute an extracted binary away from source. |

## Visual review

Screenshots from real Chromium runs are retained locally under ignored
`test-results/compact-macos/` and `test-results/compact-linux/`. CI retains the
same suite's screenshots/traces as artifacts. The following basenames identify
the inspected views inside the scenario subdirectories:

| View | Screenshots and observations |
| --- | --- |
| Desktop table/detail, 1440×1000 | `compact-table.png`, `compact-detail.png`: 40-pixel single-line rows, capped chips with full accessible values, long-title truncation, no empty detail placeholder, and a table wider than 700 pixels beside details. Compact connection state; no permanent normal-state banner. |
| Creation and pickers | `create-desktop.png`, `create-picker-320.png`: title/description, chips, pending input, visible focus, top-right close and Create fit. Automated modal bounds also cover 390 and 760 pixels. |
| Narrow details and dirty text | `narrow-edit.png`, `compact-narrow-detail.png`, `dirty-detail-live-desktop.png`: full-width Back navigation on narrow screens, retained dirty title/body, visible Save/Cancel/focus, immediate metadata controls and lower relationship sections. Actionable draft notices appear only with an active draft. |
| Deep hierarchy and Under | `hierarchy-narrow-deep.png`, `hierarchy-under-320.png`: bounded indentation, visible context labels and usable picker bounds. The 120-level browser hierarchy is a synthetic read fixture. |
| Inline entry | `inline-children-desktop.png`, `inline-children-320.png`: focused blank sibling, compact confirm/cancel, explicit parent context and distinguishable created-outside-query markers. Width checks also cover 390 and 760 pixels. |

Automated checks assert no unwanted horizontal page/table overflow, independent
pane scroll, normal-state page height, and narrow disclosure bounds (including
320/390/760/800-pixel transitions). Exact screenshots can be reproduced by rerunning
the suite; these temporary projects and image artifacts are not committed.

## Findings and limits

- Fixed the standalone verifier's explicit asset set, which omitted `inline.mjs`.
  The existing UI already embedded and imported that module; no runtime or storage
  change was needed. Added three integrated regressions and current workflow docs.
- Initial new-test assertions incorrectly expected a nonexistent created-row
  attribute and focus on the parent link; the UI exposes an accessible created
  description and restores the parent actions button. Assertions now verify those
  user-facing contracts. A draft-scroll setup used macOS Home-key animated scrolling;
  setting a stable initial selection/scroll removed that test artifact. No product
  timeout was relaxed or integrity check removed.
- Chromium on macOS/Linux arm64 is the runtime/browser coverage here. WebKit,
  Firefox, mobile touch/virtual keyboards, assistive-technology sessions, and amd64
  runtime checks were not performed. Keyboard and accessibility assertions use the
  browser accessibility model, not a screen reader.
- Sleep/resume and failed publication responses are simulations; there was no
  physical suspend, disk fault or power loss. Actual CLI/filesystem writes and the
  writer lock are exercised. The browser depth fixture supplements the iterative
  10,000-level model check; it is not a 10,000-row browser performance claim.
- Hosted CI, release publication, upgrades and network-filesystem certification
  were not rerun. No new packaging/runtime dependency was introduced. Drafts still
  live in tab memory and explicit uncertain-create retry can create a duplicate;
  the documented review/acknowledgement is required.

All five feature children and the six parent acceptance criteria are reviewed in
[wrk-d70e8056](../.wrk/wrk-d70e8056.md). Closing this verification child does not
automatically change that parent's status.

## Parent completion

The direct request to implement wrk-d70e8056 on 2026-10-09 completed its separate
acceptance review and closure. All requested functionality was already implemented;
no additional product change was needed. Fresh macOS arm64 Go normal/race, vet,
build, standalone smoke, 15 Node tests, 6 Python tests and all 51 Chromium scenarios
passed. Formatting and diff checks also passed. The Linux results above remain the
cross-platform evidence; Linux was not rerun for this closure.

Fresh screenshots are under ignored `test-results/compact-parent/`. Visual review
covered desktop table/detail, narrow detail, a 320px creation picker, desktop/320px
inline children and narrow deep hierarchy. The parent ticket records the final
checks and completion; the limitations above still apply.
