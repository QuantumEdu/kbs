package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/quantum-6/skillvault/internal/domain"
)

func (s *sqliteImportExportStore) ExportAll(ctx context.Context) (domain.VaultExport, error) {
	export := domain.VaultExport{
		SchemaVersion: 2,
		AppVersion:    "v3",
		ExportedAt:    time.Now().UTC().Format(time.RFC3339),
		Source:        "skillvault",
	}

	var err error
	export.Data.Projects, err = s.exportProjects(ctx)
	if err != nil {
		return export, fmt.Errorf("export projects: %w", err)
	}
	export.Data.Entries, err = s.exportEntries(ctx)
	if err != nil {
		return export, fmt.Errorf("export entries: %w", err)
	}
	export.Data.EntryTags, err = s.exportEntryTags(ctx)
	if err != nil {
		return export, fmt.Errorf("export entry_tags: %w", err)
	}
	export.Data.Tags, err = s.exportTags(ctx)
	if err != nil {
		return export, fmt.Errorf("export tags: %w", err)
	}
	export.Data.Series, err = s.exportSeries(ctx)
	if err != nil {
		return export, fmt.Errorf("export series: %w", err)
	}
	export.Data.SeriesEntries, err = s.exportSeriesEntries(ctx)
	if err != nil {
		return export, fmt.Errorf("export series_entries: %w", err)
	}
	export.Data.Workflows, err = s.exportWorkflows(ctx)
	if err != nil {
		return export, fmt.Errorf("export workflows: %w", err)
	}
	export.Data.WorkflowSteps, err = s.exportWorkflowSteps(ctx)
	if err != nil {
		return export, fmt.Errorf("export workflow_steps: %w", err)
	}
	export.Data.EntryLinks, err = s.exportEntryLinks(ctx)
	if err != nil {
		return export, fmt.Errorf("export entry_links: %w", err)
	}
	export.Data.Artifacts, err = s.exportArtifacts(ctx)
	if err != nil {
		return export, fmt.Errorf("export artifacts: %w", err)
	}

	return export, nil
}

func (s *sqliteImportExportStore) ImportAll(ctx context.Context, data domain.VaultExport) error {
	if data.SchemaVersion == 0 {
		return fmt.Errorf("import rejected: missing schema_version")
	}
	if data.SchemaVersion > 2 {
		return fmt.Errorf("import rejected: schema_version %d is newer than supported (max 2)", data.SchemaVersion)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	for _, t := range data.Data.Tags {
		if t.Slug == "" {
			t.Slug = t.Name
		}
		_, err := tx.ExecContext(ctx,
			`INSERT INTO tags (id, name, slug) VALUES (?, ?, ?)
			 ON CONFLICT(id) DO UPDATE SET name=excluded.name, slug=excluded.slug`,
			t.ID, t.Name, t.Slug)
		if err != nil {
			return fmt.Errorf("import tag %s: %w", t.ID, err)
		}
	}

	for _, p := range data.Data.Projects {
		if p.Status == "" {
			p.Status = domain.StatusActive
		}
		if p.Slug == "" {
			p.Slug = p.Name
		}
		_, err := tx.ExecContext(ctx,
			`INSERT INTO projects (id, name, slug, description, status, updated_at)
			 VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
			 ON CONFLICT(id) DO UPDATE SET
			 name=excluded.name, slug=excluded.slug,
			 description=excluded.description, status=excluded.status,
			 updated_at=CURRENT_TIMESTAMP`,
			p.ID, p.Name, p.Slug, p.Description, string(p.Status))
		if err != nil {
			return fmt.Errorf("import project %s: %w", p.ID, err)
		}
	}

	for _, wf := range data.Data.Workflows {
		if wf.Status == "" {
			wf.Status = domain.StatusActive
		}
		if wf.Slug == "" {
			wf.Slug = wf.Name
		}
		_, err := tx.ExecContext(ctx,
			`INSERT INTO workflows (id, name, slug, description, status, updated_at)
			 VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
			 ON CONFLICT(id) DO UPDATE SET
			 name=excluded.name, slug=excluded.slug,
			 description=excluded.description, status=excluded.status,
			 updated_at=CURRENT_TIMESTAMP`,
			wf.ID, wf.Name, wf.Slug, wf.Description, string(wf.Status))
		if err != nil {
			return fmt.Errorf("import workflow %s: %w", wf.ID, err)
		}
	}

	for _, s := range data.Data.Series {
		if s.Status == "" {
			s.Status = domain.StatusActive
		}
		if s.Slug == "" {
			s.Slug = s.Name
		}
		_, err := tx.ExecContext(ctx,
			`INSERT INTO series (id, name, slug, description, status, updated_at)
			 VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
			 ON CONFLICT(id) DO UPDATE SET
			 name=excluded.name, slug=excluded.slug,
			 description=excluded.description, status=excluded.status,
			 updated_at=CURRENT_TIMESTAMP`,
			s.ID, s.Name, s.Slug, s.Description, string(s.Status))
		if err != nil {
			return fmt.Errorf("import series %s: %w", s.ID, err)
		}
	}

	for _, e := range data.Data.Entries {
		if e.Status == "" {
			e.Status = domain.StatusActive
		}
		if e.Slug == "" {
			e.Slug = e.Title
		}
		projectID := interface{}(nil)
		if e.ProjectID != nil {
			projectID = *e.ProjectID
		}
		artifactID := interface{}(nil)
		if e.ArtifactID != nil {
			artifactID = *e.ArtifactID
		}
		_, err := tx.ExecContext(ctx,
			`INSERT INTO entries (id, name, title, slug, type, content, summary, body_optional, status, project_id, artifact_id, external_ref, tags_denorm, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '', CURRENT_TIMESTAMP)
			 ON CONFLICT(id) DO UPDATE SET
			 name=excluded.name, title=excluded.title, slug=excluded.slug,
			 type=excluded.type, content=excluded.content,
			 summary=excluded.summary, body_optional=excluded.body_optional,
			 status=excluded.status, project_id=excluded.project_id,
			 artifact_id=excluded.artifact_id, external_ref=excluded.external_ref,
			 updated_at=CURRENT_TIMESTAMP`,
			e.ID, e.Title, e.Title, e.Slug, string(e.Type), e.BodyOptional, e.Summary, e.BodyOptional, string(e.Status), projectID, artifactID, e.ExternalRef)
		if err != nil {
			return fmt.Errorf("import entry %s: %w", e.ID, err)
		}
	}

	for _, a := range data.Data.Artifacts {
		projectID := interface{}(nil)
		if a.ProjectID != nil {
			projectID = *a.ProjectID
		}
		sourceEntryID := interface{}(nil)
		if a.SourceEntryID != nil {
			sourceEntryID = *a.SourceEntryID
		}
		_, err := tx.ExecContext(ctx,
			`INSERT INTO artifacts (id, title, slug, type, file_path, mime_type, summary, content_hash, size_bytes, project_id, source_entry_id, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
			 ON CONFLICT(id) DO UPDATE SET
			 title=excluded.title, slug=excluded.slug,
			 type=excluded.type, file_path=excluded.file_path,
			 mime_type=excluded.mime_type, summary=excluded.summary,
			 content_hash=excluded.content_hash, size_bytes=excluded.size_bytes,
			 project_id=excluded.project_id, source_entry_id=excluded.source_entry_id,
			 updated_at=CURRENT_TIMESTAMP`,
			a.ID, a.Title, a.Slug, string(a.Type), a.FilePath, a.MimeType, a.Summary, a.ContentHash, a.SizeBytes, projectID, sourceEntryID)
		if err != nil {
			return fmt.Errorf("import artifact %s: %w", a.ID, err)
		}
	}

	for _, et := range data.Data.EntryTags {
		_, err := tx.ExecContext(ctx,
			"INSERT OR REPLACE INTO entry_tags (entry_id, tag) VALUES (?, ?)",
			et.EntryID, et.TagID)
		if err != nil {
			return fmt.Errorf("import entry_tag %s/%s: %w", et.EntryID, et.TagID, err)
		}
	}

	for _, se := range data.Data.SeriesEntries {
		_, err := tx.ExecContext(ctx,
			`INSERT OR REPLACE INTO series_entries (series_id, entry_id, order_index)
			 VALUES (?, ?, ?)`,
			se.SeriesID, se.EntryID, se.OrderIndex)
		if err != nil {
			return fmt.Errorf("import series_entry %s/%s: %w", se.SeriesID, se.EntryID, err)
		}
	}

	for _, ws := range data.Data.WorkflowSteps {
		required := 0
		if ws.Required {
			required = 1
		}
		id := interface{}(nil)
		if ws.ID != "" {
			id = ws.ID
		}
		_, err := tx.ExecContext(ctx,
			`INSERT INTO workflow_steps (id, workflow_id, order_index, title, instruction, required, expected_output)
			 VALUES (?, ?, ?, ?, ?, ?, ?)
			 ON CONFLICT(id) DO UPDATE SET
			 workflow_id=excluded.workflow_id, order_index=excluded.order_index,
			 title=excluded.title, instruction=excluded.instruction,
			 required=excluded.required, expected_output=excluded.expected_output`,
			id, ws.WorkflowID, ws.OrderIndex, ws.Title, ws.Instruction, required, ws.ExpectedOutput)
		if err != nil {
			return fmt.Errorf("import workflow_step: %w", err)
		}
	}

	for _, el := range data.Data.EntryLinks {
		active := 1
		_ = active // unused; imported entry_links are active by default
		_, err := tx.ExecContext(ctx,
			`INSERT INTO entry_links (from_entry_id, to_entry_id, relation_type, label, active)
			 VALUES (?, ?, ?, ?, 1)
			 ON CONFLICT(from_entry_id, to_entry_id, relation_type) DO UPDATE SET
			 label=excluded.label, active=1`,
			el.FromEntryID, el.ToEntryID, string(el.RelationType), el.Label)
		if err != nil {
			return fmt.Errorf("import entry_link %s/%s: %w", el.FromEntryID, el.ToEntryID, err)
		}
	}

	if _, err := tx.ExecContext(ctx, "INSERT INTO entries_fts(entries_fts) VALUES('rebuild')"); err != nil {
		return fmt.Errorf("rebuild fts after import: %w", err)
	}

	return tx.Commit()
}

func (s *sqliteImportExportStore) exportProjects(ctx context.Context) ([]domain.Project, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, name, COALESCE(slug,''), COALESCE(description,''), COALESCE(status,'active') FROM projects ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanProjects(rows)
}

func (s *sqliteImportExportStore) exportEntries(ctx context.Context) ([]domain.Entry, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, COALESCE(title,''), COALESCE(slug,''), type, project_id, COALESCE(summary,''), COALESCE(body_optional,''), COALESCE(status,'active'), artifact_id, COALESCE(external_ref,'') FROM entries ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []domain.Entry
	for rows.Next() {
		var e domain.Entry
		var projID, artID, summary, body, status sql.NullString
		if err := rows.Scan(&e.ID, &e.Title, &e.Slug, &e.Type, &projID, &summary, &body, &status, &artID, &e.ExternalRef); err != nil {
			return nil, err
		}
		e.Status = domain.Status(status.String)
		if projID.Valid {
			e.ProjectID = &projID.String
		}
		if artID.Valid {
			e.ArtifactID = &artID.String
		}
		if summary.Valid {
			e.Summary = summary.String
		}
		if body.Valid {
			e.BodyOptional = body.String
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
		if err := rows.Scan(&et.EntryID, &et.TagID); err != nil {
			return nil, err
		}
		results = append(results, et)
	}
	return results, nil
}

func (s *sqliteImportExportStore) exportTags(ctx context.Context) ([]domain.Tag, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, name, slug FROM tags ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []domain.Tag
	for rows.Next() {
		var t domain.Tag
		if err := rows.Scan(&t.ID, &t.Name, &t.Slug); err != nil {
			return nil, err
		}
		results = append(results, t)
	}
	return results, nil
}

func (s *sqliteImportExportStore) exportSeries(ctx context.Context) ([]domain.Series, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, COALESCE(slug,''), COALESCE(description,''), COALESCE(status,'active') FROM series ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []domain.Series
	for rows.Next() {
		var se domain.Series
		var status string
		if err := rows.Scan(&se.ID, &se.Name, &se.Slug, &se.Description, &status); err != nil {
			return nil, err
		}
		se.Status = domain.Status(status)
		results = append(results, se)
	}
	return results, nil
}

func (s *sqliteImportExportStore) exportSeriesEntries(ctx context.Context) ([]domain.SeriesEntry, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT series_id, entry_id, COALESCE(order_index,0) FROM series_entries ORDER BY series_id, order_index`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []domain.SeriesEntry
	for rows.Next() {
		var se domain.SeriesEntry
		if err := rows.Scan(&se.SeriesID, &se.EntryID, &se.OrderIndex); err != nil {
			return nil, err
		}
		results = append(results, se)
	}
	return results, nil
}

func (s *sqliteImportExportStore) exportWorkflows(ctx context.Context) ([]domain.Workflow, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, COALESCE(slug,''), COALESCE(description,''), COALESCE(status,'active') FROM workflows ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []domain.Workflow
	for rows.Next() {
		var w domain.Workflow
		var status string
		var desc sql.NullString
		if err := rows.Scan(&w.ID, &w.Name, &w.Slug, &desc, &status); err != nil {
			return nil, err
		}
		w.Status = domain.Status(status)
		if desc.Valid {
			w.Description = desc.String
		}
		results = append(results, w)
	}
	return results, nil
}

func (s *sqliteImportExportStore) exportWorkflowSteps(ctx context.Context) ([]domain.WorkflowStep, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, workflow_id, order_index, COALESCE(title,''), COALESCE(instruction,''), required, COALESCE(expected_output,'') FROM workflow_steps ORDER BY workflow_id, order_index`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []domain.WorkflowStep
	for rows.Next() {
		var ws domain.WorkflowStep
		var stepID int64
		var required int
		if err := rows.Scan(&stepID, &ws.WorkflowID, &ws.OrderIndex, &ws.Title, &ws.Instruction, &required, &ws.ExpectedOutput); err != nil {
			return nil, err
		}
		ws.ID = fmt.Sprintf("%d", stepID)
		ws.Required = required == 1
		results = append(results, ws)
	}
	return results, nil
}

func (s *sqliteImportExportStore) exportArtifacts(ctx context.Context) ([]domain.Artifact, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, title, COALESCE(slug,''), type, COALESCE(file_path,''), COALESCE(mime_type,''), COALESCE(summary,''), COALESCE(content_hash,''), COALESCE(size_bytes,0), project_id, source_entry_id FROM artifacts ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []domain.Artifact
	for rows.Next() {
		var a domain.Artifact
		var projID, srcID, summary sql.NullString
		if err := rows.Scan(&a.ID, &a.Title, &a.Slug, &a.Type, &a.FilePath, &a.MimeType, &summary, &a.ContentHash, &a.SizeBytes, &projID, &srcID); err != nil {
			return nil, err
		}
		if projID.Valid {
			a.ProjectID = &projID.String
		}
		if srcID.Valid {
			a.SourceEntryID = &srcID.String
		}
		if summary.Valid {
			a.Summary = summary.String
		}
		results = append(results, a)
	}
	return results, nil
}

func scanProjects(rows *sql.Rows) ([]domain.Project, error) {
	var results []domain.Project
	for rows.Next() {
		var p domain.Project
		var status string
		if err := rows.Scan(&p.ID, &p.Name, &p.Slug, &p.Description, &status); err != nil {
			return nil, err
		}
		p.Status = domain.Status(status)
		results = append(results, p)
	}
	return results, nil
}

var _ ImportExportStore = (*sqliteImportExportStore)(nil)

func (s *sqliteImportExportStore) exportEntryLinks(ctx context.Context) ([]domain.EntryLink, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT from_entry_id, to_entry_id, relation_type, COALESCE(label,'') FROM entry_links WHERE active = 1 ORDER BY from_entry_id, to_entry_id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []domain.EntryLink
	for rows.Next() {
		var el domain.EntryLink
		var rt string
		if err := rows.Scan(&el.FromEntryID, &el.ToEntryID, &rt, &el.Label); err != nil {
			return nil, err
		}
		el.RelationType = domain.RelationType(rt)
		el.Active = true
		results = append(results, el)
	}
	return results, nil
}

var _ = json.Marshal
