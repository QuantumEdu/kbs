-- 009_entry_versions.sql: Entry version history table.
--
-- Archives previous content (title, summary, body_optional) before
-- Save() overwrites it. Each version records the state at the moment
-- of archival so the user can list history and restore a past version.
--
-- Design choice: only title, summary, and body_optional are versioned;
-- structural metadata (type, status, project_id, etc.) is NOT versioned.
-- This keeps the version table slim and focused on content history.

CREATE TABLE IF NOT EXISTS entry_versions (
    version_id      TEXT PRIMARY KEY,
    entry_id        TEXT NOT NULL REFERENCES entries(id),
    version_number  INTEGER NOT NULL,
    title           TEXT NOT NULL DEFAULT '',
    summary         TEXT NOT NULL DEFAULT '',
    body_optional   TEXT NOT NULL DEFAULT '',
    saved_at        DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(entry_id, version_number)
);

CREATE INDEX IF NOT EXISTS idx_entry_versions_entry_id ON entry_versions(entry_id);

INSERT OR IGNORE INTO schema_migrations (version, name) VALUES (9, 'entry_versions');
