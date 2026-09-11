package line

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var githubIssueURL = regexp.MustCompile(`(?i)^https://github\.com/[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+/issues/[0-9]+/?$`)

type Executor interface {
	Run(ctx context.Context, cwd, prompt string) (string, error)
}

const defaultMaxRepairs = 3

type Engine struct {
	store      *Store
	executor   Executor
	promptsDir string
	notifier   Notifier
	checker    Checker
	maxRepairs int
	now        func() time.Time
	id         func() string
}

func NewEngine(store *Store, executor Executor, promptsDir string) *Engine {
	return &Engine{
		store:      store,
		executor:   executor,
		promptsDir: promptsDir,
		notifier:   NopNotifier{},
		checker:    NewGHChecker(),
		maxRepairs: defaultMaxRepairs,
		now:        func() time.Time { return time.Now().UTC() },
		id:         newID,
	}
}

func (e *Engine) WithNotifier(n Notifier) *Engine {
	if n == nil {
		e.notifier = NopNotifier{}
		return e
	}
	e.notifier = n
	return e
}

func (e *Engine) WithChecker(c Checker) *Engine {
	e.checker = c
	return e
}

func (e *Engine) RunPlan(ctx context.Context, issueURL, repoPath string) (Job, error) {
	if err := validateIssueURL(issueURL); err != nil {
		return Job{}, err
	}
	if strings.TrimSpace(repoPath) == "" {
		return Job{}, fmt.Errorf("repo path is required")
	}
	now := e.now()
	job := Job{
		ID:        e.id(),
		IssueURL:  strings.TrimRight(issueURL, "/"),
		RepoPath:  repoPath,
		Phase:     PhasePlan,
		Status:    StatusRunning,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := e.store.CreateJob(ctx, job); err != nil {
		return Job{}, err
	}
	if err := e.note(ctx, job, "started", "plan"); err != nil {
		return job, err
	}
	return e.executePhase(ctx, job)
}

func (e *Engine) Continue(ctx context.Context, jobID, answer string) (Job, error) {
	job, err := e.store.GetJob(ctx, jobID)
	if err != nil {
		return Job{}, err
	}
	if job.Status != StatusNeedsHuman {
		return job, fmt.Errorf("job %s is %s, not needs_human", job.ID, job.Status)
	}
	if !runnablePhase(job.Phase) {
		return job, fmt.Errorf("phase %s is not implemented yet", job.Phase)
	}
	if strings.TrimSpace(answer) == "" {
		return job, fmt.Errorf("continue requires an answer")
	}
	job.HumanAnswer = strings.TrimSpace(answer)
	job.Status = StatusRunning
	job.Question = ""
	job.UpdatedAt = e.now()
	if err := e.store.UpdateJob(ctx, job); err != nil {
		return job, err
	}
	if err := e.note(ctx, job, "continue", job.HumanAnswer); err != nil {
		return job, err
	}
	return e.executePhase(ctx, job)
}

func (e *Engine) Advance(ctx context.Context, jobID string) (Job, error) {
	job, err := e.store.GetJob(ctx, jobID)
	if err != nil {
		return Job{}, err
	}
	if job.Status != StatusAwaitingNext {
		return job, fmt.Errorf("job %s is %s, not awaiting_next", job.ID, job.Status)
	}
	if !runnablePhase(job.Phase) {
		return job, fmt.Errorf("phase %s is not implemented yet", job.Phase)
	}
	job.HumanAnswer = ""
	job.Status = StatusRunning
	job.UpdatedAt = e.now()
	if err := e.store.UpdateJob(ctx, job); err != nil {
		return job, err
	}
	if err := e.note(ctx, job, "advance", string(job.Phase)); err != nil {
		return job, err
	}
	return e.executePhase(ctx, job)
}

func (e *Engine) Status(ctx context.Context, jobID string) (Job, []Event, error) {
	if strings.TrimSpace(jobID) == "" {
		job, err := e.store.LatestJob(ctx)
		if err != nil {
			return Job{}, nil, err
		}
		jobID = job.ID
	}
	job, err := e.store.GetJob(ctx, jobID)
	if err != nil {
		return Job{}, nil, err
	}
	events, err := e.store.ListEvents(ctx, job.ID)
	if err != nil {
		return job, nil, err
	}
	return job, events, nil
}

func (e *Engine) List(ctx context.Context) ([]Job, error) {
	return e.store.ListJobs(ctx)
}

func (e *Engine) executePhase(ctx context.Context, job Job) (Job, error) {
	if job.Phase == PhaseWaitCI {
		return e.executeWaitCI(ctx, job)
	}
	prompt, err := RenderPhasePrompt(e.promptsDir, job.Phase, PromptVars{
		IssueURL:    job.IssueURL,
		RepoPath:    job.RepoPath,
		HumanAnswer: job.HumanAnswer,
		Summary:     job.Summary,
		CIDetail:    job.Summary,
		PullRequest: job.PullRequest,
	})
	if err != nil {
		return e.fail(ctx, job, err)
	}
	stdout, execErr := e.executor.Run(ctx, job.RepoPath, prompt)
	if execErr != nil {
		return e.fail(ctx, job, execErr)
	}
	outcome, err := ParseOutcome(stdout)
	if err != nil {
		return e.fail(ctx, job, err)
	}
	job.Summary = outcome.Summary
	job.UpdatedAt = e.now()
	switch outcome.Status {
	case OutcomeNeedsHuman:
		job.Status = StatusNeedsHuman
		job.Question = outcome.Question
	case OutcomeBlocked:
		job.Status = StatusBlocked
		job.Question = ""
	case OutcomeOK:
		job.Question = ""
		job.HumanAnswer = ""
		job.Phase, job.Status = nextOnOK(job.Phase)
	}
	if err := e.store.UpdateJob(ctx, job); err != nil {
		return job, err
	}
	if err := e.note(ctx, job, string(outcome.Status), outcome.Summary+outcome.Question); err != nil {
		return job, err
	}
	e.notifyPark(ctx, job)
	return job, nil
}

func (e *Engine) executeWaitCI(ctx context.Context, job Job) (Job, error) {
	if e.checker == nil {
		return e.block(ctx, job, "no CI checker configured")
	}
	result, err := e.checker.Check(ctx, job)
	if err != nil {
		return e.block(ctx, job, err.Error())
	}
	if result.HeadSHA != "" {
		job.HeadSHA = result.HeadSHA
	}
	if result.PullRequest != "" {
		job.PullRequest = result.PullRequest
	}
	job.Summary = result.Detail
	job.UpdatedAt = e.now()
	switch result.State {
	case CIPass:
		job.Phase = PhaseReady
		job.Status = StatusReady
	case CIPending:
		job.Phase = PhaseWaitCI
		job.Status = StatusAwaitingNext
	case CIFail:
		job.RepairCount++
		if job.RepairCount > e.maxRepairs {
			job.Status = StatusBlocked
			job.Summary = fmt.Sprintf("repair budget exhausted (%d): %s", e.maxRepairs, result.Detail)
			break
		}
		job.Phase = PhaseRepair
		job.Status = StatusAwaitingNext
	default:
		return e.block(ctx, job, "unknown CI state "+string(result.State))
	}
	if err := e.store.UpdateJob(ctx, job); err != nil {
		return job, err
	}
	if err := e.note(ctx, job, "ci_"+string(result.State), job.Summary); err != nil {
		return job, err
	}
	e.notifyPark(ctx, job)
	return job, nil
}

func (e *Engine) block(ctx context.Context, job Job, summary string) (Job, error) {
	job.Status = StatusBlocked
	job.Summary = summary
	job.UpdatedAt = e.now()
	if err := e.store.UpdateJob(ctx, job); err != nil {
		return job, err
	}
	if err := e.note(ctx, job, "blocked", summary); err != nil {
		return job, err
	}
	e.notifyPark(ctx, job)
	return job, nil
}

func nextOnOK(phase Phase) (Phase, Status) {
	switch phase {
	case PhasePlan:
		return PhaseBuild, StatusAwaitingNext
	case PhaseBuild:
		return PhaseReview, StatusAwaitingNext
	case PhaseReview:
		return PhaseWaitCI, StatusAwaitingNext
	case PhaseRepair:
		return PhaseReview, StatusAwaitingNext
	default:
		return phase, StatusAwaitingNext
	}
}

func runnablePhase(phase Phase) bool {
	switch phase {
	case PhasePlan, PhaseBuild, PhaseReview, PhaseRepair, PhaseWaitCI:
		return true
	default:
		return false
	}
}

func (e *Engine) notifyPark(ctx context.Context, job Job) {
	note, ok := notificationFor(job)
	if !ok {
		return
	}
	if err := e.notifier.Notify(ctx, note); err != nil {
		_ = e.note(ctx, job, "ntfy_error", err.Error())
		return
	}
	if _, isNop := e.notifier.(NopNotifier); isNop {
		return
	}
	_ = e.note(ctx, job, "ntfy", note.Title)
}

func (e *Engine) fail(ctx context.Context, job Job, cause error) (Job, error) {
	job.Status = StatusFailed
	job.Summary = cause.Error()
	job.UpdatedAt = e.now()
	_ = e.store.UpdateJob(ctx, job)
	_ = e.note(ctx, job, "failed", cause.Error())
	return job, cause
}

func (e *Engine) note(ctx context.Context, job Job, kind, detail string) error {
	return e.store.AppendEvent(ctx, Event{
		ID:        e.id(),
		JobID:     job.ID,
		Phase:     job.Phase,
		Kind:      kind,
		Detail:    detail,
		CreatedAt: e.now(),
	})
}

func validateIssueURL(issueURL string) error {
	if !githubIssueURL.MatchString(strings.TrimSpace(issueURL)) {
		return fmt.Errorf("issue url must be https://github.com/owner/repo/issues/N")
	}
	return nil
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
