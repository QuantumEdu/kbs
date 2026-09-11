package line

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestStoreCreateGetUpdate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := openTestStore(t)

	job := Job{
		ID:        "job-1",
		IssueURL:  "https://github.com/owner/repo/issues/1",
		RepoPath:  "/tmp/repo",
		Phase:     PhasePlan,
		Status:    StatusRunning,
		CreatedAt: time.Unix(1700000000, 0).UTC(),
		UpdatedAt: time.Unix(1700000000, 0).UTC(),
	}
	if err := store.CreateJob(ctx, job); err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	if err := store.AppendEvent(ctx, Event{
		ID: "evt-1", JobID: job.ID, Phase: PhasePlan, Kind: "started", Detail: "plan",
		CreatedAt: job.CreatedAt,
	}); err != nil {
		t.Fatalf("AppendEvent: %v", err)
	}

	got, err := store.GetJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if got.IssueURL != job.IssueURL || got.Phase != PhasePlan || got.Status != StatusRunning {
		t.Fatalf("unexpected job %+v", got)
	}

	got.Status = StatusNeedsHuman
	got.Question = "public API?"
	got.UpdatedAt = time.Unix(1700000060, 0).UTC()
	if err := store.UpdateJob(ctx, got); err != nil {
		t.Fatalf("UpdateJob: %v", err)
	}

	latest, err := store.LatestJob(ctx)
	if err != nil {
		t.Fatalf("LatestJob: %v", err)
	}
	if latest.ID != job.ID || latest.Question != "public API?" {
		t.Fatalf("latest %+v", latest)
	}

	events, err := store.ListEvents(ctx, job.ID)
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if len(events) != 1 || events[0].Kind != "started" {
		t.Fatalf("events %+v", events)
	}
}

func TestStoreDuplicateJobID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := openTestStore(t)
	job := Job{ID: "dup", IssueURL: "https://github.com/o/r/issues/2", RepoPath: ".", Phase: PhasePlan, Status: StatusRunning}
	if err := store.CreateJob(ctx, job); err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	if err := store.CreateJob(ctx, job); err == nil {
		t.Fatal("expected duplicate id error")
	}
}

func openTestStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "line.db")
	store, err := OpenStore(path)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}
