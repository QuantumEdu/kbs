package agenttelemetry

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

func TestOpenStoreMigratesLegacyZeroTokensToUnknownCoverage(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "legacy.db")
	legacyStore, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	if _, err := legacyStore.db.Exec("DROP TABLE run_evidence; DROP TABLE evidence_metadata"); err != nil {
		t.Fatalf("remove new schema: %v", err)
	}

	if err := legacyStore.SaveRun(context.Background(), AgentRun{
		ID: "legacy-run", AgentID: "legacy", Workspace: "/tmp", StartedAt: time.Now(), Status: "running",
	}); err != nil {
		t.Fatalf("SaveRun: %v", err)
	}
	if err := legacyStore.Close(); err != nil {
		t.Fatalf("close legacy store: %v", err)
	}

	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("reopen legacy database: %v", err)
	}
	defer store.Close()

	var coverage string
	if err := store.db.QueryRow("SELECT token_coverage FROM run_evidence WHERE run_id = ?", "legacy-run").Scan(&coverage); err != nil {
		t.Fatalf("read coverage: %v", err)
	}
	if coverage != CoverageUnknown {
		t.Errorf("coverage = %q, want %q", coverage, CoverageUnknown)
	}
}

func TestMigrateEvidencePreservesNonzeroLegacyTokenMarker(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "legacy.db"))
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer store.Close()
	if err := store.SaveRun(context.Background(), AgentRun{
		ID: "measured-run", AgentID: "legacy", Workspace: "/tmp", StartedAt: time.Now(), Status: "running", TotalTokens: 12,
	}); err != nil {
		t.Fatalf("SaveRun: %v", err)
	}
	if err := store.MigrateEvidence(); err != nil {
		t.Fatalf("MigrateEvidence: %v", err)
	}
	var coverage string
	if err := store.db.QueryRow("SELECT token_coverage FROM run_evidence WHERE run_id = ?", "measured-run").Scan(&coverage); err != nil {
		t.Fatalf("read coverage: %v", err)
	}
	if coverage != "legacy" {
		t.Errorf("coverage = %q, want legacy", coverage)
	}
}

func TestMigrateEvidenceMarksLegacyScopedTokenDimensionsUnknown(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "legacy-scoped.db")
	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, column := range []string{"input_known", "output_known", "cache_read_known", "cache_write_known", "reasoning_known"} {
		if _, err := store.db.Exec("ALTER TABLE usage_scope_aggregates DROP COLUMN " + column); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.db.Exec(`INSERT INTO usage_scope_aggregates VALUES ('provider','v2','run','run','{}','measured','measured','complete',0,0,0,0,0)`); err != nil {
		t.Fatal(err)
	}
	store.Close()
	store, err = OpenStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	var inputKnown, outputKnown int
	if err := store.db.QueryRow(`SELECT input_known, output_known FROM usage_scope_aggregates`).Scan(&inputKnown, &outputKnown); err != nil {
		t.Fatal(err)
	}
	if inputKnown != 0 || outputKnown != 0 {
		t.Fatalf("legacy zero dimensions must remain unknown: %d/%d", inputKnown, outputKnown)
	}
}

func TestSecurityPipelineHashesCommandBeforePersistence(t *testing.T) {
	pipeline, err := NewSecurityPipeline(filepath.Join(t.TempDir(), "salt"), nil)
	if err != nil {
		t.Fatalf("NewSecurityPipeline: %v", err)
	}
	e := Event{RedactionPolicy: "hash-args", Payload: json.RawMessage(`{"command":"deploy --token secret","raw_line":"wrapper deploy --token secret"}`)}
	if err := pipeline.Process(&e); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if string(e.Payload) == `{"command":"deploy --token secret","raw_line":"wrapper deploy --token secret"}` {
		t.Fatal("payload was not transformed")
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(e.Payload, &payload); err != nil {
		t.Fatalf("unmarshal transformed payload: %v", err)
	}
	if _, ok := payload["command"]; ok {
		t.Error("raw command was retained")
	}
	if _, ok := payload["raw_line"]; ok {
		t.Error("raw wrapper line was retained")
	}
	if _, ok := payload["args_hash"]; !ok {
		t.Error("hashed command correlation was not retained")
	}
}

func TestStoreRejectsUntransformedProtectedPayload(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "telemetry.db"))
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer store.Close()
	err = store.SaveEvent(context.Background(), Event{
		EventID: "unsafe", EventType: "command.started", Timestamp: time.Now(), RunID: "run-1", AgentID: "agent-1",
		Source: "wrapper", RedactionPolicy: "hash-args", ConfidenceLevel: "measured", Payload: json.RawMessage(`{"command":"secret command"}`),
	})
	if err == nil {
		t.Fatal("SaveEvent accepted a raw protected command")
	}
}

func TestEventEvidenceMetadataMakesMissingIdentityExplicitlyUnknown(t *testing.T) {
	e := Event{RunID: "run-1", AgentID: "agent-1", Source: "plugin", ConfidenceLevel: "measured"}
	meta := e.EvidenceMetadata()
	if meta.ProjectID != CoverageUnknown || meta.ChangeID != CoverageUnknown || meta.Model != CoverageUnknown {
		t.Errorf("missing identities must be unknown: %+v", meta)
	}
	if meta.RunID != "run-1" || meta.AgentID != "agent-1" || meta.Source != "plugin" {
		t.Errorf("supplied identities must survive: %+v", meta)
	}
	if meta.InteractionID != CoverageUnknown {
		t.Errorf("interaction id = %q, want unknown", meta.InteractionID)
	}
}

func TestEventEvidenceMetadataPreservesSuppliedOptionalIdentity(t *testing.T) {
	e := Event{ProjectID: "project-1", ChangeID: "change-1", SessionID: "session-1", InteractionID: "interaction-1", Provider: "openai", Model: "gpt", Effort: "high", Coverage: "measured"}
	meta := e.EvidenceMetadata()
	if meta.ProjectID != "project-1" || meta.ChangeID != "change-1" || meta.SessionID != "session-1" || meta.InteractionID != "interaction-1" {
		t.Errorf("identity was not preserved: %+v", meta)
	}
	if meta.Provider != "openai" || meta.Model != "gpt" || meta.Effort != "high" || meta.Coverage != "measured" {
		t.Errorf("provider evidence was not preserved: %+v", meta)
	}
}
