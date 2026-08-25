# Changelog

All notable changes to SkillVault Qu@ntum are documented here.

## Unreleased

### Changed

- Adopted Semantic Versioning (`MAJOR.MINOR.PATCH`) with `internal/version` as the single source of truth; binary, MCP initialize response, and export stamps now report `v3.0.0` instead of bare `v3`.

## v3.0.0 — Workflow Pipelines

**2026-06-23**

### Added

- **Workflow Pipelines**: ejecución secuencial de steps con `{{input}}`, `{{previous_output}}`, `{{final_output}}`
  - Nuevas tablas `runs` + `run_steps` (migración 004)
  - `WorkflowRun` / `WorkflowRunStep` domain entities
  - `WorkflowRunStore` CRUD (CreateRun, GetRun, ListRuns, UpdateStepStatus)
  - `WorkflowRunService.RunPipeline()` — pre-flight, variable injection, IO paso a paso
- **CLI `run`**: `skillvault run <workflow> <file> [--save output.md]`
  - Flujo interactivo: stdout prompt → stdin respuesta por step
  - Truncación de `{{previous_output}}` a 32K
- **MCP Tools** (2 nuevas):
  - `search_by_tags(tags, match=all|any)` — búsqueda por tags con intersección/unión
  - `get_context_bundle(project)` — bundle estructurado con entradas agrupadas por tipo
- **`entry_slug`** en `workflow_steps` para vincular steps a entries ejecutables
- **Graceful shutdown**: MCP server responde a SIGTERM/SIGINT, HTTP API drena conexiones
- **Partial indexes**: `WHERE status='active'` en entries, `WHERE active=1` en entry_links
- **`go 1.25.0`** — portabilidad con Go toolchain estándar

### Changed

- Version bump: `v2-quantum` → `v3` (main.go, import_export_store.go, mcp/server.go)
- `schema.sql` sync: `entry_tags` usa `tag TEXT NOT NULL` (no `tag_id`), +runs/run_steps DDL
- README actualizado: 19 CLI commands, 15 MCP tools, v3 status

### Fixed

- Conflictos de migración: `002_hermes.sql` → `003_hermes.sql` (version duplicada)
- FTS5 defensivo: verificado sin CGO
- HTTP API: `Close()` → `Shutdown(ctx)` con 5s deadline

---

## v2.0.0 — Release

**2026-06-20**

### Added

- **HTTP REST API** (13 endpoints) en `internal/api/` — `skillvault http` sirve en `127.0.0.1:7438`
  - `GET/POST /entries`, `GET/DELETE /entries/{id}`, `GET /entries?q=`
  - `POST /artifacts`, `POST /context`, `POST /projects`, `GET /projects`
  - `POST /sessions/wrap`, `POST /workflows`, `GET /workflows/{id}`
  - `POST /export`, `POST /import`
- **CI workflow** (GitHub Actions): test en ubuntu + macOS, cross-build a linux/darwin/windows
- **Documentación completa**:
  - `docs/quickstart.md` — instalación en 5 minutos
  - `docs/commands.md` — referencia CLI (14 comandos con flags)
  - `docs/mcp.md` — setup MCP para Claude/OpenCode (10 herramientas)
  - `docs/tutorial.md` — workflow real paso a paso
  - `docs/architecture.md` — Clean Architecture, flujos, decisiones de diseño

### Changed

- Branding: "Hermes" → "Qu@ntum" en README y naming interno
- `.gitignore` anclado a root (`/skillvault`) para no excluir `cmd/skillvault/` ni `openspec/specs/skillvault/`

### Fixed

- Test tokens partidos con concatenación para evitar GitHub push protection
- OpenSpec `spec.md` movido a `specs/spec.md` para cumplir con el formato del dispatcher

### Release

- Tag `v2.0.0` creado con binarios para linux amd64, darwin amd64/arm64, windows amd64
- SDD cycle completo: proposal → spec → design → tasks → apply → verify → archive

---

## Sprint 4 — `skillvault graph` CLI + Wikilinks flag

**2026-06-20**

### Added

- `skillvault graph --entry <id> [--depth 3] [--format mermaid|json|dot] [--direction both]`
  - **Mermaid**: `graph TD` con labels — renderizable en GitHub nativo
  - **JSON**: estructura `{root_entry, nodes[], edges[], node_count, edge_count}`
  - **DOT**: `digraph G { ... }` — compatible con Graphviz
- `--wikilinks` flag en `skillvault memory index --path <dir> --project <id> --wikilinks`
  - Parsea `[[target]]` en el body de .md y crea `entry_refs related_to`
- CLI tests para `graph`, `memory index/reindex/list-external`, `entry ref add`

### Files changed

- `internal/cli/commands.go` — ParseGraphFlags, ParseMemoryIndexFlags +wikilinks
- `internal/cli/cli_test.go` — tests de ParseCommand para nuevos subcommands
- `cmd/skillvault/main.go` — handler graph + memory --wikilinks

---

## Sprint 3 — MCP Contract Tests

**2026-06-20**

### Added

- 4 nuevas pruebas MCP:
  - `TestSaveEntryRefMCP` — crear ref entre dos entries via MCP, con label
  - `TestSaveEntryRefMissingArgsMCP` — validación de args requeridos
  - `TestListEntryRefsMCP` — listar refs con filtros source y relation_type
  - `TestGetEntryGraphMCP` — traversión A→B→C depth 3 + cycle detection

### Fixed

- `entry_links_store.go` — columnas CTE corregidas de source_id/target_id a from_entry_id/to_entry_id
- Edge query comparte el mismo CTE `WITH RECURSIVE reachable` con node query
- `setupMCPServices` ahora incluye `WithEntryRefService`

### Files changed

- `internal/mcp/mcp_test.go` — 4 nuevos tests + helpers extractEntryID/saveTestEntry
- `internal/db/entry_links_store.go` — CTE column fix + edge query CTE reuse

---

## Sprint 2 — pi-memory Integration (memory index + shadow entries)

**2026-06-20**

### Added

- **Frontmatter parser** (`internal/vars/frontmatter.go`): parsea bloques YAML `---...---` sin dependencias
  - keys: description, tags (inline y multiline), created, updated
  - `[[wikilinks]]` opcional (con/sin pipe)
  - `FirstHeading()` y `FirstParagraph()` para shadow entry content
- **MemoryIndexService** (`internal/app/memory_index.go`): indexación unidireccional pi-memory → SkillVault
  - `Index(memDir, projectID, parseWikilinks)`: camina directorio, parsea, upsert shadow entries
  - Shadow entries con `external_ref` = path relativo + tag fijo `pimem`
  - Orphan cleanup: entries cuyo .md fue borrado se marcan archived
  - Wikilinks → entry_refs `related_to`
- **CLI**: `skillvault memory index --path <dir> --project <id> [--wikilinks]`
  - `skillvault memory reindex` (alias)
  - `skillvault memory list-external --project <id>`
- **13 tests** para frontmatter parser (frontmatter básico, multiline, unclosed, wikilinks, etc.)

### Files created

- `internal/vars/frontmatter.go` — parser minimal YAML
- `internal/vars/frontmatter_test.go` — 13 tests
- `internal/app/memory_index.go` — MemoryIndexService

### Files changed

- `internal/cli/commands.go` — ParseMemoryIndexFlags, ParseCommand memory subcommand
- `cmd/skillvault/main.go` — vaultServices.memoryIndexSvc, CLI handlers

---

## Sprint 1 — entry_refs + handoff + Qu@ntum Migration

**2026-06-20**

### Added

#### entry_refs (grafo de relaciones)

- `entry_links` table con 11 tipos de relación:
  - `references`, `supersedes`, `related_to`, `part_of`, `derived_from`, `implements`
  - `uses`, `extends`, `handoff_of`, `generated_from`, `depends_on`
- Campos: `label TEXT`, `active INTEGER`, `created_at DATETIME`
- **Cycle detection**: `WITH RECURSIVE` CTE en `ReachableRefs` + validación en `SaveRef` para `depends_on`, `part_of`, `supersedes`
- **Traversión multi-dirección**: `GetEntryGraph` soporta `outgoing|incoming|both` con profundidad configurable (max 10)
- Store: `Save`, `GetLinks`, `GetLinksByType`, `ListRefs`, `RemoveRef`, `ReachableRefs`, `GetEntryGraph`

#### `type=handoff`

- Nuevo EntryType: `handoff`
- Validación en domain + CHECK constraint en SQLite
- Diseñado para no duplicar contenido — referencia artefactos via `entry_refs`

#### MCP Tools (3 nuevas)

- `save_entry_ref` — crear/actualizar arista entre dos entries
- `list_entry_refs` — listar aristas con filtros
- `get_entry_graph` — traversión desde un entry inicial

#### CLI

- `skillvault entry ref add <source> <target> <ref_type> [--label <txt>]`
- `skillvault entry ref list [--source] [--target] [--type]`
- `skillvault entry ref remove <source> <target> <ref_type>`

### Migration: Qu@ntum v2 Schema (`002_entry_refs_and_handoff.sql`)

Builds on 001_init (v1 schema) to create the full Qu@ntum schema:

| Table | Changes |
|---|---|
| `entries` | +title, +slug, +summary, +body_optional, +status, +artifact_id, +external_ref, 'handoff' type |
| `projects` | +slug, +status |
| `series` | +slug, +status |
| `entry_tags` | Recreada con FK correcta post-entries-swap |
| `tags` | Creada (nueva) |
| `workflows` | Creada (nueva) |
| `artifacts` | Creada (nueva) |
| `entry_links` | Creada (nueva) |
| `entries_fts` | Reconstruida con external_ref |

### Files created

- `internal/db/migrations/002_entry_refs_and_handoff.sql`
- `internal/app/entry_refs.go` — EntryRefService (SaveRef, ListRefs, RemoveRef, GetEntryGraph)
- `skillvault-spec-v1.patch.md` — spec actualizado con entry_refs, handoff, external_ref, memory index

### Files changed

- `internal/domain/entry.go` — EntryTypeHandoff, ExternalRef field
- `internal/domain/entry_link.go` — 5 nuevos RelationTypes, Label/Active/CreatedAt, CycleProne()
- `internal/domain/validation.go` — ValidateRelationType actualizado
- `internal/db/entry_links_store.go` — ListRefs, RemoveRef, ReachableRefs (CTE), GetEntryGraph
- `internal/db/store.go` — EntryLinkFilter, EntryLinkNode, interfaces expandidas
- `internal/db/entries_store.go` — external_ref en Save/Get/Search/List/syncFTS
- `internal/db/import_export_store.go` — external_ref + entry_links label en export/import
- `internal/db/schema.sql` — handoff type, external_ref, entry_links actualizado
- `internal/mcp/tools.go` — save_entry_ref, list_entry_refs, get_entry_graph
- `internal/mcp/mcp_test.go` — tool count 13, nuevos tool names
- `internal/cli/commands.go` — ParseCommand entry/ref routing
- `cmd/skillvault/main.go` — vaultServices.entryRefSvc, entry-ref CLI handler, MCP wiring
