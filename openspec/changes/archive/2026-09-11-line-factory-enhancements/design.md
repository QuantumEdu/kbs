# Design: Line Factory Enhancements & GitHub Backlog

## Architecture Overview
This change addresses the complete `line` factory backlog (GitHub issues #77-#87) across four key architectural domains:
1. **Cross-Platform Portability (`internal/cli`)**:
   - Separate Unix vs Windows process attribute assignment for background telemetry daemons via Go build tags.
2. **Autonomous Execution Engine (`internal/line`)**:
   - **Auto-Advance**: `RunAuto(ctx, jobID)` loops through phases until reaching terminal states (`ready`, `blocked`, `failed`) or an interactive pause (`needs_human`).
   - **Worktree Isolation**: Manages a detached Git worktree per job for mutating phases (`build`, `repair`), safeguarding the user's working tree.
   - **Independent Reviewer**: Supports dedicated review commands and flags (`ReviewExec`, `ReviewExecArgs`) distinct from the author's executor.
3. **Tracking & CI Automation**:
   - **PR/SHA Persistence**: Parses pull request URLs and commit SHAs directly from agent JSON outcomes and updates the SQLite job record.
   - **Continuous CI Poller**: Actively polls `gh pr checks` or equivalent CI interface in a non-blocking loop with a configurable timeout.
4. **Usability & SkillVault Ecosystem**:
   - **Loopback UI**: Web interface on `127.0.0.1:7340` supporting new job origination.
   - **ntfy Reply Path**: Mobile-friendly Action buttons on notifications.
   - **Prompt Fallback**: Dynamic prompt retrieval from SkillVault entry slugs with embedded fallback.
   - **Documentation**: Updated `docs/line.md` and `Makefile`.

## Detailed Component Design

### 1. Build Tag Isolation for SysProcAttr
- In `internal/cli/handlers_telemetry.go`, remove inline `&syscall.SysProcAttr{Setsid: true}`.
- Introduce `configureDaemonProcess(cmd *exec.Cmd)`:
  - `handlers_telemetry_unix.go` (`//go:build !windows`): Sets `cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}`.
  - `handlers_telemetry_windows.go` (`//go:build windows`): No-op function.

### 2. Auto-Advance & Engine State Machine
- `Engine.Advance(ctx, jobID)` advances a single step.
- `Engine.AutoAdvance(ctx, jobID)` loops `Advance` while `job.Status == StatusAwaitingNext`, breaking when `job.Status` is `StatusNeedsHuman`, `StatusBlocked`, `StatusFailed`, or `StatusReady`.
- Add `--auto` flag to `line run` (default `true`).

### 3. Isolated Worktree Management
- New module `internal/line/worktree.go`:
  - `CreateWorktree(repoPath string, jobID string) (string, error)` executes `git worktree add --detach <path> HEAD`.
  - `RemoveWorktree(repoPath string, worktreePath string) error` executes `git worktree remove --force <path>`.
  - Mutating phases set `workingDir = job.WorktreePath`.
  - Worktree cleanup triggers upon `ready` transition; preserved upon failure or block.

### 4. Independent Reviewer Configuration
- In `internal/line/job.go`:
  - Add fields `ReviewExec string` and `ReviewExecArgs []string` to `Job` and `Config`.
- In `internal/line/engine.go`:
  - During the `review` phase, check if `job.ReviewExec` is populated. If so, instantiate `NewProcessExecutor(job.ReviewExec, job.ReviewExecArgs)`. Otherwise, use default `job.Exec`.

### 5. PR URL and Head SHA Persistence
- In `internal/line/outcome.go`:
  - Add `PullRequest string json:"pull_request,omitempty"` and `HeadSHA string json:"head_sha,omitempty"` to `Outcome`.
- In `internal/line/store.go`:
  - Add columns `pull_request TEXT DEFAULT ''` and `head_sha TEXT DEFAULT ''` to `jobs` table.
  - Persist values when storing `build` and `repair` outcomes.
- In `internal/line/ci.go`:
  - Check `job.PullRequest` and `job.HeadSHA` first; only invoke `gh pr view` if empty.

### 6. Continuous CI Polling
- In `internal/line/ci.go`:
  - Implement `Poll(ctx context.Context, checker Checker, repo, pr, sha string, interval, timeout time.Duration) (CIStatus, string, error)`.
  - Returns `CIPass`, `CIFail`, or `CITimeout`.
  - Handles timeout by setting status to `StatusBlocked` with details.

### 7. Loopback UI Job Origination
- In `internal/line/ui.go`:
  - Extend HTML template with a collapsible or top "New Job" form (`POST /job/new`).
  - Inputs: `repo` (required), `issue` (required), `ntfy` (optional), `exec` (optional), `review_exec` (optional).
  - Validates inputs and dispatches `RunPlan`.

### 8. ntfy Actions & Continuation
- In `internal/line/ntfy.go`:
  - Add `Action: view, Open Issue, <issue_url>` and `Action: http, Continue, <server_url>/job/<id>/continue`.
  - When `needs_human` is emitted, provide interactive buttons on mobile.

### 9. SkillVault Prompt Retrieval
- In `internal/line/prompt.go`:
  - Provide `LoadPrompt(ctx context.Context, phase Phase, customDir string) (string, error)`.
  - Attempts `skillvault get prompt:line-<phase>` or searches entries in local vault if available.
  - Falls back to embedded template `prompts/<phase>.md`.

### 10. Documentation & Makefile
- Update `docs/line.md` quickstart: add `make install-line`.
- Update `Makefile` target comments.
