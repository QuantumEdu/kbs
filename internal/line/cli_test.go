package line

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIHelp(t *testing.T) {
	t.Parallel()
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	cli := testCLI(t, stdout, stderr, &fakeExecutor{})
	if code := cli.Run(context.Background(), []string{"help"}); code != 0 {
		t.Fatalf("exit %d stderr %s", code, stderr)
	}
	out := stdout.String()
	if !strings.Contains(out, "line serve") || !strings.Contains(out, "ntfy") {
		t.Fatalf("help missing serve/ntfy: %s", out)
	}
}

func TestCLIRunAndStatus(t *testing.T) {
	t.Parallel()
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	exec := &fakeExecutor{stdout: `{"status":"needs_human","question":"Scope?"}`}
	cli := testCLI(t, stdout, stderr, exec)
	db := filepath.Join(t.TempDir(), "line.db")
	code := cli.Run(context.Background(), []string{
		"run", "--db", db, "--repo", t.TempDir(), "https://github.com/o/r/issues/11",
	})
	if code != 0 {
		t.Fatalf("run exit %d stderr %s", code, stderr)
	}
	out := stdout.String()
	if !strings.Contains(out, "status needs_human") || !strings.Contains(out, "question Scope?") {
		t.Fatalf("run output %s", out)
	}
	stdout.Reset()
	if code := cli.Run(context.Background(), []string{"status", "--db", db}); code != 0 {
		t.Fatalf("status exit %d stderr %s", code, stderr)
	}
	if !strings.Contains(stdout.String(), "event plan started") {
		t.Fatalf("status %s", stdout)
	}
}

func TestCLIContinue(t *testing.T) {
	t.Parallel()
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	exec := &fakeExecutor{stdout: `{"status":"needs_human","question":"Scope?"}`}
	cli := testCLI(t, stdout, stderr, exec)
	db := filepath.Join(t.TempDir(), "line.db")
	if code := cli.Run(context.Background(), []string{
		"run", "--db", db, "--repo", t.TempDir(), "https://github.com/o/r/issues/12",
	}); code != 0 {
		t.Fatalf("run: %s", stderr)
	}
	exec.stdout = `{"status":"ok","summary":"scoped"}`
	stdout.Reset()
	if code := cli.Run(context.Background(), []string{
		"continue", "--db", db, "--answer", "auth only",
	}); code != 0 {
		t.Fatalf("continue exit %d stderr %s", code, stderr)
	}
	if !strings.Contains(stdout.String(), "status awaiting_next") {
		t.Fatalf("continue %s", stdout)
	}
}

func TestCLINext(t *testing.T) {
	t.Parallel()
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	exec := &scriptedExecutor{replies: []string{
		`{"status":"ok","summary":"spec"}`,
		`{"status":"ok","summary":"built"}`,
	}}
	cli := testCLI(t, stdout, stderr, exec)
	db := filepath.Join(t.TempDir(), "line.db")
	if code := cli.Run(context.Background(), []string{
		"run", "--auto=false", "--db", db, "--repo", t.TempDir(), "https://github.com/o/r/issues/13",
	}); code != 0 {
		t.Fatalf("run: %s", stderr)
	}
	stdout.Reset()
	if code := cli.Run(context.Background(), []string{"next", "--db", db}); code != 0 {
		t.Fatalf("next exit %d stderr %s", code, stderr)
	}
	if !strings.Contains(stdout.String(), "phase review") {
		t.Fatalf("next %s", stdout)
	}
}

func TestCLIServeUsesHandler(t *testing.T) {
	t.Parallel()
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	called := false
	cli := testCLI(t, stdout, stderr, &fakeExecutor{})
	cli.Serve = func(ctx context.Context, addr string, engine *Engine) error {
		called = true
		if addr != "127.0.0.1:7340" {
			t.Fatalf("addr %s", addr)
		}
		if engine == nil {
			t.Fatal("nil engine")
		}
		return nil
	}
	db := filepath.Join(t.TempDir(), "line.db")
	if code := cli.Run(context.Background(), []string{"serve", "--db", db}); code != 0 {
		t.Fatalf("serve exit %d %s", code, stderr)
	}
	if !called {
		t.Fatal("expected serve handler")
	}
}

func TestCLIRunRequiresRepo(t *testing.T) {
	t.Parallel()
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	cli := testCLI(t, stdout, stderr, &fakeExecutor{})
	code := cli.Run(context.Background(), []string{"run", "https://github.com/o/r/issues/1"})
	if code != 2 {
		t.Fatalf("exit %d", code)
	}
}

func testCLI(t *testing.T, stdout, stderr *bytes.Buffer, exec Executor) CLI {
	t.Helper()
	return CLI{
		Stdout:    stdout,
		Stderr:    stderr,
		OpenStore: OpenStore,
		NewExec:   func(string, []string) Executor { return exec },
		DefaultDB: filepath.Join(t.TempDir(), "default.db"),
	}
}
