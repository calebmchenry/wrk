---
id: wrk-87f1ef2a
title: Create items with a minimal modal and tag/parent chips
status: done
parent: wrk-d70e8056
depends_on:
  - wrk-9be86c55
priority: normal
labels:
  - web
  - web-ux
---
## Outcome

Create a work item from a focused modal with only a title required, keeping optional description, tags and parent lightweight.

## Acceptance criteria

- [x] New opens a modal with an accessible name, top-right X, autofocus on a borderless Title input, a borderless plain textarea with a useful placeholder, + Add tag, optional parent chip and a compact Create button. Visible field labels and boxed form styling are unnecessary; accessible field names remain required.
- [x] Require only a valid title using existing validation rules. Enter in the textarea inserts a newline; submission is explicit through Create or a discoverable keyboard equivalent. Whitespace-only titles fail without losing input.
- [x] Tag chips support choosing existing tags, adding a new tag and removal. The parent chip opens a searchable existing-item picker showing title plus ID; selecting/removing it sets/clears the optional parent. Include closed items where valid; reject invalid references using existing server validation.
- [x] New items start todo and honor configured priority/label defaults. Show effective default tags compactly. Untouched tags retain the existing project-default-at-publication behavior; explicit additions/removals send an explicit list, including empty. Do not silently infer tags or parents from active filters.
- [x] Implement reusable compact tag/item pickers and mutation/draft hooks for the later detail and inline-row work. Suggestions and pending chip input must not disappear or be omitted accidentally at save time.
- [x] Trap focus in the modal, support keyboard picker navigation and Escape/X close, and restore focus to the opener. Escape closes an inner picker before the modal. Empty drafts close immediately; closing a dirty draft requires an explicit discard choice or retains it through the existing draft lifecycle.
- [x] Keep one protected text/create draft per tab. Opening another creation/edit surface must not silently replace it; internal browsing and live updates retain entered values. Preserve existing full-page departure protection.
- [x] Disable duplicate submission while saving. Retain input on validation/BUSY/conflict/transport problems, display errors near relevant controls, announce confirmed success and preserve existing uncertain-create review with no automatic retry. Successful creation closes the modal and opens the saved item in details without losing table/filter context, including when it is outside current filters.
- [x] Verify title-only persistence through the CLI, optional chips/body, defaults and explicit empty tags, keyboard/close behavior, narrow layout, live draft retention, double submission and lost-response handling.

## Scope and handoff

Depends on the compact shell. Replace the creation surface without requiring a large metadata form. Dependencies and related links remain available in the detail editor after creation. Reuse existing validated HTTP/store mutations and draft recovery; do not weaken publication/uncertainty handling to simplify presentation.

## Starting points

internal/web/assets/editor.mjs, live.mjs and app.js; docs/browser.md creating/editing and draft interface; existing browser mutation tests.

## Implementation and handoff

Creation uses a native modal dialog over the retained workspace, with a top-right
close button, borderless fields, explicit Create / Cmd-or-Ctrl+Enter, focus wrapping,
inner-picker Escape handling and opener focus restoration. Empty drafts close
immediately; dirty drafts retain the existing explicit discard confirmation.

`pickers.mjs` exports reusable tag and item pickers. They own pending input, expose
commit/refresh/selection hooks, retain queries when closed or when another picker
opens, and mark collapsed pending input visibly/accessibly. Live suggestions include
closed items and preserve unchanged DOM and active selection. Committed chip intent
is applied before focus changes so blur-triggered refresh cannot restore defaults.

Untouched tags track effective config defaults and are omitted from the request;
committed additions/removals freeze an explicit list, including empty. Parent starts
empty regardless of filters. Pending item search must be selected, cleared, or an
exact ID before saving; invalid IDs still go through server validation. Dependencies
and related links remain available in the detail editor after creation.

Both surfaces share the single protected draft, existing conflict/uncertain-create
review, and exported one-shot `publishItem` hook. Confirmed creation closes the modal
and focuses saved details, preserving filters and table context. In-flight and
uncertain guards prevent automatic or duplicate submission. Field errors retain
input and appear beside the affected control. The module is embedded and served by
the standalone binary; docs/browser.md documents the reusable interfaces.

## Verification

- macOS: clean gofmt, `go test -count=1 ./...`, `go test -race -count=1 ./...`,
  `go vet ./...`, native build, all 6 Python runner tests and all 9 Node model tests.
- Chromium: all 32 browser tests passed in the final complete run. New coverage
  checks title-only CLI persistence, optional body/tags/closed parents, changed
  defaults at publication, explicit empty tags, filter preservation, whitespace
  title/invalid parent errors, keyboard picker selection, pending text, focus trap,
  Escape/X and discard choices, narrow bounds, departure protection, live updates,
  competing creation/edit surfaces, BUSY, double submission and lost-create review
  with explicit acknowledgement before reenabling creation. Existing edit/conflict,
  reciprocal-link and exact-body tests still pass.
- Visually inspected desktop and 320-pixel expanded-picker screenshots; automated
  modal bounds checks also cover 390 and 760 pixels. Evidence is under
  `test-results/final-modal/workspace-creation-modal-t-cb1ec-ose-and-restores-its-opener/`
  (`create-desktop.png` and `create-picker-{320,390,760}.png`, ignored artifacts).
- Linux arm64, `golang:1.25.4-bookworm`, native container filesystem: clean gofmt,
  uncached Go and race suites, vet, native build and all 6 Python tests passed.
  Corrected an initial container setup error that put `.wrk` at `/tmp`, causing
  discovery tests to find an unintended ancestor project; reran in `/work/wrk`
  with `/tmp` isolated. Documented this workflow and linked it from docs/index.md.
- Standalone `scripts/verify-workspace.py` passed on the rebuilt macOS and Linux
  executables: every embedded module (including pickers), empty executable PATH,
  API/CLI persistence, stale-write rejection, related links and SIGTERM.
- `git diff --check` passed. Final ticket status and project validation are checked
  through the current checkout's CLI after this body update.

No work remains in this ticket. Detail editing can use these shared hooks; the
parent milestone remains open for its other implementation and verification work.
