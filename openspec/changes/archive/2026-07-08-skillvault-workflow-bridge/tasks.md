# Tasks: SkillVault ↔ Workflow-Builder Bridge (Phase 1)

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~350 |
| 400-line budget risk | Medium |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 → PR 2 → PR 3 |
| Delivery strategy | auto-chain |
| Chain strategy | feature-branch-chain |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: feature-branch-chain
400-line budget risk: Medium

### Suggested Work Units

| Unit | Goal | Likely PR | Notes |
|------|------|-----------|-------|
| 1 | Domain + Validation + Migration | PR 1 | base: `feature/skillvault-workflow-bridge`. Additive; no behavior change. |
| 2 | Import Store + CLI Wiring | PR 2 | base: PR 1 branch. `ImportWorkflowWithEntries()` on Store, `import-workflow` cmd. |
| 3 | Tests + MCP/TUI touchup | PR 3 | base: PR 2 branch. Unit + integration + CLI golden-file tests. |

## Phase 1: Foundation (PR #1)

- [x] 1.1 Add `EntryTypeRouting = "routing"` const to `internal/domain/entry.go`, add to `IsValid()` switch
- [x] 1.2 Update `ValidateEntryType` error message in `internal/domain/validation.go` to include `routing` and `handoff`
- [x] 1.3 Add `'routing'` to entries type CHECK in `internal/db/schema.sql` line 28
- [x] 1.4 Create `internal/db/migrations/006_routing_and_import.sql` — table rebuild (002 pattern): backup entries → rebuild with `'routing'` in CHECK → recreate children → rebuild FTS + indexes. PRAGMA foreign_keys=OFF/ON. No `purpose` column.

## Phase 2: Core Import (PR #2)

- [ ] 2.1 Create `internal/db/workflow_import_store.go` — `PhaseYAML` + `WorkflowYAML` structs with yaml tags; `ImportWorkflowWithEntries(ctx, yamlData []byte, projectID *string) (*domain.Workflow, []string, error)` on `*Store`. Single tx: creates N entries (type=`skill`, tags=[`workflow-phase`,`<phase-id>`]), creates Workflow (slug collision append `-2`), creates N Steps with EntrySlug linking to entry slugs. PhaseYAML → SaveEntryInput per design mapping; body as phase-skill template YAML.
- [ ] 2.2 Add `ImportWorkflowFlags` struct (File, Project string) + `ParseImportWorkflowFlags()` to `internal/cli/commands.go`
- [ ] 2.3 Add `import-workflow` case to `ParseCommand()` in `internal/cli/commands.go` (requires `--file`)
- [ ] 2.4 Add `import-workflow` case to `switch cmd` in `cmd/skillvault/main.go` runCLI(): read file → call `store.ImportWorkflowWithEntries()` → print workflow ID + N phases imported
- [ ] 2.5 Add `"routing"` to `save_entry` type description in `internal/mcp/tools.go` line 85
- [ ] 2.6 Add `EntryTypeRouting` case to `typeBadge()` in `internal/tui/views.go` (use color "213" or fall to default)

## Phase 3: Testing (PR #3)

- [ ] 3.1 Add `routing` test case to existing `IsValid` table-driven test in `internal/domain/entry_test.go`
- [ ] 3.2 Update `ValidateEntryType` error message assertion in `internal/domain/validation_test.go`
- [ ] 3.3 Unit test YAML parsing + PhaseYAML mapping in `internal/db/workflow_import_store_test.go` (fixture YAML bytes, verify entry fields)
- [ ] 3.4 Unit test workflow slug collision resolution (pre-create workflow, verify `-2` suffix)
- [ ] 3.5 Integration test: migration 006 applies cleanly on real DB (verify CHECK accepts `routing`, rejects invalid)
- [ ] 3.6 Integration test: `ImportWorkflowWithEntries` tx rollback on invalid YAML (verify no entries/workflow created)
- [ ] 3.7 CLI golden-file test: `import-workflow --file fixture.yaml [--project <name>]` in `internal/cli/cli_test.go`
- [ ] 3.8 Spec scenario verification: valid workflow.yaml import, missing file, empty workflow (REQ-WFI scenarios)
- [ ] 3.9 Spec scenario verification: routing entry saved and searchable (REQ-ENT-07)
