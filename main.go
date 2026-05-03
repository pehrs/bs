package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── App state ─────────────────────────────────────────────────────────────────

type viewState int

const (
	viewKindSelect viewState = iota
	viewLoading
	viewList
	viewDetail
	viewError
)

var allKinds = []string{
	"All", "Component", "API", "User", "Group",
	"Resource", "System", "Domain", "Template", "Location",
}

var kindDescs = map[string]string{
	"All":       "browse all entity kinds",
	"Component": "services, websites, libraries, etc.",
	"API":       "API definitions (OpenAPI, gRPC, GraphQL…)",
	"User":      "user accounts",
	"Group":     "teams and organisational units",
	"Resource":  "databases, queues, buckets, etc.",
	"System":    "collections of related entities",
	"Domain":    "groups of systems",
	"Template":  "Scaffolder templates",
	"Location":  "entity source file locations",
}

type model struct {
	state          viewState
	list           list.Model
	vp             viewport.Model
	spin           spinner.Model
	selectedEntity *Entity
	kindIdx        int
	totalItems     int
	fetchingMore   bool
	width          int
	height         int
	err            error
	client         backstageClient
}

func newModel(client backstageClient) model {
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

	l := list.New([]list.Item{}, delegate, 0, 0)
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)

	vp := viewport.New(0, 0)

	return model{
		state:  viewKindSelect,
		list:   l,
		vp:     vp,
		spin:   sp,
		client: client,
	}
}

// ── Init ──────────────────────────────────────────────────────────────────────

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) doFetch() tea.Cmd {
	kind := allKinds[m.kindIdx]
	client := m.client
	return func() tea.Msg {
		entities, total, nextCursor, err := client.fetchPage(kind, "")
		if err != nil {
			return errMsg{err}
		}
		return pageLoadedMsg{entities: entities, totalItems: total, nextCursor: nextCursor, kind: kind}
	}
}

func doFetchNext(client backstageClient, kind, cursor string) tea.Cmd {
	return func() tea.Msg {
		entities, total, nextCursor, err := client.fetchPage(kind, cursor)
		if err != nil {
			return errMsg{err}
		}
		return pageLoadedMsg{entities: entities, totalItems: total, nextCursor: nextCursor, kind: kind}
	}
}

// ── Update ────────────────────────────────────────────────────────────────────

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// List view: 1 line tabs + list + 1 line help
		m.list.SetSize(msg.Width, msg.Height-2)
		// Detail view: 1 line header + viewport + 1 line help
		m.vp.Width = msg.Width
		m.vp.Height = msg.Height - 2
		return m, nil

	case spinner.TickMsg:
		if m.state == viewLoading || m.fetchingMore {
			var cmd tea.Cmd
			m.spin, cmd = m.spin.Update(msg)
			return m, cmd
		}
		return m, nil

	case pageLoadedMsg:
		// Discard results that arrived after the user switched to a different kind.
		if msg.kind != allKinds[m.kindIdx] {
			return m, nil
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
			m.state = viewList
			return m, tea.Batch(setCmd, doFetchNext(m.client, msg.kind, msg.nextCursor))
		}
		m.fetchingMore = false
		m.state = viewList
		return m, setCmd

	case errMsg:
		m.err = msg.err
		m.state = viewError
		return m, nil

	case tea.KeyMsg:
		// ctrl+c always quits
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		// While the list filter input is active, pass everything to the list
		if m.state == viewList && m.list.FilterState() == list.Filtering {
			var cmd tea.Cmd
			m.list, cmd = m.list.Update(msg)
			return m, cmd
		}

		switch msg.String() {
		case "q":
			return m, tea.Quit

		case "esc":
			switch m.state {
			case viewDetail:
				m.state = viewList
				return m, nil
			case viewList, viewError:
				m.state = viewKindSelect
				return m, nil
			}

		case "up", "k":
			if m.state == viewKindSelect {
				m.kindIdx = (m.kindIdx - 1 + len(allKinds)) % len(allKinds)
				return m, nil
			}

		case "down", "j":
			if m.state == viewKindSelect {
				m.kindIdx = (m.kindIdx + 1) % len(allKinds)
				return m, nil
			}

		case "enter":
			if m.state == viewKindSelect {
				m.state = viewLoading
				m.fetchingMore = false
				_ = m.list.SetItems([]list.Item{})
				return m, tea.Batch(m.spin.Tick, m.doFetch())
			}
			if m.state == viewList {
				if item, ok := m.list.SelectedItem().(entityItem); ok {
					e := item.entity
					m.selectedEntity = &e
					m.state = viewDetail
					m.vp.SetContent(renderEntityDetail(e))
					m.vp.GotoTop()
					return m, nil
				}
			}

		case "tab":
			if m.state == viewList {
				m.kindIdx = (m.kindIdx + 1) % len(allKinds)
				m.state = viewLoading
				m.fetchingMore = false
				_ = m.list.SetItems([]list.Item{})
				return m, tea.Batch(m.spin.Tick, m.doFetch())
			}

		case "shift+tab":
			if m.state == viewList {
				m.kindIdx = (m.kindIdx - 1 + len(allKinds)) % len(allKinds)
				m.state = viewLoading
				m.fetchingMore = false
				_ = m.list.SetItems([]list.Item{})
				return m, tea.Batch(m.spin.Tick, m.doFetch())
			}

		case "r":
			if m.state == viewList {
				m.state = viewLoading
				m.fetchingMore = false
				_ = m.list.SetItems([]list.Item{})
				return m, tea.Batch(m.spin.Tick, m.doFetch())
			}
		}
	}

	// Pass remaining events to the active component
	switch m.state {
	case viewList:
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	case viewDetail:
		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(msg)
		return m, cmd
	}

	return m, nil
}

// ── View ──────────────────────────────────────────────────────────────────────

func (m model) View() string {
	switch m.state {
	case viewKindSelect:
		return renderKindMenu(m.kindIdx, m.width, m.height)

	case viewLoading:
		return "\n  " + m.spin.View() + "  Loading " + allKinds[m.kindIdx] + " entities…"

	case viewError:
		tabs := renderKindTabs(m.kindIdx)
		help := helpStyle.Render("esc: back  q: quit")
		return tabs + "\n\n  " + errorStyle.Render("Error: "+m.err.Error()) + "\n\n  " + help

	case viewList:
		tabs := renderKindTabs(m.kindIdx)
		helpText := "↑/↓: navigate  enter: details  tab/shift+tab: kind  /: search  r: refresh  esc: menu  q: quit"
		var bottomLine string
		if m.fetchingMore {
			bottomLine = m.spin.View() + " " + helpStyle.Render("loading more…  "+helpText)
		} else {
			bottomLine = helpStyle.Render(helpText)
		}
		return tabs + "\n" + m.list.View() + "\n" + bottomLine

	case viewDetail:
		if m.selectedEntity == nil {
			return ""
		}
		title := headerStyle.Render(m.selectedEntity.Kind + "  " + m.selectedEntity.Metadata.Name)
		help := helpStyle.Render("↑/↓/pgup/pgdn: scroll  esc: back  q: quit")
		return title + "\n" + m.vp.View() + "\n" + help
	}
	return ""
}

func renderKindMenu(activeIdx, _, height int) string {
	var sb strings.Builder
	sb.WriteString(menuTitleStyle.Render("Backstage Catalog Browser"))
	sb.WriteString("\n\n")
	sb.WriteString("  Select entity kind:\n\n")

	for i, kind := range allKinds {
		cursor := "  "
		if i == activeIdx {
			cursor = "▶ "
			sb.WriteString("  ")
			sb.WriteString(menuSelectedStyle.Render(cursor + kind))
		} else {
			sb.WriteString("  ")
			sb.WriteString(menuNormalStyle.Render(cursor + kind))
		}
		desc := kindDescs[kind]
		sb.WriteString("  ")
		sb.WriteString(menuDescStyle.Render(desc))
		sb.WriteString("\n")
	}

	// pad to push help to bottom if terminal is tall enough
	used := 4 + len(allKinds) + 2
	for i := used; i < height-1; i++ {
		sb.WriteString("\n")
	}
	sb.WriteString("  ")
	sb.WriteString(helpStyle.Render("↑/↓: navigate  enter: select  q: quit"))
	return sb.String()
}

func renderKindTabs(activeIdx int) string {
	var sb strings.Builder
	for i, kind := range allKinds {
		if i == activeIdx {
			sb.WriteString(activeTabStyle.Render(kind))
		} else {
			sb.WriteString(inactiveTabStyle.Render(kind))
		}
	}
	return sb.String()
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	urlFlag := flag.String("url", "", "Backstage base URL (default: $BACKSTAGE_URL or http://localhost:7007)")
	tokenFlag := flag.String("token", "", "Backstage access token (default: $BACKSTAGE_TOKEN)")
	flag.Parse()

	baseURL := *urlFlag
	if baseURL == "" {
		baseURL = os.Getenv("BACKSTAGE_URL")
	}
	if baseURL == "" {
		baseURL = "http://localhost:7007"
	}

	token := *tokenFlag
	if token == "" {
		token = os.Getenv("BACKSTAGE_TOKEN")
	}

	p := tea.NewProgram(
		newModel(newBackstageClient(baseURL, token)),
		tea.WithAltScreen(),
	)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
