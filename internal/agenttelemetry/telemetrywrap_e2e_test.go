//go:build integration

package agenttelemetry

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// TestTelemetrywrapE2E_Smoke builds the telemetrywrap binary, starts a real
// daemon, runs telemetrywrap wrapping a trivial echo command, and verifies
// that events were written to the daemon's database.
//
// This test is skipped when -short is passed because it requires building
// a Go binary.
func TestTelemetrywrapE2E_Smoke(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode (binary build required)")
	}

	// Locate the project root from the current working directory.
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	// Walk up to find go.mod.
	projectRoot := findProjectRoot(t, cwd)
	if projectRoot == "" {
		t.Skip("cannot locate project root (go.mod not found)")
	}

	// Build telemetrywrap binary.
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "telemetrywrap")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "./internal/agenttelemetry/telemetrywrap")
	buildCmd.Dir = projectRoot
	buildOut, err := buildCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build telemetrywrap: %v\n%s", err, buildOut)
	}

	// Start a daemon (Collector + Store) on a temp socket.
	dbPath := filepath.Join(tmpDir, "test.db")
	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer store.Close()

	socketPath := filepath.Join(tmpDir, "test.sock")
	collector := NewCollector(store, socketPath)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go collector.Listen(ctx)
	time.Sleep(50 * time.Millisecond)

	// Run telemetrywrap.
	wrapCmd := exec.Command(binaryPath, "--agent", "test-agent", "--", "echo", "hello")
	wrapCmd.Env = append(os.Environ(), "TELEMETRY_SOCKET="+socketPath)
	wrapOut, err := wrapCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("telemetrywrap: %v\n%s", err, wrapOut)
	}

	// Verify events were written.
	var count int
	for i := 0; i < 20; i++ {
		time.Sleep(100 * time.Millisecond)
		store.db.QueryRow("SELECT COUNT(*) FROM events").Scan(&count)
		if count > 0 {
			break
		}
	}
	if count == 0 {
		t.Error("expected at least 1 event from telemetrywrap, got 0")
	}
	t.Logf("telemetrywrap produced %d events in DB", count)

	// Also verify at least one event has source "wrapper".
	var wrapperCount int
	store.db.QueryRow("SELECT COUNT(*) FROM events WHERE source = 'wrapper'").Scan(&wrapperCount)
	if wrapperCount == 0 {
		t.Error("expected at least 1 event with source='wrapper', got 0")
	}
	t.Logf("wrapper-source events: %d", wrapperCount)

	cancel()
	collector.Shutdown(context.Background())
}

// TestTelemetrywrapE2E_BinaryExitsSuccessfully verifies that the telemetrywrap
// binary exits with code 0 when wrapping a successful command.
func TestTelemetrywrapE2E_BinaryExitsSuccessfully(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode (binary build required)")
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	projectRoot := findProjectRoot(t, cwd)
	if projectRoot == "" {
		t.Skip("cannot locate project root (go.mod not found)")
	}

	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "telemetrywrap")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "./internal/agenttelemetry/telemetrywrap")
	buildCmd.Dir = projectRoot
	buildOut, err := buildCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build telemetrywrap: %v\n%s", err, buildOut)
	}

	// Start daemon.
	dbPath := filepath.Join(tmpDir, "test.db")
	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer store.Close()

	socketPath := filepath.Join(tmpDir, "test.sock")
	collector := NewCollector(store, socketPath)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go collector.Listen(ctx)
	time.Sleep(50 * time.Millisecond)

	// Run telemetrywrap — should exit 0.
	wrapCmd := exec.Command(binaryPath, "--agent", "test-agent", "--", "echo", "hello")
	wrapCmd.Env = append(os.Environ(), "TELEMETRY_SOCKET="+socketPath)
	err = wrapCmd.Run()
	if err != nil {
		t.Fatalf("telemetrywrap exited with error: %v", err)
	}

	cancel()
	collector.Shutdown(context.Background())
}

// findProjectRoot walks up the directory tree looking for go.mod.
func findProjectRoot(t *testing.T, start string) string {
	t.Helper()
	dir := start
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
