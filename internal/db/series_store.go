package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/quantum-6/skillvault/internal/domain"
)

// UpsertSeries creates or updates a series header.
func (s *sqliteSeriesStore) UpsertSeries(ctx context.Context, series domain.Series) error {
	active := 0
	if series.Active {
		active = 1
	}
	projectID := interface{}(nil)
	if series.ProjectID != nil {
		projectID = *series.ProjectID
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO series (id, name, project_id, description, vars, active, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			name=excluded.name,
			project_id=excluded.project_id,
			description=excluded.description,
			vars=excluded.vars,
			active=excluded.active,
			updated_at=CURRENT_TIMESTAMP
	`, series.ID, series.Name, projectID, series.Description, series.Vars, active)
	if err != nil {
		return fmt.Errorf("upsert series: %w", err)
	}
	return nil
}

// GetSeries retrieves a series with its entries and calculated step metadata.
func (s *sqliteSeriesStore) GetSeries(ctx context.Context, id string, includeArchived bool) (domain.SeriesResult, error) {
	var result domain.SeriesResult
	var projectID sql.NullString
	var description sql.NullString
	var vars sql.NullString
	var active int

	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, project_id, description, vars, active
		FROM series WHERE id = ?
	`, id).Scan(&result.Series.ID, &result.Series.Name, &projectID,
		&description, &vars, &active)
	if err == sql.ErrNoRows {
		return result, fmt.Errorf("series %q not found", id)
	}
	if err != nil {
		return result, fmt.Errorf("get series: %w", err)
	}

	result.Series.Active = active == 1
	if projectID.Valid {
		result.Series.ProjectID = &projectID.String
	}
	if description.Valid {
		result.Series.Description = description.String
	}
	if vars.Valid {
		result.Series.Vars = vars.String
	}

	// Load entries
	query := `SELECT se.series_id, se.entry_id, se.step_num, COALESCE(se.label,''), se.required, COALESCE(se.notes,''), se.active
		FROM series_entries se WHERE se.series_id = ?`
	if !includeArchived {
		query += " AND se.active = 1"
	}
	query += " ORDER BY se.step_num"

	rows, err := s.db.QueryContext(ctx, query, id)
	if err != nil {
		return result, fmt.Errorf("get series entries: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var entry domain.SeriesEntry
		var req int
		var entryActive int
		if err := rows.Scan(&entry.SeriesID, &entry.EntryID, &entry.StepNum,
			&entry.Label, &req, &entry.Notes, &entryActive); err != nil {
			return result, fmt.Errorf("scan series entry: %w", err)
		}
		entry.Required = req == 1
		entry.Active = entryActive == 1
		result.Entries = append(result.Entries, entry)
	}

	result.TotalSteps = len(result.Entries)
	if result.Entries == nil {
		result.Entries = []domain.SeriesEntry{}
	}

	return result, nil
}

// ListSeries returns series matching the filter.
func (s *sqliteSeriesStore) ListSeries(ctx context.Context, filter domain.SeriesFilter) ([]domain.SeriesListResult, error) {
	query := "SELECT s.id, s.name, s.project_id, s.description, s.vars, s.active FROM series s WHERE 1=1"
	var args []interface{}

	if !filter.IncludeArchived {
		query += " AND s.active = 1"
	}
	if filter.ProjectID != nil {
		query += " AND s.project_id = ?"
		args = append(args, *filter.ProjectID)
	}
	query += " ORDER BY s.name"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list series: %w", err)
	}
	defer rows.Close()

	var results []domain.SeriesListResult
	for rows.Next() {
		var r domain.SeriesListResult
		var projectID sql.NullString
		var description sql.NullString
		var vars sql.NullString
		var active int
		if err := rows.Scan(&r.Series.ID, &r.Series.Name, &projectID,
			&description, &vars, &active); err != nil {
			return nil, fmt.Errorf("scan series: %w", err)
		}
		r.Series.Active = active == 1
		if projectID.Valid {
			r.Series.ProjectID = &projectID.String
		}
		if description.Valid {
			r.Series.Description = description.String
		}
		if vars.Valid {
			r.Series.Vars = vars.String
		}

		// Count active steps
		err = s.db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM series_entries WHERE series_id = ? AND active = 1", r.Series.ID).Scan(&r.TotalSteps)
		if err != nil {
			r.TotalSteps = 0
		}

		results = append(results, r)
	}

	if results == nil {
		results = []domain.SeriesListResult{}
	}
	return results, nil
}

// ReplaceSeriesEntries transactionally replaces all entries in a series.
func (s *sqliteSeriesStore) ReplaceSeriesEntries(ctx context.Context, seriesID string, entries []domain.SeriesEntryInput) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Delete existing entries
	if _, err := tx.ExecContext(ctx, "DELETE FROM series_entries WHERE series_id = ?", seriesID); err != nil {
		return fmt.Errorf("delete old entries: %w", err)
	}

	// Insert new entries renumbered 1..N
	for i, e := range entries {
		stepNum := i + 1
		required := 1
		if !e.Required {
			required = 0
		}
		_, err := tx.ExecContext(ctx,
			"INSERT INTO series_entries (series_id, entry_id, step_num, label, required, notes, active) VALUES (?, ?, ?, ?, ?, ?, 1)",
			seriesID, e.EntryID, stepNum, e.Label, required, e.Notes,
		)
		if err != nil {
			return fmt.Errorf("insert entry %d (%s): %w", stepNum, e.EntryID, err)
		}
	}

	return tx.Commit()
}

// Ensure sqliteSeriesStore implements SeriesStore
var _ SeriesStore = (*sqliteSeriesStore)(nil)
