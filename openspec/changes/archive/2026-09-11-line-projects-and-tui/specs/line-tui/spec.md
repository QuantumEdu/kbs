# Capability Spec: Line Interactive Terminal User Interface (TUI)

## Purpose
Specify requirements for the Charm Bubble Tea interactive terminal dashboard for `line` (GitHub Issue #90).

## Requirements

### Requirement: TUI Launch Command (REQ-TUI-01)
* The CLI MUST provide `line tui [--db <path>]` which launches an interactive Bubble Tea terminal application.

### Requirement: Jobs Dashboard & Inspector (REQ-TUI-02)
* The TUI MUST render a split view containing a table of factory jobs on the left and an inspection details pane on the right.
* The jobs table MUST display Job ID, Phase badge, Status, and Issue URL, navigable via arrow keys (`up`/`down` or `k`/`j`).
* The inspection pane MUST display current phase, issue link, summary, prompt question (if any), and a timeline of recent SQLite events.

### Requirement: Quick Human Continuation Modal (REQ-TUI-03)
* When a selected job is in `needs_human` status, pressing `c` MUST open an interactive modal with a text input/textarea allowing the operator to provide an answer.
* Submitting the answer MUST call `engine.Continue` and resume the factory workflow.

### Requirement: Project Switcher & Job Origination (REQ-TUI-04)
* Pressing `p` MUST display a project switcher list to activate a different project and re-filter the jobs view.
* Pressing `n` MUST open a modal to submit a new GitHub issue URL, launching a job against the active project.

### Requirement: Keyboard Controls & Auto-Advance (REQ-TUI-05)
* The TUI MUST support single-step advance (`space` / `enter`), auto-advance toggle (`a`), manual refresh (`r`), and exit (`q` / `esc`).
