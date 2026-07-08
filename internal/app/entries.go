package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"

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

// RouteType discriminates the target of a resolved route.
type RouteType string

const (
	RouteTypeWorkflow RouteType = "workflow"
	RouteTypeSkill    RouteType = "skill"
)

const (
	routingTagName   = "workflow-route"
	routeSearchLimit = 50
)

// RouteTarget represents a routing body YAML key value with workflow and/or skill targets.
type RouteTarget struct {
	Workflow string `yaml:"workflow"`
	Skill    string `yaml:"skill"`
}

// RouteResult holds the resolved scenario routing result.
type RouteResult struct {
	Scenario    string           `json:"scenario"`
	Type        RouteType        `json:"type"`
	Target      string           `json:"target"`
	Description string           `json:"description"`
	Workflow    *domain.Workflow `json:"workflow,omitempty"`
}

type EntryService struct {
	store         db.EntryStore
	projectStore  db.ProjectStore
	artifactStore db.ArtifactStore
	workflowStore db.WorkflowStore
	vector        *VectorService
}

func NewEntryService(store db.EntryStore, projectStore db.ProjectStore, artifactStore db.ArtifactStore) *EntryService {
	return &EntryService{
		store:         store,
		projectStore:  projectStore,
		artifactStore: artifactStore,
	}
}

// SetVectorService injects a VectorService for auto-embedding on SaveEntry.
// When nil or not set, auto-embed is silently skipped.
func (s *EntryService) SetVectorService(svc *VectorService) {
	s.vector = svc
}

// SetWorkflowStore injects a WorkflowStore for route scenario resolution.
func (s *EntryService) SetWorkflowStore(store db.WorkflowStore) {
	s.workflowStore = store
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

	// Auto-embed if VectorService is configured and GloVe is loaded.
	// Silently skips when no VectorService or GloVe not loaded.
	if s.vector != nil {
		_ = s.vector.EnsureEmbedded(ctx, entry.ID)
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

// RouteScenario resolves a scenario string to a matching workflow or skill
// by searching routing-type entries. Resolution cascade: FTS5 → tag fallback →
// YAML body key match → workflow lookup.
func (s *EntryService) RouteScenario(ctx context.Context, scenario string) (*RouteResult, error) {
	routingType := string(domain.EntryTypeRouting)

	// Step 1: FTS5 search on routing entries.
	results, err := s.SearchEntries(ctx, scenario, domain.SearchQuery{
		Type:  &routingType,
		Limit: routeSearchLimit,
	})
	if err != nil {
		return nil, fmt.Errorf("search routing entries: %w", err)
	}

	// Step 2: Tag fallback — search by routing tag when FTS5 is empty.
	if len(results) == 0 {
		tagResults, tagErr := s.SearchByTags(ctx, []string{routingTagName}, false, &routingType, nil, routeSearchLimit)
		if tagErr != nil {
			// Tag fallback is best-effort; continue with empty.
			tagResults = nil
		}
		results = tagResults
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no routing entries found for scenario %q; create one with: add-entry --type routing --title \"Route to X\" --body \"%s:\n  workflow: your-workflow-slug\" --tags %s", scenario, scenario, routingTagName)
	}

	// Step 3: Parse YAML bodies for exact key match.
	var lastErr error
	for _, r := range results {
		if r.Entry.BodyOptional == "" {
			continue
		}

		var routeMap map[string]RouteTarget
		if err := yaml.Unmarshal([]byte(r.Entry.BodyOptional), &routeMap); err != nil {
			// Malformed YAML: warn and skip per spec.
			fmt.Fprintf(os.Stderr, "[sk-vault] warning: skipping malformed YAML in routing entry %q: %v\n", r.Entry.ID, err)
			continue
		}

		target, ok := routeMap[scenario]
		if !ok {
			continue
		}

		// Found a matching scenario key.
		if target.Workflow != "" {
			if s.workflowStore == nil {
				return nil, fmt.Errorf("workflow store not wired; route resolution requires it")
			}

			wf, err := s.workflowStore.Get(ctx, target.Workflow)
			if err != nil {
				// Stale workflow reference: warn and continue.
				fmt.Fprintf(os.Stderr, "[sk-vault] warning: referenced workflow %q not found (entry %q)\n", target.Workflow, r.Entry.ID)
				lastErr = fmt.Errorf("referenced workflow %q not found", target.Workflow)
				continue
			}

			return &RouteResult{
				Scenario:    scenario,
				Type:        RouteTypeWorkflow,
				Target:      target.Workflow,
				Description: wf.Description,
				Workflow:    &wf,
			}, nil
		}

		if target.Skill != "" {
			return &RouteResult{
				Scenario:    scenario,
				Type:        RouteTypeSkill,
				Target:      target.Skill,
				Description: r.Entry.Summary,
			}, nil
		}
	}

	if lastErr != nil {
		return nil, lastErr
	}

	// Entry bodies exist but no YAML key matched the scenario.
	return nil, fmt.Errorf("no routing entries found for scenario %q", scenario)
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
