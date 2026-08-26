package agenttelemetry

import (
	"context"
	"fmt"
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

func TestProjectEventsRetainsLifecyclePhasesAndChainsHeartbeats(t *testing.T) {
	store, err := OpenStore(tempDBPath(t))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	payload := func(phase, head string) []byte {
		return []byte(`{"schema_version":1,"git_snapshot":{"phase":"` + phase + `","capture":"ok","root_id":"hmac:r","head":"` + head + `","branch":null,"detached":true,"dirty":false,"staged":0,"unstaged":0,"untracked":0,"captured_at":"2026-08-26T01:00:00Z","error_code":null}}`)
	}
	heartbeat := func(id, at string) []byte {
		return []byte(`{"schema_version":1,"sample_id":"` + id + `","kind":"heartbeat","method":"inferred","start":null,"end":null,"at":"` + at + `","clock":{"source":"provider_wall","clock_id":"hmac:c","uncertainty_ms":0}}`)
	}
	for _, e := range []Event{{EventID: "start", RunID: "r", EventType: "run.started", Payload: payload("start", "0123456789012345678901234567890123456789")}, {EventID: "end", RunID: "r", EventType: "run.completed", Payload: payload("end", "1123456789012345678901234567890123456789")}, {EventID: "h1", RunID: "r", EventType: "activity.sample", Payload: heartbeat("h1", "2026-08-26T01:00:00Z")}, {EventID: "h2", RunID: "r", EventType: "activity.sample", Payload: heartbeat("h2", "2026-08-26T01:10:00Z")}} {
		e.Timestamp = time.Now()
		e.Source = "test"
		if err := store.SaveEvent(context.Background(), e); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.ProjectEvents(context.Background(), "v2"); err != nil {
		t.Fatal(err)
	}
	for table, want := range map[string]int{"git_lifecycle_projection_samples": 2, "activity_projection_samples": 1} {
		var n int
		if err := store.db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); err != nil || n != want {
			t.Fatalf("%s = %d, %v", table, n, err)
		}
	}
}

func TestProjectEventsRollsBackInjectedCheckpointCrash(t *testing.T) {
	store, err := OpenStore(tempDBPath(t))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	if err := store.SaveEvent(ctx, Event{EventID: "crash", RunID: "run", EventType: "tool.called", Timestamp: time.Now(), Source: "test", Payload: []byte(`{}`)}); err != nil {
		t.Fatal(err)
	}
	store.projectEventsAfterRow = func(int64) error { return context.Canceled }
	if err := store.ProjectEvents(ctx, "v2"); err == nil {
		t.Fatal("injected crash must abort the transaction")
	}
	store.projectEventsAfterRow = nil
	if err := store.ProjectEvents(ctx, "v2"); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM projected_events`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("recovered projection rows = %d, %v", count, err)
	}
}

func TestProjectEventsProjectsCumulativeUsageAsDeltas(t *testing.T) {
	store, err := OpenStore(tempDBPath(t))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	payload := func(id string, total int) []byte {
		return []byte(fmt.Sprintf(`{"schema_version":1,"sample_id":%q,"interaction_id":"i","mode":"cumulative","segment_id":"first","reset":false,"method":"measured","estimated_method":null,"tokens":{"input":%d,"output":0,"cache_read":null,"cache_write":null,"reasoning":null}}`, id, total))
	}
	for _, e := range []Event{{EventID: "one", RunID: "r", EventType: "model.usage", Provider: "p", InteractionID: "i", Payload: payload("one", 10)}, {EventID: "two", RunID: "r", EventType: "model.usage", Provider: "p", InteractionID: "i", Payload: payload("two", 16)}} {
		e.Timestamp, e.Source = time.Now(), "test"
		if err := store.SaveEvent(context.Background(), e); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.ProjectEvents(context.Background(), "v2"); err != nil {
		t.Fatal(err)
	}
	var total int
	if err := store.db.QueryRow(`SELECT COALESCE(SUM(total), 0) FROM usage_projection_samples WHERE provider='p'`).Scan(&total); err != nil || total != 16 {
		t.Fatalf("cumulative deltas total = %d, %v; want 16", total, err)
	}
}

func TestProjectEventsPersistsUnknownCumulativeRegressionAndInvalidReset(t *testing.T) {
	store, err := OpenStore(tempDBPath(t))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	payload := func(id, segment string, total int, reset bool) []byte {
		return []byte(fmt.Sprintf(`{"schema_version":1,"sample_id":%q,"interaction_id":"i","mode":"cumulative","segment_id":%q,"reset":%t,"method":"measured","estimated_method":null,"tokens":{"input":%d,"output":0,"cache_read":null,"cache_write":null,"reasoning":null}}`, id, segment, reset, total))
	}
	for _, e := range []Event{{EventID: "one", RunID: "r", EventType: "model.usage", Provider: "p", InteractionID: "i", Payload: payload("one", "first", 10, false)}, {EventID: "regression", RunID: "r", EventType: "model.usage", Provider: "p", InteractionID: "i", Payload: payload("regression", "first", 3, false)}, {EventID: "invalid-reset", RunID: "r", EventType: "model.usage", Provider: "p", InteractionID: "i", Payload: payload("invalid-reset", "first", 2, true)}, {EventID: "new-reset", RunID: "r", EventType: "model.usage", Provider: "p", InteractionID: "i", Payload: payload("new-reset", "second", 4, true)}} {
		e.Timestamp, e.Source = time.Now(), "test"
		if err := store.SaveEvent(context.Background(), e); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.ProjectEvents(context.Background(), "v2"); err != nil {
		t.Fatal(err)
	}
	if err := store.ProjectEvents(context.Background(), "v2"); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM usage_projection_unknown_samples`).Scan(&n); err != nil || n != 2 {
		t.Fatalf("unknown usage samples = %d, %v; want 2", n, err)
	}
}

func TestCaptureGitSnapshotHandlesDetachedAndCommandFailure(t *testing.T) {
	root := t.TempDir()
	initGitRepo(t, root)
	runGitAt(t, root, "checkout", "--detach")
	got, err := CaptureGitSnapshot(root)
	if err != nil || !got.Detached || got.Branch != "" {
		t.Fatalf("detached snapshot = %+v, %v", got, err)
	}
	previous := gitExecutable
	gitExecutable = filepath.Join(t.TempDir(), "missing-git")
	t.Cleanup(func() { gitExecutable = previous })
	if _, err := CaptureGitSnapshot(root); err == nil {
		t.Fatal("git command failure must not produce a snapshot")
	}
}

func TestProjectEventsPersistsScopedTokenEvidenceReplaySafe(t *testing.T) {
	store, err := OpenStore(tempDBPath(t))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	payload := []byte(`{"schema_version":1,"sample_id":"sample","interaction_id":"interaction","mode":"delta","method":"measured","estimated_method":null,"tokens":{"input":12,"output":3,"cache_read":2,"cache_write":1,"reasoning":4}}`)
	e := Event{EventID: "scoped", RunID: "run", EventType: "model.usage", Timestamp: time.Now(), Source: "plugin", Provider: "opencode", Model: "gpt-5", Effort: "high", ProjectID: "project", ChangeID: "change", SessionID: "session", InteractionID: "interaction", Coverage: "complete", ConfidenceLevel: "measured", Payload: payload}
	if err := store.SaveEvent(context.Background(), e); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := store.ProjectEvents(context.Background(), "v2"); err != nil {
			t.Fatal(err)
		}
	}
	var rows, input, output, cacheRead, cacheWrite, reasoning int
	if err := store.db.QueryRow(`SELECT COUNT(*), SUM(input), SUM(output), SUM(cache_read), SUM(cache_write), SUM(reasoning) FROM usage_scope_projection_samples WHERE sample_id='sample'`).Scan(&rows, &input, &output, &cacheRead, &cacheWrite, &reasoning); err != nil {
		t.Fatal(err)
	}
	if rows != 5 || input != 60 || output != 15 || cacheRead != 10 || cacheWrite != 5 || reasoning != 20 {
		t.Fatalf("scoped evidence rows=%d dimensions=%d/%d/%d/%d/%d", rows, input, output, cacheRead, cacheWrite, reasoning)
	}
}
