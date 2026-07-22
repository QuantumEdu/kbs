package agenttelemetry

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func tempDBPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "test.db")
}

func TestStoreOpenCreateTables(t *testing.T) {
	dbPath := tempDBPath(t)
	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer store.Close()

	// Verify tables exist by querying sqlite_master.
	rows, err := store.db.Query("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")
	if err != nil {
		t.Fatalf("query tables: %v", err)
	}
	defer rows.Close()

	tables := make(map[string]bool)
	for rows.Next() {
		var name string
		rows.Scan(&name)
		tables[name] = true
	}

	expected := []string{"agent_runs", "agent_steps", "events", "token_usage", "tool_calls"}
	for _, tbl := range expected {
		if !tables[tbl] {
			t.Errorf("table %q not created", tbl)
		}
	}
}

func TestStoreSaveAndGetRun(t *testing.T) {
	dbPath := tempDBPath(t)
	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	started := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)

	run := AgentRun{
		ID: "run-001", AgentID: "opencode", AgentVersion: "0.1.0",
		Workspace: "/home/user/project", RepoURL: "https://github.com/org/repo",
		Branch: "main", CommitSHA: "abc123",
		StartedAt: started, Status: "running",
	}

	if err := store.SaveRun(ctx, run); err != nil {
		t.Fatalf("SaveRun: %v", err)
	}

	got, err := store.GetRun(ctx, "run-001")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if got.ID != run.ID || got.AgentID != run.AgentID || got.Status != "running" {
		t.Errorf("GetRun mismatch: got %+v", got)
	}
	if got.Workspace != run.Workspace || got.RepoURL != run.RepoURL {
		t.Errorf("workspace/repo mismatch: %q %q", got.Workspace, got.RepoURL)
	}
	if got.Branch != "main" || got.CommitSHA != "abc123" {
		t.Errorf("branch/commit mismatch: %q %q", got.Branch, got.CommitSHA)
	}
}

func TestStoreGetRunNotFound(t *testing.T) {
	dbPath := tempDBPath(t)
	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer store.Close()

	_, err = store.GetRun(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent run")
	}
}

func TestStoreSaveEvent(t *testing.T) {
	dbPath := tempDBPath(t)
	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	started := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)

	// Create a run first (FK constraint).
	store.SaveRun(ctx, AgentRun{
		ID: "run-001", AgentID: "opencode", AgentVersion: "0.1.0",
		Workspace: "/tmp", StartedAt: started, Status: "running",
	})

	e := Event{
		EventID: "evt-001", EventType: "tool.called",
		Timestamp: started.Add(time.Minute),
		RunID: "run-001", AgentID: "opencode", AgentVersion: "0.1.0",
		Source: "plugin", RedactionPolicy: "hash-args", ConfidenceLevel: "measured",
		Payload: json.RawMessage(`{"tool_name":"bash","args_hash":"abc"}`),
	}

	if err := store.SaveEvent(ctx, e); err != nil {
		t.Fatalf("SaveEvent: %v", err)
	}

	// Verify event was saved by counting.
	var count int
	err = store.db.QueryRow("SELECT COUNT(*) FROM events WHERE id = ?", e.EventID).Scan(&count)
	if err != nil {
		t.Fatalf("count events: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 event, got %d", count)
	}
}

func TestStoreListRuns(t *testing.T) {
	dbPath := tempDBPath(t)
	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	started := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)

	runs := []AgentRun{
		{ID: "run-001", AgentID: "opencode", AgentVersion: "0.1.0", Workspace: "/tmp/a", StartedAt: started, Status: "completed", TotalTokens: 100},
		{ID: "run-002", AgentID: "claude-code", AgentVersion: "1.0.0", Workspace: "/tmp/b", StartedAt: started.Add(time.Hour), Status: "running", TotalTokens: 200},
		{ID: "run-003", AgentID: "opencode", AgentVersion: "0.1.0", Workspace: "/tmp/c", StartedAt: started.Add(2 * time.Hour), Status: "failed", TotalTokens: 300},
	}

	for _, r := range runs {
		if err := store.SaveRun(ctx, r); err != nil {
			t.Fatalf("SaveRun %s: %v", r.ID, err)
		}
	}

	// List all runs.
	all, err := store.ListRuns(ctx, RunFilter{Limit: 10})
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 runs, got %d", len(all))
	}

	// Filter by agent.
	filtered, err := store.ListRuns(ctx, RunFilter{AgentID: "opencode", Limit: 10})
	if err != nil {
		t.Fatalf("ListRuns filtered: %v", err)
	}
	if len(filtered) != 2 {
		t.Errorf("expected 2 opencode runs, got %d", len(filtered))
	}
	for _, r := range filtered {
		if r.AgentID != "opencode" {
			t.Errorf("unexpected agent %q in filtered results", r.AgentID)
		}
	}

	// Limit.
	limited, err := store.ListRuns(ctx, RunFilter{Limit: 1})
	if err != nil {
		t.Fatalf("ListRuns limited: %v", err)
	}
	if len(limited) != 1 {
		t.Errorf("expected 1 run with limit, got %d", len(limited))
	}
}

func TestStoreStatus(t *testing.T) {
	dbPath := tempDBPath(t)
	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	started := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)

	store.SaveRun(ctx, AgentRun{
		ID: "run-001", AgentID: "opencode", AgentVersion: "0.1.0",
		Workspace: "/tmp", StartedAt: started, Status: "running",
	})
	store.SaveEvent(ctx, Event{
		EventID: "evt-001", EventType: "run.started",
		Timestamp: started, RunID: "run-001", AgentID: "opencode", AgentVersion: "0.1.0",
		Source: "plugin", RedactionPolicy: "hash-args", ConfidenceLevel: "measured",
		Payload: json.RawMessage(`{}`),
	})

	status, err := store.Status(ctx)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.EventsIngested < 1 {
		t.Errorf("expected at least 1 event ingested, got %d", status.EventsIngested)
	}

	// DB size should be positive.
	info, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("stat db: %v", err)
	}
	if info.Size() == 0 {
		t.Error("db file should be non-zero")
	}
}

func TestStoreWALMode(t *testing.T) {
	dbPath := tempDBPath(t)
	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer store.Close()

	var journalMode string
	err = store.db.QueryRow("PRAGMA journal_mode").Scan(&journalMode)
	if err != nil {
		t.Fatalf("pragma journal_mode: %v", err)
	}
	if journalMode != "wal" {
		t.Errorf("expected WAL journal mode, got %q", journalMode)
	}
}

func TestStoreRunWithCompletedAt(t *testing.T) {
	dbPath := tempDBPath(t)
	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	started := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	completed := time.Date(2026, 7, 22, 10, 5, 0, 0, time.UTC)

	run := AgentRun{
		ID: "run-001", AgentID: "opencode", AgentVersion: "0.1.0",
		Workspace: "/tmp", StartedAt: started, CompletedAt: &completed,
		Status: "completed", TotalTokens: 1000, TotalCostUSD: 0.05,
	}

	if err := store.SaveRun(ctx, run); err != nil {
		t.Fatalf("SaveRun: %v", err)
	}

	got, err := store.GetRun(ctx, "run-001")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if got.Status != "completed" {
		t.Errorf("expected completed, got %q", got.Status)
	}
	if got.CompletedAt == nil {
		t.Fatal("expected non-nil CompletedAt")
	}
	if !got.CompletedAt.Equal(completed) {
		t.Errorf("CompletedAt mismatch: got %v, want %v", got.CompletedAt, completed)
	}
	if got.TotalTokens != 1000 || got.TotalCostUSD != 0.05 {
		t.Errorf("token/cost mismatch: %d / %f", got.TotalTokens, got.TotalCostUSD)
	}
}

func TestStoreSaveEventWithOptionalFields(t *testing.T) {
	dbPath := tempDBPath(t)
	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	started := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)

	store.SaveRun(ctx, AgentRun{
		ID: "run-001", AgentID: "opencode", AgentVersion: "0.1.0",
		Workspace: "/tmp", StartedAt: started, Status: "running",
	})

	corrID := "evt-parent-001"
	stepID := "step-001"

	e := Event{
		EventID: "evt-002", EventType: "tool.called",
		Timestamp: started, RunID: "run-001", AgentID: "opencode", AgentVersion: "0.1.0",
		Source: "plugin", CorrelationID: &corrID, StepID: &stepID,
		RedactionPolicy: "none", ConfidenceLevel: "estimated",
		Payload: []byte(`{"tool_name":"read","args_hash":"sha256:xyz"}`),
	}

	if err := store.SaveEvent(ctx, e); err != nil {
		t.Fatalf("SaveEvent: %v", err)
	}

	var corr, step, policy, level sql.NullString
	err = store.db.QueryRow(
		"SELECT correlation_id, step_id, redaction_policy, confidence_level FROM events WHERE id = ?",
		e.EventID,
	).Scan(&corr, &step, &policy, &level)
	if err != nil {
		t.Fatalf("query event: %v", err)
	}
	if !corr.Valid || corr.String != "evt-parent-001" {
		t.Errorf("correlation_id: %v", corr)
	}
	if !step.Valid || step.String != "step-001" {
		t.Errorf("step_id: %v", step)
	}
	if policy.String != "none" {
		t.Errorf("redaction_policy: %q", policy.String)
	}
	if level.String != "estimated" {
		t.Errorf("confidence_level: %q", level.String)
	}
}

func TestStoreListRunsEmpty(t *testing.T) {
	dbPath := tempDBPath(t)
	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer store.Close()

	runs, err := store.ListRuns(context.Background(), RunFilter{Limit: 10})
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(runs) != 0 {
		t.Errorf("expected 0 runs, got %d", len(runs))
	}
}

func TestStoreClose(t *testing.T) {
	dbPath := tempDBPath(t)
	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}

	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// After close, operations should fail.
	err = store.SaveRun(context.Background(), AgentRun{ID: "run-001", AgentID: "x", Workspace: "/tmp", StartedAt: time.Now(), Status: "running"})
	if err == nil {
		t.Error("expected error after Close")
	}
}
