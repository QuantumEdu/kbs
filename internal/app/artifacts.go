package app

import (
	"context"
	"fmt"

	"github.com/quantum-6/skillvault/internal/db"
	"github.com/quantum-6/skillvault/internal/domain"
)

type SaveArtifactInput struct {
	Title    string
	Type     string
	Content  string
	FilePath string
	Summary  string
	Project  string
	Tags     []string
}

type ArtifactService struct {
	artifactStore db.ArtifactStore
	entryStore    db.EntryStore
	projectStore  db.ProjectStore
}

func NewArtifactService(artifactStore db.ArtifactStore, entryStore db.EntryStore, projectStore db.ProjectStore) *ArtifactService {
	return &ArtifactService{
		artifactStore: artifactStore,
		entryStore:    entryStore,
		projectStore:  projectStore,
	}
}

func (s *ArtifactService) SaveArtifact(ctx context.Context, input SaveArtifactInput) (*domain.Artifact, error) {
	if input.Title == "" {
		return nil, fmt.Errorf("title is required")
	}
	if err := domain.ValidateArtifactType(input.Type); err != nil {
		return nil, fmt.Errorf("validate type: %w", err)
	}
	if input.Content == "" && input.FilePath == "" {
		return nil, fmt.Errorf("either content or file_path must be provided")
	}

	var projectID *string
	if input.Project != "" {
		proj, err := s.projectStore.Get(ctx, input.Project)
		if err != nil {
			return nil, fmt.Errorf("project %q not found: %w", input.Project, err)
		}
		projectID = &proj.ID
	}

	filePath := input.FilePath
	if filePath == "" {
		filePath = "objects/inline/" + slugify(input.Title)
	}

	artifact := domain.Artifact{
		ID:        generateArtifactID(),
		Title:     input.Title,
		Slug:      slugify(input.Title),
		Type:      domain.ArtifactType(input.Type),
		FilePath:  filePath,
		MimeType:  detectMimeType(input.Type),
		Summary:   input.Summary,
		ProjectID: projectID,
	}
	if input.Content != "" {
		artifact.SizeBytes = int64(len(input.Content))
	}

	if err := s.artifactStore.Save(ctx, artifact); err != nil {
		return nil, fmt.Errorf("save artifact: %w", err)
	}

	return &artifact, nil
}

func (s *ArtifactService) GetArtifact(ctx context.Context, id string) (*domain.Artifact, error) {
	a, err := s.artifactStore.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *ArtifactService) LinkArtifactToEntry(ctx context.Context, artifactID, entryID string) error {
	_, err := s.artifactStore.Get(ctx, artifactID)
	if err != nil {
		return fmt.Errorf("artifact %q not found: %w", artifactID, err)
	}

	entry, err := s.entryStore.Get(ctx, entryID, true)
	if err != nil {
		return fmt.Errorf("entry %q not found: %w", entryID, err)
	}

	entry.Entry.ArtifactID = &artifactID
	if err := s.entryStore.Save(ctx, entry.Entry, nil); err != nil {
		return fmt.Errorf("save entry with artifact link: %w", err)
	}

	return nil
}

func detectMimeType(artifactType string) string {
	switch domain.ArtifactType(artifactType) {
	case domain.ArtifactTypeMarkdown:
		return "text/markdown"
	case domain.ArtifactTypeJSON:
		return "application/json"
	case domain.ArtifactTypeHTML:
		return "text/html"
	case domain.ArtifactTypeTXT:
		return "text/plain"
	default:
		return "application/octet-stream"
	}
}
