package db

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/quantum-6/skillvault/internal/domain"
)

// PhaseYAML represents a single phase from a workflow-builder YAML file.
type PhaseYAML struct {
	ID                 string   `yaml:"id"`
	Name               string   `yaml:"name"`
	Skill              string   `yaml:"skill"`
	Description        string   `yaml:"description"`
	Outputs            []string `yaml:"outputs"`
	CompletionCriteria []string `yaml:"completion_criteria"`
	DependsOn          []string `yaml:"depends_on"`
}

// WorkflowYAML is the top-level structure of a workflow-builder YAML file.
type WorkflowYAML struct {
	Workflow struct {
		Name    string `yaml:"name"`
		Type    string `yaml:"type"`
		Created string `yaml:"created"`
	} `yaml:"workflow"`
	Phases []PhaseYAML `yaml:"phases"`
}

// ImportWorkflowWithEntries imports a workflow-builder YAML file into a single
// SQLite transaction. It creates skill entries for each phase, a Workflow row,
// and WorkflowStep rows linking entries to the workflow.
//
// Returns the created Workflow, the list of entry slugs, and any error.
// On failure the entire transaction is rolled back.
func (s *Store) ImportWorkflowWithEntries(ctx context.Context, yamlData []byte, projectID *string) (*domain.Workflow, []string, error) {
	var wfYAML WorkflowYAML
	if err := yaml.Unmarshal(yamlData, &wfYAML); err != nil {
		return nil, nil, fmt.Errorf("parse workflow YAML: %w", err)
	}

	if len(wfYAML.Phases) == 0 {
		return nil, nil, fmt.Errorf("workflow must have at least one phase")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// 1. Create skill entries for each phase
	var entrySlugs []string
	for _, phase := range wfYAML.Phases {
		slug, err := s.insertPhaseEntry(ctx, tx, phase, projectID)
		if err != nil {
			return nil, nil, fmt.Errorf("insert phase entry %q: %w", phase.ID, err)
		}
		entrySlugs = append(entrySlugs, slug)
	}

	// 2. Create the Workflow (with slug collision handling)
	wfID, err := generateImportWorkflowID()
	if err != nil {
		return nil, nil, fmt.Errorf("generate workflow id: %w", err)
	}
	wfSlug := slugifyForImport(wfYAML.Workflow.Name)

	finalSlug := wfSlug
	counter := 2
	for {
		var existingID string
		err := tx.QueryRowContext(ctx, "SELECT id FROM workflows WHERE slug = ?", finalSlug).Scan(&existingID)
		if err != nil {
			// No row found — slug is available
			break
		}
		finalSlug = fmt.Sprintf("%s-%d", wfSlug, counter)
		counter++
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO workflows (id, name, slug, description, status, updated_at)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`, wfID, wfYAML.Workflow.Name, finalSlug, "Auto-imported from YAML", string(domain.StatusActive))
	if err != nil {
		return nil, nil, fmt.Errorf("insert workflow: %w", err)
	}

	// 3. Create WorkflowStep rows
	for i, slug := range entrySlugs {
		phase := wfYAML.Phases[i]
		instruction := phase.Description
		if phase.Skill != "" {
			instruction = phase.Skill
		}
		_, err := tx.ExecContext(ctx,
			`INSERT INTO workflow_steps (workflow_id, order_index, title, instruction, required, expected_output, entry_slug)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			wfID, i+1, phase.Name, instruction, 1, "", slug)
		if err != nil {
			return nil, nil, fmt.Errorf("insert step %d: %w", i+1, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, nil, fmt.Errorf("commit: %w", err)
	}

	wf := &domain.Workflow{
		ID:          wfID,
		Name:        wfYAML.Workflow.Name,
		Slug:        finalSlug,
		Description: "Auto-imported from YAML",
		Status:      domain.StatusActive,
	}
	return wf, entrySlugs, nil
}

// insertPhaseEntry creates a skill-type entry for a single phase inside the
// given transaction. Returns the entry's slug.
func (s *Store) insertPhaseEntry(ctx context.Context, tx *sql.Tx, phase PhaseYAML, projectID *string) (string, error) {
	tags := []string{"workflow-phase", phase.ID}
	sort.Strings(tags)

	bodyYAML, err := buildPhaseSkillYAML(phase)
	if err != nil {
		return "", err
	}

	entryID, err := generateImportWorkflowID()
	if err != nil {
		return "", fmt.Errorf("generate entry id: %w", err)
	}
	entrySlug := slugifyForImport(phase.Name)
	finalSlug := entrySlug
	counter := 2
	for {
		var existingID string
		err := tx.QueryRowContext(ctx, "SELECT id FROM entries WHERE slug = ?", finalSlug).Scan(&existingID)
		if err != nil {
			break
		}
		finalSlug = fmt.Sprintf("%s-%d", entrySlug, counter)
		counter++
	}

	projVal := interface{}(nil)
	if projectID != nil {
		projVal = *projectID
	}

	tagsDenorm := strings.Join(tags, " ")

	// Upsert tags
	for _, tag := range tags {
		if _, err := tx.ExecContext(ctx,
			"INSERT OR IGNORE INTO tags (id, name, slug) VALUES (?, ?, ?)",
			tag, tag, tag); err != nil {
			return "", fmt.Errorf("upsert tag %q: %w", tag, err)
		}
	}

	// Upsert entry
	_, err = tx.ExecContext(ctx, `
		INSERT INTO entries (id, name, title, slug, type, content, summary, body_optional, status, project_id, tags_denorm, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`, entryID, phase.Name, phase.Name, finalSlug, string(domain.EntryTypeSkill), "", phase.Description, bodyYAML, string(domain.StatusActive), projVal, tagsDenorm)
	if err != nil {
		return "", fmt.Errorf("upsert entry: %w", err)
	}

	// Delete old entry_tags (for idempotency)
	if _, err := tx.ExecContext(ctx, "DELETE FROM entry_tags WHERE entry_id = ?", entryID); err != nil {
		return "", fmt.Errorf("delete old entry tags: %w", err)
	}

	// Insert entry_tags
	for _, tag := range tags {
		if _, err := tx.ExecContext(ctx, "INSERT INTO entry_tags (entry_id, tag) VALUES (?, ?)", entryID, tag); err != nil {
			return "", fmt.Errorf("insert tag %q: %w", tag, err)
		}
	}

	// Sync FTS
	if _, err := tx.ExecContext(ctx, "DELETE FROM entries_fts WHERE id = ?", entryID); err != nil {
		return "", fmt.Errorf("delete from fts: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO entries_fts (id, title, summary, body_optional, tags_denorm, external_ref) VALUES (?, ?, ?, ?, ?, ?)",
		entryID, phase.Name, phase.Description, bodyYAML, tagsDenorm, "",
	); err != nil {
		return "", fmt.Errorf("insert into fts: %w", err)
	}

	return finalSlug, nil
}

// phaseSkillBody is the serializable shape stored in body_optional.
type phaseSkillBody struct {
	Name               string   `yaml:"name"`
	Skill              string   `yaml:"skill,omitempty"`
	Description        string   `yaml:"description,omitempty"`
	Outputs            []string `yaml:"outputs,omitempty"`
	CompletionCriteria []string `yaml:"completion_criteria,omitempty"`
	DependsOn          []string `yaml:"depends_on,omitempty"`
}

// buildPhaseSkillYAML builds a YAML body for a phase-skill template entry.
func buildPhaseSkillYAML(phase PhaseYAML) (string, error) {
	body := phaseSkillBody{
		Name:               phase.Name,
		Skill:              phase.Skill,
		Description:        phase.Description,
		Outputs:            phase.Outputs,
		CompletionCriteria: phase.CompletionCriteria,
		DependsOn:          phase.DependsOn,
	}
	out, err := yaml.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshal phase skill body: %w", err)
	}
	return strings.TrimRight(string(out), "\n"), nil
}

// slugifyForImport creates a lowercase slug from a name for import IDs.
func slugifyForImport(name string) string {
	s := strings.ToLower(name)
	var result strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == ' ' {
			if r == ' ' {
				result.WriteByte('-')
			} else {
				result.WriteRune(r)
			}
		}
	}
	slug := result.String()
	// Collapse consecutive dashes
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "imported-entry"
	}
	return slug
}

// generateImportWorkflowID generates a random ID for imported workflows/entries.
func generateImportWorkflowID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	return "imp-" + hex.EncodeToString(b), nil
}
