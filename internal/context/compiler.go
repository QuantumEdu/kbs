package context

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/quantum-6/skillvault/internal/domain"
)

type Input struct {
	Mode            string
	Project         string
	Query           string
	Include         []string
	ExcludeArchived bool
	MaxChars        int
}

type EntryStore interface {
	List(ctx context.Context, filter domain.EntryFilter) ([]domain.EntryListResult, error)
	Search(ctx context.Context, q domain.SearchQuery) ([]domain.EntrySearchResult, error)
}

type ProjectStore interface {
	Get(ctx context.Context, id string) (domain.Project, error)
	List(ctx context.Context, includeArchived bool) ([]domain.Project, error)
}

type WorkflowStore interface {
	List(ctx context.Context, includeArchived bool) ([]domain.Workflow, error)
	GetSteps(ctx context.Context, workflowID string) ([]domain.WorkflowStep, error)
}

type ArtifactStore interface {
	List(ctx context.Context, projectID *string) ([]domain.Artifact, error)
}

type HermesCompiler struct {
	entries   EntryStore
	projects  ProjectStore
	workflows WorkflowStore
	artifacts ArtifactStore
}

func NewHermesCompiler(entries EntryStore, projects ProjectStore, workflows WorkflowStore, artifacts ArtifactStore) *HermesCompiler {
	return &HermesCompiler{
		entries:   entries,
		projects:  projects,
		workflows: workflows,
		artifacts: artifacts,
	}
}

type section struct {
	title    string
	content  string
	priority int
}

func (c *HermesCompiler) Compile(ctx context.Context, input Input) (string, error) {
	maxChars := input.MaxChars
	if maxChars <= 0 {
		maxChars = 12000
	}

	includeSet := buildIncludeSet(input.Include)

	scope := fmt.Sprintf("Mode: %s", input.Mode)
	if input.Project != "" {
		scope = fmt.Sprintf("Project: %s\nMode: %s", input.Project, input.Mode)
	}
	if input.Query != "" {
		scope += "\nQuery: " + input.Query
	}

	var sections []section

	switch input.Mode {
	case "profile":
		sections = c.buildProfile(ctx, input)
	case "project":
		sections = c.buildProject(ctx, input)
	case "workflow":
		sections = c.buildWorkflowSections(ctx, input)
	case "skill":
		sections = c.buildSkillSections(ctx, input)
	case "planning":
		sections = c.buildPlanning(ctx, input)
	case "session_recall":
		sections = c.buildSessionRecall(ctx, input)
	case "full_brief":
		sections = c.buildFullBrief(ctx, input, includeSet)
	}

	if !input.ExcludeArchived && hasInclude(includeSet, "archived") {
		sec := c.buildArchived(ctx, input)
		if sec.content != "" {
			sections = append(sections, sec)
		}
	}

	if input.ExcludeArchived {
		var filtered []section
		for _, s := range sections {
			if s.priority > 0 {
				filtered = append(filtered, s)
			}
		}
		sections = filtered
	}

	suggestion := modeSuggestion(input.Mode)
	suggestion = strings.TrimSpace(suggestion)

	var b strings.Builder
	b.WriteString("# CONTEXT PACK\n\n")
	b.WriteString("## Scope\n")
	b.WriteString(scope)
	b.WriteString("\n")

	headerLen := b.Len()
	suggestionBlock := ""
	if suggestion != "" {
		suggestionBlock = "\n## Suggested Next Action\n" + suggestion + "\n"
	}

	sort.Slice(sections, func(i, j int) bool {
		return sections[i].priority < sections[j].priority
	})

	total := headerLen
	for _, s := range sections {
		total += sectionLen(s) + 2
	}
	total += len(suggestionBlock)

	if total > maxChars {
		removed := 0
		for total > maxChars && len(sections) > 0 {
			last := sections[len(sections)-1]
			total -= sectionLen(last) + 2
			sections = sections[:len(sections)-1]
			removed++
		}
		if removed > 0 {
			sections = append(sections, section{
				title:    "",
				content:  fmt.Sprintf("[%d section(s) omitted due to context limit]", removed),
				priority: 99,
			})
		}
	}

	for _, s := range sections {
		if s.title == "" {
			b.WriteString("\n" + s.content + "\n")
		} else {
			b.WriteString(fmt.Sprintf("\n## %s\n%s\n", s.title, s.content))
		}
	}

	if suggestion != "" {
		b.WriteString("\n## Suggested Next Action\n" + suggestion + "\n")
	}

	result := b.String()
	if len(result) > maxChars {
		result = result[:maxChars] + "\n[truncated]"
	}

	return result, nil
}

func sectionLen(s section) int {
	if s.title == "" {
		return len(s.content)
	}
	return 4 + len(s.title) + len(s.content)
}

func buildIncludeSet(include []string) map[string]bool {
	set := make(map[string]bool)
	for _, v := range include {
		set[strings.ToLower(v)] = true
	}
	return set
}

func hasInclude(set map[string]bool, key string) bool {
	if len(set) == 0 {
		return true
	}
	return set[key]
}

func (c *HermesCompiler) buildProfile(ctx context.Context, input Input) []section {
	var sections []section

	prefs := c.collectEntries(ctx, "", domain.EntryTypeUser, input.ExcludeArchived)
	fb := c.collectEntries(ctx, "", domain.EntryTypeFeedback, input.ExcludeArchived)

	var prefsList, fbList []string
	for _, e := range prefs {
		line := "- " + e.Entry.Title
		if e.Entry.Summary != "" {
			line += ": " + e.Entry.Summary
		}
		prefsList = append(prefsList, line)
	}
	for _, e := range fb {
		line := "- " + e.Entry.Title
		if e.Entry.Summary != "" {
			line += ": " + e.Entry.Summary
		}
		fbList = append(fbList, line)
	}

	if len(prefsList) > 0 {
		sections = append(sections, section{
			title:    "User Preferences",
			content:  strings.Join(prefsList, "\n"),
			priority: 1,
		})
	}
	if len(fbList) > 0 {
		sections = append(sections, section{
			title:    "User Preferences",
			content:  strings.Join(fbList, "\n"),
			priority: 1,
		})
	}

	return sections
}

func (c *HermesCompiler) buildProject(ctx context.Context, input Input) []section {
	var sections []section

	ps := c.collectProjectState(ctx, input.Project, input.ExcludeArchived)
	if ps != "" {
		sections = append(sections, section{
			title:    "Project State",
			content:  ps,
			priority: 2,
		})
	}

	if hasInclude(buildIncludeSet(input.Include), "decisions") {
		dec := c.collectDecisions(ctx, input.Project, input.ExcludeArchived)
		if dec != "" {
			sections = append(sections, section{
				title:    "Active Decisions",
				content:  dec,
				priority: 3,
			})
		}
	}

	if hasInclude(buildIncludeSet(input.Include), "recent_sessions") {
		sess := c.collectRecentSessions(ctx, input.Project, input.ExcludeArchived)
		if sess != "" {
			sections = append(sections, section{
				title:    "Recent Sessions",
				content:  sess,
				priority: 5,
			})
		}
	}

	return sections
}

func (c *HermesCompiler) buildWorkflowSections(ctx context.Context, input Input) []section {
	wf := c.collectWorkflows(ctx, input.ExcludeArchived)
	if wf == "" {
		return nil
	}
	return []section{{
		title:    "Relevant Workflows",
		content:  wf,
		priority: 4,
	}}
}

func (c *HermesCompiler) buildSkillSections(ctx context.Context, input Input) []section {
	skillContent := c.collectSkills(ctx, input.Query, input.ExcludeArchived)
	if skillContent == "" {
		return nil
	}
	return []section{{
		title:    "Skills",
		content:  skillContent,
		priority: 4,
	}}
}

func (c *HermesCompiler) buildPlanning(ctx context.Context, input Input) []section {
	var sections []section

	ps := c.collectProjectState(ctx, input.Project, input.ExcludeArchived)
	if ps != "" {
		sections = append(sections, section{
			title:    "Project State",
			content:  ps,
			priority: 2,
		})
	}

	if hasInclude(buildIncludeSet(input.Include), "decisions") {
		dec := c.collectDecisions(ctx, input.Project, input.ExcludeArchived)
		if dec != "" {
			sections = append(sections, section{
				title:    "Active Decisions",
				content:  dec,
				priority: 3,
			})
		}
	}

	wf := c.collectWorkflows(ctx, input.ExcludeArchived)
	if wf != "" {
		sections = append(sections, section{
			title:    "Relevant Workflows",
			content:  wf,
			priority: 4,
		})
	}

	if hasInclude(buildIncludeSet(input.Include), "recent_sessions") {
		sess := c.collectRecentSessions(ctx, input.Project, input.ExcludeArchived)
		if sess != "" {
			sections = append(sections, section{
				title:    "Recent Sessions",
				content:  sess,
				priority: 5,
			})
		}
	}

	return sections
}

func (c *HermesCompiler) buildSessionRecall(ctx context.Context, input Input) []section {
	var sections []section

	sess := c.collectRecentSessions(ctx, input.Project, input.ExcludeArchived)
	if sess != "" {
		sections = append(sections, section{
			title:    "Recent Sessions",
			content:  sess,
			priority: 5,
		})
	}

	if hasInclude(buildIncludeSet(input.Include), "decisions") {
		dec := c.collectDecisions(ctx, input.Project, input.ExcludeArchived)
		if dec != "" {
			sections = append(sections, section{
				title:    "Active Decisions",
				content:  dec,
				priority: 3,
			})
		}
	}

	return sections
}

func (c *HermesCompiler) buildFullBrief(ctx context.Context, input Input, includeSet map[string]bool) []section {
	var sections []section

	prefs := c.collectEntries(ctx, "", domain.EntryTypeUser, input.ExcludeArchived)
	fb := c.collectEntries(ctx, "", domain.EntryTypeFeedback, input.ExcludeArchived)
	var prefsLines []string
	for _, e := range prefs {
		prefsLines = append(prefsLines, "- "+e.Entry.Title)
	}
	for _, e := range fb {
		prefsLines = append(prefsLines, "- "+e.Entry.Title)
	}
	if len(prefsLines) > 0 {
		sections = append(sections, section{
			title:    "User Preferences",
			content:  strings.Join(prefsLines, "\n"),
			priority: 1,
		})
	}

	ps := c.collectProjectState(ctx, input.Project, input.ExcludeArchived)
	if ps != "" {
		sections = append(sections, section{
			title:    "Project State",
			content:  ps,
			priority: 2,
		})
	}

	if hasInclude(includeSet, "decisions") {
		dec := c.collectDecisions(ctx, input.Project, input.ExcludeArchived)
		if dec != "" {
			sections = append(sections, section{
				title:    "Active Decisions",
				content:  dec,
				priority: 3,
			})
		}
	}

	wf := c.collectWorkflows(ctx, input.ExcludeArchived)
	if wf != "" {
		sections = append(sections, section{
			title:    "Relevant Workflows",
			content:  wf,
			priority: 4,
		})
	}

	if hasInclude(includeSet, "recent_sessions") {
		sess := c.collectRecentSessions(ctx, input.Project, input.ExcludeArchived)
		if sess != "" {
			sections = append(sections, section{
				title:    "Recent Sessions",
				content:  sess,
				priority: 5,
			})
		}
	}

	if hasInclude(includeSet, "artifact_summaries") {
		arts := c.collectArtifactSummaries(ctx, input.Project)
		if arts != "" {
			sections = append(sections, section{
				title:    "Artifact Summaries",
				content:  arts,
				priority: 6,
			})
		}
	}

	if hasInclude(includeSet, "references") {
		refs := c.collectReferences(ctx, input.Project, input.ExcludeArchived)
		if refs != "" {
			sections = append(sections, section{
				title:    "References",
				content:  refs,
				priority: 7,
			})
		}
	}

	return sections
}

func modeSuggestion(mode string) string {
	switch mode {
	case "profile":
		return "Review and update user preferences and feedback before starting work."
	case "project":
		return "Review project state and active decisions. Check recent sessions for continuity."
	case "workflow":
		return "Follow the workflow steps above in order. Mark each step complete before proceeding."
	case "skill":
		return "Load the matching skill and apply it to the current task."
	case "planning":
		return "Use project state, decisions, and workflows to form an implementation plan."
	case "session_recall":
		return "Review recent sessions and pending decisions before continuing."
	case "full_brief":
		return "Use the full context above to guide the next interaction."
	default:
		return ""
	}
}

func (c *HermesCompiler) collectEntries(ctx context.Context, projectID string, entryType domain.EntryType, excludeArchived bool) []domain.EntryListResult {
	filter := domain.EntryFilter{
		Type:            strPtr(string(entryType)),
		IncludeArchived: !excludeArchived,
	}
	if projectID != "" {
		filter.ProjectID = &projectID
	}
	entries, err := c.entries.List(ctx, filter)
	if err != nil {
		return nil
	}
	return entries
}

func (c *HermesCompiler) collectProjectState(ctx context.Context, projectID string, excludeArchived bool) string {
	if projectID == "" {
		projs, err := c.projects.List(ctx, !excludeArchived)
		if err != nil || len(projs) == 0 {
			return ""
		}
		var lines []string
		for _, p := range projs {
			lines = append(lines, fmt.Sprintf("- %s: %s", p.Name, p.Description))
		}
		return strings.Join(lines, "\n")
	}

	proj, err := c.projects.Get(ctx, projectID)
	if err != nil {
		return ""
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("- %s: %s\n", proj.Name, proj.Description))

	stateEntries := c.collectEntries(ctx, projectID, domain.EntryTypeProjectState, excludeArchived)
	for _, e := range stateEntries {
		b.WriteString(fmt.Sprintf("- %s: %s\n", e.Entry.Title, e.Entry.Summary))
	}

	return strings.TrimSpace(b.String())
}

func (c *HermesCompiler) collectDecisions(ctx context.Context, projectID string, excludeArchived bool) string {
	var filter domain.EntryFilter
	if projectID != "" {
		filter.ProjectID = &projectID
		filter.Type = strPtr(string(domain.EntryTypeDecision))
		filter.IncludeArchived = !excludeArchived
		entries, err := c.entries.List(ctx, filter)
		if err != nil {
			return ""
		}
		var lines []string
		for _, e := range entries {
			line := "- " + e.Entry.Title
			if e.Entry.Summary != "" {
				line += ": " + e.Entry.Summary
			}
			lines = append(lines, line)
		}
		return strings.Join(lines, "\n")
	}

	entries, err := c.entries.List(ctx, domain.EntryFilter{
		Type:            strPtr(string(domain.EntryTypeDecision)),
		IncludeArchived: !excludeArchived,
	})
	if err != nil {
		return ""
	}
	var lines []string
	for _, e := range entries {
		line := "- " + e.Entry.Title
		if e.Entry.Summary != "" {
			line += ": " + e.Entry.Summary
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func (c *HermesCompiler) collectWorkflows(ctx context.Context, excludeArchived bool) string {
	workflows, err := c.workflows.List(ctx, !excludeArchived)
	if err != nil {
		return ""
	}
	var b strings.Builder
	for _, wf := range workflows {
		b.WriteString(fmt.Sprintf("- %s", wf.Name))
		if wf.Description != "" {
			b.WriteString(": " + wf.Description)
		}
		steps, err := c.workflows.GetSteps(ctx, wf.ID)
		if err == nil && len(steps) > 0 {
			b.WriteString("\n")
			for _, step := range steps {
				b.WriteString(fmt.Sprintf("  %d. %s", step.OrderIndex, step.Title))
				if step.Instruction != "" {
					b.WriteString(" — " + step.Instruction)
				}
				b.WriteString("\n")
			}
		} else {
			b.WriteString("\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func (c *HermesCompiler) collectRecentSessions(ctx context.Context, projectID string, excludeArchived bool) string {
	filter := domain.EntryFilter{
		Type:            strPtr(string(domain.EntryTypeSession)),
		IncludeArchived: !excludeArchived,
	}
	if projectID != "" {
		filter.ProjectID = &projectID
	}
	entries, err := c.entries.List(ctx, filter)
	if err != nil {
		return ""
	}
	if len(entries) > 5 {
		entries = entries[:5]
	}
	var lines []string
	for _, e := range entries {
		line := "- " + e.Entry.Title
		if e.Entry.Summary != "" {
			line += ": " + e.Entry.Summary
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func (c *HermesCompiler) collectSkills(ctx context.Context, query string, excludeArchived bool) string {
	if query != "" {
		results, err := c.entries.Search(ctx, domain.SearchQuery{
			Query:           query,
			Type:            strPtr(string(domain.EntryTypeSkill)),
			IncludeArchived: !excludeArchived,
			Limit:           10,
		})
		if err != nil {
			return ""
		}
		var lines []string
		for _, r := range results {
			line := "- " + r.Entry.Title
			if r.Entry.Summary != "" {
				line += ": " + r.Entry.Summary
			}
			lines = append(lines, line)
		}
		return strings.Join(lines, "\n")
	}

	entries, err := c.entries.List(ctx, domain.EntryFilter{
		Type:            strPtr(string(domain.EntryTypeSkill)),
		IncludeArchived: !excludeArchived,
	})
	if err != nil {
		return ""
	}
	var lines []string
	for _, e := range entries {
		line := "- " + e.Entry.Title
		if e.Entry.Summary != "" {
			line += ": " + e.Entry.Summary
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func (c *HermesCompiler) collectArtifactSummaries(ctx context.Context, projectID string) string {
	var projID *string
	if projectID != "" {
		projID = &projectID
	}
	artifacts, err := c.artifacts.List(ctx, projID)
	if err != nil {
		return ""
	}
	var lines []string
	for _, a := range artifacts {
		line := "- " + a.Title + " (" + string(a.Type) + ")"
		if a.Summary != "" {
			line += ": " + a.Summary
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func (c *HermesCompiler) collectReferences(ctx context.Context, projectID string, excludeArchived bool) string {
	filter := domain.EntryFilter{
		Type:            strPtr(string(domain.EntryTypeReference)),
		IncludeArchived: !excludeArchived,
	}
	if projectID != "" {
		filter.ProjectID = &projectID
	}
	entries, err := c.entries.List(ctx, filter)
	if err != nil {
		return ""
	}
	var lines []string
	for _, e := range entries {
		line := "- " + e.Entry.Title
		if e.Entry.Summary != "" {
			line += ": " + e.Entry.Summary
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func (c *HermesCompiler) buildArchived(ctx context.Context, input Input) section {
	filter := domain.EntryFilter{
		IncludeArchived: true,
	}
	if input.Project != "" {
		filter.ProjectID = &input.Project
	}
	entries, err := c.entries.List(ctx, filter)
	if err != nil {
		return section{title: "Archived", content: "", priority: 8}
	}
	var lines []string
	for _, e := range entries {
		if e.Entry.Status == domain.StatusArchived {
			lines = append(lines, fmt.Sprintf("- %s (%s)", e.Entry.Title, e.Entry.Type))
		}
		if len(lines) >= 10 {
			break
		}
	}
	if len(lines) == 0 {
		return section{title: "Archived", content: "", priority: 8}
	}
	return section{
		title:    "Archived",
		content:  strings.Join(lines, "\n"),
		priority: 8,
	}
}

func strPtr(s string) *string {
	return &s
}
