# CLI Reference — 19 comandos

Todas las entradas usan **slugs** como identificadores. Un slug es el título en kebab-case: `"Clean Architecture Review"` → `clean-architecture-review`.

---

## `init`

Inicializa el vault: crea directorios y la base de datos SQLite.

```bash
skillvault init
```

Idempotente: si ya existe `~/.skillvault/`, solo asegura que los subdirectorios estén.

---

## `add-entry`

Guarda una entrada reutilizable (prompt, skill, decisión, feedback, etc.).

```bash
skillvault add-entry \
  --title "CSS Grid Layout" \
  --type skill \
  --summary "Guía rápida de CSS Grid" \
  --content "grid-template-columns: repeat(...)" \
  --project web \
  --tags "css,frontend" \
  --status active
```

| Flag | Requerido | Descripción |
|------|-----------|-------------|
| `--title` | ✅ | Título de la entrada (genera slug automático) |
| `--type` | ✅ | Tipo: `prompt`, `skill`, `workflow_note`, `reference`, `user`, `feedback`, `project_state`, `session`, `decision`, `artifact_summary` |
| `--summary` | ✅ | Resumen corto (indexado en FTS5) |
| `--content` | ❌ | Contenido largo (opcional si solo querés el resumen) |
| `--project` | ❌ | Slug del proyecto al que pertenece |
| `--tags` | ❌ | Tags separados por coma |
| `--status` | ❌ | `draft`, `active` (default), `archived`, `deprecated`, `canonical` |

El vault rechaza la entrada si detecta secretos (API keys, tokens, private keys).

---

## `search`

Búsqueda full-text con filtros.

```bash
skillvault search "grid css" --type skill --project web --status active --limit 10
```

| Flag | Descripción |
|------|-------------|
| `--type` | Filtrar por tipo de entrada |
| `--project` | Filtrar por proyecto |
| `--tags` | Filtrar por tags (separados por coma) |
| `--status` | Filtrar por estado |
| `--limit` | Máximo de resultados (default: 20) |

La búsqueda usa FTS5 con tokenizer `porter unicode61`. Busca en título, summary, content y tags.

---

## `get`

Obtiene una entrada por ID o slug.

```bash
skillvault get css-grid-layout
skillvault get skill:css-grid-layout        # con prefijo de tipo
```

Devuelve:

```json
{
  "id": "css-grid-layout",
  "type": "skill",
  "title": "CSS Grid Layout",
  "status": "active",
  "summary": "Guía rápida de CSS Grid",
  "created_at": "2026-06-20T..."
}
```

---

## `save-artifact`

Guarda un artefacto largo respaldado por el filesystem.

```bash
skillvault save-artifact \
  --title "Reporte de auditoría" \
  --type pdf_analysis \
  --content "$(cat reporte.md)" \
  --project miapp \
  --tags "seguridad"
```

| Flag | Requerido | Descripción |
|------|-----------|-------------|
| `--title` | ✅ | Título del artefacto |
| `--type` | ✅ | Tipo: `pdf_analysis`, `spec_doc`, `architecture_doc`, `research_note`, `prompt_response`, `ai_output`, `raw_document`, `report`, `log_analysis`, `generated_code` |
| `--content` | ✅ | Contenido (se almacena en `~/.skillvault/objects/YYYY/MM/`) |
| `--project` | ❌ | Proyecto asociado |
| `--tags` | ❌ | Tags |

El contenido se guarda en disco con hash SHA256. La metadata (título, tipo, tags, slug del archivo) va a SQLite.

---

## `get-context`

Compila un paquete de contexto para agentes.

```bash
skillvault get-context \
  --mode planning \
  --project miapp \
  --max-chars 8000
```

| Flag | Requerido | Descripción |
|------|-----------|-------------|
| `--mode` | ✅ | Modo: `profile`, `project`, `workflow`, `skill`, `planning`, `session_recall`, `full_brief` |
| `--project` | ✅ | Proyecto |
| `--max-chars` | ❌ | Máximo de caracteres (default: 10000) |
| `--include` | ❌ | Filtrar secciones específicas |

### Modos de contexto

| Modo | Qué incluye |
|------|-------------|
| `profile` | User feedback + entradas tipo user |
| `project` | Estado activo del proyecto + decisiones + resúmenes de artefactos |
| `workflow` | Workflows + sus pasos |
| `skill` | Skills activos + prompts |
| `planning` | profile + project + workflow combinados |
| `session_recall` | Últimas 10 sesiones |
| `full_brief` | Todas las secciones |

Si el contenido excede `max_chars`, se truncan las secciones de menor prioridad primero.

---

## `add-project`

Crea un proyecto.

```bash
skillvault add-project \
  --name "MiApp" \
  --description "Aplicación web de ejemplo"
```

Los proyectos agrupan entradas, artefactos, sesiones y workflows.

---

## `list-projects`

Lista todos los proyectos.

```bash
skillvault list-projects
```

---

## `archive`

Archiva una entrada (soft delete: cambia status a `archived`).

```bash
skillvault archive css-grid-layout
```

Las entradas archivadas siguen siendo searchables pero se excluyen de los context packs.

---

## `add-workflow`

Crea un workflow desde un archivo JSON.

```bash
skillvault add-workflow workflow.json
```

Formato del JSON:

```json
{
  "id": "spec-plan-task",
  "name": "Spec → Plan → Task",
  "steps": [
    {"order": 1, "description": "Write spec", "type": "prompt"},
    {"order": 2, "description": "Review with team"},
    {"order": 3, "description": "Create tasks"}
  ]
}
```

---

## `render-workflow`

Renderiza un workflow como checklist.

```bash
skillvault render-workflow spec-plan-task
```

Output:

```
- [ ] Write spec
- [ ] Review with team
- [ ] Create tasks
```

---

## `run`

Ejecuta un workflow como pipeline paso a paso.

```bash
skillvault run <workflow-slug> <input-file> [--save output.md]
skillvault run research-article article.md --save result.md
skillvault run research-article -                  # leer input desde stdin
```

| Flag | Requerido | Descripción |
|------|-----------|-------------|
| `--save` | ❌ | Guarda el output final en un archivo |

El pipeline ejecuta cada paso del workflow que tenga `entry_slug` configurado:
1. Resuelve la entry asociada y verifica que esté activa
2. Inyecta `{{input}}`, `{{previous_output}}`, `{{final_output}}` en el contenido
3. Muestra el prompt renderizado en stdout
4. Lee la respuesta del agente desde stdin
5. Pasa el resultado al siguiente paso como `{{previous_output}}`

Pasos sin `entry_slug` se saltean (checklists renderizables).

---

## `session-wrap`

Crea una entrada de sesión con decisiones, pendientes y aprendizajes.

```bash
skillvault session-wrap \
  --project miapp \
  --summary "Sprint planning" \
  --decisions "Migrar a SQLite,usar FTS5" \
  --pending "Benchmark queries" \
  --learnings "FTS5 necesita tokenizer explícito"
```

Parámetros separados por coma. Opcionalmente puede linkear un artefacto.

---

## `graph`

Visualiza el grafo de relaciones entre entradas.

```bash
skillvault graph --entry clean-architecture-review --depth 3 --format mermaid
skillvault graph --entry clean-architecture-review --format json
skillvault graph --entry clean-architecture-review --format dot
```

| Flag | Requerido | Descripción |
|------|-----------|-------------|
| `--entry` | ✅ | ID de la entrada raíz |
| `--depth` | ❌ | Profundidad de traversión (default: 3, max: 10) |
| `--format` | ❌ | `mermaid`, `json` o `dot` (default: mermaid) |
| `--direction` | ❌ | `outgoing`, `incoming` o `both` (default: both) |

El formato `mermaid` genera `graph TD` que se renderiza nativamente en GitHub.

---

## `entry ref`

Gestiona aristas del grafo entre entradas (entry_links).

```bash
# Añadir relación
skillvault entry ref add <source> <target> <type> --label "opcional"

# Listar relaciones
skillvault entry ref list [--source <id>] [--target <id>] [--type <rel>]

# Eliminar relación
skillvault entry ref remove <source> <target> <type>
```

Tipos de relación: `references`, `supersedes`, `related_to`, `part_of`, `derived_from`, `implements`, `uses`, `extends`, `handoff_of`, `generated_from`, `depends_on`.

Las relaciones `depends_on`, `part_of` y `supersedes` tienen detección de ciclos.

---

## `memory index` / `memory reindex` / `memory list-external`

Indexa archivos pi-memory (.md) como shadow entries en el vault.

```bash
# Indexar un directorio de memoria
skillvault memory index --path ~/memory --project myapp [--wikilinks]

# Reindexar (alias)
skillvault memory reindex --path ~/memory --project myapp

# Listar entradas externas indexadas
skillvault memory list-external --project myapp
```

| Flag | Requerido | Descripción |
|------|-----------|-------------|
| `--path` | ✅ | Directorio con archivos .md |
| `--project` | ✅ | Proyecto destino |
| `--wikilinks` | ❌ | Parsea `[[wikilinks]]` y crea entry_refs |

Soporta frontmatter YAML (description, tags, created, updated). Archivos eliminados del directorio se archivan automáticamente (orphan cleanup).

---

Exporta todo el vault a un archivo JSON.

```bash
skillvault export backup.json
```

Incluye todos los tipos de entrada, proyectos, workflows, series, tags, entry_links y metadata de artefactos.

---

## `import`

Importa un vault desde un archivo JSON.

```bash
skillvault import backup.json
```

Resuelve conflictos de slug automáticamente (agrega sufijo numérico a duplicados).

---

## `http`

Inicia el servidor HTTP REST API.

```bash
skillvault http
# Sirve en http://127.0.0.1:7438
```

Endpoints disponibles: health, entries CRUD, artifacts, context, projects, sessions, workflows, export/import. Ver [`docs/quickstart.md`](quickstart.md) o [`docs/architecture.md`](architecture.md) para detalles.
