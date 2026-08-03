//go:build tui

package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/quantum-6/skillvault/internal/domain"
)

var (
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	helpStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	mutedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))

	paneStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1)

	focusedPaneStyle = paneStyle.Copy().BorderForeground(lipgloss.Color("63"))

	selectedLineStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Background(lipgloss.Color("63")).Bold(true)
	detailHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63")).MarginBottom(1)
	detailLabelStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Width(10)
	detailValueStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	bodyStyle         = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240")).Padding(1, 2)
	tagStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Italic(true)
)

func dashboardView(m *model) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("SkillVault TUI"))
	b.WriteByte('\n')
	b.WriteString(mutedStyle.Render("Project overview, pending review, entry browsing, and compact context preview."))
	b.WriteString("\n")
	b.WriteString(headerLine(m))
	if m.statusMessage != "" {
		b.WriteByte('\n')
		if m.statusIsError {
			b.WriteString(errorStyle.Render(m.statusMessage))
		} else {
			b.WriteString(successStyle.Render(m.statusMessage))
		}
	}
	b.WriteString("\n\n")

	if m.loading {
		b.WriteString(mutedStyle.Render("Loading project surfaces..."))
		b.WriteString("\n")
	}
	if m.err != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
		b.WriteString("\n")
	}

	if len(m.projects) == 0 {
		b.WriteString(renderPane("Projects", "No projects yet. Create one with `skillvault project start --name \"MyApp\"`.", m.focus == focusProjects, m.width-2, 8))
		b.WriteString("\n")
		b.WriteString(helpFooter(m))
		return b.String()
	}

	if m.width < 110 {
		b.WriteString(renderNarrowLayout(m))
	} else {
		b.WriteString(renderWideLayout(m))
	}

	b.WriteString("\n")
	b.WriteString(helpFooter(m))
	return b.String()
}

func renderWideLayout(m *model) string {
	leftWidth := min(34, max(28, m.width/4))
	rightWidth := max(50, m.width-leftWidth-3)
	topHeight := max(8, (m.height-9)/2)
	contextHeight := m.contextPaneHeight()
	halfRight := (rightWidth - 1) / 2

	projects := renderProjectsPane(m, leftWidth, topHeight+contextHeight+1)
	pending := renderPendingPane(m, halfRight, topHeight)
	search := renderSearchPane(m, rightWidth-halfRight-1, topHeight)
	context := renderContextPane(m, rightWidth, contextHeight)

	rightTop := lipgloss.JoinHorizontal(lipgloss.Top, pending, search)
	right := lipgloss.JoinVertical(lipgloss.Left, rightTop, context)
	return lipgloss.JoinHorizontal(lipgloss.Top, projects, right)
}

func renderNarrowLayout(m *model) string {
	width := max(40, m.width-2)
	parts := []string{
		renderProjectsPane(m, width, 8),
		renderPendingPane(m, width, 8),
		renderSearchPane(m, width, 10),
		renderContextPane(m, width, m.contextPaneHeight()+2),
	}
	return strings.Join(parts, "\n")
}

func renderProjectsPane(m *model, width, height int) string {
	lines := make([]string, 0, len(m.projects)+1)
	for i, project := range m.projects {
		stats := m.pendingStatsFor(project.ID)
		line := fmt.Sprintf("%s  %s", statusBadge(project.Status), truncate(project.Name, width-12))
		line += fmt.Sprintf("\n%s", mutedStyle.Render(truncate(projectPendingHint(stats), width-6)))
		if i == m.projectCursor {
			lines = append(lines, selectedLineStyle.Render(line))
		} else {
			lines = append(lines, line)
		}
	}
	return renderPane("Projects", fitLines(lines, height-2), m.focus == focusProjects, width, height)
}

func renderPendingPane(m *model, width, height int) string {
	title := fmt.Sprintf("Pending (%d)", len(m.pending))
	if m.confirmResolve {
		title += " [confirm done]"
	}
	if len(m.pending) == 0 {
		return renderPane(title, "No active pending items for this project.", m.focus == focusPending, width, height)
	}

	lines := make([]string, 0, len(m.pending))
	if m.confirmResolve {
		if item, ok := m.selectedPendingItem(); ok {
			lines = append(lines, errorStyle.Render(truncate(fmt.Sprintf("Mark done? y/enter yes, esc no: %s", item.Entry.Title), width-4)))
		}
	}
	for i, item := range m.pending {
		text := truncate(item.Entry.Title, width-8)
		if note := strings.TrimSpace(item.Entry.BodyOptional); note != "" {
			text += fmt.Sprintf("\n%s", mutedStyle.Render(truncate(note, width-6)))
		}
		if i == m.pendingCursor {
			lines = append(lines, selectedLineStyle.Render(text))
		} else {
			lines = append(lines, text)
		}
	}
	return renderPane(title, fitLines(lines, height-2), m.focus == focusPending, width, height)
}

func renderSearchPane(m *model, width, height int) string {
	query := m.searchQuery
	if query == "" {
		query = "all entries"
	}
	title := fmt.Sprintf("Browse (%d)", len(m.results))
	lines := []string{searchBarStyle(m.searchInput.View(), m.searchInput.Focused()), mutedStyle.Render("Query: " + query)}
	if len(m.results) == 0 {
		lines = append(lines, "No matching entries for this project.")
		return renderPane(title, fitLines(lines, height-2), m.focus == focusSearch, width, height)
	}

	for i, result := range m.results {
		row := fmt.Sprintf("%s  %s", typeBadge(result.Entry.Type), truncate(result.Entry.Title, width-14))
		if result.Entry.Summary != "" {
			row += fmt.Sprintf("\n%s", mutedStyle.Render(truncate(result.Entry.Summary, width-6)))
		}
		if i == m.resultCursor {
			lines = append(lines, selectedLineStyle.Render(row))
		} else {
			lines = append(lines, row)
		}
	}
	return renderPane(title, fitLines(lines, height-2), m.focus == focusSearch, width, height)
}

func renderContextPane(m *model, width, height int) string {
	lines := contextLines(m.contextPreview)
	visible := lines
	if len(lines) > height-2 {
		start := min(m.contextOffset, max(0, len(lines)-(height-2)))
		end := min(len(lines), start+height-2)
		visible = lines[start:end]
	}
	if len(lines) > len(visible) {
		visible = append(visible, mutedStyle.Render(fmt.Sprintf("[%d/%d]", min(len(lines), m.contextOffset+len(visible)), len(lines))))
	}
	return renderPane(contextPaneTitle(m), strings.Join(visible, "\n"), m.focus == focusContext, width, height)
}

func renderPane(title, body string, focused bool, width, height int) string {
	style := paneStyle
	if focused {
		style = focusedPaneStyle
	}
	content := titleStyle.Render(title) + "\n" + body
	return style.Width(width).Height(height).Render(content)
}

func searchBarStyle(content string, focused bool) string {
	style := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	if focused {
		style = style.BorderForeground(lipgloss.Color("63"))
	} else {
		style = style.BorderForeground(lipgloss.Color("240"))
	}
	return style.Render(content)
}

func headerLine(m *model) string {
	project := "no project selected"
	pending := ""
	if len(m.projects) > 0 && m.projectCursor < len(m.projects) {
		project = m.projects[m.projectCursor].Name
		stats := m.pendingStatsFor(m.projects[m.projectCursor].ID)
		pending = fmt.Sprintf(" | Pending: %s", projectPendingHint(stats))
	}
	return mutedStyle.Render(fmt.Sprintf("Project: %s%s | Focus: %s", project, pending, focusLabel(m.focus)))
}

func helpFooter(m *model) string {
	if m.confirmResolve {
		return helpStyle.Render("y confirm done | esc cancel | q quit")
	}
	return helpStyle.Render("tab switch pane | j/k move | / filter entries | enter open detail | x mark pending done | r reload | esc back | q quit")
}

func focusLabel(area focusArea) string {
	switch area {
	case focusProjects:
		return "projects"
	case focusPending:
		return "pending"
	case focusSearch:
		return "browse"
	case focusContext:
		return "context"
	default:
		return "dashboard"
	}
}

func fitLines(lines []string, maxLines int) string {
	if maxLines <= 0 {
		return ""
	}
	var out []string
	used := 0
	for _, line := range lines {
		pieces := strings.Split(line, "\n")
		if used+len(pieces) > maxLines {
			remaining := maxLines - used
			if remaining > 0 {
				out = append(out, pieces[:remaining]...)
			}
			break
		}
		out = append(out, pieces...)
		used += len(pieces)
	}
	return strings.Join(out, "\n")
}

func contextPaneTitle(m *model) string {
	if len(m.projects) == 0 || m.projectCursor >= len(m.projects) {
		return "Context Preview"
	}
	stats := m.pendingStatsFor(m.projects[m.projectCursor].ID)
	return fmt.Sprintf("Context Preview (%s)", projectPendingHint(stats))
}

func projectPendingHint(stats pendingStats) string {
	total := stats.Open + stats.Done
	if total == 0 {
		return "clear"
	}
	return fmt.Sprintf("%d open | %d/%d done", stats.Open, stats.Done, total)
}

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
	case domain.EntryTypePending:
		color = "220"
	case domain.EntryTypeRouting:
		color = "213"
	default:
		color = "244"
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Bold(true).Render(string(t))
}

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
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(string(s))
}

func detailView(m *model) string {
	var b strings.Builder
	b.WriteString(mutedStyle.Render("esc back to dashboard | q quit"))
	b.WriteByte('\n')

	if m.detailErr != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", m.detailErr)))
		return b.String()
	}
	if m.loading && m.detail == nil {
		b.WriteString(mutedStyle.Render("Loading entry..."))
		return b.String()
	}
	if m.detail == nil {
		b.WriteString("No entry selected.")
		return b.String()
	}

	e := m.detail.Entry.Entry
	tags := m.detail.Entry.Tags
	b.WriteString(detailHeaderStyle.Render(e.Title))
	b.WriteByte('\n')
	fmt.Fprintf(&b, "%s%s\n", detailLabelStyle.Render("ID:"), detailValueStyle.Render(e.ID))
	fmt.Fprintf(&b, "%s%s\n", detailLabelStyle.Render("Type:"), typeBadge(e.Type))
	fmt.Fprintf(&b, "%s%s\n", detailLabelStyle.Render("Status:"), statusBadge(e.Status))
	project := "global"
	if e.ProjectID != nil {
		project = *e.ProjectID
	}
	fmt.Fprintf(&b, "%s%s\n", detailLabelStyle.Render("Project:"), detailValueStyle.Render(project))
	if len(tags) > 0 {
		names := make([]string, len(tags))
		for i, tag := range tags {
			names[i] = tag.Name
		}
		fmt.Fprintf(&b, "%s%s\n", detailLabelStyle.Render("Tags:"), tagStyle.Render(strings.Join(names, ", ")))
	}
	if e.Summary != "" {
		b.WriteByte('\n')
		fmt.Fprintf(&b, "%s\n", detailLabelStyle.Render("Summary:"))
		b.WriteString(detailValueStyle.Render(e.Summary))
		b.WriteByte('\n')
	}
	if e.BodyOptional != "" {
		b.WriteByte('\n')
		b.WriteString(bodyStyle.Render(e.BodyOptional))
		b.WriteByte('\n')
	}
	if m.detail.Artifact != nil {
		b.WriteByte('\n')
		fmt.Fprintf(&b, "%s\n", detailLabelStyle.Render("Artifact:"))
		fmt.Fprintf(&b, "  ID:    %s\n", m.detail.Artifact.ID)
		fmt.Fprintf(&b, "  Title: %s\n", m.detail.Artifact.Title)
		fmt.Fprintf(&b, "  Type:  %s\n", m.detail.Artifact.Type)
	}
	return b.String()
}

func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n-1]) + "..."
}
