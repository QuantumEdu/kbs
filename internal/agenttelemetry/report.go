package agenttelemetry

import (
	"context"
	"database/sql"
	"time"
)

type NextChangeRecommendation struct {
	EvidenceID, Tool, Severity, Location, Confidence, Coverage string
}

type NextChangeReport struct {
	Recommendations []NextChangeRecommendation
	Gaps            []string
}

// EvidenceNumber keeps an unavailable dimension distinct from a measured zero.
type EvidenceNumber struct {
	Value int64
	Known bool
}

type TokenEvidence struct {
	Scope, ScopeID, Provider, Provenance, Confidence, Coverage string
	Input, Output, CacheRead, CacheWrite, Reasoning            EvidenceNumber
}

type TimeEvidence struct {
	RunID                             string
	Wall, Measured, Inferred, Unknown EvidenceDuration
}

type EvidenceDuration struct {
	Value time.Duration
	Known bool
}

type GitEvidence struct {
	RunID, StartRevision, EndRevision, Transition, CommitCount string
}

// TelemetryEvidenceReport is a lossless report projection. It deliberately
// reports gaps as unknown instead of treating absent evidence as zero.
type TelemetryEvidenceReport struct {
	Tokens []TokenEvidence
	Time   []TimeEvidence
	Git    []GitEvidence
}

func (s *Store) ReportTelemetryEvidence(ctx context.Context) (TelemetryEvidenceReport, error) {
	return ReportTelemetryEvidenceDB(ctx, s.db)
}

func ReportTelemetryEvidenceDB(ctx context.Context, db *sql.DB) (TelemetryEvidenceReport, error) {
	var out TelemetryEvidenceReport
	tokens, err := db.QueryContext(ctx, `SELECT provider,scope,scope_id,provenance,confidence,coverage,input,output,cache_read,cache_write,reasoning,input_known,output_known,cache_read_known,cache_write_known,reasoning_known FROM usage_scope_aggregates ORDER BY scope,scope_id,provider`)
	if err != nil {
		return out, err
	}
	for tokens.Next() {
		var row TokenEvidence
		var values [5]int64
		var known [5]int
		if err := tokens.Scan(&row.Provider, &row.Scope, &row.ScopeID, &row.Provenance, &row.Confidence, &row.Coverage, &values[0], &values[1], &values[2], &values[3], &values[4], &known[0], &known[1], &known[2], &known[3], &known[4]); err != nil {
			return out, err
		}
		row.Input, row.Output = EvidenceNumber{values[0], known[0] != 0}, EvidenceNumber{values[1], known[1] != 0}
		row.CacheRead, row.CacheWrite, row.Reasoning = EvidenceNumber{values[2], known[2] != 0}, EvidenceNumber{values[3], known[3] != 0}, EvidenceNumber{values[4], known[4] != 0}
		out.Tokens = append(out.Tokens, row)
	}
	if err := tokens.Err(); err != nil {
		tokens.Close()
		return out, err
	}
	tokens.Close()

	runs, err := db.QueryContext(ctx, `SELECT id,started_at,completed_at FROM agent_runs ORDER BY started_at,id`)
	if err != nil {
		return out, err
	}
	type runBounds struct {
		id, started string
		completed   sql.NullString
	}
	var bounds []runBounds
	for runs.Next() {
		var bound runBounds
		if err := runs.Scan(&bound.id, &bound.started, &bound.completed); err != nil {
			runs.Close()
			return out, err
		}
		bounds = append(bounds, bound)
	}
	if err := runs.Err(); err != nil {
		runs.Close()
		return out, err
	}
	runs.Close()
	for _, bound := range bounds {
		id, started, completed := bound.id, bound.started, bound.completed
		t := TimeEvidence{RunID: id}
		start, startErr := time.Parse(time.RFC3339, started)
		if startErr == nil && completed.Valid {
			if end, err := time.Parse(time.RFC3339, completed.String); err == nil && end.After(start) {
				t.Wall = EvidenceDuration{end.Sub(start), true}
				rows, err := db.QueryContext(ctx, `SELECT started_at,completed_at,measured FROM activity_projection_samples WHERE run_id=? ORDER BY started_at`, id)
				if err != nil {
					return out, err
				}
				var intervals []ActivityInterval
				for rows.Next() {
					var a, b string
					var measured bool
					if err := rows.Scan(&a, &b, &measured); err != nil {
						rows.Close()
						return out, err
					}
					from, aerr := time.Parse(time.RFC3339, a)
					to, berr := time.Parse(time.RFC3339, b)
					if aerr == nil && berr == nil {
						intervals = append(intervals, ActivityInterval{Start: from, End: to, Measured: measured})
					}
				}
				if err := rows.Err(); err != nil {
					rows.Close()
					return out, err
				}
				rows.Close()
				summary := SummarizeActivity(start, end, intervals, 5*time.Minute)
				t.Measured, t.Inferred, t.Unknown = EvidenceDuration{summary.Measured, true}, EvidenceDuration{summary.Inferred, true}, EvidenceDuration{summary.Unknown, true}
			}
		}
		out.Time = append(out.Time, t)
		git := GitEvidence{RunID: id, StartRevision: "unknown", EndRevision: "unknown", Transition: "unknown", CommitCount: "unknown"}
		_ = db.QueryRowContext(ctx, `SELECT head FROM git_lifecycle_projection_samples WHERE run_id=? AND phase='start' ORDER BY captured_at DESC LIMIT 1`, id).Scan(&git.StartRevision)
		_ = db.QueryRowContext(ctx, `SELECT head FROM git_lifecycle_projection_samples WHERE run_id=? AND phase='end' ORDER BY captured_at DESC LIMIT 1`, id).Scan(&git.EndRevision)
		out.Git = append(out.Git, git)
	}
	return out, nil
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
