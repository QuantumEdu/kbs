package db

import (
	"context"
	"fmt"

	"github.com/quantum-6/skillvault/internal/domain"
)

func (s *sqliteEntryLinkStore) Save(ctx context.Context, link domain.EntryLink) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO entry_links (from_entry_id, to_entry_id, relation_type)
		VALUES (?, ?, ?)
		ON CONFLICT(from_entry_id, to_entry_id, relation_type) DO NOTHING
	`, link.FromEntryID, link.ToEntryID, string(link.RelationType))
	if err != nil {
		return fmt.Errorf("save link: %w", err)
	}
	return nil
}

func (s *sqliteEntryLinkStore) GetLinks(ctx context.Context, entryID string) ([]domain.EntryLink, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT from_entry_id, to_entry_id, relation_type FROM entry_links
		WHERE from_entry_id = ? OR to_entry_id = ?
		ORDER BY relation_type
	`, entryID, entryID)
	if err != nil {
		return nil, fmt.Errorf("get links: %w", err)
	}
	defer rows.Close()

	var results []domain.EntryLink
	for rows.Next() {
		var link domain.EntryLink
		var rt string
		if err := rows.Scan(&link.FromEntryID, &link.ToEntryID, &rt); err != nil {
			return nil, fmt.Errorf("scan link: %w", err)
		}
		link.RelationType = domain.RelationType(rt)
		results = append(results, link)
	}

	if results == nil {
		results = []domain.EntryLink{}
	}
	return results, nil
}

func (s *sqliteEntryLinkStore) GetLinksByType(ctx context.Context, entryID string, relationType string) ([]domain.EntryLink, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT from_entry_id, to_entry_id, relation_type FROM entry_links
		WHERE (from_entry_id = ? OR to_entry_id = ?) AND relation_type = ?
		ORDER BY relation_type
	`, entryID, entryID, relationType)
	if err != nil {
		return nil, fmt.Errorf("get links by type: %w", err)
	}
	defer rows.Close()

	var results []domain.EntryLink
	for rows.Next() {
		var link domain.EntryLink
		var rt string
		if err := rows.Scan(&link.FromEntryID, &link.ToEntryID, &rt); err != nil {
			return nil, fmt.Errorf("scan link: %w", err)
		}
		link.RelationType = domain.RelationType(rt)
		results = append(results, link)
	}

	if results == nil {
		results = []domain.EntryLink{}
	}
	return results, nil
}
