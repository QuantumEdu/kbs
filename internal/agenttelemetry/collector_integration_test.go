//go:build integration

package agenttelemetry

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestCollectorIntegration_EndToEnd starts a real Collector + Store on a temp
// socket, connects a client, sends a valid run.started event, and verifies the
// acknowledgment and that the event exists in the database.
func TestCollectorIntegration_EndToEnd(t *testing.T) {
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

	eventJSON := `{"event_id":"evt-e2e-001","event_type":"run.started","timestamp":"2026-07-22T10:00:00Z","run_id":"run-e2e","agent_id":"test-agent","agent_version":"0.1.0","source":"plugin","redaction_policy":"hash-args","confidence_level":"measured","payload":{"workspace":"/tmp/test"}}` + "\n"
	if _, err := conn.Write([]byte(eventJSON)); err != nil {
		t.Fatalf("write: %v", err)
	}

	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read ack: %v", err)
	}
	if line != AckOK {
		t.Errorf("expected ok ack, got %q", line)
	}

	// Verify event exists in DB.
	var count int
	for i := 0; i < 10; i++ {
		time.Sleep(50 * time.Millisecond)
		store.db.QueryRow("SELECT COUNT(*) FROM events WHERE id = ?", "evt-e2e-001").Scan(&count)
		if count == 1 {
			break
		}
	}
	if count != 1 {
		t.Errorf("expected 1 event in store, got %d", count)
	}

	cancel()
	collector.Shutdown(context.Background())
}

// TestCollectorIntegration_InvalidEvent sends a malformed event (missing
// required fields) and verifies an error acknowledgment and that the event
// is NOT stored in the database.
func TestCollectorIntegration_InvalidEvent(t *testing.T) {
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

	// Missing run_id.
	eventJSON := `{"event_id":"evt-bad","event_type":"run.started","timestamp":"2026-07-22T10:00:00Z","agent_id":"test-agent","source":"plugin","redaction_policy":"hash-args","confidence_level":"measured","payload":{}}` + "\n"
	conn.Write([]byte(eventJSON))

	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read ack: %v", err)
	}
	if line == AckOK {
		t.Error("expected error ack for invalid event, got ok")
	}

	// Verify event NOT in DB.
	var count int
	store.db.QueryRow("SELECT COUNT(*) FROM events WHERE id = ?", "evt-bad").Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 events for malformed event, got %d", count)
	}

	cancel()
	collector.Shutdown(context.Background())
}

// TestCollectorIntegration_MultipleEvents sends 5 events in sequence and
// verifies all 5 are stored in the database.
func TestCollectorIntegration_MultipleEvents(t *testing.T) {
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

	events := []string{
		`{"event_id":"evt-seq-1","event_type":"run.started","timestamp":"2026-07-22T10:00:00Z","run_id":"run-seq","agent_id":"test-agent","agent_version":"0.1.0","source":"plugin","redaction_policy":"hash-args","confidence_level":"measured","payload":{"workspace":"/tmp"}}`,
		`{"event_id":"evt-seq-2","event_type":"tool.called","timestamp":"2026-07-22T10:01:00Z","run_id":"run-seq","agent_id":"test-agent","agent_version":"0.1.0","source":"plugin","redaction_policy":"hash-args","confidence_level":"measured","payload":{"tool_name":"bash","args_hash":"abc"}}`,
		`{"event_id":"evt-seq-3","event_type":"tool.completed","timestamp":"2026-07-22T10:01:05Z","run_id":"run-seq","agent_id":"test-agent","agent_version":"0.1.0","source":"plugin","redaction_policy":"hash-args","confidence_level":"measured","payload":{"tool_name":"bash","success":true}}`,
		`{"event_id":"evt-seq-4","event_type":"model.usage","timestamp":"2026-07-22T10:02:00Z","run_id":"run-seq","agent_id":"test-agent","agent_version":"0.1.0","source":"plugin","redaction_policy":"hash-args","confidence_level":"measured","payload":{"model":"gpt-4","input_tokens":100,"output_tokens":50}}`,
		`{"event_id":"evt-seq-5","event_type":"run.completed","timestamp":"2026-07-22T10:03:00Z","run_id":"run-seq","agent_id":"test-agent","agent_version":"0.1.0","source":"plugin","redaction_policy":"hash-args","confidence_level":"measured","payload":{"status":"completed"}}`,
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

	// Verify all 5 events.
	var count int
	for i := 0; i < 10; i++ {
		time.Sleep(50 * time.Millisecond)
		store.db.QueryRow("SELECT COUNT(*) FROM events WHERE run_id = ?", "run-seq").Scan(&count)
		if count == 5 {
			break
		}
	}
	if count != 5 {
		t.Errorf("expected 5 events, got %d", count)
	}

	cancel()
	collector.Shutdown(context.Background())
}

// TestCollectorIntegration_ConcurrentClients connects 3 concurrent clients,
// each sending 3 events, and verifies all 9 are stored in the database.
func TestCollectorIntegration_ConcurrentClients(t *testing.T) {
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

	var wg sync.WaitGroup
	errCh := make(chan error, 9)
	eventTypes := []string{"run.started", "tool.called", "model.usage"}

	for clientID := 0; clientID < 3; clientID++ {
		wg.Add(1)
		go func(cid int) {
			defer wg.Done()

			conn, err := net.Dial("unix", socketPath)
			if err != nil {
				errCh <- fmt.Errorf("client %d dial: %v", cid, err)
				return
			}
			defer conn.Close()

			reader := bufio.NewReader(conn)
			runID := fmt.Sprintf("run-concurrent-%d", cid)
			for i, etype := range eventTypes {
				evtID := fmt.Sprintf("evt-cc-%d-%d", cid, i)
				evt := map[string]interface{}{
					"event_id":         evtID,
					"event_type":       etype,
					"timestamp":        "2026-07-22T10:00:00Z",
					"run_id":           runID,
					"agent_id":         "test-agent",
					"agent_version":    "0.1.0",
					"source":           "plugin",
					"redaction_policy": "hash-args",
					"confidence_level": "measured",
					"payload":          json.RawMessage(`{}`),
				}
				data, _ := json.Marshal(evt)
				data = append(data, '\n')

				// Retry on SQLITE_BUSY — concurrent writes to SQLite may
				// temporarily lock the database under heavy contention.
				var lastErr error
				for attempt := 0; attempt < 5; attempt++ {
					if attempt > 0 {
						time.Sleep(20 * time.Millisecond)
					}
					if _, err := conn.Write(data); err != nil {
						lastErr = fmt.Errorf("client %d write: %v", cid, err)
						continue
					}
					line, err := reader.ReadString('\n')
					if err != nil {
						lastErr = fmt.Errorf("client %d read: %v", cid, err)
						continue
					}
					if line == AckOK {
						lastErr = nil
						break
					}
					if strings.Contains(line, "SQLITE_BUSY") {
						lastErr = nil // retry
						continue
					}
					lastErr = fmt.Errorf("client %d ack: %q", cid, line)
					break
				}
				if lastErr != nil {
					errCh <- lastErr
					return
				}
			}
		}(clientID)
	}

	wg.Wait()
	close(errCh)

	for e := range errCh {
		t.Error(e)
	}

	// Verify all 9 events.
	var count int
	for i := 0; i < 10; i++ {
		time.Sleep(50 * time.Millisecond)
		store.db.QueryRow("SELECT COUNT(*) FROM events").Scan(&count)
		if count == 9 {
			break
		}
	}
	if count != 9 {
		t.Errorf("expected 9 events from concurrent clients, got %d", count)
	}

	cancel()
	collector.Shutdown(context.Background())
}
