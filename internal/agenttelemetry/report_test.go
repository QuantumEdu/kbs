package agenttelemetry

import (
	"context"
	"testing"
	"time"
)

func TestReportNextChangesCitesEvidenceAndGaps(t *testing.T) {
	store, err := OpenStore(tempDBPath(t))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	_, err = store.ImportAnalyzerEvidence(context.Background(), "head", []AnalyzerEvidence{
		{Tool: "vet", Version: "1", InvocationID: "a", TargetCommit: "head", ArtifactHash: "sha256:a", Severity: "warning", Location: "a.go:1", Confidence: "measured", Coverage: "complete", Evidence: "unsafe", ObservedAt: time.Now()},
		{Tool: "lint", Version: "1", InvocationID: "b", TargetCommit: "old", ArtifactHash: "sha256:b", Severity: "error", Location: "b.go:2", Confidence: "measured", Coverage: "partial", Evidence: "stale", ObservedAt: time.Now()},
	})
	if err != nil {
		t.Fatal(err)
	}
	report, err := store.ReportNextChanges(context.Background(), "head")
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Recommendations) != 1 || report.Recommendations[0].EvidenceID == "" || report.Recommendations[0].Location != "a.go:1" {
		t.Fatalf("recommendations = %+v", report.Recommendations)
	}
	if len(report.Gaps) != 1 || report.Gaps[0] != "stale analyzer evidence: lint" {
		t.Fatalf("gaps = %+v", report.Gaps)
	}
}

func TestReportNextChangesDoesNotInventDebtWithoutEvidence(t *testing.T) {
	store, err := OpenStore(tempDBPath(t))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	report, err := store.ReportNextChanges(context.Background(), "head")
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Recommendations) != 0 || len(report.Gaps) != 1 || report.Gaps[0] != "analyzer evidence unavailable" {
		t.Fatalf("report = %+v", report)
	}
}

func TestReportTelemetryEvidencePreservesProvenanceAndUnknowns(t *testing.T) {
	store, err := OpenStore(tempDBPath(t))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	started := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	finished := started.Add(10 * time.Minute)
	if err := store.SaveRun(ctx, AgentRun{ID: "run-1", AgentID: "codex", Workspace: "/repo", StartedAt: started, CompletedAt: &finished, Status: "completed"}); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`INSERT INTO usage_scope_aggregates (provider,projector_version,scope,scope_id,identity,provenance,confidence,coverage,input,output,cache_read,cache_write,reasoning,input_known,output_known,cache_read_known,cache_write_known,reasoning_known) VALUES ('codex','v2','run','run-1','{}','measured','high','partial',12,3,0,0,2,1,1,0,0,1)`,
		`INSERT INTO activity_projection_samples VALUES ('run-1','2026-08-26T12:00:00Z','2026-08-26T12:02:00Z',1)`,
		`INSERT INTO git_lifecycle_projection_samples VALUES ('run-1','start','v2','hmac:repo','aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','main',0,0,0,0,'2026-08-26T12:00:00Z')`,
		`INSERT INTO git_lifecycle_projection_samples VALUES ('run-1','end','v2','hmac:repo','bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb','main',0,0,0,0,'2026-08-26T12:10:00Z')`,
	} {
		if _, err := store.db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	report, err := ReportTelemetryEvidenceDB(ctx, store.db)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Tokens) != 1 || report.Tokens[0].Input.Value != 12 || report.Tokens[0].CacheRead.Known || report.Tokens[0].Coverage != "partial" || report.Tokens[0].Provenance != "measured" {
		t.Fatalf("tokens = %+v", report.Tokens)
	}
	if len(report.Time) != 1 || !report.Time[0].Wall.Known || report.Time[0].Measured.Value != 2*time.Minute || report.Time[0].Unknown.Value != 8*time.Minute {
		t.Fatalf("time = %+v", report.Time)
	}
	if len(report.Git) != 1 || report.Git[0].StartRevision != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" || report.Git[0].EndRevision != "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" || report.Git[0].Transition != "unknown" {
		t.Fatalf("git = %+v", report.Git)
	}
}

func TestReportTelemetryEvidenceRendersUnknownForAbsentSamples(t *testing.T) {
	store, err := OpenStore(tempDBPath(t))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.SaveRun(context.Background(), AgentRun{ID: "run-unknown", AgentID: "codex", Workspace: "/repo", StartedAt: time.Now(), Status: "running"}); err != nil {
		t.Fatal(err)
	}
	report, err := ReportTelemetryEvidenceDB(context.Background(), store.db)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Time) != 1 || report.Time[0].Wall.Known || len(report.Git) != 1 || report.Git[0].Transition != "unknown" {
		t.Fatalf("report = %+v", report)
	}
}
