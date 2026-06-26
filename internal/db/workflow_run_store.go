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
