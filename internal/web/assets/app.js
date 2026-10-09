import { readState, stateHash, filterKey, indexItems, selectItems, progress } from './model.mjs';

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
let detailRequest;
let projectRequest;
let detailVersion = 0;
let lastSelected = '';
let loading = false;

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
  if (previous.item !== state.item) renderDetail(focus, scroll.detailScroll || 0);
  else $('#detail').scrollTop = scroll.detailScroll || 0;
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
      el('p', '', missing ? `${state.focus} is not in this project. Choose another parent focus or reset filters.` : index.items.length ? 'Try another search, clear labels, or choose All to include done and canceled work.' : 'Create your first item with wrk new "Title", then reload.'));
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
  $('#detail').replaceChildren(top);
}
async function request(path, signal) {
  const response = await fetch(path, { cache: 'no-store', signal });
  const data = await response.json();
  if (!response.ok || !data.ok) {
    const error = new Error(data.errors.map(d => `${d.code}: ${d.path || ''}${d.field ? ` [${d.field}]` : ''}${d.line ? `:${d.line}` : ''} ${d.message}`).join('\n'));
    error.status = response.status;
    if (data.project_root) $('#project').textContent = data.project_root;
    throw error;
  }
  return data;
}
function showProblem(error) {
  index = null;
  detailVersion++;
  detailRequest?.abort();
  $('#workspace').hidden = true;
  $('#problem').hidden = false;
  $('#diagnostics').textContent = error.message;
  $('#status').textContent = error.status ? 'Project needs attention' : 'Unable to reach wrk — check the server, then reload';
}
async function renderDetail(focus = false, scroll = 0) {
  detailRequest?.abort();
  const version = ++detailVersion;
  const selected = state.item;
  $('#workspace').classList.toggle('has-selection', Boolean(selected));
  if (!selected) {
    $('#detail').replaceChildren(el('div', 'empty detail-placeholder', 'Select an item to explore its description and connections.'));
    return;
  }
  if (!index || loading) return;
  lastSelected = selected;
  detailShell();
  $('#detail').append(el('p', 'empty', 'Loading item…'));
  $('#detail').setAttribute('aria-busy', 'true');
  detailRequest = new AbortController();
  try {
    const data = await request(`/api/items/${encodeURIComponent(selected)}`, AbortSignal.any([detailRequest.signal, AbortSignal.timeout(20000)]));
    if (version !== detailVersion) return;
    const item = data.result.ticket;
    detailShell();
    const title = el('h2', 'detail-title', item.title);
    const badges = el('div', 'badges');
    badges.append(badge(item.status), el('span', `badge priority-${item.priority}`, `${item.priority} priority`));
    for (const label of item.labels) badges.append(el('span', 'badge', label));
    $('#detail').append(title, badges);
    if (item.status === 'blocked') $('#detail').append(el('p', 'blockers', 'Manually blocked. This status stays explicit even when dependencies finish.'));
    if (item.blockers.length) $('#detail').append(el('p', 'blockers', `Unfinished dependencies: ${item.blockers.map(b => `${b.id} (${b.status})`).join(', ')}`));
    if (!selectItems(index, state).some(t => t.id === selected)) $('#detail').append(el('p', 'hint', 'This item is outside the current list filters. Your filters are preserved.'));
    const focusButton = el('button', 'focus-button', 'Focus this item + descendants');
    focusButton.type = 'button';
    focusButton.addEventListener('click', () => navigate({ ...state, focus: selected }));
    $('#detail').append(focusButton);
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
    $('#detail').append(description,
      relationships('Parent', data.result.parent ? [data.result.parent] : []),
      relationships('Children', data.result.children, `${progress(data.result.children)} · Direct children only; parent status stays explicit.`),
      relationships('Dependencies', data.result.dependencies, 'Only done dependencies satisfy readiness; canceled dependencies still block.'),
      relationships('Dependents', data.result.dependents, 'Items that depend on this one.'), metadata, source);
    updateLinks();
  } catch (error) {
    if (version !== detailVersion) return;
    if (error.status === 404) {
      detailShell();
      $('#detail').append(el('h2', 'detail-title', 'Item not found'), el('p', '', `${selected} is not in this project. It may have been removed. Select another item or reload.`));
    } else {
      showProblem(error);
    }
  } finally {
    if (version === detailVersion) {
      $('#detail').setAttribute('aria-busy', 'false');
      $('#detail').scrollTop = scroll;
      if (focus) $('#detail').focus({ preventScroll: true });
    }
  }
}
async function loadProject() {
  projectRequest?.abort();
  projectRequest = new AbortController();
  const controller = projectRequest;
  detailRequest?.abort();
  detailVersion++;
  loading = true;
  const scroll = { list: $('#list-scroll').scrollTop, detail: $('#detail').scrollTop };
  $('#reload').disabled = true;
  $('#problem').hidden = true;
  $('#workspace').hidden = true;
  $('#workspace').setAttribute('aria-busy', 'true');
  $('#status').textContent = 'Loading project…';
  try {
    const data = await request('/api/workspace', AbortSignal.any([controller.signal, AbortSignal.timeout(20000)]));
    if (controller !== projectRequest) return;
    index = indexItems(data.result.tickets);
    $('#project').textContent = data.project_root;
    $('#project-name').textContent = data.project_root.split('/').filter(Boolean).pop() || data.result.project.config.prefix;
    document.title = `${$('#project-name').textContent} · wrk`;
    $('#status').textContent = `${index.items.length} items · ${data.result.project.config.prefix} · Stored in .wrk`;
    $('#workspace').hidden = false;
    buildFilters();
    renderList();
    $('#list-scroll').scrollTop = scroll.list;
    loading = false;
    await renderDetail(false, scroll.detail);
  } catch (error) {
    if (controller === projectRequest) showProblem(error);
  } finally {
    if (controller === projectRequest) {
      loading = false;
      $('#reload').disabled = false;
      $('#workspace').setAttribute('aria-busy', 'false');
    }
  }
}
$('#search').addEventListener('input', () => navigate({ ...state, q: $('#search').value }, { replace: true }));
for (const button of document.querySelectorAll('[data-view]')) button.addEventListener('click', () => navigate({ ...state, view: button.dataset.view }));
$('#focus').addEventListener('change', () => navigate({ ...state, focus: $('#focus').value }));
$('#reset').addEventListener('click', () => navigate({ ...readState(''), item: state.item }));
$('#reload').addEventListener('click', loadProject);
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
loadProject();
