package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/quantum-6/skillvault/internal/db"
	"github.com/quantum-6/skillvault/internal/domain"
)

// VaultExportService handles vault export operations.
type VaultExportService struct {
	store db.ImportExportStore
}

// VaultImportService handles vault import operations.
type VaultImportService struct {
	store db.ImportExportStore
}

// NewVaultExportService creates a new export service.
func NewVaultExportService(store db.ImportExportStore) *VaultExportService {
	return &VaultExportService{store: store}
}

// NewVaultImportService creates a new import service.
func NewVaultImportService(store db.ImportExportStore) *VaultImportService {
	return &VaultImportService{store: store}
}

// Export writes the vault to a JSON file.
func (s *VaultExportService) Export(ctx context.Context, path string) error {
	data, err := s.store.ExportAll(ctx)
	if err != nil {
		return fmt.Errorf("export: %w", err)
	}

	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal export: %w", err)
	}

	if err := os.WriteFile(path, jsonBytes, 0644); err != nil {
		return fmt.Errorf("write export file: %w", err)
	}

	return nil
}

// Import reads a vault from a JSON file and imports it.
func (s *VaultImportService) Import(ctx context.Context, path string) error {
	jsonBytes, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read import file: %w", err)
	}

	var data domain.VaultExport
	if err := json.Unmarshal(jsonBytes, &data); err != nil {
		return fmt.Errorf("parse import file: %w", err)
	}

	if err := s.store.ImportAll(ctx, data); err != nil {
		return fmt.Errorf("import: %w", err)
	}

	return nil
}
