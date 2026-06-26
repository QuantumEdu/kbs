# MCP Server — 15 herramientas para agentes AI

SkillVault funciona como servidor MCP (Model Context Protocol) sobre stdio JSON-RPC 2.0. Esto permite que agentes como Claude Code, OpenCode, o cualquier cliente MCP lean y escriban directamente en tu vault.

---

## Setup

### 1. Instalación

```bash
go build -o ~/tools/skillvault ./cmd/skillvault
```

### 2. Configuración

Agregá a tu `opencode.json` (o claude_desktop_config.json, según el cliente):

```json
{
  "mcpServers": {
    "skillvault": {
      "command": "/home/tu-user/tools/skillvault",
      "args": ["mcp"]
    }
  }
}
```

### 3. Symlink para acceso directo (opcional)

```bash
ln -sf ~/tools/skillvault ~/tools/mcp
# Ahora los agentes pueden llamar "mcp" directamente
# El binario detecta el symlink y entra en modo MCP automáticamente
```

Si el binario se ejecuta como `mcp` (por el nombre del symlink), entra directo a modo MCP sin necesidad del argumento.

### 4. Verificación

```bash
skillvault mcp
# Espera conexiones stdio JSON-RPC. Probalo con:
echo '{"jsonrpc":"2.0","id":1,"method":"list_projects","params":{}}' | skillvault mcp
```

---

## Herramientas

### `save_entry`

Guarda cualquier tipo de entrada en el vault.

**Parámetros:**
- `title` (string, requerido) — Título
- `type` (string, requerido) — Tipo: `prompt`, `skill`, `workflow_note`, `reference`, `user`, `feedback`, `project_state`, `session`, `decision`, `artifact_summary`
- `summary` (string, requerido) — Resumen
- `content` (string, opcional) — Contenido largo
- `project` (string, opcional) — Slug del proyecto
- `tags` (string[], opcional) — Tags
- `status` (string, opcional) — `draft`, `active`, `archived`, `deprecated`, `canonical`

**Ejemplo desde el agente:**
```json
{
  "method": "save_entry",
  "params": {
    "title": "Clean Architecture Rules",
    "type": "skill",
    "summary": "Reglas de Clean Architecture para el proyecto",
    "content": "1. Domain no depende de nada\n2. App depende de domain\n3. Adapters dependen de app",
    "project": "miapp",
    "tags": ["architecture", "clean-code"]
  }
}
```

**Respuesta:**
```json
{
  "id": "clean-architecture-rules",
  "status": "created"
}
```

---

### `search_entries`

Búsqueda full-text con filtros.

**Parámetros:**
- `query` (string, requerido) — Términos de búsqueda
- `type` (string, opcional) — Filtrar por tipo
- `project` (string, opcional) — Filtrar por proyecto
- `tags` (string[], opcional) — Filtrar por tags
- `status` (string, opcional) — Filtrar por estado
- `limit` (int, opcional) — Máximo de resultados

---

### `get_entry`

Obtiene una entrada por ID o slug.

**Parámetros:**
- `id` (string, requerido) — ID o slug de la entrada

---

### `save_artifact`

Guarda un artefacto largo respaldado por filesystem.

**Parámetros:**
- `title` (string, requerido) — Título
- `type` (string, requerido) — Tipo de artefacto
- `content` (string, requerido) — Contenido completo
- `project` (string, opcional) — Proyecto
- `tags` (string[], opcional) — Tags

**Cómo funciona:** el contenido se escribe en `~/.skillvault/objects/YYYY/MM/<slug>.<ext>` con hash SHA256. La metadata (título, tipo, slug, fecha) va a SQLite. Ideal para outputs largos de AI, análisis de PDFs, reportes, etc.

---

### `get_context`

Compila un paquete de contexto para el agente.

**Parámetros:**
- `mode` (string, requerido) — `profile`, `project`, `workflow`, `skill`, `planning`, `session_recall`, `full_brief`
- `project` (string, requerido) — Proyecto
- `max_chars` (int, opcional) — Máximo de caracteres (default: 10000)
- `include` (string[], opcional) — Filtrar secciones específicas

**Respuesta:** texto estructurado con secciones priorizadas. Ideal para inyectar directamente en el prompt del agente.

---

### `compose_series`

Obtiene entradas ordenadas de una serie.

**Parámetros:**
- `id` (string, requerido) — ID de la serie

---

### `render_workflow`

Obtiene los pasos de un workflow como checklist ordenado.

**Parámetros:**
- `id` (string, requerido) — ID del workflow

---

### `session_wrap`

Crea una entrada de sesión con decisiones, pendientes y aprendizajes.

**Parámetros:**
- `project` (string, requerido) — Slug del proyecto
- `summary` (string, requerido) — Resumen de la sesión
- `decisions` (string[], opcional) — Decisiones tomadas
- `pending` (string[], opcional) — Pendientes
- `learnings` (string[], opcional) — Aprendizajes

**Ejemplo desde el agente:**
```json
{
  "method": "session_wrap",
  "params": {
    "project": "miapp",
    "summary": "Implementamos el módulo de auth",
    "decisions": ["JWT con refresh tokens", "SQLite para sesiones"],
    "pending": ["Agregar rate limiting", "Documentar endpoints"],
    "learnings": ["El middleware de JWT debe ir antes del CORS handler"]
  }
}
```

---

### `archive_entry`

Archiva una entrada (cambia status a `archived`).

**Parámetros:**
- `id` (string, requerido) — ID de la entrada a archivar

---

### `list_projects`

Lista todos los proyectos con su estado.

**Parámetros:** ninguno.

---

### `search_by_tags`

Busca entradas por tags usando intersección (all) o unión (any).

**Parámetros:**
- `tags` (string[], requerido) — Tags a buscar
- `match` (string, opcional) — `all` (intersección, default) o `any` (unión)
- `type` (string, opcional) — Filtrar por tipo de entrada
- `project` (string, opcional) — Filtrar por proyecto
- `limit` (int, opcional) — Máximo de resultados (default: 20)

**Ejemplo desde el agente:**
```json
{
  "method": "search_by_tags",
  "params": {
    "tags": ["tdd", "go"],
    "match": "all"
  }
}
```

---

### `get_context_bundle`

Obtiene un bundle estructurado de contexto de proyecto en una sola llamada.

**Parámetros:**
- `project` (string, opcional) — Slug del proyecto

**Respuesta:** JSON estructurado con información del proyecto, entradas agrupadas por tipo y referencias a artefactos.

Útil como primera llamada cuando un agente comienza a trabajar en un proyecto conocido.

---

### `save_entry_ref`

Crea o actualiza una arista (relación) entre dos entradas.

**Parámetros:**
- `source_id` (string, requerido) — ID de la entrada origen
- `target_id` (string, requerido) — ID de la entrada destino
- `ref_type` (string, requerido) — Tipo: `references`, `supersedes`, `related_to`, `part_of`, `derived_from`, `implements`, `uses`, `extends`, `handoff_of`, `generated_from`, `depends_on`
- `label` (string, opcional) — Etiqueta descriptiva

**Ejemplo:**
```json
{
  "method": "save_entry_ref",
  "params": {
    "source_id": "clean-architecture-rules",
    "target_id": "hexagonal-architecture-guide",
    "ref_type": "related_to",
    "label": "Ambos son patrones arquitectónicos"
  }
}
```

**Nota:** Las relaciones `depends_on`, `part_of` y `supersedes` tienen detección de ciclos — no se permite crear una arista que genere un ciclo.

---

### `list_entry_refs`

Lista aristas del grafo con filtros opcionales.

**Parámetros:**
- `source_id` (string, opcional) — Filtrar por origen
- `target_id` (string, opcional) — Filtrar por destino
- `ref_type` (string, opcional) — Filtrar por tipo de relación

---

### `get_entry_graph`

Traversa el grafo desde una entrada raíz y devuelve nodos y aristas conectados.

**Parámetros:**
- `entry_id` (string, requerido) — ID de la entrada raíz
- `depth` (int, opcional) — Profundidad máxima (default: 3, max: 10)
- `direction` (string, opcional) — `outgoing`, `incoming` o `both` (default: both)

**Respuesta:**
```json
{
  "root_entry": "clean-architecture-rules",
  "nodes": [{"id": "...", "title": "...", "type": "..."}],
  "edges": [{"source_id": "...", "target_id": "...", "ref_type": "..."}],
  "node_count": 5,
  "edge_count": 4
}
```

---

## Flujo típico desde el agente

```
1. Al inicio de sesión → get_context_bundle(project=miapp)
   → Bundle completo: proyecto + entradas agrupadas por tipo + artefactos

2. Durante la sesión → save_entry(...) o save_artifact(...)
   → Guarda skills, decisiones, outputs largos

3. Búsqueda específica → search_by_tags(tags=["go","tdd"], match="all")
   → Encuentra entradas exactas por tags

4. Al cerrar → session_wrap(project, summary, decisions, pending, learnings)
   → Persiste el estado de la sesión para la próxima
```

## Configuración por editor

### OpenCode

```json
// opencode.json
{
  "mcpServers": {
    "skillvault": {
      "command": "/home/user/tools/skillvault",
      "args": ["mcp"]
    }
  }
}
```

### Claude Code (VS Code extension)

```json
// claude_desktop_config.json
{
  "mcpServers": {
    "skillvault": {
      "command": "/home/user/tools/skillvault",
      "args": ["mcp"]
    }
  }
}
```

### Cline / Continue

Misma configuración — apuntan al mismo binario con `args: ["mcp"]`.
