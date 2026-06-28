//go:build !tui

package main

import (
	"fmt"
	"os"
)

func runTUI(svc *vaultServices) {
	_ = svc
	fmt.Fprintln(os.Stderr, "TUI not available. Rebuild with: go build -tags tui ./cmd/skillvault")
	os.Exit(1)
}
