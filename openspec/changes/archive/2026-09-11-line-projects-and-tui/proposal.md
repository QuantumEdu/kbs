# Proposal: Project Management & Bubble Tea TUI for Line

## Intent
Enable multi-project administration in `line` (Issue #89) with persistent project metadata, active project context selection, and automatic repository resolution for `line run`. Complement this with a keyboard-driven, rich Terminal User Interface (TUI) built on `bubbletea` (Issue #90) allowing developers to inspect jobs, switch projects, answer `needs_human` prompts, and trigger factory phases without leaving the terminal.

## Scope
1. **Projects Domain (`internal/line`)**:
   - `projects` table in SQLite schema with unique slug ID, name, repo path, executor defaults, ntfy topic, and active flag.
   - Project management CLI: `line project {add|list|use|current|remove}`.
   - Automatic fallback to active project in `line run` when `--repo` is omitted.
   - Job association with project ID.
2. **Terminal UI Domain (`internal/line/tui`)**:
   - Bubble Tea application model with Elm architecture (`Init`, `Update`, `View`).
   - Interactive jobs table with phase badges and status styling using `lipgloss`.
   - Inspector viewport displaying job details, PR URL, head SHA, and real-time SQLite event stream.
   - Project switcher modal (`p`) with `bubbles/list`.
   - Continuation modal (`c`) with `bubbles/textarea` for fast `needs_human` responses.
   - New job creation modal (`n`).
   - Hotkeys for single-step (`space`), auto-advance (`a`), refresh (`r`), and quit (`q`).
   - CLI command `line tui`.

## Success Criteria
- 100% test coverage across existing and new packages (`go test ./...`).
- Zero regressions in Windows cross-compilation (`GOOS=windows go build ./cmd/skillvault`).
- `line project add/list/use` reliably manages project contexts.
- `line run <issue-url>` executes against the active project without `--repo`.
- `line tui` compiles cleanly and executes unit tests covering model state transitions.
- SDD verification validation passing via `gentle-ai sdd-verify-validate`.
