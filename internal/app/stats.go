package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/quantum-6/skillvault/internal/db"
	"github.com/quantum-6/skillvault/internal/domain"
)

// VaultStats holds aggregated vault statistics.
type VaultStats struct {
	TotalEntries   int                  `json:"total_entries"`
	TotalArtifacts int                  `json:"total_artifacts"`
	TotalProjects  int                  `json:"total_projects"`
	TotalChars     int                  `json:"total_chars"`
	TodayEntries   int                  `json:"today_entries"`
	TodayArtifacts int                  `json:"today_artifacts"`
	TodayChars     int                  `json:"today_chars"`
	TokenEstimate  int                  `json:"token_estimate"`
	WorkflowRuns   *db.WorkflowRunStats `json:"workflow_runs,omitempty"`
}

// StatsService provides vault-level aggregation.
type StatsService struct {
	entryStore       EntryStore
	artifactStore    ArtifactStore
	projectStore     ProjectStore
	workflowRunStore WorkflowRunStore
}

// NewStatsService creates a StatsService.
func NewStatsService(entryStore EntryStore, artifactStore ArtifactStore, projectStore ProjectStore) *StatsService {
	return &StatsService{
		entryStore:    entryStore,
		artifactStore: artifactStore,
		projectStore:  projectStore,
	}
}

// WithWorkflowRunStore attaches an optional WorkflowRunStore for run analytics.
func (s *StatsService) WithWorkflowRunStore(store WorkflowRunStore) *StatsService {
	s.workflowRunStore = store
	return s
}

// GetStats aggregates vault statistics.
func (s *StatsService) GetStats(ctx context.Context) (*VaultStats, error) {
	// All entries (including archived) for totals.
	allEntries, err := s.entryStore.List(ctx, domain.EntryFilter{IncludeArchived: true})
	if err != nil {
		return nil, fmt.Errorf("list entries: %w", err)
	}

	// All artifacts.
	artifacts, err := s.artifactStore.List(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("list artifacts: %w", err)
	}

	// All projects.
	projects, err := s.projectStore.List(ctx, true)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}

	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	stats := &VaultStats{
		TotalArtifacts: len(artifacts),
		TotalProjects:  len(projects),
	}

	for _, entry := range allEntries {
		stats.TotalEntries++
		charLen := len(entry.Entry.Summary) + len(entry.Entry.BodyOptional)
		stats.TotalChars += charLen

		if entry.Entry.CreatedAt.After(todayStart) || entry.Entry.CreatedAt.Equal(todayStart) {
			stats.TodayEntries++
			stats.TodayChars += charLen
		}
	}

	for _, a := range artifacts {
		created := a.CreatedAt
		if created.After(todayStart) || created.Equal(todayStart) {
			stats.TodayArtifacts++
		}
	}

	stats.TokenEstimate = stats.TotalChars / 4

	// Workflow run analytics (optional — only when store is wired).
	if s.workflowRunStore != nil {
		runStats, err := s.workflowRunStore.GetRunStats(ctx, nil)
		if err != nil {
			return nil, fmt.Errorf("get run stats: %w", err)
		}
		stats.WorkflowRuns = runStats
	}

	return stats, nil
}

// FormatStats produces a single-line summary string.
func FormatStats(s *VaultStats) string {
	var b strings.Builder
	b.WriteString("📊 Vault: ")
	b.WriteString(fmt.Sprintf("%d entries, %d artifacts, %d projects", s.TotalEntries, s.TotalArtifacts, s.TotalProjects))

	var todayParts []string
	if s.TodayEntries > 0 {
		todayParts = append(todayParts, fmt.Sprintf("+%d entries", s.TodayEntries))
	}
	if s.TodayArtifacts > 0 {
		todayParts = append(todayParts, fmt.Sprintf("+%d artifacts", s.TodayArtifacts))
	}
	if len(todayParts) > 0 {
		b.WriteString(" | Today: ")
		b.WriteString(strings.Join(todayParts, ", "))
	}

	if s.TotalChars >= 1000 {
		b.WriteString(fmt.Sprintf(" | ~%dK chars (~%dK tokens est.)", s.TotalChars/1000, s.TokenEstimate/1000))
	} else {
		b.WriteString(fmt.Sprintf(" | %d chars (~%d tokens est.)", s.TotalChars, s.TokenEstimate))
	}

	if s.TodayChars > 0 {
		b.WriteString(fmt.Sprintf(" | Today: +%d chars", s.TodayChars))
	}

	return b.String()
}

// FormatWorkflowRunStats produces a summary of workflow run analytics.
func FormatWorkflowRunStats(s *db.WorkflowRunStats) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("\nWorkflow Runs: %d total (%d completed, %d failed)\n", s.TotalRuns, s.CompletedRuns, s.FailedRuns))
	if s.AvgDurationSecs > 0 || s.MaxDurationSecs > 0 || s.MinDurationSecs > 0 {
		b.WriteString(fmt.Sprintf("  Duration: avg %.1fs, max %.1fs, min %.1fs\n", s.AvgDurationSecs, s.MaxDurationSecs, s.MinDurationSecs))
	}
	if s.FailedStepCount > 0 {
		b.WriteString(fmt.Sprintf("  Failed steps: %d\n", s.FailedStepCount))
	}
	if len(s.PerWorkflow) > 0 {
		b.WriteString("\nPer Workflow:\n")
		for _, pw := range s.PerWorkflow {
			b.WriteString(fmt.Sprintf("  %s: %d runs, %d completed, avg %.1fs\n",
				pw.WorkflowID, pw.TotalRuns, pw.CompletedRuns, pw.AvgDurationSecs))
		}
	}
	return b.String()
}

// EntryStore is the subset of db.EntryStore needed by StatsService.
type EntryStore interface {
	List(ctx context.Context, filter domain.EntryFilter) ([]domain.EntryListResult, error)
}

// ArtifactStore is the subset of db.ArtifactStore needed by StatsService.
type ArtifactStore interface {
	List(ctx context.Context, projectID *string) ([]domain.Artifact, error)
}

// ProjectStore is the subset of db.ProjectStore needed by StatsService.
type ProjectStore interface {
	List(ctx context.Context, includeArchived bool) ([]domain.Project, error)
}

// WorkflowRunStore is the subset of db.WorkflowRunStore needed by StatsService.
type WorkflowRunStore interface {
	GetRunStats(ctx context.Context, workflowID *string) (*db.WorkflowRunStats, error)
}

// Compile-time interface checks.
var _ EntryStore = (EntryStore)(nil)
var _ ArtifactStore = (ArtifactStore)(nil)
var _ ProjectStore = (ProjectStore)(nil)
var _ WorkflowRunStore = (WorkflowRunStore)(nil)
