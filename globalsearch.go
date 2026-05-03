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

// ── API types ─────────────────────────────────────────────────────────────────

type globalSearchDoc struct {
	Title         string `json:"title"`
	Text          string `json:"text"`
	ComponentType string `json:"componentType"`
	Type          string `json:"type"`
	Namespace     string `json:"namespace"`
	Kind          string `json:"kind"`
	Lifecycle     string `json:"lifecycle"`
	Owner         string `json:"owner"`
	Location      string `json:"location"`
}

type globalSearchResult struct {
	Type     string          `json:"type"`
	Document globalSearchDoc `json:"document"`
	Rank     int             `json:"rank"`
}

type globalSearchResponse struct {
	NumberOfResults int                  `json:"numberOfResults"`
	NextPageCursor  string               `json:"nextPageCursor"`
	Results         []globalSearchResult `json:"results"`
}

// ── List item ─────────────────────────────────────────────────────────────────

type globalSearchItem struct {
	result globalSearchResult
}

func (g globalSearchItem) Title() string {
	if g.result.Document.Title != "" {
		return g.result.Document.Title
	}
	return g.result.Document.Location
}

func (g globalSearchItem) Description() string {
	doc := g.result.Document
	var parts []string
	if doc.Kind != "" {
		parts = append(parts, doc.Kind)
	}
	if doc.Owner != "" {
		parts = append(parts, "owner:"+doc.Owner)
	}
	if doc.Text != "" {
		t := doc.Text
		if len(t) > 72 {
			t = t[:69] + "…"
		}
		parts = append(parts, t)
	}
	return strings.Join(parts, "  ")
}

func (g globalSearchItem) FilterValue() string {
	return g.result.Document.Title + " " + g.result.Document.Text
}

// ── States ────────────────────────────────────────────────────────────────────

type globalSearchState int

const (
	gsInput   globalSearchState = iota
	gsLoading
	gsResults
	gsDetail
	gsError
)

// ── Model ─────────────────────────────────────────────────────────────────────

type globalSearchModel struct {
	state          globalSearchState
	input          textinput.Model
	list           list.Model
	vp             viewport.Model
	spin           spinner.Model
	term           string
	totalItems     int
	fetchingMore   bool
	selectedResult *globalSearchResult
	width          int
	height         int
	err            error
	client         backstageClient
}

func newGlobalSearchModel(client backstageClient, width, height int) globalSearchModel {
	ti := textinput.New()
	ti.Placeholder = "search all Backstage content…"
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
	l.SetFilteringEnabled(true)

	vp := viewport.New(width, max(0, height-2))

	return globalSearchModel{
		state:  gsInput,
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

func (m globalSearchModel) doSearch() tea.Cmd {
	term, client := m.term, m.client
	return func() tea.Msg {
		results, total, nextCursor, err := client.querySearch(term, "")
		if err != nil {
			return errMsg{err}
		}
		return querySearchPageMsg{results: results, totalItems: total, nextCursor: nextCursor, term: term}
	}
}

func doQuerySearchNext(client backstageClient, term, cursor string) tea.Cmd {
	return func() tea.Msg {
		results, total, nextCursor, err := client.querySearch(term, cursor)
		if err != nil {
			return errMsg{err}
		}
		return querySearchPageMsg{results: results, totalItems: total, nextCursor: nextCursor, term: term}
	}
}

// ── Update ────────────────────────────────────────────────────────────────────

func (m globalSearchModel) update(msg tea.Msg) (globalSearchModel, tea.Cmd) {
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
		if m.state == gsLoading || m.fetchingMore {
			var cmd tea.Cmd
			m.spin, cmd = m.spin.Update(msg)
			return m, cmd
		}
		return m, nil

	case querySearchPageMsg:
		if msg.term != m.term {
			return m, nil // stale result from a previous search
		}
		m.totalItems = msg.totalItems
		existing := m.list.Items()
		newItems := make([]list.Item, len(msg.results))
		for i, r := range msg.results {
			r := r
			newItems[i] = globalSearchItem{result: r}
		}
		setCmd := m.list.SetItems(append(existing, newItems...))
		if msg.nextCursor != "" {
			m.fetchingMore = true
			m.state = gsResults
			return m, tea.Batch(setCmd, doQuerySearchNext(m.client, msg.term, msg.nextCursor))
		}
		m.fetchingMore = false
		m.state = gsResults
		return m, setCmd

	case errMsg:
		m.err = msg.err
		m.state = gsError
		return m, nil

	case tea.KeyMsg:
		// In the text input only intercept esc/enter; pass everything else
		// (including q) to textinput so the user can type normally.
		if m.state == gsInput {
			switch msg.String() {
			case "esc":
				return m, func() tea.Msg { return backToMenuMsg{} }
			case "enter":
				term := strings.TrimSpace(m.input.Value())
				if term == "" {
					return m, nil
				}
				m.term = term
				m.state = gsLoading
				m.fetchingMore = false
				m.input.Blur()
				_ = m.list.SetItems([]list.Item{})
				return m, tea.Batch(m.spin.Tick, m.doSearch())
			}
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}

		if m.state == gsResults && m.list.FilterState() == list.Filtering {
			var cmd tea.Cmd
			m.list, cmd = m.list.Update(msg)
			return m, cmd
		}

		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "esc":
			switch m.state {
			case gsDetail:
				m.state = gsResults
				return m, nil
			case gsResults, gsError:
				m.state = gsInput
				m.input.Focus()
				return m, nil
			}
		case "enter":
			if m.state == gsResults {
				if item, ok := m.list.SelectedItem().(globalSearchItem); ok {
					r := item.result
					m.selectedResult = &r
					m.state = gsDetail
					m.vp.SetContent(renderGlobalSearchDetail(r))
					m.vp.GotoTop()
					return m, nil
				}
			}
		case "r":
			if m.state == gsResults && m.term != "" {
				m.state = gsLoading
				m.fetchingMore = false
				_ = m.list.SetItems([]list.Item{})
				return m, tea.Batch(m.spin.Tick, m.doSearch())
			}
		}
	}

	switch m.state {
	case gsResults:
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	case gsDetail:
		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(msg)
		return m, cmd
	}

	return m, nil
}

// ── View ──────────────────────────────────────────────────────────────────────

func (m globalSearchModel) view() string {
	switch m.state {
	case gsInput:
		return m.viewInput()
	case gsLoading:
		return "\n  " + m.spin.View() + "  Searching for \"" + m.term + "\"…"
	case gsResults:
		return m.viewResults()
	case gsDetail:
		return m.viewDetail()
	case gsError:
		help := helpStyle.Render("esc: back  q: quit")
		return "\n\n  " + errorStyle.Render("Error: "+m.err.Error()) + "\n\n  " + help
	}
	return ""
}

func (m globalSearchModel) viewInput() string {
	var sb strings.Builder
	sb.WriteString(menuTitleStyle.Render("Search Backstage"))
	sb.WriteString("\n\n  ")
	sb.WriteString(fieldLabelStyle.Render("Search: "))
	sb.WriteString(m.input.View())
	sb.WriteString("\n")
	for i := 4; i < m.height-1; i++ {
		sb.WriteString("\n")
	}
	sb.WriteString("  ")
	sb.WriteString(helpStyle.Render("enter: search  esc: main menu"))
	return sb.String()
}

func (m globalSearchModel) viewResults() string {
	header := headerStyle.Render(fmt.Sprintf("Search: \"%s\"  %d results", m.term, m.totalItems))

	var body string
	if len(m.list.Items()) == 0 && !m.fetchingMore {
		body = "\n\n  " + helpStyle.Render("No results found.")
		for i := 3; i < m.height-2; i++ {
			body += "\n"
		}
	} else {
		body = m.list.View()
	}

	helpText := "↑/↓: navigate  enter: details  /: filter  r: re-search  esc: back  q: quit"
	var bottomLine string
	if m.fetchingMore {
		bottomLine = m.spin.View() + " " + helpStyle.Render("loading more…  "+helpText)
	} else {
		bottomLine = helpStyle.Render(helpText)
	}
	return header + "\n" + body + "\n" + bottomLine
}

func (m globalSearchModel) viewDetail() string {
	if m.selectedResult == nil {
		return ""
	}
	doc := m.selectedResult.Document
	title := doc.Title
	if title == "" {
		title = doc.Location
	}
	header := headerStyle.Render(m.selectedResult.Type + "  " + title)
	help := helpStyle.Render("↑/↓/pgup/pgdn: scroll  esc: back  q: quit")
	return header + "\n" + m.vp.View() + "\n" + help
}

func renderGlobalSearchDetail(r globalSearchResult) string {
	var sb strings.Builder
	doc := r.Document

	section := func(name string) {
		sb.WriteString("\n")
		sb.WriteString(sectionHeaderStyle.Render("── " + name))
		sb.WriteString("\n")
	}
	field := func(label, value string) {
		if value == "" {
			return
		}
		sb.WriteString(fieldLabelStyle.Render(label))
		sb.WriteString(fieldValueStyle.Render(value))
		sb.WriteString("\n")
	}

	section("Result")
	field("result type:  ", r.Type)
	field("rank:         ", fmt.Sprintf("%d", r.Rank))
	field("location:     ", doc.Location)

	section("Document")
	field("title:         ", doc.Title)
	field("kind:          ", doc.Kind)
	field("namespace:     ", doc.Namespace)
	field("type:          ", doc.Type)
	field("componentType: ", doc.ComponentType)
	field("owner:         ", doc.Owner)
	field("lifecycle:     ", doc.Lifecycle)

	if doc.Text != "" {
		section("Description")
		sb.WriteString(fieldValueStyle.Render(doc.Text))
		sb.WriteString("\n")
	}

	return sb.String()
}
