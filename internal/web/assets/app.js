import { readState, stateHash, filterKey, indexItems, selectItems, treeRows, progress } from './model.mjs';
import { createPoller, drafts } from './live.mjs';
import { createEditor } from './editor.mjs';
import { createDetailEditor } from './detail.mjs';
import { createItemPicker } from './pickers.mjs';
import { createInlineChildren } from './inline.mjs';

const $ = selector => document.querySelector(selector);
const el = (tag, className, text) => {
  const node = document.createElement(tag);
  if (className) node.className = className;
  if (text !== undefined) node.textContent = text;
  return node;
};
const names = { todo: 'Todo', 'in-progress': 'In progress', blocked: 'Manually blocked', done: 'Done', canceled: 'Canceled' };
const help = { active: 'Todo, in progress, and manually blocked.', all: 'Every status, including done and canceled.', ready: 'Todo with every dependency done.', blocked: 'Explicitly blocked status, independent of dependencies.' };
let state = readState(location.hash);
let index = null;
let snapshot = null;
let lastSelected = '';
let stale = false;
let detailFocus = false;
let detailScroll = history.state?.detailScroll || 0;
let renderedDetail = '';
let renderedList = '';
let noticeTimer;
let copyTimer;
let collapsed = new Set(history.state?.collapsed || []);
let table = null;
let renderedLabels = '';
let renderedChips = '';
let inlineParent = '';
let inlinePins = [];
const inlineMount = el('li', 'inline-child-row');
const statusMount = el('section', 'row-status-recovery');
statusMount.setAttribute('aria-label', 'Row status change');
$('#list-scroll').before(statusMount);
const underPicker = createItemPicker({ id: 'under', label: 'Under parent', caption: 'Entire project', removeLabel: 'Clear Under filter',
  getItems: () => index?.items || [], onChange: committed => {
    if (committed) navigate({ ...state, under: underPicker.get() });
  } });
$('#under-filter').append(underPicker.root);
// This toolbar picker retains its query/focus during live refresh like editor pickers.
document.addEventListener('click', event => {
  if (!$('#under-filter').contains(event.target)) underPicker.close(false);
});

function badge(status) { return el('span', `badge status-${status}`, names[status] || status); }
function itemHref(id) { return stateHash({ ...state, item: id }); }
function itemLink(item, className = '') {
  const link = el('a', className, item.title);
  link.href = itemHref(item.id);
  link.dataset.item = item.id;
  return link;
}
function listScroll() {
  // display:none reports zero on narrow screens. Keep the visible table's
  // position in the current history entry while the detail replaces it.
  return $('#browse').getClientRects().length ? $('#list-scroll').scrollTop : history.state?.listScroll || 0;
}
function saveScroll() {
  history.replaceState({ ...history.state, collapsed: [...collapsed], listScroll: listScroll(), detailScroll: $('#detail').getClientRects().length ? $('#detail').scrollTop : history.state?.detailScroll || 0 }, '', location.href);
}
// Scroll is stored per history entry, including before a browser Back action.
$('#list-scroll').addEventListener('scroll', saveScroll, { passive: true });
$('#detail').addEventListener('scroll', saveScroll, { passive: true });
history.scrollRestoration = 'manual';

function navigate(next, { replace = false, focus = false } = {}) {
  saveScroll();
  const scroll = { collapsed: [...collapsed], listScroll: filterKey(next) === filterKey(state) ? listScroll() : 0, detailScroll: next.item === state.item ? $('#detail').scrollTop : 0 };
  history[replace ? 'replaceState' : 'pushState'](scroll, '', stateHash(next));
  syncState(focus);
}
function syncState(focus = false) {
  const previous = state;
  const scroll = history.state || {};
  state = readState(location.hash);
  collapsed = new Set(scroll.collapsed || []);
  $('#workspace').classList.toggle('has-selection', Boolean(state.item));
  if (index) { buildFilters(); renderList(); }
  else renderControls();
  updateLinks();
  $('#list-scroll').scrollTop = scroll.listScroll || 0;
  if (previous.item !== state.item) {
    detailFocus = focus;
    detailScroll = scroll.detailScroll || 0;
    renderDetail();
    poller.refresh();
  } else {
    renderDetail();
    $('#detail').scrollTop = scroll.detailScroll || 0;
  }
  updateDraft();
  if (!state.item && focus && lastSelected) {
    const link = [...$('#items').querySelectorAll('a')].find(a => a.dataset.item === lastSelected);
    (link || $('#search')).focus({ preventScroll: true });
  }
}
window.addEventListener('popstate', () => syncState(true));
window.addEventListener('hashchange', () => syncState(true));

function renderControls() {
  $('#search').value = state.q;
  for (const button of document.querySelectorAll('[data-view]')) button.setAttribute('aria-pressed', String(state.view === button.dataset.view));
  $('#view-help').textContent = help[state.view];
  for (const button of document.querySelectorAll('[data-view]')) button.title = help[button.dataset.view];
  $('#label-count').textContent = state.labels.length ? `(${state.labels.length})` : '';
  for (const input of $('#label-options').querySelectorAll('input')) input.checked = state.labels.includes(input.value);
  underPicker.set(state.under, { preservePending: true });
  const key = JSON.stringify(state.labels);
  if (key !== renderedChips) {
    renderedChips = key;
    $('#tag-chips').replaceChildren();
    for (const label of state.labels) {
      const chip = el('span', 'compact-chip');
      const remove = el('button', '', '×');
      remove.type = 'button';
      remove.setAttribute('aria-label', `Remove tag filter ${label}`);
      remove.addEventListener('click', () => {
        navigate({ ...state, labels: state.labels.filter(value => value !== label) });
        ($('#tag-chips button') || $('#label-filter summary')).focus({ preventScroll: true });
      });
      chip.append(el('span', '', label), remove);
      $('#tag-chips').append(chip);
    }
  }
}
function buildFilters() {
  const labels = [...new Set([...index.items.flatMap(item => item.labels), ...state.labels])].sort();
  const key = JSON.stringify(labels);
  if (key !== renderedLabels) {
    renderedLabels = key;
    $('#label-options').replaceChildren();
    for (const label of labels) {
      const wrapper = el('label');
      const input = el('input');
      input.type = 'checkbox';
      input.value = label;
      input.addEventListener('change', () => navigate({ ...state, labels: [...$('#label-options').querySelectorAll('input:checked')].map(input => input.value) }));
      wrapper.append(input, document.createTextNode(label));
      $('#label-options').append(wrapper);
    }
    if (!labels.length) $('#label-options').append(el('p', 'hint', 'No tags in this project.'));
  }
  underPicker.refresh();
  renderControls();
}
function renderList() {
  if (!index) return;
  table = treeRows(index, state, collapsed, inlinePins, inlineParent ? [inlineParent] : []);
  const hidden = table.matches.filter(item => !table.visible.has(item.id)).length;
  $('#results').textContent = `${table.matches.length} of ${index.items.length} items match${hidden ? ` · ${hidden} hidden by collapse` : ''}`;
  // Selection-only refreshes should not replace links, menus or keyboard focus.
  const key = JSON.stringify([filterKey(state), index.items, [...collapsed], inlinePins]);
  if (key === renderedList) { updateLinks(); return; }
  renderedList = key;
  const active = document.activeElement;
  const activeRow = active?.closest('[data-row]')?.dataset.row;
  const activeKind = active?.dataset.toggle ? '[data-toggle]' : active?.dataset.item ? '.item-link' : active?.dataset.rowAction ? '.row-actions' : active?.dataset.rowStatus ? '.row-status' : '';
  const scroll = listScroll();
  const fragment = document.createDocumentFragment();
  for (const { item, depth, context, hasChildren, expanded, forced } of table.rows) {
    const row = el('li', `item-row${context ? ' context-row' : ''}`);
    row.dataset.row = item.id;
    row.dataset.depth = depth;
    row.dataset.context = String(context);
    row.style.setProperty('--depth', Math.min(depth, 6));
    const chevron = el(hasChildren ? 'button' : 'span', 'row-chevron', hasChildren ? expanded ? '⌄' : '›' : '');
    if (hasChildren) {
      chevron.type = 'button';
      chevron.dataset.toggle = item.id;
      chevron.setAttribute('aria-label', `${expanded ? 'Collapse' : 'Expand'} ${item.title}`);
      chevron.setAttribute('aria-expanded', String(expanded));
      chevron.disabled = forced;
      chevron.title = forced ? 'Matching paths stay expanded while exploring filters or adding children.' : `${expanded ? 'Collapse' : 'Expand'} children`;
      chevron.addEventListener('click', () => {
        if (collapsed.has(item.id)) collapsed.delete(item.id); else collapsed.add(item.id);
        renderList();
        saveScroll();
      });
    } else chevron.setAttribute('aria-hidden', 'true');
    const status = el('select', `badge row-status status-${item.status}`);
    status.dataset.rowStatus = item.id;
    status.setAttribute('aria-label', `Status for ${item.title}`);
    for (const [value, name] of Object.entries(names)) status.add(new Option(value === 'blocked' ? 'Blocked' : name, value));
    status.value = item.status;
    status.title = names[item.status];
    status.addEventListener('change', () => {
      const requested = status.value;
      // The row continues to show the saved value until publication is confirmed.
      status.value = index.byID.get(item.id)?.status || item.status;
      changeItemStatus(item.id, requested);
    });
    const link = itemLink(item, 'item-link');
    link.title = item.title;
    const createdOutside = context && inlinePins.slice(1).includes(item.id);
    const description = el('span', 'sr-only', `${createdOutside ? 'Created in this session; outside the current filters. ' : context ? 'Context ancestor; does not match the current filters. ' : ''}Hierarchy level ${depth + 1}. ${item.id}. ${names[item.status]}. ${item.priority} priority. Tags: ${item.labels.join(', ') || 'none'}.`);
    description.id = `row-description-${item.id}`;
    if (item.blockers.length) description.append(` Waiting on ${item.blockers.map(b => `${b.id} (${b.status})`).join(', ')}.`);
    const children = index.children.get(item.id) || [];
    if (children.length) description.append(` ${progress(children)}.`);
    link.setAttribute('aria-describedby', description.id);
    const tags = el('span', 'row-tags');
    tags.setAttribute('aria-hidden', 'true');
    tags.title = item.labels.join(', ');
    for (const label of item.labels.slice(0, 2)) tags.append(el('span', 'badge', label));
    if (item.labels.length > 2) tags.append(el('span', 'badge tag-overflow', `+${item.labels.length - 2}`));
    const id = el('span', 'item-id', item.id);
    id.title = item.id;
    const actions = el('button', 'row-actions', '···');
    actions.type = 'button';
    actions.dataset.rowAction = item.id;
    actions.setAttribute('aria-label', `Actions for ${item.title}`);
    const menu = el('div', 'row-menu');
    menu.id = `row-menu-${item.id}`;
    menu.setAttribute('popover', 'auto');
    menu.setAttribute('aria-label', `Actions for ${item.id}`);
    actions.setAttribute('popovertarget', menu.id);
    menu.append(el('p', '', item.title), el('p', 'item-id', item.id), el('p', 'hint', `Tags: ${item.labels.join(', ') || 'none'}`),
      copyButton('Copy ID', () => item.id), copyButton('Copy link', () => new URL(itemHref(item.id), location.href).href));
    const addChild = el('button', '', 'Add child'); addChild.type = 'button';
    addChild.addEventListener('click', () => {
      menu.hidePopover();
      if (inlineChildren.isOpen()) { inlineChildren.open(item.id); return; }
      if (editor.isOpen()) { editor.open(); return; }
      if (detailEditor.yieldDraft()) inlineChildren.open(item.id);
    });
    menu.append(addChild);
    menu.addEventListener('toggle', () => {
      if (!menu.matches(':popover-open')) return;
      const rect = actions.getBoundingClientRect();
      menu.style.left = `${Math.max(12, Math.min(innerWidth - menu.offsetWidth - 12, rect.right - menu.offsetWidth))}px`;
      menu.style.top = `${Math.max(12, Math.min(innerHeight - menu.offsetHeight - 12, rect.bottom + 4))}px`;
    });
    const title = el('span', 'row-title');
    title.append(link);
    if (context) {
      const label = el('span', `context-label${createdOutside ? ' created-context' : ''}`, createdOutside ? 'created · outside filters' : 'context');
      if (createdOutside) label.title = 'Created in this session; outside the current filters';
      title.append(label);
    }
    row.append(chevron, status, title, tags, id, actions, description, menu);
    row.addEventListener('click', event => {
      if (event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey || event.target.closest('a, button, select, [popover]')) return;
      navigate({ ...state, item: item.id }, { focus: true });
    });
    fragment.append(row);
    if (item.id === inlineParent) {
      inlineMount.style.setProperty('--depth', Math.min(depth + 1, 6));
      fragment.append(inlineMount);
    }
  }
  // Keep a draft mount available for explicit missing-parent recovery too.
  if (inlineParent && !table.visible.has(inlineParent)) fragment.append(inlineMount);
  $('#items').replaceChildren(fragment);
  $('#list-scroll').scrollTop = scroll;
  if (activeRow && activeKind && !active.isConnected) {
    const row = [...$('#items').children].find(node => node.dataset.row === activeRow);
    (row?.querySelector(activeKind) || $('#list-scroll')).focus({ preventScroll: true });
  } else if (inlineMount.contains(active)) active.focus({ preventScroll: true });
  $('#empty').hidden = table.matches.length > 0 || Boolean(inlineParent);
  $('#empty').replaceChildren();
  if (!table.matches.length) {
    const missing = state.under && !index.byID.has(state.under);
    $('#empty').append(el('h2', '', missing ? 'Under item not found' : index.items.length ? 'No matching items' : 'No items yet'),
      el('p', '', missing ? `${state.under} is not in this project. Choose another Under parent or clear filters.` : index.items.length ? 'Try another search, clear tags, or choose All to include done and canceled work.' : 'Choose New item to create your first item.'));
    if (index.items.length || state.under) {
      const clear = el('button', 'text-button', 'Clear filters');
      clear.type = 'button';
      clear.addEventListener('click', clearFilters);
      $('#empty').append(clear);
    }
  }
  updateLinks();
  refreshRowStatuses();
}
// Stable mount and visibility hook for repeated inline child creation. The caller
// owns its input DOM; pins never count as matches or mutate filters/collapse state.
export function setInlineChildContext(parent = '', createdIDs = []) {
  inlineParent = parent;
  inlinePins = parent ? [parent, ...createdIDs] : [];
  renderList();
  return inlineMount;
}
function updateSelectionContext() {
  const note = $('#selection-context');
  const selected = index?.byID.get(state.item);
  note.hidden = !selected || (table?.matching.has(state.item) && table.visible.has(state.item));
  note.textContent = !selected ? '' : !table?.matching.has(state.item)
    ? 'This item is outside the current list filters. Your filters are preserved.'
    : 'This item is hidden by a collapsed ancestor. Your collapse choices are preserved.';
}
function updateLinks() {
  updateSelectionContext();
  for (const link of document.querySelectorAll('a[data-item]')) {
    link.href = itemHref(link.dataset.item);
    if (link.classList.contains('item-link')) {
      if (link.dataset.item === state.item) link.setAttribute('aria-current', 'true');
      else link.removeAttribute('aria-current');
    }
  }
}
function section(title) {
  const section = el('section', 'detail-section');
  section.append(el('h3', '', title));
  return section;
}
function relationships(title, items, note) {
  const part = section(title);
  if (note) part.append(el('p', 'relationship-note', note));
  if (!items.length) part.append(el('p', 'hint', 'None'));
  const list = el('ul', 'relations');
  for (const item of items) {
    const row = el('li');
    row.append(el('span', 'item-id', item.id), itemLink(item), badge(item.status));
    list.append(row);
  }
  part.append(list);
  return part;
}
function announce(text) {
  clearTimeout(noticeTimer);
  $('#save-notice').textContent = text;
  if (text) noticeTimer = setTimeout(() => { $('#save-notice').textContent = ''; }, 5000);
}
function copyButton(label, value) {
  const button = el('button', '', label);
  button.type = 'button';
  button.addEventListener('click', async () => {
    const text = value();
    const feedback = $('#copy-feedback');
    // Close the row popover before showing fallback text outside the top layer.
    button.closest('[popover]')?.hidePopover();
    clearTimeout(copyTimer);
    feedback.replaceChildren();
    feedback.hidden = true;
    try {
      await navigator.clipboard.writeText(text);
      feedback.textContent = label === 'Copy ID' ? 'ID copied.' : 'Link copied.';
      feedback.hidden = false;
      copyTimer = setTimeout(() => { feedback.hidden = true; }, 4000);
    } catch {
      feedback.append(el('span', '', 'Could not copy automatically. Select and copy this value:'));
      const input = el('input');
      input.readOnly = true;
      input.value = text;
      input.setAttribute('aria-label', label === 'Copy ID' ? 'ID to copy' : 'Link to copy');
      const dismiss = el('button', '', 'Dismiss');
      dismiss.type = 'button';
      dismiss.addEventListener('click', () => {
        feedback.hidden = true;
        const trigger = button.closest('[popover]');
        (trigger ? document.querySelector(`[popovertarget="${trigger.id}"]`) : button)?.focus({ preventScroll: true });
      });
      feedback.append(input, dismiss);
      feedback.hidden = false;
      input.focus();
      input.select();
    }
  });
  return button;
}
function detailShell() {
  const top = el('div', 'detail-top');
  const back = el('button', 'text-button', '');
  back.type = 'button';
  back.append(el('span', 'detail-back', '← Back to table'), el('span', 'detail-close', '× Close'));
  back.addEventListener('click', () => navigate({ ...state, item: '' }, { focus: true }));
  top.append(back, el('span', 'item-id', state.item), copyButton('Copy ID', () => state.item), copyButton('Copy link', () => location.href));
  $('#detail-top').replaceChildren(top);
  $('#detail-content').replaceChildren();
}
function updateDraft() {
  $('#new-item').disabled = !index || stale;
  refreshRowStatuses();
  const context = drafts.inspect(index, new Set(index ? selectItems(index, state).map(item => item.id) : []), stale);
  const notice = $('#draft-notice');
  $('#workspace').classList.toggle('has-inline', inlineChildren.isOpen());
  $('#workspace').classList.toggle('has-editor', Boolean(context?.dirty && context.id));
  if (inlineChildren.isOpen()) {
    notice.hidden = false;
    notice.textContent = 'Child creation is active. Created siblings are saved; the draft stays under its original parent. New* marks created items outside filters. ';
    const back = el('button', 'text-button', 'Return to child draft'); back.type = 'button';
    back.addEventListener('click', inlineChildren.focus); notice.append(back);
    return;
  }
  notice.hidden = !context?.dirty;
  if (!context?.dirty) return;
  notice.textContent = `Unsaved draft for ${context.id || 'a new item'} is preserved while browsing. Save or cancel before editing another item. ` +
    (context.stale ? 'Project data is stale; reconnect or repair the named files before saving.' :
      context.id && context.missing ? 'This item was removed. Review the draft before discarding it.' :
        context.changed ? 'This item or its related links changed externally. Review the current version before saving.' : '') +
    (context.outsideFilters ? ' The item is outside the current list filters.' : '');
  if (context.id && context.id !== state.item) {
    const link = itemLink({ id: context.id, title: 'Return to draft' });
    notice.append(' ', link);
  }
}
function showProblem(error) {
  stale = true;
  $('#workspace').hidden = !index;
  $('#problem').hidden = false;
  $('#diagnostics').textContent = error.message;
  $('#status').textContent = (error.status ? 'Project needs attention — retrying automatically' : 'Reconnecting to wrk…') +
    (index ? ' · Showing stale last-valid data' : ' · No valid snapshot available');
  $('.connection').dataset.state = 'stale';
  $('#connection-label').textContent = error.status ? 'Needs attention' : 'Reconnecting';
  $('#problem-summary').textContent = (error.status ? 'Project needs attention' : 'Reconnecting to wrk') + (index ? ' · Showing stale data' : ' · Waiting for project data');
  updateDraft();
}
function renderDetail() {
  const selected = state.item;
  $('#workspace').classList.toggle('has-selection', Boolean(selected));
  // Skip unchanged detail DOM, preserving disclosure, keyboard and scroll state.
  const detail = snapshot?.result.selected === selected ? snapshot.result.detail : null;
  const outside = index && !selectItems(index, state).some(t => t.id === selected);
  const pending = selected && index?.byID.has(selected) && snapshot?.result.selected !== selected;
  const key = JSON.stringify([selected, detail, outside, pending]);
  if (key === renderedDetail) {
    if (detail) detailEditor.sync({ ...detail.ticket, body: detail.body }, detail.body_html);
    return;
  }
  renderedDetail = key;
  const scroll = detailScroll ?? $('#detail').scrollTop;
  const focusedControl = $('#detail-content').contains(document.activeElement) ? document.activeElement : null;
  const disclosures = [...$('#detail-content').querySelectorAll('details')].map(node => node.open);
  $('#detail').setAttribute('aria-busy', String(Boolean(pending)));
  if (!selected) {
    detailEditor.sync(null);
    $('#detail-top').replaceChildren();
    $('#detail-content').replaceChildren();
    detailFocus = false;
    detailScroll = null;
    return;
  }
  lastSelected = selected;
  detailShell();
  if (pending) {
    $('#detail-content').append(el('p', 'empty', 'Loading item…'));
    return;
  }
  if (!detail) {
    detailEditor.sync(null);
    $('#detail-content').append(el('h2', 'detail-title', 'Item not found'), el('p', '', `${selected} is not in this project. It may have been removed. Your selection and any draft are preserved.`));
  } else {
    const data = { result: detail };
    const item = detail.ticket;
    detailEditor.sync({ ...item, body: detail.body }, detail.body_html);
    if (detailEditor.id() !== selected) $('#detail-content').append(el('h2', 'detail-title', item.title));
    if (item.status === 'blocked') $('#detail-content').append(el('p', 'blockers', 'Manually blocked. This status stays explicit even when dependencies finish.'));
    if (item.blockers.length) $('#detail-content').append(el('p', 'blockers', `Unfinished dependencies: ${item.blockers.map(b => `${b.id} (${b.status})`).join(', ')}`));
    const focusButton = el('button', 'focus-button', 'Show descendants');
    focusButton.type = 'button';
    focusButton.addEventListener('click', () => navigate({ ...state, under: selected }));
    $('#detail-content').append(focusButton);
    const metadata = el('details', 'detail-section');
    metadata.append(el('summary', '', 'Custom fields & original metadata (YAML)'), el('pre', '', data.result.metadata));
    const source = el('details', 'detail-section');
    source.append(el('summary', '', 'Original Markdown source'), el('pre', '', data.result.body));
    $('#detail-content').append(el('p', 'hint', `${item.priority} priority`),
      relationships('Children', data.result.children, `${progress(data.result.children)} · Direct children only; parent status stays explicit.`),
      relationships('Dependents', data.result.dependents, 'Items that depend on this one.'),
      metadata, source);
    updateLinks();
  }
  [...$('#detail-content').querySelectorAll('details')].forEach((node, i) => { node.open = disclosures[i] || false; });
  $('#detail').scrollTop = scroll;
  if (detailFocus || (focusedControl && !focusedControl.isConnected)) $('#detail').focus({ preventScroll: true });
  detailFocus = false;
  detailScroll = null;
}
function applySnapshot(data) {
  const changed = Boolean(data);
  if (data) snapshot = data;
  if (!snapshot) throw new Error('Missing current snapshot; retrying');
  stale = false;
  $('#problem').hidden = true;
  $('.connection').dataset.state = 'live';
  $('#workspace').hidden = false;
  if (changed) {
    // Capture context when the response arrives, not when the request started:
    // people may scroll, type, or change filters while a read is in flight.
    const scroll = index ? listScroll() : history.state?.listScroll || 0;
    const labelScroll = $('#label-options').scrollTop;
    const focusDetail = detailFocus;
    const active = document.activeElement;
    const activeItem = active?.dataset.item;
    const activeAction = active?.dataset.rowAction;
    const activeLabel = active?.closest('#label-options') ? active.value : null;
    index = indexItems(snapshot.result.tickets);
    $('#project').textContent = snapshot.project_root;
    $('#project-name').textContent = snapshot.project_root.split('/').filter(Boolean).pop() || snapshot.result.project.config.prefix;
    $('#project-name').title = $('#project-name').textContent;
    $('#project-info').textContent = `${index.items.length} items · ${snapshot.result.project.config.prefix} · Stored in .wrk`;
    document.title = `${$('#project-name').textContent} · wrk`;
    buildFilters();
    renderList();
    renderDetail();
    $('#list-scroll').scrollTop = scroll;
    $('#label-options').scrollTop = labelScroll;
    if (!focusDetail && activeItem && !active.isConnected) [...$('#items').querySelectorAll('a')].find(node => node.dataset.item === activeItem)?.focus({ preventScroll: true });
    if (!focusDetail && activeAction && !active.isConnected) document.querySelector(`[data-row-action="${activeAction}"]`)?.focus({ preventScroll: true });
    if (!focusDetail && activeLabel !== null) [...$('#label-options').querySelectorAll('input')].find(node => node.value === activeLabel)?.focus({ preventScroll: true });
  }
  $('#connection-label').textContent = 'Live';
  $('#status').textContent = `Live · ${index.items.length} items · ${snapshot.result.project.config.prefix} · Stored in .wrk`;
  $('#reload').disabled = false;
  $('#workspace').setAttribute('aria-busy', 'false');
  updateDraft();
}
const editor = createEditor({
  mount: $('#draft'), modal: $('#create-dialog'), drafts,
  getData: () => ({ index, project: snapshot?.result.project, stale }),
  refresh: () => poller.refresh(),
  onSaved: async id => { navigate({ ...state, item: id }, { focus: true }); await poller.refresh(); },
  onToggle: (open, creating) => {
    saveScroll();
    $('#workspace').classList.toggle('has-editor', open && !creating);
    $('#detail-content').hidden = false;
    if (!open) {
      $('#list-scroll').scrollTop = history.state?.listScroll || 0;
      (state.item ? $('#detail') : $('#new-item')).focus({ preventScroll: true });
    }
  },
  onChange: updateDraft,
  announce,
});
const detailEditor = createDetailEditor({ mount: $('#draft'), drafts,
  statusMount,
  getData: () => ({ index, stale }), refresh: () => poller.refresh(), onChange: updateDraft, announce, itemLink,
  modalOpen: () => editor.isOpen(), reveal: id => { if (state.item !== id) navigate({ ...state, item: id }); },
  creationOpen: () => editor.isOpen() || inlineChildren.isOpen(),
});
const inlineChildren = createInlineChildren({ drafts, getData: () => ({ index, stale }), setContext: setInlineChildContext,
  refresh: () => poller.refresh(), onChange: updateDraft, announce, itemLink,
  onCreated: item => { index = indexItems([...index.items.filter(current => current.id !== item.id), { ...item, body: '' }]); },
  restoreFocus: id => {
    renderDetail(); poller.refresh();
    (document.querySelector(`[data-row-action="${id}"]`) || $('#search')).focus({ preventScroll: true });
  },
});
$('#new-item').addEventListener('click', () => {
  if (inlineChildren.isOpen()) { announce('Finish or cancel the child draft before starting another item.'); inlineChildren.focus(); return; }
  if (detailEditor.yieldDraft()) editor.open();
});
function refreshRowStatuses() {
  detailEditor.refreshStatus();
  for (const control of document.querySelectorAll('[data-row-status]')) control.disabled = stale || editor.isOpen() || detailEditor.statusBusy();
}
// Shared entry point for panel/row status controls; always uses the active draft's preconditions.
export function changeItemStatus(id, status) {
  const item = index?.byID.get(id);
  return item ? detailEditor.mutate(item, { status }) : Promise.resolve(false);
}
const poller = createPoller({
  async read({ signal, etag }) {
    const selected = /^[a-z][a-z0-9]{0,15}-[a-f0-9]{8}$/.test(state.item) ? state.item : '';
    const path = '/api/workspace' + (selected ? `?selected=${encodeURIComponent(selected)}` : '');
    const response = await fetch(path, { cache: 'no-store', signal: AbortSignal.any([signal, AbortSignal.timeout(20000)]),
      headers: etag ? { 'If-None-Match': etag } : {} });
    if (response.status === 304 && etag) return { data: null, etag };
    const data = await response.json();
    if (!response.ok || !data.ok) {
      const error = new Error(data.errors.map(d => `${d.code}: ${d.path || ''}${d.field ? ` [${d.field}]` : ''}${d.line ? `:${d.line}` : ''} ${d.message}`).join('\n'));
      error.status = response.status;
      throw error;
    }
    return { data, etag: response.headers.get('ETag') };
  },
  onData: applySnapshot,
  onChecking(force) {
    if (force && index) {
      stale = true;
      $('.connection').dataset.state = 'stale';
      $('#connection-label').textContent = 'Checking…';
      $('#status').textContent = 'Checking for updates · Retained data is stale until checked';
      updateDraft();
    }
  },
  onError(error) {
    showProblem(error);
    $('#reload').disabled = false;
    $('#workspace').setAttribute('aria-busy', 'false');
  },
});
// Background throttling and BFCache can suspend timers. Each resume signal
// abandons obsolete requests and resynchronizes with a full current snapshot.
for (const event of ['online', 'focus']) window.addEventListener(event, () => {
  if (document.visibilityState === 'visible') poller.refresh();
});
window.addEventListener('pageshow', event => { if (event.persisted && document.visibilityState === 'visible') poller.refresh(); });
window.addEventListener('offline', () => showProblem(new Error('Connection interrupted; retrying automatically.')));
window.addEventListener('pagehide', () => poller.stop());
function syncVisibility() {
  if (document.visibilityState === 'visible') poller.refresh();
  else {
    poller.stop();
    stale = true;
    $('.connection').dataset.state = 'stale';
    $('#connection-label').textContent = 'Paused';
    $('#status').textContent = 'Updates paused while hidden · ' + (index ? 'Showing stale last-valid data' : 'No valid snapshot loaded');
    updateDraft();
  }
}
document.addEventListener('visibilitychange', syncVisibility);
$('#search').addEventListener('input', () => navigate({ ...state, q: $('#search').value }, { replace: true }));
for (const button of document.querySelectorAll('[data-view]')) button.addEventListener('click', () => navigate({ ...state, view: button.dataset.view }));
function clearFilters() {
  underPicker.clear();
  underPicker.close(false);
  navigate({ ...readState(''), item: state.item });
  $('#search').focus({ preventScroll: true });
}
$('#reset').addEventListener('click', clearFilters);
$('#reload').addEventListener('click', () => {
  $('#project-menu').open = false;
  $('#project-menu summary').focus();
  poller.refresh();
});
// Native disclosures keep controls keyboard-operable without a custom menu role.
for (const disclosure of document.querySelectorAll('.disclosure')) {
  disclosure.addEventListener('keydown', event => {
    if (event.key === 'Escape') { disclosure.open = false; disclosure.querySelector('summary').focus(); }
  });
  document.addEventListener('click', event => {
    if (!disclosure.contains(event.target)) disclosure.open = false;
  });
}
$('.skip-link').addEventListener('click', event => {
  event.preventDefault();
  (state.item ? $('#detail') : $('#list-scroll')).focus({ preventScroll: true });
});
document.addEventListener('click', event => {
  const link = event.target.closest('a[data-item]');
  if (!link || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
  event.preventDefault();
  navigate({ ...state, item: link.dataset.item }, { focus: true });
});
renderControls();
$('#reload').disabled = true;
if (document.visibilityState === 'hidden') syncVisibility();
else poller.refresh();
