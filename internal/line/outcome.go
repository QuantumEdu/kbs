package line

import (
	"encoding/json"
	"fmt"
	"strings"
)

type OutcomeStatus string

const (
	OutcomeOK         OutcomeStatus = "ok"
	OutcomeNeedsHuman OutcomeStatus = "needs_human"
	OutcomeBlocked    OutcomeStatus = "blocked"
)

type Outcome struct {
	Status   OutcomeStatus
	Question string
	Summary  string
}

type outcomeJSON struct {
	Status   string `json:"status"`
	Question string `json:"question"`
	Summary  string `json:"summary"`
}

// ParseOutcome extracts the last JSON object in stdout as a classified phase result.
func ParseOutcome(stdout string) (Outcome, error) {
	raw, err := lastJSONObject(stdout)
	if err != nil {
		return Outcome{}, err
	}
	var parsed outcomeJSON
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return Outcome{}, fmt.Errorf("parse outcome json: %w", err)
	}
	status := OutcomeStatus(strings.TrimSpace(parsed.Status))
	switch status {
	case OutcomeOK, OutcomeNeedsHuman, OutcomeBlocked:
	default:
		return Outcome{}, fmt.Errorf("unknown outcome status %q", parsed.Status)
	}
	if status == OutcomeNeedsHuman && strings.TrimSpace(parsed.Question) == "" {
		return Outcome{}, fmt.Errorf("needs_human outcome requires a question")
	}
	return Outcome{
		Status:   status,
		Question: strings.TrimSpace(parsed.Question),
		Summary:  strings.TrimSpace(parsed.Summary),
	}, nil
}

func lastJSONObject(stdout string) (string, error) {
	s := strings.TrimSpace(stdout)
	end := strings.LastIndex(s, "}")
	if end < 0 {
		return "", fmt.Errorf("no json object in agent output")
	}
	start := strings.LastIndex(s[:end+1], "{")
	if start < 0 {
		return "", fmt.Errorf("no json object in agent output")
	}
	return s[start : end+1], nil
}
