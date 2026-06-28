//go:build tui

package tui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/quantum-6/skillvault/internal/app"
)

// Run starts the Bubble Tea TUI with the given entry service.
func Run(entrySvc *app.EntryService) {
	m := NewModel(entrySvc)

	p := tea.NewProgram(
		m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
		os.Exit(1)
	}
}
