import { test, expect } from '@playwright/test';
import { mkdtemp, mkdir, writeFile, rm, readFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
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
  binary = join(root, 'wrk');
  execFileSync('go', ['build', '-o', binary, './cmd/wrk']);
  server = spawn(binary, ['--project', root, 'serve', '--port', '0', '--json']);
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
const detailTitle = page => page.locator('#detail .detail-title');

test('views, exact labels, body search and parent focus intersect', async ({ page }) => {
  await ready(page);
  await expect(page.locator('#project')).toHaveText(root);
  await expect(listLink(page, 3)).toHaveCount(0);
  await expect(listLink(page, 6)).toHaveCount(0);
  await expect(listLink(page, 5)).toContainText(`Waiting on ${id(6)} (canceled)`);
  await page.getByRole('button', { name: 'Ready', exact: true }).click();
  await expect(listLink(page, 5)).toHaveCount(0);
  await expect(listLink(page, 4)).toHaveCount(0);
  await page.locator('#focus').selectOption(id(1));
  await page.locator('#label-filter summary').click();
  await page.getByLabel('web', { exact: true }).check();
  await page.getByLabel('backend', { exact: true }).check();
  await page.getByRole('searchbox').fill('NEEDLE');
  await expect(page.locator('#items > li')).toHaveCount(1);
  await expect(listLink(page, 2)).toBeVisible();
  await page.getByRole('searchbox').fill('');
  await page.getByRole('button', { name: 'Blocked', exact: true }).click();
  await expect(page.locator('#items > li')).toHaveCount(1);
  await expect(listLink(page, 4)).toBeVisible();
  await page.getByRole('button', { name: 'Reset filters' }).click();
  await page.getByRole('button', { name: 'All', exact: true }).click();
  await page.locator('#focus').selectOption(id(1));
  await expect(page.locator('#items > li')).toHaveCount(6);
  await expect(listLink(page, 1)).toContainText('1/4 children done');
  await expect(listLink(page, 6)).toContainText('Canceled');
});

test('relationships, Markdown links and browser history retain filters and scroll', async ({ page }) => {
  await ready(page, '#view=all&label=web');
  await page.getByRole('link', { name: 'Skip to workspace' }).focus();
  await page.keyboard.press('Enter');
  await expect(page.locator('#list-scroll')).toBeFocused();
  await expect(page).toHaveURL(/label=web/);
  await listLink(page, 1).click();
  await expect(detailTitle(page)).toHaveText('Build a local workspace');
  await page.getByRole('button', { name: 'Focus this item + descendants' }).click();
  await expect(page.locator('#focus')).toHaveValue(id(1));
  await page.locator('.markdown').getByRole('link', { name: 'Build search' }).click();
  await expect(detailTitle(page)).toHaveText('Build search');
  await expect(page).toHaveURL(/label=web/);
  await expect(page.locator('#detail')).toContainText('0/1 children done');
  await page.locator('#detail').getByRole('link', { name: 'Deep descendant' }).click();
  await expect(detailTitle(page)).toHaveText('Deep descendant');
  await page.locator('#detail').getByRole('link', { name: 'Build search' }).click();
  await expect(detailTitle(page)).toHaveText('Build search');
  const dependency = page.locator('#detail section').filter({ has: page.getByRole('heading', { name: 'Dependencies', exact: true }) });
  await dependency.getByRole('link', { name: 'Confirm contracts' }).click();
  await expect(detailTitle(page)).toHaveText('Confirm contracts');
  await expect(page.locator('#detail')).toContainText('outside the current list filters');
  await expect(page.locator('#detail')).toContainText('Build search');
  await page.goBack();
  await expect(detailTitle(page)).toHaveText('Build search');
  await page.goForward();
  await expect(detailTitle(page)).toHaveText('Confirm contracts');
  await page.reload();
  await expect(detailTitle(page)).toHaveText('Confirm contracts');
  await page.getByRole('button', { name: 'Reset filters' }).click();
  await listLink(page, 30).click();
  await expect(detailTitle(page)).toHaveText('Review integration 30');
  const scroll = await page.locator('#list-scroll').evaluate(node => node.scrollTop);
  expect(scroll).toBeGreaterThan(0);
  await page.getByRole('button', { name: 'Back to list' }).click();
  await expect.poll(() => page.locator('#list-scroll').evaluate(node => node.scrollTop)).toBe(scroll);
  await page.goBack();
  await expect(detailTitle(page)).toHaveText('Review integration 30');
  await expect.poll(() => page.locator('#list-scroll').evaluate(node => node.scrollTop)).toBe(scroll);
});

test('untrusted content stays inert, images never load, custom YAML is preserved', async ({ page }, testInfo) => {
  const external = [], errors = [];
  page.on('request', request => { if (!request.url().startsWith(base)) external.push(request.url()); });
  page.on('pageerror', error => errors.push(error.message));
  await ready(page, `#item=${id(1)}`);
  await expect(detailTitle(page)).toHaveText('Build a local workspace');
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
  await expect(detailTitle(page)).toHaveText('<img src=x onerror=alert(1)>');
  await expect(page.locator('#detail img')).toHaveCount(0);
});

test('missing selection and missing focus recover without resetting context', async ({ page }) => {
  await ready(page, '#q=Build&item=task-deadbeef');
  await expect(detailTitle(page)).toHaveText('Item not found');
  await expect(page.getByRole('searchbox')).toHaveValue('Build');
  await listLink(page, 1).click();
  await expect(detailTitle(page)).toHaveText('Build a local workspace');
  await page.goto(base + '#focus=task-deadbeef');
  await expect(page.locator('#empty')).toContainText('Focus item not found');
  await page.getByRole('button', { name: 'Reset filters' }).click();
  await expect(listLink(page, 1)).toBeVisible();
  await page.getByRole('searchbox').fill('does not exist');
  await expect(page.locator('#empty')).toContainText('No matching items');
});

test('loading, validation failures, recovery, and an empty project are explicit', async ({ page }) => {
  let release;
  await page.route('**/api/workspace', async route => { await new Promise(resolve => { release = resolve; }); await route.continue(); });
  await page.goto(base);
  await expect(page.locator('#status')).toHaveText('Loading project…');
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
  await expect(detailTitle(page)).toHaveText('Build search');
  await expect(page.locator('#browse')).toBeHidden();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({ path: testInfo.outputPath('narrow.png'), fullPage: true });
  await page.getByRole('button', { name: 'Back to list' }).click();
  await expect(page.getByRole('searchbox')).toHaveValue('Build');
  await expect(listLink(page, 2)).toBeFocused();
  await page.goBack();
  await expect(detailTitle(page)).toHaveText('Build search');
});

test('a stale detail response cannot overwrite the new selection', async ({ page }) => {
  await ready(page);
  let release;
  await page.route(`**/api/workspace?selected=${id(1)}`, async route => { await new Promise(resolve => { release = resolve; }); await route.continue(); });
  await listLink(page, 1).click();
  await expect(page.locator('#detail')).toContainText('Loading item…');
  await expect.poll(() => Boolean(release)).toBe(true);
  await listLink(page, 2).click();
  await expect(detailTitle(page)).toHaveText('Build search');
  release();
  await expect(detailTitle(page)).toHaveText('Build search');
  await expect(page.locator('#status')).toContainText('Live');
});


const cli = (...args) => JSON.parse(execFileSync(binary, ['--project', root, ...args, '--json'], { encoding: 'utf8' })).result;
const currentDetail = page => page.locator('#detail-content');

test('separate CLI create/status/labels/parent/dependencies/body changes refresh within two seconds', async ({ page }) => {
  await ready(page, '#q=Live');
  let created;
  try {
    created = cli('new', 'Live agent item', '--label', 'live').ticket.id;
    const link = page.locator(`#items a[data-item="${created}"]`);
    await expect(link).toBeVisible({ timeout: 2000 });
    await link.click();
    await expect(detailTitle(page)).toHaveText('Live agent item');
    for (const status of ['in-progress', 'done']) {
      cli('update', created, '--status', status);
      await expect(currentDetail(page).locator(`.badges > .status-${status}`).first()).toBeVisible({ timeout: 2000 });
    }
    await expect(link).toHaveCount(0);
    await expect(currentDetail(page)).toContainText('outside the current list filters');
    await expect(page).toHaveURL(new RegExp(`item=${created}`));
    await expect(page.getByRole('searchbox')).toHaveValue('Live');
    cli('update', created, '--status', 'todo', '--add-label', 'reviewed', '--parent', id(1), '--add-dependency', id(6));
    await expect(link).toBeVisible({ timeout: 2000 });
    await expect(currentDetail(page).locator('.badges').first()).toContainText('reviewed');
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
    await expect(detailTitle(page)).toHaveText('Item not found', { timeout: 2000 });
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
  await expect(detailTitle(page)).toHaveText('Build a local workspace');
  await page.getByText('Custom fields & original metadata (YAML)', { exact: true }).click();
  const revision = await page.evaluate(async selected => {
    const { drafts } = await import('/live.mjs');
    const response = await (await fetch(`/api/items/${selected}`)).json();
    const revision = response.result.ticket.revision;
    const input = document.createElement('textarea');
    input.setAttribute('aria-label', 'Draft body');
    input.value = 'My unsaved work';
    document.querySelector('#draft').append(input);
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
    document.querySelector('#draft').append(input);
    drafts.register({ id: selected, revision: data.result.ticket.revision, isDirty: () => true, onRemote: data => { window.draftContext = data; } });
  }, id(40));
  try {
    cli('update', id(40), '--title', 'Changed externally');
    await expect(page.locator('#draft-notice')).toContainText('changed externally', { timeout: 2000 });
    await expect(page.locator('#draft-notice')).toContainText('outside the current list filters');
    await expect(detailTitle(page)).toHaveText('Changed externally');
    await expect(page.getByLabel('Unsaved title')).toHaveValue('Keep my title');
    await rm(ticketPath(40));
    await expect(detailTitle(page)).toHaveText('Item not found', { timeout: 2000 });
    await expect(page.locator('#draft-notice')).toContainText('removed');
    await expect(page.getByLabel('Unsaved title')).toHaveValue('Keep my title');
  } finally { await writeFile(ticketPath(40), original); }
});

test('offline, dropped responses and resume signals resynchronize a full snapshot', async ({ page, context }) => {
  const original = await readFile(ticketPath(40));
  await ready(page, `#item=${id(40)}`);
  await expect(detailTitle(page)).toHaveText('Review integration 40');
  const headers = [];
  page.on('request', request => { if (request.url().includes('/api/workspace')) headers.push(request.headers()); });
  try {
    await context.setOffline(true);
    await expect(page.locator('#status')).toContainText('Reconnecting', { timeout: 2000 });
    cli('update', id(40), '--title', 'Changed while offline');
    headers.length = 0;
    await context.setOffline(false);
    await expect(detailTitle(page)).toHaveText('Changed while offline', { timeout: 2000 });
    await expect(page.locator('#status')).toContainText('Live');
    expect(headers[0]['if-none-match']).toBeUndefined();
    let dropped = false;
    await page.route('**/api/workspace?*', route => {
      if (!dropped) { dropped = true; return route.abort('failed'); }
      return route.continue();
    });
    await expect(page.locator('#status')).toContainText('Reconnecting', { timeout: 2000 });
    cli('update', id(40), '--title', 'After dropped response');
    await expect(detailTitle(page)).toHaveText('After dropped response', { timeout: 2000 });
    await page.unroute('**/api/workspace?*');
    // BFCache/page suspension: stopped timers cannot miss edits after resume.
    await page.evaluate(() => window.dispatchEvent(new PageTransitionEvent('pagehide')));
    cli('update', id(40), '--title', 'Changed while asleep');
    headers.length = 0;
    await page.evaluate(() => window.dispatchEvent(new PageTransitionEvent('pageshow', { persisted: true })));
    await expect(detailTitle(page)).toHaveText('Changed while asleep', { timeout: 2000 });
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
    await expect(detailTitle(page)).toHaveText('Changed during initial load', { timeout: 2000 });
    await page.unroute('**/api/workspace?*');
    await page.evaluate(() => { window.savedDetail = document.querySelector('.detail-title'); });
    await writeFile(join(root, '.wrk/.wrk-stage-ignore.md'), 'invalid staging contents');
    await writeFile(join(root, '.wrk/runtime.log'), 'runtime contents');
    await page.waitForResponse(response => response.url().includes('/api/workspace') && response.status() === 304);
    expect(await page.evaluate(() => window.savedDetail === document.querySelector('.detail-title'))).toBe(true);
  } finally { release?.(); await writeFile(ticketPath(40), original); }
});
