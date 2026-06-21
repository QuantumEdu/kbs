# Changelog

All notable changes to SkillVault are documented here.

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

## Sprint 1 — entry_refs + handoff + Hermes Migration

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

### Migration: Hermes v2 Schema (`002_entry_refs_and_handoff.sql`)

Builds on 001_init (v1 schema) to create the full Hermes schema:

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
