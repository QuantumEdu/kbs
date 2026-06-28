-- 006_entry_versions.sql: Entry versioning — archive previous content on update.
CREATE TABLE IF NOT EXISTS entry_versions (
    version_id      TEXT PRIMARY KEY,
    entry_id        TEXT NOT NULL REFERENCES entries(id),
    version_number  INTEGER NOT NULL,
    title           TEXT NOT NULL,
    summary         TEXT DEFAULT '',
    body_optional   TEXT DEFAULT '',
    saved_at        DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(entry_id, version_number)
);

CREATE INDEX IF NOT EXISTS idx_entry_versions_entry ON entry_versions(entry_id);

INSERT OR IGNORE INTO schema_migrations (version, name) VALUES (6, 'entry_versions');
