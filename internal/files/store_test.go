package files

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteReadArtifact(t *testing.T) {
	tmp := t.TempDir()
	svc, err := NewArtifactFileService(tmp)
	if err != nil {
		t.Fatal(err)
	}

	content := []byte("# Hello\n\nTest artifact.")
	relPath, hash, size, err := svc.WriteArtifact("test-artifact", content, "text/markdown")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(relPath, "/") || !strings.HasSuffix(relPath, ".md") {
		t.Errorf("unexpected path: %s", relPath)
	}

	if hash != HashContent(content) {
		t.Errorf("hash = %q, want %q", hash, HashContent(content))
	}
	if size != int64(len(content)) {
		t.Errorf("size = %d, want %d", size, len(content))
	}

	got, err := svc.ReadArtifact(relPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(content) {
		t.Errorf("content = %q, want %q", string(got), string(content))
	}

	absGot, err := svc.ReadArtifact(filepath.Join(tmp, relPath))
	if err != nil {
		t.Fatal(err)
	}
	if string(absGot) != string(content) {
		t.Errorf("absolute path read = %q", string(absGot))
	}
}

func TestWriteArtifactDirectoryStructure(t *testing.T) {
	tmp := t.TempDir()
	svc, err := NewArtifactFileService(tmp)
	if err != nil {
		t.Fatal(err)
	}

	relPath, _, _, err := svc.WriteArtifact("slug", []byte("hello"), "text/plain")
	if err != nil {
		t.Fatal(err)
	}

	parts := strings.Split(relPath, string(filepath.Separator))
	if len(parts) != 4 {
		t.Fatalf("expected 4 segments (objects/YYYY/MM/slug.txt), got %d in %q", len(parts), relPath)
	}
	if parts[0] != "objects" {
		t.Errorf("segment 0 = %q, want 'objects'", parts[0])
	}
	if len(parts[1]) != 4 {
		t.Errorf("segment 1 (year) = %q, want 4 digits", parts[1])
	}
	if len(parts[2]) != 2 {
		t.Errorf("segment 2 (month) = %q, want 2 digits", parts[2])
	}
	if parts[3] != "slug.txt" {
		t.Errorf("segment 3 = %q, want 'slug.txt'", parts[3])
	}

	if _, err := os.Stat(filepath.Join(tmp, relPath)); os.IsNotExist(err) {
		t.Errorf("file not found at %s", filepath.Join(tmp, relPath))
	}
}

func TestDeleteArtifact(t *testing.T) {
	tmp := t.TempDir()
	svc, err := NewArtifactFileService(tmp)
	if err != nil {
		t.Fatal(err)
	}

	relPath, _, _, err := svc.WriteArtifact("to-delete", []byte("delete me"), "text/plain")
	if err != nil {
		t.Fatal(err)
	}

	if err := svc.DeleteArtifact(relPath); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(tmp, relPath)); !os.IsNotExist(err) {
		t.Error("file should be gone after delete")
	}

	if err := svc.DeleteArtifact("nonexistent.txt"); err == nil {
		t.Error("expected error deleting nonexistent file")
	}
}

func TestEmptyContent(t *testing.T) {
	tmp := t.TempDir()
	svc, err := NewArtifactFileService(tmp)
	if err != nil {
		t.Fatal(err)
	}

	content := []byte{}
	relPath, hash, size, err := svc.WriteArtifact("empty", content, "text/plain")
	if err != nil {
		t.Fatal(err)
	}

	if hash != HashContent(content) {
		t.Errorf("hash = %q, want %q", hash, HashContent(content))
	}
	if size != 0 {
		t.Errorf("size = %d, want 0", size)
	}

	got, err := svc.ReadArtifact(relPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 bytes, got %d", len(got))
	}
}

func TestLongContent(t *testing.T) {
	tmp := t.TempDir()
	svc, err := NewArtifactFileService(tmp)
	if err != nil {
		t.Fatal(err)
	}

	content := []byte(strings.Repeat("A", 100000))
	relPath, hash, size, err := svc.WriteArtifact("long", content, "text/plain")
	if err != nil {
		t.Fatal(err)
	}

	if hash != HashContent(content) {
		t.Errorf("hash mismatch")
	}
	if size != 100000 {
		t.Errorf("size = %d, want 100000", size)
	}

	got, err := svc.ReadArtifact(relPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 100000 {
		t.Errorf("expected 100000 bytes, got %d", len(got))
	}
}

func TestHashConsistency(t *testing.T) {
	c1 := []byte("hello world")
	c2 := []byte("hello world")
	c3 := []byte("hello world!")

	h1 := HashContent(c1)
	h2 := HashContent(c2)
	h3 := HashContent(c3)

	if h1 != h2 {
		t.Error("same content must produce same hash")
	}
	if h1 == h3 {
		t.Error("different content must produce different hash")
	}

	expected := "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
	if h1 != expected {
		t.Errorf("sha256(\"hello world\") = %q, want %q", h1, expected)
	}
}

func TestDetectMIME(t *testing.T) {
	tests := []struct {
		name    string
		content []byte
		want    string
	}{
		{"markdown starts with #", []byte("# Header"), "text/markdown"},
		{"json object", []byte(`{"key": "val"}`), "application/json"},
		{"json array", []byte(`[1, 2, 3]`), "application/json"},
		{"html", []byte("<html><body>Hi</body></html>"), "text/html"},
		{"html doctype", []byte("<!DOCTYPE html><html>"), "text/plain"}, // doesn't match <html prefix
		{"plain text", []byte("Just regular text"), "text/plain"},
		{"empty", []byte{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectMIME(tt.content)
			if got != tt.want {
				t.Errorf("DetectMIME(%q) = %q, want %q", string(tt.content), got, tt.want)
			}
		})
	}
}

func TestWriteArtifactAutoDetectMIME(t *testing.T) {
	tmp := t.TempDir()
	svc, err := NewArtifactFileService(tmp)
	if err != nil {
		t.Fatal(err)
	}

	relPath, _, _, err := svc.WriteArtifact("autodetect", []byte("# Markdown"), "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(relPath, ".md") {
		t.Errorf("expected .md extension for markdown content, got %s", relPath)
	}
}

func TestNewArtifactFileServiceDefault(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("SKILLVAULT_HOME", tmp)

	svc, err := NewArtifactFileService("")
	if err != nil {
		t.Fatal(err)
	}

	relPath, _, _, err := svc.WriteArtifact("env-test", []byte("data"), "text/plain")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(tmp, relPath)); os.IsNotExist(err) {
		t.Errorf("file should exist under SKILLVAULT_HOME at %s", filepath.Join(tmp, relPath))
	}
}
