# Exploration: SkillVault ↔ Workflow-Builder Bridge + LifeOS Alignment

## 1. Current State — SkillVault Workflow Capabilities

### Domain Layer (`internal/domain/workflow.go`, `internal/domain/entry.go`)

**Workflows**: `Workflow` struct (ID, Name, Slug, Description, Status) + `WorkflowStep` (ID, OrderIndex, Title, Instruction, Required, ExpectedOutput, EntrySlug). Steps link to entries via `entry_slug` — an entry body is resolved at run time as the prompt template.

**Workflow Runs**: `WorkflowRun` + `WorkflowRunStep` track execution state (pending/running/completed/failed) with input/output per step. Runs are persisted in `runs` + `run_steps` tables.

**Entry Types**: 11 types: `prompt`, `skill`, `workflow_note`, `reference`, `user`, `feedback`, `project_state`, `session`, `decision`, `artifact_summary`, `handoff`. No `routing` type exists.

### CLI Commands (`cmd/skillvault/main.go:runCLI`)

- **`add-workflow`** (lines 499–525): Reads a JSON file, unmarshals to `SaveWorkflowInput`, calls `SaveWorkflow()`. **JSON-only — no YAML support.**
- **`render-workflow`** (lines 527–554): Renders steps as a numbered checklist from the DB.
- **`run`** (lines 556–597): `RunPipeline(workflowSlug, input, stdin, stdout)` — sequential pipeline that reads step prompts from stdin, writes rendered prompts to stdout. Supports `--save` for output file.

### MCP Tools (`internal/mcp/tools.go`)

- **`render_workflow`** (lines 488–516): Read-only rendering of workflow steps. Uses `WorkflowService.RenderWorkflow()` which delegates to `WorkflowStore.Render() → Get() → GetSteps()`.
- **No `run_workflow` tool**: The run pipeline is CLI-only (stdin/stdout I/O model), not exposed via MCP.
- **No `add_workflow` tool**: Workflow creation is CLI-only.

### Variable Resolution (`internal/vars/resolver.go`)

`Resolve(content, providedVars, globals)` replaces `{{key}}` placeholders. During pipeline execution, 3 system vars are provided:
- `{{input}}` — the initial input to the run
- `{{previous_output}}` — output of the immediately previous step (truncated at 32KB)
- `{{final_output}}` — accumulated output of all completed steps

`PrepareGlobals()` adds `{{date}}` and `{{project}}` as global fallback vars.

### Store layer (`internal/db/schema.sql`)

- `workflows` table: id, name, slug, description, status (matching Entry Status enum: draft/active/archived/deprecated/canonical)
- `workflow_steps` table: id, workflow_id FK, order_index, title, instruction, required (int), expected_output, entry_slug
- `runs` table: id, workflow_id FK, input, output, status (pending/running/completed/failed)
- `run_steps` table: id, run_id FK, step_id INTEGER FK (not TEXT), entry_id FK, input, output, status

**Key observation**: `workflow_steps.id` is TEXT in the schema but stored as an auto-increment INTEGER in practice (see `GetSteps()` which does `fmt.Sprintf("%d", stepID)`). `run_steps.step_id` is INTEGER referencing the same. This is a type mismatch (TEXT column, INTEGER value) that works in SQLite's flexible typing.

### Execution Model (`internal/app/workflow_runs.go`)

`RunPipeline()` is purely sequential:
1. Resolve workflow by slug → get steps
2. Pre-flight: resolve each step's `entry_slug` to an entry ID + body
3. Create run + run_steps records
4. For each step: resolve variables in entry body → write to stdout → read response from stdin → next
5. No parallelism, no conditional branching, no phase dependencies beyond sequence

### Summary of current state

| Capability | Status |
|-----------|--------|
| Workflow creation | CLI only, JSON only |
| Workflow rendering | CLI + MCP (read-only) |
| Workflow execution | CLI only, sequential, stdin/stdout pipeline |
| Variable substitution | 3 system vars + globals, `{{key}}` syntax |
| YAML support | None |
| Import from external workflows | None |
| Routing / scenario resolution | None |
| Purpose taxonomy on entries | None |
| Phase-skill template entries | None (no standard format) |

---

## 2. What workflow-builder Produces and Expects

From the `sdd-govplan/workflow-builder/SKILL.md` and `governance/04-workflow-map.md`:

### Generated Artifacts

All files live under `.agent/skills/{workflow-name}/`:

1. **`workflow.yaml`** — top-level definition:
   ```yaml
   workflow:
     name: "{workflow-name}"
     type: "{project-type}"  # thesis, consulting, pnl-coaching, career-coaching, custom
     created: "{YYYY-MM-DD}"
   phases:
     - id: "001"
       name: "{Phase name}"
       skill: "phase-001-{slug}"
       description: "..."
       outputs: ["file1", "file2"]
       completion_criteria: ["criterion"]
       depends_on: []        # phase dependency DAG
     - id: "002"
       ...
       depends_on: ["001"]
   ```

2. **Phase skill files** (`phase-{id}-{slug}.SKILL.md`): One per phase, generated from `assets/phase-skill-template.md`. ~50–150 lines each. Include phase name, description, outputs, completion criteria, dependencies.

3. **`progress.yaml`** — execution tracker:
   ```yaml
   workflow: "{workflow-name}"
   current_phase: 0
   total_phases: {N}
   phases:
     - id: "001"
       name: "{Phase 1}"
       status: "pending"      # pending|in_progress|completed|skipped
       completed_at: null
       artifacts: []
   ```

### Built-in Templates

| Template | Phases | Domain |
|----------|--------|--------|
| `thesis.yaml` | 8 | Academic research |
| `consulting.yaml` | 6 | ISO 27001, consulting |
| `pnl-coaching.yaml` | 5 | NLP coaching |
| `career-coaching.yaml` | 5 | Career coaching |

### Execution Model

- `/workflow continue` reads `progress.yaml`, finds first pending phase, loads its SKILL.md, executes.
- After execution: updates `progress.yaml` (status → completed, artifacts list, timestamp).
- Sequential by default, respecting `depends_on` DAG.

### What workflow-builder expects from a host system

1. File system access under `.agent/skills/{name}/`
2. Skill loading mechanism (the `skill()` function)
3. Progress tracking persistence
4. Sequential phase execution with context passing between phases

---

## 3. LifeOS Patterns Applicable to SkillVault

### Memory by Purpose (v7.6)

LifeOS structures memory by **purpose**, not just type:
- **WORK** — active projects, tasks, deliverables
- **KNOWLEDGE** — typed graph of facts, concepts, references
- **LEARNING** — skills acquired, lessons, progress
- **RELATIONSHIP** — people, organizations, contacts
- **OBSERVABILITY** — metrics, events, system state
- **STATE** — current state snapshots (routing toward Ideal State)

**Applicability**: SkillVault entries currently classify by type (prompt, skill, decision) but not by purpose. A `purpose` field would enable:
- Filtering by purpose across types
- LifeOS-compatible memory import/export
- Purpose-aware context compilation

### Router System

LifeOS maps triggers to workflows explicitly in SKILL.md:
```
| Trigger | Workflow |
|---------|----------|
| "research this" | research workflow |
| "interview", "onboard me" | interview workflow |
```
The router is a **lookup table** — scenario keyword → workflow.

**Applicability**: SkillVault's `render_workflow` already knows how to list steps. A `workflow-map` entry type could store routing rules (trigger → workflow slug), and a `skillvault route <scenario>` command would resolve which workflow to run.

### Two-Tier Distribution (Core + Enhancements)

LifeOS ships Core (skills + runtime) as one consent bundle, then à la carte enhancements (hooks, agents, Pulse, statusline). Each enhancement is independently deployable, idempotent, and reversible.

**Applicability**: The `import-workflow` command should be similarly additive — import a `workflow.yaml`, create entries + workflow, but never clobber existing entries with matching slugs.

### Workflow Routing Table (within SKILL.md)

The LifeOS SKILL.md includes an explicit workflow routing table:
```
## Workflow Routing
| Trigger | Workflow |
```

**Applicability**: SkillVault's `workflow-map` entry type could store an equivalent routing table, making it queryable, exportable, and syncable — unlike a comment in a SKILL.md file.

### ISA (Ideal State Artifact)

LifeOS defines 5 identities across 12 sections in TOML. The Algorithm (OBSERVE → THINK → PLAN → BUILD → EXECUTE → VERIFY → LEARN) drives progress from Current State to Ideal State.

**Applicability**: The 7-phase Algorithm could be a built-in workflow template in SkillVault. The `purpose` taxonomy maps directly to LifeOS memory structure.

---

## 4. Gaps and How Phase 1 + Phase 2 Close Them

### Gap 1: No YAML workflow support
**Root cause**: `add-workflow` only accepts JSON via `json.Unmarshal`.
**Phase 1 fix**: `import-workflow` command parses workflow-builder's `workflow.yaml` format and translates it to `SaveWorkflowInput`.

### Gap 2: No routing/scenario resolution
**Root cause**: Workflows exist in DB but there's no mechanism to map "what I want to do" → "which workflow to run."
**Phase 1 fix**: Add `routing` entry type. Store workflow-map rules in entries (trigger → workflow slug, scenario context).
**Phase 2 fix**: `skillvault route <scenario>` command that queries routing entries and returns the matching workflow.

### Gap 3: No entry body as phase-skill template
**Root cause**: Entry `body_optional` exists but has no standard schema for phase-skill templates.
**Phase 1 fix**: Define a phase-skill template schema in `docs/` that `import-workflow` can generate from workflow.yaml phases.

### Gap 4: No purpose taxonomy
**Root cause**: Entries classified by functional type only (prompt, skill, etc.), not by purpose domain.
**Phase 2 fix**: Add `purpose` field to entries (WORK/KNOWLEDGE/LEARNING/RELATIONSHIP/STATE) aligned with LifeOS memory taxonomy.

### Gap 5: No MCP run capability
**Root cause**: `RunPipeline` is CLI-only, using stdin/stdout for step I/O. MCP tools use structured JSON args/results.
**Phase 2 fix**: `skillvault run` bridge that maps workflow-builder phase skills to SkillVault workflow steps. Expose `run_workflow` as an MCP tool with structured I/O (not stdin/stdout).

### Gap 6: No YAML parser in SkillVault
**Root cause**: SkillVault has YAML only in `sync/config.go` for cloud config. No general YAML parser.
**Phase 1 fix**: Use existing `gopkg.in/yaml.v3` dependency (already in go.mod) to parse workflow-builder YAML.

---

## 5. Phase-by-Phase Delivery

### Phase 1 (now): Foundation

| # | Capability | What changes |
|---|-----------|-------------|
| 1 | `routing` entry type | Add to `EntryType` enum (`internal/domain/entry.go`), schema CHECK constraint, CLI + MCP |
| 2 | `import-workflow` command | New CLI command. Parses `workflow.yaml` (YAML), maps phases → `SaveWorkflowInput` with steps, creates entries for each phase as type `skill` with phase-skill template body |
| 3 | Phase-skill template body format | Document a standard `body_optional` schema for phase skills (name, description, outputs, completion criteria, depends_on, next/prev phase references) |

**Files affected**:
- `internal/domain/entry.go` — add `EntryTypeRouting`
- `internal/db/schema.sql` — update CHECK constraint (migration)
- `internal/cli/commands.go` — add `ImportWorkflowFlags`, `ParseImportWorkflowFlags`
- `cmd/skillvault/main.go` — add `import-workflow` case in `runCLI`
- `internal/app/workflows.go` — add `ImportWorkflow()` method (or new `WorkflowImportService`)
- New: `docs/phase-skill-template.md` — schema documentation

### Phase 2 (next): Router + Run Bridge + Purpose

| # | Capability | What changes |
|---|-----------|-------------|
| 4 | `skillvault route <scenario>` | Queries routing entries, resolves scenario → workflow, renders matching workflow |
| 5 | `purpose` taxonomy | Add `purpose` field to Entry domain, schema, CLI flags, MCP tool args. Values: WORK, KNOWLEDGE, LEARNING, RELATIONSHIP, STATE |
| 6 | `skillvault run` bridge | Extend `RunPipeline` to accept structured step input (not just stdin). Expose `run_workflow` MCP tool. Support workflow-builder `progress.yaml` update on completion. |

---

## 6. Risks and Unknowns

| Risk | Severity | Mitigation |
|------|----------|------------|
| **workflow-builder YAML format may evolve** | Medium | Phase 1 `import-workflow` parses a known snapshot. Version field in workflow.yaml enables future format detection. Document expected schema clearly. |
| **`agentcore_publish` is Windows-only, inaccessible** | High | Phase 1 produces portable artifacts (new command, entry type, docs). No code touches Windows paths. `import-workflow` can be tested with fixtures. |
| **LifeOS purpose taxonomy mismatch with SkillVault entry model** | Low | Purpose is additive (new field, not replacing type). Entries keep their type; purpose is orthogonal. LifeOS uses 6 purposes; SkillVault starts with 5 (WORK/KNOWLEDGE/LEARNING/RELATIONSHIP/STATE), skipping OBSERVABILITY for Phase 2. |
| **`run` MCP tool needs I/O model redesign** | Medium | Current `RunPipeline` uses stdin/stdout — incompatible with MCP's JSON-RPC request/response. Phase 2 needs a new `RunPipelineStructured()` that takes step inputs as arguments and returns outputs as results. |
| **Migration impact on existing vaults** | Low | Adding `routing` type and `purpose` column are additive. CHECK constraints need careful migration (SQLite doesn't support ALTER CHECK). Use migration pattern established in prior changes. |
| **YAML parser already in go.mod** | None | `gopkg.in/yaml.v3` is already a dependency (used by `internal/sync/config.go`). No new dependency needed. |
| **workflow_steps.id type mismatch** | Low | TEXT column storing INTEGER values. Works in SQLite but needs attention if strict typing is ever enforced. Not in scope for this change. |

---

## 7. OpenSpec Conventions Check

File to check: `openspec/config.yaml` — if it exists, rules for explore phase apply.

No prior `skillvault-workflow-bridge` artifacts exist in Engram or OpenSpec. This is the first artifact for this change.

## 8. Ready for Proposal

**Yes**. Phase 1 is well-scoped (3 capabilities, 4–5 files changed), additive (no breaking changes), and produces immediately usable artifacts (import-workflow command, routing entry type, documented template schema). Phase 2 builds on Phase 1's foundation and can be planned after Phase 1 implementation.
