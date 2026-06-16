# Web Terminal Emulator — Design Spec

**Date:** 2026-06-16  
**Status:** Approved

---

## Overview

A personal, web-based terminal emulator running on localhost. Users visit a URL referencing a named "job" configuration; the server opens N browser-based terminal tabs, each running a predefined command template with variable values supplied via query parameters. Sessions are persistent — PTY processes survive browser close/refresh, and reconnecting reattaches to live sessions.

No authentication. Single-user, personal tool.

---

## Stack

| Layer | Choice |
|---|---|
| Server | Go (single binary) |
| Database | SQLite via `modernc.org/sqlite` (pure Go, no CGo) |
| PTY | `github.com/creack/pty` |
| Terminal UI | xterm.js (embedded as static asset) |
| Frontend | Vanilla JS, embedded via `go:embed` |

---

## Data Model

### Job

Defines a named set of command templates and the variables they accept.

```json
{
  "id": "deploy-check",
  "name": "Deploy Check",
  "commands": [
    { "label": "Status", "template": "curl -s {{host}}/status" },
    { "label": "Logs",   "template": "tail -f /var/log/{{service}}.log" }
  ],
  "variables": [
    { "name": "host",    "regex": "^[\\w.-]+$",    "description": "Target hostname" },
    { "name": "service", "regex": "^[a-z0-9-]+$",  "description": "Service name" }
  ]
}
```

- `id` is a URL-safe slug used in `?job=<id>`
- `commands` is an ordered list; each entry produces one terminal tab
- `variables` defines every placeholder that appears across all templates in the job, with its per-variable validation regex

### Session

Created when a URL is visited with a valid job and passing variable values.

```
Session
  id          TEXT PRIMARY KEY   — UUID v4
  job_id      TEXT
  vars        TEXT               — JSON object: {name: value}
  created_at  INTEGER            — Unix timestamp

Pane
  id          TEXT PRIMARY KEY   — UUID v4
  session_id  TEXT
  cmd_index   INTEGER            — index into job.commands
  pid         INTEGER            — OS PID of the PTY process
  alive       BOOLEAN
  output_path TEXT               — path to scrollback file on disk
```

Sessions are never automatically deleted. Pane `alive` is set to false when the PTY process exits.

**Scrollback files:** All PTY output for a pane is appended to a file at `output_path` (e.g. `./data/panes/<pane-id>.log`). The file is a raw byte stream — exactly what the PTY produced. On reconnect, the server streams the full file contents to the new WebSocket client before switching to live output, restoring the visible scrollback buffer in xterm.js.

**Scrollback size limit:** A configurable max size (default 10 MB, set via `-scrollback` flag in bytes) is enforced per pane. After each write batch, if the file exceeds the limit, the PTY manager trims it: it reads the last `maxScrollback/2` bytes from the file, truncates the file, and rewrites those bytes from offset 0. This keeps the most recent output and bounds disk usage. The trim is a synchronous operation in the PTY write goroutine — no separate janitor process needed. Reconnecting clients may see a `--- scrollback trimmed ---` sentinel line injected at the start of the replayed bytes so the truncation is visible.

---

## Request Flow

### Launching a job

1. User visits `/?job=deploy-check&host=myserver.local&service=nginx`
2. Server looks up job `deploy-check` in SQLite → 404 page if not found
3. Server validates each variable value against its per-variable regex:
   - `host` = `myserver.local` vs `^[\w.-]+$` → pass
   - `service` = `nginx` vs `^[a-z0-9-]+$` → pass
   - Any failure → styled HTML error page listing which variables failed and their expected pattern
4. Server creates a `Session` row with the resolved vars
5. Server creates N `Pane` rows, one per command in the job
6. Variable substitution happens server-side: `curl -s {{host}}/status` → `curl -s myserver.local/status`
7. Each pane spawns a PTY: `bash -c "<substituted command>"`
8. Server redirects browser to `/#session=<session-id>`
9. Frontend JS reads the session ID from the URL fragment, fetches pane list from `/api/sessions/<id>`, opens one xterm.js instance per pane with a WebSocket connection to `/ws/pane/<pane-id>`

### Browser reconnect

- Session ID is stored in the URL fragment (`#session=<id>`) and in `sessionStorage`
- On page load, if a session ID is present and the session is active in SQLite, the frontend reattaches to existing panes without spawning new PTYs
- If a pane's PTY has exited (`alive = false`), the tab shows an "exited" banner rather than an active terminal

---

## Variable Validation

- Validation occurs **server-side only**, before any substitution or PTY spawn
- Each variable in a job has its own regex stored in SQLite
- Validation is applied to URL query parameter values at request time
- Regex is a Go `regexp` pattern; the value must fully match (`regexp.MatchString`)
- Raw variable values are never passed to the shell; only the fully substituted command string is passed to `bash -c`
- The substituted command is also never sent to the browser

---

## Terminal UI Layout

**Main view (tabbed):** One terminal fills the viewport. Tabs across the top label each pane (using the command's `label` field). Active tab is highlighted. Clicking a tab switches the visible xterm.js instance; all instances remain connected to their WebSockets in the background.

**Nav bar:** Minimal top bar with app name and a link to the Jobs editor.

**Error page:** Server-rendered HTML (no JS required). Lists each failing variable, its supplied value, and its expected regex pattern. Styled consistently with the rest of the app.

---

## Job Editor UI

Accessible at `/jobs`. Two-pane layout:

- **Left sidebar:** Scrollable list of job names/IDs. "New Job" button at bottom.
- **Right panel:** Edit form for the selected job:
  - Fields: ID (slug), Name
  - Commands list: each row has Label + Template inputs; rows can be added, removed, reordered
  - Variables list: each row has Name + Regex + Description inputs
  - "Copy URL" button: generates the URL template with `__varname__` placeholders for all variables
  - Save / Delete buttons

Changes are saved immediately to SQLite via `PUT /api/jobs/:id`.

---

## REST API

```
GET  /                        HTML — launches a job (validates vars, creates session,
                                     redirects to /#session=<id>); OR serves the
                                     main app shell if no ?job= param is present
GET  /jobs                    HTML — job editor page

GET    /api/jobs              List all jobs
POST   /api/jobs              Create job
GET    /api/jobs/:id          Get job
PUT    /api/jobs/:id          Update job
DELETE /api/jobs/:id          Delete job

GET  /api/sessions/:id        Get session + pane list

WS   /ws/pane/:pane-id       Bidirectional terminal I/O
```

**WebSocket framing:** Two frame types share the same connection:
- **Binary frames** — raw PTY bytes, forwarded directly to/from the PTY
- **Text frames** — JSON control messages, e.g. `{"type":"resize","cols":220,"rows":50}`

The client sends binary for keystrokes and text for resize events. The server sends binary for PTY output only.

**Multi-client broadcast:** Multiple browser clients may connect to the same pane simultaneously (no authentication, any user can join any session). The PTY manager maintains a set of active WebSocket connections per pane. PTY output is broadcast to all connected clients. Input from any connected client is forwarded to the PTY. Terminal resize uses the dimensions of the most recently connected client.

**Reconnect with scrollback:** When a client connects to a pane, the server first streams the full contents of the pane's scrollback file as binary frames, then switches to live PTY output. This restores the terminal history in xterm.js without any client-side state.

---

## Package Structure

```
console-web/
  main.go                     — flag parsing, server startup, signal handling
  go.mod
  go.sum

  internal/
    db/
      store.go                — SQLite schema init, job CRUD, session/pane CRUD
    pty/
      manager.go              — spawn PTY (bash -c cmd), track by pane ID, broadcast to N clients, append output to scrollback file
    session/
      manager.go              — create session, reconnect logic, alive checks
    validate/
      vars.go                 — per-variable regex validation
    api/
      handlers.go             — HTTP handlers (jobs REST, session create)
      ws.go                   — WebSocket handler, PTY I/O bridge, resize messages

  frontend/
    index.html                — main app shell (xterm tabs)
    editor.html               — job editor shell
    app.js                    — xterm.js init, WebSocket wiring, tab switching
    editor.js                 — job list + detail editor CRUD
    style.css                 — shared styles (dark terminal theme)
    xterm/                    — vendored xterm.js + xterm-addon-fit
```

---

## Configuration

Server is configured via CLI flags:

```
-addr       string   Listen address (default "127.0.0.1:8080")
-db         string   SQLite database file path (default "./console-web.db")
-data       string   Directory for pane scrollback files (default "./data")
-scrollback int      Max scrollback bytes per pane before trimming (default 10485760 = 10 MB)
```

No config file. No environment variables. Intentionally minimal.

The `-data` flag sets the directory for scrollback files (default `./data`). Created automatically on startup.

---

## Out of Scope

- Authentication / access control
- Remote server execution (commands always run on the same machine as the server)
