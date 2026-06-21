# Tutorial — Workflow completo

Este tutorial te guía por un flujo real: crear un proyecto, guardar conocimiento, sesionar con contexto y cerrar el ciclo.

## Scenario

Estás arrancando una app nueva llamada "Forense Digital" y querés que SkillVault mantenga la memoria del proyecto para vos y tus agentes.

---

## Paso 1: Inicializar

```bash
skillvault init
```

## Paso 2: Crear el proyecto

```bash
skillvault add-project \
  --name "Forense Digital" \
  --description "App de análisis forense digital con extracción de artefactos"
```

## Paso 3: Guardar skills del dominio

```bash
# Skill: cómo estructurar un análisis forense
skillvault add-entry \
  --title "Forense Digital - Pipeline de Análisis" \
  --type skill \
  --summary "Pipeline de 5 fases: adquisición, extracción, normalización, correlación, reporte" \
  --project forense-digital \
  --tags "forense,pipeline"

# Skill: buenas prácticas de extracción
skillvault add-entry \
  --title "Extracción de Artefactos - Reglas" \
  --type skill \
  --summary "Reglas para extraer artefactos: preservar metadatos, hash cadenas de custodia, priorizar volatilidad" \
  --project forense-digital \
  --tags "forense,extraccion"
```

## Paso 4: Guardar una decisión de arquitectura

```bash
skillvault add-entry \
  --title "Decisión: SQLite vs PostgreSQL" \
  --type decision \
  --summary "Usamos SQLite porque es local-first, no requiere servidor, y el análisis forense es por caso" \
  --content "SQLite permite que cada investigador tenga su propio vault sin infraestructura compartida" \
  --project forense-digital \
  --tags "arquitectura"
```

## Paso 5: Guardar un artefacto pesado

Imaginá que un agente AI te generó un análisis de 500 líneas:

```bash
cat > /tmp/analisis-memoria.txt << 'EOF'
# Análisis de Volcado de Memoria — Caso 2026-006

## Procesos sospechosos
- PID 1337: mint.exe (firmado, pero hash en lista negra)
- PID 2048: svchost.exe (anomalía: ruta no estándar)

## Conexiones de red
- 10.0.0.5:4444 → C2 conocido (malware XYZ)
...
EOF

skillvault save-artifact \
  --title "Análisis de Memoria — Caso 2026-006" \
  --type pdf_analysis \
  --content "$(cat /tmp/analisis-memoria.txt)" \
  --project forense-digital \
  --tags "memoria,malware"
```

## Paso 6: Buscar conocimiento guardado

```bash
# Buscar skills del proyecto
skillvault search "extraccion" --type skill --project forense-digital

# Buscar decisiones
skillvault search "SQLite" --project forense-digital

# Buscar artefactos
skillvault search "memoria" --project forense-digital --type artifact_summary
```

## Paso 7: Obtener contexto para un agente

Antes de arrancar una sesión de trabajo, tu agente necesita contexto:

```bash
skillvault get-context \
  --mode planning \
  --project forense-digital \
  --max-chars 8000
```

Esto devuelve: preferencias del usuario + estado del proyecto + decisiones activas + skills relevantes. El agente arranca sabiendo qué se hizo y qué falta.

## Paso 8: Cerrar una sesión

Después de trabajar, guardás lo que pasó:

```bash
skillvault session-wrap \
  --project forense-digital \
  --summary "Implementamos el módulo de extracción de procesos" \
  --decisions "Usar go-ps para listar procesos,no depender de sysinternals" \
  --pending "Agregar correlación con reglas YARA" \
  --learnings "El parseo de /proc requiere manejo de permisos en Linux"
```

## Paso 9: Ver la continuidad

En la próxima sesión, cuando el agente pida:

```
get_context(mode="planning", project="forense-digital")
```

Va a recibir:

- El resumen de la última sesión
- Las decisiones tomadas
- Los pendientes
- Los skills y artefactos guardados

Sin necesidad de que el usuario explique todo de nuevo.

---

## Resumen del ciclo

```
Inicio de proyecto
  → add-project
  → add-entry (skills, decisiones)
  → save-artifact (outputs largos)
  → get-context (antes de cada sesión)
  → session-wrap (después de cada sesión)
```

SkillVault mantiene la memoria. Los agentes arrancan rápido. El usuario no repite contexto.
