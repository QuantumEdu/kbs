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
	if !strings.Contains(stderr.String(), "--repo is required or an active project must be set") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestCLIProjectManagement(t *testing.T) {
	t.Parallel()
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	cli := testCLI(t, stdout, stderr, &fakeExecutor{})
	db := filepath.Join(t.TempDir(), "line.db")

	// 1. Add first project (should become active automatically)
	code := cli.Run(context.Background(), []string{
		"project", "add", "web", "--name", "Web Frontend", "--repo", "/repos/web", "--db", db,
	})
	if code != 0 {
		t.Fatalf("add web exit %d: %s", code, stderr)
	}
	if !strings.Contains(stdout.String(), "project web added (active)") {
		t.Fatalf("expected web to be active: %s", stdout)
	}

	// 2. Add second project (should NOT become active)
	stdout.Reset()
	code = cli.Run(context.Background(), []string{
		"project", "add", "api", "--name", "API Server", "--repo", "/repos/api", "--db", db,
	})
	if code != 0 {
		t.Fatalf("add api exit %d: %s", code, stderr)
	}
	if !strings.Contains(stdout.String(), "project api added") {
		t.Fatalf("expected api added: %s", stdout)
	}

	// 3. Current should show web
	stdout.Reset()
	code = cli.Run(context.Background(), []string{"project", "current", "--db", db})
	if code != 0 {
		t.Fatalf("current exit %d: %s", code, stderr)
	}
	if !strings.Contains(stdout.String(), "active project: web") {
		t.Fatalf("expected web as active: %s", stdout)
	}

	// 4. List should show both with * on web
	stdout.Reset()
	code = cli.Run(context.Background(), []string{"project", "list", "--db", db})
	if code != 0 {
		t.Fatalf("list exit %d: %s", code, stderr)
	}
	out := stdout.String()
	if !strings.Contains(out, "* web") || !strings.Contains(out, "api") {
		t.Fatalf("list output unexpected: %s", out)
	}

	// 5. Switch to api
	stdout.Reset()
	code = cli.Run(context.Background(), []string{"project", "use", "api", "--db", db})
	if code != 0 {
		t.Fatalf("use api exit %d: %s", code, stderr)
	}
	if !strings.Contains(stdout.String(), "switched to project api") {
		t.Fatalf("expected switched to api: %s", stdout)
	}

	// 6. Current should now show api
	stdout.Reset()
	code = cli.Run(context.Background(), []string{"project", "current", "--db", db})
	if code != 0 {
		t.Fatalf("current exit %d: %s", code, stderr)
	}
	if !strings.Contains(stdout.String(), "active project: api") {
		t.Fatalf("expected api as active: %s", stdout)
	}

	// 7. Remove web
	stdout.Reset()
	code = cli.Run(context.Background(), []string{"project", "remove", "web", "--db", db})
	if code != 0 {
		t.Fatalf("remove web exit %d: %s", code, stderr)
	}
	if !strings.Contains(stdout.String(), "removed project web") {
		t.Fatalf("expected removed web: %s", stdout)
	}

	// 8. List should only show api
	stdout.Reset()
	code = cli.Run(context.Background(), []string{"project", "list", "--db", db})
	if code != 0 {
		t.Fatalf("list exit %d: %s", code, stderr)
	}
	out = stdout.String()
	if strings.Contains(out, "web") || !strings.Contains(out, "* api") {
		t.Fatalf("expected only api in list: %s", out)
	}
}

func TestCLIRunActiveProjectFallback(t *testing.T) {
	t.Parallel()
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	exec := &fakeExecutor{stdout: `{"status":"ok","summary":"done"}`}
	cli := testCLI(t, stdout, stderr, exec)
	db := filepath.Join(t.TempDir(), "line.db")
	repoDir := t.TempDir()

	// Register project
	if code := cli.Run(context.Background(), []string{
		"project", "add", "core", "--repo", repoDir, "--exec", "claude", "--db", db,
	}); code != 0 {
		t.Fatalf("add exit %d: %s", code, stderr)
	}

	// Run without --repo
	stdout.Reset()
	stderr.Reset()
	code := cli.Run(context.Background(), []string{
		"run", "--db", db, "https://github.com/o/r/issues/50",
	})
	if code != 0 {
		t.Fatalf("run fallback exit %d stderr: %s", code, stderr)
	}
	if !strings.Contains(stdout.String(), "issue https://github.com/o/r/issues/50") {
		t.Fatalf("expected run job output: %s", stdout)
	}
}

func TestCLITUIHandler(t *testing.T) {
	t.Parallel()
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	cli := testCLI(t, stdout, stderr, &fakeExecutor{})
	called := false
	cli.RunTUI = func(ctx context.Context, engine *Engine) error {
		called = true
		if engine == nil {
			t.Fatal("nil engine passed to RunTUI")
		}
		return nil
	}
	db := filepath.Join(t.TempDir(), "line.db")
	code := cli.Run(context.Background(), []string{"tui", "--db", db})
	if code != 0 {
		t.Fatalf("tui exit %d stderr: %s", code, stderr)
	}
	if !called {
		t.Fatal("expected RunTUI to be called")
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
