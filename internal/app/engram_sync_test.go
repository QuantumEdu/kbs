package app

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/quantum-6/skillvault/internal/db"
	_ "modernc.org/sqlite"
)

func createMockEngramDB(t *testing.T, dbPath string) {
	t.Helper()
	sqldb, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer sqldb.Close()

	schema := `
	CREATE TABLE sessions (
		id TEXT PRIMARY KEY,
		project TEXT,
		directory TEXT
	);
	CREATE TABLE observations (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT,
		type TEXT,
		title TEXT,
		content TEXT,
		project TEXT,
		topic_key TEXT,
		deleted_at TEXT
	);`
	if _, err := sqldb.Exec(schema); err != nil {
		t.Fatal(err)
	}

	_, err = sqldb.Exec(`
		INSERT INTO observations (session_id, type, title, content, project, topic_key) VALUES
		('s1', 'decision', 'Use SQLite for metadata', 'Chose modernc.org/sqlite for CGO-free portability', 'kbs', 'db/driver'),
		('s1', 'architecture', 'Telemetry architecture', 'Standalone daemon with Unix socket', 'kbs', 'arch/telemetry'),
		('s2', 'discovery', 'SIGHUP kills process group', 'Use Setsid to daemonize properly', 'kbs', 'gotchas/signals'),
		('s2', 'decision', 'Other project decision', 'Not for kbs', 'other', 'other/key');
	`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestSyncEngram_ImportAndIdempotency(t *testing.T) {
	tmpDir := t.TempDir()
	engramDBPath := filepath.Join(tmpDir, "mock_engram.db")
	createMockEngramDB(t, engramDBPath)

	vaultDBPath := filepath.Join(tmpDir, "vault.db")
	sqlDB, err := db.OpenDB(vaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	if err := db.RunMigrations(sqlDB); err != nil {
		t.Fatal(err)
	}

	store := db.NewStore(sqlDB)
	entrySvc := NewEntryService(store.Entries, store.Projects, store.Artifacts)

	ctx := context.Background()

	// 1. Initial import with project filter
	res, err := entrySvc.SyncEngram(ctx, EngramSyncOptions{
		DBPath:  engramDBPath,
		Project: "kbs",
	})
	if err != nil {
		t.Fatalf("SyncEngram failed: %v", err)
	}
	if res.Total != 3 {
		t.Errorf("expected 3 total observations for kbs, got %d", res.Total)
	}
	if res.Imported != 3 {
		t.Errorf("expected 3 imported observations, got %d", res.Imported)
	}

	// 2. Second import (idempotency check)
	res2, err := entrySvc.SyncEngram(ctx, EngramSyncOptions{
		DBPath:  engramDBPath,
		Project: "kbs",
	})
	if err != nil {
		t.Fatalf("second SyncEngram failed: %v", err)
	}
	if res2.Skipped != 3 {
		t.Errorf("expected 3 skipped observations on second run, got %d", res2.Skipped)
	}
	if res2.Imported != 0 {
		t.Errorf("expected 0 imported on second run, got %d", res2.Imported)
	}
}
