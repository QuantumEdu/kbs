# Design: skillvault route \<scenario\>

## Technical Approach

Additive change — one CLI command (`route`), one app method (`RouteScenario`), zero migrations. Leverages existing FTS5, tag search, `yaml.v3`, and `WorkflowService.Get()`. The resolution cascade: FTS5 type=routing → tag `workflow-route` fallback → YAML body key match → workflow lookup.

```
CLI (cmd/) ──→ cli/ParseRouteFlags ──→ runCLI: case "route"
                    │
                    └── EntryService.RouteScenario(ctx, scenario)
                          ├── SearchEntries(type="routing", query=scenario)
                          ├── SearchByTags(tags=["workflow-route"], type="routing")
                          ├── yaml.Unmarshal body_optional → key match
                          └── WorkflowService.Get(slug) → verify target
```

## Architecture Decisions

| Decision | Choice | Tradeoff | Rationale |
|----------|--------|----------|-----------|
| Resolution location | App service on `EntryService` | +70 LOC vs CLI-only, adds yaml.v3 to `internal/app` | Testable, reusable by future MCP tool. yaml.v3 already in go.mod via `workflow_import_store.go`. |
| Resolution cascade order | FTS5 → tags → YAML key match | Multiple search passes vs single query | FTS5 handles fuzzy/natural language; YAML exact key match is fallback for structured routing bodies. |
| Malformed YAML handling | Skip + warn stderr, continue | Silent skip vs hard error | Spec: one bad routing entry MUST NOT block resolution of others. |
| Stale workflow reference | Warn + continue | Warn vs hard error | Routing entries outlive workflows; resolution should degrade gracefully. |
| JSON output flag | `--json` boolean | Single flag vs `--format json` | Matches proposal. Simpler than sub-command style. |
| No new CLI dispatch | Inline in `runCLI` switch | No new file | Matches existing pattern. All 31 commands are inline in main.go's switch. |

## Data Flow

```
skillvault route "research"
  │
  ├─ ParseRouteFlags(os.Args) → RouteFlags{Scenario:"research", JSON:false}
  ├─ svc.entrySvc.RouteScenario(ctx, "research")
  │    │
  │    ├─ SearchEntries(type="routing", query="research") → []EntrySearchResult
  │    │   (FTS5 MATCH against entries_fts, filtered by type=routing)
  │    │
  │    ├─ if empty: SearchByTags(["workflow-route"], type="routing") 
  │    │
  │    ├─ For each candidate entry:
  │    │   ├─ yaml.Unmarshal(body_optional) → map[string]RouteTarget
  │    │   ├─ Check exact key match against scenario
  │    │   └─ On malformed YAML: warn stderr, skip
  │    │
  │    ├─ Best match → extract workflow slug or skill name
  │    ├─ WorkflowService.Get(slug) → domain.Workflow
  │    │   (via sqliteWorkflowStore.Get: WHERE id = ? OR slug = ?)
  │    │
  │    └─ Return RouteResult{Scenario, Type, Target, Description, Workflow}
  │
  ├─ if --json: json.Marshal + fmt.Println
  └─ else: human-readable (workflow name, steps from RenderWorkflow)
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/cli/commands.go` | Modify | Add `RouteFlags{Scenario, JSON bool}`, `ParseRouteFlags()` (~25 LOC) |
| `internal/app/entries.go` | Modify | Add `RouteScenario(ctx, scenario) (*RouteResult, error)` (~70 LOC) |
| `cmd/skillvault/main.go` | Modify | Add `"route"` to `commandDescs`, `case "route":` in `runCLI` (~50 LOC) |

## Interfaces / Contracts

```go
// internal/app/entries.go — new types

type RouteTarget struct {
    Workflow string `yaml:"workflow"`
    Skill    string `yaml:"skill"`
}

type RouteResult struct {
    Scenario    string           `json:"scenario"`
    Type        string           `json:"type"`      // "workflow" or "skill"
    Target      string           `json:"target"`    // slug
    Description string           `json:"description"`
    Workflow    *domain.Workflow `json:"workflow,omitempty"`
}

func (s *EntryService) RouteScenario(ctx context.Context, scenario string) (*RouteResult, error)
```

```go
// internal/cli/commands.go — new flag type

type RouteFlags struct {
    Scenario string
    JSON     bool
}

func ParseRouteFlags(args []string) (*RouteFlags, error)
```

## Testing Strategy

| Layer | What | How |
|-------|------|-----|
| Integration | `app.RouteScenario` — creates routing entry + workflow, resolves by scenario | In-memory SQLite in `internal/app/app_test.go` |
| Integration | Malformed YAML skip + continue | Same test, 2 entries: 1 bad YAML, 1 valid |
| Unit | `cli.ParseRouteFlags` — positional arg + --json | Table-driven in `internal/cli/cli_test.go` |

## Migration / Rollout

No migration required. Routing entry type and FTS5 index already exist. Rollback: revert commit.

## Open Questions

- [ ] Should `RouteScenario` also resolve skill-type entries (not just workflow slugs)? Proposal says "workflow or skill" but imports only `WorkflowService.Get`. Defer to Phase 2.
- [ ] Should `--json` output include rendered workflow steps? Proposal says json fields are `scenario, type, target, description`. Defer rendered steps to human-readable output only.
