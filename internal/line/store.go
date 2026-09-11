package line

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS jobs (
	id            TEXT PRIMARY KEY,
	issue_url     TEXT NOT NULL,
	repo_path     TEXT NOT NULL,
	phase         TEXT NOT NULL,
	status        TEXT NOT NULL,
	question      TEXT NOT NULL DEFAULT '',
	summary       TEXT NOT NULL DEFAULT '',
	human_answer  TEXT NOT NULL DEFAULT '',
	head_sha      TEXT NOT NULL DEFAULT '',
	pull_request  TEXT NOT NULL DEFAULT '',
	repair_count  INTEGER NOT NULL DEFAULT 0,
	worktree_path TEXT NOT NULL DEFAULT '',
	review_exec   TEXT NOT NULL DEFAULT '',
	review_args   TEXT NOT NULL DEFAULT '',
	created_at    DATETIME NOT NULL,
	updated_at    DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS events (
	id         TEXT PRIMARY KEY,
	job_id     TEXT NOT NULL REFERENCES jobs(id),
	phase      TEXT NOT NULL,
	kind       TEXT NOT NULL,
	detail     TEXT NOT NULL DEFAULT '',
	created_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS projects (
	id           TEXT PRIMARY KEY,
	name         TEXT NOT NULL,
	repo_path    TEXT NOT NULL UNIQUE,
	default_exec TEXT NOT NULL DEFAULT 'claude',
	review_exec  TEXT NOT NULL DEFAULT '',
	ntfy_topic   TEXT NOT NULL DEFAULT '',
	is_active    INTEGER NOT NULL DEFAULT 0,
	created_at   DATETIME NOT NULL,
	updated_at   DATETIME NOT NULL
);
`

type Store struct {
	db *sql.DB
}

func OpenStore(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if _, err := db.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("pragma foreign_keys: %w", err)
	}
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	_, _ = db.Exec(`ALTER TABLE jobs ADD COLUMN worktree_path TEXT NOT NULL DEFAULT '';`)
	_, _ = db.Exec(`ALTER TABLE jobs ADD COLUMN review_exec TEXT NOT NULL DEFAULT '';`)
	_, _ = db.Exec(`ALTER TABLE jobs ADD COLUMN review_args TEXT NOT NULL DEFAULT '';`)
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) CreateJob(ctx context.Context, job Job) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO jobs (
			id, issue_url, repo_path, phase, status, question, summary, human_answer,
			head_sha, pull_request, repair_count, worktree_path, review_exec, review_args,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		job.ID, job.IssueURL, job.RepoPath, job.Phase, job.Status, job.Question, job.Summary, job.HumanAnswer,
		job.HeadSHA, job.PullRequest, job.RepairCount, job.WorktreePath, job.ReviewExec, job.ReviewArgs,
		job.CreatedAt.UTC(), job.UpdatedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("create job: %w", err)
	}
	return nil
}

func (s *Store) UpdateJob(ctx context.Context, job Job) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE jobs SET
			issue_url=?, repo_path=?, phase=?, status=?, question=?, summary=?, human_answer=?,
			head_sha=?, pull_request=?, repair_count=?, worktree_path=?, review_exec=?, review_args=?,
			updated_at=?
		WHERE id=?`,
		job.IssueURL, job.RepoPath, job.Phase, job.Status, job.Question, job.Summary, job.HumanAnswer,
		job.HeadSHA, job.PullRequest, job.RepairCount, job.WorktreePath, job.ReviewExec, job.ReviewArgs,
		job.UpdatedAt.UTC(), job.ID,
	)
	if err != nil {
		return fmt.Errorf("update job: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("job %s not found", job.ID)
	}
	return nil
}

func (s *Store) GetJob(ctx context.Context, id string) (Job, error) {
	return scanJob(s.db.QueryRowContext(ctx, `SELECT `+jobColumns+` FROM jobs WHERE id=?`, id))
}

func (s *Store) LatestJob(ctx context.Context) (Job, error) {
	return scanJob(s.db.QueryRowContext(ctx, `SELECT `+jobColumns+` FROM jobs ORDER BY created_at DESC, id DESC LIMIT 1`))
}

func (s *Store) ListJobs(ctx context.Context) ([]Job, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+jobColumns+` FROM jobs ORDER BY created_at DESC, id DESC`)
	if err != nil {
		return nil, fmt.Errorf("list jobs: %w", err)
	}
	defer rows.Close()
	var jobs []Job
	for rows.Next() {
		job, err := scanJobRow(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func (s *Store) AppendEvent(ctx context.Context, event Event) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO events (id, job_id, phase, kind, detail, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		event.ID, event.JobID, event.Phase, event.Kind, event.Detail, event.CreatedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("append event: %w", err)
	}
	return nil
}

func (s *Store) ListEvents(ctx context.Context, jobID string) ([]Event, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, job_id, phase, kind, detail, created_at
		FROM events WHERE job_id=? ORDER BY created_at ASC, id ASC`, jobID)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()
	var events []Event
	for rows.Next() {
		var e Event
		var created time.Time
		if err := rows.Scan(&e.ID, &e.JobID, &e.Phase, &e.Kind, &e.Detail, &created); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		e.CreatedAt = created.UTC()
		events = append(events, e)
	}
	return events, rows.Err()
}

const jobColumns = `id, issue_url, repo_path, phase, status, question, summary, human_answer, head_sha, pull_request, repair_count, worktree_path, review_exec, review_args, created_at, updated_at`

func scanJob(row *sql.Row) (Job, error) {
	return scanJobRow(row)
}

type scanner interface {
	Scan(dest ...any) error
}

func scanJobRow(row scanner) (Job, error) {
	var job Job
	var created, updated time.Time
	err := row.Scan(
		&job.ID, &job.IssueURL, &job.RepoPath, &job.Phase, &job.Status, &job.Question, &job.Summary, &job.HumanAnswer,
		&job.HeadSHA, &job.PullRequest, &job.RepairCount, &job.WorktreePath, &job.ReviewExec, &job.ReviewArgs,
		&created, &updated,
	)
	if err != nil {
		return Job{}, fmt.Errorf("get job: %w", err)
	}
	job.CreatedAt = created.UTC()
	job.UpdatedAt = updated.UTC()
	return job, nil
}

func (s *Store) CreateProject(ctx context.Context, p Project) error {
	var count int
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM projects`).Scan(&count)
	if count == 0 {
		p.IsActive = true
	}
	if p.IsActive {
		_, _ = s.db.ExecContext(ctx, `UPDATE projects SET is_active = 0`)
	}
	activeInt := 0
	if p.IsActive {
		activeInt = 1
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now().UTC()
	}
	if p.UpdatedAt.IsZero() {
		p.UpdatedAt = p.CreatedAt
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO projects (id, name, repo_path, default_exec, review_exec, ntfy_topic, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.RepoPath, p.DefaultExec, p.ReviewExec, p.NtfyTopic, activeInt, p.CreatedAt.UTC(), p.UpdatedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("create project: %w", err)
	}
	return nil
}

func (s *Store) ListProjects(ctx context.Context) ([]Project, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, repo_path, default_exec, review_exec, ntfy_topic, is_active, created_at, updated_at
		FROM projects ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()
	var projects []Project
	for rows.Next() {
		var p Project
		var activeInt int
		if err := rows.Scan(&p.ID, &p.Name, &p.RepoPath, &p.DefaultExec, &p.ReviewExec, &p.NtfyTopic, &activeInt, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		p.IsActive = (activeInt == 1)
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

func (s *Store) GetProject(ctx context.Context, id string) (Project, error) {
	var p Project
	var activeInt int
	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, repo_path, default_exec, review_exec, ntfy_topic, is_active, created_at, updated_at
		FROM projects WHERE id = ?`, id).Scan(
		&p.ID, &p.Name, &p.RepoPath, &p.DefaultExec, &p.ReviewExec, &p.NtfyTopic, &activeInt, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return Project{}, fmt.Errorf("get project %s: %w", id, err)
	}
	p.IsActive = (activeInt == 1)
	return p, nil
}

func (s *Store) GetActiveProject(ctx context.Context) (Project, error) {
	var p Project
	var activeInt int
	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, repo_path, default_exec, review_exec, ntfy_topic, is_active, created_at, updated_at
		FROM projects WHERE is_active = 1 LIMIT 1`).Scan(
		&p.ID, &p.Name, &p.RepoPath, &p.DefaultExec, &p.ReviewExec, &p.NtfyTopic, &activeInt, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return Project{}, fmt.Errorf("get active project: %w", err)
	}
	p.IsActive = (activeInt == 1)
	return p, nil
}

func (s *Store) SetActiveProject(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE projects SET is_active = 0`); err != nil {
		return err
	}
	res, err := tx.ExecContext(ctx, `UPDATE projects SET is_active = 1, updated_at = ? WHERE id = ?`, time.Now().UTC(), id)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("project %q not found", id)
	}
	return tx.Commit()
}

func (s *Store) DeleteProject(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM projects WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("project %q not found", id)
	}
	return nil
}
