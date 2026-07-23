package agenttelemetry

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestValidatorRequiredFields(t *testing.T) {
	now := time.Now().UTC()
	base := Event{
		EventID: "evt-001", EventType: "run.started",
		Timestamp: now, RunID: "run-001", AgentID: "opencode", AgentVersion: "0.1.0",
		Source: "plugin", RedactionPolicy: "hash-args", ConfidenceLevel: "measured",
		Payload: json.RawMessage(`{"workspace":"/tmp"}`),
	}

	tests := []struct {
		name    string
		mutate  func(e *Event)
		wantErr bool
		contains string
	}{
		{"valid event", func(e *Event) {}, false, ""},
		{"missing event_id", func(e *Event) { e.EventID = "" }, true, "event_id"},
		{"missing event_type", func(e *Event) { e.EventType = "" }, true, "event_type"},
		{"missing run_id", func(e *Event) { e.RunID = "" }, true, "run_id"},
		{"missing agent_id", func(e *Event) { e.AgentID = "" }, true, "agent_id"},
		{"missing source", func(e *Event) { e.Source = "" }, true, "source"},
		{"missing redaction_policy", func(e *Event) { e.RedactionPolicy = "" }, true, "redaction_policy"},
		{"missing confidence_level", func(e *Event) { e.ConfidenceLevel = "" }, true, "confidence_level"},
		{"zero timestamp", func(e *Event) { e.Timestamp = time.Time{} }, true, "timestamp"},
		{"nil payload", func(e *Event) { e.Payload = nil }, true, "payload"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := base
			tt.mutate(&e)
			err := ValidateEvent(e)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateEvent() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.contains != "" && !strings.Contains(err.Error(), tt.contains) {
				t.Errorf("error %q should contain %q", err.Error(), tt.contains)
			}
		})
	}
}

func TestValidatorEnumValidation(t *testing.T) {
	now := time.Now().UTC()
	base := Event{
		EventID: "evt-001", EventType: "run.started",
		Timestamp: now, RunID: "run-001", AgentID: "opencode", AgentVersion: "0.1.0",
		Source: "plugin", RedactionPolicy: "hash-args", ConfidenceLevel: "measured",
		Payload: json.RawMessage(`{}`),
	}

	tests := []struct {
		name    string
		mutate  func(e *Event)
		wantErr bool
		contains string
	}{
		{"valid source plugin", func(e *Event) { e.Source = "plugin" }, false, ""},
		{"valid source wrapper", func(e *Event) { e.Source = "wrapper" }, false, ""},
		{"valid source daemon", func(e *Event) { e.Source = "daemon" }, false, ""},
		{"invalid source", func(e *Event) { e.Source = "invalid" }, true, "source"},
		{"valid redaction hash-args", func(e *Event) { e.RedactionPolicy = "hash-args" }, false, ""},
		{"valid redaction none", func(e *Event) { e.RedactionPolicy = "none" }, false, ""},
		{"valid redaction scanned-warning", func(e *Event) { e.RedactionPolicy = "scanned-warning" }, false, ""},
		{"invalid redaction", func(e *Event) { e.RedactionPolicy = "bad" }, true, "redaction_policy"},
		{"valid confidence measured", func(e *Event) { e.ConfidenceLevel = "measured" }, false, ""},
		{"valid confidence estimated", func(e *Event) { e.ConfidenceLevel = "estimated" }, false, ""},
		{"valid confidence heuristic", func(e *Event) { e.ConfidenceLevel = "heuristic" }, false, ""},
		{"invalid confidence", func(e *Event) { e.ConfidenceLevel = "guess" }, true, "confidence_level"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := base
			tt.mutate(&e)
			err := ValidateEvent(e)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateEvent() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.contains != "" && !strings.Contains(err.Error(), tt.contains) {
				t.Errorf("error %q should contain %q", err.Error(), tt.contains)
			}
		})
	}
}

func TestValidatorEventType(t *testing.T) {
	now := time.Now().UTC()
	base := Event{
		EventID: "evt-001",
		Timestamp: now, RunID: "run-001", AgentID: "opencode", AgentVersion: "0.1.0",
		Source: "plugin", RedactionPolicy: "hash-args", ConfidenceLevel: "measured",
		Payload: json.RawMessage(`{}`),
	}

	validTypes := []string{
		"run.started", "run.completed", "run.failed",
		"prompt.submitted", "response.received", "model.usage",
		"step.started", "step.completed",
		"tool.called", "tool.completed",
		"command.started", "command.completed",
		"file.created", "file.modified", "file.deleted",
		"test.started", "test.completed",
		"approval.recorded",
		"loop.detected", "policy.violation",
	}

	for _, et := range validTypes {
		t.Run("valid-"+et, func(t *testing.T) {
			e := base
			e.EventType = et
			err := ValidateEvent(e)
			if err != nil {
				t.Errorf("ValidateEvent(%q) unexpected error: %v", et, err)
			}
		})
	}

	invalidTypes := []string{"", "unknown.event", "run.Unknown", "tool_called", "run.started.extra"}
	for _, et := range invalidTypes {
		t.Run("invalid-"+strings.ReplaceAll(et, ".", "_"), func(t *testing.T) {
			e := base
			e.EventType = et
			err := ValidateEvent(e)
			if err == nil {
				t.Errorf("ValidateEvent(%q) expected error, got nil", et)
			}
			if !strings.Contains(err.Error(), "event_type") {
				t.Errorf("error %q should mention event_type", err.Error())
			}
		})
	}
}

func TestValidatorMalformedJSON(t *testing.T) {
	now := time.Now().UTC()

	e := Event{
		EventID: "evt-001", EventType: "run.started",
		Timestamp: now, RunID: "run-001", AgentID: "opencode", AgentVersion: "0.1.0",
		Source: "plugin", RedactionPolicy: "hash-args", ConfidenceLevel: "measured",
		Payload: json.RawMessage(`{bad`),
	}

	err := ValidateEvent(e)
	if err == nil {
		t.Fatal("expected error for malformed JSON payload")
	}
	if !strings.Contains(err.Error(), "payload") {
		t.Errorf("error %q should mention payload", err.Error())
	}
}

func TestValidateRaw(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr bool
		contains string
	}{
		{"valid JSON", `{"event_id":"evt-01","event_type":"run.started","timestamp":"2026-07-22T10:00:00Z","run_id":"run-01","agent_id":"opencode","agent_version":"0.1.0","source":"plugin","redaction_policy":"hash-args","confidence_level":"measured","payload":{}}`, false, ""},
		{"garbage bytes", "not-json", true, "json"},
		{"empty string", "", true, "empty"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRaw([]byte(tt.raw))
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRaw() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.contains != "" && !strings.Contains(err.Error(), tt.contains) {
				t.Errorf("error %q should contain %q", err.Error(), tt.contains)
			}
		})
	}
}
