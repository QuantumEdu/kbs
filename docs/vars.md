# Variable Detection & Injection

SkillVault supports template-style variables in entry content, letting you reuse entries with **dynamic values** at output time.

## Frontmatter

Any entry can have YAML-like frontmatter wrapped in `---` delimiters at the top of its body content:

```
---
type: prompt
version: 1.0
tags: [go, testing]
---

Content starts here...
```

Frontmatter is **extracted and parsed** from the body on save. It is NOT stored as part of the body. The parsed key-value pairs are available for variable injection.

## Variable Syntax

Use `{{variable}}` in any entry body: `summary`, `body`, or `instruction` fields. When rendering or outputting the entry, SkillVault replaces these placeholders with provided values.

**Example:**

Body content:
```
You are a {{role}} expert. The user is working on {{project}}.
Your task: {{description}}
```

With `--vars role=Go,project=SkillVault,description=review architecture`:

```
You are a Go expert. The user is working on SkillVault.
Your task: review architecture
```

## Using Variables

### CLI

```bash
# Pass variables inline
skillvault get-context --project myapp --vars "role=Go,project=SkillVault"

# Pass variables from file
skillvault get-context --project myapp --vars @vars.txt

# In-place replace (modifies the entry in the vault)
skillvault get-context --project myapp --vars "name=test" -i
```

The `--vars` flag accepts:
- **Inline**: `key1=value1,key2=value2`
- **From file**: `@path/to/vars.txt` (one `key=value` per line)
- **JSON**: `{"key": "value"}`

### Replace Mode

| Mode | Flag | Behavior |
|------|------|----------|
| **Inline** (default) | none | Variables replaced in the output copy only. Original entry unchanged. |
| **In-place** | `-i` | Variables replaced and saved back to the vault entry. **Irreversible.** |

## Use Cases

- **Reusable prompts**: `"Review the {{file}} for {{language}} issues"`
- **Dynamic context**: `"The project {{project}} has {{n}} open items"`
- **Templated skills**: `"Use {{framework}} to implement {{feature}}"`

## Implementation

The `internal/vars/` package handles all variable operations:

| Component | File | Purpose |
|-----------|------|---------|
| Detector | `detect.go` | Finds `{{variable}}` patterns in content |
| Frontmatter parser | `frontmatter.go` | Extracts `---` blocks from body content |
| Resolver | `resolver.go` | Replaces variables with provided values |

Variables use `strings.ReplaceAll` for substitution. No nesting, no expressions, no piped transforms.
