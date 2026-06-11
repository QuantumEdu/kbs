package db

import (
	"context"
	"fmt"

	"github.com/quantum-6/skillvault/internal/domain"
)

// UpsertWorkflowSteps replaces all workflow steps for an entry with the given steps.
// Steps are stored with sequential step_num starting from the values provided.
func (s *sqliteWorkflowStore) UpsertWorkflowSteps(ctx context.Context, entryID string, steps []domain.WorkflowStep) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "DELETE FROM workflow_steps WHERE entry_id = ?", entryID); err != nil {
		return fmt.Errorf("delete old steps: %w", err)
	}

	for _, step := range steps {
		_, err := tx.ExecContext(ctx,
			"INSERT INTO workflow_steps (entry_id, step_num, role, content, label) VALUES (?, ?, ?, ?, ?)",
			entryID, step.StepNum, string(step.Role), step.Content, step.Label,
		)
		if err != nil {
			return fmt.Errorf("insert step %d: %w", step.StepNum, err)
		}
	}

	return tx.Commit()
}

// GetWorkflowSteps returns all steps for a workflow entry, ordered by step_num.
func (s *sqliteWorkflowStore) GetWorkflowSteps(ctx context.Context, entryID string) ([]domain.WorkflowStep, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, entry_id, step_num, role, content, COALESCE(label,'') FROM workflow_steps WHERE entry_id = ? ORDER BY step_num",
		entryID)
	if err != nil {
		return nil, fmt.Errorf("get workflow steps: %w", err)
	}
	defer rows.Close()

	var steps []domain.WorkflowStep
	for rows.Next() {
		var step domain.WorkflowStep
		if err := rows.Scan(&step.ID, &step.EntryID, &step.StepNum, &step.Role, &step.Content, &step.Label); err != nil {
			return nil, fmt.Errorf("scan step: %w", err)
		}
		steps = append(steps, step)
	}

	if steps == nil {
		steps = []domain.WorkflowStep{}
	}
	return steps, nil
}

var _ WorkflowStore = (*sqliteWorkflowStore)(nil)
