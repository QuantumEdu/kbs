package agenttelemetry

import (
	"context"
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
