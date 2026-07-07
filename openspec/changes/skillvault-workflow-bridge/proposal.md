# Proposal: SkillVault ↔ workflow-builder Bridge (Phase 1)

## Intent

SkillVault workflows are CLI-only and JSON-only — workflow-builder produces YAML phase definitions that can't be imported. There's no routing entry type to map scenarios to workflows. This Phase 1 bridges the gap by adding YAML workflow import and a `routing` entry type, making workflow-builder artifacts consumable by SkillVault.

## Scope

### In Scope
- `routing` entry type — maps scenario triggers to workflow slugs
- `import-workflow` CLI command — parses `workflow.yaml`, creates Workflow + Steps + phase-skill entries
- Phase-skill template body schema — standardized `body_optional` format for imported phase entries
- Migration: new CHECK constraint for `routing` type, new `purpose` column (nullable, future use)

### Out of Scope
- `skillvault route <scenario>` command (Phase 2)
- `purpose` taxonomy filter/search (Phase 2)
- `skillvault run` bridge — MCP exposure + structured I/O (Phase 2)
- Workflow builder output as MCP tool (Phase 2)

## Capabilities

### New Capabilities
- `workflow-import`: Parse workflow-builder YAML, translate phases into SaveWorkflowInput with step entries

### Modified Capabilities
- `entry-types`: Add `routing` to EntryType enum (11→12). Update CHECK constraint in schema. Add to CLI/MCP type validation.
- `cli-commands`: Add `import-workflow` command accepting `--file <path>` (YAML input)

## Approach

Two additive changes, no breaking behavior:

1. **EntryType `routing`**: Add const to `internal/domain/entry.go`, update `IsValid()`, migration for CHECK constraint, validate in CLI/MCP arg parsing.
2. **`import-workflow`**: New `WorkflowImportService` in `internal/app/`. Decodes YAML with existing `gopkg.in/yaml.v3` dependency. Maps phases to `SaveWorkflowInput` (name, description, steps with order_index + entry_slug). For each phase, creates a `skill`-type entry with phase-skill template body (name, description, outputs, completion criteria, depends_on). Creates routing entries for trigger→workflow mappings if specified in YAML.

Both are purely additive — no existing workflows, entries, or API contracts change.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/domain/entry.go` | Modified | Add `EntryTypeRouting` const, update `IsValid()` |
| `internal/db/schema.sql` | Modified | Migration: CHECK constraint update + `purpose` column |
| `internal/app/workflows.go` | Modified | Add `ImportWorkflow()` method |
| `internal/cli/commands.go` | Modified | Add `ImportWorkflowFlags` |
| `cmd/skillvault/main.go` | Modified | Add `import-workflow` case |
| `docs/phase-skill-template.md` | New | Schema documentation |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| workflow-builder YAML format evolves | Medium | Version field detection in YAML; document expected schema |
| Migration CHECK constraint in SQLite | Low | Use migration pattern from prior changes (new table copy) |
| `routing` type confuses existing queries | Low | Additive — no filter changes. `routing` is treated like any entry type. |

## Rollback Plan

1. Revert migration (no data loss — `routing` type is additive, `purpose` column nullable)
2. Revert CLI command registration in `main.go`
3. Revert domain/service additions
4. No API contract changes to unwind

## Dependencies

- `gopkg.in/yaml.v3` (already in `go.mod`, used by `internal/sync/config.go`)

## Success Criteria

- [ ] `skillvault import-workflow --file workflow.yaml` creates Workflow + Steps + skill entries from a workflow-builder YAML
- [ ] `routing` entries save, search, and export correctly
- [ ] Existing workflows, entries, and search unaffected
- [ ] Migration runs without errors on existing vaults
