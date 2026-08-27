package agenttelemetry

// TokenScope identifies the only permitted direction for token aggregation.
type TokenScope string

const (
	InteractionScope TokenScope = "interaction"
	RunScope         TokenScope = "run"
	SessionScope     TokenScope = "session"
	ChangeScope      TokenScope = "change"
	ProjectScope     TokenScope = "project"
)

// TokenSample retains every provider-defined token dimension. Nil means the
// provider omitted the dimension; it must never be turned into measured zero.
type TokenSample struct {
	SampleID                                        string
	Identity                                        EvidenceMeta
	Method, Coverage                                string
	Input, Output, CacheRead, CacheWrite, Reasoning *int64
}

// TokenAggregate is deterministic evidence for one upward-only scope.
type TokenAggregate struct {
	Scope                                           TokenScope
	Identity                                        EvidenceMeta
	Provenance, Confidence, Coverage                string
	Input, Output, CacheRead, CacheWrite, Reasoning *int64
}

// AggregateTokenSamples preserves every token dimension and never converts
// missing provider evidence to a measurement. Replay protection is owned by
// the projection sample identity; this pure helper is deterministic by input.
func AggregateTokenSamples(samples []TokenSample) map[TokenScope]TokenAggregate {
	out := make(map[TokenScope]TokenAggregate, 5)
	for _, sample := range samples {
		identity := normalizeEvidenceMeta(sample.Identity)
		coverage := unknown(sample.Coverage)
		if coverage == CoverageUnknown {
			coverage = unknown(identity.Coverage)
		}
		for _, scope := range []TokenScope{InteractionScope, RunScope, SessionScope, ChangeScope, ProjectScope} {
			row, exists := out[scope]
			if !exists {
				row = TokenAggregate{Scope: scope, Identity: identity, Provenance: unknown(sample.Method), Confidence: unknown(identity.Confidence), Coverage: coverage,
					Input: cloneToken(sample.Input), Output: cloneToken(sample.Output), CacheRead: cloneToken(sample.CacheRead), CacheWrite: cloneToken(sample.CacheWrite), Reasoning: cloneToken(sample.Reasoning)}
				out[scope] = row
				continue
			}
			out[scope] = mergeTokenAggregate(row, TokenAggregate{Scope: scope, Identity: identity, Provenance: unknown(sample.Method), Confidence: unknown(identity.Confidence), Coverage: coverage,
				Input: sample.Input, Output: sample.Output, CacheRead: sample.CacheRead, CacheWrite: sample.CacheWrite, Reasoning: sample.Reasoning})
		}
	}
	return out
}

func mergeTokenAggregate(left, right TokenAggregate) TokenAggregate {
	return TokenAggregate{Scope: left.Scope, Identity: mergeEvidenceMeta(left.Identity, right.Identity),
		Provenance: mergeEvidenceValue(left.Provenance, right.Provenance), Confidence: mergeEvidenceValue(left.Confidence, right.Confidence), Coverage: mergeEvidenceValue(left.Coverage, right.Coverage),
		Input: mergeTokenDimension(left.Input, right.Input), Output: mergeTokenDimension(left.Output, right.Output), CacheRead: mergeTokenDimension(left.CacheRead, right.CacheRead), CacheWrite: mergeTokenDimension(left.CacheWrite, right.CacheWrite), Reasoning: mergeTokenDimension(left.Reasoning, right.Reasoning)}
}

func cloneToken(value *int64) *int64 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func mergeTokenDimension(left, right *int64) *int64 {
	if left == nil || right == nil {
		return nil
	}
	total := *left + *right
	return &total
}

func normalizeEvidenceMeta(meta EvidenceMeta) EvidenceMeta {
	meta.ProjectID, meta.ChangeID, meta.SessionID = unknown(meta.ProjectID), unknown(meta.ChangeID), unknown(meta.SessionID)
	meta.RunID, meta.InteractionID, meta.AgentID = unknown(meta.RunID), unknown(meta.InteractionID), unknown(meta.AgentID)
	meta.Provider, meta.Model, meta.Effort = unknown(meta.Provider), unknown(meta.Model), unknown(meta.Effort)
	meta.Source, meta.Confidence, meta.Coverage = unknown(meta.Source), unknown(meta.Confidence), unknown(meta.Coverage)
	return meta
}

func mergeEvidenceMeta(left, right EvidenceMeta) EvidenceMeta {
	return EvidenceMeta{
		ProjectID: mergeEvidenceValue(left.ProjectID, right.ProjectID), ChangeID: mergeEvidenceValue(left.ChangeID, right.ChangeID), SessionID: mergeEvidenceValue(left.SessionID, right.SessionID),
		RunID: mergeEvidenceValue(left.RunID, right.RunID), InteractionID: mergeEvidenceValue(left.InteractionID, right.InteractionID), AgentID: mergeEvidenceValue(left.AgentID, right.AgentID),
		Provider: mergeEvidenceValue(left.Provider, right.Provider), Model: mergeEvidenceValue(left.Model, right.Model), Effort: mergeEvidenceValue(left.Effort, right.Effort),
		Source: mergeEvidenceValue(left.Source, right.Source), Confidence: mergeEvidenceValue(left.Confidence, right.Confidence), Coverage: mergeEvidenceValue(left.Coverage, right.Coverage),
	}
}

func mergeEvidenceValue(left, right string) string {
	if left == right {
		return left
	}
	if left == CoverageUnknown || right == CoverageUnknown {
		return CoverageUnknown
	}
	return "mixed"
}
