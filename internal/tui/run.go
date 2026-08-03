//go:build tui

package tui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/quantum-6/skillvault/internal/app"
)

func Run(entrySvc *app.EntryService, projectSvc *app.ProjectService, contextSvc *app.ContextService) {
	m := NewModel(entrySvc, projectSvc, contextSvc)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
		os.Exit(1)
	}
}
