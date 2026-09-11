package line

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeExecutor struct {
	stdout string
	err    error
	prompt string
	cwd    string
}

func (f *fakeExecutor) Run(_ context.Context, cwd, prompt string) (string, error) {
	f.cwd = cwd
	f.prompt = prompt
	return f.stdout, f.err
}

func TestEngineRunPlanNeedsHuman(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := openTestStore(t)
	exec := &fakeExecutor{stdout: `{"status":"needs_human","question":"Public API?"}`}
	engine := NewEngine(store, exec, "")

	job, err := engine.RunPlan(ctx, "https://github.com/o/r/issues/4", "/repo")
	if err != nil {
		t.Fatalf("RunPlan: %v", err)
	}
	if job.Status != StatusNeedsHuman || job.Phase != PhasePlan || job.Question != "Public API?" {
		t.Fatalf("job %+v", job)
	}
	if !strings.Contains(exec.prompt, "issues/4") {
		t.Fatalf("prompt not injected: %q", exec.prompt)
	}
	if exec.cwd != "/repo" {
		t.Fatalf("cwd %q", exec.cwd)
	}
}

func TestEngineRunPlanAdvancesToBuildWithoutRunningIt(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := openTestStore(t)
	exec := &fakeExecutor{stdout: `{"status":"ok","summary":"spec ready"}`}
	engine := NewEngine(store, exec, "")

	job, err := engine.RunPlan(ctx, "https://github.com/o/r/issues/5", "/repo")
	if err != nil {
		t.Fatalf("RunPlan: %v", err)
	}
	if job.Phase != PhaseBuild || job.Status != StatusAwaitingNext {
		t.Fatalf("expected parked build, got phase=%s status=%s", job.Phase, job.Status)
	}
	if job.Summary != "spec ready" {
		t.Fatalf("summary %q", job.Summary)
	}
}

func TestEngineRunPlanBlocked(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := openTestStore(t)
	exec := &fakeExecutor{stdout: `{"status":"blocked","summary":"no gh"}`}
	engine := NewEngine(store, exec, "")

	job, err := engine.RunPlan(ctx, "https://github.com/o/r/issues/6", "/repo")
	if err != nil {
		t.Fatalf("RunPlan: %v", err)
	}
	if job.Status != StatusBlocked || job.Phase != PhasePlan {
		t.Fatalf("job %+v", job)
	}
}

func TestEngineRunPlanExecutorError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := openTestStore(t)
	exec := &fakeExecutor{err: errors.New("spawn failed")}
	engine := NewEngine(store, exec, "")

	job, err := engine.RunPlan(ctx, "https://github.com/o/r/issues/7", "/repo")
	if err == nil {
		t.Fatal("expected executor error")
	}
	if job.Status != StatusFailed {
		t.Fatalf("status %s", job.Status)
	}
}

func TestEngineContinuePlanWithAnswer(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := openTestStore(t)
	exec := &fakeExecutor{stdout: `{"status":"needs_human","question":"Public API?"}`}
	engine := NewEngine(store, exec, "")
	job, err := engine.RunPlan(ctx, "https://github.com/o/r/issues/8", "/repo")
	if err != nil {
		t.Fatalf("RunPlan: %v", err)
	}

	exec.stdout = `{"status":"ok","summary":"API is public"}`
	continued, err := engine.Continue(ctx, job.ID, "yes, public")
	if err != nil {
		t.Fatalf("Continue: %v", err)
	}
	if continued.Status != StatusAwaitingNext || continued.Phase != PhaseBuild {
		t.Fatalf("continued %+v", continued)
	}
	if !strings.Contains(exec.prompt, "yes, public") {
		t.Fatalf("human answer not in prompt: %q", exec.prompt)
	}
}

func TestEngineContinueRequiresNeedsHuman(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := openTestStore(t)
	exec := &fakeExecutor{stdout: `{"status":"ok","summary":"done"}`}
	engine := NewEngine(store, exec, "")
	job, err := engine.RunPlan(ctx, "https://github.com/o/r/issues/10", "/repo")
	if err != nil {
		t.Fatalf("RunPlan: %v", err)
	}
	if _, err := engine.Continue(ctx, job.ID, "later"); err == nil {
		t.Fatal("expected continue to refuse awaiting_next")
	}
}

func TestEngineRejectsNonIssueURL(t *testing.T) {
	t.Parallel()
	engine := NewEngine(openTestStore(t), &fakeExecutor{}, "")
	if _, err := engine.RunPlan(context.Background(), "https://github.com/o/r", "/repo"); err == nil {
		t.Fatal("expected invalid issue url")
	}
}

type scriptedExecutor struct {
	replies []string
	i       int
	prompts []string
}

func (s *scriptedExecutor) Run(_ context.Context, _, prompt string) (string, error) {
	s.prompts = append(s.prompts, prompt)
	if s.i >= len(s.replies) {
		return "", errors.New("no scripted reply")
	}
	out := s.replies[s.i]
	s.i++
	return out, nil
}

type recordingNotifier struct {
	notes []Notification
	err   error
}

func (r *recordingNotifier) Notify(_ context.Context, n Notification) error {
	r.notes = append(r.notes, n)
	return r.err
}

type fakeChecker struct {
	result CIResult
	err    error
	calls  int
}

func (f *fakeChecker) Check(context.Context, Job) (CIResult, error) {
	f.calls++
	return f.result, f.err
}

func TestEngineAdvanceBuildAndReviewToReady(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	exec := &scriptedExecutor{replies: []string{
		`{"status":"ok","summary":"spec ready"}`,
		`{"status":"ok","summary":"implemented"}`,
		`{"status":"ok","summary":"approved"}`,
	}}
	engine := NewEngine(openTestStore(t), exec, "").WithChecker(&fakeChecker{result: CIResult{
		State: CIPass, Detail: "all checks passed", HeadSHA: "deadbeef", PullRequest: "https://github.com/o/r/pull/1",
	}})
	job, err := engine.RunPlan(ctx, "https://github.com/o/r/issues/20", "/repo")
	if err != nil {
		t.Fatalf("RunPlan: %v", err)
	}
	job, err = engine.Advance(ctx, job.ID)
	if err != nil {
		t.Fatalf("advance build: %v", err)
	}
	if job.Phase != PhaseReview || job.Status != StatusAwaitingNext {
		t.Fatalf("after build %+v", job)
	}
	if !strings.Contains(exec.prompts[1], "build step of line") {
		t.Fatalf("expected build prompt, got %q", exec.prompts[1])
	}
	job, err = engine.Advance(ctx, job.ID)
	if err != nil {
		t.Fatalf("advance review: %v", err)
	}
	if job.Phase != PhaseWaitCI || job.Status != StatusAwaitingNext {
		t.Fatalf("after review %+v", job)
	}
	job, err = engine.Advance(ctx, job.ID)
	if err != nil {
		t.Fatalf("advance wait_ci: %v", err)
	}
	if job.Phase != PhaseReady || job.Status != StatusReady || job.HeadSHA != "deadbeef" {
		t.Fatalf("after wait_ci %+v", job)
	}
}

func TestEngineContinueBuildNeedsHuman(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	exec := &scriptedExecutor{replies: []string{
		`{"status":"ok","summary":"spec ready"}`,
		`{"status":"needs_human","question":"Which table?"}`,
		`{"status":"ok","summary":"users table"}`,
	}}
	engine := NewEngine(openTestStore(t), exec, "")
	job, err := engine.RunPlan(ctx, "https://github.com/o/r/issues/21", "/repo")
	if err != nil {
		t.Fatalf("RunPlan: %v", err)
	}
	job, err = engine.Advance(ctx, job.ID)
	if err != nil {
		t.Fatalf("advance: %v", err)
	}
	if job.Status != StatusNeedsHuman || job.Phase != PhaseBuild {
		t.Fatalf("job %+v", job)
	}
	job, err = engine.Continue(ctx, job.ID, "users")
	if err != nil {
		t.Fatalf("continue: %v", err)
	}
	if job.Phase != PhaseReview || job.Status != StatusAwaitingNext {
		t.Fatalf("continued %+v", job)
	}
	if !strings.Contains(exec.prompts[2], "users") {
		t.Fatalf("answer missing in prompt %q", exec.prompts[2])
	}
}

func TestEngineNotifiesNeedsHuman(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	notes := &recordingNotifier{}
	exec := &fakeExecutor{stdout: `{"status":"needs_human","question":"Public API?"}`}
	engine := NewEngine(openTestStore(t), exec, "").WithNotifier(notes)
	job, err := engine.RunPlan(ctx, "https://github.com/o/r/issues/22", "/repo")
	if err != nil {
		t.Fatalf("RunPlan: %v", err)
	}
	if len(notes.notes) != 1 || notes.notes[0].Message != "Public API?" {
		t.Fatalf("notes %+v", notes.notes)
	}
	if notes.notes[0].Click != job.IssueURL {
		t.Fatalf("click %s", notes.notes[0].Click)
	}
	_, events, err := engine.Status(ctx, job.ID)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	found := false
	for _, event := range events {
		if event.Kind == "ntfy" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected ntfy event, got %+v", events)
	}
}

func TestEngineNtfyErrorDoesNotFailJob(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	notes := &recordingNotifier{err: errors.New("ntfy down")}
	exec := &fakeExecutor{stdout: `{"status":"needs_human","question":"X?"}`}
	engine := NewEngine(openTestStore(t), exec, "").WithNotifier(notes)
	job, err := engine.RunPlan(ctx, "https://github.com/o/r/issues/23", "/repo")
	if err != nil {
		t.Fatalf("RunPlan: %v", err)
	}
	if job.Status != StatusNeedsHuman {
		t.Fatalf("status %s", job.Status)
	}
}

func TestEngineWaitCIFailureStartsRepairThenReview(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	exec := &scriptedExecutor{replies: []string{
		`{"status":"ok","summary":"spec"}`,
		`{"status":"ok","summary":"built"}`,
		`{"status":"ok","summary":"reviewed"}`,
		`{"status":"ok","summary":"fixed lint"}`,
	}}
	checker := &fakeChecker{result: CIResult{State: CIFail, Detail: "failed: lint"}}
	engine := NewEngine(openTestStore(t), exec, "").WithChecker(checker)
	job, err := engine.RunPlan(ctx, "https://github.com/o/r/issues/40", "/repo")
	if err != nil {
		t.Fatalf("RunPlan: %v", err)
	}
	if job, err = engine.Advance(ctx, job.ID); err != nil {
		t.Fatalf("build: %v", err)
	}
	if job, err = engine.Advance(ctx, job.ID); err != nil {
		t.Fatalf("review: %v", err)
	}
	job, err = engine.Advance(ctx, job.ID)
	if err != nil {
		t.Fatalf("wait_ci: %v", err)
	}
	if job.Phase != PhaseRepair || job.Status != StatusAwaitingNext || job.RepairCount != 1 {
		t.Fatalf("after ci fail %+v", job)
	}
	job, err = engine.Advance(ctx, job.ID)
	if err != nil {
		t.Fatalf("repair: %v", err)
	}
	if job.Phase != PhaseReview || job.Status != StatusAwaitingNext {
		t.Fatalf("after repair %+v", job)
	}
	if !strings.Contains(exec.prompts[3], "repair step of line") || !strings.Contains(exec.prompts[3], "failed: lint") {
		t.Fatalf("repair prompt %q", exec.prompts[3])
	}
}

func TestEngineWaitCIPendingStaysWaiting(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	exec := &scriptedExecutor{replies: []string{
		`{"status":"ok","summary":"spec"}`,
		`{"status":"ok","summary":"built"}`,
		`{"status":"ok","summary":"reviewed"}`,
	}}
	engine := NewEngine(openTestStore(t), exec, "").WithChecker(&fakeChecker{result: CIResult{State: CIPending, Detail: "1 checks pending"}})
	job, err := engine.RunPlan(ctx, "https://github.com/o/r/issues/41", "/repo")
	if err != nil {
		t.Fatalf("RunPlan: %v", err)
	}
	if job, err = engine.Advance(ctx, job.ID); err != nil {
		t.Fatalf("build: %v", err)
	}
	if job, err = engine.Advance(ctx, job.ID); err != nil {
		t.Fatalf("review: %v", err)
	}
	job, err = engine.Advance(ctx, job.ID)
	if err != nil {
		t.Fatalf("wait_ci: %v", err)
	}
	if job.Phase != PhaseWaitCI || job.Status != StatusAwaitingNext {
		t.Fatalf("%+v", job)
	}
}

func TestEngineRepairBudgetBlocks(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := openTestStore(t)
	engine := NewEngine(store, &fakeExecutor{}, "").WithChecker(&fakeChecker{result: CIResult{State: CIFail, Detail: "failed: test"}})
	engine.maxRepairs = 1
	now := engine.now()
	job := Job{
		ID: "budget", IssueURL: "https://github.com/o/r/issues/42", RepoPath: "/repo",
		Phase: PhaseWaitCI, Status: StatusAwaitingNext, CreatedAt: now, UpdatedAt: now,
	}
	if err := store.CreateJob(ctx, job); err != nil {
		t.Fatalf("create: %v", err)
	}
	first, err := engine.Advance(ctx, job.ID)
	if err != nil {
		t.Fatalf("first fail: %v", err)
	}
	if first.Phase != PhaseRepair || first.RepairCount != 1 {
		t.Fatalf("first %+v", first)
	}
	first.Phase = PhaseWaitCI
	first.Status = StatusAwaitingNext
	first.UpdatedAt = engine.now()
	if err := store.UpdateJob(ctx, first); err != nil {
		t.Fatalf("update: %v", err)
	}
	second, err := engine.Advance(ctx, first.ID)
	if err != nil {
		t.Fatalf("second fail: %v", err)
	}
	if second.Status != StatusBlocked || second.RepairCount != 2 {
		t.Fatalf("budget %+v", second)
	}
}

func TestEngineAdvanceRefusesNeedsHuman(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	engine := NewEngine(openTestStore(t), &fakeExecutor{stdout: `{"status":"needs_human","question":"X?"}`}, "")
	job, err := engine.RunPlan(ctx, "https://github.com/o/r/issues/24", "/repo")
	if err != nil {
		t.Fatalf("RunPlan: %v", err)
	}
	if _, err := engine.Advance(ctx, job.ID); err == nil {
		t.Fatal("expected refuse")
	}
}

func TestEngineAutoAdvance(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := openTestStore(t)
	exec := &scriptedExecutor{replies: []string{
		`{"status":"ok","summary":"plan done"}`,
		`{"status":"ok","summary":"built","pull_request":"https://github.com/o/r/pull/88","head_sha":"fedcba9"}`,
		`{"status":"ok","summary":"reviewed and approved"}`,
	}}
	checker := &fakeChecker{result: CIResult{State: CIPass, Detail: "checks passed"}}
	engine := NewEngine(store, exec, "").WithChecker(checker)

	job, err := engine.RunPlan(ctx, "https://github.com/o/r/issues/88", "/repo")
	if err != nil {
		t.Fatalf("RunPlan: %v", err)
	}
	finalJob, err := engine.AutoAdvance(ctx, job.ID)
	if err != nil {
		t.Fatalf("AutoAdvance: %v", err)
	}
	if finalJob.Phase != PhaseReady || finalJob.Status != StatusReady {
		t.Fatalf("expected ready, got phase=%s status=%s", finalJob.Phase, finalJob.Status)
	}
	if finalJob.PullRequest != "https://github.com/o/r/pull/88" || finalJob.HeadSHA != "fedcba9" {
		t.Fatalf("expected PR and SHA persisted, got %+v", finalJob)
	}
}

func TestEngineAutoAdvanceHaltsOnNeedsHuman(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := openTestStore(t)
	exec := &scriptedExecutor{replies: []string{
		`{"status":"ok","summary":"plan done"}`,
		`{"status":"needs_human","question":"Which database?"}`,
	}}
	engine := NewEngine(store, exec, "")

	job, err := engine.RunPlan(ctx, "https://github.com/o/r/issues/89", "/repo")
	if err != nil {
		t.Fatalf("RunPlan: %v", err)
	}
	finalJob, err := engine.AutoAdvance(ctx, job.ID)
	if err != nil {
		t.Fatalf("AutoAdvance: %v", err)
	}
	if finalJob.Phase != PhaseBuild || finalJob.Status != StatusNeedsHuman || finalJob.Question != "Which database?" {
		t.Fatalf("expected halt at needs_human, got %+v", finalJob)
	}
}

func TestEngineReviewExecutor(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := openTestStore(t)
	mainExec := &scriptedExecutor{replies: []string{
		`{"status":"ok","summary":"plan done"}`,
		`{"status":"ok","summary":"build done"}`,
	}}
	reviewExec := &fakeExecutor{stdout: `{"status":"ok","summary":"review approved"}`}
	checker := &fakeChecker{result: CIResult{State: CIPass, Detail: "checks pass"}}

	engine := NewEngine(store, mainExec, "").
		WithReviewExecutor(reviewExec).
		WithChecker(checker)

	job, err := engine.RunPlan(ctx, "https://github.com/o/r/issues/90", "/repo")
	if err != nil {
		t.Fatalf("RunPlan: %v", err)
	}
	job, err = engine.Advance(ctx, job.ID) // build
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	job, err = engine.Advance(ctx, job.ID) // review
	if err != nil {
		t.Fatalf("review: %v", err)
	}
	if reviewExec.cwd != "/repo" || !strings.Contains(reviewExec.prompt, "issues/90") {
		t.Fatalf("review executor was not used: %+v", reviewExec)
	}
}

type mockWorktreeManager struct {
	created []string
	cleaned []string
}

func (m *mockWorktreeManager) EnsureWorktree(_ context.Context, repoPath, jobID string) (string, error) {
	wt := repoPath + "-worktree-" + jobID
	m.created = append(m.created, wt)
	return wt, nil
}

func (m *mockWorktreeManager) CleanupWorktree(_ context.Context, repoPath, worktreePath string) error {
	m.cleaned = append(m.cleaned, worktreePath)
	return nil
}

func TestEngineWorktreeManager(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := openTestStore(t)
	wtMgr := &mockWorktreeManager{}
	exec := &scriptedExecutor{replies: []string{
		`{"status":"ok","summary":"plan done"}`,
		`{"status":"ok","summary":"build done"}`,
		`{"status":"ok","summary":"review done"}`,
	}}
	checker := &fakeChecker{result: CIResult{State: CIPass, Detail: "checks pass"}}
	engine := NewEngine(store, exec, "").
		WithWorktreeManager(wtMgr).
		WithChecker(checker)

	job, err := engine.RunPlan(ctx, "https://github.com/o/r/issues/91", "/repo")
	if err != nil {
		t.Fatalf("RunPlan: %v", err)
	}
	if len(wtMgr.created) != 0 {
		t.Fatalf("worktree created during plan: %v", wtMgr.created)
	}
	job, err = engine.Advance(ctx, job.ID)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if len(wtMgr.created) != 1 {
		t.Fatalf("expected 1 worktree, got: %v", wtMgr.created)
	}
	if job.WorktreePath != wtMgr.created[0] {
		t.Fatalf("worktree not recorded on job: %s vs %s", job.WorktreePath, wtMgr.created[0])
	}
	job, err = engine.Advance(ctx, job.ID)
	if err != nil {
		t.Fatalf("review: %v", err)
	}
	job, err = engine.Advance(ctx, job.ID)
	if err != nil {
		t.Fatalf("wait_ci: %v", err)
	}
	if job.Status != StatusReady {
		t.Fatalf("expected ready, got %s", job.Status)
	}
	if len(wtMgr.cleaned) != 1 || wtMgr.cleaned[0] != wtMgr.created[0] {
		t.Fatalf("expected worktree cleanup, got: %v", wtMgr.cleaned)
	}
}
