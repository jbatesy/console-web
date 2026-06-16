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

	req := httptest.NewRequest("GET", "/?job=greet&name=bad+name!", nil)
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
