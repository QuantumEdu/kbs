package agenttelemetry

import (
	"encoding/json"
	"time"
)

// Event is the canonical JSON-L telemetry event.
type Event struct {
	EventID         string          `json:"event_id"`
	EventType       string          `json:"event_type"` // run.started, tool.called, ...
	Timestamp       time.Time       `json:"timestamp"`
	RunID           string          `json:"run_id"`
	AgentID         string          `json:"agent_id"`
	AgentVersion    string          `json:"agent_version"`
	Source          string          `json:"source"` // plugin|wrapper|daemon
	CorrelationID   *string         `json:"correlation_id,omitempty"`
	StepID          *string         `json:"step_id,omitempty"`
	RedactionPolicy string          `json:"redaction_policy"` // hash-args|none|scanned-warning
	ConfidenceLevel string          `json:"confidence_level"` // measured|estimated|heuristic
	ProjectID       string          `json:"project_id,omitempty"`
	ChangeID        string          `json:"change_id,omitempty"`
	SessionID       string          `json:"session_id,omitempty"`
	InteractionID   string          `json:"interaction_id,omitempty"`
	Provider        string          `json:"provider,omitempty"`
	Model           string          `json:"model,omitempty"`
	Effort          string          `json:"effort,omitempty"`
	Coverage        string          `json:"coverage,omitempty"`
	Payload         json.RawMessage `json:"payload"`
}

const CoverageUnknown = "unknown"

// EvidenceMeta carries explicit identity and provenance for an event. Missing
// observations are represented as "unknown" so consumers never mistake them
// for measured zero values.
type EvidenceMeta struct {
	ProjectID     string
	ChangeID      string
	SessionID     string
	RunID         string
	InteractionID string
	AgentID       string
	Provider      string
	Model         string
	Effort        string
	Source        string
	Confidence    string
	Coverage      string
}

// EvidenceMetadata normalizes optional event identity without inventing facts.
func (e Event) EvidenceMetadata() EvidenceMeta {
	return EvidenceMeta{
		ProjectID: unknown(e.ProjectID), ChangeID: unknown(e.ChangeID),
		SessionID: unknown(e.SessionID), RunID: unknown(e.RunID),
		InteractionID: unknown(e.InteractionID), AgentID: unknown(e.AgentID),
		Provider: unknown(e.Provider), Model: unknown(e.Model), Effort: unknown(e.Effort),
		Source: unknown(e.Source), Confidence: unknown(e.ConfidenceLevel), Coverage: unknown(e.Coverage),
	}
}

func unknown(value string) string {
	if value == "" {
		return CoverageUnknown
	}
	return value
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

// RunFilter provides query criteria for listing runs.
type RunFilter struct {
	Limit   int
	AgentID string
	Since   *time.Time
}

// AgentStep captures a step boundary within a run.
type AgentStep struct {
	ID          string     `json:"id"`
	RunID       string     `json:"run_id"`
	StepName    string     `json:"step_name"`
	StepIndex   int        `json:"step_index"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	DurationMs  int64      `json:"duration_ms"`
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
