package line

import "time"

// Phase is the factory state-machine step.
// Review success parks at wait_ci; CI failure parks at repair.
type Phase string

const (
	PhasePlan   Phase = "plan"
	PhaseBuild  Phase = "build"
	PhaseReview Phase = "review"
	PhaseWaitCI Phase = "wait_ci"
	PhaseRepair Phase = "repair"
	PhaseReady  Phase = "ready"
)

// Status is the job parking state. needs_human parks until `line continue`
// or the local UI. ntfy is optional.
type Status string

const (
	StatusRunning      Status = "running"
	StatusNeedsHuman   Status = "needs_human"
	StatusBlocked      Status = "blocked"
	StatusFailed       Status = "failed"
	StatusAwaitingNext Status = "awaiting_next"
	StatusReady        Status = "ready"
)

type Job struct {
	ID          string
	IssueURL    string
	RepoPath    string
	Phase       Phase
	Status      Status
	Question    string
	Summary     string
	HumanAnswer string
	HeadSHA     string
	PullRequest string
	RepairCount int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Event struct {
	ID        string
	JobID     string
	Phase     Phase
	Kind      string
	Detail    string
	CreatedAt time.Time
}
