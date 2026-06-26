# SkillVault — spec.md
**Knowledge Operating System · Propuestas de mejora post-análisis**
**Versión:** 2.1.0-proposal · **Fecha:** 2026-06-22 · **Autor:** Quantum

---

## 0. Contexto y alcance

Este documento especifica las mejoras derivadas del análisis de código del repositorio `QuantumEdu/kbs` (branch `feat/skillvault-v1-alpha`). El análisis identificó:

- 3 bugs críticos que afectan runtime y compilación
- 4 advertencias estructurales
- 5 mejoras de flujo profesional (AI engineering / agentic workflows)
- 2 vectores ocultos de valor no explotados

Las propuestas están ordenadas por **impacto × esfuerzo**, no por categoría. Cada ítem incluye criterio de aceptación verificable.

---

## 1. Bugfixes críticos (deben resolverse antes de cualquier nueva feature)

### BUG-01 · `go.mod` declara versión inexistente

**Problema:** El módulo declara `go 1.26`. REVISAR VERSION ACTUAL ESTABLE

**Fix:**
```go
// go.mod
go 1.26
```

**Criterio de aceptación:** `go build ./...` pasa sin advertencias de versión en Go 1.24.x estándar.

---

### BUG-02 · FTS5 no habilitado en `modernc.org/sqlite`

**Problema:** El driver pure-Go `modernc.org/sqlite` no compila FTS5 por defecto. Las queries de búsqueda fulltext retornan `no such module: fts5` en runtime. En contexto agéntico esto es silencioso — el agente recibe respuesta vacía y asume que no hay skills relevantes.

**Fix — opción A (recomendada, sin CGO):**
```go
// internal/db/db.go — agregar import side-effect
import (
    _ "modernc.org/sqlite"
    _ "modernc.org/sqlite/lib/fts5"
)
```

**Fix — opción B (si se migra a `mattn/go-sqlite3`):**
```go
// Requiere CGO_ENABLED=1 en build
// build tag: go build -tags "fts5" ./...
import _ "github.com/mattn/go-sqlite3"
```

**Criterio de aceptación:**
```bash
skillvault search "fastapi" --type prompt
# Retorna resultados o mensaje "no entries found" — nunca error de módulo FTS5
```

---

### BUG-03 · Variable injection sin protección contra referencias circulares

**Problema:** `internal/vars` resuelve variables en workflows/prompts sin límite de profundidad ni detección de ciclos. Un workflow con `{{var_a}}` que referencia `{{var_b}}` que referencia `{{var_a}}` causa loop infinito o stack overflow — especialmente peligroso cuando un agente construye workflows dinámicamente.

**Fix:**
```go
// internal/vars/resolver.go

const maxDepth = 5

func Resolve(content string, vars map[string]string, depth int) (string, error) {
    if depth > maxDepth {
        return "", fmt.Errorf("variable resolution exceeded max depth (%d): possible circular reference", maxDepth)
    }
    // ... lógica actual de resolución
    // llamadas recursivas: Resolve(val, vars, depth+1)
}

// Detección de ciclo simple por visited set
func ResolveWithCycleDetection(content string, vars map[string]string) (string, error) {
    visited := make(map[string]bool)
    return resolveInternal(content, vars, visited, 0)
}
```

**Criterio de aceptación:** Un workflow con referencia circular retorna error descriptivo en lugar de colgar el proceso.

---

### BUG-04 · `upsert <file>` sin validación de entrada

**Problema:** El CLI acepta un path de archivo para upsert sin validar que sea JSON válido ni que el archivo exista. Genera errores de runtime opacos, no mensajes útiles para el agente que invoca via MCP.

**Fix:**
```go
// cmd/entry.go — antes de pasarlo a db.Upsert

data, err := os.ReadFile(path)
if err != nil {
    return fmt.Errorf("cannot read file %q: %w", path, err)
}

var entry models.Entry
if err := json.Unmarshal(data, &entry); err != nil {
    return fmt.Errorf("invalid JSON in %q: %w\nHint: validate with 'jq . %s'", path, err, path)
}
```

**Criterio de aceptación:** Archivo vacío o JSON malformado retorna mensaje accionable con hint de diagnóstico.

---

## 2. Mejoras estructurales

### STRUCT-01 · Índice parcial en soft-delete

**Problema:** La columna `active` sin índice fuerza full table scan en cada `LIST` query. No duele en alpha con 100 entries. Duele desde los primeros 1,000.

**Migración:**
```sql
-- migrations/002_indexes.sql
CREATE INDEX IF NOT EXISTS idx_entries_active
    ON entries(active)
    WHERE active = 1;

CREATE INDEX IF NOT EXISTS idx_entries_type_active
    ON entries(type, active)
    WHERE active = 1;

CREATE INDEX IF NOT EXISTS idx_entries_series_active
    ON entries(series_id, active)
    WHERE active = 1;
```

**Criterio de aceptación:** `EXPLAIN QUERY PLAN SELECT * FROM entries WHERE active=1` muestra uso de índice, no "SCAN TABLE entries".

---

### STRUCT-02 · Graceful shutdown en MCP stdio server

**Problema:** El servidor MCP sobre stdio no maneja señales de terminación. Cuando el cliente (Claude Code, OpenCode) cierra la conexión, el proceso puede quedar zombie.

**Fix:**
```go
// cmd/mcp.go

func runMCPServer() error {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

    go func() {
        <-sigChan
        cancel()
    }()

    return server.ServeStdio(ctx)
}
```

**Criterio de aceptación:** `kill -TERM $(pgrep skillvault)` termina limpiamente sin procesos zombie.

---

## 3. Nuevas features — capa de datos

### FEAT-01 · Tag system nativo en entries

**Motivación:** En arquitecturas multi-agente, diferentes agentes especializados consumen subsets distintos de skills. Tags permiten routing por `agent_role`, `project`, `domain` sin queries complejas.

**Schema:**
```sql
-- Agregar a tabla entries (migración no destructiva)
ALTER TABLE entries ADD COLUMN tags TEXT DEFAULT '[]';
-- Formato: JSON array de strings ["prompt", "cybersec", "fastapi"]

-- FTS5 virtual table ya indexa content como JSON — tags quedan buscables
-- Para queries específicas de tags:
CREATE INDEX IF NOT EXISTS idx_entries_tags ON entries(tags);
```

**MCP tool nuevo:**
```go
// MCP tool: search_by_tags
// Input: { "tags": ["cybersec", "fastapi"], "match": "all" | "any" }
// Output: []Entry

func SearchByTags(tags []string, match string) ([]models.Entry, error) {
    // match="all": todos los tags deben estar presentes
    // match="any": al menos uno
}
```

**CLI:**
```bash
skillvault entry list --tags cybersec,fastapi
skillvault entry search "autenticación" --tags prompt,security
```

**Criterio de aceptación:** `search_by_tags(["cybersec"], "any")` retorna todas las entries que incluyen el tag "cybersec" en su array JSON.

---

### FEAT-02 · Versioning por entry

**Motivación:** AI engineering profesional requiere saber qué versión de un prompt generó qué resultado. Sin historial, el debugging de regresiones es ciego.

**Schema:**
```sql
-- Tabla de historial (append-only, no modifica entries)
CREATE TABLE IF NOT EXISTS entry_versions (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    entry_id    INTEGER NOT NULL REFERENCES entries(id),
    version     INTEGER NOT NULL,
    content     TEXT    NOT NULL,
    metadata    TEXT    DEFAULT '{}',
    changed_by  TEXT    DEFAULT 'user',  -- 'user' | 'agent' | 'import'
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_entry_versions_entry ON entry_versions(entry_id, version DESC);
```

**Comportamiento:**
- Cada `UPDATE` de una entry guarda el contenido anterior en `entry_versions`
- La tabla `entries` mantiene solo la versión actual (sin overhead de lectura)
- `version` se incrementa automáticamente via trigger o en la capa de repositorio

**MCP tools:**
```go
// get_entry_history(entry_id) → []EntryVersion
// restore_entry_version(entry_id, version) → Entry
```

**CLI:**
```bash
skillvault entry history <id>
skillvault entry restore <id> --version 3
```

**Criterio de aceptación:** Después de 3 updates consecutivos a una entry, `entry history <id>` muestra 3 versiones anteriores con timestamps.

---

### FEAT-03 · Execution log para workflows

**Motivación:** Sin log de ejecuciones, el debugging de agentes en producción es imposible. El log también sirve como dataset para análisis de efectividad de prompts.

**Schema:**
```sql
CREATE TABLE IF NOT EXISTS workflow_runs (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    workflow_id  INTEGER NOT NULL REFERENCES entries(id),
    vars_snapshot TEXT   NOT NULL DEFAULT '{}',  -- variables usadas en la ejecución
    output       TEXT,                            -- resultado generado
    duration_ms  INTEGER,
    status       TEXT    NOT NULL DEFAULT 'ok',   -- 'ok' | 'error' | 'partial'
    error_msg    TEXT,
    triggered_by TEXT    DEFAULT 'user',          -- 'user' | 'agent:<name>'
    created_at   DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_workflow_runs_workflow ON workflow_runs(workflow_id, created_at DESC);
```

**MCP tool:**
```go
// get_workflow_history(workflow_id, limit=10) → []WorkflowRun
// get_recent_runs(limit=20) → []WorkflowRun  — útil para dashboards de agentes
```

**CLI:**
```bash
skillvault workflow history <id>
skillvault workflow runs --last 10
```

**Criterio de aceptación:** Después de `skillvault workflow run <id>`, `workflow history <id>` muestra la ejecución con vars_snapshot y duración.

---

## 4. Nuevas features — capa MCP (alto impacto agéntico)

### FEAT-04 · Tool `get_context_bundle`

**Motivación:** Hoy un agente necesita 3-4 llamadas MCP para tener contexto operativo de un proyecto: `list_series` → `get_entries` → `get_workflows`. Con un bundle tool, una sola llamada retorna todo lo necesario. Reduce tokens de tool-use, reduce latencia, reduce riesgo de contexto incompleto.

**Spec del tool:**
```go
// MCP tool: get_context_bundle
// Descripción: "Returns all relevant context for a project in a single call.
//               Use this as the first tool call when starting work on a known project."

Input: {
    "project_id":   string,           // requerido
    "include":      []string,         // opcional: ["skills","prompts","workflows","agents"]
    "max_entries":  int,              // default: 20 por tipo
    "active_only":  bool              // default: true
}

Output: {
    "project":   SeriesInfo,
    "skills":    []Entry,
    "prompts":   []Entry,
    "workflows": []Entry,
    "agents":    []Entry,
    "summary":   string               // descripción del bundle para el agente
}
```

**Implementación sugerida:**
```go
func GetContextBundle(db *sql.DB, req BundleRequest) (*Bundle, error) {
    bundle := &Bundle{}

    // Query paralela por tipo usando goroutines
    var wg sync.WaitGroup
    types := req.IncludeTypes()  // default: todos los tipos

    for _, t := range types {
        wg.Add(1)
        go func(entryType string) {
            defer wg.Done()
            entries, _ := db.QueryEntriesBySeriesAndType(req.ProjectID, entryType, req.MaxEntries)
            bundle.SetType(entryType, entries)
        }(t)
    }

    wg.Wait()
    bundle.Summary = bundle.GenerateSummary()
    return bundle, nil
}
```

**Criterio de aceptación:** Una sola llamada `get_context_bundle("cyberautopilot")` retorna todas las entries activas del proyecto en <200ms con vault de hasta 500 entries.

---

### FEAT-05 · Tool `search_semantic` (preparación para embeddings)

**Motivación:** FTS5 es búsqueda léxica — encuentra "FastAPI" pero no "framework web Python". Los AI engineers profesionales usan similarity search para recuperar contexto relevante aunque las palabras exactas no coincidan. Este feature prepara la infraestructura sin requerir un embedding model hoy.

**Diseño en dos fases:**

**Fase 1 — ahora (sin embeddings):**
```go
// Búsqueda híbrida: FTS5 + scoring de relevancia por tags + recencia
// MCP tool: search_entries_ranked
Input: {
    "query":       string,
    "boost_tags":  []string,   // tags que aumentan score
    "boost_recent": bool,      // entradas recientes tienen mayor peso
    "limit":       int
}
```

**Fase 2 — cuando agentes requieran búsqueda semántica real:**
```sql
-- Columna para vector de embedding (sqlite-vec o modernc-vec)
ALTER TABLE entries ADD COLUMN embedding BLOB;
-- Índice virtual: CREATE VIRTUAL TABLE entries_vec USING vec0(embedding float[1536])
```

La interfaz MCP del tool no cambia entre fases — el agente llama `search_semantic` en ambos casos. La implementación interna mejora sin breaking change.

**Criterio de aceptación Fase 1:** `search_entries_ranked("autenticación JWT")` retorna entries con tags "security" y "auth" con mayor score que entries sin esos tags, incluso si "JWT" no aparece textualmente en todas ellas.

---

### FEAT-06 · Tool `compare_entries`

**Motivación:** Durante refinamiento de prompts, los AI engineers necesitan comparar versiones para decidir cuál es más efectiva. Sin esta capacidad, la comparación es manual y propensa a errores.

```go
// MCP tool: compare_entries
Input: {
    "entry_ids":  []int,      // 2-4 entries a comparar
    "focus":      string      // "content" | "metadata" | "both"
}

Output: {
    "entries":    []Entry,
    "diff":       []DiffChunk,    // diff unificado entre contenidos
    "common_tags": []string,
    "unique_tags": map[int][]string
}
```

**Criterio de aceptación:** `compare_entries([42, 43])` retorna diff legible entre el content de las dos entries identificando adiciones, eliminaciones y secciones comunes.

---

## 5. Feature de sincronización — `serve --watch`

### FEAT-07 · File watcher para sync desde Obsidian/editor

**Motivación:** El vault debe sincronizarse con el flujo de trabajo real, no requerir comandos manuales. Un watcher sobre un directorio de skills en formato Markdown permite que la edición en Obsidian o cualquier editor actualice el vault automáticamente.

**Comportamiento:**
```
~/.skills/                    ← directorio monitoreado
  prompts/
    fastapi-auth.md           → entry tipo "prompt", series "fastapi"
    sql-query-builder.md      → entry tipo "prompt", series "database"
  workflows/
    cyberautopilot-init.md    → entry tipo "workflow", series "cyberautopilot"
```

**Frontmatter convención (Markdown):**
```yaml
---
type: prompt          # skill | prompt | agent | workflow | snippet | context
series: fastapi
tags: [auth, jwt, security]
version: 1.2
---

# FastAPI Auth Prompt

Tu eres un experto en FastAPI...
```

**Implementación:**
```go
// cmd/serve.go — nuevo subcomando

import "github.com/fsnotify/fsnotify"

func runWatch(dir string) error {
    watcher, err := fsnotify.NewWatcher()
    // ...
    // On CREATE/WRITE .md file:
    //   1. Parse frontmatter
    //   2. Extract body (contenido después del ---)
    //   3. Upsert entry via db.UpsertFromFile(entry)
    //   4. Log: "✓ synced prompts/fastapi-auth.md → entry #42"
}
```

**CLI:**
```bash
skillvault serve --watch ~/.skills
# Output:
# Watching ~/.skills for changes...
# ✓ synced prompts/fastapi-auth.md → entry #42 (updated)
# ✓ synced workflows/cyberautopilot-init.md → entry #17 (created)
```

**Criterio de aceptación:** Guardar un archivo `.md` en el directorio monitoreado actualiza la entry correspondiente en el vault en menos de 500ms, sin reiniciar el proceso.

---

## 6. Feature oculta — Skill Pack Export

### FEAT-08 · Export como skill pack distribuible

**Motivación:** El sistema ya tiene import/export JSON. Con metadata adicional, un export se convierte en un "skill pack" intercambiable entre equipos o proyectos. El vault puede ser tanto consumidor como proveedor de conocimiento.

**Formato del skill pack:**
```json
{
    "pack_id":      "cyberautopilot-v1.0",
    "name":         "CyberSec Autopilot Skills",
    "author":       "QuantumEdu",
    "version":      "1.0.0",
    "created_at":   "2026-06-22T00:00:00Z",
    "description":  "Skills, prompts y workflows del agente CyberSec Autopilot",
    "tags":         ["cybersec", "fastapi", "agents"],
    "entries": [
        {
            "type":     "prompt",
            "name":     "vuln-triage",
            "content":  "...",
            "tags":     ["triage", "cvss"],
            "series":   "cyberautopilot"
        }
    ]
}
```

**CLI:**
```bash
skillvault export --series cyberautopilot --format pack > cyberautopilot-v1.0.svpack
skillvault import --pack cyberautopilot-v1.0.svpack --prefix "imported/"
```

**MCP tool:**
```go
// export_series_as_pack(series_id) → PackJSON
// import_pack(pack_json, options) → ImportResult
```

**Criterio de aceptación:** Un pack exportado de un vault puede importarse en otro vault vacío y retener toda la metadata, tags y versiones.

---

## 7. Seguridad — preparación para HTTP API

### SEC-01 · Auth layer antes de exponer HTTP

**Motivación:** La spec indica HTTP API en v1-final. Un vault de skills sin autenticación expuesto en red (Hetzner VPS, Railway) es un vector real: cualquier agente o proceso con acceso a la red puede leer, modificar o eliminar el conocimiento acumulado.

**Requisitos mínimos para HTTP API:**

```go
// Opción A — API Key simple (recomendada para uso personal/equipo pequeño)
// Header: Authorization: Bearer <api_key>
// Config: ~/.skillvault/config.yaml → api_key: "sv_xxxxx"

// Opción B — mTLS para integración agéntica en infraestructura propia
// Cert del cliente requerido para conectar

// Middleware de autenticación:
func AuthMiddleware(apiKey string) fiber.Handler {
    return func(c *fiber.Ctx) error {
        token := c.Get("Authorization")
        if token != "Bearer "+apiKey {
            return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
        }
        return c.Next()
    }
}
```

**Rate limiting básico:**
```go
// 100 requests/min por API key — previene abuso de agentes con loops
limiter := limiter.New(limiter.Config{
    Max:        100,
    Expiration: 1 * time.Minute,
})
```

**Criterio de aceptación:** El endpoint HTTP retorna 401 para requests sin API key válida. Los endpoints MCP stdio no requieren auth (permanecen locales).

---

## 8. Documentación de `internal/vars`

### DOC-01 · Spec del motor de variables

**Problema:** `internal/vars` es el módulo más poderoso del sistema y el menos documentado. Ningún colaborador ni agente puede usar correctamente workflows con variables sin entender la sintaxis.

**Documentación requerida en `docs/vars.md`:**

```markdown
## Sintaxis de variables

Variables se declaran con doble llave: `{{nombre_variable}}`

### Tipos de resolución
1. **Literal**: `{{fecha}}` → valor string directo del mapa de vars
2. **Entry reference**: `{{entry:42}}` → content de la entry con id 42
3. **Series reference**: `{{series:fastapi.latest}}` → última entry de la serie

### Scope model
- Variables se resuelven en orden de precedencia: CLI flags > env vars > defaults
- Resolución máxima: 5 niveles de profundidad
- Referencias circulares: retornan error descriptivo

### Ejemplos
workflow content:
  "Analiza el siguiente código usando {{entry:17}} como referencia.
   Contexto del proyecto: {{project_context}}"

vars map:
  "project_context": "Sistema de autenticación FastAPI con JWT"
```

**Criterio de aceptación:** Un nuevo colaborador puede escribir un workflow con variables en <10 minutos usando solo `docs/vars.md` como referencia.

---

## 9. Roadmap de implementación

| Prioridad | ID | Descripción | Esfuerzo | Impacto |
|---|---|---|---|---|
| 🔴 P0 | BUG-01 | Fix go.mod versión | 5 min | Crítico |
| 🔴 P0 | BUG-02 | Fix FTS5 build tag | 15 min | Crítico |
| 🔴 P0 | BUG-03 | Var injection sandbox | 2h | Crítico |
| 🔴 P0 | BUG-04 | Validación upsert file | 30 min | Alto |
| 🟠 P1 | STRUCT-01 | Índices soft-delete | 30 min | Alto |
| 🟠 P1 | STRUCT-02 | Graceful shutdown MCP | 1h | Alto |
| 🟡 P2 | FEAT-01 | Tag system nativo | 3h | Alto |
| 🟡 P2 | FEAT-04 | Tool get_context_bundle | 4h | Muy alto |
| 🟡 P2 | FEAT-07 | File watcher --watch | 4h | Alto |
| 🟢 P3 | FEAT-02 | Versioning por entry | 3h | Medio |
| 🟢 P3 | FEAT-03 | Execution log workflows | 2h | Medio |
| 🟢 P3 | FEAT-06 | Tool compare_entries | 2h | Medio |
| 🔵 P4 | FEAT-05 | Search ranked (Fase 1) | 3h | Medio |
| 🔵 P4 | FEAT-08 | Skill pack export | 4h | Medio |
| 🔵 P4 | SEC-01 | Auth layer HTTP API | 3h | Alto (cuando HTTP) |
| 🔵 P4 | DOC-01 | Spec motor de variables | 2h | Medio |

**Total estimado P0+P1:** ~5h de trabajo real  
**Total estimado P0–P3:** ~22h  
**Total completo:** ~37h

---

## 10. Criterios de éxito del sistema post-mejoras

El vault se considera Production-Ready para workflows agénticos cuando:

1. Un agente puede llamar `get_context_bundle("proyecto")` y recibir contexto completo en una sola llamada
2. Cambiar un archivo `.md` en Obsidian actualiza el vault en <500ms sin comandos manuales
3. Dos skills con el mismo nombre pero distinto contenido son distinguibles por versión y timestamp
4. Un workflow con referencia circular retorna error descriptivo, no cuelga
5. `skillvault search "término"` retorna resultados correctos via FTS5 (no error de módulo)
6. El proceso MCP termina limpiamente con `SIGTERM` sin dejar zombies

---

*Generado a partir del análisis técnico del repositorio `QuantumEdu/kbs` · 2026-06-22*
