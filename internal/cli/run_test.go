package cli

import (
	"io"
	"os"
	"strings"
	"testing"
)

// captureStdout redirects os.Stdout to a pipe and returns a function that
// restores it and returns everything written since capture began.
func captureStdout(t *testing.T) func() string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	return func() string {
		_ = w.Close()
		os.Stdout = old
		out, _ := io.ReadAll(r)
		return string(out)
	}
}

func TestRunDispatchesToHandler(t *testing.T) {
	dir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	defer os.Setenv("HOME", origHome)

	RunInit()

	restore := captureStdout(t)
	Run("add-project", []string{"skillvault", "add-project", "--name", "codex"})
	out := restore()

	if !strings.Contains(out, "Project saved:") {
		t.Fatalf("expected add-project dispatch output, got: %q", out)
	}
	if !strings.Contains(out, "codex") {
		t.Fatalf("expected project name in dispatch output, got: %q", out)
	}
}
