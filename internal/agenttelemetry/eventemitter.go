package agenttelemetry

import "context"

// EventEmitter is the interface for agent telemetry plugins and wrappers.
// Implementations connect to the telemetry daemon and emit structured
// events for agent run lifecycle, tool calls, model usage, etc.
type EventEmitter interface {
	StartRun(ctx context.Context, opts RunOpts) (string, error)
	CompleteRun(ctx context.Context, runID string) error
	FailRun(ctx context.Context, runID string, errMsg error) error
	EmitEvent(ctx context.Context, e Event) error
	EmitEvents(ctx context.Context, events []Event) error
	Close() error
}
