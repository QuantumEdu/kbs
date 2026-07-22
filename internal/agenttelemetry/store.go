package agenttelemetry

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// ddl is the SQLite DDL executed on OpenStore.
const ddl = `
CREATE TABLE IF NOT EXISTS agent_runs (
	id            TEXT PRIMARY KEY,
	agent_id      TEXT NOT NULL,
	agent_version TEXT NOT NULL DEFAULT '',
	repo_url      TEXT,
	branch        TEXT,
	commit_sha    TEXT,
	workspace     TEXT NOT NULL,
	started_at    DATETIME NOT NULL,
	completed_at  DATETIME,
	status        TEXT NOT NULL DEFAULT 'running'
	              CHECK(status IN ('running','completed','failed')),
	total_tokens  INTEGER DEFAULT 0,
	total_cost_usd REAL DEFAULT 0.0,
	error_type    TEXT,
	error_message TEXT
);

CREATE TABLE IF NOT EXISTS agent_steps (
	id            TEXT PRIMARY KEY,
	run_id        TEXT NOT NULL REFERENCES agent_runs(id),
	step_name     TEXT NOT NULL,
	step_index    INTEGER NOT NULL DEFAULT 0,
	started_at    DATETIME NOT NULL,
	completed_at  DATETIME,
	duration_ms   INTEGER DEFAULT 0
);

CREATE TABLE IF NOT EXISTS tool_calls (
	id            TEXT PRIMARY KEY,
	run_id        TEXT NOT NULL REFERENCES agent_runs(id),
	step_id       TEXT REFERENCES agent_steps(id),
	tool_name     TEXT NOT NULL,
	args_hash     TEXT NOT NULL,
	call_index    INTEGER NOT NULL DEFAULT 0,
	started_at    DATETIME NOT NULL,
	completed_at  DATETIME,
	duration_ms   INTEGER DEFAULT 0,
	success       INTEGER DEFAULT 1,
	error_type    TEXT
);

CREATE TABLE IF NOT EXISTS token_usage (
	id                TEXT PRIMARY KEY,
	run_id            TEXT NOT NULL REFERENCES agent_runs(id),
	step_id           TEXT REFERENCES agent_steps(id),
	model             TEXT NOT NULL,
	input_tokens      INTEGER NOT NULL DEFAULT 0,
	output_tokens     INTEGER NOT NULL DEFAULT 0,
	total_tokens      INTEGER NOT NULL DEFAULT 0,
	cost_usd          REAL DEFAULT 0.0,
	estimation_method TEXT NOT NULL DEFAULT 'char-div-4'
	                  CHECK(estimation_method IN ('api-response','char-div-4','manual')),
	efficiency_ratio  REAL,
	recorded_at       DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS events (
	id              TEXT PRIMARY KEY,
	run_id          TEXT NOT NULL,
	event_type      TEXT NOT NULL,
	timestamp       DATETIME NOT NULL,
	source          TEXT NOT NULL DEFAULT 'plugin',
	correlation_id  TEXT,
	step_id         TEXT,
	redaction_policy TEXT NOT NULL DEFAULT 'hash-args',
	confidence_level TEXT NOT NULL DEFAULT 'measured',
	payload         TEXT NOT NULL,
	created_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_events_run_id ON events(run_id);
CREATE INDEX IF NOT EXISTS idx_events_event_type ON events(event_type);
CREATE INDEX IF NOT EXISTS idx_events_timestamp ON events(timestamp);
CREATE INDEX IF NOT EXISTS idx_agent_runs_status ON agent_runs(status);
`

// Store provides CRUD access to the SQLite telemetry database.
type Store struct {
	db *sql.DB
}

// OpenStore opens the SQLite database at dbPath, creates tables,
// and enables WAL mode. The caller must call Close().
func OpenStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	// WAL mode.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("wal mode: %w", err)
	}
	if _, err := db.Exec("PRAGMA synchronous=NORMAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("synchronous: %w", err)
	}

	// Create tables.
	if _, err := db.Exec(ddl); err != nil {
		db.Close()
		return nil, fmt.Errorf("ddl: %w", err)
	}

	return &Store{db: db}, nil
}

// SaveRun inserts or replaces an agent run record.
func (s *Store) SaveRun(ctx context.Context, r AgentRun) error {
	if s.db == nil {
		return fmt.Errorf("store is closed")
	}
	query := `INSERT OR REPLACE INTO agent_runs
		(id, agent_id, agent_version, repo_url, branch, commit_sha, workspace,
		 started_at, completed_at, status, total_tokens, total_cost_usd,
		 error_type, error_message)
	VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`
	_, err := s.db.ExecContext(ctx, query,
		r.ID, r.AgentID, r.AgentVersion,
		nullString(r.RepoURL), nullString(r.Branch), nullString(r.CommitSHA),
		r.Workspace,
		r.StartedAt.UTC().Format(time.RFC3339),
		nullTime(r.CompletedAt),
		r.Status, r.TotalTokens, r.TotalCostUSD,
		nullString(r.ErrorType), nullString(r.ErrorMessage),
	)
	return err
}

// GetRun retrieves a single agent run by ID.
func (s *Store) GetRun(ctx context.Context, runID string) (AgentRun, error) {
	if s.db == nil {
		return AgentRun{}, fmt.Errorf("store is closed")
	}
	query := `SELECT id, agent_id, agent_version, COALESCE(repo_url,''), COALESCE(branch,''),
		COALESCE(commit_sha,''), workspace, started_at, completed_at, status,
		total_tokens, total_cost_usd, COALESCE(error_type,''), COALESCE(error_message,'')
	FROM agent_runs WHERE id = ?`
	row := s.db.QueryRowContext(ctx, query, runID)

	var r AgentRun
	var startedStr, completedStr sql.NullString
	err := row.Scan(
		&r.ID, &r.AgentID, &r.AgentVersion, &r.RepoURL, &r.Branch,
		&r.CommitSHA, &r.Workspace, &startedStr, &completedStr, &r.Status,
		&r.TotalTokens, &r.TotalCostUSD, &r.ErrorType, &r.ErrorMessage,
	)
	if err != nil {
		return AgentRun{}, err
	}
	r.StartedAt, _ = time.Parse(time.RFC3339, startedStr.String)
	if completedStr.Valid {
		t, _ := time.Parse(time.RFC3339, completedStr.String)
		r.CompletedAt = &t
	}
	return r, nil
}

// ListRuns returns agent runs matching the filter, ordered by started_at DESC.
func (s *Store) ListRuns(ctx context.Context, f RunFilter) ([]AgentRun, error) {
	if s.db == nil {
		return nil, fmt.Errorf("store is closed")
	}
	query := `SELECT id, agent_id, agent_version, COALESCE(repo_url,''), COALESCE(branch,''),
		COALESCE(commit_sha,''), workspace, started_at, completed_at, status,
		total_tokens, total_cost_usd, COALESCE(error_type,''), COALESCE(error_message,'')
	FROM agent_runs`
	var args []interface{}

	if f.AgentID != "" {
		query += " WHERE agent_id = ?"
		args = append(args, f.AgentID)
	}
	query += " ORDER BY started_at DESC"
	if f.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, f.Limit)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []AgentRun
	for rows.Next() {
		var r AgentRun
		var startedStr, completedStr sql.NullString
		if err := rows.Scan(
			&r.ID, &r.AgentID, &r.AgentVersion, &r.RepoURL, &r.Branch,
			&r.CommitSHA, &r.Workspace, &startedStr, &completedStr, &r.Status,
			&r.TotalTokens, &r.TotalCostUSD, &r.ErrorType, &r.ErrorMessage,
		); err != nil {
			return nil, err
		}
		r.StartedAt, _ = time.Parse(time.RFC3339, startedStr.String)
		if completedStr.Valid {
			t, _ := time.Parse(time.RFC3339, completedStr.String)
			r.CompletedAt = &t
		}
		runs = append(runs, r)
	}
	if runs == nil {
		runs = []AgentRun{}
	}
	return runs, rows.Err()
}

// SaveEvent inserts an event record.
func (s *Store) SaveEvent(ctx context.Context, e Event) error {
	if s.db == nil {
		return fmt.Errorf("store is closed")
	}
	query := `INSERT INTO events
		(id, run_id, event_type, timestamp, source, correlation_id, step_id,
		 redaction_policy, confidence_level, payload)
	VALUES (?,?,?,?,?,?,?,?,?,?)`
	_, err := s.db.ExecContext(ctx, query,
		e.EventID, e.RunID, e.EventType,
		e.Timestamp.UTC().Format(time.RFC3339),
		e.Source,
		nullStringPtr(e.CorrelationID), nullStringPtr(e.StepID),
		e.RedactionPolicy, e.ConfidenceLevel,
		string(e.Payload),
	)
	return err
}

// Status returns daemon operational metrics.
func (s *Store) Status(ctx context.Context) (DaemonStatus, error) {
	if s.db == nil {
		return DaemonStatus{}, fmt.Errorf("store is closed")
	}
	var ds DaemonStatus

	row := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM events")
	if err := row.Scan(&ds.EventsIngested); err != nil {
		return ds, err
	}

	// DB size will be populated by the caller (daemon) via os.Stat.
	return ds, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	if s.db == nil {
		return nil
	}
	err := s.db.Close()
	s.db = nil
	return err
}

// nullString returns sql.NullString: valid if s is non-empty.
func nullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

// nullStringPtr returns sql.NullString for a string pointer.
func nullStringPtr(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}

// nullTime returns sql.NullTime for a *time.Time.
func nullTime(t *time.Time) sql.NullString {
	if t == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: t.UTC().Format(time.RFC3339), Valid: true}
}
