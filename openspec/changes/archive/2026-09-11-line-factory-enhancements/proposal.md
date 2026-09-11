# Change Proposal: Line Factory Enhancements & Backlog Implementation

## Motivation & Context
`line` is a sibling CLI in the `kbs` suite designed as a deterministic, unattended issue-to-PR factory. Following PR #74 and PR #76, a backlog of product, usability, and debt items was recorded in `docs/line.md` and tracked across GitHub issues #77 through #87:
- **Issue #86**: Windows cross-compilation failure due to `SysProcAttr.Setsid` in `internal/cli/handlers_telemetry.go`.
- **Issue #82**: Documentation of `make install-line` in quickstart and alignment of Makefile comments.
- **Issue #77**: Auto-advance through phases (`plan → build → review → wait_ci`), stopping only on `needs_human`, `blocked`, `failed`, or `ready`.
- **Issue #78**: Polling loop in `wait_ci` waiting for checks to complete instead of single-shot discovery.
- **Issue #79**: Independent review executor flags (`--review-exec`, `--review-exec-arg`) to prevent author-reviewer bias.
- **Issue #80**: Running mutating agent phases in isolated git worktrees.
- **Issue #81**: Persisting PR URL and Head SHA from build/repair JSON outcome directly to the job.
- **Issue #83**: Starting jobs directly from the loopback UI (`line serve`).
- **Issue #84**: Bidirectional ntfy reply/continue support from mobile devices.
- **Issue #85**: Loading phase prompts from SkillVault entries with fallback to embedded templates.
- **Issue #87**: Master tracker for the factory enhancements.

## Proposed Changes
1. **Windows Build Fix**: Extract Unix-specific process group handling into OS-tagged files (`handlers_telemetry_unix.go` and `handlers_telemetry_windows.go`).
2. **Auto-Advance Engine**: Enable uninterrupted execution through `awaiting_next` states while keeping `needs_human` as a strict interactive pause.
3. **CI Polling Loop**: Replace single-shot `wait_ci` checks with an interval poll loop and timeout.
4. **Independent Reviewer**: Support configuring separate review model and command flags.
5. **Worktree Isolation**: Isolate agent mutations in dedicated git worktrees, preserving operator workspaces.
6. **PR/SHA Persistence**: Parse and persist `pull_request` and `head_sha` directly in Job storage.
7. **UI Origination**: Add a clean job origination form in `line serve`.
8. **Mobile ntfy Reply**: Add action buttons and reply handling for phone-based job continuation.
9. **SkillVault Prompts Integration**: Load phase prompts from SkillVault with embedded template fallback.
10. **Documentation**: Update `docs/line.md` and Makefile quickstarts.

## Success Criteria
- `GOOS=windows go build ./cmd/skillvault` succeeds.
- All new and existing tests pass (`go test ./...`).
- SDD specification and verification pass 100%.
- All corresponding GitHub issues (#77-#87) are resolved.
