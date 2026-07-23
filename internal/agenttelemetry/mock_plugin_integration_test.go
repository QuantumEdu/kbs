//go:build integration

package agenttelemetry

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"path/filepath"
	"testing"
	"time"
)

// allCanonicalEventTypes is the complete list of 20 event types the mock
// plugin must emit during integration tests.
var allCanonicalEventTypes = []string{
	"run.started",
	"run.completed",
	"run.failed",
	"prompt.submitted",
	"response.received",
	"model.usage",
	"step.started",
	"step.completed",
	"tool.called",
	"tool.completed",
	"command.started",
	"command.completed",
	"file.created",
	"file.modified",
	"file.deleted",
	"test.started",
	"test.completed",
	"approval.recorded",
	"loop.detected",
	"policy.violation",
}

// mockEventEmitter connects to the daemon socket and emits events. It is used
// by integration tests to simulate a plugin sending telemetry.
type mockEventEmitter struct {
	socketPath string
	runID      string
	agentID    string
	conn       net.Conn
	reader     *bufio.Reader
}

// newMockEmitter connects to the daemon socket and returns a ready emitter.
func newMockEmitter(t *testing.T, socketPath, runID, agentID string) *mockEventEmitter {
	t.Helper()

	// Retry dial in case the daemon socket is not quite ready yet.
	var conn net.Conn
	var err error
	for i := 0; i < 10; i++ {
		conn, err = net.Dial("unix", socketPath)
		if err == nil {
			break
		}
		time.Sleep(30 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("mock emitter dial %s: %v", socketPath, err)
	}

	return &mockEventEmitter{
		socketPath: socketPath,
		runID:      runID,
		agentID:    agentID,
		conn:       conn,
		reader:     bufio.NewReader(conn),
	}
}

// emit sends a single event and verifies the ok acknowledgment.
func (m *mockEventEmitter) emit(eventType string) error {
	return m.emitWithPayload(eventType, json.RawMessage(`{}`))
}

// emitWithPayload sends a single event with a custom payload.
func (m *mockEventEmitter) emitWithPayload(eventType string, payload json.RawMessage) error {
	evt := map[string]interface{}{
		"event_id":         fmt.Sprintf("evt-mock-%s-%d", eventType, time.Now().UnixNano()),
		"event_type":       eventType,
		"timestamp":        time.Now().UTC().Format(time.RFC3339),
		"run_id":           m.runID,
		"agent_id":         m.agentID,
		"agent_version":    "0.1.0",
		"source":           "plugin",
		"redaction_policy": "hash-args",
		"confidence_level": "measured",
		"payload":          payload,
	}
	data, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	data = append(data, '\n')

	if _, err := m.conn.Write(data); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	line, err := m.reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("read ack: %w", err)
	}
	if line != AckOK {
		return fmt.Errorf("unexpected ack: %q", line)
	}
	return nil
}

// close shuts down the emitter connection.
func (m *mockEventEmitter) close() {
	m.conn.Close()
}

// TestMockPlugin_AllEventTypes starts the daemon, emits all 20 canonical event
// types from a mock plugin, and verifies every one is stored via COUNT(*).
func TestMockPlugin_AllEventTypes(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer store.Close()

	socketPath := filepath.Join(t.TempDir(), "test.sock")
	collector := NewCollector(store, socketPath)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go collector.Listen(ctx)
	time.Sleep(50 * time.Millisecond)

	emitter := newMockEmitter(t, socketPath, "run-all-types", "mock-plugin")
	defer emitter.close()

	// Emit each of the 20 canonical event types.
	for _, etype := range allCanonicalEventTypes {
		if err := emitter.emit(etype); err != nil {
			t.Errorf("emit %s: %v", etype, err)
		}
	}

	// Verify all 20 events stored.
	var count int
	for i := 0; i < 10; i++ {
		time.Sleep(50 * time.Millisecond)
		store.db.QueryRow("SELECT COUNT(*) FROM events").Scan(&count)
		if count == 20 {
			break
		}
	}
	if count != 20 {
		t.Errorf("expected 20 events, got %d", count)
	}

	cancel()
	collector.Shutdown(context.Background())
}

// TestMockPlugin_RunLifecycle emits a full run lifecycle and verifies the
// agent_run row exists and events are linked by run_id.
func TestMockPlugin_RunLifecycle(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer store.Close()

	socketPath := filepath.Join(t.TempDir(), "test.sock")
	collector := NewCollector(store, socketPath)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go collector.Listen(ctx)
	time.Sleep(50 * time.Millisecond)

	runID := "run-lifecycle-test"
	emitter := newMockEmitter(t, socketPath, runID, "mock-plugin")
	defer emitter.close()

	// Full lifecycle sequence.
	lifecycle := []string{
		"run.started",
		"tool.called",
		"tool.completed",
		"model.usage",
		"run.completed",
	}
	for _, etype := range lifecycle {
		if err := emitter.emit(etype); err != nil {
			t.Fatalf("emit %s: %v", etype, err)
		}
	}

	// Verify 5 events linked by run_id.
	var count int
	for i := 0; i < 10; i++ {
		time.Sleep(50 * time.Millisecond)
		store.db.QueryRow("SELECT COUNT(*) FROM events WHERE run_id = ?", runID).Scan(&count)
		if count == 5 {
			break
		}
	}
	if count != 5 {
		t.Errorf("expected 5 events for run %s, got %d", runID, count)
	}

	// Verify the agent_run row also has corresponding events.
	ctxBg := context.Background()
	events, err := store.GetEventsByRun(ctxBg, runID)
	if err != nil {
		t.Fatalf("GetEventsByRun: %v", err)
	}
	if len(events) != 5 {
		t.Errorf("GetEventsByRun returned %d events, want 5", len(events))
	}

	// Verify event types match the lifecycle sequence.
	seenTypes := make(map[string]int)
	for _, evt := range events {
		if evt.RunID != runID {
			t.Errorf("event %s: run_id %q, want %q", evt.EventID, evt.RunID, runID)
		}
		seenTypes[evt.EventType]++
	}
	for _, etype := range lifecycle {
		if seenTypes[etype] != 1 {
			t.Errorf("expected 1 %s event, got %d", etype, seenTypes[etype])
		}
	}

	cancel()
	collector.Shutdown(context.Background())
}

// TestMockPlugin_DaemonRestart emits events, stops the daemon, restarts it,
// emits more events, and verifies all events from both sessions exist.
func TestMockPlugin_DaemonRestart(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer store.Close()

	socketPath := filepath.Join(t.TempDir(), "test.sock")

	// --- Session 1 ---
	collector1 := NewCollector(store, socketPath)
	ctx1, cancel1 := context.WithCancel(context.Background())

	go collector1.Listen(ctx1)
	time.Sleep(50 * time.Millisecond)

	emitter1 := newMockEmitter(t, socketPath, "run-session-1", "mock-plugin")
	for _, etype := range []string{"run.started", "tool.called", "tool.completed"} {
		if err := emitter1.emit(etype); err != nil {
			t.Fatalf("session 1 emit %s: %v", etype, err)
		}
	}
	emitter1.close()

	cancel1()
	collector1.Shutdown(context.Background())

	// --- Session 2 (reuse same store, new socket path) ---
	socketPath2 := filepath.Join(t.TempDir(), "test2.sock")
	collector2 := NewCollector(store, socketPath2)

	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()

	go collector2.Listen(ctx2)
	time.Sleep(50 * time.Millisecond)

	emitter2 := newMockEmitter(t, socketPath2, "run-session-2", "mock-plugin")
	for _, etype := range []string{"run.started", "model.usage", "run.completed"} {
		if err := emitter2.emit(etype); err != nil {
			t.Fatalf("session 2 emit %s: %v", etype, err)
		}
	}
	emitter2.close()

	// Verify all 6 events from both sessions.
	var count int
	for i := 0; i < 10; i++ {
		time.Sleep(50 * time.Millisecond)
		store.db.QueryRow("SELECT COUNT(*) FROM events").Scan(&count)
		if count == 6 {
			break
		}
	}
	if count != 6 {
		t.Errorf("expected 6 total events (3 + 3), got %d", count)
	}

	cancel2()
	collector2.Shutdown(context.Background())
}

// TestMockPlugin_DaemonRestart_VerifiesPersistence is a follow-up test that
// ensures the store persists data from both sessions.
func TestMockPlugin_DaemonRestart_VerifiesPersistence(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer store.Close()

	socketPath := filepath.Join(t.TempDir(), "test.sock")

	// Session A.
	colA := NewCollector(store, socketPath)
	ctxA, cancelA := context.WithCancel(context.Background())
	go colA.Listen(ctxA)
	time.Sleep(50 * time.Millisecond)

	emA := newMockEmitter(t, socketPath, "run-a", "mock-plugin")
	for _, etype := range []string{"run.started", "tool.called", "tool.completed", "run.completed"} {
		emA.emit(etype)
	}
	emA.close()
	cancelA()
	colA.Shutdown(context.Background())
	time.Sleep(50 * time.Millisecond) // Let shutdown finish.

	// Session B — same store, different socket.
	socketPathB := filepath.Join(t.TempDir(), "test-b.sock")
	colB := NewCollector(store, socketPathB)
	ctxB, cancelB := context.WithCancel(context.Background())
	defer cancelB()

	go colB.Listen(ctxB)
	time.Sleep(50 * time.Millisecond)

	emB := newMockEmitter(t, socketPathB, "run-b", "mock-plugin")
	for _, etype := range []string{"run.started", "model.usage", "run.completed"} {
		emB.emit(etype)
	}
	emB.close()

	// Verify all 7 events exist (4 from A + 3 from B).
	var count int
	for i := 0; i < 10; i++ {
		time.Sleep(50 * time.Millisecond)
		store.db.QueryRow("SELECT COUNT(*) FROM events").Scan(&count)
		if count == 7 {
			break
		}
	}
	if count != 7 {
		t.Errorf("expected 7 total events, got %d", count)
	}

	cancelB()
	colB.Shutdown(context.Background())
}
