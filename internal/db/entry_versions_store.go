package db

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"

	"github.com/quantum-6/skillvault/internal/domain"
)

// SaveVersion inserts a new version row. The caller is responsible for
// generating the VersionID and calculating the correct VersionNumber
// (next auto-increment for the entry).
func (s *sqliteEntryVersionStore) SaveVersion(ctx context.Context, v domain.EntryVersion) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO entry_versions (version_id, entry_id, version_number, title, summary, body_optional, saved_at)
		 VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		v.VersionID, v.EntryID, v.VersionNumber, v.Title, v.Summary, v.BodyOptional,
	)
	if err != nil {
		return fmt.Errorf("save version: %w", err)
	}
	return nil
}

// ListVersions returns all versions for an entry in descending order
// (newest first). Returns an empty slice when no versions exist.
func (s *sqliteEntryVersionStore) ListVersions(ctx context.Context, entryID string) ([]domain.EntryVersion, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT version_id, entry_id, version_number, title, summary, body_optional,
		        COALESCE(saved_at, '') as saved_at
		 FROM entry_versions
		 WHERE entry_id = ?
		 ORDER BY version_number DESC`, entryID,
	)
	if err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}
	defer rows.Close()

	var versions []domain.EntryVersion
	for rows.Next() {
		var v domain.EntryVersion
		if err := rows.Scan(&v.VersionID, &v.EntryID, &v.VersionNumber,
			&v.Title, &v.Summary, &v.BodyOptional, &v.SavedAt); err != nil {
			return nil, fmt.Errorf("scan version: %w", err)
		}
		versions = append(versions, v)
	}
	if versions == nil {
		versions = []domain.EntryVersion{}
	}
	return versions, nil
}

// GetVersion retrieves a specific version by entry ID and version number.
// Returns sql.ErrNoRows when the version does not exist.
func (s *sqliteEntryVersionStore) GetVersion(ctx context.Context, entryID string, versionNumber int) (domain.EntryVersion, error) {
	var v domain.EntryVersion
	err := s.db.QueryRowContext(ctx,
		`SELECT version_id, entry_id, version_number, title, summary, body_optional,
		        COALESCE(saved_at, '') as saved_at
		 FROM entry_versions
		 WHERE entry_id = ? AND version_number = ?`,
		entryID, versionNumber,
	).Scan(&v.VersionID, &v.EntryID, &v.VersionNumber,
		&v.Title, &v.Summary, &v.BodyOptional, &v.SavedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return v, fmt.Errorf("entry %q version %d: %w", entryID, versionNumber, err)
		}
		return v, fmt.Errorf("get version: %w", err)
	}
	return v, nil
}

func generateVersionID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "ver-" + hex.EncodeToString(b)
}
