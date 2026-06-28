package app

import (
	"context"
	"fmt"

	"github.com/quantum-6/skillvault/internal/db"
	"github.com/quantum-6/skillvault/internal/domain"
)

// EntryVersionService provides operations on historical entry versions.
type EntryVersionService struct {
	versions db.EntryVersionStore
	entries  db.EntryStore
}

// NewEntryVersionService creates a version service backed by store interfaces.
func NewEntryVersionService(versions db.EntryVersionStore, entries db.EntryStore) *EntryVersionService {
	return &EntryVersionService{versions: versions, entries: entries}
}

// ListVersions returns all historical versions for an entry, newest first.
func (s *EntryVersionService) ListVersions(ctx context.Context, entryID string) ([]domain.EntryVersion, error) {
	return s.versions.ListVersions(ctx, entryID)
}

// RestoreVersion restores an entry to a previous version's content.
// The current (pre-restore) content is automatically archived by the store's
// Save() method, so the restored state replaces the current entry.
func (s *EntryVersionService) RestoreVersion(ctx context.Context, entryID string, versionNumber int) (*domain.Entry, error) {
	// 1. Retrieve the archived version content.
	version, err := s.versions.GetVersion(ctx, entryID, versionNumber)
	if err != nil {
		return nil, fmt.Errorf("version %d not found for entry %q: %w", versionNumber, entryID, err)
	}

	// 2. Retrieve the current entry (with tags).
	current, err := s.entries.Get(ctx, entryID, false)
	if err != nil {
		return nil, fmt.Errorf("get entry %q: %w", entryID, err)
	}

	// 3. Overwrite title/summary/body with version content, keeping other fields.
	entry := current.Entry
	entry.Title = version.Title
	entry.Summary = version.Summary
	entry.BodyOptional = version.BodyOptional

	// 4. Extract tag names from the current entry.
	tags := make([]string, len(current.Tags))
	for i, t := range current.Tags {
		tags[i] = t.Name
	}

	// 5. Save — the store's Save() auto-archives the pre-restore state.
	if err := s.entries.Save(ctx, entry, tags); err != nil {
		return nil, fmt.Errorf("save restored entry: %w", err)
	}

	return &entry, nil
}
