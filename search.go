package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── States ────────────────────────────────────────────────────────────────────

type searchState int

const (
	searchInput   searchState = iota // text box is active
	searchLoading                    // waiting for first page
	searchResults                    // showing the results list
	searchDetail                     // showing entity detail
	searchError                      // API error
)

// ── Model ─────────────────────────────────────────────────────────────────────

type searchModel struct {
	state          searchState
	input          textinput.Model
	list           list.Model
	vp             viewport.Model
	spin           spinner.Model
	selectedEntity *Entity
	term           string // term used for the current result set
	totalItems     int
	fetchingMore   bool
	width          int
	height         int
	err            error
	client         backstageClient
}

func newSearchModel(client backstageClient, width, height int) searchModel {
	ti := textinput.New()
	ti.Placeholder = "type to search…"
	ti.CharLimit = 256
	ti.Width = max(20, width-12)
	ti.Focus()

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = spinnerStyle

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(lipgloss.Color("#00db54")).
		BorderLeftForeground(lipgloss.Color("#0079C6"))
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(lipgloss.Color("#0093F9")).
		BorderLeftForeground(lipgloss.Color("#0079C6"))

	l := list.New([]list.Item{}, delegate, width, max(0, height-2))
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(false) // search is handled server-side

	vp := viewport.New(width, max(0, height-2))

	return searchModel{
		state:  searchInput,
		input:  ti,
		list:   l,
		vp:     vp,
		spin:   sp,
		width:  width,
		height: height,
		client: client,
	}
}

// ── Commands ──────────────────────────────────────────────────────────────────

func (m searchModel) doSearch() tea.Cmd {
	term := m.term
	client := m.client
	return func() tea.Msg {
		entities, total, nextCursor, err := client.searchEntities(term, "")
		if err != nil {
			return errMsg{err}
		}
		return searchPageMsg{entities: entities, totalItems: total, nextCursor: nextCursor, term: term}
	}
}

func doSearchNext(client backstageClient, term, cursor string) tea.Cmd {
	return func() tea.Msg {
		entities, total, nextCursor, err := client.searchEntities(term, cursor)
		if err != nil {
			return errMsg{err}
		}
		return searchPageMsg{entities: entities, totalItems: total, nextCursor: nextCursor, term: term}
	}
}

// ── Update ────────────────────────────────────────────────────────────────────

func (m searchModel) update(msg tea.Msg) (searchModel, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.Width = max(20, msg.Width-12)
		m.list.SetSize(msg.Width, msg.Height-2)
		m.vp.Width = msg.Width
		m.vp.Height = msg.Height - 2
		return m, nil

	case spinner.TickMsg:
		if m.state == searchLoading || m.fetchingMore {
			var cmd tea.Cmd
			m.spin, cmd = m.spin.Update(msg)
			return m, cmd
		}
		return m, nil

	case searchPageMsg:
		if msg.term != m.term {
			return m, nil // stale result from a previous search
		}
		m.totalItems = msg.totalItems
		existing := m.list.Items()
		newItems := make([]list.Item, len(msg.entities))
		for i, e := range msg.entities {
			newItems[i] = entityItem{entity: e}
		}
		setCmd := m.list.SetItems(append(existing, newItems...))
		if msg.nextCursor != "" {
			m.fetchingMore = true
			m.state = searchResults
			return m, tea.Batch(setCmd, doSearchNext(m.client, msg.term, msg.nextCursor))
		}
		m.fetchingMore = false
		m.state = searchResults
		return m, setCmd

	case errMsg:
		m.err = msg.err
		m.state = searchError
		return m, nil

	case tea.KeyMsg:
		// In the text input, only intercept esc and enter; let everything else
		// flow to textinput so the user can type normally (including 'q').
		if m.state == searchInput {
			switch msg.String() {
			case "esc":
				return m, func() tea.Msg { return backToMenuMsg{} }
			case "enter":
				term := strings.TrimSpace(m.input.Value())
				if term == "" {
					return m, nil
				}
				m.term = term
				m.state = searchLoading
				m.fetchingMore = false
				m.input.Blur()
				_ = m.list.SetItems([]list.Item{})
				return m, tea.Batch(m.spin.Tick, m.doSearch())
			}
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}

		// All other states: q quits, esc navigates back.
		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "esc":
			switch m.state {
			case searchDetail:
				m.state = searchResults
				return m, nil
			case searchResults, searchError:
				m.state = searchInput
				m.input.Focus()
				return m, nil
			}
		case "enter":
			if m.state == searchResults {
				if item, ok := m.list.SelectedItem().(entityItem); ok {
					e := item.entity
					m.selectedEntity = &e
					m.state = searchDetail
					m.vp.SetContent(renderEntityDetail(e))
					m.vp.GotoTop()
					return m, nil
				}
			}
		case "r":
			if m.state == searchResults && m.term != "" {
				m.state = searchLoading
				m.fetchingMore = false
				_ = m.list.SetItems([]list.Item{})
				return m, tea.Batch(m.spin.Tick, m.doSearch())
			}
		}
	}

	// Delegate to the active component.
	switch m.state {
	case searchResults:
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	case searchDetail:
		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(msg)
		return m, cmd
	}

	return m, nil
}

// ── View ──────────────────────────────────────────────────────────────────────

func (m searchModel) view() string {
	switch m.state {
	case searchInput:
		return m.viewInput()
	case searchLoading:
		return "\n  " + m.spin.View() + "  Searching for “" + m.term + "”…"
	case searchResults:
		return m.viewResults()
	case searchDetail:
		return m.viewDetail()
	case searchError:
		help := helpStyle.Render("esc: back  q: quit")
		return "\n\n  " + errorStyle.Render("Error: "+m.err.Error()) + "\n\n  " + help
	}
	return ""
}

func (m searchModel) viewInput() string {
	var sb strings.Builder
	sb.WriteString(menuTitleStyle.Render("Search Catalog Entities"))
	sb.WriteString("\n\n  ")
	sb.WriteString(fieldLabelStyle.Render("Search: "))
	sb.WriteString(m.input.View())
	sb.WriteString("\n")

	used := 4
	for i := used; i < m.height-1; i++ {
		sb.WriteString("\n")
	}
	sb.WriteString("  ")
	sb.WriteString(helpStyle.Render("enter: search  esc: main menu"))
	return sb.String()
}

func (m searchModel) viewResults() string {
	count := fmt.Sprintf("%d", m.totalItems)
	header := headerStyle.Render("Search: “" + m.term + "”  " + count + " results")

	var body string
	if len(m.list.Items()) == 0 && !m.fetchingMore {
		body = "\n\n  " + helpStyle.Render("No results found.")
		// pad remaining height
		for i := 3; i < m.height-2; i++ {
			body += "\n"
		}
	} else {
		body = m.list.View()
	}

	helpText := "↑/↓: navigate  enter: details  r: re-search  esc: back to search  q: quit"
	var bottomLine string
	if m.fetchingMore {
		bottomLine = m.spin.View() + " " + helpStyle.Render("loading more…  "+helpText)
	} else {
		bottomLine = helpStyle.Render(helpText)
	}
	return header + "\n" + body + "\n" + bottomLine
}

func (m searchModel) viewDetail() string {
	if m.selectedEntity == nil {
		return ""
	}
	title := headerStyle.Render(m.selectedEntity.Kind + "  " + m.selectedEntity.Metadata.Name)
	help := helpStyle.Render("↑/↓/pgup/pgdn: scroll  esc: back  q: quit")
	return title + "\n" + m.vp.View() + "\n" + help
}
