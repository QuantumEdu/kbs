package agenttelemetry

import (
	"context"
	"crypto/sha256"
	"fmt"
)

// AnalyzerEvidence is normalized, commit-bound evidence; no recommendation is inferred here.
type AnalyzerEvidence struct {
	EvidenceID, Tool, Version, InvocationID, TargetCommit, ArtifactHash string
	Severity, Location, Confidence, Coverage, Evidence                  string
	Stale                                                               bool
}

func (s *Store) ImportAnalyzerEvidence(ctx context.Context, currentCommit string, records []AnalyzerEvidence) ([]AnalyzerEvidence, error) {
	if len(records) == 0 {
		return []AnalyzerEvidence{}, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	for i := range records {
		r := &records[i]
		if r.Tool == "" || r.Version == "" || r.InvocationID == "" || r.TargetCommit == "" || r.ArtifactHash == "" || r.Severity == "" || r.Location == "" || r.Confidence == "" || r.Coverage == "" || r.Evidence == "" {
			return nil, fmt.Errorf("incomplete analyzer evidence")
		}
		r.Stale = currentCommit != "" && r.TargetCommit != currentCommit
		if r.EvidenceID == "" {
			sum := sha256.Sum256([]byte(r.Tool + "\x00" + r.Version + "\x00" + r.InvocationID + "\x00" + r.TargetCommit + "\x00" + r.ArtifactHash + "\x00" + r.Severity + "\x00" + r.Location))
			r.EvidenceID = fmt.Sprintf("sha256:%x", sum)
		}
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO analyzer_evidence VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`, r.EvidenceID, r.Tool, r.Version, r.InvocationID, r.TargetCommit, r.ArtifactHash, r.Severity, r.Location, r.Confidence, r.Coverage, r.Evidence, r.Stale); err != nil {
			return nil, err
		}
	}
	return records, tx.Commit()
}
