import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readState, stateHash, indexItems, selectItems, treeRows, fuzzyTitle, progress } from './assets/model.mjs';

const item = (id, status, extra = {}) => ({ id, title: id, status, body: '', labels: [], blockers: [], parent: null, ...extra });
const items = [item('root', 'in-progress', { labels: ['web'] }),
  item('child', 'todo', { parent: 'root', labels: ['web', 'CLI'], body: 'Needle in description' }),
  item('grandchild', 'blocked', { parent: 'child', labels: ['web'] }),
  item('done', 'done', { parent: 'child' }), item('canceled', 'canceled', { parent: 'root' }),
  item('waiting', 'todo', { labels: ['web'], blockers: [{ id: 'canceled', status: 'canceled' }] }),
  item('manual', 'blocked')];
const index = indexItems(items);
const ids = overrides => selectItems(index, { ...readState(''), ...overrides }).map(i => i.id);

test('status, fuzzy titles, exact AND tags and descendant-only Under intersect', () => {
  assert.deepEqual(ids({}), ['child', 'grandchild', 'manual', 'root', 'waiting']);
  assert.equal(ids({ view: 'all' }).length, 7);
  assert.deepEqual(ids({ view: 'ready' }), ['child']);
  assert.deepEqual(ids({ view: 'blocked' }), ['grandchild', 'manual']);
  assert.deepEqual(ids({ q: 'CHD', labels: ['web', 'CLI'], view: 'ready', under: 'root' }), ['child']);
  assert.deepEqual(ids({ q: 'needle' }), []);
  assert.deepEqual(ids({ labels: ['cli'] }), []);
  assert.deepEqual(ids({ q: 'waiting', view: 'ready', under: 'root' }), []);
  assert.deepEqual(ids({ under: 'root' }), ['child', 'grandchild']);
  assert.deepEqual(ids({ under: 'root', view: 'all' }), ['canceled', 'child', 'done', 'grandchild']);
  assert.deepEqual(ids({ under: 'child', view: 'all' }), ['done', 'grandchild']);
  assert.deepEqual(ids({ under: 'missing' }), []);
  assert.equal(progress(index.children.get('child')), '1/2 children done');
  assert.equal(progress(index.children.get('root')), '0/2 children done');
});

test('fuzzy rule is an ordered Unicode subsequence of the title only', () => {
  for (const [title, query, expected] of [
    ['Build search', 'BSRCH', true], ['Build search', ' search  ', true],
    ['Build search', 'search build', false], ['Build search', 'bbb', false],
    ['Build search', 'b s', true], ['Build search', '', true],
    ['Ship 🦊 tools', '🦊t', true], ['Ångström', 'åö', true],
  ]) assert.equal(fuzzyTitle(title, query), expected, `${title}: ${query}`);
  const fixture = indexItems([item('needle', 'todo', { title: 'Ship tools', body: 'needle' })]);
  assert.equal(selectItems(fixture, { ...readState(''), q: 'needle' }).length, 0);
});

test('fragments round-trip and legacy focus migrates without losing selection', () => {
  const state = { q: 'symbols & # + 🦊', view: 'all', labels: ['CLI', 'two words'], under: 'root', item: 'done' };
  assert.deepEqual(readState(stateHash(state)), state);
  assert.deepEqual(readState('#view=unknown&label=x&label=x&label=&item=missing'), { ...readState(''), labels: ['x'], item: 'missing' });
  assert.equal(readState('#focus=root&item=done').under, 'root');
  assert.equal(readState('#under=child&focus=root').under, 'child');
  assert.equal(stateHash(readState('#focus=root&item=done')), '#under=root&item=done');
});

const tree = (overrides = {}, collapsed = new Set(), source = index, pins = []) => treeRows(source, { ...readState(''), ...overrides }, collapsed, pins);
const rowIDs = result => result.rows.map(row => row.item.id);
test('stable preorder, context membership, collapse and expansion restoration', () => {
  const collapsed = new Set(['root', 'child']);
  assert.deepEqual(rowIDs(tree()), ['manual', 'root', 'child', 'grandchild', 'waiting']);
  assert.deepEqual(rowIDs(tree({}, collapsed)), ['manual', 'root', 'waiting']);
  assert.equal(tree({}, collapsed).matches.length, 5); // Includes matches hidden by collapse.
  for (const filters of [{ q: 'gch' }, { labels: ['web'] }, { under: 'child' }, { view: 'blocked' }, { view: 'ready' }]) {
    const result = tree(filters, collapsed);
    assert.ok(result.rows.some(row => row.depth > 0));
    assert.ok(result.rows.every(row => !row.hasChildren || row.expanded));
  }
  assert.deepEqual(rowIDs(tree({}, collapsed)), ['manual', 'root', 'waiting']);
  assert.deepEqual([...collapsed], ['root', 'child']);
  const under = tree({ under: 'child', view: 'all' });
  assert.deepEqual(rowIDs(under), ['child', 'done', 'grandchild']);
  assert.equal(under.rows[0].context, true);
  assert.equal(under.rows[0].depth, 0);
  assert.equal(under.matches.length, 2);
  assert.deepEqual(rowIDs(tree({ under: 'missing' })), []);
  assert.deepEqual(rowIDs(tree({ q: 'not present' })), []);
});

test('closed ancestors, live reparenting/status changes and creation pins preserve honest counts', () => {
  const source = indexItems([item('a', 'done'), item('b', 'canceled', { parent: 'a' }),
    item('c', 'todo', { parent: 'b' }), item('d', 'todo')]);
  let result = tree({}, new Set(['a']), source);
  assert.deepEqual(rowIDs(result), ['a', 'd']);
  assert.equal(result.matches.length, 2);
  result = tree({ q: 'c' }, new Set(['a']), source);
  assert.deepEqual(rowIDs(result), ['a', 'b', 'c']);
  assert.deepEqual(result.rows.map(row => row.context), [true, true, false]);
  const changed = indexItems(source.items.map(i => i.id === 'c' ? { ...i, parent: 'd' } : i));
  assert.deepEqual(rowIDs(tree({}, new Set(['a']), changed)), ['d', 'c']);
  const completed = indexItems(changed.items.map(i => i.id === 'c' ? { ...i, status: 'done' } : i));
  assert.deepEqual(rowIDs(tree({}, new Set(['a']), completed)), ['d']);
  result = tree({ q: 'not present', under: 'd' }, new Set(['a']), source, ['c']);
  assert.deepEqual(rowIDs(result), ['a', 'b', 'c']);
  assert.equal(result.matches.length, 0);
  assert.ok(result.rows.every(row => row.context));
});

test('10,000-deep hierarchy traversal, ancestor closure and collapse are iterative', () => {
  const chain = indexItems(Array.from({ length: 10000 }, (_, n) => item(String(n), 'todo', { parent: n ? String(n - 1) : null })));
  assert.equal(selectItems(chain, { ...readState(''), under: '0' }).length, 9999);
  const result = tree({ q: '9999' }, new Set(['0']), chain);
  assert.equal(result.rows.length, 10000);
  assert.equal(result.matches.length, 1);
  assert.equal(result.rows.at(-1).depth, 9999);
  assert.equal(tree({}, new Set(['0']), chain).rows.length, 1);
  assert.equal(tree({ under: '9900' }, new Set(['0']), chain).rows.length, 100);
});

// Manually clock the poller and settle individual reads: no race-prone sleeps.
import { createPoller, createDraftGuard } from './assets/live.mjs';
function pollingHarness() {
  const reads = [], data = [], errors = [], timers = new Map();
  let clock = 0, time = 0;
  const poller = createPoller({
    read(args) { return new Promise((resolve, reject) => reads.push({ ...args, resolve, reject })); },
    onData: value => data.push(value), onError: error => errors.push(error),
    schedule(fn, ms) { assert.equal(ms, 750); timers.set(++clock, fn); return clock; },
    cancel: key => timers.delete(key), now: () => time,
  });
  return { poller, reads, data, errors, timers, advance: ms => { time += ms; },
    async tick() { assert.equal(timers.size, 1); const [key, fn] = [...timers][0]; timers.delete(key); time += 750; fn(); },
    async settle(n, value = { data: { n }, etag: `v${n}` }) { reads[n].resolve(value); await Promise.resolve(); },
  };
}
test('polling closes initial-load gaps, coalesces changes and has one pending timer/read', async () => {
  const h = pollingHarness();
  h.poller.refresh();
  assert.equal(h.reads[0].etag, '');
  assert.equal(h.timers.size, 0); // No overlapping interval while a read is slow.
  await h.settle(0);
  await h.tick();
  assert.equal(h.reads[1].etag, 'v0');
  await h.settle(1, { data: { current: 'latest after several changes' }, etag: 'latest' });
  assert.deepEqual(h.data[1], { current: 'latest after several changes' });
  await h.tick();
  await h.settle(2, { data: null, etag: 'latest' });
  assert.equal(h.data[2], null); // Unchanged snapshot leaves DOM alone.
  h.poller.stop();
  assert.equal(h.timers.size, 0);
});
test('invalid state and transport errors force full recovery even for identical repaired bytes', async () => {
  const h = pollingHarness();
  h.poller.refresh();
  await h.settle(0);
  for (const error of [Object.assign(new Error('INVALID_CONFIG'), { status: 503 }), new Error('offline')]) {
    await h.tick();
    h.reads.at(-1).reject(error);
    await Promise.resolve();
    assert.equal(h.errors.at(-1), error);
    await h.tick();
    assert.equal(h.reads.at(-1).etag, '');
    await h.settle(h.reads.length - 1, { data: { repaired: true }, etag: 'v0' });
    assert.deepEqual(h.data.at(-1), { repaired: true });
  }
  h.poller.stop();
});
test('selection/reconnect/resume discards obsolete responses and stops cannot resurrect timers', async () => {
  const h = pollingHarness();
  h.poller.refresh();
  h.poller.refresh();
  assert.equal(h.reads[0].signal.aborted, true);
  await h.settle(1);
  await h.settle(0); // Simulate a transport ignoring abort.
  assert.deepEqual(h.data, [{ n: 1 }]);
  assert.equal(h.timers.size, 1);
  await h.tick();
  h.poller.stop();
  assert.equal(h.reads[2].signal.aborted, true);
  await h.settle(2);
  assert.equal(h.timers.size, 0);
  h.poller.refresh();
  assert.equal(h.reads[3].etag, '');
  await h.settle(3);
  h.poller.stop();
});
test('draft base revision and input ownership survive external changes, filtering, deletion and stale data', () => {
  const guard = createDraftGuard();
  const notifications = [];
  const form = { value: 'unsaved text', revision: 'original' };
  const dispose = guard.register({ id: 'draft', revision: form.revision, isDirty: () => true, onRemote: event => notifications.push(event) });
  assert.throws(() => guard.register({}), /already registered/);
  const latest = indexItems([{ id: 'draft', revision: 'external' }]);
  assert.deepEqual(guard.inspect(latest, new Set()), {
    id: 'draft', baseRevision: 'original', item: latest.items[0], dirty: true, changed: true, missing: false, outsideFilters: true, stale: false,
  });
  assert.equal(guard.inspect(indexItems([]), new Set()).missing, true);
  assert.equal(guard.inspect(latest, new Set(['draft']), true).stale, true);
  assert.deepEqual(form, { value: 'unsaved text', revision: 'original' });
  assert.equal(notifications.length, 3);
  dispose();
  assert.equal(guard.inspect(latest, new Set()), null);
});

test('a late timer after system sleep forces full resynchronization without browser events', async () => {
  const h = pollingHarness();
  h.poller.refresh();
  await h.settle(0);
  h.advance(30000);
  await h.tick();
  assert.equal(h.reads[1].etag, '');
  await h.settle(1);
  h.poller.stop();
});

test('incoming related edits notify drafts without advancing either original revision', () => {
  const guard = createDraftGuard();
  let context;
  guard.register({ id: 'draft', revision: 'source', relatedRevision: 'links-before', isDirty: () => true, onRemote: value => { context = value; } });
  const index = indexItems([{ id: 'draft', revision: 'source', related_revision: 'links-after' }]);
  guard.inspect(index, new Set(['draft']), false);
  assert.equal(context.changed, true);
  assert.equal(context.baseRevision, 'source');
});

import { draftPatch } from './assets/editor.mjs';
import { detailValues, confirmedBaseline, reconcileDetail } from './assets/detail.mjs';
test('confirmed metadata baselines preserve untouched CRLF bodies and never adopt a later poll', () => {
  const initial = { title: 'Original', body: '\r\nExact\r\nbytes', status: 'todo', labels: ['old'], parent: null, depends_on: [], related: [] };
  const base = detailValues(initial);
  const typed = { ...base, title: 'Typed before and during save' };
  const confirmed = confirmedBaseline(base, { status: 'done' }, { ...initial, status: 'done' });
  assert.equal(confirmed.body, '\nExact\nbytes');
  assert.deepEqual(draftPatch(confirmed, { ...typed, status: 'done' }), { title: 'Typed before and during save' });
  assert.deepEqual(draftPatch(base, { ...base, body: '' }), { body: '' });
  const submitted = confirmedBaseline(base, { body: 'Submitted body' }, initial);
  assert.deepEqual(draftPatch(submitted, { ...base, body: 'More typing' }), { body: 'More typing' });
});
test('explicit review merges collection intent without losing external additions or text', () => {
  const base = detailValues({ title: 'Old', body: 'Old body', status: 'todo', parent: null, labels: ['old'], depends_on: ['a'], related: ['b'] });
  const mine = { ...base, title: 'My title', labels: ['mine'], depends_on: [], related: ['b', 'mine'] };
  const reviewed = { ...base, title: 'Agent title', body: 'Agent body', status: 'done', labels: ['old', 'agent'], depends_on: ['a', 'agent'], related: ['b', 'agent'] };
  assert.deepEqual(reconcileDetail(base, mine, reviewed), { ...reviewed, title: 'My title', labels: ['agent', 'mine'], depends_on: ['agent'], related: ['b', 'agent', 'mine'] });
});
test('pending writes defer revision notices until publication is resolved', () => {
  const guard = createDraftGuard();
  let saving = true;
  guard.register({ id: 'draft', revision: 'before', isDirty: () => true, isSaving: () => saving, onRemote() {} });
  const latest = indexItems([{ id: 'draft', revision: 'after' }]);
  assert.equal(guard.inspect(latest, new Set()).changed, false);
  saving = false;
  assert.equal(guard.inspect(latest, new Set()).changed, true);
});
