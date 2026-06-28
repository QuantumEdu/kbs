package app

import (
	"context"
	"fmt"
	"io"
	"sync"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/quantum-6/skillvault/internal/db"
)

// mockTransport is an in-memory Transport implementation for testing.
// It stores pushed data in a map and returns it on pull.
type mockTransport struct {
	mu   sync.Mutex
	data map[string][]byte
}

func newMockTransport() *mockTransport {
	return &mockTransport{data: make(map[string][]byte)}
}

func (m *mockTransport) Push(ctx context.Context, reader io.Reader, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Store the raw bytes (Test should verify transport receives data)
	b, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("mock push read: %w", err)
	}
	m.data[key] = b
	return nil
}

func (m *mockTransport) Pull(ctx context.Context, writer io.Writer, key string) error {
	m.mu.Lock()
	b, ok := m.data[key]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("mock: key %q not found", key)
	}
	_, err := writer.Write(b)
	return err
}

// errorTransport always returns an error.
type errorTransport struct{}

func (e *errorTransport) Push(ctx context.Context, reader io.Reader, key string) error {
	return fmt.Errorf("push failed")
}

func (e *errorTransport) Pull(ctx context.Context, writer io.Writer, key string) error {
	return fmt.Errorf("pull failed")
}

// setupSyncTest creates an in-memory SQLite vault and a SyncService backed by a mock transport.
func setupSyncTest(t *testing.T) (*SyncService, *mockTransport, *db.Store, func()) {
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

	exportSvc := NewVaultExportService(store.ImportExport, store.Artifacts, store.Entries, store.Projects, store.Workflows)
	importSvc := NewVaultImportService(store.ImportExport, store.Entries, store.Projects, store.Artifacts)

	mock := newMockTransport()
	syncSvc := NewSyncService(exportSvc, importSvc, mock)

	cleanup := func() { sqlDB.Close() }
	return syncSvc, mock, store, cleanup
}

func TestSyncServicePush(t *testing.T) {
	svc, mock, store, cleanup := setupSyncTest(t)
	defer cleanup()
	ctx := context.Background()

	// Add a project and an entry so the vault has data.
	projectSvc := NewProjectService(store.Projects)
	projectSvc.SaveProject(ctx, SaveProjectInput{Name: "testproj"})

	entrySvc := NewEntryService(store.Entries, store.Projects, store.Artifacts)
	result, err := entrySvc.SaveEntry(ctx, SaveEntryInput{
		Title:   "Test Entry",
		Type:    "skill",
		Summary: "Push round-trip test",
		Project: "testproj",
	})
	if err != nil {
		t.Fatalf("SaveEntry failed: %v", err)
	}

	// Push the vault snapshot.
	remotePath := "vault-snapshot.json.gz"
	err = svc.Push(ctx, remotePath, false)
	if err != nil {
		t.Fatalf("Push failed: %v", err)
	}

	// Verify the mock transport received data.
	mock.mu.Lock()
	data, ok := mock.data[remotePath]
	mock.mu.Unlock()
	if !ok {
		t.Fatalf("mock transport has no data for key %q", remotePath)
	}
	if len(data) == 0 {
		t.Error("pushed data should not be empty")
	}

	// Verify the entry exists after push (vault unchanged by push).
	got, err := entrySvc.GetEntry(ctx, result.Entry.Entry.ID)
	if err != nil {
		t.Fatalf("GetEntry after push failed: %v", err)
	}
	if got.Entry.Entry.Title != "Test Entry" {
		t.Errorf("Title = %q, want 'Test Entry'", got.Entry.Entry.Title)
	}
}

func TestSyncServicePull(t *testing.T) {
	svc, mock, store, cleanup := setupSyncTest(t)
	defer cleanup()
	ctx := context.Background()

	// Set up some data in the export service's vault.
	projectSvc := NewProjectService(store.Projects)
	projectSvc.SaveProject(ctx, SaveProjectInput{Name: "testproj"})

	entrySvc := NewEntryService(store.Entries, store.Projects, store.Artifacts)
	result, err := entrySvc.SaveEntry(ctx, SaveEntryInput{
		Title:   "Original Entry",
		Type:    "skill",
		Summary: "Will be pushed then pulled to a fresh vault",
		Project: "testproj",
	})
	if err != nil {
		t.Fatalf("SaveEntry failed: %v", err)
	}

	// Push to populate the mock transport.
	remotePath := "snapshot.gz"
	if err := svc.Push(ctx, remotePath, false); err != nil {
		t.Fatalf("Push failed: %v", err)
	}

	// Create a fresh vault (second DB) for the pull.
	sqlDB2, err := db.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB2 failed: %v", err)
	}
	defer sqlDB2.Close()
	if err := db.RunMigrations(sqlDB2); err != nil {
		t.Fatalf("RunMigrations2 failed: %v", err)
	}
	store2 := db.NewStore(sqlDB2)
	importSvc2 := NewVaultImportService(store2.ImportExport, store2.Entries, store2.Projects, store2.Artifacts)

	// Create a SyncService for the fresh vault, using the SAME mock transport.
	syncSvc2 := NewSyncService(nil, importSvc2, mock) // exportSvc is nil — only pull matters.

	// Pull into the fresh vault.
	if err := syncSvc2.Pull(ctx, remotePath, false); err != nil {
		t.Fatalf("Pull failed: %v", err)
	}

	// Verify the entry was imported into the fresh vault.
	entrySvc2 := NewEntryService(store2.Entries, store2.Projects, store2.Artifacts)
	got, err := entrySvc2.GetEntry(ctx, result.Entry.Entry.ID)
	if err != nil {
		t.Fatalf("GetEntry after pull failed: %v", err)
	}
	if got.Entry.Entry.Title != "Original Entry" {
		t.Errorf("Title = %q, want 'Original Entry'", got.Entry.Entry.Title)
	}
	if got.Entry.Entry.Summary != "Will be pushed then pulled to a fresh vault" {
		t.Errorf("Summary = %q, want full summary", got.Entry.Entry.Summary)
	}
}

func TestSyncServicePushDryRun(t *testing.T) {
	svc, mock, store, cleanup := setupSyncTest(t)
	defer cleanup()
	ctx := context.Background()

	projectSvc := NewProjectService(store.Projects)
	projectSvc.SaveProject(ctx, SaveProjectInput{Name: "testproj"})

	entrySvc := NewEntryService(store.Entries, store.Projects, store.Artifacts)
	entrySvc.SaveEntry(ctx, SaveEntryInput{
		Title:   "Dry Run Entry",
		Type:    "skill",
		Summary: "Should not be transferred",
		Project: "testproj",
	})

	remotePath := "snapshot.gz"

	// Dry-run push should NOT transfer to mock.
	err := svc.Push(ctx, remotePath, true)
	if err != nil {
		t.Fatalf("Push dry-run failed: %v", err)
	}

	// Verify mock transport has NO data (transfer was skipped).
	mock.mu.Lock()
	_, exists := mock.data[remotePath]
	mock.mu.Unlock()
	if exists {
		t.Error("mock transport should have no data after dry-run push")
	}
}

func TestSyncServicePullDryRun(t *testing.T) {
	svc, mock, store, cleanup := setupSyncTest(t)
	defer cleanup()
	ctx := context.Background()

	// Pre-populate mock with some compressed-like data.
	mock.mu.Lock()
	mock.data["snapshot.gz"] = []byte("compressed-data")
	mock.mu.Unlock()

	// Dry-run pull should NOT call ImportVault.
	err := svc.Pull(ctx, "snapshot.gz", true)
	if err != nil {
		t.Fatalf("Pull dry-run failed: %v", err)
	}

	// Verify no entry was imported (vault is unchanged).
	entrySvc := NewEntryService(store.Entries, store.Projects, store.Artifacts)
	_, err = entrySvc.GetEntry(ctx, "anything")
	if err == nil {
		t.Error("dry-run pull should not import any entries")
	}
}

func TestSyncServicePushTransportError(t *testing.T) {
	svc, _, store, cleanup := setupSyncTest(t)
	defer cleanup()
	ctx := context.Background()

	// Replace the transport with one that always errors.
	svc.transport = &errorTransport{}

	err := svc.Push(ctx, "key", false)
	if err == nil {
		t.Fatal("expected error from transport, got nil")
	}
	// Should still have the error from transport.
	if err.Error() != "push failed" {
		t.Errorf("error = %q, want 'push failed'", err.Error())
	}
	_ = store
}

func TestSyncServicePullTransportError(t *testing.T) {
	svc, _, store, cleanup := setupSyncTest(t)
	defer cleanup()
	ctx := context.Background()

	// Replace the transport with one that always errors on pull.
	svc.transport = &errorTransport{}

	err := svc.Pull(ctx, "key", false)
	if err == nil {
		t.Fatal("expected error from transport, got nil")
	}
	_ = store
}
