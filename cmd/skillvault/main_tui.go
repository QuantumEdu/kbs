//go:build tui

package main

import "github.com/quantum-6/skillvault/internal/tui"

func runTUI(svc *vaultServices) {
	tui.Run(svc.entrySvc, svc.projectSvc, svc.contextSvc)
}
