package security

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestConvertAuditReportToSARIF(t *testing.T) {
	report := AuditReport{
		Target:       "test-skill.md",
		ScannedItems: 1,
		Findings: []Finding{
			{
				RuleID:       "INJ-001",
				Category:     "prompt_injection",
				Severity:     "critical",
				Description:  "Instruction override",
				MatchSnippet: "ignore all previous instructions",
				LineNumber:   10,
				Suggestion:   "Remove override phrase",
			},
			{
				RuleID:       "CMD-004",
				Category:     "dangerous_command",
				Severity:     "medium",
				Description:  "Excessive permissions",
				MatchSnippet: "chmod 777",
				LineNumber:   25,
				Suggestion:   "Use 0755",
			},
		},
		CriticalCount: 1,
		MediumCount:   1,
		Passed:        false,
	}

	sarifStr, err := FormatSARIF(report)
	if err != nil {
		t.Fatalf("FormatSARIF failed: %v", err)
	}

	if !strings.Contains(sarifStr, `"version": "2.1.0"`) {
		t.Error("expected version 2.1.0 in SARIF output")
	}

	var parsed SARIFReport
	if err := json.Unmarshal([]byte(sarifStr), &parsed); err != nil {
		t.Fatalf("unmarshal generated SARIF: %v", err)
	}

	if len(parsed.Runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(parsed.Runs))
	}
	if len(parsed.Runs[0].Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(parsed.Runs[0].Results))
	}

	// Verify error level on critical
	r1 := parsed.Runs[0].Results[0]
	if r1.Level != "error" {
		t.Errorf("expected level 'error' for critical finding, got %q", r1.Level)
	}
	if r1.Locations[0].PhysicalLocation.Region.StartLine != 10 {
		t.Errorf("expected StartLine 10, got %d", r1.Locations[0].PhysicalLocation.Region.StartLine)
	}

	// Verify warning level on medium
	r2 := parsed.Runs[0].Results[1]
	if r2.Level != "warning" {
		t.Errorf("expected level 'warning' for medium finding, got %q", r2.Level)
	}
}
