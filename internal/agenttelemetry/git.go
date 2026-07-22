package agenttelemetry

import (
	"os/exec"
	"strings"
)

// CaptureGitContext returns git repository metadata from the current
// working directory. Returns empty strings if git is not available or
// the directory is not a git repository.
func CaptureGitContext() (repoURL, branch, commitSHA string) {
	repoURL = runGitCmd("remote", "get-url", "origin")
	branch = runGitCmd("branch", "--show-current")
	commitSHA = runGitCmd("rev-parse", "HEAD")

	return repoURL, branch, commitSHA
}

// runGitCmd executes a git command and returns its trimmed stdout.
// Returns empty string on any error (git not installed, not a repo, etc.).
func runGitCmd(args ...string) string {
	cmd := exec.Command("git", args...)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
