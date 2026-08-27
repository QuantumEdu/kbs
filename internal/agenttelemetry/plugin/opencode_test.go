package plugin

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/quantum-6/skillvault/internal/agenttelemetry"
)

func testPluginSocketPath(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	if runtime.GOOS == "darwin" {
		shortDir, err := os.MkdirTemp("/tmp", "agenttelemetry-")
		if err != nil {
			t.Fatalf("MkdirTemp: %v", err)
		}
		t.Cleanup(func() {
			_ = os.RemoveAll(shortDir)
		})
		dir = shortDir
	}

	return filepath.Join(dir, "test.sock")
}

func waitForCollectorReady(t *testing.T, socketPath string, errCh <-chan error) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for {
		conn, err := net.DialTimeout("unix", socketPath, 25*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}

		select {
		case listenErr := <-errCh:
			if listenErr == nil {
				t.Fatal("collector listener exited before becoming ready")
			}
			t.Fatalf("collector listen: %v", listenErr)
		default:
		}

		if time.Now().After(deadline) {
			t.Fatalf("collector socket %q was not ready before timeout: %v", socketPath, err)
		}

		time.Sleep(10 * time.Millisecond)
	}
}

// startTempDaemon creates a real Collector + Store on a temp socket and
// returns the socket path, store, and cancel function.
func startTempDaemon(t *testing.T) (string, *agenttelemetry.Store, context.CancelFunc) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := agenttelemetry.OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}

	socketPath := testPluginSocketPath(t)
	collector := agenttelemetry.NewCollector(store, socketPath)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)

	// Remove stale socket if exists.
	_ = os.Remove(socketPath)

	go func() {
		errCh <- collector.Listen(ctx)
	}()

	waitForCollectorReady(t, socketPath, errCh)

	t.Cleanup(func() {
		cancel()
		if err := collector.Shutdown(context.Background()); err != nil {
			t.Errorf("Shutdown: %v", err)
		}
		select {
		case err := <-errCh:
			if err != nil {
				t.Errorf("collector listen: %v", err)
			}
		case <-time.After(time.Second):
			t.Error("collector listener did not exit")
		}
		store.Close()
	})

	return socketPath, store, cancel
}

func TestOpenCodeEmitterValidEmission(t *testing.T) {
	socketPath, store, cancel := startTempDaemon(t)
	defer cancel()

	emitter := NewOpenCodeEmitter("opencode", "0.1.0", "/tmp/test", socketPath)
	defer emitter.Close()

	ctx := context.Background()

	// Start a run.
	runID, err := emitter.StartRun(ctx, agenttelemetry.RunOpts{
		Workspace: "/tmp/test",
	})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	if runID == "" {
		t.Fatal("expected non-empty run_id")
	}

	// Emit a tool.called event.
	payload, _ := json.Marshal(map[string]interface{}{
		"tool_name": "bash",
		"args_hash": "abc123",
	})
	err = emitter.EmitEvent(ctx, agenttelemetry.Event{
		EventType: "tool.called",
		RunID:     runID,
		Payload:   json.RawMessage(payload),
	})
	if err != nil {
		t.Fatalf("EmitEvent: %v", err)
	}

	// Complete the run.
	err = emitter.CompleteRun(ctx, runID)
	if err != nil {
		t.Fatalf("CompleteRun: %v", err)
	}

	// Close the emitter to flush.
	emitter.Close()

	// Poll for events to appear.
	var count int
	for i := 0; i < 10; i++ {
		time.Sleep(50 * time.Millisecond)
		count, err = store.EventCount(ctx, runID)
		if err != nil {
			t.Fatalf("EventCount: %v", err)
		}
		if count >= 3 {
			break
		}
	}
	if count < 3 {
		t.Errorf("expected at least 3 events for run %q, got %d", runID, count)
	}
}

func TestOpenCodeEmitterAllEventTypes(t *testing.T) {
	socketPath, store, cancel := startTempDaemon(t)
	defer cancel()

	emitter := NewOpenCodeEmitter("opencode", "0.1.0", "/tmp/test", socketPath)
	defer emitter.Close()

	ctx := context.Background()

	// Start a new run for each type test by using StartRun.
	runID, err := emitter.StartRun(ctx, agenttelemetry.RunOpts{Workspace: "/tmp/test"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	// Emit all 9 event types defined in the spec.
	eventTypes := []string{
		"prompt.submitted",
		"response.received",
		"model.usage",
		"step.started",
		"step.completed",
		"tool.called",
		"run.failed",
		"run.started",
		"run.completed",
	}

	for _, evtType := range eventTypes {
		payload, _ := json.Marshal(map[string]string{"test": evtType})
		err := emitter.EmitEvent(ctx, agenttelemetry.Event{
			EventType: evtType,
			RunID:     runID,
			Payload:   json.RawMessage(payload),
		})
		if err != nil {
			t.Errorf("EmitEvent(%q): %v", evtType, err)
		}
	}

	emitter.Close()

	// Poll for all events.
	var totalCount int
	for i := 0; i < 10; i++ {
		time.Sleep(50 * time.Millisecond)
		totalCount, err = store.EventCount(ctx, runID)
		if err != nil {
			t.Fatalf("EventCount: %v", err)
		}
		if totalCount >= 10 { // run.started + 9 emitted
			break
		}
	}

	// Verify each type was stored (at least the ones we emitted).
	for _, evtType := range eventTypes {
		count, err := store.EventCountByType(ctx, runID, evtType)
		if err != nil {
			t.Errorf("EventCountByType(%q): %v", evtType, err)
			continue
		}
		if count < 1 {
			t.Errorf("expected at least 1 event of type %q, got %d", evtType, count)
		}
	}
}

func TestOpenCodeEmitterCorrelationChain(t *testing.T) {
	socketPath, store, cancel := startTempDaemon(t)
	defer cancel()

	emitter := NewOpenCodeEmitter("opencode", "0.1.0", "/tmp/test", socketPath)
	defer emitter.Close()

	ctx := context.Background()

	// Start run — sets first correlation point.
	runID, err := emitter.StartRun(ctx, agenttelemetry.RunOpts{Workspace: "/tmp/test"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	// Emit 3 events in sequence.
	for i := 0; i < 3; i++ {
		payload, _ := json.Marshal(map[string]int{"seq": i})
		err := emitter.EmitEvent(ctx, agenttelemetry.Event{
			EventType: "tool.called",
			RunID:     runID,
			Payload:   json.RawMessage(payload),
		})
		if err != nil {
			t.Fatalf("EmitEvent %d: %v", i, err)
		}
	}

	emitter.Close()

	// Poll for events.
	var events []agenttelemetry.EventRow
	for i := 0; i < 10; i++ {
		time.Sleep(50 * time.Millisecond)
		events, err = store.GetEventsByRun(ctx, runID)
		if err != nil {
			t.Fatalf("GetEventsByRun: %v", err)
		}
		if len(events) >= 4 { // run.started + 3 tool.called
			break
		}
	}

	if len(events) < 4 {
		t.Fatalf("expected at least 4 events, got %d", len(events))
	}

	// First event should have no correlation_id.
	if events[0].CorrelationID != nil {
		t.Errorf("first event should have nil correlation_id, got %v", *events[0].CorrelationID)
	}
	// Subsequent events should reference the previous event.
	for i := 1; i < len(events); i++ {
		if events[i].CorrelationID == nil {
			t.Errorf("event %d should have correlation_id referencing event %d", i, i-1)
		} else if *events[i].CorrelationID != events[i-1].EventID {
			t.Errorf("event %d correlation_id=%q, expected %q",
				i, *events[i].CorrelationID, events[i-1].EventID)
		}
	}
}

func TestOpenCodeEmitterDaemonUnreachable(t *testing.T) {
	// Use a socket path that doesn't exist.
	socketPath := filepath.Join(t.TempDir(), "nonexistent", "test.sock")

	emitter := NewOpenCodeEmitter("opencode", "0.1.0", "/tmp/test", socketPath)
	defer emitter.Close()

	ctx := context.Background()

	// StartRun should succeed (buffered, no error returned).
	runID, err := emitter.StartRun(ctx, agenttelemetry.RunOpts{Workspace: "/tmp/test"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	if runID == "" {
		t.Fatal("expected non-empty run_id even when buffered")
	}

	// Emit several events (should buffer without error).
	payload, _ := json.Marshal(map[string]string{"test": "buffered"})
	for i := 0; i < 5; i++ {
		err := emitter.EmitEvent(ctx, agenttelemetry.Event{
			EventType: "tool.called",
			RunID:     runID,
			Payload:   json.RawMessage(payload),
		})
		if err != nil {
			t.Errorf("EmitEvent %d: %v (should buffer without error)", i, err)
		}
	}

	// CompleteRun should also succeed (buffered).
	err = emitter.CompleteRun(ctx, runID)
	if err != nil {
		t.Fatalf("CompleteRun: %v", err)
	}

	// FailRun with an error message.
	err = emitter.FailRun(ctx, runID, &testError{msg: "simulated crash"})
	if err != nil {
		t.Fatalf("FailRun: %v", err)
	}

	// Close should attempt flush but may fail silently — should not panic.
	emitter.Close()
}

func TestOpenCodeEmitterFailRun(t *testing.T) {
	socketPath, store, cancel := startTempDaemon(t)
	defer cancel()

	emitter := NewOpenCodeEmitter("opencode", "0.1.0", "/tmp/test", socketPath)
	defer emitter.Close()

	ctx := context.Background()

	runID, err := emitter.StartRun(ctx, agenttelemetry.RunOpts{Workspace: "/tmp/test"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	// Simulate a failure with an error message.
	err = emitter.FailRun(ctx, runID, &testError{msg: "simulated crash"})
	if err != nil {
		t.Fatalf("FailRun: %v", err)
	}

	emitter.Close()

	// Poll for run.failed event.
	for i := 0; i < 10; i++ {
		time.Sleep(50 * time.Millisecond)
		count, err := store.EventCountByType(ctx, runID, "run.failed")
		if err != nil {
			t.Fatalf("EventCountByType: %v", err)
		}
		if count >= 1 {
			return
		}
	}
	t.Error("expected run.failed event in store")
}

// testError is a simple error type for testing.
type testError struct {
	msg string
}

func (e *testError) Error() string { return e.msg }

func TestOpenCodeEmitterEmitUsageRejectsUnboundedIdentity(t *testing.T) {
	emitter := NewOpenCodeEmitter("opencode", "0.1.0", t.TempDir(), filepath.Join(t.TempDir(), "missing.sock"))
	if err := emitter.EmitUsage(context.Background(), ProviderUsage{Provider: "opencode", Model: "gpt-5"}); err == nil {
		t.Fatal("usage without bounded sample and interaction identity must fail")
	}
}

func TestOpenCodeEmitterEmitUsageWritesBoundedProviderEvent(t *testing.T) {
	socketPath, store, cancel := startTempDaemon(t)
	defer cancel()
	emitter := NewOpenCodeEmitter("opencode", "0.1.0", t.TempDir(), socketPath)
	defer emitter.Close()
	runID, err := emitter.StartRun(context.Background(), agenttelemetry.RunOpts{})
	if err != nil {
		t.Fatal(err)
	}
	input, output := int64(12), int64(3)
	if err := emitter.EmitUsage(context.Background(), ProviderUsage{SampleID: "sample", InteractionID: "turn", Provider: "opencode", Model: "gpt-5", Effort: "high", Input: &input, Output: &output}); err != nil {
		t.Fatal(err)
	}
	emitter.Close()
	for range 10 {
		time.Sleep(20 * time.Millisecond)
		count, err := store.EventCountByType(context.Background(), runID, "model.usage")
		if err != nil {
			t.Fatal(err)
		} else if count == 1 {
			return
		}
	}
	t.Fatal("bounded provider usage event was not persisted")
}
