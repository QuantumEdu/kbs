# Design: Line Project Management & Bubble Tea TUI

## Architecture Overview
This enhancement augments `line` with two cohesive layers:
1. **Projects Engine (`internal/line`)**:
   - `Project` struct and SQLite persistence in `internal/line/store.go`.
   - Dedicated helper methods: `CreateProject`, `ListProjects`, `GetActiveProject`, `SetActiveProject`, `DeleteProject`.
   - Command router `cmdProject` in `internal/line/cli.go` handling `add`, `list`, `use`, `current`, `remove`.
   - `cmdRun` resolution: if `--repo` is empty, query `store.GetActiveProject(ctx)`.
2. **Interactive TUI (`internal/line/tui`)**:
   - Built on `github.com/charmbracelet/bubbletea`, `bubbles/table`, `bubbles/textarea`, `bubbles/list`, and `lipgloss`.
   - Architectural model follows Elm state loop:
     - `Model`: Stores active project, list of projects, jobs table, selected job events, active modal state (`viewMain`, `viewContinue`, `viewNewJob`, `viewSwitchProject`).
     - `Init`: Loads initial jobs, active project, and fires initial tick.
     - `Update`: Handles keyboard messages, tick timers, modal inputs, and background engine dispatches.
     - `View`: Renders lipgloss styled header, split panels (jobs table + inspector), modal overlays, and footer help bar.

## State Transitions in TUI
- Normal mode:
  - `j`/`k` or `down`/`up`: Navigate jobs table.
  - `space`: Call `engine.Advance(ctx, selectedJobID)`.
  - `a`: Call `engine.AutoAdvance(ctx, selectedJobID)`.
  - `c`: If status is `needs_human`, switch to `viewContinue`.
  - `p`: Switch to `viewSwitchProject`.
  - `n`: Switch to `viewNewJob`.
  - `r`: Force reload jobs and events.
  - `q`: Exit TUI.
- Modal mode (`viewContinue`, `viewNewJob`, `viewSwitchProject`):
  - `enter`: Submit action and return to `viewMain`.
  - `esc`: Cancel modal and return to `viewMain`.
