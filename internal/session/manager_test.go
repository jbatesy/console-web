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
