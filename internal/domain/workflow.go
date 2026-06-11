package domain

// WorkflowRole defines the role of a step in a workflow.
type WorkflowRole string

const (
	WorkflowRoleSystem    WorkflowRole = "system"
	WorkflowRoleUser      WorkflowRole = "user"
	WorkflowRoleAssistant WorkflowRole = "assistant"
)

// WorkflowStep is a single step in a workflow.
type WorkflowStep struct {
	ID       int
	EntryID  string
	StepNum  int
	Role     WorkflowRole
	Content  string
	Label    string
}

// RenderedStep is a workflow step with variables resolved.
type RenderedStep struct {
	Role        WorkflowRole
	Content     string
	Label       string
	MissingVars []string
}

// RenderedWorkflow is the result of running a workflow.
type RenderedWorkflow struct {
	EntryID string
	Steps   []RenderedStep
}
