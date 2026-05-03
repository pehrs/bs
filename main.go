package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── Entity types ──────────────────────────────────────────────────────────────

type Entity struct {
	APIVersion string                 `json:"apiVersion"`
	Kind       string                 `json:"kind"`
	Metadata   EntityMetadata         `json:"metadata"`
	Spec       map[string]interface{} `json:"spec"`
	Relations  []Relation             `json:"relations"`
}

type EntityMetadata struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	UID         string            `json:"uid"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	Tags        []string          `json:"tags"`
}

type Relation struct {
	TargetRef string `json:"targetRef"`
	Type      string `json:"type"`
}

type queryResponse struct {
	Items      []Entity `json:"items"`
	TotalItems int      `json:"totalItems"`
}

// ── Backstage client ──────────────────────────────────────────────────────────

type backstageClient struct {
	baseURL string
	token   string
	http    *http.Client
}

func newBackstageClient(baseURL, token string) backstageClient {
	return backstageClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		http:    &http.Client{},
	}
}

func (c backstageClient) fetchEntities(kind string) ([]Entity, int, error) {
	endpoint := c.baseURL + "/api/catalog/entities/by-query"

	params := url.Values{}
	params.Set("limit", "1024")
	if kind != "" && kind != "All" {
		params.Set("filter", "kind="+kind)
	}

	req, err := http.NewRequest("GET", endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return nil, 0, fmt.Errorf("creating request: %w", err)
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("connecting to Backstage: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, 0, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result queryResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, 0, fmt.Errorf("decoding response: %w", err)
	}
	return result.Items, result.TotalItems, nil
}

// ── Messages ──────────────────────────────────────────────────────────────────

type entitiesLoadedMsg struct {
	entities   []Entity
	totalItems int
}

type errMsg struct{ err error }

// ── List item ─────────────────────────────────────────────────────────────────

type entityItem struct{ entity Entity }

func (i entityItem) Title() string {
	if i.entity.Metadata.Title != "" {
		return i.entity.Metadata.Title
	}
	return i.entity.Metadata.Name
}

func (i entityItem) Description() string {
	parts := []string{i.entity.Kind}
	if ns := i.entity.Metadata.Namespace; ns != "" && ns != "default" {
		parts = append(parts, ns)
	}
	if d := i.entity.Metadata.Description; d != "" {
		if len(d) > 72 {
			d = d[:69] + "..."
		}
		parts = append(parts, d)
	}
	return strings.Join(parts, " · ")
}

func (i entityItem) FilterValue() string {
	return i.entity.Kind + "/" + i.entity.Metadata.Namespace + "/" + i.entity.Metadata.Name
}

// ── Styles ────────────────────────────────────────────────────────────────────

var (
	menuTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFDF5")).
			Background(lipgloss.Color("#25A065")).
			Padding(0, 1).
			MarginBottom(1)

	menuSelectedStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#282A36")).
				Background(lipgloss.Color("#FF79C6")).
				Padding(0, 1)

	menuNormalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F8F8F2")).
			Padding(0, 1)

	menuDescStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6272A4"))

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFDF5")).
			Background(lipgloss.Color("#25A065")).
			Padding(0, 1)

	activeTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#282A36")).
			Background(lipgloss.Color("#FF79C6")).
			Padding(0, 1).
			MarginRight(1)

	inactiveTabStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFFDF5")).
				Background(lipgloss.Color("#44475A")).
				Padding(0, 1).
				MarginRight(1)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6272A4"))

	fieldLabelStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF79C6"))

	fieldValueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F8F8F2"))

	sectionHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#50FA7B"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF5555")).
			Bold(true)

	spinnerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF79C6"))
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
		entities, total, err := client.fetchEntities(kind)
		if err != nil {
			return errMsg{err}
		}
		return entitiesLoadedMsg{entities: entities, totalItems: total}
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
		if m.state == viewLoading {
			var cmd tea.Cmd
			m.spin, cmd = m.spin.Update(msg)
			return m, cmd
		}
		return m, nil

	case entitiesLoadedMsg:
		m.totalItems = msg.totalItems
		items := make([]list.Item, len(msg.entities))
		for i, e := range msg.entities {
			items[i] = entityItem{entity: e}
		}
		cmd := m.list.SetItems(items)
		m.state = viewList
		return m, cmd

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
				return m, tea.Batch(m.spin.Tick, m.doFetch())
			}

		case "shift+tab":
			if m.state == viewList {
				m.kindIdx = (m.kindIdx - 1 + len(allKinds)) % len(allKinds)
				m.state = viewLoading
				return m, tea.Batch(m.spin.Tick, m.doFetch())
			}

		case "r":
			if m.state == viewList {
				m.state = viewLoading
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
		help := helpStyle.Render(
			"↑/↓: navigate  enter: details  tab/shift+tab: kind  /: search  r: refresh  q: quit",
		)
		return tabs + "\n" + m.list.View() + "\n" + help

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
			sb.WriteString(menuSelectedStyle.Render(cursor+kind))
		} else {
			sb.WriteString("  ")
			sb.WriteString(menuNormalStyle.Render(cursor+kind))
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

// ── Detail renderer ───────────────────────────────────────────────────────────

func renderEntityDetail(e Entity) string {
	var sb strings.Builder

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

	section("Metadata")
	field("name:         ", e.Metadata.Name)
	field("namespace:    ", e.Metadata.Namespace)
	field("uid:          ", e.Metadata.UID)
	field("apiVersion:   ", e.APIVersion)
	if e.Metadata.Title != "" {
		field("title:        ", e.Metadata.Title)
	}
	if e.Metadata.Description != "" {
		field("description:  ", e.Metadata.Description)
	}
	if len(e.Metadata.Tags) > 0 {
		field("tags:         ", strings.Join(e.Metadata.Tags, ", "))
	}

	if len(e.Metadata.Labels) > 0 {
		section("Labels")
		for _, k := range sortedStringKeys(e.Metadata.Labels) {
			field("  "+k+": ", e.Metadata.Labels[k])
		}
	}

	if len(e.Metadata.Annotations) > 0 {
		section("Annotations")
		for _, k := range sortedStringKeys(e.Metadata.Annotations) {
			field("  "+k+": ", e.Metadata.Annotations[k])
		}
	}

	if len(e.Spec) > 0 {
		section("Spec")
		renderSpecMap(&sb, e.Spec, 1)
	}

	if len(e.Relations) > 0 {
		section("Relations")
		byType := make(map[string][]string)
		for _, r := range e.Relations {
			byType[r.Type] = append(byType[r.Type], r.TargetRef)
		}
		relTypes := make([]string, 0, len(byType))
		for t := range byType {
			relTypes = append(relTypes, t)
		}
		sort.Strings(relTypes)
		for _, t := range relTypes {
			field("  "+t+": ", strings.Join(byType[t], "\n             "))
		}
	}

	return sb.String()
}

func renderSpecMap(sb *strings.Builder, m map[string]interface{}, depth int) {
	indent := strings.Repeat("  ", depth)
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		switch val := m[k].(type) {
		case map[string]interface{}:
			sb.WriteString(indent)
			sb.WriteString(fieldLabelStyle.Render(k + ":"))
			sb.WriteString("\n")
			renderSpecMap(sb, val, depth+1)
		case []interface{}:
			strs := make([]string, len(val))
			for i, item := range val {
				strs[i] = fmt.Sprintf("%v", item)
			}
			sb.WriteString(indent)
			sb.WriteString(fieldLabelStyle.Render(k + ": "))
			sb.WriteString(fieldValueStyle.Render(strings.Join(strs, ", ")))
			sb.WriteString("\n")
		default:
			sb.WriteString(indent)
			sb.WriteString(fieldLabelStyle.Render(k + ": "))
			sb.WriteString(fieldValueStyle.Render(fmt.Sprintf("%v", val)))
			sb.WriteString("\n")
		}
	}
}

func sortedStringKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
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
