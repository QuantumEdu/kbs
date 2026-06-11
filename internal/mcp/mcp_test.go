package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/quantum-6/skillvault/internal/app"
	"github.com/quantum-6/skillvault/internal/db"
	"github.com/quantum-6/skillvault/internal/domain"

	_ "modernc.org/sqlite"
)

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

func TestToolsListReturns12Tools(t *testing.T) {
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
		if len(tools) != 12 {
			t.Errorf("expected 12 tools, got %d", len(tools))
		}
		return
	}
	if len(interfaces) != 12 {
		t.Errorf("expected 12 tools, got %d", len(interfaces))
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

func TestSavePromptResultMCPIntegration(t *testing.T) {
	t.Setenv("SKILLVAULT_DB", ":memory:")

	sqlDB, err := db.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB failed: %v", err)
	}
	defer sqlDB.Close()

	if err := db.RunMigrations(sqlDB); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	store := db.NewStore(sqlDB)
	entrySvc := app.NewEntryService(store.Entries)
	saveResultSvc := app.NewSavePromptResultService(entrySvc)

	// Create handler that maps save_prompt_result to the service
	handler := func(ctx context.Context, name string, args map[string]interface{}) (*ToolCallResult, error) {
		if name != "save_prompt_result" {
			return nil, nil
		}
		return handleSavePromptResult(ctx, saveResultSvc, args)
	}

	reg := NewToolRegistry(handler)

	// Call the tool
	args := map[string]interface{}{
		"name":     "Test Integration",
		"content":  "Integration test content",
		"type":     "prompt",
		"category": "testing",
		"tags":     []interface{}{"go", "test"},
		"model":    "claude-3",
	}

	result, err := reg.Call(context.Background(), "save_prompt_result", args)
	if err != nil {
		t.Fatalf("Tool call failed: %v", err)
	}

	if result.Content[0].Type != "text" {
		t.Errorf("content type = %q, want 'text'", result.Content[0].Type)
	}

	// Verify FTS5 discoverability
	searchResults, err := store.Search.SearchEntries(context.Background(), domain.SearchQuery{
		Query: "Integration",
	})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(searchResults) == 0 {
		t.Fatal("expected search to find the saved entry")
	}

	found := false
	for _, r := range searchResults {
		if r.Entry.Name == "Test Integration" {
			found = true
			if r.Entry.Type != domain.EntryTypePrompt {
				t.Errorf("Type = %q, want 'prompt'", r.Entry.Type)
			}
			if r.Entry.Content != "Integration test content" {
				t.Errorf("Content = %q, want 'Integration test content'", r.Entry.Content)
			}
			break
		}
	}
	if !found {
		t.Error("search results did not contain the saved entry")
	}
}

func handleSavePromptResult(ctx context.Context, svc *app.SavePromptResultService, args map[string]interface{}) (*ToolCallResult, error) {
	name, _ := args["name"].(string)
	content, _ := args["content"].(string)
	typ, _ := args["type"].(string)
	category, _ := args["category"].(string)
	projectID, _ := args["project_id"].(string)
	sourcePromptID, _ := args["source_prompt_id"].(string)
	model, _ := args["model"].(string)

	var tags []string
	if tagsRaw, ok := args["tags"].([]interface{}); ok {
		for _, t := range tagsRaw {
			if s, ok := t.(string); ok {
				tags = append(tags, s)
			}
		}
	}

	input := app.SavePromptResultInput{
		Name:           name,
		Content:        content,
		Type:           typ,
		Category:       category,
		Tags:           tags,
		ProjectID:      projectID,
		SourcePromptID: sourcePromptID,
		Model:          model,
	}

	output, err := svc.Save(ctx, input)
	if err != nil {
		return &ToolCallResult{
			Content: []ToolContent{{Type: "text", Text: "Error: " + err.Error()}},
			IsError: true,
		}, nil
	}

	resultText := "Saved: " + output.EntryID + "\n" +
		"  Name:    " + output.Name + "\n" +
		"  Type:    " + output.Type + "\n" +
		"  Project: " + output.ProjectID + "\n"

	return &ToolCallResult{
		Content: []ToolContent{{Type: "text", Text: resultText}},
	}, nil
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
