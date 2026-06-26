-- 003_hermes.sql: SkillVault v2 schema (Hermes)
-- Resolves version number conflict (two 002_*.sql files).
-- All Hermes schema changes were applied by 002_entry_refs_and_handoff.sql.
-- This migration is idempotent: safe to run after 002.

-- New: standalone tags table (promoted from inline strings)
CREATE TABLE IF NOT EXISTS tags (
    id      TEXT PRIMARY KEY,
    name    TEXT NOT NULL,
    slug    TEXT NOT NULL UNIQUE
);

-- New: artifacts (file-backed long content)
CREATE TABLE IF NOT EXISTS artifacts (
    id              TEXT PRIMARY KEY,
    title           TEXT NOT NULL,
    slug            TEXT NOT NULL UNIQUE,
    type            TEXT NOT NULL CHECK(type IN ('markdown','json','txt','html','pdf_reference','ai_output','pdf_analysis','spec','report','session_output')),
    file_path       TEXT DEFAULT '',
    mime_type       TEXT DEFAULT '',
    summary         TEXT DEFAULT '',
    content_hash    TEXT DEFAULT '',
    size_bytes      INTEGER DEFAULT 0,
    project_id      TEXT REFERENCES projects(id),
    source_entry_id TEXT,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- New: top-level workflows (separate from entries)
CREATE TABLE IF NOT EXISTS workflows (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    slug        TEXT NOT NULL UNIQUE,
    description TEXT DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('draft','active','archived','deprecated','canonical')),
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- New: entry-to-entry links
CREATE TABLE IF NOT EXISTS entry_links (
    from_entry_id  TEXT NOT NULL,
    to_entry_id    TEXT NOT NULL,
    relation_type  TEXT NOT NULL CHECK(relation_type IN ('references','supersedes','related_to','part_of','derived_from','implements')),
    PRIMARY KEY (from_entry_id, to_entry_id, relation_type)
);

-- Copy v1 data to v2 columns for entries (only if old columns exist and new are empty)
UPDATE entries SET title = name WHERE title = '' AND name != '';
UPDATE entries SET slug = name WHERE slug = '' AND name != '';
UPDATE entries SET summary = COALESCE(description, '') WHERE summary = '';
UPDATE entries SET body_optional = COALESCE(content, '') WHERE body_optional = '';
UPDATE entries SET status = CASE WHEN active = 1 THEN 'active' ELSE 'archived' END WHERE status = 'active';

-- Copy v1 data for projects
UPDATE projects SET slug = name WHERE slug = '' AND name != '';
UPDATE projects SET status = CASE WHEN active = 1 THEN 'active' ELSE 'archived' END WHERE status = 'active';

-- Copy v1 data for series
UPDATE series SET slug = name WHERE slug = '' AND name != '';
UPDATE series SET status = CASE WHEN active = 1 THEN 'active' ELSE 'archived' END WHERE status = 'active';

-- Copy step_num → order_index for series_entries
UPDATE series_entries SET order_index = step_num WHERE order_index = 0 AND step_num IS NOT NULL;

-- Evolve workflow_steps data (columns already added by 002)
UPDATE workflow_steps SET title = COALESCE(label, role, '') WHERE title = '';
UPDATE workflow_steps SET instruction = COALESCE(content, '') WHERE instruction = '';
UPDATE workflow_steps SET order_index = step_num WHERE order_index = 0 AND step_num IS NOT NULL;
UPDATE workflow_steps SET workflow_id = entry_id WHERE workflow_id = '';

-- Add v2 indexes (safe with IF NOT EXISTS)
CREATE INDEX IF NOT EXISTS idx_entries_type ON entries(type);
CREATE INDEX IF NOT EXISTS idx_entries_project_id ON entries(project_id);
DROP INDEX IF EXISTS idx_entries_status;
CREATE INDEX idx_entries_status ON entries(status) WHERE status = 'active';
CREATE INDEX IF NOT EXISTS idx_entries_slug ON entries(slug);
CREATE INDEX IF NOT EXISTS idx_projects_slug ON projects(slug);
CREATE INDEX IF NOT EXISTS idx_tags_slug ON tags(slug);

CREATE INDEX IF NOT EXISTS idx_artifacts_project_id ON artifacts(project_id);
CREATE INDEX IF NOT EXISTS idx_artifacts_slug ON artifacts(slug);
CREATE INDEX IF NOT EXISTS idx_workflows_slug ON workflows(slug);
CREATE INDEX IF NOT EXISTS idx_series_slug ON series(slug);
CREATE INDEX IF NOT EXISTS idx_series_entries_series_order ON series_entries(series_id, order_index);
CREATE INDEX IF NOT EXISTS idx_series_status ON series(status);
CREATE INDEX IF NOT EXISTS idx_entry_links_from ON entry_links(from_entry_id);
CREATE INDEX IF NOT EXISTS idx_entry_links_to ON entry_links(to_entry_id);
CREATE INDEX IF NOT EXISTS idx_workflow_steps_workflow_order ON workflow_steps(workflow_id, order_index);
CREATE INDEX IF NOT EXISTS idx_workflow_steps_entry_step ON workflow_steps(entry_id, step_num);

-- Mark this migration as applied
INSERT OR IGNORE INTO schema_migrations (version, name) VALUES (3, '003_hermes');
