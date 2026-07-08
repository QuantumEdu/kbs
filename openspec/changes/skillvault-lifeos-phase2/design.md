# Design: SkillVault LifeOS Phase 2 — Purpose + Run Bridge + MCP Tools

## Technical Approach

Two additive layers built in chained PRs (PR A → PR B), each under 400 changed lines:

1. **PR A (purpose taxonomy)**: Data-driven — new domain type, migration 007, store propagation, CLI/MCP params, import/export. Default empty string ensures full backward compatibility.
2. **PR B (run bridge + MCP tools)**: Service-oriented — new `RunPipelineStructured` method on existing `WorkflowRunService`, plus two new MCP tools (`run_workflow`, `route_scenario`) wired via builder pattern on `ToolRegistry`.

## Architecture Decisions

| Decision | Option | Tradeoff | Choice |
|----------|--------|----------|--------|
| Purpose type | (A) `type Purpose string` with `IsValid()` | Matches EntryType/Status pattern exactly vs free string validation | **(A)** — follow project convention |
| Migration 007 rebuild | (A) Rebuild entries table (like 006) | Proven pattern, preserves all data vs ALTER which SQLite CHECK-locks | **(A)** — same as 006 |
| Purpose in FTS5 | (A) Add purpose column to FTS5 | Full-text search of purpose values vs (B) SQL WHERE only | **(B)** Purpose is a structured enum, not free text. Filter via `AND e.purpose = ?` |
| Export schema version | (A) Bump from 2 → 3 | Signals new column to consumers vs (B) keep at 2 | **(A)** — bump to 3 for forward-awareness |
| RunPipelineStructured return | (A) New `StructuredRunResult` domain type | Structured JSON vs reusing WorkflowRun | **(A)** — returned inline, not stored differently |
| MCP wiring | (A) Builder `WithWorkflowRunService()` | Same pattern as WithEntryRefService vs (B) constructor param | **(A)** — follow existing ToolRegistry builder convention |

## Data Flow

```
CLI add-entry --purpose KNOWLEDGE
    │
    ▼
AddEntryFlags.Purpose → SaveEntryInput.Purpose → EntryService.SaveEntry
    │
    ▼
domain.Entry.Purpose → db.entries_store.Save() ─→ SQLite (entries.purpose)
    │
    ▼
CLI search --purpose WORK   →   SearchFlags.Purpose → domain.SearchQuery.Purpose
    │
    ▼
entries_store.Search() → SQL: AND e.purpose = ? → filtered results

MCP save_entry {purpose: "LEARNING"}
    │
    ▼
ToolRegistry.handleSaveEntry → SaveEntryInput.Purpose → (same path as CLI)
```

```
MCP run_workflow {workflow: "slug", steps: {1: "input", 2: ""}}
    │
    ▼
ToolRegistry.dispatch("run_workflow") → WorkflowRunService.RunPipelineStructured(ctx, "slug", {1:"input", 2:""})
    │
    ▼
Pre-flight: resolve entry_slugs → validate entries exist & not archived
    │
    ▼  (sequential execution)
variables.Resolve(body, {input, previous_output, final_output}) → execute step N → record run_step
    │
    ▼
return StructuredRunResult{steps: [...], status: "completed"|"failed"}

MCP route_scenario {scenario: "write spec"}
    │
    ▼
ToolRegistry.dispatch("route_scenario") → EntryService.RouteScenario(ctx, "write spec")
    │
    ▼
return RouteResult (JSON-tagged struct from existing PR #16 implementation)
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/domain/entry.go` | Modify | +`Purpose` type, constants, `IsValid()`, `Entry.Purpose`, `EntryFilter.Purpose` |
| `internal/domain/filters.go` | Modify | +`Purpose *string` in `SearchQuery` |
| `internal/domain/validation.go` | Modify | +`ValidatePurpose()` |
| `internal/domain/workflow.go` | Modify | +`StructuredRunResult`, `StructuredStepResult` types |
| `internal/db/migrations/007_purpose.sql` | Create | Rebuild entries table with `purpose TEXT DEFAULT ''` |
| `internal/db/schema.sql` | Modify | +`purpose` column, +index on entries |
| `internal/db/entries_store.go` | Modify | INSERT/SELECT/Search/List/syncFTS include purpose |
| `internal/db/import_export_store.go` | Modify | Export/import includes purpose; bump schema to 3 |
| `internal/app/entries.go` | Modify | `SaveEntryInput.Purpose`, `SaveEntry` uses purpose, pass to search filter |
| `internal/app/workflow_runs.go` | Modify | +`RunPipelineStructured()` method (~95 lines) |
| `internal/mcp/tools.go` | Modify | +`workflowRunSvc` field, `WithWorkflowRunService()`, +purpose params on save_entry/search_entries, +`run_workflow`/`route_scenario` tools and dispatch handlers |
| `internal/cli/commands.go` | Modify | +`Purpose` in `AddEntryFlags`/`SearchFlags`, +`--purpose` flags in parsers |
| `cmd/skillvault/main.go` | Modify | Wire purpose through CLI, wire workflowRunSvc into MCP ToolRegistry |
| `internal/app/app_test.go` | Modify | +purpose round-trip, search filter, validation, import/export tests |
| `internal/app/workflow_runs_test.go` | Modify | +`RunPipelineStructured` tests (success, failure, pre-flight, CLI compat) |

## Interfaces / Contracts

### Purpose domain type (PR A)

```go
// internal/domain/entry.go
type Purpose string

const (
    PurposeWork         Purpose = "WORK"
    PurposeKnowledge    Purpose = "KNOWLEDGE"
    PurposeLearning     Purpose = "LEARNING"
    PurposeRelationship Purpose = "RELATIONSHIP"
    PurposeState        Purpose = "STATE"
)

func (p Purpose) IsValid() bool {
    switch p {
    case PurposeWork, PurposeKnowledge, PurposeLearning,
         PurposeRelationship, PurposeState, "":
        return true
    }
    return false
}

// Added to Entry struct:
//   Purpose Purpose
// Added to EntryFilter:
//   Purpose *string
// Added to SearchQuery:
//   Purpose *string
```

### StructuredRunResult (PR B)

```go
// internal/domain/workflow.go
type StructuredRunResult struct {
    RunID        string                  `json:"run_id"`
    WorkflowID   string                  `json:"workflow_id"`
    WorkflowSlug string                  `json:"workflow_slug"`
    Status       RunStatus               `json:"status"`
    Steps        []StructuredStepResult  `json:"steps"`
    StartedAt    time.Time               `json:"started_at"`
    FinishedAt   *time.Time              `json:"finished_at"`
}

type StructuredStepResult struct {
    StepIndex int       `json:"step_index"`
    Status    RunStatus `json:"status"`
    Output    string    `json:"output,omitempty"`
    Error     string    `json:"error,omitempty"`
}
```

### RunPipelineStructured signature (PR B)

```go
func (s *WorkflowRunService) RunPipelineStructured(
    ctx context.Context,
    workflowRef string,
    stepInputs map[int]string,
) (*domain.StructuredRunResult, error)
```

Returns `error` only on pre-flight or system failures (workflow not found, entry not found, DB errors). Step-level failures produce `status: "failed"` in the result, not a Go error — matching the spec's partial-run semantics. Internally reuses the same `WorkflowRunService` store fields, same `maxPreviousOutputLen` constant, same `variables.Resolve` call chain.

### ToolRegistry builder (PR B)

```go
func (r *ToolRegistry) WithWorkflowRunService(svc *app.WorkflowRunService) *ToolRegistry {
    r.workflowRunSvc = svc
    return r
}
```

### MCP tool schemas (PR A adds purpose parameters; PR B adds two new tools)

- **save_entry** gains `purpose` (string, optional, values from 5-enum set)
- **search_entries** gains `purpose` (string, optional filter)
- **run_workflow** (new): `workflow` (string, required), `steps` (object of "{int}":"string", required)
- **route_scenario** (new): `scenario` (string, required)

## Migration / Rollout

### Migration 007

Follows the exact pattern of 006: rebuild entries table with `purpose TEXT DEFAULT ''` added. Steps: create `entries_new`, copy data, drop old, rename, recreate indexes. Add `purpose TEXT DEFAULT ''` column to schema.sql for fresh databases. Bump `exportSchemaVersion` from 2 to 3 in import_export_store.go (old v2 exports import fine — purpose defaults to `""`).

### Rollback

- **Purpose**: Reverse migration drops purpose column (table rebuild without it). No data loss — purpose is metadata.
- **Run bridge**: Delete `RunPipelineStructured` method and two MCP tool registrations. `RunPipeline` untouched.
- **MCP tools**: Remove `run_workflow` and `route_scenario` from `registerV2Tools()` and dispatch. Remove `workflowRunSvc` field. Existing 16 tools unaffected.

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| **PR A unit** | Purpose validation, search filter, store CRUD, import/export round-trip | `go test ./internal/app/ -run Purpose` — same `:memory:` test pattern as existing `app_test.go` |
| **PR A integration** | CLI `--purpose` flag parsing, MCP `purpose` param passthrough | Add purpose assertions to existing add-entry/search test cases |
| **PR B unit** | `RunPipelineStructured` success, step failure, pre-flight rejection, CLI `run` unchanged | New tests in `workflow_runs_test.go` using existing `setupRunServices` pattern |
| **PR B integration** | MCP `run_workflow` dispatches correctly, `route_scenario` wraps RouteScenario | `mcp/mcp_test.go` — exercise tool dispatch with mock/reference services |
| **Backward compat** | Old entries (no purpose) survive search, save, export/import | Dedicated test: save entry without purpose, verify empty string in DB |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Migration table rebuild corrupts FTS5 index | Low | Follow 006 pattern exactly. Rebuild FTS with `INSERT INTO entries_fts(entries_fts) VALUES('rebuild')` after migration. |
| Purpose enum case-sensitivity mismatch | Low | Store and compare as-is (`TEXT`). Validation accepts only exact uppercase constants. Input normalization to uppercase at validation boundary. |
| Structured run shares mutable state with CLI run (same service instance) | Low | `RunPipelineStructured` uses separate code path, no shared stdin/stdout dependency. Reuses same store, same `maxPreviousOutputLen`, same var resolver. Thread-safe by design (no shared mutable fields). |
| `route_scenario` MCP tool depends on EntryService wiring | Low | Already wired in `main.go` via `SetWorkflowStore`. Tool checks `r.workflowSvc == nil` and returns error. |
| Chained PR dependency: PR B tests may fail without PR A migration | Low | Both PRs share the same migration (007). PR B test setup runs migrations including 007. Tests use `:memory:` DBs. |

## Open Questions

- [ ] Confirm `exportSchemaVersion` bump to 3 is acceptable (minor — backwards-compatible import handles v2 data). *(Resolved: yes, bump to 3.)*
