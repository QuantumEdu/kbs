package app

import (
	"context"
	"fmt"

	"github.com/quantum-6/skillvault/internal/db"
	"github.com/quantum-6/skillvault/internal/domain"
	"github.com/quantum-6/skillvault/internal/security"
)

// AuditService coordinates security audits across vault entries, files, and packs.
type AuditService struct {
	entryStore db.EntryStore
	auditor    *security.Auditor
}

// NewAuditService creates a new AuditService.
func NewAuditService(entryStore db.EntryStore, auditor *security.Auditor) *AuditService {
	if auditor == nil {
		auditor = security.NewAuditor()
	}
	return &AuditService{
		entryStore: entryStore,
		auditor:    auditor,
	}
}

// AuditVault audits all active entries in the vault database.
func (s *AuditService) AuditVault(ctx context.Context) (security.AuditReport, error) {
	filter := domain.EntryFilter{
		IncludeArchived: false,
	}

	results, err := s.entryStore.List(ctx, filter)
	if err != nil {
		return security.AuditReport{}, fmt.Errorf("list vault entries for audit: %w", err)
	}

	report := security.AuditReport{
		Target:       "vault:active-entries",
		Findings:     []security.Finding{},
		ScannedItems: len(results),
	}

	for _, res := range results {
		entry := res.Entry
		contentToScan := fmt.Sprintf("Title: %s\nSummary: %s\nBody: %s", entry.Title, entry.Summary, entry.BodyOptional)
		targetLabel := fmt.Sprintf("entry:%s (%s)", entry.ID, entry.Slug)
		
		sub := s.auditor.AuditContent(targetLabel, contentToScan)
		report.Findings = append(report.Findings, sub.Findings...)
		report.CriticalCount += sub.CriticalCount
		report.HighCount += sub.HighCount
		report.MediumCount += sub.MediumCount
		report.LowCount += sub.LowCount
	}

	report.Passed = report.CriticalCount == 0 && report.HighCount == 0
	return report, nil
}

// AuditPath audits a file, directory, or pack file.
func (s *AuditService) AuditPath(path string) (security.AuditReport, error) {
	return s.auditor.AuditFile(path)
}

// AuditPackBytes audits a raw JSON or .svpack payload.
func (s *AuditService) AuditPackBytes(data []byte) (security.AuditReport, error) {
	return s.auditor.AuditPack(data)
}

// Auditor returns the underlying security.Auditor.
func (s *AuditService) Auditor() *security.Auditor {
	return s.auditor
}
