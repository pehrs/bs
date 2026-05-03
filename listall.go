package main

import (
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── States ────────────────────────────────────────────────────────────────────

type listAllState int

const (
	listAllKindSelect listAllState = iota
	listAllLoading
	listAllList
	listAllDetail
	listAllError
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

// ── Model ─────────────────────────────────────────────────────────────────────

type listAllModel struct {
	state          listAllState
	list           list.Model
	vp             viewport.Model
	spin           spinner.Model
	selectedEntity *Entity
	kindIdx        int
	totalItems     int
	fetchingMore   bool
	sortOrder      sortOrder
	sortReverse    bool
	width          int
	height         int
	err            error
	client         backstageClient
}

func newListAllModel(client backstageClient, width, height int) listAllModel {
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

	return listAllModel{
		state:  listAllKindSelect,
		list:   l,
		vp:     vp,
		spin:   sp,
		width:  width,
		height: height,
		client: client,
	}
}

// ── Commands ──────────────────────────────────────────────────────────────────

func (m listAllModel) doFetch() tea.Cmd {
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

func (m listAllModel) update(msg tea.Msg) (listAllModel, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width, msg.Height-2)
		m.vp.Width = msg.Width
		m.vp.Height = msg.Height - 2
		return m, nil

	case spinner.TickMsg:
		if m.state == listAllLoading || m.fetchingMore {
			var cmd tea.Cmd
			m.spin, cmd = m.spin.Update(msg)
			return m, cmd
		}
		return m, nil

	case pageLoadedMsg:
		if msg.kind != allKinds[m.kindIdx] {
			return m, nil
		}
		m.totalItems = msg.totalItems
		existing := m.list.Items()
		newItems := make([]list.Item, len(msg.entities))
		for i, e := range msg.entities {
			newItems[i] = entityItem{entity: e}
		}
		sorted := sortItems(append(existing, newItems...), m.sortOrder, m.sortReverse)
		setCmd := m.list.SetItems(sorted)
		if msg.nextCursor != "" {
			m.fetchingMore = true
			m.state = listAllList
			return m, tea.Batch(setCmd, doFetchNext(m.client, msg.kind, msg.nextCursor))
		}
		m.fetchingMore = false
		m.state = listAllList
		return m, setCmd

	case errMsg:
		m.err = msg.err
		m.state = listAllError
		return m, nil

	case tea.KeyMsg:
		if m.state == listAllList && m.list.FilterState() == list.Filtering {
			var cmd tea.Cmd
			m.list, cmd = m.list.Update(msg)
			return m, cmd
		}

		switch msg.String() {
		case "q":
			return m, tea.Quit

		case "esc":
			switch m.state {
			case listAllDetail:
				m.state = listAllList
				return m, nil
			case listAllList, listAllError:
				m.state = listAllKindSelect
				return m, nil
			case listAllKindSelect:
				return m, func() tea.Msg { return backToMenuMsg{} }
			}

		case "up", "ctrl-p", "k":
			if m.state == listAllKindSelect {
				m.kindIdx = (m.kindIdx - 1 + len(allKinds)) % len(allKinds)
				return m, nil
			}

		case "down", "ctrl-n", "j":
			if m.state == listAllKindSelect {
				m.kindIdx = (m.kindIdx + 1) % len(allKinds)
				return m, nil
			}

		case "enter":
			if m.state == listAllKindSelect {
				m.state = listAllLoading
				m.fetchingMore = false
				_ = m.list.SetItems([]list.Item{})
				return m, tea.Batch(m.spin.Tick, m.doFetch())
			}
			if m.state == listAllList {
				if item, ok := m.list.SelectedItem().(entityItem); ok {
					e := item.entity
					m.selectedEntity = &e
					m.state = listAllDetail
					m.vp.SetContent(renderEntityDetail(e))
					m.vp.GotoTop()
					return m, nil
				}
			}

		case "tab":
			if m.state == listAllList {
				m.kindIdx = (m.kindIdx + 1) % len(allKinds)
				m.state = listAllLoading
				m.fetchingMore = false
				_ = m.list.SetItems([]list.Item{})
				return m, tea.Batch(m.spin.Tick, m.doFetch())
			}

		case "shift+tab":
			if m.state == listAllList {
				m.kindIdx = (m.kindIdx - 1 + len(allKinds)) % len(allKinds)
				m.state = listAllLoading
				m.fetchingMore = false
				_ = m.list.SetItems([]list.Item{})
				return m, tea.Batch(m.spin.Tick, m.doFetch())
			}

		case "r":
			if m.state == listAllList {
				m.state = listAllLoading
				m.fetchingMore = false
				_ = m.list.SetItems([]list.Item{})
				return m, tea.Batch(m.spin.Tick, m.doFetch())
			}

		case "s":
			if m.state == listAllList {
				m.sortOrder = (m.sortOrder + 1) % numSortOrders
				sorted := sortItems(m.list.Items(), m.sortOrder, m.sortReverse)
				return m, m.list.SetItems(sorted)
			}

		case "S":
			if m.state == listAllList {
				m.sortReverse = !m.sortReverse
				sorted := sortItems(m.list.Items(), m.sortOrder, m.sortReverse)
				return m, m.list.SetItems(sorted)
			}
		}
	}

	switch m.state {
	case listAllList:
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	case listAllDetail:
		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(msg)
		return m, cmd
	}

	return m, nil
}

// ── View ──────────────────────────────────────────────────────────────────────

func (m listAllModel) view() string {
	switch m.state {
	case listAllKindSelect:
		return renderKindMenu(m.kindIdx, m.width, m.height)

	case listAllLoading:
		return "\n  " + m.spin.View() + "  Loading " + allKinds[m.kindIdx] + " entities…"

	case listAllError:
		tabs := renderKindTabs(m.kindIdx)
		help := helpStyle.Render("esc: back  q: quit")
		return tabs + "\n\n  " + errorStyle.Render("Error: "+m.err.Error()) + "\n\n  " + help

	case listAllList:
		tabs := renderKindTabs(m.kindIdx)
		dir := "↑"
		if m.sortReverse {
			dir = "↓"
		}
		sortLabel := sortIndicatorStyle.Render("  sort:" + sortOrderLabels[m.sortOrder] + dir)
		helpText := "↑/↓: navigate  enter: details  tab/shift+tab: kind  /: filter  s: sort field  S: reverse  r: refresh  esc: menu  q: quit"
		var bottomLine string
		if m.fetchingMore {
			bottomLine = m.spin.View() + " " + helpStyle.Render("loading more…  "+helpText)
		} else {
			bottomLine = helpStyle.Render(helpText)
		}
		return tabs + sortLabel + "\n" + m.list.View() + "\n" + bottomLine

	case listAllDetail:
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
	sb.WriteString(menuTitleStyle.Render("List Catalog Entities"))
	sb.WriteString("\n\n")
	sb.WriteString("  Select entity kind:\n\n")

	for i, kind := range allKinds {
		if i == activeIdx {
			sb.WriteString("  ")
			sb.WriteString(menuSelectedStyle.Render("▶ " + kind))
		} else {
			sb.WriteString("  ")
			sb.WriteString(menuNormalStyle.Render("  " + kind))
		}
		sb.WriteString("  ")
		sb.WriteString(menuDescStyle.Render(kindDescs[kind]))
		sb.WriteString("\n")
	}

	used := 4 + len(allKinds) + 2
	for i := used; i < height-1; i++ {
		sb.WriteString("\n")
	}
	sb.WriteString("  ")
	sb.WriteString(helpStyle.Render("↑/↓: navigate  enter: select  esc: main menu  q: quit"))
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

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
