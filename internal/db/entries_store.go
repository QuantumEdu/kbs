package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/quantum-6/skillvault/internal/domain"
)

func (s *sqliteEntryStore) Save(ctx context.Context, entry domain.Entry, tags []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	projectID := interface{}(nil)
	if entry.ProjectID != nil {
		projectID = *entry.ProjectID
	}
	artifactID := interface{}(nil)
	if entry.ArtifactID != nil {
		artifactID = *entry.ArtifactID
	}
	if entry.Status == "" {
		entry.Status = domain.StatusActive
	}
	if entry.Slug == "" {
		entry.Slug = entry.Title
	}

	tagsDenorm := strings.Join(tags, " ")

	for _, tag := range tags {
		_, _ = tx.ExecContext(ctx,
			"INSERT OR IGNORE INTO tags (id, name, slug) VALUES (?, ?, ?)",
			tag, tag, tag)
	}

	// Archive previous content before UPSERT when the entry exists and
	// title, summary, or body_optional changed.
	archiveErr := s.archiveBeforeSave(ctx, tx, entry)
	if archiveErr != nil {
		return fmt.Errorf("archive previous version: %w", archiveErr)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO entries (id, name, title, slug, type, content, summary, body_optional, purpose, status, project_id, artifact_id, external_ref, tags_denorm, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			name=excluded.name,
			title=excluded.title,
			slug=excluded.slug,
			type=excluded.type,
			content=excluded.content,
			summary=excluded.summary,
			body_optional=excluded.body_optional,
			purpose=excluded.purpose,
			status=excluded.status,
			project_id=excluded.project_id,
			artifact_id=excluded.artifact_id,
			external_ref=excluded.external_ref,
			tags_denorm=excluded.tags_denorm,
			updated_at=CURRENT_TIMESTAMP
	`, entry.ID, entry.Title, entry.Title, entry.Slug, string(entry.Type), entry.BodyOptional, entry.Summary, entry.BodyOptional, string(entry.Purpose), string(entry.Status), projectID, artifactID, entry.ExternalRef, tagsDenorm)
	if err != nil {
		return fmt.Errorf("upsert entry: %w", err)
	}

	if _, err := tx.ExecContext(ctx, "DELETE FROM entry_tags WHERE entry_id = ?", entry.ID); err != nil {
		return fmt.Errorf("delete old tags: %w", err)
	}
	for _, tag := range tags {
		if _, err := tx.ExecContext(ctx, "INSERT INTO entry_tags (entry_id, tag) VALUES (?, ?)", entry.ID, tag); err != nil {
			return fmt.Errorf("insert tag %q: %w", tag, err)
		}
	}

	if err := s.syncFTS(ctx, tx, entry.ID, entry.Title, entry.Summary, entry.BodyOptional, entry.ExternalRef, tagsDenorm); err != nil {
		return fmt.Errorf("sync FTS5: %w", err)
	}

	return tx.Commit()
}

// archiveBeforeSave inserts a version row for the current entry content
// when the entry exists and its title, summary, or body_optional changed.
// Runs inside the caller's transaction.
func (s *sqliteEntryStore) archiveBeforeSave(ctx context.Context, tx *sql.Tx, newEntry domain.Entry) error {
	// Look up the existing entry to compare content.
	var oldTitle, oldSummary, oldBody string
	err := tx.QueryRowContext(ctx,
		`SELECT title, COALESCE(summary, ''), COALESCE(body_optional, '')
		 FROM entries WHERE id = ?`, newEntry.ID,
	).Scan(&oldTitle, &oldSummary, &oldBody)
	if err != nil {
		if err == sql.ErrNoRows {
			// New entry — nothing to archive.
			return nil
		}
		return fmt.Errorf("lookup existing entry: %w", err)
	}

	// Only archive if content actually changed.
	if oldTitle == newEntry.Title && oldSummary == newEntry.Summary && oldBody == newEntry.BodyOptional {
		return nil
	}

	// Calculate next version number.
	var maxVersion int
	err = tx.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(version_number), 0) FROM entry_versions WHERE entry_id = ?`,
		newEntry.ID,
	).Scan(&maxVersion)
	if err != nil {
		return fmt.Errorf("max version number: %w", err)
	}
	nextVersion := maxVersion + 1

	// Generate version ID.
	versionID := generateVersionID()

	_, err = tx.ExecContext(ctx,
		`INSERT INTO entry_versions (version_id, entry_id, version_number, title, summary, body_optional, saved_at)
		 VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		versionID, newEntry.ID, nextVersion, oldTitle, oldSummary, oldBody,
	)
	if err != nil {
		return fmt.Errorf("insert version row: %w", err)
	}

	return nil
}

func (s *sqliteEntryStore) Get(ctx context.Context, id string, includeArchived bool) (domain.EntryResult, error) {
	var result domain.EntryResult
	var projectID sql.NullString
	var artifactID sql.NullString
	var summary sql.NullString
	var bodyOptional sql.NullString
	var status string

	err := s.db.QueryRowContext(ctx, `
		SELECT id, title, slug, type, purpose, project_id, summary, body_optional, status, artifact_id, COALESCE(external_ref,'')
		FROM entries WHERE (id = ? OR slug = ?)
	`, id, id).Scan(&result.Entry.ID, &result.Entry.Title, &result.Entry.Slug, &result.Entry.Type,
		&result.Entry.Purpose, &projectID, &summary, &bodyOptional, &status, &artifactID, &result.Entry.ExternalRef)
	if err == sql.ErrNoRows {
		return result, fmt.Errorf("entry %q not found", id)
	}
	if err != nil {
		return result, fmt.Errorf("get entry: %w", err)
	}

	result.Entry.Status = domain.Status(status)
	if projectID.Valid {
		result.Entry.ProjectID = &projectID.String
	}
	if artifactID.Valid {
		result.Entry.ArtifactID = &artifactID.String
	}
	if summary.Valid {
		result.Entry.Summary = summary.String
	}
	if bodyOptional.Valid {
		result.Entry.BodyOptional = bodyOptional.String
	}

	if !includeArchived && result.Entry.Status == domain.StatusArchived {
		return result, fmt.Errorf("archived: entry %q exists but is archived. Retry with include_archived=true.", id)
	}

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
		result.Tags = append(result.Tags, domain.Tag{ID: tag, Name: tag, Slug: tag})
	}

	return result, nil
}

func (s *sqliteEntryStore) Search(ctx context.Context, q domain.SearchQuery) ([]domain.EntrySearchResult, error) {
	var conditions []string
	var args []interface{}

	if q.Query != "" {
		conditions = append(conditions, "e_fts.entries_fts MATCH ?")
		args = append(args, q.Query)
	}

	query := `SELECT e.id, e.title, e.slug, e.type, e.purpose, e.project_id, e.summary, e.body_optional, e.status, e.artifact_id, COALESCE(e.external_ref,'')
		FROM entries_fts e_fts
		JOIN entries e ON e.id = e_fts.id`

	if len(conditions) > 0 || !q.IncludeArchived || q.ProjectID != nil || q.Type != nil {
		if len(conditions) > 0 {
			query += " WHERE " + conditions[0]
			for _, c := range conditions[1:] {
				query += " AND " + c
			}
		} else {
			query += " WHERE 1=1"
		}

		if !q.IncludeArchived {
			query += " AND e.status != 'archived'"
		}
		if q.ProjectID != nil {
			query += " AND e.project_id = ?"
			args = append(args, *q.ProjectID)
		}
		if q.Type != nil {
			query += " AND e.type = ?"
			args = append(args, *q.Type)
		}
		if q.Purpose != nil && *q.Purpose != "" {
			query += " AND e.purpose = ?"
			args = append(args, *q.Purpose)
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
		var artifactID sql.NullString
		var summary sql.NullString
		var bodyOptional sql.NullString
		var status string
		if err := rows.Scan(&r.Entry.ID, &r.Entry.Title, &r.Entry.Slug, &r.Entry.Type, &r.Entry.Purpose, &projectID,
			&summary, &bodyOptional, &status, &artifactID, &r.Entry.ExternalRef); err != nil {
			return nil, fmt.Errorf("scan search result: %w", err)
		}
		r.Entry.Status = domain.Status(status)
		if projectID.Valid {
			r.Entry.ProjectID = &projectID.String
		}
		if artifactID.Valid {
			r.Entry.ArtifactID = &artifactID.String
		}
		if summary.Valid {
			r.Entry.Summary = summary.String
		}
		if bodyOptional.Valid {
			r.Entry.BodyOptional = bodyOptional.String
		}

		tagRows, err := s.db.QueryContext(ctx, "SELECT tag FROM entry_tags WHERE entry_id = ? ORDER BY tag", r.Entry.ID)
		if err == nil {
			for tagRows.Next() {
				var tag string
				if err := tagRows.Scan(&tag); err != nil {
					break
				}
				r.Tags = append(r.Tags, domain.Tag{ID: tag, Name: tag, Slug: tag})
			}
			tagRows.Close()
		}
		if r.Tags == nil {
			r.Tags = []domain.Tag{}
		}

		results = append(results, r)
	}

	if results == nil {
		results = []domain.EntrySearchResult{}
	}
	return results, nil
}

func (s *sqliteEntryStore) SearchByTags(ctx context.Context, tags []string, matchAll bool, typePtr, projectPtr *string, limit int) ([]domain.EntrySearchResult, error) {
	if len(tags) == 0 {
		return nil, fmt.Errorf("at least one tag is required")
	}
	if limit <= 0 {
		limit = 20
	}

	query := "SELECT e.id, e.title, e.slug, e.type, e.purpose, e.project_id, e.summary, e.body_optional, e.status, e.artifact_id, COALESCE(e.external_ref,'') FROM entries e JOIN entry_tags et ON e.id = et.entry_id WHERE e.status != 'archived' AND et.tag IN (" + placeholders(len(tags)) + ")"
	var args []interface{}
	for _, t := range tags {
		args = append(args, t)
	}

	if typePtr != nil {
		query += " AND e.type = ?"
		args = append(args, *typePtr)
	}
	if projectPtr != nil {
		query += " AND e.project_id = ?"
		args = append(args, *projectPtr)
	}

	if matchAll {
		query += " GROUP BY e.id HAVING COUNT(DISTINCT et.tag) = ?"
		args = append(args, len(tags))
	} else {
		query += " GROUP BY e.id"
	}

	query += " ORDER BY e.title LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("search by tags: %w", err)
	}
	defer rows.Close()

	type row struct {
		result       domain.EntrySearchResult
		projectID    sql.NullString
		artifactID   sql.NullString
		summary      sql.NullString
		bodyOptional sql.NullString
		status       string
	}
	var rowsData []row
	var entryIDs []string

	for rows.Next() {
		var r row
		if err := rows.Scan(&r.result.Entry.ID, &r.result.Entry.Title, &r.result.Entry.Slug, &r.result.Entry.Type, &r.result.Entry.Purpose,
			&r.projectID, &r.summary, &r.bodyOptional, &r.status, &r.artifactID, &r.result.Entry.ExternalRef); err != nil {
			return nil, fmt.Errorf("scan search by tags: %w", err)
		}
		r.result.Entry.Status = domain.Status(r.status)
		if r.projectID.Valid {
			r.result.Entry.ProjectID = &r.projectID.String
		}
		if r.artifactID.Valid {
			r.result.Entry.ArtifactID = &r.artifactID.String
		}
		if r.summary.Valid {
			r.result.Entry.Summary = r.summary.String
		}
		if r.bodyOptional.Valid {
			r.result.Entry.BodyOptional = r.bodyOptional.String
		}
		rowsData = append(rowsData, r)
		entryIDs = append(entryIDs, r.result.Entry.ID)
	}
	rows.Close()

	tagMap := make(map[string][]string)
	if len(entryIDs) > 0 {
		tagRows, err := s.db.QueryContext(ctx, "SELECT entry_id, tag FROM entry_tags WHERE entry_id IN ("+placeholders(len(entryIDs))+") ORDER BY entry_id, tag",
			strSliceToInterface(entryIDs)...)
		if err != nil {
			return nil, fmt.Errorf("get tags for search: %w", err)
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

	results := make([]domain.EntrySearchResult, 0, len(rowsData))
	for _, r := range rowsData {
		tagNames := tagMap[r.result.Entry.ID]
		for _, tn := range tagNames {
			r.result.Tags = append(r.result.Tags, domain.Tag{ID: tn, Name: tn, Slug: tn})
		}
		if r.result.Tags == nil {
			r.result.Tags = []domain.Tag{}
		}
		results = append(results, r.result)
	}

	return results, nil
}

func (s *sqliteEntryStore) Archive(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, "UPDATE entries SET status = 'archived', updated_at = CURRENT_TIMESTAMP WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("archive entry: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("entry %q not found", id)
	}
	return nil
}

func (s *sqliteEntryStore) List(ctx context.Context, filter domain.EntryFilter) ([]domain.EntryListResult, error) {
	query := "SELECT e.id, e.title, e.slug, e.type, e.purpose, e.project_id, e.summary, e.body_optional, e.status, e.artifact_id, COALESCE(e.external_ref,'') FROM entries e WHERE 1=1"
	var args []interface{}

	if !filter.IncludeArchived {
		query += " AND e.status != 'archived'"
	}
	if filter.ProjectID != nil {
		query += " AND e.project_id = ?"
		args = append(args, *filter.ProjectID)
	}
	if filter.Type != nil {
		query += " AND e.type = ?"
		args = append(args, *filter.Type)
	}
	query += " ORDER BY e.title"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list entries: %w", err)
	}
	defer rows.Close()

	type row struct {
		entry        domain.EntryListResult
		projID       sql.NullString
		artifactID   sql.NullString
		summary      sql.NullString
		bodyOptional sql.NullString
		status       string
	}
	var rowsData []row
	var entryIDs []string

	for rows.Next() {
		var r row
		if err := rows.Scan(&r.entry.Entry.ID, &r.entry.Entry.Title, &r.entry.Entry.Slug, &r.entry.Entry.Type, &r.entry.Entry.Purpose,
			&r.projID, &r.summary, &r.bodyOptional, &r.status, &r.artifactID, &r.entry.Entry.ExternalRef); err != nil {
			return nil, fmt.Errorf("scan entry: %w", err)
		}
		r.entry.Entry.Status = domain.Status(r.status)
		if r.projID.Valid {
			r.entry.Entry.ProjectID = &r.projID.String
		}
		if r.artifactID.Valid {
			r.entry.Entry.ArtifactID = &r.artifactID.String
		}
		if r.summary.Valid {
			r.entry.Entry.Summary = r.summary.String
		}
		if r.bodyOptional.Valid {
			r.entry.Entry.BodyOptional = r.bodyOptional.String
		}
		rowsData = append(rowsData, r)
		entryIDs = append(entryIDs, r.entry.Entry.ID)
	}
	rows.Close()

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
		tagNames := tagMap[r.entry.Entry.ID]
		for _, tn := range tagNames {
			r.entry.Tags = append(r.entry.Tags, domain.Tag{ID: tn, Name: tn, Slug: tn})
		}
		if r.entry.Tags == nil {
			r.entry.Tags = []domain.Tag{}
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

func (s *sqliteEntryStore) syncFTS(ctx context.Context, tx *sql.Tx, id, title, summary, body, externalRef, tagsDenorm string) error {
	if _, err := tx.ExecContext(ctx, "DELETE FROM entries_fts WHERE id = ?", id); err != nil {
		return fmt.Errorf("delete from fts: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO entries_fts (id, title, summary, body_optional, tags_denorm, external_ref) VALUES (?, ?, ?, ?, ?, ?)",
		id, title, summary, body, tagsDenorm, externalRef,
	); err != nil {
		return fmt.Errorf("insert into fts: %w", err)
	}
	return nil
}
