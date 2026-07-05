package session

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"console-web/internal/db"
	"console-web/internal/pty"
	"console-web/internal/validate"

	"github.com/google/uuid"
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
	}
	return sess, panes, nil
}
