package agenttelemetry

import (
	"bufio"
	"context"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func testCollectorSocketPath(t *testing.T) string {
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

func startTestCollector(t *testing.T, collector *Collector, socketPath string) (context.CancelFunc, <-chan error) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- collector.Listen(ctx)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for {
		conn, err := net.DialTimeout("unix", socketPath, 25*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return cancel, errCh
		}

		select {
		case listenErr := <-errCh:
			cancel()
			if listenErr == nil {
				t.Fatal("collector listener exited before becoming ready")
			}
			t.Fatalf("collector listen: %v", listenErr)
		default:
		}

		if time.Now().After(deadline) {
			cancel()
			t.Fatalf("collector socket %q was not ready before timeout: %v", socketPath, err)
		}

		time.Sleep(10 * time.Millisecond)
	}
}

func stopTestCollector(t *testing.T, collector *Collector, cancel context.CancelFunc, errCh <-chan error) {
	t.Helper()

	cancel()
	if err := collector.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("collector listen: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("collector listener did not exit")
	}
}

func TestCollectorIngestValidEvent(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer store.Close()

	socketPath := testCollectorSocketPath(t)
	collector := NewCollector(store, socketPath)

	cancel, errCh := startTestCollector(t, collector, socketPath)
	defer stopTestCollector(t, collector, cancel, errCh)

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

}

func TestCollectorIngestInvalidEvent(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer store.Close()

	socketPath := testCollectorSocketPath(t)
	collector := NewCollector(store, socketPath)

	cancel, errCh := startTestCollector(t, collector, socketPath)
	defer stopTestCollector(t, collector, cancel, errCh)

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Send invalid JSON (missing run_id).
	eventJSON := `{"event_id":"evt-001","event_type":"run.started","timestamp":"2026-07-22T10:00:00Z","agent_id":"opencode","source":"plugin","redaction_policy":"hash-args","confidence_level":"measured","payload":{}}` + "\n"
	if _, err := conn.Write([]byte(eventJSON)); err != nil {
		t.Fatalf("write: %v", err)
	}

	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read ack: %v", err)
	}
	// Should receive error ack.
	if line == `{"status":"ok"}`+"\n" {
		t.Error("expected error ack for invalid event, got ok")
	}
}

func TestCollectorMultipleEvents(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer store.Close()

	socketPath := testCollectorSocketPath(t)
	collector := NewCollector(store, socketPath)

	cancel, errCh := startTestCollector(t, collector, socketPath)
	defer stopTestCollector(t, collector, cancel, errCh)

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
}

func TestCollectorMalformedJSON(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer store.Close()

	socketPath := testCollectorSocketPath(t)
	collector := NewCollector(store, socketPath)

	cancel, errCh := startTestCollector(t, collector, socketPath)
	defer stopTestCollector(t, collector, cancel, errCh)

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Send garbage.
	if _, err := conn.Write([]byte("not-json\n")); err != nil {
		t.Fatalf("write: %v", err)
	}

	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read ack: %v", err)
	}
	if line == AckOK {
		t.Error("expected error ack for garbage input")
	}
}

func TestCollectorShutdown(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer store.Close()

	socketPath := testCollectorSocketPath(t)
	collector := NewCollector(store, socketPath)

	cancel, errCh := startTestCollector(t, collector, socketPath)

	// Shutdown should close the socket.
	cancel()
	if err := collector.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	// Socket file should be removed.
	if _, err := os.Stat(socketPath); !os.IsNotExist(err) {
		t.Error("socket file should be removed after shutdown")
	}

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("collector listen: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("collector listener did not exit")
	}
}
