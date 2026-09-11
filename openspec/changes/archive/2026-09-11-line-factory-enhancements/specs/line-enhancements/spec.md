# Capability Spec: Line Factory Enhancements & GitHub Backlog

## Purpose
Specify requirements and contracts for the Line Factory enhancements covering cross-platform build fixes, auto-advancement, CI polling, reviewer isolation, worktree encapsulation, PR/SHA persistence, web UI job creation, ntfy actions, and SkillVault prompt loading.

## Requirements

### Requirement: Windows Cross-Build Portability (REQ-LINE-01)
* The SkillVault CLI build MUST succeed on Windows (`GOOS=windows go build ./cmd/skillvault`).
* Unix process-group detachment (`Setsid: true`) in `internal/cli/handlers_telemetry.go` MUST be isolated to Unix builds (`!windows`), and Windows builds MUST provide a no-op process configuration.

### Requirement: Auto-Advance Phase Execution (REQ-LINE-02)
* `line run` MUST support auto-advance mode (flag `--auto` with default `true`), automatically progressing through `awaiting_next` states without requiring manual `line next` calls.
* Auto-advance MUST halt immediately when reaching `needs_human`, `blocked`, `failed`, or `ready`.
* Single-step execution via `line next` MUST remain supported.

### Requirement: Continuous CI Polling Loop (REQ-LINE-03)
* `wait_ci` phase MUST poll check runs until completion or timeout (default interval 10s, timeout up to 20m).
* If all checks pass, the job MUST transition to `ready`.
* If any check fails, the job MUST transition to `repair` (decrementing repair budget).
* If checks timeout, the job MUST transition to `blocked` with diagnostic evidence.

### Requirement: Independent Reviewer Configuration (REQ-LINE-04)
* `line` MUST support `--review-exec` and `--review-exec-arg` flags.
* When configured, the `review` phase MUST execute with the specified review command and arguments instead of inheriting the author/build executor.
* When unset, the reviewer MUST default to the primary executor.

### Requirement: Isolated Git Worktree Execution (REQ-LINE-05)
* Mutating agent phases (`build`, `repair`) MUST execute in an isolated git worktree rather than the operator's active checkout.
* Worktrees MUST be created under a managed directory and recorded on the job (`worktree` field/events).
* Worktrees MUST be cleaned up on job success, but preserved for operator inspection on failure or block.

### Requirement: PR URL and Head SHA Persistence (REQ-LINE-06)
* The classified JSON outcome from `build` and `repair` MUST parse optional `pull_request` and `head_sha` fields.
* The job store MUST persist `pull_request` and `head_sha` directly on the `Job` record.
* The `wait_ci` phase MUST use stored PR and commit identities when present, avoiding redundant CLI discovery.

### Requirement: Loopback UI Job Origination (REQ-LINE-07)
* The loopback UI (`line serve`) MUST provide an HTML form allowing operators to submit new jobs with `Issue URL`, `Repo Path`, optional `NTFY Topic`, and `Executor`.
* The handler MUST validate inputs and initialize the job via `RunPlan`.
* The server MUST strictly bind to loopback addresses (`127.0.0.1`).

### Requirement: Mobile ntfy Continuation Action (REQ-LINE-08)
* Notifications sent to ntfy for `needs_human` MUST include structured action buttons or deep links allowing operator continuation from mobile devices.
* A continuation endpoint or polling listener MUST allow posting an answer back to unblock `needs_human`.

### Requirement: SkillVault Prompt Retrieval Fallback (REQ-LINE-09)
* Phase prompts (`plan`, `build`, `review`, `repair`) MUST query SkillVault entries by slug (e.g. `prompt:line-<phase>` or `line-<phase>`) when a vault is available.
* If the SkillVault entry is missing or inaccessible, the loader MUST gracefully fall back to embedded prompt templates.

### Requirement: Install & Quickstart Documentation (REQ-LINE-10)
* `docs/line.md` and the quickstart MUST document `make install-line` to install the `line` binary to `~/tools`.
* Comments in `Makefile` MUST accurately reflect the implemented status of the factory.
