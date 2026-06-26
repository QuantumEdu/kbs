# Inicio rápido

## 1. Descargar el binario

```bash
curl -L -o ~/tools/skillvault https://github.com/QuantumEdu/kbs/releases/download/v3.0.0/skillvault-linux-amd64
chmod +x ~/tools/skillvault
export PATH="$HOME/tools:$PATH"
```

Si estás en macOS:

```bash
# Apple Silicon (M1/M2/M3)
curl -L -o ~/tools/skillvault https://github.com/QuantumEdu/kbs/releases/download/v3.0.0/skillvault-darwin-arm64

# Intel
curl -L -o ~/tools/skillvault https://github.com/QuantumEdu/kbs/releases/download/v3.0.0/skillvault-darwin-amd64

chmod +x ~/tools/skillvault
export PATH="$HOME/tools:$PATH"
```

Si estás en Windows (Git Bash / WSL):

```bash
curl -L -o ~/tools/skillvault.exe https://github.com/QuantumEdu/kbs/releases/download/v3.0.0/skillvault-windows-amd64.exe
```

### Build desde fuente

```bash
git clone https://github.com/QuantumEdu/kbs
cd kbs
go build -ldflags="-s -w" -o ~/tools/skillvault ./cmd/skillvault
```

## 2. Inicializar el vault

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

## 3. Crear un proyecto

```bash
skillvault add-project --name "MiApp" --description "Mi primera app"
```

## 4. Guardar un skill

```bash
skillvault add-entry \
  --title "Mi primer skill" \
  --type skill \
  --summary "Esto es un skill reutilizable para el proyecto" \
  --project miapp
```

## 5. Buscar

```bash
skillvault search "skill" --project miapp
```

## 6. Obtener contexto (modo agente)

```bash
skillvault get-context --mode planning --project miapp
```

Devuelve texto estructurado con el estado del proyecto, decisiones activas y acciones sugeridas.

## 7. Cerrar una sesión

```bash
skillvault session-wrap \
  --project miapp \
  --summary "Hoy instalé SkillVault" \
  --decisions "Usar SQLite,no servidor,local-first" \
  --pending "Probar MCP,escribir docs" \
  --learnings "FTS5 necesita tokenizer explícito"
```

## 8. Usar desde un agente AI (MCP)

Agregá a tu `opencode.json`:

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

También podés crear un symlink:

```bash
ln -sf ~/tools/skillvault ~/tools/mcp
# Ahora los agentes pueden llamar "mcp" directamente
# El binario detecta el symlink y entra en modo MCP automáticamente
```

Para verificar que MCP funciona:

```bash
echo '{"jsonrpc":"2.0","id":1,"method":"list_projects","params":{}}' | skillvault mcp
```

## 9. Usar REST API

```bash
skillvault http
```

En otra terminal:

```bash
curl http://127.0.0.1:7438/health
curl -X POST http://127.0.0.1:7438/entries \
  -H 'Content-Type: application/json' \
  -d '{"title":"Test","type":"skill","summary":"desde API"}'
```

## Comandos disponibles

| Comando | Descripción |
|---------|-------------|
| `init` | Inicializar vault |
| `add-entry` | Guardar entrada |
| `search` | Buscar |
| `get` | Obtener entrada por ID |
| `save-artifact` | Guardar artefacto largo |
| `get-context` | Compilar contexto |
| `add-project` | Crear proyecto |
| `list-projects` | Listar proyectos |
| `archive` | Archivar entrada |
| `add-workflow` | Crear workflow |
| `render-workflow` | Ver pasos del workflow |
| `session-wrap` | Cerrar sesión |
| `graph` | Visualizar grafo de relaciones |
| `entry ref` | Gestionar aristas del grafo |
| `memory index` | Indexar archivos .md |
| `export` | Exportar vault |
| `import` | Importar vault |
| `http` | Iniciar servidor REST |

Ver [`docs/commands.md`](commands.md) para la referencia completa con flags.
