let jobs = [];
let currentJob = null;

async function loadJobs() {
  const resp = await fetch('/api/jobs');
  jobs = await resp.json();
  renderJobList();
}

function renderJobList() {
  const el = document.getElementById('job-list-items');
  el.innerHTML = '';
  jobs.forEach(j => {
    const item = document.createElement('div');
    item.className = 'job-item' + (currentJob?.id === j.id ? ' active' : '');
    item.textContent = j.name || j.id;
    item.addEventListener('click', () => selectJob(j));
    el.appendChild(item);
  });
}

function selectJob(job) {
  currentJob = job;
  renderJobList();
  renderEditor(job);
}

function renderEditor(job) {
  const panel = document.getElementById('editor-panel');
  panel.innerHTML = `
    <h2>${job.id ? 'Edit Job' : 'New Job'}</h2>
    <div class="field-group">
      <label>ID (slug)</label>
      <input type="text" id="f-id" value="${esc(job.id)}" ${job.id ? 'readonly style="opacity:.5"' : ''} placeholder="my-job">
    </div>
    <div class="field-group">
      <label>Name</label>
      <input type="text" id="f-name" value="${esc(job.name)}" placeholder="My Job">
    </div>

    <div class="section-label">Commands <button class="add-btn" id="add-cmd">+ Add</button></div>
    <div id="cmd-list"></div>

    <div class="section-label">Variables <button class="add-btn" id="add-var">+ Add</button></div>
    <div id="var-list"></div>

    <div class="url-preview" id="url-preview"></div>

    <div class="action-bar">
      <button class="btn btn-primary" id="save-btn">Save</button>
      ${job.id ? `<button class="btn btn-danger" id="del-btn">Delete</button>` : ''}
      <button class="btn btn-copy" id="copy-btn">Copy URL</button>
    </div>
  `;

  renderCmds(job.commands || []);
  renderVars(job.variables || []);
  updateURLPreview();

  document.getElementById('add-cmd').addEventListener('click', () => {
    const cmds = collectCmds();
    cmds.push({ label: '', template: '' });
    renderCmds(cmds);
    updateURLPreview();
  });

  document.getElementById('add-var').addEventListener('click', () => {
    const vars = collectVars();
    vars.push({ name: '', regex: '', description: '' });
    renderVars(vars);
    updateURLPreview();
  });

  document.getElementById('save-btn').addEventListener('click', saveJob);
  document.getElementById('copy-btn').addEventListener('click', copyURL);
  document.getElementById('del-btn')?.addEventListener('click', deleteJob);

  document.getElementById('f-name').addEventListener('input', updateURLPreview);
}

function renderCmds(cmds) {
  const el = document.getElementById('cmd-list');
  el.innerHTML = '';
  cmds.forEach((c, i) => {
    const row = document.createElement('div');
    row.className = 'row-item cmd-row';
    row.innerHTML = `
      <input type="text" placeholder="Label" value="${esc(c.label)}" data-cmd-label="${i}">
      <input type="text" placeholder="Template: echo {{var}}" value="${esc(c.template)}" data-cmd-tmpl="${i}" style="font-family:monospace">
      <button class="remove-btn" data-rm-cmd="${i}">×</button>
    `;
    el.appendChild(row);
  });
  el.querySelectorAll('[data-rm-cmd]').forEach(btn => {
    btn.addEventListener('click', () => {
      const cmds = collectCmds();
      cmds.splice(parseInt(btn.dataset.rmCmd), 1);
      renderCmds(cmds);
    });
  });
  el.querySelectorAll('[data-cmd-tmpl]').forEach(inp => inp.addEventListener('input', updateURLPreview));
}

function renderVars(vars) {
  const el = document.getElementById('var-list');
  el.innerHTML = '';
  vars.forEach((v, i) => {
    const row = document.createElement('div');
    row.className = 'row-item var-row';
    row.innerHTML = `
      <input type="text" placeholder="name" value="${esc(v.name)}" data-var-name="${i}">
      <input type="text" placeholder="^regex$" value="${esc(v.regex)}" data-var-regex="${i}" style="font-family:monospace">
      <input type="text" placeholder="description" value="${esc(v.description)}" data-var-desc="${i}">
      <button class="remove-btn" data-rm-var="${i}">×</button>
    `;
    el.appendChild(row);
  });
  el.querySelectorAll('[data-rm-var]').forEach(btn => {
    btn.addEventListener('click', () => {
      const vars = collectVars();
      vars.splice(parseInt(btn.dataset.rmVar), 1);
      renderVars(vars);
      updateURLPreview();
    });
  });
  el.querySelectorAll('[data-var-name]').forEach(inp => inp.addEventListener('input', updateURLPreview));
}

function collectCmds() {
  return [...document.querySelectorAll('[data-cmd-label]')].map((el, i) => ({
    label: el.value,
    template: document.querySelector(`[data-cmd-tmpl="${i}"]`)?.value || '',
  }));
}

function collectVars() {
  return [...document.querySelectorAll('[data-var-name]')].map((el, i) => ({
    name: el.value,
    regex: document.querySelector(`[data-var-regex="${i}"]`)?.value || '',
    description: document.querySelector(`[data-var-desc="${i}"]`)?.value || '',
  }));
}

function buildJobFromForm() {
  return {
    id: document.getElementById('f-id').value.trim(),
    name: document.getElementById('f-name').value.trim(),
    commands: collectCmds(),
    variables: collectVars(),
  };
}

function updateURLPreview() {
  const job = buildJobFromForm();
  const vars = job.variables.map(v => `${encodeURIComponent(v.name)}=__${v.name}__`).join('&');
  const url = `${location.origin}/?job=${encodeURIComponent(job.id)}${vars ? '&' + vars : ''}`;
  const el = document.getElementById('url-preview');
  if (el) el.textContent = url;
}

async function saveJob() {
  const job = buildJobFromForm();
  const isNew = !currentJob?.id;
  const method = isNew ? 'POST' : 'PUT';
  const url = isNew ? '/api/jobs' : `/api/jobs/${job.id}`;
  const resp = await fetch(url, {
    method,
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(job),
  });
  if (!resp.ok) {
    alert('Save failed: ' + await resp.text());
    return;
  }
  const saved = await resp.json();
  currentJob = saved;
  await loadJobs();
  renderEditor(saved);
}

async function deleteJob() {
  if (!confirm(`Delete job "${currentJob.id}"?`)) return;
  const resp = await fetch(`/api/jobs/${currentJob.id}`, { method: 'DELETE' });
  if (!resp.ok) { alert('Delete failed'); return; }
  currentJob = null;
  await loadJobs();
  document.getElementById('editor-panel').innerHTML = '<p class="empty-state">Select a job or create a new one.</p>';
}

function copyURL() {
  const url = document.getElementById('url-preview')?.textContent;
  if (url && navigator.clipboard) navigator.clipboard.writeText(url);
}

function esc(s) {
  return (s || '').replace(/&/g,'&amp;').replace(/"/g,'&quot;').replace(/</g,'&lt;').replace(/>/g,'&gt;');
}

document.getElementById('new-job-btn').addEventListener('click', () => {
  currentJob = null;
  renderJobList();
  renderEditor({ id: '', name: '', commands: [], variables: [] });
});

loadJobs();
