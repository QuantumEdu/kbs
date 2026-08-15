package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/quantum-6/skillvault/internal/security"
)

func runAudit(ctx context.Context, svc *Services, args []string) {
	flags, err := ParseAuditFlags(args)
	if err != nil {
		PrintError(err)
		os.Exit(1)
	}

	var report security.AuditReport
	if flags.PackPath != "" {
		report, err = svc.auditSvc.AuditPath(flags.PackPath)
	} else if flags.Target != "" {
		report, err = svc.auditSvc.AuditPath(flags.Target)
	} else {
		report, err = svc.auditSvc.AuditVault(ctx)
	}

	if err != nil {
		PrintError(err)
		os.Exit(1)
	}

	if flags.Format == "json" {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			PrintError(fmt.Errorf("marshal audit report: %w", err))
			os.Exit(1)
		}
		fmt.Println(string(data))
	} else if flags.Format == "sarif" {
		sarifStr, err := security.FormatSARIF(report)
		if err != nil {
			PrintError(fmt.Errorf("format sarif report: %w", err))
			os.Exit(1)
		}
		fmt.Println(sarifStr)
	} else {
		printAuditReport(report)
	}

	failed := false
	switch flags.FailOn {
	case "critical":
		failed = report.CriticalCount > 0
	case "high":
		failed = report.CriticalCount > 0 || report.HighCount > 0
	case "medium":
		failed = report.CriticalCount > 0 || report.HighCount > 0 || report.MediumCount > 0
	}

	if failed {
		os.Exit(2)
	}
}

func runMCPAudit(ctx context.Context, svc *Services, args []string) {
	flags, err := ParseMCPAuditFlags(args)
	if err != nil {
		PrintError(err)
		os.Exit(1)
	}

	auditor := security.NewMCPConfigAuditor()
	var targetPaths []string
	if flags.ConfigPath != "" {
		targetPaths = append(targetPaths, flags.ConfigPath)
	} else {
		for _, kp := range security.GetKnownConfigPaths() {
			if _, err := os.Stat(kp.Path); err == nil {
				targetPaths = append(targetPaths, kp.Path)
			}
		}
		if len(targetPaths) == 0 {
			fmt.Println("[sk-vault] No standard MCP config files found. Pass --config <path> to audit a custom file.")
			return
		}
	}

	var reports []security.MCPConfigAuditReport
	for _, p := range targetPaths {
		rep, err := auditor.AuditConfigFile(p)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[sk-vault] Warning: failed to audit %s: %v\n", p, err)
			continue
		}
		reports = append(reports, rep)
	}

	if flags.Format == "json" {
		data, err := json.MarshalIndent(reports, "", "  ")
		if err != nil {
			PrintError(fmt.Errorf("marshal mcp audit reports: %w", err))
			os.Exit(1)
		}
		fmt.Println(string(data))
	} else {
		printMCPAuditReports(reports)
	}
}

func printAuditReport(report security.AuditReport) {
	fmt.Println("======================================================================")
	fmt.Printf("SKILLVAULT SECURITY AUDIT REPORT\n")
	fmt.Printf("Target:  %s\n", report.Target)
	fmt.Printf("Scanned: %d items\n", report.ScannedItems)
	if report.Passed {
		fmt.Printf("Status:  PASSED (0 critical, 0 high, %d medium, %d low)\n", report.MediumCount, report.LowCount)
	} else {
		fmt.Printf("Status:  FAILED (%d critical, %d high, %d medium, %d low)\n", report.CriticalCount, report.HighCount, report.MediumCount, report.LowCount)
	}
	fmt.Println("======================================================================")

	if len(report.Findings) == 0 {
		fmt.Println("✓ No security risks, prompt injections, or exposed secrets detected.")
		return
	}

	for _, f := range report.Findings {
		lineStr := ""
		if f.LineNumber > 0 {
			lineStr = fmt.Sprintf(" - Line %d", f.LineNumber)
		}
		fmt.Printf("\n[%s] %s (%s)%s\n", strings.ToUpper(f.Severity), f.RuleID, f.Category, lineStr)
		fmt.Printf("  Description: %s\n", f.Description)
		if f.MatchSnippet != "" {
			fmt.Printf("  Match:       %s\n", f.MatchSnippet)
		}
		if f.Suggestion != "" {
			fmt.Printf("  Suggestion:  %s\n", f.Suggestion)
		}
	}
	fmt.Println("\n======================================================================")
}

func printMCPAuditReports(reports []security.MCPConfigAuditReport) {
	fmt.Println("======================================================================")
	fmt.Println("SKILLVAULT MCP CONFIGURATION SECURITY AUDIT")
	fmt.Printf("Configurations Scanned: %d\n", len(reports))
	fmt.Println("======================================================================")

	totalFindings := 0
	for _, rep := range reports {
		fmt.Printf("\nClient / File: %s (%s)\n", rep.ClientType, rep.ConfigPath)
		fmt.Printf("Servers Defined: %d\n", rep.ServersFound)
		if rep.Passed {
			fmt.Println("Status: PASSED (No security risks detected)")
		} else {
			fmt.Printf("Status: FAILED (%d critical, %d high, %d medium, %d low)\n",
				rep.CriticalCount, rep.HighCount, rep.MediumCount, rep.LowCount)
		}

		for _, f := range rep.Findings {
			totalFindings++
			fmt.Printf("\n  [%s] %s in [%s] -> %s\n", strings.ToUpper(f.Severity), f.RuleID, f.ServerName, f.Location)
			fmt.Printf("    Description: %s\n", f.Description)
			if f.MatchSnippet != "" {
				fmt.Printf("    Match:       %s\n", f.MatchSnippet)
			}
			if f.Suggestion != "" {
				fmt.Printf("    Suggestion:  %s\n", f.Suggestion)
			}
		}
	}

	fmt.Println("\n======================================================================")
	if totalFindings > 0 {
		fmt.Println("Recommendation: Move hardcoded secrets into SkillVault's encrypted store:")
		fmt.Println("  skillvault secrets set <KEY> <VALUE>")
		fmt.Println("======================================================================")
	}
}
