package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/quantum-6/skillvault/internal/db"
	"github.com/quantum-6/skillvault/internal/domain"
)

type ContextInput struct {
	Mode            string
	Project         string
	Query           string
	Include         []string
	ExcludeArchived bool
	MaxChars        int
}

type ContextPack struct {
	Header   string
	Sections []ContextSection
	Raw      string
}

type ContextSection struct {
	Title   string
	Content string
}

type ContextService struct {
	entryStore   db.EntryStore
	projectStore db.ProjectStore
	seriesStore  db.SeriesStore
	workflowStore db.WorkflowStore
	artifactStore db.ArtifactStore
	entryService *EntryService
}

func NewContextService(
	entryStore db.EntryStore,
	projectStore db.ProjectStore,
	seriesStore db.SeriesStore,
	workflowStore db.WorkflowStore,
	artifactStore db.ArtifactStore,
	entryService *EntryService,
) *ContextService {
	return &ContextService{
		entryStore:    entryStore,
		projectStore:  projectStore,
		seriesStore:   seriesStore,
		workflowStore: workflowStore,
		artifactStore: artifactStore,
		entryService:  entryService,
	}
}

func (s *ContextService) GetContext(ctx context.Context, input ContextInput) (*ContextPack, error) {
	if input.Mode == "" {
		input.Mode = "project"
	}
	if input.ExcludeArchived && input.Include == nil {
		input.Include = []string{"profile", "decisions", "workflows", "recent_sessions"}
	}
	maxChars := input.MaxChars
	if maxChars <= 0 {
		maxChars = 12000
	}

	pack := &ContextPack{}
	pack.Header = "# CONTEXT PACK\n\n## Scope\n"

	if input.Project != "" {
		proj, err := s.projectStore.Get(ctx, input.Project)
		if err == nil {
			pack.Header += fmt.Sprintf("Project: %s\nMode: %s\n\n## Project State\n- %s: %s\n",
				proj.Name, input.Mode, proj.Name, proj.Description)
		}
	} else {
		pack.Header += fmt.Sprintf("Mode: %s\n", input.Mode)
	}

	var sections []ContextSection
	totalChars := len(pack.Header)

	addSection := func(title, content string) {
		block := fmt.Sprintf("\n## %s\n%s\n", title, content)
		if totalChars+len(block) > maxChars {
			remaining := maxChars - totalChars
			if remaining > 50 {
				sections = append(sections, ContextSection{
					Title:   title,
					Content: content[:min(len(content), remaining-50)] + "\n[truncated]",
				})
			}
			return
		}
		sections = append(sections, ContextSection{Title: title, Content: content})
		totalChars += len(block)
	}

	includeSet := make(map[string]bool)
	for _, i := range input.Include {
		includeSet[strings.ToLower(i)] = true
	}
	if len(includeSet) == 0 {
		includeSet["profile"] = true
		includeSet["decisions"] = true
		includeSet["workflows"] = true
		includeSet["recent_sessions"] = true
	}

	if includeSet["profile"] || includeSet["user"] {
		profileContent := s.collectProfile(ctx)
		if profileContent != "" {
			addSection("User Preferences", profileContent)
		}
	}

	if input.Project != "" && (includeSet["decisions"] || includeSet["project_state"]) {
		decisions := s.collectDecisions(ctx, input.Project, input.ExcludeArchived)
		if decisions != "" {
			addSection("Active Decisions", decisions)
		}
	}

	if includeSet["workflows"] {
		workflows := s.collectWorkflows(ctx, input.ExcludeArchived)
		if workflows != "" {
			addSection("Relevant Workflows", workflows)
		}
	}

	if includeSet["recent_sessions"] {
		sessions := s.collectRecentSessions(ctx, input.Project, input.ExcludeArchived)
		if sessions != "" {
			addSection("Recent Sessions", sessions)
		}
	}

	if includeSet["artifact_summaries"] {
		artifacts := s.collectArtifactSummaries(ctx, input.Project)
		if artifacts != "" {
			addSection("Artifact Summaries", artifacts)
		}
	}

	if includeSet["references"] {
		refs := s.collectReferences(ctx, input.Project, input.ExcludeArchived)
		if refs != "" {
			addSection("References", refs)
		}
	}

	if !input.ExcludeArchived {
		archived := s.collectArchived(ctx, input.Project)
		if archived != "" {
			addSection("Archived", archived)
		}
	}

	var b strings.Builder
	b.WriteString(pack.Header)
	for _, sec := range sections {
		b.WriteString("\n")
		b.WriteString("## ")
		b.WriteString(sec.Title)
		b.WriteString("\n")
		b.WriteString(sec.Content)
		b.WriteString("\n")
	}

	pack.Sections = sections
	pack.Raw = b.String()
	if len(pack.Raw) > maxChars {
		pack.Raw = pack.Raw[:maxChars] + "\n[truncated]"
	}

	return pack, nil
}

func (s *ContextService) collectProfile(ctx context.Context) string {
	filter := domain.EntryFilter{
		Type:            strPtr("feedback"),
		IncludeArchived: false,
	}
	entries, err := s.entryStore.List(ctx, filter)
	if err != nil {
		return ""
	}
	var b strings.Builder
	for _, e := range entries {
		b.WriteString(fmt.Sprintf("- %s", e.Entry.Summary))
		if e.Entry.BodyOptional != "" {
			b.WriteString(": ")
			b.WriteString(e.Entry.BodyOptional)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func (s *ContextService) collectDecisions(ctx context.Context, projectID string, excludeArchived bool) string {
	filter := domain.EntryFilter{
		Type:            strPtr("decision"),
		ProjectID:       &projectID,
		IncludeArchived: !excludeArchived,
	}
	entries, err := s.entryStore.List(ctx, filter)
	if err != nil {
		return ""
	}
	var b strings.Builder
	for _, e := range entries {
		b.WriteString(fmt.Sprintf("- %s", e.Entry.Title))
		if e.Entry.Summary != "" {
			b.WriteString(": ")
			b.WriteString(e.Entry.Summary)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func (s *ContextService) collectWorkflows(ctx context.Context, excludeArchived bool) string {
	workflows, err := s.workflowStore.List(ctx, !excludeArchived)
	if err != nil {
		return ""
	}
	var b strings.Builder
	for _, wf := range workflows {
		b.WriteString(fmt.Sprintf("- %s", wf.Name))
		if wf.Description != "" {
			b.WriteString(": ")
			b.WriteString(wf.Description)
		}
		steps, err := s.workflowStore.GetSteps(ctx, wf.ID)
		if err == nil && len(steps) > 0 {
			b.WriteString("\n")
			for _, step := range steps {
				b.WriteString(fmt.Sprintf("  %d. %s", step.OrderIndex, step.Title))
				if step.Instruction != "" {
					b.WriteString(" — ")
					b.WriteString(step.Instruction)
				}
				b.WriteString("\n")
			}
		} else {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (s *ContextService) collectRecentSessions(ctx context.Context, projectID string, excludeArchived bool) string {
	filter := domain.EntryFilter{
		Type:            strPtr("session"),
		IncludeArchived: !excludeArchived,
	}
	if projectID != "" {
		filter.ProjectID = &projectID
	}
	entries, err := s.entryStore.List(ctx, filter)
	if err != nil {
		return ""
	}
	var b strings.Builder
	limit := 5
	if len(entries) > limit {
		entries = entries[:limit]
	}
	for _, e := range entries {
		b.WriteString(fmt.Sprintf("- %s", e.Entry.Title))
		if e.Entry.Summary != "" {
			b.WriteString(": ")
			b.WriteString(e.Entry.Summary)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func (s *ContextService) collectArtifactSummaries(ctx context.Context, projectID string) string {
	var projID *string
	if projectID != "" {
		projID = &projectID
	}
	artifacts, err := s.artifactStore.List(ctx, projID)
	if err != nil {
		return ""
	}
	var b strings.Builder
	for _, a := range artifacts {
		b.WriteString(fmt.Sprintf("- %s (%s)", a.Title, a.Type))
		if a.Summary != "" {
			b.WriteString(": ")
			b.WriteString(a.Summary)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func (s *ContextService) collectReferences(ctx context.Context, projectID string, excludeArchived bool) string {
	filter := domain.EntryFilter{
		Type:            strPtr("reference"),
		IncludeArchived: !excludeArchived,
	}
	if projectID != "" {
		filter.ProjectID = &projectID
	}
	entries, err := s.entryStore.List(ctx, filter)
	if err != nil {
		return ""
	}
	var b strings.Builder
	for _, e := range entries {
		b.WriteString(fmt.Sprintf("- %s", e.Entry.Title))
		if e.Entry.Summary != "" {
			b.WriteString(": ")
			b.WriteString(e.Entry.Summary)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func (s *ContextService) collectArchived(ctx context.Context, projectID string) string {
	filter := domain.EntryFilter{
		IncludeArchived: true,
	}
	if projectID != "" {
		filter.ProjectID = &projectID
	}
	entries, err := s.entryStore.List(ctx, filter)
	if err != nil {
		return ""
	}
	var b strings.Builder
	count := 0
	for _, e := range entries {
		if e.Entry.Status == domain.StatusArchived {
			count++
			b.WriteString(fmt.Sprintf("- %s (%s)\n", e.Entry.Title, e.Entry.Type))
			if count >= 10 {
				break
			}
		}
	}
	return b.String()
}

func strPtr(s string) *string {
	return &s
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
