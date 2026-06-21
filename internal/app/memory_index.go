package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/quantum-6/skillvault/internal/db"
	"github.com/quantum-6/skillvault/internal/domain"
	"github.com/quantum-6/skillvault/internal/vars"
)

// MemoryIndexService indexes pi-memory-md markdown files as SkillVault shadow entries.
// Shadow entries have external_ref set to the relative path of the .md file.
// They are searchable via FTS5 alongside native entries.
type MemoryIndexService struct {
	entryStore   db.EntryStore
	projectStore db.ProjectStore
	entryRefSvc  *EntryRefService
}

func NewMemoryIndexService(entryStore db.EntryStore, projectStore db.ProjectStore, entryRefSvc *EntryRefService) *MemoryIndexService {
	return &MemoryIndexService{
		entryStore:   entryStore,
		projectStore: projectStore,
		entryRefSvc:  entryRefSvc,
	}
}

// IndexResult summarizes a memory index operation.
type IndexResult struct {
	Indexed        int
	Skipped        int
	Failed         int
	FailedFiles    []string
	Orphaned       int
	MissingTargets []string
}

// Index walks a pi-memory-md directory, parses all .md files,
// and upserts shadow entries in SkillVault.
func (s *MemoryIndexService) Index(ctx context.Context, memDir, projectID string, parseWikilinks bool) (*IndexResult, error) {
	result := &IndexResult{}

	// Validate project exists
	if _, err := s.projectStore.Get(ctx, projectID); err != nil {
		proj := domain.Project{
			ID:          projectID,
			Name:        projectID,
			Slug:        projectID,
			Description: "Auto-created from pi-memory index",
			Status:      domain.StatusActive,
		}
		if err := s.projectStore.Save(ctx, proj); err != nil {
			return nil, fmt.Errorf("create project %q: %w", projectID, err)
		}
	}

	var indexedIDs []string

	err := filepath.Walk(memDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			result.Failed++
			result.FailedFiles = append(result.FailedFiles, path)
			return nil // skip, don't abort walk
		}
		if info.IsDir() || !strings.HasSuffix(info.Name(), ".md") {
			return nil
		}

		rel, err := filepath.Rel(memDir, path)
		if err != nil {
			result.Failed++
			result.FailedFiles = append(result.FailedFiles, path)
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			result.Failed++
			result.FailedFiles = append(result.FailedFiles, rel)
			return nil
		}

		fm, body, err := vars.ParseFrontmatter(string(content), parseWikilinks)
		if err != nil {
			result.Failed++
			result.FailedFiles = append(result.FailedFiles, rel)
			return nil
		}

		// Determine entry type based on path
		entryType := mapPathToType(rel)
		isArchived := strings.Contains(rel, string(filepath.Separator)+"archive"+string(filepath.Separator)) ||
			strings.HasPrefix(rel, "archive"+string(filepath.Separator))

		// Build ID from path
		id := "pimem-" + projectID + "-" + slugifyPath(rel)

		// Build name from description or first heading
		name := fm.Description
		if name == "" {
			name = vars.FirstHeading(body)
		}
		if name == "" {
			name = strings.TrimSuffix(info.Name(), ".md")
		}

		// Build content from description + first paragraph (not full body, to avoid duplication)
		entryContent := fm.Description
		if entryContent != "" {
			p := vars.FirstParagraph(body)
			if p != "" && p != fm.Description {
				entryContent += "\n\n" + p
			}
		} else {
			entryContent = vars.FirstParagraph(body)
		}

		// Build tags: inherit from frontmatter + fixed tag
		tags := fm.Tags
		if tags == nil {
			tags = []string{}
		}
		tags = append(tags, "pimem")
		if isArchived {
			tags = append(tags, "archived")
		}

		status := domain.StatusActive
		if isArchived {
			status = domain.StatusArchived
		}

		entry := domain.Entry{
			ID:           id,
			Title:        name,
			Slug:         id,
			Type:         entryType,
			Summary:      fm.Description,
			BodyOptional: entryContent,
			Status:       status,
			ProjectID:    &projectID,
			ExternalRef:  rel,
		}

		if err := s.entryStore.Save(ctx, entry, tags); err != nil {
			result.Failed++
			result.FailedFiles = append(result.FailedFiles, rel)
			return nil
		}

		indexedIDs = append(indexedIDs, id)

		// Parse wikilinks → entry_refs
		for _, link := range fm.Links {
			targetID := resolveWikilinkID(link, projectID)
			if targetID == "" {
				result.MissingTargets = append(result.MissingTargets, fmt.Sprintf("%s -> %s", rel, link))
				continue
			}
			_, err := s.entryRefSvc.SaveRef(ctx, AddRefInput{
				SourceID: id,
				TargetID: targetID,
				RefType:  string(domain.RelationRelatedTo),
				Label:    "wikilink",
			})
			if err != nil {
				// Target might not exist yet; skip silently
				_ = err
			}
		}

		return nil
	})

	result.Indexed = len(indexedIDs)

	// Deactivate orphans: entries with external_ref not in this index
	orphans, err := s.deactivateOrphans(ctx, projectID, indexedIDs)
	if err != nil {
		return result, fmt.Errorf("orphan cleanup: %w", err)
	}
	result.Orphaned = orphans

	return result, nil
}

// deactivateOrphans archives shadow entries whose .md file was deleted.
func (s *MemoryIndexService) deactivateOrphans(ctx context.Context, projectID string, activeIDs []string) (int, error) {
	// List entries with external_ref set for this project
	activeSet := make(map[string]bool, len(activeIDs))
	for _, id := range activeIDs {
		activeSet[id] = true
	}

	// We need to find all entries with external_ref non-empty for this project.
	// The EntryStore has a List method with EntryFilter — but no filter for external_ref.
	// Use a direct approach: search by tag "pimem" and project ID.
	results, err := s.entryStore.Search(ctx, domain.SearchQuery{
		Query:           "pimem",
		ProjectID:       &projectID,
		IncludeArchived: true,
		Limit:           9999,
	})
	if err != nil {
		return 0, fmt.Errorf("search shadow entries: %w", err)
	}

	count := 0
	for _, r := range results {
		if r.Entry.ExternalRef == "" {
			continue
		}
		if !activeSet[r.Entry.ID] && r.Entry.Status != domain.StatusArchived {
			// Only archive if currently active
			if err := s.entryStore.Archive(ctx, r.Entry.ID); err == nil {
				count++
			}
		}
	}

	return count, nil
}

// mapPathToType maps a pi-memory-md relative path to an EntryType.
func mapPathToType(rel string) domain.EntryType {
	parts := strings.Split(rel, string(filepath.Separator))
	for _, part := range parts {
		switch {
		case part == "archive" || part == "research":
			return domain.EntryTypeReference
		case part == "docs" || part == "reference" || part == "references":
			return domain.EntryTypeReference
		}
	}
	return domain.EntryTypeReference
}

// slugifyPath converts a relative file path to an ID-safe slug.
func slugifyPath(rel string) string {
	// Remove .md extension
	rel = strings.TrimSuffix(rel, ".md")
	// Replace path separators with hyphens
	rel = strings.ReplaceAll(rel, string(filepath.Separator), "-")
	// Lowercase
	rel = strings.ToLower(rel)
	// Replace any non-alphanumeric with hyphens
	var b strings.Builder
	for _, r := range rel {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		} else if r == '_' || r == ' ' {
			b.WriteRune('-')
		}
		// skip other chars
	}
	return b.String()
}

// resolveWikilinkID converts a wikilink target name to a shadow entry ID.
// Wikilinks in pi-memory reference other .md files by name or slug.
// We resolve by scanning the project for matching shadow entries.
func resolveWikilinkID(target, projectID string) string {
	// Simple heuristic: try the expected pimem-{project}-{target-slug} pattern
	slug := strings.ToLower(strings.ReplaceAll(target, " ", "-"))
	return "pimem-" + projectID + "-" + slug
}
