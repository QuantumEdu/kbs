package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/quantum-6/skillvault/internal/domain"
)

func (s *sqliteWorkflowStore) Save(ctx context.Context, w domain.Workflow, steps []domain.WorkflowStep) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if w.Status == "" {
		w.Status = domain.StatusActive
	}
	if w.Slug == "" {
		w.Slug = w.Name
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO workflows (id, name, slug, description, status, updated_at)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			name=excluded.name,
			slug=excluded.slug,
			description=excluded.description,
			status=excluded.status,
			updated_at=CURRENT_TIMESTAMP
	`, w.ID, w.Name, w.Slug, w.Description, string(w.Status))
	if err != nil {
		return fmt.Errorf("save workflow: %w", err)
	}

	if _, err := tx.ExecContext(ctx, "DELETE FROM workflow_steps WHERE workflow_id = ?", w.ID); err != nil {
		return fmt.Errorf("delete old steps: %w", err)
	}

	for _, step := range steps {
		required := 0
		if step.Required {
			required = 1
		}
		_, err := tx.ExecContext(ctx,
			`INSERT INTO workflow_steps (workflow_id, order_index, title, instruction, required, expected_output, entry_slug)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			w.ID, step.OrderIndex, step.Title, step.Instruction, required, step.ExpectedOutput, step.EntrySlug)
		if err != nil {
			return fmt.Errorf("insert step %d: %w", step.OrderIndex, err)
		}
	}

	return tx.Commit()
}

func (s *sqliteWorkflowStore) Get(ctx context.Context, id string) (domain.Workflow, error) {
	var w domain.Workflow
	var description sql.NullString
	var status string

	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, slug, description, status
		FROM workflows WHERE id = ? OR slug = ?
	`, id, id).Scan(&w.ID, &w.Name, &w.Slug, &description, &status)
	if err == sql.ErrNoRows {
		return w, fmt.Errorf("workflow %q not found", id)
	}
	if err != nil {
		return w, fmt.Errorf("get workflow: %w", err)
	}

	w.Status = domain.Status(status)
	if description.Valid {
		w.Description = description.String
	}
	return w, nil
}

func (s *sqliteWorkflowStore) GetSteps(ctx context.Context, workflowID string) ([]domain.WorkflowStep, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, workflow_id, order_index, title, instruction, required, COALESCE(expected_output,''), COALESCE(entry_slug,'')
		FROM workflow_steps WHERE workflow_id = ? ORDER BY order_index
	`, workflowID)
	if err != nil {
		return nil, fmt.Errorf("get workflow steps: %w", err)
	}
	defer rows.Close()

	var steps []domain.WorkflowStep
	for rows.Next() {
		var step domain.WorkflowStep
		var stepID int64
		var required int
		if err := rows.Scan(&stepID, &step.WorkflowID, &step.OrderIndex,
			&step.Title, &step.Instruction, &required, &step.ExpectedOutput, &step.EntrySlug); err != nil {
			return nil, fmt.Errorf("scan step: %w", err)
		}
		step.ID = fmt.Sprintf("%d", stepID)
		step.Required = required == 1
		steps = append(steps, step)
	}

	if steps == nil {
		steps = []domain.WorkflowStep{}
	}
	return steps, nil
}

func (s *sqliteWorkflowStore) Render(ctx context.Context, id string) ([]domain.WorkflowStep, error) {
	w, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.GetSteps(ctx, w.ID)
}

func (s *sqliteWorkflowStore) List(ctx context.Context, includeArchived bool) ([]domain.Workflow, error) {
	query := "SELECT id, name, slug, COALESCE(description,''), status FROM workflows"
	if !includeArchived {
		query += " WHERE status != 'archived'"
	}
	query += " ORDER BY name"

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list workflows: %w", err)
	}
	defer rows.Close()

	var results []domain.Workflow
	for rows.Next() {
		var w domain.Workflow
		var description sql.NullString
		var status string
		if err := rows.Scan(&w.ID, &w.Name, &w.Slug, &description, &status); err != nil {
			return nil, fmt.Errorf("scan workflow: %w", err)
		}
		w.Status = domain.Status(status)
		if description.Valid {
			w.Description = description.String
		}
		results = append(results, w)
	}

	if results == nil {
		results = []domain.Workflow{}
	}
	return results, nil
}
