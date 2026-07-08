# SkillVault: Sistema Operativo de Conocimiento Local-First para Agentes AI

**Presentación para Nerdearla México 2026**  
Duración: 25 min — Track: Development / AI & Data Science  
Speaker: [Anónimo para CFP]

---

## Slide 1 — Portada

```
+-----------------------------------------------+
|                                               |
|   ╻ ╻┏━┓┏┓╻┏━┓┏━┓┏━┓┏━┓┏┓┏┓                 |
|   ┃┃┃┣━┫┃┗┫┣━┫┃ ┃┃ ┃┣━┫┃┃┃┃                 |
|   ┗┻┛╹ ╹╹ ╹╹ ╹┗━┛┗━┛╹ ╹╹┗┛┗┛                 |
|   Local-first knowledge OS for AI agents       |
|                                               |
|   SkillVault Qu@ntum                          |
|   Nerdearla México 2026                       |
|                                               |
+-----------------------------------------------+
```

**Speaker notes:**
- "Buenos días/seres — welcome to Nerdearla México"
- "Vengo a contarles una historia sobre un problema que cada vez duele más, y cómo lo resolvimos con Go, SQLite, y cero frameworks."

---

## Slide 2 — The Pain (3 min)

```
+-----------------------------------------------+
|                                               |
|   😤 El problema que todos conocemos          |
|                                               |
|   ✕ Prompts y skills:                        |
|     • En chats de ChatGPT/Claude              |
|     • En archivos .md perdidos                |
|     • En repos de GitHub que nunca leemos     |
|                                               |
|   ✕ Outputs largos de AI:                    |
|     • Análisis de PDFs, specs, reportes       |
|     • Valiosos... pero contaminan contexto     |
|                                               |
|   ✕ Decisiones de arquitectura:              |
|     • "¿Por qué elegimos SQLite?"             |
|     • "¿Qué decidimos en la sesión pasada?"   |
|                                               |
|   ✕ Workflows:                                |
|     • "Siempre me olvido del paso 3"          |
|     • Checklists que existen en la cabeza     |
|                                               |
+-----------------------------------------------+
```

**Speaker notes:**
- "Levanten la mano si alguna vez perdieron un prompt increíble que hicieron"
- "El contexto de los agentes AI es como un buffer: o rebalsa o no alcanza nunca"
- "Este problema NO es de falta de herramientas. Es de **arquitectura de conocimiento**"
- Transición: "Así que decidimos construir algo al respecto..."

---

## Slide 3 — SkillVault: La Visión (2 min)

```
+-----------------------------------------------+
|                                               |
|   💡 SkillVault                               |
|                                               |
|   "Local-first knowledge OS                   |
|    for developers and AI agents."             |
|                                               |
|   ┌───────────────────────────────────────┐  |
|   │                                       │  |
|   │   DB decides.                         │  |
|   │   Disk remembers.                     │  |
|   │   Qu@ntum delivers.                   │  |
|   │                                       │  |
|   └───────────────────────────────────────┘  |
|                                               |
|   • Un solo binario: 7MB                      |
|   • Sin servidor, sin daemon                  |
|   • Go 1.26, zero CGO                         |
|   • 750+ tests                                |
|   • MIT License                               |
|                                               |
+-----------------------------------------------+
```

**Speaker notes:**
- "Tres principios: la DB decide qué existe y cómo buscarlo. El disco recuerda los contenidos largos. El compilador Qu@ntum entrega el contexto justo."
- "Sin frameworks. Sin ORM. Sin Docker. Sin Kubernetes. Sin nada que no sea Go puro y SQLite."
- "El binario cabe en un Slack attachment."

---

## Slide 4 — Clean Architecture en Go (4 min)

```
+-----------------------------------------------+
|                                               |
|   🧱 Arquitectura                             |
|                                               |
|   cmd/skillvault/main.go                      |
|       │                                       |
|       ├── internal/cli/     → stdlib flags    |
|       ├── internal/mcp/     → JSON-RPC 2.0    |
|       ├── internal/api/     → HTTP REST       |
|       │                                       |
|       ├── internal/app/     → Use cases       |
|       ├── internal/domain/  → Pure entities   |
|       │                                       |
|       ├── internal/db/      → SQLite + FTS5   |
|       ├── internal/files/   → Filesystem      |
|       ├── internal/context/ → Qu@ntum         |
|       ├── internal/security/→ Secret scanner  |
|       ├── internal/sync/    → S3 + GitHub     |
|       ├── internal/vars/    → Var injection   |
|       ├── internal/vector/  → Cosine search   |
|       └── internal/diff/    → Entry diffing   |
|                                               |
|   Domain NO importa nada del proyecto.        |
|   App orquesta domain + stores.               |
|   Adapters (CLI/MCP/HTTP) llaman a App.       |
|                                               |
+-----------------------------------------------+
```

**Speaker notes:**
- "14 paquetes internos. Cada uno con su responsabilidad bien definida."
- "La regla de oro: `internal/domain/` no sabe que SQLite existe. No sabe que JSON existe. Es Go puro con validadores."
- "Capa App es donde vive la orquestación: SaveEntryService corre validación → scanner de secretos → guarda en store."
- "Tres formas de hablarle: CLI (21 comandos con `flag` de stdlib), MCP (19 herramientas JSON-RPC), HTTP API (REST). Todas llaman a los mismos services."
- "¿Por qué sin Cobra? Porque `flag` de stdlib alcanza para comandos planos y nos ahorramos una dependencia."

---

## Slide 5 — Decisiones de Diseño (2 min)

```
+-----------------------------------------------+
|                                               |
|   ⚡ Decisiones de Diseño                     |
|                                               |
|   ┌──────────────────────────────────────┐   |
|   │ Sin ORM → SQLite queries simples     │   |
|   │ Sin CGO → modernc.org/sqlite         │   |
|   │ Sin Cobra → stdlib flag alcanza      │   |
|   │ Slugs como IDs → legibles, estables  │   |
|   │ Soft delete → archived excluye ctx   │   |
|   │ FTS5 standalone → in-memory en tests │   |
|   │ SHA256 → integridad sin git          │   |
|   └──────────────────────────────────────┘   |
|                                               |
|   Dependencias externas:                      |
|   • modernc.org/sqlite (pure Go SQLite)       |
|   • (Opcional: Bubble Tea para TUI)           |
|                                               |
+-----------------------------------------------+
```

**Speaker notes:**
- "Cada decisión está documentada en `docs/architecture.md` con su razón técnica."
- "La dependencia más controversial: decidimos NO tener ORM porque las queries SQLite de SkillVault son simples y el overhead de GORM o similar no aporta nada — solo complejidad y magia."
- "FTS5 standalone significa que podemos correr tests en memoria sin necesidad de archivos físicos."

---

## Slide 6 — El Vault por Dentro (3 min)

```
+-----------------------------------------------+
|                                               |
|   🗄️ Storage Layout                          |
|                                               |
|   ~/.skillvault/                              |
|   ├── vault.db        ← SQLite + FTS5        |
|   ├── objects/        ← Artifacts on disk    |
|   │   └── 2026/06/                           |
|   │       └── analisis-seguridad.md          |
|   ├── exports/        ← Backups JSON         |
|   └── cache/          ← Temp cache           |
|                                               |
|   Entity Model:                               |
|   • 11 Entry Types (prompt, skill, decision,  |
|     session, handoff, reference, ...)         |
|   • 5 Statuses (draft, active, archived,      |
|     deprecated, canonical)                    |
|   • 11 Relation Types (references, part_of,   |
|     depends_on, implements, ...)              |
|                                               |
+-----------------------------------------------+
```

**Speaker notes:**
- "11 tipos de entrada cubren todo el ciclo de vida del conocimiento: desde prompts reutilizables hasta sesiones completas con decisiones."
- "5 estados permiten granularidad: draft para lo que está cocinándose, active para producción, canonical para la versión de verdad."
- "Las relaciones entre entradas forman un grafo dirigido con detección de ciclos — no podés crear una dependencia circular."

---

## Slide 7 — El Grafo de Conocimiento (2 min)

```
+-----------------------------------------------+
|                                               |
|   🕸️ Entry Relationship Graph                |
|                                               |
|   ┌──────────────┐                            |
|   │  API Design  │──depends_on──▶ SQL Schema  |
|   └──────┬───────┘                            |
|          │                                    |
|     implements                                |
|          │                                    |
|          ▼                                    |
|   ┌──────────────┐    references    ┌──────┐ |
|   │ Auth Handler  │───────────────▶│ JWT  │ |
|   └──────────────┘                 │ Spec │ |
|                                     └──────┘ |
|                                               |
|   11 tipos de relación:                       |
|   references, supersedes, related_to,         |
|   part_of, derived_from, implements,          |
|   uses, extends, handoff_of,                  |
|   generated_from, depends_on                  |
|                                               |
|   Detección de ciclos con CTE:               |
|   WITH RECURSIVE reachable AS (               |
|     SELECT ... UNION ALL SELECT ...           |
|   )                                           |
|                                               |
+-----------------------------------------------+
```

**Speaker notes:**
- "11 tipos de relación. Cada una tiene semántica distinta y algunas tienen protección de ciclos."
- "La detección de ciclos usa una CTE recursiva de SQLite. Si A depende de B y B depende de A, la CTE lo detecta antes de insertar."
- "`depends_on`, `part_of` y `supersedes` son las que validan contra ciclos — porque crean dependencias transitivas."
- "El CLI puede renderizar el grafo como Mermaid, JSON o DOT."

---

## Slide 8 — Qu@ntum Context Compiler (3 min)

```
+-----------------------------------------------+
|                                               |
|   🧠 Qu@ntum Context Compiler                |
|                                               |
|   7 modos de compilación:                     |
|                                               |
|   profile       → feedback + preferencias     |
|   project       → estado + decisiones         |
|   workflow      → pasos del pipeline          |
|   skill         → skills + prompts activos    |
|   planning      → TODO combinado             |
|   session_recall→ últimas sesiones            |
|   full_brief    → TODO                        |
|                                               |
|   Prioridad de secciones (1 = más importante):|
|                                               |
|   1. User Preferences                         |
|   2. Project State                            |
|   3. Active Decisions                         |
|   4. Current Workflow                         |
|   5. Recent Sessions                          |
|   6. Skills & Prompts                         |
|   7. Artifact Summaries                       |
|   8. References                               |
|                                               |
|   Si excede max_chars → trunca de abajo      |
|                                               |
+-----------------------------------------------+
```

**Speaker notes:**
- "La innovación real: no es solo guardar cosas, es **entregar el contexto correcto** en el momento correcto."
- "7 modos para 7 situaciones distintas. Si arrancás una sesión de planning, no necesitas saber los 15 skills instalados — necesitas el estado del proyecto y las decisiones activas."
- "El truncado inteligente elimina secciones de menor prioridad primero. El agente nunca recibe más de `max_chars`."
- "El output es texto plano estructurado, listo para inyectar directo en el prompt del agente."

---

## Slide 9 — MCP: La Magia (4 min)

```
+-----------------------------------------------+
|                                               |
|   🔌 MCP — Model Context Protocol            |
|                                               |
|   19 herramientas sobre stdio JSON-RPC 2.0    |
|                                               |
|   ┌──────────────────────────────────────┐   |
|   │ save_entry       search_entries     │   |
|   │ get_entry        save_artifact      │   |
|   │ save_result      get_context        │   |
|   │ compose_series   render_workflow    │   |
|   │ session_wrap     archive_entry      │   |
|   │ list_projects    search_by_tags     │   |
|   │ run_workflow     route_scenario     │   |
|   │ get_context_bundle                  │   |
|   │ save_entry_ref   list_entry_refs    │   |
|   │ get_entry_graph                     │   |
|   └──────────────────────────────────────┘   |
|                                               |
|   Flujo típico del agente:                    |
|                                               |
|   1. get_context_bundle(project)              |
|   2. save_entry(title, type, summary)         |
|   3. search_by_tags(tags=["go","tdd"])        |
|   4. session_wrap(summary, decisions)         |
|                                               |
+-----------------------------------------------+
```

**Speaker notes:**
- "MCP es el protocolo que permite que Claude Code, OpenCode, Cline y otros agentes hablen directamente con herramientas externas."
- "SkillVault expone 19 herramientas MCP. El agente puede guardar conocimiento, buscarlo, obtener contexto, cerrar sesiones — todo desde su propio flujo."
- "El setup es trivial: agregás 5 líneas al JSON de configuración MCP."
- "El flujo completo: el agente arranca pidiendo contexto, trabaja, guarda lo que aprende, y al cerrar la sesión persiste las decisiones. Todo sin intervención del humano."

---

## Slide 10 — Live Demo (6 min)

```
+-----------------------------------------------+
|                                               |
|   🎬 Demo en Vivo                            |
|                                               |
|   ▶ Paso 1: Init                             |
|     $ skillvault init                         |
|                                               |
|   ▶ Paso 2: Crear proyecto                   |
|     $ skillvault add-project \                |
|         --name "Nerdearla App" \              |
|         --description "App de ejemplo"        |
|                                               |
|   ▶ Paso 3: Guardar un skill                 |
|     $ skillvault add-entry \                  |
|         --title "Go Testing Patterns" \       |
|         --type skill \                        |
|         --summary "Table-driven tests + fuzz" |
|                                               |
|   ▶ Paso 4: Guardar una decisión             |
|     $ skillvault add-entry \                  |
|         --title "Decisión: SQLite local" \    |
|         --type decision \                     |
|         --summary "Local-first sin servidor"  |
|                                               |
|   ▶ Paso 5: get-context (modo planning)      |
|     $ skillvault get-context \                |
|         --mode planning \                     |
|         --project nerdearla-app               |
|                                               |
|   ▶ Paso 6: Cerrar sesión                    |
|     $ skillvault session-wrap \               |
|         --project nerdearla-app \             |
|         --summary "Creamos estructura"        |
|                                               |
+-----------------------------------------------+
```

**Speaker notes:**
- "Voy a hacer esto en vivo desde mi terminal. NO es grabado."
- "Arrancamos de cero: `skillvault init` crea todo el vault en milisegundos."
- "Cada comando muestra feedback claro — esto es Go con stdlib, sin magia."
- "Al final, `get-context` devuelve texto estructurado listo para pegarle a un agente."
- "Si el tiempo lo permite, muestro cómo un agente AI (Claude Code con el MCP configurado) puede hacer todo esto automáticamente."
- *Backup plan: si la terminal falla, tengo capturas de cada paso*

---

## Slide 11 — Secret Scanner + Pipeline (1 min)

```
+-----------------------------------------------+
|                                               |
|   🔒 Seguridad ante todo                     |
|                                               |
|   4 patrones de secretos detectados:          |
|   • OpenAI API Keys: sk-...                   |
|   • Private Keys: -----BEGIN PRIVATE KEY----- |
|   • GitHub Tokens: ghp_...                   |
|   • Slack Tokens: xoxb-...                   |
|                                               |
|   Modos: Scan (rechaza) o Redact (limpia)    |
|                                               |
|                                               |
|   ⚙️ Workflow Pipelines (v3)                 |
|                                               |
|   Ejecución secuencial de steps:              |
|   Paso 1: Inyecta {{input}} + {{prev_out}}    |
|   Paso 2: Muestra prompt por stdout           |
|   Paso 3: Espera respuesta del agente        |
|   Paso 4: Pasa al siguiente step             |
|                                               |
+-----------------------------------------------+
```

**Speaker notes:**
- "Los secretos se detectan antes de entrar al vault. OpenAI keys, tokens de GitHub, Slack, claves privadas."
- "Dos modos: Scan rechaza la entrada con un warning. Redact reemplaza con `[REDACTED]` y permite guardar."
- "Workflow pipelines: ejecutás un pipeline de pasos secuenciales donde cada paso recibe el output del anterior. Ideal para procesos multi-paso como 'investigar → escribir spec → generar código'."
- "Cada paso inyecta variables (`{{input}}`, `{{previous_output}}`), muestra el prompt, espera la respuesta del agente, y pasa al siguiente."

---

## Slide 12 — Open Source + Go Ecosystem (1 min)

```
+-----------------------------------------------+
|                                               |
|   🌍 Open Source · MIT · Hecho en Go         |
|                                               |
|   github.com/QuantumEdu/kbs                  |
|                                               |
|   ┌──────────────────────────────────────┐   |
|   │  Stack:                              │   |
|   │  • Go 1.26 (toolchain estándar)      │   |
|   │  • SQLite via modernc.org/sqlite     │   |
|   │  • Bubble Tea (TUI, opcional)        │   |
|   │  • MinIO (S3 sync, opcional)         │   |
|   │  • go-github (GitHub sync, opcional) │   |
|   │  • 0 frameworks                      │   |
|   └──────────────────────────────────────┘   |
|                                               |
|   Build: go build -o skillvault ./cmd/...     |
|   Tests: go test ./...  (750+ tests)         |
|   Tamaño: ~7MB (stripped)                    |
|                                               |
+-----------------------------------------------+
```

**Speaker notes:**
- "El proyecto es 100% open source, MIT. No hay versión enterprise, no hay SaaS, no hay trampa."
- "Dependencias externas mínimas y todas opcionales. El core del vault corre con UNA dependencia: modernc.org/sqlite."
- "Si no necesitás sync S3 o TUI, ni siquiera se compilan. Build tags para mantener el binario chico."
- "Go fue la elección obvia: compilación cruzada, sin runtime, binario static, tipado fuerte."

---

## Slide 13 — Lessons Learned (1 min)

```
+-----------------------------------------------+
|                                               |
|   📚 Lessons Learned                         |
|                                               |
|   ✅ SQLite + FTS5 es suficiente             |
|      para búsqueda estructurada + full-text  |
|                                               |
|   ✅ Clean Architecture funciona en Go       |
|      sin frameworks — interfaces + paquetes  |
|                                               |
|   ✅ MCP es el protocolo correcto            |
|      para integración con agentes AI         |
|                                               |
|   ✅ El contexto necesita ser explícito      |
|      no implícito — el compilador lo fuerza  |
|                                               |
|   ❌ No subestimar el ciclo de feedback      |
|      entre lo que el agente necesita y       |
|      lo que el vault entrega                 |
|                                               |
+-----------------------------------------------+
```

**Speaker notes:**
- "Lo que aprendimos construyendo esto: SQLite con FTS5 bancó todo — búsqueda estructurada + full-text + relaciones + tags en una sola DB."
- "Clean Architecture en Go puro funciona y es liberador. Interfaces chicas en el lugar correcto."
- "MCP es EL protocolo para integración agente-herramienta. Si están construyendo tools para AI agents, usen MCP."
- "El mayor aprendizaje: el contexto no es un problema de storage. Es un problema de **curaduría**. No se trata de guardar todo, se trata de entregar lo justo."

---

## Slide 14 — Cierre + Q&A (1 min)

```
+-----------------------------------------------+
|                                               |
|   🎯 Takeaways                               |
|                                               |
|   1. El contexto de los agentes AI           |
|      no es opcional — hay que diseñarlo     |
|                                               |
|   2. Local-first > cloud-first               |
|      para tooling de desarrollo              |
|                                               |
|   3. Go + SQLite es un combo imbatible       |
|      para herramientas de terminal           |
|                                               |
|   4. MCP está cambiando cómo los agentes     |
|      interactúan con el mundo real           |
|                                               |
|                                               |
|   🔗 github.com/QuantumEdu/kbs               |
|   🐦 @nerdearla                               |
|                                               |
|   ¿Preguntas?                                 |
|                                               |
+-----------------------------------------------+
```

**Speaker notes:**
- "Cuatro takeaways para que se lleven a casa."
- "El más importante: si están usando AI agents para codeo, **el contexto no es opcional**. Hay que diseñar el sistema de conocimiento tanto como el código."
- "Local-first importa porque tu conocimiento no debería depender de un servidor que no controlás."
- "Go + SQLite: si están construyendo tooling, este combo les da binarios portables de 7MB. No hay excusa."
- "MCP no es una moda — es un protocolo que va a estar en todos lados el año que viene."
- "Gracias Nerdearla, gracias México, gracias Sysarmy. Preguntas →"

---

## 🎬 Apéndice Técnico — Para el Presentador

### Setup del Demo (verificar ANTES)

```bash
# 1. Buildear el binario
go build -o /tmp/skillvault-demo ./cmd/skillvault

# 2. Limpiar estado anterior
rm -rf ~/.skillvault/

# 3. Tener los comandos en un script de respaldo
cat > /tmp/demo-script.sh << 'SCRIPT'
#!/bin/bash
SK=~/tools/skillvault

echo "=== 1. INIT ==="
$SK init

echo "=== 2. ADD PROJECT ==="
$SK add-project --name "Nerdearla App" --description "App de ejemplo para la charla"

echo "=== 3. ADD SKILL ==="
$SK add-entry \
  --title "Go Testing Patterns" \
  --type skill \
  --summary "Table-driven tests + property-based testing + fuzzing" \
  --project nerdearla-app \
  --tags "go,testing"

echo "=== 4. ADD DECISION ==="
$SK add-entry \
  --title "Decisión: SQLite local" \
  --type decision \
  --summary "Usamos SQLite como storage local-first. Sin servidor, sin cloud." \
  --content "SQLite permite que cada dev tenga su vault sin infraestructura compartida. Sync opcional via S3/GitHub." \
  --project nerdearla-app \
  --tags "arquitectura,storage"

echo "=== 5. SAVE ARTIFACT ==="
$SK save-artifact \
  --title "Análisis de Seguridad" \
  --type pdf_analysis \
  --content "$(cat /tmp/analisis-seguridad.md 2>/dev/null || echo 'Reporte demo')" \
  --project nerdearla-app

echo "=== 6. GET CONTEXT ==="
$SK get-context --mode planning --project nerdearla-app --max-chars 3000

echo "=== 7. SEARCH ==="
$SK search "go" --project nerdearla-app

echo "=== 8. SESSION WRAP ==="
$SK session-wrap \
  --project nerdearla-app \
  --summary "Preparamos demo para Nerdearla México 2026" \
  --decisions "SQLite como storage,Clean Architecture en Go,MCP para agentes" \
  --pending "Agregar vector search,Mejorar sync" \
  --learnings "FTS5 necesita tokenizer explícito,las CTE recursivas son lentas con depth > 10"
SCRIPT
```

### Backup Plan

| Si falla... | Hacer... |
|-------------|----------|
| Terminal no muestra colores | Usar `SCRIPT` sin formato |
| `skillvault init` ya existe | `rm -rf ~/.skillvault/ && skillvault init` |
| MCP no conecta | Mostrar captura de pantalla |
| Tiempo justo | Saltar Slide 11 (secret scanner) |

### Timing Estricto

| Slide | Tiempo | Acumulado |
|-------|--------|-----------|
| 1. Portada | 0:30 | 0:30 |
| 2. The Pain | 3:00 | 3:30 |
| 3. La Visión | 1:30 | 5:00 |
| 4. Arquitectura | 4:00 | 9:00 |
| 5. Decisiones | 1:30 | 10:30 |
| 6. El Vault | 2:00 | 12:30 |
| 7. El Grafo | 1:30 | 14:00 |
| 8. Qu@ntum | 2:30 | 16:30 |
| 9. MCP | 2:00 | 18:30 |
| 10. DEMO | 4:00 | 22:30 |
| 11. Seguridad | 0:30 | 23:00 |
| 12. Open Source | 0:30 | 23:30 |
| 13. Lessons | 1:00 | 24:30 |
| 14. Cierre + Q&A | 0:30 | 25:00 |

---

## 📌 Checklist Pre-Charla

- [ ] Binario compilado y funcionando en la máquina de la charla
- [ ] `~/.skillvault/` limpiado (`rm -rf ~/.skillvault/`)
- [ ] Script de backup listo en `/tmp/demo-script.sh`
- [ ] Capturas de pantalla de cada paso por si el demo falla
- [ ] Tamaño de fuente de terminal visible desde la última fila
- [ ] Sin nombres de usuario visibles en el prompt de la terminal
- [ ] Prueba de conectividad a internet (por si muestro MCP en vivo)
- [ ] Slides exportadas a PDF + PPTX (compatibilidad)
- [ ] Speaker notes impresas o en segunda pantalla
- [ ] Reloj visible para mantener timing
