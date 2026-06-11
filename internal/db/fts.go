package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/quantum-6/skillvault/internal/domain"
)

// SearchEntries performs FTS5 search with filters.
func (s *sqliteSearchStore) SearchEntries(ctx context.Context, q domain.SearchQuery) ([]domain.EntrySearchResult, error) {
	var conditions []string
	var args []interface{}

	// FTS5 MATCH clause
	if q.Query != "" {
		conditions = append(conditions, "e_fts.entries_fts MATCH ?")
		args = append(args, q.Query)
	}

	// Join with entries for filters
	query := `SELECT e.id, e.name, e.type, e.project_id, e.description, e.content, e.vars, e.active
		FROM entries_fts e_fts
		JOIN entries e ON e.id = e_fts.id`

	if len(conditions) > 0 || !q.IncludeArchived || q.ProjectID != nil || q.Type != nil {
		if len(conditions) > 0 {
			query += " WHERE " + conditions[0]
		} else {
			query += " WHERE 1=1"
		}
		for _, c := range conditions[1:] {
			query += " AND " + c
		}

		if !q.IncludeArchived {
			query += " AND e.active = 1"
		}
		if q.ProjectID != nil {
			query += " AND e.project_id = ?"
			args = append(args, *q.ProjectID)
		}
		if q.Type != nil {
			query += " AND e.type = ?"
			args = append(args, *q.Type)
		}
	}

	query += " ORDER BY rank"
	if q.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", q.Limit)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("search entries: %w", err)
	}
	defer rows.Close()

	var results []domain.EntrySearchResult
	for rows.Next() {
		var r domain.EntrySearchResult
		var projectID sql.NullString
		var description sql.NullString
		var vars sql.NullString
		var active int
		if err := rows.Scan(&r.Entry.ID, &r.Entry.Name, &r.Entry.Type, &projectID,
			&description, &r.Entry.Content, &vars, &active); err != nil {
			return nil, fmt.Errorf("scan search result: %w", err)
		}
		r.Entry.Active = active == 1
		if projectID.Valid {
			r.Entry.ProjectID = &projectID.String
		}
		if description.Valid {
			r.Entry.Description = description.String
		}
		if vars.Valid {
			r.Entry.Vars = vars.String
		}

		// Load tags
		tagRows, err := s.db.QueryContext(ctx, "SELECT tag FROM entry_tags WHERE entry_id = ? ORDER BY tag", r.Entry.ID)
		if err == nil {
			for tagRows.Next() {
				var tag string
				if err := tagRows.Scan(&tag); err != nil {
					break
				}
				r.Tags = append(r.Tags, tag)
			}
			tagRows.Close()
		}
		if r.Tags == nil {
			r.Tags = []string{}
		}

		// Load series refs (max 3)
		refRows, err := s.db.QueryContext(ctx,
			`SELECT se.series_id, s.name, se.step_num, se.label
			 FROM series_entries se
			 JOIN series s ON s.id = se.series_id
			 WHERE se.entry_id = ? AND se.active = 1
			 ORDER BY se.series_id LIMIT 3`, r.Entry.ID)
		if err == nil {
			for refRows.Next() {
				var ref domain.SeriesRef
				var label sql.NullString
				if err := refRows.Scan(&ref.SeriesID, &ref.SeriesName, &ref.StepNum, &label); err != nil {
					break
				}
				if label.Valid {
					ref.Label = label.String
				}
				// Get total_steps
				var total int
				if err := s.db.QueryRowContext(ctx,
					"SELECT COUNT(*) FROM series_entries WHERE series_id = ? AND active = 1", ref.SeriesID).Scan(&total); err == nil {
					ref.TotalSteps = total
				}
				r.SeriesRefs = append(r.SeriesRefs, ref)
			}
			refRows.Close()
		}
		if r.SeriesRefs == nil {
			r.SeriesRefs = []domain.SeriesRef{}
		}

		results = append(results, r)
	}

	if results == nil {
		results = []domain.EntrySearchResult{}
	}
	return results, nil
}

// RebuildFTS rebuilds the FTS5 index from the entries table.
func (s *sqliteSearchStore) RebuildFTS(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, "INSERT INTO entries_fts(entries_fts) VALUES('rebuild')")
	if err != nil {
		return fmt.Errorf("rebuild fts: %w", err)
	}
	return nil
}

var _ SearchStore = (*sqliteSearchStore)(nil)

// unused import guard
var _ = sql.NullString{}
