package line

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

type CIState string

const (
	CIPass    CIState = "pass"
	CIFail    CIState = "fail"
	CIPending CIState = "pending"
)

type CIResult struct {
	State       CIState
	Detail      string
	HeadSHA     string
	PullRequest string
}

type Checker interface {
	Check(ctx context.Context, job Job) (CIResult, error)
}

type commandRunner func(ctx context.Context, dir, name string, args ...string) ([]byte, error)

type GHChecker struct {
	run commandRunner
}

func NewGHChecker() GHChecker {
	return GHChecker{run: runCommand}
}

func runCommand(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return out, fmt.Errorf("%s: %s", name, detail)
	}
	return out, nil
}

func (c GHChecker) Check(ctx context.Context, job Job) (CIResult, error) {
	run := c.run
	if run == nil {
		run = runCommand
	}
	prViewArgs := []string{"pr", "view"}
	if job.PullRequest != "" {
		prViewArgs = append(prViewArgs, job.PullRequest)
	}
	prViewArgs = append(prViewArgs, "--json", "url,headRefOid,state")

	prJSON, err := run(ctx, job.RepoPath, "gh", prViewArgs...)
	if err != nil {
		return CIResult{}, fmt.Errorf("no open pull request: %w", err)
	}
	var pr struct {
		URL        string `json:"url"`
		HeadRefOID string `json:"headRefOid"`
		State      string `json:"state"`
	}
	if err := json.Unmarshal(prJSON, &pr); err != nil {
		return CIResult{}, fmt.Errorf("parse pr view: %w", err)
	}
	if !strings.EqualFold(pr.State, "OPEN") {
		return CIResult{}, fmt.Errorf("pull request is %s", pr.State)
	}

	prChecksArgs := []string{"pr", "checks"}
	if job.PullRequest != "" {
		prChecksArgs = append(prChecksArgs, job.PullRequest)
	}
	prChecksArgs = append(prChecksArgs, "--json", "name,state,bucket")
	checksJSON, err := run(ctx, job.RepoPath, "gh", prChecksArgs...)
	if err != nil {
		return CIResult{}, fmt.Errorf("pr checks: %w", err)
	}
	state, detail, err := classifyChecks(checksJSON)
	if err != nil {
		return CIResult{}, err
	}
	return CIResult{
		State:       state,
		Detail:      detail,
		HeadSHA:     pr.HeadRefOID,
		PullRequest: pr.URL,
	}, nil
}

type ghCheck struct {
	Name   string `json:"name"`
	State  string `json:"state"`
	Bucket string `json:"bucket"`
}

func classifyChecks(raw []byte) (CIState, string, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "[]" {
		return CIPending, "no checks reported yet", nil
	}
	var checks []ghCheck
	if err := json.Unmarshal(raw, &checks); err != nil {
		return "", "", fmt.Errorf("parse pr checks: %w", err)
	}
	pending, failed := 0, 0
	var failNames []string
	for _, check := range checks {
		switch classifyCheck(check) {
		case CIFail:
			failed++
			failNames = append(failNames, check.Name)
		case CIPending:
			pending++
		}
	}
	switch {
	case failed > 0:
		return CIFail, "failed: " + strings.Join(failNames, ", "), nil
	case pending > 0:
		return CIPending, fmt.Sprintf("%d checks pending", pending), nil
	default:
		return CIPass, "all checks passed", nil
	}
}

func classifyCheck(check ghCheck) CIState {
	bucket := strings.ToLower(strings.TrimSpace(check.Bucket))
	state := strings.ToLower(strings.TrimSpace(check.State))
	switch bucket {
	case "fail", "failed":
		return CIFail
	case "pending":
		return CIPending
	case "pass", "skipping", "skip":
		return CIPass
	}
	switch state {
	case "failure", "failed", "cancelled", "canceled", "timed_out", "action_required", "startup_failure", "stale":
		return CIFail
	case "pending", "queued", "in_progress", "waiting", "requested", "expected":
		return CIPending
	default:
		return CIPass
	}
}
