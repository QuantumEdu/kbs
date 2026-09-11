package line

import (
	"context"
	"testing"
	"time"
)

func TestProjectStoreCRUD(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := openTestStore(t)

	// 1. Create first project - should automatically become active
	p1 := Project{
		ID:          "pos-app",
		Name:        "Sistema POS",
		RepoPath:    "/home/ubuntu/dev/pos-app",
		DefaultExec: "claude",
		ReviewExec:  "claude-review",
		NtfyTopic:   "pos-alerts",
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	if err := store.CreateProject(ctx, p1); err != nil {
		t.Fatalf("CreateProject p1: %v", err)
	}

	active, err := store.GetActiveProject(ctx)
	if err != nil {
		t.Fatalf("GetActiveProject: %v", err)
	}
	if active.ID != "pos-app" || !active.IsActive {
		t.Fatalf("expected p1 active, got %+v", active)
	}

	// 2. Create second project - should NOT be active by default
	p2 := Project{
		ID:          "kbs",
		Name:        "Knowledge Base",
		RepoPath:    "/home/ubuntu/dev/kbs",
		DefaultExec: "claude",
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	if err := store.CreateProject(ctx, p2); err != nil {
		t.Fatalf("CreateProject p2: %v", err)
	}

	list, err := store.ListProjects(ctx)
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(list))
	}

	// 3. Switch active project to kbs
	if err := store.SetActiveProject(ctx, "kbs"); err != nil {
		t.Fatalf("SetActiveProject kbs: %v", err)
	}

	active, err = store.GetActiveProject(ctx)
	if err != nil {
		t.Fatalf("GetActiveProject after switch: %v", err)
	}
	if active.ID != "kbs" || !active.IsActive {
		t.Fatalf("expected kbs active, got %+v", active)
	}

	// Verify pos-app is no longer active
	oldP1, err := store.GetProject(ctx, "pos-app")
	if err != nil {
		t.Fatalf("GetProject p1: %v", err)
	}
	if oldP1.IsActive {
		t.Fatalf("expected pos-app to be inactive, got %+v", oldP1)
	}

	// 4. Delete project
	if err := store.DeleteProject(ctx, "pos-app"); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}
	list, err = store.ListProjects(ctx)
	if err != nil {
		t.Fatalf("ListProjects after delete: %v", err)
	}
	if len(list) != 1 || list[0].ID != "kbs" {
		t.Fatalf("expected 1 project kbs, got %+v", list)
	}
}
