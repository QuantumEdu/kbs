package domain

import (
	"fmt"
	"strings"
)

// NotFoundError is returned when a resource is not found.
type NotFoundError struct {
	Resource string
	ID       string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s %q not found", e.Resource, e.ID)
}

// NormalizeTags applies tag normalization rules: trim whitespace, lowercase,
// replace spaces with dashes, reject empty strings, and deduplicate.
// Returns nil for nil input.
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

// ValidateEntryType checks if the string is a valid entry type.
func ValidateEntryType(t string) error {
	et := EntryType(t)
	if !et.IsValid() {
		return fmt.Errorf("invalid entry type: %q (expected one of: skill, agent, workflow, prompt, context, note)", t)
	}
	return nil
}

// ValidateSearchQuery checks the search query fields for validity.
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

// ValidateSeriesScope validates that an entry can be added to a series
// based on the scope rules:
// - Global series accepts only global entries
// - Project series accepts global entries or same-project entries
// - Cross-project entries are rejected
func ValidateSeriesScope(seriesProjectID, entryProjectID *string) error {
	// No series project = global series
	if seriesProjectID == nil {
		// Global series can only contain global entries
		if entryProjectID != nil {
			return fmt.Errorf("global series cannot contain project entries")
		}
		return nil
	}

	// Project series
	if entryProjectID == nil {
		// Project series can contain global entries
		return nil
	}

	// Both have projects — must be the same
	if *seriesProjectID != *entryProjectID {
		return fmt.Errorf("series belongs to project %q, cannot contain entry from project %q", *seriesProjectID, *entryProjectID)
	}

	return nil
}

// ValidateStepNumbers checks that the step numbers are sequential starting from 1 with no gaps.
func ValidateStepNumbers(steps []int) error {
	for i, step := range steps {
		expected := i + 1
		if step != expected {
			return fmt.Errorf("step numbers must be sequential from 1: expected %d at position %d, got %d", expected, i, step)
		}
	}
	return nil
}
