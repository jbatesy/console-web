package main

import (
	"bufio"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"mime"
	"net"
	"net/http"
	"os"

	"console-web/internal/api"
	"console-web/internal/db"
	"console-web/internal/pty"
	"console-web/internal/session"
)

func init() {
	mime.AddExtensionType(".js", "application/javascript")
	mime.AddExtensionType(".css", "text/css")
}

// all: is required so the Next.js export's _next/ directory (underscore-prefixed,
// which plain go:embed excludes) is bundled into the binary.
//
//go:embed all:frontend/out
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

	// No PTYs survive a restart, so any pane still flagged alive in the DB is
	// stale. Clear them up front before serving.
	if err := store.SetAllPanesAlive(false); err != nil {
		log.Fatalf("reset pane liveness: %v", err)
	}

	ptyMgr := pty.NewManager(*dataDir, *maxScrollback)
	sessMgr := session.NewManager(store, ptyMgr, *dataDir)
	h := api.NewHandler(store, sessMgr, ptyMgr)

	mux := h.Routes()
	h.RegisterWS(mux)

	// Serve the statically-exported Next.js frontend (next build → frontend/out).
	sub, err := fs.Sub(frontendFS, "frontend/out")
	if err != nil {
		log.Fatalf("frontend fs: %v", err)
	}
	fileServer := http.FileServer(http.FS(sub))

	// Wrap the mux: serve the SPA shells for the app routes, let the mux handle
	// /api/*, /ws/*, and GET / with ?job=, and fall through to the file server
	// for hashed assets (/_next/*) and anything else.
	wrapped := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Bare app shell — but not when launching a job (?job=...), which the
		// mux's index handler must validate and redirect.
		if r.URL.Path == "/" && r.URL.Query().Get("job") == "" {
			http.ServeFileFS(w, r, sub, "index.html")
			return
		}
		// Job editor page. trailingSlash:true exports it at out/jobs/index.html;
		// accept both /jobs and /jobs/.
		if r.URL.Path == "/jobs" || r.URL.Path == "/jobs/" {
			http.ServeFileFS(w, r, sub, "jobs/index.html")
			return
		}
		// Try API mux first; if it returns 404, try file server.
		// The mux's 404 handler writes Content-Type and X-Content-Type-Options
		// to the header map before we can intercept it, so clear them first.
		rw := &statusRecorder{ResponseWriter: w}
		mux.ServeHTTP(rw, r)
		if rw.status == http.StatusNotFound {
			w.Header().Del("Content-Type")
			w.Header().Del("X-Content-Type-Options")
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
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	if status != http.StatusNotFound {
		r.ResponseWriter.WriteHeader(status)
	}
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == http.StatusNotFound {
		return len(b), nil // discard the 404 body
	}
	return r.ResponseWriter.Write(b)
}

func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("underlying ResponseWriter does not implement http.Hijacker")
	}
	return h.Hijack()
}
