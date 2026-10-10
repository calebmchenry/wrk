import { test, expect } from '@playwright/test';
import { mkdtemp, mkdir, writeFile, rm, readFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { execFileSync, spawn } from 'node:child_process';
import { createInterface } from 'node:readline';
import { once } from 'node:events';

let root, server, base, binary;
const id = n => `task-${n.toString(16).padStart(8, '0')}`;
const ticketPath = n => join(root, '.wrk', `${id(n)}.md`);
async function ticket(n, title, status, metadata = '', body = '') {
  await writeFile(ticketPath(n), `---\nid: ${id(n)}\ntitle: ${JSON.stringify(title)}\nstatus: ${status}\n${metadata}---\n${body}`);
}

test.beforeAll(async () => {
  root = await mkdtemp(join(tmpdir(), 'wrk-browser-'));
  await mkdir(join(root, '.wrk'));
  await writeFile(join(root, '.wrk/config.yaml'), 'version: 1\nprefix: task\n');
  await ticket(1, 'Build a local workspace', 'in-progress', 'priority: high\nlabels: [web]\nfields: {estimate: 5, opaque: &cycle [*cycle]}\n',
    `## Overview\n\nA **searchable** workspace, with *context* intact.\n\n- [x] Read contracts\n- [ ] Browse work\n\n| Area | State |\n|---|---|\n| Browser | Building |\n\n[Build search](${id(2)}.md) · [External](https://example.com/docs)\n\n![Tracking image](https://example.com/tracker.png)\n\n[Unsafe](javascript&colon;alert%281%29)\n\n<script>window.pwned=true</script>\n<img src="https://example.com/evil" onerror="window.pwned=true">\n`);
  await ticket(2, 'Build search', 'todo', `parent: ${id(1)}\ndepends_on: [${id(3)}]\nlabels: [web, backend]\n`, 'Needle appears only in this description.');
  await ticket(3, 'Confirm contracts', 'done', `parent: ${id(1)}\n`, 'Done prerequisite.');
  await ticket(4, 'Deep descendant', 'blocked', `parent: ${id(2)}\nlabels: [web, backend]\n`);
  await ticket(5, 'Waiting on review', 'todo', `parent: ${id(1)}\ndepends_on: [${id(6)}]\nlabels: [web]\n`);
  await ticket(6, 'Retired experiment', 'canceled', `parent: ${id(1)}\n`);
  await ticket(7, 'Independent manual block', 'blocked', 'labels: [ops]\n');
  await ticket(8, '<img src=x onerror=alert(1)>', 'todo', 'labels: ["<script>bad</script>"]\n');
  for (let n = 9; n <= 40; n++) await ticket(n, `Review integration ${n}`, 'todo', 'labels: [backlog]\n');
  binary = process.env.WRK_BROWSER_BINARY ? resolve(process.env.WRK_BROWSER_BINARY) : join(root, 'wrk');
  if (!process.env.WRK_BROWSER_BINARY) execFileSync('go', ['build', '-o', binary, './cmd/wrk']);
  server = spawn(binary, ['--project', root, 'serve', '--port', '0', '--json'], { cwd: root });
  let stderr = '';
  server.stderr.on('data', data => { stderr += data; });
  base = await new Promise((resolve, reject) => {
    const lines = createInterface({ input: server.stdout });
    const timer = setTimeout(() => reject(new Error(`Server startup timeout: ${stderr}`)), 10000);
    lines.on('line', line => {
      const data = JSON.parse(line);
      if (data.result?.event === 'started') { clearTimeout(timer); resolve(data.result.url); }
      else if (!data.ok) { clearTimeout(timer); reject(new Error(line)); }
    });
    server.once('error', error => { clearTimeout(timer); reject(error); });
    server.once('exit', code => { clearTimeout(timer); reject(new Error(`Server exited ${code}: ${stderr}`)); });
  });
});
test.afterAll(async () => {
  if (server && server.exitCode === null) {
    const exited = once(server, 'exit');
    server.kill('SIGINT');
    await exited;
  }
  if (root) await rm(root, { recursive: true, force: true });
});

async function ready(page, hash = '') {
  await page.goto(base + hash);
  await expect(page.locator('#status')).toContainText('40 items');
}
const listLink = (page, n) => page.locator(`#items a[data-item="${id(n)}"]`);
async function expectTitle(page, title, options) {
  if (title === 'Item not found') await expect(page.locator('#detail .detail-title')).toHaveText(title, options);
  else await expect(page.locator('#detail-title')).toHaveValue(title, options);
}

async function chooseUnder(page, n) {
  await page.getByRole('button', { name: 'Under parent', exact: true }).click();
  await page.getByRole('combobox', { name: 'Under parent', exact: true }).fill(id(n));
  await page.keyboard.press('Enter');
}
const toggleRow = (page, n) => page.locator(`[data-toggle="${id(n)}"]`);

test('views, exact tags, fuzzy title search and Under intersect with honest context counts', async ({ page }) => {
  await ready(page);
  await expect(page.locator('#project')).toHaveText(root);
  await expect(listLink(page, 3)).toHaveCount(0);
  await expect(listLink(page, 6)).toHaveCount(0);
  await expect(listLink(page, 5)).toHaveAccessibleDescription(new RegExp(`Waiting on ${id(6)}`));
  await page.getByRole('button', { name: 'Ready', exact: true }).click();
  await expect(listLink(page, 5)).toHaveCount(0);
  await expect(listLink(page, 4)).toHaveCount(0);
  await chooseUnder(page, 1);
  await page.locator('#label-filter summary').click();
  await page.getByLabel('web', { exact: true }).check();
  await page.getByLabel('backend', { exact: true }).check();
  await page.getByRole('searchbox').fill('NEEDLE');
  await expect(page.locator('#items > li')).toHaveCount(0); // Description-only match.
  await page.getByRole('searchbox').fill(id(2));
  await expect(page.locator('#items > li')).toHaveCount(0); // IDs are picker-only.
  await page.getByRole('searchbox').fill('BSRCH');
  await expect(page.locator('#items > li')).toHaveCount(2);
  await expect(page.locator('#results')).toHaveText('1 of 40 items match');
  await expect(page.locator(`[data-row="${id(1)}"]`)).toHaveAttribute('data-context', 'true');
  await expect(listLink(page, 2)).toBeVisible();
  await expect(page.getByRole('button', { name: 'Remove tag filter web', exact: true })).toBeVisible();
  await page.getByRole('searchbox').fill('');
  await page.getByRole('button', { name: 'Blocked', exact: true }).click();
  await expect(page.locator('#items > li')).toHaveCount(3);
  await expect(page.locator('#results')).toHaveText('1 of 40 items match');
  await expect(listLink(page, 4)).toBeVisible();
  await page.locator('#reset').click();
  await page.getByRole('button', { name: 'All', exact: true }).click();
  await chooseUnder(page, 1);
  await expect(page.locator('#items > li')).toHaveCount(6);
  await expect(page.locator('#results')).toHaveText('5 of 40 items match');
  await expect(listLink(page, 1)).toHaveAccessibleDescription(/1\/4 children done/);
  await expect(listLink(page, 6)).toHaveAccessibleDescription(/Canceled/);
});

test('relationships, Markdown links and browser history retain filters and scroll', async ({ page }) => {
  await ready(page, '#view=all&label=web');
  await page.getByRole('link', { name: 'Skip to workspace' }).focus();
  await page.keyboard.press('Enter');
  await expect(page.locator('#list-scroll')).toBeFocused();
  await expect(page).toHaveURL(/label=web/);
  await listLink(page, 1).click();
  await expectTitle(page, 'Build a local workspace');
  await page.getByRole('button', { name: 'Show descendants' }).click();
  await expect(page.getByRole('button', { name: 'Under parent', exact: true })).toContainText(id(1));
  await page.locator('.markdown').getByRole('link', { name: 'Build search' }).click();
  await expectTitle(page, 'Build search');
  await expect(page).toHaveURL(/label=web/);
  await expect(page.locator('#detail')).toContainText('0/1 children done');
  await page.locator('#detail').getByRole('link', { name: 'Deep descendant' }).click();
  await expectTitle(page, 'Deep descendant');
  await page.locator('#detail').getByRole('link', { name: 'Build search' }).click();
  await expectTitle(page, 'Build search');
  const dependency = page.locator('#detail section').filter({ has: page.getByRole('heading', { name: 'Dependencies', exact: true }) });
  await dependency.getByRole('link', { name: 'Confirm contracts' }).click();
  await expectTitle(page, 'Confirm contracts');
  await expect(page.locator('#detail')).toContainText('outside the current list filters');
  await expect(page.locator('#detail')).toContainText('Build search');
  await page.goBack();
  await expectTitle(page, 'Build search');
  await page.goForward();
  await expectTitle(page, 'Confirm contracts');
  await page.reload();
  await expectTitle(page, 'Confirm contracts');
  await page.locator('#reset').click();
  await listLink(page, 30).click();
  await expectTitle(page, 'Review integration 30');
  const scroll = await page.locator('#list-scroll').evaluate(node => node.scrollTop);
  expect(scroll).toBeGreaterThan(0);
  await page.getByRole('button', { name: /Close|Back to table/ }).click();
  await expect.poll(() => page.locator('#list-scroll').evaluate(node => node.scrollTop)).toBe(scroll);
  await page.goBack();
  await expectTitle(page, 'Review integration 30');
  await expect.poll(() => page.locator('#list-scroll').evaluate(node => node.scrollTop)).toBe(scroll);
});

test('untrusted content stays inert, images never load, custom YAML is preserved', async ({ page }, testInfo) => {
  const external = [], errors = [];
  page.on('request', request => { if (!request.url().startsWith(base)) external.push(request.url()); });
  page.on('pageerror', error => errors.push(error.message));
  await ready(page, `#item=${id(1)}`);
  await expectTitle(page, 'Build a local workspace');
  await expect(page.locator('.markdown strong')).toHaveText('searchable');
  await expect(page.locator('.markdown table')).toBeVisible();
  await expect(page.locator('.markdown input[type=checkbox]').first()).toBeDisabled();
  await expect(page.locator('.markdown img, .markdown script, .markdown iframe')).toHaveCount(0);
  await expect(page.locator('.markdown').getByRole('link', { name: 'Unsafe' })).toHaveCount(0);
  await expect(page.locator('.markdown').getByRole('link', { name: 'External' })).toHaveAttribute('rel', 'noopener noreferrer');
  await expect(page.locator('.image-placeholder')).toContainText('Tracking image');
  await page.getByText('Custom fields & original metadata (YAML)', { exact: true }).click();
  await expect(page.locator('#detail pre').first()).toContainText('opaque: &cycle [*cycle]');
  expect(await page.evaluate(() => window.pwned)).toBeUndefined();
  expect(external).toEqual([]);
  expect(errors).toEqual([]);
  await page.locator('#detail').evaluate(node => { node.scrollTop = 0; });
  await page.screenshot({ path: testInfo.outputPath('desktop.png'), fullPage: true });
  await listLink(page, 8).click();
  await expectTitle(page, '<img src=x onerror=alert(1)>');
  await expect(page.locator('#detail img')).toHaveCount(0);
});

test('missing selection and missing focus recover without resetting context', async ({ page }) => {
  await ready(page, '#q=Build&item=task-deadbeef');
  await expectTitle(page, 'Item not found');
  await expect(page.getByRole('searchbox')).toHaveValue('Build');
  await listLink(page, 1).click();
  await expectTitle(page, 'Build a local workspace');
  await page.goto(base + '#focus=task-deadbeef');
  await expect(page.locator('#empty')).toContainText('Under item not found');
  await page.locator('#reset').click();
  await expect(listLink(page, 1)).toBeVisible();
  await page.getByRole('searchbox').fill('does not exist');
  await expect(page.locator('#empty')).toContainText('No matching items');
});

test('loading, validation failures, recovery, and an empty project are explicit', async ({ page }) => {
  let release;
  await page.route('**/api/workspace', async route => { await new Promise(resolve => { release = resolve; }); await route.continue(); });
  await page.goto(base);
  await expect(page.locator('#status')).toHaveText('Loading project…');
  await page.getByLabel('Project information and actions').click();
  await expect(page.getByRole('button', { name: 'Reload project' })).toBeDisabled();
  await expect.poll(() => Boolean(release)).toBe(true);
  release();
  await expect(page.locator('#status')).toContainText('40 items');
  await page.unroute('**/api/workspace');
  const original = await readFile(ticketPath(8));
  try {
    await writeFile(ticketPath(8), 'broken');
    await expect(page.locator('#diagnostics')).toContainText('INVALID_TICKET', { timeout: 2000 });
    await expect(page.locator('#status')).toContainText('stale last-valid data');
    await expect(page.locator('#workspace')).toBeVisible();
  } finally { await writeFile(ticketPath(8), original); }
  await expect(page.locator('#problem')).toBeHidden({ timeout: 2000 });
  await expect(page.locator('#status')).toContainText('Live');
  await page.route('**/api/workspace', async route => {
    const response = await route.fetch();
    const data = await response.json();
    data.result.tickets = [];
    data.result.project.ticket_count = 0;
    await route.fulfill({ json: data });
  });
  if (!await page.getByRole('button', { name: 'Reload project' }).isVisible()) await page.getByLabel('Project information and actions').click();
  await page.getByRole('button', { name: 'Reload project' }).click();
  await expect(page.locator('#empty')).toContainText('No items yet');
});

test('narrow layout and keyboard navigation keep the list usable', async ({ page }, testInfo) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await ready(page, '#q=Build');
  await listLink(page, 2).focus();
  await expect(listLink(page, 2)).toBeFocused();
  expect(await listLink(page, 2).evaluate(node => getComputedStyle(node).outlineStyle)).toBe('solid');
  await page.keyboard.press('Enter');
  await expectTitle(page, 'Build search');
  await expect(page.locator('#browse')).toBeHidden();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({ path: testInfo.outputPath('narrow.png'), fullPage: true });
  await page.getByRole('button', { name: /Close|Back to table/ }).click();
  await expect(page.getByRole('searchbox')).toHaveValue('Build');
  await expect(listLink(page, 2)).toBeFocused();
  await page.goBack();
  await expectTitle(page, 'Build search');
});

test('a stale detail response cannot overwrite the new selection', async ({ page }) => {
  await ready(page);
  let release;
  await page.route(`**/api/workspace?selected=${id(1)}`, async route => { await new Promise(resolve => { release = resolve; }); await route.continue(); });
  await listLink(page, 1).click();
  await expect(page.locator('#detail')).toContainText('Loading item…');
  await expect.poll(() => Boolean(release)).toBe(true);
  await listLink(page, 2).click();
  await expectTitle(page, 'Build search');
  release();
  await expectTitle(page, 'Build search');
  await expect(page.locator('#status')).toContainText('Live');
});


const cli = (...args) => JSON.parse(execFileSync(binary, ['--project', root, ...args, '--json'], { encoding: 'utf8' })).result;
const currentDetail = page => page.locator('#detail');

test('separate CLI create/status/labels/parent/dependencies/body changes refresh within two seconds', async ({ page }) => {
  await ready(page, '#q=Live');
  let created;
  try {
    created = cli('new', 'Live agent item', '--label', 'live').ticket.id;
    const link = page.locator(`#items a[data-item="${created}"]`);
    await expect(link).toBeVisible({ timeout: 2000 });
    await link.click();
    await expectTitle(page, 'Live agent item');
    for (const status of ['in-progress', 'done']) {
      cli('update', created, '--status', status);
      await expect(editForm(page).getByLabel('Status', { exact: true })).toHaveValue(status, { timeout: 2000 });
    }
    await expect(link).toHaveCount(0);
    await expect(currentDetail(page)).toContainText('outside the current list filters');
    await expect(page).toHaveURL(new RegExp(`item=${created}`));
    await expect(page.getByRole('searchbox')).toHaveValue('Live');
    cli('update', created, '--status', 'todo', '--add-label', 'reviewed', '--parent', id(1), '--add-dependency', id(6));
    await expect(link).toBeVisible({ timeout: 2000 });
    await expect(editForm(page).getByRole('button', { name: 'Remove tag reviewed', exact: true })).toBeVisible();
    await expect(currentDetail(page)).toContainText(`Unfinished dependencies: ${id(6)} (canceled)`);
    await expect(currentDetail(page).getByRole('link', { name: 'Build a local workspace' })).toBeVisible();
    // Body update through another CLI process, then an external direct body edit.
    const bodyPath = join(root, 'body.txt');
    await writeFile(bodyPath, '## Live CLI description');
    cli('update', created, '--body-file', bodyPath);
    await expect(page.locator('.markdown h2')).toHaveText('Live CLI description', { timeout: 2000 });
    const path = join(root, '.wrk', `${created}.md`);
    await writeFile(path, (await readFile(path, 'utf8')).replace('Live CLI description', 'Live direct description'));
    await expect(page.locator('.markdown h2')).toHaveText('Live direct description', { timeout: 2000 });
    cli('update', created, '--remove-dependency', id(6), '--no-parent');
    await expect(currentDetail(page)).not.toContainText('Unfinished dependencies:', { timeout: 2000 });
    await expect(currentDetail(page).getByRole('link', { name: 'Build a local workspace' })).toHaveCount(0);
    await rm(path);
    await expectTitle(page, 'Item not found', { timeout: 2000 });
    await expect(link).toHaveCount(0);
    await expect(page.getByRole('searchbox')).toHaveValue('Live');
  } finally {
    if (created) await rm(join(root, '.wrk', `${created}.md`), { force: true });
  }
});

test('live relationships/progress/config update while pane context and draft input survive', async ({ page }) => {
  const original = await readFile(ticketPath(2));
  const configPath = join(root, '.wrk/config.yaml');
  const config = await readFile(configPath);
  await ready(page, `#view=all&item=${id(1)}`);
  await expectTitle(page, 'Build a local workspace');
  await page.getByText('Custom fields & original metadata (YAML)', { exact: true }).click();
  const revision = await page.evaluate(async selected => {
    const { drafts } = await import('/live.mjs');
    const response = await (await fetch(`/api/items/${selected}`)).json();
    const revision = response.result.ticket.revision;
    const input = document.createElement('textarea');
    input.setAttribute('aria-label', 'Draft body');
    input.value = 'My unsaved work';
    document.querySelector('#detail').append(input);
    window.disposeDraft = drafts.register({ id: selected, revision, isDirty: () => true, onRemote: data => { window.draftContext = data; } });
    return revision;
  }, id(1));
  await page.locator('#list-scroll').evaluate(node => { node.scrollTop = 900; });
  await page.locator('#detail').evaluate(node => { node.scrollTop = 120; });
  const scroll = await page.evaluate(() => [document.querySelector('#list-scroll').scrollTop, document.querySelector('#detail').scrollTop]);
  try {
    cli('update', id(2), '--status', 'done');
    await expect(currentDetail(page)).toContainText('2/4 children done', { timeout: 2000 });
    await expect(page.getByLabel('Draft body')).toHaveValue('My unsaved work');
    await expect(page.locator('#detail details').first()).toHaveAttribute('open', '');
    expect(await page.evaluate(() => [document.querySelector('#list-scroll').scrollTop, document.querySelector('#detail').scrollTop])).toEqual(scroll);
    await writeFile(configPath, 'version: 1\nprefix: changed\n');
    await expect(page.locator('#status')).toContainText('changed', { timeout: 2000 });
    await writeFile(configPath, 'version: [');
    await expect(page.locator('#status')).toContainText('stale', { timeout: 2000 });
    await expect(page.getByLabel('Draft body')).toHaveValue('My unsaved work');
    await writeFile(configPath, config);
    await expect(page.locator('#status')).toContainText('Live', { timeout: 2000 });
    expect(await page.evaluate(() => window.draftContext.baseRevision)).toBe(revision);
  } finally {
    await writeFile(ticketPath(2), original);
    await writeFile(configPath, config);
  }
});

test('dirty selected draft is retained on external revision, filtering and deletion', async ({ page }) => {
  const original = await readFile(ticketPath(40));
  await ready(page, `#q=Review&item=${id(40)}`);
  await page.evaluate(async selected => {
    const { drafts } = await import('/live.mjs');
    const data = await (await fetch(`/api/items/${selected}`)).json();
    const input = document.createElement('input');
    input.setAttribute('aria-label', 'Unsaved title');
    input.value = 'Keep my title';
    document.querySelector('#detail').append(input);
    drafts.register({ id: selected, revision: data.result.ticket.revision, isDirty: () => true, onRemote: data => { window.draftContext = data; } });
  }, id(40));
  try {
    cli('update', id(40), '--title', 'Changed externally');
    await expect(page.locator('#draft-notice')).toContainText('changed externally', { timeout: 2000 });
    await expect(page.locator('#draft-notice')).toContainText('outside the current list filters');
    await expectTitle(page, 'Changed externally');
    await expect(page.getByLabel('Unsaved title')).toHaveValue('Keep my title');
    await rm(ticketPath(40));
    await expectTitle(page, 'Item not found', { timeout: 2000 });
    await expect(page.locator('#draft-notice')).toContainText('removed');
    await expect(page.getByLabel('Unsaved title')).toHaveValue('Keep my title');
  } finally { await writeFile(ticketPath(40), original); }
});

test('offline, dropped responses and resume signals resynchronize a full snapshot', async ({ page, context }) => {
  const original = await readFile(ticketPath(40));
  await ready(page, `#item=${id(40)}`);
  await expectTitle(page, 'Review integration 40');
  const headers = [];
  page.on('request', request => { if (request.url().includes('/api/workspace')) headers.push(request.headers()); });
  try {
    await context.setOffline(true);
    await expect(page.locator('#status')).toContainText('Reconnecting', { timeout: 2000 });
    cli('update', id(40), '--title', 'Changed while offline');
    headers.length = 0;
    await context.setOffline(false);
    await expectTitle(page, 'Changed while offline', { timeout: 2000 });
    await expect(page.locator('#status')).toContainText('Live');
    expect(headers[0]['if-none-match']).toBeUndefined();
    let dropped = false;
    await page.route('**/api/workspace?*', route => {
      if (!dropped) { dropped = true; return route.abort('failed'); }
      return route.continue();
    });
    await expect(page.locator('#status')).toContainText('Reconnecting', { timeout: 2000 });
    cli('update', id(40), '--title', 'After dropped response');
    await expectTitle(page, 'After dropped response', { timeout: 2000 });
    await page.unroute('**/api/workspace?*');
    // BFCache/page suspension: stopped timers cannot miss edits after resume.
    await page.evaluate(() => window.dispatchEvent(new PageTransitionEvent('pagehide')));
    cli('update', id(40), '--title', 'Changed while asleep');
    headers.length = 0;
    await page.evaluate(() => window.dispatchEvent(new PageTransitionEvent('pageshow', { persisted: true })));
    await expectTitle(page, 'Changed while asleep', { timeout: 2000 });
    expect(headers[0]['if-none-match']).toBeUndefined();
  } finally {
    await context.setOffline(false);
    await writeFile(ticketPath(40), original);
  }
});

test('changes during initial load are detected on the next poll; runtime churn leaves DOM intact', async ({ page }) => {
  const original = await readFile(ticketPath(40));
  let release, captured;
  // Hold a completed initial snapshot, then mutate before the browser receives it.
  await page.route('**/api/workspace?*', async route => {
    if (captured) return route.continue();
    captured = true;
    const response = await route.fetch();
    await new Promise(resolve => { release = resolve; });
    await route.fulfill({ response });
  });
  try {
    await page.goto(base + `#item=${id(40)}`);
    await expect.poll(() => Boolean(release)).toBe(true);
    cli('update', id(40), '--title', 'Changed during initial load');
    release();
    await expectTitle(page, 'Changed during initial load', { timeout: 2000 });
    await page.unroute('**/api/workspace?*');
    await page.evaluate(() => { window.savedDetail = document.querySelector('.detail-title'); });
    await writeFile(join(root, '.wrk/.wrk-stage-ignore.md'), 'invalid staging contents');
    await writeFile(join(root, '.wrk/runtime.log'), 'runtime contents');
    await page.waitForResponse(response => response.url().includes('/api/workspace') && response.status() === 304);
    expect(await page.evaluate(() => window.savedDetail === document.querySelector('.detail-title'))).toBe(true);
  } finally { release?.(); await writeFile(ticketPath(40), original); }
});

const editForm = page => page.getByRole('form', { name: 'Edit item', exact: true });
const createForm = page => page.getByRole('form', { name: 'Create item', exact: true });
async function saveEdit(page) {
  await editForm(page).getByRole('button', { name: 'Save changes' }).click();
  await expect(editForm(page).getByRole('button', { name: 'Save changes' })).toBeHidden();
  await expect(page.locator('#save-notice')).toContainText('Saved');
}
async function cancelDraft(page) {
  page.once('dialog', dialog => dialog.accept());
  await page.getByRole('button', { name: /^(Cancel|Close new item|Discard pending changes)$/ }).filter({ visible: true }).click();
  await expect(page.locator('#create-dialog form')).toHaveCount(0);
  await expect(page.locator('#draft-notice')).toBeHidden();
}

async function addRelationship(page, kind, value) {
  const label = kind === 'related' ? 'Add related item' : 'Add dependency';
  await editForm(page).getByRole('button', { name: label, exact: true }).click();
  const input = editForm(page).getByRole('combobox', { name: label, exact: true });
  await input.fill(value);
  const response = page.waitForResponse(response => response.request().method() === 'PATCH');
  await input.press('Enter');
  await response;
  await expect(editForm(page).getByLabel('Status', { exact: true })).toBeEnabled();
}
async function changeStatus(page, value) {
  const response = page.waitForResponse(response => response.request().method() === 'PATCH');
  await editForm(page).getByLabel('Status', { exact: true }).selectOption(value);
  await response;
  await expect(editForm(page).getByLabel('Status', { exact: true })).toBeEnabled();
}
async function addTag(page, value) {
  await editForm(page).getByRole('button', { name: 'Add tag', exact: true }).click();
  const input = editForm(page).getByRole('combobox', { name: 'Tag to add' });
  await input.fill(value);
  const response = page.waitForResponse(response => response.request().method() === 'PATCH');
  await input.press('Enter');
  await response;
  await expect(editForm(page).getByLabel('Status', { exact: true })).toBeEnabled();
}
async function changeParent(page, value) {
  await editForm(page).getByRole('button', { name: 'Parent item', exact: true }).click();
  const input = editForm(page).getByRole('combobox', { name: 'Find parent item' });
  await input.fill(value);
  const response = page.waitForResponse(response => response.request().method() === 'PATCH');
  await input.press('Enter');
  await response;
  await expect(editForm(page).getByLabel('Status', { exact: true })).toBeEnabled();
}

test('browser creation honors defaults and optional fields; CLI reads saved bytes', async ({ page }) => {
  const configPath = join(root, '.wrk/config.yaml');
  const config = await readFile(configPath);
  const created = [];
  try {
    await writeFile(configPath, 'version: 1\nprefix: task\ndefaults: {priority: urgent, labels: [default]}\n');
    await ready(page);
    await page.getByRole('button', { name: 'New item', exact: true }).click();
    await createForm(page).getByLabel('Title', { exact: true }).fill('Created with just a title');
    await createForm(page).getByRole('button', { name: 'Create', exact: true }).click();
    await expect(createForm(page)).toHaveCount(0);
    await expectTitle(page, 'Created with just a title');
    created.push(new URL(page.url()).hash.match(/item=([^&]+)/)[1]);
    let saved = cli('show', created[0]);
    expect(saved.ticket).toMatchObject({ status: 'todo', priority: 'urgent', labels: ['default'], parent: null, depends_on: [] });
    await page.getByRole('button', { name: 'New item', exact: true }).click();
    const form = createForm(page);
    await form.getByLabel('Title', { exact: true }).fill('Created with all optional fields');
    await form.getByLabel('Description (Markdown)', { exact: true }).fill('## Browser body 🦊\nNo final newline');
    await form.getByRole('button', { name: 'Remove tag default', exact: true }).click();
    await form.getByRole('button', { name: 'Parent item', exact: true }).click();
    await form.getByRole('combobox', { name: 'Find parent item' }).fill('Retired');
    await form.getByRole('option', { name: `Retired experiment ${id(6)}` }).click();
    await form.getByRole('button', { name: 'Add tag', exact: true }).click();
    await form.getByRole('combobox', { name: 'Tag to add' }).fill('label with spaces');
    // Pending collection input is included on explicit Save, without needing Add.
    await form.getByRole('button', { name: 'Create', exact: true }).click();
    await expectTitle(page, 'Created with all optional fields');
    created.push(new URL(page.url()).hash.match(/item=([^&]+)/)[1]);
    saved = cli('show', created[1]);
    expect(saved.ticket).toMatchObject({ priority: 'urgent', labels: ['label with spaces'], parent: id(6), depends_on: [] });
    expect(saved.source.endsWith('## Browser body 🦊\nNo final newline')).toBe(true);
    expect(saved.ticket.blockers).toEqual([]);
  } finally {
    for (const item of created) await rm(join(root, '.wrk', `${item}.md`), { force: true });
    await writeFile(configPath, config);
  }
});

test('accessible edit fields, all statuses, quick reopening, clearing and shared cycle errors', async ({ page }) => {
  const original = await readFile(ticketPath(1));
  try {
    await ready(page, `#view=all&item=${id(1)}`);
    await editForm(page).getByLabel('Title', { exact: true }).focus();
    await expect(editForm(page).getByLabel('Title', { exact: true })).toBeFocused();
    const revision = cli('show', id(1)).ticket.revision;
    await changeParent(page, id(2));
    await expect(page.locator('.edit-message')).toContainText('CYCLE [parent]');
    await expect(editForm(page).getByRole('button', { name: 'Parent item', exact: true })).toHaveAttribute('aria-invalid', 'true');
    expect(cli('show', id(1)).ticket.revision).toBe(revision);
    await editForm(page).getByRole('button', { name: 'Remove parent', exact: true }).click();
    await expect(editForm(page).getByLabel('Status', { exact: true })).toBeEnabled();
    await editForm(page).getByLabel('Title', { exact: true }).fill('Edited in browser');
    await editForm(page).getByRole('button', { name: 'Edit description', exact: true }).click();
    await editForm(page).getByLabel('Description (Markdown)', { exact: true }).fill('');
    await editForm(page).getByRole('button', { name: 'Remove tag web', exact: true }).click();
    await changeStatus(page, 'canceled');
    await saveEdit(page);
    let saved = cli('show', id(1));
    expect(saved.ticket).toMatchObject({ title: 'Edited in browser', priority: 'high', labels: [], status: 'canceled' });
    expect(saved.source).toContain('opaque: &');
    expect(saved.source).toContain('estimate: 5');
    expect(saved.source.endsWith('---\n')).toBe(true);
    await changeStatus(page, 'todo');
    for (const status of ['blocked', 'in-progress', 'done', 'todo']) {
      await editForm(page).getByLabel('Title', { exact: true }).focus();
      await changeStatus(page, status);
      expect(cli('show', id(1)).ticket.status).toBe(status);
    }
    await changeStatus(page, 'done');
    expect(cli('show', id(1)).ticket.status).toBe('done');
  } finally { await writeFile(ticketPath(1), original); }
});

test('agent edits reject the original browser revision and preserve input until explicit review', async ({ page }) => {
  const original = await readFile(ticketPath(40));
  const requests = [];
  page.on('request', request => { if (request.method() === 'PATCH') requests.push(request.postDataJSON()); });
  try {
    await ready(page, `#item=${id(40)}`);
    await editForm(page).getByLabel('Title', { exact: true }).focus();
    const baseRevision = cli('show', id(40)).ticket.revision;
    await editForm(page).getByLabel('Title', { exact: true }).fill('My retained title');
    await editForm(page).getByRole('button', { name: 'Edit description', exact: true }).click();
    await editForm(page).getByLabel('Description (Markdown)', { exact: true }).fill('My unsaved description');
    await addTag(page, 'browser-label');
    cli('update', id(40), '--title', 'Agent title', '--add-label', 'agent-label');
    await expect(page.locator('#draft-notice')).toContainText('changed externally', { timeout: 2000 });
    await expect(editForm(page).getByLabel('Title', { exact: true })).toHaveValue('My retained title');
    await editForm(page).getByRole('button', { name: 'Save changes' }).click();
    await expect(page.locator('.edit-message')).toContainText('CONFLICT');
    expect(requests).toHaveLength(2);
    expect(requests[0].expected_revision).toBe(baseRevision);
    expect(requests[1].expected_revision).not.toBe(baseRevision);
    expect(cli('show', id(40)).ticket.title).toBe('Agent title');
    await expect(editForm(page).getByLabel('Description (Markdown)', { exact: true })).toHaveValue('My unsaved description');
    await editForm(page).getByRole('button', { name: 'Review current version', exact: true }).click();
    await expect(page.locator('.edit-review')).toContainText('Agent title');
    await expect(page.locator('.edit-review')).toContainText('My retained title');
    await editForm(page).getByRole('button', { name: 'Keep draft using reviewed revision', exact: true }).click();
    await expect(editForm(page).getByRole('button', { name: 'Remove tag agent-label', exact: true })).toBeVisible();
    await saveEdit(page);
    expect(requests).toHaveLength(3);
    expect(requests[2].expected_revision).not.toBe(requests[1].expected_revision);
    const saved = cli('show', id(40));
    expect(saved.ticket).toMatchObject({ title: 'My retained title', labels: ['agent-label', 'backlog', 'browser-label'] });
    expect(saved.source.endsWith('My unsaved description')).toBe(true);
  } finally { await writeFile(ticketPath(40), original); }
});

test('draft survives navigation and deletion; reload warns and cancel/reopen uses saved values', async ({ page }) => {
  const original = await readFile(ticketPath(40));
  try {
    await ready(page, `#item=${id(40)}`);
    await editForm(page).getByLabel('Title', { exact: true }).focus();
    await editForm(page).getByLabel('Title', { exact: true }).fill('Keep through navigation');
    await listLink(page, 2).click();
    await expect(page.locator('#draft-notice')).toContainText('preserved while browsing');
    await expect(editForm(page).getByLabel('Title', { exact: true })).toHaveValue('Keep through navigation');
    await page.goBack();
    await expect(editForm(page).getByLabel('Title', { exact: true })).toHaveValue('Keep through navigation');
    // A full-page departure is warned even though internal navigation keeps DOM.
    const beforeUnload = await page.evaluate(() => {
      const event = new Event('beforeunload', { cancelable: true });
      window.dispatchEvent(event); return event.defaultPrevented;
    });
    expect(beforeUnload).toBe(true);
    await rm(ticketPath(40));
    await expect(page.locator('#draft-notice')).toContainText('removed', { timeout: 2000 });
    await editForm(page).getByRole('button', { name: 'Save changes' }).click();
    await expect(page.locator('.edit-message')).toContainText('NOT_FOUND');
    await expect(editForm(page).getByLabel('Title', { exact: true })).toHaveValue('Keep through navigation');
    await cancelDraft(page);
    await writeFile(ticketPath(40), original);
    await expectTitle(page, 'Review integration 40', { timeout: 2000 });
    await editForm(page).getByLabel('Title', { exact: true }).focus();
    await expect(editForm(page).getByLabel('Title', { exact: true })).toHaveValue('Review integration 40');
    await expect(page.getByRole('button', { name: 'Cancel', exact: true })).toBeHidden();
  } finally { await writeFile(ticketPath(40), original); }
});

test('BUSY, committed errors and lost responses never silently retry a save', async ({ page }) => {
  const original = await readFile(ticketPath(40));
  let requests = 0;
  try {
    await ready(page, `#item=${id(40)}`);
    await editForm(page).getByLabel('Title', { exact: true }).focus();
    await editForm(page).getByLabel('Title', { exact: true }).fill('Save with uncertain response');
    await page.route('**/api/items/*', async route => {
      if (route.request().method() !== 'PATCH') return route.continue();
      requests++;
      await route.fulfill({ status: 503, json: { ok: false, result: null, errors: [{ code: 'BUSY', message: 'Writer lock held' }] } });
    });
    await editForm(page).getByRole('button', { name: 'Save changes' }).click();
    await expect(page.locator('.edit-message')).toContainText('BUSY');
    await expect(editForm(page).getByRole('button', { name: 'Save changes' })).toBeEnabled();
    expect(requests).toBe(1);
    expect(cli('show', id(40)).ticket.title).toBe('Review integration 40');
    await page.unroute('**/api/items/*');
    await page.route('**/api/items/*', async route => {
      if (route.request().method() !== 'PATCH') return route.continue();
      requests++;
      const response = await route.fetch();
      const data = await response.json();
      data.ok = false;
      data.errors = [{ code: 'DURABILITY_UNCERTAIN', message: 'Injected directory sync failure after publication' }];
      await route.fulfill({ status: 500, json: data });
    });
    await editForm(page).getByRole('button', { name: 'Save changes' }).click();
    await expect(page.locator('.edit-message')).toContainText('Published');
    await expect(page.locator('.edit-message')).toContainText('DURABILITY_UNCERTAIN');
    await expect(editForm(page).getByRole('button', { name: 'Save changes' })).toBeDisabled();
    expect(requests).toBe(2);
    expect(cli('show', id(40)).ticket.title).toBe('Save with uncertain response');
    await editForm(page).getByRole('button', { name: 'Review current version', exact: true }).click();
    page.once('dialog', dialog => dialog.accept());
    await editForm(page).getByRole('button', { name: 'Reload saved values', exact: true }).click();
    await expect(editForm(page).getByRole('button', { name: 'Save changes' })).toBeHidden();
    await page.unroute('**/api/items/*');
    await page.route('**/api/items/*', async route => {
      if (route.request().method() !== 'PATCH') return route.continue();
      requests++; await route.fetch(); await route.abort('failed');
    });
    await editForm(page).getByLabel('Title', { exact: true }).fill('Published with a dropped response');
    await editForm(page).getByRole('button', { name: 'Save changes' }).click();
    await expect(page.locator('.edit-message')).toContainText('Save outcome unknown');
    await expect(editForm(page).getByRole('button', { name: 'Save changes' })).toBeDisabled();
    await expect(editForm(page).getByLabel('Title', { exact: true })).toHaveValue('Published with a dropped response');
    expect(cli('show', id(40)).ticket.title).toBe('Published with a dropped response');
    expect(requests).toBe(3);
    await cancelDraft(page);
  } finally { await writeFile(ticketPath(40), original); }
});

test('a real external writer lock preserves the browser draft until manual retry', async ({ page }) => {
  const original = await readFile(ticketPath(40));
  let writer;
  try {
    await ready(page, `#item=${id(40)}`);
    await editForm(page).getByLabel('Title', { exact: true }).focus();
    await editForm(page).getByLabel('Title', { exact: true }).fill('Saved after writer finished');
    writer = spawn('python3', ['-u', '-c',
      'import fcntl, sys\nwith open(sys.argv[1], "a") as lock:\n fcntl.flock(lock, fcntl.LOCK_EX)\n print("locked", flush=True)\n sys.stdin.read()\n',
      join(root, '.wrk/.lock')]);
    const acquired = await Promise.race([
      once(writer.stdout, 'data').then(([data]) => data.toString()),
      once(writer, 'exit').then(([code]) => { throw new Error(`Lock holder exited early: ${code}`); }),
    ]);
    expect(acquired.trim()).toBe('locked');
    await editForm(page).getByRole('button', { name: 'Save changes' }).click();
    await expect(page.locator('.edit-message')).toContainText('BUSY');
    await expect(editForm(page).getByLabel('Title', { exact: true })).toHaveValue('Saved after writer finished');
    expect(await readFile(ticketPath(40))).toEqual(original);
    expect(cli('show', id(40)).ticket.title).toBe('Review integration 40');
    const exited = once(writer, 'exit');
    writer.stdin.end();
    expect((await exited)[0]).toBe(0);
    await editForm(page).getByRole('button', { name: 'Save changes' }).click();
    await expect(editForm(page).getByRole('button', { name: 'Save changes' })).toBeHidden();
    expect(cli('show', id(40)).ticket.title).toBe('Saved after writer finished');
  } finally {
    if (writer && writer.exitCode === null) {
      const exited = once(writer, 'exit');
      writer.kill('SIGKILL');
      await exited;
    }
    await writeFile(ticketPath(40), original);
  }
});

test('narrow create/edit forms fit and keyboard actions remain accessible', async ({ page }, testInfo) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await ready(page, `#item=${id(2)}`);
  await editForm(page).getByLabel('Title', { exact: true }).focus();
  await expect(editForm(page).getByLabel('Title', { exact: true })).toBeFocused();
  await editForm(page).getByLabel('Title', { exact: true }).fill('Draft on a narrow screen');
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({ path: testInfo.outputPath('narrow-edit.png'), fullPage: true });
  await addRelationship(page, 'related', id(6));
  await expect(editForm(page).getByRole('button', { name: `Remove related ${id(6)}`, exact: true })).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await editForm(page).getByRole('button', { name: 'Save changes', exact: true }).scrollIntoViewIfNeeded();
  await page.screenshot({ path: testInfo.outputPath('narrow-related.png'), fullPage: true });
  await cancelDraft(page);
  cli('update', id(2), '--remove-related', id(6));
  await page.getByRole('button', { name: 'New item', exact: true }).click();
  await expect(createForm(page)).toBeVisible();
  await expect(page.getByRole('dialog', { name: 'New item' })).toBeVisible();
  await createForm(page).getByLabel('Title', { exact: true }).fill('Discarded new item');
  await cancelDraft(page);
  expect(cli('list', '--all').tickets).toHaveLength(40);
});

test('editing relationships preserves exact untouched body bytes and quick actions carry a revision', async ({ page }) => {
  const original = await readFile(ticketPath(2));
  try {
    const text = original.toString();
    const boundary = text.indexOf('\n---\n') + 5;
    await writeFile(ticketPath(2), text.slice(0, boundary) + '\r\nExact body\r\nNo final newline');
    await ready(page, `#item=${id(2)}`);
    await editForm(page).getByLabel('Title', { exact: true }).focus();
    await editForm(page).getByRole('button', { name: 'Remove parent', exact: true }).click();
    await expect(editForm(page).getByLabel('Status', { exact: true })).toBeEnabled();
    await editForm(page).getByRole('button', { name: `Remove ${id(3)}`, exact: true }).click();
    await addRelationship(page, 'dependency', id(6));
    let saved = cli('show', id(2));
    expect(saved.ticket).toMatchObject({ parent: null, depends_on: [id(6)] });
    expect(saved.source.endsWith('\r\nExact body\r\nNo final newline')).toBe(true);
    const baseRevision = saved.ticket.revision;
    let payload;
    await page.route('**/api/items/*', async route => {
      if (route.request().method() !== 'PATCH') return route.continue();
      payload = route.request().postDataJSON();
      cli('update', id(2), '--title', 'Agent changed before quick save');
      await route.continue();
    });
    await page.evaluate(async id => (await import('/app.js')).changeItemStatus(id, 'done'), id(2));
    await expect(page.locator('.edit-message')).toContainText('CONFLICT');
    expect(payload).toMatchObject({ expected_revision: baseRevision, status: 'done' });
    expect(cli('show', id(2)).ticket.status).toBe('todo');
    await expect(editForm(page).getByLabel('Status', { exact: true })).toHaveValue('done');
    await cancelDraft(page);
  } finally { await writeFile(ticketPath(2), original); }
});

test('a dropped create response retains input, resynchronizes and prevents duplicate automatic creation', async ({ page }) => {
  let created, requests = 0;
  try {
    await ready(page);
    await page.getByRole('button', { name: 'New item', exact: true }).click();
    await createForm(page).getByLabel('Title', { exact: true }).fill('Possibly created');
    await page.route('**/api/items', async route => {
      if (route.request().method() !== 'POST') return route.continue();
      requests++;
      const response = await route.fetch();
      created = (await response.json()).result.ticket.id;
      await route.abort('failed');
    });
    await createForm(page).getByRole('button', { name: 'Create', exact: true }).click();
    await expect(page.locator('.edit-message')).toContainText('Save outcome unknown');
    await expect(createForm(page).getByRole('button', { name: 'Create', exact: true })).toBeDisabled();
    await expect(createForm(page).getByLabel('Title', { exact: true })).toHaveValue('Possibly created');
    await expect(page.locator('#status')).toContainText('41 items');
    await createForm(page).getByRole('button', { name: 'Review current version', exact: true }).click();
    await expect(page.locator('.edit-review')).toContainText(`${created} · Possibly created`);
    await createForm(page).evaluate(form => form.requestSubmit());
    expect(requests).toBe(1);
    expect(cli('show', created).ticket.title).toBe('Possibly created');
    await createForm(page).getByRole('button', { name: 'I checked the items; enable another create', exact: true }).click();
    await expect(createForm(page).getByRole('button', { name: 'Create', exact: true })).toBeEnabled();
    expect(requests).toBe(1);
    await cancelDraft(page);
  } finally { if (created) await rm(join(root, '.wrk', `${created}.md`), { force: true }); }
});

test('related items work from either endpoint and refresh without replacing a draft', async ({ page }, testInfo) => {
  const originals = await Promise.all([6, 39, 40].map(n => readFile(ticketPath(n))));
  const links = () => currentDetail(page).locator('.detail-section').filter({ has: page.getByRole('heading', { name: 'Related items', exact: true, includeHidden: true }) });
  try {
    await ready(page, `#item=${id(40)}`);
    await editForm(page).getByLabel('Title', { exact: true }).focus();
    await addRelationship(page, 'related', id(6));
    expect(cli('show', id(40)).ticket.related).toEqual([id(6)]);
    await expect(links().locator('a')).toHaveText('Retired experiment');
    await links().locator('a').click();
    await expectTitle(page, 'Retired experiment');
    await expect(links().locator('a')).toHaveText('Review integration 40');
    // Remove from the reverse endpoint, alongside an ordinary title change.
    await editForm(page).getByLabel('Title', { exact: true }).focus();
    await editForm(page).getByRole('button', { name: `Remove related ${id(40)}`, exact: true }).click();
    await editForm(page).getByLabel('Title', { exact: true }).fill('Related experiment');
    await saveEdit(page);
    expect(cli('show', id(40)).ticket.related).toEqual([]);
    expect(cli('show', id(6)).ticket.title).toBe('Related experiment');
    await listLink(page, 40).click();
    await expectTitle(page, 'Review integration 40');
    await editForm(page).getByLabel('Title', { exact: true }).focus();
    await editForm(page).getByLabel('Title', { exact: true }).fill('My retained related draft');
    await addRelationship(page, 'related', id(6));
    const originalRevision = cli('show', id(40)).ticket.revision;
    cli('update', id(39), '--add-related', id(40));
    await expect(page.locator('#draft-notice')).toContainText('changed externally', { timeout: 2000 });
    await expect(page.locator('#draft-notice')).toContainText('changed externally');
    expect(cli('show', id(40)).ticket.revision).toBe(originalRevision);
    await expect(editForm(page).getByLabel('Title', { exact: true })).toHaveValue('My retained related draft');
    await editForm(page).getByRole('button', { name: 'Save changes' }).click();
    await expect(page.locator('.edit-message')).toContainText('CONFLICT');
    expect(cli('show', id(40)).ticket.title).toBe('Review integration 40');
    await editForm(page).getByRole('button', { name: 'Review current version', exact: true }).click();
    await editForm(page).getByRole('button', { name: 'Keep draft using reviewed revision', exact: true }).click();
    await expect(editForm(page).getByRole('button', { name: `Remove related ${id(39)}`, exact: true })).toBeVisible();
    await saveEdit(page);
    expect(cli('show', id(40)).ticket.related).toEqual([id(6), id(39)]);
    expect(cli('show', id(39)).ticket.related).toEqual([id(40)]);
    await page.screenshot({ path: testInfo.outputPath('related-items.png'), fullPage: true });
    cli('update', id(39), '--remove-related', id(40));
    await expect(links().locator('a')).toHaveCount(1, { timeout: 2000 });
  } finally { await Promise.all([6, 39, 40].map((n, i) => writeFile(ticketPath(n), originals[i]))); }
});

test('created items support related links in details and self links retain the draft', async ({ page }) => {
  let created;
  try {
    await ready(page);
    await page.getByRole('button', { name: 'New item', exact: true }).click();
    await createForm(page).getByLabel('Title', { exact: true }).fill('Created with related context');
    await createForm(page).getByRole('button', { name: 'Create', exact: true }).click();
    await expect(createForm(page)).toHaveCount(0);
    created = new URL(page.url()).hash.match(/item=([^&]+)/)[1];
    await editForm(page).getByLabel('Title', { exact: true }).focus();
    await addRelationship(page, 'related', id(6));
    expect(cli('show', created).ticket.related).toEqual([id(6)]);
    expect(cli('show', id(6)).ticket.related).toEqual([created]);
    await editForm(page).getByLabel('Title', { exact: true }).focus();
    await addRelationship(page, 'related', created);
    await expect(page.locator('.edit-message')).toContainText('SELF_REFERENCE [related]');
    await expect(editForm(page).getByRole('combobox', { name: 'Add related item', exact: true, includeHidden: true })).toHaveAttribute('aria-invalid', 'true');
    await cancelDraft(page);
  } finally { if (created) await rm(join(root, '.wrk', `${created}.md`), { force: true }); }
});

test('compact rows keep titles primary and expose full values with usable copy actions', async ({ page, context }, testInfo) => {
  const original = await readFile(ticketPath(9));
  const title = 'Review a very long integration title that stays on one line even beside an open detail pane';
  try {
    await ticket(9, title, 'todo', 'labels: [backlog, design, extraordinarily-long-tag-name, frontend, review]\n');
    await context.grantPermissions(['clipboard-read', 'clipboard-write']);
    await ready(page);
    await expect(page.locator('#detail')).toBeHidden();
    expect(await page.locator('#browse').evaluate(node => node.clientWidth)).toBe(1440);
    expect(await page.locator('#list-scroll').evaluate(node => node.getBoundingClientRect().top)).toBeLessThan(140);
    expect(await page.evaluate(() => document.documentElement.scrollHeight <= innerHeight)).toBe(true);
    const row = page.locator(`[data-row="${id(9)}"]`);
    expect((await row.boundingBox()).height).toBe(40);
    await expect(row.locator('.row-tags .badge')).toHaveCount(3);
    await expect(row.locator('.tag-overflow')).toHaveText('+3');
    await expect(listLink(page, 9)).toHaveAttribute('title', title);
    await expect(listLink(page, 9)).toHaveAccessibleDescription(/extraordinarily-long-tag-name/);
    await page.screenshot({ path: testInfo.outputPath('compact-table.png'), fullPage: true });
    const action = row.getByRole('button', { name: `Actions for ${title}`, exact: true });
    await action.focus();
    await page.keyboard.press('Enter');
    const menu = row.locator('[popover]');
    await expect(menu).toBeVisible();
    await expect(menu).toContainText('extraordinarily-long-tag-name');
    await expect(page.locator('#detail')).toBeHidden();
    await page.keyboard.press('Tab');
    await expect(menu.getByRole('button', { name: 'Copy ID', exact: true })).toBeFocused();
    await page.keyboard.press('Enter');
    await expect(page.locator('#copy-feedback')).toHaveText('ID copied.');
    expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(id(9));
    await expect(page.locator('#detail')).toBeHidden();
    await action.click();
    await page.keyboard.press('Escape');
    await expect(menu).toBeHidden();
    await expect(action).toBeFocused();
    // Clicking row whitespace opens the same native-link destination.
    await row.click({ position: { x: 4, y: 20 } });
    await expectTitle(page, title);
    await expect(page.locator('#detail')).toBeFocused();
    expect(await page.locator('#browse').evaluate(node => node.clientWidth)).toBeGreaterThan(700);
    expect((await row.boundingBox()).height).toBe(40);
    await page.locator('#detail').getByRole('button', { name: 'Copy link', exact: true }).click();
    expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(page.url());
    await page.screenshot({ path: testInfo.outputPath('compact-detail.png'), fullPage: true });
    await page.getByRole('button', { name: 'Close', exact: false }).click();
    await expect(page.locator('#detail')).toBeHidden();
    await expect(listLink(page, 9)).toBeFocused();
  } finally { await writeFile(ticketPath(9), original); }
});

test('denied clipboard access provides selectable ID and link without navigating a row', async ({ page }) => {
  await page.addInitScript(() => {
    Object.defineProperty(navigator.clipboard, 'writeText', { value: async () => { throw new DOMException('Denied', 'NotAllowedError'); } });
  });
  await ready(page, '#view=all&label=web');
  const action = page.locator(`[data-row="${id(1)}"] .row-actions`);
  await action.click();
  await page.locator('[popover]:popover-open').getByRole('button', { name: 'Copy ID', exact: true }).click();
  await expect(page.getByLabel('ID to copy')).toHaveValue(id(1));
  await expect(page.getByLabel('ID to copy')).toBeFocused();
  await expect(page.locator('#detail')).toBeHidden();
  await page.getByRole('button', { name: 'Dismiss', exact: true }).click();
  await expect(action).toBeFocused();
  await listLink(page, 1).click();
  await expectTitle(page, 'Build a local workspace');
  await page.locator('#detail').getByRole('button', { name: 'Copy link', exact: true }).click();
  await expect(page.getByLabel('Link to copy')).toHaveValue(page.url());
  await page.getByRole('button', { name: 'Dismiss', exact: true }).click();
  await expect(page.locator('#detail').getByRole('button', { name: 'Copy link', exact: true })).toBeFocused();
});

test('narrow panes retain deep table scroll through relationships, polling, close and history', async ({ page }, testInfo) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await ready(page);
  expect(await page.evaluate(() => document.documentElement.scrollHeight <= innerHeight)).toBe(true);
  await page.screenshot({ path: testInfo.outputPath('compact-narrow-table.png'), fullPage: true });
  await listLink(page, 30).click();
  await expectTitle(page, 'Review integration 30');
  const scroll = await page.evaluate(() => history.state.listScroll);
  expect(scroll).toBeGreaterThan(0);
  await page.locator('#detail').evaluate(node => { node.scrollTop = 150; });
  // Trigger a full read while the table is display:none, then navigate a relationship.
  await page.getByLabel('Project information and actions').click();
  await page.getByRole('button', { name: 'Reload project', exact: true }).click();
  await expect(page.locator('#status')).toContainText('Live');
  expect(await page.evaluate(() => history.state.listScroll)).toBe(scroll);
  await page.getByRole('button', { name: 'Back to table', exact: false }).click();
  await expect(listLink(page, 30)).toBeFocused();
  await expect.poll(() => page.locator('#list-scroll').evaluate(node => node.scrollTop)).toBe(scroll);
  await page.goBack();
  await expectTitle(page, 'Review integration 30');
  await page.goForward();
  await expect(page.locator('#detail')).toBeHidden();
  await expect.poll(() => page.locator('#list-scroll').evaluate(node => node.scrollTop)).toBe(scroll);
  // Selected links and relationship navigation preserve filters on narrow screens.
  await page.getByRole('searchbox').fill('Build');
  await listLink(page, 2).click();
  await expectTitle(page, 'Build search');
  await page.locator('#detail').getByRole('link', { name: 'Confirm contracts', exact: true }).click();
  await expectTitle(page, 'Confirm contracts');
  await expect(page.locator('#detail')).toContainText('outside the current list filters');
  await expect(page.getByRole('searchbox')).toHaveValue('Build');
  await page.screenshot({ path: testInfo.outputPath('compact-narrow-detail.png'), fullPage: true });
  for (const width of [320, 390, 760, 800]) {
    await page.setViewportSize({ width, height: 844 });
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  }
});


test('compact disclosures stay within narrow viewports and expose actionable diagnostics', async ({ page }, testInfo) => {
  await page.setViewportSize({ width: 320, height: 700 });
  await ready(page);
  for (const trigger of ['#project-menu summary', '#label-filter summary']) {
    await page.locator(trigger).click();
    const panel = page.locator(trigger).locator('..').locator('.disclosure-panel');
    await expect(panel).toBeVisible();
    const box = await panel.boundingBox();
    expect(box.x).toBeGreaterThanOrEqual(0);
    expect(box.x + box.width).toBeLessThanOrEqual(320);
    await page.keyboard.press('Escape');
    await expect(panel).toBeHidden();
    await expect(page.locator(trigger)).toBeFocused();
  }
  const original = await readFile(ticketPath(8));
  try {
    await writeFile(ticketPath(8), 'broken');
    await expect(page.locator('#problem-summary')).toContainText('Showing stale data', { timeout: 2000 });
    await expect(page.locator('#diagnostics')).toBeHidden();
    await page.locator('#problem-summary').click();
    await expect(page.locator('#diagnostics')).toBeVisible();
    await expect(page.locator('#diagnostics')).toContainText('INVALID_TICKET');
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await page.screenshot({ path: testInfo.outputPath('compact-narrow-error.png'), fullPage: true });
  } finally { await writeFile(ticketPath(8), original); }
  await expect(page.locator('#problem')).toBeHidden({ timeout: 2000 });
});

test('filtered row progress stays current and reload restores the table position', async ({ page }) => {
  const original = await readFile(ticketPath(3));
  try {
    await ready(page, '#q=Build%20a%20local');
    await expect(page.locator('#items > li')).toHaveCount(1);
    await expect(listLink(page, 1)).toHaveAccessibleDescription(/1\/4 children done/);
    cli('update', id(3), '--status', 'canceled');
    await expect(listLink(page, 1)).toHaveAccessibleDescription(/0\/4 children done/, { timeout: 2000 });
    await page.locator('#reset').click();
    await listLink(page, 30).click();
    await expectTitle(page, 'Review integration 30');
    const scroll = await page.locator('#list-scroll').evaluate(node => node.scrollTop);
    expect(scroll).toBeGreaterThan(0);
    await page.reload();
    await expectTitle(page, 'Review integration 30');
    await expect.poll(() => page.locator('#list-scroll').evaluate(node => node.scrollTop)).toBe(scroll);
  } finally { await writeFile(ticketPath(3), original); }
});

test('creation modal traps focus, preserves picker queries, confirms dirty close and restores its opener', async ({ page }, testInfo) => {
  await ready(page);
  const opener = page.getByRole('button', { name: 'New item', exact: true });
  await opener.click();
  const modal = page.getByRole('dialog', { name: 'New item', exact: true });
  const form = createForm(page);
  const title = form.getByLabel('Title', { exact: true });
  await expect(modal).toBeVisible();
  await expect(title).toBeFocused();
  await page.keyboard.press('Escape');
  await expect(modal).not.toBeVisible();
  await expect(opener).toBeFocused();
  await opener.click();
  await form.getByRole('button', { name: 'Create', exact: true }).focus();
  await page.keyboard.press('Tab');
  await expect(form.getByRole('button', { name: 'Close new item' })).toBeFocused();
  await page.keyboard.press('Shift+Tab');
  await expect(form.getByRole('button', { name: 'Create', exact: true })).toBeFocused();
  await title.fill('Keyboard creation draft');
  await title.press('Enter');
  await expect(form).toBeVisible();
  const body = form.getByLabel('Description (Markdown)');
  await body.fill('First line');
  await body.press('Enter');
  await body.pressSequentially('Second line');
  await expect(body).toHaveValue('First line\nSecond line');
  await form.getByRole('button', { name: 'Add tag', exact: true }).click();
  const tags = form.getByRole('combobox', { name: 'Tag to add' });
  await tags.fill('backl');
  await expect(form.getByRole('option', { name: 'backlog', exact: true })).toBeVisible();
  await tags.press('ArrowDown');
  await tags.press('Home');
  await tags.press('Enter');
  await expect(form.getByRole('button', { name: 'Remove tag backlog' })).toBeVisible();
  await form.getByRole('button', { name: 'Add tag', exact: true }).click();
  await tags.fill('unfinished tag');
  await page.keyboard.press('Escape');
  await expect(modal).toBeVisible();
  await expect(form.getByRole('button', { name: 'Add tag', exact: true })).toBeFocused();
  await expect(form.getByRole('button', { name: 'Add tag', exact: true })).toHaveAccessibleDescription('Pending input: unfinished tag');
  await form.getByRole('button', { name: 'Add tag', exact: true }).click();
  await expect(tags).toHaveValue('unfinished tag');
  await page.keyboard.press('Escape');
  page.once('dialog', dialog => dialog.dismiss());
  await page.keyboard.press('Escape');
  await expect(title).toHaveValue('Keyboard creation draft');
  const protectedDeparture = await page.evaluate(() => {
    const event = new Event('beforeunload', { cancelable: true });
    window.dispatchEvent(event); return event.defaultPrevented;
  });
  expect(protectedDeparture).toBe(true);
  await page.screenshot({ path: testInfo.outputPath('create-desktop.png'), fullPage: true });
  await form.getByRole('button', { name: 'Parent item', exact: true }).click();
  const parent = form.getByRole('combobox', { name: 'Find parent item' });
  await parent.fill('Confirm');
  await parent.press('End');
  await parent.press('ArrowUp');
  await parent.press('ArrowDown');
  await parent.press('Enter');
  await expect(form.getByRole('button', { name: 'Parent item', exact: true })).toContainText('Confirm contracts');
  await form.getByRole('button', { name: 'Remove parent' }).click();
  await expect(form.getByRole('button', { name: 'Parent item', exact: true })).toHaveText('+ Add parent');
  for (const width of [320, 390, 760]) {
    await page.setViewportSize({ width, height: 700 });
    await form.getByRole('button', { name: 'Add tag', exact: true }).click();
    const bounds = await modal.boundingBox();
    expect(bounds.x).toBeGreaterThanOrEqual(0);
    expect(bounds.x + bounds.width).toBeLessThanOrEqual(width);
    expect(await modal.evaluate(node => node.scrollWidth <= node.clientWidth)).toBe(true);
    await page.screenshot({ path: testInfo.outputPath(`create-picker-${width}.png`), fullPage: true });
    await page.keyboard.press('Escape');
  }
  await cancelDraft(page);
  await expect(opener).toBeFocused();
  expect(cli('list', '--all').tickets).toHaveLength(40);
});

test('creation uses publication defaults, freezes explicit empty tags and preserves filter context', async ({ page }) => {
  const configPath = join(root, '.wrk/config.yaml');
  const config = await readFile(configPath);
  const created = [], payloads = [];
  try {
    await writeFile(configPath, 'version: 1\nprefix: task\ndefaults: {priority: high, labels: [default]}\n');
    await ready(page, `#q=Build&label=web&focus=${id(1)}`);
    await page.route('**/api/items', async route => {
      if (route.request().method() === 'POST') payloads.push(route.request().postDataJSON());
      await route.continue();
    });
    await page.getByRole('button', { name: 'New item', exact: true }).click();
    const form = createForm(page);
    await expect(form.getByRole('button', { name: 'Remove tag default' })).toBeVisible();
    await expect(form.getByRole('button', { name: 'Parent item', exact: true })).toHaveText('+ Add parent');
    // Config changes refresh effective tags without marking an empty draft dirty.
    await writeFile(configPath, 'version: 1\nprefix: task\ndefaults: {priority: urgent, labels: [changed]}\n');
    await expect(form.getByRole('button', { name: 'Remove tag changed' })).toBeVisible({ timeout: 2000 });
    await page.keyboard.press('Escape');
    await expect(form).toHaveCount(0);
    await page.getByRole('button', { name: 'New item', exact: true }).click();
    await form.getByLabel('Title', { exact: true }).fill('Outside filters');
    // The server applies a newer default even before the next browser poll.
    await page.route('**/api/items', async route => {
      payloads.push(route.request().postDataJSON());
      await writeFile(configPath, 'version: 1\nprefix: task\ndefaults: {priority: urgent, labels: [at-publication]}\n');
      await route.continue();
    });
    await form.getByLabel('Title', { exact: true }).press('Control+Enter');
    await expect(form).toHaveCount(0);
    await expectTitle(page, 'Outside filters');
    await expect(page.locator('#detail')).toBeFocused();
    created.push(new URL(page.url()).hash.match(/item=([^&]+)/)[1]);
    expect(payloads[0]).not.toHaveProperty('labels');
    expect(cli('show', created[0]).ticket).toMatchObject({ labels: ['at-publication'], priority: 'urgent', status: 'todo', parent: null });
    await expect(page.locator('#detail')).toContainText('outside the current list filters');
    await expect(page.getByRole('searchbox')).toHaveValue('Build');
    expect(new URL(page.url()).hash).toContain('label=web');
    expect(new URL(page.url()).hash).toContain(`under=${id(1)}`);
    await page.unroute('**/api/items');
    await page.route('**/api/items', async route => { payloads.push(route.request().postDataJSON()); await route.continue(); });
    await page.getByRole('button', { name: 'New item', exact: true }).click();
    await form.getByLabel('Title', { exact: true }).fill('Explicitly no tags');
    await form.getByRole('button', { name: 'Remove tag at-publication' }).click();
    await writeFile(configPath, 'version: 1\nprefix: task\ndefaults: {priority: low, labels: [later]}\n');
    // Force a current snapshot while retaining the modal and its explicit list.
    await page.evaluate(() => window.dispatchEvent(new Event('focus')));
    await expect(page.locator('#status')).toContainText('Live');
    await expect(form.getByRole('button', { name: 'Remove tag later' })).toHaveCount(0);
    await form.getByRole('button', { name: 'Create', exact: true }).click();
    await expectTitle(page, 'Explicitly no tags');
    created.push(new URL(page.url()).hash.match(/item=([^&]+)/)[1]);
    expect(payloads.at(-1).labels).toEqual([]);
    expect(cli('show', created[1]).ticket).toMatchObject({ labels: [], priority: 'low', parent: null });
  } finally {
    for (const item of created) await rm(join(root, '.wrk', `${item}.md`), { force: true });
    await writeFile(configPath, config);
  }
});

test('creation retains input through live changes, navigation and save errors, with one in-flight submit', async ({ page }) => {
  const original = await readFile(ticketPath(40));
  let created, requests = 0, release;
  try {
    await ready(page);
    await page.getByRole('button', { name: 'New item', exact: true }).click();
    const form = createForm(page);
    const title = form.getByLabel('Title', { exact: true });
    await title.fill('   ');
    await form.getByRole('button', { name: 'Create', exact: true }).click();
    await expect(title).toHaveAttribute('aria-invalid', 'true');
    await expect(title).toHaveValue('   ');
    await expect(form.locator('#edit-error-title')).toBeVisible();
    await title.fill('Retained modal draft');
    await form.getByRole('button', { name: 'Parent item', exact: true }).click();
    const parent = form.getByRole('combobox', { name: 'Find parent item' });
    await parent.fill('not selected');
    await form.getByRole('button', { name: 'Create', exact: true }).click();
    await expect(form.locator('#edit-error-parent')).toContainText('Choose a parent');
    await expect(parent).toHaveValue('not selected');
    await parent.fill('task-ffffffff');
    await form.getByRole('button', { name: 'Create', exact: true }).click();
    await expect(form.locator('.edit-message')).toContainText('parent');
    await expect(form.getByRole('button', { name: 'Parent item', exact: true })).toHaveAttribute('aria-invalid', 'true');
    await form.getByRole('button', { name: 'Remove parent' }).click();
    await form.getByRole('button', { name: 'Add tag', exact: true }).click();
    const tags = form.getByRole('combobox', { name: 'Tag to add' });
    await tags.fill('live-tag');
    cli('update', id(40), '--add-label', 'live-tag');
    await expect(form.getByRole('option', { name: 'live-tag', exact: true })).toBeVisible({ timeout: 2000 });
    await expect(tags).toHaveValue('live-tag');
    // Internal navigation and a request to open another surface cannot replace this draft.
    await page.evaluate(selected => { location.hash = `#q=Build&item=${selected}`; }, id(2));
    await expect(title).toHaveValue('Retained modal draft');
    await expect(page.locator('#detail .detail-title')).toHaveText('Build search');
    await page.locator('#new-item').evaluate(button => button.click());
    await expect(editForm(page)).toHaveCount(0);
    await expect(title).toBeFocused();
    await expect(tags).toHaveValue('live-tag');
    await page.route('**/api/items', async route => {
      requests++;
      if (requests === 1) return route.fulfill({ status: 503, contentType: 'application/json', body: JSON.stringify({ ok: false, errors: [{ code: 'BUSY', message: 'Writer owns the project lock' }] }) });
      await new Promise(resolve => { release = resolve; });
      await route.continue();
    });
    await form.getByRole('button', { name: 'Create', exact: true }).click();
    await expect(form.locator('.edit-message')).toContainText('BUSY');
    await expect(title).toHaveValue('Retained modal draft');
    await expect(form.getByRole('button', { name: 'Remove tag live-tag' })).toBeVisible();
    await expect(form.getByRole('button', { name: 'Create', exact: true })).toBeEnabled();
    await form.getByRole('button', { name: 'Create', exact: true }).click();
    await expect.poll(() => requests).toBe(2);
    await expect(form.getByRole('button', { name: 'Create', exact: true })).toBeDisabled();
    await expect(form.getByRole('button', { name: 'Close new item' })).toBeDisabled();
    await form.evaluate(form => { form.requestSubmit(); form.requestSubmit(); });
    await page.keyboard.press('Escape');
    await expect(form).toBeVisible();
    expect(requests).toBe(2);
    release();
    await expect(form).toHaveCount(0);
    await expectTitle(page, 'Retained modal draft');
    created = new URL(page.url()).hash.match(/item=([^&]+)/)[1];
    expect(cli('show', created).ticket).toMatchObject({ labels: ['live-tag'], parent: null });
    expect(requests).toBe(2);
  } finally {
    release?.();
    if (created) await rm(join(root, '.wrk', `${created}.md`), { force: true });
    await writeFile(ticketPath(40), original);
  }
});

test('an existing text draft remains protected when New is requested', async ({ page }) => {
  await ready(page, `#item=${id(2)}`);
  await editForm(page).getByLabel('Title', { exact: true }).focus();
  await editForm(page).getByLabel('Title', { exact: true }).fill('Keep this text draft');
  await page.getByRole('button', { name: 'New item', exact: true }).click();
  await expect(createForm(page)).toHaveCount(0);
  await expect(editForm(page).getByLabel('Title', { exact: true })).toHaveValue('Keep this text draft');
  await expect(editForm(page).getByLabel('Title', { exact: true })).toBeFocused();
  await cancelDraft(page);
});

test('confirmed metadata and row writes preserve in-flight text and only advance their own revision', async ({ page }) => {
  const original = await readFile(ticketPath(40));
  const requests = [];
  let release, published;
  try {
    await ready(page, `#item=${id(40)}`);
    await editForm(page).getByLabel('Title', { exact: true }).fill('Typed before metadata save');
    await page.route('**/api/items/*', async route => {
      const payload = route.request().postDataJSON(); requests.push(payload);
      const response = await route.fetch();
      published = (await response.json()).result.ticket.revision;
      await new Promise(resolve => { release = resolve; });
      await route.fulfill({ response });
    });
    await editForm(page).getByLabel('Status', { exact: true }).selectOption('blocked');
    await expect.poll(() => Boolean(release)).toBe(true);
    await editForm(page).getByLabel('Title', { exact: true }).fill('Typed while save was pending');
    await editForm(page).getByRole('button', { name: 'Edit description', exact: true }).click();
    await editForm(page).getByLabel('Description (Markdown)', { exact: true }).fill('Retained body');
    expect(await page.evaluate(async id => (await import('/app.js')).changeItemStatus(id, 'done'), id(40))).toBe(false);
    expect(requests).toHaveLength(1);
    expect(Object.keys(requests[0]).sort()).toEqual(['expected_related_revision', 'expected_revision', 'status']);
    release();
    await expect(editForm(page).getByLabel('Status', { exact: true })).toBeEnabled();
    await expect(editForm(page).getByLabel('Title', { exact: true })).toHaveValue('Typed while save was pending');
    await expect(page.locator('#draft-notice')).not.toContainText('changed externally');
    await page.unroute('**/api/items/*');
    await page.route('**/api/items/*', async route => {
      requests.push(route.request().postDataJSON());
      const response = await route.fetch();
      if (requests.length === 2) cli('update', id(40), '--title', 'Agent after confirmed metadata');
      await route.fulfill({ response });
    });
    await page.locator(`[data-row-status="${id(40)}"]`).selectOption('done');
    await expect(editForm(page).getByLabel('Status', { exact: true })).toBeEnabled();
    expect(requests[1].expected_revision).toBe(published);
    await expect(page.locator('#draft-notice')).toContainText('changed externally');
    await editForm(page).getByRole('button', { name: 'Save changes' }).click();
    await expect(page.locator('.edit-message')).toContainText('CONFLICT');
    expect(requests).toHaveLength(3);
    expect(requests[2].expected_revision).not.toBe(cli('show', id(40)).ticket.revision);
    await expect(editForm(page).getByLabel('Description (Markdown)', { exact: true })).toHaveValue('Retained body');
    expect(cli('show', id(40)).ticket).toMatchObject({ title: 'Agent after confirmed metadata', status: 'done' });
  } finally { release?.(); await writeFile(ticketPath(40), original); }
});

test('live CLI edits preserve real document input, caret, expansion and both pane scroll positions', async ({ page }, testInfo) => {
  const originals = await Promise.all([39, 40].map(n => readFile(ticketPath(n))));
  try {
    await ready(page);
    await toggleRow(page, 1).click();
    await listLink(page, 40).click();
    await editForm(page).getByRole('button', { name: 'Edit description' }).click();
    const body = editForm(page).getByLabel('Description (Markdown)', { exact: true });
    const draft = 'Retained document draft\n'.repeat(30);
    await body.fill(draft);
    await body.evaluate(input => { input.setSelectionRange(10, 16); input.scrollTo({ top: 120, behavior: 'instant' }); });
    await page.locator('#list-scroll').evaluate(node => { node.scrollTop = 700; });
    await page.locator('#detail').evaluate(node => { node.scrollTop = 140; });
    const context = () => page.evaluate(() => ({
      caret: [document.querySelector('#detail-body').selectionStart, document.querySelector('#detail-body').selectionEnd],
      scroll: ['#list-scroll', '#detail', '#detail-body'].map(selector => document.querySelector(selector).scrollTop),
    }));
    const before = await context();
    expect(before.scroll[0]).toBeGreaterThan(0);
    expect(before.scroll[1]).toBeGreaterThan(0);
    expect(before.scroll[2]).toBeGreaterThan(0);
    cli('update', id(39), '--title', 'Agent updated the neighboring row');
    cli('update', id(40), '--title', 'Agent updated this document', '--add-label', 'agent-tag');
    await expect(page.locator('#draft-notice')).toContainText('changed externally', { timeout: 2000 });
    await expect(listLink(page, 39)).toHaveText('Agent updated the neighboring row');
    await expect(body).toHaveValue(draft);
    await expect(body).toBeFocused();
    expect(await context()).toEqual(before);
    await expect(toggleRow(page, 1)).toHaveAttribute('aria-expanded', 'false');
    await expect(page).toHaveURL(new RegExp(`item=${id(40)}`));
    await page.screenshot({ path: testInfo.outputPath('dirty-detail-live-desktop.png'), fullPage: true });
    await editForm(page).getByRole('button', { name: 'Save changes' }).click();
    await expect(editForm(page).locator('.edit-message')).toContainText('CONFLICT');
    await expect(body).toHaveValue(draft);
    expect(cli('show', id(40)).ticket).toMatchObject({ title: 'Agent updated this document', labels: ['agent-tag', 'backlog'] });
    expect(cli('show', id(40)).source).not.toContain('Retained document draft');
  } finally { await Promise.all([39, 40].map((n, i) => writeFile(ticketPath(n), originals[i]))); }
});

test('cancel keeps confirmed metadata, title is required, reparenting preserves children and dependency cycles stay local', async ({ page }) => {
  const original = await readFile(ticketPath(2));
  const child = await readFile(ticketPath(4));
  try {
    await ready(page, `#item=${id(2)}`);
    await editForm(page).getByLabel('Title', { exact: true }).fill('');
    await expect(editForm(page).getByRole('button', { name: 'Save changes' })).toBeDisabled();
    await changeParent(page, id(6));
    expect(cli('show', id(2)).ticket).toMatchObject({ parent: id(6), status: 'todo', title: 'Build search' });
    expect(await readFile(ticketPath(4))).toEqual(child);
    await expect(editForm(page).getByLabel('Title', { exact: true })).toHaveValue('');
    await cancelDraft(page);
    await expectTitle(page, 'Build search');
    expect(cli('show', id(2)).ticket.parent).toBe(id(6));
    // 2 depends on 3 already; adding 2 to 3 must report a cycle beside the picker.
    await page.locator('.relationship-editor').getByRole('link', { name: 'Confirm contracts' }).click();
    await addRelationship(page, 'dependency', id(2));
    await expect(page.locator('.edit-message')).toContainText('CYCLE');
    await expect(editForm(page).getByRole('combobox', { name: 'Add dependency', includeHidden: true })).toHaveAttribute('aria-invalid', 'true');
    await expect(editForm(page).getByRole('button', { name: `Remove ${id(2)}` })).toBeVisible();
    expect(cli('show', id(3)).ticket.depends_on).toEqual([]);
  } finally { await writeFile(ticketPath(2), original); }
});

test('metadata uncertainty retains input, requires review and never retries related publication automatically', async ({ page }) => {
  const originals = await Promise.all([39, 40].map(n => readFile(ticketPath(n))));
  let requests = 0;
  try {
    await ready(page, `#item=${id(40)}`);
    await editForm(page).getByLabel('Title', { exact: true }).fill('Retain through metadata uncertainty');
    await page.route('**/api/items/*', async route => {
      requests++;
      const response = await route.fetch();
      const data = await response.json();
      data.ok = false;
      data.errors = [{ code: 'DURABILITY_UNCERTAIN', field: 'related', message: 'Injected partial link completion failure' }];
      data.result.updates = [{ ticket: { id: id(40) }, publication: 'committed' }, { ticket: { id: id(39) }, publication: 'pending' }];
      await route.fulfill({ status: 500, json: data });
    });
    await editForm(page).getByRole('button', { name: 'Add related item', exact: true }).click();
    await editForm(page).getByRole('combobox', { name: 'Add related item' }).fill(id(39));
    await editForm(page).getByRole('combobox', { name: 'Add related item' }).press('Enter');
    await expect(page.locator('.edit-message')).toContainText(`${id(39)}: pending`);
    await expect(editForm(page).getByLabel('Status', { exact: true })).toBeDisabled();
    await expect(editForm(page).getByRole('button', { name: 'Save changes' })).toBeDisabled();
    expect(requests).toBe(1);
    await editForm(page).getByRole('button', { name: 'Review current version', exact: true }).click();
    await expect(page.locator('.edit-review')).toContainText(id(39));
    await editForm(page).getByRole('button', { name: 'Keep draft using reviewed revision', exact: true }).click();
    await expect(editForm(page).getByLabel('Title', { exact: true })).toHaveValue('Retain through metadata uncertainty');
    await expect(editForm(page).getByLabel('Status', { exact: true })).toBeEnabled();
    expect(requests).toBe(1);
    await page.unroute('**/api/items/*');
    // A second agent write after review must still reject the explicitly retried text save.
    cli('update', id(40), '--title', 'Agent after review');
    await editForm(page).getByRole('button', { name: 'Save changes' }).click();
    await expect(page.locator('.edit-message')).toContainText('CONFLICT');
    expect(cli('show', id(40)).ticket.title).toBe('Agent after review');
  } finally { await Promise.all([39, 40].map((n, i) => writeFile(ticketPath(n), originals[i]))); }
});

test('document focus, pending pickers, narrow close/back and reconnect retain the same draft', async ({ page, context }) => {
  const original = await readFile(ticketPath(40));
  try {
    await page.setViewportSize({ width: 390, height: 844 });
    await ready(page, `#item=${id(40)}`);
    const title = editForm(page).getByLabel('Title', { exact: true });
    await title.focus();
    expect(await title.evaluate(input => getComputedStyle(input).borderTopWidth)).toBe('0px');
    expect(await title.evaluate(input => getComputedStyle(input).outlineStyle)).not.toBe('none');
    await title.fill('Retained through reconnect');
    await editForm(page).getByRole('button', { name: 'Parent item', exact: true }).click();
    const parentInput = editForm(page).getByRole('combobox', { name: 'Find parent item', includeHidden: true });
    await parentInput.fill('Retired');
    await addTag(page, 'retained-tag');
    await expect(parentInput).toHaveValue('Retired');
    await page.getByRole('button', { name: /Back to table/ }).click();
    await expect(page.locator('#browse')).toBeVisible();
    await expect(page.locator('#draft-notice')).toContainText('Return to draft');
    await page.locator('#draft-notice').getByRole('link', { name: 'Return to draft' }).click();
    await expect(title).toHaveValue('Retained through reconnect');
    await expect(parentInput).toHaveValue('Retired');
    await context.setOffline(true);
    await page.evaluate(() => window.dispatchEvent(new Event('offline')));
    await expect(page.locator('#draft-notice')).toContainText('stale');
    await expect(editForm(page).getByRole('button', { name: 'Save changes' })).toBeDisabled();
    cli('update', id(40), '--title', 'Agent during reconnect');
    await context.setOffline(false);
    await page.evaluate(() => window.dispatchEvent(new Event('online')));
    await expect(page.locator('#draft-notice')).toContainText('changed externally');
    await editForm(page).getByRole('button', { name: 'Review current version', exact: true }).click();
    await editForm(page).getByRole('button', { name: 'Keep draft using reviewed revision', exact: true }).click();
    await expect(parentInput).toHaveValue('Retired');
    await expect(title).toHaveValue('Retained through reconnect');
    await saveEdit(page);
    expect(cli('show', id(40)).ticket).toMatchObject({ title: 'Retained through reconnect', labels: ['backlog', 'retained-tag'], parent: null });
    await expect(parentInput).toHaveValue('Retired');
  } finally { await context.setOffline(false); await writeFile(ticketPath(40), original); }
});

test('collapse keeps selected details and manual choices survive exploration, live reads and history', async ({ page }, testInfo) => {
  await ready(page);
  await listLink(page, 4).click();
  await expectTitle(page, 'Deep descendant');
  await toggleRow(page, 2).focus();
  await page.keyboard.press('Enter');
  await expect(toggleRow(page, 2)).toBeFocused();
  await expect(toggleRow(page, 2)).toHaveAttribute('aria-expanded', 'false');
  await expect(listLink(page, 4)).toHaveCount(0);
  await expect(page.locator('#selection-context')).toContainText('hidden by a collapsed ancestor');
  await expectTitle(page, 'Deep descendant');
  await toggleRow(page, 1).click();
  await expect(page.locator('#results')).toHaveText('38 of 40 items match · 3 hidden by collapse');
  await page.getByRole('searchbox').fill('dDSC');
  await expect(listLink(page, 4)).toBeVisible();
  await expect(page.locator('#results')).toHaveText('1 of 40 items match');
  await expect(toggleRow(page, 1)).toBeDisabled();
  await expect(listLink(page, 1)).toHaveAccessibleDescription(/Context ancestor; does not match/);
  await expect(page.locator('#selection-context')).toBeHidden();
  await page.screenshot({ path: testInfo.outputPath('hierarchy-search-desktop.png'), fullPage: true });
  await page.getByRole('searchbox').fill('');
  await expect(toggleRow(page, 1)).toHaveAttribute('aria-expanded', 'false');
  await page.getByRole('button', { name: 'Blocked', exact: true }).click();
  await expect(listLink(page, 4)).toBeVisible();
  await page.goBack();
  await expect(toggleRow(page, 1)).toHaveAttribute('aria-expanded', 'false');
  await page.getByLabel('Project information and actions').click();
  await page.getByRole('button', { name: 'Reload project' }).click();
  await expect(page.locator('#status')).toContainText('Live');
  await expect(toggleRow(page, 1)).toHaveAttribute('aria-expanded', 'false');
  await page.reload();
  await expectTitle(page, 'Deep descendant');
  await expect(toggleRow(page, 1)).toHaveAttribute('aria-expanded', 'false');
  await toggleRow(page, 1).click();
  await expect(toggleRow(page, 2)).toHaveAttribute('aria-expanded', 'false');
  await page.locator('#label-filter summary').click();
  await page.getByLabel('backend', { exact: true }).check();
  await expect(listLink(page, 4)).toBeVisible();
  await page.getByRole('button', { name: 'Remove tag filter backend' }).click();
  await expect(toggleRow(page, 2)).toHaveAttribute('aria-expanded', 'false');
  await expect(page.locator('#label-filter summary')).toBeFocused();
  await chooseUnder(page, 2);
  await expect(page.locator('#results')).toHaveText('1 of 40 items match');
  await expect(listLink(page, 1)).toHaveCount(0);
  await expect(listLink(page, 2)).toHaveAccessibleDescription(/Context ancestor/);
  await page.getByRole('button', { name: 'Clear Under filter' }).click();
  await expect(toggleRow(page, 2)).toHaveAttribute('aria-expanded', 'false');
});

test('live closed ancestors, reparenting and status changes retain search and picker focus', async ({ page }) => {
  const originals = await Promise.all([1, 4, 7].map(n => readFile(ticketPath(n))));
  try {
    await ready(page);
    await toggleRow(page, 1).click();
    cli('update', id(1), '--status', 'done');
    await expect(listLink(page, 1)).toHaveAccessibleDescription(/Context ancestor/, { timeout: 2000 });
    await expect(toggleRow(page, 1)).toHaveAttribute('aria-expanded', 'false');
    await page.getByRole('searchbox').fill('DDsc');
    await listLink(page, 4).click();
    await expectTitle(page, 'Deep descendant');
    await page.getByRole('searchbox').focus();
    cli('update', id(7), '--status', 'canceled');
    cli('update', id(4), '--parent', id(7));
    await expect(listLink(page, 1)).toHaveCount(0, { timeout: 2000 });
    await expect(listLink(page, 7)).toHaveAccessibleDescription(/Context ancestor/, { timeout: 2000 });
    await expect(page.getByRole('searchbox')).toBeFocused();
    await expect(page.getByRole('searchbox')).toHaveValue('DDsc');
    await expect(page.locator('#results')).toHaveText('1 of 40 items match');
    await page.getByRole('button', { name: 'Under parent', exact: true }).click();
    const input = page.getByRole('combobox', { name: 'Under parent', exact: true });
    await input.fill('Indep');
    cli('update', id(4), '--status', 'done');
    await expect(page.locator('#empty')).toContainText('No matching items', { timeout: 2000 });
    await expect(page.locator('#selection-context')).toContainText('outside the current list filters');
    await expectTitle(page, 'Deep descendant');
    await expect(input).toBeFocused();
    await expect(input).toHaveValue('Indep');
    await page.keyboard.press('Escape');
    await page.getByRole('button', { name: 'All', exact: true }).click();
    await expect(listLink(page, 4)).toBeVisible();
    await expect(page.locator('#items > li')).toHaveCount(2);
    await expect(page.locator('#results')).toHaveText('1 of 40 items match');
  } finally { await Promise.all([1, 4, 7].map((n, i) => writeFile(ticketPath(n), originals[i]))); }
});

test('legacy focus preserves selection, migrates links to Under and clears missing scopes', async ({ page }) => {
  await ready(page, `#view=all&focus=${id(1)}&item=${id(1)}`);
  await expectTitle(page, 'Build a local workspace');
  await expect(page.locator('#results')).toHaveText('5 of 40 items match');
  await expect(page.locator('#selection-context')).toContainText('outside the current list filters');
  await expect(listLink(page, 2)).toHaveAttribute('href', new RegExp(`under=${id(1)}&item=${id(2)}`));
  await listLink(page, 2).click();
  await expect(page).toHaveURL(new RegExp(`under=${id(1)}`));
  await page.goBack();
  await expectTitle(page, 'Build a local workspace');
  await page.goto(base + `#under=task-deadbeef&item=${id(2)}`);
  await expectTitle(page, 'Build search');
  await expect(page.locator('#empty')).toContainText('Under item not found');
  await page.locator('#empty').getByRole('button', { name: 'Clear filters' }).click();
  await expectTitle(page, 'Build search');
  await expect(page.getByRole('searchbox')).toBeFocused();
  await expect(listLink(page, 2)).toBeVisible();
});

test('deep rendered hierarchy bounds indentation and toolbar pickers on narrow screens', async ({ page }, testInfo) => {
  await page.route('**/api/workspace', async route => {
    const response = await route.fetch();
    if (response.status() === 304) return route.fulfill({ response });
    const data = await response.json();
    const sample = data.result.tickets[0];
    data.result.tickets = Array.from({ length: 120 }, (_, n) => ({ ...sample, id: id(1000 + n),
      title: n === 119 ? 'Needle leaf' : `Ancestor ${n}`, status: 'todo', labels: [], blockers: [],
      parent: n ? id(999 + n) : null }));
    await route.fulfill({ response, json: data });
  });
  await page.goto(base);
  await expect(page.locator('#items > li')).toHaveCount(120);
  const leaf = page.locator(`[data-row="${id(1119)}"]`);
  await expect(leaf).toHaveAttribute('data-depth', '119');
  expect(await leaf.evaluate(node => node.style.getPropertyValue('--depth'))).toBe('6');
  await page.locator(`[data-toggle="${id(1000)}"]`).click();
  await expect(page.locator('#items > li')).toHaveCount(1);
  await page.getByRole('searchbox').fill('NDLLF');
  await expect(page.locator('#items > li')).toHaveCount(120);
  await expect(page.locator('#results')).toHaveText('1 of 120 items match');
  for (const width of [320, 390, 760]) {
    await page.setViewportSize({ width, height: 844 });
    await leaf.scrollIntoViewIfNeeded();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    expect(await page.locator('#list-scroll').evaluate(node => node.scrollWidth <= node.clientWidth)).toBe(true);
    await expect(page.getByRole('searchbox')).toBeVisible();
    await expect(page.locator('.context-label').last()).toBeVisible();
    await page.getByRole('button', { name: 'Under parent', exact: true }).click();
    const panel = page.locator('#under-panel');
    const box = await panel.boundingBox();
    expect(box.x).toBeGreaterThanOrEqual(0);
    expect(box.x + box.width).toBeLessThanOrEqual(width);
    await page.screenshot({ path: testInfo.outputPath(`hierarchy-under-${width}.png`), fullPage: true });
    await page.keyboard.press('Escape');
  }
  await page.setViewportSize({ width: 390, height: 844 });
  await page.screenshot({ path: testInfo.outputPath('hierarchy-narrow-deep.png'), fullPage: true });
});

test('inline child visibility hook preserves its owned input through collapse and live updates', async ({ page }) => {
  const original = await readFile(ticketPath(1));
  try {
    await ready(page);
    await toggleRow(page, 1).click();
    await page.getByRole('searchbox').fill('no matching title');
    await page.evaluate(async parent => {
      const { setInlineChildContext } = await import('/app.js');
      const mount = setInlineChildContext(parent);
      const input = document.createElement('input');
      input.setAttribute('aria-label', 'Hook draft');
      mount.append(input);
      input.focus();
    }, id(2));
    await page.getByLabel('Hook draft').fill('Keep this draft');
    await page.getByLabel('Hook draft').evaluate(node => node.setSelectionRange(3, 7));
    await expect(page.locator('#results')).toHaveText('0 of 40 items match');
    await expect(toggleRow(page, 1)).toBeDisabled();
    cli('update', id(1), '--title', 'Agent changes ancestor');
    await expect(listLink(page, 1)).toHaveText('Agent changes ancestor', { timeout: 2000 });
    await expect(page.getByLabel('Hook draft')).toHaveValue('Keep this draft');
    await expect(page.getByLabel('Hook draft')).toBeFocused();
    expect(await page.getByLabel('Hook draft').evaluate(node => [node.selectionStart, node.selectionEnd])).toEqual([3, 7]);
    await page.evaluate(async () => (await import('/app.js')).setInlineChildContext());
    await page.locator('#reset').click();
    await expect(toggleRow(page, 1)).toHaveAttribute('aria-expanded', 'false');
  } finally { await writeFile(ticketPath(1), original); }
});

test('Under and exact AND tags use globally completed prerequisites, including live cancellation recovery', async ({ page }) => {
  const originals = await Promise.all([6, 9, 10].map(n => readFile(ticketPath(n))));
  try {
    await ticket(9, 'Scoped ready', 'todo', `parent: ${id(2)}\ndepends_on: [${id(3)}]\nlabels: [web, Case]\n`);
    await ticket(10, 'Scoped waiting', 'todo', `parent: ${id(2)}\ndepends_on: [${id(6)}]\nlabels: [web, case]\n`);
    await ready(page, `#view=ready&under=${id(2)}&label=web`);
    await expect(listLink(page, 1)).toHaveCount(0); // Ancestors above Under are omitted.
    await expect(listLink(page, 2)).toHaveAccessibleDescription(/Context ancestor/);
    await expect(listLink(page, 9)).toBeVisible(); // Done prerequisite is outside Under.
    await expect(listLink(page, 10)).toHaveCount(0); // Canceled prerequisite still blocks.
    await expect(page.locator('#results')).toHaveText('1 of 40 items match');
    await page.getByRole('button', { name: 'Active', exact: true }).click();
    await page.locator('#label-filter summary').click();
    await page.getByLabel('Case', { exact: true }).check();
    await expect(listLink(page, 9)).toBeVisible();
    await expect(listLink(page, 10)).toHaveCount(0);
    await page.getByLabel('case', { exact: true }).check();
    await expect(page.locator('#items > li')).toHaveCount(0);
    await page.getByRole('button', { name: 'Remove tag filter Case', exact: true }).click();
    await expect(listLink(page, 10)).toBeVisible();
    await expect(listLink(page, 9)).toHaveCount(0);
    await page.getByRole('button', { name: 'Ready', exact: true }).click();
    await expect(page.locator('#items > li')).toHaveCount(0);
    cli('update', id(6), '--status', 'done');
    await expect(listLink(page, 10)).toBeVisible({ timeout: 2000 });
    await expect(page.locator('#results')).toHaveText('1 of 40 items match');
    // A fragment navigation can introduce a tag absent from the project. Adding
    // another checkbox must keep that existing, restrictive URL filter intact.
    await page.evaluate(() => { location.hash += '&label=missing'; });
    await expect(page.locator('#items > li')).toHaveCount(0);
    await page.locator('#label-filter summary').click();
    await expect(page.getByLabel('missing', { exact: true })).toBeChecked();
    await page.getByLabel('Case', { exact: true }).check();
    await expect(page).toHaveURL(/label=missing/);
    await expect(page.getByRole('button', { name: 'Remove tag filter missing', exact: true })).toBeVisible();
  } finally { await Promise.all([6, 9, 10].map((n, i) => writeFile(ticketPath(n), originals[i]))); }
});

const inlineForm = page => page.locator('.inline-child-form');
const rowStatus = (page, n) => page.locator(`[data-row-status="${id(n)}"]`);

// Reach controls through the real tab order; no focus(), click(), fill(), or
// selectOption() shortcuts in the integrated keyboard walkthrough below.
async function tabTo(page, target, backwards = false) {
  for (let step = 0; step < 250; step++) {
    if (await target.evaluate(node => node === document.activeElement)) return;
    await page.keyboard.press(backwards ? 'Shift+Tab' : 'Tab');
  }
  await expect(target).toBeFocused();
}

for (const width of [1440, 390]) test(`keyboard-only compact workflow at ${width}px persists through CLI and live updates`, async ({ page }) => {
  const original = await readFile(ticketPath(2)), created = [];
  try {
    await page.setViewportSize({ width, height: 900 });
    await ready(page);
    await tabTo(page, page.getByRole('searchbox'));
    await page.keyboard.type('Build');
    await page.keyboard.press('ControlOrMeta+A');
    await page.keyboard.press('Backspace');
    await tabTo(page, toggleRow(page, 1));
    await page.keyboard.press('Space');
    await expect(toggleRow(page, 1)).toHaveAttribute('aria-expanded', 'false');
    await page.keyboard.press('Enter');
    await expect(toggleRow(page, 1)).toHaveAttribute('aria-expanded', 'true');
    await tabTo(page, listLink(page, 2));
    await page.keyboard.press('Enter');
    const title = editForm(page).getByLabel('Title', { exact: true });
    await tabTo(page, title);
    await page.keyboard.press('ControlOrMeta+A');
    await page.keyboard.type('Canceled keyboard draft');
    await tabTo(page, editForm(page).getByRole('button', { name: 'Cancel', exact: true }));
    page.once('dialog', dialog => dialog.accept());
    await page.keyboard.press('Enter');
    await expect(title).toHaveValue('Build search');
    expect(cli('show', id(2)).ticket.title).toBe('Build search');
    await tabTo(page, title);
    await page.keyboard.press('ControlOrMeta+A');
    await page.keyboard.type('Build search keyboard');
    await tabTo(page, editForm(page).getByRole('button', { name: 'Edit description' }));
    await page.keyboard.press('Enter');
    await page.keyboard.press('ControlOrMeta+A');
    await page.keyboard.type('Keyboard description');
    await tabTo(page, editForm(page).getByRole('button', { name: 'Save changes' }));
    await page.keyboard.press('Enter');
    await expect(editForm(page).getByRole('button', { name: 'Save changes' })).toBeHidden();
    expect(cli('show', id(2)).source).toContain('Keyboard description');
    await tabTo(page, page.getByRole('button', { name: /Close|Back to table/ }), true);
    await page.keyboard.press('Enter');
    await expect(listLink(page, 2)).toBeFocused();

    await tabTo(page, page.getByRole('button', { name: 'New item', exact: true }), true);
    await page.keyboard.press('Enter');
    await expect(createForm(page).getByLabel('Title', { exact: true })).toBeFocused();
    await page.keyboard.press('Escape');
    await expect(page.getByRole('button', { name: 'New item', exact: true })).toBeFocused();
    await page.keyboard.press('Enter');
    await page.keyboard.type(`Keyboard modal ${width}`);
    await tabTo(page, createForm(page).getByRole('button', { name: 'Add tag', exact: true }));
    await page.keyboard.press('Enter');
    await page.keyboard.type('web');
    await page.keyboard.press('Enter');
    await tabTo(page, createForm(page).getByRole('button', { name: 'Parent item', exact: true }));
    await page.keyboard.press('Enter');
    await page.keyboard.type(id(1));
    await page.keyboard.press('Enter');
    await tabTo(page, createForm(page).getByRole('button', { name: 'Create', exact: true }));
    await page.keyboard.press('Tab');
    await expect(createForm(page).getByRole('button', { name: 'Close new item' })).toBeFocused();
    await page.keyboard.press('Shift+Tab');
    await expect(createForm(page).getByRole('button', { name: 'Create', exact: true })).toBeFocused();
    await page.keyboard.press('Enter');
    await expect(page.getByRole('dialog', { name: 'New item', exact: true })).toBeHidden();
    const modalItem = cli('list', '--all').tickets.find(item => item.title === `Keyboard modal ${width}`);
    created.push(modalItem.id);
    expect(cli('show', modalItem.id).ticket).toMatchObject({ parent: id(1), labels: ['web'], status: 'todo' });
    await tabTo(page, page.getByRole('button', { name: /Close|Back to table/ }));
    await page.keyboard.press('Enter');
    await tabTo(page, page.getByRole('searchbox'), true);
    await page.keyboard.type('Build search');
    await tabTo(page, rowStatus(page, 2));
    await page.keyboard.press('b');
    await page.keyboard.press('Enter');
    await expect(rowStatus(page, 2)).toHaveValue('blocked');
    await expect(rowStatus(page, 2)).toBeEnabled();
    expect(cli('show', id(2)).ticket.status).toBe('blocked');
    await expect(page.locator('#detail')).toBeHidden();
    await tabTo(page, page.locator(`[data-row-action="${id(2)}"]`));
    await page.keyboard.press('Enter');
    await tabTo(page, page.locator(`#row-menu-${id(2)}`).getByRole('button', { name: 'Add child', exact: true }));
    await page.keyboard.press('Enter');
    const childTitle = inlineForm(page).getByLabel('Title', { exact: true });
    await expect(childTitle).toBeFocused();
    await page.keyboard.type(`Keyboard child ${width}`);
    await page.keyboard.press('ArrowLeft');
    const caret = await childTitle.evaluate(input => input.selectionStart);
    cli('update', id(2), '--title', 'Build search agent');
    await expect(listLink(page, 2)).toHaveText('Build search agent', { timeout: 2000 });
    await expect(childTitle).toBeFocused();
    expect(await childTitle.evaluate(input => input.selectionStart)).toBe(caret);
    for (const childName of [`Keyboard child ${width}`, `Keyboard sibling ${width}`]) {
      if (childName.includes('sibling')) await page.keyboard.type(childName);
      await page.keyboard.press('Enter');
      await expect(childTitle).toHaveValue('');
      await expect(childTitle).toBeFocused();
      const child = cli('list', '--all').tickets.find(item => item.title === childName);
      created.push(child.id);
      expect(cli('show', child.id).ticket).toMatchObject({ parent: id(2), status: 'todo', labels: [] });
      await expect(page.locator(`#items a[data-item="${child.id}"]`)).toHaveAccessibleDescription(/Created in this session; outside the current filters/);
    }
    await page.keyboard.press('Escape');
    await expect(inlineForm(page)).toHaveCount(0);
    await expect(page.locator(`[data-row-action="${id(2)}"]`)).toBeFocused();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    cli('validate');
  } finally {
    for (const item of created) await rm(join(root, '.wrk', `${item}.md`), { force: true });
    await writeFile(ticketPath(2), original);
  }
});

async function addChild(page, n) {
  await page.locator(`[data-row-action="${id(n)}"]`).click();
  await page.locator(`#row-menu-${id(n)}`).getByRole('button', { name: 'Add child', exact: true }).click();
}

test('inline repeated children use defaults and explicit parents with confirmed, single submissions', async ({ page }, testInfo) => {
  const configPath = join(root, '.wrk/config.yaml'), config = await readFile(configPath);
  const created = [], payloads = [];
  let release;
  try {
    await writeFile(configPath, 'version: 1\nprefix: task\ndefaults:\n  priority: high\n  labels: [project-default]\n');
    await ready(page);
    await toggleRow(page, 2).click();
    await addChild(page, 2);
    const form = inlineForm(page), title = form.getByLabel('Title', { exact: true });
    await expect(title).toBeFocused();
    await expect(toggleRow(page, 2)).toHaveAttribute('aria-expanded', 'true');
    await expect(toggleRow(page, 2)).toBeDisabled();
    expect(await title.evaluate(input => getComputedStyle(input).borderTopWidth)).toBe('0px');
    await page.getByRole('searchbox').fill('Build search');
    await page.route('**/api/items', async route => {
      payloads.push(route.request().postDataJSON());
      const response = await route.fetch();
      created.push((await response.json()).result.ticket.id);
      if (payloads.length === 1) await new Promise(resolve => { release = resolve; });
      await route.fulfill({ response });
    });
    await title.fill('   '); await title.press('Enter');
    await expect(form.getByRole('button', { name: 'Create child', exact: true })).toBeDisabled();
    expect(payloads).toHaveLength(0);
    await title.fill('First sibling');
    // IME confirmation must remain a text-input action, not a child submission.
    expect(await title.evaluate(input => input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', isComposing: true, bubbles: true, cancelable: true })))).toBe(true);
    expect(payloads).toHaveLength(0);
    await title.press('Enter');
    await expect.poll(() => Boolean(release)).toBe(true);
    await form.evaluate(form => { form.requestSubmit(); form.requestSubmit(); });
    await page.keyboard.press('Enter');
    await expect(form.getByRole('button', { name: 'Cancel child creation' })).toBeDisabled();
    expect(payloads).toHaveLength(1);
    await expect(title).toHaveValue('First sibling');
    release();
    await expect(title).toHaveValue(''); await expect(title).toBeFocused();
    await expect(page.locator(`[data-row="${created[0]}"]`)).toContainText('created · outside filters');
    await expect(page.locator('#results')).toHaveText('1 of 41 items match');
    await title.fill('Second sibling');
    await form.getByRole('button', { name: 'Create child', exact: true }).click();
    await expect(title).toHaveValue(''); await expect(title).toBeFocused();
    expect(payloads).toEqual([{ title: 'First sibling', parent: id(2) }, { title: 'Second sibling', parent: id(2) }]);
    for (const child of created) expect(cli('show', child).ticket).toMatchObject({ parent: id(2), labels: ['project-default'], status: 'todo', priority: 'high' });
    await expect(page.locator('#detail')).toBeHidden();
    await page.screenshot({ path: testInfo.outputPath('inline-children-desktop.png'), fullPage: true });
    for (const width of [320, 390, 760]) {
      await page.setViewportSize({ width, height: 844 });
      expect(await page.locator('#list-scroll').evaluate(node => node.scrollWidth <= node.clientWidth)).toBe(true);
      await expect(title).toBeVisible();
      await page.screenshot({ path: testInfo.outputPath(`inline-children-${width}.png`), fullPage: true });
    }
    await title.press('Escape'); await expect(form).toHaveCount(0);
    for (const child of created) await expect(page.locator(`[data-row="${child}"]`)).toHaveCount(0);
    await page.getByRole('searchbox').fill('');
    await expect(toggleRow(page, 2)).toHaveAttribute('aria-expanded', 'false');
    cli('validate');
  } finally {
    release?.();
    for (const child of created) await rm(join(root, '.wrk', `${child}.md`), { force: true });
    await writeFile(configPath, config);
  }
});

test('inline draft survives live movement, status writes, filtering, deletion and explicit discard', async ({ page }) => {
  const original = await readFile(ticketPath(40));
  const created = [];
  try {
    await ready(page);
    await addChild(page, 40);
    const form = inlineForm(page), title = form.getByLabel('Title', { exact: true });
    await title.fill('Pinned child');
    await title.evaluate(input => { window.inlineTitle = input; input.setSelectionRange(2, 6); });
    cli('update', id(40), '--parent', id(7));
    await expect(page.locator(`[data-row="${id(40)}"]`)).toHaveAttribute('data-depth', '1', { timeout: 2000 });
    await expect(title).toBeFocused();
    expect(await title.evaluate(input => [input === window.inlineTitle, input.selectionStart, input.selectionEnd])).toEqual([true, 2, 6]);
    await rowStatus(page, 40).selectOption('done');
    await expect(page.locator('#save-notice')).toContainText('Status saved');
    await expect(title).toHaveValue('Pinned child');
    await expect(listLink(page, 40)).toHaveAccessibleDescription(/Context ancestor/);
    await page.getByRole('searchbox').fill('does not match');
    await expect(form).toBeVisible();
    await title.press('Enter');
    await expect(title).toHaveValue('');
    const child = cli('list', '--all').tickets.find(item => item.title === 'Pinned child').id; created.push(child);
    expect(cli('show', child).ticket.parent).toBe(id(40));
    cli('update', child, '--parent', id(1));
    await expect(page.locator(`[data-row="${child}"]`)).toBeVisible({ timeout: 2000 });
    await title.fill('Retain after deletion');
    await rm(ticketPath(40));
    await expect(form).toContainText('was removed', { timeout: 2000 });
    await expect(form.getByRole('button', { name: 'Create child', exact: true })).toBeDisabled();
    await expect(title).toHaveValue('Retain after deletion');
    await title.press('Escape'); await expect(form).toBeVisible();
    page.once('dialog', dialog => dialog.dismiss());
    await form.getByRole('button', { name: 'Cancel child creation' }).click();
    await expect(title).toHaveValue('Retain after deletion');
    await writeFile(ticketPath(40), original);
    await expect(form.getByRole('button', { name: 'Create child', exact: true })).toBeEnabled({ timeout: 2000 });
    page.once('dialog', dialog => dialog.accept());
    await form.getByRole('button', { name: 'Cancel child creation' }).click();
    await expect(form).toHaveCount(0);
    expect(cli('show', child).ticket.parent).toBe(id(1));
  } finally {
    for (const child of created) await rm(join(root, '.wrk', `${child}.md`), { force: true });
    await writeFile(ticketPath(40), original);
  }
});

test('inline creation shares the single draft guard with modal and detail text edits', async ({ page }) => {
  await ready(page, `#item=${id(2)}`);
  await editForm(page).getByLabel('Title', { exact: true }).fill('Keep detail text');
  await addChild(page, 40);
  await expect(inlineForm(page)).toHaveCount(0);
  await expect(editForm(page).getByLabel('Title', { exact: true })).toBeFocused();
  await cancelDraft(page);
  await page.getByRole('button', { name: 'New item', exact: true }).click();
  await createForm(page).getByLabel('Title', { exact: true }).fill('Keep modal text');
  // A programmatic row action is also guarded while a modal makes the table inert.
  await page.locator(`#row-menu-${id(40)}`).getByRole('button', { name: 'Add child', includeHidden: true }).evaluate(button => button.click());
  await expect(inlineForm(page)).toHaveCount(0);
  await expect(createForm(page).getByLabel('Title', { exact: true })).toHaveValue('Keep modal text');
  await cancelDraft(page);
  await addChild(page, 2);
  await inlineForm(page).getByLabel('Title', { exact: true }).fill('Keep child draft');
  await page.getByRole('button', { name: 'New item', exact: true }).click();
  await expect(createForm(page)).toHaveCount(0);
  await expect(inlineForm(page).getByLabel('Title', { exact: true })).toBeFocused();
  await addChild(page, 40);
  await expect(inlineForm(page)).toHaveAttribute('aria-label', `Add children to ${id(2)}`);
  await listLink(page, 7).click();
  await expect(editForm(page)).toHaveCount(0);
  await expect(inlineForm(page).getByLabel('Title', { exact: true })).toHaveValue('Keep child draft');
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(inlineForm(page)).toBeVisible();
  page.once('dialog', dialog => dialog.accept());
  await inlineForm(page).getByRole('button', { name: 'Cancel child creation' }).click();
  await expectTitle(page, 'Independent manual block');
});

test('inline validation, BUSY and missing-parent races preserve titles for explicit retry', async ({ page }) => {
  const original = await readFile(ticketPath(40));
  let requests = 0;
  try {
    await ready(page); await addChild(page, 40);
    const form = inlineForm(page), title = form.getByLabel('Title', { exact: true });
    await title.fill('Retain on failure');
    await page.route('**/api/items', async route => {
      requests++;
      if (requests === 1) return route.fulfill({ status: 422, json: { ok: false, errors: [{ code: 'INVALID_TITLE', field: 'title', message: 'Injected title validation' }] } });
      if (requests === 2) return route.fulfill({ status: 503, json: { ok: false, errors: [{ code: 'BUSY', message: 'Writer owns the project lock' }] } });
      await rm(ticketPath(40)); await route.continue();
    });
    await title.press('Enter');
    await expect(title).toHaveAttribute('aria-invalid', 'true');
    await expect(title).toHaveValue('Retain on failure');
    await title.press('Enter'); await expect(form).toContainText('BUSY');
    await expect(title).toHaveValue('Retain on failure');
    await title.press('Enter'); await expect(form).toContainText('was removed');
    await expect(form.locator('.edit-message')).toContainText('parent');
    await expect(title).toHaveValue('Retain on failure');
    expect(requests).toBe(3);
    expect(cli('list', '--all').tickets.some(item => item.title === 'Retain on failure')).toBe(false);
  } finally { await writeFile(ticketPath(40), original); }
});

test('inline lost and committed-error create responses require review without duplicate children', async ({ page }) => {
  const created = [];
  let requests = 0;
  try {
    await ready(page); await addChild(page, 2);
    const form = inlineForm(page), title = form.getByLabel('Title', { exact: true });
    await page.route('**/api/items', async route => {
      requests++;
      const response = await route.fetch(), data = await response.json(); created.push(data.result.ticket.id);
      if (requests === 1) return route.abort('failed');
      data.ok = false; data.errors = [{ code: 'DURABILITY_UNCERTAIN', message: 'Injected completion error' }];
      await route.fulfill({ status: 500, json: data });
    });
    for (const [n, text] of ['Lost inline response', 'Committed inline error'].entries()) {
      await title.fill(text); await title.press('Enter');
      await expect(form.locator('.edit-message')).toContainText(n === 0 ? 'outcome unknown' : 'completion reported an error');
      await expect(title).toHaveValue(text);
      await expect(form.getByRole('button', { name: 'Create child', exact: true })).toBeDisabled();
      await form.evaluate(form => { form.requestSubmit(); form.requestSubmit(); });
      expect(requests).toBe(n + 1);
      expect(cli('show', created[n]).ticket).toMatchObject({ title: text, parent: id(2) });
      await form.getByRole('button', { name: 'Review current items' }).click();
      await expect(form.locator('.edit-review')).toContainText(created[n]);
      await expect(form.locator('.edit-review')).toContainText('duplicate');
      page.once('dialog', dialog => dialog.accept());
      await form.getByRole('button', { name: 'Cancel child creation' }).click();
      if (n === 0) await addChild(page, 2);
    }
    expect(cli('list', '--all').tickets.filter(item => item.title === 'Lost inline response')).toHaveLength(1);
    cli('validate');
  } finally { for (const child of created) await rm(join(root, '.wrk', `${child}.md`), { force: true }); }
});

test('row status controls save all statuses without navigation and recover stale or lost writes', async ({ page }) => {
  const original = await readFile(ticketPath(40));
  let requests = 0;
  try {
    await ready(page, '#view=all');
    for (const value of ['in-progress', 'blocked', 'done', 'canceled', 'todo']) {
      await rowStatus(page, 40).selectOption(value);
      await expect(rowStatus(page, 40)).toBeEnabled();
      await expect(rowStatus(page, 40)).toHaveValue(value);
      expect(cli('show', id(40)).ticket.status).toBe(value);
      await expect(page.locator('#detail')).toBeHidden();
      expect(new URL(page.url()).hash).toBe('#view=all');
    }
    await page.route('**/api/items/*', async route => {
      requests++;
      if (requests === 1) cli('update', id(40), '--title', 'Agent before row write');
      if (requests === 3) { await route.fetch(); return route.abort('failed'); }
      await route.continue();
    });
    await rowStatus(page, 40).selectOption('done');
    const recovery = page.getByRole('region', { name: 'Row status change' });
    await expect(recovery).toContainText('CONFLICT');
    await expect(rowStatus(page, 40)).toHaveValue('todo');
    await recovery.getByRole('button', { name: 'Review current status' }).click();
    await recovery.getByRole('button', { name: 'Use reviewed revision' }).click();
    expect(requests).toBe(1);
    await recovery.getByRole('button', { name: 'Retry status' }).click();
    await expect(rowStatus(page, 40)).toBeEnabled();
    expect(cli('show', id(40)).ticket).toMatchObject({ title: 'Agent before row write', status: 'done' });
    await rowStatus(page, 40).selectOption('blocked');
    await expect(recovery).toContainText('outcome unknown');
    await expect(recovery.getByRole('button', { name: 'Retry status' })).toBeDisabled();
    await recovery.getByRole('button', { name: 'Review current status' }).click();
    await expect(recovery).toContainText('Current: Manually blocked');
    expect(requests).toBe(3);
    page.once('dialog', dialog => dialog.accept());
    await recovery.getByRole('button', { name: 'Discard status change' }).click();
    await expect(rowStatus(page, 40)).toBeEnabled();
    await expect(page.locator('#detail')).toBeHidden();
  } finally { await writeFile(ticketPath(40), original); }
});
