package agenttelemetry

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestUsageAccumulatorDeduplicatesCumulativeAndRegression(t *testing.T) {
	var usage UsageAccumulator
	if got := usage.Add(UsageSample{Provider: "codex", ID: "one", Total: 10, Cumulative: true, Measured: true}); got.Total != 10 || !got.Measured {
		t.Fatalf("first cumulative delta = %+v", got)
	}
	if got := usage.Add(UsageSample{Provider: "codex", ID: "two", Total: 16, Cumulative: true, Measured: true}); got.Total != 6 {
		t.Fatalf("second cumulative delta = %+v, want 6", got)
	}
	if got := usage.Add(UsageSample{Provider: "codex", ID: "two", Total: 16, Cumulative: true, Measured: true}); got.Total != 0 {
		t.Fatalf("duplicate delta = %+v, want zero", got)
	}
	if got := usage.Add(UsageSample{Provider: "codex", ID: "three", Total: 3, Cumulative: true, Measured: true}); !got.Unknown {
		t.Fatalf("regression must become unknown, got %+v", got)
	}
}

func TestSummarizeActivityMeasuredPrecedenceAndIdleCap(t *testing.T) {
	base := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	got := SummarizeActivity(base, base.Add(2*time.Hour), []ActivityInterval{
		{Start: base, End: base.Add(30 * time.Minute), Measured: true},
		{Start: base.Add(30 * time.Minute), End: base.Add(90 * time.Minute)},
		{Start: base.Add(105 * time.Minute), End: base.Add(2 * time.Hour)},
	}, 15*time.Minute)
	if got.Wall != 2*time.Hour || got.Measured != 30*time.Minute || got.Inferred != 30*time.Minute || got.Unknown != time.Hour {
		t.Fatalf("summary = %+v", got)
	}
}

func TestProjectEventsIsReplaySafe(t *testing.T) {
	store, err := OpenStore(tempDBPath(t))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	if err := store.SaveEvent(ctx, Event{EventID: "event-1", RunID: "run", EventType: "tool.called", Timestamp: time.Now(), Source: "test", Payload: []byte(`{}`)}); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := store.ProjectEvents(ctx, "v1"); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := store.db.QueryRow("SELECT COUNT(*) FROM projected_events").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("projected events = %d, want 1", count)
	}
}

func TestCaptureGitSnapshotAndVerifiedTransition(t *testing.T) {
	root := t.TempDir()
	initGitRepo(t, root)
	start, err := CaptureGitSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	if start.Root != filepath.Clean(root) || start.Head == "" || start.Branch == "" {
		t.Fatalf("snapshot = %+v", start)
	}
	runGitAt(t, root, "commit", "--allow-empty", "-m", "next")
	end, err := CaptureGitSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	count, err := CommitsCreated(root, start, end)
	if err != nil || count != 1 {
		t.Fatalf("transition = %d, %v", count, err)
	}
	if _, err := CaptureGitSnapshot("-not-a-root"); err == nil {
		t.Fatal("option-like root must fail")
	}
}

func runGitAt(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

func TestSaveTypedProjectionSamples(t *testing.T) {
	store, err := OpenStore(tempDBPath(t))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	base := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	err = store.SaveTypedProjectionSamples(context.Background(), "run-1",
		[]UsageSample{{Provider: "codex", ID: "sample-1", Total: 12, Measured: true}},
		[]ActivityInterval{{Start: base, End: base.Add(time.Minute), Measured: true}},
		GitSnapshot{Root: "/repo", Head: "abc", Branch: "main", CapturedAt: base})
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"usage_projection_samples", "activity_projection_samples", "git_projection_samples"} {
		var count int
		if err := store.db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil || count != 1 {
			t.Fatalf("%s = %d, %v", table, count, err)
		}
	}
}

func TestDecodeProjectionPayloadBoundsAndContracts(t *testing.T) {
	tests := []struct {
		name, eventType, payload string
		coverage                 string
		usage, activity, git     bool
	}{
		{"usage", "model.usage", `{"schema_version":1,"sample_id":"s","interaction_id":"i","mode":"delta","method":"measured","estimated_method":null,"tokens":{"input":12,"output":3,"cache_read":null,"cache_write":null,"reasoning":0}}`, "measured", true, false, false},
		{"cumulative without segment", "model.usage", `{"schema_version":1,"sample_id":"s","interaction_id":"i","mode":"cumulative","method":"measured","estimated_method":null,"tokens":{"input":12,"output":3,"cache_read":null,"cache_write":null,"reasoning":0}}`, CoverageUnknown, false, false, false},
		{"activity span", "activity.sample", `{"schema_version":1,"sample_id":"a","kind":"span","method":"measured","start":"2026-08-26T01:00:00Z","end":"2026-08-26T01:00:05Z","at":null,"clock":{"source":"provider_wall","clock_id":"hmac:x","uncertainty_ms":20}}`, "measured", false, true, false},
		{"lifecycle mismatch", "run.started", `{"schema_version":1,"git_snapshot":{"phase":"end","capture":"ok","root_id":"hmac:r","head":"0123456789012345678901234567890123456789","branch":null,"detached":true,"dirty":false,"staged":0,"unstaged":0,"untracked":0,"captured_at":"2026-08-26T01:05:00Z","error_code":null}}`, CoverageUnknown, false, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DecodeProjectionPayload(Event{EventType: tt.eventType, InteractionID: "i", Payload: []byte(tt.payload)})
			if got.Coverage != tt.coverage || (got.Usage != nil) != tt.usage || (got.Activity != nil) != tt.activity || (got.Git != nil) != tt.git {
				t.Fatalf("projection = %+v", got)
			}
		})
	}
}

func TestUsageAccumulatorAllowsDeclaredResetSegment(t *testing.T) {
	var usage UsageAccumulator
	usage.Add(UsageSample{Provider: "codex", ID: "one", Total: 10, Cumulative: true, Measured: true, Segment: "first"})
	if got := usage.Add(UsageSample{Provider: "codex", ID: "two", Total: 3, Cumulative: true, Measured: true, Segment: "second", Reset: true}); got.Total != 3 || got.Unknown {
		t.Fatalf("reset delta = %+v", got)
	}
}

func TestProjectEventsMapsBoundedUsagePayload(t *testing.T) {
	store, err := OpenStore(tempDBPath(t))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	e := Event{EventID: "usage-1", RunID: "run-1", EventType: "model.usage", Timestamp: time.Now(), Source: "test", Provider: "codex", InteractionID: "i", Payload: []byte(`{"schema_version":1,"sample_id":"s","interaction_id":"i","mode":"delta","method":"measured","estimated_method":null,"tokens":{"input":12,"output":3,"cache_read":null,"cache_write":null,"reasoning":0}}`)}
	if err := store.SaveEvent(context.Background(), e); err != nil {
		t.Fatal(err)
	}
	if err := store.ProjectEvents(context.Background(), "v2"); err != nil {
		t.Fatal(err)
	}
	var total int
	if err := store.db.QueryRow("SELECT total FROM usage_projection_samples WHERE provider='codex'").Scan(&total); err != nil || total != 15 {
		t.Fatalf("usage total = %d, %v", total, err)
	}
}

func TestProjectEventsMapsActivityAndGitPayloads(t *testing.T) {
	store, err := OpenStore(tempDBPath(t))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	at := time.Now().UTC().Format(time.RFC3339)
	events := []Event{
		{EventID: "activity-1", RunID: "run-2", EventType: "activity.sample", Timestamp: time.Now(), Source: "test", Payload: []byte(`{"schema_version":1,"sample_id":"a","kind":"span","method":"measured","start":"2026-08-26T01:00:00Z","end":"2026-08-26T01:00:05Z","at":null,"clock":{"source":"provider_wall","clock_id":"hmac:x","uncertainty_ms":0}}`)},
		{EventID: "git-1", RunID: "run-2", EventType: "run.started", Timestamp: time.Now(), Source: "test", Payload: []byte(`{"schema_version":1,"git_snapshot":{"phase":"start","capture":"ok","root_id":"hmac:r","head":"0123456789012345678901234567890123456789","branch":"main","detached":false,"dirty":true,"staged":1,"unstaged":1,"untracked":1,"captured_at":"` + at + `","error_code":null}}`)},
	}
	for _, e := range events {
		if err := store.SaveEvent(context.Background(), e); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.ProjectEvents(context.Background(), "v2"); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"activity_projection_samples", "git_projection_samples"} {
		var n int
		if err := store.db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); err != nil || n != 1 {
			t.Fatalf("%s = %d, %v", table, n, err)
		}
	}
}

func TestCaptureGitSnapshotCountsWorktreeStatesAndNonRepo(t *testing.T) {
	root := t.TempDir()
	initGitRepo(t, root)
	if err := os.WriteFile(filepath.Join(root, "staged"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	runGitAt(t, root, "add", "staged")
	if err := os.WriteFile(filepath.Join(root, "unstaged"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	runGitAt(t, root, "add", "unstaged")
	runGitAt(t, root, "commit", "-m", "tracked")
	if err := os.WriteFile(filepath.Join(root, "staged-2"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	runGitAt(t, root, "add", "staged-2")
	if err := os.WriteFile(filepath.Join(root, "unstaged"), []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "untracked"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := CaptureGitSnapshot(root)
	if err != nil || got.Staged != 1 || got.Unstaged != 1 || got.Untracked != 1 {
		t.Fatalf("snapshot = %+v, %v", got, err)
	}
	if _, err := CaptureGitSnapshot(t.TempDir()); err == nil {
		t.Fatal("non-repository must fail")
	}
}
