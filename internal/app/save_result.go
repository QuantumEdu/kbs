package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/quantum-6/skillvault/internal/db"
	"github.com/quantum-6/skillvault/internal/domain"
)

type SavePromptResultInput struct {
	Name           string
	Content        string
	Type           string
	Category       string
	Tags           []string
	ProjectID      string
	SourcePromptID string
	Model          string
}

type SavePromptResultOutput struct {
	EntryID   string
	Name      string
	Type      string
	ProjectID string
}

type SavePromptResultService struct {
	entries *EntryService
}

func NewSavePromptResultService(store db.EntryStore, projectStore db.ProjectStore, artifactStore db.ArtifactStore) *SavePromptResultService {
	return &SavePromptResultService{entries: NewEntryService(store, projectStore, artifactStore)}
}

func (s *SavePromptResultService) Save(ctx context.Context, input SavePromptResultInput) (*SavePromptResultOutput, error) {
	if input.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if input.Content == "" {
		return nil, fmt.Errorf("content is required")
	}

	entryType := domain.EntryTypePrompt
	if input.Type != "" {
		if err := domain.ValidateEntryType(input.Type); err != nil {
			return nil, fmt.Errorf("validate type: %w", err)
		}
		entryType = domain.EntryType(input.Type)
	}

	tags := domain.NormalizeTags(input.Tags)

	var projectID *string
	if input.ProjectID != "" {
		projectID = &input.ProjectID
	}

	entryID := generateResultID()

	entry := domain.Entry{
		ID:           entryID,
		Title:        input.Name,
		Type:         entryType,
		ProjectID:    projectID,
		Summary:      input.Category,
		BodyOptional: input.Content,
		Status:       domain.StatusActive,
		Slug:         input.Name,
	}

	if err := s.entries.Save(ctx, entry, tags); err != nil {
		return nil, fmt.Errorf("save prompt result: %w", err)
	}

	projOut := "global"
	if projectID != nil {
		projOut = *projectID
	}

	return &SavePromptResultOutput{
		EntryID:   entryID,
		Name:      input.Name,
		Type:      string(entryType),
		ProjectID: projOut,
	}, nil
}

func buildVarsJSON(model, sourcePromptID string) string {
	vars := map[string]string{}
	if model != "" {
		vars["model"] = model
	}
	if sourcePromptID != "" {
		vars["source_prompt_id"] = sourcePromptID
	}
	if len(vars) == 0 {
		return ""
	}
	data, _ := json.Marshal(vars)
	return string(data)
}

func generateResultID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "res-" + hex.EncodeToString(b)
}
