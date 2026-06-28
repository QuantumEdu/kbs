-- 005_vector_search.sql: Add entry_embeddings table for vector embeddings (BLOB storage).
CREATE TABLE IF NOT EXISTS entry_embeddings (
    entry_id    TEXT PRIMARY KEY REFERENCES entries(id),
    embedding   BLOB NOT NULL,
    dims        INTEGER NOT NULL,
    model       TEXT NOT NULL DEFAULT 'glove',
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

INSERT OR IGNORE INTO schema_migrations (version, name) VALUES (5, 'vector_search');
