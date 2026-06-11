package db

import (
	"database/sql"
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
	if count != 1 {
		t.Errorf("expected 1 migration record, got %d", count)
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
		"idx_entries_active",
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
