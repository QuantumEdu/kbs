package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/quantum-6/skillvault/internal/db"
	"github.com/quantum-6/skillvault/internal/domain"
)

// EntryVersionService provides read operations over entry version history
// and the ability to restore a past version as the current entry content.
type EntryVersionService struct {
	versionStore db.EntryVersionStore
	entryStore   db.EntryStore
}

func NewEntryVersionService(versionStore db.EntryVersionStore, entryStore db.EntryStore) *EntryVersionService {
	return &EntryVersionService{
		versionStore: versionStore,
		entryStore:   entryStore,
	}
}

// ListVersions returns all known versions for an entry in descending
// order (newest first). Returns an empty slice when the entry has no
// versions or the entry does not exist.
func (s *EntryVersionService) ListVersions(ctx context.Context, entryID string) ([]domain.EntryVersion, error) {
	return s.versionStore.ListVersions(ctx, entryID)
}

// RestoreVersion retrieves a specific version's content and saves it
// as the current entry state. The pre-restore current state is
// automatically archived by Save() before the UPSERT.
// Restoring a version that does not exist returns an error.
func (s *EntryVersionService) RestoreVersion(ctx context.Context, entryID string, versionNumber int) error {
	version, err := s.versionStore.GetVersion(ctx, entryID, versionNumber)
	if err != nil {
		return fmt.Errorf("restore version %d: %w", versionNumber, err)
	}

	// Get the current entry to preserve all non-content fields.
	current, err := s.entryStore.Get(ctx, entryID, true)
	if err != nil {
		return fmt.Errorf("get current entry for restore: %w", err)
	}

	// Override content fields with the archived version.
	current.Entry.Title = version.Title
	current.Entry.Summary = version.Summary
	current.Entry.BodyOptional = version.BodyOptional

	// Save triggers archiveBeforeSave which captures the pre-restore
	// state as a new version row.
	if err := s.entryStore.Save(ctx, current.Entry, nil); err != nil {
		return fmt.Errorf("save restored entry: %w", err)
	}

	return nil
}

func generateVersionID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "ver-" + hex.EncodeToString(b)
}
