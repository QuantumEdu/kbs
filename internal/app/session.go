package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/quantum-6/skillvault/internal/db"
	"github.com/quantum-6/skillvault/internal/domain"
)

type SessionWrapInput struct {
	Project   string
	Summary   string
	Decisions []string
	Pending   []string
	Learnings []string
	Artifacts []string
}

type SessionWrapOutput struct {
	Entry    *GetEntryResult
	Artifact *domain.Artifact
}

type SessionService struct {
	entryService    *EntryService
	artifactService *ArtifactService
	projectService  *ProjectService
	entryStore      db.EntryStore
	artifactStore   db.ArtifactStore
	projectStore    db.ProjectStore
}

func NewSessionService(
	entryService *EntryService,
	artifactService *ArtifactService,
	projectService *ProjectService,
	entryStore db.EntryStore,
	artifactStore db.ArtifactStore,
	projectStore db.ProjectStore,
) *SessionService {
	return &SessionService{
		entryService:    entryService,
		artifactService: artifactService,
		projectService:  projectService,
		entryStore:      entryStore,
		artifactStore:   artifactStore,
		projectStore:    projectStore,
	}
}

func (s *SessionService) SessionWrap(ctx context.Context, input SessionWrapInput) (*SessionWrapOutput, error) {
	if input.Summary == "" {
		return nil, fmt.Errorf("summary is required")
	}

	var projectID *string
	if input.Project != "" {
		proj, err := s.projectStore.Get(ctx, input.Project)
		if err != nil {
			return nil, fmt.Errorf("project %q not found: %w", input.Project, err)
		}
		projectID = &proj.ID
	}

	sessionBody := input.Summary
	if len(input.Decisions) > 0 {
		sessionBody += "\n\n## Decisions\n"
		for _, d := range input.Decisions {
			sessionBody += fmt.Sprintf("- %s\n", d)
		}
	}
	if len(input.Pending) > 0 {
		sessionBody += "\n## Pending\n"
		for _, p := range input.Pending {
			sessionBody += fmt.Sprintf("- %s\n", p)
		}
	}
	if len(input.Learnings) > 0 {
		sessionBody += "\n## Learnings\n"
		for _, l := range input.Learnings {
			sessionBody += fmt.Sprintf("- %s\n", l)
		}
	}

	slug := "ses-" + generateSessionID()
	entry := domain.Entry{
		ID:           generateSessionID(),
		Title:        "Session: " + truncate(input.Summary, 60),
		Slug:         slug,
		Type:         domain.EntryTypeSession,
		Summary:      input.Summary,
		BodyOptional: sessionBody,
		Status:       domain.StatusActive,
		ProjectID:    projectID,
	}

	if err := s.entryStore.Save(ctx, entry, nil); err != nil {
		return nil, fmt.Errorf("save session entry: %w", err)
	}

	var artifact *domain.Artifact
	if len(input.Artifacts) > 0 {
		artifactList := make([]string, 0, len(input.Artifacts))
		for _, aID := range input.Artifacts {
			a, err := s.artifactStore.Get(ctx, aID)
			if err == nil {
				artifactList = append(artifactList, a.Title)
			}
		}
		if len(artifactList) > 0 {
			body := sessionBody + "\n\n## Linked Artifacts\n"
			for _, t := range artifactList {
				body += fmt.Sprintf("- %s\n", t)
			}

			a := domain.Artifact{
				ID:            generateArtifactID(),
				Title:         "Session Output: " + truncate(input.Summary, 60),
				Slug:          "sess-out-" + generateSessionID(),
				Type:          domain.ArtifactTypeSessionOutput,
				FilePath:      "objects/sessions/" + slug + ".md",
				MimeType:      "text/markdown",
				Summary:       input.Summary,
				SizeBytes:     int64(len(body)),
				ProjectID:     projectID,
				SourceEntryID: &entry.ID,
			}
			if err := s.artifactStore.Save(ctx, a); err != nil {
				return nil, fmt.Errorf("save session artifact: %w", err)
			}
			artifact = &a

			entry.ArtifactID = &a.ID
			if err := s.entryStore.Save(ctx, entry, nil); err != nil {
				return nil, fmt.Errorf("update entry with artifact link: %w", err)
			}
		}
	}

	result, err := s.entryStore.Get(ctx, entry.ID, true)
	if err != nil {
		return nil, fmt.Errorf("get saved session: %w", err)
	}

	return &SessionWrapOutput{
		Entry:    &GetEntryResult{Entry: result, Artifact: artifact},
		Artifact: artifact,
	}, nil
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

func (s SessionWrapInput) summaryLine() string {
	parts := make([]string, 0, 3)
	if s.Summary != "" {
		parts = append(parts, s.Summary)
	}
	if len(s.Decisions) > 0 {
		parts = append(parts, fmt.Sprintf("%d decisions", len(s.Decisions)))
	}
	if len(s.Pending) > 0 {
		parts = append(parts, fmt.Sprintf("%d pending", len(s.Pending)))
	}
	if len(s.Learnings) > 0 {
		parts = append(parts, fmt.Sprintf("%d learnings", len(s.Learnings)))
	}
	return strings.Join(parts, " | ")
}
