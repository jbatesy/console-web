package pty

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"

	cpty "github.com/creack/pty"
	"github.com/gorilla/websocket"
)

type client struct {
	conn *websocket.Conn
	send chan []byte
}

type paneState struct {
	mu      sync.Mutex
	ptmx    *os.File
	clients map[*websocket.Conn]*client
	done    chan struct{}
}

// Manager tracks running PTY panes and their connected WebSocket clients.
type Manager struct {
	mu            sync.Mutex
	panes         map[string]*paneState
	dataDir       string
	maxScrollback int64
	exitHooksMu   sync.Mutex
	exitHooks     []func(string)
}

func NewManager(dataDir string, maxScrollback int64) *Manager {
	return &Manager{
		panes:         make(map[string]*paneState),
		dataDir:       dataDir,
		maxScrollback: maxScrollback,
	}
}

func (m *Manager) AddExitHook(fn func(paneID string)) {
	m.exitHooksMu.Lock()
	m.exitHooks = append(m.exitHooks, fn)
	m.exitHooksMu.Unlock()
}

// Spawn starts bash -c cmd for paneID, appending output to outputPath.
// Returns paneID on success.
func (m *Manager) Spawn(paneID, cmd, outputPath string) (string, error) {
	c := exec.Command("bash", "-c", cmd)
	ptmx, err := cpty.Start(c)
	if err != nil {
		return "", fmt.Errorf("pty start: %w", err)
	}

	outFile, err := os.OpenFile(outputPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		ptmx.Close()
		return "", fmt.Errorf("open output file: %w", err)
	}

	ps := &paneState{
		ptmx:    ptmx,
		clients: make(map[*websocket.Conn]*client),
		done:    make(chan struct{}),
	}

	m.mu.Lock()
	if _, exists := m.panes[paneID]; exists {
		m.mu.Unlock()
		ptmx.Close()
		outFile.Close()
		return "", fmt.Errorf("pane %q already running", paneID)
	}
	m.panes[paneID] = ps
	m.mu.Unlock()

	go m.readLoop(paneID, ps, outFile, c)

	return paneID, nil
}

func (m *Manager) readLoop(paneID string, ps *paneState, outFile *os.File, cmd *exec.Cmd) {
	defer func() {
		outFile.Close()
		ps.ptmx.Close()
		close(ps.done)
		m.mu.Lock()
		delete(m.panes, paneID)
		m.mu.Unlock()
		m.exitHooksMu.Lock()
		hooks := make([]func(string), len(m.exitHooks))
		copy(hooks, m.exitHooks)
		m.exitHooksMu.Unlock()
		for _, fn := range hooks {
			fn(paneID)
		}
	}()

	buf := make([]byte, 4096)
	for {
		n, err := ps.ptmx.Read(buf)
		if n > 0 {
			chunk := make([]byte, n)
			copy(chunk, buf[:n])

			outFile.Write(chunk)
			outFile.Sync()
			m.maybeTrim(outFile.Name())

			ps.mu.Lock()
			for _, cl := range ps.clients {
				select {
				case cl.send <- chunk:
				default:
				}
			}
			ps.mu.Unlock()
		}
		if err != nil {
			break
		}
	}
	cmd.Wait()
}

func (m *Manager) maybeTrim(path string) {
	info, err := os.Stat(path)
	if err != nil || info.Size() <= m.maxScrollback {
		return
	}
	TrimScrollback(path, m.maxScrollback)
}

// AddClient registers a WebSocket conn to receive output from paneID.
// It first replays the scrollback file, then streams live output.
// Returns a send channel the ws handler should drain, and a done channel
// that closes when the PTY exits.
func (m *Manager) AddClient(paneID string, conn *websocket.Conn, outputPath string) (<-chan []byte, <-chan struct{}, error) {
	m.mu.Lock()
	ps, ok := m.panes[paneID]
	m.mu.Unlock()
	if !ok {
		return nil, nil, fmt.Errorf("pane %q not found or not running", paneID)
	}

	ch := make(chan []byte, 256)
	cl := &client{conn: conn, send: ch}

	ps.mu.Lock()
	ps.clients[conn] = cl
	ps.mu.Unlock()

	// Replay scrollback in a goroutine so we don't hold the lock
	go func() {
		if data, err := os.ReadFile(outputPath); err == nil && len(data) > 0 {
			sentinel := []byte("\r\n\033[33m--- scrollback start ---\033[0m\r\n")
			select {
			case ch <- sentinel:
			case <-ps.done:
				return
			}
			// send in chunks to avoid oversized messages
			for len(data) > 0 {
				sz := 4096
				if sz > len(data) {
					sz = len(data)
				}
				chunk := make([]byte, sz)
				copy(chunk, data[:sz])
				select {
				case ch <- chunk:
				case <-ps.done:
					return
				}
				data = data[sz:]
			}
		}
	}()

	return ch, ps.done, nil
}

// RemoveClient unregisters a WebSocket conn from paneID.
func (m *Manager) RemoveClient(paneID string, conn *websocket.Conn) {
	m.mu.Lock()
	ps, ok := m.panes[paneID]
	m.mu.Unlock()
	if !ok {
		return
	}
	ps.mu.Lock()
	delete(ps.clients, conn)
	ps.mu.Unlock()
}

// WriteInput sends bytes from a client to the PTY stdin.
func (m *Manager) WriteInput(paneID string, data []byte) error {
	m.mu.Lock()
	ps, ok := m.panes[paneID]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("pane %q not found", paneID)
	}
	_, err := ps.ptmx.Write(data)
	return err
}

// Resize sets the PTY terminal size.
func (m *Manager) Resize(paneID string, cols, rows uint16) error {
	m.mu.Lock()
	ps, ok := m.panes[paneID]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("pane %q not found", paneID)
	}
	return cpty.Setsize(ps.ptmx, &cpty.Winsize{Cols: cols, Rows: rows})
}

// IsAlive reports whether paneID has a running PTY.
func (m *Manager) IsAlive(paneID string) bool {
	m.mu.Lock()
	_, ok := m.panes[paneID]
	m.mu.Unlock()
	return ok
}

// TrimScrollback is exported for testing. Trims path to its last maxBytes/2 bytes when size > maxBytes.
func TrimScrollback(path string, maxBytes int64) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.Size() <= maxBytes {
		return nil
	}
	f, err := os.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	keep := maxBytes / 2
	offset := info.Size() - keep
	tail := make([]byte, keep)
	if _, err := f.ReadAt(tail, offset); err != nil && err != io.EOF {
		return err
	}
	if err := f.Truncate(0); err != nil {
		return err
	}
	if _, err := f.WriteAt(tail, 0); err != nil {
		return err
	}
	return nil
}
