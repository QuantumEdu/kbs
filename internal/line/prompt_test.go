package line

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderPlanPromptFromDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	body := "Issue: {{.IssueURL}}\nRepo: {{.RepoPath}}\nAnswer: {{.HumanAnswer}}\n"
	if err := os.WriteFile(filepath.Join(dir, "plan.md"), []byte(body), 0o600); err != nil {
		t.Fatalf("write prompt: %v", err)
	}
	got, err := RenderPhasePrompt(dir, PhasePlan, PromptVars{
		IssueURL:    "https://github.com/o/r/issues/9",
		RepoPath:    "/repo",
		HumanAnswer: "public",
	})
	if err != nil {
		t.Fatalf("RenderPhasePrompt: %v", err)
	}
	if !strings.Contains(got, "issues/9") || !strings.Contains(got, "public") {
		t.Fatalf("unexpected prompt %q", got)
	}
}

func TestRenderPlanPromptMissingFile(t *testing.T) {
	t.Parallel()
	_, err := RenderPhasePrompt(t.TempDir(), PhasePlan, PromptVars{})
	if err == nil {
		t.Fatal("expected missing prompt error")
	}
}

func TestDefaultPlanPromptRenders(t *testing.T) {
	t.Parallel()
	got, err := RenderPhasePrompt("", PhasePlan, PromptVars{
		IssueURL: "https://github.com/o/r/issues/3",
		RepoPath: "/repo",
	})
	if err != nil {
		t.Fatalf("default prompt: %v", err)
	}
	if !strings.Contains(got, "issues/3") || !strings.Contains(got, `"status"`) {
		t.Fatalf("default prompt missing contract: %q", got)
	}
}

func TestLoadPromptFromSkillVault(t *testing.T) {
	orig := skillvaultPromptReader
	t.Cleanup(func() { skillvaultPromptReader = orig })

	skillvaultPromptReader = func(_ context.Context, phase Phase) (string, error) {
		if phase == PhasePlan {
			return "Custom vault prompt for {{.IssueURL}}", nil
		}
		return "", errors.New("not found")
	}

	got, err := LoadPrompt(context.Background(), PhasePlan, "")
	if err != nil {
		t.Fatalf("LoadPrompt: %v", err)
	}
	if got != "Custom vault prompt for {{.IssueURL}}" {
		t.Fatalf("unexpected prompt: %q", got)
	}
}

func TestLoadPromptFallbackToEmbedded(t *testing.T) {
	orig := skillvaultPromptReader
	t.Cleanup(func() { skillvaultPromptReader = orig })

	skillvaultPromptReader = func(_ context.Context, _ Phase) (string, error) {
		return "", errors.New("vault unavailable")
	}

	got, err := LoadPrompt(context.Background(), PhasePlan, "")
	if err != nil {
		t.Fatalf("LoadPrompt: %v", err)
	}
	if !strings.Contains(got, `"status"`) || !strings.Contains(got, `"summary"`) {
		t.Fatalf("expected embedded fallback, got: %q", got)
	}
}
