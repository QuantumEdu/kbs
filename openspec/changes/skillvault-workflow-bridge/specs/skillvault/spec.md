# Delta for SkillVault

## ADDED Requirements

### Requirement: Workflow Import (REQ-WFI)

The system SHALL import workflow-builder YAML files via `import-workflow --file <path>`. Each phase in the YAML MUST produce: one WorkflowStep (order_index, title, instruction, entry_slug), one `skill`-type entry with phase-skill template body (name, description, outputs, completion_criteria, depends_on), and optionally `routing` entries if trigger→workflow mappings are specified.

#### Scenario: Valid workflow.yaml with phases
- GIVEN a `workflow.yaml` with 3 phases, each having name, description, outputs, depends_on
- WHEN `skillvault import-workflow --file workflow.yaml` runs
- THEN a Workflow is created with 3 Steps, 3 skill entries are created with template bodies, AND output confirms N phases imported.

#### Scenario: Missing file or invalid YAML
- GIVEN `--file` points to nonexistent or malformed YAML
- WHEN `import-workflow` runs
- THEN command exits with error, no partial workflow or entries created.

#### Scenario: Empty workflow (no phases)
- GIVEN a `workflow.yaml` with zero phases
- WHEN `import-workflow` runs
- THEN command exits with error: "workflow must have at least one phase".

### Requirement: Routing Entry Type (REQ-ENT-07)

The system SHALL add `routing` as a valid entry type. Routing entries map scenario triggers to workflow slugs. They MUST be valid for save, search, export, import, and all existing entry operations. The database CHECK constraint on `entries.type` MUST be updated to include `routing`.

#### Scenario: Routing entry saved and searchable
- GIVEN entry with type `routing`, title "research trigger", summary "maps research to research-workflow"
- WHEN `save_entry` MCP tool or `add-entry` CLI command is called
- THEN entry is persisted with type `routing`, searchable, exportable, and importable.

## MODIFIED Requirements

### Requirement: Entry Types (REQ-ENT-02)

Required entry types: `prompt`, `skill`, `workflow_note`, `reference`, `user`, `feedback`, `project_state`, `session`, `decision`, `artifact_summary`, `handoff`, `routing`.
(Previously: listed 10 types, no `handoff` or `routing`)

### Requirement: CLI Commands (REQ-CLI-02)

Required commands: `init`, `add-entry`, `search`, `get`, `save-artifact`, `save-result`, `get-context`, `add-project`, `list-projects`, `archive`, `add-workflow`, `render-workflow`, `run`, `import-workflow`, `session-wrap`, `export`, `import`, `sync`, `tui`, `version`, `compare-entries`, `setup-vectors`, `reindex-embeddings`.
(Previously: 22 commands, no `import-workflow`)

#### Scenario: Valid workflow.yaml imported
- GIVEN a valid `workflow.yaml` file with phases
- WHEN `skillvault import-workflow --file workflow.yaml` runs
- THEN Workflow, Steps, and skill entries are created with phase-skill template bodies.

#### Scenario: File not found
- GIVEN `--file missing.yaml` points to nonexistent file
- WHEN `import-workflow` runs
- THEN exit code 1, error "file not found", no data created.
