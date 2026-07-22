package agenttelemetry

import (
	"encoding/json"
	"time"
)

// Event is the canonical JSON-L telemetry event.
type Event struct {
	EventID         string          `json:"event_id"`
	EventType       string          `json:"event_type"`       // run.started, tool.called, ...
	Timestamp       time.Time       `json:"timestamp"`
	RunID           string          `json:"run_id"`
	AgentID         string          `json:"agent_id"`
	AgentVersion    string          `json:"agent_version"`
	Source          string          `json:"source"`           // plugin|wrapper|daemon
	CorrelationID   *string         `json:"correlation_id,omitempty"`
	StepID          *string         `json:"step_id,omitempty"`
	RedactionPolicy string          `json:"redaction_policy"` // hash-args|none|scanned-warning
	ConfidenceLevel string          `json:"confidence_level"` // measured|estimated|heuristic
	Payload         json.RawMessage `json:"payload"`
}

// EventEnvelope is the metadata wrapper without payload.
type EventEnvelope struct {
	EventID         string    `json:"event_id"`
	EventType       string    `json:"event_type"`
	Timestamp       time.Time `json:"timestamp"`
	RunID           string    `json:"run_id"`
	AgentID         string    `json:"agent_id"`
	AgentVersion    string    `json:"agent_version"`
	Source          string    `json:"source"`
	CorrelationID   *string   `json:"correlation_id,omitempty"`
	StepID          *string   `json:"step_id,omitempty"`
	RedactionPolicy string    `json:"redaction_policy"`
	ConfidenceLevel string    `json:"confidence_level"`
}

// RunOpts configures a new agent run.
type RunOpts struct {
	AgentID      string `json:"agent_id"`
	AgentVersion string `json:"agent_version"`
	Workspace    string `json:"workspace"`
	RepoURL      string `json:"repo_url,omitempty"`
	Branch       string `json:"branch,omitempty"`
	CommitSHA    string `json:"commit_sha,omitempty"`
}

// AgentRun represents a single agent invocation lifecycle.
type AgentRun struct {
	ID           string     `json:"id"`
	AgentID      string     `json:"agent_id"`
	AgentVersion string     `json:"agent_version"`
	RepoURL      string     `json:"repo_url,omitempty"`
	Branch       string     `json:"branch,omitempty"`
	CommitSHA    string     `json:"commit_sha,omitempty"`
	Workspace    string     `json:"workspace"`
	StartedAt    time.Time  `json:"started_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	Status       string     `json:"status"` // running|completed|failed
	TotalTokens  int64      `json:"total_tokens"`
	TotalCostUSD float64    `json:"total_cost_usd"`
	ErrorType    string     `json:"error_type,omitempty"`
	ErrorMessage string     `json:"error_message,omitempty"`
}

// ToolCallRecord captures a single tool invocation.
type ToolCallRecord struct {
	ID          string     `json:"id"`
	RunID       string     `json:"run_id"`
	StepID      string     `json:"step_id,omitempty"`
	ToolName    string     `json:"tool_name"`
	ArgsHash    string     `json:"args_hash"`
	CallIndex   int        `json:"call_index"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	DurationMs  int64      `json:"duration_ms"`
	Success     bool       `json:"success"`
	ErrorType   string     `json:"error_type,omitempty"`
}

// TokenUsage records token counts and cost for a model interaction.
type TokenUsage struct {
	ID               string   `json:"id"`
	RunID            string   `json:"run_id"`
	StepID           string   `json:"step_id,omitempty"`
	Model            string   `json:"model"`
	InputTokens      int64    `json:"input_tokens"`
	OutputTokens     int64    `json:"output_tokens"`
	TotalTokens      int64    `json:"total_tokens"`
	CostUSD          float64  `json:"cost_usd"`
	EstimationMethod string   `json:"estimation_method"` // api-response|char-div-4|manual
	EfficiencyRatio  *float64 `json:"efficiency_ratio,omitempty"`
}

// DaemonStatus provides health and operational metrics for telemetryd.
type DaemonStatus struct {
	UptimeSeconds     int64    `json:"uptime_seconds"`
	EventsIngested    int64    `json:"events_ingested"`
	DBSizeBytes       int64    `json:"db_size_bytes"`
	SaltFingerprint   string   `json:"salt_fingerprint"`
	RedactionPatterns []string `json:"redaction_patterns"`
	PromptStorage     bool     `json:"prompt_storage"`
}
