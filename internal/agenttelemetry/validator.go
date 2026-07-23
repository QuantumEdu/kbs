package agenttelemetry

import (
	"encoding/json"
	"fmt"
)

// validSources defines allowed values for Event.Source.
var validSources = map[string]bool{
	"plugin":  true,
	"wrapper": true,
	"daemon":  true,
}

// validRedactionPolicies defines allowed values for Event.RedactionPolicy.
var validRedactionPolicies = map[string]bool{
	"hash-args":       true,
	"none":            true,
	"scanned-warning": true,
}

// validConfidenceLevels defines allowed values for Event.ConfidenceLevel.
var validConfidenceLevels = map[string]bool{
	"measured":  true,
	"estimated": true,
	"heuristic": true,
}

// validEventTypes defines the canonical set of event types.
var validEventTypes = map[string]bool{
	"run.started":       true,
	"run.completed":     true,
	"run.failed":        true,
	"prompt.submitted":  true,
	"response.received": true,
	"model.usage":       true,
	"step.started":      true,
	"step.completed":    true,
	"tool.called":       true,
	"tool.completed":    true,
	"command.started":   true,
	"command.completed": true,
	"file.created":      true,
	"file.modified":     true,
	"file.deleted":      true,
	"test.started":      true,
	"test.completed":    true,
	"approval.recorded": true,
	"loop.detected":     true,
	"policy.violation":  true,
}

// ValidateEvent checks that an Event has all required fields and valid enumerations.
func ValidateEvent(e Event) error {
	if e.EventID == "" {
		return fmt.Errorf("required field event_id is empty")
	}
	if e.EventType == "" {
		return fmt.Errorf("required field event_type is empty")
	}
	if !validEventTypes[e.EventType] {
		return fmt.Errorf("invalid event_type %q", e.EventType)
	}
	if e.Timestamp.IsZero() {
		return fmt.Errorf("required field timestamp is zero")
	}
	if e.RunID == "" {
		return fmt.Errorf("required field run_id is empty")
	}
	if e.AgentID == "" {
		return fmt.Errorf("required field agent_id is empty")
	}
	if !validSources[e.Source] {
		return fmt.Errorf("invalid source %q", e.Source)
	}
	if !validRedactionPolicies[e.RedactionPolicy] {
		return fmt.Errorf("invalid redaction_policy %q", e.RedactionPolicy)
	}
	if !validConfidenceLevels[e.ConfidenceLevel] {
		return fmt.Errorf("invalid confidence_level %q", e.ConfidenceLevel)
	}
	if e.Payload == nil {
		return fmt.Errorf("required field payload is nil")
	}
	if !json.Valid(e.Payload) {
		return fmt.Errorf("payload is not valid JSON")
	}
	return nil
}

// ValidateRaw parses raw JSON bytes into an Event and validates it.
func ValidateRaw(raw []byte) error {
	if len(raw) == 0 {
		return fmt.Errorf("empty input")
	}
	var e Event
	if err := json.Unmarshal(raw, &e); err != nil {
		return fmt.Errorf("invalid json: %w", err)
	}
	return ValidateEvent(e)
}

