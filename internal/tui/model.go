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

// viewState tracks which screen the TUI is showing.
type viewState int

const (
	stateBrowse viewState = iota
	stateDetail
)

// ---- messages ----

// searchResultsMsg carries entries returned from a search query.
type searchResultsMsg []domain.EntrySearchResult

// entryDetailMsg carries the result of fetching a single entry.
type entryDetailMsg struct {
	result *app.GetEntryResult
	err    error
}

// searchErrMsg wraps an error that occurred during search.
type searchErrMsg struct{ err error }

// ---- model ----

// model is the Bubble Tea Model for the SkillVault TUI.
type model struct {
	entrySvc    *app.EntryService
	entries     []domain.EntrySearchResult
	cursor      int
	searchInput textinput.Model
	state       viewState
	detail      *app.GetEntryResult
	detailErr   error
	loading     bool
	err         error
	width       int
	height      int
}

// NewModel creates a new TUI model wired to the given entry service.
func NewModel(entrySvc *app.EntryService) *model {
	ti := textinput.New()
	ti.Placeholder = "Search entries (FTS5)…"
	ti.CharLimit = 120
	ti.Width = 60
	ti.Prompt = "/ "

	return &model{
		entrySvc:    entrySvc,
		entries:     nil,
		cursor:      0,
		searchInput: ti,
		state:       stateBrowse,
		loading:     true,
	}
}

// Init triggers the initial search to populate the entry list.
func (m *model) Init() tea.Cmd {
	return searchCmd(m.entrySvc, "")
}

// ---- update ----

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.searchInput.Width = max(20, msg.Width-10)
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case searchResultsMsg:
		m.entries = msg
		m.loading = false
		m.err = nil
		if m.cursor >= len(m.entries) && len(m.entries) > 0 {
			m.cursor = len(m.entries) - 1
		}
		if len(m.entries) == 0 {
			m.cursor = 0
		}
		return m, nil

	case entryDetailMsg:
		m.loading = false
		if msg.err != nil {
			m.detailErr = msg.err
			m.state = stateDetail
			return m, nil
		}
		m.detail = msg.result
		m.detailErr = nil
		m.state = stateDetail
		return m, nil

	case searchErrMsg:
		m.loading = false
		m.err = msg.err
		return m, nil
	}

	// Delegate to textinput for all other messages when it's focused.
	if m.searchInput.Focused() {
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch m.state {

	case stateBrowse:
		// Global quit.
		if key == "q" || key == "ctrl+c" {
			return m, tea.Quit
		}

		// Focus search bar on "/".
		if key == "/" {
			if !m.searchInput.Focused() {
				m.searchInput.Focus()
				m.searchInput.SetValue("")
				return m, nil
			}
		}

		// If search input is focused, delegate to it. Enter triggers search.
		if m.searchInput.Focused() {
			if key == "enter" {
				query := strings.TrimSpace(m.searchInput.Value())
				m.searchQuery(query)
				m.searchInput.Blur()
				m.loading = true
				return m, searchCmd(m.entrySvc, query)
			}
			if key == "esc" {
				m.searchInput.Blur()
				m.searchInput.SetValue("")
				return m, nil
			}

			var cmd tea.Cmd
			m.searchInput, cmd = m.searchInput.Update(msg)
			return m, cmd
		}

		// Not focused: navigation keys.
		switch key {
		case "j", "down":
			if m.cursor < len(m.entries)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "enter":
			if len(m.entries) > 0 && m.cursor < len(m.entries) {
				entry := m.entries[m.cursor]
				m.loading = true
				return m, getEntryCmd(m.entrySvc, entry.Entry.ID)
			}
		case "/":
			m.searchInput.Focus()
		case "esc":
			// Clear search and reload all.
			m.searchInput.SetValue("")
			m.cursor = 0
			m.loading = true
			return m, searchCmd(m.entrySvc, "")
		}

	case stateDetail:
		if key == "q" || key == "ctrl+c" || key == "esc" {
			m.state = stateBrowse
			m.detail = nil
			m.detailErr = nil
			return m, nil
		}
	}

	return m, nil
}

func (m *model) searchQuery(q string) {
	// No-op: actual search happens via cmd. This is a placeholder for testability.
	_ = q
}

// ---- view ----

func (m *model) View() string {
	if m.width == 0 {
		return "Loading…"
	}

	switch m.state {
	case stateBrowse:
		return browseView(m)
	case stateDetail:
		return detailView(m)
	default:
		return ""
	}
}

// ---- commands ----

func searchCmd(entrySvc *app.EntryService, query string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		results, err := entrySvc.SearchEntries(ctx, query, domain.SearchQuery{
			Limit:           100,
			IncludeArchived: false,
		})
		if err != nil {
			return searchErrMsg{err: fmt.Errorf("search: %w", err)}
		}
		return searchResultsMsg(results)
	}
}

func getEntryCmd(entrySvc *app.EntryService, id string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		result, err := entrySvc.GetEntry(ctx, id)
		if err != nil {
			return entryDetailMsg{err: fmt.Errorf("get entry: %w", err)}
		}
		return entryDetailMsg{result: result}
	}
}
