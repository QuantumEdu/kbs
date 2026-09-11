# Tasks: Line Factory Enhancements & GitHub Backlog

## Phase 1: Portability, Build Fixes & Docs (Issues #86, #82)
- [x] 1.1 Fix Windows cross-compilation in `internal/cli/` by separating `SysProcAttr` into `handlers_telemetry_unix.go` and `handlers_telemetry_windows.go`.
- [x] 1.2 Verify `GOOS=windows go build ./cmd/skillvault` succeeds.
- [x] 1.3 Document `make install-line` in `docs/line.md` quickstart and align comments in `Makefile`.

## Phase 2: Auto-Advance, Reviewer Independence & Worktrees (Issues #77, #79, #80)
- [x] 2.1 Implement `AutoAdvance` loop in `internal/line/engine.go` that runs through phases until a terminal state or `needs_human`.
- [x] 2.2 Wire `--auto` flag into `line run` in `cmd/line/main.go`.
- [x] 2.3 Implement independent reviewer configuration (`--review-exec`, `--review-exec-arg`) in `internal/line/job.go` and `engine.go`.
- [x] 2.4 Implement git worktree isolation helper in `internal/line/worktree.go` for mutating phases (`build`, `repair`).
- [x] 2.5 Add unit tests for auto-advance, independent reviewer, and worktree isolation.

## Phase 3: PR/SHA Persistence & CI Polling Loop (Issues #81, #78)
- [x] 3.1 Update `Outcome` in `internal/line/outcome.go` to parse optional `pull_request` and `head_sha`.
- [x] 3.2 Update `internal/line/store.go` and schema to persist `pull_request` and `head_sha` on the `Job` record.
- [x] 3.3 Update `wait_ci` phase in `internal/line/ci.go` to prefer persisted `job.PullRequest` and `job.HeadSHA`.
- [x] 3.4 Implement continuous CI polling loop with timeout in `internal/line/ci.go`.
- [x] 3.5 Add unit tests for PR/SHA persistence and CI polling loop using mock checker.

## Phase 4: UI Origination, ntfy Actions & SkillVault Prompts (Issues #83, #84, #85)
- [x] 4.1 Add new job creation form and `POST /job/new` handler in `internal/line/ui.go`.
- [x] 4.2 Add mobile action buttons and reply continuation in `internal/line/ntfy.go`.
- [x] 4.3 Implement SkillVault prompt entry retrieval in `internal/line/prompt.go` with embedded fallback.
- [x] 4.4 Add unit tests for UI form submission, ntfy actions, and SkillVault prompt loading.

## Phase 5: Verification & End-to-End Integration (Issue #87)
- [x] 5.1 Run `go test ./...` across the entire codebase to verify zero regressions.
- [x] 5.2 Build and test `line` CLI end-to-end (`make build-line`).
- [x] 5.3 Validate SDD report with `gentle-ai sdd-verify-validate` and check status with `gentle-ai sdd-status`.
