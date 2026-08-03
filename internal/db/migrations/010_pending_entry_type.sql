-- 010_pending_entry_type.sql: Add pending to the entry type CHECK constraint.
--
-- This keeps the current entries schema intact and only expands the allowed
-- type set so per-project pending items can reuse the existing entry model.
-- Existing rows are copied explicitly to avoid shape drift during rebuild.

PRAGMA foreign_keys=OFF;

CREATE TABLE entries_new (
    id              TEXT PRIMARY KEY,
    name            TEXT,
    title           TEXT,
    slug            TEXT,
    type            TEXT NOT NULL CHECK(type IN ('prompt','skill','workflow_note','reference','user','feedback','project_state','session','decision','artifact_summary','handoff','pending','routing','agent','workflow','context','note')),
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

DROP TABLE entries;
ALTER TABLE entries_new RENAME TO entries;

CREATE INDEX IF NOT EXISTS idx_entries_type ON entries(type);
CREATE INDEX IF NOT EXISTS idx_entries_project_id ON entries(project_id);
CREATE INDEX IF NOT EXISTS idx_entries_status ON entries(status) WHERE status = 'active';
CREATE INDEX IF NOT EXISTS idx_entries_slug ON entries(slug);
CREATE INDEX IF NOT EXISTS idx_entries_active ON entries(active);

PRAGMA foreign_keys=ON;

INSERT INTO entries_fts(entries_fts) VALUES('rebuild');

INSERT OR IGNORE INTO schema_migrations (version, name) VALUES (10, 'pending_entry_type');
