package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/quantum-6/skillvault/internal/line"
)

type ViewMode int

const (
	ModeMain ViewMode = iota
	ModeContinue
	ModeNewJob
	ModeSwitchProject
)

var (
	headerStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#ffffff")).
		Background(lipgloss.Color("#1e293b")).
		Padding(0, 1)

	statusBarStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#94a3b8")).
		Background(lipgloss.Color("#0f172a")).
		Padding(0, 1)

	panelStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#334155")).
		Padding(1)

	modalBoxStyle = lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(lipgloss.Color("#38bdf8")).
		Background(lipgloss.Color("#0b1120")).
		Padding(1, 2)

	accentStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#38bdf8")).
		Bold(true)

	warnStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#fbbf24")).
		Bold(true)

	successStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#34d399")).
		Bold(true)
)

type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

type Model struct {
	engine        *line.Engine
	store         *line.Store
	projects      []line.Project
	activeProject line.Project
	jobs          []line.Job
	selectedJob   line.Job
	events        []line.Event
	table         table.Model
	continueInput textarea.Model
	newJobInput   textinput.Model
	projectIndex  int
	mode          ViewMode
	statusMsg     string
	width         int
	height        int
}

func NewModel(engine *line.Engine) Model {
	store := engine.Store()
	t := table.New(
		table.WithColumns([]table.Column{
			{Title: "ID", Width: 10},
			{Title: "Phase", Width: 10},
			{Title: "Status", Width: 14},
			{Title: "Issue", Width: 30},
		}),
		table.WithFocused(true),
		table.WithHeight(12),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(true).
		Foreground(lipgloss.Color("#38bdf8"))
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("#ffffff")).
		Background(lipgloss.Color("#2563eb")).
		Bold(true)
	t.SetStyles(s)

	ta := textarea.New()
	ta.Placeholder = "Enter your answer to the agent's question..."
	ta.Focus()
	ta.SetHeight(4)
	ta.SetWidth(60)

	ti := textinput.New()
	ti.Placeholder = "https://github.com/owner/repo/issues/123"
	ti.Focus()
	ti.Width = 60

	m := Model{
		engine:        engine,
		store:         store,
		table:         t,
		continueInput: ta,
		newJobInput:   ti,
		mode:          ModeMain,
		width:         100,
		height:        30,
	}
	m.reload(context.Background())
	return m
}

func (m *Model) reload(ctx context.Context) {
	if m.store == nil {
		return
	}
	projs, _ := m.store.ListProjects(ctx)
	m.projects = projs
	if active, err := m.store.GetActiveProject(ctx); err == nil {
		m.activeProject = active
	} else {
		m.activeProject = line.Project{}
	}

	jobs, _ := m.engine.List(ctx)
	m.jobs = jobs

	var rows []table.Row
	for _, j := range jobs {
		rows = append(rows, table.Row{
			j.ID,
			string(j.Phase),
			string(j.Status),
			j.IssueURL,
		})
	}
	m.table.SetRows(rows)

	m.updateSelection(ctx)
}

func (m *Model) updateSelection(ctx context.Context) {
	if len(m.jobs) == 0 {
		m.selectedJob = line.Job{}
		m.events = nil
		return
	}
	idx := m.table.Cursor()
	if idx >= 0 && idx < len(m.jobs) {
		m.selectedJob = m.jobs[idx]
		if job, events, err := m.engine.Status(ctx, m.selectedJob.ID); err == nil {
			m.selectedJob = job
			m.events = events
		}
	}
}

func (m Model) Init() tea.Cmd {
	return tickCmd()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	ctx := context.Background()

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tickMsg:
		m.reload(ctx)
		return m, tickCmd()

	case tea.KeyMsg:
		switch m.mode {
		case ModeMain:
			switch msg.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			case "r":
				m.reload(ctx)
				m.statusMsg = "Refreshed"
				return m, nil
			case "c":
				if m.selectedJob.Status == line.StatusNeedsHuman {
					m.mode = ModeContinue
					m.continueInput.Reset()
					m.continueInput.Focus()
				} else {
					m.statusMsg = "Selected job is not in needs_human status"
				}
				return m, nil
			case "n":
				m.mode = ModeNewJob
				m.newJobInput.Reset()
				m.newJobInput.Focus()
				return m, nil
			case "p":
				m.mode = ModeSwitchProject
				m.projectIndex = 0
				return m, nil
			case " ":
				if m.selectedJob.ID != "" {
					job, err := m.engine.Advance(ctx, m.selectedJob.ID)
					if err != nil {
						m.statusMsg = fmt.Sprintf("Advance error: %v", err)
					} else {
						m.selectedJob = job
						m.statusMsg = fmt.Sprintf("Advanced to %s", job.Phase)
					}
					m.reload(ctx)
				}
				return m, nil
			case "a":
				if m.selectedJob.ID != "" {
					job, err := m.engine.AutoAdvance(ctx, m.selectedJob.ID)
					if err != nil {
						m.statusMsg = fmt.Sprintf("AutoAdvance error: %v", err)
					} else {
						m.selectedJob = job
						m.statusMsg = fmt.Sprintf("Auto-advanced to %s (%s)", job.Phase, job.Status)
					}
					m.reload(ctx)
				}
				return m, nil
			}

			var tableCmd tea.Cmd
			m.table, tableCmd = m.table.Update(msg)
			m.updateSelection(ctx)
			return m, tableCmd

		case ModeContinue:
			switch msg.String() {
			case "esc":
				m.mode = ModeMain
				return m, nil
			case "ctrl+s", "enter":
				answer := strings.TrimSpace(m.continueInput.Value())
				if answer != "" {
					job, err := m.engine.Continue(ctx, m.selectedJob.ID, answer)
					if err != nil {
						m.statusMsg = fmt.Sprintf("Continue error: %v", err)
					} else {
						m.selectedJob = job
						m.statusMsg = "Continued job"
					}
					m.mode = ModeMain
					m.reload(ctx)
				}
				return m, nil
			}
			m.continueInput, cmd = m.continueInput.Update(msg)
			return m, cmd

		case ModeNewJob:
			switch msg.String() {
			case "esc":
				m.mode = ModeMain
				return m, nil
			case "enter":
				issueURL := strings.TrimSpace(m.newJobInput.Value())
				repo := m.activeProject.RepoPath
				if issueURL != "" && repo != "" {
					job, err := m.engine.RunPlan(ctx, issueURL, repo)
					if err != nil {
						m.statusMsg = fmt.Sprintf("RunPlan error: %v", err)
					} else {
						m.statusMsg = fmt.Sprintf("Created job %s", job.ID)
					}
					m.mode = ModeMain
					m.reload(ctx)
				} else if repo == "" {
					m.statusMsg = "No active project. Switch project with 'p' first."
					m.mode = ModeMain
				}
				return m, nil
			}
			m.newJobInput, cmd = m.newJobInput.Update(msg)
			return m, cmd

		case ModeSwitchProject:
			switch msg.String() {
			case "esc":
				m.mode = ModeMain
				return m, nil
			case "up", "k":
				if m.projectIndex > 0 {
					m.projectIndex--
				}
				return m, nil
			case "down", "j":
				if m.projectIndex < len(m.projects)-1 {
					m.projectIndex++
				}
				return m, nil
			case "enter":
				if len(m.projects) > 0 && m.projectIndex < len(m.projects) {
					targetID := m.projects[m.projectIndex].ID
					if err := m.store.SetActiveProject(ctx, targetID); err == nil {
						m.statusMsg = fmt.Sprintf("Active project set to %s", targetID)
					}
					m.mode = ModeMain
					m.reload(ctx)
				}
				return m, nil
			}
		}
	}

	return m, nil
}

func (m Model) View() string {
	header := headerStyle.Render(fmt.Sprintf(
		" ⚡ LINE FACTORY  │  Active Project: [ %s ]  │  Total Jobs: %d ",
		activeProjectName(m.activeProject), len(m.jobs),
	))

	switch m.mode {
	case ModeContinue:
		modalContent := fmt.Sprintf(
			"Continue Job %s (%s)\n\nQuestion:\n%s\n\nAnswer (Enter to submit, Esc to cancel):\n%s",
			m.selectedJob.ID, m.selectedJob.IssueURL,
			warnStyle.Render(m.selectedJob.Question),
			m.continueInput.View(),
		)
		return lipgloss.JoinVertical(lipgloss.Center, header, modalBoxStyle.Render(modalContent))

	case ModeNewJob:
		modalContent := fmt.Sprintf(
			"Start New Job on [ %s ]\nRepo: %s\n\nIssue URL (Enter to submit, Esc to cancel):\n%s",
			activeProjectName(m.activeProject), m.activeProject.RepoPath,
			m.newJobInput.View(),
		)
		return lipgloss.JoinVertical(lipgloss.Center, header, modalBoxStyle.Render(modalContent))

	case ModeSwitchProject:
		var b strings.Builder
		b.WriteString("Select Active Project (Enter to select, Esc to cancel):\n\n")
		if len(m.projects) == 0 {
			b.WriteString("No projects registered. Use 'line project add'.")
		} else {
			for i, p := range m.projects {
				cursor := " "
				if i == m.projectIndex {
					cursor = ">"
				}
				activeMark := ""
				if p.IsActive {
					activeMark = " (active)"
				}
				b.WriteString(fmt.Sprintf("%s %-12s %s%s\n", cursor, p.ID, p.RepoPath, activeMark))
			}
		}
		return lipgloss.JoinVertical(lipgloss.Center, header, modalBoxStyle.Render(b.String()))
	}

	// Main split view
	leftPanel := panelStyle.Width(66).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			accentStyle.Render("Factory Jobs:"),
			m.table.View(),
		),
	)

	inspectorContent := m.renderInspector()
	rightPanel := panelStyle.Width(50).Render(inspectorContent)

	mainContent := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)

	status := m.statusMsg
	if status == "" {
		status = "Ready"
	}
	statusBar := statusBarStyle.Render(fmt.Sprintf(" %s ", status))
	helpBar := lipgloss.NewStyle().Foreground(lipgloss.Color("#64748b")).Render(
		" [↑/↓] Select  [space] Next  [a] Auto  [c] Continue  [n] New Job  [p] Projects  [r] Refresh  [q] Quit",
	)

	return lipgloss.JoinVertical(lipgloss.Left, header, mainContent, statusBar, helpBar)
}

func (m Model) renderInspector() string {
	if m.selectedJob.ID == "" {
		return "No job selected."
	}
	j := m.selectedJob
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s\n", accentStyle.Render("Job Details: "+j.ID)))
	b.WriteString(fmt.Sprintf("Phase:  %s\nStatus: %s\nIssue:  %s\n", j.Phase, j.Status, j.IssueURL))
	if j.WorktreePath != "" {
		b.WriteString(fmt.Sprintf("Worktree: %s\n", j.WorktreePath))
	}
	if j.PullRequest != "" {
		b.WriteString(fmt.Sprintf("PR: %s\n", successStyle.Render(j.PullRequest)))
	}
	if j.HeadSHA != "" {
		b.WriteString(fmt.Sprintf("SHA: %s\n", j.HeadSHA))
	}
	if j.Summary != "" {
		b.WriteString(fmt.Sprintf("\nSummary:\n%s\n", j.Summary))
	}
	if j.Question != "" {
		b.WriteString(fmt.Sprintf("\n%s\n%s\n", warnStyle.Render("Question (needs_human):"), j.Question))
	}
	if len(m.events) > 0 {
		b.WriteString(fmt.Sprintf("\n%s\n", accentStyle.Render("Recent Events:")))
		start := 0
		if len(m.events) > 5 {
			start = len(m.events) - 5
		}
		for _, ev := range m.events[start:] {
			b.WriteString(fmt.Sprintf("• [%s] %s %s\n", ev.Phase, ev.Kind, ev.Detail))
		}
	}
	return b.String()
}

func activeProjectName(p line.Project) string {
	if p.ID == "" {
		return "None"
	}
	if p.Name != "" {
		return p.ID + " - " + p.Name
	}
	return p.ID
}

func Run(ctx context.Context, engine *line.Engine) error {
	m := NewModel(engine)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
