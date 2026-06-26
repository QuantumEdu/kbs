# SkillVault v3 — Workflow Pipelines Extension Spec

**Documento:** SV-WORKFLOW-001  
**Versión:** v3  
**Estado:** Draft Aprobado para Diseño

## Objetivo

Extender SkillVault para permitir la ejecución secuencial de Entries y Series utilizando la salida de un paso como entrada del siguiente, inspirado parcialmente en Fabric, manteniendo el enfoque principal de SkillVault: recuperación de conocimiento antes que ejecución.

## Principios

### Retrieval First
¿Qué debo ejecutar? antes que ¿Cómo lo ejecuto?

### Simplicidad Operativa
Un flujo debe entenderse leyendo un único archivo.

### Unix Philosophy
Input → Skill → Skill → Skill → Output

### Agnosticismo de Modelo
SkillVault compone contexto y workflows. El LLM o agente externo ejecuta.

## Pipeline

Antes:

```text
Series
Paso 1
Paso 2
Paso 3
```

Después:

```text
Input
 ↓
Paso 1
 ↓
Paso 2
 ↓
Paso 3
 ↓
Output
```

## Variables del Sistema

- `{{input}}`
- `{{previous_output}}`
- `{{final_output}}`

## Workflow Definition

```yaml
workflow:
  id: research_article

steps:
  - id: extract
    entry: extract_wisdom

  - id: summarize
    entry: summarize

  - id: tags
    entry: create_tags
```

## Variables Nombradas (v1-final)

```yaml
steps:
  - id: extract
    entry: extract_wisdom
    output: wisdom

  - id: summarize
    entry: summarize
    input: wisdom
    output: summary

  - id: tags
    entry: create_tags
    input: summary
```

## Persistencia

### runs

- id
- workflow_id
- started_at
- finished_at
- status

### run_steps

- id
- run_id
- step_id
- input
- output
- started_at
- finished_at
- status

## CLI

```bash
skillvault run research_article article.md
```

```bash
skillvault run research_article article.md --output
```

```bash
skillvault run research_article article.md --save result.md
```

## MCP Futuro

### workflow_run

```json
{
  "workflow": "research_article",
  "input": "..."
}
```

## Casos de Uso

### Investigación

PDF → Extract Wisdom → Summarize → Tags

### OpenSpec

Idea → Spec → Plan → Tasks

### Auditoría

Informe → Hallazgos → Clasificación → Resumen Ejecutivo

### Ciberseguridad

CVE → Análisis → Riesgo → Mitigación

## Fuera de Alcance

- DAGs
- Branching
- Loops
- Multi-Agent
- GraphRAG nativo
- Memoria persistente nativa

## Roadmap

### v3

- Pipeline lineal
- previous_output
- run
- run_steps
- CLI run

### v4

- Variables nombradas
- Export/import workflows
- MCP workflow_run

### v5

- Adaptadores externos
- GraphRAG Connector
- Mem0 Connector
- Graphiti Connector
- Cognee Connector

## Decisión Arquitectónica

SkillVault continúa siendo un Knowledge Retrieval System con capacidades básicas de ejecución.

La prioridad estratégica permanece:

```text
Encontrar el conocimiento correcto
antes de ejecutar el conocimiento.
```

## Insight Estratégico

Fabric optimiza ejecución:

```text
Input
 ↓
Prompt
 ↓
Prompt
 ↓
Prompt
```

SkillVault optimiza recuperación:

```text
Knowledge
 ↓
Retrieval
 ↓
Context
 ↓
Execution
```

### Métrica propuesta

TTCK — Time To Correct Knowledge

Tiempo necesario para localizar la skill, workflow o contexto correcto para resolver una tarea.
