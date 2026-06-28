package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/quantum-6/skillvault/internal/domain"
)

type sqliteEntryVersionStore struct{ db *sql.DB }

func newVersionID() string {
	return uuid.New().String()
}

func (s *sqliteEntryVersionStore) SaveVersion(ctx context.Context, v domain.EntryVersion) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO entry_versions (version_id, entry_id, version_number, title, summary, body_optional)
		VALUES (?, ?, ?, ?, ?, ?)
	`, v.VersionID, v.EntryID, v.VersionNumber, v.Title, v.Summary, v.BodyOptional)
	if err != nil {
		return fmt.Errorf("save version: %w", err)
	}
	return nil
}

func (s *sqliteEntryVersionStore) ListVersions(ctx context.Context, entryID string) ([]domain.EntryVersion, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT version_id, entry_id, version_number, title, COALESCE(summary,''), COALESCE(body_optional,''), saved_at
		FROM entry_versions
		WHERE entry_id = ?
		ORDER BY version_number DESC
	`, entryID)
	if err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}
	defer rows.Close()

	var versions []domain.EntryVersion
	for rows.Next() {
		var v domain.EntryVersion
		if err := rows.Scan(&v.VersionID, &v.EntryID, &v.VersionNumber, &v.Title, &v.Summary, &v.BodyOptional, &v.SavedAt); err != nil {
			return nil, fmt.Errorf("scan version: %w", err)
		}
		versions = append(versions, v)
	}

	if versions == nil {
		versions = []domain.EntryVersion{}
	}
	return versions, nil
}

func (s *sqliteEntryVersionStore) GetVersion(ctx context.Context, entryID string, versionNumber int) (domain.EntryVersion, error) {
	var v domain.EntryVersion
	err := s.db.QueryRowContext(ctx, `
		SELECT version_id, entry_id, version_number, title, COALESCE(summary,''), COALESCE(body_optional,''), saved_at
		FROM entry_versions
		WHERE entry_id = ? AND version_number = ?
	`, entryID, versionNumber).Scan(&v.VersionID, &v.EntryID, &v.VersionNumber, &v.Title, &v.Summary, &v.BodyOptional, &v.SavedAt)
	if err == sql.ErrNoRows {
		return v, fmt.Errorf("version %d not found for entry %q", versionNumber, entryID)
	}
	if err != nil {
		return v, fmt.Errorf("get version: %w", err)
	}
	return v, nil
}
