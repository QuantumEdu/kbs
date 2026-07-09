-- 008_observability_purpose.sql: Add OBSERVABILITY to the purpose CHECK constraint.
--
-- Expands the purpose column to accept six values: WORK, KNOWLEDGE, LEARNING,
-- RELATIONSHIP, STATE, OBSERVABILITY (plus empty). This is backward-compatible:
-- existing entries with any of the five existing values or empty purpose are
-- unaffected; only the CHECK constraint expands.
--
-- IMPORTANT: this migration modifies the purpose CHECK constraint ONLY; the
-- type CHECK is preserved exactly as in migration 007.
--
-- Gate: explicit INSERT column list; purpose is copied as-is (not defaulted).
-- Do NOT use bare SELECT *.

PRAGMA foreign_keys=OFF;

-- Step 1: Create new entries table matching post-007 structure plus purpose CHECK.
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
    purpose         TEXT DEFAULT '' CHECK(purpose IN ('', 'WORK', 'KNOWLEDGE', 'LEARNING', 'RELATIONSHIP', 'STATE', 'OBSERVABILITY')),
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

-- Step 2: Copy all data with explicit column list (gate: explicit INSERT).
-- purpose is copied as-is since it already exists after migration 007.
INSERT INTO entries_new (
    id, name, title, slug, type, description, content,
    summary, body_optional, purpose, status, project_id,
    artifact_id, external_ref, vars, tags_denorm, active,
    created_at, updated_at
) SELECT
    id, name, title, slug, type, description, content,
    summary, body_optional, purpose, status, project_id,
    artifact_id, external_ref, vars, tags_denorm, active,
    created_at, updated_at
FROM entries;

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

-- Step 5: Rebuild FTS5 (entries_fts lost during DROP TABLE)
INSERT INTO entries_fts(entries_fts) VALUES('rebuild');

-- Mark migration
INSERT OR IGNORE INTO schema_migrations (version, name) VALUES (8, 'observability_purpose');
