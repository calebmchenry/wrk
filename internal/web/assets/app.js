'use strict';
const reload = document.querySelector('#reload');
async function loadProject() {
  reload.disabled = true;
  const diagnostics = document.querySelector('#diagnostics');
  diagnostics.hidden = true;
  document.querySelector('#status').textContent = 'Loading project';
  document.querySelector('#summary').textContent = '';
  try {
    const response = await fetch('/api/project', { cache: 'no-store', signal: AbortSignal.timeout(20000) });
    const data = await response.json();
    document.querySelector('#project').textContent = data.project_root || 'Local project';
    if (!data.ok) {
      document.querySelector('#status').textContent = 'Project needs attention';
      diagnostics.textContent = data.errors.map(error => `${error.code}: ${error.path || ''} ${error.message}`).join('\n');
      diagnostics.hidden = false;
      return;
    }
    document.querySelector('#status').textContent = 'Project ready';
    const count = data.result.ticket_count;
    document.querySelector('#summary').textContent = `${count} ${count === 1 ? 'item' : 'items'} · ${data.result.config.prefix} · Stored in .wrk`;
  } catch (error) {
    document.querySelector('#status').textContent = 'Unable to reach wrk';
    document.querySelector('#summary').textContent = 'Check that wrk serve is running, then reload.';
  } finally {
    reload.disabled = false;
  }
}
reload.addEventListener('click', loadProject);
loadProject();
