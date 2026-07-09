package db

import (
	"database/sql"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestMigrationInitCreatesAllTables(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory DB: %v", err)
	}
	defer db.Close()

	if err := RunMigrations(db); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	// All 8 alpha tables should exist
	tables := []string{
		"schema_migrations",
		"projects",
		"entries",
		"entry_tags",
		"series",
		"series_entries",
		"workflow_steps",
		"entries_fts",
	}

	for _, table := range tables {
		var count int
		err := db.QueryRow(
			"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?",
			table,
		).Scan(&count)
		if err != nil {
			t.Errorf("failed to check table %s: %v", table, err)
			continue
		}
		if count != 1 {
			t.Errorf("table %s does not exist", table)
		}
	}

	// Verify schema_migrations has version 1 recorded
	var version int
	err = db.QueryRow("SELECT version FROM schema_migrations WHERE version = 1").Scan(&version)
	if err != nil {
		t.Errorf("migration version 1 not recorded: %v", err)
	}
}

func TestFTS5VirtualTable(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory DB: %v", err)
	}
	defer db.Close()

	if err := RunMigrations(db); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	// Verify FTS5 table was created
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='entries_fts'").Scan(&count)
	if err != nil {
		t.Fatalf("failed to check entries_fts: %v", err)
	}
	if count != 1 {
		t.Errorf("entries_fts FTS5 virtual table does not exist")
	}
}

func TestAllIndexesCreated(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory DB: %v", err)
	}
	defer db.Close()

	if err := RunMigrations(db); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	indexes := []string{
		"idx_entries_type",
		"idx_entries_project_id",
		"idx_entries_status",
		"idx_entries_slug",
		"idx_series_project_id",
		"idx_series_active",
		"idx_series_entries_series_step",
		"idx_entry_tags_tag",
		"idx_workflow_steps_entry_step",
	}

	for _, idx := range indexes {
		var count int
		err := db.QueryRow(
			"SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?",
			idx,
		).Scan(&count)
		if err != nil {
			t.Errorf("failed to check index %s: %v", idx, err)
			continue
		}
		if count != 1 {
			t.Errorf("index %s does not exist", idx)
		}
	}
}

func TestPartialIndexStatusActive(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory DB: %v", err)
	}
	defer db.Close()

	if err := RunMigrations(db); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	var sqlDef string
	err = db.QueryRow(
		"SELECT sql FROM sqlite_master WHERE type='index' AND name='idx_entries_status'",
	).Scan(&sqlDef)
	if err != nil {
		t.Fatalf("failed to read idx_entries_status definition: %v", err)
	}
	if !strings.Contains(sqlDef, "WHERE status = 'active'") {
		t.Errorf("idx_entries_status should be a partial index with WHERE status = 'active', got: %s", sqlDef)
	}
}

func TestMigration004CreatesRunTables(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory DB: %v", err)
	}
	defer db.Close()

	if err := RunMigrations(db); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	// Verify runs table exists with correct columns
	var count int
	err = db.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='runs'",
	).Scan(&count)
	if err != nil {
		t.Errorf("failed to check runs table: %v", err)
	}
	if count != 1 {
		t.Fatalf("runs table does not exist — migration 004 missing?")
	}

	// Verify runs columns
	cols := []struct{ name, typ string }{
		{"id", "TEXT"},
		{"workflow_id", "TEXT"},
		{"input", "TEXT"},
		{"output", "TEXT"},
		{"status", "TEXT"},
		{"started_at", "DATETIME"},
		{"finished_at", "DATETIME"},
	}
	for _, c := range cols {
		var colCount int
		err := db.QueryRow(
			"SELECT COUNT(*) FROM pragma_table_info('runs') WHERE name=? AND upper(type)=?",
			c.name, c.typ,
		).Scan(&colCount)
		if err != nil {
			t.Errorf("failed to check column runs.%s: %v", c.name, err)
			continue
		}
		if colCount != 1 {
			t.Errorf("runs.%s %s column missing", c.name, c.typ)
		}
	}

	// Verify run_steps table exists
	err = db.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='run_steps'",
	).Scan(&count)
	if err != nil {
		t.Errorf("failed to check run_steps table: %v", err)
	}
	if count != 1 {
		t.Fatalf("run_steps table does not exist — migration 004 missing?")
	}

	// Verify run_steps columns
	runStepCols := []struct{ name, typ string }{
		{"id", "TEXT"},
		{"run_id", "TEXT"},
		{"step_id", "INTEGER"},
		{"entry_id", "TEXT"},
		{"input", "TEXT"},
		{"output", "TEXT"},
		{"status", "TEXT"},
		{"started_at", "DATETIME"},
		{"finished_at", "DATETIME"},
	}
	for _, c := range runStepCols {
		var colCount int
		err := db.QueryRow(
			"SELECT COUNT(*) FROM pragma_table_info('run_steps') WHERE name=? AND upper(type)=?",
			c.name, c.typ,
		).Scan(&colCount)
		if err != nil {
			t.Errorf("failed to check column run_steps.%s: %v", c.name, err)
			continue
		}
		if colCount != 1 {
			t.Errorf("run_steps.%s %s column missing", c.name, c.typ)
		}
	}

	// Verify entry_slug column on workflow_steps
	var colCount int
	err = db.QueryRow(
		"SELECT COUNT(*) FROM pragma_table_info('workflow_steps') WHERE name='entry_slug'",
	).Scan(&colCount)
	if err != nil {
		t.Errorf("failed to check entry_slug column: %v", err)
	}
	if colCount != 1 {
		t.Errorf("workflow_steps.entry_slug column missing")
	}

	// Verify migration version 4 was recorded
	var version int
	err = db.QueryRow("SELECT version FROM schema_migrations WHERE version = 4").Scan(&version)
	if err != nil {
		t.Errorf("migration version 4 not recorded: %v", err)
	}

	// Verify indexes
	indexes := []string{"idx_runs_workflow", "idx_run_steps_run"}
	for _, idx := range indexes {
		var idxCount int
		err := db.QueryRow(
			"SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?",
			idx,
		).Scan(&idxCount)
		if err != nil {
			t.Errorf("failed to check index %s: %v", idx, err)
			continue
		}
		if idxCount != 1 {
			t.Errorf("index %s does not exist", idx)
		}
	}
}

func TestMigrationIsIdempotent(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory DB: %v", err)
	}
	defer db.Close()

	// Run migrations twice
	if err := RunMigrations(db); err != nil {
		t.Fatalf("first RunMigrations failed: %v", err)
	}
	if err := RunMigrations(db); err != nil {
		t.Fatalf("second RunMigrations should be idempotent but failed: %v", err)
	}

	// Should still have exactly one migration record
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	if err != nil {
		t.Fatalf("failed to count migrations: %v", err)
	}
	if count != 9 {
		t.Errorf("expected 9 migration records (v1..v9), got %d", count)
	}
}

func TestPartialIndexEntryLinksActive(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory DB: %v", err)
	}
	defer db.Close()

	if err := RunMigrations(db); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	var sqlDef string
	err = db.QueryRow(
		"SELECT sql FROM sqlite_master WHERE type='index' AND name='idx_entry_links_active'",
	).Scan(&sqlDef)
	if err != nil {
		t.Fatalf("failed to read idx_entry_links_active definition: %v", err)
	}
	if !strings.Contains(sqlDef, "WHERE active = 1") {
		t.Errorf("idx_entry_links_active should be a partial index with WHERE active = 1, got: %s", sqlDef)
	}
}

func TestMigration005EntryEmbeddings(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory DB: %v", err)
	}
	defer db.Close()

	if err := RunMigrations(db); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	// Verify entry_embeddings table exists.
	var count int
	err = db.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='entry_embeddings'",
	).Scan(&count)
	if err != nil {
		t.Fatalf("failed to check entry_embeddings: %v", err)
	}
	if count != 1 {
		t.Fatalf("entry_embeddings table does not exist — migration 005 missing?")
	}

	// Verify columns: entry_id (PK), embedding (BLOB), dims (INTEGER), model (TEXT), updated_at.
	cols := []struct{ name, typ string }{
		{"entry_id", "TEXT"},
		{"embedding", "BLOB"},
		{"dims", "INTEGER"},
		{"model", "TEXT"},
		{"updated_at", "DATETIME"},
	}
	for _, c := range cols {
		var colCount int
		err := db.QueryRow(
			"SELECT COUNT(*) FROM pragma_table_info('entry_embeddings') WHERE name=? AND upper(type)=?",
			c.name, c.typ,
		).Scan(&colCount)
		if err != nil {
			t.Errorf("failed to check column entry_embeddings.%s: %v", c.name, err)
			continue
		}
		if colCount != 1 {
			t.Errorf("entry_embeddings.%s %s column missing", c.name, c.typ)
		}
	}

	// Verify migration version 5 was recorded.
	var version int
	err = db.QueryRow("SELECT version FROM schema_migrations WHERE version = 5").Scan(&version)
	if err != nil {
		t.Errorf("migration version 5 not recorded: %v", err)
	}
}

func TestMigration006RoutingEntryType(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory DB: %v", err)
	}
	defer db.Close()

	if err := RunMigrations(db); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	// Verify migration version 6 was recorded
	var version int
	err = db.QueryRow("SELECT version FROM schema_migrations WHERE version = 6").Scan(&version)
	if err != nil {
		t.Fatalf("migration version 6 not recorded: %v", err)
	}

	// CHECK constraint must accept 'routing'
	_, err = db.Exec(`INSERT INTO entries (id, title, slug, type) VALUES ('rt-1', 'Test Routing', 'test-routing', 'routing')`)
	if err != nil {
		t.Errorf("CHECK constraint rejected 'routing' type: %v", err)
	}

	// CHECK constraint must reject invalid types
	_, err = db.Exec(`INSERT INTO entries (id, title, slug, type) VALUES ('bad-1', 'Bad', 'bad-type', 'bogus')`)
	if err == nil {
		t.Error("CHECK constraint accepted invalid type 'bogus'")
	}
	if err != nil && !strings.Contains(err.Error(), "CHECK constraint") {
		t.Errorf("expected CHECK constraint error, got: %v", err)
	}

	// Verify routing entry is insertable and retrievable
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM entries WHERE type = 'routing'").Scan(&count)
	if err != nil {
		t.Errorf("failed to query routing entries: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 routing entry, got %d", count)
	}

	// Verify idempotency — running migration again should not fail
	if err := RunMigrations(db); err != nil {
		t.Errorf("RunMigrations should be idempotent but failed: %v", err)
	}
}

// applyMigrationVersion reads and executes a single embedded migration by version.
// It also records the version in schema_migrations.
func applyMigrationVersion(t *testing.T, db *sql.DB, version int) {
	t.Helper()

	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		t.Fatalf("failed to read migrations directory: %v", err)
	}

	var matches []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		parts := strings.SplitN(e.Name(), "_", 2)
		if len(parts) < 2 {
			continue
		}
		v, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}
		if v == version {
			matches = append(matches, e.Name())
		}
	}
	if len(matches) == 0 {
		t.Fatalf("migration version %d not found", version)
	}
	sort.Strings(matches)
	name := matches[0]

	sqlBytes, err := fs.ReadFile(migrationsFS, "migrations/"+name)
	if err != nil {
		t.Fatalf("failed to read migration %s: %v", name, err)
	}

	if _, err := db.Exec(string(sqlBytes)); err != nil {
		t.Fatalf("migration %s failed: %v", name, err)
	}
	if _, err := db.Exec("INSERT OR IGNORE INTO schema_migrations (version, name) VALUES (?, ?)", version, strings.TrimSuffix(name, ".sql")); err != nil {
		t.Fatalf("failed to record migration %s: %v", name, err)
	}
}

// TestMigration006UpgradeFromV5 verifies that a database at migration 005
// (post-005 schema, with legacy type CHECK and no title/slug/status constraints)
// upgrades cleanly to 006 without data loss or constraint failures.
func TestMigration006UpgradeFromV5(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory DB: %v", err)
	}
	defer db.Close()

	// Create schema_migrations table manually so we can record applied versions.
	if _, err := db.Exec(`CREATE TABLE schema_migrations (
		version     INTEGER PRIMARY KEY,
		name        TEXT NOT NULL,
		applied_at  DATETIME DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		t.Fatalf("failed to create schema_migrations: %v", err)
	}

	// Apply migrations 001-005 to simulate an existing vault.
	for v := 1; v <= 5; v++ {
		applyMigrationVersion(t, db, v)
	}

	// Insert rows that reflect the permissive post-005 schema:
	// legacy types, empty/NULL slugs, and NULL status.
	if _, err := db.Exec(`
		INSERT INTO entries (id, name, type, summary, body_optional) VALUES
		('e-legacy', 'Legacy Entry', 'agent', 'legacy summary', 'legacy body'),
		('e-nulls', 'Nulls Entry', 'prompt', '', ''),
		('e-empty', '', 'note', '', '')
	`); err != nil {
		t.Fatalf("failed to seed v5 entries: %v", err)
	}

	// Apply migration 006.
	applyMigrationVersion(t, db, 6)

	// Verify legacy rows survived with original values.
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM entries").Scan(&count); err != nil {
		t.Fatalf("failed to count entries: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 preserved entries, got %d", count)
	}

	var name string
	var title, slug, status sql.NullString
	if err := db.QueryRow("SELECT name, title, slug, status FROM entries WHERE id = 'e-nulls'").Scan(&name, &title, &slug, &status); err != nil {
		t.Errorf("failed to query preserved row: %v", err)
	} else {
		if name != "Nulls Entry" {
			t.Errorf("expected name 'Nulls Entry', got %q", name)
		}
		if title.Valid {
			t.Errorf("expected NULL title, got %q", title.String)
		}
		if slug.Valid {
			t.Errorf("expected NULL slug, got %q", slug.String)
		}
		if !status.Valid || status.String != "active" {
			t.Errorf("expected status 'active', got %v", status)
		}
	}

	// Verify 'routing' is now accepted.
	if _, err := db.Exec(`INSERT INTO entries (id, type) VALUES ('rt-1', 'routing')`); err != nil {
		t.Errorf("CHECK constraint rejected 'routing' type on upgraded DB: %v", err)
	}

	// Verify invalid types are still rejected.
	_, err = db.Exec(`INSERT INTO entries (id, type) VALUES ('bad-1', 'bogus')`)
	if err == nil {
		t.Error("CHECK constraint accepted invalid type 'bogus' on upgraded DB")
	}

	// Verify critical indexes were recreated.
	for _, idx := range []string{"idx_entries_type", "idx_entries_project_id", "idx_entries_status", "idx_entries_slug", "idx_entries_active"} {
		var idxCount int
		if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?", idx).Scan(&idxCount); err != nil {
			t.Errorf("failed to check index %s: %v", idx, err)
		} else if idxCount != 1 {
			t.Errorf("index %s missing after migration", idx)
		}
	}

	// Verify RunMigrations is idempotent on the upgraded DB.
	if err := RunMigrations(db); err != nil {
		t.Errorf("RunMigrations should be idempotent after v5->v6 upgrade but failed: %v", err)
	}

	// Sanity-check that migration version 6 was recorded.
	var version int
	err = db.QueryRow("SELECT version FROM schema_migrations WHERE version = 6").Scan(&version)
	if err != nil {
		t.Errorf("migration version 6 not recorded after upgrade: %v", err)
	}
}

// TestMigration008OBSERVABILITY verifies that migration 008:
//  1. Preserves all existing entries after the purpose CHECK constraint expands.
//  2. Accepts OBSERVABILITY as a valid purpose.
//  3. Rejects invalid purpose values.
func TestMigration008OBSERVABILITY(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory DB: %v", err)
	}
	defer db.Close()

	// Create schema_migrations table manually for staged migration.
	if _, err := db.Exec(`CREATE TABLE schema_migrations (
		version     INTEGER PRIMARY KEY,
		name        TEXT NOT NULL,
		applied_at  DATETIME DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		t.Fatalf("failed to create schema_migrations: %v", err)
	}

	// Apply migrations 001-007 to simulate an existing vault.
	for v := 1; v <= 7; v++ {
		applyMigrationVersion(t, db, v)
	}

	// Seed entries with various purpose values that exist pre-008.
	if _, err := db.Exec(`
		INSERT INTO entries (id, name, title, slug, type, summary, body_optional, purpose, status) VALUES
		('e-work',     'Work Entry',     'Work Entry',     'work-entry',      'prompt', 'work summary',     '', 'WORK',          'active'),
		('e-knowledge','Knowledge Entry','Knowledge Entry','knowledge-entry',  'prompt', 'knowledge summary','', 'KNOWLEDGE',     'active'),
		('e-empty',    'Empty Purpose',  'Empty Purpose',  'empty-purpose',   'prompt', 'empty summary',    '', '',              'active'),
		('e-state',    'State Entry',    'State Entry',    'state-entry',     'prompt', 'state summary',    '', 'STATE',         'active')
	`); err != nil {
		t.Fatalf("failed to seed entries before migration 008: %v", err)
	}

	// Apply migration 008.
	applyMigrationVersion(t, db, 8)

	// Verify migration version 8 was recorded.
	var version int
	if err := db.QueryRow("SELECT version FROM schema_migrations WHERE version = 8").Scan(&version); err != nil {
		t.Fatalf("migration version 8 not recorded: %v", err)
	}

	// --- Data preservation: all 4 seeded entries must still exist ---
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM entries").Scan(&count); err != nil {
		t.Fatalf("failed to count entries after migration: %v", err)
	}
	if count != 4 {
		t.Errorf("expected 4 preserved entries, got %d", count)
	}

	// Verify each seeded entry retained its purpose value.
	rows, err := db.Query("SELECT id, purpose FROM entries ORDER BY id")
	if err != nil {
		t.Fatalf("failed to query entries: %v", err)
	}
	defer rows.Close()

	expected := map[string]string{
		"e-empty":    "",
		"e-knowledge": "KNOWLEDGE",
		"e-state":    "STATE",
		"e-work":     "WORK",
	}
	for rows.Next() {
		var id, purpose string
		if err := rows.Scan(&id, &purpose); err != nil {
			t.Errorf("failed to scan row: %v", err)
			continue
		}
		if want, ok := expected[id]; ok && purpose != want {
			t.Errorf("entry %s: purpose = %q, want %q", id, purpose, want)
		}
	}

	// --- OBSERVABILITY is now accepted by the CHECK constraint ---
	obsID := "e-obs"
	if _, err := db.Exec("INSERT INTO entries (id, name, title, slug, type, summary, body_optional, purpose, status) VALUES (?, ?, ?, ?, 'prompt', 'obs summary', '', 'OBSERVABILITY', 'active')",
		obsID, "Obs Entry", "Obs Entry", "obs-entry"); err != nil {
		t.Errorf("CHECK constraint rejected OBSERVABILITY purpose: %v", err)
	}

	// Verify the OBSERVABILITY entry is retrievable.
	var obsPurpose string
	if err := db.QueryRow("SELECT purpose FROM entries WHERE id = ?", obsID).Scan(&obsPurpose); err != nil {
		t.Errorf("failed to query OBSERVABILITY entry: %v", err)
	} else if obsPurpose != "OBSERVABILITY" {
		t.Errorf("expected purpose OBSERVABILITY, got %q", obsPurpose)
	}

	// --- Invalid purpose is rejected by CHECK constraint ---
	_, err = db.Exec("INSERT INTO entries (id, name, title, slug, type, summary, body_optional, purpose, status) VALUES ('e-bad', 'Bad', 'Bad', 'bad-entry', 'prompt', 'bad', '', 'INVALID', 'active')")
	if err == nil {
		t.Error("CHECK constraint accepted invalid purpose 'INVALID'")
	}
	if err != nil && !strings.Contains(err.Error(), "CHECK constraint") {
		t.Errorf("expected CHECK constraint error, got: %v", err)
	}

	// --- Type CHECK is unchanged (valid types from 007 still work) ---
	if _, err := db.Exec("INSERT INTO entries (id, name, title, slug, type, summary, body_optional, purpose, status) VALUES ('e-type', 'Type Test', 'Type Test', 'type-test', 'handoff', 'type summary', '', '', 'active')"); err != nil {
		t.Errorf("type CHECK regressed — 'handoff' rejected: %v", err)
	}

	// --- Critical indexes were recreated ---
	for _, idx := range []string{"idx_entries_type", "idx_entries_project_id", "idx_entries_status", "idx_entries_slug", "idx_entries_active"} {
		var idxCount int
		if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?", idx).Scan(&idxCount); err != nil {
			t.Errorf("failed to check index %s: %v", idx, err)
		} else if idxCount != 1 {
			t.Errorf("index %s missing after migration 008", idx)
		}
	}

	// --- RunMigrations is idempotent after 008 ---
	if err := RunMigrations(db); err != nil {
		t.Errorf("RunMigrations should be idempotent after 008 but failed: %v", err)
	}
}
