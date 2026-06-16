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
