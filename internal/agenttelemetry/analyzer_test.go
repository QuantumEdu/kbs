package agenttelemetry

import (
	"context"
	"testing"
	"time"
)

func TestImportAnalyzerEvidenceIsIdempotentAndMarksStale(t *testing.T) {
	store, err := OpenStore(tempDBPath(t))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	at := time.Date(2026, 8, 26, 1, 2, 3, 0, time.UTC)
	records := []AnalyzerEvidence{{Tool: "go vet", Version: "1", InvocationID: "run", TargetCommit: "old", ArtifactHash: "sha256:x", Severity: "warning", Location: "a.go:1", Confidence: "measured", Coverage: "complete", Evidence: "finding", ObservedAt: at}}
	for range 2 {
		got, err := store.ImportAnalyzerEvidence(context.Background(), "new", records)
		if err != nil || len(got) != 1 || !got[0].Stale || got[0].EvidenceID == "" {
			t.Fatalf("import = %+v, %v", got, err)
		}
	}
	var n int
	if err := store.db.QueryRow("SELECT COUNT(*) FROM analyzer_evidence").Scan(&n); err != nil || n != 1 {
		t.Fatalf("rows = %d, %v", n, err)
	}
	var stored time.Time
	if err := store.db.QueryRow("SELECT observed_at FROM analyzer_evidence").Scan(&stored); err != nil || !stored.Equal(at) {
		t.Fatalf("stored timestamp = %v, %v", stored, err)
	}
	got, err := store.ImportAnalyzerEvidence(context.Background(), "new", nil)
	if err != nil || len(got) != 0 {
		t.Fatalf("empty = %+v, %v", got, err)
	}
}

func TestImportAnalyzerEvidenceRejectsIncompleteRecord(t *testing.T) {
	store, err := OpenStore(tempDBPath(t))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	_, err = store.ImportAnalyzerEvidence(context.Background(), "commit", []AnalyzerEvidence{{
		Tool: "go vet", TargetCommit: "commit", ArtifactHash: "sha256:artifact", ObservedAt: time.Now(),
	}})
	if err == nil {
		t.Fatal("incomplete analyzer evidence must not be imported")
	}
}
