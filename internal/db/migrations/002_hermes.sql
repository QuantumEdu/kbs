-- 002_hermes.sql: SkillVault v2 schema (Hermes)
-- Creates all v2 tables (safe for fresh installs) and migrates v1 data (safe for upgrades).

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

-- Migrate projects: add slug and status columns
ALTER TABLE projects ADD COLUMN slug TEXT DEFAULT '';
ALTER TABLE projects ADD COLUMN status TEXT NOT NULL DEFAULT 'active';

-- Migrate entries: add v2 columns (title, slug, summary, body_optional, status, artifact_id)
ALTER TABLE entries ADD COLUMN title TEXT DEFAULT '';
ALTER TABLE entries ADD COLUMN slug TEXT DEFAULT '';
ALTER TABLE entries ADD COLUMN summary TEXT DEFAULT '';
ALTER TABLE entries ADD COLUMN body_optional TEXT DEFAULT '';
ALTER TABLE entries ADD COLUMN status TEXT NOT NULL DEFAULT 'active';
ALTER TABLE entries ADD COLUMN artifact_id TEXT DEFAULT '';
-- Migrate series: add slug and status
ALTER TABLE series ADD COLUMN slug TEXT DEFAULT '';
ALTER TABLE series ADD COLUMN status TEXT NOT NULL DEFAULT 'active';

-- Migrate series_entries: add order_index (replaces step_num for new code)
ALTER TABLE series_entries ADD COLUMN order_index INTEGER DEFAULT 0;

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

-- Evolve workflow_steps to v2 schema (new columns alongside v1 columns)
ALTER TABLE workflow_steps ADD COLUMN workflow_id TEXT DEFAULT '';
ALTER TABLE workflow_steps ADD COLUMN order_index INTEGER DEFAULT 0;
ALTER TABLE workflow_steps ADD COLUMN required INTEGER DEFAULT 1;
ALTER TABLE workflow_steps ADD COLUMN expected_output TEXT DEFAULT '';
ALTER TABLE workflow_steps ADD COLUMN title TEXT DEFAULT '';
ALTER TABLE workflow_steps ADD COLUMN instruction TEXT DEFAULT '';
UPDATE workflow_steps SET title = COALESCE(label, role, '') WHERE title = '';
UPDATE workflow_steps SET instruction = COALESCE(content, '') WHERE instruction = '';
UPDATE workflow_steps SET order_index = step_num WHERE order_index = 0 AND step_num IS NOT NULL;
UPDATE workflow_steps SET workflow_id = entry_id WHERE workflow_id = '';
-- Remove NOT NULL constraint on entry_id for v2 (workflow_id is now primary reference)
-- SQLite does not support ALTER COLUMN, so we recreate to drop NOT NULL.
PRAGMA foreign_keys=OFF;
CREATE TABLE IF NOT EXISTS workflow_steps_v2 (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    entry_id        TEXT REFERENCES entries(id),
    step_num        INTEGER,
    role            TEXT,
    content         TEXT,
    label           TEXT,
    workflow_id     TEXT DEFAULT '' REFERENCES workflows(id),
    order_index     INTEGER DEFAULT 0,
    title           TEXT DEFAULT '',
    instruction     TEXT DEFAULT '',
    required        INTEGER DEFAULT 1,
    expected_output TEXT DEFAULT ''
);
INSERT INTO workflow_steps_v2 (id, entry_id, step_num, role, content, label, workflow_id, order_index, title, instruction, required, expected_output)
SELECT id, entry_id, step_num, role, content, label, workflow_id, order_index, title, instruction, required, expected_output FROM workflow_steps;
DROP TABLE workflow_steps;
ALTER TABLE workflow_steps_v2 RENAME TO workflow_steps;
PRAGMA foreign_keys=ON;

-- Recreate entries table to update type CHECK constraint (v1: skill,agent,workflow,prompt,context,note → v2: expanded)
PRAGMA foreign_keys=OFF;
CREATE TABLE IF NOT EXISTS entries_v2 (
    id TEXT PRIMARY KEY,
    name TEXT DEFAULT '',
    title TEXT NOT NULL DEFAULT '',
    slug TEXT NOT NULL DEFAULT '',
    type TEXT NOT NULL CHECK(type IN ('prompt','skill','workflow_note','reference','user','feedback','project_state','session','decision','artifact_summary')),
    summary TEXT DEFAULT '',
    body_optional TEXT DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('draft','active','archived','deprecated','canonical')),
    project_id TEXT REFERENCES projects(id),
    artifact_id TEXT,
    tags_denorm TEXT DEFAULT '',
    content TEXT DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO entries_v2 (id, name, title, slug, type, summary, body_optional, status, project_id, artifact_id, tags_denorm, content, created_at, updated_at)
SELECT
    id,
    COALESCE(title, name),
    COALESCE(title, name, ''),
    COALESCE(slug, name, ''),
    CASE type
        WHEN 'skill' THEN 'skill'
        WHEN 'agent' THEN 'prompt'
        WHEN 'workflow' THEN 'workflow_note'
        WHEN 'prompt' THEN 'prompt'
        WHEN 'context' THEN 'reference'
        WHEN 'note' THEN 'reference'
        ELSE 'reference'
    END,
    COALESCE(summary, description, ''),
    COALESCE(body_optional, content, ''),
    COALESCE(status, CASE WHEN active = 1 THEN 'active' ELSE 'archived' END),
    project_id,
    COALESCE(artifact_id, NULL),
    COALESCE(tags_denorm, ''),
    COALESCE(content, ''),
    created_at,
    updated_at
FROM entries;
DROP TABLE IF EXISTS entries;
ALTER TABLE entries_v2 RENAME TO entries;
PRAGMA foreign_keys=ON;

-- Recreate FTS5 with v2 columns (v1 had: id, name, description, content, tags_denorm)
DROP TABLE IF EXISTS entries_fts;
CREATE VIRTUAL TABLE IF NOT EXISTS entries_fts USING fts5(
    id,
    title,
    summary,
    body_optional,
    tags_denorm,
    tokenize='porter unicode61'
);

-- Populate FTS5 from migrated entries (columns already mapped during table recreation)
INSERT INTO entries_fts (id, title, summary, body_optional, tags_denorm)
SELECT id, COALESCE(title, ''), COALESCE(summary, ''), COALESCE(body_optional, ''), COALESCE(tags_denorm, '')
FROM entries;

-- Add v2 indexes (v1 indexes on entries table were lost during table recreation)
CREATE INDEX IF NOT EXISTS idx_entries_type ON entries(type);
CREATE INDEX IF NOT EXISTS idx_entries_project_id ON entries(project_id);
CREATE INDEX IF NOT EXISTS idx_entries_status ON entries(status);
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
INSERT OR IGNORE INTO schema_migrations (version, name) VALUES (2, '002_hermes');
