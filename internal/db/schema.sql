-- SkillVault Schema Reference (v1-alpha)
-- This file is the consolidated reference schema.
-- Keep in sync with internal/db/migrations/001_init.sql.
-- Generated from 001_init.sql

-- Track applied migrations
CREATE TABLE IF NOT EXISTS schema_migrations (
    version     INTEGER PRIMARY KEY,
    name        TEXT NOT NULL,
    applied_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Projects: logical containers
CREATE TABLE IF NOT EXISTS projects (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    description TEXT,
    active      INTEGER DEFAULT 1,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Entries: core knowledge units
CREATE TABLE IF NOT EXISTS entries (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    type            TEXT NOT NULL CHECK(type IN ('skill','agent','workflow','prompt','context','note')),
    project_id      TEXT REFERENCES projects(id),
    description     TEXT,
    content         TEXT NOT NULL,
    vars            TEXT,
    tags_denorm     TEXT,
    active          INTEGER DEFAULT 1,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Tags: normalized, deduplicated per entry
CREATE TABLE IF NOT EXISTS entry_tags (
    entry_id TEXT NOT NULL REFERENCES entries(id),
    tag      TEXT NOT NULL,
    PRIMARY KEY (entry_id, tag)
);

-- Series: ordered sequences of entries
CREATE TABLE IF NOT EXISTS series (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    project_id  TEXT REFERENCES projects(id),
    description TEXT,
    vars        TEXT,
    active      INTEGER DEFAULT 1,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Series membership with ordering
CREATE TABLE IF NOT EXISTS series_entries (
    series_id TEXT NOT NULL REFERENCES series(id),
    entry_id  TEXT NOT NULL REFERENCES entries(id),
    step_num  INTEGER NOT NULL,
    label     TEXT,
    required  INTEGER DEFAULT 1,
    notes     TEXT,
    active    INTEGER DEFAULT 1,
    PRIMARY KEY (series_id, entry_id),
    UNIQUE(series_id, step_num)
);

-- Workflow steps: self-contained recipe steps
CREATE TABLE IF NOT EXISTS workflow_steps (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    entry_id  TEXT NOT NULL REFERENCES entries(id),
    step_num  INTEGER NOT NULL,
    role      TEXT NOT NULL CHECK(role IN ('system','user','assistant')),
    content   TEXT NOT NULL,
    label     TEXT,
    UNIQUE(entry_id, step_num)
);

-- FTS5 full-text search on entries
CREATE VIRTUAL TABLE IF NOT EXISTS entries_fts USING fts5(
    id,
    name,
    description,
    content,
    tags_denorm,
    tokenize='porter unicode61'
);

-- Indexes for common query patterns
CREATE INDEX IF NOT EXISTS idx_entries_type ON entries(type);
CREATE INDEX IF NOT EXISTS idx_entries_project_id ON entries(project_id);
CREATE INDEX IF NOT EXISTS idx_entries_active ON entries(active);
CREATE INDEX IF NOT EXISTS idx_series_project_id ON series(project_id);
CREATE INDEX IF NOT EXISTS idx_series_active ON series(active);
CREATE INDEX IF NOT EXISTS idx_series_entries_series_step ON series_entries(series_id, step_num);
CREATE INDEX IF NOT EXISTS idx_entry_tags_tag ON entry_tags(tag);
CREATE INDEX IF NOT EXISTS idx_workflow_steps_entry_step ON workflow_steps(entry_id, step_num);
