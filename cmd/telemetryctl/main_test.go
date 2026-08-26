package main

import "testing"

func TestCurrentReportCommitRejectsUnavailableGitState(t *testing.T) {
	previous := gitCurrentCommit
	gitCurrentCommit = func() (string, error) { return "", errNoCurrentCommit }
	t.Cleanup(func() { gitCurrentCommit = previous })
	if _, err := currentReportCommit(); err == nil {
		t.Fatal("report must not query every commit when the repository commit is unavailable")
	}
}
