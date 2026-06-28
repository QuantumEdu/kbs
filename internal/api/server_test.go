package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/quantum-6/skillvault/internal/app"
	"github.com/quantum-6/skillvault/internal/db"
)

func setupTestServer(t *testing.T) (*Server, func()) {
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
	projectSvc := app.NewProjectService(store.Projects)
	contextSvc := app.NewContextService(store.Entries, store.Projects, store.Series, store.Workflows, store.Artifacts, entrySvc)
	sessionSvc := app.NewSessionService(entrySvc, artifactSvc, projectSvc, store.Entries, store.Artifacts, store.Projects)
	exportSvc := app.NewVaultExportService(store.ImportExport, store.Artifacts, store.Entries, store.Projects, store.Workflows)
	importSvc := app.NewVaultImportService(store.ImportExport, store.Entries, store.Projects, store.Artifacts)

	server := NewServer("127.0.0.1", 7438, entrySvc, artifactSvc, contextSvc, projectSvc, sessionSvc, workflowSvc, exportSvc, importSvc)

	cleanup := func() { sqlDB.Close() }
	return server, cleanup
}

func buildTestMux(srv *Server) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", srv.handleHealth)
	mux.HandleFunc("/entries", srv.handleEntries)
	mux.HandleFunc("/entries/", srv.handleEntryByID)
	mux.HandleFunc("/artifacts", srv.handleArtifacts)
	mux.HandleFunc("/context", srv.handleContext)
	mux.HandleFunc("/projects", srv.handleProjects)
	mux.HandleFunc("/sessions/wrap", srv.handleSessionWrap)
	mux.HandleFunc("/workflows", srv.handleWorkflows)
	mux.HandleFunc("/workflows/", srv.handleWorkflowByID)
	mux.HandleFunc("/export", srv.handleExport)
	mux.HandleFunc("/import", srv.handleImport)
	return mux
}

func doReq(mux *http.ServeMux, method, path string, body any) *httptest.ResponseRecorder {
	var reqBody io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reqBody = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestHealthEndpoint(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()
	mux := buildTestMux(srv)

	rec := doReq(mux, "GET", "/health", nil)
	if rec.Code != 200 {
		t.Errorf("health status = %d, want 200", rec.Code)
	}
	var healthResp map[string]string
	json.NewDecoder(rec.Body).Decode(&healthResp)
	if healthResp["status"] != "ok" {
		t.Errorf("health status = %q, want 'ok'", healthResp["status"])
	}
}

func TestPostEntriesValid(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()
	mux := buildTestMux(srv)

	// Create a project first
	doReq(mux, "POST", "/projects", map[string]string{
		"name":        "testproj",
		"description": "Test project",
	})

	rec := doReq(mux, "POST", "/entries", map[string]any{
		"title":   "Test Entry",
		"type":    "skill",
		"summary": "A test entry",
		"body":    "Entry body",
		"project": "testproj",
		"tags":    []string{"go", "api"},
		"status":  "active",
	})
	if rec.Code != 201 {
		t.Fatalf("POST /entries status = %d, want 201. Body: %s", rec.Code, rec.Body.String())
	}
	var result map[string]any
	json.NewDecoder(rec.Body).Decode(&result)
	entryMap, ok := result["Entry"].(map[string]any)
	if !ok {
		t.Fatalf("expected Entry in response, got %v", result)
	}
	inner, ok := entryMap["Entry"].(map[string]any)
	if !ok {
		t.Fatalf("expected Entry.Entry in response, got %v", entryMap)
	}
	if inner["Title"] != "Test Entry" {
		t.Errorf("Title = %v, want 'Test Entry'", inner["Title"])
	}
	if inner["ID"] == nil || inner["ID"] == "" {
		t.Error("expected non-empty ID")
	}
}

func TestPostEntriesMissingTitle(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()
	mux := buildTestMux(srv)

	rec := doReq(mux, "POST", "/entries", map[string]string{
		"type": "skill",
	})
	if rec.Code != 400 {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	var errResp map[string]string
	json.NewDecoder(rec.Body).Decode(&errResp)
	if !strings.Contains(errResp["error"], "title") {
		t.Errorf("error should mention title: got %v", errResp)
	}
}

func TestGetEntryExists(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()
	mux := buildTestMux(srv)

	// Create an entry first
	postRec := doReq(mux, "POST", "/entries", map[string]any{
		"title":   "Get Me",
		"type":    "skill",
		"summary": "Getting this entry",
	})
	var postResult map[string]any
	json.NewDecoder(postRec.Body).Decode(&postResult)
	entryMap := postResult["Entry"].(map[string]any)
	inner := entryMap["Entry"].(map[string]any)
	id := inner["ID"].(string)

	rec := doReq(mux, "GET", "/entries/"+id, nil)
	if rec.Code != 200 {
		t.Fatalf("GET /entries/{id} status = %d, want 200. Body: %s", rec.Code, rec.Body.String())
	}
	var result map[string]any
	json.NewDecoder(rec.Body).Decode(&result)
	gotEntry := result["Entry"].(map[string]any)
	gotInner := gotEntry["Entry"].(map[string]any)
	if gotInner["Title"] != "Get Me" {
		t.Errorf("Title = %v, want 'Get Me'", gotInner["Title"])
	}
}

func TestGetEntryNotFound(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()
	mux := buildTestMux(srv)

	rec := doReq(mux, "GET", "/entries/nonexistent-id", nil)
	if rec.Code != 404 {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestEntriesSearch(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()
	mux := buildTestMux(srv)

	doReq(mux, "POST", "/entries", map[string]any{
		"title":   "Searchable REST API",
		"type":    "skill",
		"summary": "Building REST APIs",
	})
	doReq(mux, "POST", "/entries", map[string]any{
		"title":   "Another Entry",
		"type":    "reference",
		"summary": "Some reference",
	})

	rec := doReq(mux, "GET", "/entries?q=REST", nil)
	if rec.Code != 200 {
		t.Fatalf("search status = %d, want 200. Body: %s", rec.Code, rec.Body.String())
	}
	var results []any
	json.NewDecoder(rec.Body).Decode(&results)
	if len(results) == 0 {
		t.Error("expected at least 1 search result")
	}
}

func TestPostProjects(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()
	mux := buildTestMux(srv)

	rec := doReq(mux, "POST", "/projects", map[string]string{
		"name":        "new-project",
		"description": "A brand new project",
	})
	if rec.Code != 201 {
		t.Fatalf("POST /projects status = %d, want 201. Body: %s", rec.Code, rec.Body.String())
	}
	var result map[string]any
	json.NewDecoder(rec.Body).Decode(&result)
	if result["Name"] != "new-project" {
		t.Errorf("Name = %v, want 'new-project'", result["Name"])
	}
	if result["ID"] == nil || result["ID"] == "" {
		t.Error("expected non-empty ID")
	}
}

func TestListProjects(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()
	mux := buildTestMux(srv)

	doReq(mux, "POST", "/projects", map[string]string{"name": "Alpha"})
	doReq(mux, "POST", "/projects", map[string]string{"name": "Beta"})

	rec := doReq(mux, "GET", "/projects", nil)
	if rec.Code != 200 {
		t.Fatalf("GET /projects status = %d, want 200. Body: %s", rec.Code, rec.Body.String())
	}
	var results []any
	json.NewDecoder(rec.Body).Decode(&results)
	if len(results) != 2 {
		t.Errorf("expected 2 projects, got %d", len(results))
	}
}

func TestDeleteEntryArchive(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()
	mux := buildTestMux(srv)

	// Create entry
	postRec := doReq(mux, "POST", "/entries", map[string]any{
		"title":   "To Archive",
		"type":    "skill",
		"summary": "Will be archived",
	})
	var postResult map[string]any
	json.NewDecoder(postRec.Body).Decode(&postResult)
	entryMap := postResult["Entry"].(map[string]any)
	inner := entryMap["Entry"].(map[string]any)
	id := inner["ID"].(string)

	rec := doReq(mux, "DELETE", "/entries/"+id, nil)
	if rec.Code != 200 {
		t.Fatalf("DELETE /entries/{id} status = %d, want 200. Body: %s", rec.Code, rec.Body.String())
	}
	var delResult map[string]string
	json.NewDecoder(rec.Body).Decode(&delResult)
	if delResult["status"] != "archived" {
		t.Errorf("status = %q, want 'archived'", delResult["status"])
	}

	// Verify archived entry returns 404
	rec2 := doReq(mux, "GET", "/entries/"+id, nil)
	if rec2.Code != 404 {
		t.Errorf("GET archived entry status = %d, want 404", rec2.Code)
	}
}

func TestPostArtifacts(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()
	mux := buildTestMux(srv)

	ctx := context.Background()
	srv.projectSvc.SaveProject(ctx, app.SaveProjectInput{Name: "art-proj"})

	rec := doReq(mux, "POST", "/artifacts", map[string]any{
		"title":   "Test Artifact",
		"type":    "markdown",
		"content": "# Hello",
		"summary": "A markdown artifact",
		"project": "art-proj",
	})
	if rec.Code != 201 {
		t.Fatalf("POST /artifacts status = %d, want 201. Body: %s", rec.Code, rec.Body.String())
	}
	var result map[string]any
	json.NewDecoder(rec.Body).Decode(&result)
	if result["Title"] != "Test Artifact" {
		t.Errorf("Title = %v, want 'Test Artifact'", result["Title"])
	}
}

func TestPostContext(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()
	mux := buildTestMux(srv)

	ctx := context.Background()
	proj, _ := srv.projectSvc.SaveProject(ctx, app.SaveProjectInput{Name: "ctx-proj", Description: "Context project"})
	srv.entrySvc.SaveEntry(ctx, app.SaveEntryInput{
		Title: "Use Go", Type: "decision", Summary: "Use Go for backend", Project: proj.ID, Status: "active",
	})

	rec := doReq(mux, "POST", "/context", map[string]any{
		"mode":      "planning",
		"project":   proj.ID,
		"max_chars": 5000,
	})
	if rec.Code != 200 {
		t.Fatalf("POST /context status = %d, want 200. Body: %s", rec.Code, rec.Body.String())
	}
	var result map[string]any
	json.NewDecoder(rec.Body).Decode(&result)
	if result["Raw"] == nil {
		t.Error("expected Raw field in context response")
	}
	raw := result["Raw"].(string)
	if !strings.Contains(raw, "CONTEXT PACK") {
		t.Error("expected CONTEXT PACK in response")
	}
	if !strings.Contains(raw, "Use Go") {
		t.Error("expected 'Use Go' decision in context pack")
	}
}

func TestSessionWrap(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()
	mux := buildTestMux(srv)

	ctx := context.Background()
	srv.projectSvc.SaveProject(ctx, app.SaveProjectInput{Name: "ses-proj"})

	rec := doReq(mux, "POST", "/sessions/wrap", map[string]any{
		"project":   "ses-proj",
		"summary":   "Fixed auth bug",
		"decisions": []string{"Use JWT"},
		"pending":   []string{"Add tests"},
		"learnings": []string{"JWT expiry short"},
	})
	if rec.Code != 201 {
		t.Fatalf("POST /sessions/wrap status = %d, want 201. Body: %s", rec.Code, rec.Body.String())
	}
	var result map[string]any
	json.NewDecoder(rec.Body).Decode(&result)
	if result["Entry"] == nil {
		t.Error("expected Entry in session wrap response")
	}
}

func TestWorkflows(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()
	mux := buildTestMux(srv)

	wfBody := map[string]any{
		"name":        "Review Flow",
		"description": "Code review steps",
		"status":      "active",
		"steps": []map[string]any{
			{"Title": "Check style", "Instruction": "Run linter", "Required": true},
			{"Title": "Run tests", "Instruction": "go test ./...", "Required": true},
		},
	}
	rec := doReq(mux, "POST", "/workflows", wfBody)
	if rec.Code != 201 {
		t.Fatalf("POST /workflows status = %d, want 201. Body: %s", rec.Code, rec.Body.String())
	}
	var result map[string]any
	json.NewDecoder(rec.Body).Decode(&result)
	wfID := result["ID"].(string)
	if wfID == "" {
		t.Fatal("expected workflow ID")
	}

	rec2 := doReq(mux, "GET", "/workflows/"+wfID, nil)
	if rec2.Code != 200 {
		t.Fatalf("GET /workflows/{id} status = %d, want 200. Body: %s", rec2.Code, rec2.Body.String())
	}
	var steps []any
	json.NewDecoder(rec2.Body).Decode(&steps)
	if len(steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(steps))
	}
}

func TestExportImport(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()
	mux := buildTestMux(srv)

	tmpDir := t.TempDir()
	exportPath := filepath.Join(tmpDir, "export.json")

	// Export
	rec := doReq(mux, "POST", "/export", map[string]string{
		"path": exportPath,
	})
	if rec.Code != 200 {
		t.Fatalf("POST /export status = %d, want 200. Body: %s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(exportPath); os.IsNotExist(err) {
		t.Fatal("export file not created")
	}

	// Import
	rec2 := doReq(mux, "POST", "/import", map[string]string{
		"path": exportPath,
	})
	if rec2.Code != 200 {
		t.Fatalf("POST /import status = %d, want 200. Body: %s", rec2.Code, rec2.Body.String())
	}
	var impResult map[string]string
	json.NewDecoder(rec2.Body).Decode(&impResult)
	if impResult["status"] != "imported" {
		t.Errorf("status = %q, want 'imported'", impResult["status"])
	}
}

func TestMethodNotAllowed(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()
	mux := buildTestMux(srv)

	rec := doReq(mux, "POST", "/health", nil)
	if rec.Code != 405 {
		t.Errorf("POST /health status = %d, want 405", rec.Code)
	}
}

func TestServerConstruction(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	if srv == nil {
		t.Fatal("NewServer returned nil")
	}
	if srv.host != "127.0.0.1" {
		t.Errorf("host = %q, want '127.0.0.1'", srv.host)
	}
	if srv.port != 7438 {
		t.Errorf("port = %d, want 7438", srv.port)
	}
}

func TestEntriesSearchWithFilters(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()
	mux := buildTestMux(srv)

	ctx := context.Background()
	srv.projectSvc.SaveProject(ctx, app.SaveProjectInput{Name: "proj-a"})
	pb, _ := srv.projectSvc.SaveProject(ctx, app.SaveProjectInput{Name: "proj-b"})

	srv.entrySvc.SaveEntry(ctx, app.SaveEntryInput{
		Title: "Skill Entry", Type: "skill", Summary: "A skill", Project: "proj-a", Status: "active",
	})
	srv.entrySvc.SaveEntry(ctx, app.SaveEntryInput{
		Title: "Reference Entry", Type: "reference", Summary: "A ref", Project: "proj-b", Status: "active",
	})

	// Search by type with query
	rec := doReq(mux, "GET", "/entries?q=Entry&type=skill&limit=10", nil)
	if rec.Code != 200 {
		t.Fatalf("search by type status = %d, want 200", rec.Code)
	}
	var results []any
	json.NewDecoder(rec.Body).Decode(&results)
	found := false
	for _, r := range results {
		rm := r.(map[string]any)
		entry := rm["Entry"].(map[string]any)
		if entry["Title"] == "Skill Entry" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find 'Skill Entry' when filtering by type=skill")
	}

	// Search by project with query
	rec = doReq(mux, "GET", "/entries?q=Entry&project="+pb.ID+"&limit=10", nil)
	if rec.Code != 200 {
		t.Fatalf("search by project status = %d, want 200", rec.Code)
	}
	json.NewDecoder(rec.Body).Decode(&results)
	found = false
	for _, r := range results {
		rm := r.(map[string]any)
		entry := rm["Entry"].(map[string]any)
		if entry["Title"] == "Reference Entry" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find 'Reference Entry' when filtering by project")
	}
}

func TestWorkflowRenderNotFound(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()
	mux := buildTestMux(srv)

	rec := doReq(mux, "GET", "/workflows/nonexistent", nil)
	if rec.Code != 404 {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestServerGracefulShutdown(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	// Create a real HTTP server on a random port with a slow handler
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	mux := http.NewServeMux()
	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		// Simulate work that takes time — Shutdown must drain this
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(200)
		w.Write([]byte("pong"))
	})

	srv.srv = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Start server in background
	serveDone := make(chan struct{})
	go func() {
		defer close(serveDone)
		_ = srv.srv.Serve(listener)
	}()

	// Wait for server to be ready
	baseURL := fmt.Sprintf("http://%s", addr)
	client := &http.Client{Timeout: 5 * time.Second}
	var ok bool
	for i := 0; i < 20; i++ {
		resp, err := client.Get(baseURL + "/ping")
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			ok = true
			break
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !ok {
		t.Fatal("server did not become ready")
	}

	// Send a request that's still in-flight when Stop() is called
	var reqSucceeded bool
	reqDone := make(chan struct{})
	go func() {
		defer close(reqDone)
		resp, err := client.Get(baseURL + "/ping")
		if err != nil {
			t.Logf("in-flight request error: %v", err)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode == 200 {
			reqSucceeded = true
		}
	}()

	// Give the goroutine time to start the request
	time.Sleep(50 * time.Millisecond)

	// Graceful shutdown via Stop() — must drain in-flight request
	if err := srv.Stop(); err != nil {
		t.Fatalf("Stop() failed: %v", err)
	}

	// Wait for Serve to return
	select {
	case <-serveDone:
	case <-time.After(2 * time.Second):
		t.Fatal("Serve did not return after Stop()")
	}

	// In-flight request MUST complete (drained, not killed)
	select {
	case <-reqDone:
	case <-time.After(1 * time.Second):
		t.Error("in-flight request was not drained by Stop()")
	}
	if !reqSucceeded {
		t.Error("Stop() must drain in-flight requests; use Shutdown(ctx) not Close()")
	}

	// Verify server is no longer accepting connections
	_, err = client.Get(baseURL + "/ping")
	if err == nil {
		t.Error("expected connection to be refused after shutdown")
	}
}

func TestAuthSkippedWhenNoKey(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()
	ts := httptest.NewServer(srv.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))
	defer ts.Close()

	// POST without any auth header should work when no key configured.
	resp, err := http.Post(ts.URL, "application/json", bytes.NewReader([]byte(`{}`)))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 without auth when key unset, got %d", resp.StatusCode)
	}
}

func TestAuthRequiredWhenKeySet(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()
	srv.WithAPIKey("test-key-123")
	ts := httptest.NewServer(srv.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))
	defer ts.Close()

	// POST without key → 401.
	resp, err := http.Post(ts.URL, "application/json", bytes.NewReader([]byte(`{}`)))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 without key, got %d", resp.StatusCode)
	}

	// POST with wrong key → 401.
	req, _ := http.NewRequest(http.MethodPost, ts.URL, bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Authorization", "Bearer wrong-key")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 with wrong key, got %d", resp.StatusCode)
	}

	// POST with correct key → 200.
	req, _ = http.NewRequest(http.MethodPost, ts.URL, bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Authorization", "Bearer test-key-123")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 with correct key, got %d", resp.StatusCode)
	}
}

func TestHealthAlwaysOpen(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()
	srv.WithAPIKey("secret")
	ts := httptest.NewServer(srv.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))
	defer ts.Close()

	// GET /health should always work regardless of auth.
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/health", nil)
	req.Header.Set("Authorization", "Bearer wrong-key")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for /health even with wrong key, got %d", resp.StatusCode)
	}
}
