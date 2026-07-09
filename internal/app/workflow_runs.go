package app

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/quantum-6/skillvault/internal/db"
	"github.com/quantum-6/skillvault/internal/domain"
	"github.com/quantum-6/skillvault/internal/vars"
)

const maxPreviousOutputLen = 32768

// WorkflowRunService executes workflow steps as sequential pipeline steps.
type WorkflowRunService struct {
	workflowStore    db.WorkflowStore
	workflowRunStore db.WorkflowRunStore
	entryStore       db.EntryStore
}

// NewWorkflowRunService creates a new WorkflowRunService.
func NewWorkflowRunService(
	workflowStore db.WorkflowStore,
	workflowRunStore db.WorkflowRunStore,
	entryStore db.EntryStore,
) *WorkflowRunService {
	return &WorkflowRunService{
		workflowStore:    workflowStore,
		workflowRunStore: workflowRunStore,
		entryStore:       entryStore,
	}
}

// RunPipeline executes a workflow as a sequential pipeline.
// It reads step responses from stdin and writes rendered prompts to stdout.
func (s *WorkflowRunService) RunPipeline(
	ctx context.Context,
	workflowSlug string,
	input string,
	stdin io.Reader,
	stdout io.Writer,
) (*domain.WorkflowRun, string, error) {
	// Resolve workflow
	wf, err := s.workflowStore.Get(ctx, workflowSlug)
	if err != nil {
		return nil, "", fmt.Errorf("workflow %q not found: %w", workflowSlug, err)
	}

	steps, err := s.workflowStore.GetSteps(ctx, wf.ID)
	if err != nil {
		return nil, "", fmt.Errorf("get workflow steps: %w", err)
	}

	// Pre-flight: resolve entry_slugs to entry IDs and validate
	type resolvedStep struct {
		step    domain.WorkflowStep
		entryID string
		body    string
	}
	var resolved []resolvedStep
	for _, step := range steps {
		if step.EntrySlug == "" {
			// Renderable-only step, skip during execution
			resolved = append(resolved, resolvedStep{step: step})
			continue
		}
		entryResult, err := s.entryStore.Get(ctx, step.EntrySlug, false)
		if err != nil {
			return nil, "", fmt.Errorf("step %q: entry slug %q: %w", step.Title, step.EntrySlug, err)
		}
		resolved = append(resolved, resolvedStep{
			step:    step,
			entryID: entryResult.Entry.ID,
			body:    entryResult.Entry.BodyOptional,
		})
	}

	// Create run
	runID := generateRunID()
	run := domain.WorkflowRun{
		ID:         runID,
		WorkflowID: wf.ID,
		Input:      input,
		Status:     domain.RunStatusPending,
		StartedAt:  time.Now(),
	}

	var runSteps []domain.WorkflowRunStep
	for _, rs := range resolved {
		if rs.entryID == "" {
			// Renderable-only step, no run step needed
			continue
		}
		stepID := int64(0)
		if rs.step.ID != "" {
			fmt.Sscanf(rs.step.ID, "%d", &stepID)
		}
		runStep := domain.WorkflowRunStep{
			ID:        generateRunStepID(),
			RunID:     runID,
			StepID:    stepID,
			EntryID:   rs.entryID,
			Status:    domain.RunStatusPending,
			StartedAt: time.Now(),
		}
		runSteps = append(runSteps, runStep)
	}

	if err := s.workflowRunStore.CreateRun(ctx, run, runSteps); err != nil {
		return nil, "", fmt.Errorf("create run: %w", err)
	}

	// Execute steps sequentially
	scanner := bufio.NewScanner(stdin)
	var previousOutput string
	var finalOutputParts []string

	for _, rs := range resolved {
		if rs.entryID == "" {
			// Renderable-only step, skip
			continue
		}

		// Find corresponding run step
		var currentRunStep *domain.WorkflowRunStep
		for j := range runSteps {
			if runSteps[j].EntryID == rs.entryID {
				currentRunStep = &runSteps[j]
				break
			}
		}
		if currentRunStep == nil {
			continue
		}

		// Update step to running
		if err := s.workflowRunStore.UpdateStepStatus(ctx, currentRunStep.ID, domain.RunStatusRunning, ""); err != nil {
			s.workflowRunStore.UpdateRunStatus(ctx, runID, domain.RunStatusFailed, err.Error())
			return nil, "", fmt.Errorf("step %q: update to running: %w", rs.step.Title, err)
		}

		// Resolve variables
		truncatedPrev := previousOutput
		truncated := false
		if len(truncatedPrev) > maxPreviousOutputLen {
			truncatedPrev = truncatedPrev[:maxPreviousOutputLen]
			truncated = true
		}

		// Build final_output for variable substitution (accumulated so far)
		currentFinal := strings.Join(finalOutputParts, "\n")

		providedVars := map[string]string{
			"input":           input,
			"previous_output": truncatedPrev,
			"final_output":    currentFinal,
		}

		resolvedBody, missing := vars.Resolve(rs.body, providedVars, nil)

		// Emit truncation warning if needed
		if truncated {
			fmt.Fprintf(stdout, "[WARNING] previous_output truncated from %d to %d characters\n",
				len(previousOutput), maxPreviousOutputLen)
		}

		// Emit missing variable warnings
		for _, m := range missing {
			fmt.Fprintf(stdout, "[WARNING] variable {{%s}} was not resolved\n", m)
		}

		// Write rendered prompt to stdout
		fmt.Fprintln(stdout, resolvedBody)

		// Read response from stdin
		var response string
		if scanner.Scan() {
			response = scanner.Text()
		} else {
			scanErr := scanner.Err()
			if scanErr == nil {
				scanErr = io.EOF
			}
			s.workflowRunStore.UpdateStepStatus(ctx, currentRunStep.ID, domain.RunStatusFailed, scanErr.Error())
			s.workflowRunStore.UpdateRunStatus(ctx, runID, domain.RunStatusFailed, scanErr.Error())
			// Return the partial run so caller can inspect state
			run.Status = domain.RunStatusFailed
			run.Output = strings.Join(finalOutputParts, "\n")
			return &run, run.Output, nil
		}

		// Update step with response
		if err := s.workflowRunStore.UpdateStepStatus(ctx, currentRunStep.ID, domain.RunStatusCompleted, response); err != nil {
			s.workflowRunStore.UpdateRunStatus(ctx, runID, domain.RunStatusFailed, err.Error())
			return nil, "", fmt.Errorf("step %q: update completed: %w", rs.step.Title, err)
		}

		previousOutput = response
		finalOutputParts = append(finalOutputParts, response)
	}

	// Update run as completed
	finalOutput := strings.Join(finalOutputParts, "\n")
	if err := s.workflowRunStore.UpdateRunStatus(ctx, runID, domain.RunStatusCompleted, finalOutput); err != nil {
		return nil, "", fmt.Errorf("update run status: %w", err)
	}

	run.Status = domain.RunStatusCompleted
	run.Output = finalOutput

	return &run, finalOutput, nil
}

// RunPipelineStructured executes a workflow with pre-provided step responses
// instead of reading from stdin. Returns a structured result suitable for
// JSON-RPC responses. Pre-flight validation errors are returned as Go errors.
func (s *WorkflowRunService) RunPipelineStructured(
	ctx context.Context,
	workflowRef string,
	stepInputs map[int]string,
) (*domain.StructuredRunResult, error) {
	// Resolve workflow
	wf, err := s.workflowStore.Get(ctx, workflowRef)
	if err != nil {
		return nil, fmt.Errorf("workflow %q not found: %w", workflowRef, err)
	}

	steps, err := s.workflowStore.GetSteps(ctx, wf.ID)
	if err != nil {
		return nil, fmt.Errorf("get workflow steps: %w", err)
	}

	// Pre-flight: resolve entry_slugs to entry IDs and validate
	type resolvedStep struct {
		step    domain.WorkflowStep
		entryID string
		body    string
	}
	var resolved []resolvedStep
	for _, step := range steps {
		if step.EntrySlug == "" {
			resolved = append(resolved, resolvedStep{step: step})
			continue
		}
		entryResult, err := s.entryStore.Get(ctx, step.EntrySlug, false)
		if err != nil {
			return nil, fmt.Errorf("step %q: entry slug %q: %w", step.Title, step.EntrySlug, err)
		}
		resolved = append(resolved, resolvedStep{
			step:    step,
			entryID: entryResult.Entry.ID,
			body:    entryResult.Entry.BodyOptional,
		})
	}

	// Create run
	runID := generateRunID()
	run := domain.WorkflowRun{
		ID:         runID,
		WorkflowID: wf.ID,
		Input:      fmt.Sprintf("%v", stepInputs),
		Status:     domain.RunStatusRunning,
		StartedAt:  time.Now(),
	}

	var runSteps []domain.WorkflowRunStep
	structuredSteps := make([]domain.StructuredStepResult, 0, len(resolved))
	for _, rs := range resolved {
		if rs.entryID == "" {
			continue
		}
		stepID := int64(0)
		if rs.step.ID != "" {
			fmt.Sscanf(rs.step.ID, "%d", &stepID)
		}
		runStep := domain.WorkflowRunStep{
			ID:        generateRunStepID(),
			RunID:     runID,
			StepID:    stepID,
			EntryID:   rs.entryID,
			Status:    domain.RunStatusPending,
			StartedAt: time.Now(),
		}
		runSteps = append(runSteps, runStep)

		// Initialize structured step result as pending
		structuredSteps = append(structuredSteps, domain.StructuredStepResult{
			StepIndex: rs.step.OrderIndex,
			Status:    domain.RunStatusPending,
		})
	}

	if err := s.workflowRunStore.CreateRun(ctx, run, runSteps); err != nil {
		return nil, fmt.Errorf("create run: %w", err)
	}

	// Execute steps sequentially
	var previousOutput string
	var finalOutputParts []string
	anyFailed := false

	for i, rs := range resolved {
		if rs.entryID == "" {
			continue
		}

		if i >= len(structuredSteps) {
			continue
		}
		currentStep := &structuredSteps[i]

		// Update step to running
		if err := s.workflowRunStore.UpdateStepStatus(ctx, runSteps[i].ID, domain.RunStatusRunning, ""); err != nil {
			s.workflowRunStore.UpdateRunStatus(ctx, runID, domain.RunStatusFailed, err.Error())
			currentStep.Status = domain.RunStatusFailed
			currentStep.Error = err.Error()
			anyFailed = true
			break
		}

		// Resolve variables
		truncatedPrev := previousOutput
		if len(truncatedPrev) > maxPreviousOutputLen {
			truncatedPrev = truncatedPrev[:maxPreviousOutputLen]
		}
		currentFinal := strings.Join(finalOutputParts, "\n")

		providedVars := map[string]string{
			"input":           "",
			"previous_output": truncatedPrev,
			"final_output":    currentFinal,
		}

		_, _ = vars.Resolve(rs.body, providedVars, nil)

		// Get step response from input map
		response, hasInput := stepInputs[rs.step.OrderIndex]
		if !hasInput || response == "" {
			// Step has no input — mark as failed
			failErr := fmt.Errorf("step %q requires input but none provided", rs.step.Title)
			s.workflowRunStore.UpdateStepStatus(ctx, runSteps[i].ID, domain.RunStatusFailed, failErr.Error())
			s.workflowRunStore.UpdateRunStatus(ctx, runID, domain.RunStatusFailed, failErr.Error())
			currentStep.Status = domain.RunStatusFailed
			currentStep.Error = failErr.Error()
			anyFailed = true
			break
		}

		// Update step with response
		if err := s.workflowRunStore.UpdateStepStatus(ctx, runSteps[i].ID, domain.RunStatusCompleted, response); err != nil {
			s.workflowRunStore.UpdateRunStatus(ctx, runID, domain.RunStatusFailed, err.Error())
			currentStep.Status = domain.RunStatusFailed
			currentStep.Error = err.Error()
			anyFailed = true
			break
		}

		currentStep.Status = domain.RunStatusCompleted
		currentStep.Output = response
		previousOutput = response
		finalOutputParts = append(finalOutputParts, response)
	}

	// Finalize run status
	finalStatus := domain.RunStatusCompleted
	now := time.Now()
	if anyFailed {
		finalStatus = domain.RunStatusFailed
	}
	finalOutput := strings.Join(finalOutputParts, "\n")
	if err := s.workflowRunStore.UpdateRunStatus(ctx, runID, finalStatus, finalOutput); err != nil {
		return nil, fmt.Errorf("update run status: %w", err)
	}

	result := &domain.StructuredRunResult{
		RunID:        runID,
		WorkflowID:   wf.ID,
		WorkflowSlug: wf.Slug,
		Status:       finalStatus,
		Steps:        structuredSteps,
		StartedAt:    run.StartedAt,
		FinishedAt:   &now,
	}
	return result, nil
}

func generateRunID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "run-" + hex.EncodeToString(b)
}

func generateRunStepID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "rst-" + hex.EncodeToString(b)
}

// ListAllRuns delegates to the workflow run store for paginated run listing with progress.
func (s *WorkflowRunService) ListAllRuns(ctx context.Context, workflowID *string, limit, offset int) ([]domain.WorkflowRun, []db.RunProgress, error) {
	return s.workflowRunStore.ListAllRuns(ctx, workflowID, limit, offset)
}

// GetRun delegates to the workflow run store for fetching a single run with its steps.
func (s *WorkflowRunService) GetRun(ctx context.Context, runID string) (domain.WorkflowRun, []domain.WorkflowRunStep, error) {
	return s.workflowRunStore.GetRun(ctx, runID)
}
