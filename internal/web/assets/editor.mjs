import { createTagPicker, createItemPicker } from './pickers.mjs';

// Form state and the original revision live here, independently of polling DOM.
const node = (tag, text) => {
  const result = document.createElement(tag);
  if (text !== undefined) result.textContent = text;
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
const normalizeBody = text => text.replace(/\r\n?/g, '\n');
const valuesOf = item => ({ title: item.title, body: normalizeBody(item.body), status: item.status,
  labels: [...item.labels], parent: item.parent || '', depends_on: [...item.depends_on], related: [...(item.related || [])] });

// Only changed fields are sent: untouched CRLF bodies and YAML list order survive.
export function draftPatch(base, values) {
  const patch = {};
  for (const key of ['title', 'body', 'status', 'labels', 'parent']) {
    if (!equal(base[key], values[key])) patch[key] = values[key];
  }
  const add = values.depends_on.filter(id => !base.depends_on.includes(id));
  const remove = base.depends_on.filter(id => !values.depends_on.includes(id));
  if (add.length) patch.add_dependencies = add;
  if (remove.length) patch.remove_dependencies = remove;
  const addRelated = (values.related || []).filter(id => !(base.related || []).includes(id));
  const removeRelated = (base.related || []).filter(id => !(values.related || []).includes(id));
  if (addRelated.length) patch.add_related = addRelated;
  if (removeRelated.length) patch.remove_related = removeRelated;
  return patch;
}

// Shared one-shot publication hook. Callers own duplicate-submit and uncertain-save
// guards; transport/unrecognized responses throw and must never trigger auto retry.
export async function publishItem({ id = '', payload }) {
  const response = await fetch(id ? `/api/items/${id}` : '/api/items', {
    method: id ? 'PATCH' : 'POST', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload), signal: AbortSignal.timeout(20000),
  });
  const result = await response.json();
  const confirmed = response.ok && result.ok && ['committed', 'unchanged'].includes(result.result?.publication) && Boolean(result.result?.ticket?.id);
  if (!confirmed && (!Array.isArray(result.errors) || !result.errors.length)) throw new Error('Unrecognized save response');
  return { confirmed, result };
}

export function createEditor({ mount, modal, drafts, getData, refresh, onSaved, onToggle, onChange, announce }) {
  let active;
  function dirty() { return Boolean(active && (active.saving || active.uncertain || !equal(active.initial, active.values()) || active.pending())); }
  window.addEventListener('beforeunload', event => {
    if (dirty()) { event.preventDefault(); event.returnValue = ''; }
  });
  function close() {
    const session = active;
    session?.dispose?.();
    if (modal.open) modal.close();
    active = undefined;
    mount.replaceChildren();
    modal.replaceChildren();
    onToggle(false);
    onChange();
    if (session?.opener?.isConnected) session.opener.focus({ preventScroll: true });
  }
  function cancel() {
    if (active?.saving) return;
    if (dirty() && !window.confirm('Discard this unsaved draft? A submitted save may already have been published.')) return;
    close();
  }
  modal.addEventListener('cancel', event => { event.preventDefault(); cancel(); });
  modal.addEventListener('keydown', event => {
    if (event.key !== 'Tab') return;
    const stops = [...modal.querySelectorAll('a[href], button, input, textarea, select, [tabindex]')]
      .filter(control => control.tabIndex >= 0 && !control.matches(':disabled') && control.getClientRects().length);
    const target = event.shiftKey ? stops.at(-1) : stops[0];
    if (!stops.length || document.activeElement === (event.shiftKey ? stops[0] : stops.at(-1))) {
      event.preventDefault(); target?.focus();
    }
  });
  function open(item = null, quickStatus) {
    if (active) { active.title.focus(); return; }
    const data = getData();
    if (!data.index || data.stale) return;
    const opener = document.activeElement;
    announce('');
    const creating = !item;
    const form = node('form');
    form.setAttribute('aria-label', creating ? 'Create item' : 'Edit item');
    const heading = node('h2', creating ? 'New item' : `Edit ${item.id}`);
    if (creating) { heading.className = 'sr-only'; form.append(heading); }
    else form.append(heading, node('p', 'Changes stay in this draft until you save.'));
    const fields = node('fieldset');
    const legend = node('legend', 'Item fields');
    legend.className = 'sr-only';
    fields.append(legend);
    const controls = {};
    function field(key, label, tag = 'input') {
      const input = node(tag);
      input.id = `edit-${key}`;
      if (tag === 'input') input.type = 'text';
      const caption = node('label', label);
      caption.htmlFor = input.id;
      if (creating) caption.className = 'sr-only';
      fields.append(caption, input);
      controls[key] = input;
      return input;
    }
    const title = field('title', 'Title');
    title.required = true;
    title.maxLength = 4096;
    const body = field('body', 'Description (Markdown)', 'textarea');
    body.rows = creating ? 5 : 8;
    if (creating) { title.placeholder = 'Title'; title.autofocus = true; body.placeholder = 'Add a description…'; }
    const status = field('status', 'Status', 'select');
    for (const [value, name] of Object.entries(names)) status.add(new Option(name, value));
    if (creating) { status.value = 'todo'; status.disabled = true; status.hidden = true; }
    let useDefaultLabels = creating;
    const labelSuggestions = node('datalist');
    labelSuggestions.id = 'edit-label-suggestions';
    for (const value of creating ? [] : [...new Set(data.index.items.flatMap(item => item.labels))].sort()) labelSuggestions.append(new Option(value, value));
    const itemSuggestions = node('datalist');
    itemSuggestions.id = 'edit-item-suggestions';
    for (const suggestion of creating ? [] : data.index.items.filter(other => other.id !== item?.id)) itemSuggestions.append(new Option(`${suggestion.id} · ${suggestion.title}`, suggestion.id));
    fields.append(labelSuggestions, itemSuggestions);
    function collection(key, label, listID, initial) {
      let values = [...initial];
      const input = field(key, label);
      input.setAttribute('list', listID);
      input.autocomplete = 'off';
      const list = node('ul');
      list.className = 'edit-chips';
      const add = button(key === 'labels' ? 'Add label' : key === 'related' ? 'Add related item' : 'Add dependency', commit);
      fields.append(add, list);
      function render() {
        list.replaceChildren();
        for (const value of values) {
          const row = node('li');
          row.append(node('span', value), button(key === 'related' ? `Remove related ${value}` : `Remove ${value}`,  () => {
            values = values.filter(entry => entry !== value); if (key === 'labels') useDefaultLabels = false; render(); changed();
          }));
          list.append(row);
        }
      }
      function commit() {
        if (!input.value) return;
        if (!values.includes(input.value)) values.push(input.value);
        input.value = '';
        if (key === 'labels') useDefaultLabels = false;
        render(); changed();
      }
      input.addEventListener('keydown', event => { if (event.key === 'Enter') { event.preventDefault(); commit(); } });
      render();
      return { input, get: () => [...values], set(next) { values = [...next]; input.value = ''; render(); }, commit };
    }
    let labels, parentPicker, parent, dependencies, related;
    const emptyCollection = () => ({ input: { value: '' }, get: () => [], set() {}, commit() {} });
    const defaultsHint = node('span', 'Project default tags');
    defaultsHint.className = 'hint default-tags';
    if (creating) {
      labels = createTagPicker({ id: 'create-tags', initial: data.project.config.defaults.labels || [],
        getSuggestions: () => [...getData().index.items.flatMap(item => item.labels), ...(getData().project.config.defaults.labels || [])],
        onChange(committed) {
          // Typing alone keeps defaults live; only committed collection edits freeze them.
          if (committed) useDefaultLabels = false;
          defaultsHint.hidden = !useDefaultLabels || !labels.get().length;
          changed();
        } });
      parentPicker = createItemPicker({ id: 'create-parent', getItems: () => getData().index.items, onChange: changed });
      controls.labels = labels.trigger;
      controls.parent = parentPicker.trigger;
      const chips = node('div');
      chips.className = 'create-metadata';
      chips.append(labels.root, defaultsHint, parentPicker.root);
      defaultsHint.hidden = !labels.get().length;
      fields.append(chips);
      dependencies = emptyCollection(); related = emptyCollection();
      let openPicker;
      fields.addEventListener('picker-open', event => {
        if (openPicker !== event.detail.close) openPicker?.(false);
        openPicker = event.detail.close;
      });
    } else {
      labels = collection('labels', 'Label to add', labelSuggestions.id, item.labels);
      parent = field('parent', 'Parent item');
      parent.setAttribute('list', itemSuggestions.id);
      parent.placeholder = 'None — enter or choose an item ID';
      dependencies = collection('depends_on', 'Dependency to add', itemSuggestions.id, item.depends_on);
      related = collection('related', 'Related item to add', itemSuggestions.id, item.related || []);
      fields.append(node('p', 'Related items provide context. Add or remove a link from either item.'));
    }
    form.append(fields);
    const message = node('p');
    message.className = 'edit-message';
    message.setAttribute('role', 'alert');
    const remote = node('p');
    remote.className = 'hint';
    remote.setAttribute('role', 'status');
    const review = node('section');
    review.className = 'edit-review';
    const reviewButton = button('Review current version', reviewCurrent);
    reviewButton.hidden = creating;
    const save = node('button', creating ? 'Create' : 'Save changes');
    if (creating) save.setAttribute('aria-keyshortcuts', 'Control+Enter Meta+Enter');
    save.type = 'submit';
    const actions = node('div');
    actions.className = 'edit-actions';
    const cancelButton = button(creating ? '×' : 'Cancel', cancel);
    if (creating) {
      cancelButton.className = 'modal-close';
      cancelButton.setAttribute('aria-label', 'Close new item');
      form.prepend(cancelButton);
      const shortcut = node('span', '⌘/Ctrl + Enter to create');
      shortcut.className = 'hint';
      actions.append(shortcut, save, reviewButton);
    } else actions.append(save, cancelButton, reviewButton);
    form.append(remote, message, actions, review);
    let base = creating ? { title: '', body: '', status: 'todo', labels: labels.get(), parent: '', depends_on: [], related: [] } : valuesOf(item);
    function setValues(values) {
      for (const key of ['title', 'body', 'status']) controls[key].value = values[key];
      if (creating) parentPicker.set(values.parent); else parent.value = values.parent;
      labels.set(values.labels); dependencies.set(values.depends_on); related.set(values.related);
    }
    setValues(base);
    function values() { return { title: title.value, body: body.value, status: status.value, labels: labels.get(), parent: creating ? parentPicker.get() : parent.value, depends_on: dependencies.get(), related: related.get(), defaults: useDefaultLabels }; }
    active = { id: item?.id || '', revision: item?.revision, relatedRevision: item?.related_revision, form, title, opener, saving: false, uncertain: false, values,
      pending: () => Boolean(labels.input.value || parentPicker?.pending() || dependencies.input.value || related.input.value), initial: values() };
    const session = active;
    function register() {
      session.dispose?.();
      session.dispose = drafts.register({ id: session.id, revision: session.revision, relatedRevision: session.relatedRevision, isDirty: dirty, onRemote(context) {
        if (creating && !context.stale) {
          if (useDefaultLabels) {
            const next = getData().project.config.defaults.labels || [];
            labels.set(next); base.labels = [...next]; session.initial.labels = [...next];
            defaultsHint.hidden = !next.length;
          }
          labels.refresh(); parentPicker.refresh();
        }
        remote.textContent = context.stale ? 'Current project data is stale. Your input is retained; reconnect before saving.' :
          !creating && context.missing ? 'This item was removed. Your draft is retained.' :
            context.changed ? 'This item or its related links changed externally. Saving with the original revision will be rejected. Review current version to reconcile.' : '';
        save.disabled = session.saving || session.reviewing || session.uncertain || context.stale;
      } });
    }
    register();
    function changed() { if (active === session) onChange(); }
    form.addEventListener('input', changed);
    form.addEventListener('change', changed);
    function showErrors(errors) {
      form.querySelectorAll('.field-error').forEach(node => node.remove());
      for (const control of Object.values(controls)) { control.removeAttribute('aria-invalid'); control.removeAttribute('aria-describedby'); }
      message.id = 'edit-errors';
      message.textContent = errors.map(error => `${error.code}${error.field ? ` [${error.field}]` : ''}: ${error.message}${error.ids?.length ? ` (${error.ids.join(', ')})` : ''}`).join('\n');
      for (const error of errors) {
        const control = controls[error.field === 'add_dependencies' || error.field === 'remove_dependencies' ? 'depends_on' : ['add_related', 'remove_related', 'expected_related_revision'].includes(error.field) ? 'related' : error.field];
        if (control) {
          const detail = node('p', error.message);
          detail.className = 'field-error';
          detail.id = `edit-error-${error.field}`;
          (control.closest('.compact-tags, .compact-parent') || control).after(detail);
          control.setAttribute('aria-invalid', 'true'); control.setAttribute('aria-describedby', detail.id);
        }
      }
    }
    async function reviewCurrent() {
      labels.commit(); dependencies.commit(); related.commit();
      session.reviewing = true;
      changed();
      reviewButton.disabled = true;
      await refresh();
      reviewButton.disabled = false;
      if (active !== session) return;
      session.reviewing = false;
      changed();
      const currentData = getData();
      review.replaceChildren();
      if (currentData.stale) { message.textContent = 'Unable to read current state. Keep this draft and retry review after reconnecting.'; return; }
      review.append(node('h3', creating ? 'Review current items before creating again' : 'Review before retrying'));
      if (creating) {
        review.append(node('p', 'The previous create may have succeeded. Inspect current items for the saved draft; cancel if it already exists. Another create can produce a duplicate.'));
        const list = node('ul');
        for (const current of currentData.index.items) {
          const entry = node('li');
          const link = node('a', `${current.id} · ${current.title}`);
          link.href = `#item=${encodeURIComponent(current.id)}`;
          link.dataset.item = current.id;
          entry.append(link); list.append(entry);
        }
        review.append(list, button('I checked the items; enable another create', () => {
          session.uncertain = false; message.textContent = 'Another create is enabled. Save only if the previous request did not create your item.'; changed(); review.replaceChildren();
        }));
        return;
      }
      const current = currentData.index.byID.get(session.id);
      if (!current) { review.append(node('p', 'The item no longer exists. Copy your draft before canceling.')); return; }
      const reviewed = valuesOf(current);
      review.append(node('h4', 'Your draft'), node('pre', JSON.stringify(values(), null, 2)), node('h4', 'Current saved version'), node('pre', JSON.stringify(reviewed, null, 2)));
      function accept(keep) {
        if (!keep && dirty() && !window.confirm('Replace your draft with the reviewed saved values?')) return;
        labels.commit(); dependencies.commit(); related.commit();
        const edited = draftPatch(base, values());
        const next = { ...reviewed };
        if (keep) {
          for (const key of ['title', 'body', 'status', 'parent']) if (key in edited) next[key] = edited[key];
          // Reconcile collection intent so unrelated agent additions survive.
          const removedLabels = base.labels.filter(label => !values().labels.includes(label));
          next.labels = reviewed.labels.filter(label => !removedLabels.includes(label));
          for (const label of values().labels) if (!base.labels.includes(label) && !next.labels.includes(label)) next.labels.push(label);
          next.depends_on = reviewed.depends_on.filter(id => !edited.remove_dependencies?.includes(id));
          for (const id of edited.add_dependencies || []) if (!next.depends_on.includes(id)) next.depends_on.push(id);
          next.related = reviewed.related.filter(id => !edited.remove_related?.includes(id));
          for (const id of edited.add_related || []) if (!next.related.includes(id)) next.related.push(id);
        }
        base = reviewed;
        session.initial = { ...reviewed, defaults: false };
        session.revision = current.revision;
        session.relatedRevision = current.related_revision;
        session.uncertain = false;
        useDefaultLabels = false;
        setValues(next);
        register();
        review.replaceChildren();
        message.textContent = keep ? 'Draft kept against the reviewed version. Review your fields, then save explicitly.' : 'Loaded the reviewed saved values.';
        changed();
      }
      review.append(button('Keep draft using reviewed revision', () => accept(true)), button('Reload saved values', () => accept(false)));
    }
    form.addEventListener('submit', async event => {
      event.preventDefault();
      if (session.saving || session.reviewing || session.uncertain || getData().stale) return;
      if (creating && !parentPicker.commit()) {
        showErrors([{ code: 'INVALID_PARENT', field: 'parent', message: 'Choose a parent from the results, enter its exact ID, or clear the search.' }]);
        return;
      }
      labels.commit(); dependencies.commit(); related.commit();
      const valuesNow = values();
      const payload = creating ? { title: valuesNow.title, body: valuesNow.body, parent: valuesNow.parent, depends_on: valuesNow.depends_on, related: valuesNow.related,
        ...(valuesNow.defaults ? {} : { labels: valuesNow.labels }) } : { ...draftPatch(base, valuesNow), expected_revision: session.revision, expected_related_revision: session.relatedRevision };
      // A no-op still checks the original revision under the writer lock.
      if (!creating && Object.keys(payload).length === 2) payload.title = valuesNow.title;
      session.saving = true;
      fields.disabled = true;
      save.disabled = cancelButton.disabled = reviewButton.disabled = true;
      review.replaceChildren();
      message.textContent = 'Saving…';
      changed();
      try {
        const { confirmed, result } = await publishItem({ id: session.id, payload });
        if (confirmed) {
          const savedID = result.result.ticket.id;
          close();
          announce(result.result.publication === 'unchanged' ? 'No changes needed; current revision confirmed.' : `Saved ${savedID}.`);
          await onSaved(savedID);
          return;
        }
        showErrors(result.errors);
        if (result.result?.publication === 'committed') {
          session.uncertain = true;
          const affected = result.result.updates?.map(entry => `${entry.ticket.id}: ${entry.publication}`).join('\n') || '';
          message.textContent = affected + '\n' + `Published changes for ${result.result.ticket.id}, but completion reported an error. Inspect all affected items before retrying.\n` + message.textContent;
        }
        reviewButton.hidden = false;
        await refresh();
      } catch {
        session.uncertain = true;
        message.textContent = 'Save outcome unknown: the response was lost or unreadable. Your draft is retained. Review current state before another save; the request may already have been published.';
        reviewButton.hidden = false;
        await refresh();
      } finally {
        if (active === session) {
          session.saving = false;
          fields.disabled = false;
          if (creating) status.disabled = true;
          cancelButton.disabled = reviewButton.disabled = false;
          changed();
        }
      }
    });
    if (creating) {
      form.className = 'create-form';
      // Plain Enter never implicitly creates; textarea Enter remains a newline.
      form.addEventListener('keydown', event => {
        if (event.key === 'Escape') {
          const expanded = fields.querySelector('.picker-panel:not([hidden])');
          if (expanded) {
            event.preventDefault(); event.stopPropagation();
            const trigger = expanded.previousElementSibling;
            labels.close(false); parentPicker.close(false); trigger.focus();
          }
        }
        if (event.key !== 'Enter' || event.isComposing) return;
        if (event.ctrlKey || event.metaKey) { event.preventDefault(); form.requestSubmit(); }
        else if (event.target.tagName === 'INPUT') event.preventDefault();
      });
      modal.replaceChildren(form);
      modal.showModal();
    } else mount.replaceChildren(form);
    onToggle(true, creating);
    onChange();
    title.focus();
    if (quickStatus) { status.value = quickStatus; form.requestSubmit(); }
  }
  return { open, dirty, isOpen: () => Boolean(active), cancel };
}
