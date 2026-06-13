package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/quantum-6/skillvault/internal/domain"
)

func (s *sqliteProjectStore) Save(ctx context.Context, p domain.Project) error {
	if p.Status == "" {
		p.Status = domain.StatusActive
	}
	if p.Slug == "" {
		p.Slug = p.Name
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO projects (id, name, slug, description, status, updated_at)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			name=excluded.name,
			slug=excluded.slug,
			description=excluded.description,
			status=excluded.status,
			updated_at=CURRENT_TIMESTAMP
	`, p.ID, p.Name, p.Slug, p.Description, string(p.Status))
	if err != nil {
		return fmt.Errorf("save project: %w", err)
	}
	return nil
}

func (s *sqliteProjectStore) Get(ctx context.Context, id string) (domain.Project, error) {
	var p domain.Project
	var status string
	var desc sql.NullString

	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, slug, description, status
		FROM projects WHERE id = ? OR slug = ?
	`, id, id).Scan(&p.ID, &p.Name, &p.Slug, &desc, &status)
	if err == sql.ErrNoRows {
		return p, fmt.Errorf("project %q not found", id)
	}
	if err != nil {
		return p, fmt.Errorf("get project: %w", err)
	}

	p.Status = domain.Status(status)
	if desc.Valid {
		p.Description = desc.String
	}
	return p, nil
}

func (s *sqliteProjectStore) List(ctx context.Context, includeArchived bool) ([]domain.Project, error) {
	query := "SELECT id, name, slug, COALESCE(description,''), status FROM projects"
	var args []interface{}

	if !includeArchived {
		query += " WHERE status != 'archived'"
	}
	query += " ORDER BY name"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	var results []domain.Project
	for rows.Next() {
		var p domain.Project
		var status string
		var desc sql.NullString
		if err := rows.Scan(&p.ID, &p.Name, &p.Slug, &desc, &status); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		p.Status = domain.Status(status)
		if desc.Valid {
			p.Description = desc.String
		}
		results = append(results, p)
	}

	if results == nil {
		results = []domain.Project{}
	}
	return results, nil
}

func (s *sqliteProjectStore) Archive(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, "UPDATE projects SET status = 'archived', updated_at = CURRENT_TIMESTAMP WHERE id = ? AND status != 'archived'", id)
	if err != nil {
		return fmt.Errorf("archive project: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("project %q not found or already archived", id)
	}
	return nil
}
