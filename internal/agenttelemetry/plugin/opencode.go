// Package plugin provides agent-specific telemetry emitter implementations.
package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/quantum-6/skillvault/internal/agenttelemetry"
)

// maxBuffer is the maximum number of events to buffer when the daemon is unreachable.
const maxBuffer = 1000

// OpenCodeEmitter implements agenttelemetry.EventEmitter for the OpenCode agent.
// It connects to the telemetry daemon via Unix socket and emits event as
// line-delimited JSON. Events are correlated via a correlation_id chain.
type OpenCodeEmitter struct {
	agentID      string
	agentVersion string
	workspace    string
	socketPath   string

	mu            sync.Mutex
	conn          net.Conn
	buffer        []agenttelemetry.Event
	lastCorrID    string
	currentRunID  string
	closed        bool
}

// NewOpenCodeEmitter creates a new OpenCodeEmitter. The emitter connects to
// the daemon lazily on the first emit. Use the socketPath from
// agenttelemetry.EnvSocketPath().
func NewOpenCodeEmitter(agentID, agentVersion, workspace, socketPath string) *OpenCodeEmitter {
	return &OpenCodeEmitter{
		agentID:      agentID,
		agentVersion: agentVersion,
		workspace:    workspace,
		socketPath:   socketPath,
	}
}

// StartRun emits a run.started event and returns the generated run_id.
func (e *OpenCodeEmitter) StartRun(ctx context.Context, opts agenttelemetry.RunOpts) (string, error) {
	runID := fmt.Sprintf("run-%d", time.Now().UnixNano())
	e.mu.Lock()
	e.currentRunID = runID
	e.mu.Unlock()

	opts.AgentID = e.agentID
	opts.AgentVersion = e.agentVersion
	if opts.Workspace == "" {
		opts.Workspace = e.workspace
	}

	payload, _ := json.Marshal(opts)
	evt := agenttelemetry.Event{
		EventID:         fmt.Sprintf("evt-%d", time.Now().UnixNano()),
		EventType:       "run.started",
		Timestamp:       time.Now(),
		RunID:           runID,
		AgentID:         e.agentID,
		AgentVersion:    e.agentVersion,
		Source:          "plugin",
		RedactionPolicy: "hash-args",
		ConfidenceLevel: "measured",
		Payload:         json.RawMessage(payload),
	}
	return runID, e.emit(ctx, evt)
}

// CompleteRun emits a run.completed event for the given run.
func (e *OpenCodeEmitter) CompleteRun(ctx context.Context, runID string) error {
	payload, _ := json.Marshal(map[string]string{"status": "completed"})
	evt := agenttelemetry.Event{
		EventID:         fmt.Sprintf("evt-%d", time.Now().UnixNano()),
		EventType:       "run.completed",
		Timestamp:       time.Now(),
		RunID:           runID,
		AgentID:         e.agentID,
		AgentVersion:    e.agentVersion,
		Source:          "plugin",
		RedactionPolicy: "hash-args",
		ConfidenceLevel: "measured",
		Payload:         json.RawMessage(payload),
	}
	return e.emit(ctx, evt)
}

// FailRun emits a run.failed event with the error message.
func (e *OpenCodeEmitter) FailRun(ctx context.Context, runID string, errMsg error) error {
	errStr := ""
	if errMsg != nil {
		errStr = errMsg.Error()
	}
	payload, _ := json.Marshal(map[string]string{"error": errStr})
	evt := agenttelemetry.Event{
		EventID:         fmt.Sprintf("evt-%d", time.Now().UnixNano()),
		EventType:       "run.failed",
		Timestamp:       time.Now(),
		RunID:           runID,
		AgentID:         e.agentID,
		AgentVersion:    e.agentVersion,
		Source:          "plugin",
		RedactionPolicy: "hash-args",
		ConfidenceLevel: "measured",
		Payload:         json.RawMessage(payload),
	}
	return e.emit(ctx, evt)
}

// EmitEvent sends a single event through the emitter, setting correlation
// and agent metadata automatically.
func (e *OpenCodeEmitter) EmitEvent(ctx context.Context, evt agenttelemetry.Event) error {
	e.mu.Lock()
	if e.currentRunID != "" && evt.RunID == "" {
		evt.RunID = e.currentRunID
	}
	if evt.AgentID == "" {
		evt.AgentID = e.agentID
	}
	if evt.AgentVersion == "" {
		evt.AgentVersion = e.agentVersion
	}
	if evt.Source == "" {
		evt.Source = "plugin"
	}
	if evt.RedactionPolicy == "" {
		evt.RedactionPolicy = "hash-args"
	}
	if evt.ConfidenceLevel == "" {
		evt.ConfidenceLevel = "measured"
	}
	if evt.EventID == "" {
		evt.EventID = fmt.Sprintf("evt-%d", time.Now().UnixNano())
	}
	if evt.Timestamp.IsZero() {
		evt.Timestamp = time.Now()
	}
	e.mu.Unlock()

	return e.emit(ctx, evt)
}

// EmitEvents sends multiple events in sequence.
func (e *OpenCodeEmitter) EmitEvents(ctx context.Context, events []agenttelemetry.Event) error {
	for _, evt := range events {
		if err := e.EmitEvent(ctx, evt); err != nil {
			return err
		}
	}
	return nil
}

// Close flushes buffered events and closes the socket connection.
func (e *OpenCodeEmitter) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.closed = true

	// Try to flush buffer.
	e.tryConnectLocked()
	if e.conn != nil && len(e.buffer) > 0 {
		for _, evt := range e.buffer {
			e.writeEventLocked(evt)
		}
		e.buffer = nil
	}

	if e.conn != nil {
		e.conn.Close()
		e.conn = nil
	}
	return nil
}

// emit sends an event. If the daemon is unreachable, it buffers the event.
func (e *OpenCodeEmitter) emit(ctx context.Context, evt agenttelemetry.Event) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return fmt.Errorf("emitter is closed")
	}

	// Update correlation chain.
	prevCorrID := e.lastCorrID
	corrID := evt.EventID
	e.lastCorrID = corrID
	if evt.CorrelationID == nil && prevCorrID != "" {
		evt.CorrelationID = &prevCorrID
	}
	// Ensure the current event's correlation_id refers to the previous event,
	// not to itself. Move the assignment after we captured the previous value.
	if evt.CorrelationID != nil && *evt.CorrelationID == evt.EventID {
		evt.CorrelationID = nil
		if prevCorrID != "" {
			evt.CorrelationID = &prevCorrID
		}
	}

	// Try to connect if not connected.
	if e.conn == nil {
		e.tryConnectLocked()
	}

	if e.conn != nil {
		if err := e.writeEventLocked(evt); err != nil {
			// Connection lost.
			e.conn.Close()
			e.conn = nil
			// Buffer the event.
			return e.bufferLocked(evt)
		}
		return nil
	}

	// Buffer when not connected.
	return e.bufferLocked(evt)
}

// bufferLocked adds an event to the buffer, dropping oldest if at capacity.
// Caller must hold e.mu.
func (e *OpenCodeEmitter) bufferLocked(evt agenttelemetry.Event) error {
	if len(e.buffer) >= maxBuffer {
		// Drop oldest.
		e.buffer = e.buffer[1:]
	}
	e.buffer = append(e.buffer, evt)
	return nil
}

// tryConnectLocked attempts to connect to the daemon. Caller must hold e.mu.
func (e *OpenCodeEmitter) tryConnectLocked() {
	conn, err := net.Dial("unix", e.socketPath)
	if err != nil {
		return
	}
	e.conn = conn

	// Flush buffered events.
	for _, evt := range e.buffer {
		e.writeEventLocked(evt)
	}
	e.buffer = nil
}

// writeEventLocked marshals and writes an event to the connection. Caller must hold e.mu.
func (e *OpenCodeEmitter) writeEventLocked(evt agenttelemetry.Event) error {
	data, err := json.Marshal(evt)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = e.conn.Write(data)
	return err
}

// SetRunID overrides the current run ID for the emitter. Useful when
// resuming an existing run from a wrapper.
func (e *OpenCodeEmitter) SetRunID(runID string) {
	e.mu.Lock()
	e.currentRunID = runID
	e.mu.Unlock()
}
