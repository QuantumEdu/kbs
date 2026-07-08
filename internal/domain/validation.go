package domain

import (
	"fmt"
	"strings"
)

type NotFoundError struct {
	Resource string
	ID       string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s %q not found", e.Resource, e.ID)
}

func NormalizeTags(tags []string) []string {
	if tags == nil {
		return nil
	}
	seen := make(map[string]bool)
	result := make([]string, 0, len(tags))
	for _, tag := range tags {
		normalized := strings.ToLower(strings.TrimSpace(tag))
		normalized = strings.ReplaceAll(normalized, " ", "-")
		if normalized == "" {
			continue
		}
		if !seen[normalized] {
			seen[normalized] = true
			result = append(result, normalized)
		}
	}
	return result
}

func ValidateEntryType(t string) error {
	et := EntryType(t)
	if !et.IsValid() {
		return fmt.Errorf("invalid entry type: %q (expected one of: prompt, skill, workflow_note, reference, user, feedback, project_state, session, decision, artifact_summary, handoff, routing)", t)
	}
	return nil
}

func ValidateStatus(s string) error {
	st := Status(s)
	if !st.IsValid() {
		return fmt.Errorf("invalid status: %q (expected one of: draft, active, archived, deprecated, canonical)", s)
	}
	return nil
}

func ValidatePurpose(p string) error {
	pu := Purpose(p)
	if !pu.IsValid() {
		return fmt.Errorf("invalid purpose: %q (expected one of: WORK, KNOWLEDGE, LEARNING, RELATIONSHIP, STATE, or empty)", p)
	}
	return nil
}

func ValidateArtifactType(t string) error {
	at := ArtifactType(t)
	if !at.IsValid() {
		return fmt.Errorf("invalid artifact type: %q (expected one of: markdown, json, txt, html, pdf_reference, ai_output, pdf_analysis, spec, report, session_output)", t)
	}
	return nil
}

func ValidateRelationType(t string) error {
	rt := RelationType(t)
	if !rt.IsValid() {
		return fmt.Errorf("invalid relation type: %q (expected one of: references, supersedes, related_to, part_of, derived_from, implements, uses, extends, handoff_of, generated_from, depends_on)", t)
	}
	return nil
}

func ValidateSearchQuery(q SearchQuery) error {
	if q.Type != nil {
		if err := ValidateEntryType(*q.Type); err != nil {
			return err
		}
	}
	if q.Limit < 0 {
		return fmt.Errorf("limit must be >= 0, got %d", q.Limit)
	}
	return nil
}

func ValidateSeriesScope(seriesProjectID, entryProjectID *string) error {
	if seriesProjectID == nil {
		if entryProjectID != nil {
			return fmt.Errorf("global series cannot contain project entries")
		}
		return nil
	}
	if entryProjectID == nil {
		return nil
	}
	if *seriesProjectID != *entryProjectID {
		return fmt.Errorf("series belongs to project %q, cannot contain entry from project %q", *seriesProjectID, *entryProjectID)
	}
	return nil
}
