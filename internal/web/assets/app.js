import { readState, stateHash, filterKey, indexItems, selectItems, progress } from './model.mjs';
import { createPoller, drafts } from './live.mjs';

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

function badge(status) { return el('span', `badge status-${status}`, names[status] || status); }
function itemHref(id) { return stateHash({ ...state, item: id }); }
function itemLink(item, className = '') {
  const link = el('a', className, item.title);
  link.href = itemHref(item.id);
  link.dataset.item = item.id;
  return link;
}
function saveScroll() {
  history.replaceState({ listScroll: $('#list-scroll').scrollTop, detailScroll: $('#detail').scrollTop }, '', location.href);
}
// Scroll is stored per history entry, including before a browser Back action.
$('#list-scroll').addEventListener('scroll', saveScroll, { passive: true });
$('#detail').addEventListener('scroll', saveScroll, { passive: true });
history.scrollRestoration = 'manual';

function navigate(next, { replace = false, focus = false } = {}) {
  saveScroll();
  const scroll = { listScroll: filterKey(next) === filterKey(state) ? $('#list-scroll').scrollTop : 0, detailScroll: next.item === state.item ? $('#detail').scrollTop : 0 };
  history[replace ? 'replaceState' : 'pushState'](scroll, '', stateHash(next));
  syncState(focus);
}
function syncState(focus = false) {
  const previous = state;
  const scroll = history.state || {};
  state = readState(location.hash);
  $('#workspace').classList.toggle('has-selection', Boolean(state.item));
  renderControls();
  if (index && filterKey(previous) !== filterKey(state)) renderList();
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
window.addEventListener('popstate', () => syncState());
window.addEventListener('hashchange', () => syncState());

function renderControls() {
  $('#search').value = state.q;
  for (const button of document.querySelectorAll('[data-view]')) button.setAttribute('aria-pressed', String(state.view === button.dataset.view));
  $('#view-help').textContent = help[state.view];
  $('#label-count').textContent = state.labels.length ? `(${state.labels.length})` : '';
  for (const input of $('#label-options').querySelectorAll('input')) input.checked = state.labels.includes(input.value);
  $('#focus').value = state.focus;
}
function buildFilters() {
  const labels = [...new Set([...index.items.flatMap(item => item.labels), ...state.labels])].sort();
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
  if (!labels.length) $('#label-options').append(el('p', 'hint', 'No labels in this project.'));
  $('#focus').replaceChildren(new Option('Entire project', ''));
  // Every item can be a focus root, including leaves and completed parents.
  for (const item of index.items) $('#focus').add(new Option(`${item.id} · ${item.title}`, item.id));
  if (state.focus && !index.byID.has(state.focus)) $('#focus').add(new Option(`${state.focus} · Missing item`, state.focus));
  renderControls();
}
function renderList() {
  if (!index) return;
  const items = selectItems(index, state);
  $('#total').textContent = index.items.length;
  $('#results').textContent = `${items.length} of ${index.items.length} items`;
  const fragment = document.createDocumentFragment();
  for (const item of items) {
    const row = el('li');
    const link = itemLink(item, 'item-link');
    link.textContent = '';
    const top = el('div', 'item-top');
    top.append(el('span', 'item-id', item.id), badge(item.status));
    const badges = el('div', 'badges');
    badges.append(el('span', `badge priority-${item.priority}`, `${item.priority} priority`));
    for (const label of item.labels) badges.append(el('span', 'badge', label));
    link.append(top, el('span', 'item-title', item.title), badges);
    if (item.blockers.length) link.append(el('p', 'blockers', `Waiting on ${item.blockers.map(b => `${b.id} (${b.status})`).join(', ')}`));
    const children = index.children.get(item.id) || [];
    if (children.length) link.append(el('p', 'child-count', progress(children)));
    row.append(link);
    fragment.append(row);
  }
  $('#items').replaceChildren(fragment);
  $('#empty').hidden = items.length > 0;
  $('#empty').replaceChildren();
  if (!items.length) {
    const missing = state.focus && !index.byID.has(state.focus);
    $('#empty').append(el('h2', '', missing ? 'Focus item not found' : index.items.length ? 'No matching items' : 'No items yet'),
      el('p', '', missing ? `${state.focus} is not in this project. Choose another parent focus or reset filters.` : index.items.length ? 'Try another search, clear labels, or choose All to include done and canceled work.' : 'Create your first item with wrk new "Title", and it will appear automatically.'));
  }
  updateLinks();
}
function updateLinks() {
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
function detailShell() {
  const top = el('div', 'detail-top');
  const back = el('button', 'text-button', '← Back to list');
  back.type = 'button';
  back.addEventListener('click', () => navigate({ ...state, item: '' }, { focus: true }));
  top.append(back, el('span', 'item-id', state.item));
  $('#detail-content').replaceChildren(top);
}
function updateDraft() {
  const context = drafts.inspect(index, new Set(index ? selectItems(index, state).map(item => item.id) : []), stale);
  const notice = $('#draft-notice');
  notice.hidden = !context?.dirty;
  if (!context?.dirty) return;
  notice.textContent = `Unsaved draft for ${context.id} is preserved. ` +
    (context.stale ? 'Project data is stale; reconnect or repair the named files before saving.' :
      context.missing ? 'This item was removed. Review the draft before discarding it.' :
        context.changed ? 'This item changed externally. Review the current version before saving.' : '') +
    (context.outsideFilters ? ' The item is outside the current list filters.' : '');
}
function showProblem(error) {
  stale = true;
  $('#workspace').hidden = !index;
  $('#problem').hidden = false;
  $('#diagnostics').textContent = error.message;
  $('#status').textContent = (error.status ? 'Project needs attention — retrying automatically' : 'Reconnecting to wrk…') +
    (index ? ' · Showing stale last-valid data' : ' · No valid snapshot available');
  $('.connection').dataset.state = 'stale';
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
  if (key === renderedDetail) return;
  renderedDetail = key;
  const scroll = detailScroll ?? $('#detail').scrollTop;
  const disclosures = [...$('#detail-content').querySelectorAll('details')].map(node => node.open);
  $('#detail').setAttribute('aria-busy', String(Boolean(pending)));
  if (!selected) {
    $('#detail-content').replaceChildren(el('div', 'empty detail-placeholder', 'Select an item to explore its description and connections.'));
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
    $('#detail-content').append(el('h2', 'detail-title', 'Item not found'), el('p', '', `${selected} is not in this project. It may have been removed. Your selection and any draft are preserved.`));
  } else {
    const data = { result: detail };
    const item = detail.ticket;
    const title = el('h2', 'detail-title', item.title);
    const badges = el('div', 'badges');
    badges.append(badge(item.status), el('span', `badge priority-${item.priority}`, `${item.priority} priority`));
    for (const label of item.labels) badges.append(el('span', 'badge', label));
    $('#detail-content').append(title, badges);
    if (item.status === 'blocked') $('#detail-content').append(el('p', 'blockers', 'Manually blocked. This status stays explicit even when dependencies finish.'));
    if (item.blockers.length) $('#detail-content').append(el('p', 'blockers', `Unfinished dependencies: ${item.blockers.map(b => `${b.id} (${b.status})`).join(', ')}`));
    if (!selectItems(index, state).some(t => t.id === selected)) $('#detail-content').append(el('p', 'hint', 'This item is outside the current list filters. Your filters are preserved.'));
    const focusButton = el('button', 'focus-button', 'Focus this item + descendants');
    focusButton.type = 'button';
    focusButton.addEventListener('click', () => navigate({ ...state, focus: selected }));
    $('#detail-content').append(focusButton);
    const description = section('Description');
    const markdown = el('div', 'markdown');
    // This is the only HTML sink: the server's restricted Goldmark renderer owns
    // body_html. All metadata, diagnostics, raw source and titles use text nodes.
    markdown.innerHTML = data.result.body_html;
    if (!data.result.body.trim()) markdown.append(el('p', 'hint', 'No description yet.'));
    description.append(markdown);
    const metadata = el('details', 'detail-section');
    metadata.append(el('summary', '', 'Custom fields & original metadata (YAML)'), el('pre', '', data.result.metadata));
    const source = el('details', 'detail-section');
    source.append(el('summary', '', 'Original Markdown source'), el('pre', '', data.result.body));
    $('#detail-content').append(description,
      relationships('Parent', data.result.parent ? [data.result.parent] : []),
      relationships('Children', data.result.children, `${progress(data.result.children)} · Direct children only; parent status stays explicit.`),
      relationships('Dependencies', data.result.dependencies, 'Only done dependencies satisfy readiness; canceled dependencies still block.'),
      relationships('Dependents', data.result.dependents, 'Items that depend on this one.'), metadata, source);
    updateLinks();
  }
  [...$('#detail-content').querySelectorAll('details')].forEach((node, i) => { node.open = disclosures[i] || false; });
  $('#detail').scrollTop = scroll;
  if (detailFocus) $('#detail').focus({ preventScroll: true });
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
    const scroll = $('#list-scroll').scrollTop;
    const filterScroll = $('.filters').scrollTop;
    const labelScroll = $('#label-options').scrollTop;
    const focusDetail = detailFocus;
    const active = document.activeElement;
    const activeItem = active?.dataset.item;
    const activeLabel = active?.closest('#label-options') ? active.value : null;
    index = indexItems(snapshot.result.tickets);
    $('#project').textContent = snapshot.project_root;
    $('#project-name').textContent = snapshot.project_root.split('/').filter(Boolean).pop() || snapshot.result.project.config.prefix;
    document.title = `${$('#project-name').textContent} · wrk`;
    buildFilters();
    renderList();
    renderDetail();
    $('#list-scroll').scrollTop = scroll;
    $('.filters').scrollTop = filterScroll;
    $('#label-options').scrollTop = labelScroll;
    if (!focusDetail && activeItem && !active.isConnected) [...$('#items').querySelectorAll('a')].find(node => node.dataset.item === activeItem)?.focus({ preventScroll: true });
    if (!focusDetail && activeLabel !== null) [...$('#label-options').querySelectorAll('input')].find(node => node.value === activeLabel)?.focus({ preventScroll: true });
  }
  $('#status').textContent = `Live · ${index.items.length} items · ${snapshot.result.project.config.prefix} · Stored in .wrk`;
  $('#reload').disabled = false;
  $('#workspace').setAttribute('aria-busy', 'false');
  updateDraft();
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
    $('#status').textContent = 'Updates paused while hidden · ' + (index ? 'Showing stale last-valid data' : 'No valid snapshot loaded');
    updateDraft();
  }
}
document.addEventListener('visibilitychange', syncVisibility);
$('#search').addEventListener('input', () => navigate({ ...state, q: $('#search').value }, { replace: true }));
for (const button of document.querySelectorAll('[data-view]')) button.addEventListener('click', () => navigate({ ...state, view: button.dataset.view }));
$('#focus').addEventListener('change', () => navigate({ ...state, focus: $('#focus').value }));
$('#reset').addEventListener('click', () => navigate({ ...readState(''), item: state.item }));
$('#reload').addEventListener('click', () => poller.refresh());
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
