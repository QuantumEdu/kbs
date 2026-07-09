package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/quantum-6/skillvault/internal/db"
	"github.com/quantum-6/skillvault/internal/domain"
)

// VaultPackExportService produces skill pack exports wrapping a full
// vault export with identifying metadata (author, version, description).
type VaultPackExportService struct {
	store         db.ImportExportStore
	entryStore    db.EntryStore
	projectStore  db.ProjectStore
	artifactStore db.ArtifactStore
	workflowStore db.WorkflowStore
}

func NewVaultPackExportService(
	store db.ImportExportStore,
	entryStore db.EntryStore,
	projectStore db.ProjectStore,
	artifactStore db.ArtifactStore,
	workflowStore db.WorkflowStore,
) *VaultPackExportService {
	return &VaultPackExportService{
		store:         store,
		entryStore:    entryStore,
		projectStore:  projectStore,
		artifactStore: artifactStore,
		workflowStore: workflowStore,
	}
}

// ExportPackInput holds the parameters for a pack export.
type ExportPackInput struct {
	Author      string
	Version     string
	Description string
	OutputPath  string
}

// ExportPack exports the full vault contents wrapped in a VaultPackExport
// envelope with pack metadata, and writes the result as JSON to the
// configured output path.
func (s *VaultPackExportService) ExportPack(ctx context.Context, input ExportPackInput) error {
	data, err := s.store.ExportAll(ctx)
	if err != nil {
		return fmt.Errorf("export all: %w", err)
	}

	pack := domain.VaultPackExport{
		Pack: domain.PackMetadata{
			PackID:      generatePackID(),
			Author:      input.Author,
			Version:     input.Version,
			Description: input.Description,
			ExportedAt:  time.Now().UTC().Format(time.RFC3339),
		},
		Data: data,
	}

	jsonBytes, err := json.MarshalIndent(pack, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal pack export: %w", err)
	}

	path := input.OutputPath
	if path == "" {
		path = "skillvault-pack.svpack"
	}
	if err := os.WriteFile(path, jsonBytes, 0644); err != nil {
		return fmt.Errorf("write pack export file: %w", err)
	}

	return nil
}

func generatePackID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "pack-" + hex.EncodeToString(b)
}
