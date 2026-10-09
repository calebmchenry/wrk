# Local browser workspace

Run `go run ./cmd/wrk serve --port 0` from the repository root and open the printed
URL, or select another existing project with `--project`/`--config`. The header
shows its directory name, absolute path, prefix, and total item count. All assets
are embedded in the binary. No Node installation or network assets are needed
to use the workspace.

## Browsing and filters

- **Active** is the default: todo, in-progress, and explicitly blocked items.
- **All** includes done and canceled items.
- **Ready** means todo with every dependency done, exactly like CLI `--ready`.
  A canceled dependency remains a blocker; manually blocked items never qualify.
- **Blocked** means explicit blocked status. Separate “Waiting on” text identifies
  unfinished dependencies, even on an item whose status is todo or in-progress.
- Search is a case-insensitive substring over ID, title, and raw Markdown body.
- Label checkboxes require every selected label, with exact case-sensitive
  matching, like repeated CLI `--label` flags.
- Parent focus follows parent edges at every depth and **includes the root**.
  CLI `list --under` excludes the root. Search, labels, and the chosen status view
  then intersect this scope; a completed root appears when All is selected.

Items are ordered by ID, matching the CLI. Direct-child progress counts only
done children as complete; canceled children remain in the total. Descendant
counts never change an item's status. The detail pane links to its parent,
children, prerequisites, and reverse dependents, including closed items outside
the current list filters. Priority and labels are shown in both panes.

## Navigation and refresh

Search, view, labels, focus, and selection are encoded in URL fragments. Ticket
links preserve the current filters, and browser Back/Forward restores selection
and list position. Search typing replaces the current history entry instead of
adding an entry for each character. Links can be opened in another tab or copied;
fragments reload the same view. Scroll positions are per history entry in that
tab, not part of a copied URL.

On narrow screens, selecting an item replaces the list with its detail pane.
**Back to list** restores filters and list position. Native links, buttons,
checkboxes, and select controls work with the keyboard and show visible focus.
The skip link focuses the workspace without changing its URL state.

Use **Reload project** after CLI/agent edits. No automatic refresh or browser
editing is implemented in this ticket. Each workspace read validates the entire
project before returning all summaries and bodies; each detail read also loads
and validates the project. These separate reads are not a transaction. An item
removed after listing gets an explicit missing-item state. A validation/network
failure hides the browser views and shows diagnostics; repair the project or
restart the server, then reload. Missing focus roots and empty search results
have their own recovery guidance. Existing [server resource limits](cli.md#resource-limits-and-recovery)
apply, including the 32 MiB total project input bound.

## Markdown and custom values

Goldmark renders headings, emphasis, lists, code, quotes, tables, strikethrough,
and disabled task checkboxes. Raw HTML is omitted. Images become labeled text
placeholders and never load local or remote resources. CSP also blocks images.
Only explicit `http`, `https`, and `mailto` destinations become external links;
they open a separate tab with no opener or referrer. Link destinations naming
ticket IDs or sibling ticket filenames (`id.md` or `./id.md`) navigate within the workspace while
preserving filters. Other local paths, fragment-only destinations, and dangerous
schemes are not links. Original Markdown remains available in a disclosure.

“Custom fields & original metadata (YAML)” displays the exact original
frontmatter, including custom values and unknown fields. This preserves nested
values, tags, anchors, recursive aliases, and scalar precision without coercing
them through JSON. It is read-only. Titles, metadata, labels, and diagnostics are
always inserted as text; the sole HTML sink accepts only the restricted Markdown
renderer's output.

## Verification

Run the Go development checks in the README on macOS and Linux. Browser model
tests require only Node (22 or newer):

```sh
node --test internal/web/model.test.mjs
```

Install the locked development dependencies and Chromium for the real browser
suite:

```sh
npm ci
npx playwright install chromium
npm run test:browser
```

The browser suite builds the current CLI, creates a disposable 40-item project,
starts it on an allocated loopback port, and stops/removes it after testing.
It checks search/view/label/focus intersections, all five statuses, hierarchy,
closed-item links, Back/Forward/reload and scroll retention, missing selections,
loading/empty/invalid states and recovery, stale detail responses, inert unsafe
content, no remote loads, YAML preservation, keyboard operation, and a 390-pixel
narrow viewport. Desktop/narrow screenshots are saved under ignored
`test-results/`; inspect them when changing layout. Traces are retained on failure.
Go tests cover the Markdown/link policy, strict API validation, and embedding
the model module in an executable extracted away from the checkout. The model
suite also exercises a 10,000-level parent chain without recursive JS traversal.
