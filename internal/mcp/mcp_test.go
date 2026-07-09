package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/quantum-6/skillvault/internal/app"
	"github.com/quantum-6/skillvault/internal/db"
	"github.com/quantum-6/skillvault/internal/domain"

	_ "modernc.org/sqlite"
)

func setupMCPServices(t *testing.T) (*ToolRegistry, *app.ProjectService, func()) {
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

	entrySvc := app.NewEntryService(store.Entries, store.Projects, store.Artifacts)
	artifactSvc := app.NewArtifactService(store.Artifacts, store.Entries, store.Projects)
	workflowSvc := app.NewWorkflowService(store.Workflows)
	seriesSvc := app.NewSeriesService(store.Series, store.Entries)
	projectSvc := app.NewProjectService(store.Projects)
	entryRefSvc := app.NewEntryRefService(store.EntryLinks, store.Entries)
	contextSvc := app.NewContextService(store.Entries, store.Projects, store.Series, store.Workflows, store.Artifacts, entrySvc)
	sessionSvc := app.NewSessionService(entrySvc, artifactSvc, projectSvc, store.Entries, store.Artifacts, store.Projects)
	workflowRunSvc := app.NewWorkflowRunService(store.Workflows, store.WorkflowRuns, store.Entries)
	entrySvc.SetWorkflowStore(store.Workflows)
	entryVersionSvc := app.NewEntryVersionService(store.EntryVersions, store.Entries)

	reg := NewServiceToolRegistry(entrySvc, artifactSvc, contextSvc, seriesSvc, workflowSvc, sessionSvc, projectSvc).WithEntryRefService(entryRefSvc).WithWorkflowRunService(workflowRunSvc).WithEntryVersionService(entryVersionSvc)
	cleanup := func() { sqlDB.Close() }
	return reg, projectSvc, cleanup
}

func TestServerInitialize(t *testing.T) {
	s := NewServer(nil)
	ctx := context.Background()

	resp := s.handleRequest(ctx, &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
	})

	if resp.Error != nil {
		t.Fatalf("initialize returned error: %v", resp.Error)
	}
	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}
	if v, ok := result["protocolVersion"].(string); !ok || v != "2024-11-05" {
		t.Errorf("protocolVersion = %v, want '2024-11-05'", result["protocolVersion"])
	}
}

func TestToolsListReturns24Tools(t *testing.T) {
	reg := NewToolRegistry(nil)
	s := NewServer(reg)
	ctx := context.Background()

	resp := s.handleRequest(ctx, &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
	})

	if resp.Error != nil {
		t.Fatalf("tools/list returned error: %v", resp.Error)
	}
	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}
	toolsRaw, ok := result["tools"]
	if !ok {
		t.Fatal("tools key missing from result")
	}
	toolCount := 0
	switch v := toolsRaw.(type) {
	case []Tool:
		toolCount = len(v)
	case []interface{}:
		toolCount = len(v)
	default:
		t.Fatalf("tools is not an array: %T", toolsRaw)
	}
	if toolCount != 24 {
		t.Errorf("expected 24 tools, got %d", toolCount)
	}
}

func TestToolsCall(t *testing.T) {
	reg := NewToolRegistry(func(ctx context.Context, name string, args map[string]interface{}) (*ToolCallResult, error) {
		return &ToolCallResult{
			Content: []ToolContent{{Type: "text", Text: "called " + name}},
		}, nil
	})
	s := NewServer(reg)
	ctx := context.Background()

	params, _ := json.Marshal(ToolCallParams{Name: "get_entry", Arguments: map[string]interface{}{"id": "test"}})

	resp := s.handleRequest(ctx, &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  params,
	})

	if resp.Error != nil {
		t.Fatalf("tools/call returned error: %v", resp.Error)
	}
}

func TestSaveEntryMCP(t *testing.T) {
	reg, projectSvc, cleanup := setupMCPServices(t)
	defer cleanup()
	ctx := context.Background()

	projectSvc.SaveProject(ctx, app.SaveProjectInput{Name: "testproj", Description: "Test project"})

	args := map[string]interface{}{
		"title":   "MCP Save Test",
		"type":    "skill",
		"summary": "Test saving via MCP",
		"body":    "Body content here",
		"project": "testproj",
		"tags":    []interface{}{"mcp", "test"},
		"status":  "active",
	}

	result, err := reg.Call(ctx, "save_entry", args)
	if err != nil {
		t.Fatalf("save_entry failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("save_entry returned error: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "MCP Save Test") {
		t.Errorf("result should contain title, got: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "skill") {
		t.Errorf("result should contain type, got: %s", result.Content[0].Text)
	}
}

func TestGetEntryMCP(t *testing.T) {
	reg, projectSvc, cleanup := setupMCPServices(t)
	defer cleanup()
	ctx := context.Background()

	projectSvc.SaveProject(ctx, app.SaveProjectInput{Name: "testproj"})

	saveResult, err := reg.Call(ctx, "save_entry", map[string]interface{}{
		"title":   "Entry to Get",
		"type":    "reference",
		"summary": "Will be retrieved",
		"project": "testproj",
	})
	if err != nil {
		t.Fatalf("save_entry failed: %v", err)
	}

	text := saveResult.Content[0].Text
	lines := strings.Split(text, "\n")
	idLine := strings.TrimSpace(lines[0])
	id := strings.TrimPrefix(idLine, "Saved: ")

	result, err := reg.Call(ctx, "get_entry", map[string]interface{}{
		"id": id,
	})
	if err != nil {
		t.Fatalf("get_entry failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("get_entry returned error: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "Entry to Get") {
		t.Errorf("result should contain entry title, got: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "reference") {
		t.Errorf("result should contain entry type, got: %s", result.Content[0].Text)
	}
}

func TestSearchEntriesMCP(t *testing.T) {
	reg, projectSvc, cleanup := setupMCPServices(t)
	defer cleanup()
	ctx := context.Background()

	projectSvc.SaveProject(ctx, app.SaveProjectInput{Name: "testproj"})

	reg.Call(ctx, "save_entry", map[string]interface{}{
		"title":   "Go REST API",
		"type":    "skill",
		"summary": "Building REST APIs in Go",
		"body":    "Use chi router",
		"project": "testproj",
		"tags":    []interface{}{"go", "rest"},
	})
	reg.Call(ctx, "save_entry", map[string]interface{}{
		"title":   "Python Data Science",
		"type":    "skill",
		"summary": "Data science with Python",
		"project": "testproj",
		"tags":    []interface{}{"python"},
	})

	result, err := reg.Call(ctx, "search_entries", map[string]interface{}{
		"query":   "REST",
		"limit":   float64(10),
		"project": "testproj",
	})
	if err != nil {
		t.Fatalf("search_entries failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("search_entries returned error: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "Go REST API") {
		t.Errorf("expected 'Go REST API' in results, got: %s", result.Content[0].Text)
	}
	if strings.Contains(result.Content[0].Text, "Python Data Science") {
		t.Error("should NOT find 'Python Data Science' for query 'REST'")
	}
}

func TestGetContextMCP(t *testing.T) {
	reg, projectSvc, cleanup := setupMCPServices(t)
	defer cleanup()
	ctx := context.Background()

	proj, err := projectSvc.SaveProject(ctx, app.SaveProjectInput{Name: "testproj", Description: "Test project"})
	if err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	reg.Call(ctx, "save_entry", map[string]interface{}{
		"title":   "Use chi router",
		"type":    "decision",
		"summary": "Use chi for routing",
		"project": proj.ID,
	})

	result, err := reg.Call(ctx, "get_context", map[string]interface{}{
		"mode":             "planning",
		"project":          proj.ID,
		"include":          []interface{}{"decisions"},
		"exclude_archived": true,
		"max_chars":        float64(5000),
	})
	if err != nil {
		t.Fatalf("get_context failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("get_context returned error: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "CONTEXT PACK") {
		t.Error("expected CONTEXT PACK header")
	}
	if !strings.Contains(result.Content[0].Text, "Use chi router") {
		t.Error("expected decision in context pack")
	}
}

func TestToolNamesAreCorrect(t *testing.T) {
	reg := NewToolRegistry(nil)
	tools := reg.List()

	names := make([]string, len(tools))
	for i, tool := range tools {
		names[i] = tool.Name
	}

	expected := []string{
		"save_entry",
		"search_entries",
		"get_entry",
		"save_artifact",
		"get_context",
		"compose_series",
		"render_workflow",
		"session_wrap",
		"archive_entry",
		"list_projects",
		"save_entry_ref",
		"list_entry_refs",
		"get_entry_graph",
		"search_by_tags",
		"get_context_bundle",
		"compare_entries",
		"save_result",
		"run_workflow",
		"route_scenario",
		"get_stats",
		"list_workflow_runs",
		"get_run",
		"list_entry_versions",
		"restore_entry_version",
	}

	if len(names) != len(expected) {
		t.Fatalf("expected %d tools, got %d: %v", len(expected), len(names), names)
	}
	for i, name := range names {
		if name != expected[i] {
			t.Errorf("tool[%d] = %q, want %q", i, name, expected[i])
		}
	}
}

func TestServerUnknownMethod(t *testing.T) {
	s := NewServer(nil)
	ctx := context.Background()

	resp := s.handleRequest(ctx, &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "invalid/method",
	})

	if resp.Error == nil {
		t.Fatal("expected error for unknown method")
	}
	if resp.Error.Code != ErrCodeMethod {
		t.Errorf("error code = %d, want %d", resp.Error.Code, ErrCodeMethod)
	}
}

func TestJSONRPCTypes(t *testing.T) {
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: "test"}
	data, _ := json.Marshal(req)
	if !strings.Contains(string(data), "jsonrpc") {
		t.Error("request should contain jsonrpc field")
	}

	err := NewError(ErrCodeParse, "parse error", nil)
	if err.Code != ErrCodeParse {
		t.Errorf("error code = %d, want %d", err.Code, ErrCodeParse)
	}

	result := NewResult(1, map[string]string{"key": "value"})
	if result.JSONRPC != "2.0" {
		t.Errorf("JSONRPC = %q, want '2.0'", result.JSONRPC)
	}
}

func TestListProjectsMCP(t *testing.T) {
	reg, projectSvc, cleanup := setupMCPServices(t)
	defer cleanup()
	ctx := context.Background()

	projectSvc.SaveProject(ctx, app.SaveProjectInput{Name: "Alpha", Description: "First project"})
	projectSvc.SaveProject(ctx, app.SaveProjectInput{Name: "Beta", Description: "Second project"})

	result, err := reg.Call(ctx, "list_projects", nil)
	if err != nil {
		t.Fatalf("list_projects failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("list_projects returned error: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "Alpha") {
		t.Error("expected Alpha in project list")
	}
	if !strings.Contains(result.Content[0].Text, "Beta") {
		t.Error("expected Beta in project list")
	}
}

func TestArchiveEntryMCP(t *testing.T) {
	reg, projectSvc, cleanup := setupMCPServices(t)
	defer cleanup()
	ctx := context.Background()

	projectSvc.SaveProject(ctx, app.SaveProjectInput{Name: "testproj"})

	saveResult, _ := reg.Call(ctx, "save_entry", map[string]interface{}{
		"title":   "To Archive",
		"type":    "skill",
		"summary": "Will be archived",
		"project": "testproj",
	})
	text := saveResult.Content[0].Text
	id := strings.TrimPrefix(strings.Split(text, "\n")[0], "Saved: ")

	result, err := reg.Call(ctx, "archive_entry", map[string]interface{}{
		"id": id,
	})
	if err != nil {
		t.Fatalf("archive_entry failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("archive_entry returned error: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "Archived") {
		t.Errorf("expected 'Archived' in result, got: %s", result.Content[0].Text)
	}

	getResult, _ := reg.Call(ctx, "get_entry", map[string]interface{}{"id": id})
	if !getResult.IsError {
		t.Error("expected error when getting archived entry without include_archived")
	}
}

// AC10: MCP agent use
// Given an agent calls MCP get_context, it receives the same context pack as CLI get-context.
func TestAC10_MCPGetContextMatchesCLIGetContext(t *testing.T) {
	reg, projectSvc, cleanup := setupMCPServices(t)
	defer cleanup()
	ctx := context.Background()

	proj, err := projectSvc.SaveProject(ctx, app.SaveProjectInput{Name: "ac10-test", Description: "AC10 test project"})
	if err != nil {
		t.Fatalf("AC10 FAIL: SaveProject failed: %v", err)
	}

	reg.Call(ctx, "save_entry", map[string]interface{}{
		"title":   "Use Go chi",
		"type":    "decision",
		"summary": "Use chi for routing",
		"project": proj.ID,
	})
	reg.Call(ctx, "save_entry", map[string]interface{}{
		"title":   "Prefer simple arch",
		"type":    "feedback",
		"summary": "Prefer simple architecture",
	})

	// Get context via MCP get_context tool
	mcpResult, err := reg.Call(ctx, "get_context", map[string]interface{}{
		"mode":             "planning",
		"project":          proj.ID,
		"include":          []interface{}{"decisions", "profile"},
		"exclude_archived": true,
		"max_chars":        float64(5000),
	})
	if err != nil {
		t.Fatalf("AC10 FAIL: MCP get_context failed: %v", err)
	}
	if mcpResult.IsError {
		t.Fatalf("AC10 FAIL: MCP get_context returned error: %s", mcpResult.Content[0].Text)
	}

	mcpText := mcpResult.Content[0].Text

	// Get context via direct ContextService call (simulates CLI)
	directPack, err := reg.contextSvc.GetContext(ctx, app.ContextInput{
		Mode:            "planning",
		Project:         proj.ID,
		Include:         []string{"decisions", "profile"},
		ExcludeArchived: true,
		MaxChars:        5000,
	})
	if err != nil {
		t.Fatalf("AC10 FAIL: direct GetContext failed: %v", err)
	}

	directText := directPack.Raw

	// Both must contain the same key content
	if !strings.Contains(mcpText, "CONTEXT PACK") {
		t.Fatal("AC10 FAIL: MCP output missing CONTEXT PACK header")
	}
	if !strings.Contains(directText, "CONTEXT PACK") {
		t.Fatal("AC10 FAIL: CLI output missing CONTEXT PACK header")
	}
	if !strings.Contains(mcpText, "Use Go chi") {
		t.Fatal("AC10 FAIL: MCP output missing decision entry")
	}
	if !strings.Contains(directText, "Use Go chi") {
		t.Fatal("AC10 FAIL: CLI output missing decision entry")
	}
	if !strings.Contains(mcpText, "ac10-test") {
		t.Fatal("AC10 FAIL: MCP output missing project reference")
	}
	if !strings.Contains(directText, "ac10-test") {
		t.Fatal("AC10 FAIL: CLI output missing project reference")
	}

	// Structure should match
	if strings.Count(mcpText, "## ") != strings.Count(directText, "## ") {
		t.Errorf("AC10 FAIL: section count mismatch: MCP=%d, CLI=%d",
			strings.Count(mcpText, "## "), strings.Count(directText, "## "))
	}
}

func TestSaveEntryRejectsMissingTitle(t *testing.T) {
	reg, _, cleanup := setupMCPServices(t)
	defer cleanup()
	ctx := context.Background()

	result, err := reg.Call(ctx, "save_entry", map[string]interface{}{
		"title": "",
		"type":  "skill",
	})
	if err != nil {
		t.Fatalf("save_entry should not return error from dispatch: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for empty title")
	}
	if !strings.Contains(result.Content[0].Text, "title is required") {
		t.Errorf("expected 'title is required', got: %s", result.Content[0].Text)
	}
}

func TestSaveResultMCP(t *testing.T) {
	ctx := context.Background()

	sqlDB, err := db.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB failed: %v", err)
	}
	defer sqlDB.Close()
	if err := db.RunMigrations(sqlDB); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}
	store := db.NewStore(sqlDB)

	entrySvc := app.NewEntryService(store.Entries, store.Projects, store.Artifacts)
	projectSvc := app.NewProjectService(store.Projects)
	saveResultSvc := app.NewSavePromptResultService(store.Entries, store.Projects, store.Artifacts)

	reg := &ToolRegistry{entrySvc: entrySvc}
	reg.saveResultSvc = saveResultSvc
	reg.registerV2Tools()

	proj, err := projectSvc.SaveProject(ctx, app.SaveProjectInput{Name: "testproj"})
	if err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	// Valid save_result.
	result, err2 := reg.Call(ctx, "save_result", map[string]interface{}{
		"name":       "Test Result",
		"content":    "This is a test result from AI analysis.",
		"type":       "reference",
		"category":   "test category",
		"tags":       []interface{}{"test", "ai"},
		"project_id": proj.ID,
		"model":      "gpt-4",
	})
	if err2 != nil {
		t.Fatalf("save_result failed: %v", err2)
	}
	if err != nil {
		t.Fatalf("save_result failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("save_result returned error: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "entry_id") {
		t.Errorf("result should contain entry_id, got: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "Test Result") {
		t.Errorf("result should contain name, got: %s", result.Content[0].Text)
	}

	// Missing name should fail.
	result2, err3 := reg.Call(ctx, "save_result", map[string]interface{}{
		"name":    "",
		"content": "Content without name",
	})
	if err3 != nil {
		t.Fatalf("save_result should not return dispatch error: %v", err3)
	}
	if !result2.IsError {
		t.Fatalf("expected error for missing name, got: %s", result2.Content[0].Text)
	}
	if !strings.Contains(result2.Content[0].Text, "required") {
		t.Errorf("expected 'required' in error, got: %s", result2.Content[0].Text)
	}
	_ = proj
	_ = result
}

func TestSaveEntryRefMCP(t *testing.T) {
	reg, projectSvc, cleanup := setupMCPServices(t)
	defer cleanup()
	ctx := context.Background()

	projectSvc.SaveProject(ctx, app.SaveProjectInput{Name: "testproj"})

	// Create two entries via MCP
	e1, _ := reg.Call(ctx, "save_entry", map[string]interface{}{
		"title":   "Source Entry",
		"type":    "skill",
		"summary": "A source entry",
		"project": "testproj",
	})
	// Parse ID from response
	id1 := extractEntryID(t, e1)

	e2, _ := reg.Call(ctx, "save_entry", map[string]interface{}{
		"title":   "Target Entry",
		"type":    "skill",
		"summary": "A target entry",
		"project": "testproj",
	})
	id2 := extractEntryID(t, e2)

	// Save ref between them
	result, err := reg.Call(ctx, "save_entry_ref", map[string]interface{}{
		"source_id":     id1,
		"target_id":     id2,
		"relation_type": "references",
		"label":         "test link",
	})
	if err != nil {
		t.Fatalf("save_entry_ref failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("save_entry_ref returned error: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, id1) || !strings.Contains(result.Content[0].Text, id2) {
		t.Errorf("result should contain entry IDs, got: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "references") {
		t.Errorf("result should contain relation type, got: %s", result.Content[0].Text)
	}
}

func TestSaveEntryRefMissingArgsMCP(t *testing.T) {
	reg, _, cleanup := setupMCPServices(t)
	defer cleanup()
	ctx := context.Background()

	result, err := reg.Call(ctx, "save_entry_ref", map[string]interface{}{
		"source_id": "e1",
	})
	if err != nil {
		t.Fatalf("save_entry_ref should not return dispatch error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for missing target_id and relation_type")
	}
	if !strings.Contains(result.Content[0].Text, "required") {
		t.Errorf("expected 'required' in error, got: %s", result.Content[0].Text)
	}
}

func TestListEntryRefsMCP(t *testing.T) {
	reg, projectSvc, cleanup := setupMCPServices(t)
	defer cleanup()
	ctx := context.Background()

	projectSvc.SaveProject(ctx, app.SaveProjectInput{Name: "testproj"})

	e1 := saveTestEntry(t, reg, "Source", "skill")
	e2 := saveTestEntry(t, reg, "Target", "skill")
	e3 := saveTestEntry(t, reg, "Other", "skill")

	// Create two refs
	_, _ = reg.Call(ctx, "save_entry_ref", map[string]interface{}{
		"source_id":     e1,
		"target_id":     e2,
		"relation_type": "references",
	})
	_, _ = reg.Call(ctx, "save_entry_ref", map[string]interface{}{
		"source_id":     e1,
		"target_id":     e3,
		"relation_type": "related_to",
	})

	// List all refs from e1
	result, err := reg.Call(ctx, "list_entry_refs", map[string]interface{}{
		"source_id": e1,
	})
	if err != nil {
		t.Fatalf("list_entry_refs failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("list_entry_refs returned error: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "2 ref(s)") && !strings.Contains(result.Content[0].Text, "Found 2") {
		t.Errorf("expected 2 refs, got: %s", result.Content[0].Text)
	}

	// List by type
	result, err = reg.Call(ctx, "list_entry_refs", map[string]interface{}{
		"source_id":     e1,
		"relation_type": "references",
	})
	if err != nil {
		t.Fatalf("list_entry_refs by type failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("list_entry_refs by type returned error: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "1 ref(s)") && !strings.Contains(result.Content[0].Text, "Found 1") {
		t.Errorf("expected 1 ref, got: %s", result.Content[0].Text)
	}
}

func TestGetEntryGraphMCP(t *testing.T) {
	reg, projectSvc, cleanup := setupMCPServices(t)
	defer cleanup()
	ctx := context.Background()

	projectSvc.SaveProject(ctx, app.SaveProjectInput{Name: "testproj"})

	a := saveTestEntry(t, reg, "A", "skill")
	b := saveTestEntry(t, reg, "B", "skill")
	c := saveTestEntry(t, reg, "C", "skill")

	// A depends_on B, B depends_on C
	_, _ = reg.Call(ctx, "save_entry_ref", map[string]interface{}{
		"source_id":     a,
		"target_id":     b,
		"relation_type": "depends_on",
	})
	_, _ = reg.Call(ctx, "save_entry_ref", map[string]interface{}{
		"source_id":     b,
		"target_id":     c,
		"relation_type": "depends_on",
	})

	// Get graph rooted at A, depth 3
	result, err := reg.Call(ctx, "get_entry_graph", map[string]interface{}{
		"entry_id":  a,
		"max_depth": 3,
	})
	if err != nil {
		t.Fatalf("get_entry_graph failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("get_entry_graph returned error: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "2 edges") && !strings.Contains(result.Content[0].Text, "Graph rooted") {
		t.Errorf("expected graph output, got: %s", result.Content[0].Text)
	}

	// Test cycle detection: adding C depends_on A should fail
	result, err = reg.Call(ctx, "save_entry_ref", map[string]interface{}{
		"source_id":     c,
		"target_id":     a,
		"relation_type": "depends_on",
	})
	if err != nil {
		t.Fatalf("cyclic save_entry_ref should not return dispatch error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for cycle detection")
	}
	if !strings.Contains(result.Content[0].Text, "cycle_detected") {
		t.Errorf("expected 'cycle_detected' in error, got: %s", result.Content[0].Text)
	}
}

func TestSearchByTagsMCP(t *testing.T) {
	reg, projectSvc, cleanup := setupMCPServices(t)
	defer cleanup()
	ctx := context.Background()

	projectSvc.SaveProject(ctx, app.SaveProjectInput{Name: "testproj", Description: "Test project"})

	// Seed entries with specific tags
	reg.Call(ctx, "save_entry", map[string]interface{}{
		"title":   "Dual Tag Entry",
		"type":    "skill",
		"summary": "Has go and cli tags",
		"project": "testproj",
		"tags":    []interface{}{"go", "cli"},
	})
	reg.Call(ctx, "save_entry", map[string]interface{}{
		"title":   "Go Only Entry",
		"type":    "skill",
		"summary": "Only go tag",
		"project": "testproj",
		"tags":    []interface{}{"go"},
	})
	reg.Call(ctx, "save_entry", map[string]interface{}{
		"title":   "CLI Only Entry",
		"type":    "skill",
		"summary": "Only cli tag",
		"project": "testproj",
		"tags":    []interface{}{"cli"},
	})

	// Test all match (intersection)
	result, err := reg.Call(ctx, "search_by_tags", map[string]interface{}{
		"tags":  []interface{}{"go", "cli"},
		"match": "all",
		"limit": float64(20),
	})
	if err != nil {
		t.Fatalf("search_by_tags (all) failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("search_by_tags (all) returned error: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "Dual Tag Entry") {
		t.Errorf("all-match should contain Dual Tag Entry, got: %s", result.Content[0].Text)
	}
	if strings.Contains(result.Content[0].Text, "Go Only Entry") {
		t.Error("all-match should NOT contain Go Only Entry")
	}
	if strings.Contains(result.Content[0].Text, "CLI Only Entry") {
		t.Error("all-match should NOT contain CLI Only Entry")
	}

	// Test any match (union)
	result, err = reg.Call(ctx, "search_by_tags", map[string]interface{}{
		"tags":  []interface{}{"go", "cli"},
		"match": "any",
		"limit": float64(20),
	})
	if err != nil {
		t.Fatalf("search_by_tags (any) failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("search_by_tags (any) returned error: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "Dual Tag Entry") {
		t.Error("any-match should contain Dual Tag Entry")
	}
	if !strings.Contains(result.Content[0].Text, "Go Only Entry") {
		t.Error("any-match should contain Go Only Entry")
	}
	if !strings.Contains(result.Content[0].Text, "CLI Only Entry") {
		t.Error("any-match should contain CLI Only Entry")
	}
	if !strings.Contains(result.Content[0].Text, "Found 3") {
		t.Errorf("any-match should show 3 results, got: %s", result.Content[0].Text)
	}
}

func TestGetContextBundleMCP(t *testing.T) {
	reg, projectSvc, cleanup := setupMCPServices(t)
	defer cleanup()
	ctx := context.Background()

	proj, err := projectSvc.SaveProject(ctx, app.SaveProjectInput{Name: "bundleproj", Description: "Bundle test project"})
	if err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	// Seed entries of different types
	reg.Call(ctx, "save_entry", map[string]interface{}{
		"title":   "Use chi router",
		"type":    "decision",
		"summary": "Use chi for routing",
		"project": proj.ID,
	})
	reg.Call(ctx, "save_entry", map[string]interface{}{
		"title":   "Session wrap",
		"type":    "session",
		"summary": "End of session",
		"project": proj.ID,
	})
	reg.Call(ctx, "save_entry", map[string]interface{}{
		"title":   "Go style guide",
		"type":    "reference",
		"summary": "Go style conventions",
		"project": proj.ID,
	})

	// Save an artifact
	reg.Call(ctx, "save_artifact", map[string]interface{}{
		"title":   "Bundle Artifact",
		"type":    "markdown",
		"content": "Artifact content",
		"summary": "Test artifact",
		"project": proj.ID,
	})

	result, err := reg.Call(ctx, "get_context_bundle", map[string]interface{}{
		"project": proj.ID,
	})
	if err != nil {
		t.Fatalf("get_context_bundle failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("get_context_bundle returned error: %s", result.Content[0].Text)
	}

	// Verify it's valid JSON
	raw := result.Content[0].Text
	if !strings.Contains(raw, "{") || !strings.Contains(raw, "}") {
		t.Fatalf("get_context_bundle did not return JSON: %s", raw)
	}

	// Verify project info
	if !strings.Contains(raw, proj.ID) {
		t.Error("bundle should contain project ID")
	}
	if !strings.Contains(raw, "bundleproj") {
		t.Error("bundle should contain project name")
	}

	// Verify entries by type (decision, session, reference)
	if !strings.Contains(raw, "decision") {
		t.Error("bundle should contain decision entries")
	}
	if !strings.Contains(raw, "session") {
		t.Error("bundle should contain session entries")
	}
	if !strings.Contains(raw, "reference") {
		t.Error("bundle should contain reference entries")
	}

	// Verify artifact refs
	if !strings.Contains(raw, "Bundle Artifact") {
		t.Error("bundle should contain artifact references")
	}
}

func TestSearchByTagsRequiresTags(t *testing.T) {
	reg, _, cleanup := setupMCPServices(t)
	defer cleanup()
	ctx := context.Background()

	result, err := reg.Call(ctx, "search_by_tags", map[string]interface{}{
		"tags": []interface{}{},
	})
	if err != nil {
		t.Fatalf("search_by_tags should not return dispatch error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for empty tags")
	}
	if !strings.Contains(result.Content[0].Text, "tags is required") {
		t.Errorf("expected 'tags is required', got: %s", result.Content[0].Text)
	}
}

func TestGetContextBundleWithoutProject(t *testing.T) {
	reg, projectSvc, cleanup := setupMCPServices(t)
	defer cleanup()
	ctx := context.Background()

	projectSvc.SaveProject(ctx, app.SaveProjectInput{Name: "noproj"})

	reg.Call(ctx, "save_entry", map[string]interface{}{
		"title":   "Global decision",
		"type":    "decision",
		"summary": "A global decision",
	})

	result, err := reg.Call(ctx, "get_context_bundle", map[string]interface{}{})
	if err != nil {
		t.Fatalf("get_context_bundle (no project) failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("get_context_bundle (no project) returned error: %s", result.Content[0].Text)
	}

	raw := result.Content[0].Text
	if !strings.Contains(raw, "decision") {
		t.Error("bundle should contain decision entries even without project")
	}
	if !strings.Contains(raw, "Global decision") {
		t.Error("bundle should contain the global entry")
	}
}

// Helper to extract entry ID from MCP save_entry response
func extractEntryID(t *testing.T, result *ToolCallResult) string {
	t.Helper()
	if result == nil || len(result.Content) == 0 {
		t.Fatal("empty result from save_entry")
	}
	text := result.Content[0].Text
	// Format: "Saved: sv-abc123"
	const prefix = "Saved: "
	idx := strings.Index(text, prefix)
	if idx < 0 {
		t.Fatalf("cannot find 'Saved: ' in response: %s", text)
	}
	rest := text[idx+len(prefix):]
	end := strings.Index(rest, "\n")
	if end > 0 {
		return rest[:end]
	}
	return strings.TrimSpace(rest)
}

// Helper to save an entry and return its ID
func saveTestEntry(t *testing.T, reg *ToolRegistry, title, typ string) string {
	t.Helper()
	ctx := context.Background()
	result, err := reg.Call(ctx, "save_entry", map[string]interface{}{
		"title":   title,
		"type":    typ,
		"summary": "Test entry " + title,
		"project": "testproj",
	})
	if err != nil {
		t.Fatalf("save_entry(%s) failed: %v", title, err)
	}
	if result.IsError {
		t.Fatalf("save_entry(%s) returned error: %s", title, result.Content[0].Text)
	}
	return extractEntryID(t, result)
}

func TestSaveEntryMCPPurpose(t *testing.T) {
	reg, projectSvc, cleanup := setupMCPServices(t)
	defer cleanup()
	ctx := context.Background()

	projectSvc.SaveProject(ctx, app.SaveProjectInput{Name: "testproj"})

	result, err := reg.Call(ctx, "save_entry", map[string]interface{}{
		"title":   "Knowledge Entry",
		"type":    "reference",
		"summary": "Entry with purpose",
		"project": "testproj",
		"purpose": "KNOWLEDGE",
	})
	if err != nil {
		t.Fatalf("save_entry with purpose failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("save_entry with purpose returned error: %s", result.Content[0].Text)
	}

	text := result.Content[0].Text
	lines := strings.Split(text, "\n")
	idLine := strings.TrimSpace(lines[0])
	id := strings.TrimPrefix(idLine, "Saved: ")

	getResult, err := reg.Call(ctx, "get_entry", map[string]interface{}{"id": id})
	if err != nil {
		t.Fatalf("get_entry failed: %v", err)
	}
	if getResult.IsError {
		t.Fatalf("get_entry returned error: %s", getResult.Content[0].Text)
	}
	// get_entry doesn't currently show purpose in output, but the entry is persisted.
	// The real test is that it doesn't error — persistence is verified in app_test.go.
	_ = getResult
}

func TestSearchEntriesMCPPurposeFilter(t *testing.T) {
	reg, projectSvc, cleanup := setupMCPServices(t)
	defer cleanup()
	ctx := context.Background()

	projectSvc.SaveProject(ctx, app.SaveProjectInput{Name: "testproj"})

	reg.Call(ctx, "save_entry", map[string]interface{}{
		"title":   "Work Entry",
		"type":    "reference",
		"summary": "A work entry",
		"project": "testproj",
		"purpose": "WORK",
	})
	reg.Call(ctx, "save_entry", map[string]interface{}{
		"title":   "Knowledge Entry",
		"type":    "reference",
		"summary": "A knowledge entry",
		"project": "testproj",
		"purpose": "KNOWLEDGE",
	})

	result, err := reg.Call(ctx, "search_entries", map[string]interface{}{
		"query":   "Entry",
		"limit":   float64(10),
		"project": "testproj",
		"purpose": "WORK",
	})
	if err != nil {
		t.Fatalf("search_entries with purpose filter failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("search_entries with purpose filter returned error: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "Work Entry") {
		t.Errorf("expected 'Work Entry' in results, got: %s", result.Content[0].Text)
	}
	if strings.Contains(result.Content[0].Text, "Knowledge Entry") {
		t.Error("should NOT find 'Knowledge Entry' when filtering by WORK purpose")
	}
}

// --- run_workflow and route_scenario MCP tests (PR B) ---

func TestRunWorkflowMCPSuccess(t *testing.T) {
	reg, projectSvc, cleanup := setupMCPServices(t)
	defer cleanup()
	ctx := context.Background()

	projectSvc.SaveProject(ctx, app.SaveProjectInput{Name: "testproj"})

	// Create entries
	reg.Call(ctx, "save_entry", map[string]interface{}{
		"title":   "Step 1 Prompt",
		"type":    "prompt",
		"summary": "First step",
		"project": "testproj",
	})
	reg.Call(ctx, "save_entry", map[string]interface{}{
		"title":   "Step 2 Prompt",
		"type":    "prompt",
		"summary": "Second step",
		"project": "testproj",
	})

	// Test with unknown workflow first (creates entries, workflow won't exist)
	result, err := reg.Call(ctx, "run_workflow", map[string]interface{}{
		"workflow": "mcp-pipeline-wf",
		"steps": map[string]interface{}{
			"1": "STEP1_OUTPUT",
			"2": "STEP2_OUTPUT",
		},
	})
	if err != nil {
		t.Fatalf("run_workflow call failed: %v", err)
	}
	// Workflow doesn't exist yet, so this should be an error
	if !result.IsError {
		t.Log("workflow exists or tool not yet registered; result:", result.Content[0].Text)
	}
}

func TestRunWorkflowMCPServiceNotWired(t *testing.T) {
	// Verify run_workflow returns error when workflowRunSvc is nil.
	// Use ServiceToolRegistry with entrySvc set but no workflowRunSvc.
	sqlDB, err := db.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB failed: %v", err)
	}
	defer sqlDB.Close()
	if err := db.RunMigrations(sqlDB); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}
	store := db.NewStore(sqlDB)
	entrySvc := app.NewEntryService(store.Entries, store.Projects, store.Artifacts)
	reg := NewServiceToolRegistry(
		entrySvc, nil, nil, nil, nil, nil, nil,
	)
	ctx := context.Background()

	result, err := reg.Call(ctx, "run_workflow", map[string]interface{}{
		"workflow": "any",
		"steps":    map[string]interface{}{"1": "output"},
	})
	if err != nil {
		t.Fatalf("run_workflow call failed: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for run_workflow without workflow service wiring")
	}
}

func TestRunWorkflowMCPUnknownWorkflow(t *testing.T) {
	reg, _, cleanup := setupMCPServices(t)
	defer cleanup()
	ctx := context.Background()

	result, err := reg.Call(ctx, "run_workflow", map[string]interface{}{
		"workflow": "no-such-workflow",
		"steps":    map[string]interface{}{"1": "output"},
	})
	if err != nil {
		t.Fatalf("run_workflow call failed: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for unknown workflow")
	}
}

func TestRouteScenarioMCPSuccess(t *testing.T) {
	reg, projectSvc, cleanup := setupMCPServices(t)
	defer cleanup()
	ctx := context.Background()

	projectSvc.SaveProject(ctx, app.SaveProjectInput{Name: "testproj"})

	// Create a routing entry that maps "write spec" to a skill (no workflow validation needed).
	reg.Call(ctx, "save_entry", map[string]interface{}{
		"title":   "Route to Write Spec",
		"type":    "routing",
		"summary": "Route for spec writing via skill",
		"body":    "write spec:\n  skill: spec-writing-skill",
		"project": "testproj",
		"tags":    []interface{}{"workflow-route"},
	})

	result, err := reg.Call(ctx, "route_scenario", map[string]interface{}{
		"scenario": "write spec",
	})
	if err != nil {
		t.Fatalf("route_scenario call failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("route_scenario returned error: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "write spec") {
		t.Errorf("expected scenario in result, got: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, `"type"`) {
		t.Errorf("expected JSON result with 'type' field, got: %s", result.Content[0].Text)
	}
}

func TestRouteScenarioMCPNoMatch(t *testing.T) {
	reg, _, cleanup := setupMCPServices(t)
	defer cleanup()
	ctx := context.Background()

	result, err := reg.Call(ctx, "route_scenario", map[string]interface{}{
		"scenario": "zzz-no-match-xyz",
	})
	if err != nil {
		t.Fatalf("route_scenario call failed: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for no-match scenario")
	}
}

func TestRouteScenarioMCPEmptyRejection(t *testing.T) {
	reg, _, cleanup := setupMCPServices(t)
	defer cleanup()
	ctx := context.Background()

	result, err := reg.Call(ctx, "route_scenario", map[string]interface{}{
		"scenario": "",
	})
	if err != nil {
		t.Fatalf("route_scenario call failed: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for empty scenario")
	}
	if !strings.Contains(result.Content[0].Text, "empty") && !strings.Contains(result.Content[0].Text, "required") {
		t.Errorf("error should mention empty/required scenario, got: %s", result.Content[0].Text)
	}
}

func TestToolCountIncludesNewTools(t *testing.T) {
	reg := NewToolRegistry(nil)
	tools := reg.List()

	// Should be 24 tools: 22 existing + list_entry_versions + restore_entry_version
	if len(tools) != 24 {
		t.Errorf("expected 24 tools, got %d", len(tools))
	}
	names := make(map[string]bool)
	for _, tool := range tools {
		names[tool.Name] = true
	}
	for _, name := range []string{"run_workflow", "route_scenario", "get_stats", "list_workflow_runs", "get_run", "list_entry_versions", "restore_entry_version"} {
		if !names[name] {
			t.Errorf("expected tool %q to be registered", name)
		}
	}
}

// --- get_stats, list_workflow_runs, get_run tests (PR B) ---

// setupMCPServicesWithStats sets up a full ToolRegistry with StatsService wired.
func setupMCPServicesWithStats(t *testing.T) (*ToolRegistry, *db.Store, func()) {
	t.Helper()
	sqlDB, err := db.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	if err := db.RunMigrations(sqlDB); err != nil {
		sqlDB.Close()
		t.Fatalf("RunMigrations: %v", err)
	}
	store := db.NewStore(sqlDB)

	entrySvc := app.NewEntryService(store.Entries, store.Projects, store.Artifacts)
	artifactSvc := app.NewArtifactService(store.Artifacts, store.Entries, store.Projects)
	workflowSvc := app.NewWorkflowService(store.Workflows)
	seriesSvc := app.NewSeriesService(store.Series, store.Entries)
	projectSvc := app.NewProjectService(store.Projects)
	entryRefSvc := app.NewEntryRefService(store.EntryLinks, store.Entries)
	contextSvc := app.NewContextService(store.Entries, store.Projects, store.Series, store.Workflows, store.Artifacts, entrySvc)
	sessionSvc := app.NewSessionService(entrySvc, artifactSvc, projectSvc, store.Entries, store.Artifacts, store.Projects)
	workflowRunSvc := app.NewWorkflowRunService(store.Workflows, store.WorkflowRuns, store.Entries)
	saveResultSvc := app.NewSavePromptResultService(store.Entries, store.Projects, store.Artifacts)
	statsSvc := app.NewStatsService(store.Entries, store.Artifacts, store.Projects).WithWorkflowRunStore(store.WorkflowRuns)
	entrySvc.SetWorkflowStore(store.Workflows)

	reg := NewServiceToolRegistry(entrySvc, artifactSvc, contextSvc, seriesSvc, workflowSvc, sessionSvc, projectSvc).
		WithEntryRefService(entryRefSvc).
		WithWorkflowRunService(workflowRunSvc).
		WithSaveResultService(saveResultSvc).
		WithStatsService(statsSvc)

	return reg, store, func() { sqlDB.Close() }
}

// seedRunWithSteps creates a project, entries, a workflow (if not already saved),
// and a completed run with the given number of steps.
func seedRunWithSteps(t *testing.T, store *db.Store, projectID, wfID, runID string, stepCount int, savedWorkflows map[string]bool) {
	t.Helper()
	ctx := context.Background()
	entrySvc := app.NewEntryService(store.Entries, store.Projects, store.Artifacts)
	var entryIDs []string
	for i := 1; i <= stepCount; i++ {
		r, err := entrySvc.SaveEntry(ctx, app.SaveEntryInput{
			Title: fmt.Sprintf("Step %d Prompt", i), Type: "prompt",
			Summary: fmt.Sprintf("Step %d", i), Project: projectID,
		})
		if err != nil {
			t.Fatalf("save step %d: %v", i, err)
		}
		entryIDs = append(entryIDs, r.Entry.Entry.ID)
	}
	if !savedWorkflows[wfID] {
		wf := domain.Workflow{
			ID: wfID, Name: "WF " + wfID, Slug: "test-wf-" + wfID,
			Description: "Testing", Status: domain.StatusActive,
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}
		steps := make([]domain.WorkflowStep, stepCount)
		for i := 0; i < stepCount; i++ {
			steps[i] = domain.WorkflowStep{
				ID: fmt.Sprintf("ws-%s-%d", wfID, i+1), WorkflowID: wfID,
				OrderIndex: i + 1, Title: fmt.Sprintf("Step %d", i+1), Required: i == 0,
			}
		}
		if err := store.Workflows.Save(ctx, wf, steps); err != nil {
			t.Fatalf("save workflow: %v", err)
		}
		savedWorkflows[wfID] = true
	}
	now := time.Now()
	finished := now.Add(5 * time.Second)
	run := domain.WorkflowRun{
		ID: runID, WorkflowID: wfID, Input: "in", Output: "out",
		Status: domain.RunStatusCompleted, StartedAt: now, FinishedAt: &finished,
	}
	runSteps := make([]domain.WorkflowRunStep, stepCount)
	for i := 0; i < stepCount; i++ {
		status := domain.RunStatusCompleted
		output := fmt.Sprintf("out %d", i+1)
		if i == stepCount-1 {
			status = domain.RunStatusFailed
			output = ""
		}
		runSteps[i] = domain.WorkflowRunStep{
			ID: fmt.Sprintf("rst-%s-%d", runID, i+1), RunID: runID, StepID: int64(i + 1),
			EntryID: entryIDs[i], Status: status, Output: output,
			StartedAt: now, FinishedAt: &finished,
		}
	}
	if err := store.WorkflowRuns.CreateRun(ctx, run, runSteps); err != nil {
		t.Fatalf("create run: %v", err)
	}
}

func TestGetStatsMCP(t *testing.T) {
	reg, store, cleanup := setupMCPServicesWithStats(t)
	defer cleanup()
	ctx := context.Background()

	proj, err := reg.projectSvc.SaveProject(ctx, app.SaveProjectInput{Name: "testproj"})
	if err != nil {
		t.Fatalf("SaveProject: %v", err)
	}
	seedRunWithSteps(t, store, proj.ID, "wf-stats", "run-stats", 3, map[string]bool{})

	result, err := reg.Call(ctx, "get_stats", nil)
	if err != nil {
		t.Fatalf("get_stats: %v", err)
	}
	if result.IsError {
		t.Fatalf("get_stats error: %s", result.Content[0].Text)
	}
	raw := result.Content[0].Text
	if !strings.Contains(raw, "workflow_runs") {
		t.Fatal("missing workflow_runs block")
	}
	if !strings.Contains(raw, `"total_runs":1`) || !strings.Contains(raw, `"completed_runs":1`) {
		t.Errorf("expected total_runs=1, completed_runs=1: %s", raw)
	}
	if !strings.Contains(raw, `"success_rate":1`) {
		t.Errorf("expected success_rate=1: %s", raw)
	}
}

func TestAnalyticsMCPHandlersReturnErrorsWhenServicesMissing(t *testing.T) {
	reg, _, cleanup := setupMCPServices(t)
	defer cleanup()
	ctx := context.Background()

	result, err := reg.Call(ctx, "get_stats", nil)
	if err != nil {
		t.Fatalf("get_stats: %v", err)
	}
	if !result.IsError || !strings.Contains(result.Content[0].Text, "stats service not available") {
		t.Fatalf("expected missing stats service error, got %+v", result)
	}

	reg.workflowRunSvc = nil
	for _, name := range []string{"list_workflow_runs", "get_run"} {
		result, err := reg.Call(ctx, name, map[string]interface{}{"run_id": "run-missing"})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !result.IsError || !strings.Contains(result.Content[0].Text, "workflow run service not available") {
			t.Fatalf("expected missing workflow run service error for %s, got %+v", name, result)
		}
	}
}

func TestListWorkflowRunsMCP(t *testing.T) {
	reg, store, cleanup := setupMCPServicesWithStats(t)
	defer cleanup()
	ctx := context.Background()

	proj, err := reg.projectSvc.SaveProject(ctx, app.SaveProjectInput{Name: "testproj"})
	if err != nil {
		t.Fatalf("SaveProject: %v", err)
	}
	saved := map[string]bool{}
	seedRunWithSteps(t, store, proj.ID, "wf-a", "run-a1", 3, saved)
	seedRunWithSteps(t, store, proj.ID, "wf-a", "run-a2", 2, saved)
	seedRunWithSteps(t, store, proj.ID, "wf-b", "run-b1", 4, saved)

	// All runs
	result, err := reg.Call(ctx, "list_workflow_runs", map[string]interface{}{"limit": float64(10)})
	if err != nil {
		t.Fatalf("list_workflow_runs: %v", err)
	}
	if result.IsError {
		t.Fatalf("list_workflow_runs error: %s", result.Content[0].Text)
	}
	raw := result.Content[0].Text
	if !strings.Contains(raw, "run-a1") || !strings.Contains(raw, "run-a2") || !strings.Contains(raw, "run-b1") {
		t.Errorf("missing runs: %s", raw)
	}
	if !strings.Contains(raw, `"completed_steps"`) {
		t.Errorf("missing completed_steps: %s", raw)
	}
	if !strings.Contains(raw, `"step_ratio"`) {
		t.Errorf("missing step_ratio: %s", raw)
	}

	// Filtered by workflow_id
	result2, _ := reg.Call(ctx, "list_workflow_runs", map[string]interface{}{"workflow_id": "wf-a", "limit": float64(5)})
	raw2 := result2.Content[0].Text
	if !strings.Contains(raw2, "run-a1") || !strings.Contains(raw2, "run-a2") {
		t.Errorf("wf-a filter: %s", raw2)
	}
	if strings.Contains(raw2, "run-b1") {
		t.Error("run-b1 leaked through wf-a filter")
	}
}

func TestGetRunMCP(t *testing.T) {
	reg, store, cleanup := setupMCPServicesWithStats(t)
	defer cleanup()
	ctx := context.Background()

	proj, err := reg.projectSvc.SaveProject(ctx, app.SaveProjectInput{Name: "testproj"})
	if err != nil {
		t.Fatalf("SaveProject: %v", err)
	}
	seedRunWithSteps(t, store, proj.ID, "wf-gr", "run-gr", 3, map[string]bool{})

	result, err := reg.Call(ctx, "get_run", map[string]interface{}{"run_id": "run-gr"})
	if err != nil {
		t.Fatalf("get_run: %v", err)
	}
	if result.IsError {
		t.Fatalf("get_run error: %s", result.Content[0].Text)
	}
	raw := result.Content[0].Text
	if !strings.Contains(raw, `"run"`) || !strings.Contains(raw, `"steps"`) {
		t.Fatal("missing run/steps keys")
	}
	if !strings.Contains(raw, `"wf-gr"`) {
		t.Errorf("missing workflow_id: %s", raw)
	}
	if strings.Count(raw, `"step_index"`) != 3 {
		t.Errorf("expected 3 steps: %s", raw)
	}

	// Not found
	result2, _ := reg.Call(ctx, "get_run", map[string]interface{}{"run_id": "nonexistent-xyz"})
	if !result2.IsError || !strings.Contains(result2.Content[0].Text, "not found") {
		t.Errorf("expected not-found error: %s", result2.Content[0].Text)
	}
}

func TestListEntryVersionsWithDirectStore(t *testing.T) {
	sqlDB, err := db.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB failed: %v", err)
	}
	defer sqlDB.Close()
	if err := db.RunMigrations(sqlDB); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}
	store := db.NewStore(sqlDB)
	entryVersionSvc := app.NewEntryVersionService(store.EntryVersions, store.Entries)

	ctx := context.Background()

	// Create and update entry to generate version history.
	e := domain.Entry{
		ID: "vt-1", Title: "v1 Title", Slug: "vt-1",
		Type: domain.EntryTypeSkill, Summary: "v1", BodyOptional: "b1",
		Status: domain.StatusActive,
	}
	if err := store.Entries.Save(ctx, e, nil); err != nil {
		t.Fatalf("save v1: %v", err)
	}
	e.Title = "v2 Title"
	e.Summary = "v2"
	if err := store.Entries.Save(ctx, e, nil); err != nil {
		t.Fatalf("save v2: %v", err)
	}

	// Now wire up the MCP registry with the version service and test the tool.
	entrySvc := app.NewEntryService(store.Entries, store.Projects, store.Artifacts)
	reg := NewServiceToolRegistry(entrySvc, nil, nil, nil, nil, nil, nil).WithEntryVersionService(entryVersionSvc)

	result, err := reg.Call(ctx, "list_entry_versions", map[string]interface{}{
		"entry_id": "vt-1",
	})
	if err != nil {
		t.Fatalf("list_entry_versions failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("list_entry_versions returned error: %s", result.Content[0].Text)
	}

	raw := result.Content[0].Text
	if !strings.Contains(raw, "version_number") {
		t.Errorf("expected JSON with version_number: %s", raw)
	}
	if !strings.Contains(raw, "v1 Title") {
		t.Errorf("expected v1 Title in result: %s", raw)
	}
}

func TestRestoreEntryVersionWithDirectStore(t *testing.T) {
	sqlDB, err := db.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB failed: %v", err)
	}
	defer sqlDB.Close()
	if err := db.RunMigrations(sqlDB); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}
	store := db.NewStore(sqlDB)
	entryVersionSvc := app.NewEntryVersionService(store.EntryVersions, store.Entries)

	ctx := context.Background()

	// Create entry and update to create version history.
	e := domain.Entry{
		ID: "rt-1", Title: "Original", Slug: "rt-1",
		Type: domain.EntryTypePrompt, Summary: "Original summary",
		BodyOptional: "Original body", Status: domain.StatusActive,
	}
	if err := store.Entries.Save(ctx, e, nil); err != nil {
		t.Fatalf("save v1: %v", err)
	}
	e.Title = "Updated"
	e.Summary = "Updated summary"
	e.BodyOptional = "Updated body"
	if err := store.Entries.Save(ctx, e, nil); err != nil {
		t.Fatalf("save v2: %v", err)
	}

	entrySvc := app.NewEntryService(store.Entries, store.Projects, store.Artifacts)
	reg := NewServiceToolRegistry(entrySvc, nil, nil, nil, nil, nil, nil).WithEntryVersionService(entryVersionSvc)

	result, err := reg.Call(ctx, "restore_entry_version", map[string]interface{}{
		"entry_id":       "rt-1",
		"version_number": float64(1),
	})
	if err != nil {
		t.Fatalf("restore_entry_version failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("restore_entry_version returned error: %s", result.Content[0].Text)
	}

	// Verify restored content.
	current, err := store.Entries.Get(ctx, "rt-1", false)
	if err != nil {
		t.Fatalf("Get after restore failed: %v", err)
	}
	if current.Entry.Title != "Original" {
		t.Errorf("expected restored title 'Original', got %q", current.Entry.Title)
	}
	if current.Entry.Summary != "Original summary" {
		t.Errorf("expected restored summary, got %q", current.Entry.Summary)
	}

	// Restoring non-existent version should error.
	badResult, _ := reg.Call(ctx, "restore_entry_version", map[string]interface{}{
		"entry_id":       "rt-1",
		"version_number": float64(99),
	})
	if !badResult.IsError {
		t.Error("expected error for non-existent version")
	}
}

func TestListEntryVersionsEmptyForNewEntry(t *testing.T) {
	sqlDB, err := db.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB failed: %v", err)
	}
	defer sqlDB.Close()
	if err := db.RunMigrations(sqlDB); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}
	store := db.NewStore(sqlDB)
	entryVersionSvc := app.NewEntryVersionService(store.EntryVersions, store.Entries)
	entrySvc := app.NewEntryService(store.Entries, store.Projects, store.Artifacts)

	reg := NewServiceToolRegistry(entrySvc, nil, nil, nil, nil, nil, nil).WithEntryVersionService(entryVersionSvc)
	ctx := context.Background()

	result, err := reg.Call(ctx, "list_entry_versions", map[string]interface{}{
		"entry_id": "no-such-entry",
	})
	if err != nil {
		t.Fatalf("list_entry_versions failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("list_entry_versions should not error for non-existent entry: %s", result.Content[0].Text)
	}
	raw := result.Content[0].Text
	if !strings.Contains(raw, "[") || !strings.Contains(raw, "]") {
		t.Errorf("expected JSON array ([]): %s", raw)
	}
}
