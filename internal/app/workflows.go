package app

import (
	"context"
	"fmt"

	"github.com/quantum-6/skillvault/internal/db"
	"github.com/quantum-6/skillvault/internal/domain"
)

type SaveWorkflowStep struct {
	OrderIndex     int
	Title          string
	Instruction    string
	Required       bool
	ExpectedOutput string
	EntrySlug      string
}

type SaveWorkflowInput struct {
	Name        string
	Description string
	Status      string
	Steps       []SaveWorkflowStep
}

type WorkflowService struct {
	workflowStore db.WorkflowStore
}

func NewWorkflowService(workflowStore db.WorkflowStore) *WorkflowService {
	return &WorkflowService{workflowStore: workflowStore}
}

func (s *WorkflowService) SaveWorkflow(ctx context.Context, input SaveWorkflowInput) (*domain.Workflow, error) {
	if input.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	status := domain.StatusActive
	if input.Status != "" {
		if err := domain.ValidateStatus(input.Status); err != nil {
			return nil, fmt.Errorf("validate status: %w", err)
		}
		status = domain.Status(input.Status)
	}

	wf := domain.Workflow{
		ID:          generateWorkflowID(),
		Name:        input.Name,
		Slug:        slugify(input.Name),
		Description: input.Description,
		Status:      status,
	}

	steps := make([]domain.WorkflowStep, 0, len(input.Steps))
	for _, s := range input.Steps {
		orderIdx := s.OrderIndex
		if orderIdx == 0 {
			orderIdx = len(steps) + 1
		}
		steps = append(steps, domain.WorkflowStep{
			WorkflowID:     wf.ID,
			OrderIndex:     orderIdx,
			Title:          s.Title,
			Instruction:    s.Instruction,
			Required:       s.Required,
			ExpectedOutput: s.ExpectedOutput,
			EntrySlug:      s.EntrySlug,
		})
	}

	if err := s.workflowStore.Save(ctx, wf, steps); err != nil {
		return nil, fmt.Errorf("save workflow: %w", err)
	}

	return &wf, nil
}

func (s *WorkflowService) RenderWorkflow(ctx context.Context, id string) ([]domain.WorkflowStep, error) {
	return s.workflowStore.Render(ctx, id)
}

func (s *WorkflowService) ListWorkflows(ctx context.Context, includeArchived bool) ([]domain.Workflow, error) {
	return s.workflowStore.List(ctx, includeArchived)
}

func (s *WorkflowService) Get(ctx context.Context, id string) (domain.Workflow, error) {
	return s.workflowStore.Get(ctx, id)
}
