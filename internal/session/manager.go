package session

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"console-web/internal/db"
	"console-web/internal/pty"
	"console-web/internal/validate"

	"github.com/google/uuid"
)

// DefaultRetention is how long a pane's output stays viewable after its
// process terminates.
const DefaultRetention = time.Hour

// ErrOutputExpired means the pane's retained output has been discarded.
var ErrOutputExpired = errors.New("pane output expired")

type Manager struct {
	store     *db.Store
	ptyMgr    *pty.Manager
	dataDir   string
	retention time.Duration
	now       func() time.Time
}

func NewManager(store *db.Store, ptyMgr *pty.Manager, dataDir string, retention time.Duration) *Manager {
	// Record the end time as soon as a process terminates so retention counts
	// from then, not from the next time someone looks at the session.
	ptyMgr.SetOnExit(func(paneID string) {
		if err := store.SetPaneAlive(paneID, false); err != nil {
			log.Printf("mark pane %s ended: %v", paneID, err)
		}
	})
	return &Manager{store: store, ptyMgr: ptyMgr, dataDir: dataDir, retention: retention, now: time.Now}
}

// annotate fills the derived expiry fields of p.
func (m *Manager) annotate(p *db.Pane) {
	p.ExpiresAt, p.Expired = 0, false
	if p.Alive {
		return
	}
	if p.EndedAt > 0 {
		p.ExpiresAt = p.EndedAt + int64(m.retention/time.Second)
	}
	p.Expired = p.OutputPath == "" || p.EndedAt == 0 || m.now().Unix() >= p.ExpiresAt
}

// PaneOutput returns the path of a pane's retained output file, or
// ErrOutputExpired once it is past the retention window.
func (m *Manager) PaneOutput(paneID string) (string, error) {
	p, err := m.store.GetPane(paneID)
	if err != nil {
		return "", err
	}
	p.Alive = m.ptyMgr.IsAlive(paneID)
	m.annotate(p)
	if p.Expired {
		return "", ErrOutputExpired
	}
	return p.OutputPath, nil
}

// Sweep deletes output files of panes that ended more than the retention
// period ago.
func (m *Manager) Sweep() {
	panes, err := m.store.ListExpiredPanes(m.now().Add(-m.retention).Unix())
	if err != nil {
		log.Printf("sweep panes: %v", err)
		return
	}
	panesDir := filepath.Join(m.dataDir, "panes")
	for _, p := range panes {
		// Only ever delete inside the panes directory.
		if filepath.Dir(p.OutputPath) == panesDir {
			if err := os.Remove(p.OutputPath); err != nil && !os.IsNotExist(err) {
				log.Printf("remove %s: %v", p.OutputPath, err)
				continue
			}
		}
		if err := m.store.ClearPaneOutput(p.ID); err != nil {
			log.Printf("clear pane output %s: %v", p.ID, err)
		}
	}
}

// RunJanitor sweeps expired output until ctx is cancelled.
func (m *Manager) RunJanitor(ctx context.Context, interval time.Duration) {
	m.Sweep()
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			m.Sweep()
		}
	}
}

// Launch creates a session for the given job+vars, spawns PTYs, and returns
// the session and its panes. Caller must have already validated vars.
func (m *Manager) Launch(jobID string, vars map[string]string) (*db.Session, []db.Pane, error) {
	job, err := m.store.GetJob(jobID)
	if err != nil {
		return nil, nil, fmt.Errorf("job %q: %w", jobID, err)
	}

	type resolved struct {
		cmdIndex int
		template string
	}
	var resolvedCmds []resolved
	for i, cmd := range job.Commands {
		tmpl, ok, err := validate.ResolveCommand(cmd, vars)
		if err != nil {
			return nil, nil, fmt.Errorf("resolve command %d: %w", i, err)
		}
		if ok {
			resolvedCmds = append(resolvedCmds, resolved{cmdIndex: i, template: tmpl})
		}
	}
	if len(resolvedCmds) == 0 {
		return nil, nil, validate.ErrNoMatchingCommands
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
	for _, rc := range resolvedCmds {
		paneID := uuid.New().String()
		outputPath := filepath.Join(m.dataDir, "panes", paneID+".log")

		// Create the file now so it exists for reconnects even before output
		if f, err := os.Create(outputPath); err == nil {
			f.Close()
		}

		substituted := validate.Substitute(rc.template, vars)

		pane := &db.Pane{
			ID:         paneID,
			SessionID:  sess.ID,
			CmdIndex:   rc.cmdIndex,
			Alive:      true,
			OutputPath: outputPath,
			PID:        0, // will be updated after spawn
		}
		if err := m.store.CreatePane(pane); err != nil {
			return nil, nil, fmt.Errorf("create pane: %w", err)
		}

		if _, pid, perr := m.ptyMgr.Spawn(paneID, substituted, outputPath); perr != nil {
			m.store.SetPaneAlive(paneID, false)
			pane.Alive = false
		} else {
			pane.PID = pid
			m.store.SetPanePID(pane.ID, pane.PID)
		}

		panes = append(panes, *pane)
	}

	return sess, panes, nil
}

// Get retrieves a session and its panes from the store, syncing alive status from PTY manager.
func (m *Manager) Get(sessionID string) (*db.Session, []db.Pane, error) {
	sess, err := m.store.GetSession(sessionID)
	if err != nil {
		return nil, nil, err
	}
	panes, err := m.store.ListPanes(sessionID)
	if err != nil {
		return nil, nil, err
	}
	// Sync alive status from PTY manager (more accurate than DB for live panes)
	for i := range panes {
		live := m.ptyMgr.IsAlive(panes[i].ID)
		if panes[i].Alive && !live {
			m.store.SetPaneAlive(panes[i].ID, false)
		}
		panes[i].Alive = live
		if !live && panes[i].EndedAt == 0 {
			if p, err := m.store.GetPane(panes[i].ID); err == nil {
				panes[i].EndedAt = p.EndedAt
			}
		}
		m.annotate(&panes[i])
	}
	return sess, panes, nil
}
