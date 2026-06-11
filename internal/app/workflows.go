package app

import (
	"context"
	"fmt"
	"time"

	"github.com/quantum-6/skillvault/internal/db"
	"github.com/quantum-6/skillvault/internal/domain"
	"github.com/quantum-6/skillvault/internal/vars"
)

// WorkflowService orchestrates workflow execution.
type WorkflowService struct {
	entryStore    db.EntryStore
	workflowStore db.WorkflowStore
}

// NewWorkflowService creates a new WorkflowService.
func NewWorkflowService(entryStore db.EntryStore, workflowStore db.WorkflowStore) *WorkflowService {
	return &WorkflowService{entryStore: entryStore, workflowStore: workflowStore}
}

// RunWorkflow renders a workflow by resolving variables in each step.
func (s *WorkflowService) RunWorkflow(ctx context.Context, entryID string, providedVars map[string]string) (domain.RenderedWorkflow, error) {
	var result domain.RenderedWorkflow
	result.EntryID = entryID

	// Get the workflow entry
	entry, err := s.entryStore.GetEntry(ctx, entryID, false)
	if err != nil {
		return result, fmt.Errorf("get workflow entry: %w", err)
	}

	// Validate it's a workflow
	if entry.Entry.Type != domain.EntryTypeWorkflow {
		return result, fmt.Errorf("entry %q is not a workflow (type=%s)", entryID, entry.Entry.Type)
	}

	// Get workflow steps
	steps, err := s.workflowStore.GetWorkflowSteps(ctx, entryID)
	if err != nil {
		return result, fmt.Errorf("get workflow steps: %w", err)
	}

	// Prepare globals
	globals := vars.PrepareGlobals(entry.Entry.ProjectID)
	globals["date"] = time.Now().UTC().Format("2006-01-02")

	// Resolve variables in each step
	for _, step := range steps {
		content, missing := vars.Resolve(step.Content, providedVars, globals)
		result.Steps = append(result.Steps, domain.RenderedStep{
			Role:        step.Role,
			Content:     content,
			Label:       step.Label,
			MissingVars: missing,
		})
	}

	if result.Steps == nil {
		result.Steps = []domain.RenderedStep{}
	}

	return result, nil
}
