# Quickstart — 5 minutos

## Instalación

```bash
# Requisito: Go 1.26+
git clone https://github.com/QuantumEdu/kbs
cd kbs

# Build (binario único, sin CGO, ~7 MB)
go build -ldflags="-s -w" -o ~/tools/skillvault ./cmd/skillvault

# Agregá ~/tools a tu PATH si no está
export PATH="$HOME/tools:$PATH"
```

## Inicializar el vault

```bash
skillvault init
```

Crea `~/.skillvault/` con:

```
~/.skillvault/
├── vault.db       # SQLite + FTS5
├── objects/       # Artefactos largos (archivos)
├── exports/       # Backups JSON
└── cache/         # Caché temporal
```

## Primeros pasos

### 1. Creá un proyecto

```bash
skillvault add-project \
  --name "MiApp" \
  --description "Mi primera app con SkillVault"
```

### 2. Guardá un skill reutilizable

```bash
skillvault add-entry \
  --title "Code Review Checklist" \
  --type skill \
  --summary "Checklist para revisar PRs" \
  --content "1. Funciona?\n2. Tiene tests?\n3. Maneja errores?" \
  --project miapp \
  --tags "review,pr"
```

### 3. Buscá

```bash
skillvault search "review" --type skill --project miapp
```

### 4. Guardá un artefacto pesado (va al filesystem)

```bash
skillvault save-artifact \
  --title "Análisis de seguridad" \
  --type pdf_analysis \
  --content "$(cat /tmp/reporte-seguridad.md)" \
  --project miapp \
  --tags "seguridad"
```

### 5. Obtené contexto para tu agente

```bash
skillvault get-context \
  --mode planning \
  --project miapp \
  --max-chars 5000
```

Devuelve texto estructurado como:

```
## Scope
Project: MiApp
Mode: planning

## Active Decisions
...

## Suggested Next Action
...
```

### 6. Cerra una sesión con decisiones

```bash
skillvault session-wrap \
  --project miapp \
  --summary "Revisamos el middleware de auth" \
  --decisions "Usar JWT,no sessions" \
  --pending "Agregar refresh token rotation"
```

## Qué sigue?

- [`docs/commands.md`](commands.md) — referencia completa de comandos
- [`docs/mcp.md`](mcp.md) — configurar MCP para Claude Code / OpenCode
- [`docs/tutorial.md`](tutorial.md) — tutorial completo con workflow real
