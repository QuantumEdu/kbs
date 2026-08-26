package agenttelemetry

import "fmt"

// MigrateEvidence applies additive, idempotent evidence schema migrations.
// Existing records retain their values but gain explicit unknown coverage.
func (s *Store) MigrateEvidence() error {
	if s.db == nil {
		return fmt.Errorf("store is closed")
	}
	_, err := s.db.Exec(`
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
	return err
}
