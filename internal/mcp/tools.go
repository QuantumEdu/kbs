package mcp

import (
	"context"
	"encoding/json"
	"fmt"
)

// ToolRegistry manages MCP tools and dispatches calls.
type ToolRegistry struct {
	tools   []Tool
	handler ToolHandler
}

// ToolHandler is called when a tool is invoked.
type ToolHandler func(ctx context.Context, toolName string, args map[string]interface{}) (*ToolCallResult, error)

// NewToolRegistry creates a new tool registry with the 11 alpha tools.
func NewToolRegistry(handler ToolHandler) *ToolRegistry {
	reg := &ToolRegistry{
		handler: handler,
	}
	reg.registerAlphaTools()
	return reg
}

// registerAlphaTools registers the 12 v1-alpha tools.
func (r *ToolRegistry) registerAlphaTools() {
	r.tools = []Tool{
		{Name: "get_entry", Description: "Get a vault entry by ID", InputSchema: schemaObj(map[string]interface{}{
			"id": map[string]interface{}{"type": "string", "description": "Entry ID"},
		})},
		{Name: "search_entries", Description: "Search entries with FTS5", InputSchema: schemaObj(map[string]interface{}{
			"query":            map[string]interface{}{"type": "string", "description": "Search query"},
			"project_id":       map[string]interface{}{"type": "string", "description": "Filter by project ID"},
			"type":             map[string]interface{}{"type": "string", "description": "Filter by entry type"},
		})},
		{Name: "list_entries", Description: "List entries with optional filters", InputSchema: schemaObj(map[string]interface{}{
			"project_id": map[string]interface{}{"type": "string", "description": "Filter by project ID"},
			"type":       map[string]interface{}{"type": "string", "description": "Filter by type"},
		})},
		{Name: "upsert_entry", Description: "Create or update an entry", InputSchema: schemaObj(map[string]interface{}{
			"id":      map[string]interface{}{"type": "string", "description": "Entry ID"},
			"name":    map[string]interface{}{"type": "string", "description": "Entry name"},
			"type":    map[string]interface{}{"type": "string", "description": "Entry type (skill|agent|workflow|prompt|context|note)"},
			"content": map[string]interface{}{"type": "string", "description": "Entry content"},
		})},
		{Name: "archive_entry", Description: "Soft-delete an entry", InputSchema: schemaObj(map[string]interface{}{
			"id": map[string]interface{}{"type": "string", "description": "Entry ID"},
		})},
		{Name: "get_series", Description: "Get a series with entries", InputSchema: schemaObj(map[string]interface{}{
			"id": map[string]interface{}{"type": "string", "description": "Series ID"},
		})},
		{Name: "list_series", Description: "List series", InputSchema: schemaObj(map[string]interface{}{
			"project_id": map[string]interface{}{"type": "string", "description": "Filter by project ID"},
		})},
		{Name: "upsert_series", Description: "Create or update a series", InputSchema: schemaObj(map[string]interface{}{
			"id":   map[string]interface{}{"type": "string", "description": "Series ID"},
			"name": map[string]interface{}{"type": "string", "description": "Series name"},
		})},
		{Name: "replace_series_entries", Description: "Replace all entries in a series", InputSchema: schemaObj(map[string]interface{}{
			"series_id": map[string]interface{}{"type": "string", "description": "Series ID"},
			"entries":   map[string]interface{}{"type": "array", "description": "Array of entry objects"},
		})},
		{Name: "get_context", Description: "Get project context (entries + series)", InputSchema: schemaObj(map[string]interface{}{
			"project_id": map[string]interface{}{"type": "string", "description": "Project ID"},
		})},
		{Name: "run_workflow", Description: "Run a workflow (render variables)", InputSchema: schemaObj(map[string]interface{}{
			"id":   map[string]interface{}{"type": "string", "description": "Workflow entry ID"},
			"vars": map[string]interface{}{"type": "object", "description": "Variable values"},
		})},
		{Name: "save_prompt_result", Description: "Save an LLM prompt execution result as a structured entry. Client executes the prompt; SkillVault stores the output. Defaults type to 'note'. Requires name and content.", InputSchema: schemaObj(map[string]interface{}{
			"name":             map[string]interface{}{"type": "string", "description": "Result name (required)"},
			"content":          map[string]interface{}{"type": "string", "description": "Result content (required)"},
			"type":             map[string]interface{}{"type": "string", "description": "Entry type (skill|agent|workflow|prompt|context|note)"},
			"category":         map[string]interface{}{"type": "string", "description": "Classification label"},
			"tags":             map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
			"project_id":       map[string]interface{}{"type": "string", "description": "Target project ID"},
			"source_prompt_id": map[string]interface{}{"type": "string", "description": "ID of the prompt entry used"},
			"model":            map[string]interface{}{"type": "string", "description": "LLM model identifier"},
		})},
	}
}

// List returns all registered tools.
func (r *ToolRegistry) List() []Tool {
	return r.tools
}

// Call dispatches a tool call to the handler.
func (r *ToolRegistry) Call(ctx context.Context, name string, args map[string]interface{}) (*ToolCallResult, error) {
	if r.handler == nil {
		return nil, fmt.Errorf("no tool handler registered")
	}
	return r.handler(ctx, name, args)
}

func schemaObj(props map[string]interface{}) json.RawMessage {
	schema := map[string]interface{}{
		"type":       "object",
		"properties": props,
	}
	data, _ := json.Marshal(schema)
	return data
}
