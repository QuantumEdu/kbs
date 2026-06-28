package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/quantum-6/skillvault/internal/domain"
)

// VaultStats holds aggregated vault statistics.
type VaultStats struct {
	TotalEntries   int
	TotalArtifacts int
	TotalProjects  int
	TotalChars     int // sum of body + summary lengths across all entries
	TodayEntries   int
	TodayArtifacts int
	TodayChars     int // sum of body + summary lengths for today's entries
	TokenEstimate  int // TotalChars / 4 (rough heuristic)
}

// StatsService provides vault-level aggregation.
type StatsService struct {
	entryStore    EntryStore
	artifactStore ArtifactStore
	projectStore  ProjectStore
}

// NewStatsService creates a StatsService.
func NewStatsService(entryStore EntryStore, artifactStore ArtifactStore, projectStore ProjectStore) *StatsService {
	return &StatsService{
		entryStore:    entryStore,
		artifactStore: artifactStore,
		projectStore:  projectStore,
	}
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

// Compile-time interface checks.
var _ EntryStore = (EntryStore)(nil)
var _ ArtifactStore = (ArtifactStore)(nil)
var _ ProjectStore = (ProjectStore)(nil)
