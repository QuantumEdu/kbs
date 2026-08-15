package security

import (
	"encoding/json"
	"fmt"
)

// SARIFReport represents the root of a SARIF v2.1.0 document.
type SARIFReport struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []SARIFRun `json:"runs"`
}

type SARIFRun struct {
	Tool    SARIFTool     `json:"tool"`
	Results []SARIFResult `json:"results"`
}

type SARIFTool struct {
	Driver SARIFDriver `json:"driver"`
}

type SARIFDriver struct {
	Name           string      `json:"name"`
	Version        string      `json:"version"`
	InformationURI string      `json:"informationUri"`
	Rules          []SARIFRule `json:"rules"`
}

type SARIFRule struct {
	ID                   string                 `json:"id"`
	Name                 string                 `json:"name"`
	ShortDescription     SARIFMessage           `json:"shortDescription"`
	DefaultConfiguration SARIFRuleConfiguration `json:"defaultConfiguration"`
}

type SARIFRuleConfiguration struct {
	Level string `json:"level"` // "error", "warning", "note"
}

type SARIFResult struct {
	RuleID    string          `json:"ruleId"`
	Level     string          `json:"level"`
	Message   SARIFMessage    `json:"message"`
	Locations []SARIFLocation `json:"locations,omitempty"`
}

type SARIFLocation struct {
	PhysicalLocation SARIFPhysicalLocation `json:"physicalLocation"`
}

type SARIFPhysicalLocation struct {
	ArtifactLocation SARIFArtifactLocation `json:"artifactLocation"`
	Region           *SARIFRegion          `json:"region,omitempty"`
}

type SARIFArtifactLocation struct {
	URI string `json:"uri"`
}

type SARIFRegion struct {
	StartLine int `json:"startLine"`
}

type SARIFMessage struct {
	Text string `json:"text"`
}

// ConvertAuditReportToSARIF translates a standard AuditReport into a SARIF v2.1.0 structure.
func ConvertAuditReportToSARIF(report AuditReport) SARIFReport {
	ruleMap := make(map[string]SARIFRule)
	results := make([]SARIFResult, 0, len(report.Findings))

	for _, f := range report.Findings {
		level := "note"
		switch f.Severity {
		case "critical", "high":
			level = "error"
		case "medium":
			level = "warning"
		case "low":
			level = "note"
		}

		if _, exists := ruleMap[f.RuleID]; !exists {
			ruleMap[f.RuleID] = SARIFRule{
				ID:   f.RuleID,
				Name: f.Category,
				ShortDescription: SARIFMessage{
					Text: f.Description,
				},
				DefaultConfiguration: SARIFRuleConfiguration{
					Level: level,
				},
			}
		}

		msg := fmt.Sprintf("[%s] %s", f.Category, f.Description)
		if f.Suggestion != "" {
			msg += fmt.Sprintf(" (Remediation: %s)", f.Suggestion)
		}

		res := SARIFResult{
			RuleID:  f.RuleID,
			Level:   level,
			Message: SARIFMessage{Text: msg},
		}

		uri := report.Target
		if uri == "" {
			uri = "vault:active-entries"
		}

		loc := SARIFLocation{
			PhysicalLocation: SARIFPhysicalLocation{
				ArtifactLocation: SARIFArtifactLocation{URI: uri},
			},
		}
		if f.LineNumber > 0 {
			loc.PhysicalLocation.Region = &SARIFRegion{StartLine: f.LineNumber}
		}
		res.Locations = []SARIFLocation{loc}

		results = append(results, res)
	}

	rules := make([]SARIFRule, 0, len(ruleMap))
	for _, r := range ruleMap {
		rules = append(rules, r)
	}

	return SARIFReport{
		Schema:  "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/master/Schemata/sarif-schema-2.1.0.json",
		Version: "2.1.0",
		Runs: []SARIFRun{
			{
				Tool: SARIFTool{
					Driver: SARIFDriver{
						Name:           "SkillVault Security Audit",
						Version:        "3.0.0",
						InformationURI: "https://github.com/QuantumEdu/kbs",
						Rules:          rules,
					},
				},
				Results: results,
			},
		},
	}
}

// FormatSARIF formats an AuditReport as indented SARIF JSON.
func FormatSARIF(report AuditReport) (string, error) {
	sarif := ConvertAuditReportToSARIF(report)
	bytes, err := json.MarshalIndent(sarif, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal sarif: %w", err)
	}
	return string(bytes), nil
}
