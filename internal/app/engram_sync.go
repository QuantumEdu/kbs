package app

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/quantum-6/skillvault/internal/domain"
	_ "modernc.org/sqlite"
)

// EngramSyncOptions controls how observations from Engram are imported into SkillVault.
type EngramSyncOptions struct {
	DBPath  string
	Project string
	DryRun  bool
}

// EngramSyncResult summarizes the import run.
type EngramSyncResult struct {
	Total    int
	Imported int
	Skipped  int
	Errors   int
}

// MapEngramType maps Engram observation types to SkillVault entry types.
func MapEngramType(t string) string {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "decision":
		return "decision"
	case "preference":
		return "feedback"
	case "architecture":
		return "skill"
	case "workflow":
		return "workflow_note"
	case "discovery", "pattern", "bugfix", "config":
		return "reference"
	default:
		return "reference"
	}
}

// SyncEngram imports observations from an Engram SQLite database into the vault.
func (s *EntryService) SyncEngram(ctx context.Context, opts EngramSyncOptions) (EngramSyncResult, error) {
	dbPath := opts.DBPath
	if dbPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return EngramSyncResult{}, err
		}
		dbPath = filepath.Join(home, ".engram", "engram.db")
	}

	if _, err := os.Stat(dbPath); err != nil {
		return EngramSyncResult{}, fmt.Errorf("engram database not found at %s: %w", dbPath, err)
	}

	engramDB, err := sql.Open("sqlite", dbPath+"?mode=ro")
	if err != nil {
		return EngramSyncResult{}, fmt.Errorf("open engram db: %w", err)
	}
	defer engramDB.Close()

	query := `SELECT id, type, title, content, COALESCE(project, ''), COALESCE(topic_key, '')
		FROM observations
		WHERE deleted_at IS NULL`
	var args []interface{}
	if opts.Project != "" {
		query += " AND (project = ? OR project = '')"
		args = append(args, opts.Project)
	}
	query += " ORDER BY id ASC"

	rows, err := engramDB.QueryContext(ctx, query, args...)
	if err != nil {
		return EngramSyncResult{}, fmt.Errorf("query observations: %w", err)
	}
	defer rows.Close()

	// Load existing external_refs from skillvault to avoid duplicate imports
	existingList, err := s.store.List(ctx, domain.EntryFilter{IncludeArchived: true})
	existingRefs := make(map[string]bool)
	existingTitles := make(map[string]bool)
	if err == nil {
		for _, e := range existingList {
			if e.Entry.ExternalRef != "" {
				existingRefs[e.Entry.ExternalRef] = true
			}
			existingTitles[strings.ToLower(strings.TrimSpace(e.Entry.Title))] = true
		}
	}

	var res EngramSyncResult

	for rows.Next() {
		res.Total++
		var id int64
		var rawType, title, content, proj, topicKey string
		if err := rows.Scan(&id, &rawType, &title, &content, &proj, &topicKey); err != nil {
			res.Errors++
			continue
		}

		extRef := fmt.Sprintf("engram:%d", id)
		if existingRefs[extRef] || existingTitles[strings.ToLower(strings.TrimSpace(title))] {
			res.Skipped++
			continue
		}

		if opts.DryRun {
			res.Imported++
			continue
		}

		mappedType := MapEngramType(rawType)
		targetProj := opts.Project
		if targetProj == "" && proj != "" {
			targetProj = proj
		}

		if targetProj != "" {
			if _, err := s.projectStore.Get(ctx, targetProj); err != nil {
				_ = s.projectStore.Save(ctx, domain.Project{
					ID:     targetProj,
					Name:   targetProj,
					Slug:   targetProj,
					Status: domain.StatusActive,
				})
			}
		}

		tags := []string{"engram", "engram-type:" + rawType}
		if topicKey != "" {
			tags = append(tags, "topic:"+topicKey)
		}

		input := SaveEntryInput{
			Title:       title,
			Type:        mappedType,
			Summary:     title,
			Body:        content,
			Project:     targetProj,
			Tags:        tags,
			Status:      "active",
			Purpose:     "KNOWLEDGE",
			ExternalRef: extRef,
		}

		if _, err := s.SaveEntry(ctx, input); err != nil {
			res.Errors++
			continue
		}

		existingRefs[extRef] = true
		existingTitles[strings.ToLower(strings.TrimSpace(title))] = true
		res.Imported++
	}

	return res, nil
}
