package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/quantum-6/skillvault/internal/domain"
)

// ExportAll exports the entire vault as a JSON-serializable structure.
func (s *sqliteImportExportStore) ExportAll(ctx context.Context) (domain.VaultExport, error) {
	export := domain.VaultExport{
		SchemaVersion: 1,
		AppVersion:    "v1-alpha",
		ExportedAt:    time.Now().UTC().Format(time.RFC3339),
		Source:        "skillvault",
	}

	var err error
	export.Data.Projects, err = s.exportProjects(ctx)
	if err != nil {
		return export, err
	}
	export.Data.Entries, err = s.exportEntries(ctx)
	if err != nil {
		return export, err
	}
	export.Data.EntryTags, err = s.exportEntryTags(ctx)
	if err != nil {
		return export, err
	}
	export.Data.Series, err = s.exportSeries(ctx)
	if err != nil {
		return export, err
	}
	export.Data.SeriesEntries, err = s.exportSeriesEntries(ctx)
	if err != nil {
		return export, err
	}
	export.Data.WorkflowSteps, err = s.exportWorkflowSteps(ctx)
	if err != nil {
		return export, err
	}

	return export, nil
}

// ImportAll imports a vault export into the database in a single transaction.
func (s *sqliteImportExportStore) ImportAll(ctx context.Context, data domain.VaultExport) error {
	if data.SchemaVersion == 0 {
		return fmt.Errorf("import rejected: missing schema_version")
	}
	if data.SchemaVersion > 1 {
		return fmt.Errorf("import rejected: schema_version %d exceeds supported version 1", data.SchemaVersion)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Upsert projects
	for _, p := range data.Data.Projects {
		active := 0
		if p.Active {
			active = 1
		}
		_, err := tx.ExecContext(ctx,
			`INSERT INTO projects (id, name, description, active, updated_at) VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
			 ON CONFLICT(id) DO UPDATE SET name=excluded.name, description=excluded.description,
			 active=excluded.active, updated_at=CURRENT_TIMESTAMP`,
			p.ID, p.Name, p.Description, active)
		if err != nil {
			return fmt.Errorf("import project %s: %w", p.ID, err)
		}
	}

	// Upsert entries
	for _, e := range data.Data.Entries {
		active := 0
		if e.Active {
			active = 1
		}
		projectID := interface{}(nil)
		if e.ProjectID != nil {
			projectID = *e.ProjectID
		}
		_, err := tx.ExecContext(ctx,
			`INSERT INTO entries (id, name, type, project_id, description, content, vars, tags_denorm, active, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
			 ON CONFLICT(id) DO UPDATE SET name=excluded.name, type=excluded.type,
			 project_id=excluded.project_id, description=excluded.description,
			 content=excluded.content, vars=excluded.vars, tags_denorm=excluded.tags_denorm,
			 active=excluded.active, updated_at=CURRENT_TIMESTAMP`,
			e.ID, e.Name, string(e.Type), projectID, e.Description, e.Content, e.Vars, "", active)
		if err != nil {
			return fmt.Errorf("import entry %s: %w", e.ID, err)
		}
	}

	// Upsert entry_tags
	for _, et := range data.Data.EntryTags {
		_, err := tx.ExecContext(ctx,
			"INSERT OR REPLACE INTO entry_tags (entry_id, tag) VALUES (?, ?)",
			et.EntryID, et.Tag)
		if err != nil {
			return fmt.Errorf("import tag %s/%s: %w", et.EntryID, et.Tag, err)
		}
	}

	// Upsert series
	for _, s := range data.Data.Series {
		active := 0
		if s.Active {
			active = 1
		}
		projectID := interface{}(nil)
		if s.ProjectID != nil {
			projectID = *s.ProjectID
		}
		_, err := tx.ExecContext(ctx,
			`INSERT INTO series (id, name, project_id, description, vars, active, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
			 ON CONFLICT(id) DO UPDATE SET name=excluded.name, project_id=excluded.project_id,
			 description=excluded.description, vars=excluded.vars,
			 active=excluded.active, updated_at=CURRENT_TIMESTAMP`,
			s.ID, s.Name, projectID, s.Description, s.Vars, active)
		if err != nil {
			return fmt.Errorf("import series %s: %w", s.ID, err)
		}
	}

	// Upsert series_entries
	for _, se := range data.Data.SeriesEntries {
		required := 0
		if se.Required {
			required = 1
		}
		active := 0
		if se.Active {
			active = 1
		}
		_, err := tx.ExecContext(ctx,
			`INSERT OR REPLACE INTO series_entries (series_id, entry_id, step_num, label, required, notes, active)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			se.SeriesID, se.EntryID, se.StepNum, se.Label, required, se.Notes, active)
		if err != nil {
			return fmt.Errorf("import series_entry %s/%s: %w", se.SeriesID, se.EntryID, err)
		}
	}

	// Upsert workflow_steps
	for _, ws := range data.Data.WorkflowSteps {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO workflow_steps (id, entry_id, step_num, role, content, label)
			 VALUES (?, ?, ?, ?, ?, ?)
			 ON CONFLICT(id) DO UPDATE SET entry_id=excluded.entry_id, step_num=excluded.step_num,
			 role=excluded.role, content=excluded.content, label=excluded.label`,
			ws.ID, ws.EntryID, ws.StepNum, string(ws.Role), ws.Content, ws.Label)
		if err != nil {
			return fmt.Errorf("import workflow_step %d: %w", ws.ID, err)
		}
	}

	// Rebuild FTS5
	if _, err := tx.ExecContext(ctx, "INSERT INTO entries_fts(entries_fts) VALUES('rebuild')"); err != nil {
		return fmt.Errorf("rebuild fts after import: %w", err)
	}

	return tx.Commit()
}

// Helper export methods
func (s *sqliteImportExportStore) exportProjects(ctx context.Context) ([]domain.Project, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, name, COALESCE(description,''), active FROM projects ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanProjects(rows)
}

func (s *sqliteImportExportStore) exportEntries(ctx context.Context) ([]domain.Entry, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, type, project_id, COALESCE(description,''), content, COALESCE(vars,''), active FROM entries ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []domain.Entry
	for rows.Next() {
		var e domain.Entry
		var projID sql.NullString
		var desc, vars string
		var active int
		if err := rows.Scan(&e.ID, &e.Name, &e.Type, &projID, &desc, &e.Content, &vars, &active); err != nil {
			return nil, err
		}
		e.Active = active == 1
		e.Description = desc
		e.Vars = vars
		if projID.Valid {
			e.ProjectID = &projID.String
		}
		results = append(results, e)
	}
	return results, nil
}

func (s *sqliteImportExportStore) exportEntryTags(ctx context.Context) ([]domain.EntryTag, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT entry_id, tag FROM entry_tags ORDER BY entry_id, tag")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []domain.EntryTag
	for rows.Next() {
		var et domain.EntryTag
		if err := rows.Scan(&et.EntryID, &et.Tag); err != nil {
			return nil, err
		}
		results = append(results, et)
	}
	return results, nil
}

func (s *sqliteImportExportStore) exportSeries(ctx context.Context) ([]domain.Series, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, project_id, COALESCE(description,''), COALESCE(vars,''), active FROM series ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []domain.Series
	for rows.Next() {
		var se domain.Series
		var projID sql.NullString
		var desc, vars string
		var active int
		if err := rows.Scan(&se.ID, &se.Name, &projID, &desc, &vars, &active); err != nil {
			return nil, err
		}
		se.Active = active == 1
		se.Description = desc
		se.Vars = vars
		if projID.Valid {
			se.ProjectID = &projID.String
		}
		results = append(results, se)
	}
	return results, nil
}

func (s *sqliteImportExportStore) exportSeriesEntries(ctx context.Context) ([]domain.SeriesEntry, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT series_id, entry_id, step_num, COALESCE(label,''), required, COALESCE(notes,''), active FROM series_entries ORDER BY series_id, step_num`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []domain.SeriesEntry
	for rows.Next() {
		var se domain.SeriesEntry
		var req, active int
		if err := rows.Scan(&se.SeriesID, &se.EntryID, &se.StepNum, &se.Label, &req, &se.Notes, &active); err != nil {
			return nil, err
		}
		se.Required = req == 1
		se.Active = active == 1
		results = append(results, se)
	}
	return results, nil
}

func (s *sqliteImportExportStore) exportWorkflowSteps(ctx context.Context) ([]domain.WorkflowStep, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, entry_id, step_num, role, content, COALESCE(label,'') FROM workflow_steps ORDER BY entry_id, step_num`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []domain.WorkflowStep
	for rows.Next() {
		var ws domain.WorkflowStep
		if err := rows.Scan(&ws.ID, &ws.EntryID, &ws.StepNum, &ws.Role, &ws.Content, &ws.Label); err != nil {
			return nil, err
		}
		results = append(results, ws)
	}
	return results, nil
}

func scanProjects(rows *sql.Rows) ([]domain.Project, error) {
	var results []domain.Project
	for rows.Next() {
		var p domain.Project
		var active int
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &active); err != nil {
			return nil, err
		}
		p.Active = active == 1
		results = append(results, p)
	}
	return results, nil
}

var _ ImportExportStore = (*sqliteImportExportStore)(nil)

// unused guard
var _ = json.Marshal
