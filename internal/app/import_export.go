package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	"github.com/quantum-6/skillvault/internal/db"
	"github.com/quantum-6/skillvault/internal/domain"
)

type VaultExportService struct {
	store         db.ImportExportStore
	artifactStore db.ArtifactStore
	entryStore    db.EntryStore
	projectStore  db.ProjectStore
	workflowStore db.WorkflowStore
}

type VaultImportService struct {
	store         db.ImportExportStore
	entryStore    db.EntryStore
	projectStore  db.ProjectStore
	artifactStore db.ArtifactStore
}

type ExportVaultInput struct {
	IncludeArtifacts bool
	OutputPath       string
}

func NewVaultExportService(
	store db.ImportExportStore,
	artifactStore db.ArtifactStore,
	entryStore db.EntryStore,
	projectStore db.ProjectStore,
	workflowStore db.WorkflowStore,
) *VaultExportService {
	return &VaultExportService{
		store:         store,
		artifactStore: artifactStore,
		entryStore:    entryStore,
		projectStore:  projectStore,
		workflowStore: workflowStore,
	}
}

func NewVaultImportService(
	store db.ImportExportStore,
	entryStore db.EntryStore,
	projectStore db.ProjectStore,
	artifactStore db.ArtifactStore,
) *VaultImportService {
	return &VaultImportService{
		store:         store,
		entryStore:    entryStore,
		projectStore:  projectStore,
		artifactStore: artifactStore,
	}
}

func (s *VaultExportService) Export(ctx context.Context, path string) error {
	return s.ExportVault(ctx, ExportVaultInput{OutputPath: path, IncludeArtifacts: true})
}

func (s *VaultExportService) ExportVault(ctx context.Context, input ExportVaultInput) error {
	data, err := s.store.ExportAll(ctx)
	if err != nil {
		return fmt.Errorf("export: %w", err)
	}

	if !input.IncludeArtifacts {
		data.Data.Artifacts = nil
	}

	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal export: %w", err)
	}

	path := input.OutputPath
	if path == "" {
		path = "skillvault-export.json"
	}
	if err := os.WriteFile(path, jsonBytes, 0644); err != nil {
		return fmt.Errorf("write export file: %w", err)
	}

	return nil
}

func (s *VaultImportService) Import(ctx context.Context, path string) error {
	jsonBytes, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read import file: %w", err)
	}

	var data domain.VaultExport
	if err := json.Unmarshal(jsonBytes, &data); err != nil {
		return fmt.Errorf("parse import file: %w", err)
	}

	return s.ImportVault(ctx, data)
}

func (s *VaultImportService) ImportVault(ctx context.Context, data domain.VaultExport) error {
	if err := s.resolveSlugConflicts(ctx, &data); err != nil {
		return fmt.Errorf("slug conflict resolution: %w", err)
	}

	if err := s.store.ImportAll(ctx, data); err != nil {
		return fmt.Errorf("import: %w", err)
	}

	return nil
}

func (s *VaultImportService) resolveSlugConflicts(ctx context.Context, data *domain.VaultExport) error {
	existingEntries, err := s.entryStore.List(ctx, domain.EntryFilter{IncludeArchived: true})
	if err != nil {
		return fmt.Errorf("list existing entries: %w", err)
	}
	slugMap := make(map[string]string)
	for _, e := range existingEntries {
		slugMap[e.Entry.Slug] = e.Entry.ID
	}
	for i, e := range data.Data.Entries {
		if e.Slug == "" {
			continue
		}
		if existingID, exists := slugMap[e.Slug]; exists && existingID != e.ID {
			data.Data.Entries[i].Slug = resolveConflictSlug(e.Slug)
		}
	}

	existingProjects, err := s.projectStore.List(ctx, true)
	if err != nil {
		return fmt.Errorf("list existing projects: %w", err)
	}
	projSlugMap := make(map[string]string)
	for _, p := range existingProjects {
		projSlugMap[p.Slug] = p.ID
	}
	for i, p := range data.Data.Projects {
		if p.Slug == "" {
			continue
		}
		if existingID, exists := projSlugMap[p.Slug]; exists && existingID != p.ID {
			data.Data.Projects[i].Slug = resolveConflictSlug(p.Slug)
		}
	}

	return nil
}

func (s *VaultExportService) ExportJSON(ctx context.Context) ([]byte, error) {
	data, err := s.store.ExportAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("export: %w", err)
	}

	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}
	return jsonBytes, nil
}

func resolveConflictSlug(slug string) string {
	b := make([]byte, 3)
	rand.Read(b)
	return slug + "-import-" + hex.EncodeToString(b)
}
