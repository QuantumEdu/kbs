package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVaultDir(t *testing.T) {
	dir := vaultDir()
	if dir == "" {
		t.Fatal("vaultDir returned empty")
	}
	if !filepath.IsAbs(dir) {
		t.Errorf("vaultDir should be absolute, got %q", dir)
	}
}

func TestDBPath(t *testing.T) {
	p := dbPath()
	if p == "" {
		t.Fatal("dbPath returned empty")
	}
	if filepath.Base(p) != "vault.db" {
		t.Errorf("dbPath base should be vault.db, got %q", filepath.Base(p))
	}
}

func TestInitCreatesDirectories(t *testing.T) {
	dir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	defer os.Setenv("HOME", origHome)

	RunInit()

	vd := vaultDir()
	for _, sub := range []string{"", "/objects", "/exports", "/cache"} {
		if _, err := os.Stat(vd + sub); os.IsNotExist(err) {
			t.Errorf("directory not created: %s", vd+sub)
		}
	}

	if _, err := os.Stat(dbPath()); os.IsNotExist(err) {
		t.Error("vault.db not created")
	}
}

func TestInitThenOpenVault(t *testing.T) {
	dir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	defer os.Setenv("HOME", origHome)

	RunInit()

	svc := OpenVault()
	if svc.store == nil {
		t.Error("store is nil")
	}
	if svc.entrySvc == nil {
		t.Error("entrySvc is nil")
	}
	if svc.artifactSvc == nil {
		t.Error("artifactSvc is nil")
	}
	if svc.workflowSvc == nil {
		t.Error("workflowSvc is nil")
	}
	if svc.seriesSvc == nil {
		t.Error("seriesSvc is nil")
	}
	if svc.projectSvc == nil {
		t.Error("projectSvc is nil")
	}
	if svc.contextSvc == nil {
		t.Error("contextSvc is nil")
	}
	if svc.sessionSvc == nil {
		t.Error("sessionSvc is nil")
	}
	if svc.exportSvc == nil {
		t.Error("exportSvc is nil")
	}
	if svc.importSvc == nil {
		t.Error("importSvc is nil")
	}
	if svc.saveResultSvc == nil {
		t.Error("saveResultSvc is nil")
	}
	if svc.fileSvc == nil {
		t.Error("fileSvc is nil")
	}
	if svc.scanner == nil {
		t.Error("scanner is nil")
	}
}

func TestInitIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	defer os.Setenv("HOME", origHome)

	RunInit()
	RunInit()
}

func TestResolveQSecretsBin_EnvVar(t *testing.T) {
	tmpDir := t.TempDir()
	mockBin := filepath.Join(tmpDir, "q-secrets")
	if err := os.WriteFile(mockBin, []byte("#!/bin/sh\necho q-secrets mock\n"), 0755); err != nil {
		t.Fatalf("failed to create mock binary: %v", err)
	}

	t.Setenv("Q_SECRETS_BIN", mockBin)
	got := resolveQSecretsBin()
	if got != mockBin {
		t.Errorf("expected %q, got %q", mockBin, got)
	}
}

func TestResolveQSecretsBin_NotFound(t *testing.T) {
	// Use a temp dir as working directory so resolveRepoRoot doesn't find
	// the real kbs repo by walking up from cwd.
	tmpDir := t.TempDir()
	origCwd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origCwd) }()

	t.Setenv("Q_SECRETS_BIN", "")
	t.Setenv("PATH", tmpDir)

	got := resolveQSecretsBin()
	if got != "" {
		t.Errorf("expected empty string when binary not found, got %q", got)
	}
}
