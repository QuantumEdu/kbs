# Análisis del proyecto

> Propuesta CFP para Nerdearla México 2026 basada en SkillVault Qu@ntum — un retrieval system local-first para agentes de IA.

## Dolores o problemas que resuelve

- **Context rot de agentes:** prompts, skills, decisiones y resúmenes viven dispersos entre chats, Markdown, repos y memoria humana.
- **Contexto incorrecto:** el agente recibe demasiado ruido o muy poca señal. SkillVault compila packs por modo y prioridad.
- **Pérdida de continuidad:** las decisiones técnicas se olvidan entre sesiones; `session_wrap` y entradas tipo `decision`/`handoff` mantienen estado.
- **Outputs largos contaminantes:** reportes, specs y análisis son valiosos, pero no deben pegarse completos en prompts. Se guardan como artefactos file-backed.
- **Riesgo de fuga de secretos:** antes de persistir contenido se escanean patrones de API keys, private keys, GitHub tokens y Slack tokens.
- **Tooling pesado:** evita frameworks, ORM y servicios externos; prioriza binario Go portable, SQLite + FTS5 y MCP stdio.
- **Colaboración agente-humano:** expone CLI, MCP, HTTP y TUI para humanos, agentes y automatizaciones.

## Hallazgos desde historial y documentación del proyecto

- El proyecto nació como una base de conocimiento local-first para agentes, pero para CFP conviene evitar “sistema operativo” y usar **“retrieval system de conocimiento”** o **“motor de conocimiento contextual”**.
- Decisiones base: Go, SQLite + FTS5, `modernc.org/sqlite` sin CGO, CLI con stdlib, MCP custom JSON-RPC, SQL migrations embebidas.
- Arquitectura: Clean Architecture light con adapters (`cli`, `mcp`, `api`), app services, domain puro, DB stores, filesystem, compiler de contexto, scanner y vars.
- Evolución reciente: Cloud Sync + TUI, Vector Search, Entry Versioning + Skill Pack Export, Service Hardening, HTTP auth, graceful shutdown, LifeOS Phase 2 (`run_workflow`, `route_scenario`, purpose taxonomy, workflow-builder YAML import) y `save_result` MCP.
- Aprendizaje narrativo fuerte: **el contexto para agentes no es un prompt largo; es infraestructura de conocimiento con reglas, prioridades y seguridad.**
- Punto importante de posicionamiento: SkillVault no necesita presentarse como reemplazo de otros gestores de memoria; puede convivir o integrarse con bitácoras, artefactos SDD, knowledge bases, notas personales y memory managers externos mediante import/export, MCP, CLI o flujos de automatización.
- Datos CFP detectados: Nerdearla México 2026, 18–20 Nov 2026, CFP hasta 30 Jun 2026, charlas de hasta 25 min, track ideal Data Science / AI por RAG, agentes y MCP.

## Hallazgos desde CodeGraph

- Flujo principal: `cmd/skillvault/main.go` parsea comandos; `runMCP()` cablea services hacia `mcp.NewServiceToolRegistry()` y levanta JSON-RPC sobre stdio.
- `EntryService.SaveEntry()` valida tipo/estado, normaliza tags, genera slug estable, evita colisiones, persiste en store y dispara auto-embedding si VectorService está configurado.
- `HermesCompiler` / Qu@ntum compiler arma packs con modos, secciones priorizadas y truncado por presupuesto de caracteres.
- Blast radius destacable: `Compile` tiene cobertura en `internal/context/compiler_test.go`; `Scan` tiene cobertura; TUI `model` y MCP `Tool` aparecen como zonas con menor cobertura directa.
- CodeGraph encontró 214 símbolos en 47 archivos para la exploración arquitectónica; el grafo de llamadas confirma adapters delgados y lógica concentrada en `internal/app`.

## Análisis técnico del proyecto

### Propósito

SkillVault Qu@ntum es una **knowledge base local-first para agentes de IA y developers**. Guarda prompts, skills, workflows, decisiones, contexto de proyecto, outputs largos y relaciones entre entradas, y devuelve contexto listo para inyectar en agentes mediante CLI/MCP.

### Arquitectura general

```text
cmd/skillvault
 ├─ internal/cli      CLI stdlib, sin Cobra
 ├─ internal/mcp      JSON-RPC 2.0 sobre stdio para agentes
 ├─ internal/api      HTTP local con hardening
 ├─ internal/tui      Bubble Tea, build-tag gated
 ├─ internal/app      Use cases: entries, context, sync, vectors, stats, workflows
 ├─ internal/domain   Entidades + validadores puros
 ├─ internal/db       SQLite + FTS5 + migrations + stores
 ├─ internal/files    Artefactos largos en objects/YYYY/MM
 ├─ internal/context  Qu@ntum compiler
 ├─ internal/security Secret scanner
 ├─ internal/sync     GitHub/S3 transports
 └─ internal/vector   GloVe embeddings + similarity
```

### Estadísticas locales

| Métrica | Valor |
|---|---:|
| Archivos versionados analizados | 189 |
| Archivos Go | 116 |
| Líneas Go | 24,785 |
| Markdown/docs/OpenSpec | 56 archivos / 11,262 líneas |
| Paquetes internos | 14 |
| Tests Go | 50 archivos |
| Migrations SQL | 5 |
| MCP tools detectadas | 19 |
| CLI switch cases detectados | 39, incluyendo subcomandos/formatos |
| Verificación local | `go test ./...` ✅ |

### Tecnologías utilizadas

- **Go 1.26+**, single binary.
- **SQLite + FTS5**, driver pure Go `modernc.org/sqlite`.
- **MCP** por JSON-RPC 2.0 sobre stdio.
- **Bubble Tea / Lipgloss** para TUI.
- **S3 y GitHub** como transports de sync.
- **GloVe 300d pure Go** para vector search.
- **OpenSpec, bitácoras y gestores de memoria externos** como fuentes integrables de historial técnico.
- **Secret scanner** propio para prácticas DevSecOps.

### Casos de uso

1. Guardar skills/prompts reutilizables para agentes.
2. Compilar contexto de proyecto antes de planear, implementar o cerrar sesión.
3. Persistir outputs largos de IA como artefactos sin contaminar prompts.
4. Crear grafo de referencias, dependencias, handoffs y decisiones.
5. Detectar secretos antes de guardar conocimiento.
6. Exportar/importar packs de conocimiento reutilizable.
7. Consultar el vault desde Claude Code/OpenCode vía MCP.

### Innovación / diferencial técnico

- No intenta “ser otro Obsidian”; es **runtime de retrieval para agentes**.
- Local-first: no SaaS, no vendor lock-in, no daemon obligatorio.
- Qu@ntum compiler: prioriza y trunca secciones como si fuera un scheduler de contexto.
- Grafo de entradas con detección de ciclos vía CTE recursivas en SQLite.
- Seguridad desde el borde: secret scan antes de persistir.
- Diseño nerd-friendly: Go austero, sin ORM, sin Cobra, sin CGO, tests y terminal-first.

# Investigación sobre Nerdearla

Fuente principal consultada: página oficial de CFP en Sessionize: <https://sessionize.com/n26mx/>. También se usa el sitio del evento referenciado allí: <https://nerdearla.mx/>.

## Cultura y tono

Nerdearla se presenta como el evento gratuito de tecnología, open source y ciencia más grande de Hispanoamérica, con audiencia amplia: desde entusiastas hasta profesionales senior. El tono esperado combina contenido técnico, educativo, inspirador y accionable. No conviene vender humo corporativo; sí conviene contar una historia real, mostrar decisiones, trade-offs, demo y aprendizajes.

## Datos relevantes del CFP 2026

- Evento: **Nerdearla México 2026**.
- Fechas: **18–20 de noviembre de 2026**.
- Lugar: Expo Reforma, Ciudad de México + modalidad online.
- CFP: abre **1 de abril de 2026**, cierra **30 de junio de 2026**.
- Charlas: hasta **25 minutos**, hasta dos ponentes.
- Talleres: hasta **60 minutos**.
- Track natural: **Data Science / AI** por RAG, Agentes & Protocolos, MCP, A2A y Copilots.
- Tracks secundarios: Development, Infrastructure, Security, Open Source, Developer Relations.
- Nota importante: aceptan propuestas redactadas con ayuda de IA, pero descartan copy-paste sin edición humana.

## Enfoque recomendado para Nerdearla

La charla debe sonar a: “me cansé de perder contexto con agentes, construí un vault local-first, y estas son las piezas técnicas que sí funcionaron”. Debe mezclar IA aplicada, Go, SQLite, MCP, DevSecOps y cultura terminal/hacker.

# Propuestas de charla

## Opción 1

**Título:** SkillVault: retrieval system de conocimiento para agentes AI  
**Ángulo:** El problema de contexto en coding agents y cómo diseñar una base local-first con MCP, SQLite, FTS5, vector search y secret scanning.  
**Por qué funciona:** Está alineada con RAG, agentes, MCP, Open Source, Go y cultura hacker. Es demoable y concreta.

## Opción 2

**Título:** De prompts perdidos a memoria local-first para agentes IA  
**Ángulo:** Storytelling más accesible: dolor → arquitectura → demo → lecciones.  
**Por qué funciona:** Más emocional, menos “producto”; buena para audiencia amplia.

## Opción 3

**Título:** MCP sin humo: construyendo tooling real para agentes de IA  
**Ángulo:** Deep dive en MCP, JSON-RPC, herramientas y seguridad.  
**Por qué funciona:** Muy técnico, pero más estrecho; puede competir con muchas charlas MCP.

## Opción elegida

La más fuerte es **Opción 1**, porque combina keyword CFP (“retrieval”, “agentes”, “MCP”), demo real, arquitectura y problemas cotidianos que la audiencia ya siente.

# Presentación recomendada

## Título

**SkillVault: retrieval system de conocimiento para agentes AI**

## Subtítulo

Cómo convertir prompts, decisiones, workflows y outputs largos en contexto seguro, local-first y consumible por agentes vía MCP.

## Abstract

Trabajar con agentes de IA tiene un bug silencioso: el contexto. Prompts, skills y decisiones se dispersan entre chats, archivos y repos; los outputs largos son valiosos pero contaminan el prompt; y cada nueva sesión parece empezar con amnesia parcial.

SkillVault es un retrieval system de conocimiento local-first para agentes AI, escrito en Go como un binario portable. Usa SQLite + FTS5, MCP sobre JSON-RPC stdio, un compilador de contexto llamado Qu@ntum con modos y prioridades, grafo de relaciones con detección de ciclos, vector search, TUI y secret scanner integrado.

En esta charla vamos a diseccionar el problema, revisar la arquitectura, mostrar un demo desde terminal y extraer lecciones prácticas para construir herramientas de IA que no dependan de magia negra ni SaaS obligatorio. Spoiler: a veces el mejor RAG empieza con SQLite, disciplina de contexto y paranoia sana de DevSecOps.

## Público objetivo

Developers, DevOps, builders de tooling interno, personas usando coding agents, gente curiosa de RAG/MCP, fans de terminales, SQLite y automatización local-first.

## Nivel técnico

**Intermedio.** No requiere saber MCP en profundidad, pero ayuda conocer CLI, APIs, bases de datos y conceptos básicos de agentes/RAG.

## Objetivos de aprendizaje

- Entender por qué el contexto de agentes debe diseñarse como infraestructura.
- Ver una arquitectura local-first para retrieval de conocimiento con Go + SQLite + MCP.
- Aprender patrones para compilar contexto por prioridad y presupuesto.
- Identificar riesgos DevSecOps al guardar outputs de IA.
- Llevarse ideas para construir tooling propio sin frameworks pesados.

## Número de diapositivas recomendado

**16 diapositivas para 25 minutos**: 18–20 min de contenido, 3–5 min demo, 2 min cierre/preguntas.

# Guion de la presentación

## Diapositiva 1: Título
- Contenido: título, subtítulo, “Nerdearla México 2026”, tagline: “DB decides. Disk remembers. Qu@ntum delivers.”
- Notas del orador: Abrir con una escena cotidiana: “ayer el agente sabía todo; hoy no recuerda nada”.
- Recomendación de diseño: fondo oscuro, estética terminal, acento verde/cyan, ícono de vault + robot.

## Diapositiva 2: El bug no está en el modelo, está en el contexto
- Contenido: síntomas: prompts perdidos, decisiones olvidadas, outputs gigantes, skills globales, copy-paste infinito.
- Notas: Evitar culpar a la IA; el problema es el sistema operativo social/técnico alrededor del agente.
- Diseño: panel de errores estilo logs.

## Diapositiva 3: Dolor real: memoria de pez dorado con presupuesto de tokens
- Contenido: “demasiado contexto = ruido”, “poco contexto = alucinación operativa”, “sin historia = repetir trabajo”.
- Notas: Conectar con DevOps: si no versionarías infra a mano, ¿por qué versionarías decisiones en chats?
- Diseño: balanza tokens/señal.

## Diapositiva 4: Hipótesis SkillVault
- Contenido: “un retrieval system local-first para agentes”; principios: local-first, portable, seguro, agent-facing, terminal-first.
- Notas: Presentarlo como experimento técnico, no como pitch corporativo.
- Diseño: diagrama de caja negra: humano/agente → SkillVault → contexto.

## Diapositiva 5: Arquitectura high-level
- Contenido: CLI/MCP/HTTP/TUI → app services → domain → SQLite/FTS5 + filesystem + vector store.
- Notas: Clean Architecture light: adapters tontos, use cases expresivos, domain puro.
- Diseño: capas tipo stack retro-futurista.

## Diapositiva 6: Qu@ntum Context Compiler
- Contenido: 7 modos: profile, project, workflow, skill, planning, session_recall, full_brief; prioridades 1–8.
- Notas: Compararlo con un scheduler: no todo entra; entra lo que aporta más señal.
- Diseño: pipeline con secciones que caen por prioridad.

## Diapositiva 7: MCP: el puente para que el agente deje de pedir permiso
- Contenido: JSON-RPC stdio, `tools/list`, `tools/call`, 19 tools detectadas: save/search/get/context/session/ref/graph/compare/save_result/run_workflow/route_scenario.
- Notas: MCP como USB-C para agentes: mismo protocolo, herramientas específicas.
- Diseño: cable neon entre agente y vault.

## Diapositiva 8: SQLite no es “chiquito”: es un dojo
- Contenido: FTS5, CTE recursivas para ciclo, migrations embebidas, in-memory tests, sin CGO.
- Notas: Defender el pragmatismo: antes de meter vector DB distribuida, resuelve el 80% con una base local robusta.
- Diseño: chip SQLite como artefacto hacker.

## Diapositiva 9: Seguridad: no guardes secretos en tu memoria cyborg
- Contenido: scanner OpenAI keys, private keys, GitHub tokens, Slack tokens; reject/redact; DevSecOps shift-left.
- Notas: Un vault de agentes sin secret scanning es un pastebin con ansiedad.
- Diseño: calavera ASCII + candado.

## Diapositiva 10: Grafo de conocimiento
- Contenido: relaciones depends_on, supersedes, part_of, generated_from, handoff_of; detección de ciclos; traversal.
- Notas: El contexto no es una lista, es una red con historia y causalidad.
- Diseño: nodos de colores con flechas.

## Diapositiva 11: Vector search + FTS5: búsqueda híbrida sin nube obligatoria
- Contenido: keyword search, tags, GloVe embeddings, compare-entries, reindex.
- Notas: Explicar que RAG no empieza con hype; empieza con recuperar lo correcto.
- Diseño: dos radares: lexical y semantic.

## Diapositiva 12: Demo plan
- Contenido: `init`, `add-project`, `add-entry`, `save-result`, `get-context`, `graph`, `session-wrap`, MCP call opcional.
- Notas: Tener datos precargados. Si falla internet, no importa: todo local.
- Diseño: checklist de terminal.

## Diapositiva 13: Stats del proyecto
- Contenido: 116 Go files, 24.7k LOC Go, 14 packages, 50 test files, 5 migrations, 19 MCP tools, `go test ./...` pass.
- Notas: Las stats dan credibilidad; no abusar del número, usarlas como prueba de realidad.
- Diseño: dashboard arcade.

## Diapositiva 14: Trade-offs y cicatrices
- Contenido: sin ORM = más SQL; sin framework CLI = más boilerplate; local-first = sync complejo; TUI/MCP requieren tests específicos.
- Notas: A Nerdearla le gusta la honestidad técnica. Mostrar cicatrices suma.
- Diseño: tablero “boss fights”.

## Diapositiva 15: Lecciones para builders de agentes
- Contenido: diseña memoria, separa artefactos largos, prioriza contexto, escanea secretos, instrumenta demo, no sobre-arquitectures temprano.
- Notas: Convertir el proyecto en aprendizajes reutilizables.
- Diseño: tarjetas coleccionables estilo RPG.

## Diapositiva 16: Cierre
- Contenido: “Tu agente no necesita más prompt. Necesita mejor retrieval.” + CTA: construir un vault, medir contexto, compartir tooling.
- Notas: Cerrar con una frase memorable y abrir preguntas.
- Diseño: vault abierto emitiendo luz cyan.

# Ideas para demos, ejemplos y storytelling

- **Opening story:** “Mi agente tenía skills, prompts y decisiones… pero cada sesión era Groundhog Day con tokens.”
- **Demo segura:** usar un proyecto ficticio “CyberTaco API” con decisiones de auth, workflows CI/CD y una key falsa para mostrar rechazo del scanner.
- **Momento wow:** `get-context --mode planning --max-chars 3000` y mostrar cómo el compiler descarta ruido.
- **Momento hacker:** grafo Mermaid de dependencias y detección de ciclo.
- **MCP live:** desde un cliente MCP, pedir “recupera decisiones activas del proyecto”.

# Posibles preguntas del público y respuestas sugeridas

**¿Por qué SQLite y no Postgres/vector DB?**  
Porque el objetivo es local-first, portable y cero daemon. SQLite + FTS5 cubre muchísimo; vector search se puede agregar sin convertir el proyecto en plataforma distribuida.

**¿Esto reemplaza Obsidian/Notion?**  
No. Es agent-facing retrieval. Puede convivir con notas humanas, pero su salida principal es contexto estructurado para agentes.

**¿Qué pasa si se guarda un secreto?**  
El scanner detecta patrones críticos y puede rechazar o redactar antes de persistir. No es DLP enterprise, pero sí un cinturón DevSecOps muy necesario.

**¿Cómo evita contexto basura?**  
Con tipos, estados, tags, proyectos, relaciones, modos de compilación y truncado por prioridad.

**¿MCP es obligatorio?**  
No. CLI y TUI funcionan para humanos. MCP es el puente para agentes.

**¿Qué mejorarías?**  
Más observabilidad de tokens/contexto, cobertura específica de TUI/MCP registry, UI de navegación del grafo y sync conflict resolution más avanzado.

# Recomendaciones finales

- Enviar al track **Data Science / AI** y mencionar MCP/RAG/agentes explícitamente.
- Mantener el abstract humano: no sonar a texto generado sin editar.
- Evitar “sistema operativo”; usar “retrieval system de conocimiento” o “motor de conocimiento contextual”.
- Preparar demo local, sin depender de internet.
- Mostrar trade-offs y tests: Nerdearla premia contenido técnico real, no humo.
- Llevar backup: video corto o asciinema de la demo.

# Fuentes

- Nerdearla México 2026 CFP en Sessionize: <https://sessionize.com/n26mx/>
- Sitio del evento referenciado por el CFP: <https://nerdearla.mx/>
- README del proyecto: `README.md`
- Arquitectura del proyecto: `docs/architecture.md`
- Historial técnico del proyecto `kbs` y documentación local
- CodeGraph sobre `/home/ubuntu/dev/kbs`
