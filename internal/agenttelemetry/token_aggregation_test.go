package agenttelemetry

import (
	"reflect"
	"testing"
)

func TestAggregateTokenSamplesPreservesDimensionsAndScopes(t *testing.T) {
	input, output, cacheRead, cacheWrite, reasoning := int64(12), int64(3), int64(2), int64(1), int64(4)
	samples := []TokenSample{{
		SampleID: "usage-1", Identity: EvidenceMeta{ProjectID: "project", ChangeID: "change", SessionID: "session", RunID: "run", InteractionID: "interaction", Provider: "opencode", Model: "gpt-5", Effort: "high", Source: "plugin", Confidence: "measured", Coverage: "complete"},
		Method: "measured", Input: &input, Output: &output, CacheRead: &cacheRead, CacheWrite: &cacheWrite, Reasoning: &reasoning,
	}}
	got := AggregateTokenSamples(samples)
	for _, scope := range []TokenScope{InteractionScope, RunScope, SessionScope, ChangeScope, ProjectScope} {
		row, ok := got[scope]
		if !ok {
			t.Fatalf("missing %s aggregate", scope)
		}
		if row.Input == nil || *row.Input != 12 || row.Output == nil || *row.Output != 3 || row.CacheRead == nil || *row.CacheRead != 2 || row.CacheWrite == nil || *row.CacheWrite != 1 || row.Reasoning == nil || *row.Reasoning != 4 {
			t.Fatalf("%s dimensions = %+v", scope, row)
		}
		if row.Identity.Provider != "opencode" || row.Provenance != "measured" || row.Confidence != "measured" || row.Coverage != "complete" {
			t.Fatalf("%s evidence = %+v", scope, row)
		}
	}
}

func TestAggregateTokenSamplesMakesUnsupportedEvidenceExplicitlyUnknown(t *testing.T) {
	got := AggregateTokenSamples([]TokenSample{{SampleID: "unsupported", Identity: EvidenceMeta{ProjectID: "project", RunID: "run", Source: "wrapper", Coverage: CoverageUnknown}, Coverage: CoverageUnknown}})
	if got[RunScope].Coverage != CoverageUnknown || got[RunScope].Input != nil || got[RunScope].Identity.Provider != CoverageUnknown {
		t.Fatalf("unsupported aggregate = %+v", got[RunScope])
	}
}

func TestAggregateTokenSamplesMergesMetadataAndUnknownDimensionsOrderIndependently(t *testing.T) {
	input, output := int64(12), int64(3)
	samples := []TokenSample{
		{SampleID: "one", Identity: EvidenceMeta{ProjectID: "project", RunID: "run", Provider: "opencode", Model: "a", Source: "plugin", Confidence: "measured", Coverage: "complete"}, Method: "measured", Input: &input},
		{SampleID: "two", Identity: EvidenceMeta{ProjectID: "project", RunID: "run", Provider: "opencode", Model: "b", Source: "wrapper", Confidence: "estimated", Coverage: "partial"}, Method: "estimated", Output: &output},
	}
	forward := AggregateTokenSamples(samples)[RunScope]
	reverse := AggregateTokenSamples([]TokenSample{samples[1], samples[0]})[RunScope]
	if !reflect.DeepEqual(forward, reverse) {
		t.Fatalf("aggregate must be order independent: forward=%+v reverse=%+v", forward, reverse)
	}
	if forward.Input != nil || forward.Output != nil || forward.CacheRead != nil {
		t.Fatalf("a dimension omitted by either sample must remain unknown: %+v", forward)
	}
	if forward.Identity.Model != "mixed" || forward.Identity.Source != "mixed" || forward.Provenance != "mixed" || forward.Confidence != "mixed" || forward.Coverage != "mixed" {
		t.Fatalf("metadata must merge conservatively: %+v", forward)
	}
}
