package agenttelemetry

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"
)

var update = flag.Bool("update", false, "update golden files")

func TestEventJSONRoundTrip(t *testing.T) {
	corrID := "evt-parent-001"
	stepID := "step-001"
	payload := json.RawMessage(`{"tool_name":"bash","args_hash":"abc123"}`)

	original := Event{
		EventID: "evt-001", EventType: "tool.called",
		Timestamp: time.Date(2026, 7, 22, 10, 15, 0, 0, time.UTC),
		RunID: "run-001", AgentID: "opencode", AgentVersion: "0.1.0",
		Source: "plugin", CorrelationID: &corrID, StepID: &stepID,
		RedactionPolicy: "hash-args", ConfidenceLevel: "measured",
		Payload: payload,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var rt Event
	if err := json.Unmarshal(data, &rt); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if rt.EventID != "evt-001" || rt.EventType != "tool.called" ||
		rt.RunID != "run-001" || rt.AgentID != "opencode" ||
		rt.Source != "plugin" || rt.RedactionPolicy != "hash-args" ||
		rt.ConfidenceLevel != "measured" {
		t.Errorf("header fields mismatch: %+v", rt)
	}
	if rt.CorrelationID == nil || *rt.CorrelationID != "evt-parent-001" {
		t.Errorf("CorrelationID: %v", rt.CorrelationID)
	}
	if rt.StepID == nil || *rt.StepID != "step-001" {
		t.Errorf("StepID: %v", rt.StepID)
	}
	if string(rt.Payload) != string(payload) {
		t.Errorf("Payload: %s", rt.Payload)
	}
}

func TestEventJSONOmitEmpty(t *testing.T) {
	e := Event{
		EventID: "evt-002", EventType: "run.started",
		Timestamp: time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC),
		RunID: "run-002", AgentID: "opencode", AgentVersion: "0.1.0",
		Source: "plugin", RedactionPolicy: "hash-args", ConfidenceLevel: "measured",
		Payload: json.RawMessage(`{"workspace":"/tmp"}`),
	}
	data, _ := json.Marshal(e)
	var raw map[string]interface{}
	json.Unmarshal(data, &raw)
	if _, ok := raw["correlation_id"]; ok {
		t.Error("correlation_id should be omitted when nil")
	}
	if _, ok := raw["step_id"]; ok {
		t.Error("step_id should be omitted when nil")
	}
}

func TestEventGoldenFile(t *testing.T) {
	corrID := "evt-01"
	e := Event{
		EventID: "evt-01", EventType: "prompt.submitted",
		Timestamp: time.Date(2026, 7, 22, 10, 15, 0, 0, time.UTC),
		RunID: "run-01", AgentID: "opencode", AgentVersion: "0.1.0",
		Source: "plugin", CorrelationID: &corrID,
		RedactionPolicy: "hash-args", ConfidenceLevel: "measured",
		Payload: json.RawMessage(`{"prompt_hash":"sha256:abc123","prompt_char_count":2450,"model":"claude-sonnet-4-5"}`),
	}
	got, _ := json.MarshalIndent(e, "", "  ")
	got = append(got, '\n')
	golden := filepath.Join("testdata", "event_golden.json")
	if *update {
		os.MkdirAll("testdata", 0o755)
		os.WriteFile(golden, got, 0o644)
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden: %v (run with -update)", err)
	}
	if string(got) != string(want) {
		t.Errorf("golden mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestEventRequiredFields(t *testing.T) {
	e := Event{}
	for _, f := range []string{e.EventID, e.EventType, e.RunID, e.AgentID, e.Source, e.RedactionPolicy, e.ConfidenceLevel} {
		if f != "" {
			t.Errorf("required field zero value should be empty, got %q", f)
		}
	}
	if e.Payload != nil || !e.Timestamp.IsZero() {
		t.Error("Payload/Timestamp zero value")
	}
}

func TestTypeRoundTrips(t *testing.T) {
	tests := []struct {
		name string
		v    interface{}
		ok   func(t *testing.T, data []byte)
	}{
		{"EventEnvelope", EventEnvelope{
			EventID: "evt-003", EventType: "run.failed",
			Timestamp: time.Date(2026, 7, 22, 11, 0, 0, 0, time.UTC),
			RunID: "run-003", AgentID: "claude-code", AgentVersion: "1.0.0",
			Source: "wrapper", RedactionPolicy: "none", ConfidenceLevel: "heuristic",
		}, func(t *testing.T, data []byte) {
			var v EventEnvelope
			json.Unmarshal(data, &v)
			if v.EventID != "evt-003" || v.Source != "wrapper" {
				t.Errorf("envelope: %q %q", v.EventID, v.Source)
			}
		}},
		{"AgentRun", AgentRun{
			ID: "run-001", AgentID: "opencode", AgentVersion: "0.1.0",
			Workspace: "/home/user/project", StartedAt: time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC),
			Status: "running",
		}, func(t *testing.T, data []byte) {
			var v AgentRun
			json.Unmarshal(data, &v)
			if v.ID != "run-001" || v.Status != "running" || v.CompletedAt != nil {
				t.Errorf("run: %q %q %v", v.ID, v.Status, v.CompletedAt)
			}
		}},
		{"TokenUsage", func() interface{} {
			r := 0.25
			return TokenUsage{ID: "tok-001", RunID: "run-001", Model: "claude-sonnet-4-5",
				InputTokens: 2450, OutputTokens: 820, TotalTokens: 3270,
				CostUSD: 0.049, EstimationMethod: "api-response", EfficiencyRatio: &r}
		}(), func(t *testing.T, data []byte) {
			var v TokenUsage
			json.Unmarshal(data, &v)
			if v.TotalTokens != 3270 || v.EstimationMethod != "api-response" {
				t.Errorf("usage: %d %s", v.TotalTokens, v.EstimationMethod)
			}
		}},
		{"DaemonStatus", DaemonStatus{
			UptimeSeconds: 3600, EventsIngested: 15000, DBSizeBytes: 1048576,
			SaltFingerprint: "a1b2c3d4", RedactionPatterns: []string{"sk-.*", "Bearer .*"},
			PromptStorage: false,
		}, func(t *testing.T, data []byte) {
			var v DaemonStatus
			json.Unmarshal(data, &v)
			if v.UptimeSeconds != 3600 || len(v.RedactionPatterns) != 2 {
				t.Errorf("status: %d %d", v.UptimeSeconds, len(v.RedactionPatterns))
			}
		}},
		{"ToolCallRecord", func() interface{} {
			s := time.Date(2026, 7, 22, 10, 16, 0, 0, time.UTC)
			c := time.Date(2026, 7, 22, 10, 16, 2, 0, time.UTC)
			return ToolCallRecord{ID: "call-001", RunID: "run-001", ToolName: "bash",
				ArgsHash: "sha256:def456", CallIndex: 1, StartedAt: s, CompletedAt: &c,
				DurationMs: 2000, Success: true}
		}(), func(t *testing.T, data []byte) {
			var v ToolCallRecord
			json.Unmarshal(data, &v)
			if v.ToolName != "bash" || !v.Success || v.CallIndex != 1 {
				t.Errorf("tool: %s %v %d", v.ToolName, v.Success, v.CallIndex)
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.v)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			tt.ok(t, data)
		})
	}
}
