import { createTagPicker, createItemPicker } from './pickers.mjs';
import { draftPatch, publishItem } from './editor.mjs';

const node = (tag, text, className = '') => {
  const result = document.createElement(tag);
  if (text !== undefined) result.textContent = text;
  result.className = className;
  return result;
};
const button = (text, action) => {
  const result = node('button', text);
  result.type = 'button';
  result.addEventListener('click', action);
  return result;
};
const names = { todo: 'Todo', 'in-progress': 'In progress', blocked: 'Manually blocked', done: 'Done', canceled: 'Canceled' };
const equal = (a, b) => JSON.stringify(a) === JSON.stringify(b);
export const detailValues = item => ({ title: item.title, body: item.body.replace(/\r\n?/g, '\n'), status: item.status,
  labels: [...item.labels], parent: item.parent || '', depends_on: [...item.depends_on], related: [...item.related] });

// A successful response is the sole authority for advancing a local draft. Never
// use a subsequent GET here: an agent may already have written again by then.
export function confirmedBaseline(base, payload, ticket) {
  return { ...detailValues({ ...ticket, body: base.body }), ...('body' in payload ? { body: payload.body } : {}) };
}

export function reconcileDetail(base, draft, reviewed) {
  const next = { ...reviewed };
  for (const key of ['title', 'body', 'status', 'parent']) if (!equal(base[key], draft[key])) next[key] = draft[key];
  for (const key of ['labels', 'depends_on', 'related']) {
    next[key] = reviewed[key].filter(value => !base[key].includes(value) || draft[key].includes(value));
    for (const value of draft[key]) if (!base[key].includes(value) && !next[key].includes(value)) next[key].push(value);
  }
  return next;
}

export function createDetailEditor({ mount, statusMount, drafts, getData, refresh, onChange, announce, itemLink, modalOpen, creationOpen = modalOpen, reveal }) {
  let session, quick;
  const dirty = () => Boolean(session && (session.saving || session.reviewing || session.uncertain || session.pending() || Object.keys(draftPatch(session.base, session.values())).length));
  function release() {
    session?.dispose?.(); session = undefined; mount.replaceChildren(); onChange();
  }
  // Shared by New and inline creation: never replace another dirty draft.
  function yieldDraft() {
    if (quick) { announce('Resolve the pending row status change first.'); statusMount.querySelector('button')?.focus(); return false; }
    if (dirty()) { announce('Save or cancel the current draft before starting another item.'); reveal(session.id); session.title.focus(); return false; }
    release(); return true;
  }
  window.addEventListener('beforeunload', event => {
    if (dirty() || quick) { event.preventDefault(); event.returnValue = ''; }
  });
  function sync(item, html = '') {
    if (creationOpen() || quick) return;
    if (session && session.id === item?.id) {
      // A pristine document is a live view, not a registered draft. Once editing
      // starts, even reverting text doesn't adopt remotely supplied revisions.
      if (!session.dispose && !dirty()) session.load(item);
      session.preview(html, item);
      session.refresh();
      return;
    }
    if (dirty()) return;
    release();
    if (item) open(item, html);
  }
  function open(item, html) {
    let base = detailValues(item), revision = item.revision, relatedRevision = item.related_revision;
    let editingBody = false;
    const form = node('form', undefined, 'document-form');
    form.setAttribute('aria-label', 'Edit item');
    const identity = node('p', item.id, 'item-id draft-identity');
    const controls = {}, feedback = {}, retries = {};
    function field(key, label, tag = 'input') {
      const control = node(tag);
      control.id = `detail-${key}`;
      if (tag === 'input') control.type = 'text';
      const caption = node('label', label, 'sr-only');
      caption.htmlFor = control.id;
      controls[key] = control;
      form.append(caption, control);
      return control;
    }
    form.append(identity);
    const title = field('title', 'Title');
    title.className = 'document-title'; title.required = true; title.maxLength = 4096;
    const metadata = node('fieldset', undefined, 'document-metadata');
    const legend = node('legend', 'Item metadata', 'sr-only');
    metadata.append(legend);
    const status = node('select');
    status.setAttribute('aria-label', 'Status'); controls.status = status;
    for (const [value, label] of Object.entries(names)) status.add(new Option(label, value));
    status.addEventListener('change', () => { changed(); persistField('status'); });
    const labels = createTagPicker({ id: 'detail-tags', initial: base.labels,
      getSuggestions: () => getData().index.items.flatMap(item => item.labels),
      onChange(committed) { changed(); if (committed) persistField('labels'); } });
    const parent = createItemPicker({ id: 'detail-parent', initial: base.parent, exclude: item.id,
      getItems: () => getData().index.items,
      onChange(committed) { changed(); if (committed) persistField('parent'); } });
    controls.labels = labels.trigger; controls.parent = parent.trigger;
    const parentLink = node('span', undefined, 'parent-navigation');
    function localFeedback(key, root) {
      const wrapper = node('div', undefined, `metadata-field field-${key}`);
      feedback[key] = node('span', '', 'field-feedback'); feedback[key].setAttribute('role', 'status');
      retries[key] = button(`Retry ${key === 'labels' ? 'tags' : key === 'depends_on' ? 'dependencies' : key}`, () => persistField(key));
      retries[key].hidden = true;
      wrapper.append(root, feedback[key], retries[key]);
      return wrapper;
    }
    metadata.append(localFeedback('status', status), localFeedback('labels', labels.root), localFeedback('parent', parent.root), parentLink);
    form.append(metadata);
    const body = field('body', 'Description (Markdown)', 'textarea');
    body.rows = 7; body.placeholder = 'Add a description…';
    const markdown = node('div', undefined, 'markdown');
    const editBody = button('Edit description', () => { editingBody = true; renderBody(); body.focus(); });
    const bodySection = node('section', undefined, 'document-description');
    bodySection.append(body.previousSibling, body, markdown, editBody);
    form.append(bodySection);
    const actions = node('div', undefined, 'edit-actions');
    const save = node('button', 'Save changes'); save.type = 'submit';
    const cancel = button('Cancel', () => {
      if (s.saving || s.reviewing) return;
      if (dirty() && !window.confirm('Discard unsaved changes? Confirmed metadata saves will remain. A submitted save may already have been published.')) return;
      release(); onChange(); refresh();
    });
    actions.append(save, cancel);
    const discard = button('Discard pending changes', () => cancel.click());
    const remote = node('p', '', 'hint'); remote.setAttribute('role', 'status');
    const message = node('p', '', 'edit-message'); message.setAttribute('role', 'alert');
    const reviewButton = button('Review current version', reviewCurrent); reviewButton.hidden = true;
    const review = node('section', undefined, 'edit-review');
    form.append(actions, discard, remote, message, reviewButton, review);
    const collections = {};
    function collection(key, heading, addLabel) {
      let values = [...base[key]];
      const part = node('section', undefined, 'detail-section relationship-editor');
      part.append(node('h3', heading));
      const list = node('ul', undefined, 'relations');
      const picker = createItemPicker({ id: `detail-${key}`, exclude: item.id, label: addLabel, caption: `+ ${addLabel}`,
        getItems: () => getData().index.items.filter(item => !values.includes(item.id)),
        onChange(committed) {
          if (committed && picker.get()) { values.push(picker.get()); picker.set(''); render(); changed(); persistField(key); }
          else changed();
        } });
      controls[key] = picker.input;
      function render() {
        const keyNow = JSON.stringify(values.map(id => [id, getData().index.byID.get(id)?.title, getData().index.byID.get(id)?.status]));
        if (list.dataset.key === keyNow) return;
        list.dataset.key = keyNow; list.replaceChildren();
        for (const id of values) {
          const row = node('li');
          const target = getData().index.byID.get(id) || { id, title: id };
          const remove = button('×', () => {
            values = values.filter(value => value !== id); render(); changed(); persistField(key);
          });
          remove.setAttribute('aria-label', `Remove ${key === 'related' ? 'related ' : ''}${id}`);
          row.append(itemLink(target), node('span', names[target.status] || 'Missing', `badge status-${target.status}`), remove);
          list.append(row);
        }
      }
      const fields = node('fieldset'); fields.append(list, picker.root);
      part.append(localFeedback(key, fields)); form.append(part); render();
      return { get: () => [...values], set(next) { values = [...next]; render(); }, picker, fields, refresh() { render(); picker.refresh(); } };
    }
    collections.depends_on = collection('depends_on', 'Dependencies', 'Add dependency');
    collections.related = collection('related', 'Related items', 'Add related item');
    let openPicker;
    form.addEventListener('picker-open', event => { if (openPicker !== event.detail.close) openPicker?.(false); openPicker = event.detail.close; });
    function values() { return { title: title.value, body: body.value, status: status.value, labels: labels.get(), parent: parent.get(),
      depends_on: collections.depends_on.get(), related: collections.related.get() }; }
    function setValues(next, preservePending = false) {
      title.value = next.title; body.value = next.body; status.value = next.status;
      labels.set(next.labels); parent.set(next.parent, { preservePending });
      for (const key of ['depends_on', 'related']) collections[key].set(next[key]);
    }
    function pending() { return labels.pending() || parent.pending() || Object.values(collections).some(collection => collection.picker.pending()); }
    const s = { id: item.id, title, base, values, pending, saving: false, uncertain: false, reviewing: false,
      load(current) { base = detailValues(current); s.base = base; revision = current.revision; relatedRevision = current.related_revision; setValues(base); },
      preview(html, current) {
        if (current.id !== s.id) return;
        // Server-rendered, restricted Markdown is the only HTML accepted here.
        if (markdown.dataset.html !== html) { markdown.innerHTML = html; markdown.dataset.html = html; }
        if (!current.body.trim()) markdown.textContent = 'No description yet.';
      }, refresh() {
        labels.refresh(); parent.refresh(); Object.values(collections).forEach(collection => collection.refresh());
        const target = getData().index.byID.get(parent.get());
        if (parentLink.dataset.id !== target?.id || parentLink.textContent !== target?.title) {
          parentLink.replaceChildren(); if (target) parentLink.append(itemLink(target)); parentLink.dataset.id = target?.id || '';
        }
        renderState();
      }, persist, setStatus(value) { status.value = value; changed(); } };
    session = s;
    setValues(base); s.preview(html, item); mount.replaceChildren(form);
    function register() {
      s.dispose?.();
      s.dispose = drafts.register({ id: s.id, revision, relatedRevision, isDirty: dirty, isSaving: () => s.saving, onRemote(context) {
        remote.textContent = context.stale ? 'Current project data is stale. Your input is retained; reconnect before saving.' :
          context.missing ? 'This item was removed. Your draft is retained.' :
            context.changed && !s.saving ? 'This item or its related links changed externally. Review current version to reconcile before saving.' : '';
        if (context.changed || context.missing) reviewButton.hidden = false;
        renderState();
      } });
    }
    function changed() {
      if (!s.dispose) register();
      renderState(); onChange();
    }
    function renderBody() {
      body.hidden = !editingBody && body.value === base.body;
      markdown.hidden = !body.hidden; editBody.hidden = !body.hidden;
    }
    body.addEventListener('blur', () => { if (body.value === base.body) { editingBody = false; renderBody(); } });
    function renderState() {
      title.title = title.value;
      const textDirty = title.value !== base.title || body.value !== base.body;
      actions.hidden = !textDirty;
      discard.hidden = textDirty || !s.dispose;
      discard.disabled = s.saving || s.reviewing;
      cancel.hidden = false;
      save.disabled = !title.value.trim() || !textDirty || s.saving || s.reviewing || s.uncertain || getData().stale;
      cancel.disabled = s.saving || s.reviewing;
      const disabled = s.saving || s.reviewing || s.uncertain || getData().stale;
      metadata.disabled = disabled;
      Object.values(collections).forEach(collection => { collection.fields.disabled = disabled; });
      Object.values(retries).forEach(retry => { retry.disabled = disabled; });
      reviewButton.disabled = s.saving || s.reviewing;
      renderBody();
    }
    form.addEventListener('input', changed);
    form.addEventListener('keydown', event => {
      if (event.key === 'Enter' && !event.isComposing && event.target.tagName === 'INPUT') event.preventDefault();
      if (event.key === 'Enter' && (event.ctrlKey || event.metaKey)) { event.preventDefault(); form.requestSubmit(); }
    });
    function showErrors(errors, field) {
      form.querySelectorAll('.field-error').forEach(node => node.remove());
      Object.values(controls).forEach(control => { control.removeAttribute('aria-invalid'); control.removeAttribute('aria-describedby'); });
      message.textContent = errors.map(error => `${error.code}${error.field ? ` [${error.field}]` : ''}: ${error.message}`).join('\n');
      for (const error of errors) {
        const key = ['add_dependencies', 'remove_dependencies'].includes(error.field) ? 'depends_on' :
          ['add_related', 'remove_related', 'expected_related_revision'].includes(error.field) ? 'related' :
            error.field === 'expected_revision' || !error.field ? field : error.field;
        const control = controls[key];
        if (!control) continue;
        const detail = node('p', error.message, 'field-error'); detail.id = `detail-error-${key}`;
        (control.closest('.compact-tags, .compact-parent') || control).after(detail); control.setAttribute('aria-invalid', 'true'); control.setAttribute('aria-describedby', detail.id);
      }
    }
    function patchFor(key) {
      const patch = draftPatch(base, values());
      const keys = key === 'depends_on' ? ['add_dependencies', 'remove_dependencies'] : key === 'related' ? ['add_related', 'remove_related'] : [key];
      return Object.fromEntries(Object.entries(patch).filter(([name]) => keys.includes(name)));
    }
    async function persistField(key) {
      const patch = patchFor(key);
      if (Object.keys(patch).length) await persist(patch, key);
      else {
        feedback[key].textContent = ''; retries[key].hidden = true;
        controls[key].removeAttribute('aria-invalid'); controls[key].removeAttribute('aria-describedby');
        form.querySelectorAll(`#detail-error-${key}`).forEach(error => error.remove());
        if (!form.querySelector('.field-error')) message.textContent = '';
      }
    }
    async function persist(patch, field = 'title') {
      if (s.saving || s.reviewing || s.uncertain || getData().stale) return false;
      if (!s.dispose) register();
      const restoreFocus = field === 'labels' ? labels.trigger : field === 'parent' ? parent.trigger : collections[field]?.picker.trigger || controls[field];
      const payload = { ...patch, expected_revision: revision, expected_related_revision: relatedRevision };
      s.saving = true; showErrors([], field); review.replaceChildren();
      if (feedback[field]) feedback[field].textContent = 'Saving…';
      renderState(); onChange();
      try {
        const { confirmed, result } = await publishItem({ id: s.id, payload });
        if (confirmed) {
          // Retain edits typed while this request was in flight. Apply only the
          // confirmed response baseline; pending unrelated fields keep their intent.
          const before = base, now = values();
          base = confirmedBaseline(before, payload, result.result.ticket); s.base = base;
          revision = result.result.ticket.revision; relatedRevision = result.result.ticket.related_revision;
          for (const key of ['status', 'labels', 'parent', 'depends_on', 'related']) {
            if (key === field || equal(now[key], before[key])) now[key] = base[key];
          }
          status.value = now.status; labels.set(now.labels);
          if (parent.get() !== now.parent) parent.set(now.parent);
          for (const key of ['depends_on', 'related']) collections[key].set(now[key]);
          register();
          if (feedback[field]) { feedback[field].textContent = 'Saved'; retries[field].hidden = true; }
          message.textContent = '';
          if ('title' in patch || 'body' in patch) announce(`Saved ${s.id}.`);
          if ('body' in payload && body.value === base.body) editingBody = false;
          // A clean confirmed session may return to live viewing after refresh.
          if (!Object.keys(draftPatch(base, values())).length && !pending()) { s.dispose(); s.dispose = null; }
          await refresh(); return true;
        }
        showErrors(result.errors, field);
        if (feedback[field]) { feedback[field].textContent = 'Not saved'; retries[field].hidden = false; }
        if (result.result?.publication === 'committed') {
          s.uncertain = true;
          message.textContent = `Published changes for ${s.id}, but completion reported an error. Review all affected items before retrying.\n` +
            (result.result.updates || []).map(entry => `${entry.ticket.id}: ${entry.publication}`).join('\n') + '\n' + message.textContent;
        }
        reviewButton.hidden = false; await refresh();
      } catch {
        s.uncertain = true; reviewButton.hidden = false;
        if (feedback[field]) { feedback[field].textContent = 'Outcome unknown'; retries[field].hidden = false; }
        message.textContent = 'Save outcome unknown: the response was lost or unreadable. Your draft is retained. Review current state before another save; the request may already have been published.';
        await refresh();
      } finally {
        s.saving = false; renderState(); onChange();
        if (document.activeElement === document.body && restoreFocus?.isConnected && mount.getClientRects().length) restoreFocus.focus({ preventScroll: true });
      }
      return false;
    }
    form.addEventListener('submit', event => {
      event.preventDefault();
      const patch = { ...patchFor('title'), ...patchFor('body') };
      if (title.value.trim() && Object.keys(patch).length) persist(patch, 'title' in patch ? 'title' : 'body');
    });
    async function reviewCurrent() {
      s.reviewing = true; renderState(); await refresh(); s.reviewing = false; renderState();
      if (session !== s) return;
      review.replaceChildren();
      if (getData().stale) { message.textContent = 'Unable to read current state. Keep this draft and retry review after reconnecting.'; return; }
      const current = getData().index.byID.get(s.id);
      if (!current) { review.append(node('p', 'The item no longer exists. Copy your draft before canceling.')); return; }
      const reviewed = detailValues(current);
      review.append(node('h3', 'Review before retrying'), node('h4', 'Your draft'), node('pre', JSON.stringify(values(), null, 2)), node('h4', 'Current saved version'), node('pre', JSON.stringify(reviewed, null, 2)));
      function accept(keep) {
        if (!keep && dirty() && !window.confirm('Replace your draft with the reviewed saved values?')) return;
        const next = keep ? reconcileDetail(base, values(), reviewed) : reviewed;
        base = reviewed; s.base = base; revision = current.revision; relatedRevision = current.related_revision; s.uncertain = false;
        setValues(next, keep);
        if (!keep) { labels.clear(); Object.values(collections).forEach(collection => collection.picker.clear()); }
        register(); showErrors([]); review.replaceChildren();
        message.textContent = keep ? 'Draft kept against the reviewed version. Save text or retry changed metadata explicitly.' : 'Loaded the reviewed saved values.';
        for (const key of Object.keys(retries)) { retries[key].hidden = !Object.keys(patchFor(key)).length; feedback[key].textContent = retries[key].hidden ? '' : 'Not saved'; }
        if (!keep) editingBody = false;
        renderState(); onChange();
      }
      review.append(button('Keep draft using reviewed revision', () => accept(true)), button('Reload saved values', () => accept(false)));
    }
    s.refresh();
  }
  // Row controls must call this same coordinator. A same-item draft uses its
  // captured revision, and successful own writes advance it without changing text.
  async function mutate(item, patch) {
    if (modalOpen()) return false;
    if (quick || getData().stale) return false;
    if (session?.id !== item.id) {
      if (!yieldDraft()) return false;
      // A row-only write does not acquire the text-draft slot. This lets a
      // creation session keep its form and pins while status moves outside filters.
      return quickStatus(item, patch.status);
    }
    if (session.saving || session.uncertain || session.reviewing || getData().stale) return false;
    if ('status' in patch) session.setStatus(patch.status);
    const saved = await session.persist(patch, 'status');
    if (saved) announce(`Status saved for ${item.id}.`);
    else announce('Status not saved. Review the retained detail draft to recover.');
    return saved;
  }
  async function quickStatus(item, status) {
    const s = quick = { item, status, saving: false, uncertain: false, reviewing: false };
    const heading = node('p', `Status for ${item.id}: ${names[status]}`);
    const message = node('p', '', 'edit-message'); message.setAttribute('role', 'status');
    const review = node('div');
    const retry = button('Retry status', persist);
    const reviewButton = button('Review current status', async () => {
      s.reviewing = true; render(); await refresh(); s.reviewing = false; render();
      if (getData().stale) { message.textContent = 'Unable to read current status. Reconnect and review again.'; return; }
      const current = getData().index.byID.get(item.id);
      review.replaceChildren();
      if (!current) { message.textContent = 'The item was removed. Restore it or discard the pending status change.'; return; }
      review.append(node('p', `Current: ${names[current.status]}. Requested: ${names[status]}.`),
        button('Use reviewed revision', () => {
          s.item = current; s.uncertain = false; review.replaceChildren();
          message.textContent = 'Reviewed revision accepted. Retry status explicitly to apply the requested value.'; render();
        }));
    });
    const discard = button('Discard status change', () => {
      if (s.saving || s.reviewing || !window.confirm('Discard the pending status change? A submitted change may already have been published.')) return;
      quick = undefined; statusMount.replaceChildren(); onChange(); refresh();
    });
    statusMount.replaceChildren(heading, message, retry, reviewButton, discard, review);
    function render() {
      retry.hidden = s.saving;
      retry.disabled = s.saving || s.reviewing || s.uncertain || getData().stale;
      reviewButton.disabled = discard.disabled = s.saving || s.reviewing;
      reviewButton.hidden = s.saving;
      onChange();
    }
    s.refresh = () => { retry.disabled = s.saving || s.reviewing || s.uncertain || getData().stale; };
    async function persist() {
      if (s.saving || s.reviewing || s.uncertain || getData().stale) return false;
      s.saving = true; message.textContent = 'Saving…'; review.replaceChildren(); render();
      let saved = false;
      try {
        const { confirmed, result } = await publishItem({ id: item.id, payload: { status,
          expected_revision: s.item.revision, expected_related_revision: s.item.related_revision } });
        if (confirmed) {
          saved = true;
          announce(`Status saved for ${item.id}.`);
        } else {
          message.textContent = result.errors.map(error => `${error.code}: ${error.message}`).join('\n');
          if (result.result?.publication === 'committed') {
            s.uncertain = true; message.textContent = 'Published with a completion error. Review current status before retrying.\n' + message.textContent;
          }
        }
      } catch {
        s.uncertain = true;
        message.textContent = 'Status outcome unknown: the response was lost or unreadable. Review current status before retrying; the change may already have been published.';
      }
      await refresh();
      s.saving = false;
      if (saved) { quick = undefined; statusMount.replaceChildren(); await refresh(); onChange(); }
      else render();
      return saved;
    }
    return persist();
  }
  return { sync, dirty, yieldDraft, mutate, id: () => session?.id, release,
    statusBusy: () => Boolean(quick || session?.saving || session?.reviewing || session?.uncertain),
    refreshStatus: () => quick?.refresh?.(),
  };
}
