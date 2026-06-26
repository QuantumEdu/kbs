-- 004_workflow_pipelines.sql: SkillVault v3 Workflow Pipeline Execution
-- Adds runs table (execution tracking), run_steps table (per-step state),
-- entry_slug column on workflow_steps (step-to-entry linking for pipelines).

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

ALTER TABLE workflow_steps ADD COLUMN entry_slug TEXT DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_runs_workflow ON runs(workflow_id);
CREATE INDEX IF NOT EXISTS idx_run_steps_run ON run_steps(run_id);

INSERT OR IGNORE INTO schema_migrations (version, name) VALUES (4, 'workflow_pipelines');
