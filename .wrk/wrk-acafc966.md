---
id: wrk-acafc966
title: Clarify nested web rows and scope deeper branches
status: done
priority: normal
labels:
  - web
  - web-ux
---
## Outcome

Make recursive browser hierarchy clear with four visible item levels, tree guides, wider nested indentation, and status immediately before tags after the title.

## Acceptance criteria

- Render up to four item levels per scope, with a See more row under boundary parents that have deeper included descendants.
- See more navigates to Under that parent using a native filter-preserving URL; deeper matches stay counted and are distinguished from collapsed matches.
- Add useful tree branch guides and larger nested padding across desktop and narrow layouts.
- Move status after the title, immediately before tags, preserving status mutation and row actions.
- Preserve filtering, collapse/history, selection, live updates and inline child draft behavior; cover recursive branching, depth limits and navigation in model/browser tests.
- Update browser guidance and run required checks and CLI validation.

## Decisions and progress

The existing model traverses all depths iteratively, but 8px indentation and an eventual indentation cap obscure the hierarchy. Four levels means a root plus three nested item levels in each scope; a continuation link can occupy the next line. Existing unrelated ticket changes are preserved.

Implemented the model depth limit and continuation rows, with matching counts separated from collapse counts. Under links preserve the current view/search/tags/selection and support browser history, reload and keyboard navigation. Selection below the cap gets its own explanation and scoping link. Connectors derive from visible sibling relationships, including filtering and collapse. Status now follows the title immediately before tags. Rows are 44px high; indentation is 24px on desktop and 16px in narrow tables.

Inline creation pins temporarily bypass the depth cap so drafts and saved children remain reachable after live reparenting; visual indentation stays bounded. Exiting creation restores the normal limit. Browser docs and their index link now describe the behavior.

## Verification and completion

Completed 2026-10-15. All acceptance criteria met.

- All 17 Node model tests passed, including the 10,000-level chain, recursive sibling connectors, filtering/scoping, depth versus collapse counts, and deep creation pins.
- All 52 Chromium scenarios passed (1.1 minutes), including the new recursive branch scenario and updated 120-level depth fixture. Navigation, history/reload, selected hidden items, live reparenting, status changes, keyboard workflows, creation/recovery and 320/390/760px bounds passed.
- Inspected the final desktop recursive tree, narrow deep tree, and 320px inline creation screenshots under ignored test-results/. The desktop reference is test-results/workspace-recursive-branch-c0866-lters-selection-and-history/recursive-tree-desktop.png.
- macOS arm64: clean gofmt; uncached Go tests and race tests; go vet; four Python burn tests; current ./bin/wrk build; and verify-workspace.py embedded assets/API/CLI persistence with empty PATH all passed. git diff --check passed.
- Linux local verification was unavailable: docker version reports no running daemon at /var/run/docker.sock. This is an environment limitation, not a passing Linux run.

Final CLI validation and completion status are checked after updating this body. No implementation work remains. Unrelated pre-existing changes to wrk-ef04f589 and wrk-b92b4796 were preserved.
