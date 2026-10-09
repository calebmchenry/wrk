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
