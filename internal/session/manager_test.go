package session_test

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

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
	mgr := session.NewManager(store, ptyMgr, dataDir, session.DefaultRetention)

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

func TestLaunchSkipsMiddleCommand(t *testing.T) {
	store, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	dataDir := t.TempDir()
	ptyMgr := pty.NewManager(dataDir, 1024*1024)
	mgr := session.NewManager(store, ptyMgr, dataDir, session.DefaultRetention)

	job := &db.Job{
		ID:   "skip",
		Name: "Skip",
		Commands: []db.Command{
			{Label: "First", Template: "echo first"},
			{
				Label: "Middle",
				Branches: []db.CommandBranch{
					{When: map[string]string{"env": "^prod$"}, Template: "echo middle"},
				},
			},
			{Label: "Third", Template: "echo third"},
		},
		Variables: []db.Variable{{Name: "env", Regex: `^(prod|dev)$`}},
	}
	if err := store.CreateJob(job); err != nil {
		t.Fatal(err)
	}

	sess, panes, err := mgr.Launch("skip", map[string]string{"env": "dev"})
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	if sess.JobID != "skip" {
		t.Errorf("job_id: %q", sess.JobID)
	}
	if len(panes) != 2 {
		t.Fatalf("expected 2 panes, got %d", len(panes))
	}
	if panes[0].CmdIndex != 0 {
		t.Errorf("first pane cmd_index: %d", panes[0].CmdIndex)
	}
	if panes[1].CmdIndex != 2 {
		t.Errorf("second pane cmd_index: %d", panes[1].CmdIndex)
	}
}

func TestLaunchAllCommandsSkipped(t *testing.T) {
	store, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	dataDir := t.TempDir()
	ptyMgr := pty.NewManager(dataDir, 1024*1024)
	mgr := session.NewManager(store, ptyMgr, dataDir, session.DefaultRetention)

	job := &db.Job{
		ID:   "none",
		Name: "None",
		Commands: []db.Command{{
			Branches: []db.CommandBranch{
				{When: map[string]string{"env": "^prod$"}, Template: "echo prod"},
			},
		}},
		Variables: []db.Variable{{Name: "env", Regex: `^(prod|dev)$`}},
	}
	if err := store.CreateJob(job); err != nil {
		t.Fatal(err)
	}

	_, _, err = mgr.Launch("none", map[string]string{"env": "dev"})
	if err == nil {
		t.Fatal("expected error when all commands skipped")
	}
}

func TestGet(t *testing.T) {
	store, _ := db.Open(":memory:")
	defer store.Close()
	dataDir := t.TempDir()
	ptyMgr := pty.NewManager(dataDir, 1024*1024)
	mgr := session.NewManager(store, ptyMgr, dataDir, session.DefaultRetention)

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

// waitDead blocks until the pane's process has exited and been recorded.
func waitDead(t *testing.T, mgr *session.Manager, sessID string) db.Pane {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		_, panes, err := mgr.Get(sessID)
		if err != nil {
			t.Fatal(err)
		}
		if !panes[0].Alive && panes[0].EndedAt > 0 {
			return panes[0]
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("pane never ended")
	return db.Pane{}
}

func launchEcho(t *testing.T, retention time.Duration) (*session.Manager, *db.Session) {
	t.Helper()
	store, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })

	dataDir := t.TempDir()
	mgr := session.NewManager(store, pty.NewManager(dataDir, 1024*1024), dataDir, retention)
	if err := store.CreateJob(&db.Job{
		ID:       "echo",
		Name:     "Echo",
		Commands: []db.Command{{Label: "Echo", Template: "echo kept-output"}},
	}); err != nil {
		t.Fatal(err)
	}
	sess, _, err := mgr.Launch("echo", map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	return mgr, sess
}

func TestOutputRetainedAfterExit(t *testing.T) {
	mgr, sess := launchEcho(t, time.Hour)
	pane := waitDead(t, mgr, sess.ID)

	if pane.Expired {
		t.Error("pane expired immediately")
	}
	if want := pane.EndedAt + 3600; pane.ExpiresAt != want {
		t.Errorf("expires_at = %d, want %d", pane.ExpiresAt, want)
	}
	path, err := mgr.PaneOutput(pane.ID)
	if err != nil {
		t.Fatalf("PaneOutput: %v", err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "kept-output") {
		t.Errorf("output = %q", data)
	}

	// Sweeping inside the window must keep the file.
	mgr.Sweep()
	if _, err := os.Stat(path); err != nil {
		t.Errorf("output removed within retention: %v", err)
	}
}

func TestOutputRemovedAfterRetention(t *testing.T) {
	mgr, sess := launchEcho(t, 0)
	pane := waitDead(t, mgr, sess.ID)

	if !pane.Expired {
		t.Error("pane not expired with zero retention")
	}
	if _, err := mgr.PaneOutput(pane.ID); !errors.Is(err, session.ErrOutputExpired) {
		t.Errorf("PaneOutput err = %v, want ErrOutputExpired", err)
	}
	mgr.Sweep()
	if _, err := os.Stat(pane.OutputPath); !os.IsNotExist(err) {
		t.Errorf("output file still present after sweep: %v", err)
	}
}
