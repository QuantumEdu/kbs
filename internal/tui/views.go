//go:build tui

package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/quantum-6/skillvault/internal/domain"
)

// ---- styles ----

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("63")).
			MarginBottom(1)

	searchBarStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63")).
			Padding(0, 1)

	entryNormalStyle = lipgloss.NewStyle().
				Padding(0, 1)

	entrySelectedStyle = lipgloss.NewStyle().
				Padding(0, 1).
				Background(lipgloss.Color("63")).
				Foreground(lipgloss.Color("255")).
				Bold(true)

	loadingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Italic(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginTop(1)

	detailHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("63")).
				MarginBottom(1)

	detailLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("244")).
				Width(10)

	detailValueStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252"))

	bodyStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(1, 2)

	tagStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Italic(true)
)

// typeBadge returns a styled badge for an entry type.
func typeBadge(t domain.EntryType) string {
	var color string
	switch t {
	case domain.EntryTypePrompt:
		color = "75"
	case domain.EntryTypeSkill:
		color = "42"
	case domain.EntryTypeDecision:
		color = "208"
	case domain.EntryTypeSession:
		color = "141"
	case domain.EntryTypeReference:
		color = "39"
	case domain.EntryTypeProjectState:
		color = "178"
	default:
		color = "244"
	}
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(color)).
		Bold(true).
		Render(string(t))
}

// statusBadge returns a styled badge for a status.
func statusBadge(s domain.Status) string {
	var color string
	switch s {
	case domain.StatusActive:
		color = "42"
	case domain.StatusDraft:
		color = "244"
	case domain.StatusArchived:
		color = "241"
	case domain.StatusDeprecated:
		color = "196"
	case domain.StatusCanonical:
		color = "75"
	default:
		color = "244"
	}
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(color)).
		Render(string(s))
}

// ---- browse view ----

func browseView(m *model) string {
	var b strings.Builder

	// Title
	b.WriteString(titleStyle.Render("SkillVault"))
	b.WriteByte('\n')

	// Search bar
	searchBar := searchBarStyle.Render(m.searchInput.View())
	b.WriteString(searchBar)
	b.WriteString("\n\n")

	// Loading indicator
	if m.loading && len(m.entries) == 0 {
		b.WriteString(loadingStyle.Render("Loading entries…"))
		b.WriteByte('\n')
		return b.String()
	}

	// Error
	if m.err != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
		b.WriteByte('\n')
		return b.String()
	}

	// Entry count
	countStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	b.WriteString(countStyle.Render(fmt.Sprintf("%d entries", len(m.entries))))
	b.WriteByte('\n')

	if len(m.entries) == 0 {
		b.WriteString(loadingStyle.Render("No entries found."))
		b.WriteByte('\n')
		// Help
		b.WriteString(helpStyle.Render("/ search • ↑↓ navigate • enter view • q quit"))
		return b.String()
	}

	// Available height for entries (subtract header, search bar, count, help).
	// Use 5 lines for chrome, rest for entries.
	visibleHeight := m.height - 6
	if visibleHeight < 1 {
		visibleHeight = 10
	}

	// Scroll window.
	start := m.cursor - visibleHeight/2
	if start < 0 {
		start = 0
	}
	end := start + visibleHeight
	if end > len(m.entries) {
		end = len(m.entries)
		start = end - visibleHeight
		if start < 0 {
			start = 0
		}
	}

	// Render entries
	maxWidth := m.width - 4
	if maxWidth < 20 {
		maxWidth = 80
	}
	for i := start; i < end; i++ {
		e := m.entries[i].Entry
		line := fmt.Sprintf("%s  %s  %s",
			typeBadge(e.Type),
			truncate(e.Title, maxWidth-30),
			statusBadge(e.Status),
		)
		if i == m.cursor {
			b.WriteString(entrySelectedStyle.Render(line))
		} else {
			b.WriteString(entryNormalStyle.Render(line))
		}
		b.WriteByte('\n')
	}

	// Help footer
	b.WriteString(helpStyle.Render("/ search • ↑↓ navigate • enter view • q quit"))

	return b.String()
}

// ---- detail view ----

func detailView(m *model) string {
	var b strings.Builder

	// Back button hint
	backHint := lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render("← esc/q back to list")
	b.WriteString(backHint)
	b.WriteByte('\n')

	// Error case
	if m.detailErr != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", m.detailErr)))
		return b.String()
	}

	// Loading
	if m.loading && m.detail == nil {
		b.WriteString(loadingStyle.Render("Loading entry…"))
		return b.String()
	}

	if m.detail == nil {
		b.WriteString("No entry selected.")
		return b.String()
	}

	e := m.detail.Entry.Entry
	tags := m.detail.Entry.Tags

	// Title
	b.WriteString(detailHeaderStyle.Render(e.Title))
	b.WriteByte('\n')

	// Metadata table
	fmt.Fprintf(&b, "%s%s\n", detailLabelStyle.Render("ID:"), detailValueStyle.Render(e.ID))
	fmt.Fprintf(&b, "%s%s\n", detailLabelStyle.Render("Type:"), typeBadge(e.Type))
	fmt.Fprintf(&b, "%s%s\n", detailLabelStyle.Render("Status:"), statusBadge(e.Status))

	project := "global"
	if e.ProjectID != nil {
		project = *e.ProjectID
	}
	fmt.Fprintf(&b, "%s%s\n", detailLabelStyle.Render("Project:"), detailValueStyle.Render(project))

	if len(tags) > 0 {
		tagNames := make([]string, len(tags))
		for i, t := range tags {
			tagNames[i] = t.Name
		}
		fmt.Fprintf(&b, "%s%s\n", detailLabelStyle.Render("Tags:"), tagStyle.Render(strings.Join(tagNames, ", ")))
	}

	if e.Summary != "" {
		b.WriteByte('\n')
		fmt.Fprintf(&b, "%s\n", detailLabelStyle.Render("Summary:"))
		b.WriteString(detailValueStyle.Render(e.Summary))
		b.WriteByte('\n')
	}

	// Body content
	if e.BodyOptional != "" {
		b.WriteByte('\n')
		b.WriteString(bodyStyle.Render(e.BodyOptional))
		b.WriteByte('\n')
	}

	// Artifact link
	if m.detail.Artifact != nil {
		b.WriteByte('\n')
		fmt.Fprintf(&b, "%s\n", detailLabelStyle.Render("Artifact:"))
		fmt.Fprintf(&b, "  ID:    %s\n", m.detail.Artifact.ID)
		fmt.Fprintf(&b, "  Title: %s\n", m.detail.Artifact.Title)
		fmt.Fprintf(&b, "  Type:  %s\n", m.detail.Artifact.Type)
	}

	return b.String()
}

// truncate shortens a string to n runes, appending … if truncated.
func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n-1]) + "…"
}
