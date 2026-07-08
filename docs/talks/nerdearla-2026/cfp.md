# Nerdearla México 2026 — Call for Papers

**Evento:** 18-20 Nov 2026, Expo Reforma CDMX (+ virtual)  
**Formato:** Session 25 min / Workshop 60 min  
**CFP cierra:** 30 Jun 2026  
**Link:** https://sessionize.com/n26mx

---

## Session Title (máx 70 caracteres)

Opciones sin "sistema operativo":

| # | Título | Caracteres |
|---|-------|------------|
| 1 | **SkillVault: retrieval system de conocimiento para agentes AI** | 67 ✅ |
| 2 | **SkillVault: motor de conocimiento contextual para agentes AI** | 68 ✅ |
| 3 | **SkillVault: knowledge base local-first para agentes de IA** | 66 ✅ |
| 4 | **SkillVault: repositorio de contexto inteligente para agentes AI** | 70 ✅ |

**Recomendada → Opción 1:** *SkillVault: retrieval system de conocimiento para agentes AI*

Corto, claro, y "retrieval system" es un término técnico reconocible sin traducirlo.

---

## Description (50-1500 caracteres)

### Español (recomendado — ~1480 caracteres)

Trabajar con agentes de IA para desarrollar software tiene un problema que crece cada día: el contexto. Los prompts y skills quedan dispersos entre chats, archivos .md, repos de GitHub y herramientas diversas. Las decisiones de arquitectura se olvidan entre sesiones. Los outputs largos de IA (análisis, reportes, especificaciones) son valiosos pero contaminan el contexto del agente. Y el contexto que recibe el agente siempre es o muy poco o demasiado.

SkillVault es un sistema de recuperación de conocimiento local-first para agentes de IA, escrito en Go, con un solo binario de 7MB y cero dependencias externas — ni CGO, ni ORM, ni frameworks. Su compilador de contexto Qu@ntum tiene 7 modos que entregan justo lo que el agente necesita, truncando por prioridad si es necesario. Expone 19 herramientas MCP (Model Context Protocol) para que agentes como Claude Code o OpenCode lean y escriban directamente en el vault. Incluye un grafo de relaciones entre entradas con detección de ciclos mediante CTE recursivas en SQLite, un secret scanner integrado, y más de 750 tests.

En esta charla veremos la arquitectura interna: Clean Architecture en Go, 14 paquetes, SQLite + FTS5, y las decisiones de diseño (sin ORM, sin Cobra, sin CGO). Haremos un demo en vivo desde la terminal — init, proyectos, skills, decisiones, contexto compilado, cierre de sesión. Y cerraremos con las lecciones aprendidas sobre por qué el contexto de los agentes de IA necesita ser diseñado como parte fundamental del desarrollo, no un afterthought.

### English (~1450 caracteres)

Working with AI coding agents has a growing problem: context. Prompts and skills scatter across chats, .md files, GitHub repos, and tools. Architecture decisions are forgotten between sessions. Long AI outputs are valuable but pollute the agent's context. And the context the agent receives is always either too little or too much.

SkillVault is a local-first knowledge retrieval system for AI agents, written in Go, delivered as a single 7MB binary with zero external dependencies — no CGO, no ORM, no frameworks. Its Qu@ntum context compiler has 7 modes that deliver exactly what the agent needs, truncating by priority when necessary. It exposes 19 MCP (Model Context Protocol) tools so agents like Claude Code or OpenCode can read and write directly to the vault. It includes an entry relationship graph with cycle detection using SQLite recursive CTEs, a built-in secret scanner, and over 750 tests.

In this talk we'll dive into the internal architecture: Clean Architecture in Go, 14 packages, SQLite + FTS5, and the design decisions behind it (no ORM, no Cobra, no CGO). We'll run a live terminal demo — init, projects, skills, decisions, compiled context, session wrap. And we'll close with lessons learned on why AI agent context needs to be designed as a fundamental part of development — not an afterthought.

---

## Session Format

> **Session** (25 min)

---

## In-person or Virtual

> **[Elegí según corresponda]**
> - In-person (Expo Reforma, 19-20 Nov)
> - Virtual (grabación previa, 18 Nov)

Sin presupuesto de viaje. Si no estás en CDMX, elegí Virtual.

---

## Track

> **Data Science / AI**

Justificación: MCP (Model Context Protocol), Agentes & Protocolos, y Copilots están explícitamente listados como temas que quieren cubrir. SkillVault es un caso de estudio real de MCP funcionando — te diferencia de otras propuestas abstractas.

---

## Level

> **Intermediate**

Asume que sabés qué es un agente de IA y tenés algo de experiencia con desarrollo o tooling, pero no requiere ser experto. El contenido es accesible para la audiencia variada de Nerdearla.

---

## Language

> **Spanish**

---

## Speaker

Tu perfil ya está cargado en Sessionize. Recordá:

- **El abstract NO debe incluir tu nombre, redes ni datos personales** — el proceso es anónimo y si el comité detecta algo que revele tu identidad, puede ser **descalificado**
- Después de la selección (primera semana de septiembre) te contactan por Sessionize

---

## Checklist de verificación

- [ ] Título ≤ 70 caracteres
- [ ] Descripción entre 50 y 1500 caracteres
- [ ] Sin nombre del speaker ni datos personales en el abstract
- [ ] Track: Data Science / AI
- [ ] Level: Intermediate
- [ ] Language: Spanish
- [ ] Formato: Session (25 min)
- [ ] Virtual o In-person según disponibilidad
- [ ] Código de conducta aceptado

---

> **Nota:** Estadísticas actualizadas después del merge de LifeOS Phase 2 (MCP tools: 16 → 19, tests: 400+ → 750+). El texto original del CFP fue enviado con las cifras previas.
