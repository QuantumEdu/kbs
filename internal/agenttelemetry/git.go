package agenttelemetry

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var gitExecutable = "git"

type GitSnapshot struct {
	Root, Head, Branch          string
	Detached                    bool
	Staged, Unstaged, Untracked int
	CapturedAt                  time.Time
}

// CaptureGitSnapshot captures repository evidence using an explicit, absolute root.
func CaptureGitSnapshot(root string) (GitSnapshot, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return GitSnapshot{}, err
	}
	if out, err := gitAt(root, "rev-parse", "--is-inside-work-tree"); err != nil || out != "true" {
		return GitSnapshot{}, fmt.Errorf("not a git repository: %s", root)
	}
	head, err := gitAt(root, "rev-parse", "HEAD")
	if err != nil {
		return GitSnapshot{}, err
	}
	branch, _ := gitAt(root, "branch", "--show-current")
	status, err := gitAt(root, "status", "--porcelain=v1")
	if err != nil {
		return GitSnapshot{}, err
	}
	s := GitSnapshot{Root: filepath.Clean(root), Head: head, Branch: branch, Detached: branch == "", CapturedAt: time.Now().UTC()}
	for _, line := range strings.Split(status, "\n") {
		if len(line) < 2 {
			continue
		}
		if line[0] == '?' {
			s.Untracked++
			continue
		}
		if line[0] != ' ' {
			s.Staged++
		}
		if line[1] != ' ' {
			s.Unstaged++
		}
	}
	return s, nil
}

// CommitsCreated returns commits only if start is a verified ancestor of end.
func CommitsCreated(root string, start, end GitSnapshot) (int, error) {
	if start.Root != filepath.Clean(root) || end.Root != filepath.Clean(root) {
		return 0, fmt.Errorf("snapshot root mismatch")
	}
	if _, err := gitAt(root, "merge-base", "--is-ancestor", start.Head, end.Head); err != nil {
		return 0, fmt.Errorf("unverified ancestry: %w", err)
	}
	out, err := gitAt(root, "rev-list", "--count", start.Head+".."+end.Head)
	if err != nil {
		return 0, err
	}
	var n int
	_, err = fmt.Sscan(out, &n)
	return n, err
}

// CaptureGitContext returns git repository metadata from the current working directory.
func CaptureGitContext() (repoURL, branch, commitSHA string) {
	return runGitCmd("remote", "get-url", "origin"), runGitCmd("branch", "--show-current"), runGitCmd("rev-parse", "HEAD")
}
func runGitCmd(args ...string) string {
	cmd := exec.Command(gitExecutable, args...)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
func gitAt(root string, args ...string) (string, error) {
	cmd := exec.Command(gitExecutable, append([]string{"-C", root}, args...)...)
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}
