//go:build tui

package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/quantum-6/skillvault/internal/app"
	"github.com/quantum-6/skillvault/internal/domain"
)

const (
	searchResultLimit   = 12
	pendingPreviewLimit = 10
	contextPreviewSize  = 2400
)

type viewState int

const (
	stateDashboard viewState = iota
	stateDetail
)

type focusArea int

const (
	focusProjects focusArea = iota
	focusPending
	focusSearch
	focusContext
)

type services struct {
	entrySvc   *app.EntryService
	projectSvc *app.ProjectService
	contextSvc *app.ContextService
}

type pendingStats struct {
	Open int
	Done int
}

type dashboardLoadedMsg struct {
	projects        []domain.Project
	projectStats    map[string]pendingStats
	selectedProject string
	pending         []domain.EntryListResult
	results         []domain.EntrySearchResult
	contextPreview  string
	err             error
}

type entryDetailMsg struct {
	result *app.GetEntryResult
	err    error
}

type resolvePendingMsg struct {
	dashboard dashboardLoadedMsg
	message   string
	err       error
}

type model struct {
	svcs           services
	projects       []domain.Project
	projectStats   map[string]pendingStats
	projectCursor  int
	pending        []domain.EntryListResult
	pendingCursor  int
	results        []domain.EntrySearchResult
	resultCursor   int
	contextPreview string
	contextOffset  int
	searchInput    textinput.Model
	searchQuery    string
	focus          focusArea
	state          viewState
	detail         *app.GetEntryResult
	detailErr      error
	confirmResolve bool
	statusMessage  string
	statusIsError  bool
	loading        bool
	err            error
	width          int
	height         int
}

func NewModel(entrySvc *app.EntryService, projectSvc *app.ProjectService, contextSvc *app.ContextService) *model {
	ti := textinput.New()
	ti.Placeholder = "Filter selected project entries..."
	ti.CharLimit = 120
	ti.Width = 36
	ti.Prompt = "/ "

	return &model{
		svcs: services{
			entrySvc:   entrySvc,
			projectSvc: projectSvc,
			contextSvc: contextSvc,
		},
		searchInput: ti,
		focus:       focusProjects,
		state:       stateDashboard,
		loading:     true,
	}
}

func (m *model) Init() tea.Cmd {
	return loadDashboardCmd(m.svcs, "", "")
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.searchInput.Width = max(18, min(48, msg.Width/3))
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case dashboardLoadedMsg:
		m.loading = false
		m.err = msg.err
		if msg.err != nil {
			m.statusMessage = ""
			m.statusIsError = false
			return m, nil
		}
		m.projects = msg.projects
		m.projectStats = msg.projectStats
		m.pending = msg.pending
		m.results = msg.results
		m.contextPreview = msg.contextPreview
		m.alignProjectCursor(msg.selectedProject)
		m.pendingCursor = clamp(m.pendingCursor, len(m.pending))
		m.resultCursor = clamp(m.resultCursor, len(m.results))
		m.contextOffset = 0
		m.confirmResolve = false
		return m, nil

	case entryDetailMsg:
		m.loading = false
		m.detail = msg.result
		m.detailErr = msg.err
		m.state = stateDetail
		return m, nil

	case resolvePendingMsg:
		m.loading = false
		m.statusMessage = msg.message
		m.statusIsError = msg.err != nil
		m.confirmResolve = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.err = nil
		loaded := msg.dashboard
		m.projects = loaded.projects
		m.projectStats = loaded.projectStats
		m.pending = loaded.pending
		m.results = loaded.results
		m.contextPreview = loaded.contextPreview
		m.alignProjectCursor(loaded.selectedProject)
		m.pendingCursor = clamp(m.pendingCursor, len(m.pending))
		m.resultCursor = clamp(m.resultCursor, len(m.results))
		m.contextOffset = 0
		return m, nil
	}

	if m.searchInput.Focused() {
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if m.state == stateDetail {
		switch key {
		case "esc":
			m.state = stateDashboard
			m.detail = nil
			m.detailErr = nil
			return m, nil
		case "q", "ctrl+c":
			return m, tea.Quit
		}
		return m, nil
	}

	if m.searchInput.Focused() {
		switch key {
		case "enter":
			m.searchQuery = strings.TrimSpace(m.searchInput.Value())
			m.searchInput.Blur()
			m.loading = true
			m.resultCursor = 0
			return m, loadDashboardCmd(m.svcs, m.selectedProjectID(), m.searchQuery)
		case "esc":
			m.searchInput.SetValue(m.searchQuery)
			m.searchInput.Blur()
			return m, nil
		}

		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		return m, cmd
	}

	if m.confirmResolve {
		switch key {
		case "y", "enter":
			item, ok := m.selectedPendingItem()
			if !ok {
				m.confirmResolve = false
				return m, nil
			}
			m.loading = true
			m.err = nil
			return m, resolvePendingCmd(m.svcs, item.Entry.ID, item.Entry.Title, m.selectedProjectID(), m.searchQuery)
		case "n", "esc":
			m.confirmResolve = false
			m.statusMessage = ""
			m.statusIsError = false
			return m, nil
		}
		return m, nil
	}

	switch key {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "/":
		m.searchInput.SetValue(m.searchQuery)
		m.searchInput.Focus()
		return m, nil
	case "tab", "l", "right":
		m.focus = (m.focus + 1) % 4
		return m, nil
	case "shift+tab", "h", "left":
		m.focus = (m.focus + 3) % 4
		return m, nil
	case "r":
		m.loading = true
		m.statusMessage = ""
		m.statusIsError = false
		return m, loadDashboardCmd(m.svcs, m.selectedProjectID(), m.searchQuery)
	}

	switch m.focus {
	case focusProjects:
		return m.handleProjectKeys(key)
	case focusPending:
		return m.handlePendingKeys(key)
	case focusSearch:
		return m.handleSearchKeys(key)
	case focusContext:
		return m.handleContextKeys(key)
	}

	return m, nil
}

func (m *model) handleProjectKeys(key string) (tea.Model, tea.Cmd) {
	if len(m.projects) == 0 {
		return m, nil
	}

	next := m.projectCursor
	switch key {
	case "j", "down":
		if next < len(m.projects)-1 {
			next++
		}
	case "k", "up":
		if next > 0 {
			next--
		}
	default:
		return m, nil
	}

	if next == m.projectCursor {
		return m, nil
	}
	m.projectCursor = next
	m.pendingCursor = 0
	m.resultCursor = 0
	m.contextOffset = 0
	m.confirmResolve = false
	m.statusMessage = ""
	m.statusIsError = false
	m.loading = true
	return m, loadDashboardCmd(m.svcs, m.selectedProjectID(), m.searchQuery)
}

func (m *model) handlePendingKeys(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "j", "down":
		if m.pendingCursor < len(m.pending)-1 {
			m.pendingCursor++
		}
	case "k", "up":
		if m.pendingCursor > 0 {
			m.pendingCursor--
		}
	case "x", "d":
		if len(m.pending) > 0 {
			m.confirmResolve = true
			m.statusMessage = ""
			m.statusIsError = false
		}
	case "enter":
		if len(m.pending) > 0 {
			m.loading = true
			return m, getEntryCmd(m.svcs.entrySvc, m.pending[m.pendingCursor].Entry.ID)
		}
	}
	return m, nil
}

func (m *model) handleSearchKeys(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "j", "down":
		if m.resultCursor < len(m.results)-1 {
			m.resultCursor++
		}
	case "k", "up":
		if m.resultCursor > 0 {
			m.resultCursor--
		}
	case "enter":
		if len(m.results) > 0 {
			m.loading = true
			return m, getEntryCmd(m.svcs.entrySvc, m.results[m.resultCursor].Entry.ID)
		}
	}
	return m, nil
}

func (m *model) handleContextKeys(key string) (tea.Model, tea.Cmd) {
	maxOffset := max(0, len(contextLines(m.contextPreview))-m.contextPaneHeight())
	switch key {
	case "j", "down":
		if m.contextOffset < maxOffset {
			m.contextOffset++
		}
	case "k", "up":
		if m.contextOffset > 0 {
			m.contextOffset--
		}
	}
	return m, nil
}

func (m *model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	switch m.state {
	case stateDetail:
		return detailView(m)
	default:
		return dashboardView(m)
	}
}

func (m *model) selectedProjectID() string {
	if len(m.projects) == 0 || m.projectCursor >= len(m.projects) {
		return ""
	}
	return m.projects[m.projectCursor].ID
}

func (m *model) alignProjectCursor(projectID string) {
	if len(m.projects) == 0 {
		m.projectCursor = 0
		return
	}
	for i, project := range m.projects {
		if project.ID == projectID {
			m.projectCursor = i
			return
		}
	}
	m.projectCursor = clamp(m.projectCursor, len(m.projects))
}

func (m *model) pendingStatsFor(projectID string) pendingStats {
	if m.projectStats == nil {
		return pendingStats{}
	}
	return m.projectStats[projectID]
}

func (m *model) selectedPendingItem() (domain.EntryListResult, bool) {
	if len(m.pending) == 0 || m.pendingCursor >= len(m.pending) {
		return domain.EntryListResult{}, false
	}
	return m.pending[m.pendingCursor], true
}

func (m *model) contextPaneHeight() int {
	if m.height < 26 {
		return 8
	}
	return 10
}

func loadDashboardCmd(svcs services, projectID, query string) tea.Cmd {
	return func() tea.Msg {
		loaded, err := loadDashboardData(context.Background(), svcs, projectID, query)
		if err != nil {
			return dashboardLoadedMsg{err: err}
		}
		return loaded
	}
}

func resolvePendingCmd(svcs services, id, title, projectID, query string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		if err := svcs.entrySvc.ResolvePending(ctx, id); err != nil {
			return resolvePendingMsg{
				message: fmt.Sprintf("Could not mark pending item done: %v", err),
				err:     fmt.Errorf("resolve pending: %w", err),
			}
		}

		loaded, err := loadDashboardData(ctx, svcs, projectID, query)
		if err != nil {
			return resolvePendingMsg{
				message: fmt.Sprintf("Marked pending item done, but reload failed: %v", err),
				err:     err,
			}
		}

		return resolvePendingMsg{
			dashboard: loaded,
			message:   fmt.Sprintf("Marked pending item done: %s", title),
		}
	}
}

func loadDashboardData(ctx context.Context, svcs services, projectID, query string) (dashboardLoadedMsg, error) {
	projects, err := svcs.projectSvc.ListProjects(ctx)
	if err != nil {
		return dashboardLoadedMsg{}, fmt.Errorf("list projects: %w", err)
	}

	if len(projects) == 0 {
		return dashboardLoadedMsg{projects: projects, projectStats: map[string]pendingStats{}}, nil
	}

	selected := strings.TrimSpace(projectID)
	if selected == "" {
		selected = projects[0].ID
	}

	statsByProject := make(map[string]pendingStats, len(projects))
	selectedPending := make([]domain.EntryListResult, 0)
	for _, project := range projects {
		items, err := svcs.entrySvc.ListPendingWithOptions(ctx, app.ListPendingInput{
			Project:         project.ID,
			IncludeArchived: true,
		})
		if err != nil {
			return dashboardLoadedMsg{}, fmt.Errorf("list pending for project %q: %w", project.ID, err)
		}

		stats := summarizePending(items)
		statsByProject[project.ID] = stats
		if project.ID == selected {
			selectedPending = activePendingPreview(items, pendingPreviewLimit)
		}
	}

	searchQuery := domain.SearchQuery{
		ProjectID:       strPtr(selected),
		IncludeArchived: false,
		Limit:           searchResultLimit,
	}
	results, err := svcs.entrySvc.SearchEntries(ctx, strings.TrimSpace(query), searchQuery)
	if err != nil {
		return dashboardLoadedMsg{}, fmt.Errorf("search entries: %w", err)
	}

	pack, err := svcs.contextSvc.GetContext(ctx, app.ContextInput{
		Mode:            "planning",
		Project:         selected,
		ExcludeArchived: true,
		MaxChars:        contextPreviewSize,
	})
	if err != nil {
		return dashboardLoadedMsg{}, fmt.Errorf("get context: %w", err)
	}

	return dashboardLoadedMsg{
		projects:        projects,
		projectStats:    statsByProject,
		selectedProject: selected,
		pending:         selectedPending,
		results:         results,
		contextPreview:  pack.Raw,
	}, nil
}

func summarizePending(items []domain.EntryListResult) pendingStats {
	stats := pendingStats{}
	for _, item := range items {
		if item.Entry.Status == domain.StatusArchived {
			stats.Done++
			continue
		}
		stats.Open++
	}
	return stats
}

func activePendingPreview(items []domain.EntryListResult, limit int) []domain.EntryListResult {
	preview := make([]domain.EntryListResult, 0, min(limit, len(items)))
	for _, item := range items {
		if item.Entry.Status == domain.StatusArchived {
			continue
		}
		preview = append(preview, item)
		if limit > 0 && len(preview) >= limit {
			break
		}
	}
	return preview
}

func getEntryCmd(entrySvc *app.EntryService, id string) tea.Cmd {
	return func() tea.Msg {
		result, err := entrySvc.GetEntry(context.Background(), id)
		if err != nil {
			return entryDetailMsg{err: fmt.Errorf("get entry: %w", err)}
		}
		return entryDetailMsg{result: result}
	}
}

func contextLines(content string) []string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return []string{"No context preview available."}
	}
	return strings.Split(trimmed, "\n")
}

func clamp(current, length int) int {
	if length <= 0 {
		return 0
	}
	if current < 0 {
		return 0
	}
	if current >= length {
		return length - 1
	}
	return current
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func strPtr(value string) *string {
	return &value
}
