package app

import (
	"context"
	"testing"
	"time"

	"github.com/quantum-6/skillvault/internal/db"
	"github.com/quantum-6/skillvault/internal/domain"
)

type mockEntryStore struct {
	entries []domain.EntryListResult
}

func (m *mockEntryStore) List(ctx context.Context, filter domain.EntryFilter) ([]domain.EntryListResult, error) {
	return m.entries, nil
}

type mockArtifactStore struct {
	artifacts []domain.Artifact
}

func (m *mockArtifactStore) List(ctx context.Context, projectID *string) ([]domain.Artifact, error) {
	return m.artifacts, nil
}

type mockProjectStore struct {
	projects []domain.Project
}

func (m *mockProjectStore) List(ctx context.Context, includeArchived bool) ([]domain.Project, error) {
	return m.projects, nil
}

func TestStatsService_EmptyVault(t *testing.T) {
	svc := NewStatsService(&mockEntryStore{}, &mockArtifactStore{}, &mockProjectStore{})
	stats, err := svc.GetStats(context.Background())
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}
	if stats.TotalEntries != 0 {
		t.Errorf("TotalEntries = %d, want 0", stats.TotalEntries)
	}
	if stats.TotalArtifacts != 0 {
		t.Errorf("TotalArtifacts = %d, want 0", stats.TotalArtifacts)
	}
	if stats.TotalProjects != 0 {
		t.Errorf("TotalProjects = %d, want 0", stats.TotalProjects)
	}
	if stats.TokenEstimate != 0 {
		t.Errorf("TokenEstimate = %d, want 0", stats.TokenEstimate)
	}

	out := FormatStats(stats)
	if out == "" {
		t.Error("FormatStats returned empty string")
	}
}

func TestStatsService_WithData(t *testing.T) {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 10, 0, 0, 0, now.Location())
	yesterday := today.AddDate(0, 0, -1)

	entryStore := &mockEntryStore{
		entries: []domain.EntryListResult{
			{
				Entry: domain.Entry{
					ID:           "e1",
					Summary:      "Today's entry",
					BodyOptional: "This is the body content with some text",
					CreatedAt:    today,
				},
			},
			{
				Entry: domain.Entry{
					ID:           "e2",
					Summary:      "Yesterday's entry",
					BodyOptional: "More body text here for testing",
					CreatedAt:    yesterday,
				},
			},
		},
	}

	artifactStore := &mockArtifactStore{
		artifacts: []domain.Artifact{
			{
				ID:        "a1",
				Title:     "Today's artifact",
				CreatedAt: today,
			},
		},
	}

	projectStore := &mockProjectStore{
		projects: []domain.Project{
			{ID: "p1", Name: "Test Project"},
		},
	}

	svc := NewStatsService(entryStore, artifactStore, projectStore)
	stats, err := svc.GetStats(context.Background())
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}

	if stats.TotalEntries != 2 {
		t.Errorf("TotalEntries = %d, want 2", stats.TotalEntries)
	}
	if stats.TotalArtifacts != 1 {
		t.Errorf("TotalArtifacts = %d, want 1", stats.TotalArtifacts)
	}
	if stats.TotalProjects != 1 {
		t.Errorf("TotalProjects = %d, want 1", stats.TotalProjects)
	}
	if stats.TodayEntries != 1 {
		t.Errorf("TodayEntries = %d, want 1", stats.TodayEntries)
	}
	if stats.TodayArtifacts != 1 {
		t.Errorf("TodayArtifacts = %d, want 1", stats.TodayArtifacts)
	}
	if stats.TotalChars == 0 {
		t.Errorf("TotalChars should be > 0")
	}

	out := FormatStats(stats)
	t.Logf("Stats output: %s", out)
	if out == "" {
		t.Error("FormatStats returned empty string")
	}
}

func TestFormatStats_Empty(t *testing.T) {
	s := &VaultStats{}
	out := FormatStats(s)
	expected := "📊 Vault: 0 entries, 0 artifacts, 0 projects | 0 chars (~0 tokens est.)"
	if out != expected {
		t.Errorf("FormatStats = %q, want %q", out, expected)
	}
}

func TestFormatStats_WithToday(t *testing.T) {
	s := &VaultStats{
		TotalEntries:   10,
		TotalArtifacts: 3,
		TotalProjects:  2,
		TotalChars:     40000,
		TodayEntries:   2,
		TodayArtifacts: 1,
		TodayChars:     5000,
		TokenEstimate:  10000,
	}
	out := FormatStats(s)
	t.Logf("Stats output: %s", out)
	if out == "" {
		t.Error("FormatStats returned empty string")
	}
}

type mockWorkflowRunStore struct {
	stats *db.WorkflowRunStats
}

func (m *mockWorkflowRunStore) GetRunStats(ctx context.Context, workflowID *string) (*db.WorkflowRunStats, error) {
	return m.stats, nil
}

// Task 1.8: TestGetStats_WorkflowRunsPopulated
func TestGetStats_WorkflowRunsPopulated(t *testing.T) {
	mockWRS := &mockWorkflowRunStore{
		stats: &db.WorkflowRunStats{
			TotalRuns:     10,
			CompletedRuns: 7,
			FailedRuns:    2,
			SuccessRate:   0.7,
		},
	}
	svc := NewStatsService(&mockEntryStore{}, &mockArtifactStore{}, &mockProjectStore{})
	svc.WithWorkflowRunStore(mockWRS)

	stats, err := svc.GetStats(context.Background())
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}
	if stats.WorkflowRuns == nil {
		t.Fatal("WorkflowRuns should be populated, got nil")
	}
	if stats.WorkflowRuns.TotalRuns != 10 {
		t.Errorf("WorkflowRuns.TotalRuns = %d, want 10", stats.WorkflowRuns.TotalRuns)
	}
	if stats.WorkflowRuns.CompletedRuns != 7 {
		t.Errorf("WorkflowRuns.CompletedRuns = %d, want 7", stats.WorkflowRuns.CompletedRuns)
	}
	if stats.WorkflowRuns.FailedRuns != 2 {
		t.Errorf("WorkflowRuns.FailedRuns = %d, want 2", stats.WorkflowRuns.FailedRuns)
	}
	if stats.WorkflowRuns.SuccessRate != 0.7 {
		t.Errorf("WorkflowRuns.SuccessRate = %f, want 0.7", stats.WorkflowRuns.SuccessRate)
	}
}

// Task 1.9: TestGetStats_NoWorkflowRunsWhenStoreNil
func TestGetStats_NoWorkflowRunsWhenStoreNil(t *testing.T) {
	svc := NewStatsService(&mockEntryStore{}, &mockArtifactStore{}, &mockProjectStore{})
	// Do NOT call WithWorkflowRunStore — workflowRunStore stays nil.

	stats, err := svc.GetStats(context.Background())
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}
	if stats.WorkflowRuns != nil {
		t.Errorf("WorkflowRuns should be nil without store, got %+v", stats.WorkflowRuns)
	}
}
