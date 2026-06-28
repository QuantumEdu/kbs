package app

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"

	"github.com/quantum-6/skillvault/internal/domain"
	"github.com/quantum-6/skillvault/internal/sync"
)

// SyncService wires vault export/import through a sync.Transport for cloud push/pull.
type SyncService struct {
	exportSvc *VaultExportService
	importSvc *VaultImportService
	transport sync.Transport
}

// NewSyncService creates a SyncService backed by the given transport.
// The transport should typically be a *sync.GzipTransport wrapping an S3 or GitHub backend.
// Use SetTransport to configure or replace the transport after construction.
func NewSyncService(exportSvc *VaultExportService, importSvc *VaultImportService, transport sync.Transport) *SyncService {
	return &SyncService{
		exportSvc: exportSvc,
		importSvc: importSvc,
		transport: transport,
	}
}

// SetTransport replaces the current transport (useful for CLI flag-based configuration).
func (s *SyncService) SetTransport(t sync.Transport) {
	s.transport = t
}

// Push exports the full vault as JSON, streams it through the transport,
// and uploads it to the remote path. When dryRun is true, the export is
// compressed to measure the payload size but no transfer occurs.
func (s *SyncService) Push(ctx context.Context, remotePath string, dryRun bool) error {
	if s.exportSvc == nil {
		return fmt.Errorf("export service is nil")
	}
	if !dryRun && s.transport == nil {
		return fmt.Errorf("transport is nil: no sync backend configured")
	}

	jsonBytes, err := s.exportSvc.ExportJSON(ctx)
	if err != nil {
		return fmt.Errorf("export: %w", err)
	}

	if dryRun {
		size, err := compressedSize(jsonBytes)
		if err != nil {
			return fmt.Errorf("compress measure: %w", err)
		}
		fmt.Printf("Dry-run: %d bytes compressed, remote path: %s\n", size, remotePath)
		return nil
	}

	return s.transport.Push(ctx, bytes.NewReader(jsonBytes), remotePath)
}

// Pull downloads the vault snapshot from the remote path via the transport,
// decompresses it, and imports the data into the vault. When dryRun is true,
// the pull is skipped and only informational output is printed.
func (s *SyncService) Pull(ctx context.Context, remotePath string, dryRun bool) error {
	if s.importSvc == nil {
		return fmt.Errorf("import service is nil")
	}
	if !dryRun && s.transport == nil {
		return fmt.Errorf("transport is nil: no sync backend configured")
	}

	if dryRun {
		fmt.Printf("Dry-run: would pull from %s\n", remotePath)
		return nil
	}

	var buf bytes.Buffer
	if err := s.transport.Pull(ctx, &buf, remotePath); err != nil {
		return fmt.Errorf("pull: %w", err)
	}

	var data domain.VaultExport
	if err := json.Unmarshal(buf.Bytes(), &data); err != nil {
		return fmt.Errorf("unmarshal vault data: %w", err)
	}

	return s.importSvc.ImportVault(ctx, data)
}

// compressedSize returns the gzip-compressed size of data in bytes.
func compressedSize(data []byte) (int, error) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	if _, err := gw.Write(data); err != nil {
		return 0, fmt.Errorf("gzip write: %w", err)
	}
	if err := gw.Close(); err != nil {
		return 0, fmt.Errorf("gzip close: %w", err)
	}
	return buf.Len(), nil
}
