package db

import (
	"database/sql"
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
	if count != 6 {
		t.Errorf("expected 6 migration records (v1..v6), got %d", count)
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
