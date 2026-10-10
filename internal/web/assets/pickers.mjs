// Compact pickers own their input/selection DOM. Refresh only replaces suggestions.
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

function picker({ id, label, caption, options, allowNew = false, select, changed }) {
  const root = node('div');
  root.className = 'compact-picker';
  const trigger = button(caption, () => panel.hidden ? open() : close());
  trigger.setAttribute('aria-label', label);
  trigger.setAttribute('aria-expanded', 'false');
  const panel = node('div');
  panel.className = 'picker-panel';
  panel.hidden = true;
  panel.id = `${id}-panel`;
  trigger.setAttribute('aria-controls', panel.id);
  const input = node('input');
  input.type = 'text';
  input.id = id;
  input.autocomplete = 'off';
  input.placeholder = allowNew ? 'Find or add a tag…' : 'Search title or ID…';
  input.setAttribute('aria-label', allowNew ? 'Tag to add' : label === 'Parent item' ? 'Find parent item' : label);
  input.setAttribute('role', 'combobox');
  input.setAttribute('aria-autocomplete', 'list');
  input.setAttribute('aria-expanded', 'false');
  const list = node('div');
  list.id = `${id}-options`;
  list.className = 'picker-options';
  list.setAttribute('role', 'listbox');
  list.setAttribute('aria-label', label);
  input.setAttribute('aria-controls', list.id);
  const empty = node('p', 'No matching items.');
  empty.className = 'hint';
  panel.append(input, list, empty);
  root.append(trigger, panel);
  let matches = [], selected = 0, rendered = '';
  function pendingHint() {
    trigger.dataset.pending = String(Boolean(input.value) && panel.hidden);
    if (input.value) trigger.setAttribute('aria-description', `Pending input: ${input.value}`);
    else trigger.removeAttribute('aria-description');
  }
  function render() {
    pendingHint();
    const query = input.value.toLocaleLowerCase();
    const previous = matches[selected]?.value;
    matches = options().filter(option => `${option.label} ${option.detail || ''}`.toLocaleLowerCase().includes(query));
    // Keep DOM bounded for large projects; searching still covers the whole set.
    matches = matches.slice(0, 80);
    if (allowNew && input.value && !matches.some(option => option.value === input.value)) {
      matches.push({ value: input.value, label: `Add “${input.value}”` });
    }
    selected = Math.max(0, matches.findIndex(option => option.value === previous));
    const key = JSON.stringify(matches);
    if (key === rendered) { highlight(); return; }
    rendered = key;
    list.replaceChildren();
    matches.forEach((option, i) => {
      const entry = node('div');
      entry.id = `${id}-option-${i}`;
      entry.setAttribute('role', 'option');
      entry.append(node('span', option.label));
      if (option.detail) { const detail = node('small', option.detail); entry.append(detail); }
      // Retain input focus so the listbox has a single keyboard stop.
      entry.addEventListener('mousedown', event => event.preventDefault());
      entry.addEventListener('click', () => choose(option.value));
      list.append(entry);
    });
    empty.hidden = Boolean(matches.length);
    highlight();
  }
  function highlight() {
    [...list.children].forEach((entry, i) => entry.setAttribute('aria-selected', String(i === selected)));
    if (matches.length && !panel.hidden) input.setAttribute('aria-activedescendant', `${id}-option-${selected}`);
    else input.removeAttribute('aria-activedescendant');
  }
  function open() {
    // Only one picker is expanded in a surface; closing never clears its query.
    root.dispatchEvent(new CustomEvent('picker-open', { bubbles: true, detail: { close } }));
    panel.hidden = false;
    trigger.setAttribute('aria-expanded', 'true');
    input.setAttribute('aria-expanded', 'true');
    render(); input.focus();
  }
  function close(focus = true) {
    panel.hidden = true;
    trigger.setAttribute('aria-expanded', 'false');
    input.setAttribute('aria-expanded', 'false');
    input.removeAttribute('aria-activedescendant');
    pendingHint();
    if (focus) trigger.focus();
  }
  function choose(value) {
    input.value = '';
    select(value);
    // Publish selection intent before moving focus (blur may synchronously refresh defaults).
    changed(true); close();
  }
  function commit() {
    if (!input.value) return true;
    // A pending tag is literal. An item query must be selected or be an exact ID;
    // even a missing ID goes through the authoritative server validation.
    if (allowNew || /^[a-z][a-z0-9]{0,15}-[a-f0-9]{8}$/.test(input.value)) {
      choose(input.value); return true;
    }
    open();
    return false;
  }
  input.addEventListener('input', () => { selected = 0; matches = []; render(); changed(false); });
  root.addEventListener('keydown', event => {
    if (event.isComposing) return;
    if (event.key === 'Escape' && !panel.hidden) {
      event.preventDefault(); event.stopPropagation(); close();
    } else if (event.target === input && !event.ctrlKey && !event.metaKey) {
      if (['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) {
        event.preventDefault();
        if (event.key === 'Home') selected = 0;
        else if (event.key === 'End') selected = matches.length - 1;
        else selected = (selected + (event.key === 'ArrowDown' ? 1 : -1) + matches.length) % (matches.length || 1);
        highlight(); list.children[selected]?.scrollIntoView({ block: 'nearest' });
      } else if (event.key === 'Enter') {
        event.preventDefault(); event.stopPropagation();
        if (matches[selected]) choose(matches[selected].value);
        else commit();
      }
    }
  });
  return { root, trigger, input, open, close, commit, refresh: render,
    pending: () => input.value, clear() { input.value = ''; render(); } };
}

export function createTagPicker({ id, initial = [], getSuggestions, onChange }) {
  let values = [...initial];
  const root = node('div');
  root.className = 'compact-tags';
  const chips = node('div');
  chips.className = 'compact-chips';
  const menu = picker({ id, label: 'Add tag', caption: '+ Add tag', allowNew: true,
    options: () => [...new Set(getSuggestions())].sort().filter(value => !values.includes(value)).map(value => ({ value, label: value })),
    select(value) { if (!values.includes(value)) values.push(value); render(); }, changed: onChange });
  function render() {
    chips.replaceChildren();
    for (const value of values) {
      const chip = node('span');
      chip.className = 'compact-chip';
      const remove = button('×', () => {
        values = values.filter(entry => entry !== value); render(); menu.refresh(); onChange(true); menu.trigger.focus();
      });
      remove.setAttribute('aria-label', `Remove tag ${value}`);
      chip.append(node('span', value), remove); chips.append(chip);
    }
  }
  root.append(chips, menu.root);
  render();
  return { ...menu, root, get: () => [...values], set(next) { if (JSON.stringify(values) === JSON.stringify(next)) return; values = [...next]; render(); menu.refresh(); } };
}

export function createItemPicker({ id, initial = '', getItems, onChange, exclude = '', label = 'Parent item', caption = '+ Add parent', removeLabel = 'Remove parent' }) {
  let value = initial;
  const root = node('div');
  root.className = 'compact-parent';
  const menu = picker({ id, label, caption,
    options: () => getItems().filter(item => item.id !== exclude).map(item => ({ value: item.id, label: item.title, detail: item.id })),
    select(next) { value = next; render(); }, changed: onChange });
  const remove = button('×', () => { value = ''; menu.clear(); render(); onChange(true); menu.trigger.focus(); });
  remove.setAttribute('aria-label', removeLabel);
  function render() {
    const item = getItems().find(item => item.id === value);
    menu.trigger.textContent = value ? `${item?.title || 'Missing item'} · ${value}` : caption;
    menu.trigger.title = menu.trigger.textContent;
    remove.hidden = !value;
  }
  root.append(menu.root, remove);
  render();
  return { ...menu, root, get: () => value, set(next, { preservePending = false } = {}) { value = next; if (!preservePending) menu.clear(); render(); }, refresh() { render(); menu.refresh(); } };
}
