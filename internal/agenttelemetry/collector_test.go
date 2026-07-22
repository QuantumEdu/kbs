package agenttelemetry

import (
	"bufio"
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCollectorIngestValidEvent(t *testing.T) {
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

	// Start listener in background.
	errCh := make(chan error, 1)
	go func() {
		errCh <- collector.Listen(ctx)
	}()

	// Wait for socket to be ready.
	time.Sleep(50 * time.Millisecond)

	// Connect client.
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Send a valid event.
	eventJSON := `{"event_id":"evt-001","event_type":"run.started","timestamp":"2026-07-22T10:00:00Z","run_id":"run-001","agent_id":"opencode","agent_version":"0.1.0","source":"plugin","redaction_policy":"hash-args","confidence_level":"measured","payload":{"workspace":"/tmp"}}` + "\n"
	if _, err := conn.Write([]byte(eventJSON)); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Read ack.
	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read ack: %v", err)
	}
	if line != `{"status":"ok"}`+"\n" {
		t.Errorf("unexpected ack: %q", line)
	}

	// Verify event was saved (poll for up to 500ms).
	var count int
	for i := 0; i < 10; i++ {
		time.Sleep(50 * time.Millisecond)
		store.db.QueryRow("SELECT COUNT(*) FROM events WHERE id = ?", "evt-001").Scan(&count)
		if count == 1 {
			break
		}
	}
	if count != 1 {
		t.Errorf("expected 1 event in store, got %d", count)
	}

	// Shutdown.
	cancel()
	collector.Shutdown(context.Background())
}

func TestCollectorIngestInvalidEvent(t *testing.T) {
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

	errCh := make(chan error, 1)
	go func() {
		errCh <- collector.Listen(ctx)
	}()

	time.Sleep(50 * time.Millisecond)

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Send invalid JSON (missing run_id).
	eventJSON := `{"event_id":"evt-001","event_type":"run.started","timestamp":"2026-07-22T10:00:00Z","agent_id":"opencode","source":"plugin","redaction_policy":"hash-args","confidence_level":"measured","payload":{}}` + "\n"
	conn.Write([]byte(eventJSON))

	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read ack: %v", err)
	}
	// Should receive error ack.
	if line == `{"status":"ok"}`+"\n" {
		t.Error("expected error ack for invalid event, got ok")
	}

	cancel()
	collector.Shutdown(context.Background())
}

func TestCollectorMultipleEvents(t *testing.T) {
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

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Send 3 valid events.
	events := []string{
		`{"event_id":"evt-a","event_type":"run.started","timestamp":"2026-07-22T10:00:00Z","run_id":"run-a","agent_id":"opencode","agent_version":"0.1.0","source":"plugin","redaction_policy":"hash-args","confidence_level":"measured","payload":{}}`,
		`{"event_id":"evt-b","event_type":"tool.called","timestamp":"2026-07-22T10:01:00Z","run_id":"run-a","agent_id":"opencode","agent_version":"0.1.0","source":"plugin","redaction_policy":"hash-args","confidence_level":"measured","payload":{"tool_name":"bash","args_hash":"abc"}}`,
		`{"event_id":"evt-c","event_type":"tool.completed","timestamp":"2026-07-22T10:01:05Z","run_id":"run-a","agent_id":"opencode","agent_version":"0.1.0","source":"plugin","redaction_policy":"hash-args","confidence_level":"measured","payload":{"tool_name":"bash","success":true}}`,
	}

	reader := bufio.NewReader(conn)
	for _, eventJSON := range events {
		conn.Write([]byte(eventJSON + "\n"))
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read ack: %v", err)
		}
		if line != AckOK {
			t.Errorf("expected ok ack, got %q", line)
		}
	}

	// Verify all 3 events.
	var count int
	for i := 0; i < 10; i++ {
		time.Sleep(50 * time.Millisecond)
		store.db.QueryRow("SELECT COUNT(*) FROM events").Scan(&count)
		if count == 3 {
			break
		}
	}
	if count != 3 {
		t.Errorf("expected 3 events, got %d", count)
	}

	cancel()
	collector.Shutdown(context.Background())
}

func TestCollectorMalformedJSON(t *testing.T) {
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

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Send garbage.
	conn.Write([]byte("not-json\n"))

	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read ack: %v", err)
	}
	if line == AckOK {
		t.Error("expected error ack for garbage input")
	}

	cancel()
	collector.Shutdown(context.Background())
}

func TestCollectorShutdown(t *testing.T) {
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

	// Shutdown should close the socket.
	if err := collector.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	// Socket file should be removed.
	if _, err := os.Stat(socketPath); !os.IsNotExist(err) {
		t.Error("socket file should be removed after shutdown")
	}
}
