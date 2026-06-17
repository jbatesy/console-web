package db_test

import (
	"console-web/internal/db"
	"testing"
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
