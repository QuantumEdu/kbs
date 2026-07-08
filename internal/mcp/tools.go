package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/quantum-6/skillvault/internal/app"
	"github.com/quantum-6/skillvault/internal/domain"
)

// ToolHandler is called when a tool is invoked (legacy/testing).
type ToolHandler func(ctx context.Context, toolName string, args map[string]interface{}) (*ToolCallResult, error)

// ToolRegistry manages MCP tools and dispatches calls.
type ToolRegistry struct {
	tools   []Tool
	handler ToolHandler

	entrySvc       *app.EntryService
	entryRefSvc    *app.EntryRefService
	compareSvc     *app.VectorService
	artifactSvc    *app.ArtifactService
	contextSvc     *app.ContextService
	seriesSvc      *app.SeriesService
	workflowSvc    *app.WorkflowService
	sessionSvc     *app.SessionService
	projectSvc     *app.ProjectService
	saveResultSvc  *app.SavePromptResultService
}

// NewToolRegistry creates a registry with a generic handler (for testing).
func NewToolRegistry(handler ToolHandler) *ToolRegistry {
	reg := &ToolRegistry{handler: handler}
	reg.registerV2Tools()
	return reg
}

// WithEntryRefService sets the entry ref service for graph operations.
func (r *ToolRegistry) WithEntryRefService(svc *app.EntryRefService) *ToolRegistry {
	r.entryRefSvc = svc
	return r
}

// WithCompareService sets the vector compare service for entry comparison.
func (r *ToolRegistry) WithCompareService(svc *app.VectorService) *ToolRegistry {
	r.compareSvc = svc
	return r
}

// WithSaveResultService sets the save-result service.
func (r *ToolRegistry) WithSaveResultService(svc *app.SavePromptResultService) *ToolRegistry {
	r.saveResultSvc = svc
	return r
}

// NewServiceToolRegistry creates a registry backed by app services.
func NewServiceToolRegistry(
	entrySvc *app.EntryService,
	artifactSvc *app.ArtifactService,
	contextSvc *app.ContextService,
	seriesSvc *app.SeriesService,
	workflowSvc *app.WorkflowService,
	sessionSvc *app.SessionService,
	projectSvc *app.ProjectService,
) *ToolRegistry {
	reg := &ToolRegistry{
		entrySvc:    entrySvc,
		artifactSvc: artifactSvc,
		contextSvc:  contextSvc,
		seriesSvc:   seriesSvc,
		workflowSvc: workflowSvc,
		sessionSvc:  sessionSvc,
		projectSvc:  projectSvc,
	}
	reg.registerV2Tools()
	return reg
}

func (r *ToolRegistry) registerV2Tools() {
	r.tools = []Tool{
		{Name: "save_entry", Description: "Save a vault entry (prompt, skill, decision, etc.)", InputSchema: schemaObj(map[string]interface{}{
			"title":   map[string]interface{}{"type": "string", "description": "Entry title (required)"},
			"type":    map[string]interface{}{"type": "string", "description": "Entry type: prompt|skill|reference|user|feedback|project_state|session|decision|artifact_summary|handoff|routing"},
			"summary": map[string]interface{}{"type": "string", "description": "Short summary"},
			"body":    map[string]interface{}{"type": "string", "description": "Optional body content"},
			"project": map[string]interface{}{"type": "string", "description": "Project name or ID"},
			"tags":    map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
			"status":  map[string]interface{}{"type": "string", "description": "draft|active|archived|deprecated|canonical"},
			"purpose": map[string]interface{}{"type": "string", "description": "Entry purpose: WORK|KNOWLEDGE|LEARNING|RELATIONSHIP|STATE"},
		})},
		{Name: "search_entries", Description: "Search entries with FTS5 and filters", InputSchema: schemaObj(map[string]interface{}{
			"query":            map[string]interface{}{"type": "string", "description": "Search query"},
			"type":             map[string]interface{}{"type": "string", "description": "Filter by entry type"},
			"project":          map[string]interface{}{"type": "string", "description": "Filter by project"},
			"tags":             map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
			"purpose":          map[string]interface{}{"type": "string", "description": "Filter by purpose (WORK|KNOWLEDGE|LEARNING|RELATIONSHIP|STATE)"},
			"include_archived": map[string]interface{}{"type": "boolean", "description": "Include archived entries"},
			"limit":            map[string]interface{}{"type": "number", "description": "Max results (default 10)"},
			"vector":           map[string]interface{}{"type": "boolean", "description": "Use vector/cosine similarity search instead of FTS5"},
		})},
		{Name: "get_entry", Description: "Get a vault entry by ID (includes artifact reference if linked)", InputSchema: schemaObj(map[string]interface{}{
			"id": map[string]interface{}{"type": "string", "description": "Entry ID"},
		})},
		{Name: "save_artifact", Description: "Save a long AI output or file-backed artifact", InputSchema: schemaObj(map[string]interface{}{
			"title":     map[string]interface{}{"type": "string", "description": "Artifact title (required)"},
			"type":      map[string]interface{}{"type": "string", "description": "Artifact type: ai_output|pdf_analysis|spec|report|session_output|markdown|json|txt"},
			"content":   map[string]interface{}{"type": "string", "description": "Artifact content (optional if file_path provided)"},
			"file_path": map[string]interface{}{"type": "string", "description": "Path to file (optional if content provided)"},
			"summary":   map[string]interface{}{"type": "string", "description": "Short summary"},
			"project":   map[string]interface{}{"type": "string", "description": "Project name or ID"},
			"tags":      map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
		})},
		{Name: "get_context", Description: "Compile agent-ready context pack from vault data", InputSchema: schemaObj(map[string]interface{}{
			"mode":             map[string]interface{}{"type": "string", "description": "profile|project|workflow|skill|planning|session_recall|full_brief"},
			"project":          map[string]interface{}{"type": "string", "description": "Project name or ID"},
			"query":            map[string]interface{}{"type": "string", "description": "Optional search query"},
			"include":          map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Sections: profile,decisions,workflows,recent_sessions,artifact_summaries,references"},
			"exclude_archived": map[string]interface{}{"type": "boolean", "description": "Exclude archived entries"},
			"max_chars":        map[string]interface{}{"type": "number", "description": "Maximum characters (default 12000)"},
		})},
		{Name: "compose_series", Description: "Return ordered entries in a series", InputSchema: schemaObj(map[string]interface{}{
			"series_id": map[string]interface{}{"type": "string", "description": "Series ID"},
		})},
		{Name: "render_workflow", Description: "Return workflow steps as a checklist", InputSchema: schemaObj(map[string]interface{}{
			"workflow_id": map[string]interface{}{"type": "string", "description": "Workflow ID"},
		})},
		{Name: "session_wrap", Description: "Save a compact session summary with decisions, pending items, and learnings", InputSchema: schemaObj(map[string]interface{}{
			"project":   map[string]interface{}{"type": "string", "description": "Project name or ID"},
			"summary":   map[string]interface{}{"type": "string", "description": "Session summary (required)"},
			"decisions": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
			"pending":   map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
			"learnings": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
			"artifacts": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
		})},
		{Name: "archive_entry", Description: "Soft-delete an entry (set status to archived)", InputSchema: schemaObj(map[string]interface{}{
			"id": map[string]interface{}{"type": "string", "description": "Entry ID"},
		})},
		{Name: "list_projects", Description: "List all projects and their statuses", InputSchema: schemaObj(map[string]interface{}{})},
		{Name: "save_entry_ref", Description: "Create or update a link between two entries (graph edge)", InputSchema: schemaObj(map[string]interface{}{
			"source_id":     map[string]interface{}{"type": "string", "description": "Source entry ID (required)"},
			"target_id":     map[string]interface{}{"type": "string", "description": "Target entry ID (required)"},
			"relation_type": map[string]interface{}{"type": "string", "description": "Relation type: references, supersedes, related_to, part_of, derived_from, implements, uses, extends, handoff_of, generated_from, depends_on"},
			"label":         map[string]interface{}{"type": "string", "description": "Optional label for the link"},
		})},
		{Name: "list_entry_refs", Description: "List graph edges between entries", InputSchema: schemaObj(map[string]interface{}{
			"source_id":        map[string]interface{}{"type": "string", "description": "Filter by source entry ID"},
			"target_id":        map[string]interface{}{"type": "string", "description": "Filter by target entry ID"},
			"relation_type":    map[string]interface{}{"type": "string", "description": "Filter by relation type"},
			"include_archived": map[string]interface{}{"type": "boolean", "description": "Include soft-deleted refs"},
		})},
		{Name: "get_entry_graph", Description: "Traverse entry graph from a starting entry, returning connected nodes and edges", InputSchema: schemaObj(map[string]interface{}{
			"entry_id":  map[string]interface{}{"type": "string", "description": "Starting entry ID (required)"},
			"ref_types": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Filter by relation types"},
			"direction": map[string]interface{}{"type": "string", "description": "outgoing, incoming, or both (default)"},
			"max_depth": map[string]interface{}{"type": "number", "description": "Max traversal depth (default 3, max 10)"},
		})},
		{Name: "search_by_tags", Description: "Search entries by tags with all/any match against the junction table", InputSchema: schemaObj(map[string]interface{}{
			"tags":    map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Tags to match (required)"},
			"match":   map[string]interface{}{"type": "string", "description": "all (intersection) or any (union). Default: all"},
			"type":    map[string]interface{}{"type": "string", "description": "Filter by entry type"},
			"project": map[string]interface{}{"type": "string", "description": "Filter by project"},
			"limit":   map[string]interface{}{"type": "number", "description": "Max results (default 20)"},
		})},
		{Name: "get_context_bundle", Description: "Return structured JSON bundle: project info, entries grouped by type, and artifact refs", InputSchema: schemaObj(map[string]interface{}{
			"project": map[string]interface{}{"type": "string", "description": "Project name or ID"},
		})},
		{Name: "compare_entries", Description: "Compute line-based LCS unified diff between two entries", InputSchema: schemaObj(map[string]interface{}{
			"id1": map[string]interface{}{"type": "string", "description": "First entry ID"},
			"id2": map[string]interface{}{"type": "string", "description": "Second entry ID"},
		})},
		{Name: "save_result", Description: "Save an AI prompt result as a vault entry", InputSchema: schemaObj(map[string]interface{}{
			"name":             map[string]interface{}{"type": "string", "description": "Result name (required)"},
			"content":          map[string]interface{}{"type": "string", "description": "Result content (required)"},
			"type":             map[string]interface{}{"type": "string", "description": "Entry type override"},
			"category":         map[string]interface{}{"type": "string", "description": "Summary/category"},
			"tags":             map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
			"project_id":       map[string]interface{}{"type": "string", "description": "Project ID or name"},
			"source_prompt_id": map[string]interface{}{"type": "string", "description": "Source prompt entry ID"},
			"model":            map[string]interface{}{"type": "string", "description": "Model identifier"},
		})},
	}
}

// List returns all registered tools.
func (r *ToolRegistry) List() []Tool {
	return r.tools
}

// Call dispatches a tool call to the handler or service dispatch.
func (r *ToolRegistry) Call(ctx context.Context, name string, args map[string]interface{}) (*ToolCallResult, error) {
	if r.entrySvc != nil {
		return r.dispatch(ctx, name, args)
	}
	if r.handler == nil {
		return nil, fmt.Errorf("no tool handler registered")
	}
	return r.handler(ctx, name, args)
}

func (r *ToolRegistry) dispatch(ctx context.Context, name string, args map[string]interface{}) (*ToolCallResult, error) {
	switch name {
	case "save_entry":
		return r.handleSaveEntry(ctx, args)
	case "search_entries":
		return r.handleSearchEntries(ctx, args)
	case "get_entry":
		return r.handleGetEntry(ctx, args)
	case "save_artifact":
		return r.handleSaveArtifact(ctx, args)
	case "get_context":
		return r.handleGetContext(ctx, args)
	case "compose_series":
		return r.handleComposeSeries(ctx, args)
	case "render_workflow":
		return r.handleRenderWorkflow(ctx, args)
	case "session_wrap":
		return r.handleSessionWrap(ctx, args)
	case "archive_entry":
		return r.handleArchiveEntry(ctx, args)
	case "list_projects":
		return r.handleListProjects(ctx, args)
	case "save_entry_ref":
		return r.handleSaveEntryRef(ctx, args)
	case "list_entry_refs":
		return r.handleListEntryRefs(ctx, args)
	case "get_entry_graph":
		return r.handleGetEntryGraph(ctx, args)
	case "search_by_tags":
		return r.handleSearchByTags(ctx, args)
	case "get_context_bundle":
		return r.handleGetContextBundle(ctx, args)
	case "compare_entries":
		return r.handleCompareEntries(ctx, args)
	case "save_result":
		return r.handleSaveResult(ctx, args)
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

func (r *ToolRegistry) handleSaveEntry(ctx context.Context, args map[string]interface{}) (*ToolCallResult, error) {
	input := app.SaveEntryInput{
		Title:   strArg(args, "title"),
		Type:    strArg(args, "type"),
		Summary: strArg(args, "summary"),
		Body:    strArg(args, "body"),
		Project: strArg(args, "project"),
		Tags:    parseStrings(args["tags"]),
		Status:  strArg(args, "status"),
		Purpose: strArg(args, "purpose"),
	}
	if input.Type == "" {
		input.Type = "note"
	}

	result, err := r.entrySvc.SaveEntry(ctx, input)
	if err != nil {
		return errResult("Error: " + err.Error()), nil
	}

	proj := "global"
	if result.Entry.Entry.ProjectID != nil {
		proj = *result.Entry.Entry.ProjectID
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Saved: %s\n", result.Entry.Entry.ID))
	b.WriteString(fmt.Sprintf("  Title:   %s\n", result.Entry.Entry.Title))
	b.WriteString(fmt.Sprintf("  Type:    %s\n", result.Entry.Entry.Type))
	b.WriteString(fmt.Sprintf("  Project: %s\n", proj))
	b.WriteString(fmt.Sprintf("  Status:  %s\n", result.Entry.Entry.Status))
	if len(result.Entry.Tags) > 0 {
		tags := make([]string, len(result.Entry.Tags))
		for i, t := range result.Entry.Tags {
			tags[i] = t.Name
		}
		b.WriteString(fmt.Sprintf("  Tags:    %s\n", strings.Join(tags, ", ")))
	}
	return textResult(b.String()), nil
}

func (r *ToolRegistry) handleSearchEntries(ctx context.Context, args map[string]interface{}) (*ToolCallResult, error) {
	query := strArg(args, "query")
	project := strArg(args, "project")
	typ := strArg(args, "type")
	tags := parseStrings(args["tags"])
	includeArchived := boolArg(args, "include_archived")
	limit := intArg(args, "limit")
	if limit <= 0 {
		limit = 10
	}
	useVector := boolArg(args, "vector")

	// Vector search path — delegate to VectorService.
	if useVector {
		if r.compareSvc == nil {
			return errResult("Error: vector search not available"), nil
		}
		if query == "" {
			return errResult("Error: query is required for vector search"), nil
		}
		results, err := r.compareSvc.SearchVectors(ctx, query, limit)
		if err != nil {
			return errResult("Error: " + err.Error()), nil
		}
		if len(results) == 0 {
			return textResult("No results found."), nil
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("Found %d result(s) (vector):\n", len(results)))
		for _, r := range results {
			proj := "global"
			if r.Entry.ProjectID != nil {
				proj = *r.Entry.ProjectID
			}
			b.WriteString(fmt.Sprintf("\n  [%s] %s\n", r.Entry.ID, r.Entry.Title))
			b.WriteString(fmt.Sprintf("    Type:    %s\n", r.Entry.Type))
			b.WriteString(fmt.Sprintf("    Summary: %s\n", r.Entry.Summary))
			b.WriteString(fmt.Sprintf("    Project: %s\n", proj))
			b.WriteString(fmt.Sprintf("    Status:  %s\n", r.Entry.Status))
			if len(r.Tags) > 0 {
				tagNames := make([]string, len(r.Tags))
				for i, t := range r.Tags {
					tagNames[i] = t.Name
				}
				b.WriteString(fmt.Sprintf("    Tags:    %s\n", strings.Join(tagNames, ", ")))
			}
		}
		return textResult(b.String()), nil
	}

	// FTS5 search path (existing behavior).
	var projectID, typePtr *string
	if project != "" {
		proj, err := r.projectSvc.GetProject(ctx, project)
		if err == nil {
			projectID = &proj.ID
		} else {
			projectID = &project
		}
	}
	if typ != "" {
		typePtr = &typ
	}

	purpose := strArg(args, "purpose")
	var purposePtr *string
	if purpose != "" {
		purposePtr = &purpose
	}

	results, err := r.entrySvc.SearchEntries(ctx, query, domain.SearchQuery{
		ProjectID:       projectID,
		Type:            typePtr,
		Purpose:         purposePtr,
		Tags:            tags,
		IncludeArchived: includeArchived,
		Limit:           limit,
	})
	if err != nil {
		return errResult("Error: " + err.Error()), nil
	}

	if len(results) == 0 {
		return textResult("No results found."), nil
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Found %d result(s):\n", len(results)))
	for _, r := range results {
		proj := "global"
		if r.Entry.ProjectID != nil {
			proj = *r.Entry.ProjectID
		}
		b.WriteString(fmt.Sprintf("\n  [%s] %s\n", r.Entry.ID, r.Entry.Title))
		b.WriteString(fmt.Sprintf("    Type:    %s\n", r.Entry.Type))
		b.WriteString(fmt.Sprintf("    Summary: %s\n", r.Entry.Summary))
		b.WriteString(fmt.Sprintf("    Project: %s\n", proj))
		b.WriteString(fmt.Sprintf("    Status:  %s\n", r.Entry.Status))
		if len(r.Tags) > 0 {
			tagNames := make([]string, len(r.Tags))
			for i, t := range r.Tags {
				tagNames[i] = t.Name
			}
			b.WriteString(fmt.Sprintf("    Tags:    %s\n", strings.Join(tagNames, ", ")))
		}
	}
	return textResult(b.String()), nil
}

func (r *ToolRegistry) handleGetEntry(ctx context.Context, args map[string]interface{}) (*ToolCallResult, error) {
	id := strArg(args, "id")
	if id == "" {
		return errResult("Error: id is required"), nil
	}

	result, err := r.entrySvc.GetEntry(ctx, id)
	if err != nil {
		return errResult("Error: " + err.Error()), nil
	}

	proj := "global"
	if result.Entry.Entry.ProjectID != nil {
		proj = *result.Entry.Entry.ProjectID
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("ID:      %s\n", result.Entry.Entry.ID))
	b.WriteString(fmt.Sprintf("Title:   %s\n", result.Entry.Entry.Title))
	b.WriteString(fmt.Sprintf("Type:    %s\n", result.Entry.Entry.Type))
	b.WriteString(fmt.Sprintf("Summary: %s\n", result.Entry.Entry.Summary))
	b.WriteString(fmt.Sprintf("Project: %s\n", proj))
	b.WriteString(fmt.Sprintf("Status:  %s\n", result.Entry.Entry.Status))
	if len(result.Entry.Tags) > 0 {
		tags := make([]string, len(result.Entry.Tags))
		for i, t := range result.Entry.Tags {
			tags[i] = t.Name
		}
		b.WriteString(fmt.Sprintf("Tags:    %s\n", strings.Join(tags, ", ")))
	}
	if result.Entry.Entry.BodyOptional != "" {
		b.WriteString(fmt.Sprintf("\nBody:\n%s\n", result.Entry.Entry.BodyOptional))
	}
	if result.Artifact != nil {
		b.WriteString(fmt.Sprintf("\nArtifact:\n  ID:   %s\n  Title: %s\n  Type:  %s\n  File:  %s\n",
			result.Artifact.ID, result.Artifact.Title, result.Artifact.Type, result.Artifact.FilePath))
	}
	return textResult(b.String()), nil
}

func (r *ToolRegistry) handleSaveArtifact(ctx context.Context, args map[string]interface{}) (*ToolCallResult, error) {
	input := app.SaveArtifactInput{
		Title:    strArg(args, "title"),
		Type:     strArg(args, "type"),
		Content:  strArg(args, "content"),
		FilePath: strArg(args, "file_path"),
		Summary:  strArg(args, "summary"),
		Project:  strArg(args, "project"),
		Tags:     parseStrings(args["tags"]),
	}
	if input.Type == "" {
		input.Type = "markdown"
	}

	artifact, err := r.artifactSvc.SaveArtifact(ctx, input)
	if err != nil {
		return errResult("Error: " + err.Error()), nil
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Saved artifact: %s\n", artifact.ID))
	b.WriteString(fmt.Sprintf("  Title:   %s\n", artifact.Title))
	b.WriteString(fmt.Sprintf("  Type:    %s\n", artifact.Type))
	b.WriteString(fmt.Sprintf("  File:    %s\n", artifact.FilePath))
	b.WriteString(fmt.Sprintf("  Summary: %s\n", artifact.Summary))
	return textResult(b.String()), nil
}

func (r *ToolRegistry) handleGetContext(ctx context.Context, args map[string]interface{}) (*ToolCallResult, error) {
	input := app.ContextInput{
		Mode:            strArg(args, "mode"),
		Project:         strArg(args, "project"),
		Query:           strArg(args, "query"),
		Include:         parseStrings(args["include"]),
		ExcludeArchived: boolArg(args, "exclude_archived"),
		MaxChars:        intArg(args, "max_chars"),
	}

	pack, err := r.contextSvc.GetContext(ctx, input)
	if err != nil {
		return errResult("Error: " + err.Error()), nil
	}
	return textResult(pack.Raw), nil
}

func (r *ToolRegistry) handleComposeSeries(ctx context.Context, args map[string]interface{}) (*ToolCallResult, error) {
	seriesID := strArg(args, "series_id")
	if seriesID == "" {
		return errResult("Error: series_id is required"), nil
	}

	entries, err := r.seriesSvc.ComposeSeries(ctx, seriesID)
	if err != nil {
		return errResult("Error: " + err.Error()), nil
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Series %s (%d entries):\n", seriesID, len(entries)))
	for i, e := range entries {
		b.WriteString(fmt.Sprintf("  %d. [%s] %s\n", i+1, e.ID, e.Title))
		if e.Summary != "" {
			b.WriteString(fmt.Sprintf("      %s\n", e.Summary))
		}
	}
	return textResult(b.String()), nil
}

func (r *ToolRegistry) handleRenderWorkflow(ctx context.Context, args map[string]interface{}) (*ToolCallResult, error) {
	workflowID := strArg(args, "workflow_id")
	if workflowID == "" {
		return errResult("Error: workflow_id is required"), nil
	}

	steps, err := r.workflowSvc.RenderWorkflow(ctx, workflowID)
	if err != nil {
		return errResult("Error: " + err.Error()), nil
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Workflow: %s\n", workflowID))
	b.WriteString("Steps:\n")
	for _, s := range steps {
		req := ""
		if s.Required {
			req = " [REQUIRED]"
		}
		b.WriteString(fmt.Sprintf("  %d. %s%s\n", s.OrderIndex, s.Title, req))
		if s.Instruction != "" {
			b.WriteString(fmt.Sprintf("     %s\n", s.Instruction))
		}
		if s.ExpectedOutput != "" {
			b.WriteString(fmt.Sprintf("     Expected: %s\n", s.ExpectedOutput))
		}
	}
	return textResult(b.String()), nil
}

func (r *ToolRegistry) handleSessionWrap(ctx context.Context, args map[string]interface{}) (*ToolCallResult, error) {
	input := app.SessionWrapInput{
		Project:   strArg(args, "project"),
		Summary:   strArg(args, "summary"),
		Decisions: parseStrings(args["decisions"]),
		Pending:   parseStrings(args["pending"]),
		Learnings: parseStrings(args["learnings"]),
		Artifacts: parseStrings(args["artifacts"]),
	}

	output, err := r.sessionSvc.SessionWrap(ctx, input)
	if err != nil {
		return errResult("Error: " + err.Error()), nil
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Session saved: %s\n", output.Entry.Entry.Entry.ID))
	b.WriteString(fmt.Sprintf("  Summary: %s\n", output.Entry.Entry.Entry.Summary))
	b.WriteString(fmt.Sprintf("  Decisions: %d\n", len(input.Decisions)))
	b.WriteString(fmt.Sprintf("  Pending:   %d\n", len(input.Pending)))
	b.WriteString(fmt.Sprintf("  Learnings: %d\n", len(input.Learnings)))
	if output.Artifact != nil {
		b.WriteString(fmt.Sprintf("  Artifact:  %s\n", output.Artifact.ID))
	}
	return textResult(b.String()), nil
}

func (r *ToolRegistry) handleArchiveEntry(ctx context.Context, args map[string]interface{}) (*ToolCallResult, error) {
	id := strArg(args, "id")
	if id == "" {
		return errResult("Error: id is required"), nil
	}

	if err := r.entrySvc.ArchiveEntry(ctx, id); err != nil {
		return errResult("Error: " + err.Error()), nil
	}
	return textResult(fmt.Sprintf("Archived entry: %s", id)), nil
}

func (r *ToolRegistry) handleListProjects(ctx context.Context, _ map[string]interface{}) (*ToolCallResult, error) {
	projects, err := r.projectSvc.ListProjects(ctx)
	if err != nil {
		return errResult("Error: " + err.Error()), nil
	}

	if len(projects) == 0 {
		return textResult("No projects found."), nil
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Projects (%d):\n", len(projects)))
	for _, p := range projects {
		b.WriteString(fmt.Sprintf("\n  [%s] %s\n", p.ID, p.Name))
		b.WriteString(fmt.Sprintf("    Status:      %s\n", p.Status))
		if p.Description != "" {
			b.WriteString(fmt.Sprintf("    Description: %s\n", p.Description))
		}
	}
	return textResult(b.String()), nil
}

// --- helpers ---

func strArg(m map[string]interface{}, key string) string {
	v, _ := m[key].(string)
	return v
}

func boolArg(m map[string]interface{}, key string) bool {
	v, _ := m[key].(bool)
	return v
}

func intArg(m map[string]interface{}, key string) int {
	v, ok := m[key]
	if !ok {
		return 0
	}
	f, ok := v.(float64)
	if !ok {
		return 0
	}
	return int(f)
}

func parseStrings(v interface{}) []string {
	raw, ok := v.([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func textResult(text string) *ToolCallResult {
	return &ToolCallResult{
		Content: []ToolContent{{Type: "text", Text: text}},
	}
}

func jsonResult(v any) *ToolCallResult {
	data, _ := json.Marshal(v)
	return &ToolCallResult{
		Content: []ToolContent{{Type: "text", Text: string(data)}},
	}
}

func (r *ToolRegistry) handleSaveEntryRef(ctx context.Context, args map[string]interface{}) (*ToolCallResult, error) {
	sourceID := strArg(args, "source_id")
	targetID := strArg(args, "target_id")
	refType := strArg(args, "relation_type")
	label := strArg(args, "label")

	if sourceID == "" || targetID == "" || refType == "" {
		return errResult("Error: source_id, target_id, and relation_type are required"), nil
	}

	var svc *app.EntryRefService
	if r.entryRefSvc != nil {
		svc = r.entryRefSvc
	} else {
		return errResult("Error: entry ref service not available"), nil
	}

	input := app.AddRefInput{
		SourceID: sourceID,
		TargetID: targetID,
		RefType:  refType,
		Label:    label,
	}

	link, err := svc.SaveRef(ctx, input)
	if err != nil {
		return errResult("Error: " + err.Error()), nil
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Saved ref: %s --[%s]--> %s\n", link.FromEntryID, link.RelationType, link.ToEntryID))
	if link.Label != "" {
		b.WriteString(fmt.Sprintf("  Label: %s\n", link.Label))
	}
	return textResult(b.String()), nil
}

func (r *ToolRegistry) handleListEntryRefs(ctx context.Context, args map[string]interface{}) (*ToolCallResult, error) {
	sourceID := strArg(args, "source_id")
	targetID := strArg(args, "target_id")
	refType := strArg(args, "relation_type")
	includeArchived := boolArg(args, "include_archived")

	var svc *app.EntryRefService
	if r.entryRefSvc != nil {
		svc = r.entryRefSvc
	} else {
		return errResult("Error: entry ref service not available"), nil
	}

	var srcPtr, tgtPtr, typePtr *string
	if sourceID != "" {
		srcPtr = &sourceID
	}
	if targetID != "" {
		tgtPtr = &targetID
	}
	if refType != "" {
		typePtr = &refType
	}

	links, err := svc.ListRefs(ctx, app.ListRefsInput{
		SourceID:        srcPtr,
		TargetID:        tgtPtr,
		RefType:         typePtr,
		IncludeArchived: includeArchived,
	})
	if err != nil {
		return errResult("Error: " + err.Error()), nil
	}

	if len(links) == 0 {
		return textResult("No refs found."), nil
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Found %d ref(s):\n", len(links)))
	for _, l := range links {
		la := ""
		if l.Label != "" {
			la = fmt.Sprintf(" (%s)", l.Label)
		}
		b.WriteString(fmt.Sprintf("  %s --[%s%s]--> %s\n", l.FromEntryID, l.RelationType, la, l.ToEntryID))
	}
	return textResult(b.String()), nil
}

func (r *ToolRegistry) handleGetEntryGraph(ctx context.Context, args map[string]interface{}) (*ToolCallResult, error) {
	entryID := strArg(args, "entry_id")
	if entryID == "" {
		return errResult("Error: entry_id is required"), nil
	}

	refTypes := parseStrings(args["ref_types"])
	direction := strArg(args, "direction")
	maxDepth := intArg(args, "max_depth")

	var svc *app.EntryRefService
	if r.entryRefSvc != nil {
		svc = r.entryRefSvc
	} else {
		return errResult("Error: entry ref service not available"), nil
	}

	result, err := svc.GetEntryGraph(ctx, app.GetGraphInput{
		EntryID:   entryID,
		RefTypes:  refTypes,
		Direction: direction,
		MaxDepth:  maxDepth,
	})
	if err != nil {
		return errResult("Error: " + err.Error()), nil
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Graph rooted at %s (%d nodes, %d edges):\n", entryID, len(result.Nodes), len(result.Edges)))
	if len(result.Edges) > 0 {
		b.WriteString("\nEdges:\n")
		for _, e := range result.Edges {
			la := ""
			if e.Label != "" {
				la = fmt.Sprintf(" (%s)", e.Label)
			}
			b.WriteString(fmt.Sprintf("  %s --[%s%s]--> %s\n", e.FromEntryID, e.RelationType, la, e.ToEntryID))
		}
	}
	return textResult(b.String()), nil
}

func (r *ToolRegistry) handleSearchByTags(ctx context.Context, args map[string]interface{}) (*ToolCallResult, error) {
	tags := parseStrings(args["tags"])
	if len(tags) == 0 {
		return errResult("Error: tags is required"), nil
	}

	match := strArg(args, "match")
	matchAll := match != "any"

	typ := strArg(args, "type")
	project := strArg(args, "project")
	limit := intArg(args, "limit")
	if limit <= 0 {
		limit = 20
	}

	var typePtr, projectID *string
	if typ != "" {
		typePtr = &typ
	}
	if project != "" {
		proj, err := r.projectSvc.GetProject(ctx, project)
		if err == nil {
			projectID = &proj.ID
		} else {
			projectID = &project
		}
	}

	results, err := r.entrySvc.SearchByTags(ctx, tags, matchAll, typePtr, projectID, limit)
	if err != nil {
		return errResult("Error: " + err.Error()), nil
	}

	if len(results) == 0 {
		return textResult("No results found."), nil
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Found %d result(s):\n", len(results)))
	for _, r := range results {
		proj := "global"
		if r.Entry.ProjectID != nil {
			proj = *r.Entry.ProjectID
		}
		b.WriteString(fmt.Sprintf("\n  [%s] %s\n", r.Entry.ID, r.Entry.Title))
		b.WriteString(fmt.Sprintf("    Type:    %s\n", r.Entry.Type))
		b.WriteString(fmt.Sprintf("    Summary: %s\n", r.Entry.Summary))
		b.WriteString(fmt.Sprintf("    Project: %s\n", proj))
		b.WriteString(fmt.Sprintf("    Status:  %s\n", r.Entry.Status))
		if len(r.Tags) > 0 {
			tagNames := make([]string, len(r.Tags))
			for i, t := range r.Tags {
				tagNames[i] = t.Name
			}
			b.WriteString(fmt.Sprintf("    Tags:    %s\n", strings.Join(tagNames, ", ")))
		}
	}
	return textResult(b.String()), nil
}

func (r *ToolRegistry) handleGetContextBundle(ctx context.Context, args map[string]interface{}) (*ToolCallResult, error) {
	project := strArg(args, "project")

	type bundleProject struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Status      string `json:"status"`
	}

	type bundleEntry struct {
		ID      string   `json:"id"`
		Title   string   `json:"title"`
		Summary string   `json:"summary"`
		Status  string   `json:"status"`
		Tags    []string `json:"tags"`
	}

	type bundleArtifact struct {
		ID      string `json:"id"`
		Title   string `json:"title"`
		Type    string `json:"type"`
		Summary string `json:"summary"`
	}

	type bundle struct {
		Project   *bundleProject           `json:"project,omitempty"`
		Entries   map[string][]bundleEntry `json:"entries"`
		Artifacts []bundleArtifact         `json:"artifacts"`
	}

	out := bundle{
		Entries:   make(map[string][]bundleEntry),
		Artifacts: []bundleArtifact{},
	}

	var projectID *string
	if project != "" {
		proj, err := r.projectSvc.GetProject(ctx, project)
		if err != nil {
			return errResult("Error: project not found: " + err.Error()), nil
		}
		projectID = &proj.ID

		out.Project = &bundleProject{
			ID:          proj.ID,
			Name:        proj.Name,
			Description: proj.Description,
			Status:      string(proj.Status),
		}
	}

	filter := domain.EntryFilter{}
	if projectID != nil {
		filter.ProjectID = projectID
	}
	entries, err := r.entrySvc.List(ctx, filter)
	if err != nil {
		return errResult("Error: listing entries: " + err.Error()), nil
	}
	for _, e := range entries {
		typ := string(e.Entry.Type)
		be := bundleEntry{
			ID:      e.Entry.ID,
			Title:   e.Entry.Title,
			Summary: e.Entry.Summary,
			Status:  string(e.Entry.Status),
			Tags:    make([]string, 0, len(e.Tags)),
		}
		for _, t := range e.Tags {
			be.Tags = append(be.Tags, t.Name)
		}
		out.Entries[typ] = append(out.Entries[typ], be)
	}

	artifacts, err := r.artifactSvc.ListArtifacts(ctx, projectID)
	if err == nil {
		for _, a := range artifacts {
			out.Artifacts = append(out.Artifacts, bundleArtifact{
				ID:      a.ID,
				Title:   a.Title,
				Type:    string(a.Type),
				Summary: a.Summary,
			})
		}
	}

	data, err := json.Marshal(out)
	if err != nil {
		return errResult("Error: marshal bundle: " + err.Error()), nil
	}

	return textResult(string(data)), nil
}

func (r *ToolRegistry) handleCompareEntries(ctx context.Context, args map[string]interface{}) (*ToolCallResult, error) {
	if r.compareSvc == nil {
		return errResult("Error: compare service not available"), nil
	}

	id1 := strArg(args, "id1")
	id2 := strArg(args, "id2")
	if id1 == "" || id2 == "" {
		return errResult("Error: id1 and id2 are required"), nil
	}

	result, err := r.compareSvc.CompareEntries(ctx, id1, id2)
	if err != nil {
		return errResult("Error: " + err.Error()), nil
	}

	return textResult(result), nil
}

func (r *ToolRegistry) handleSaveResult(ctx context.Context, args map[string]interface{}) (*ToolCallResult, error) {
	if r.saveResultSvc == nil {
		return errResult("Error: save result service not available"), nil
	}

	input := app.SavePromptResultInput{
		Name:           strArg(args, "name"),
		Content:        strArg(args, "content"),
		Type:           strArg(args, "type"),
		Category:       strArg(args, "category"),
		Tags:           parseStrings(args["tags"]),
		ProjectID:      strArg(args, "project_id"),
		SourcePromptID: strArg(args, "source_prompt_id"),
		Model:          strArg(args, "model"),
	}

	output, err := r.saveResultSvc.Save(ctx, input)
	if err != nil {
		return errResult("Error: " + err.Error()), nil
	}

	proj := "global"
	if output.ProjectID != "" {
		proj = output.ProjectID
	}

	result := map[string]interface{}{
		"entry_id":   output.EntryID,
		"name":       output.Name,
		"type":       output.Type,
		"project_id": proj,
	}
	return jsonResult(result), nil
}

func errResult(text string) *ToolCallResult {
	return &ToolCallResult{
		Content: []ToolContent{{Type: "text", Text: text}},
		IsError: true,
	}
}

func schemaObj(props map[string]interface{}) json.RawMessage {
	schema := map[string]interface{}{
		"type":       "object",
		"properties": props,
	}
	data, _ := json.Marshal(schema)
	return data
}
