# Tasks: Line Project Management & Bubble Tea TUI

## Phase 1: Project Store Schema & Engine Integration (Issue #89)
- [x] 1.1 Add `Project` struct in `internal/line/project.go` and `projects` table in `internal/line/store.go`.
- [x] 1.2 Implement store methods `CreateProject`, `ListProjects`, `GetActiveProject`, `SetActiveProject`, `DeleteProject`.
- [x] 1.3 Add unit tests for project store operations in `internal/line/project_test.go`.

## Phase 2: Project CLI & Active Fallback in Line Run (Issue #89)
- [x] 2.1 Implement `cmdProject` in `internal/line/cli.go` supporting `add`, `list`, `use`, `current`, and `remove`.
- [x] 2.2 Update `cmdRun` to resolve `--repo` and defaults from `store.GetActiveProject` when omitted.
- [x] 2.3 Add CLI unit tests in `internal/line/cli_test.go` for project subcommands and active fallback.

## Phase 3: Bubble Tea Interactive TUI (Issue #90)
- [x] 3.1 Implement Bubble Tea model and styling in `internal/line/tui/tui.go` with split view and modals.
- [x] 3.2 Wire `line tui` subcommand in `internal/line/cli.go`.
- [x] 3.3 Add unit tests for TUI state updates, modals, and key bindings in `internal/line/tui/tui_test.go`.

## Phase 4: Verification & Integration (Issues #89, #90)
- [x] 4.1 Run `go test ./...` across the entire codebase to verify zero regressions.
- [x] 4.2 Verify Windows cross-compilation (`GOOS=windows go build ./cmd/skillvault`).
- [x] 4.3 Validate SDD report with `gentle-ai sdd-verify-validate` and archive change.
- [x] 4.4 Create GitHub PR, merge into `main`, and install updated binaries (`make install-all && make install-line`).

## Phase 5: Tutorial HTML Documentation
- [x] 5.1 Update `docs/tutorial-line-factory.html` and `tuts-advanced/agent-engineering/tutorial-line-factory.html` with project management and TUI guides.
