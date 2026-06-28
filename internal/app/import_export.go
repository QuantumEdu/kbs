package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"

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

func (s *VaultImportService) Import(ctx context.Context, path string, prefix string) error {
	jsonBytes, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read import file: %w", err)
	}

	// Detect pack format: if the top-level JSON has a "pack" key, it's a VaultPackExport.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(jsonBytes, &raw); err != nil {
		return fmt.Errorf("parse import file: %w", err)
	}

	if _, isPack := raw["pack"]; isPack {
		var pack domain.VaultPackExport
		if err := json.Unmarshal(jsonBytes, &pack); err != nil {
			return fmt.Errorf("parse pack import: %w", err)
		}
		if prefix != "" {
			applyPrefix(prefix, &pack.Data.Data)
		}
		return s.ImportVault(ctx, pack.Data)
	}

	// Bare VaultExport (backward compat).
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
		slugMap[strings.ToLower(e.Entry.Slug)] = e.Entry.ID
	}
	for i, e := range data.Data.Entries {
		if e.Slug == "" {
			continue
		}
		if existingID, exists := slugMap[strings.ToLower(e.Slug)]; exists && existingID != e.ID {
			data.Data.Entries[i].Slug = resolveConflictSlug(e.Slug)
		}
	}

	existingProjects, err := s.projectStore.List(ctx, true)
	if err != nil {
		return fmt.Errorf("list existing projects: %w", err)
	}
	projSlugMap := make(map[string]string)
	for _, p := range existingProjects {
		projSlugMap[strings.ToLower(p.Slug)] = p.ID
	}
	for i, p := range data.Data.Projects {
		if p.Slug == "" {
			continue
		}
		if existingID, exists := projSlugMap[strings.ToLower(p.Slug)]; exists && existingID != p.ID {
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

// applyPrefix prepends the given string to every entity ID field in VaultData,
// enabling namespace-safe imports.
func applyPrefix(prefix string, data *domain.VaultData) {
	for i := range data.Entries {
		data.Entries[i].ID = prefix + data.Entries[i].ID
		if data.Entries[i].ProjectID != nil {
			newVal := prefix + *data.Entries[i].ProjectID
			data.Entries[i].ProjectID = &newVal
		}
		if data.Entries[i].ArtifactID != nil {
			newVal := prefix + *data.Entries[i].ArtifactID
			data.Entries[i].ArtifactID = &newVal
		}
	}
	for i := range data.Projects {
		data.Projects[i].ID = prefix + data.Projects[i].ID
	}
	for i := range data.Artifacts {
		data.Artifacts[i].ID = prefix + data.Artifacts[i].ID
		if data.Artifacts[i].ProjectID != nil {
			newVal := prefix + *data.Artifacts[i].ProjectID
			data.Artifacts[i].ProjectID = &newVal
		}
		if data.Artifacts[i].SourceEntryID != nil {
			newVal := prefix + *data.Artifacts[i].SourceEntryID
			data.Artifacts[i].SourceEntryID = &newVal
		}
	}
	for i := range data.Workflows {
		data.Workflows[i].ID = prefix + data.Workflows[i].ID
	}
	for i := range data.WorkflowSteps {
		data.WorkflowSteps[i].WorkflowID = prefix + data.WorkflowSteps[i].WorkflowID
		data.WorkflowSteps[i].ID = prefix + data.WorkflowSteps[i].ID
	}
	for i := range data.Series {
		data.Series[i].ID = prefix + data.Series[i].ID
	}
	for i := range data.SeriesEntries {
		data.SeriesEntries[i].SeriesID = prefix + data.SeriesEntries[i].SeriesID
		data.SeriesEntries[i].EntryID = prefix + data.SeriesEntries[i].EntryID
	}
	for i := range data.EntryTags {
		data.EntryTags[i].EntryID = prefix + data.EntryTags[i].EntryID
	}
	for i := range data.EntryLinks {
		data.EntryLinks[i].FromEntryID = prefix + data.EntryLinks[i].FromEntryID
		data.EntryLinks[i].ToEntryID = prefix + data.EntryLinks[i].ToEntryID
	}
	for i := range data.WorkflowRuns {
		data.WorkflowRuns[i].WorkflowID = prefix + data.WorkflowRuns[i].WorkflowID
	}
	for i := range data.WorkflowRunSteps {
		data.WorkflowRunSteps[i].RunID = prefix + data.WorkflowRunSteps[i].RunID
		data.WorkflowRunSteps[i].EntryID = prefix + data.WorkflowRunSteps[i].EntryID
	}
}

func resolveConflictSlug(slug string) string {
	b := make([]byte, 3)
	rand.Read(b)
	return slug + "-import-" + hex.EncodeToString(b)
}
