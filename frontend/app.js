(async () => {
  try {
    const sessionId = getSessionId();
    if (!sessionId) return;

    const resp = await fetch(`/api/sessions/${sessionId}`);
    if (!resp.ok) {
      document.getElementById('terminal-container').innerHTML =
        `<div class="exited-banner">Session not found.</div>`;
      return;
    }

    const data = await resp.json();
    const panes = data.panes || [];
    const tabBar = document.getElementById('tab-bar');
    const container = document.getElementById('terminal-container');

    panes.forEach((pane, idx) => {
    const tab = document.createElement('div');
    tab.className = 'tab' + (idx === 0 ? ' active' : '');
    tab.dataset.idx = idx;

    const dot = document.createElement('span');
    dot.className = 'status' + (pane.alive ? '' : ' dead');
    tab.appendChild(dot);
    tab.appendChild(document.createTextNode(pane.label || `Pane ${idx + 1}`));
    tabBar.appendChild(tab);

    const paneEl = document.createElement('div');
    paneEl.className = 'terminal-pane' + (idx === 0 ? ' active' : '');
    paneEl.id = `pane-${pane.id}`;
    container.appendChild(paneEl);

    tab.addEventListener('click', () => switchTab(idx));
  });

    // Fetch job to get command labels
    if (data.job_id) {
      const jobResp = await fetch(`/api/jobs/${data.job_id}`);
      if (jobResp.ok) {
        const job = await jobResp.json();
        document.querySelectorAll('.tab').forEach((tab, idx) => {
          const label = job.commands?.[idx]?.label;
          if (label) {
            tab.childNodes[1].textContent = label;
          }
        });
      }
    }

    const terms = panes.map((pane, idx) => {
      const term = new Terminal({ cursorBlink: true, fontSize: 14, fontFamily: 'monospace' });
      const fitAddon = new FitAddon.FitAddon();
      term.loadAddon(fitAddon);

      const el = document.getElementById(`pane-${pane.id}`);
      term.open(el);
      fitAddon.fit();

      if (!pane.alive) {
        term.writeln('\r\n\x1b[2m[process exited]\x1b[0m');
        return { term, fitAddon, ws: null };
      }

      const ws = new WebSocket(`ws://${location.host}/ws/pane/${pane.id}`);
      ws.binaryType = 'arraybuffer';

      ws.onerror = () => term.writeln('\r\n\x1b[31m[WebSocket error]\x1b[0m');
      ws.onclose = (e) => {
        window.removeEventListener('resize', sendResize);
        if (!e.wasClean) term.writeln('\r\n\x1b[31m[connection lost]\x1b[0m');
      };

      ws.onmessage = (e) => {
        if (typeof e.data === 'string') {
          try {
            const msg = JSON.parse(e.data);
            if (msg.type === 'exited') {
              const dot = tabBar.querySelectorAll('.tab')[idx]?.querySelector('.status');
              if (dot) dot.classList.add('dead');
              term.writeln('\r\n\x1b[2m[process exited]\x1b[0m');
            }
          } catch {}
        } else {
          term.write(new Uint8Array(e.data));
        }
      };

      term.onData((data) => {
        if (ws.readyState === WebSocket.OPEN) {
          ws.send(new TextEncoder().encode(data));
        }
      });

      const sendResize = () => {
        fitAddon.fit();
        if (ws.readyState === WebSocket.OPEN) {
          ws.send(JSON.stringify({ type: 'resize', cols: term.cols, rows: term.rows }));
        }
      };

      ws.onopen = sendResize;
      window.addEventListener('resize', sendResize);

      return { term, fitAddon, ws };
    });

    function switchTab(idx) {
      document.querySelectorAll('.tab').forEach((t, i) => t.classList.toggle('active', i === idx));
      document.querySelectorAll('.terminal-pane').forEach((p, i) => p.classList.toggle('active', i === idx));
      terms[idx]?.fitAddon.fit();
    }

    function getSessionId() {
      const hash = location.hash.replace('#', '');
      const params = new URLSearchParams(hash);
      const id = params.get('session');
      if (id) { sessionStorage.setItem('sessionId', id); return id; }
      return sessionStorage.getItem('sessionId');
    }
  } catch (err) {
    const c = document.getElementById('terminal-container');
    if (c) c.innerHTML = `<div class="exited-banner">Failed to load session: ${err.message}</div>`;
  }
})();
