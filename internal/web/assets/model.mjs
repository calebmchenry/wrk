// URL state and list semantics are shared by the browser and dependency-free tests.
export const views = ['active', 'all', 'ready', 'blocked'];
export function readState(hash) {
  const p = new URLSearchParams(hash.replace(/^#/, ''));
  return { q: p.get('q') || '', view: views.includes(p.get('view')) ? p.get('view') : 'active',
    labels: [...new Set(p.getAll('label').filter(Boolean))].sort(), focus: p.get('focus') || '', item: p.get('item') || '' };
}
export function stateHash(state) {
  const p = new URLSearchParams();
  if (state.q) p.set('q', state.q);
  if (state.view !== 'active') p.set('view', state.view);
  for (const label of [...state.labels].sort()) p.append('label', label);
  if (state.focus) p.set('focus', state.focus);
  if (state.item) p.set('item', state.item);
  return `#${p}`;
}
export function filterKey(state) { return stateHash({ ...state, item: '' }); }
export function indexItems(items) {
  const byID = new Map(items.map(item => [item.id, item]));
  const children = new Map();
  for (const item of items) {
    if (!children.has(item.parent)) children.set(item.parent, []);
    children.get(item.parent).push(item);
  }
  return { items, byID, children };
}
export function selectItems(index, state) {
  let scope;
  if (state.focus) {
    scope = new Set();
    const queue = index.byID.has(state.focus) ? [state.focus] : [];
    while (queue.length) {
      const id = queue.pop();
      if (scope.has(id)) continue;
      scope.add(id);
      for (const item of index.children.get(id) || []) queue.push(item.id);
    }
  }
  const query = state.q.trim().toLowerCase();
  return index.items.filter(item => {
    if (scope && !scope.has(item.id)) return false;
    if (!state.labels.every(label => item.labels.includes(label))) return false;
    if (state.view === 'active' && !['todo', 'in-progress', 'blocked'].includes(item.status)) return false;
    if (state.view === 'ready' && (item.status !== 'todo' || item.blockers.length)) return false;
    if (state.view === 'blocked' && item.status !== 'blocked') return false;
    return !query || `${item.id}\n${item.title}\n${item.body}`.toLowerCase().includes(query);
  });
}
export function progress(children) {
  return `${children.filter(item => item.status === 'done').length}/${children.length} children done`;
}
