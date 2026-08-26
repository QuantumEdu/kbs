package agenttelemetry

import (
	"fmt"
	"strings"
)

// MigrateEvidence applies additive, idempotent evidence schema migrations.
// Existing records retain their values but gain explicit unknown coverage.
func (s *Store) MigrateEvidence() error {
	if s.db == nil {
		return fmt.Errorf("store is closed")
	}
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS projector_checkpoints (
			name TEXT NOT NULL, version TEXT NOT NULL, last_rowid INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY(name, version)
		);
		CREATE TABLE IF NOT EXISTS projected_events (
			source_event_id TEXT NOT NULL REFERENCES events(id), projector_version TEXT NOT NULL,
			PRIMARY KEY(source_event_id, projector_version)
		);
		CREATE TABLE IF NOT EXISTS usage_projection_samples (run_id TEXT NOT NULL, provider TEXT NOT NULL, sample_id TEXT NOT NULL, total INTEGER NOT NULL, measured INTEGER NOT NULL, PRIMARY KEY(provider, sample_id));
		CREATE TABLE IF NOT EXISTS usage_scope_projection_samples (
			sample_id TEXT NOT NULL, provider TEXT NOT NULL, projector_version TEXT NOT NULL, scope TEXT NOT NULL, scope_id TEXT NOT NULL, identity TEXT NOT NULL,
			provenance TEXT NOT NULL, confidence TEXT NOT NULL, coverage TEXT NOT NULL,
			input INTEGER NOT NULL, output INTEGER NOT NULL, cache_read INTEGER NOT NULL, cache_write INTEGER NOT NULL, reasoning INTEGER NOT NULL,
			PRIMARY KEY(provider, sample_id, projector_version, scope)
		);
		CREATE TABLE IF NOT EXISTS usage_cumulative_states (run_id TEXT NOT NULL, provider TEXT NOT NULL, interaction_id TEXT NOT NULL, segment_id TEXT NOT NULL, projector_version TEXT NOT NULL, total INTEGER NOT NULL, PRIMARY KEY(run_id, provider, interaction_id, segment_id, projector_version));
		CREATE TABLE IF NOT EXISTS activity_projection_samples (run_id TEXT NOT NULL, started_at DATETIME NOT NULL, completed_at DATETIME NOT NULL, measured INTEGER NOT NULL, PRIMARY KEY(run_id, started_at, completed_at, measured));
		CREATE TABLE IF NOT EXISTS git_projection_samples (run_id TEXT PRIMARY KEY, root TEXT NOT NULL, head TEXT NOT NULL, branch TEXT NOT NULL, detached INTEGER NOT NULL, staged INTEGER NOT NULL, unstaged INTEGER NOT NULL, untracked INTEGER NOT NULL, captured_at DATETIME NOT NULL);
		CREATE TABLE IF NOT EXISTS git_lifecycle_projection_samples (run_id TEXT NOT NULL, phase TEXT NOT NULL, projector_version TEXT NOT NULL, root TEXT NOT NULL, head TEXT NOT NULL, branch TEXT NOT NULL, detached INTEGER NOT NULL, staged INTEGER NOT NULL, unstaged INTEGER NOT NULL, untracked INTEGER NOT NULL, captured_at DATETIME NOT NULL, PRIMARY KEY(run_id, phase, projector_version));
		CREATE TABLE IF NOT EXISTS activity_heartbeat_samples (run_id TEXT NOT NULL, clock_id TEXT NOT NULL, observed_at DATETIME NOT NULL, projector_version TEXT NOT NULL, PRIMARY KEY(run_id, clock_id, observed_at, projector_version));
		CREATE TABLE IF NOT EXISTS analyzer_evidence (evidence_id TEXT PRIMARY KEY, tool TEXT NOT NULL, version TEXT NOT NULL, invocation_id TEXT NOT NULL, target_commit TEXT NOT NULL, artifact_hash TEXT NOT NULL, severity TEXT NOT NULL, location TEXT NOT NULL, confidence TEXT NOT NULL, coverage TEXT NOT NULL, evidence TEXT NOT NULL, observed_at DATETIME NOT NULL DEFAULT '', stale INTEGER NOT NULL);
		CREATE TABLE IF NOT EXISTS usage_projection_unknown_samples (run_id TEXT NOT NULL, provider TEXT NOT NULL, sample_id TEXT NOT NULL, projector_version TEXT NOT NULL, reason TEXT NOT NULL, PRIMARY KEY(provider, sample_id, projector_version));
		CREATE TABLE IF NOT EXISTS run_evidence (
			run_id TEXT PRIMARY KEY REFERENCES agent_runs(id),
			token_coverage TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS evidence_metadata (
			event_id TEXT PRIMARY KEY REFERENCES events(id),
			project_id TEXT NOT NULL, change_id TEXT NOT NULL, session_id TEXT NOT NULL,
			run_id TEXT NOT NULL, interaction_id TEXT NOT NULL, agent_id TEXT NOT NULL,
			provider TEXT NOT NULL, model TEXT NOT NULL, effort TEXT NOT NULL,
			source TEXT NOT NULL, confidence TEXT NOT NULL, coverage TEXT NOT NULL
		);
		INSERT OR IGNORE INTO run_evidence (run_id, token_coverage)
		SELECT id, CASE WHEN total_tokens = 0 THEN 'unknown' ELSE 'legacy' END FROM agent_runs;
		INSERT OR IGNORE INTO evidence_metadata
		(event_id, project_id, change_id, session_id, run_id, interaction_id, agent_id, provider, model, effort, source, confidence, coverage)
		SELECT id, 'unknown', 'unknown', 'unknown', run_id, 'unknown', 'unknown', 'unknown', 'unknown', 'unknown', source, confidence_level, 'unknown' FROM events;
	`)
	if err != nil {
		return err
	}
	if _, err = s.db.Exec(`ALTER TABLE analyzer_evidence ADD COLUMN observed_at DATETIME NOT NULL DEFAULT ''`); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
		return err
	}
	return nil
}
