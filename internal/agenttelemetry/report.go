package agenttelemetry

import (
	"context"
	"database/sql"
)

type NextChangeRecommendation struct {
	EvidenceID, Tool, Severity, Location, Confidence, Coverage string
}

type NextChangeReport struct {
	Recommendations []NextChangeRecommendation
	Gaps            []string
}

func (s *Store) ReportNextChanges(ctx context.Context, currentCommit string) (NextChangeReport, error) {
	return ReportNextChangesDB(ctx, s.db, currentCommit)
}

// ReportNextChangesDB ranks only current, complete analyzer evidence. Activity
// is deliberately absent: it can identify investigation targets, never debt.
func ReportNextChangesDB(ctx context.Context, db *sql.DB, currentCommit string) (NextChangeReport, error) {
	var out NextChangeReport
	rows, err := db.QueryContext(ctx, `SELECT evidence_id, tool, severity, location, confidence, coverage FROM analyzer_evidence WHERE (target_commit=? OR ?='') AND stale=0 AND coverage='complete' ORDER BY CASE severity WHEN 'error' THEN 0 WHEN 'warning' THEN 1 ELSE 2 END, location, evidence_id`, currentCommit, currentCommit)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var r NextChangeRecommendation
		if err := rows.Scan(&r.EvidenceID, &r.Tool, &r.Severity, &r.Location, &r.Confidence, &r.Coverage); err != nil {
			return out, err
		}
		out.Recommendations = append(out.Recommendations, r)
	}
	if err := rows.Err(); err != nil {
		return out, err
	}
	var staleTool string
	if err := db.QueryRowContext(ctx, `SELECT tool FROM analyzer_evidence WHERE (target_commit<>? AND ?<>'') OR stale=1 OR coverage<>'complete' ORDER BY tool LIMIT 1`, currentCommit, currentCommit).Scan(&staleTool); err == nil {
		out.Gaps = append(out.Gaps, "stale analyzer evidence: "+staleTool)
	} else if err == sql.ErrNoRows && len(out.Recommendations) == 0 {
		out.Gaps = append(out.Gaps, "analyzer evidence unavailable")
	} else if err != nil {
		return out, err
	}
	return out, nil
}
