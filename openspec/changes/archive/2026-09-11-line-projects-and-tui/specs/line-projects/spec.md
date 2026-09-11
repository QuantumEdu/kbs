# Capability Spec: Line Project Management & Active Context

## Purpose
Specify the requirements for managing multi-repository projects and active context in `line` (GitHub Issue #89).

## Requirements

### Requirement: Persistent Projects Store (REQ-PROJ-01)
* The SQLite store MUST maintain a `projects` table with fields `id` (slug, PK), `name`, `repo_path` (unique), `default_exec`, `review_exec`, `ntfy_topic`, `is_active` (boolean 0/1), `created_at`, and `updated_at`.
* Only one project MAY have `is_active = 1` at any given time.

### Requirement: Project Registration and Deletion (REQ-PROJ-02)
* The store and CLI MUST support registering a project with `line project add <id> --name <name> --repo <path> [--exec <exec>] [--review-exec <exec>] [--ntfy-topic <topic>]`.
* If it is the first registered project, it MUST be set as active automatically.
* The store and CLI MUST support deleting a project with `line project remove <id>`.

### Requirement: Active Context Switching (REQ-PROJ-03)
* The CLI MUST support `line project use <id>`, marking `<id>` as active (`is_active = 1`) and clearing active status on all other projects.
* The CLI MUST support `line project current`, outputting the active project details or reporting that none is active.

### Requirement: Project Listing (REQ-PROJ-04)
* The CLI MUST support `line project list`, displaying all registered projects with indicators marking the active project.

### Requirement: Automatic Repo Resolution in Line Run (REQ-PROJ-05)
* When `line run` is executed without an explicit `--repo` flag, it MUST check for an active project in the store.
* If an active project exists, it MUST inherit `repo_path`, default `exec`, `review_exec`, and `ntfy_topic` from that project.
* If no active project exists and `--repo` is omitted, `line run` MUST exit with an informative error.
