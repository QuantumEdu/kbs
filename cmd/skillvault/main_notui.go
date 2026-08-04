//go:build !tui

package main

import (
	"fmt"
	"os"

	"github.com/quantum-6/skillvault/internal/cli"
)

func runTUI(svc *cli.Services) {
	_ = svc
	fmt.Fprintln(os.Stderr, "TUI not available. Rebuild with: go build -tags tui ./cmd/skillvault")
	os.Exit(1)
}
