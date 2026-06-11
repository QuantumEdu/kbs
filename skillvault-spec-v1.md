# SkillVault — Spec v1

> Knowledge Operating System personal.  
> Go binary · SQLite + FTS5 · MCP stdio · CLI · HTTP API opcional  
> Single binary · Local-first · Zero framework dependencies · SQLite driver only

**Autor:** Dr. Gabriel Magallón Sánchez / Quantum-6  
**Estado:** Spec v1 confirmado  
**Cortes:** `v1-alpha` y `v1-final`  
**Fecha:** 2026-06-10

---

## 0. Implementation Contract

Este documento reemplaza el contexto conversacional previo.

Reglas de interpretación:

1. Si algo no está en este spec, no se asume.
2. Si hay conflicto entre roadmap y scope `v1-alpha` / `v1-final`, manda el scope.
3. No implementar features de roadmap dentro de v1.
4. No agregar dependencias salvo `modernc.org/sqlite`.
5. No usar Fiber, Cobra, ORM, frameworks HTTP, frameworks CLI o template engines externos.
6. SkillVault v1 no ejecuta agentes, no llama LLMs, no mantiene memoria de sesión y no corre loops autónomos.
7. SkillVault v1 es una biblioteca recuperable con ejecución mínima controlada.
8. La ejecución permitida en v1 significa únicamente renderizar variables y devolver workflows/series preparados.
9. SQLite es la fuente de verdad operativa.
10. GitHub/export es respaldo manual, no sync vivo.

---

## 1. Visión

SkillVault es una capa local de **ejecución y recuperación de conocimiento estructurado**.

Almacena y sirve:

- skills
- prompts
- definiciones de agentes
- workflows multi-paso
- series de prompts/skills/contextos
- contextos de proyectos
- notas técnicas

Es consumido por:

- Claude Code
- OpenCode
- clientes MCP compatibles
- CLI local
- HTTP API local opcional en `v1-final`

SkillVault **no es** memoria conversacional de sesión.  
SkillVault **no es** un sistema de sync.  
SkillVault **no es** un runtime de agentes.  
SkillVault **no es** un gestor documental con archivos adjuntos.

SkillVault es una biblioteca local, portable y estructurada de conocimiento reutilizable.

---

## 2. Principios de diseño

### 2.1 Local-first

La base de datos vive localmente en:

```bash
~/.skillvault/vault.db
```

El binario vive recomendado en:

```bash
~/tools/skillvault
```

### 2.2 SQLite como fuente de verdad operativa

SQLite es la fuente real para uso diario.

GitHub, JSON export y archivos Markdown son respaldo o artefactos de documentación, no fuente viva bidireccional en v1.

### 2.3 Biblioteca recuperable, no runtime

SkillVault v1:

- guarda
- busca
- recupera
- lista
- organiza
- compone series
- renderiza variables
- devuelve workflows como pasos preparados

SkillVault v1 no:

- llama modelos LLM
- evalúa outputs
- hace branching automático
- mantiene estado conversacional
- ejecuta loops agentic
- sincroniza cloud
- resuelve merges

### 2.4 Semántica explícita

El sistema distingue:

```text
entry    = pieza mínima reutilizable
series   = composición flexible de piezas
workflow = receta autocontenida renderizable
project  = contenedor lógico de contexto
```

### 2.5 Mínimas dependencias

SkillVault v1 usa:

- Go standard library
- `modernc.org/sqlite`

No usa:

- Fiber
- Cobra
- Gin
- Echo
- ORM
- template engines externos
- SDK MCP externo salvo bloqueo real de compatibilidad

### 2.6 TDD pragmático

Toda regla de dominio importante debe tener prueba.

No se exige cobertura perfecta, pero sí pruebas para:

- invariantes de series
- archivado
- búsqueda
- variables
- import/export
- MCP core
- migraciones

---

## 3. Scope v1-alpha

`v1-alpha` debe ser pequeño pero usable por agentes.

Objetivo: validar que SkillVault funciona como biblioteca estructurada recuperable desde CLI y MCP.

### 3.1 Incluye

#### DB

- `schema_migrations`
- `projects`
- `entries`
- `entry_tags`
- `series`
- `series_entries`
- `workflow_steps`
- `entries_fts`

#### Store Go

- init DB
- migraciones embebidas
- upsert/get/list/search entries
- tags
- archive_entry
- upsert/get/list projects
- upsert/get/list series
- replace_series_entries
- workflow_steps
- run_workflow como renderizado
- get_context básico
- variable detection
- variable injection
- import/export JSON básico

#### CLI

- `skillvault init`
- `skillvault get`
- `skillvault search`
- `skillvault list`
- `skillvault entry upsert`
- `skillvault entry archive`
- `skillvault project upsert`
- `skillvault project list`
- `skillvault series get`
- `skillvault series list`
- `skillvault series upsert`
- `skillvault series replace`
- `skillvault workflow run`
- `skillvault export`
- `skillvault import`
- `skillvault mcp`
- `skillvault version`

#### MCP core

11 tools:

1. `get_entry`
2. `search_entries`
3. `list_entries`
4. `upsert_entry`
5. `archive_entry`
6. `get_series`
7. `list_series`
8. `upsert_series`
9. `replace_series_entries`
10. `get_context`
11. `run_workflow`

#### Import/export

- JSON full vault
- archivo único
- sin filtros avanzados
- con `schema_version`
- con `app_version`
- con `exported_at`

### 3.2 Fuera de v1-alpha

- HTTP API
- `project_refs`
- `copy_entry`
- `copy_series`
- `archive_series`
- `archive_project`
- `add_entry_to_series`
- `remove_entry_from_series`
- stats avanzadas
- setup automático de Claude Code/OpenCode
- Git sync
- cloud
- Obsidian export
- Raycast
- TUI

---

## 4. Scope v1-final

`v1-final` completa la administración local del vault, sin entrar en integraciones de roadmap.

### 4.1 Agrega

- HTTP API con `net/http`
- `project_refs`
- `archive_series`
- `archive_project`
- `add_entry_to_series`
- `remove_entry_from_series`
- `copy_entry`
- `copy_series`
- `stats`
- `setup claude-code`
- `setup opencode`
- CLI ampliada
- MCP ampliado
- validaciones completas de alcance
- tests de edge cases
- README quickstart

### 4.2 No incluye

- sync
- cloud/VPS
- Obsidian export
- Raycast extension
- TUI
- ejecución autónoma de agentes
- hard delete
- merge/diff inteligente
- import/export MCP
- autenticación HTTP remota

---

## 5. Fuera de scope v1

Explícitamente fuera de v1:

- Git sync vivo
- sync bidireccional Markdown/SQLite
- sync cloud
- Obsidian export
- Raycast extension
- Bubble Tea TUI
- hard delete
- purge
- archivos adjuntos
- PDFs/imágenes/templates multiarchivo
- workflows anidados
- series dentro de workflows
- workflows dentro de workflows
- ejecución de LLM
- loops agentic
- memoria conversacional
- permisos multiusuario
- auth remota
- API pública en red

---

## 6. Stack técnico

| Componente | Decisión v1 | Razón |
|---|---|---|
| Lenguaje | Go | single binary, portable, estándar |
| DB | SQLite + FTS5 | archivo único, búsqueda full-text local |
| SQLite driver | `modernc.org/sqlite` | evita CGO, portable |
| CLI | Go stdlib | sin Cobra, cero framework CLI |
| HTTP | `net/http` | sin Fiber, suficiente para local |
| MCP | stdio + JSON-RPC propio mínimo | control, sin dependencia inicial |
| Migraciones | SQL embebido con `embed` | single binary real |
| Fuente de verdad | SQLite | operativa local |
| Export | JSON | respaldo manual |
| GitHub | respaldo manual | no sync vivo |

---

## 7. Arquitectura limpia ligera

SkillVault usará una arquitectura limpia básica, no sobreingenierizada.

### 7.1 Capas

```text
cmd/skillvault
    ↓
internal/cli
internal/mcp
internal/api
    ↓
internal/app
    ↓
internal/domain
    ↓
internal/db
```

### 7.2 Responsabilidades

#### `cmd/skillvault`

Entry point.

Responsabilidades:

- leer `os.Args`
- inicializar comandos principales
- despachar CLI/MCP/serve
- no contener lógica de dominio

#### `internal/cli`

Parsing manual con `os.Args` y `flag`.

Responsabilidades:

- traducir argumentos CLI a comandos de aplicación
- formatear salida humana
- manejar exit codes

No debe:

- acceder directamente a SQLite
- duplicar reglas de dominio

#### `internal/mcp`

Servidor MCP stdio mínimo.

Responsabilidades:

- leer/escribir JSON-RPC por stdio
- declarar tools
- validar input superficial
- llamar casos de uso en `internal/app`

No debe:

- implementar reglas de negocio
- tocar SQLite directo
- ejecutar LLMs

#### `internal/api`

HTTP API `net/http` en v1-final.

Responsabilidades:

- rutas
- JSON request/response
- status codes
- llamar `internal/app`

No debe:

- contener SQL
- duplicar validaciones complejas

#### `internal/app`

Capa de casos de uso.

Responsabilidades:

- orquestar operaciones
- aplicar reglas de dominio
- controlar transacciones a través del store
- preparar DTOs

Ejemplos:

- `UpsertEntry`
- `SearchEntries`
- `GetSeries`
- `ReplaceSeriesEntries`
- `RunWorkflow`
- `ExportVault`

#### `internal/domain`

Modelos y reglas puras.

Responsabilidades:

- tipos
- constantes
- validaciones puras
- reglas de alcance

Ejemplos:

- `Entry`
- `Series`
- `Project`
- `WorkflowStep`
- `ValidateSeriesMembership`
- `NormalizeTag`

#### `internal/db`

Persistencia SQLite.

Responsabilidades:

- conexión
- migraciones
- queries
- transacciones
- FTS5
- implementación de repositorios/store

No debe:

- formatear salida CLI
- conocer MCP
- conocer HTTP

---

## 8. SOLID aplicado de forma pragmática

### 8.1 Single Responsibility Principle

Cada paquete tiene una razón clara de cambio:

- CLI cambia por ergonomía de terminal.
- MCP cambia por protocolo.
- API cambia por rutas HTTP.
- App cambia por casos de uso.
- Domain cambia por reglas.
- DB cambia por persistencia.

### 8.2 Open/Closed Principle

Agregar una nueva interfaz de consumo, como Raycast en futuro, no debe cambiar el dominio.

Patrón:

```text
nuevo adapter → llama app → usa domain/store
```

### 8.3 Liskov Substitution Principle

No aplica de forma pesada; evitar jerarquías innecesarias.

### 8.4 Interface Segregation Principle

Interfaces pequeñas.

Ejemplo:

```go
type EntryStore interface {
    UpsertEntry(ctx context.Context, e Entry, tags []string, steps []WorkflowStep) error
    GetEntry(ctx context.Context, id string, includeArchived bool) (EntryResult, error)
    SearchEntries(ctx context.Context, q SearchQuery) ([]EntrySearchResult, error)
}
```

No crear una interfaz gigante tipo `VaultStore` con todo si no es necesario.

### 8.5 Dependency Inversion Principle

`internal/app` depende de interfaces pequeñas, no de detalles de SQLite cuando sea útil.

En alpha se permite pragmatismo: una implementación SQLite directa está bien, siempre que no contamine CLI/MCP/API.

---

## 9. Patrones de diseño mínimos indispensables

No usar patrones por decoración.

### 9.1 Repository / Store

Usado para persistencia.

```text
app → store interface → sqlite store
```

Aplicar en:

- entries
- projects
- series
- workflows
- import/export

### 9.2 Service / Use Case

Casos de uso en `internal/app`.

Ejemplos:

- `EntryService`
- `SeriesService`
- `WorkflowService`
- `VaultExportService`

### 9.3 Adapter

CLI, MCP y HTTP son adapters.

```text
CLI adapter
MCP adapter
HTTP adapter
```

Todos llaman la misma capa `app`.

### 9.4 Transaction Script

Operaciones como `replace_series_entries` se implementan como scripts transaccionales claros.

Ejemplo:

```text
begin tx
validate series
validate entries
delete old series_entries
insert new series_entries renumbered
commit
```

### 9.5 Strategy mínima para render

Si el resolver crece, puede separarse como estrategia:

- `PlainVarResolver`
- futuro `MetadataVarResolver`

En v1 basta una implementación.

---

## 10. Modelo de dominio

### 10.1 Entry

Unidad mínima de conocimiento textual.

Tipos válidos:

| Tipo | Descripción |
|---|---|
| `skill` | Prompt estructurado reutilizable |
| `agent` | Definición de agente |
| `workflow` | Flujo autocontenido con steps |
| `prompt` | Prompt suelto |
| `context` | Contexto de proyecto |
| `note` | Nota técnica o decisión |

Reglas:

- `entries.id` es único globalmente.
- Entry vive completo en SQLite.
- No hay artifact files en v1.
- `project_id NULL` significa global.
- Entry de proyecto usa prefijo estable en ID.
- Workflows son entries con `type = workflow`.

### 10.2 Project

Contenedor lógico de contexto.

Reglas:

- `projects.id` es slug estable.
- `projects.name` es display humano.
- `projects.active = 0` archiva el proyecto.
- Archivar proyecto no archiva entries ni series.
- Contenido de proyectos archivados no aparece por default.

### 10.3 Series

Secuencia conceptual/práctica de entries.

Reglas:

- `series.id` es único globalmente.
- Puede contener entries heterogéneos.
- Puede contener prompts, skills, contexts, notes, agents.
- No debe contener workflows en v1.
- Una serie global solo puede contener entries globales.
- Una serie de proyecto puede contener entries globales o del mismo proyecto.
- Una serie de proyecto no puede contener entries de otro proyecto.

### 10.4 Workflow

Receta autocontenida renderizable.

Reglas:

- Workflow vive como `entry type = workflow`.
- Pasos viven en `workflow_steps`.
- Pasos guardan contenido directo.
- No referencia entries externos.
- No contiene series.
- No contiene otros workflows.
- Puede tener roles `system`, `user`, `assistant`.
- No ejecuta LLM.

### 10.5 Project refs

Solo v1-final.

Vinculan proyecto con entries/series globales.

Reglas:

- No duplican contenido.
- Solo apuntan a contenido global.
- No apuntan a otros proyectos.
- No son sync.
- No son herencia.

---

## 11. Schema SQL

### 11.1 `schema_migrations`

```sql
CREATE TABLE IF NOT EXISTS schema_migrations (
  version     INTEGER PRIMARY KEY,
  name        TEXT NOT NULL,
  applied_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

### 11.2 `projects`

```sql
CREATE TABLE projects (
  id          TEXT PRIMARY KEY,
  name        TEXT NOT NULL,
  description TEXT,
  active      INTEGER DEFAULT 1,
  created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

### 11.3 `entries`

```sql
CREATE TABLE entries (
  id              TEXT PRIMARY KEY,
  name            TEXT NOT NULL,
  type            TEXT NOT NULL CHECK(type IN ('skill','agent','workflow','prompt','context','note')),
  category        TEXT,
  project_id      TEXT REFERENCES projects(id),
  description     TEXT,
  content         TEXT NOT NULL,
  vars            TEXT,
  source_entry_id TEXT REFERENCES entries(id),
  active          INTEGER DEFAULT 1,
  created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
  last_used       DATETIME
);
```

### 11.4 `entry_tags`

```sql
CREATE TABLE entry_tags (
  entry_id TEXT NOT NULL REFERENCES entries(id) ON DELETE CASCADE,
  tag      TEXT NOT NULL,
  PRIMARY KEY (entry_id, tag)
);
```

### 11.5 `series`

```sql
CREATE TABLE series (
  id          TEXT PRIMARY KEY,
  name        TEXT NOT NULL,
  project_id  TEXT REFERENCES projects(id),
  description TEXT,
  vars        TEXT,
  active      INTEGER DEFAULT 1,
  created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

### 11.6 `series_entries`

```sql
CREATE TABLE series_entries (
  series_id TEXT NOT NULL REFERENCES series(id) ON DELETE CASCADE,
  entry_id  TEXT NOT NULL REFERENCES entries(id) ON DELETE CASCADE,
  step_num  INTEGER NOT NULL,
  label     TEXT,
  required  INTEGER DEFAULT 1,
  notes     TEXT,
  active    INTEGER DEFAULT 1,
  PRIMARY KEY (series_id, entry_id),
  UNIQUE(series_id, step_num)
);
```

### 11.7 `workflow_steps`

```sql
CREATE TABLE workflow_steps (
  id        INTEGER PRIMARY KEY AUTOINCREMENT,
  entry_id  TEXT NOT NULL REFERENCES entries(id) ON DELETE CASCADE,
  step_num  INTEGER NOT NULL,
  role      TEXT NOT NULL CHECK(role IN ('system','user','assistant')),
  content   TEXT NOT NULL,
  label     TEXT,
  UNIQUE(entry_id, step_num)
);
```

### 11.8 `project_refs` — v1-final

```sql
CREATE TABLE project_refs (
  project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  ref_type   TEXT NOT NULL CHECK(ref_type IN ('entry','series')),
  ref_id     TEXT NOT NULL,
  label      TEXT,
  notes      TEXT,
  active     INTEGER DEFAULT 1,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (project_id, ref_type, ref_id)
);
```

### 11.9 FTS5

```sql
CREATE VIRTUAL TABLE entries_fts USING fts5(
  id,
  name,
  description,
  content,
  tags_denorm,
  content='entries',
  tokenize='porter unicode61'
);
```

### 11.10 Índices

```sql
CREATE INDEX idx_entries_type ON entries(type);
CREATE INDEX idx_entries_project_id ON entries(project_id);
CREATE INDEX idx_entries_active ON entries(active);
CREATE INDEX idx_series_project_id ON series(project_id);
CREATE INDEX idx_series_active ON series(active);
CREATE INDEX idx_series_entries_series_step ON series_entries(series_id, step_num);
CREATE INDEX idx_entry_tags_tag ON entry_tags(tag);
CREATE INDEX idx_workflow_steps_entry_step ON workflow_steps(entry_id, step_num);
```

En `v1-final`:

```sql
CREATE INDEX idx_project_refs_project_id ON project_refs(project_id);
CREATE INDEX idx_project_refs_ref ON project_refs(ref_type, ref_id);
```

---

## 12. Migraciones

### 12.1 Archivos

```text
internal/db/
├── migrations/
│   └── 001_init.sql
├── schema.sql
├── migrate.go
└── store.go
```

### 12.2 Reglas

- `001_init.sql` es la fuente ejecutable.
- `schema.sql` es referencia consolidada.
- Ambos existen desde v1-alpha.
- Migraciones se embeben con `embed`.
- `skillvault init` aplica migraciones pendientes.
- `schema_migrations` registra versiones aplicadas.
- `init` debe ser idempotente.
- En v1-alpha, `schema.sql` se mantiene manualmente.
- Cualquier cambio de migración debe actualizar `schema.sql` en el mismo commit.

### 12.3 Go embed

```go
import "embed"

//go:embed migrations/*.sql
var migrationsFS embed.FS
```

---

## 13. Variable injection

### 13.1 Motor mínimo

Solo resuelve:

```text
{{key}} → value
```

No es template engine completo.

### 13.2 Reglas

- `{{key}}` se reemplaza por valor en `vars`.
- Si falta una variable, se deja visible.
- No falla silenciosamente.
- Devuelve `missing_vars`.
- Case-sensitive.
- `{{Project_Name}}` y `{{project_name}}` son distintas.
- Opera sobre:
  - `entries.content`
  - `workflow_steps.content`
  - entries dentro de series
- No ejecuta código.
- No evalúa expresiones.
- No hace condicionales.

### 13.3 Vars globales

Disponibles sin pasarlas explícitamente:

```text
{{date}}    → fecha actual ISO
{{project}} → project_id del entry/series si existe
```

### 13.4 Declaración y detección

SkillVault hace ambas:

1. Detección automática de `{{var}}`
2. Declaración manual opcional en `vars`

En v1, `vars` es JSON array simple:

```json
["project_name", "stack", "domain"]
```

El resolver debe diseñarse para poder evolucionar a metadata en v2.

---

## 14. Búsqueda FTS5

### 14.1 Alcance

FTS5 indexa principalmente `entries`.

Campos:

- `entries.id`
- `entries.name`
- `entries.description`
- `entries.content`
- `tags_denorm`

### 14.2 Filtros

`search_entries` soporta:

- `query`
- `project_id`
- `series_id`
- `type`
- `tags`
- `active`
- `include_archived`
- `limit`

### 14.3 Metadata de series

`search_entries` devuelve metadata ligera de series donde aparece cada entry.

Máximo 3 refs por resultado.

```json
{
  "series_id": "sdd-cycle",
  "series_name": "SDD Cycle",
  "step_num": 3,
  "total_steps": 6,
  "label": "Generate PRD"
}
```

No devuelve contenido completo de series en búsqueda.

### 14.4 Archivados

Default:

- no mostrar entries archivados
- no mostrar entries activos de proyectos archivados
- no mostrar series archivadas
- no mostrar series activas de proyectos archivados

Con:

```json
{ "include_archived": true }
```

sí se incluyen.

---

## 15. Archivado

### 15.1 v1-alpha

Incluye:

- `archive_entry`

Reglas:

- `active = 0`
- no borra físicamente
- no aparece por default
- requiere `include_archived = true` para recuperarse

Si se pide por ID sin `include_archived`:

```json
{
  "error": "archived",
  "message": "Entry exists but is archived. Retry with include_archived=true.",
  "id": "old-prd-fastapi",
  "type": "entry"
}
```

### 15.2 v1-final

Agrega:

- `archive_series`
- `archive_project`

Reglas:

- archivar serie no archiva entries
- archivar proyecto no archiva entries ni series
- archivar proyecto solo cambia visibilidad normal
- no existe hard delete en v1

### 15.3 No hard delete

Fuera de v1:

- `delete_entry`
- `delete_series`
- `delete_project`
- `purge`
- `hard_delete`
- cleanup destructivo

---

## 16. Series

### 16.1 Propósito

Una serie representa un camino mental o práctico compuesto por entries reutilizables.

Ejemplo:

```text
sdd-cycle
1. constitution prompt
2. grill-me skill
3. spec generator
4. plan generator
5. tasks generator
```

### 16.2 Series heterogéneas

Una serie puede contener:

- prompt
- skill
- context
- note
- agent

No debe contener workflows en v1.

### 16.3 Orden

El orden vive en `series_entries`, no en `entries`.

Razón:

- un entry puede pertenecer a varias series
- evita duplicación
- permite reutilización

### 16.4 `total_steps`

No se guarda.

Se calcula dinámicamente:

```text
total_steps = COUNT(active steps in series)
```

### 16.5 Numeración

`step_num`:

- inicia en 1
- es secuencial
- no tiene huecos
- se renumera desde store Go
- DB solo garantiza unicidad

### 16.6 Integridad de orden

Responsable: store de Go.

La DB garantiza:

```sql
UNIQUE(series_id, step_num)
```

El store garantiza:

- sin huecos
- transacción
- renumeración

---

## 17. Workflows

### 17.1 Propósito

Workflow es una receta autocontenida renderizable.

Diferencia:

```text
series   = composición flexible
workflow = receta cerrada
```

### 17.2 Reglas

- `entry.type = workflow`
- pasos en `workflow_steps`
- steps guardan contenido directo
- no referencias externas
- no series
- no workflow anidado
- no LLM calls

### 17.3 Roles

`workflow_steps.role` permite:

- `system`
- `user`
- `assistant`

SkillVault preserva roles, no los ejecuta.

### 17.4 Label

`label` es opcional en DB.

CLI/import/export deben recomendarlo.

Si falta:

```text
Step N
```

### 17.5 Numeración

`step_num`:

- inicia en 1
- secuencial
- sin huecos
- store Go renumera
- DB garantiza `UNIQUE(entry_id, step_num)`

---

## 18. Project refs — v1-final

### 18.1 Propósito

Vincular un proyecto con conocimiento global recomendado.

Ejemplo:

```text
project: vitacare
refs:
- sdd-cycle
- prd-fastapi
- clean-architecture-lite
```

### 18.2 Reglas

- solo contenido global
- no contenido de otros proyectos
- no duplica
- no sincroniza
- no hereda cambios de manera especial
- solo referencia ligera

### 18.3 get_context

Default:

- devuelve refs ligeras

Opcional:

- `include_refs=true` puede devolver contenido completo de esas refs en v1-final

---

## 19. Copy

### 19.1 `copy_entry` — v1-final

Copia un entry global hacia un proyecto.

```bash
skillvault copy prd-fastapi --project vitacare --id vitacare-prd-fastapi
```

Reglas:

- solo copia entries globales
- nuevo entry recibe `project_id`
- no mantiene sync
- no hace merge
- puede guardar `source_entry_id` como trazabilidad

### 19.2 `copy_series` — v1-final

Copia una serie global hacia un proyecto.

Modos:

#### `link-entries`

- crea nueva serie de proyecto
- apunta a entries globales existentes
- no duplica entries

#### `copy-entries`

- crea nueva serie de proyecto
- copia cada entry global como entry del proyecto
- nueva serie apunta a copias

Default:

```text
link-entries
```

---

## 20. MCP tools

### 20.1 v1-alpha — 11 tools

#### Entries

1. `get_entry`
2. `search_entries`
3. `list_entries`
4. `upsert_entry`
5. `archive_entry`

#### Series

6. `get_series`
7. `list_series`
8. `upsert_series`
9. `replace_series_entries`

#### Context

10. `get_context`

#### Workflow

11. `run_workflow`

### 20.2 v1-final — 22 tools

Incluye todo v1-alpha y agrega:

#### Series management

12. `add_entry_to_series`
13. `remove_entry_from_series`
14. `archive_series`

#### Projects

15. `upsert_project`
16. `list_projects`
17. `archive_project`

#### Project refs

18. `add_project_ref`
19. `remove_project_ref`

#### Copy

20. `copy_entry`
21. `copy_series`

#### Utility

22. `stats`

### 20.3 No MCP en v1

No serán MCP tools:

- `import_vault`
- `export_vault`
- `serve_http`
- `setup_claude_code`
- `setup_opencode`

Esas operaciones quedan en CLI.

### 20.4 MCP implementation

- stdio
- JSON-RPC
- implementación propia mínima
- sin SDK externo inicialmente
- SDK externo permitido solo si compatibilidad bloquea Claude Code/OpenCode

---

## 21. CLI

CLI implementada con biblioteca estándar.

No Cobra.

### 21.1 v1-alpha

```bash
skillvault init

skillvault get <entry_id> [--include-archived]
skillvault search <query> [--project <id>] [--series <id>] [--type <type>] [--tag <tag>] [--include-archived]
skillvault list [--project <id>] [--type <type>] [--include-archived]

skillvault entry upsert <file.json>
skillvault entry archive <entry_id>

skillvault project upsert <file.json>
skillvault project list [--include-archived]

skillvault series get <series_id> [--vars <file.json>] [--include-archived]
skillvault series list [--project <id>] [--include-archived]
skillvault series upsert <file.json>
skillvault series replace <series_id> <file.json>

skillvault workflow run <workflow_id> [--vars <file.json>]

skillvault export <file.json>
skillvault import <file.json>

skillvault mcp
skillvault version
```

### 21.2 v1-final

```bash
skillvault serve [--host 127.0.0.1] [--port 7438]

skillvault series add <series_id> <entry_id> [--at <n>]
skillvault series remove <series_id> <entry_id>
skillvault series archive <series_id>
skillvault series copy <series_id> --project <project_id> --id <new_id> --mode link-entries|copy-entries

skillvault project archive <project_id>
skillvault project ref add <project_id> --type entry|series --id <ref_id>
skillvault project ref remove <project_id> --type entry|series --id <ref_id>

skillvault copy <entry_id> --project <project_id> --id <new_id>

skillvault stats
skillvault setup claude-code
skillvault setup opencode
```

---

## 22. HTTP API — v1-final

HTTP entra en `v1-final`, no en alpha.

### 22.1 Reglas

- `net/http`
- no corre por default
- se activa con `skillvault serve`
- host default: `127.0.0.1`
- puerto default: `7438`
- sin auth en localhost para v1
- exposición remota fuera de scope
- no usar `DELETE` para archivar
- usar `/archive`

### 22.2 Rutas

```http
GET  /health
GET  /version

GET  /entries
GET  /entries/search?q=...
GET  /entries/{id}
POST /entries
PUT  /entries/{id}
POST /entries/{id}/archive
POST /entries/{id}/copy

GET  /projects
GET  /projects/{id}
POST /projects
PUT  /projects/{id}
POST /projects/{id}/archive

GET  /series
GET  /series/{id}
POST /series
PUT  /series/{id}
POST /series/{id}/archive
PUT  /series/{id}/entries
POST /series/{id}/entries
POST /series/{id}/copy

GET  /context/{project_id}

POST /workflows/{id}/run

GET  /stats
```

Fuera de v1:

```http
DELETE /entries/{id}
DELETE /series/{id}
DELETE /projects/{id}
```

---

## 23. Import/export JSON

### 23.1 v1-alpha

CLI only.

```bash
skillvault export vault.json
skillvault import vault.json
```

No MCP.

### 23.2 Export

Exporta:

- projects
- entries
- entry_tags
- series
- series_entries
- workflow_steps

Incluye activos y archivados.

Formato:

```json
{
  "schema_version": 1,
  "app_version": "v1-alpha",
  "exported_at": "2026-06-10T18:30:00Z",
  "source": "skillvault",
  "data": {
    "projects": [],
    "entries": [],
    "entry_tags": [],
    "series": [],
    "series_entries": [],
    "workflow_steps": []
  }
}
```

En v1-final agrega:

- project_refs

### 23.3 Import

Reglas:

- corre en transacción
- hace upsert
- no borra ausentes
- valida IDs y referencias
- falla completo si hay inconsistencia estructural
- rechaza exports sin `schema_version`
- rechaza `schema_version` mayor al soportado

Fuera de alpha:

- export por proyecto
- export por serie
- export por entry
- Markdown export
- Git sync
- diff
- merge inteligente

---

## 24. Estructura Go

```text
skillvault/
├── cmd/
│   └── skillvault/
│       └── main.go
├── internal/
│   ├── cli/
│   │   ├── commands.go
│   │   └── output.go
│   ├── mcp/
│   │   ├── server.go
│   │   ├── jsonrpc.go
│   │   └── tools.go
│   ├── api/
│   │   ├── server.go
│   │   └── handlers.go
│   ├── app/
│   │   ├── entries.go
│   │   ├── projects.go
│   │   ├── series.go
│   │   ├── workflows.go
│   │   ├── context.go
│   │   ├── import_export.go
│   │   └── stats.go
│   ├── domain/
│   │   ├── entry.go
│   │   ├── project.go
│   │   ├── series.go
│   │   ├── workflow.go
│   │   ├── tags.go
│   │   └── validation.go
│   ├── db/
│   │   ├── migrations/
│   │   │   └── 001_init.sql
│   │   ├── schema.sql
│   │   ├── migrate.go
│   │   ├── store.go
│   │   ├── entries_store.go
│   │   ├── projects_store.go
│   │   ├── series_store.go
│   │   ├── workflow_store.go
│   │   └── fts.go
│   └── vars/
│       ├── detect.go
│       └── resolver.go
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

---

## 25. Reglas de dominio críticas

### 25.1 IDs

- `entries.id` único global
- `series.id` único global
- `projects.id` único global
- IDs humanos, estables, kebab-case recomendado

### 25.2 Tags

No tabla `tags` en v1.

Normalización:

- trim
- lowercase
- espacios → guiones
- rechazar vacíos
- evitar duplicados

### 25.3 Alcance de series

Permitido:

```text
serie global → entries globales
serie proyecto → entries globales
serie proyecto → entries del mismo proyecto
```

No permitido:

```text
serie global → entries de proyecto
serie proyecto → entries de otro proyecto
```

### 25.4 Project refs

Permitido:

```text
project → entry global
project → series global
```

No permitido:

```text
project → entry de otro proyecto
project → series de otro proyecto
```

### 25.5 Workflows

Siempre autocontenidos.

No composición externa.

### 25.6 Archiving

Default oculta.

`include_archived=true` abre archivo completo.

### 25.7 Get directo de archivados

Sin `include_archived`, no devuelve contenido.

Debe responder error `archived` y sugerir `include_archived=true`.

---

## 26. TDD

### 26.1 Filosofía

TDD en SkillVault será práctico, no dogmático.

Regla:

1. Cada regla de dominio importante debe tener test.
2. Cada bug corregido debe agregar test.
3. Cada store transaccional debe tener test.
4. Cada MCP core tool debe tener al menos test de contrato.
5. No perseguir cobertura cosmética.

### 26.2 Red-Green-Refactor básico

Para casos de dominio:

1. Escribir test fallido.
2. Implementar mínimo.
3. Refactorizar sin romper.
4. Confirmar invariantes.

### 26.3 Pirámide de tests

Prioridad:

1. Unit tests de dominio.
2. Integration tests de store SQLite.
3. Tests de app/use cases.
4. Contract tests MCP.
5. HTTP tests en v1-final.

### 26.4 No testear en exceso

No crear mocks complejos si SQLite in-memory resuelve mejor.

Preferido:

```text
SQLite temp DB + migraciones reales
```

---

## 27. Tests obligatorios

### 27.1 v1-alpha

#### DB / migrations

- `skillvault init` crea DB y aplica `001_init.sql`.
- `schema_migrations` registra migración.
- `init` repetido es idempotente.

#### Entries

- `upsert_entry` crea entry con tags.
- `upsert_entry` actualiza entry sin duplicar tags.
- `archive_entry` marca `active = 0`.
- `get_entry` no devuelve archivados sin `include_archived`.
- `get_entry` archivado responde `archived`.

#### FTS5

- search encuentra por name.
- search encuentra por description.
- search encuentra por content.
- search encuentra por tags.
- search filtra por `project_id`.
- search filtra por `series_id`.
- search filtra por `type`.
- search filtra por tags.
- archivados no aparecen por default.

#### Vars

- detecta `{{vars}}`.
- respeta vars declaradas.
- render reemplaza valores.
- render deja faltantes visibles.
- devuelve `missing_vars`.

#### Series

- `upsert_series` crea/actualiza metadata.
- `replace_series_entries` renumera desde 1.
- `series_entries` no permite huecos.
- serie global solo acepta entries globales.
- serie de proyecto acepta entries globales.
- serie de proyecto acepta entries del mismo proyecto.
- serie de proyecto rechaza entries de otro proyecto.
- `get_series` devuelve `step_num/total` calculado.

#### Workflows

- workflow es autocontenido.
- `run_workflow` renderiza steps.
- roles `system/user/assistant` se preservan.
- no llama LLM.
- no ejecuta pasos.
- mantiene orden secuencial.

#### Import/export

- export genera `schema_version`.
- export genera `app_version`.
- export genera `exported_at`.
- export genera `source`.
- import corre en transacción.
- import hace upsert.
- import no borra ausentes.
- import falla completo con referencias inválidas.

#### MCP

- expone las 11 tools alpha.
- `get_entry` funciona por MCP.
- `search_entries` funciona por MCP.
- `get_series` funciona por MCP.
- `run_workflow` funciona por MCP.
- `upsert_entry` funciona por MCP.
- `replace_series_entries` funciona por MCP.

### 27.2 v1-final

- `archive_series` no archiva entries.
- `archive_project` no archiva entries ni series.
- contenido de proyecto archivado no aparece por default.
- `project_refs` solo apunta a contenido global.
- `copy_entry` copia global hacia proyecto sin sync.
- `copy_series link-entries` no duplica entries.
- `copy_series copy-entries` sí duplica entries.
- HTTP endpoints responden correctamente.
- stats cuenta por tipo/proyecto/estado.
- setup Claude Code/OpenCode escribe config esperada.

---

## 28. Criterios de Done

### 28.1 Done v1-alpha

- [ ] Compila como binario Go.
- [ ] Usa `modernc.org/sqlite`.
- [ ] No usa Fiber.
- [ ] No usa Cobra.
- [ ] No usa ORM.
- [ ] `skillvault init` crea/aplica DB.
- [ ] Migraciones embebidas con `embed`.
- [ ] `001_init.sql` existe.
- [ ] `schema.sql` consolidado existe.
- [ ] Entries CRUD mínimo funcional.
- [ ] Tags normalizados.
- [ ] Projects upsert/list.
- [ ] Series upsert/get/list/replace.
- [ ] Workflows autocontenidos.
- [ ] FTS5 funcional.
- [ ] Variable detection/injection funcional.
- [ ] Archive entry funcional.
- [ ] Import/export JSON básico.
- [ ] CLI alpha completa.
- [ ] MCP alpha 11 tools expuestas.
- [ ] MCP alpha probado con inspector o prueba equivalente.
- [ ] Tests obligatorios alpha pasan.
- [ ] README alpha con quickstart.

### 28.2 Done v1-final

- [ ] HTTP API local con `net/http`.
- [ ] `project_refs` implementado.
- [ ] `archive_series` implementado.
- [ ] `archive_project` implementado.
- [ ] `add_entry_to_series` implementado.
- [ ] `remove_entry_from_series` implementado.
- [ ] `copy_entry` implementado.
- [ ] `copy_series` implementado.
- [ ] stats implementado.
- [ ] CLI final completa.
- [ ] MCP final 22 tools.
- [ ] setup Claude Code.
- [ ] setup OpenCode.
- [ ] Tests final pasan.
- [ ] README final.
- [ ] ADRs mínimos escritos.

---

## 29. README quickstart esperado

El README debe incluir:

```bash
go build -o ~/tools/skillvault ./cmd/skillvault

skillvault init

skillvault project upsert examples/project-vitacare.json

skillvault entry upsert examples/prd-fastapi.json

skillvault search "fastapi"

skillvault mcp
```

También debe explicar:

- DB path
- binary path
- alpha vs final
- MCP setup manual
- import/export
- cómo correr tests

---

## 30. ADRs recomendados

Crear en:

```text
docs/adr/
```

ADRs mínimos:

1. SQLite como fuente operativa.
2. Biblioteca recuperable, no runtime agente.
3. Series vs workflows.
4. IDs globales únicos.
5. net/http sin Fiber.
6. CLI sin Cobra.
7. modernc.org/sqlite.
8. MCP propio mínimo.
9. Import/export solo CLI en v1.
10. No hard delete en v1.

---

## 31. Roadmap futuro

### v2.1 — Git Sync

Exportar vault como chunks versionables en git.

Fuera de v1.

### v2.2 — Cloud / VPS

HTTP API remota con autenticación.

Fuera de v1.

### v2.3 — Obsidian Export

Exportar entries como notas Markdown con frontmatter.

Fuera de v1.

### v2.4 — Raycast Extension

Búsqueda instantánea desde Raycast.

Fuera de v1.

### v2.5 — TUI

Bubble Tea TUI con vim keys.

Fuera de v1.

### v2.6 — Variable metadata

Evolucionar vars de array simple a metadata:

```json
[
  {
    "name": "project_name",
    "required": true,
    "description": "Nombre del proyecto",
    "example": "VitaCare"
  }
]
```

---

## 32. Resumen final

SkillVault v1 será una herramienta local, portable y estructurada para capturar, buscar y reutilizar conocimiento operativo.

`v1-alpha` valida la columna vertebral:

- SQLite
- entries
- projects
- series
- workflows
- FTS5
- CLI
- MCP core
- import/export básico

`v1-final` completa administración local:

- HTTP
- project refs
- copy
- archivado extendido
- stats
- setup de clientes
- MCP ampliado

La regla central:

```text
SkillVault no piensa por el usuario.
SkillVault organiza, recupera y prepara conocimiento para que humanos y agentes piensen mejor.
```
