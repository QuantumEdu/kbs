package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/quantum-6/skillvault/internal/domain"
)

func (s *sqliteEntryLinkStore) Save(ctx context.Context, link domain.EntryLink) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO entry_links (from_entry_id, to_entry_id, relation_type, label, active)
		VALUES (?, ?, ?, ?, 1)
		ON CONFLICT(from_entry_id, to_entry_id, relation_type) DO UPDATE SET
			label=excluded.label
	`, link.FromEntryID, link.ToEntryID, string(link.RelationType), link.Label)
	if err != nil {
		return fmt.Errorf("save link: %w", err)
	}
	return nil
}

func (s *sqliteEntryLinkStore) GetLinks(ctx context.Context, entryID string) ([]domain.EntryLink, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT from_entry_id, to_entry_id, relation_type, COALESCE(label,''), active
		FROM entry_links
		WHERE (from_entry_id = ? OR to_entry_id = ?) AND active = 1
		ORDER BY relation_type
	`, entryID, entryID)
	if err != nil {
		return nil, fmt.Errorf("get links: %w", err)
	}
	defer rows.Close()
	return scanLinks(rows)
}

func (s *sqliteEntryLinkStore) GetLinksByType(ctx context.Context, entryID string, relationType string) ([]domain.EntryLink, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT from_entry_id, to_entry_id, relation_type, COALESCE(label,''), active
		FROM entry_links
		WHERE (from_entry_id = ? OR to_entry_id = ?) AND relation_type = ? AND active = 1
		ORDER BY relation_type
	`, entryID, entryID, relationType)
	if err != nil {
		return nil, fmt.Errorf("get links by type: %w", err)
	}
	defer rows.Close()
	return scanLinks(rows)
}

func (s *sqliteEntryLinkStore) ListRefs(ctx context.Context, filter EntryLinkFilter) ([]domain.EntryLink, error) {
	var conditions []string
	var args []interface{}

	if filter.SourceID != nil {
		conditions = append(conditions, "from_entry_id = ?")
		args = append(args, *filter.SourceID)
	}
	if filter.TargetID != nil {
		conditions = append(conditions, "to_entry_id = ?")
		args = append(args, *filter.TargetID)
	}
	if filter.RefType != nil {
		conditions = append(conditions, "relation_type = ?")
		args = append(args, *filter.RefType)
	}
	if !filter.IncludeArchived {
		conditions = append(conditions, "active = 1")
	} else if filter.Active != nil {
		a := 1
		if !*filter.Active {
			a = 0
		}
		conditions = append(conditions, fmt.Sprintf("active = %d", a))
	}

	query := "SELECT from_entry_id, to_entry_id, relation_type, COALESCE(label,''), active FROM entry_links"
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY relation_type, from_entry_id, to_entry_id"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list refs: %w", err)
	}
	defer rows.Close()
	return scanLinks(rows)
}

func (s *sqliteEntryLinkStore) RemoveRef(ctx context.Context, fromEntryID, toEntryID, relationType string) error {
	// Soft delete: set active=0 instead of hard delete
	result, err := s.db.ExecContext(ctx, `
		UPDATE entry_links SET active = 0
		WHERE from_entry_id = ? AND to_entry_id = ? AND relation_type = ?
	`, fromEntryID, toEntryID, relationType)
	if err != nil {
		return fmt.Errorf("remove ref: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("ref not found: %s -> %s (%s)", fromEntryID, toEntryID, relationType)
	}
	return nil
}

// ReachableRefs returns all entry IDs reachable from entryID following
// outgoing refs of the given type, up to maxDepth.
// Used for cycle detection.
func (s *sqliteEntryLinkStore) ReachableRefs(ctx context.Context, entryID string, refType string, maxDepth int) ([]EntryLinkNode, error) {
	if maxDepth <= 0 {
		maxDepth = 10
	}
	query := `
		WITH RECURSIVE reachable AS (
			SELECT to_entry_id AS entry_id, 1 AS depth
			FROM entry_links
			WHERE from_entry_id = ?1 AND relation_type = ?2 AND active = 1
			UNION ALL
			SELECT el.to_entry_id, r.depth + 1
			FROM entry_links el
			JOIN reachable r ON el.from_entry_id = r.entry_id
			WHERE el.relation_type = ?2 AND el.active = 1 AND r.depth < ?3
		)
		SELECT DISTINCT entry_id, depth FROM reachable ORDER BY depth`

	rows, err := s.db.QueryContext(ctx, query, entryID, refType, maxDepth)
	if err != nil {
		return nil, fmt.Errorf("reachable refs: %w", err)
	}
	defer rows.Close()
	return scanNodes(rows)
}

// GetEntryGraph returns all nodes reachable from entryID via the given ref types,
// along with all edges (entry_links) connecting them.
// direction: "outgoing" (follow source->target), "incoming" (follow target->source), "both"
func (s *sqliteEntryLinkStore) GetEntryGraph(ctx context.Context, entryID string, refTypes []string, direction string, maxDepth int) ([]EntryLinkNode, []domain.EntryLink, error) {
	if maxDepth <= 0 {
		maxDepth = 3
	}
	if maxDepth > 10 {
		maxDepth = 10
	}

	typeFilter := ""
	args := []interface{}{entryID, maxDepth}
	if len(refTypes) > 0 {
		placeholders := make([]string, len(refTypes))
		for i, rt := range refTypes {
			placeholders[i] = fmt.Sprintf("?%d", i+3)
			args = append(args, rt)
		}
		typeFilter = fmt.Sprintf(" AND el.relation_type IN (%s)", strings.Join(placeholders, ","))
	}

	// CTE depends on direction
	var cteJoin, cteBase, selectEdges string

	switch direction {
	case "incoming":
		cteBase = `SELECT from_entry_id AS entry_id, to_entry_id AS from_id, 1 AS depth FROM entry_links el WHERE el.to_entry_id = ?1 AND el.active = 1` + typeFilter
		cteJoin = `ON el.to_entry_id = r.from_id`
		selectEdges = `SELECT el.from_entry_id, el.to_entry_id, el.relation_type, COALESCE(el.label,''), el.active FROM entry_links el JOIN reachable r ON (el.to_entry_id = r.from_id OR el.from_entry_id = r.entry_id) WHERE el.active = 1`
	case "outgoing":
		cteBase = `SELECT to_entry_id AS entry_id, from_entry_id AS from_id, 1 AS depth FROM entry_links el WHERE el.from_entry_id = ?1 AND el.active = 1` + typeFilter
		cteJoin = `ON el.from_entry_id = r.from_id`
		selectEdges = `SELECT el.from_entry_id, el.to_entry_id, el.relation_type, COALESCE(el.label,''), el.active FROM entry_links el JOIN reachable r ON (el.from_entry_id = r.from_id OR el.to_entry_id = r.entry_id) WHERE el.active = 1`
	default: // both
		cteBase = `SELECT to_entry_id AS entry_id, from_entry_id AS from_id, 1 AS depth FROM entry_links el WHERE (el.from_entry_id = ?1 OR el.to_entry_id = ?1) AND el.active = 1` + typeFilter
		cteJoin = `ON (el.from_entry_id = r.from_id OR el.to_entry_id = r.entry_id)`
		selectEdges = `SELECT el.from_entry_id, el.to_entry_id, el.relation_type, COALESCE(el.label,''), el.active FROM entry_links el JOIN reachable r ON (el.from_entry_id = r.from_id OR el.to_entry_id = r.entry_id OR el.to_entry_id = r.from_id OR el.from_entry_id = r.entry_id) WHERE el.active = 1`
	}

	cteSQL := fmt.Sprintf(`
		WITH RECURSIVE reachable AS (
			%s
			UNION ALL
			SELECT el.to_entry_id AS entry_id, el.from_entry_id AS from_id, r.depth + 1
			FROM entry_links el
			JOIN reachable r %s
			WHERE el.active = 1 AND r.depth < ?2
		)`, cteBase, cteJoin)

	nodeQuery := cteSQL + " SELECT DISTINCT entry_id, MIN(depth) FROM reachable GROUP BY entry_id ORDER BY depth"
	edgeQuery := cteSQL + " " + selectEdges + " GROUP BY el.from_entry_id, el.to_entry_id, el.relation_type ORDER BY el.relation_type"

	nodeRows, err := s.db.QueryContext(ctx, nodeQuery, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("get graph nodes: %w", err)
	}
	defer nodeRows.Close()

	nodes, err := scanNodes(nodeRows)
	if err != nil {
		return nil, nil, fmt.Errorf("scan graph nodes: %w", err)
	}
	nodeRows.Close()

	edgeRows, err := s.db.QueryContext(ctx, edgeQuery, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("get graph edges: %w", err)
	}
	defer edgeRows.Close()

	edges, err := scanLinks(edgeRows)
	if err != nil {
		return nil, nil, fmt.Errorf("scan graph edges: %w", err)
	}

	return nodes, edges, nil
}

// --- helpers ---

func scanLinks(rows *sql.Rows) ([]domain.EntryLink, error) {
	var results []domain.EntryLink
	for rows.Next() {
		var link domain.EntryLink
		var rt string
		var active int
		var label sql.NullString
		if err := rows.Scan(&link.FromEntryID, &link.ToEntryID, &rt, &label, &active); err != nil {
			return nil, fmt.Errorf("scan link: %w", err)
		}
		link.RelationType = domain.RelationType(rt)
		link.Active = active == 1
		if label.Valid {
			link.Label = label.String
		}
		results = append(results, link)
	}
	if results == nil {
		results = []domain.EntryLink{}
	}
	return results, nil
}

func scanNodes(rows *sql.Rows) ([]EntryLinkNode, error) {
	var results []EntryLinkNode
	for rows.Next() {
		var n EntryLinkNode
		if err := rows.Scan(&n.EntryID, &n.Depth); err != nil {
			return nil, fmt.Errorf("scan node: %w", err)
		}
		results = append(results, n)
	}
	if results == nil {
		results = []EntryLinkNode{}
	}
	return results, nil
}
