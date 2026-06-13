package context

import (
	"context"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/quantum-6/skillvault/internal/db"
	"github.com/quantum-6/skillvault/internal/domain"
)

func setupCompiler(t *testing.T) (*HermesCompiler, *db.Store, func()) {
	t.Helper()

	sqlDB, err := db.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB failed: %v", err)
	}
	if err := db.RunMigrations(sqlDB); err != nil {
		sqlDB.Close()
		t.Fatalf("RunMigrations failed: %v", err)
	}

	store := db.NewStore(sqlDB)
	compiler := NewHermesCompiler(store.Entries, store.Projects, store.Workflows, store.Artifacts)

	cleanup := func() { sqlDB.Close() }

	ctx := context.Background()

	store.Projects.Save(ctx, domain.Project{
		ID:          "proj-1",
		Name:        "TestProject",
		Slug:        "testproject",
		Description: "A test project for compiler tests",
		Status:      domain.StatusActive,
	})

	store.Entries.Save(ctx, domain.Entry{
		ID: "fb-1", Title: "Prefer YAML over JSON", Slug: "prefer-yaml",
		Type: domain.EntryTypeFeedback, Summary: "User prefers YAML configs", Status: domain.StatusActive,
	}, nil)

	store.Entries.Save(ctx, domain.Entry{
		ID: "user-1", Title: "Short summaries", Slug: "short-summaries",
		Type: domain.EntryTypeUser, Summary: "Keep summaries under 3 lines", Status: domain.StatusActive,
	}, nil)

	store.Entries.Save(ctx, domain.Entry{
		ID: "dec-1", Title: "Use SQLite FTS5", Slug: "use-sqlite-fts5",
		Type: domain.EntryTypeDecision, Summary: "FTS5 for full-text search", Status: domain.StatusCanonical,
		ProjectID: strPtr("proj-1"),
	}, nil)

	store.Entries.Save(ctx, domain.Entry{
		ID: "dec-2", Title: "No vector DB", Slug: "no-vector-db",
		Type: domain.EntryTypeDecision, Summary: "Skip vector search in v1", Status: domain.StatusActive,
		ProjectID: strPtr("proj-1"),
	}, nil)

	store.Entries.Save(ctx, domain.Entry{
		ID: "state-1", Title: "Phase 5 complete", Slug: "phase-5-done",
		Type: domain.EntryTypeProjectState, Summary: "MCP layer is implemented", Status: domain.StatusActive,
		ProjectID: strPtr("proj-1"),
	}, nil)

	store.Entries.Save(ctx, domain.Entry{
		ID: "skill-1", Title: "Go testing patterns", Slug: "go-test",
		Type: domain.EntryTypeSkill, Summary: "Standard Go test layout with table tests", Status: domain.StatusActive,
	}, nil)

	store.Entries.Save(ctx, domain.Entry{
		ID: "skill-2", Title: "SQL migrator", Slug: "sql-migrator",
		Type: domain.EntryTypeSkill, Summary: "Database migration patterns for SQLite", Status: domain.StatusActive,
	}, nil)

	store.Entries.Save(ctx, domain.Entry{
		ID: "sess-1", Title: "Session Apr 10", Slug: "session-apr10",
		Type: domain.EntryTypeSession, Summary: "Implemented artifact store", Status: domain.StatusActive,
		ProjectID: strPtr("proj-1"),
	}, nil)

	store.Entries.Save(ctx, domain.Entry{
		ID: "sess-2", Title: "Session Apr 11", Slug: "session-apr11",
		Type: domain.EntryTypeSession, Summary: "Added workflow rendering", Status: domain.StatusActive,
		ProjectID: strPtr("proj-1"),
	}, nil)

	store.Entries.Save(ctx, domain.Entry{
		ID: "sess-3", Title: "Session Apr 12", Slug: "session-apr12",
		Type: domain.EntryTypeSession, Summary: "Context compiler design", Status: domain.StatusActive,
		ProjectID: strPtr("proj-1"),
	}, nil)

	store.Entries.Save(ctx, domain.Entry{
		ID: "ref-1", Title: "SQLite docs", Slug: "sqlite-docs",
		Type: domain.EntryTypeReference, Summary: "SQLite FTS5 documentation reference", Status: domain.StatusActive,
		ProjectID: strPtr("proj-1"),
	}, nil)

	store.Entries.Save(ctx, domain.Entry{
		ID: "arch-1", Title: "Old prototype", Slug: "old-prototype",
		Type: domain.EntryTypeSkill, Summary: "Deprecated Python prototype", Status: domain.StatusArchived,
	}, nil)

	store.Workflows.Save(ctx, domain.Workflow{
		ID: "wf-1", Name: "spec-plan-task", Description: "Spec first, then plan, then tasks",
		Status: domain.StatusActive,
	}, []domain.WorkflowStep{
		{OrderIndex: 1, Title: "Write spec", Instruction: "Draft spec doc", Required: true},
		{OrderIndex: 2, Title: "Plan tasks", Instruction: "Break into tasks", Required: true},
		{OrderIndex: 3, Title: "Implement", Instruction: "Code each task", Required: true},
	})

	store.Artifacts.Save(ctx, domain.Artifact{
		ID: "art-1", Title: "Architecture doc", Type: domain.ArtifactTypeMarkdown,
		Summary: "System architecture overview", ProjectID: strPtr("proj-1"),
	})

	return compiler, store, cleanup
}

func TestCompiler_ProfileMode(t *testing.T) {
	compiler, _, cleanup := setupCompiler(t)
	defer cleanup()
	ctx := context.Background()

	result, err := compiler.Compile(ctx, Input{
		Mode:            "profile",
		ExcludeArchived: true,
		MaxChars:        5000,
	})
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !strings.HasPrefix(result, "# CONTEXT PACK") {
		t.Errorf("expected header, got:\n%s", result)
	}
	if !strings.Contains(result, "## Scope") {
		t.Error("expected Scope section")
	}
	if !strings.Contains(result, "Prefer YAML") {
		t.Error("expected feedback entry 'Prefer YAML' in profile mode")
	}
	if !strings.Contains(result, "Short summaries") {
		t.Error("expected user entry 'Short summaries' in profile mode")
	}
	if !strings.Contains(result, "## Suggested Next Action") {
		t.Error("expected Suggested Next Action section")
	}
}

func TestCompiler_ProjectMode(t *testing.T) {
	compiler, _, cleanup := setupCompiler(t)
	defer cleanup()
	ctx := context.Background()

	result, err := compiler.Compile(ctx, Input{
		Mode:            "project",
		Project:         "proj-1",
		ExcludeArchived: true,
		MaxChars:        5000,
	})
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !strings.Contains(result, "TestProject") {
		t.Error("expected project name in project mode")
	}
	if !strings.Contains(result, "Phase 5 complete") {
		t.Error("expected project state entry")
	}
	if !strings.Contains(result, "Use SQLite FTS5") {
		t.Error("expected decision entry")
	}
	if !strings.Contains(result, "Session Apr 10") {
		t.Error("expected session entry")
	}
	if strings.Contains(result, "Prefer YAML") {
		t.Error("profile mode should not include feedback in project mode")
	}
}

func TestCompiler_WorkflowMode(t *testing.T) {
	compiler, _, cleanup := setupCompiler(t)
	defer cleanup()
	ctx := context.Background()

	result, err := compiler.Compile(ctx, Input{
		Mode:            "workflow",
		ExcludeArchived: true,
		MaxChars:        5000,
	})
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !strings.Contains(result, "spec-plan-task") {
		t.Error("expected workflow name")
	}
	if !strings.Contains(result, "Write spec") {
		t.Error("expected workflow step 'Write spec'")
	}
	if !strings.Contains(result, "Plan tasks") {
		t.Error("expected workflow step 'Plan tasks'")
	}
}

func TestCompiler_SkillMode(t *testing.T) {
	compiler, _, cleanup := setupCompiler(t)
	defer cleanup()
	ctx := context.Background()

	result, err := compiler.Compile(ctx, Input{
		Mode:            "skill",
		Query:           "test",
		ExcludeArchived: true,
		MaxChars:        5000,
	})
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !strings.Contains(result, "Go testing patterns") {
		t.Error("expected skill entry matching query")
	}
	if !strings.Contains(result, "## Skills") {
		t.Error("expected Skills section")
	}
}

func TestCompiler_SkillModeNoQuery(t *testing.T) {
	compiler, _, cleanup := setupCompiler(t)
	defer cleanup()
	ctx := context.Background()

	result, err := compiler.Compile(ctx, Input{
		Mode:            "skill",
		ExcludeArchived: true,
		MaxChars:        5000,
	})
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !strings.Contains(result, "Go testing patterns") {
		t.Error("expected skill entry without query")
	}
}

func TestCompiler_PlanningMode(t *testing.T) {
	compiler, _, cleanup := setupCompiler(t)
	defer cleanup()
	ctx := context.Background()

	result, err := compiler.Compile(ctx, Input{
		Mode:            "planning",
		Project:         "proj-1",
		ExcludeArchived: true,
		MaxChars:        5000,
	})
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !strings.Contains(result, "TestProject") {
		t.Error("expected project state in planning mode")
	}
	if !strings.Contains(result, "Use SQLite FTS5") {
		t.Error("expected decisions in planning mode")
	}
	if !strings.Contains(result, "spec-plan-task") {
		t.Error("expected workflows in planning mode")
	}
	if !strings.Contains(result, "Session Apr 10") {
		t.Error("expected recent sessions in planning mode")
	}
}

func TestCompiler_SessionRecallMode(t *testing.T) {
	compiler, _, cleanup := setupCompiler(t)
	defer cleanup()
	ctx := context.Background()

	result, err := compiler.Compile(ctx, Input{
		Mode:            "session_recall",
		Project:         "proj-1",
		ExcludeArchived: true,
		MaxChars:        5000,
	})
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !strings.Contains(result, "Session Apr 10") {
		t.Error("expected recent sessions in session_recall mode")
	}
	if !strings.Contains(result, "Implemented artifact store") {
		t.Error("expected session summary content")
	}
}

func TestCompiler_FullBriefMode(t *testing.T) {
	compiler, _, cleanup := setupCompiler(t)
	defer cleanup()
	ctx := context.Background()

	result, err := compiler.Compile(ctx, Input{
		Mode:            "full_brief",
		Project:         "proj-1",
		ExcludeArchived: true,
		MaxChars:        5000,
	})
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !strings.Contains(result, "Prefer YAML") {
		t.Error("expected feedback in full_brief mode")
	}
	if !strings.Contains(result, "TestProject") {
		t.Error("expected project state in full_brief mode")
	}
	if !strings.Contains(result, "Use SQLite FTS5") {
		t.Error("expected decisions in full_brief mode")
	}
	if !strings.Contains(result, "spec-plan-task") {
		t.Error("expected workflows in full_brief mode")
	}
	if !strings.Contains(result, "Session Apr 10") {
		t.Error("expected sessions in full_brief mode")
	}
}

func TestCompiler_ExcludeArchived(t *testing.T) {
	compiler, _, cleanup := setupCompiler(t)
	defer cleanup()
	ctx := context.Background()

	result, err := compiler.Compile(ctx, Input{
		Mode:            "full_brief",
		ExcludeArchived: true,
		MaxChars:        5000,
	})
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if strings.Contains(result, "Old prototype") {
		t.Error("archived entry should be excluded when ExcludeArchived=true")
	}
}

func TestCompiler_IncludeArchived(t *testing.T) {
	compiler, _, cleanup := setupCompiler(t)
	defer cleanup()
	ctx := context.Background()

	result, err := compiler.Compile(ctx, Input{
		Mode:            "full_brief",
		ExcludeArchived: false,
		Include:         []string{"profile", "decisions", "workflows", "recent_sessions", "artifact_summaries", "references", "archived"},
		MaxChars:        5000,
	})
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !strings.Contains(result, "Old prototype") {
		t.Error("archived entry should be included when ExcludeArchived=false and archived in include list")
	}
}

func TestCompiler_MaxCharsTruncation(t *testing.T) {
	compiler, _, cleanup := setupCompiler(t)
	defer cleanup()
	ctx := context.Background()

	result, err := compiler.Compile(ctx, Input{
		Mode:            "full_brief",
		Project:         "proj-1",
		ExcludeArchived: true,
		MaxChars:        300,
	})
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if len(result) > 350 {
		t.Errorf("expected result near 300 chars, got %d:\n%s", len(result), result)
	}
	if !strings.HasPrefix(result, "# CONTEXT PACK") {
		t.Error("header must be preserved even with truncation")
	}
}

func TestCompiler_MaxCharsPreservesHighPriority(t *testing.T) {
	compiler, _, cleanup := setupCompiler(t)
	defer cleanup()
	ctx := context.Background()

	result, err := compiler.Compile(ctx, Input{
		Mode:            "full_brief",
		Project:         "proj-1",
		ExcludeArchived: true,
		MaxChars:        500,
	})
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	hasLowPriority := strings.Contains(result, "SQLite docs") ||
		strings.Contains(result, "Architecture doc")

	hasHighPriority := strings.Contains(result, "Prefer YAML") ||
		strings.Contains(result, "Short summaries")

	if hasLowPriority && !hasHighPriority {
		t.Error("expected high priority (preferences) to be preserved over low priority when truncated")
	}
}

func TestCompiler_DefaultMaxChars(t *testing.T) {
	compiler, _, cleanup := setupCompiler(t)
	defer cleanup()
	ctx := context.Background()

	result, err := compiler.Compile(ctx, Input{
		Mode:            "profile",
		ExcludeArchived: true,
	})
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if len(result) > 12000+100 {
		t.Errorf("expected result under default 12000 chars, got %d", len(result))
	}
}

func TestCompiler_OutputStructure(t *testing.T) {
	compiler, _, cleanup := setupCompiler(t)
	defer cleanup()
	ctx := context.Background()

	for _, mode := range []string{"profile", "project", "workflow", "skill", "planning", "session_recall", "full_brief"} {
		t.Run(mode, func(t *testing.T) {
			result, err := compiler.Compile(ctx, Input{
				Mode:            mode,
				Project:         "proj-1",
				ExcludeArchived: true,
				MaxChars:        5000,
			})
			if err != nil {
				t.Fatalf("Compile failed for mode %s: %v", mode, err)
			}

			if !strings.HasPrefix(result, "# CONTEXT PACK") {
				t.Errorf("mode %s: missing header", mode)
			}
			if !strings.Contains(result, "## Scope") {
				t.Errorf("mode %s: missing Scope section", mode)
			}
			if !strings.Contains(result, "## Suggested Next Action") {
				t.Errorf("mode %s: missing Suggested Next Action", mode)
			}
			if mode == "profile" && !strings.Contains(result, "Mode: profile") {
				t.Errorf("mode %s: scope should contain mode", mode)
			}
		})
	}
}

func TestCompiler_ProjectModeIncludesProjectState(t *testing.T) {
	t.Skip("Handled by TestCompiler_ProjectMode")
}

func TestCompiler_PlanningModeIncludesWorkflows(t *testing.T) {
	t.Skip("Handled by TestCompiler_PlanningMode")
}

func TestCompiler_NoData(t *testing.T) {
	// Create a compiler with no data at all
	sqlDB, err := db.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB failed: %v", err)
	}
	if err := db.RunMigrations(sqlDB); err != nil {
		sqlDB.Close()
		t.Fatalf("RunMigrations failed: %v", err)
	}
	store := db.NewStore(sqlDB)
	compiler := NewHermesCompiler(store.Entries, store.Projects, store.Workflows, store.Artifacts)
	defer sqlDB.Close()

	ctx := context.Background()

	result, err := compiler.Compile(ctx, Input{
		Mode:            "full_brief",
		ExcludeArchived: true,
		MaxChars:        5000,
	})
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !strings.HasPrefix(result, "# CONTEXT PACK") {
		t.Error("expected header even with no data")
	}
	if !strings.Contains(result, "Mode: full_brief") {
		t.Error("expected scope with mode")
	}
}
