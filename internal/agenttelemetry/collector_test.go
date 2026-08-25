package agenttelemetry

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
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

// newProjectionTestCollector builds a store and collector for run projection
// tests that exercise ingest directly (no socket round-trip).
func newProjectionTestCollector(t *testing.T) (*Store, *Collector) {
	t.Helper()

	store, err := OpenStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	return store, NewCollector(store, filepath.Join(t.TempDir(), "test.sock"))
}

// ingestRaw feeds a raw event line to the collector and requires an ok ack.
func ingestRaw(t *testing.T, c *Collector, raw string) {
	t.Helper()

	if ack := c.ingest(context.Background(), []byte(raw)); ack != AckOK {
		t.Fatalf("ingest ack = %q, want %q", ack, AckOK)
	}
}

// lifecycleEventJSON builds a valid raw lifecycle event with the given type,
// run ID and payload.
func lifecycleEventJSON(eventType, runID, payload string) string {
	return fmt.Sprintf(`{"event_id":"evt-%s-%s","event_type":"%s","timestamp":"2026-08-25T10:00:00Z","run_id":"%s","agent_id":"opencode","agent_version":"0.1.0","source":"plugin","redaction_policy":"hash-args","confidence_level":"measured","payload":%s}`,
		runID, eventType, eventType, runID, payload)
}

func TestCollectorIngestRunStartedProjectsRun(t *testing.T) {
	store, collector := newProjectionTestCollector(t)

	started := `{"event_id":"evt-rs-1","event_type":"run.started","timestamp":"2026-08-25T10:00:00Z","run_id":"run-lc","agent_id":"opencode","agent_version":"0.1.0","source":"plugin","redaction_policy":"hash-args","confidence_level":"measured","payload":{"workspace":"/tmp/proj","repo_url":"https://example.com/repo.git","branch":"main","commit_sha":"abc123"}}`
	ingestRaw(t, collector, started)

	run, err := store.GetRun(context.Background(), "run-lc")
	if err != nil {
		t.Fatalf("GetRun after run.started: %v", err)
	}
	wantStartedAt, _ := time.Parse(time.RFC3339, "2026-08-25T10:00:00Z")
	if run.Status != "running" {
		t.Errorf("Status = %q, want %q", run.Status, "running")
	}
	if run.AgentID != "opencode" {
		t.Errorf("AgentID = %q, want %q", run.AgentID, "opencode")
	}
	if run.AgentVersion != "0.1.0" {
		t.Errorf("AgentVersion = %q, want %q", run.AgentVersion, "0.1.0")
	}
	if !run.StartedAt.Equal(wantStartedAt) {
		t.Errorf("StartedAt = %v, want %v", run.StartedAt, wantStartedAt)
	}
	if run.Workspace != "/tmp/proj" {
		t.Errorf("Workspace = %q, want %q", run.Workspace, "/tmp/proj")
	}
	if run.RepoURL != "https://example.com/repo.git" || run.Branch != "main" || run.CommitSHA != "abc123" {
		t.Errorf("git fields not projected: repo=%q branch=%q sha=%q", run.RepoURL, run.Branch, run.CommitSHA)
	}
}

func TestCollectorIngestWrapperCwdMapsToWorkspace(t *testing.T) {
	store, collector := newProjectionTestCollector(t)

	// telemetrywrap emits {"command":..., "cwd":...} on run.started.
	wrapperStarted := `{"event_id":"evt-wrap-1","event_type":"run.started","timestamp":"2026-08-25T10:05:00Z","run_id":"run-wrap","agent_id":"claude-cli","agent_version":"","source":"wrapper","redaction_policy":"hash-args","confidence_level":"heuristic","payload":{"command":"make test","cwd":"/home/dev/kbs"}}`
	ingestRaw(t, collector, wrapperStarted)

	run, err := store.GetRun(context.Background(), "run-wrap")
	if err != nil {
		t.Fatalf("GetRun after wrapper run.started: %v", err)
	}
	if run.Workspace != "/home/dev/kbs" {
		t.Errorf("Workspace = %q, want %q (cwd fallback)", run.Workspace, "/home/dev/kbs")
	}
}

func TestCollectorIngestRunCompletedPreservesStartFields(t *testing.T) {
	store, collector := newProjectionTestCollector(t)

	started := lifecycleEventJSON("run.started", "run-lc", `{"workspace":"/tmp/proj","repo_url":"https://example.com/repo.git","branch":"main","commit_sha":"abc123"}`)
	completed := `{"event_id":"evt-rc-1","event_type":"run.completed","timestamp":"2026-08-25T10:02:00Z","run_id":"run-lc","agent_id":"opencode","agent_version":"0.1.0","source":"plugin","redaction_policy":"hash-args","confidence_level":"measured","payload":{"status":"completed"}}`
	ingestRaw(t, collector, started)
	ingestRaw(t, collector, completed)

	run, err := store.GetRun(context.Background(), "run-lc")
	if err != nil {
		t.Fatalf("GetRun after run.completed: %v", err)
	}
	wantCompletedAt, _ := time.Parse(time.RFC3339, "2026-08-25T10:02:00Z")
	if run.Status != "completed" {
		t.Errorf("Status = %q, want %q", run.Status, "completed")
	}
	if run.CompletedAt == nil || !run.CompletedAt.Equal(wantCompletedAt) {
		t.Errorf("CompletedAt = %v, want %v", run.CompletedAt, wantCompletedAt)
	}
	// Fields recorded at start must survive the merge.
	if run.Workspace != "/tmp/proj" {
		t.Errorf("Workspace lost in merge: %q", run.Workspace)
	}
	if run.RepoURL != "https://example.com/repo.git" || run.Branch != "main" || run.CommitSHA != "abc123" {
		t.Errorf("start fields lost in merge: repo=%q branch=%q sha=%q", run.RepoURL, run.Branch, run.CommitSHA)
	}
	if run.AgentVersion != "0.1.0" {
		t.Errorf("AgentVersion lost in merge: %q", run.AgentVersion)
	}
}

func TestCollectorIngestLateStartDoesNotResurrectTerminalRun(t *testing.T) {
	store, collector := newProjectionTestCollector(t)

	// Instant commands (e.g. `echo`) can deliver run.completed before
	// run.started because the wrapper dials a fresh connection per event.
	completed := lifecycleEventJSON("run.completed", "run-inv", `{}`)
	lateStarted := lifecycleEventJSON("run.started", "run-inv", `{"cwd":"/tmp/proj"}`)
	ingestRaw(t, collector, completed)
	ingestRaw(t, collector, lateStarted)

	run, err := store.GetRun(context.Background(), "run-inv")
	if err != nil {
		t.Fatalf("GetRun after inverted lifecycle: %v", err)
	}
	if run.Status != "completed" {
		t.Errorf("Status = %q, want %q (late start must not resurrect a terminal run)", run.Status, "completed")
	}
	if run.CompletedAt == nil {
		t.Error("CompletedAt = nil, want set")
	}
}

func TestCollectorIngestRunFailedSetsStatusAndError(t *testing.T) {
	store, collector := newProjectionTestCollector(t)

	started := lifecycleEventJSON("run.started", "run-lc-fail", `{}`)
	failed := `{"event_id":"evt-rf-1","event_type":"run.failed","timestamp":"2026-08-25T10:03:00Z","run_id":"run-lc-fail","agent_id":"opencode","agent_version":"0.1.0","source":"wrapper","redaction_policy":"hash-args","confidence_level":"heuristic","payload":{"error":"exit status 1","error_type":"ExitError"}}`
	ingestRaw(t, collector, started)
	ingestRaw(t, collector, failed)

	run, err := store.GetRun(context.Background(), "run-lc-fail")
	if err != nil {
		t.Fatalf("GetRun after run.failed: %v", err)
	}
	wantCompletedAt, _ := time.Parse(time.RFC3339, "2026-08-25T10:03:00Z")
	if run.Status != "failed" {
		t.Errorf("Status = %q, want %q", run.Status, "failed")
	}
	if run.CompletedAt == nil || !run.CompletedAt.Equal(wantCompletedAt) {
		t.Errorf("CompletedAt = %v, want %v", run.CompletedAt, wantCompletedAt)
	}
	if run.ErrorMessage != "exit status 1" {
		t.Errorf("ErrorMessage = %q, want %q", run.ErrorMessage, "exit status 1")
	}
	if run.ErrorType != "ExitError" {
		t.Errorf("ErrorType = %q, want %q", run.ErrorType, "ExitError")
	}
}

func TestCollectorIngestTerminalEventWithoutStart(t *testing.T) {
	cases := []struct {
		name       string
		eventType  string
		payload    string
		wantStatus string
		wantErr    string
	}{
		{"completed without start", "run.completed", `{"status":"completed"}`, "completed", ""},
		{"failed without start", "run.failed", `{"error":"boom"}`, "failed", "boom"},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			store, collector := newProjectionTestCollector(t)

			raw := fmt.Sprintf(`{"event_id":"evt-orphan-%s","event_type":"%s","timestamp":"2026-08-25T11:00:00Z","run_id":"run-orphan-%s","agent_id":"opencode","agent_version":"","source":"wrapper","redaction_policy":"hash-args","confidence_level":"heuristic","payload":%s}`,
				tt.wantStatus, tt.eventType, tt.wantStatus, tt.payload)
			ingestRaw(t, collector, raw)

			run, err := store.GetRun(context.Background(), "run-orphan-"+tt.wantStatus)
			if err != nil {
				t.Fatalf("GetRun: %v", err)
			}
			if run.Status != tt.wantStatus {
				t.Errorf("Status = %q, want %q", run.Status, tt.wantStatus)
			}
			if run.CompletedAt == nil {
				t.Error("CompletedAt not set on terminal-only run")
			}
			if run.ErrorMessage != tt.wantErr {
				t.Errorf("ErrorMessage = %q, want %q", run.ErrorMessage, tt.wantErr)
			}
			if run.AgentID != "opencode" {
				t.Errorf("AgentID = %q, want %q", run.AgentID, "opencode")
			}
		})
	}
}

func TestStallDetectorGateSeesRunningRuns(t *testing.T) {
	store, collector := newProjectionTestCollector(t)
	collector.SetStallDetector(NewStallDetector())

	// Project a running run.
	started := lifecycleEventJSON("run.started", "run-stall", `{}`)
	ingestRaw(t, collector, started)

	ctx := context.Background()
	run, err := store.GetRun(ctx, "run-stall")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}

	// The stall detector gate in ingest() fires only when
	// GetRun succeeds AND Status == "running". Before this fix both halves
	// failed because agent_runs was always empty.
	if run.Status != "running" {
		t.Fatalf("gate precondition: Status = %q, want %q", run.Status, "running")
	}

	// Signal half of the gate: a run idle past the threshold must yield a signal.
	sd := NewStallDetector()
	sd.Record("run-stall", time.Now().Add(-2*time.Minute))
	if signal := sd.Check("run-stall"); signal == nil {
		t.Fatal("expected stall signal for run idle past threshold")
	}
}

func TestRunStartFromPayload(t *testing.T) {
	cases := []struct {
		name          string
		payload       string
		wantWorkspace string
		wantRepoURL   string
		wantBranch    string
		wantCommitSHA string
	}{
		{
			name:          "plugin RunOpts payload",
			payload:       `{"workspace":"/w","repo_url":"https://r.git","branch":"b","commit_sha":"s"}`,
			wantWorkspace: "/w",
			wantRepoURL:   "https://r.git",
			wantBranch:    "b",
			wantCommitSHA: "s",
		},
		{
			name:          "wrapper cwd fallback",
			payload:       `{"command":"ls","cwd":"/cwd/dir"}`,
			wantWorkspace: "/cwd/dir",
		},
		{
			name:          "workspace wins over cwd",
			payload:       `{"workspace":"/w","cwd":"/c"}`,
			wantWorkspace: "/w",
		},
		{
			name:          "missing keys stay empty",
			payload:       `{"command":"ls"}`,
			wantWorkspace: "",
		},
		{
			name:          "invalid json yields zero value",
			payload:       `not-json`,
			wantWorkspace: "",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := runStartFromPayload(json.RawMessage(tt.payload))
			if got.Workspace != tt.wantWorkspace {
				t.Errorf("Workspace = %q, want %q", got.Workspace, tt.wantWorkspace)
			}
			if got.RepoURL != tt.wantRepoURL {
				t.Errorf("RepoURL = %q, want %q", got.RepoURL, tt.wantRepoURL)
			}
			if got.Branch != tt.wantBranch {
				t.Errorf("Branch = %q, want %q", got.Branch, tt.wantBranch)
			}
			if got.CommitSHA != tt.wantCommitSHA {
				t.Errorf("CommitSHA = %q, want %q", got.CommitSHA, tt.wantCommitSHA)
			}
		})
	}
}

func TestFailureFromPayload(t *testing.T) {
	cases := []struct {
		name        string
		payload     string
		wantErrType string
		wantErrMsg  string
	}{
		{"producer error key only", `{"error":"exit status 1"}`, "", "exit status 1"},
		{"error with type", `{"error":"boom","error_type":"Timeout"}`, "Timeout", "boom"},
		{"empty object", `{}`, "", ""},
		{"invalid json", `nope`, "", ""},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			errType, errMsg := failureFromPayload(json.RawMessage(tt.payload))
			if errType != tt.wantErrType {
				t.Errorf("ErrorType = %q, want %q", errType, tt.wantErrType)
			}
			if errMsg != tt.wantErrMsg {
				t.Errorf("ErrorMessage = %q, want %q", errMsg, tt.wantErrMsg)
			}
		})
	}
}
