package line

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type CommandExecutor struct {
	Name string
	Args []string
}

func (c CommandExecutor) Run(ctx context.Context, cwd, prompt string) (string, error) {
	if strings.TrimSpace(c.Name) == "" {
		return "", fmt.Errorf("executor command is required")
	}
	cmd := exec.CommandContext(ctx, c.Name, c.Args...)
	cmd.Dir = cwd
	cmd.Stdin = strings.NewReader(prompt)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return stdout.String(), fmt.Errorf("executor %s: %s", c.Name, detail)
	}
	return stdout.String(), nil
}
