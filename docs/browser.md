# Local browser workspace

Run `go run ./cmd/wrk serve --port 0` from the repository root and open the printed
URL, or select another existing project with `--project`/`--config`. The compact
header shows the directory name and connection state. Its project menu contains
the absolute path, prefix, total item count, and **Reload project**. All assets
are embedded in the binary. No Node installation or network assets are needed
to use the workspace.

## Browser and agent walkthrough

Build the current checkout with `go build -o ./bin/wrk ./cmd/wrk`. In one terminal,
start `./bin/wrk serve --port 0 --open` from the project root. Keep this process
running in the foreground and use the exact printed URL. From a nested directory,
nearest-project discovery still applies; from elsewhere, pass an absolute
`--project /path/to/project` or `--config /path/to/project/.wrk/config.yaml`.
Relative selectors and CLI body-file paths use that terminal's working directory.

1. In the browser, choose **New**, enter a title, optionally add a description,
   tags and a parent, then choose **Create**. Add dependencies or related links
   from the detail panel after creation. Copy the displayed ID.
2. In a second terminal at the repository root, run
   `go run ./cmd/wrk show <id> --json` to inspect the persisted source and
   relationships. Run `go run ./cmd/wrk update <id> --status in-progress` and
   watch the browser update, normally within two seconds in a visible tab.
3. Edit the title directly, or choose **Edit description** and type a draft. In the terminal, change that item's title
   with `go run ./cmd/wrk update <id> --title "Agent updated this item"`.
   The form keeps your input and shows an external-change notice. Save receives
   `CONFLICT`; use **Review current version** and explicitly reload or keep your
   changes using the reviewed revision before saving again.
4. Choose a status in the detail panel, then verify with `show` and
   `go run ./cmd/wrk validate`. Parent/dependency links affect navigation and
   readiness; related links only provide context. No status change cascades.
5. Close details, search a few ordered title characters, and combine a view with
   Tags and Under. Only descendants count under a root; muted ancestors explain
   matches. Clear filters to restore manual collapse choices. In a row's actions,
   choose **Add child**, type a title and press Enter, then repeat with a different
   title to create two siblings. Each confirmation opens the next blank entry.
   Children outside the query stay marked until Escape exits the empty draft.
   Verify both IDs with `show --json` and compare `list --under <parent-id>`.
6. Stop the foreground process with Ctrl-C. Restarting it at the same port lets
   an open tab reconnect; unsaved drafts exist only in that tab's memory.

Use a disposable project for deletion, invalid-file, and recovery experiments.
Direct body/config edits and Git operations must occur between saves from both
the CLI and browser. The server does not run agents, synchronize Git, or create
commits; agents continue using the ordinary CLI and ticket files.

## Compact workspace layout

The toolbar keeps fuzzy title search, Active/All/Ready/Blocked views, removable
Tag chips, the searchable **Under** parent picker, and **Clear filters** above the
scrolling table. **Tags** maps to stored labels and CLI `--label`. The Under picker
searches titles and IDs across all items, including closed parents; its × clears
that scope. Search typing and live refresh retain picker input and keyboard focus.

Rows stay on one line with status, title, up to two tag chips and a +N count,
muted ID, and an actions button. Secondary content disappears as the table
narrows; the full title is the native item link, with ID/status/tags/priority,
blockers, and child progress available in its accessible description. The row
actions disclosure also shows full title, tags and ID and offers **Copy ID** and
**Copy link**, plus **Add child**. Clicking an action keeps the current selection.
Each row’s status select publishes any of the five statuses directly. It continues
to display the saved status until publication is confirmed. Tab/Enter and
Escape operate these controls with visible focus.

Selecting an item opens the right pane; **Close** returns all width to the table
and focus to its item link (or search when the item is filtered out). Both panes
scroll independently. The pane also offers copy controls. If clipboard access
fails, selectable text and a dismiss action allow manual copying. Copy links
include the current filters and selection. With no selection or open draft,
there is no detail placeholder. **New** (accessible name **New item**) opens a
focused creation modal. The selected document has a borderless title input,
status, tags, parent and a safe Markdown description. Choose **Edit description**
to use the plain textarea. All mutations retain revision checks.

Connection state stays in the header. Successful saves/copies use brief notices;
validation or connection errors have a compact disclosure that expands to show
diagnostics and recovery guidance. Last-valid data remains explicitly marked
stale. Unsaved draft warnings and save-recovery controls remain available.

## Browsing and filters

- **Active** is the default: todo, in-progress, and explicitly blocked items.
- **All** includes done and canceled items.
- **Ready** means todo with every dependency done, exactly like CLI `--ready`.
  A canceled dependency remains a blocker; manually blocked items never qualify.
- **Blocked** means explicit blocked status. Separate “Waiting on” text identifies
  unfinished dependencies, even on an item whose status is todo or in-progress.
- Search matches **titles only** using an ordered character subsequence: trim the
  query, lowercase it and the title without locale-specific rules, then find each
  query character in order, allowing gaps. `BSRCH` matches “Build search”;
  `search build` does not. Internal spaces and repeated characters count literally.
  Description-only and ID-only matches no longer appear. Item pickers still search
  both titles and IDs. Search does not rank or reorder the hierarchy.
- Tag checkboxes require every selected tag, with exact case-sensitive matching,
  like repeated CLI `--label` flags. Selected chips provide individual removal;
  Clear filters resets search, tags, Under and view to Active, preserving selection.
- **Under** includes descendants at every depth and **excludes the root**, matching
  CLI `list --under`. Search, tags and status intersect this scope. Its root appears
  only as context when a descendant matches; ancestors above it are omitted.

### Hierarchy and expansion

Rows use parent-before-child depth-first order, with roots and siblings sorted by
ID. Each item appears once. Nonmatching ancestors (including done/canceled parents)
are muted, marked **context**, and described accessibly as not matching the filters.
They only appear when needed to explain actual matches. Counts exclude context;
matching descendants hidden by collapse remain in the match count and have a
separate “hidden by collapse” count. No matches or a missing Under root shows an
explicit Clear filters recovery action.

Branches start expanded. Their native chevron buttons support Tab and Enter/Space,
with `aria-expanded`; leaves have no toggle. Active and All alone allow ordinary
collapse/expand. Explicit title search, tags, Under, Ready or Blocked reveal all
matching paths. Required chevrons are disabled during this exploration and explain
why in their tooltip. Clearing exploration restores the previous manual choices.
Manual choices survive live updates and are stored per browser history entry,
including reload, alongside pane scroll. They are not included in copied URLs.
Traversal and ancestor closure are iterative, including the 10,000-level fixture;
visual indentation stops growing after six levels (48 pixels), with actual depth
retained in the row's accessible description.

Selection does not force a branch open or alter filters. When collapse hides the
selected row, its detail stays visible with a compact explanation; out-of-filter
selection has a distinct explanation. Close/back focuses the row if displayed,
otherwise the title-search input. **Show descendants** in details sets Under to
that item without changing selection.

Direct-child progress counts only
done children as complete; canceled children remain in the total. Descendant
counts never change an item's status. The detail pane links to its parent,
children, prerequisites, reverse dependents, and related items from either endpoint, including closed items outside
the current list filters. Priority and tags remain visible in details; compact
rows expose secondary metadata accessibly without increasing row height.

## Navigation and refresh

Search, view, labels, Under (`under`), and selection (`item`) are encoded in URL
fragments. Existing `focus` fragments are accepted as an alias for Under, now with
descendant-only semantics: selection is retained even when it is the excluded
root. If both keys exist, `under` wins. Links and subsequent navigation serialize
`under`; loading an old fragment does not discard its other query state. Ticket
links preserve the current filters, and browser Back/Forward restores selection
and list position. Search typing replaces the current history entry instead of
adding an entry for each character. Links can be opened in another tab or copied;
fragments reload the same view. Scroll positions are per history entry in that
tab, not part of a copied URL.

On narrow screens, selecting an item replaces the list with its detail pane.
**Back to table** restores filters, list position, and keyboard focus. Native links, buttons,
checkboxes, and select controls work with the keyboard and show visible focus.
The skip link focuses the workspace without changing its URL state.

### Keyboard workflow

Tab/Shift+Tab reach every control, including the borderless title/description
inputs. Enter follows an item link; Close or Back to table returns focus to that
row, or search if it is no longer displayed. Row status, chevrons and actions are
independent controls and do not open details. There are no global letter shortcuts.

| Surface | Keys |
| --- | --- |
| Hierarchy | Enter or Space toggles a focused chevron; explicit filters reveal required paths. |
| Row status | Use native select keys (arrows/typeahead, Enter where required); publication is immediate. |
| Row actions | Enter opens; Tab reaches Copy ID, Copy link and Add child; Escape closes and returns focus. |
| New modal | Title receives focus; Tab/Shift+Tab wrap; ⌘/Ctrl+Enter creates; plain title Enter does not create. Escape closes the inner picker first, then requests discard if the modal is dirty. |
| Tag/item picker | Up/Down, Home/End and Enter select; Escape retains pending text and returns to the picker trigger. |
| Detail text | Tab to Save changes/Cancel, or ⌘/Ctrl+Enter to save text. Description Enter inserts a newline. Cancel confirms discarding unsaved changes. |
| Inline children | Enter creates and focuses the next blank sibling; Escape exits an empty draft. Tab to Cancel child creation to explicitly discard nonempty input. |

On narrow screens the table and details share the full width. Back to table
retains a dirty document with a Return to draft link; inline creation keeps the
table visible. Live updates preserve focused inputs and their caret positions.

## Live updates and recovery

The visible browser polls `/api/workspace` 750 ms after each completed read. A
normal local change should be visible within two seconds; very large projects,
slow filesystems, or background browser throttling can take longer. Each read
uses current validated ticket/config bytes, including CLI atomic replacements,
direct body edits, file creation/deletion, and `.wrk/config.yaml` edits. List
membership, selected detail, hierarchy, related links, blockers, progress, and config display
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
state. Validation errors offer shared diagnostics in an expandable disclosure and retain any last-valid
snapshot explicitly labeled **stale**. No first valid snapshot means there are
no views to retain. Repair the named file or restart the server at the same URL;
recovery is automatic. An offline event marks the view stale immediately; other
transport failures do so when the read fails or times out. Manual reload forces
an immediate read. Missing selected items retain their ID and show “Item not
found”; selected items outside filters remain visible with an explanatory note.
Missing Under roots and empty searches retain their recovery guidance.

URL filters/search/selection and pane scroll positions survive updates. Context
is captured when a response arrives so scrolling or typing during a slow read
is respected; scroll clamps naturally if content becomes shorter. Unchanged
detail DOM, open metadata/source disclosures, and the persistent draft mount
survive polling.

## Creating and editing

**New** opens an accessible modal with borderless Title and plain Markdown
Description fields, compact tag chips, an optional parent and **Create**. Only a
valid title is required; whitespace-only titles receive the shared server error
without losing input. Enter in the description inserts a newline. Create explicitly
with the button or the displayed **⌘/Ctrl + Enter** shortcut; plain Enter in the
title does not submit. Confirmed success closes the modal and opens the saved item
in details, preserving search, filters and table position even outside the filters.

New items start todo and use project priority and label defaults. Effective default
tags appear as chips and follow live config updates until you add or remove a tag.
Untouched labels are omitted from the create request, so publication uses the latest
project defaults. A committed addition/removal freezes an explicit list, including
empty. Tags and parents are never inferred from active filters.

**+ Add tag** offers existing tags and a new-tag option. **+ Add parent** searches
every existing item's title and ID, including closed items. Click the selected
parent chip to replace it; its × clears it. Picker inputs support Up/Down, Home/End
and Enter. Escape closes an expanded picker first, retaining pending input even
when another picker opens. A collapsed picker marks retained input as **pending**. Create commits pending tag text and exact parent IDs;
an unresolved parent search must be selected or cleared. The server rejects missing
or otherwise invalid references. Suggestions refresh from live data without
replacing entered values or unchanged suggestion/chip DOM.

Tab/Shift+Tab stay inside the modal. Escape or the top-right × closes an empty draft
immediately; a dirty draft requires confirmation to discard. Closing restores focus
to the opener. The shared single-draft guard prevents New or an edit/quick action
from replacing an existing draft. Internal history/navigation and live updates
retain modal input. Saving disables duplicate submission and closing until the
response arrives; field errors appear beside controls. Dependencies and related
links remain available in compact detail sections after creation.

### Row status and repeated child creation

Choose a row status to save immediately without navigating. Row writes use the
same revision coordinator as detail metadata: a dirty same-item text draft keeps
its captured revisions and typed text, and confirmed status writes advance only
their own baseline. Another item's text draft must be saved or canceled first.
Row-only errors stay in a recovery strip above the table. **Retry status** uses
the original revision; **Review current status** and **Use reviewed revision**
explicitly adopt a checked version before another retry. Lost responses or
committed completion errors disable retry until review. No automatic retry occurs.

Choose **Add child** in a row's actions menu. The parent expands and an indented,
borderless **Title** input appears immediately below it. Enter or the ✓ **Create
child** button submits once; blank/whitespace titles do nothing. The controls are
disabled while publishing. Confirmed creation displays the saved child, clears the
same input, and focuses the next blank sibling under the same explicit parent.
Each child starts with normal project defaults; parent tags and query filters are
never inherited, and no description is required.

The session pins its parent, ancestors, and confirmed children across filters,
manual collapse, navigation, and live updates without changing counts or filters.
Saved children outside filters are marked **created · outside filters** (**new***
in narrow tables). Reparented children stay visible in their current hierarchy;
the next draft still uses the original parent ID. Live parent reparenting moves
the draft with its parent. Deleted parents retain the input with a recovery
message and disable creation until restored. A deletion between submit and
publication is rejected by the server; it never produces a root item.

Escape exits an empty draft. A nonempty or uncertain draft requires **Cancel
child creation** and explicit discard confirmation. Already-created siblings
remain saved. Exiting removes visibility pins and restores ordinary filtering and
manual collapse. Input DOM, focus and caret survive surrounding renders. Narrow
screens keep the creation table visible while a session is active.
Exiting returns focus to the parent's actions button, or search if it is hidden.

Inline creation shares the single text-draft slot with New and detail editing;
trying to start another surface returns to the retained draft. Row-only status
writes can run during creation, including changes that move the parent outside
filters. Their recovery never replaces the child form. Before leaving the page,
typed or uncertain child input triggers the shared unsaved-changes warning.

Validation/BUSY failures retain the title for an explicit resubmit. A lost or
unreadable create response, or a committed completion error, retains the same
title and disables creation: **Review current items** lists all items, including
children reparented by an agent. Cancel if the child exists. Only **I checked the
items; enable another create** permits another explicit submission, which may
produce a duplicate if the previous create succeeded. A failed or uncertain save
never starts the next blank sibling.

### Direct detail editing

Edit the borderless Title directly. **Edit description** switches the safe rendered
Markdown to a plain, borderless textarea with visible keyboard focus. **Save changes**
and **Cancel** appear while title/description text is dirty. A blank title disables
Save; an empty description explicitly clears the body. Only changed text fields are
submitted. Untouched descriptions retain their exact bytes, including CRLF; edited
textarea content uses browser-normalized LF. Priority and arbitrary/custom YAML
remain read-only and preserved.

Status, tag, and parent selections publish immediately. All five statuses remain
available, and reparenting changes neither the item's status nor any child. Tags and
parents use the same keyboard-accessible pickers as New. Parent links navigate,
including to closed/out-of-filter items. Dependencies and related links have compact
lower sections with removal controls and a small add picker even when empty.
Children, direct-child progress, reverse dependents, priority, original Markdown and
exact metadata remain below the document. Tags map to CLI labels.

A small indicator beside each metadata control shows Saving, Saved, or an error.
A failed write keeps the requested value and offers a local **Retry**; validation
errors identify the control. Pending picker text is retained across polling and
unrelated saves. Text Save does not implicitly commit a pending metadata search.
**Discard pending changes** abandons pending metadata/recovery state after confirmation.
Cancel discards unsaved changes; it does not undo confirmed metadata writes.

Text stays editable while metadata writes are in flight. Metadata writes, text Save,
and the shared row-status entry point serialize through one existing-item coordinator;
a duplicate action is rejected, never queued or automatically retried. A confirmed
response advances the draft baseline using exactly its source and related revisions,
without replacing typed text. It never adopts a revision from the GET that follows
publication: an agent may already have written again by that read. Thus an own
metadata save does not create a self-conflict, but a later external write still
rejects stale preconditions. In-flight revision notices wait for the write to settle.

Before any input is changed, the pristine document follows live saved values. Once
input changes or a picker query is entered, a registered draft pins its baseline.
Polling never advances that draft's revisions, even if the text is subsequently
reverted. Only a confirmed own write or an explicit review acceptance does so.

Related links provide context without affecting readiness or status. The detail
pane and editor show one deduplicated reciprocal list, including closed items.
Add/remove from either endpoint has the same effect. A reverse removal may write
the other item's file, and clearing several links may publish several files. See
[storage and partial retry behavior](storage.md#related-link-publication).
Upgrade all clients before first use of the new optional field.

Text saves and cancellation are explicit. Polling refreshes surrounding data
without replacing form DOM or advancing the revision captured when editing began.
The draft remains visible across filters, item links, and Back/Forward navigation;
a notice identifies its item and warns that it remains unsaved while browsing.
Only one draft can be open in a tab. Cancel asks before discarding changes. Close/Back retains the draft; on narrow
screens it returns to the table with a **Return to draft** link in the notice.
Starting New or inline creation uses the shared guard and returns focus
to a retained draft instead of replacing it. Reloading/leaving the page uses the browser's unsaved-changes warning.
Drafts live in memory, so accepting departure or a browser crash loses unsaved input.

If an agent edits the item or changes an incoming/outgoing related link, an
external-change notice appears. Saving the original revisions returns `CONFLICT`, including for quick status changes and no-ops. A
removed target returns `NOT_FOUND`; neither discards input. **Review current
version** refreshes and displays your draft alongside the current saved fields.
**Reload saved values** replaces the draft after confirmation. **Keep draft using
reviewed revision** retains your changed fields and adopts unedited current fields,
then requires another explicit text Save or local metadata Retry. Label/dependency/related additions and removals are
reconciled with the reviewed lists, retaining unrelated agent additions. It uses
both the source and related revisions of the reviewed snapshot;
a subsequent agent change still rejects the save. There is no automatic retry or
silent overwrite. Repair invalid project data before saving; the form stays available
while the last valid snapshot is marked stale.

Validation diagnostics name affected fields and mark matching form controls.
`BUSY` retains the draft for manual retry. A successful save notice appears only
when the server confirms publication (or a checked no-op). A committed error
identifies the item, diagnostics, and each affected file’s publication state for link batches, keeps the draft, and disables Save.
A dropped/unreadable/timed-out response says **Save outcome unknown** and likewise
disables Save. Both immediately resynchronize current data. Review before another
save; an uncertain create may already exist, so its review lists current items and
requires an explicit acknowledgement before enabling another create. Cancel the
draft if it was already saved. No error triggers an automatic mutation retry.

### Draft interface

`detail.mjs` owns the existing-item document DOM in `#draft`; `editor.mjs` owns the
creation form in the native `#create-dialog`. The app guards modal/inline starts
with `detailEditor.yieldDraft()`. `createDetailEditor` exposes `sync`, `dirty`,
`yieldDraft`, and `mutate(item, patch)`. `app.js` exports `changeItemStatus(id, status)`
for the row status controls. It routes same-item writes through the captured draft
revisions, refuses competing/uncertain writes, and guards a different-item draft.
Row controls call it rather than constructing a fresh request from polling.

`publishItem({id, payload})` is the shared one-shot HTTP hook: a missing ID creates,
an ID patches. It returns a confirmed publication or a diagnostic envelope, and
throws for transport/unrecognized responses. Callers guard in-flight and uncertain
writes and never retry automatically. `draftPatch` omits unchanged fields;
`confirmedBaseline` uses only a confirmed response and submitted body bytes;
`reconcileDetail` preserves local collection additions/removals during explicit review.

`pickers.mjs` exports `createTagPicker` and `createItemPicker`. Both expose owned
`root`, `trigger`, `input`, `get`, `set`, `refresh`, `pending`, `commit`, `open` and
`close` hooks. `refresh` changes suggestions without clearing input. `commit` returns
false for unresolved item searches. `onChange(committed)` distinguishes selection/
removal from pending typing, so the caller can track explicit default overrides.
A bubbling `picker-open` event supplies a close hook for coordinating one expanded
picker per surface. Suggestions are capped to 80 visible matches; search covers the
whole project. The caller owns field diagnostics and mutation/draft coordination.

`live.mjs` exports the singleton `drafts`.
Register one draft with `drafts.register({id, revision, relatedRevision, isDirty, isSaving, onRemote})` and call
the returned disposal function after save/cancel. Registration rejects a second
draft. An empty ID denotes creation. `onRemote(context)` receives `id`,
`baseRevision`, current summary `item` (or undefined), and `dirty`, `changed`,
`missing`, `outsideFilters`, and `stale` flags. Notifications never replace inputs
or either base revision. `changed` includes a changed reciprocal link set when
`relatedRevision` was registered. Confirmed own writes and explicit review acceptance dispose and re-register the
draft against exactly those confirmed/reviewed revisions. Optional `isSaving`
suppresses provisional change notices until a pending write settles; it never
changes preconditions. Full-page departure protection also covers
pending collection inputs and in-flight or uncertain writes.

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
To test an exact extracted package instead of rebuilding, set
`WRK_BROWSER_BINARY=/absolute/path/to/extracted/wrk npm run test:browser`.
Both the server and its disposable project run outside the checkout. Python 3
is also required for the real external-writer-lock scenario. CI runs the suite
on macOS and Linux and retains screenshots/traces as workflow artifacts.
It checks title-search/view/Tag/Under intersections, all five statuses, hierarchy,
closed-item links, Back/Forward/reload and scroll retention, missing selections,
loading/empty/invalid states and automatic recovery, obsolete snapshot responses, inert unsafe
content, no remote loads, YAML preservation, keyboard operation, and a 390-pixel
narrow viewport. Compact-shell checks also cover capped tags and long titles, full-width browsing,
pane sizing, clipboard success/denial, row-action isolation, and deep narrow-screen
scroll retention through polling and history. Desktop/narrow screenshots are saved under ignored
`test-results/`; inspect them when changing layout. Traces are retained on failure.
Go tests cover the Markdown/link policy, strict read/write validation, write security,
BUSY/stale edits, bounded service mutations, committed-error envelopes, and embedding
every current UI module in an executable extracted away from the checkout. The model
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

Editing scenarios create and update through the browser, then read through a
separate compiled CLI to check persistence, defaults, exact untouched CRLF bodies,
empty bodies, custom YAML/priority preservation, all statuses, reopening, labels,
parenting and dependencies. They test cycle errors, stale forms and quick actions,
explicit conflict review, navigation/deletion retention, cancellation, narrow
keyboard forms, an actual separate-process writer lock and manual retry, and
injected BUSY/committed/dropped-response outcomes. Lost-create
coverage verifies resynchronization without a duplicate request. Related-link
scenarios cover creation, reciprocal navigation to closed items, reverse removal
combined with ordinary metadata, live agent additions/removals, self-link errors,
and retained drafts whose source bytes are unchanged but link preconditions are stale.

See [compact workflow acceptance](compact-workspace-verification.md) for current
platforms, requirement-level evidence and inspected screenshots. The
[original workspace verification](workspace-verification.md) records the earlier
package certification and simulation limitations.

Creation-modal scenarios also verify focus wrapping and restoration, explicit dirty
discard, picker keyboard selection and pending input, narrow 320/390/760-pixel
bounds, live default changes and publication-time defaults, explicit empty tags,
closed parents, validation/BUSY retention, duplicate-submit suppression, protected
drafts across navigation and competing edit surfaces, and lost-response review.
Desktop and expanded-picker screenshots are saved with the browser artifacts.

Direct-detail scenarios additionally cover delayed metadata responses with concurrent
typing, duplicate row actions, confirmed-write versus later-agent revision races,
empty-title blocking, cancel after confirmed metadata, closed-parent selection with
unchanged child bytes, dependency cycles, and metadata partial/uncertain recovery.
The partial-publication browser envelope is injected; Go storage tests exercise
actual related-link publication boundaries.

The integrated keyboard walkthrough uses the real Tab order, keyboard text entry,
native status typeahead, and Enter/Escape at desktop and 390 pixels, with no mouse
or programmatic focus/selection shortcuts. It reads modal/child writes and detail
Save/Cancel results through a separate CLI and retains the inline caret across an
agent update. A real dirty-document scenario checks selection, collapse choices,
caret and both pane scroll positions during simultaneous CLI changes, then verifies
that a stale Save is rejected without overwriting the agent's persisted values.

### Linux verification isolation

For container verification, copy the checkout into a dedicated local directory such
as `/work/wrk`, then run the README development checks there. Keep the system temp
directory outside any project ancestor: copying `.wrk` directly to `/tmp` makes
project-discovery tests in temporary directories find that project, and a test
expecting `serve` to reject a missing project can instead start a server. Use
`docker run --init` so interruption tests' child processes are reaped. Run checks
as an unprivileged user to exercise permission failures. Install Chromium and its
system dependencies in the container (`npx playwright install --with-deps chromium`);
when installed as root, place the browser cache in a readable shared location and
set `PLAYWRIGHT_BROWSERS_PATH` for the test user. These are test-environment
requirements; no product code or tests need to be relaxed.

### Hierarchy implementation handoff

`model.mjs` separates matching membership (`selectItems`) from context and visible
rows (`treeRows`). Collapse IDs live in the app/history state, never in the query.
The model's optional pins reveal a creation session's parents/items without
changing match counts or manual expansion. `app.js` exports
`setInlineChildContext(parentID, createdIDs = [])`, returning a persistent `<li>`
mount immediately after that parent. `inline.mjs` owns its form and
recovery controls through the shared draft guard and one-shot publication hook. Polling moves this same mount and restores input focus/caret;
a missing parent leaves the mount available for explicit recovery. Call with no
parent to exit and restore ordinary filtering. Row `data-row`/`data-depth` hooks
identify the insertion context; the mount itself is not a result.

Hierarchy checks cover filter intersections, global readiness (including canceled
prerequisites), subsequence fixtures, context/counts, collapse/history restoration,
live closed-parent/reparent/status changes, legacy focus links and missing-scope
recovery. The browser uses a synthetic 120-level read response to verify indentation
and narrow bounds; dependency-free model checks exercise the 10,000-level chain.
The inline hook test checks mount/input retention. Full creation scenarios verify
repeated siblings through CLI reads, defaults without filter/parent inheritance,
collapsed and filtered parents, live reparenting/status/deletion, explicit cancel,
modal/detail draft exclusion, duplicate Enter, validation/BUSY and lost/committed
responses. Row tests cover all five statuses without navigation, concurrent agent
conflicts, same-item dirty text and explicit uncertainty recovery. Screenshots
cover the inline session at desktop and 320/390/760-pixel widths. Injection tests
simulate response/validation failures; the child creation and CLI reads use the
real server and filesystem.
