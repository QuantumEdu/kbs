package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/quantum-6/skillvault/internal/domain"
)

// UpsertEntry creates or updates an entry along with its tags and workflow steps.
// All operations run in a single transaction.
func (s *sqliteEntryStore) UpsertEntry(ctx context.Context, entry domain.Entry, tags []string, steps []domain.WorkflowStep) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	active := 0
	if entry.Active {
		active = 1
	}
	projectID := interface{}(nil)
	if entry.ProjectID != nil {
		projectID = *entry.ProjectID
	}

	// Build denormalized tags string for FTS5
	tagsDenorm := strings.Join(tags, " ")

	_, err = tx.ExecContext(ctx, `
		INSERT INTO entries (id, name, type, project_id, description, content, vars, tags_denorm, active, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			name=excluded.name,
			type=excluded.type,
			project_id=excluded.project_id,
			description=excluded.description,
			content=excluded.content,
			vars=excluded.vars,
			tags_denorm=excluded.tags_denorm,
			active=excluded.active,
			updated_at=CURRENT_TIMESTAMP
	`, entry.ID, entry.Name, string(entry.Type), projectID, entry.Description, entry.Content, entry.Vars, tagsDenorm, active)
	if err != nil {
		return fmt.Errorf("upsert entry: %w", err)
	}

	// Replace tags: delete old, insert new
	if _, err := tx.ExecContext(ctx, "DELETE FROM entry_tags WHERE entry_id = ?", entry.ID); err != nil {
		return fmt.Errorf("delete old tags: %w", err)
	}
	for _, tag := range tags {
		if _, err := tx.ExecContext(ctx, "INSERT INTO entry_tags (entry_id, tag) VALUES (?, ?)", entry.ID, tag); err != nil {
			return fmt.Errorf("insert tag %q: %w", tag, err)
		}
	}

	// Replace workflow steps if provided
	if _, err := tx.ExecContext(ctx, "DELETE FROM workflow_steps WHERE entry_id = ?", entry.ID); err != nil {
		return fmt.Errorf("delete old steps: %w", err)
	}
	for _, step := range steps {
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO workflow_steps (entry_id, step_num, role, content, label) VALUES (?, ?, ?, ?, ?)",
			entry.ID, step.StepNum, string(step.Role), step.Content, step.Label,
		); err != nil {
			return fmt.Errorf("insert step %d: %w", step.StepNum, err)
		}
	}

	// Sync FTS5
	if err := s.syncFTS(ctx, tx, entry.ID, entry.Name, entry.Description, entry.Content, tagsDenorm); err != nil {
		return fmt.Errorf("sync FTS5: %w", err)
	}

	return tx.Commit()
}

// GetEntry retrieves an entry by ID, optionally including archived entries.
// Returns an error if the entry is archived and includeArchived is false.
func (s *sqliteEntryStore) GetEntry(ctx context.Context, id string, includeArchived bool) (domain.EntryResult, error) {
	var result domain.EntryResult
	var projectID sql.NullString
	var description sql.NullString
	var vars sql.NullString
	var active int

	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, type, project_id, description, content, vars, active
		FROM entries WHERE id = ?
	`, id).Scan(&result.Entry.ID, &result.Entry.Name, &result.Entry.Type, &projectID,
		&description, &result.Entry.Content, &vars, &active)
	if err == sql.ErrNoRows {
		return result, fmt.Errorf("entry %q not found", id)
	}
	if err != nil {
		return result, fmt.Errorf("get entry: %w", err)
	}

	result.Entry.Active = active == 1
	if projectID.Valid {
		result.Entry.ProjectID = &projectID.String
	}
	if description.Valid {
		result.Entry.Description = description.String
	}
	if vars.Valid {
		result.Entry.Vars = vars.String
	}

	// Archived check
	if !result.Entry.Active && !includeArchived {
		return result, fmt.Errorf("archived: entry %q exists but is archived. Retry with include_archived=true.", id)
	}

	// Load tags
	tagRows, err := s.db.QueryContext(ctx, "SELECT tag FROM entry_tags WHERE entry_id = ? ORDER BY tag", id)
	if err != nil {
		return result, fmt.Errorf("get tags: %w", err)
	}
	defer tagRows.Close()
	for tagRows.Next() {
		var tag string
		if err := tagRows.Scan(&tag); err != nil {
			return result, fmt.Errorf("scan tag: %w", err)
		}
		result.Tags = append(result.Tags, tag)
	}

	// Load workflow steps
	stepRows, err := s.db.QueryContext(ctx,
		"SELECT id, entry_id, step_num, role, content, COALESCE(label,'') FROM workflow_steps WHERE entry_id = ? ORDER BY step_num", id)
	if err != nil {
		return result, fmt.Errorf("get steps: %w", err)
	}
	defer stepRows.Close()
	for stepRows.Next() {
		var step domain.WorkflowStep
		if err := stepRows.Scan(&step.ID, &step.EntryID, &step.StepNum, &step.Role, &step.Content, &step.Label); err != nil {
			return result, fmt.Errorf("scan step: %w", err)
		}
		result.Steps = append(result.Steps, step)
	}

	return result, nil
}

// ListEntries returns entries matching the filter criteria.
func (s *sqliteEntryStore) ListEntries(ctx context.Context, filter domain.EntryFilter) ([]domain.EntryListResult, error) {
	query := "SELECT e.id, e.name, e.type, e.project_id, e.description, e.content, e.vars, e.active FROM entries e WHERE 1=1"
	var args []interface{}

	if !filter.IncludeArchived {
		query += " AND e.active = 1"
	}
	if filter.ProjectID != nil {
		query += " AND e.project_id = ?"
		args = append(args, *filter.ProjectID)
	}
	if filter.Type != nil {
		query += " AND e.type = ?"
		args = append(args, *filter.Type)
	}
	query += " ORDER BY e.name"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list entries: %w", err)
	}
	defer rows.Close()

	type row struct {
		entry   domain.EntryListResult
		projID  sql.NullString
		desc    sql.NullString
		vars    sql.NullString
		active  int
	}
	var rowsData []row
	var entryIDs []string

	for rows.Next() {
		var r row
		if err := rows.Scan(&r.entry.Entry.ID, &r.entry.Entry.Name, &r.entry.Entry.Type, &r.projID,
			&r.desc, &r.entry.Entry.Content, &r.vars, &r.active); err != nil {
			return nil, fmt.Errorf("scan entry: %w", err)
		}
		r.entry.Entry.Active = r.active == 1
		if r.projID.Valid {
			r.entry.Entry.ProjectID = &r.projID.String
		}
		if r.desc.Valid {
			r.entry.Entry.Description = r.desc.String
		}
		if r.vars.Valid {
			r.entry.Entry.Vars = r.vars.String
		}
		rowsData = append(rowsData, r)
		entryIDs = append(entryIDs, r.entry.Entry.ID)
	}
	rows.Close()

	// Load tags separately to avoid nested query issues
	tagMap := make(map[string][]string)
	if len(entryIDs) > 0 {
		tagRows, err := s.db.QueryContext(ctx, "SELECT entry_id, tag FROM entry_tags WHERE entry_id IN ("+placeholders(len(entryIDs))+") ORDER BY entry_id, tag",
			strSliceToInterface(entryIDs)...)
		if err != nil {
			return nil, fmt.Errorf("get tags: %w", err)
		}
		defer tagRows.Close()
		for tagRows.Next() {
			var eid, tag string
			if err := tagRows.Scan(&eid, &tag); err != nil {
				return nil, fmt.Errorf("scan tag: %w", err)
			}
			tagMap[eid] = append(tagMap[eid], tag)
		}
		tagRows.Close()
	}

	results := make([]domain.EntryListResult, 0, len(rowsData))
	for _, r := range rowsData {
		r.entry.Tags = tagMap[r.entry.Entry.ID]
		if r.entry.Tags == nil {
			r.entry.Tags = []string{}
		}
		results = append(results, r.entry)
	}

	return results, nil
}

func placeholders(n int) string {
	if n <= 0 {
		return "('')"
	}
	parts := make([]string, n)
	for i := range parts {
		parts[i] = "?"
	}
	return strings.Join(parts, ",")
}

func strSliceToInterface(s []string) []interface{} {
	result := make([]interface{}, len(s))
	for i, v := range s {
		result[i] = v
	}
	return result
}

// ArchiveEntry soft-deletes an entry by setting active=0.
func (s *sqliteEntryStore) ArchiveEntry(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, "UPDATE entries SET active = 0, updated_at = CURRENT_TIMESTAMP WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("archive entry: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("entry %q not found", id)
	}
	return nil
}

// syncFTS keeps the FTS5 index in sync with the entries table.
func (s *sqliteEntryStore) syncFTS(ctx context.Context, tx *sql.Tx, id, name, description, content, tagsDenorm string) error {
	// Delete existing FTS entry
	if _, err := tx.ExecContext(ctx, "DELETE FROM entries_fts WHERE id = ?", id); err != nil {
		return fmt.Errorf("delete from fts: %w", err)
	}
	// Insert into FTS
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO entries_fts (id, name, description, content, tags_denorm) VALUES (?, ?, ?, ?, ?)",
		id, name, description, content, tagsDenorm,
	); err != nil {
		return fmt.Errorf("insert into fts: %w", err)
	}
	return nil
}
