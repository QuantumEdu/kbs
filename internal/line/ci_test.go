package line

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestClassifyChecks(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		raw  string
		want CIState
	}{
		{name: "empty is pending", raw: "[]", want: CIPending},
		{name: "all pass", raw: `[{"name":"test","state":"SUCCESS","bucket":"pass"}]`, want: CIPass},
		{name: "one fail", raw: `[{"name":"test","state":"SUCCESS","bucket":"pass"},{"name":"lint","state":"FAILURE","bucket":"fail"}]`, want: CIFail},
		{name: "pending bucket", raw: `[{"name":"test","state":"IN_PROGRESS","bucket":"pending"}]`, want: CIPending},
		{name: "state fallback fail", raw: `[{"name":"ci","state":"FAILURE"}]`, want: CIFail},
		{name: "skip counts as pass", raw: `[{"name":"optional","state":"SKIPPED","bucket":"skipping"}]`, want: CIPass},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, _, err := classifyChecks([]byte(tc.raw))
			if err != nil {
				t.Fatalf("classifyChecks: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %s want %s", got, tc.want)
			}
		})
	}
}

func TestGHCheckerMapsPRAndChecks(t *testing.T) {
	t.Parallel()
	checker := GHChecker{run: func(_ context.Context, _, name string, args ...string) ([]byte, error) {
		if name != "gh" {
			return nil, errors.New("expected gh")
		}
		joined := strings.Join(args, " ")
		switch {
		case strings.Contains(joined, "pr view"):
			return []byte(`{"url":"https://github.com/o/r/pull/9","headRefOid":"abc1234","state":"OPEN"}`), nil
		case strings.Contains(joined, "pr checks"):
			return []byte(`[{"name":"test","bucket":"pass","state":"SUCCESS"}]`), nil
		default:
			return nil, errors.New("unexpected args " + joined)
		}
	}}
	got, err := checker.Check(context.Background(), Job{RepoPath: "/repo"})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if got.State != CIPass || got.PullRequest != "https://github.com/o/r/pull/9" || got.HeadSHA != "abc1234" {
		t.Fatalf("%+v", got)
	}
}

func TestGHCheckerMissingPR(t *testing.T) {
	t.Parallel()
	checker := GHChecker{run: func(context.Context, string, string, ...string) ([]byte, error) {
		return nil, errors.New("no pr")
	}}
	if _, err := checker.Check(context.Background(), Job{RepoPath: "/repo"}); err == nil {
		t.Fatal("expected error")
	}
}

type sequenceChecker struct {
	results []CIResult
	index   int
}

func (s *sequenceChecker) Check(_ context.Context, _ Job) (CIResult, error) {
	if s.index >= len(s.results) {
		return s.results[len(s.results)-1], nil
	}
	r := s.results[s.index]
	s.index++
	return r, nil
}

func TestEngineWaitCIPolling(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := openTestStore(t)
	checker := &sequenceChecker{
		results: []CIResult{
			{State: CIPending, Detail: "running 1/2"},
			{State: CIPending, Detail: "running 2/2"},
			{State: CIPass, Detail: "all passed", HeadSHA: "sha-pass", PullRequest: "https://github.com/o/r/pull/1"},
		},
	}
	engine := NewEngine(store, &fakeExecutor{}, "").
		WithChecker(checker).
		WithPollConfig(5*time.Millisecond, 200*time.Millisecond)

	job := Job{
		ID:        "poll-job",
		IssueURL:  "https://github.com/o/r/issues/100",
		RepoPath:  "/repo",
		Phase:     PhaseWaitCI,
		Status:    StatusAwaitingNext,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := store.CreateJob(ctx, job); err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	res, err := engine.Advance(ctx, job.ID)
	if err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if res.Phase != PhaseReady || res.Status != StatusReady {
		t.Fatalf("expected ready, got phase=%s status=%s", res.Phase, res.Status)
	}
	if res.HeadSHA != "sha-pass" || res.PullRequest != "https://github.com/o/r/pull/1" {
		t.Fatalf("unexpected res: %+v", res)
	}
}

func TestEngineWaitCIPollingTimeout(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := openTestStore(t)
	checker := &sequenceChecker{
		results: []CIResult{
			{State: CIPending, Detail: "pending forever"},
		},
	}
	engine := NewEngine(store, &fakeExecutor{}, "").
		WithChecker(checker).
		WithPollConfig(5*time.Millisecond, 20*time.Millisecond)

	job := Job{
		ID:        "timeout-job",
		IssueURL:  "https://github.com/o/r/issues/101",
		RepoPath:  "/repo",
		Phase:     PhaseWaitCI,
		Status:    StatusAwaitingNext,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := store.CreateJob(ctx, job); err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	res, err := engine.Advance(ctx, job.ID)
	if err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if res.Status != StatusBlocked || !strings.Contains(res.Summary, "timed out") {
		t.Fatalf("expected blocked with timeout, got status=%s summary=%s", res.Status, res.Summary)
	}
}
