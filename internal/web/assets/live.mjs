// Full-snapshot polling: no event cursor, event history, or subscription race.
// Timers/read callbacks are injectable so lifecycle tests need no wall-clock sleeps.
export function createPoller({ read, onData, onError, onChecking = () => {},
  schedule = setTimeout, cancel = clearTimeout, now = Date.now, interval = 750 }) {
  let timer, controller, generation = 0, etag = '', stopped = true;
  async function poll(force = false) {
    const version = ++generation;
    controller?.abort();
    controller = new AbortController();
    const signal = controller.signal;
    if (force) etag = '';
    onChecking(force);
    try {
      const result = await read({ signal, etag });
      if (stopped || version !== generation) return;
      // Only a successfully applied full snapshot establishes a validator.
      onData(result.data);
      etag = result.etag || '';
    } catch (error) {
      if (stopped || version !== generation) return;
      etag = ''; // Recovery must fetch a full current snapshot, even after repair to identical bytes.
      onError(error);
    } finally {
      if (!stopped && version === generation) {
        const due = now() + interval;
        // A sleeping machine may resume without a visibility/focus event.
        // Detect a late timer and discard the validator before resynchronizing.
        timer = schedule(() => poll(now() - due > 2000), interval);
      }
    }
  }
  return {
    refresh() {
      stopped = false;
      cancel(timer);
      return poll(true);
    },
    stop() {
      stopped = true;
      generation++;
      cancel(timer);
      controller?.abort();
      etag = '';
    },
  };
}

// Editing registers one draft and owns its form DOM in #draft. Live refresh
// only delivers remote context; it never replaces input values or the base revision.
export function createDraftGuard() {
  let draft;
  return {
    register({ id, revision, isDirty, onRemote }) {
      if (draft) throw new Error('A draft is already registered');
      const current = { id, revision, isDirty, onRemote };
      draft = current;
      return () => { if (draft === current) draft = undefined; };
    },
    inspect(index, visibleIDs, stale = false) {
      if (!draft) return null;
      const item = index?.byID.get(draft.id);
      const context = { id: draft.id, baseRevision: draft.revision, item,
        dirty: draft.isDirty(), changed: Boolean(item && item.revision !== draft.revision),
        missing: Boolean(index && !item), outsideFilters: Boolean(item && !visibleIDs.has(item.id)), stale };
      draft.onRemote(context);
      return context;
    },
  };
}
export const drafts = createDraftGuard();
