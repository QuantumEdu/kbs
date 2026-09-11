package line

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
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

func RenderPhasePrompt(dir string, phase Phase, vars PromptVars) (string, error) {
	name := string(phase) + ".md"
	var raw []byte
	var err error
	if dir != "" {
		raw, err = os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return "", fmt.Errorf("read prompt %s: %w", name, err)
		}
	} else {
		raw, err = defaultPrompts.ReadFile("prompts/" + name)
		if err != nil {
			return "", fmt.Errorf("default prompt %s: %w", name, err)
		}
	}
	tmpl, err := template.New(name).Parse(string(raw))
	if err != nil {
		return "", fmt.Errorf("parse prompt %s: %w", name, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vars); err != nil {
		return "", fmt.Errorf("render prompt %s: %w", name, err)
	}
	return buf.String(), nil
}
