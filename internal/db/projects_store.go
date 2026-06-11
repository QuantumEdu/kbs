package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/quantum-6/skillvault/internal/domain"
)

// UpsertProject creates or updates a project.
func (s *sqliteProjectStore) UpsertProject(ctx context.Context, p domain.Project) error {
	active := 0
	if p.Active {
		active = 1
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO projects (id, name, description, active, updated_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			name=excluded.name,
			description=excluded.description,
			active=excluded.active,
			updated_at=CURRENT_TIMESTAMP
	`, p.ID, p.Name, p.Description, active)
	if err != nil {
		return fmt.Errorf("upsert project: %w", err)
	}
	return nil
}

// ListProjects returns projects, optionally including archived ones.
func (s *sqliteProjectStore) ListProjects(ctx context.Context, includeArchived bool) ([]domain.Project, error) {
	query := "SELECT id, name, COALESCE(description,''), active FROM projects"
	var args []interface{}

	if !includeArchived {
		query += " WHERE active = 1"
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
		var active int
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &active); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		p.Active = active == 1
		results = append(results, p)
	}

	if results == nil {
		results = []domain.Project{}
	}
	return results, nil
}

// Ensure sqliteProjectStore implements ProjectStore
var _ ProjectStore = (*sqliteProjectStore)(nil)

// unused import guard
var _ = sql.NullString{}
