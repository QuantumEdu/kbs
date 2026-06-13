package files

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const defaultBaseDir = ".skillvault"

type ArtifactFileService struct {
	basePath string
}

func NewArtifactFileService(basePath string) (*ArtifactFileService, error) {
	if basePath == "" {
		basePath = os.Getenv("SKILLVAULT_HOME")
	}
	if basePath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("resolve home dir: %w", err)
		}
		basePath = filepath.Join(home, defaultBaseDir)
	}
	return &ArtifactFileService{basePath: basePath}, nil
}

func extFromMIME(mimeType string) string {
	switch mimeType {
	case "text/markdown":
		return ".md"
	case "application/json":
		return ".json"
	case "text/plain":
		return ".txt"
	case "text/html":
		return ".html"
	case "application/pdf":
		return ".pdf"
	default:
		return ".bin"
	}
}

func mimeFromExt(ext string) string {
	switch ext {
	case ".md":
		return "text/markdown"
	case ".json":
		return "application/json"
	case ".txt":
		return "text/plain"
	case ".html":
		return "text/html"
	case ".pdf":
		return "application/pdf"
	default:
		return "application/octet-stream"
	}
}

func DetectMIME(content []byte) string {
	if len(content) == 0 {
		return ""
	}
	if len(content) >= 4 && content[0] == '%' && content[1] == 'P' && content[2] == 'D' && content[3] == 'F' {
		return "application/pdf"
	}
	if len(content) >= 6 && content[0] == '<' && content[1] == 'h' && content[2] == 't' && content[3] == 'm' {
		return "text/html"
	}
	if content[0] == '{' || content[0] == '[' {
		return "application/json"
	}
	if content[0] == '#' {
		return "text/markdown"
	}
	return "text/plain"
}

func (s *ArtifactFileService) WriteArtifact(slug string, content []byte, mimeType string) (string, string, int64, error) {
	now := time.Now()
	year := now.Format("2006")
	month := now.Format("01")

	if mimeType == "" {
		mimeType = DetectMIME(content)
	}
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	ext := extFromMIME(mimeType)
	relPath := filepath.Join("objects", year, month, slug+ext)
	absPath := filepath.Join(s.basePath, relPath)

	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", "", 0, fmt.Errorf("mkdir %s: %w", dir, err)
	}

	h := sha256.Sum256(content)
	hash := fmt.Sprintf("%x", h)

	if err := os.WriteFile(absPath, content, 0644); err != nil {
		return "", "", 0, fmt.Errorf("write %s: %w", absPath, err)
	}

	return relPath, hash, int64(len(content)), nil
}

func (s *ArtifactFileService) ReadArtifact(filePath string) ([]byte, error) {
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(s.basePath, filePath)
	}
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", filePath, err)
	}
	return content, nil
}

func (s *ArtifactFileService) DeleteArtifact(filePath string) error {
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(s.basePath, filePath)
	}
	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("remove %s: %w", filePath, err)
	}
	return nil
}

func HashContent(content []byte) string {
	h := sha256.Sum256(content)
	return fmt.Sprintf("%x", h)
}
