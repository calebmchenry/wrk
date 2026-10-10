import { publishItem } from './editor.mjs';

const node = (tag, text, className = '') => {
  const result = document.createElement(tag);
  if (text !== undefined) result.textContent = text;
  result.className = className;
  return result;
};
const button = (text, action) => {
  const result = node('button', text);
  result.type = 'button'; result.addEventListener('click', action);
  return result;
};

// One persistent, title-only draft. Published siblings belong to the project;
// only their temporary visibility pins belong to this creation session.
export function createInlineChildren({ drafts, getData, setContext, refresh, onCreated, onChange, announce, itemLink, restoreFocus }) {
  let active;
  const dirty = () => Boolean(active && (active.title.value || active.saving || active.uncertain));
  const focus = () => { active?.title.focus(); active?.title.scrollIntoView({ block: 'nearest' }); };
  window.addEventListener('beforeunload', event => {
    if (dirty()) { event.preventDefault(); event.returnValue = ''; }
  });
  function cancel() {
    if (!active || active.saving || active.reviewing) return;
    if (dirty() && !window.confirm('Discard this unsaved child draft? Created siblings will remain. A submitted create may already have been published.')) return;
    const parent = active.parent;
    active.dispose(); active.mount.replaceChildren(); active = undefined;
    setContext(); onChange(); restoreFocus(parent);
  }
  function open(parent) {
    if (active) { announce('Finish or cancel the current child draft first.'); focus(); return; }
    if (getData().stale || !getData().index?.byID.has(parent)) return;
    const mount = setContext(parent);
    const form = node('form', undefined, 'inline-child-form');
    form.setAttribute('aria-label', `Add children to ${parent}`);
    const title = node('input'); title.type = 'text'; title.required = true; title.maxLength = 4096;
    title.setAttribute('aria-label', 'Title'); title.placeholder = 'Child title'; title.autocomplete = 'off';
    const save = node('button', '✓'); save.type = 'submit'; save.setAttribute('aria-label', 'Create child');
    const close = button('×', cancel); close.setAttribute('aria-label', 'Cancel child creation');
    const controls = node('div', undefined, 'inline-child-controls'); controls.append(title, save, close);
    const context = node('p', '', 'hint'); context.setAttribute('role', 'status');
    const message = node('p', '', 'edit-message'); message.id = 'inline-child-error'; message.setAttribute('role', 'alert');
    title.setAttribute('aria-describedby', message.id);
    const review = node('section', undefined, 'edit-review');
    const reviewButton = button('Review current items', reviewCurrent); reviewButton.hidden = true;
    form.append(controls, context, message, reviewButton, review); mount.replaceChildren(form);
    const s = active = { parent, title, mount, created: [], saving: false, reviewing: false, uncertain: false };
    function render() {
      const data = getData(), parentItem = data.index?.byID.get(parent);
      context.textContent = data.stale ? 'Project data is stale. Your child draft is retained; reconnect before creating.' :
        !parentItem ? `Parent ${parent} was removed. Restore it or cancel this draft; no root item will be created.` :
          `Adding children to ${parentItem.title} · ${parent}`;
      title.disabled = close.disabled = s.saving || s.reviewing;
      save.disabled = s.saving || s.reviewing || s.uncertain || data.stale || !parentItem || !title.value.trim();
      reviewButton.disabled = s.saving || s.reviewing;
      form.setAttribute('aria-busy', String(s.saving));
    }
    s.dispose = drafts.register({ id: '', isDirty: dirty, isSaving: () => s.saving, onRemote: render });
    title.addEventListener('input', () => { render(); onChange(); });
    form.addEventListener('keydown', event => {
      if (event.key === 'Escape') {
        event.preventDefault(); event.stopPropagation();
        // A nonempty draft requires the explicit cancel action, not Escape.
        if (!dirty()) cancel(); else announce('Use Cancel child creation to discard this draft.');
      }
      if (event.key === 'Enter' && event.target === title && !event.isComposing) {
        event.preventDefault();
        if (!event.repeat) form.requestSubmit();
      }
    });
    async function reviewCurrent() {
      s.reviewing = true; render(); await refresh(); s.reviewing = false; render();
      review.replaceChildren();
      if (getData().stale) { message.textContent = 'Unable to read current items. Reconnect, then review again.'; return; }
      review.append(node('p', 'The previous create may have succeeded. Inspect the current items, including items reparented by an agent. Cancel if your child exists; another create can produce a duplicate.'));
      const list = node('ul');
      for (const item of getData().index.items) {
        const row = node('li'); row.append(itemLink({ ...item, title: `${item.id} · ${item.title}` })); list.append(row);
      }
      review.append(list, button('I checked the items; enable another create', () => {
        s.uncertain = false; review.replaceChildren();
        message.textContent = 'Another create is enabled. Submit only if the previous request did not create your child.';
        render(); onChange();
      }));
    }
    form.addEventListener('submit', async event => {
      event.preventDefault();
      if (save.disabled || !title.value.trim()) return;
      s.saving = true; render(); onChange(); review.replaceChildren(); message.textContent = 'Creating…';
      title.removeAttribute('aria-invalid');
      try {
        // Omit every optional default: no inheritance from the parent or filters.
        const { confirmed, result } = await publishItem({ payload: { title: title.value, parent } });
        if (confirmed) {
          const ticket = result.result.ticket;
          s.created.push(ticket.id); onCreated(ticket); setContext(parent, s.created);
          title.value = ''; message.textContent = ''; reviewButton.hidden = true;
          announce(`Created ${ticket.id}. Add another child under the same parent.`);
          await refresh();
        } else {
          message.textContent = result.errors.map(error => `${error.code}${error.field ? ` [${error.field}]` : ''}: ${error.message}`).join('\n');
          if (result.errors.some(error => error.field === 'title')) title.setAttribute('aria-invalid', 'true');
          if (result.result?.publication === 'committed') {
            s.uncertain = true;
            message.textContent = `Published ${result.result.ticket.id}, but completion reported an error. Review current items before another create.\n${message.textContent}`;
          }
          reviewButton.hidden = false; await refresh();
        }
      } catch {
        s.uncertain = true; reviewButton.hidden = false;
        message.textContent = 'Create outcome unknown: the response was lost or unreadable. Your title is retained. Review current items before another create; the request may already have been published.';
        await refresh();
      } finally {
        s.saving = false; render(); onChange(); focus();
      }
    });
    render(); onChange(); focus();
  }
  return { open, cancel, focus, dirty, isOpen: () => Boolean(active) };
}
