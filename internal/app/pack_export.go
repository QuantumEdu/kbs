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

// VaultPackExportService produces portable skill packs wrapping VaultExport
// with authorship metadata.
type VaultPackExportService struct {
	store         db.ImportExportStore
	artifactStore db.ArtifactStore
	entryStore    db.EntryStore
	projectStore  db.ProjectStore
	workflowStore db.WorkflowStore
}

// PackExportInput carries the metadata required to produce a pack.
type PackExportInput struct {
	Pack        string
	Author      string
	Version     string
	Description string
	OutputPath  string
}

// NewVaultPackExportService creates a pack export service backed by the same
// store interfaces used by VaultExportService.
func NewVaultPackExportService(
	store db.ImportExportStore,
	artifactStore db.ArtifactStore,
	entryStore db.EntryStore,
	projectStore db.ProjectStore,
	workflowStore db.WorkflowStore,
) *VaultPackExportService {
	return &VaultPackExportService{
		store:         store,
		artifactStore: artifactStore,
		entryStore:    entryStore,
		projectStore:  projectStore,
		workflowStore: workflowStore,
	}
}

func newPackID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// ExportPack runs a full vault export, wraps it in VaultPackExport with the
// supplied metadata, and writes the resulting JSON to the output path.
func (s *VaultPackExportService) ExportPack(ctx context.Context, input PackExportInput) error {
	data, err := s.store.ExportAll(ctx)
	if err != nil {
		return fmt.Errorf("export pack: %w", err)
	}

	pack := domain.VaultPackExport{
		Pack: domain.PackMetadata{
			PackID:      newPackID(),
			Author:      input.Author,
			Version:     input.Version,
			Description: input.Description,
			ExportedAt:  time.Now().UTC().Format(time.RFC3339),
			Source:      "skillvault",
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
