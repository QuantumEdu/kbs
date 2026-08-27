package agenttelemetry

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
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

// ProjectionPayload is a typed, bounded view of an accepted event payload.
// Unsupported payloads stay ledger-compatible and are explicitly unknown.
type ProjectionPayload struct {
	Coverage string
	Usage    *UsageSample
	Activity *ActivityProjection
	Git      *GitSnapshot
}

type ActivityProjection struct {
	SampleID, ClockID string
	Interval          ActivityInterval
	Heartbeat         time.Time
}

// DecodeProjectionPayload accepts only the v1 shapes used by typed projectors.
func DecodeProjectionPayload(e Event) ProjectionPayload {
	if len(e.Payload) == 0 || len(e.Payload) > 16<<10 || jsonDepth(e.Payload) > 8 {
		return ProjectionPayload{Coverage: CoverageUnknown}
	}
	switch e.EventType {
	case "model.usage":
		return decodeUsagePayload(e)
	case "activity.sample":
		return decodeActivityPayload(e)
	case "run.started", "run.completed", "run.failed":
		return decodeGitPayload(e)
	default:
		return ProjectionPayload{Coverage: CoverageUnknown}
	}
}

func decodeUsagePayload(e Event) ProjectionPayload {
	var p struct {
		Version         int     `json:"schema_version"`
		SampleID        string  `json:"sample_id"`
		InteractionID   string  `json:"interaction_id"`
		Mode            string  `json:"mode"`
		SegmentID       string  `json:"segment_id"`
		Reset           bool    `json:"reset"`
		Method          string  `json:"method"`
		EstimatedMethod *string `json:"estimated_method"`
		Tokens          struct {
			Input      *int64 `json:"input"`
			Output     *int64 `json:"output"`
			CacheRead  *int64 `json:"cache_read"`
			CacheWrite *int64 `json:"cache_write"`
			Reasoning  *int64 `json:"reasoning"`
		} `json:"tokens"`
	}
	if strictJSON(e.Payload, &p) != nil || p.Version != 1 || p.SampleID == "" || p.InteractionID == "" || p.InteractionID != e.InteractionID || (p.Mode != "delta" && p.Mode != "cumulative") || (p.Method != "measured" && p.Method != "estimated") || (p.Method == "estimated" && (p.EstimatedMethod == nil || *p.EstimatedMethod == "")) || (p.Mode == "cumulative" && p.SegmentID == "") {
		return ProjectionPayload{Coverage: CoverageUnknown}
	}
	counts := []*int64{p.Tokens.Input, p.Tokens.Output, p.Tokens.CacheRead, p.Tokens.CacheWrite, p.Tokens.Reasoning}
	var total int64
	known := false
	for i, value := range counts {
		if value == nil {
			continue
		}
		if *value < 0 {
			return ProjectionPayload{Coverage: CoverageUnknown}
		}
		known = true
		if i < 2 { // cache and reasoning are dimensions, not additional total tokens.
			total += *value
		}
	}
	if !known {
		return ProjectionPayload{Coverage: CoverageUnknown}
	}
	return ProjectionPayload{Coverage: p.Method, Usage: &UsageSample{Provider: e.Provider, ID: p.SampleID, Total: total, Cumulative: p.Mode == "cumulative", Measured: p.Method == "measured", Segment: p.SegmentID, Reset: p.Reset, Input: p.Tokens.Input, Output: p.Tokens.Output, CacheRead: p.Tokens.CacheRead, CacheWrite: p.Tokens.CacheWrite, Reasoning: p.Tokens.Reasoning}}
}

func decodeActivityPayload(e Event) ProjectionPayload {
	var p struct {
		Version  int        `json:"schema_version"`
		SampleID string     `json:"sample_id"`
		Kind     string     `json:"kind"`
		Method   string     `json:"method"`
		Start    *time.Time `json:"start"`
		End      *time.Time `json:"end"`
		At       *time.Time `json:"at"`
		Clock    struct {
			Source        string `json:"source"`
			ClockID       string `json:"clock_id"`
			UncertaintyMS int    `json:"uncertainty_ms"`
		} `json:"clock"`
	}
	if strictJSON(e.Payload, &p) != nil || p.Version != 1 || p.SampleID == "" || p.Clock.ClockID == "" || (p.Clock.Source != "provider_wall" && p.Clock.Source != "collector_wall") || p.Clock.UncertaintyMS < 0 {
		return ProjectionPayload{Coverage: CoverageUnknown}
	}
	activity := &ActivityProjection{SampleID: p.SampleID, ClockID: p.Clock.ClockID}
	switch p.Kind {
	case "span":
		if p.Method != "measured" || p.Start == nil || p.End == nil || !p.End.After(*p.Start) || p.At != nil {
			return ProjectionPayload{Coverage: CoverageUnknown}
		}
		activity.Interval = ActivityInterval{Start: p.Start.UTC(), End: p.End.UTC(), Measured: true}
	case "heartbeat":
		if p.Method != "inferred" || p.At == nil || p.Start != nil || p.End != nil {
			return ProjectionPayload{Coverage: CoverageUnknown}
		}
		activity.Heartbeat = p.At.UTC()
	default:
		return ProjectionPayload{Coverage: CoverageUnknown}
	}
	return ProjectionPayload{Coverage: p.Method, Activity: activity}
}

func decodeGitPayload(e Event) ProjectionPayload {
	var p struct {
		Version int `json:"schema_version"`
		Git     struct {
			Phase      string     `json:"phase"`
			Capture    string     `json:"capture"`
			RootID     string     `json:"root_id"`
			Head       string     `json:"head"`
			Branch     *string    `json:"branch"`
			Detached   bool       `json:"detached"`
			Dirty      bool       `json:"dirty"`
			Staged     int        `json:"staged"`
			Unstaged   int        `json:"unstaged"`
			Untracked  int        `json:"untracked"`
			CapturedAt *time.Time `json:"captured_at"`
			ErrorCode  *string    `json:"error_code"`
		} `json:"git_snapshot"`
	}
	if strictJSON(e.Payload, &p) != nil || p.Version != 1 || p.Git.Phase == "" || p.Git.Phase != lifecyclePhase(e.EventType) || p.Git.RootID == "" || p.Git.CapturedAt == nil || (p.Git.Capture != "ok" && p.Git.Capture != "unavailable") {
		return ProjectionPayload{Coverage: CoverageUnknown}
	}
	if p.Git.Capture == "unavailable" || !validGitSnapshot(p.Git.Head, p.Git.Branch, p.Git.Detached, p.Git.Dirty, p.Git.Staged, p.Git.Unstaged, p.Git.Untracked, p.Git.ErrorCode) {
		return ProjectionPayload{Coverage: CoverageUnknown}
	}
	return ProjectionPayload{Coverage: "measured", Git: &GitSnapshot{Root: p.Git.RootID, Head: p.Git.Head, Branch: deref(p.Git.Branch), Detached: p.Git.Detached, Staged: p.Git.Staged, Unstaged: p.Git.Unstaged, Untracked: p.Git.Untracked, CapturedAt: p.Git.CapturedAt.UTC()}}
}

func lifecyclePhase(eventType string) string {
	if eventType == "run.started" {
		return "start"
	}
	return "end"
}
func deref(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
func validGitSnapshot(head string, branch *string, detached, dirty bool, staged, unstaged, untracked int, code *string) bool {
	if len(head) != 40 && len(head) != 64 || strings.Trim(head, "0123456789abcdef") != "" || staged < 0 || unstaged < 0 || untracked < 0 || dirty != (staged+unstaged+untracked > 0) || (detached && branch != nil) || (!detached && (branch == nil || *branch == "")) || code != nil {
		return false
	}
	return true
}
func strictJSON(data []byte, dst any) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(dst); err != nil {
		return err
	}
	if d.More() {
		return fmt.Errorf("trailing payload")
	}
	return nil
}
func jsonDepth(data []byte) int {
	var v any
	if json.Unmarshal(data, &v) != nil {
		return 9
	}
	var depth func(any, int) int
	depth = func(value any, n int) int {
		max := n
		switch x := value.(type) {
		case map[string]any:
			for _, child := range x {
				if d := depth(child, n+1); d > max {
					max = d
				}
			}
		case []any:
			for _, child := range x {
				if d := depth(child, n+1); d > max {
					max = d
				}
			}
		}
		return max
	}
	return depth(v, 0)
}

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
