-- 002_entry_refs_and_handoff.sql: Hermes schema migration, entry_links graph, 'handoff' type, external_ref
--
-- Builds on 001_init. Adds Hermes columns, rebuilds entries for type CK change,
-- creates missing Hermes tables (tags, workflows, artifacts, entry_links),
-- rebuilds FTS5. Because SQLite FK enforcement requires all referenced tables
-- to exist at CREATE time, we create missing Hermes tables before recreating
-- child tables that reference them.

-- Step 1: Save data from tables that will be dropped and recreated
CREATE TABLE IF NOT EXISTS _bk_entry_tags AS SELECT * FROM entry_tags;
CREATE TABLE IF NOT EXISTS _bk_series_entries AS SELECT * FROM series_entries;
CREATE TABLE IF NOT EXISTS _bk_workflow_steps AS SELECT * FROM workflow_steps;

-- Step 2: Add new columns to entries (ALTER TABLE for non-CK columns)
ALTER TABLE entries ADD COLUMN title TEXT;
ALTER TABLE entries ADD COLUMN slug TEXT;
ALTER TABLE entries ADD COLUMN summary TEXT DEFAULT '';
ALTER TABLE entries ADD COLUMN body_optional TEXT DEFAULT '';
ALTER TABLE entries ADD COLUMN status TEXT DEFAULT 'active';
ALTER TABLE entries ADD COLUMN artifact_id TEXT;
ALTER TABLE entries ADD COLUMN external_ref TEXT DEFAULT '';

UPDATE entries SET title = COALESCE(title, name, id);
UPDATE entries SET slug = COALESCE(slug, name, id);
UPDATE entries SET summary = COALESCE(summary, description, '');
UPDATE entries SET body_optional = COALESCE(body_optional, content, '');
UPDATE entries SET status =
    CASE WHEN COALESCE(active, 1) = 0 THEN 'archived' ELSE 'active' END;

-- Step 3: Create all Hermes tables that might not exist (before child table recreates)
CREATE TABLE IF NOT EXISTS tags (
    id      TEXT PRIMARY KEY,
    name    TEXT NOT NULL,
    slug    TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS workflows (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    slug        TEXT NOT NULL UNIQUE,
    description TEXT DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('draft','active','archived','deprecated','canonical')),
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

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
    source_entry_id TEXT REFERENCES entries(id),
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Step 4: Rebuild entries with updated type CHECK (adding 'handoff')
-- Old CK: ('skill','agent','workflow','prompt','context','note')
-- New CK: old + Hermes types + handoff
CREATE TABLE entries_v2 (
    id              TEXT PRIMARY KEY,
    name            TEXT,
    title           TEXT,
    slug            TEXT,
    type            TEXT NOT NULL CHECK(type IN ('prompt','skill','workflow_note','reference','user','feedback','project_state','session','decision','artifact_summary','handoff','skill','agent','workflow','context','note')),
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

INSERT INTO entries_v2 SELECT * FROM entries;

-- Step 5: Drop child tables, swap entries, recreate child tables (FKs valid since Hermes tables exist)
CREATE TABLE IF NOT EXISTS _bk_artifacts AS SELECT * FROM artifacts;
DROP TABLE IF EXISTS entry_tags;
DROP TABLE IF EXISTS series_entries;
DROP TABLE IF EXISTS workflow_steps;
DROP TABLE IF EXISTS artifacts;

ALTER TABLE entries RENAME TO entries_old;
ALTER TABLE entries_v2 RENAME TO entries;

CREATE TABLE entry_tags (
    entry_id TEXT NOT NULL REFERENCES entries(id),
    tag      TEXT NOT NULL,
    PRIMARY KEY (entry_id, tag)
);

CREATE TABLE series_entries (
    series_id TEXT NOT NULL REFERENCES series(id),
    entry_id  TEXT NOT NULL REFERENCES entries(id),
    step_num  INTEGER NOT NULL,
    order_index INTEGER DEFAULT 0,
    label     TEXT,
    required  INTEGER DEFAULT 1,
    notes     TEXT,
    active    INTEGER DEFAULT 1,
    PRIMARY KEY (series_id, entry_id),
    UNIQUE(series_id, step_num)
);

CREATE TABLE workflow_steps (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    entry_id        TEXT REFERENCES entries(id),
    workflow_id     TEXT REFERENCES workflows(id),
    step_num        INTEGER DEFAULT 0,
    order_index     INTEGER DEFAULT 0,
    role            TEXT CHECK(role IN ('system','user','assistant')),
    content         TEXT,
    title           TEXT DEFAULT '',
    instruction     TEXT DEFAULT '',
    required        INTEGER DEFAULT 1,
    expected_output TEXT DEFAULT '',
    label           TEXT
);

-- Restore data from backups
INSERT OR IGNORE INTO entry_tags (entry_id, tag) SELECT entry_id, tag FROM _bk_entry_tags;
INSERT OR IGNORE INTO series_entries (series_id, entry_id, step_num, label, required, notes, active)
SELECT series_id, entry_id, step_num, label, required, notes, active FROM _bk_series_entries;
INSERT OR IGNORE INTO workflow_steps (entry_id, workflow_id, step_num, role, content, title, label)
SELECT entry_id, NULL AS workflow_id, step_num, role, content, COALESCE(label, '') AS title, label FROM _bk_workflow_steps;

-- Recreate artifacts table (was created in step 3 but FK broken by entries swap)
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
    source_entry_id TEXT REFERENCES entries(id),
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);

INSERT OR IGNORE INTO artifacts (id, title, slug, type, file_path, mime_type, summary, content_hash, size_bytes, project_id, source_entry_id)
SELECT id, title, slug, type, file_path, mime_type, summary, content_hash, size_bytes, project_id, source_entry_id FROM _bk_artifacts;

DROP TABLE IF EXISTS _bk_entry_tags;
DROP TABLE IF EXISTS _bk_series_entries;
DROP TABLE IF EXISTS _bk_workflow_steps;
DROP TABLE IF EXISTS _bk_artifacts;
DROP TABLE IF EXISTS entries_old;

-- Step 6: Add missing columns to projects + series (ALTER TABLE only)
ALTER TABLE projects ADD COLUMN slug TEXT;
ALTER TABLE projects ADD COLUMN status TEXT DEFAULT 'active';
UPDATE projects SET slug = COALESCE(slug, name, id);
UPDATE projects SET status = CASE WHEN COALESCE(active, 1) = 0 THEN 'archived' ELSE 'active' END;

ALTER TABLE series ADD COLUMN slug TEXT;
ALTER TABLE series ADD COLUMN status TEXT DEFAULT 'active';
UPDATE series SET slug = COALESCE(slug, name, id);
UPDATE series SET status = CASE WHEN COALESCE(active, 1) = 0 THEN 'archived' ELSE 'active' END;

-- Step 7: Create entry_links table
CREATE TABLE IF NOT EXISTS entry_links (
    from_entry_id  TEXT NOT NULL REFERENCES entries(id),
    to_entry_id    TEXT NOT NULL REFERENCES entries(id),
    relation_type  TEXT NOT NULL CHECK(relation_type IN (
        'references','supersedes','related_to','part_of','derived_from','implements',
        'uses','extends','handoff_of','generated_from','depends_on'
    )),
    label          TEXT DEFAULT '',
    active         INTEGER DEFAULT 1,
    created_at     DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (from_entry_id, to_entry_id, relation_type)
);

-- Step 8: Rebuild FTS5 with Hermes columns + external_ref
DROP TABLE IF EXISTS entries_fts_old;
ALTER TABLE entries_fts RENAME TO entries_fts_old;

CREATE VIRTUAL TABLE entries_fts USING fts5(
    id,
    title,
    summary,
    body_optional,
    tags_denorm,
    external_ref,
    tokenize='porter unicode61'
);

INSERT INTO entries_fts (id, title, summary, body_optional, tags_denorm, external_ref)
SELECT id, COALESCE(title,name,id), COALESCE(summary,description,''), COALESCE(body_optional,content,''), COALESCE(tags_denorm,''), COALESCE(external_ref,'')
FROM entries;
DROP TABLE IF EXISTS entries_fts_old;

-- Step 9: Indexes
CREATE INDEX IF NOT EXISTS idx_entries_type ON entries(type);
CREATE INDEX IF NOT EXISTS idx_entries_project_id ON entries(project_id);
CREATE INDEX IF NOT EXISTS idx_entries_status ON entries(status);
CREATE INDEX IF NOT EXISTS idx_entries_slug ON entries(slug);
CREATE INDEX IF NOT EXISTS idx_entries_active ON entries(active);
CREATE INDEX IF NOT EXISTS idx_projects_slug ON projects(slug);
CREATE INDEX IF NOT EXISTS idx_entry_tags_tag ON entry_tags(tag);
CREATE INDEX IF NOT EXISTS idx_series_project_id ON series(project_id);
CREATE INDEX IF NOT EXISTS idx_series_active ON series(active);
CREATE INDEX IF NOT EXISTS idx_series_slug ON series(slug);
CREATE INDEX IF NOT EXISTS idx_series_status ON series(status);
CREATE INDEX IF NOT EXISTS idx_series_entries_series_step ON series_entries(series_id, step_num);
CREATE INDEX IF NOT EXISTS idx_series_entries_series_order ON series_entries(series_id, order_index);
CREATE INDEX IF NOT EXISTS idx_workflow_steps_entry_step ON workflow_steps(entry_id, step_num);
CREATE INDEX IF NOT EXISTS idx_workflow_steps_workflow_order ON workflow_steps(workflow_id, order_index);
CREATE INDEX IF NOT EXISTS idx_entry_links_from ON entry_links(from_entry_id);
CREATE INDEX IF NOT EXISTS idx_entry_links_to ON entry_links(to_entry_id);
CREATE INDEX IF NOT EXISTS idx_entry_links_active ON entry_links(active) WHERE active = 1;
CREATE INDEX IF NOT EXISTS idx_tags_slug ON tags(slug);
CREATE INDEX IF NOT EXISTS idx_artifacts_project_id ON artifacts(project_id);
CREATE INDEX IF NOT EXISTS idx_artifacts_slug ON artifacts(slug);
CREATE INDEX IF NOT EXISTS idx_workflows_slug ON workflows(slug);

-- Mark migration
INSERT OR IGNORE INTO schema_migrations (version, name) VALUES (2, 'entry_refs_and_handoff');
