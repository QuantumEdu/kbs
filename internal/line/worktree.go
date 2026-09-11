package line

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type WorktreeManager interface {
	EnsureWorktree(ctx context.Context, repoPath, jobID string) (string, error)
	CleanupWorktree(ctx context.Context, repoPath, worktreePath string) error
}

type NopWorktreeManager struct{}

func (NopWorktreeManager) EnsureWorktree(ctx context.Context, repoPath, jobID string) (string, error) {
	return repoPath, nil
}

func (NopWorktreeManager) CleanupWorktree(ctx context.Context, repoPath, worktreePath string) error {
	return nil
}

type GitWorktreeManager struct{}

func (m GitWorktreeManager) EnsureWorktree(ctx context.Context, repoPath, jobID string) (string, error) {
	absRepo, err := filepath.Abs(repoPath)
	if err != nil {
		return "", err
	}
	parent := filepath.Dir(absRepo)
	base := filepath.Base(absRepo)
	worktreeRoot := filepath.Join(parent, base+"-worktrees")
	if err := os.MkdirAll(worktreeRoot, 0o755); err != nil {
		return "", fmt.Errorf("create worktrees dir: %w", err)
	}
	wtPath := filepath.Join(worktreeRoot, "job-"+jobID)
	if _, err := os.Stat(wtPath); err == nil {
		return wtPath, nil
	}

	cmd := exec.CommandContext(ctx, "git", "worktree", "add", "--detach", wtPath, "HEAD")
	cmd.Dir = absRepo
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git worktree add: %s: %w", string(out), err)
	}
	return wtPath, nil
}

func (m GitWorktreeManager) CleanupWorktree(ctx context.Context, repoPath, worktreePath string) error {
	if worktreePath == "" || worktreePath == repoPath {
		return nil
	}
	cmd := exec.CommandContext(ctx, "git", "worktree", "remove", "--force", worktreePath)
	cmd.Dir = repoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git worktree remove: %s: %w", string(out), err)
	}
	return nil
}
