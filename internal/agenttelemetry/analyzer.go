package agenttelemetry

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"
)

// AnalyzerEvidence is normalized, commit-bound evidence; no recommendation is inferred here.
type AnalyzerEvidence struct {
	EvidenceID, Tool, Version, InvocationID, TargetCommit, ArtifactHash string
	Severity, Location, Confidence, Coverage, Evidence                  string
	ObservedAt                                                          time.Time
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
		if r.Tool == "" || r.Version == "" || r.InvocationID == "" || r.TargetCommit == "" || r.ArtifactHash == "" || r.Severity == "" || r.Location == "" || r.Confidence == "" || r.Coverage == "" || r.Evidence == "" || r.ObservedAt.IsZero() {
			return nil, fmt.Errorf("incomplete analyzer evidence")
		}
		r.Stale = currentCommit != "" && r.TargetCommit != currentCommit
		if r.EvidenceID == "" {
			sum := sha256.Sum256([]byte(r.Tool + "\x00" + r.Version + "\x00" + r.InvocationID + "\x00" + r.TargetCommit + "\x00" + r.ArtifactHash + "\x00" + r.Severity + "\x00" + r.Location))
			r.EvidenceID = fmt.Sprintf("sha256:%x", sum)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO analyzer_evidence (evidence_id,tool,version,invocation_id,target_commit,artifact_hash,severity,location,confidence,coverage,evidence,observed_at,stale) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(evidence_id) DO UPDATE SET tool=excluded.tool,version=excluded.version,invocation_id=excluded.invocation_id,target_commit=excluded.target_commit,artifact_hash=excluded.artifact_hash,severity=excluded.severity,location=excluded.location,confidence=excluded.confidence,coverage=excluded.coverage,evidence=excluded.evidence,observed_at=excluded.observed_at,stale=excluded.stale`, r.EvidenceID, r.Tool, r.Version, r.InvocationID, r.TargetCommit, r.ArtifactHash, r.Severity, r.Location, r.Confidence, r.Coverage, r.Evidence, r.ObservedAt.UTC(), r.Stale); err != nil {
			return nil, err
		}
	}
	return records, tx.Commit()
}
