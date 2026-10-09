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

## Live updates and recovery

The visible browser polls `/api/workspace` 750 ms after each completed read. A
normal local change should be visible within two seconds; very large projects,
slow filesystems, or background browser throttling can take longer. Each read
uses current validated ticket/config bytes, including CLI atomic replacements,
direct body edits, file creation/deletion, and `.wrk/config.yaml` edits. List
membership, selected detail, hierarchy, blockers, progress, and config display
refresh together from that load. This does not provide a multi-file transaction:
recursive CLI updates and external saves still have the existing per-item
[publication boundary](storage.md).

A byte-based ETag covers the selected ID, config, and ticket inventory/content.
Unchanged reads return 304 only **after validation**, saving serialization,
Markdown rendering, transfer, and DOM replacement. Each poll still reads and
validates the project; there is no server cache or filesystem watcher. Lock,
staging, agent instructions, and unrelated runtime files do not affect the ETag
or displayed data. Directory-entry limits still count ignored files. Changes
between polls coalesce into the latest state; no per-file event history or
observation of every transient state is promised.

Each tab has one active fetch and one next-read timer, with no overlapping
periodic reads or queued events. Navigation/reload/resume cancels obsolete reads;
late responses cannot overwrite newer selections. Fetches time out after 20
seconds and retry after 750 ms. The server retains its four-read concurrency
limit, 15-second read deadline, and file/project/input bounds. Hidden tabs pause
polling. Visibility restoration, window focus, online events, and BFCache resume
trigger a full read, as do selection changes and **Reload project**. A timer
running more than two seconds late also forces a full read, covering system
sleep without browser lifecycle events. Errors clear
the conditional validator, ensuring recovery uses a full snapshot even when a
repair restores identical bytes. Since polling always reads current state,
there is no initial-load/subscription gap or event cursor to lose.

The header shows **Live**, reconnecting, paused, or project-needs-attention
state. Validation errors display shared diagnostics and retain any last-valid
snapshot explicitly labeled **stale**. No first valid snapshot means there are
no views to retain. Repair the named file or restart the server at the same URL;
recovery is automatic. An offline event marks the view stale immediately; other
transport failures do so when the read fails or times out. Manual reload forces
an immediate read. Missing selected items retain their ID and show “Item not
found”; selected items outside filters remain visible with an explanatory note.
Missing focus roots and empty searches retain their recovery guidance.

URL filters/search/selection and pane scroll positions survive updates. Context
is captured when a response arrives so scrolling or typing during a slow read
is respected; scroll clamps naturally if content becomes shorter. Unchanged
detail DOM, open metadata/source disclosures, and the persistent draft mount
survive polling.

### Draft interface for browser editing

`live.mjs` exports the singleton `drafts`. Editing code owns form DOM inside
`#draft`, which live refresh never replaces. Register one draft with
`drafts.register({id, revision, isDirty, onRemote})` and call the returned disposal
function after save/cancel. Registration rejects a second active draft. The
captured `revision` is the form's base revision and is never advanced by polling.

`onRemote(context)` receives `id`, `baseRevision`, current summary `item` (or
undefined), and `dirty`, `changed`, `missing`, `outsideFilters`, and `stale`
booleans on refresh/state changes. It is a notification for surrounding UI;
editing must not use it to replace unsaved inputs or the form's base revision.
The global draft notice explains external changes, removal, stale project data,
and filter exclusion while retaining the form. A dirty draft remains mounted
even when its item disappears. Actual forms, explicit save/cancel, stale-save
rejection, and navigation-away protection are implemented by the editing ticket
`wrk-52fbccc3`; this interface does not itself save or discard data.

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
loading/empty/invalid states and automatic recovery, obsolete snapshot responses, inert unsafe
content, no remote loads, YAML preservation, keyboard operation, and a 390-pixel
narrow viewport. Desktop/narrow screenshots are saved under ignored
`test-results/`; inspect them when changing layout. Traces are retained on failure.
Go tests cover the Markdown/link policy, strict API validation, and embedding
the model module in an executable extracted away from the checkout. The model
suite also exercises a 10,000-level parent chain without recursive JS traversal.

Live scenarios also use a separate compiled CLI process to create tickets and
change status, labels, parent, dependencies, and body, with two-second browser
assertions. They cover direct body edits, config changes, progress/relationships,
selected deletion/filter exclusion, draft retention, invalid YAML then repair,
offline/reconnect, a dropped response, BFCache suspension/resume, a change during
initial load, and ignored staging/runtime churn. The Go handler tests check
same-size/mtime-preserving edits, conditional-read validation, recovery to
identical bytes, selected detail coherence, and reads while a writer owns the
lock. Node tests manually clock the poller and settle requests, so detection,
coalescing, canceled/obsolete reads, stop/resume, error recovery, and draft base
revision retention do not depend on wall-clock sleeps.

If sandbox permissions prohibit local port binding or Chromium's macOS IPC,
the real browser/server checks cannot run there; keep that verification pending
rather than treating setup failures as passing tests. A writable build cache can
be selected with `GOCACHE=/private/tmp/wrk-live-go-cache` when the default Go cache
is outside permitted write roots. This does not remove network/IPC restrictions.
