package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/quantum-6/skillvault/internal/domain"
)

func (s *sqliteTagStore) Save(ctx context.Context, tag domain.Tag) error {
	if tag.Slug == "" {
		tag.Slug = tag.Name
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO tags (id, name, slug) VALUES (?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET name=excluded.name, slug=excluded.slug
	`, tag.ID, tag.Name, tag.Slug)
	if err != nil {
		return fmt.Errorf("save tag: %w", err)
	}
	return nil
}

func (s *sqliteTagStore) List(ctx context.Context) ([]domain.Tag, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, name, slug FROM tags ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	defer rows.Close()

	var results []domain.Tag
	for rows.Next() {
		var tag domain.Tag
		if err := rows.Scan(&tag.ID, &tag.Name, &tag.Slug); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		results = append(results, tag)
	}

	if results == nil {
		results = []domain.Tag{}
	}
	return results, nil
}

func (s *sqliteTagStore) Search(ctx context.Context, query string) ([]domain.Tag, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, name, slug FROM tags WHERE name LIKE ? OR slug LIKE ? ORDER BY name",
		"%"+query+"%", "%"+query+"%")
	if err != nil {
		return nil, fmt.Errorf("search tags: %w", err)
	}
	defer rows.Close()

	var results []domain.Tag
	for rows.Next() {
		var tag domain.Tag
		if err := rows.Scan(&tag.ID, &tag.Name, &tag.Slug); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		results = append(results, tag)
	}

	if results == nil {
		results = []domain.Tag{}
	}
	return results, nil
}

var _ = sql.NullString{}
