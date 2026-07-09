package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/quantum-6/skillvault/internal/domain"
)

func (s *sqliteWorkflowRunStore) CreateRun(ctx context.Context, run domain.WorkflowRun, steps []domain.WorkflowRunStep) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if run.Status == "" {
		run.Status = domain.RunStatusPending
	}

	now := time.Now()
	run.StartedAt = now

	_, err = tx.ExecContext(ctx, `
		INSERT INTO runs (id, workflow_id, input, output, status, started_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, run.ID, run.WorkflowID, run.Input, run.Output, string(run.Status), run.StartedAt)
	if err != nil {
		return fmt.Errorf("insert run: %w", err)
	}

	for _, step := range steps {
		if step.Status == "" {
			step.Status = domain.RunStatusPending
		}
		step.StartedAt = now
		_, err := tx.ExecContext(ctx, `
			INSERT INTO run_steps (id, run_id, step_id, entry_id, input, output, status, started_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, step.ID, step.RunID, step.StepID, step.EntryID, step.Input, step.Output, string(step.Status), step.StartedAt)
		if err != nil {
			return fmt.Errorf("insert run step %s: %w", step.ID, err)
		}
	}

	return tx.Commit()
}

func (s *sqliteWorkflowRunStore) GetRun(ctx context.Context, id string) (domain.WorkflowRun, []domain.WorkflowRunStep, error) {
	var run domain.WorkflowRun
	var finishedAt sql.NullTime

	err := s.db.QueryRowContext(ctx, `
		SELECT id, workflow_id, input, COALESCE(output,''), status, started_at, finished_at
		FROM runs WHERE id = ?
	`, id).Scan(&run.ID, &run.WorkflowID, &run.Input, &run.Output, &run.Status, &run.StartedAt, &finishedAt)
	if err == sql.ErrNoRows {
		return run, nil, fmt.Errorf("run %q not found", id)
	}
	if err != nil {
		return run, nil, fmt.Errorf("get run: %w", err)
	}
	if finishedAt.Valid {
		run.FinishedAt = &finishedAt.Time
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, run_id, step_id, entry_id, COALESCE(input,''), COALESCE(output,''), status, started_at, finished_at
		FROM run_steps WHERE run_id = ? ORDER BY started_at
	`, id)
	if err != nil {
		return run, nil, fmt.Errorf("get run steps: %w", err)
	}
	defer rows.Close()

	var steps []domain.WorkflowRunStep
	for rows.Next() {
		var step domain.WorkflowRunStep
		var stepFinishedAt sql.NullTime
		if err := rows.Scan(&step.ID, &step.RunID, &step.StepID, &step.EntryID,
			&step.Input, &step.Output, &step.Status, &step.StartedAt, &stepFinishedAt); err != nil {
			return run, nil, fmt.Errorf("scan run step: %w", err)
		}
		if stepFinishedAt.Valid {
			step.FinishedAt = &stepFinishedAt.Time
		}
		steps = append(steps, step)
	}

	if steps == nil {
		steps = []domain.WorkflowRunStep{}
	}
	return run, steps, nil
}

func (s *sqliteWorkflowRunStore) ListRuns(ctx context.Context, workflowID string, limit int) ([]domain.WorkflowRun, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, workflow_id, COALESCE(input,''), COALESCE(output,''), status, started_at, finished_at
		FROM runs WHERE workflow_id = ? ORDER BY started_at DESC LIMIT ?
	`, workflowID, limit)
	if err != nil {
		return nil, fmt.Errorf("list runs: %w", err)
	}
	defer rows.Close()

	var runs []domain.WorkflowRun
	for rows.Next() {
		var run domain.WorkflowRun
		var finishedAt sql.NullTime
		if err := rows.Scan(&run.ID, &run.WorkflowID, &run.Input, &run.Output,
			&run.Status, &run.StartedAt, &finishedAt); err != nil {
			return nil, fmt.Errorf("scan run: %w", err)
		}
		if finishedAt.Valid {
			run.FinishedAt = &finishedAt.Time
		}
		runs = append(runs, run)
	}

	if runs == nil {
		runs = []domain.WorkflowRun{}
	}
	return runs, nil
}

func (s *sqliteWorkflowRunStore) UpdateStepStatus(ctx context.Context, stepID string, status domain.RunStatus, output string) error {
	now := time.Now()
	_, err := s.db.ExecContext(ctx, `
		UPDATE run_steps SET status = ?, output = ?, finished_at = ? WHERE id = ?
	`, string(status), output, now, stepID)
	if err != nil {
		return fmt.Errorf("update step status: %w", err)
	}
	return nil
}

func (s *sqliteWorkflowRunStore) UpdateRunStatus(ctx context.Context, runID string, status domain.RunStatus, output string) error {
	now := time.Now()
	_, err := s.db.ExecContext(ctx, `
		UPDATE runs SET status = ?, output = ?, finished_at = ? WHERE id = ?
	`, string(status), output, now, runID)
	if err != nil {
		return fmt.Errorf("update run status: %w", err)
	}
	return nil
}

// GetRunStats returns aggregate run analytics, optionally filtered by workflow ID.
func (s *sqliteWorkflowRunStore) GetRunStats(ctx context.Context, workflowID *string) (*WorkflowRunStats, error) {
	stats := &WorkflowRunStats{}

	// Aggregate: total, completed, failed, avg/max/min duration (only finished runs for duration).
	err := s.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*),
			COALESCE(SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'failed'    THEN 1 ELSE 0 END), 0),
			COALESCE(AVG(CASE WHEN finished_at IS NOT NULL THEN julianday(finished_at) - julianday(started_at) END) * 86400, 0),
			COALESCE(MAX(CASE WHEN finished_at IS NOT NULL THEN julianday(finished_at) - julianday(started_at) END) * 86400, 0),
			COALESCE(MIN(CASE WHEN finished_at IS NOT NULL THEN julianday(finished_at) - julianday(started_at) END) * 86400, 0)
		FROM runs
		WHERE (? IS NULL OR workflow_id = ?)
	`, workflowID, workflowID).Scan(
		&stats.TotalRuns, &stats.CompletedRuns, &stats.FailedRuns,
		&stats.AvgDurationSecs, &stats.MaxDurationSecs, &stats.MinDurationSecs,
	)
	if err != nil {
		return nil, fmt.Errorf("get run stats: %w", err)
	}

	// Failed step count across all runs (respects workflow filter via runs subquery).
	err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM run_steps
		WHERE status = 'failed'
		  AND run_id IN (SELECT id FROM runs WHERE (? IS NULL OR workflow_id = ?))
	`, workflowID, workflowID).Scan(&stats.FailedStepCount)
	if err != nil {
		return nil, fmt.Errorf("get failed step count: %w", err)
	}

	// Per-workflow breakdown.
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			workflow_id,
			COUNT(*),
			COALESCE(SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END), 0),
			COALESCE(AVG(CASE WHEN finished_at IS NOT NULL THEN julianday(finished_at) - julianday(started_at) END) * 86400, 0)
		FROM runs
		GROUP BY workflow_id
		ORDER BY workflow_id
	`)
	if err != nil {
		return nil, fmt.Errorf("get per-workflow stats: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var pw WorkflowRunPerWorkflow
		if err := rows.Scan(&pw.WorkflowID, &pw.TotalRuns, &pw.CompletedRuns, &pw.AvgDurationSecs); err != nil {
			return nil, fmt.Errorf("scan per-workflow: %w", err)
		}
		stats.PerWorkflow = append(stats.PerWorkflow, pw)
	}

	return stats, nil
}

// ListAllRuns returns runs with step completion progress, optionally filtered by workflow ID.
func (s *sqliteWorkflowRunStore) ListAllRuns(ctx context.Context, workflowID *string, limit, offset int) ([]domain.WorkflowRun, []RunProgress, error) {
	// Query runs with progress via LEFT JOIN on a step-aggregate subquery.
	type runWithProgress struct {
		domain.WorkflowRun
		FinishedAt     sql.NullTime
		CompletedSteps int
		TotalSteps     int
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT
			r.id, r.workflow_id,
			COALESCE(r.input, ''), COALESCE(r.output, ''),
			r.status, r.started_at, r.finished_at,
			COALESCE(rs_progress.completed, 0) AS completed_steps,
			COALESCE(rs_progress.total, 0)   AS total_steps
		FROM runs r
		LEFT JOIN (
			SELECT run_id,
				SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END) AS completed,
				COUNT(*) AS total
			FROM run_steps
			GROUP BY run_id
		) rs_progress ON r.id = rs_progress.run_id
		WHERE (? IS NULL OR r.workflow_id = ?)
		ORDER BY r.started_at DESC
		LIMIT ? OFFSET ?
	`, workflowID, workflowID, limit, offset)
	if err != nil {
		return nil, nil, fmt.Errorf("list all runs: %w", err)
	}
	defer rows.Close()

	var runs []domain.WorkflowRun
	var progresses []RunProgress

	for rows.Next() {
		var rwp runWithProgress
		if err := rows.Scan(
			&rwp.ID, &rwp.WorkflowID,
			&rwp.Input, &rwp.Output,
			&rwp.Status, &rwp.StartedAt, &rwp.FinishedAt,
			&rwp.CompletedSteps, &rwp.TotalSteps,
		); err != nil {
			return nil, nil, fmt.Errorf("scan run with progress: %w", err)
		}
		run := rwp.WorkflowRun
		if rwp.FinishedAt.Valid {
			run.FinishedAt = &rwp.FinishedAt.Time
		}
		runs = append(runs, run)
		progresses = append(progresses, RunProgress{
			RunID:          run.ID,
			CompletedSteps: rwp.CompletedSteps,
			TotalSteps:     rwp.TotalSteps,
		})
	}

	if runs == nil {
		runs = []domain.WorkflowRun{}
		progresses = []RunProgress{}
	}

	return runs, progresses, nil
}
