package agenttelemetry

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestCaptureGitContext_NonGitDir(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	repoURL, branch, commitSHA := CaptureGitContext()

	if repoURL != "" {
		t.Errorf("expected empty repoURL in non-git dir, got %q", repoURL)
	}
	if branch != "" {
		t.Errorf("expected empty branch in non-git dir, got %q", branch)
	}
	if commitSHA != "" {
		t.Errorf("expected empty commitSHA in non-git dir, got %q", commitSHA)
	}
}

func TestCaptureGitContext_GitRepo(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	initGitRepo(t, dir)
	repoURL, branch, commitSHA := CaptureGitContext()

	if repoURL == "" {
		t.Error("expected non-empty repoURL in git repo")
	}
	if branch == "" {
		t.Error("expected non-empty branch in git repo")
	}
	if commitSHA == "" || len(commitSHA) != 40 {
		t.Errorf("expected 40-char commit SHA, got len=%d: %q", len(commitSHA), commitSHA)
	}
}

func TestCaptureGitContext_OptionInjection(t *testing.T) {
	// Threat: arguments starting with - could be interpreted as git flags.
	// CaptureGitContext must use -- to separate paths from args.
	dir := t.TempDir()
	t.Chdir(dir)

	initGitRepo(t, dir)
	repoURL, branch, commitSHA := CaptureGitContext()

	if repoURL == "" || branch == "" || commitSHA == "" {
		t.Fatal("expected git context to be captured")
	}
	if strings.Contains(repoURL, "--") || len(commitSHA) != 40 {
		t.Errorf("suspicious git context: repoURL=%q branch=%q commitSHA=%q", repoURL, branch, commitSHA)
	}
}

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	git := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.Output()
		if err != nil {
			var stderr string
			if ee, ok := err.(*exec.ExitError); ok {
				stderr = string(ee.Stderr)
			}
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, stderr)
		}
		_ = out
	}
	git("init")
	git("config", "user.email", "test@example.com")
	git("config", "user.name", "Test")
	git("remote", "add", "origin", "https://github.com/test/example.git")
	os.WriteFile(dir+"/f.txt", []byte("test"), 0o644)
	git("add", "f.txt")
	git("commit", "-m", "init")
}
