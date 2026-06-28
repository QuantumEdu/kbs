# Technical Design — service-hardening

## Overview

Small polish changes across the service surface. No new packages or schema changes.

---

## 1. save_result MCP Tool

### Approach
Add `save_result` to `registerV2Tools()` in `internal/mcp/tools.go` following the exact same pattern as existing tools.

### Implementation
```go
// In registerV2Tools:
tools = append(tools, ToolDefinition{
    Name:        "save_result",
    Description: "Save an AI prompt result as an entry",
    InputSchema: json.RawMessage(`{
        "type": "object",
        "properties": {
            "name":        {"type": "string"},
            "content":     {"type": "string"},
            "type":        {"type": "string"},
            "category":    {"type": "string"},
            "tags":        {"type": "array", "items": {"type": "string"}},
            "project_id":  {"type": "string"},
            "source_prompt_id": {"type": "string"},
            "model":       {"type": "string"}
        },
        "required": ["name", "content"]
    }`),
})
```

Handler calls `svc.saveResultSvc.Save(ctx, input)`.
Add `saveResultSvc *app.SavePromptResultService` field and `WithSaveResultService` builder.

### Wiring
In `cmd/skillvault/main.go`, pass `svc.saveResultSvc` to tool registry.

---

## 2. HTTP Auth Layer

### Approach
- Add `apiKey string` field to `Server`.
- Add `WithAPIKey(key string)` builder.
- In `Start()`, wrap mux with auth middleware.
- `/health` passes through; all other endpoints check Bearer token when key is set.

### Auth Middleware
```go
func (s *Server) authMiddleware(next http.Handler) http.Handler {
    if s.apiKey == "" {
        return next  // no-op when no key configured
    }
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Method == http.MethodGet {
            next.ServeHTTP(w, r)
            return
        }
        auth := r.Header.Get("Authorization")
        if !strings.HasPrefix(auth, "Bearer ") || strings.TrimPrefix(auth, "Bearer ") != s.apiKey {
            writeError(w, http.StatusUnauthorized, "invalid or missing API key")
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

### CLI Flag
- Add `apiKey string` to `http` command flags.
- Parse `--api-key` in `ParseCommand` or inline in `main.go`.

---

## 3. Graceful Shutdown

### HTTP Server
Move `Start()` to use `signal.NotifyContext` internally and call `s.srv.Shutdown()`:

```go
func (s *Server) Start() error {
    // ... setup mux and s.srv ...
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()
    
    go func() {
        <-ctx.Done()
        s.Stop()
    }()
    
    if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        return err
    }
    return nil
}
```

### MCP Server
Already has graceful shutdown via `signal.NotifyContext` in `runMCP()`.

---

## 4. docs/vars.md

Create `docs/vars.md` explaining:
- Frontmatter `---` blocks for metadata
- `{{variable}}` syntax for template injection
- `--vars` flag usage
- `-i` flag for in-place replace
- Example flows

### 5. docs/commands.md

Update command list to 21. Add new commands and their flags.
