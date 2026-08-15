//go:build tui

package main

import (
	"github.com/quantum-6/skillvault/internal/cli"
	"github.com/quantum-6/skillvault/internal/tui"
)

func runTUI(svc *cli.Services) {
	tui.Run(svc.EntryService(), svc.ProjectService(), svc.ContextService())
}
