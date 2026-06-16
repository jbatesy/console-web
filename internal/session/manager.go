package session

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

		capturedID := paneID
		m.ptyMgr.AddExitHook(func(id string) {
			if id == capturedID {
				m.store.SetPaneAlive(id, false)
			}
		})

		if _, err := m.ptyMgr.Spawn(paneID, substituted, outputPath); err != nil {
			m.store.SetPaneAlive(paneID, false)
			pane.Alive = false
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
		panes[i].Alive = m.ptyMgr.IsAlive(panes[i].ID)
	}
	return sess, panes, nil
}

// substituteVars replaces {{name}} placeholders in template with values from vars.
// Reimplemented locally to avoid circular import with validate package.
func substituteVars(template string, vars map[string]string) string {
	result := template
	for k, v := range vars {
		result = strings.ReplaceAll(result, "{{"+k+"}}", v)
	}
	return result
}
