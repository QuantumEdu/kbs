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
	Input, Output, CacheRead, CacheWrite, Reasoning int64
}

// AggregateTokenSamples preserves every token dimension and never converts
// missing provider evidence to a measurement. Replay protection is owned by
// the projection sample identity; this pure helper is deterministic by input.
func AggregateTokenSamples(samples []TokenSample) map[TokenScope]TokenAggregate {
	out := make(map[TokenScope]TokenAggregate, 5)
	for _, sample := range samples {
		identity := sample.Identity
		identity.ProjectID, identity.ChangeID, identity.SessionID = unknown(identity.ProjectID), unknown(identity.ChangeID), unknown(identity.SessionID)
		identity.RunID, identity.InteractionID = unknown(identity.RunID), unknown(identity.InteractionID)
		identity.Provider, identity.Model, identity.Effort = unknown(identity.Provider), unknown(identity.Model), unknown(identity.Effort)
		coverage := unknown(sample.Coverage)
		if coverage == CoverageUnknown {
			coverage = unknown(identity.Coverage)
		}
		for _, scope := range []TokenScope{InteractionScope, RunScope, SessionScope, ChangeScope, ProjectScope} {
			row, exists := out[scope]
			if !exists {
				row = TokenAggregate{Scope: scope, Identity: identity, Provenance: unknown(sample.Method), Confidence: unknown(identity.Confidence), Coverage: coverage}
			}
			add := func(value *int64, target *int64) {
				if value != nil {
					*target += *value
				}
			}
			add(sample.Input, &row.Input)
			add(sample.Output, &row.Output)
			add(sample.CacheRead, &row.CacheRead)
			add(sample.CacheWrite, &row.CacheWrite)
			add(sample.Reasoning, &row.Reasoning)
			if row.Coverage != coverage {
				row.Coverage = CoverageUnknown
			}
			if row.Provenance != unknown(sample.Method) {
				row.Provenance = "mixed"
			}
			out[scope] = row
		}
	}
	return out
}
