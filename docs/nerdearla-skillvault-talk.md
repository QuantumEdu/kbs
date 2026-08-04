---
marp: true
theme: default
paginate: true
---

# SkillVault

**Memoria persistente y contexto para agentes de IA**

Nerdearla · 2026

---

<!-- speaker note: Apertura. Presentar el problema con empatía: todos trabajamos con agentes de IA que olvidan. El objetivo de la charla es mostrar que la memoria no se resuelve con más archivos, sino con un sistema de contexto local-first. -->

## El problema: los agentes olvidan

Cada sesión nueva, empezamos (casi) de cero.

- Prompts y skills dispersos entre chats, archivos y repositorios
- Outputs largos de IA que contaminan el contexto si se guardan como prompts
- Decisiones de proyecto que se pierden entre sesiones
- El contexto nunca es el adecuado: o falta, o sobra
- La **carga cognitiva** de re-explicar todo es el costo oculto

---

<!-- speaker note: La tesis central. Quiero que el público se lleve esta frase: el agente debe recordar SIN memorizar. La memoria debe vivir fuera del modelo. -->

## La tesis

> Un agente debería **recordar** sin tener que **memorizar** rutas ni nombres.

- La memoria no debería depender de la ventana de contexto del modelo
- Debe ser una **fuente de verdad local** que humanos y agentes comparten
- Lo que importa no es guardar, es **recuperar el contexto correcto** en el momento correcto

Métrica propuesta: **TTCK** — *Time To Correct Knowledge*.

---

<!-- speaker note: Definición de qué es SkillVault. Mapa de componentes: un CLI, secretos cifrados, telemetría local y una TUI. Todo local-first. -->

## ¿Qué es SkillVault?

Un sistema operativo de conocimiento **local-first** para desarrolladores y agentes de IA.

```
skillvault         CLI + memoria persistente + servidor MCP
q-secrets          Gestión de secretos cifrados (age + macOS Keychain)
telemetryd         Daemon de telemetría local (socket Unix)
telemetryctl       Consultas de runs y eventos
telemetrywrap      Wrapper que infiere eventos de cualquier comando
skillvault-tui     Interfaz de terminal (Bubble Tea)
```

- Un binario de ~7 MB · Go puro · sin CGO
- **Cero frameworks**: la única dependencia externa es `modernc.org/sqlite`

---

<!-- speaker note: Arquitectura. Clean Architecture ligera: los tres adapters (CLI, MCP, HTTP) hablan con los mismos casos de uso. Dominio puro, sin dependencias. -->

## Arquitectura

*Clean Architecture* ligera: los adapters comparten los mismos casos de uso.

```
   Adapters                    Use cases               Persistencia
┌────────────────────┐   ┌──────────────────┐   ┌──────────────────────┐
│ CLI  (37 comandos) │   │  internal/app    │   │  db      SQLite+FTS5 │
│ MCP  (24 tools)    │──▶│  Save · Search   │──▶│  files   objects/    │
│ HTTP (local only)  │   │  Context · ...   │   │          + SHA256    │
└────────────────────┘   └──────────────────┘   └──────────────────────┘
                                 │
                          ┌──────▼──────┐
                          │ internal/   │  Entidades y validadores
                          │ domain      │  puros, sin dependencias
                          └─────────────┘
```

- Slugs como identificadores legibles y estables
- Migraciones SQL embebidas con `go:embed`
- 12+ tipos de entrada y 6 propósitos alineados con LifeOS (`WORK`, `KNOWLEDGE`, ...)

---

<!-- speaker note: Almacenamiento híbrido. La regla de oro: qué va a la DB y qué al disco. Los artefactos largos nunca ensucian el contexto. -->

## Almacenamiento híbrido

> **DB decides. Disk remembers. Qu@ntum delivers.**

```
~/.skillvault/
├── vault.db          # SQLite + FTS5: metadata, búsqueda, relaciones
├── objects/2026/07/  # Artefactos largos (hash SHA256)
├── exports/          # Backups JSON con fecha
└── cache/
```

- **DB**: contenido pequeño y consultado con frecuencia
- **Disco**: outputs largos de IA, análisis, specs, reportes
- **Regla**: nada de nube por defecto. Solo tráfico de red si `sync` lo pide

---

<!-- speaker note: El contexto como producto. Aquí está la innovación: 7 modos de compilación, prioridad de secciones y truncado consciente del presupuesto de tokens. -->

## El contexto como producto

El compilador **Qu@ntum** entrega el contexto justo, no todo lo guardado.

- 7 modos: `profile`, `project`, `workflow`, `skill`, `planning`, `session_recall`, `full_brief`
- Compilación por prioridad: preferencias → estado → decisiones → sesiones → skills
- Truncado **token-aware**: se eliminan primero las secciones de menor prioridad

```
# CONTEXT PACK
## Scope
Project: MyApp  Mode: planning

## Active Decisions
- SQLite para almacenamiento local

## Active Pending
- [ ] Agregar refresh token rotation

## Suggested Next Action
Generar plan de implementación a partir del spec.
```

---

<!-- speaker note: Demo rápida. Estos comandos son reales y recién escritos. Notar lo cortos que son: la intención gobierna la sintaxis. -->

## Primeros pasos (en 5 minutos)

```bash
skillvault setup                              # inicializar el vault
skillvault doctor                             # verificar que esté listo

skillvault project start --name "MyApp"       # crear proyecto

skillvault add-entry \
  --title "Code Review Checklist" \
  --type skill --purpose KNOWLEDGE \
  --summary "Checklist para revisar PRs" \
  --project myapp --tags "review,pr"

skillvault find "architecture"                # buscar en FTS5
skillvault context --project myapp --mode planning   # contexto para el agente
```

---

<!-- speaker note: Inicio de la historia de UX. Antes: 37 comandos planos, flags crípticos, ayuda alfabética. El usuario tenía que memorizar la sintaxis del sistema en vez de expresar su intención. -->

## La evolución de UX: antes

Un CLI con 37 comandos planos y cero memoria de intención.

- Nombres técnicos: `add-entry`, `get-context`, `list-projects`, `render-workflow`
- Ayuda alfabética, no orientada a tareas
- El usuario debía **memorizar la sintaxis** antes de resolver su problema
- Errores de tipeo = error genérico y callejón sin salida

```
$ skillvault add-project --name "MyApp"       # ¿o era project add?
$ skillvault get-context --mode planning      # ¿o era context?
```

La carga cognitiva vivía en el CLI, no en el problema.

---

<!-- speaker note: La idea. Un registro de comandos con metadata de intención, agrupado por tarea. La misma metadata alimenta aliases, ayuda y sugerencias fuzzy. -->

## La evolución de UX: la idea

Reconstruir el CLI alrededor de **intenciones**, no de rutas memorizadas.

- `internal/cli/command_registry.go`: una fuente de verdad con metadata por comando
- **Aliases de intención**: expresan qué querés hacer, no cómo se llama el flag
- **Agrupación por tarea**: Setup, Store, Find, Context, Workflows, Maintenance, Integrations
- **Fuzzy routing** sobre comandos de solo lectura

```
"intención"  ──▶  NormalizeArgs  ──▶  comando canónico  ──▶  handler
```

Los comandos originales siguen funcionando: los aliases son aditivos.

---

<!-- speaker note: Tabla de aliases concretos. Mostrar cuán natural suena cada uno frente al comando canónico. -->

## Aliases de intención

| Intención | Comando canónico |
|-----------|------------------|
| `setup` / `setup vault` | `init` |
| `check` / `doctor` | `doctor` |
| `find` / `lookup` | `search` |
| `read` / `open entry` | `get` |
| `context project` | `get-context` |
| `projects` / `show projects` | `list-projects` |
| `project start` / `project add` | `add-project` |
| `backup all` | `backup` |
| `setup mcp` / `mcp setup` | `mcp config` |
| `workflow import` / `workflow show` | `import-workflow` / `render-workflow` |

Los comandos canónicos siguen existiendo: los aliases son atajos aditivos.

---

<!-- speaker note: Ayuda conversacional y fuzzy routing. Esto es lo que reduce la fricción: el error ya no es un callejón, sino una puerta con sugerencias. -->

## Ayuda conversacional y fuzzy routing

El `help` ahora se lee como una conversación por tarea, con *Common paths*.

```bash
skillvault help            # agrupado por tarea, con ejemplos
skillvault docs            # atajos de documentación, sin efectos
skillvault help doctor     # ayuda de un comando específico
```

Errores de tipeo tolerados en comandos de solo lectura:

```
$ skillvault docter        #  → resuelve a: doctor
$ skillvault projcts       #  → resuelve a: projects
```

Comandos desconocidos sugieren alternativas accionables:

```
$ skillvault frobnicate
unknown command: frobnicate
Try one of these intent-first commands:
  skillvault find   Find entries with full-text or vector search
  ...
```

---

<!-- speaker note: La cola de pendientes. Un caso de uso concreto que nació de la fricción diaria: guardar trabajo diferido sin romper el flujo. -->

## La cola de pendientes

Capturar trabajo diferido sin romper el flujo. Es solo una entrada más, de tipo `pending` y propósito `WORK`.

```bash
skillvault pending add --project myapp "Update presentation"
skillvault pending review --project myapp            # paseo del proyecto
skillvault pending list --project myapp --query presentation
skillvault pending show update-presentation          # detalle de un ítem
skillvault pending done update-presentation          # resolver
```

- Subcomandos: `add`, `list`/`ls`, `review`, `show`, `done` · alias global `todo`
- Filtros por proyecto y por texto; conteos de ítems activos
- Nada se borra: resolver es un cambio de estado (archived)

---

<!-- speaker note: Lo importante del pending: no es una feature aislada, está integrado al contexto que reciben los agentes. Los pendientes abiertos aparecen al inicio de cada sesión. -->

## Pending integrado al contexto

Los pendientes abiertos entran en el *context pack* que reciben los agentes.

```
$ skillvault context --project myapp --mode planning --include pending

## Active Pending
- [ ] Agregar refresh token rotation
- [ ] Documentar endpoints
```

- Sección **Active Pending** compilada automáticamente en modo planificación
- Filtrable con `--include pending`
- El agente arranca la sesión sabiendo qué falta, sin que se lo expliquen

---

<!-- speaker note: TUI MVP. Deliberadamente liviana: navegar, buscar y resolver pendientes con teclado. No es un kanban. Build tag para no inflar el binario por defecto. -->

## TUI MVP

Una interfaz terminal liviana con Bubble Tea (detrás del build tag `tui`).

```bash
make build-tui
./skillvault-tui tui
```

Superficie actual:

- **Overview de proyectos**
- **Resolución de pendientes con teclado** (confirma antes de archivar)
- **Búsqueda y navegación de entradas** acotadas al proyecto
- **Preview compacto del contexto**

Por diseño no crea ni edita entradas: navegar y resolver es suficiente.

---

<!-- speaker note: MCP. El vault se convierte en herramientas para los agentes. Una sola configuración y el agente lee y escribe memoria como si fuera suya. -->

## MCP: el vault como herramientas del agente

SkillVault es un servidor MCP (JSON-RPC 2.0 sobre stdio) con **24 herramientas**.

```json
{
  "mcpServers": {
    "skillvault": {
      "command": "/home/user/tools/skillvault",
      "args": ["mcp"]
    }
  }
}
```

```bash
skillvault mcp config     # imprime el snippet listo para pegar
```

- El agente **lee** (`get_context`, `search_entries`) y **escribe** (`save_entry`, `session_wrap`) memoria
- **Ejecuta** flujos: `run_workflow`, `route_scenario`
- **Supervisa**: `get_stats`, `list_workflow_runs`, `get_run`
- Ciclo típico: `get_context_bundle` al inicio → `session_wrap` al cerrar

---

<!-- speaker note: Telemetría. Observabilidad local-first de agentes: qué pasó, cuánto costó, sin que los datos salgan de la máquina. El pipeline de seguridad es el corazón. -->

## Telemetría local

Observabilidad local-first para agentes de CLI. Los datos no salen de la máquina.

```
Plugin / wrapper ──socket Unix──▶ telemetryd ──▶ Validación ──▶ SQLite (WAL)
                                             └──▶ Pipeline de seguridad
```

Pipeline de seguridad, por diseño:

- **Hash SHA-256 con salt** de argumentos (nunca se guardan en claro)
- **Redacción por regex**: claves OpenAI, tokens Bearer, headers de auth
- **Escaneo de entropía** base64 → `scanned-warning`
- Detectores de calidad: **loop**, **stall**, **streak**, **token** → `policy.violation`

Plugin nativo para OpenCode: 20 tipos de evento canónicos con `correlation_id`.

---

<!-- speaker note: Principios de diseño. Cuatro decisiones que explican casi todas las features. Son el "por qué" que queda en el público. -->

## Principios de diseño

- **Local-first**: sin nube por defecto, sin daemon, sin servidor de DB. Un binario.
- **Baja fricción**: intención sobre rutas; el CLI se adapta al humano, no al revés
- **Carga cognitiva como métrica**: si hay que memorizar, está mal diseñado
- **Recuperación antes que ejecución**: encontrar el conocimiento correcto primero; el agente ejecuta
- **El contexto es el producto**: no más "contexto de más" ni "contexto de menos"

---

<!-- speaker note: Cómo evolucionó. La historia en cuatro saltos, con el flujo de trabajo como hilo conductor: branch → tests → PR → CI → main. No citar PRs/numbers concretos; el CI va dentro de la narrativa, no como bullet de status. -->

## Cómo evolucionó

De un CLI que guardaba entradas a un sistema de contexto para agentes.

```
v1    → vault local: entradas, proyectos, búsqueda FTS
v2    → HTTP API, MCP, docs, primer pipeline CI (Linux + macOS)
v3    → workflow pipelines, skillvault run, más tools MCP
hoy   → intención sobre rutas: aliases, fuzzy, pending, TUI
```

Cada salto siguió el mismo camino:

```
branch → tests → PR → CI → main
```

- Cambios chicos y revisables, con tests que acompañan el código
- CI verifica en Linux y macOS; cada release deja un binario portable
- La app queda lista desde el PATH, sin daemon ni servidor

```
$ skillvault version
SkillVault v3
```

---

<!-- speaker note: Roadmap. Atado al ciclo de evolución: fricción real → cambio → ciclo. Ser honesto: hay deuda técnica conocida (main.go es un hot-spot) y crecimiento planificado. El vector search ya está en curso. -->

## Lo que sigue

La misma dinámica: fricción real → cambio → ciclo.

- **Refactor del hot-spot** `cmd/skillvault/main.go` (~1.900 líneas) en paquetes cohesivos, para que el próximo cambio sea más barato
- **Mayor riqueza semántica**: el vector search (GloVe, Go puro) ya está activo; el siguiente paso es integrar embeddings a fondo
- **Expansión de la TUI**: hoy navega y resuelve; mañana, crear y editar entradas y gestionar workflows
- **Variables nombradas** en pipelines y **export/import de workflows**
- **Sync opcional y pluggable** (S3-compatible, GitHub Releases)

El núcleo permanece: *encontrar el conocimiento correcto antes de ejecutarlo*.

---

<!-- speaker note: Cierre. Tres mensajes para llevarse. Invitar a probarlo. Links públicos del repositorio. -->

## Conclusiones

- Los agentes no deberían memorizar: deberían **recordar** desde una memoria local compartida
- La **intención sobre la ruta** reduce la carga cognitiva del humano *y* del agente
- **El contexto justo**, compilado con prioridad, es más valioso que todo el contenido guardado

```
skillvault setup && skillvault project start --name "TuProyecto"
```

Repositorio: `github.com/QuantumEdu/kbs` · Documentación en `docs/`

**Preguntas**
