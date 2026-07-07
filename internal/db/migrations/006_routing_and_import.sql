-- 006_routing_and_import.sql: Add 'routing' entry type to CHECK constraint
--
-- SQLite does not support ALTER TABLE ... MODIFY CHECK. We rebuild the
-- entries table with the updated CHECK constraint.
--
-- IMPORTANT: this migration preserves the EXACT post-005 schema produced by
-- migrations 001-005. It does NOT add new NOT NULL / UNIQUE / CHECK
-- constraints; those are reference-schema ideals managed by schema.sql for
-- fresh databases. Only the type CHECK is changed (add 'routing').

PRAGMA foreign_keys=OFF;

-- Step 1: Create new entries table matching post-005 structure with updated CHECK.
CREATE TABLE entries_new (
    id              TEXT PRIMARY KEY,
    name            TEXT,
    title           TEXT,
    slug            TEXT,
    type            TEXT NOT NULL CHECK(type IN ('prompt','skill','workflow_note','reference','user','feedback','project_state','session','decision','artifact_summary','handoff','routing','agent','workflow','context','note')),
    description     TEXT,
    content         TEXT,
    summary         TEXT DEFAULT '',
    body_optional   TEXT DEFAULT '',
    status          TEXT DEFAULT 'active',
    project_id      TEXT REFERENCES projects(id),
    artifact_id     TEXT,
    external_ref    TEXT DEFAULT '',
    vars            TEXT,
    tags_denorm     TEXT DEFAULT '',
    active          INTEGER DEFAULT 1,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Step 2: Copy all data from old entries to new entries
INSERT INTO entries_new SELECT * FROM entries;

-- Step 3: Swap tables
DROP TABLE entries;
ALTER TABLE entries_new RENAME TO entries;

-- Step 4: Recreate indexes lost during DROP TABLE
CREATE INDEX IF NOT EXISTS idx_entries_type ON entries(type);
CREATE INDEX IF NOT EXISTS idx_entries_project_id ON entries(project_id);
CREATE INDEX IF NOT EXISTS idx_entries_status ON entries(status) WHERE status = 'active';
CREATE INDEX IF NOT EXISTS idx_entries_slug ON entries(slug);
CREATE INDEX IF NOT EXISTS idx_entries_active ON entries(active);

PRAGMA foreign_keys=ON;

-- Mark migration
INSERT OR IGNORE INTO schema_migrations (version, name) VALUES (6, 'routing_and_import');
