package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/quantum-6/skillvault/internal/app"
	"github.com/quantum-6/skillvault/internal/db"

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
	contextSvc := app.NewContextService(store.Entries, store.Projects, store.Series, store.Workflows, store.Artifacts, entrySvc)
	sessionSvc := app.NewSessionService(entrySvc, artifactSvc, projectSvc, store.Entries, store.Artifacts, store.Projects)

	reg := NewServiceToolRegistry(entrySvc, artifactSvc, contextSvc, seriesSvc, workflowSvc, sessionSvc, projectSvc)
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

func TestToolsListReturns10Tools(t *testing.T) {
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
	interfaces, ok := toolsRaw.([]interface{})
	if !ok {
		tools, ok := toolsRaw.([]Tool)
		if !ok {
			t.Fatalf("tools is not an array: %T", toolsRaw)
		}
		if len(tools) != 10 {
			t.Errorf("expected 10 tools, got %d", len(tools))
		}
		return
	}
	if len(interfaces) != 10 {
		t.Errorf("expected 10 tools, got %d", len(interfaces))
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
		"query":  "REST",
		"limit":  float64(10),
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
