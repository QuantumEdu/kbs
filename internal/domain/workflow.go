package domain

import "time"

type RunStatus string

const (
	RunStatusPending   RunStatus = "pending"
	RunStatusRunning   RunStatus = "running"
	RunStatusCompleted RunStatus = "completed"
	RunStatusFailed    RunStatus = "failed"
)

type Workflow struct {
	ID          string
	Name        string
	Slug        string
	Description string
	Status      Status
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type WorkflowStep struct {
	ID             string
	WorkflowID     string
	OrderIndex     int
	Title          string
	Instruction    string
	Required       bool
	ExpectedOutput string
	EntrySlug      string // slug of the entry to execute; empty = renderable-only step
}

type WorkflowRun struct {
	ID         string
	WorkflowID string
	Input      string
	Output     string
	Status     RunStatus
	StartedAt  time.Time
	FinishedAt *time.Time
}

type WorkflowRunStep struct {
	ID         string
	RunID      string
	StepID     int64   // FK to workflow_steps.id (INTEGER)
	EntryID    string  // FK to entries.id, resolved at pre-flight
	Input      string
	Output     string
	Status     RunStatus
	StartedAt  time.Time
	FinishedAt *time.Time
}

// StructuredRunResult holds the result of a structured pipeline run.
type StructuredRunResult struct {
	RunID        string                 `json:"run_id"`
	WorkflowID   string                 `json:"workflow_id"`
	WorkflowSlug string                 `json:"workflow_slug"`
	Status       RunStatus              `json:"status"`
	Steps        []StructuredStepResult `json:"steps"`
	StartedAt    time.Time              `json:"started_at"`
	FinishedAt   *time.Time             `json:"finished_at"`
}

// StructuredStepResult holds the result of a single step in a structured run.
type StructuredStepResult struct {
	StepIndex int       `json:"step_index"`
	Status    RunStatus `json:"status"`
	Output    string    `json:"output,omitempty"`
	Error     string    `json:"error,omitempty"`
}
