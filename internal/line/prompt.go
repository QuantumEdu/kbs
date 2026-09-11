package line

import (
	"bytes"
	"context"
	"database/sql"
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	_ "modernc.org/sqlite"
)

//go:embed prompts/*.md
var defaultPrompts embed.FS

type PromptVars struct {
	IssueURL    string
	RepoPath    string
	HumanAnswer string
	Summary     string
	CIDetail    string
	PullRequest string
}

var skillvaultPromptReader = defaultSkillVaultPromptReader

func defaultSkillVaultPromptReader(ctx context.Context, phase Phase) (string, error) {
	home, err := os.UserHomeDir()
	if err == nil {
		dbPath := filepath.Join(home, ".skillvault", "vault.db")
		if _, err := os.Stat(dbPath); err == nil {
			db, err := sql.Open("sqlite", dbPath)
			if err == nil {
				defer db.Close()
				var content string
				err := db.QueryRowContext(ctx,
					"SELECT content FROM entries WHERE slug IN (?, ?) AND content != '' LIMIT 1",
					"prompt:line-"+string(phase), "line-"+string(phase),
				).Scan(&content)
				if err == nil && strings.TrimSpace(content) != "" {
					return content, nil
				}
			}
		}
	}

	for _, slug := range []string{"prompt:line-" + string(phase), "line-" + string(phase)} {
		cmd := exec.CommandContext(ctx, "skillvault", "get", slug)
		out, err := cmd.Output()
		if err == nil && len(bytes.TrimSpace(out)) > 0 {
			return string(out), nil
		}
	}

	return "", fmt.Errorf("prompt not found in skillvault")
}

// LoadPrompt resolves the prompt body for a phase from customDir, SkillVault, or embedded defaults.
func LoadPrompt(ctx context.Context, phase Phase, customDir string) (string, error) {
	name := string(phase) + ".md"
	if customDir != "" {
		raw, err := os.ReadFile(filepath.Join(customDir, name))
		if err != nil {
			return "", fmt.Errorf("read prompt %s: %w", name, err)
		}
		return string(raw), nil
	}

	if skillvaultPromptReader != nil {
		if content, err := skillvaultPromptReader(ctx, phase); err == nil && strings.TrimSpace(content) != "" {
			return content, nil
		}
	}

	raw, err := defaultPrompts.ReadFile("prompts/" + name)
	if err != nil {
		return "", fmt.Errorf("default prompt %s: %w", name, err)
	}
	return string(raw), nil
}

func RenderPhasePrompt(dir string, phase Phase, vars PromptVars) (string, error) {
	return RenderPhasePromptWithContext(context.Background(), dir, phase, vars)
}

func RenderPhasePromptWithContext(ctx context.Context, dir string, phase Phase, vars PromptVars) (string, error) {
	name := string(phase) + ".md"
	content, err := LoadPrompt(ctx, phase, dir)
	if err != nil {
		return "", err
	}
	tmpl, err := template.New(name).Parse(content)
	if err != nil {
		return "", fmt.Errorf("parse prompt %s: %w", name, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vars); err != nil {
		return "", fmt.Errorf("render prompt %s: %w", name, err)
	}
	return buf.String(), nil
}
