import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readState, stateHash, indexItems, selectItems, progress } from './assets/model.mjs';

const item = (id, status, extra = {}) => ({ id, title: id, status, body: '', labels: [], blockers: [], parent: null, ...extra });
const items = [item('root', 'in-progress', { labels: ['web'] }),
  item('child', 'todo', { parent: 'root', labels: ['web', 'CLI'], body: 'Needle in description' }),
  item('grandchild', 'blocked', { parent: 'child', labels: ['web'] }),
  item('done', 'done', { parent: 'child' }), item('canceled', 'canceled', { parent: 'root' }),
  item('waiting', 'todo', { labels: ['web'], blockers: [{ id: 'canceled', status: 'canceled' }] }),
  item('manual', 'blocked')];
const index = indexItems(items);
const ids = overrides => selectItems(index, { ...readState(''), ...overrides }).map(i => i.id);

test('status, body/title/ID search, exact AND labels and focus intersect', () => {
  assert.deepEqual(ids({}), ['root', 'child', 'grandchild', 'waiting', 'manual']);
  assert.equal(ids({ view: 'all' }).length, 7);
  assert.deepEqual(ids({ view: 'ready' }), ['child']);
  assert.deepEqual(ids({ view: 'blocked' }), ['grandchild', 'manual']);
  assert.deepEqual(ids({ q: 'NEEDLE', labels: ['web', 'CLI'], view: 'ready', focus: 'root' }), ['child']);
  assert.deepEqual(ids({ labels: ['cli'] }), []);
  assert.deepEqual(ids({ q: 'waiting' }), ['waiting']);
  assert.deepEqual(ids({ focus: 'root' }), ['root', 'child', 'grandchild']);
  assert.deepEqual(ids({ focus: 'root', view: 'all' }), ['root', 'child', 'grandchild', 'done', 'canceled']);
  assert.deepEqual(ids({ focus: 'child', view: 'all' }), ['child', 'grandchild', 'done']);
  assert.deepEqual(ids({ focus: 'missing' }), []);
  assert.equal(progress(index.children.get('child')), '1/2 children done');
  assert.equal(progress(index.children.get('root')), '0/2 children done');
});

test('fragment state round-trips labels and text without losing selection or filters', () => {
  const state = { q: 'symbols & # + 🦊', view: 'all', labels: ['CLI', 'two words'], focus: 'root', item: 'done' };
  assert.deepEqual(readState(stateHash(state)), state);
  assert.deepEqual(readState('#view=unknown&label=x&label=x&label=&item=missing'), { ...readState(''), labels: ['x'], item: 'missing' });
});

test('deep hierarchy is iterative and includes the focus root', () => {
  const chain = Array.from({ length: 10000 }, (_, n) => item(String(n), 'todo', { parent: n ? String(n - 1) : null }));
  assert.equal(selectItems(indexItems(chain), { ...readState(''), focus: '0' }).length, 10000);
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
