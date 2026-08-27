package main

import (
	"strings"
	"testing"
	"time"

	"github.com/quantum-6/skillvault/internal/agenttelemetry"
)

func TestCurrentReportCommitRejectsUnavailableGitState(t *testing.T) {
	previous := gitCurrentCommit
	gitCurrentCommit = func() (string, error) { return "", errNoCurrentCommit }
	t.Cleanup(func() { gitCurrentCommit = previous })
	if _, err := currentReportCommit(); err == nil {
		t.Fatal("report must not query every commit when the repository commit is unavailable")
	}
}

func TestRenderTelemetryEvidenceShowsDimensionsAndUnknowns(t *testing.T) {
	var output strings.Builder
	renderTelemetryEvidence(&output, agenttelemetry.TelemetryEvidenceReport{
		Tokens: []agenttelemetry.TokenEvidence{{Scope: "run", ScopeID: "r1", Provider: "codex", Provenance: "measured", Confidence: "high", Coverage: "partial", Input: agenttelemetry.EvidenceNumber{Value: 12, Known: true}, Output: agenttelemetry.EvidenceNumber{Value: 3, Known: true}, CacheRead: agenttelemetry.EvidenceNumber{}, CacheWrite: agenttelemetry.EvidenceNumber{}, Reasoning: agenttelemetry.EvidenceNumber{Value: 2, Known: true}}},
		Time:   []agenttelemetry.TimeEvidence{{RunID: "r1", Wall: agenttelemetry.EvidenceDuration{Value: time.Minute, Known: true}, Measured: agenttelemetry.EvidenceDuration{}, Inferred: agenttelemetry.EvidenceDuration{Value: time.Second, Known: true}, Unknown: agenttelemetry.EvidenceDuration{Value: 59 * time.Second, Known: true}}},
		Git:    []agenttelemetry.GitEvidence{{RunID: "r1", StartRevision: "start", EndRevision: "end", Transition: "unknown", CommitCount: "unknown"}},
	})
	got := output.String()
	for _, want := range []string{"input=12", "cache_read=unknown", "provenance=measured", "wall=1m0s", "measured_active=unknown", "start=start", "commits=unknown"} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q:\n%s", want, got)
		}
	}
}
