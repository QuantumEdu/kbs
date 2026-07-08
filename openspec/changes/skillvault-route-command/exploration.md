# Exploration: skillvault route <scenario>

## Current State

### Routing Entry Type

The `routing` entry type exists in the domain model (`internal/domain/entry.go:19` — `EntryTypeRouting = "routing"`), is validated (`internal/domain/validation.go:40`), and the schema CHECK constraint supports it via migration 006 (`internal/db/migrations/006_routing_and_import.sql`). Routing entries can be created via the existing `add-entry --type routing` CLI command.

However, **routing entries are NOT created by `import-workflow`**. The import command (`internal/db/workflow_import_store.go:174`) creates `EntryTypeSkill` entries for each phase, not routing entries. Users must create routing entries manually.

### Routing Entry Structure (per spec)

- **Type**: `routing`
- **Title**: human-readable scenario name
- **Body**: YAML mapping scenario → workflow slug or skill name
- **Tags**: `workflow-route`, plus scenario-specific tag (e.g. scenario-id)

Example body:
```yaml
scenarios:
  research:
    workflow: research-workflow
    skill: research-agent
  interview:
    workflow: interview-pipeline
  onboarding:
    skill: onboarding-skill
```

### Workflow Lookup

`WorkflowService.Get(ctx, id)` (`internal/app/workflows.go:87`) delegates to `sqliteWorkflowStore.Get()` (`internal/db/workflow_store.go:60`), which queries `WHERE id = ? OR slug = ?`. This means workflow IDs AND slugs are valid lookup targets.

`WorkflowService.RenderWorkflow(ctx, id)` (`internal/app/workflows.go:79`) returns ordered steps with titles, instructions, and entry slugs.

### Entry Search Infrastructure

- **FTS5 search**: `EntryService.SearchEntries(ctx, query, filters)` (`internal/app/entries.go:168`) uses `entries_fts MATCH ?` with FTS5 syntax. Supports prefix matching (`*`), phrase matching (`"..."`), and type filtering via `SearchQuery.Type`.
- **Tag search**: `EntryService.SearchByTags(ctx, tags, matchAll, typePtr, projectPtr, limit)` (`internal/app/entries.go:185`) searches by exact tag match with optional type/project filters.
- **Direct retrieval**: `EntryService.GetEntry(ctx, id)` (`internal/app/entries.go:144`) fetches a single entry by ID or slug.

### CLI Command Pattern

Commands are dispatched in `cmd/skillvault/main.go:runCLI` (line 264). Each command follows this pattern:
1. Parse flags via `cli.Parse<Command>Flags()` in `internal/cli/commands.go`
2. Call service method(s) in `internal/app/`
3. Format human-readable output to stdout

Flag parsing uses `flag.NewFlagSet` with `flag.ContinueOnError` and a `nullWriter` to suppress error output. Positional arguments are extracted from `os.Args[2:]` before flag parsing.

### YAML Dependency

`gopkg.in/yaml.v3` is already in `go.mod` and used in:
- `internal/db/workflow_import_store.go` (workflow YAML parsing)
- `internal/sync/config.go` (cloud sync config)

The `internal/app` package does NOT currently import `yaml.v3`. The route command will need YAML parsing in either the app layer or the CLI handler.

### No Existing Route Command

There is no existing `route` command, no route-related flags, and no routing resolution logic anywhere in the codebase. This is greenfield functionality.

---

## Affected Areas

- `cmd/skillvault/main.go:runCLI` — new `case "route":` handler (~40 lines)
- `internal/cli/commands.go` — new `RouteFlags` struct and `ParseRouteFlags()` (~30 lines)
- `internal/app/entries.go` or new `internal/app/routing.go` — new `RouteScenario()` method (~60 lines)
- `internal/cli/commands.go:ParseCommand` — add `"route"` to command dispatch (~3 lines)
- `cmd/skillvault/main.go:commandDescs` — add `"route"` description (~1 line)

---

## Approaches

### Approach 1: App Service Method on EntryService

Add a `RouteScenario(ctx, scenario)` method to `EntryService` that encapsulates all resolution logic. The CLI handler calls it and formats output.

```
skillvault route <scenario>
  → EntryService.RouteScenario(ctx, scenario)
    → SearchEntries(type="routing", query=scenario)  [FTS5, fuzzy]
    → SearchByTags(tags=["workflow-route", scenario]) [tag exact]
    → Parse YAML bodies for exact key match
    → Resolve workflow slug → WorkflowService.Get()
    → Return resolved workflow + matching entry
```

- **Pros**: Testable at the app layer, reusable by MCP tools / HTTP API later, follows existing architecture (EntryService owns entry queries).
- **Cons**: Adds yaml.v3 dependency to `internal/app`, more code than CLI-only.
- **Effort**: Medium (~4 files, ~130 lines)

### Approach 2: CLI-Only Resolution

Parse and resolve entirely in the `runCLI` switch case. Inline YAML parsing, FTS search, and workflow lookup.

- **Pros**: Minimal code, no new dependencies in app layer, faster to ship.
- **Cons**: Not testable in isolation, not reusable by MCP/API, mixes parsing with dispatch.
- **Effort**: Low (~1 file, ~60 lines)

## Recommendation

**Approach 1** — App Service Method. The extra ~70 lines are justified by:
1. Testability — route resolution logic can be tested with mock stores
2. Reusability — the same method can be exposed via MCP `route_workflow` tool and HTTP API later
3. Architecture consistency — `WorkflowService` and `EntryService` already own business logic; routing is a cross-cutting query across both
4. The yaml.v3 dependency already exists in the project; no new go.sum entries needed

The resolution algorithm:
1. **Primary**: Search entries of type `routing` via FTS5 with the scenario string as query. FTS5's built-in prefix/term matching handles fuzzy scenarios naturally.
2. **Secondary**: Search by tag `workflow-route` (match any) filtered to type `routing` if FTS5 returns nothing.
3. **Parse bodies**: Unmarshal each routing entry's `body_optional` as YAML. Check for exact key match against the scenario string.
4. **Resolve**: For the best match, read the `workflow` field (slug) or `skill` field (entry slug/skill name), call `WorkflowService.Get()` to retrieve workflow details.
5. **Output**: Display workflow name, slug, description, and rendered steps (via `RenderWorkflow`).

Fallback cascade:
```
FTS5 search ("routing" type, scenario as query)
  → if 0 results: tag search ("workflow-route" tag)
    → if 0 results: "No routing entries found for scenario X"
  → parse bodies, match scenario key
    → if no key match: list closest titles as suggestions
  → resolve workflow slug → WorkflowService.Get()
    → if workflow not found: warn "Referenced workflow Y not found"
```

## Risks

| Risk | Severity | Mitigation |
|------|----------|------------|
| **No routing entries exist** | Low | Command prints a clear message ("No routing entries found. Create one with: skillvault add-entry --type routing ...") |
| **YAML body is malformed** | Medium | Skip malformed entries with a warning to stderr. Don't crash the entire resolution. |
| **Workflow slug is stale** | Medium | When `WorkflowService.Get()` returns not-found, warn "Referenced workflow X no longer exists" and continue checking other entries. |
| **FTS5 search returns irrelevant results** | Low | Filter by type `routing` and by tag `workflow-route`. FTS5 tokenization handles word prefixes naturally. |
| **Scenario ambiguity (multiple matches)** | Low | Display all matches and let the user disambiguate. Use exact key match priority (YAML key match > FTS5 title match > tag match). |
| **yaml.v3 in app layer** | None | Already a direct dependency in go.mod. Used by `internal/db/workflow_import_store.go`. No new imports. |
| **No test coverage for routing entries** | Medium | Write an end-to-end acceptance test in `internal/app/app_test.go` that creates a routing entry with a YAML body, then routes by scenario. |

## Ready for Proposal

**Yes**. The feature is well-scoped: one new CLI command, one new app method, leveraging existing search and workflow infrastructure. No database changes needed. The `routing` entry type and migration already exist. The only new behavior is YAML body parsing in the app layer and scenario-resolution logic.
