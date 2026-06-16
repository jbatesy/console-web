package api

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"

	"console-web/internal/db"
	"console-web/internal/pty"
	"console-web/internal/session"
	"console-web/internal/validate"
)

type Handler struct {
	store    *db.Store
	sessions *session.Manager
	ptyMgr   *pty.Manager
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
		http.Error(w, err.Error(), http.StatusNotFound)
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
		if err := errPageTmpl.Execute(w, map[string]any{"JobID": jobID, "Errors": errs}); err != nil {
			log.Printf("error rendering validation error page: %v", err)
		}
		return
	}

	sess, _, err := h.sessions.Launch(jobID, vars)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/#session="+sess.ID, http.StatusFound)
}
