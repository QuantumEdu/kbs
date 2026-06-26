package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode"

	"github.com/quantum-6/skillvault/internal/db"
	"github.com/quantum-6/skillvault/internal/domain"
)

type SaveEntryInput struct {
	Title   string
	Type    string
	Summary string
	Body    string
	Project string
	Tags    []string
	Status  string
}

type GetEntryResult struct {
	Entry    domain.EntryResult
	Artifact *domain.Artifact
}

type EntryService struct {
	store         db.EntryStore
	projectStore  db.ProjectStore
	artifactStore db.ArtifactStore
}

func NewEntryService(store db.EntryStore, projectStore db.ProjectStore, artifactStore db.ArtifactStore) *EntryService {
	return &EntryService{
		store:         store,
		projectStore:  projectStore,
		artifactStore: artifactStore,
	}
}

func (s *EntryService) Save(ctx context.Context, entry domain.Entry, tags []string) error {
	if err := domain.ValidateEntryType(string(entry.Type)); err != nil {
		return fmt.Errorf("validate entry: %w", err)
	}
	tags = domain.NormalizeTags(tags)
	return s.store.Save(ctx, entry, tags)
}

func (s *EntryService) SaveEntry(ctx context.Context, input SaveEntryInput) (*GetEntryResult, error) {
	if input.Title == "" {
		return nil, fmt.Errorf("title is required")
	}
	if err := domain.ValidateEntryType(input.Type); err != nil {
		return nil, fmt.Errorf("validate type: %w", err)
	}
	status := domain.StatusActive
	if input.Status != "" {
		if err := domain.ValidateStatus(input.Status); err != nil {
			return nil, fmt.Errorf("validate status: %w", err)
		}
		status = domain.Status(input.Status)
	}

	var projectID *string
	if input.Project != "" {
		proj, err := s.projectStore.Get(ctx, input.Project)
		if err != nil {
			return nil, fmt.Errorf("project %q not found: %w", input.Project, err)
		}
		projectID = &proj.ID
	}

	tags := domain.NormalizeTags(input.Tags)

	slug := slugify(input.Title)
	if slug == "" {
		slug = "untitled"
	}
	finalID := slug
	counter := 2
	for {
		_, err := s.store.Get(ctx, finalID, true)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				break
			}
			return nil, fmt.Errorf("check slug collision: %w", err)
		}
		finalID = fmt.Sprintf("%s-%d", slug, counter)
		counter++
	}

	entry := domain.Entry{
		ID:           finalID,
		Slug:         slug,
		Title:        input.Title,
		Type:         domain.EntryType(input.Type),
		Summary:      input.Summary,
		BodyOptional: input.Body,
		Status:       status,
		ProjectID:    projectID,
	}

	if err := s.store.Save(ctx, entry, tags); err != nil {
		return nil, fmt.Errorf("save entry: %w", err)
	}

	result, err := s.store.Get(ctx, entry.ID, true)
	if err != nil {
		return nil, fmt.Errorf("get saved entry: %w", err)
	}

	var artifact *domain.Artifact
	if result.Entry.ArtifactID != nil {
		a, err := s.artifactStore.Get(ctx, *result.Entry.ArtifactID)
		if err == nil {
			artifact = &a
		}
	}

	return &GetEntryResult{Entry: result, Artifact: artifact}, nil
}

func (s *EntryService) Get(ctx context.Context, id string, includeArchived bool) (domain.EntryResult, error) {
	return s.store.Get(ctx, id, includeArchived)
}

func (s *EntryService) GetEntry(ctx context.Context, id string) (*GetEntryResult, error) {
	result, err := s.store.Get(ctx, id, false)
	if err != nil {
		return nil, err
	}

	var artifact *domain.Artifact
	if result.Entry.ArtifactID != nil {
		a, err := s.artifactStore.Get(ctx, *result.Entry.ArtifactID)
		if err == nil {
			artifact = &a
		}
	}

	return &GetEntryResult{Entry: result, Artifact: artifact}, nil
}

func (s *EntryService) Search(ctx context.Context, q domain.SearchQuery) ([]domain.EntrySearchResult, error) {
	if err := domain.ValidateSearchQuery(q); err != nil {
		return nil, fmt.Errorf("validate search: %w", err)
	}
	return s.store.Search(ctx, q)
}

func (s *EntryService) SearchEntries(ctx context.Context, query string, filters domain.SearchQuery) ([]domain.EntrySearchResult, error) {
	q := filters
	q.Query = query
	if err := domain.ValidateSearchQuery(q); err != nil {
		return nil, fmt.Errorf("validate search: %w", err)
	}
	return s.store.Search(ctx, q)
}

func (s *EntryService) List(ctx context.Context, filter domain.EntryFilter) ([]domain.EntryListResult, error) {
	return s.store.List(ctx, filter)
}

func (s *EntryService) Archive(ctx context.Context, id string) error {
	return s.store.Archive(ctx, id)
}

func (s *EntryService) SearchByTags(ctx context.Context, tags []string, matchAll bool, typePtr, projectPtr *string, limit int) ([]domain.EntrySearchResult, error) {
	tags = domain.NormalizeTags(tags)
	if len(tags) == 0 {
		return nil, fmt.Errorf("at least one tag is required")
	}
	return s.store.SearchByTags(ctx, tags, matchAll, typePtr, projectPtr, limit)
}

func (s *EntryService) ArchiveEntry(ctx context.Context, id string) error {
	return s.store.Archive(ctx, id)
}

func slugify(title string) string {
	s := strings.ToLower(title)
	s = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == ' ' {
			return r
		}
		return -1
	}, s)
	s = strings.Join(strings.Fields(s), "-")
	// Collapse consecutive dashes
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return strings.Trim(s, "-")
}

func generateArtifactID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "art-" + hex.EncodeToString(b)
}

func generateWorkflowID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "wf-" + hex.EncodeToString(b)
}

func generateSeriesID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "ser-" + hex.EncodeToString(b)
}

func generateProjectID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "proj-" + hex.EncodeToString(b)
}

func generateSessionID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "ses-" + hex.EncodeToString(b)
}
