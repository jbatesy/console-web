package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type CommandBranch struct {
	When     map[string]string `json:"when,omitempty"`
	Default  bool              `json:"default,omitempty"`
	Template string            `json:"template"`
}

type Command struct {
	Label    string          `json:"label"`
	Template string          `json:"template,omitempty"`
	Branches []CommandBranch `json:"branches,omitempty"`
}

type Variable struct {
	Name        string `json:"name"`
	Regex       string `json:"regex"`
	Description string `json:"description"`
}

type Job struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Commands  []Command  `json:"commands"`
	Variables []Variable `json:"variables"`
}

type Session struct {
	ID        string            `json:"id"`
	JobID     string            `json:"job_id"`
	Vars      map[string]string `json:"vars"`
	CreatedAt int64             `json:"created_at"`
}

type Pane struct {
	ID         string `json:"id"`
	SessionID  string `json:"session_id"`
	CmdIndex   int    `json:"cmd_index"`
	PID        int    `json:"pid"`
	Alive      bool   `json:"alive"`
	OutputPath string `json:"output_path"`
	// EndedAt is the unix time the process terminated (0 while running).
	EndedAt int64 `json:"ended_at"`
	// ExpiresAt is when the retained output is discarded (0 while running).
	// Expired reports the output is already gone. Both are filled in by the
	// session manager, not stored.
	ExpiresAt int64 `json:"expires_at"`
	Expired   bool  `json:"expired"`
}

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
        PRAGMA journal_mode=WAL;
        PRAGMA foreign_keys=ON;

        CREATE TABLE IF NOT EXISTS jobs (
            id       TEXT PRIMARY KEY,
            name     TEXT NOT NULL,
            commands TEXT NOT NULL DEFAULT '[]',
            variables TEXT NOT NULL DEFAULT '[]'
        );

        CREATE TABLE IF NOT EXISTS sessions (
            id         TEXT PRIMARY KEY,
            job_id     TEXT NOT NULL REFERENCES jobs(id),
            vars       TEXT NOT NULL DEFAULT '{}',
            created_at INTEGER NOT NULL
        );

        CREATE TABLE IF NOT EXISTS panes (
            id          TEXT PRIMARY KEY,
            session_id  TEXT NOT NULL REFERENCES sessions(id),
            cmd_index   INTEGER NOT NULL,
            pid         INTEGER NOT NULL DEFAULT 0,
            alive       BOOLEAN NOT NULL DEFAULT 1,
            output_path TEXT NOT NULL DEFAULT ''
        );
    `)
	if err != nil {
		return err
	}

	// ended_at was added after the first release; upgrade existing databases.
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('panes') WHERE name='ended_at'`).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		_, err = s.db.Exec(`ALTER TABLE panes ADD COLUMN ended_at INTEGER NOT NULL DEFAULT 0`)
	}
	return err
}

// Jobs

func (s *Store) CreateJob(j *Job) error {
	cmds, err := json.Marshal(j.Commands)
	if err != nil {
		return err
	}
	vars, err := json.Marshal(j.Variables)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(
		`INSERT INTO jobs (id, name, commands, variables) VALUES (?,?,?,?)`,
		j.ID, j.Name, string(cmds), string(vars),
	)
	return err
}

func (s *Store) GetJob(id string) (*Job, error) {
	row := s.db.QueryRow(`SELECT id, name, commands, variables FROM jobs WHERE id=?`, id)
	return scanJob(row)
}

func (s *Store) ListJobs() ([]Job, error) {
	rows, err := s.db.Query(`SELECT id, name, commands, variables FROM jobs ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	jobs := []Job{}
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, *j)
	}
	return jobs, rows.Err()
}

func (s *Store) UpdateJob(j *Job) error {
	cmds, err := json.Marshal(j.Commands)
	if err != nil {
		return err
	}
	vars, err := json.Marshal(j.Variables)
	if err != nil {
		return err
	}
	res, err := s.db.Exec(
		`UPDATE jobs SET name=?, commands=?, variables=? WHERE id=?`,
		j.Name, string(cmds), string(vars), j.ID,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("job %q not found", j.ID)
	}
	return nil
}

func (s *Store) DeleteJob(id string) error {
	res, err := s.db.Exec(`DELETE FROM jobs WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("job %q not found", id)
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanJob(r scanner) (*Job, error) {
	var j Job
	var cmds, vars string
	if err := r.Scan(&j.ID, &j.Name, &cmds, &vars); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("job not found")
		}
		return nil, err
	}
	if err := json.Unmarshal([]byte(cmds), &j.Commands); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(vars), &j.Variables); err != nil {
		return nil, err
	}
	if j.Commands == nil {
		j.Commands = []Command{}
	}
	if j.Variables == nil {
		j.Variables = []Variable{}
	}
	return &j, nil
}

// Sessions

func (s *Store) CreateSession(sess *Session) error {
	vars, err := json.Marshal(sess.Vars)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(
		`INSERT INTO sessions (id, job_id, vars, created_at) VALUES (?,?,?,?)`,
		sess.ID, sess.JobID, string(vars), sess.CreatedAt,
	)
	return err
}

func (s *Store) GetSession(id string) (*Session, error) {
	row := s.db.QueryRow(`SELECT id, job_id, vars, created_at FROM sessions WHERE id=?`, id)
	var sess Session
	var varsJSON string
	if err := row.Scan(&sess.ID, &sess.JobID, &varsJSON, &sess.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("session not found")
		}
		return nil, err
	}
	if err := json.Unmarshal([]byte(varsJSON), &sess.Vars); err != nil {
		return nil, err
	}
	if sess.Vars == nil {
		sess.Vars = map[string]string{}
	}
	return &sess, nil
}

// Panes

func (s *Store) CreatePane(p *Pane) error {
	_, err := s.db.Exec(
		`INSERT INTO panes (id, session_id, cmd_index, pid, alive, output_path) VALUES (?,?,?,?,?,?)`,
		p.ID, p.SessionID, p.CmdIndex, p.PID, p.Alive, p.OutputPath,
	)
	return err
}

func (s *Store) GetPane(id string) (*Pane, error) {
	row := s.db.QueryRow(
		`SELECT id, session_id, cmd_index, pid, alive, output_path, ended_at FROM panes WHERE id=?`, id,
	)
	var p Pane
	if err := row.Scan(&p.ID, &p.SessionID, &p.CmdIndex, &p.PID, &p.Alive, &p.OutputPath, &p.EndedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("pane not found")
		}
		return nil, err
	}
	return &p, nil
}

func (s *Store) ListPanes(sessionID string) ([]Pane, error) {
	rows, err := s.db.Query(
		`SELECT id, session_id, cmd_index, pid, alive, output_path, ended_at FROM panes WHERE session_id=? ORDER BY cmd_index`,
		sessionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	panes := []Pane{}
	for rows.Next() {
		var p Pane
		if err := rows.Scan(&p.ID, &p.SessionID, &p.CmdIndex, &p.PID, &p.Alive, &p.OutputPath, &p.EndedAt); err != nil {
			return nil, err
		}
		panes = append(panes, p)
	}
	return panes, rows.Err()
}

// SetPaneAlive flips liveness. Marking a pane dead stamps ended_at (once) so
// its output can be retained for a grace period; marking it alive clears it.
func (s *Store) SetPaneAlive(id string, alive bool) error {
	var res sql.Result
	var err error
	if alive {
		res, err = s.db.Exec(`UPDATE panes SET alive=1, ended_at=0 WHERE id=?`, id)
	} else {
		res, err = s.db.Exec(
			`UPDATE panes SET alive=0, ended_at=CASE WHEN ended_at=0 THEN ? ELSE ended_at END WHERE id=?`,
			time.Now().Unix(), id,
		)
	}
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("pane %q not found", id)
	}
	return nil
}

func (s *Store) SetAllPanesAlive(alive bool) error {
	if alive {
		_, err := s.db.Exec(`UPDATE panes SET alive=1, ended_at=0`)
		return err
	}
	_, err := s.db.Exec(
		`UPDATE panes SET alive=0, ended_at=CASE WHEN ended_at=0 THEN ? ELSE ended_at END`,
		time.Now().Unix(),
	)
	return err
}

// ListExpiredPanes returns dead panes that still reference an output file and
// ended at or before cutoff (unix seconds). Dead panes with no end time
// (pre-retention data) count as expired.
func (s *Store) ListExpiredPanes(cutoff int64) ([]Pane, error) {
	rows, err := s.db.Query(
		`SELECT id, session_id, cmd_index, pid, alive, output_path, ended_at FROM panes
		 WHERE alive=0 AND output_path<>'' AND ended_at<=?`, cutoff,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var panes []Pane
	for rows.Next() {
		var p Pane
		if err := rows.Scan(&p.ID, &p.SessionID, &p.CmdIndex, &p.PID, &p.Alive, &p.OutputPath, &p.EndedAt); err != nil {
			return nil, err
		}
		panes = append(panes, p)
	}
	return panes, rows.Err()
}

// ClearPaneOutput forgets a pane's output file after it has been deleted.
func (s *Store) ClearPaneOutput(id string) error {
	_, err := s.db.Exec(`UPDATE panes SET output_path='' WHERE id=?`, id)
	return err
}

func (s *Store) SetPanePID(id string, pid int) error {
	res, err := s.db.Exec(`UPDATE panes SET pid=? WHERE id=?`, pid, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("pane %q not found", id)
	}
	return nil
}
