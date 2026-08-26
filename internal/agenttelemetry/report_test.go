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
