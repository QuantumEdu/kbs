package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/quantum-6/skillvault/internal/domain"
)

// SavePromptResultInput is the domain-specific input for saving a prompt result.
type SavePromptResultInput struct {
	Name           string   // required
	Content        string   // required
	Type           string   // optional, defaults to "note"
	Category       string   // optional, free-text classification
	Tags           []string // optional, will be normalized
	ProjectID      string   // optional, empty = global
	SourcePromptID string   // optional, ID of the prompt entry used
	Model          string   // optional, LLM model identifier
}

// SavePromptResultOutput is returned after a successful save.
type SavePromptResultOutput struct {
	EntryID   string
	Name      string
	Type      string
	ProjectID string
}

// SavePromptResultService saves a prompt execution result as a vault entry.
type SavePromptResultService struct {
	entries *EntryService
}

// NewSavePromptResultService creates a new SavePromptResultService.
func NewSavePromptResultService(entries *EntryService) *SavePromptResultService {
	return &SavePromptResultService{entries: entries}
}

// Save validates input, maps fields to a domain Entry, and delegates to EntryService.UpsertEntry.
func (s *SavePromptResultService) Save(ctx context.Context, input SavePromptResultInput) (*SavePromptResultOutput, error) {
	// Validate required fields
	if input.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if input.Content == "" {
		return nil, fmt.Errorf("content is required")
	}

	// Default type
	entryType := domain.EntryTypeNote
	if input.Type != "" {
		if err := domain.ValidateEntryType(input.Type); err != nil {
			return nil, fmt.Errorf("validate type: %w", err)
		}
		entryType = domain.EntryType(input.Type)
	}

	// Normalize tags
	tags := domain.NormalizeTags(input.Tags)

	// Build vars JSON: package model and source_prompt_id into a JSON object
	varsJSON := buildVarsJSON(input.Model, input.SourcePromptID)

	// Build project_id pointer
	var projectID *string
	if input.ProjectID != "" {
		projectID = &input.ProjectID
	}

	// Generate entry ID
	entryID := generateResultID()

	entry := domain.Entry{
		ID:          entryID,
		Name:        input.Name,
		Type:        entryType,
		ProjectID:   projectID,
		Description: input.Category,
		Content:     input.Content,
		Vars:        varsJSON,
		Active:      true,
	}

	if err := s.entries.UpsertEntry(ctx, entry, tags, nil); err != nil {
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

// buildVarsJSON creates a JSON object string from model and source_prompt_id.
// Only non-empty fields are included. Returns empty string if both are empty.
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

// generateResultID creates a unique ID for a prompt result entry.
func generateResultID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "res-" + hex.EncodeToString(b)
}
