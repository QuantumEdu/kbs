# Design: SkillVault ↔ Workflow-Builder Bridge (Phase 1)

## Technical Approach

Two additive sub-changes: (A) `routing` entry type woven into existing validation chain, (B) `import-workflow` CLI command that decodes workflow-builder YAML and creates Workflow + Steps + skill entries in a single SQLite transaction. The import creates entries first, collects their slugs, then creates WorkflowSteps with `EntrySlug` linking back to each entry. Both changes are purely additive — no existing contracts break.

## Architecture Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Transaction boundary | `Store.ImportWorkflowWithEntries()` — single tx, creates entries + workflow + steps inline | Existing stores start their own txs; a Store-level method accesses `*sql.DB` directly. Full rollback on failure. |
| Entry mapping | `PhaseYAML` → `SaveEntryInput`: Title=name, Type="skill", Summary=description, Body=phase-skill YAML, Tags=["workflow-phase", phase-id] | Phase-skill body includes name, description, outputs, completion_criteria, depends_on, skill. |
| Slug collision | Check `workflows` for slug; append `-2`, `-3` on collision | Entry slugs handled by existing collision loop. |
| Migration strategy | Table rebuild (002 pattern): backup → rebuild entries with `'routing'` in CK → recreate children → restore → rebuild FTS + indexes | SQLite ALTER CHECK not supported. No `purpose` column (Phase 2). `PRAGMA foreign_keys=OFF`. |
| Project isolation | `--project` flag; provided → scoped, omitted → global (nil project_id) | Follows existing `EntryService` pattern. |
| Security | `filepath.Abs`, CLI-only, no HTTP/MCP endpoint | Path traversal acceptable for local CLI. |

## PhaseYAML → SaveEntryInput Mapping

```
PhaseYAML.id           → Tags: "workflow-phase" + phase-id
PhaseYAML.name         → Title
PhaseYAML.skill        → body template field (YAML)
PhaseYAML.description  → Summary
PhaseYAML.outputs      → body template field (YAML)
PhaseYAML.completion_criteria → body template field (YAML)
PhaseYAML.depends_on   → body template field (YAML)
```

Entry type is always `"skill"`, tags always include `["workflow-phase", "<phase-id>"]`.

## Data Flow

```
workflow.yaml ──→ filepath.Abs() ──→ yaml.Unmarshal → WorkflowYAML{}
                                            │
                          Store.ImportWorkflowWithEntries()
                          ┌── BEGIN TRANSACTION ──┐
                          │                        │
                          │  1. Create N entries   │
                          │     (collect slugs)    │
                          │  2. Create Workflow    │
                          │     (slug collision ✓) │
                          │  3. Create N Steps     │
                          │     (EntrySlug ← slug) │
                          │                        │
                          └── COMMIT / ROLLBACK ───┘
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/domain/entry.go` | Modify | Add `EntryTypeRouting = "routing"`, add to `IsValid()` |
| `internal/domain/validation.go` | Modify | Add `"routing"` + `"handoff"` to `ValidateEntryType` error message |
| `internal/db/schema.sql` | Modify | Add `'routing'` to entries type CHECK |
| `internal/db/migrations/006_routing_and_import.sql` | Create | Table rebuild: routing CK, no purpose, FTS + indexes |
| `internal/db/workflow_import_store.go` | Create | `ImportWorkflowWithEntries()` on Store. YAML structs. |
| `internal/cli/commands.go` | Modify | Add `ImportWorkflowFlags` (--file, --project) |
| `cmd/skillvault/main.go` | Modify | Add `import-workflow` case; wire flags |
| `internal/tui/views.go` | Modify | Add `EntryTypeRouting` case to `typeBadge` |
| `internal/mcp/tools.go` | Modify | Add `"routing"` to `save_entry` type description |

## Interfaces / Contracts

```go
// internal/db/workflow_import_store.go
type PhaseYAML struct {
    ID                 string   `yaml:"id"`
    Name               string   `yaml:"name"`
    Skill              string   `yaml:"skill"`
    Description        string   `yaml:"description"`
    Outputs            []string `yaml:"outputs"`
    CompletionCriteria []string `yaml:"completion_criteria"`
    DependsOn          []string `yaml:"depends_on"`
}

type WorkflowYAML struct {
    Workflow struct {
        Name    string `yaml:"name"`
        Type    string `yaml:"type"`
        Created string `yaml:"created"`
    } `yaml:"workflow"`
    Phases []PhaseYAML `yaml:"phases"`
}

// ImportWorkflowFlags (internal/cli/commands.go)
type ImportWorkflowFlags struct {
    File    string
    Project string
}

// ImportWorkflowWithEntries runs a single tx; returns resulting workflow + entry slugs.
func (s *Store) ImportWorkflowWithEntries(ctx context.Context, yamlData []byte, projectID *string) (*domain.Workflow, []string, error)
```

## Testing Strategy

| Layer | What | Approach |
|-------|------|----------|
| Unit | `IsValid()` with `routing` | Table-driven test in `domain/entry_test.go` |
| Unit | YAML parsing + PhaseYAML → entry mapping | Table-driven test with fixture YAML bytes |
| Unit | Workflow slug collision resolution | Test with pre-existing workflow slug |
| Integration | Migration 006 applies cleanly | Test DB: verify CHECK accepts `routing`, rejects invalid |
| Integration | `ImportWorkflowWithEntries` tx rollback on failure | Inject bad YAML, verify no entries/workflow created |
| CLI | `import-workflow --file <path> [--project <name>]` | Golden-file test with fixture YAML |

## Migration / Rollout

Migration 006 follows the 002 pattern (table rebuild). Child tables backed up, entries rebuilt with `'routing'` in CHECK, children recreated, FTS + indexes rebuilt. `PRAGMA foreign_keys=OFF/ON`. No `purpose` column. No data loss. Rollback: revert migration + code.

## Security

`import-workflow` is CLI-only. Path is canonicalized via `filepath.Abs`. Path traversal outside working directory is acceptable for local CLI use. No HTTP or MCP endpoint in Phase 1.

## Open Questions

- None blocking. Phase 2 will add `purpose` column, MCP exposure, and `skillvault route` command.
