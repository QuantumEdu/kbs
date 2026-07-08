-- SkillVault Schema Reference (v3 Workflow Pipelines) + Sprint 1 entry_refs + handoff + external_ref + routing
-- This file is the consolidated reference schema.
-- Keep in sync with internal/db/migrations/002_entry_refs_and_handoff.sql, 003_hermes.sql,
-- 004_workflow_pipelines.sql, and 006_routing_and_import.sql.

CREATE TABLE IF NOT EXISTS schema_migrations (
    version     INTEGER PRIMARY KEY,
    name        TEXT NOT NULL,
    applied_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS projects (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    slug        TEXT NOT NULL UNIQUE,
    description TEXT DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('active','archived')),
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS entries (
    id              TEXT PRIMARY KEY,
    name            TEXT,
    title           TEXT NOT NULL,
    slug            TEXT NOT NULL UNIQUE,
    content         TEXT,
            type            TEXT NOT NULL CHECK(type IN ('prompt','skill','workflow_note','reference','user','feedback','project_state','session','decision','artifact_summary','handoff','routing')),
    summary         TEXT DEFAULT '',
    body_optional   TEXT DEFAULT '',
    purpose         TEXT DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('draft','active','archived','deprecated','canonical')),
    project_id      TEXT REFERENCES projects(id),
    artifact_id     TEXT,
    external_ref    TEXT DEFAULT '',
    tags_denorm     TEXT DEFAULT '',
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS tags (
    id      TEXT PRIMARY KEY,
    name    TEXT NOT NULL,
    slug    TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS entry_tags (
    entry_id TEXT NOT NULL REFERENCES entries(id),
    tag      TEXT NOT NULL,
    PRIMARY KEY (entry_id, tag)
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

CREATE TABLE IF NOT EXISTS workflows (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    slug        TEXT NOT NULL UNIQUE,
    description TEXT DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('draft','active','archived','deprecated','canonical')),
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS workflow_steps (
    id              TEXT PRIMARY KEY,
    workflow_id     TEXT NOT NULL REFERENCES workflows(id),
    order_index     INTEGER NOT NULL,
    title           TEXT DEFAULT '',
    instruction     TEXT DEFAULT '',
    required        INTEGER DEFAULT 1,
    expected_output TEXT DEFAULT '',
    entry_slug      TEXT DEFAULT '',
    UNIQUE(workflow_id, order_index)
);

CREATE TABLE IF NOT EXISTS series (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    slug        TEXT NOT NULL UNIQUE,
    description TEXT DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('draft','active','archived','deprecated','canonical')),
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS series_entries (
    series_id   TEXT NOT NULL REFERENCES series(id),
    entry_id    TEXT NOT NULL REFERENCES entries(id),
    order_index INTEGER NOT NULL,
    PRIMARY KEY (series_id, entry_id),
    UNIQUE(series_id, order_index)
);

CREATE TABLE IF NOT EXISTS runs (
    id          TEXT PRIMARY KEY,
    workflow_id TEXT NOT NULL REFERENCES workflows(id),
    input       TEXT DEFAULT '',
    output      TEXT DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'pending'
                CHECK(status IN ('pending','running','completed','failed')),
    started_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    finished_at DATETIME
);

CREATE TABLE IF NOT EXISTS run_steps (
    id          TEXT PRIMARY KEY,
    run_id      TEXT NOT NULL REFERENCES runs(id),
    step_id     INTEGER NOT NULL REFERENCES workflow_steps(id),
    entry_id    TEXT NOT NULL REFERENCES entries(id),
    input       TEXT DEFAULT '',
    output      TEXT DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'pending'
                CHECK(status IN ('pending','running','completed','failed')),
    started_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    finished_at DATETIME
);

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

CREATE VIRTUAL TABLE IF NOT EXISTS entries_fts USING fts5(
    id,
    title,
    summary,
    body_optional,
    tags_denorm,
    external_ref,
    tokenize='porter unicode61'
);

CREATE INDEX IF NOT EXISTS idx_entries_type ON entries(type);
CREATE INDEX IF NOT EXISTS idx_entries_project_id ON entries(project_id);
CREATE INDEX IF NOT EXISTS idx_entries_status ON entries(status) WHERE status = 'active';
CREATE INDEX IF NOT EXISTS idx_entries_slug ON entries(slug);
CREATE INDEX IF NOT EXISTS idx_entries_purpose ON entries(purpose);
CREATE INDEX IF NOT EXISTS idx_projects_slug ON projects(slug);
CREATE INDEX IF NOT EXISTS idx_tags_slug ON tags(slug);
CREATE INDEX IF NOT EXISTS idx_entry_tags_tag ON entry_tags(tag);
CREATE INDEX IF NOT EXISTS idx_artifacts_project_id ON artifacts(project_id);
CREATE INDEX IF NOT EXISTS idx_artifacts_slug ON artifacts(slug);
CREATE INDEX IF NOT EXISTS idx_workflows_slug ON workflows(slug);
CREATE INDEX IF NOT EXISTS idx_workflow_steps_workflow_order ON workflow_steps(workflow_id, order_index);
CREATE INDEX IF NOT EXISTS idx_series_slug ON series(slug);
CREATE INDEX IF NOT EXISTS idx_series_entries_series_order ON series_entries(series_id, order_index);
CREATE INDEX IF NOT EXISTS idx_series_status ON series(status);
CREATE INDEX IF NOT EXISTS idx_entry_links_from ON entry_links(from_entry_id);
CREATE INDEX IF NOT EXISTS idx_entry_links_to ON entry_links(to_entry_id);
CREATE INDEX IF NOT EXISTS idx_entry_links_active ON entry_links(active) WHERE active = 1;
CREATE INDEX IF NOT EXISTS idx_runs_workflow ON runs(workflow_id);
CREATE INDEX IF NOT EXISTS idx_run_steps_run ON run_steps(run_id);

CREATE TABLE IF NOT EXISTS entry_embeddings (
    entry_id    TEXT PRIMARY KEY REFERENCES entries(id),
    embedding   BLOB NOT NULL,
    dims        INTEGER NOT NULL,
    model       TEXT NOT NULL DEFAULT 'glove',
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);
