package security

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/quantum-6/skillvault/internal/domain"
)

// Finding represents a single security issue identified during audit.
type Finding struct {
	RuleID       string `json:"rule_id"`
	Category     string `json:"category"` // "prompt_injection", "secret_leak", "dangerous_command"
	Severity     string `json:"severity"` // "critical", "high", "medium", "low"
	Description  string `json:"description"`
	MatchSnippet string `json:"match_snippet"`
	LineNumber   int    `json:"line_number,omitempty"`
	Suggestion   string `json:"suggestion"`
}

// AuditReport summarizes findings for a target (file, pack, vault, or entry).
type AuditReport struct {
	Target        string    `json:"target"`
	ScannedItems  int       `json:"scanned_items"`
	Findings      []Finding `json:"findings"`
	Passed        bool      `json:"passed"`
	CriticalCount int       `json:"critical_count"`
	HighCount     int       `json:"high_count"`
	MediumCount   int       `json:"medium_count"`
	LowCount      int       `json:"low_count"`
}

type auditRule struct {
	id          string
	category    string
	severity    string
	description string
	suggestion  string
	re          *regexp.Regexp
}

var auditRules = []auditRule{
	// Prompt Injection & Jailbreak
	{
		id:          "INJ-001",
		category:    "prompt_injection",
		severity:    "critical",
		description: "Instruction override or system prompt hijacking phrase detected",
		suggestion:  "Remove instruction override phrases; use proper parameter boundaries instead",
		re:          regexp.MustCompile(`(?i)\b(ignore\s+(all\s+)?(previous|prior)\s+instructions|disregard\s+(all\s+)?(previous|prior)\s+instructions|system\s+override)\b`),
	},
	{
		id:          "INJ-002",
		category:    "prompt_injection",
		severity:    "high",
		description: "Jailbreak mode or safety filter bypass marker detected",
		suggestion:  "Avoid jailbreak trigger phrases in skill definitions",
		re:          regexp.MustCompile(`(?i)\b(you\s+are\s+now\s+in\s+developer\s+mode|dan\s+mode\s+enabled|jailbreak\s+(mode|enabled|active)|bypass\s+(all\s+)?safety\s+filters)\b`),
	},
	{
		id:          "INJ-003",
		category:    "prompt_injection",
		severity:    "high",
		description: "System tag spoofing or delimiter injection detected",
		suggestion:  "Sanitize XML/delimiter tags that mimic LLM system instructions",
		re:          regexp.MustCompile(`(?i)(<\s*/?\s*(system|admin|override)\s*>|\\u003c\s*/?\s*(system|admin|override)\s*\\u003e|\[\s*(system|system_prompt)\s*\])`),
	},
	{
		id:          "INJ-004",
		category:    "prompt_injection",
		severity:    "medium",
		description: "Prompt exfiltration or verbatim prompt leak probe detected",
		suggestion:  "Ensure prompts do not encourage exfiltrating internal instructions",
		re:          regexp.MustCompile(`(?i)\b(repeat\s+the\s+above\s+prompt|print\s+(your\s+)?initial\s+instructions\s+verbatim|output\s+the\s+system\s+prompt)\b`),
	},

	// Secrets & Credentials
	{
		id:          "SEC-001",
		category:    "secret_leak",
		severity:    "critical",
		description: "Known API key or service token detected in plaintext",
		suggestion:  "Store credentials using q-secrets or environment variables",
		re:          regexp.MustCompile(`(sk-[A-Za-z0-9_-]{20,}|sk-ant-[A-Za-z0-9_-]{20,}|ghp_[A-Za-z0-9_]{20,}|xox[baprs]-[A-Za-z0-9-]{20,}|AKIA[0-9A-Z]{16})`),
	},
	{
		id:          "SEC-002",
		category:    "secret_leak",
		severity:    "critical",
		description: "Private cryptographic key block detected",
		suggestion:  "Remove private keys from skills; store in secure keyrings",
		re:          regexp.MustCompile(`-----BEGIN (RSA |EC |OPENSSH |)?PRIVATE KEY-----`),
	},
	{
		id:          "SEC-003",
		category:    "secret_leak",
		severity:    "high",
		description: "Hardcoded Bearer authentication token detected",
		suggestion:  "Use dynamic authentication headers or vault secrets",
		re:          regexp.MustCompile(`Bearer\s+[A-Za-z0-9_\-\.]{25,}`),
	},

	// Dangerous Commands & System Hazards
	{
		id:          "CMD-001",
		category:    "dangerous_command",
		severity:    "critical",
		description: "Destructive filesystem deletion or disk wiping command",
		suggestion:  "Restrict shell commands to safe, non-destructive operations",
		re:          regexp.MustCompile(`(?i)\b(rm\s+-(?:rf|fr|r|f)\s+([/~]|\*|\S+)|mkfs\.[a-z0-9]+|dd\s+if=\S+\s+of=/dev/\S*)`),
	},
	{
		id:          "CMD-002",
		category:    "dangerous_command",
		severity:    "high",
		description: "Unsafe remote code execution via pipe to shell",
		suggestion:  "Download and verify scripts before execution; avoid curl | sh",
		re:          regexp.MustCompile(`(?i)\b(curl|wget)\s+[^\n|;]+\|\s*(bash|sh|zsh)\b`),
	},
	{
		id:          "CMD-003",
		category:    "dangerous_command",
		severity:    "critical",
		description: "Potential reverse shell or raw network execution command",
		suggestion:  "Remove interactive network socket binds from skills",
		re:          regexp.MustCompile(`(?i)\b(nc\s+-[a-z0-9]*e\s+/bin/(ba)?sh|/dev/tcp/[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+)`),
	},
	{
		id:          "CMD-004",
		category:    "dangerous_command",
		severity:    "medium",
		description: "Excessively permissive permission modification (chmod 777)",
		suggestion:  "Use least-privilege permission masks (e.g. 0755 or 0600)",
		re:          regexp.MustCompile(`(?i)\b(chmod\s+(-R\s+)?777|chmod\s+u\+s)\b`),
	},
}

// Auditor executes static security rules against skills, prompts, files, and packs.
type Auditor struct {
	minEntropyRatio float64
	minEntropyLen   int
}

// NewAuditor creates a new Auditor instance with default settings.
func NewAuditor() *Auditor {
	return &Auditor{
		minEntropyRatio: 0.80,
		minEntropyLen:   28,
	}
}

// AuditContent scans a text payload for security findings.
func (a *Auditor) AuditContent(target string, content string) AuditReport {
	report := AuditReport{
		Target:       target,
		ScannedItems: 1,
		Findings:     []Finding{},
	}

	lines := strings.Split(content, "\n")

	// 1. Line-by-line regex scanning
	for lineNum, line := range lines {
		for _, rule := range auditRules {
			locs := rule.re.FindAllString(line, -1)
			for _, match := range locs {
				snippet := match
				if len(snippet) > 60 {
					snippet = snippet[:57] + "..."
				}
				// Redact actual secret content in snippet for safety
				if rule.category == "secret_leak" {
					snippet = redactSnippet(snippet)
				}

				finding := Finding{
					RuleID:       rule.id,
					Category:     rule.category,
					Severity:     rule.severity,
					Description:  rule.description,
					MatchSnippet: snippet,
					LineNumber:   lineNum + 1,
					Suggestion:   rule.suggestion,
				}
				report.Findings = append(report.Findings, finding)
			}
		}
	}

	// 2. High-entropy token scanner (catch opaque tokens not covered by specific regexes)
	entropyFindings := a.scanEntropyTokens(content)
	report.Findings = append(report.Findings, entropyFindings...)

	// 3. Compute counts and pass status
	for _, f := range report.Findings {
		switch f.Severity {
		case "critical":
			report.CriticalCount++
		case "high":
			report.HighCount++
		case "medium":
			report.MediumCount++
		case "low":
			report.LowCount++
		}
	}

	report.Passed = report.CriticalCount == 0 && report.HighCount == 0
	return report
}

// AuditFile reads and audits a local file or directory.
func (a *Auditor) AuditFile(path string) (AuditReport, error) {
	info, err := os.Stat(path)
	if err != nil {
		return AuditReport{}, fmt.Errorf("stat path %q: %w", path, err)
	}

	if info.IsDir() {
		return a.auditDirectory(path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return AuditReport{}, fmt.Errorf("read file %q: %w", path, err)
	}

	// Check if this is a .svpack or JSON export file
	if strings.HasSuffix(path, ".svpack") || strings.HasSuffix(path, ".json") {
		return a.AuditPack(data)
	}

	return a.AuditContent(path, string(data)), nil
}

func (a *Auditor) auditDirectory(dirPath string) (AuditReport, error) {
	combinedReport := AuditReport{
		Target:   dirPath,
		Findings: []Finding{},
	}

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		// Only scan relevant text files
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".md" && ext != ".txt" && ext != ".json" && ext != ".svpack" && ext != ".yaml" && ext != ".yml" {
			return nil
		}

		subReport, err := a.AuditFile(path)
		if err != nil {
			return nil // Skip unreadable
		}

		combinedReport.ScannedItems++
		combinedReport.Findings = append(combinedReport.Findings, subReport.Findings...)
		combinedReport.CriticalCount += subReport.CriticalCount
		combinedReport.HighCount += subReport.HighCount
		combinedReport.MediumCount += subReport.MediumCount
		combinedReport.LowCount += subReport.LowCount
		return nil
	})

	if err != nil {
		return combinedReport, err
	}

	combinedReport.Passed = combinedReport.CriticalCount == 0 && combinedReport.HighCount == 0
	return combinedReport, nil
}

// AuditPack audits the contents of an exported pack or JSON vault structure.
func (a *Auditor) AuditPack(data []byte) (AuditReport, error) {
	report := AuditReport{
		Target:   "pack-payload",
		Findings: []Finding{},
	}

	var pack domain.VaultPackExport
	if err := json.Unmarshal(data, &pack); err == nil && pack.Pack.PackID != "" {
		report.Target = fmt.Sprintf("pack:%s", pack.Pack.PackID)
		return a.auditExportData(report, pack.Data)
	}

	var export domain.VaultExport
	if err := json.Unmarshal(data, &export); err == nil {
		report.Target = "vault-export"
		return a.auditExportData(report, export)
	}

	// Fallback to raw text audit
	return a.AuditContent("raw-payload", string(data)), nil
}

func (a *Auditor) auditExportData(baseReport AuditReport, export domain.VaultExport) (AuditReport, error) {
	report := baseReport

	// Scan entries
	for _, entry := range export.Data.Entries {
		report.ScannedItems++
		contentToScan := entry.Title + "\n" + entry.Summary + "\n" + entry.BodyOptional
		sub := a.AuditContent(fmt.Sprintf("entry:%s (%s)", entry.ID, entry.Slug), contentToScan)
		report.Findings = append(report.Findings, sub.Findings...)
		report.CriticalCount += sub.CriticalCount
		report.HighCount += sub.HighCount
		report.MediumCount += sub.MediumCount
		report.LowCount += sub.LowCount
	}

	// Scan workflows
	for _, wf := range export.Data.Workflows {
		report.ScannedItems++
		contentToScan := wf.Name + "\n" + wf.Description
		sub := a.AuditContent(fmt.Sprintf("workflow:%s", wf.ID), contentToScan)
		report.Findings = append(report.Findings, sub.Findings...)
		report.CriticalCount += sub.CriticalCount
		report.HighCount += sub.HighCount
		report.MediumCount += sub.MediumCount
		report.LowCount += sub.LowCount
	}

	report.Passed = report.CriticalCount == 0 && report.HighCount == 0
	return report, nil
}

func (a *Auditor) scanEntropyTokens(content string) []Finding {
	var findings []Finding
	scanner := bufio.NewScanner(strings.NewReader(content))
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		words := strings.Fields(line)
		for _, w := range words {
			clean := strings.Trim(w, `"',;:[]{}()<>=` )
			if len(clean) < a.minEntropyLen {
				continue
			}
			// Skip markdown links or standard words
			if strings.HasPrefix(clean, "http://") || strings.HasPrefix(clean, "https://") {
				continue
			}
			alpha := 0
			for _, r := range clean {
				if unicode.IsLetter(r) || unicode.IsDigit(r) {
					alpha++
				}
			}
			ratio := float64(alpha) / float64(len(clean))
			if ratio >= a.minEntropyRatio && len(clean) >= 32 {
				// Potential unclassified secret
				findings = append(findings, Finding{
					RuleID:       "SEC-004",
					Category:     "secret_leak",
					Severity:     "high",
					Description:  "Suspicious high-entropy token detected (potential raw token/key)",
					MatchSnippet: redactSnippet(clean),
					LineNumber:   lineNum,
					Suggestion:   "Verify if this token is a secret and move it to q-secrets",
				})
			}
		}
	}
	return findings
}

func redactSnippet(s string) string {
	if len(s) <= 8 {
		return "[REDACTED]"
	}
	return s[:4] + "..." + s[len(s)-4:]
}
