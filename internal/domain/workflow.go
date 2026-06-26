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
