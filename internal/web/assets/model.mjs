// URL state and list semantics are shared by the browser and dependency-free tests.
export const views = ['active', 'all', 'ready', 'blocked'];
export function readState(hash) {
  const p = new URLSearchParams(hash.replace(/^#/, ''));
  return { q: p.get('q') || '', view: views.includes(p.get('view')) ? p.get('view') : 'active',
    labels: [...new Set(p.getAll('label').filter(Boolean))].sort(),
    // Legacy inclusive focus links now use descendant-only Under semantics.
    under: p.get('under') ?? p.get('focus') ?? '', item: p.get('item') || '' };
}
export function stateHash(state) {
  const p = new URLSearchParams();
  if (state.q) p.set('q', state.q);
  if (state.view !== 'active') p.set('view', state.view);
  for (const label of [...state.labels].sort()) p.append('label', label);
  if (state.under) p.set('under', state.under);
  if (state.item) p.set('item', state.item);
  return `#${p}`;
}
export function filterKey(state) { return stateHash({ ...state, item: '' }); }
export function indexItems(items) {
  items = [...items].sort((a, b) => a.id < b.id ? -1 : a.id > b.id ? 1 : 0);
  const byID = new Map(items.map(item => [item.id, item]));
  const children = new Map();
  for (const item of items) {
    const parent = item.parent || '';
    if (!children.has(parent)) children.set(parent, []);
    children.get(parent).push(item);
  }
  const order = [];
  const stack = (children.get('') || []).map(item => ({ item, depth: 0 })).reverse();
  while (stack.length) {
    const entry = stack.pop();
    order.push(entry);
    const descendants = children.get(entry.item.id) || [];
    for (let n = descendants.length - 1; n >= 0; n--) stack.push({ item: descendants[n], depth: entry.depth + 1 });
  }
  return { items, byID, children, order };
}
// Trim the query, lowercase without locale-dependent ordering, then require each
// Unicode character in order (gaps allowed). No ranking, tokenization or body/ID search.
export function fuzzyTitle(title, query) {
  const characters = [...query.trim().toLowerCase()];
  if (!characters.length) return true;
  let next = 0;
  for (const character of title.toLowerCase()) {
    if (character === characters[next] && ++next === characters.length) return true;
  }
  return false;
}
export function selectItems(index, state) {
  let scope;
  if (state.under) {
    scope = new Set();
    const queue = [...(index.children.get(state.under) || [])];
    while (queue.length) {
      const item = queue.pop();
      scope.add(item.id);
      for (const child of index.children.get(item.id) || []) queue.push(child);
    }
  }
  return index.items.filter(item => {
    if (scope && !scope.has(item.id)) return false;
    if (!state.labels.every(label => item.labels.includes(label))) return false;
    if (state.view === 'active' && !['todo', 'in-progress', 'blocked'].includes(item.status)) return false;
    // Blockers come from the global validated dependency graph, before filtering.
    if (state.view === 'ready' && (item.status !== 'todo' || item.blockers.length)) return false;
    if (state.view === 'blocked' && item.status !== 'blocked') return false;
    return fuzzyTitle(item.title, state.q);
  });
}
export function exploring(state) {
  return Boolean(state.q.trim() || state.labels.length || state.under || ['ready', 'blocked'].includes(state.view));
}
// Linear passes, including ancestor closure: a deep chain never recurses or walks
// every ancestor once per match. Pins are reserved for the inline-creation session;
// they reveal context without changing membership or the user's collapse choices.
export function treeRows(index, state, collapsed = new Set(), pins = [], expandedPins = []) {
  const matches = selectItems(index, state);
  const matching = new Set(matches.map(item => item.id));
  const included = new Set(matching);
  const forced = new Set(expandedPins);
  const pinned = new Set(pins.filter(id => index.byID.has(id)));
  for (const id of pinned) included.add(id);
  const explore = exploring(state);
  const hasChildren = new Set();
  for (let n = index.order.length - 1; n >= 0; n--) {
    const { item } = index.order[n];
    if (!included.has(item.id)) continue;
    const parent = item.parent;
    // Under starts at its root. Pins may be outside that scope during creation.
    if (!parent || (item.id === state.under && !pinned.has(item.id))) continue;
    included.add(parent);
    hasChildren.add(parent);
    if (explore || pinned.has(item.id)) forced.add(parent);
    if (pinned.has(item.id)) pinned.add(parent);
  }
  const rows = [], visible = new Set();
  let hiddenBelow = Infinity;
  let rootDepth = 0;
  for (const { item, depth } of index.order) {
    if (depth <= hiddenBelow) hiddenBelow = Infinity;
    if (depth > hiddenBelow || !included.has(item.id)) continue;
    if (!item.parent || !included.has(item.parent)) rootDepth = depth;
    const expanded = hasChildren.has(item.id) && (forced.has(item.id) || !collapsed.has(item.id));
    rows.push({ item, depth: depth - rootDepth, context: !matching.has(item.id),
      hasChildren: hasChildren.has(item.id), expanded, forced: forced.has(item.id) });
    visible.add(item.id);
    if (!expanded) hiddenBelow = depth;
  }
  return { rows, matches, matching, visible };
}
export function progress(children) {
  return `${children.filter(item => item.status === 'done').length}/${children.length} children done`;
}
