# Web Terminal Emulator Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Go web server that launches persistent PTY terminal sessions in the browser, defined by job configurations with per-variable regex validation and URL-based launch parameters.

**Architecture:** Single Go binary serving embedded frontend assets; SQLite stores job configs and session/pane metadata; PTY processes run on the server and stream to N browser clients via WebSockets with binary frames for terminal I/O and text frames for control messages; scrollback is persisted to per-pane files on disk.

**Tech Stack:** Go 1.22+, `modernc.org/sqlite`, `github.com/creack/pty`, `github.com/gorilla/websocket`, `github.com/google/uuid`, xterm.js 5.x + xterm-addon-fit, vanilla JS, `go:embed`

---

## Shared Types Reference

These types are defined in Task 2 and used across all later tasks. Copy this section when needed.

```go
// internal/db/store.go
type Command struct {
    Label    string `json:"label"`
    Template string `json:"template"`
}
type Variable struct {
    Name        string `json:"name"`
    Regex       string `json:"regex"`
    Description string `json:"description"`
}
type Job struct {
    ID        string     `json:"id"`
    Name      string     `json:"name"`
    Commands  []Command  `json:"commands"`
    Variables []Variable `json:"variables"`
}
type Session struct {
    ID        string            `json:"id"`
    JobID     string            `json:"job_id"`
    Vars      map[string]string `json:"vars"`
    CreatedAt int64             `json:"created_at"`
}
type Pane struct {
    ID         string `json:"id"`
    SessionID  string `json:"session_id"`
    CmdIndex   int    `json:"cmd_index"`
    PID        int    `json:"pid"`
    Alive      bool   `json:"alive"`
    OutputPath string `json:"output_path"`
}
```

---

## Task 1: Project Scaffold

**Files:**
- Create: `go.mod`
- Create: `main.go` (stub)
- Create: `internal/db/store.go` (stub)
- Create: `internal/validate/vars.go` (stub)
- Create: `internal/pty/manager.go` (stub)
- Create: `internal/session/manager.go` (stub)
- Create: `internal/api/handlers.go` (stub)
- Create: `internal/api/ws.go` (stub)
- Create: `frontend/index.html` (stub)
- Create: `frontend/editor.html` (stub)
- Create: `frontend/style.css` (stub)
- Create: `frontend/app.js` (stub)
- Create: `frontend/editor.js` (stub)

- [ ] **Step 1: Initialize Go module**

```bash
cd /Users/jesse/Documents/console-web
go mod init console-web
```

- [ ] **Step 2: Add dependencies**

```bash
go get modernc.org/sqlite
go get github.com/creack/pty
go get github.com/gorilla/websocket
go get github.com/google/uuid
```

- [ ] **Step 3: Download xterm.js assets**

```bash
mkdir -p frontend/xterm
curl -L https://unpkg.com/@xterm/xterm@5.5.0/lib/xterm.js -o frontend/xterm/xterm.js
curl -L https://unpkg.com/@xterm/xterm@5.5.0/css/xterm.css -o frontend/xterm/xterm.css
curl -L https://unpkg.com/@xterm/xterm-addon-fit@0.10.0/lib/xterm-addon-fit.js -o frontend/xterm/xterm-addon-fit.js
```

- [ ] **Step 4: Create stub main.go**

```go
package main

import "fmt"

func main() {
    fmt.Println("console-web")
}
```

- [ ] **Step 5: Create stub packages** (one file each, just the package declaration)

`internal/db/store.go`:
```go
package db
```

`internal/validate/vars.go`:
```go
package validate
```

`internal/pty/manager.go`:
```go
package pty
```

`internal/session/manager.go`:
```go
package session
```

`internal/api/handlers.go`:
```go
package api
```

`internal/api/ws.go`:
```go
package api
```

- [ ] **Step 6: Create stub frontend files**

`frontend/index.html`: empty file  
`frontend/editor.html`: empty file  
`frontend/style.css`: empty file  
`frontend/app.js`: empty file  
`frontend/editor.js`: empty file  

- [ ] **Step 7: Verify it compiles**

```bash
go build ./...
```

Expected: no output, no errors.

- [ ] **Step 8: Commit**

```bash
git init
git add .
git commit -m "feat: project scaffold"
```

---

## Task 2: DB Store

**Files:**
- Modify: `internal/db/store.go`
- Create: `internal/db/store_test.go`

- [ ] **Step 1: Write failing tests**

`internal/db/store_test.go`:
```go
package db_test

import (
    "testing"
    "console-web/internal/db"
)

func newTestStore(t *testing.T) *db.Store {
    t.Helper()
    s, err := db.Open(":memory:")
    if err != nil {
        t.Fatalf("open: %v", err)
    }
    t.Cleanup(func() { s.Close() })
    return s
}

func TestJobCRUD(t *testing.T) {
    s := newTestStore(t)

    job := &db.Job{
        ID:   "test-job",
        Name: "Test Job",
        Commands: []db.Command{
            {Label: "Run", Template: "echo {{msg}}"},
        },
        Variables: []db.Variable{
            {Name: "msg", Regex: `^\w+$`, Description: "a word"},
        },
    }

    if err := s.CreateJob(job); err != nil {
        t.Fatalf("create: %v", err)
    }

    got, err := s.GetJob("test-job")
    if err != nil {
        t.Fatalf("get: %v", err)
    }
    if got.Name != "Test Job" {
        t.Errorf("name: got %q want %q", got.Name, "Test Job")
    }
    if len(got.Commands) != 1 || got.Commands[0].Template != "echo {{msg}}" {
        t.Errorf("commands: %+v", got.Commands)
    }

    got.Name = "Updated"
    if err := s.UpdateJob(got); err != nil {
        t.Fatalf("update: %v", err)
    }
    got2, _ := s.GetJob("test-job")
    if got2.Name != "Updated" {
        t.Errorf("after update name: %q", got2.Name)
    }

    jobs, err := s.ListJobs()
    if err != nil {
        t.Fatalf("list: %v", err)
    }
    if len(jobs) != 1 {
        t.Errorf("list len: %d", len(jobs))
    }

    if err := s.DeleteJob("test-job"); err != nil {
        t.Fatalf("delete: %v", err)
    }
    if _, err := s.GetJob("test-job"); err == nil {
        t.Error("expected error after delete")
    }
}

func TestSessionAndPaneCRUD(t *testing.T) {
    s := newTestStore(t)

    // need a job first (foreign key)
    _ = s.CreateJob(&db.Job{ID: "j", Name: "J", Commands: nil, Variables: nil})

    sess := &db.Session{
        ID:        "sess-1",
        JobID:     "j",
        Vars:      map[string]string{"x": "y"},
        CreatedAt: 1000,
    }
    if err := s.CreateSession(sess); err != nil {
        t.Fatalf("create session: %v", err)
    }

    got, err := s.GetSession("sess-1")
    if err != nil {
        t.Fatalf("get session: %v", err)
    }
    if got.Vars["x"] != "y" {
        t.Errorf("vars: %v", got.Vars)
    }

    pane := &db.Pane{
        ID:         "pane-1",
        SessionID:  "sess-1",
        CmdIndex:   0,
        PID:        0,
        Alive:      true,
        OutputPath: "/tmp/pane-1.log",
    }
    if err := s.CreatePane(pane); err != nil {
        t.Fatalf("create pane: %v", err)
    }

    panes, err := s.ListPanes("sess-1")
    if err != nil {
        t.Fatalf("list panes: %v", err)
    }
    if len(panes) != 1 || panes[0].OutputPath != "/tmp/pane-1.log" {
        t.Errorf("panes: %+v", panes)
    }

    if err := s.SetPaneAlive("pane-1", false); err != nil {
        t.Fatalf("set alive: %v", err)
    }
    if err := s.SetPanePID("pane-1", 12345); err != nil {
        t.Fatalf("set pid: %v", err)
    }
    p, _ := s.GetPane("pane-1")
    if p.Alive {
        t.Error("expected alive=false")
    }
    if p.PID != 12345 {
        t.Errorf("pid: %d", p.PID)
    }
}
```

- [ ] **Step 2: Run — verify it fails**

```bash
go test ./internal/db/...
```

Expected: compilation error (Store not defined).

- [ ] **Step 3: Implement store**

`internal/db/store.go`:
```go
package db

import (
    "database/sql"
    "encoding/json"
    "fmt"

    _ "modernc.org/sqlite"
)

type Command struct {
    Label    string `json:"label"`
    Template string `json:"template"`
}

type Variable struct {
    Name        string `json:"name"`
    Regex       string `json:"regex"`
    Description string `json:"description"`
}

type Job struct {
    ID        string     `json:"id"`
    Name      string     `json:"name"`
    Commands  []Command  `json:"commands"`
    Variables []Variable `json:"variables"`
}

type Session struct {
    ID        string            `json:"id"`
    JobID     string            `json:"job_id"`
    Vars      map[string]string `json:"vars"`
    CreatedAt int64             `json:"created_at"`
}

type Pane struct {
    ID         string `json:"id"`
    SessionID  string `json:"session_id"`
    CmdIndex   int    `json:"cmd_index"`
    PID        int    `json:"pid"`
    Alive      bool   `json:"alive"`
    OutputPath string `json:"output_path"`
}

type Store struct {
    db *sql.DB
}

func Open(path string) (*Store, error) {
    db, err := sql.Open("sqlite", path)
    if err != nil {
        return nil, err
    }
    s := &Store{db: db}
    if err := s.migrate(); err != nil {
        db.Close()
        return nil, err
    }
    return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
    _, err := s.db.Exec(`
        PRAGMA journal_mode=WAL;
        PRAGMA foreign_keys=ON;

        CREATE TABLE IF NOT EXISTS jobs (
            id       TEXT PRIMARY KEY,
            name     TEXT NOT NULL,
            commands TEXT NOT NULL DEFAULT '[]',
            variables TEXT NOT NULL DEFAULT '[]'
        );

        CREATE TABLE IF NOT EXISTS sessions (
            id         TEXT PRIMARY KEY,
            job_id     TEXT NOT NULL REFERENCES jobs(id),
            vars       TEXT NOT NULL DEFAULT '{}',
            created_at INTEGER NOT NULL
        );

        CREATE TABLE IF NOT EXISTS panes (
            id          TEXT PRIMARY KEY,
            session_id  TEXT NOT NULL REFERENCES sessions(id),
            cmd_index   INTEGER NOT NULL,
            pid         INTEGER NOT NULL DEFAULT 0,
            alive       BOOLEAN NOT NULL DEFAULT 1,
            output_path TEXT NOT NULL DEFAULT ''
        );
    `)
    return err
}

// Jobs

func (s *Store) CreateJob(j *Job) error {
    cmds, err := json.Marshal(j.Commands)
    if err != nil {
        return err
    }
    vars, err := json.Marshal(j.Variables)
    if err != nil {
        return err
    }
    _, err = s.db.Exec(
        `INSERT INTO jobs (id, name, commands, variables) VALUES (?,?,?,?)`,
        j.ID, j.Name, string(cmds), string(vars),
    )
    return err
}

func (s *Store) GetJob(id string) (*Job, error) {
    row := s.db.QueryRow(`SELECT id, name, commands, variables FROM jobs WHERE id=?`, id)
    return scanJob(row)
}

func (s *Store) ListJobs() ([]Job, error) {
    rows, err := s.db.Query(`SELECT id, name, commands, variables FROM jobs ORDER BY name`)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    var jobs []Job
    for rows.Next() {
        j, err := scanJob(rows)
        if err != nil {
            return nil, err
        }
        jobs = append(jobs, *j)
    }
    return jobs, rows.Err()
}

func (s *Store) UpdateJob(j *Job) error {
    cmds, err := json.Marshal(j.Commands)
    if err != nil {
        return err
    }
    vars, err := json.Marshal(j.Variables)
    if err != nil {
        return err
    }
    res, err := s.db.Exec(
        `UPDATE jobs SET name=?, commands=?, variables=? WHERE id=?`,
        j.Name, string(cmds), string(vars), j.ID,
    )
    if err != nil {
        return err
    }
    n, _ := res.RowsAffected()
    if n == 0 {
        return fmt.Errorf("job %q not found", j.ID)
    }
    return nil
}

func (s *Store) DeleteJob(id string) error {
    _, err := s.db.Exec(`DELETE FROM jobs WHERE id=?`, id)
    return err
}

type scanner interface {
    Scan(dest ...any) error
}

func scanJob(r scanner) (*Job, error) {
    var j Job
    var cmds, vars string
    if err := r.Scan(&j.ID, &j.Name, &cmds, &vars); err != nil {
        if err == sql.ErrNoRows {
            return nil, fmt.Errorf("job not found")
        }
        return nil, err
    }
    if err := json.Unmarshal([]byte(cmds), &j.Commands); err != nil {
        return nil, err
    }
    if err := json.Unmarshal([]byte(vars), &j.Variables); err != nil {
        return nil, err
    }
    if j.Commands == nil {
        j.Commands = []Command{}
    }
    if j.Variables == nil {
        j.Variables = []Variable{}
    }
    return &j, nil
}

// Sessions

func (s *Store) CreateSession(sess *Session) error {
    vars, err := json.Marshal(sess.Vars)
    if err != nil {
        return err
    }
    _, err = s.db.Exec(
        `INSERT INTO sessions (id, job_id, vars, created_at) VALUES (?,?,?,?)`,
        sess.ID, sess.JobID, string(vars), sess.CreatedAt,
    )
    return err
}

func (s *Store) GetSession(id string) (*Session, error) {
    row := s.db.QueryRow(`SELECT id, job_id, vars, created_at FROM sessions WHERE id=?`, id)
    var sess Session
    var varsJSON string
    if err := row.Scan(&sess.ID, &sess.JobID, &varsJSON, &sess.CreatedAt); err != nil {
        if err == sql.ErrNoRows {
            return nil, fmt.Errorf("session not found")
        }
        return nil, err
    }
    if err := json.Unmarshal([]byte(varsJSON), &sess.Vars); err != nil {
        return nil, err
    }
    return &sess, nil
}

// Panes

func (s *Store) CreatePane(p *Pane) error {
    _, err := s.db.Exec(
        `INSERT INTO panes (id, session_id, cmd_index, pid, alive, output_path) VALUES (?,?,?,?,?,?)`,
        p.ID, p.SessionID, p.CmdIndex, p.PID, p.Alive, p.OutputPath,
    )
    return err
}

func (s *Store) GetPane(id string) (*Pane, error) {
    row := s.db.QueryRow(
        `SELECT id, session_id, cmd_index, pid, alive, output_path FROM panes WHERE id=?`, id,
    )
    var p Pane
    if err := row.Scan(&p.ID, &p.SessionID, &p.CmdIndex, &p.PID, &p.Alive, &p.OutputPath); err != nil {
        if err == sql.ErrNoRows {
            return nil, fmt.Errorf("pane not found")
        }
        return nil, err
    }
    return &p, nil
}

func (s *Store) ListPanes(sessionID string) ([]Pane, error) {
    rows, err := s.db.Query(
        `SELECT id, session_id, cmd_index, pid, alive, output_path FROM panes WHERE session_id=? ORDER BY cmd_index`,
        sessionID,
    )
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    var panes []Pane
    for rows.Next() {
        var p Pane
        if err := rows.Scan(&p.ID, &p.SessionID, &p.CmdIndex, &p.PID, &p.Alive, &p.OutputPath); err != nil {
            return nil, err
        }
        panes = append(panes, p)
    }
    return panes, rows.Err()
}

func (s *Store) SetPaneAlive(id string, alive bool) error {
    _, err := s.db.Exec(`UPDATE panes SET alive=? WHERE id=?`, alive, id)
    return err
}

func (s *Store) SetPanePID(id string, pid int) error {
    _, err := s.db.Exec(`UPDATE panes SET pid=? WHERE id=?`, pid, id)
    return err
}
```

- [ ] **Step 4: Run tests — verify they pass**

```bash
go test ./internal/db/... -v
```

Expected: PASS for `TestJobCRUD` and `TestSessionAndPaneCRUD`.

- [ ] **Step 5: Commit**

```bash
git add internal/db/
git commit -m "feat: SQLite store with job/session/pane CRUD"
```

---

## Task 3: Variable Validation

**Files:**
- Modify: `internal/validate/vars.go`
- Create: `internal/validate/vars_test.go`

- [ ] **Step 1: Write failing tests**

`internal/validate/vars_test.go`:
```go
package validate_test

import (
    "testing"
    "console-web/internal/db"
    "console-web/internal/validate"
)

func TestValidate_AllPass(t *testing.T) {
    job := &db.Job{
        Variables: []db.Variable{
            {Name: "host", Regex: `^[\w.-]+$`},
            {Name: "port", Regex: `^\d{2,5}$`},
        },
    }
    errs := validate.Vars(job, map[string]string{"host": "myserver.local", "port": "8080"})
    if len(errs) != 0 {
        t.Errorf("expected no errors, got %+v", errs)
    }
}

func TestValidate_Fails(t *testing.T) {
    job := &db.Job{
        Variables: []db.Variable{
            {Name: "host", Regex: `^[\w.-]+$`},
            {Name: "port", Regex: `^\d{2,5}$`},
        },
    }
    errs := validate.Vars(job, map[string]string{"host": "bad host!", "port": "99999"})
    if len(errs) != 2 {
        t.Errorf("expected 2 errors, got %d: %+v", len(errs), errs)
    }
    if errs[0].Name != "host" {
        t.Errorf("first error name: %q", errs[0].Name)
    }
}

func TestValidate_MissingVar(t *testing.T) {
    job := &db.Job{
        Variables: []db.Variable{
            {Name: "host", Regex: `^[\w.-]+$`},
        },
    }
    // not providing "host" should fail — empty string won't match
    errs := validate.Vars(job, map[string]string{})
    if len(errs) != 1 {
        t.Errorf("expected 1 error for missing var, got %d", len(errs))
    }
}

func TestSubstitute(t *testing.T) {
    result := validate.Substitute("curl -s {{host}}/status", map[string]string{"host": "myserver.local"})
    if result != "curl -s myserver.local/status" {
        t.Errorf("got %q", result)
    }
}
```

- [ ] **Step 2: Run — verify it fails**

```bash
go test ./internal/validate/...
```

Expected: compilation error.

- [ ] **Step 3: Implement**

`internal/validate/vars.go`:
```go
package validate

import (
    "regexp"
    "strings"
    "console-web/internal/db"
)

type Error struct {
    Name    string
    Value   string
    Pattern string
}

// Vars validates each job variable against the supplied vars map.
// Returns one Error per failing variable. Variables defined in the job
// but absent from vars are treated as empty string and validated normally.
func Vars(job *db.Job, vars map[string]string) []Error {
    var errs []Error
    for _, v := range job.Variables {
        val := vars[v.Name]
        matched, err := regexp.MatchString(v.Regex, val)
        if err != nil || !matched {
            errs = append(errs, Error{Name: v.Name, Value: val, Pattern: v.Regex})
        }
    }
    return errs
}

// Substitute replaces all {{name}} placeholders in template with values from vars.
func Substitute(template string, vars map[string]string) string {
    result := template
    for k, v := range vars {
        result = strings.ReplaceAll(result, "{{"+k+"}}", v)
    }
    return result
}
```

- [ ] **Step 4: Run tests — verify they pass**

```bash
go test ./internal/validate/... -v
```

Expected: PASS for all four tests.

- [ ] **Step 5: Commit**

```bash
git add internal/validate/
git commit -m "feat: per-variable regex validation and template substitution"
```

---

## Task 4: PTY Manager

**Files:**
- Modify: `internal/pty/manager.go`
- Create: `internal/pty/manager_test.go`

- [ ] **Step 1: Write failing tests**

`internal/pty/manager_test.go`:
```go
package pty_test

import (
    "os"
    "path/filepath"
    "testing"
    "strings"
    "time"
    "console-web/internal/pty"
)

func TestScrollbackTrim(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "pane.log")

    // Write 20 bytes, max is 10 — should trim to last 5
    f, err := os.Create(path)
    if err != nil {
        t.Fatal(err)
    }
    f.Write([]byte("0123456789abcdefghij")) // 20 bytes
    f.Close()

    if err := pty.TrimScrollback(path, 10); err != nil {
        t.Fatalf("trim: %v", err)
    }

    data, err := os.ReadFile(path)
    if err != nil {
        t.Fatal(err)
    }
    // should keep last 10 bytes: "abcdefghij"
    if string(data) != "abcdefghij" {
        t.Errorf("after trim: %q (len %d)", data, len(data))
    }
}

func TestScrollbackTrimBelowMax(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "pane.log")
    os.WriteFile(path, []byte("hello"), 0644)

    if err := pty.TrimScrollback(path, 100); err != nil {
        t.Fatalf("trim: %v", err)
    }

    data, _ := os.ReadFile(path)
    if string(data) != "hello" {
        t.Errorf("should be unchanged, got %q", data)
    }
}

func TestSpawnAndOutput(t *testing.T) {
    dir := t.TempDir()
    m := pty.NewManager(dir, 1024*1024)

    outPath := filepath.Join(dir, "out.log")
    paneID, err := m.Spawn("pane-test", "echo hello-from-pty", outPath)
    if err != nil {
        t.Fatalf("spawn: %v", err)
    }
    if paneID != "pane-test" {
        t.Errorf("paneID: %q", paneID)
    }

    // Give the process time to write output and exit
    time.Sleep(300 * time.Millisecond)

    data, err := os.ReadFile(outPath)
    if err != nil {
        t.Fatalf("read output: %v", err)
    }
    if !strings.Contains(string(data), "hello-from-pty") {
        t.Errorf("output file missing expected string, got: %q", data)
    }
}

```

- [ ] **Step 2: Run — verify it fails**

```bash
go test ./internal/pty/...
```

Expected: compilation error.

- [ ] **Step 3: Implement PTY manager**

`internal/pty/manager.go`:
```go
package pty

import (
    "fmt"
    "io"
    "os"
    "os/exec"
    "sync"

    "github.com/creack/pty"
    "github.com/gorilla/websocket"
)

type client struct {
    conn *websocket.Conn
    send chan []byte
}

type paneState struct {
    mu      sync.Mutex
    ptmx    *os.File
    clients map[*websocket.Conn]*client
    done    chan struct{}
}

// Manager tracks running PTY panes and their connected WebSocket clients.
type Manager struct {
    mu          sync.Mutex
    panes       map[string]*paneState
    dataDir     string
    maxScrollback int64
    // OnExit is called when a PTY process exits. Optional.
    OnExit func(paneID string)
}

func NewManager(dataDir string, maxScrollback int64) *Manager {
    return &Manager{
        panes:         make(map[string]*paneState),
        dataDir:       dataDir,
        maxScrollback: maxScrollback,
    }
}

// Spawn starts bash -c cmd for paneID, appending output to outputPath.
// Returns paneID on success.
func (m *Manager) Spawn(paneID, cmd, outputPath string) (string, error) {
    c := exec.Command("bash", "-c", cmd)
    ptmx, err := pty.Start(c)
    if err != nil {
        return "", fmt.Errorf("pty start: %w", err)
    }

    outFile, err := os.OpenFile(outputPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
    if err != nil {
        ptmx.Close()
        return "", fmt.Errorf("open output file: %w", err)
    }

    ps := &paneState{
        ptmx:    ptmx,
        clients: make(map[*websocket.Conn]*client),
        done:    make(chan struct{}),
    }

    m.mu.Lock()
    m.panes[paneID] = ps
    m.mu.Unlock()

    go m.readLoop(paneID, ps, outFile, c)

    return paneID, nil
}

func (m *Manager) readLoop(paneID string, ps *paneState, outFile *os.File, cmd *exec.Cmd) {
    defer func() {
        outFile.Close()
        ps.ptmx.Close()
        close(ps.done)
        if m.OnExit != nil {
            m.OnExit(paneID)
        }
    }()

    buf := make([]byte, 4096)
    for {
        n, err := ps.ptmx.Read(buf)
        if n > 0 {
            chunk := make([]byte, n)
            copy(chunk, buf[:n])

            outFile.Write(chunk)
            outFile.Sync()
            m.maybeTrim(outFile.Name())

            ps.mu.Lock()
            for _, cl := range ps.clients {
                select {
                case cl.send <- chunk:
                default:
                }
            }
            ps.mu.Unlock()
        }
        if err != nil {
            break
        }
    }
    cmd.Wait()
}

func (m *Manager) maybeTrim(path string) {
    info, err := os.Stat(path)
    if err != nil || info.Size() <= m.maxScrollback {
        return
    }
    TrimScrollback(path, m.maxScrollback)
}

// AddClient registers a WebSocket conn to receive output from paneID.
// It first replays the scrollback file, then streams live output.
// Returns a send channel the ws handler should drain, and a done channel
// that closes when the PTY exits.
func (m *Manager) AddClient(paneID string, conn *websocket.Conn, outputPath string) (<-chan []byte, <-chan struct{}, error) {
    m.mu.Lock()
    ps, ok := m.panes[paneID]
    m.mu.Unlock()
    if !ok {
        return nil, nil, fmt.Errorf("pane %q not found or not running", paneID)
    }

    ch := make(chan []byte, 256)
    cl := &client{conn: conn, send: ch}

    ps.mu.Lock()
    ps.clients[conn] = cl
    ps.mu.Unlock()

    // Replay scrollback in a goroutine so we don't hold the lock
    go func() {
        if data, err := os.ReadFile(outputPath); err == nil && len(data) > 0 {
            sentinel := []byte("\r\n\033[33m--- scrollback start ---\033[0m\r\n")
            ch <- sentinel
            // send in chunks to avoid oversized messages
            for len(data) > 0 {
                sz := 4096
                if sz > len(data) {
                    sz = len(data)
                }
                chunk := make([]byte, sz)
                copy(chunk, data[:sz])
                ch <- chunk
                data = data[sz:]
            }
        }
    }()

    return ch, ps.done, nil
}

// RemoveClient unregisters a WebSocket conn from paneID.
func (m *Manager) RemoveClient(paneID string, conn *websocket.Conn) {
    m.mu.Lock()
    ps, ok := m.panes[paneID]
    m.mu.Unlock()
    if !ok {
        return
    }
    ps.mu.Lock()
    delete(ps.clients, conn)
    ps.mu.Unlock()
}

// WriteInput sends bytes from a client to the PTY stdin.
func (m *Manager) WriteInput(paneID string, data []byte) error {
    m.mu.Lock()
    ps, ok := m.panes[paneID]
    m.mu.Unlock()
    if !ok {
        return fmt.Errorf("pane %q not found", paneID)
    }
    _, err := ps.ptmx.Write(data)
    return err
}

// Resize sets the PTY terminal size.
func (m *Manager) Resize(paneID string, cols, rows uint16) error {
    m.mu.Lock()
    ps, ok := m.panes[paneID]
    m.mu.Unlock()
    if !ok {
        return fmt.Errorf("pane %q not found", paneID)
    }
    return pty.Setsize(ps.ptmx, &pty.Winsize{Cols: cols, Rows: rows})
}

// IsAlive reports whether paneID has a running PTY.
func (m *Manager) IsAlive(paneID string) bool {
    m.mu.Lock()
    _, ok := m.panes[paneID]
    m.mu.Unlock()
    return ok
}

// TrimScrollback is exported for testing. Trims path to its last maxBytes bytes.
func TrimScrollback(path string, maxBytes int64) error {
    info, err := os.Stat(path)
    if err != nil {
        return err
    }
    if info.Size() <= maxBytes {
        return nil
    }
    f, err := os.OpenFile(path, os.O_RDWR, 0644)
    if err != nil {
        return err
    }
    defer f.Close()

    keep := maxBytes / 2
    offset := info.Size() - keep
    tail := make([]byte, keep)
    if _, err := f.ReadAt(tail, offset); err != nil && err != io.EOF {
        return err
    }
    if err := f.Truncate(0); err != nil {
        return err
    }
    if _, err := f.WriteAt(tail, 0); err != nil {
        return err
    }
    return nil
}
```

- [ ] **Step 4: Run tests — verify they pass**

```bash
go test ./internal/pty/... -v
```

Expected: PASS for all three tests. `TestSpawnAndOutput` spawns a real PTY.

- [ ] **Step 5: Commit**

```bash
git add internal/pty/
git commit -m "feat: PTY manager with broadcast, scrollback file, and trim"
```

---

## Task 5: Session Manager

**Files:**
- Modify: `internal/session/manager.go`
- Create: `internal/session/manager_test.go`

- [ ] **Step 1: Write failing tests**

`internal/session/manager_test.go`:
```go
package session_test

import (
    "os"
    "testing"
    "console-web/internal/db"
    "console-web/internal/pty"
    "console-web/internal/session"
)

func TestLaunch(t *testing.T) {
    store, err := db.Open(":memory:")
    if err != nil {
        t.Fatal(err)
    }
    defer store.Close()

    dataDir := t.TempDir()
    ptyMgr := pty.NewManager(dataDir, 1024*1024)
    mgr := session.NewManager(store, ptyMgr, dataDir)

    // Seed a job
    job := &db.Job{
        ID:   "greet",
        Name: "Greet",
        Commands: []db.Command{
            {Label: "Hello", Template: "echo hello {{name}}"},
        },
        Variables: []db.Variable{
            {Name: "name", Regex: `^\w+$`},
        },
    }
    if err := store.CreateJob(job); err != nil {
        t.Fatal(err)
    }

    sess, panes, err := mgr.Launch("greet", map[string]string{"name": "world"})
    if err != nil {
        t.Fatalf("launch: %v", err)
    }
    if sess.JobID != "greet" {
        t.Errorf("job_id: %q", sess.JobID)
    }
    if len(panes) != 1 {
        t.Fatalf("expected 1 pane, got %d", len(panes))
    }
    if panes[0].CmdIndex != 0 {
        t.Errorf("cmd_index: %d", panes[0].CmdIndex)
    }
    if _, err := os.Stat(panes[0].OutputPath); err != nil {
        t.Errorf("output file not created: %v", err)
    }
}

func TestGet(t *testing.T) {
    store, _ := db.Open(":memory:")
    defer store.Close()
    dataDir := t.TempDir()
    ptyMgr := pty.NewManager(dataDir, 1024*1024)
    mgr := session.NewManager(store, ptyMgr, dataDir)

    store.CreateJob(&db.Job{ID: "j", Name: "J", Commands: []db.Command{{Label: "L", Template: "sleep 0"}}, Variables: nil})
    sess, _, _ := mgr.Launch("j", map[string]string{})

    gotSess, gotPanes, err := mgr.Get(sess.ID)
    if err != nil {
        t.Fatalf("get: %v", err)
    }
    if gotSess.ID != sess.ID {
        t.Errorf("id mismatch")
    }
    if len(gotPanes) != 1 {
        t.Errorf("panes: %d", len(gotPanes))
    }
}
```

- [ ] **Step 2: Run — verify it fails**

```bash
go test ./internal/session/...
```

Expected: compilation error.

- [ ] **Step 3: Implement**

`internal/session/manager.go`:
```go
package session

import (
    "fmt"
    "os"
    "path/filepath"
    "time"

    "github.com/google/uuid"
    "console-web/internal/db"
    "console-web/internal/pty"
)

type Manager struct {
    store   *db.Store
    ptyMgr  *pty.Manager
    dataDir string
}

func NewManager(store *db.Store, ptyMgr *pty.Manager, dataDir string) *Manager {
    return &Manager{store: store, ptyMgr: ptyMgr, dataDir: dataDir}
}

// Launch creates a session for the given job+vars, spawns PTYs, and returns
// the session and its panes. Caller must have already validated vars.
func (m *Manager) Launch(jobID string, vars map[string]string) (*db.Session, []db.Pane, error) {
    job, err := m.store.GetJob(jobID)
    if err != nil {
        return nil, nil, fmt.Errorf("job %q: %w", jobID, err)
    }

    if err := os.MkdirAll(filepath.Join(m.dataDir, "panes"), 0755); err != nil {
        return nil, nil, err
    }

    sess := &db.Session{
        ID:        uuid.New().String(),
        JobID:     jobID,
        Vars:      vars,
        CreatedAt: time.Now().Unix(),
    }
    if err := m.store.CreateSession(sess); err != nil {
        return nil, nil, fmt.Errorf("create session: %w", err)
    }

    var panes []db.Pane
    for i, cmd := range job.Commands {
        paneID := uuid.New().String()
        outputPath := filepath.Join(m.dataDir, "panes", paneID+".log")

        // Create the file now so it exists for reconnects even before output
        if f, err := os.Create(outputPath); err == nil {
            f.Close()
        }

        substituted := substituteVars(cmd.Template, vars)

        pane := &db.Pane{
            ID:         paneID,
            SessionID:  sess.ID,
            CmdIndex:   i,
            Alive:      true,
            OutputPath: outputPath,
        }
        if err := m.store.CreatePane(pane); err != nil {
            return nil, nil, fmt.Errorf("create pane: %w", err)
        }

        if _, err := m.ptyMgr.Spawn(paneID, substituted, outputPath); err != nil {
            m.store.SetPaneAlive(paneID, false)
        } else {
            m.ptyMgr.OnExit = func(id string) {
                m.store.SetPaneAlive(id, false)
            }
        }

        pane.Alive = m.ptyMgr.IsAlive(paneID)
        panes = append(panes, *pane)
    }

    return sess, panes, nil
}

// Get retrieves a session and its panes from the store.
func (m *Manager) Get(sessionID string) (*db.Session, []db.Pane, error) {
    sess, err := m.store.GetSession(sessionID)
    if err != nil {
        return nil, nil, err
    }
    panes, err := m.store.ListPanes(sessionID)
    if err != nil {
        return nil, nil, err
    }
    // Sync alive status from PTY manager
    for i := range panes {
        panes[i].Alive = m.ptyMgr.IsAlive(panes[i].ID)
    }
    return sess, panes, nil
}

func substituteVars(template string, vars map[string]string) string {
    result := template
    for k, v := range vars {
        old := "{{" + k + "}}"
        newS := v
        for {
            next := replaceFirst(result, old, newS)
            if next == result {
                break
            }
            result = next
        }
    }
    return result
}

func replaceFirst(s, old, new string) string {
    i := indexOf(s, old)
    if i < 0 {
        return s
    }
    return s[:i] + new + s[i+len(old):]
}

func indexOf(s, sub string) int {
    for i := 0; i <= len(s)-len(sub); i++ {
        if s[i:i+len(sub)] == sub {
            return i
        }
    }
    return -1
}
```

> Note: `substituteVars` reimplements substitution locally here rather than importing `validate` to avoid a circular dependency. Both packages are thin — this duplication is intentional.

- [ ] **Step 4: Run tests — verify they pass**

```bash
go test ./internal/session/... -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/session/
git commit -m "feat: session manager — launch sessions, spawn PTYs, reconnect"
```

---

## Task 6: API Handlers (Jobs + Session Launch)

**Files:**
- Modify: `internal/api/handlers.go`
- Create: `internal/api/handlers_test.go`

- [ ] **Step 1: Write failing tests**

`internal/api/handlers_test.go`:
```go
package api_test

import (
    "bytes"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"
    "console-web/internal/api"
    "console-web/internal/db"
    "console-web/internal/pty"
    "console-web/internal/session"
)

func newTestHandler(t *testing.T) (*api.Handler, *db.Store) {
    t.Helper()
    store, err := db.Open(":memory:")
    if err != nil {
        t.Fatal(err)
    }
    t.Cleanup(func() { store.Close() })
    dataDir := t.TempDir()
    ptyMgr := pty.NewManager(dataDir, 1024*1024)
    sessMgr := session.NewManager(store, ptyMgr, dataDir)
    return api.NewHandler(store, sessMgr, ptyMgr), store
}

func TestCreateAndGetJob(t *testing.T) {
    h, _ := newTestHandler(t)
    mux := h.Routes()

    body := `{"id":"test","name":"Test","commands":[{"label":"L","template":"echo {{x}}"}],"variables":[{"name":"x","regex":"^\\w+$","description":""}]}`
    req := httptest.NewRequest("POST", "/api/jobs", strings.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    w := httptest.NewRecorder()
    mux.ServeHTTP(w, req)
    if w.Code != http.StatusCreated {
        t.Fatalf("create: %d %s", w.Code, w.Body.String())
    }

    req2 := httptest.NewRequest("GET", "/api/jobs/test", nil)
    w2 := httptest.NewRecorder()
    mux.ServeHTTP(w2, req2)
    if w2.Code != http.StatusOK {
        t.Fatalf("get: %d", w2.Code)
    }
    var got db.Job
    json.NewDecoder(w2.Body).Decode(&got)
    if got.ID != "test" {
        t.Errorf("id: %q", got.ID)
    }
}

func TestListJobs(t *testing.T) {
    h, store := newTestHandler(t)
    mux := h.Routes()
    store.CreateJob(&db.Job{ID: "a", Name: "A", Commands: []db.Command{}, Variables: []db.Variable{}})
    store.CreateJob(&db.Job{ID: "b", Name: "B", Commands: []db.Command{}, Variables: []db.Variable{}})

    req := httptest.NewRequest("GET", "/api/jobs", nil)
    w := httptest.NewRecorder()
    mux.ServeHTTP(w, req)
    if w.Code != http.StatusOK {
        t.Fatalf("list: %d", w.Code)
    }
    var jobs []db.Job
    json.NewDecoder(w.Body).Decode(&jobs)
    if len(jobs) != 2 {
        t.Errorf("expected 2 jobs, got %d", len(jobs))
    }
}

func TestDeleteJob(t *testing.T) {
    h, store := newTestHandler(t)
    mux := h.Routes()
    store.CreateJob(&db.Job{ID: "del", Name: "Del", Commands: []db.Command{}, Variables: []db.Variable{}})

    req := httptest.NewRequest("DELETE", "/api/jobs/del", nil)
    w := httptest.NewRecorder()
    mux.ServeHTTP(w, req)
    if w.Code != http.StatusNoContent {
        t.Fatalf("delete: %d", w.Code)
    }
}

func TestJobLaunchRedirect(t *testing.T) {
    h, store := newTestHandler(t)
    mux := h.Routes()
    store.CreateJob(&db.Job{
        ID:   "greet",
        Name: "Greet",
        Commands: []db.Command{{Label: "Hi", Template: "echo {{name}}"}},
        Variables: []db.Variable{{Name: "name", Regex: `^\w+$`}},
    })

    req := httptest.NewRequest("GET", "/?job=greet&name=world", nil)
    w := httptest.NewRecorder()
    mux.ServeHTTP(w, req)
    if w.Code != http.StatusFound {
        t.Fatalf("expected redirect, got %d: %s", w.Code, w.Body.String())
    }
    loc := w.Header().Get("Location")
    if !strings.Contains(loc, "#session=") {
        t.Errorf("redirect location missing #session=: %q", loc)
    }
}

func TestJobLaunchValidationError(t *testing.T) {
    h, store := newTestHandler(t)
    mux := h.Routes()
    store.CreateJob(&db.Job{
        ID:   "greet",
        Name: "Greet",
        Commands: []db.Command{{Label: "Hi", Template: "echo {{name}}"}},
        Variables: []db.Variable{{Name: "name", Regex: `^\w+$`}},
    })

    req := httptest.NewRequest("GET", "/?job=greet&name=bad name!", nil)
    w := httptest.NewRecorder()
    mux.ServeHTTP(w, req)
    if w.Code != http.StatusUnprocessableEntity {
        t.Fatalf("expected 422, got %d", w.Code)
    }
    if !strings.Contains(w.Body.String(), "name") {
        t.Errorf("error page missing variable name")
    }
}

func TestGetSession(t *testing.T) {
    h, store := newTestHandler(t)
    mux := h.Routes()
    store.CreateJob(&db.Job{
        ID: "j", Name: "J",
        Commands:  []db.Command{{Label: "L", Template: "sleep 0"}},
        Variables: []db.Variable{},
    })
    // Launch via HTTP to get a session ID
    req := httptest.NewRequest("GET", "/?job=j", nil)
    w := httptest.NewRecorder()
    mux.ServeHTTP(w, req)
    loc := w.Header().Get("Location")
    sessID := strings.TrimPrefix(loc, "/#session=")

    req2 := httptest.NewRequest("GET", "/api/sessions/"+sessID, nil)
    w2 := httptest.NewRecorder()
    mux.ServeHTTP(w2, req2)
    if w2.Code != http.StatusOK {
        t.Fatalf("get session: %d %s", w2.Code, w2.Body.String())
    }
    var resp map[string]any
    json.NewDecoder(w2.Body).Decode(&resp)
    if resp["id"] != sessID {
        t.Errorf("session id mismatch: %v", resp)
    }
}

func TestUpdateJob(t *testing.T) {
    h, store := newTestHandler(t)
    mux := h.Routes()
    store.CreateJob(&db.Job{ID: "upd", Name: "Old", Commands: []db.Command{}, Variables: []db.Variable{}})

    body, _ := json.Marshal(db.Job{ID: "upd", Name: "New", Commands: []db.Command{}, Variables: []db.Variable{}})
    req := httptest.NewRequest("PUT", "/api/jobs/upd", bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    w := httptest.NewRecorder()
    mux.ServeHTTP(w, req)
    if w.Code != http.StatusOK {
        t.Fatalf("update: %d %s", w.Code, w.Body.String())
    }
    j, _ := store.GetJob("upd")
    if j.Name != "New" {
        t.Errorf("name after update: %q", j.Name)
    }
}
```

- [ ] **Step 2: Run — verify it fails**

```bash
go test ./internal/api/...
```

Expected: compilation error.

- [ ] **Step 3: Implement handlers**

`internal/api/handlers.go`:
```go
package api

import (
    "encoding/json"
    "fmt"
    "html/template"
    "net/http"

    "console-web/internal/db"
    "console-web/internal/pty"
    "console-web/internal/session"
    "console-web/internal/validate"
)

type Handler struct {
    store   *db.Store
    sessions *session.Manager
    ptyMgr  *pty.Manager
}

func NewHandler(store *db.Store, sessions *session.Manager, ptyMgr *pty.Manager) *Handler {
    return &Handler{store: store, sessions: sessions, ptyMgr: ptyMgr}
}

// Routes returns an http.ServeMux with all API and page routes registered.
// The caller is responsible for mounting frontend static assets separately.
func (h *Handler) Routes() *http.ServeMux {
    mux := http.NewServeMux()

    mux.HandleFunc("GET /api/jobs", h.listJobs)
    mux.HandleFunc("POST /api/jobs", h.createJob)
    mux.HandleFunc("GET /api/jobs/{id}", h.getJob)
    mux.HandleFunc("PUT /api/jobs/{id}", h.updateJob)
    mux.HandleFunc("DELETE /api/jobs/{id}", h.deleteJob)

    mux.HandleFunc("GET /api/sessions/{id}", h.getSession)

    mux.HandleFunc("GET /", h.index)

    return mux
}

func writeJSON(w http.ResponseWriter, status int, v any) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(v)
}

func (h *Handler) listJobs(w http.ResponseWriter, r *http.Request) {
    jobs, err := h.store.ListJobs()
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    if jobs == nil {
        jobs = []db.Job{}
    }
    writeJSON(w, http.StatusOK, jobs)
}

func (h *Handler) createJob(w http.ResponseWriter, r *http.Request) {
    var job db.Job
    if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
        http.Error(w, "invalid JSON", http.StatusBadRequest)
        return
    }
    if job.ID == "" {
        http.Error(w, "id required", http.StatusBadRequest)
        return
    }
    if err := h.store.CreateJob(&job); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    writeJSON(w, http.StatusCreated, job)
}

func (h *Handler) getJob(w http.ResponseWriter, r *http.Request) {
    job, err := h.store.GetJob(r.PathValue("id"))
    if err != nil {
        http.Error(w, err.Error(), http.StatusNotFound)
        return
    }
    writeJSON(w, http.StatusOK, job)
}

func (h *Handler) updateJob(w http.ResponseWriter, r *http.Request) {
    var job db.Job
    if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
        http.Error(w, "invalid JSON", http.StatusBadRequest)
        return
    }
    job.ID = r.PathValue("id")
    if err := h.store.UpdateJob(&job); err != nil {
        http.Error(w, err.Error(), http.StatusNotFound)
        return
    }
    writeJSON(w, http.StatusOK, job)
}

func (h *Handler) deleteJob(w http.ResponseWriter, r *http.Request) {
    if err := h.store.DeleteJob(r.PathValue("id")); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) getSession(w http.ResponseWriter, r *http.Request) {
    sess, panes, err := h.sessions.Get(r.PathValue("id"))
    if err != nil {
        http.Error(w, err.Error(), http.StatusNotFound)
        return
    }
    writeJSON(w, http.StatusOK, map[string]any{
        "id":     sess.ID,
        "job_id": sess.JobID,
        "vars":   sess.Vars,
        "panes":  panes,
    })
}

var errPageTmpl = template.Must(template.New("err").Parse(`<!DOCTYPE html>
<html>
<head><meta charset="utf-8"><title>Validation Error</title>
<style>
  body{font-family:monospace;background:#1a1a2e;color:#ccc;padding:40px;max-width:700px;margin:auto}
  h1{color:#ff6b6b}
  table{border-collapse:collapse;width:100%;margin-top:20px}
  th,td{text-align:left;padding:8px 12px;border-bottom:1px solid #333}
  th{color:#7eb8f7}
  .regex{color:#f7c948}
  .value{color:#ff6b6b}
  a{color:#7eb8f7}
</style>
</head>
<body>
<h1>Variable Validation Failed</h1>
<p>Job: <strong>{{.JobID}}</strong></p>
<table>
<tr><th>Variable</th><th>Supplied Value</th><th>Expected Pattern</th></tr>
{{range .Errors}}
<tr><td>{{.Name}}</td><td class="value">{{.Value}}</td><td class="regex">{{.Pattern}}</td></tr>
{{end}}
</table>
<p><a href="/jobs">← Edit Jobs</a></p>
</body>
</html>`))

func (h *Handler) index(w http.ResponseWriter, r *http.Request) {
    jobID := r.URL.Query().Get("job")
    if jobID == "" {
        // Serve main app shell — handled by static file middleware in main.go
        http.NotFound(w, r)
        return
    }

    job, err := h.store.GetJob(jobID)
    if err != nil {
        http.Error(w, fmt.Sprintf("job %q not found", jobID), http.StatusNotFound)
        return
    }

    vars := make(map[string]string)
    for _, v := range job.Variables {
        vars[v.Name] = r.URL.Query().Get(v.Name)
    }

    errs := validate.Vars(job, vars)
    if len(errs) > 0 {
        w.Header().Set("Content-Type", "text/html")
        w.WriteHeader(http.StatusUnprocessableEntity)
        errPageTmpl.Execute(w, map[string]any{"JobID": jobID, "Errors": errs})
        return
    }

    sess, _, err := h.sessions.Launch(jobID, vars)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    http.Redirect(w, r, "/#session="+sess.ID, http.StatusFound)
}
```

- [ ] **Step 4: Run tests — verify they pass**

```bash
go test ./internal/api/... -v
```

Expected: PASS for all seven tests.

- [ ] **Step 5: Commit**

```bash
git add internal/api/handlers.go internal/api/handlers_test.go
git commit -m "feat: REST handlers for jobs, session launch, and validation error page"
```

---

## Task 7: WebSocket Handler

**Files:**
- Modify: `internal/api/ws.go`

No unit tests for the WebSocket handler — it requires a live WebSocket connection. Integration testing happens in Task 10 when the full server runs.

- [ ] **Step 1: Implement WebSocket handler**

`internal/api/ws.go`:
```go
package api

import (
    "encoding/json"
    "log"
    "net/http"

    "github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
    CheckOrigin: func(r *http.Request) bool { return true },
}

type resizeMsg struct {
    Type string `json:"type"`
    Cols uint16 `json:"cols"`
    Rows uint16 `json:"rows"`
}

// RegisterWS adds the WebSocket route to an existing mux.
func (h *Handler) RegisterWS(mux *http.ServeMux) {
    mux.HandleFunc("GET /ws/pane/{id}", h.handleWS)
}

func (h *Handler) handleWS(w http.ResponseWriter, r *http.Request) {
    paneID := r.PathValue("id")

    pane, err := h.store.GetPane(paneID)
    if err != nil {
        http.Error(w, "pane not found", http.StatusNotFound)
        return
    }

    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        log.Printf("ws upgrade: %v", err)
        return
    }
    defer conn.Close()

    sendCh, doneCh, err := h.ptyMgr.AddClient(paneID, conn, pane.OutputPath)
    if err != nil {
        // PTY not running — send scrollback only then close
        conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"exited"}`))
        return
    }
    defer h.ptyMgr.RemoveClient(paneID, conn)

    // Goroutine: drain send channel → write to WebSocket
    writeErr := make(chan error, 1)
    go func() {
        for {
            select {
            case data, ok := <-sendCh:
                if !ok {
                    writeErr <- nil
                    return
                }
                if err := conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
                    writeErr <- err
                    return
                }
            case <-doneCh:
                conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"exited"}`))
                writeErr <- nil
                return
            }
        }
    }()

    // Main loop: read from WebSocket → PTY or resize
    for {
        msgType, data, err := conn.ReadMessage()
        if err != nil {
            break
        }
        switch msgType {
        case websocket.BinaryMessage:
            if err := h.ptyMgr.WriteInput(paneID, data); err != nil {
                log.Printf("pty write %s: %v", paneID, err)
            }
        case websocket.TextMessage:
            var msg resizeMsg
            if err := json.Unmarshal(data, &msg); err == nil && msg.Type == "resize" {
                h.ptyMgr.Resize(paneID, msg.Cols, msg.Rows)
            }
        }
    }
    <-writeErr
}
```

- [ ] **Step 2: Verify it compiles**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/api/ws.go
git commit -m "feat: WebSocket handler — PTY I/O bridge with resize and scrollback replay"
```

---

## Task 8: main.go and Server Wiring

**Files:**
- Modify: `main.go`

- [ ] **Step 1: Implement main.go**

```go
package main

import (
    "embed"
    "flag"
    "io/fs"
    "log"
    "net/http"
    "os"

    "console-web/internal/api"
    "console-web/internal/db"
    "console-web/internal/pty"
    "console-web/internal/session"
)

//go:embed frontend
var frontendFS embed.FS

func main() {
    addr := flag.String("addr", "127.0.0.1:8080", "listen address")
    dbPath := flag.String("db", "./console-web.db", "SQLite database path")
    dataDir := flag.String("data", "./data", "pane scrollback directory")
    maxScrollback := flag.Int64("scrollback", 10*1024*1024, "max scrollback bytes per pane")
    flag.Parse()

    if err := os.MkdirAll(*dataDir, 0755); err != nil {
        log.Fatalf("create data dir: %v", err)
    }

    store, err := db.Open(*dbPath)
    if err != nil {
        log.Fatalf("open db: %v", err)
    }
    defer store.Close()

    ptyMgr := pty.NewManager(*dataDir, *maxScrollback)
    sessMgr := session.NewManager(store, ptyMgr, *dataDir)
    h := api.NewHandler(store, sessMgr, ptyMgr)

    mux := h.Routes()
    h.RegisterWS(mux)

    // Serve frontend static files
    sub, err := fs.Sub(frontendFS, "frontend")
    if err != nil {
        log.Fatalf("frontend fs: %v", err)
    }
    fileServer := http.FileServer(http.FS(sub))

    // Wrap the mux: fall through to file server for GET / (no ?job=) and /jobs
    wrapped := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Let the API mux handle /api/*, /ws/*, and GET / with ?job= param
        // For bare GET / and GET /jobs, serve index.html / editor.html
        if r.URL.Path == "/" && r.URL.RawQuery == "" {
            http.ServeFileFS(w, r, sub, "index.html")
            return
        }
        if r.URL.Path == "/jobs" {
            http.ServeFileFS(w, r, sub, "editor.html")
            return
        }
        // Try API mux first; if it returns 404, try file server
        rw := &statusRecorder{ResponseWriter: w}
        mux.ServeHTTP(rw, r)
        if rw.status == http.StatusNotFound {
            fileServer.ServeHTTP(w, r)
        }
    })

    log.Printf("listening on http://%s", *addr)
    if err := http.ListenAndServe(*addr, wrapped); err != nil {
        log.Fatal(err)
    }
}

type statusRecorder struct {
    http.ResponseWriter
    status  int
    written bool
}

func (r *statusRecorder) WriteHeader(status int) {
    r.status = status
    if status != http.StatusNotFound {
        r.ResponseWriter.WriteHeader(status)
        r.written = true
    }
}

func (r *statusRecorder) Write(b []byte) (int, error) {
    if r.status == http.StatusNotFound {
        return len(b), nil // discard the 404 body
    }
    return r.ResponseWriter.Write(b)
}
```

- [ ] **Step 2: Verify it builds**

```bash
go build -o console-web .
```

Expected: binary produced, no errors.

- [ ] **Step 3: Commit**

```bash
git add main.go
git commit -m "feat: main.go — server wiring, embedded frontend, CLI flags"
```

---

## Task 9: Frontend HTML and CSS

**Files:**
- Modify: `frontend/index.html`
- Modify: `frontend/editor.html`
- Modify: `frontend/style.css`

- [ ] **Step 1: Write style.css**

`frontend/style.css`:
```css
*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }

:root {
  --bg: #1a1a2e;
  --surface: #0d0d1a;
  --surface2: #111124;
  --border: #2d2d4e;
  --text: #ccc;
  --text-dim: #666;
  --accent: #7eb8f7;
  --green: #00ff88;
  --red: #ff6b6b;
  --yellow: #f7c948;
}

body { background: var(--bg); color: var(--text); font-family: system-ui, sans-serif; height: 100vh; display: flex; flex-direction: column; overflow: hidden; }

/* Nav */
nav { background: var(--surface); border-bottom: 1px solid var(--border); padding: 0 16px; height: 40px; display: flex; align-items: center; gap: 20px; flex-shrink: 0; }
nav .brand { color: var(--accent); font-weight: 600; font-size: 14px; text-decoration: none; }
nav a { color: var(--text-dim); font-size: 13px; text-decoration: none; }
nav a:hover { color: var(--text); }

/* Tab bar */
#tab-bar { background: var(--surface2); border-bottom: 1px solid var(--border); display: flex; overflow-x: auto; flex-shrink: 0; }
.tab { padding: 8px 16px; font-size: 12px; font-family: monospace; color: var(--text-dim); cursor: pointer; border-right: 1px solid var(--border); white-space: nowrap; user-select: none; }
.tab:hover { color: var(--text); background: var(--surface); }
.tab.active { color: var(--accent); background: var(--bg); border-bottom: 2px solid var(--accent); }
.tab .status { display: inline-block; width: 6px; height: 6px; border-radius: 50%; background: var(--green); margin-right: 6px; }
.tab .status.dead { background: var(--text-dim); }

/* Terminal area */
#terminal-container { flex: 1; overflow: hidden; position: relative; }
.terminal-pane { position: absolute; inset: 0; display: none; padding: 4px; }
.terminal-pane.active { display: block; }
.exited-banner { color: var(--text-dim); font-family: monospace; font-size: 13px; padding: 20px; }

/* Editor layout */
#editor-layout { display: flex; flex: 1; overflow: hidden; }
#job-list { width: 220px; background: var(--surface); border-right: 1px solid var(--border); display: flex; flex-direction: column; flex-shrink: 0; }
#job-list-items { flex: 1; overflow-y: auto; padding: 8px 0; }
.job-item { padding: 8px 16px; font-size: 13px; cursor: pointer; color: var(--text-dim); }
.job-item:hover { color: var(--text); background: var(--surface2); }
.job-item.active { color: var(--accent); background: var(--bg); }
#new-job-btn { margin: 8px; padding: 8px; background: transparent; border: 1px dashed var(--border); border-radius: 4px; color: var(--green); cursor: pointer; font-size: 12px; }
#new-job-btn:hover { border-color: var(--green); }

/* Editor panel */
#editor-panel { flex: 1; overflow-y: auto; padding: 24px; }
#editor-panel h2 { font-size: 16px; color: var(--accent); margin-bottom: 20px; }
.field-group { margin-bottom: 16px; }
label { display: block; font-size: 11px; color: var(--text-dim); text-transform: uppercase; letter-spacing: .05em; margin-bottom: 4px; }
input[type=text] { width: 100%; background: var(--surface); border: 1px solid var(--border); border-radius: 4px; padding: 7px 10px; color: var(--text); font-size: 13px; font-family: inherit; }
input[type=text]:focus { outline: none; border-color: var(--accent); }
.section-label { font-size: 11px; color: var(--text-dim); text-transform: uppercase; letter-spacing: .05em; margin: 20px 0 8px; display: flex; align-items: center; justify-content: space-between; }
.add-btn { background: none; border: 1px solid var(--border); border-radius: 3px; color: var(--green); padding: 2px 8px; font-size: 11px; cursor: pointer; }
.add-btn:hover { border-color: var(--green); }
.row-item { background: var(--surface2); border: 1px solid var(--border); border-radius: 4px; padding: 10px; margin-bottom: 6px; display: grid; gap: 6px; }
.row-item.cmd-row { grid-template-columns: 1fr 2fr auto; align-items: start; }
.row-item.var-row { grid-template-columns: 1fr 1fr 1fr auto; align-items: start; }
.remove-btn { background: none; border: none; color: var(--text-dim); cursor: pointer; font-size: 16px; padding: 2px 6px; }
.remove-btn:hover { color: var(--red); }
.action-bar { margin-top: 24px; display: flex; gap: 10px; align-items: center; }
.btn { padding: 8px 18px; border-radius: 4px; border: none; font-size: 13px; cursor: pointer; }
.btn-primary { background: var(--accent); color: var(--bg); font-weight: 600; }
.btn-primary:hover { opacity: .9; }
.btn-danger { background: transparent; border: 1px solid var(--red); color: var(--red); }
.btn-danger:hover { background: var(--red); color: var(--bg); }
.btn-copy { background: transparent; border: 1px solid var(--border); color: var(--text-dim); }
.btn-copy:hover { border-color: var(--accent); color: var(--accent); }
.url-preview { font-family: monospace; font-size: 12px; color: var(--yellow); background: var(--surface); border: 1px solid var(--border); border-radius: 4px; padding: 8px 12px; margin-top: 16px; word-break: break-all; }
.empty-state { color: var(--text-dim); font-size: 13px; padding: 40px 0; text-align: center; }
```

- [ ] **Step 2: Write index.html**

`frontend/index.html`:
```html
<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>console-web</title>
<link rel="stylesheet" href="/xterm/xterm.css">
<link rel="stylesheet" href="/style.css">
</head>
<body>
<nav>
  <a class="brand" href="/">console-web</a>
  <a href="/jobs">Jobs</a>
</nav>
<div id="tab-bar"></div>
<div id="terminal-container"></div>
<script src="/xterm/xterm.js"></script>
<script src="/xterm/xterm-addon-fit.js"></script>
<script src="/app.js"></script>
</body>
</html>
```

- [ ] **Step 3: Write editor.html**

`frontend/editor.html`:
```html
<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>Jobs — console-web</title>
<link rel="stylesheet" href="/style.css">
</head>
<body>
<nav>
  <a class="brand" href="/">console-web</a>
  <a href="/jobs" style="color:var(--accent)">Jobs</a>
</nav>
<div id="editor-layout">
  <aside id="job-list">
    <div id="job-list-items"></div>
    <button id="new-job-btn">+ New Job</button>
  </aside>
  <main id="editor-panel">
    <p class="empty-state">Select a job or create a new one.</p>
  </main>
</div>
<script src="/editor.js"></script>
</body>
</html>
```

- [ ] **Step 4: Verify server builds and serves files**

```bash
go build -o console-web . && ./console-web &
curl -s http://127.0.0.1:8080/ | grep -q "console-web" && echo "OK"
kill %1
```

Expected: prints `OK`.

- [ ] **Step 5: Commit**

```bash
git add frontend/index.html frontend/editor.html frontend/style.css
git commit -m "feat: frontend HTML shells and CSS theme"
```

---

## Task 10: Frontend JS — Terminal App (app.js)

**Files:**
- Modify: `frontend/app.js`

- [ ] **Step 1: Implement app.js**

`frontend/app.js`:
```js
(async () => {
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
})();
```

- [ ] **Step 2: Start server and smoke-test**

```bash
go build -o console-web . && ./console-web &
# Create a test job
curl -s -X POST http://127.0.0.1:8080/api/jobs \
  -H 'Content-Type: application/json' \
  -d '{"id":"hello","name":"Hello","commands":[{"label":"Greet","template":"echo hello {{name}} && sleep 30"}],"variables":[{"name":"name","regex":"^\\w+$","description":""}]}'
# Launch it
curl -v http://127.0.0.1:8080/?job=hello&name=world 2>&1 | grep Location
kill %1
```

Expected: `Location: /#session=<some-uuid>`

- [ ] **Step 3: Open in browser and verify terminal works**

```bash
go build -o console-web . && ./console-web
```

Visit `http://127.0.0.1:8080/?job=hello&name=world` — you should be redirected, see a tab labelled "Greet", and the terminal should show `hello world`.

- [ ] **Step 4: Commit**

```bash
git add frontend/app.js
git commit -m "feat: terminal app JS — xterm.js, WebSocket wiring, tab switching, scrollback"
```

---

## Task 11: Frontend JS — Job Editor (editor.js)

**Files:**
- Modify: `frontend/editor.js`

- [ ] **Step 1: Implement editor.js**

`frontend/editor.js`:
```js
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
  if (url) navigator.clipboard.writeText(url);
}

function esc(s) {
  return (s || '').replace(/&/g,'&amp;').replace(/"/g,'&quot;').replace(/</g,'&lt;');
}

document.getElementById('new-job-btn').addEventListener('click', () => {
  currentJob = null;
  renderJobList();
  renderEditor({ id: '', name: '', commands: [], variables: [] });
});

loadJobs();
```

- [ ] **Step 2: Start server and test editor in browser**

```bash
go build -o console-web . && ./console-web
```

Visit `http://127.0.0.1:8080/jobs`. Verify:
- Job list loads
- Creating a new job with a command and variable saves successfully
- URL preview updates as you type
- Copy URL copies to clipboard
- Deleting a job removes it from the list

- [ ] **Step 3: End-to-end test**

1. In the editor, create a job:
   - ID: `ping-check`
   - Name: `Ping Check`
   - Command: label `Ping`, template `ping -c 5 {{host}}`
   - Variable: name `host`, regex `^[\w.-]+$`
2. Copy the URL, replace `__host__` with `localhost`
3. Open the URL — verify a terminal tab appears running `ping -c 5 localhost`
4. Close the browser tab, reopen the URL, verify scrollback replays
5. Try an invalid host (e.g. `bad host!`) — verify the error page lists the failing variable

- [ ] **Step 4: Commit**

```bash
git add frontend/editor.js
git commit -m "feat: job editor UI — CRUD, URL preview, variable/command builder"
```

---

## Task 12: Final Wiring and Cleanup

**Files:**
- Modify: `main.go` (OnExit hook fix)
- Modify: `internal/session/manager.go` (OnExit per-pane fix)

The `OnExit` callback in session/manager.go currently overwrites the hook each iteration. Fix it.

- [ ] **Step 1: Fix OnExit registration in session manager**

In `internal/session/manager.go`, the `Launch` method sets `m.ptyMgr.OnExit` inside a loop, overwriting it each time. Replace with a per-spawn approach.

Replace the `Launch` method's loop body where PTYs are spawned:

```go
    for i, cmd := range job.Commands {
        paneID := uuid.New().String()
        outputPath := filepath.Join(m.dataDir, "panes", paneID+".log")

        if f, err := os.Create(outputPath); err == nil {
            f.Close()
        }

        substituted := substituteVars(cmd.Template, vars)

        pane := &db.Pane{
            ID:         paneID,
            SessionID:  sess.ID,
            CmdIndex:   i,
            Alive:      true,
            OutputPath: outputPath,
        }
        if err := m.store.CreatePane(pane); err != nil {
            return nil, nil, fmt.Errorf("create pane: %w", err)
        }

        capturedID := paneID
        m.ptyMgr.OnExit = func(id string) {
            if id == capturedID {
                m.store.SetPaneAlive(id, false)
            }
        }

        if _, err := m.ptyMgr.Spawn(paneID, substituted, outputPath); err != nil {
            m.store.SetPaneAlive(paneID, false)
        }

        pane.Alive = m.ptyMgr.IsAlive(paneID)
        panes = append(panes, *pane)
    }
```

The real fix is to make `OnExit` support multiple callbacks. Replace the single `OnExit func` in `Manager` with a slice and an `AddExitHook` method:

In `internal/pty/manager.go`, change:
```go
// Manager tracks running PTY panes and their connected WebSocket clients.
type Manager struct {
    mu          sync.Mutex
    panes       map[string]*paneState
    dataDir     string
    maxScrollback int64
    // OnExit is called when a PTY process exits. Optional.
    OnExit func(paneID string)
}

func NewManager(dataDir string, maxScrollback int64) *Manager {
    return &Manager{
        panes:         make(map[string]*paneState),
        dataDir:       dataDir,
        maxScrollback: maxScrollback,
    }
}
```

To:
```go
type Manager struct {
    mu            sync.Mutex
    panes         map[string]*paneState
    dataDir       string
    maxScrollback int64
    exitHooksMu   sync.Mutex
    exitHooks     []func(string)
}

func NewManager(dataDir string, maxScrollback int64) *Manager {
    return &Manager{
        panes:         make(map[string]*paneState),
        dataDir:       dataDir,
        maxScrollback: maxScrollback,
    }
}

func (m *Manager) AddExitHook(fn func(paneID string)) {
    m.exitHooksMu.Lock()
    m.exitHooks = append(m.exitHooks, fn)
    m.exitHooksMu.Unlock()
}
```

And in `readLoop`, replace the `OnExit` call:
```go
    defer func() {
        outFile.Close()
        ps.ptmx.Close()
        close(ps.done)
        m.mu.Lock()
        delete(m.panes, paneID)
        m.mu.Unlock()
        m.exitHooksMu.Lock()
        hooks := make([]func(string), len(m.exitHooks))
        copy(hooks, m.exitHooks)
        m.exitHooksMu.Unlock()
        for _, fn := range hooks {
            fn(paneID)
        }
    }()
```

In `internal/session/manager.go`, replace the `OnExit` assignment with `AddExitHook` and remove the `OnExit` field usage:
```go
        m.ptyMgr.AddExitHook(func(id string) {
            if id == paneID {
                m.store.SetPaneAlive(id, false)
            }
        })
        if _, err := m.ptyMgr.Spawn(paneID, substituted, outputPath); err != nil {
            m.store.SetPaneAlive(paneID, false)
        }
```

- [ ] **Step 2: Run all tests**

```bash
go test ./... -v
```

Expected: all tests pass.

- [ ] **Step 3: Build final binary**

```bash
go build -o console-web .
```

- [ ] **Step 4: Final commit**

```bash
git add -A
git commit -m "fix: use AddExitHook instead of single OnExit to support multiple panes"
```

---

## Done

The server is a single `./console-web` binary. Run it, visit `http://127.0.0.1:8080/jobs` to create job configs, then open `/?job=<id>&var=value` URLs to launch terminal sessions.
