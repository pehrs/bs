package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// ── Top-level screen state ────────────────────────────────────────────────────

type screenState int

const (
	screenMenu screenState = iota
	screenListAll
)

type mainMenuEntry struct {
	title string
	desc  string
}

var mainMenuEntries = []mainMenuEntry{
	{"List Catalog Entities", "browse and search catalog entities by kind"},
}

// ── App model ─────────────────────────────────────────────────────────────────

type appModel struct {
	screen  screenState
	menuIdx int
	listAll listAllModel
	width   int
	height  int
	client  backstageClient
}

func newAppModel(client backstageClient) appModel {
	return appModel{
		screen:  screenMenu,
		listAll: newListAllModel(client, 0, 0),
		client:  client,
	}
}

func (m appModel) Init() tea.Cmd {
	return nil
}

func (m appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Always propagate to listAll so dimensions are correct when entering it.
		m.listAll, _ = m.listAll.update(msg)
		return m, nil

	case backToMenuMsg:
		m.screen = screenMenu
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}

	switch m.screen {
	case screenMenu:
		return m.updateMenu(msg)
	case screenListAll:
		var cmd tea.Cmd
		m.listAll, cmd = m.listAll.update(msg)
		return m, cmd
	}

	return m, nil
}

func (m appModel) updateMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "q":
		return m, tea.Quit
	case "up", "ctrl+p", "k":
		if m.menuIdx > 0 {
			m.menuIdx--
		}
	case "down", "ctrl+n", "j":
		if m.menuIdx < len(mainMenuEntries)-1 {
			m.menuIdx++
		}
	case "enter":
		switch m.menuIdx {
		case 0:
			m.listAll = newListAllModel(m.client, m.width, m.height)
			m.screen = screenListAll
		}
	}
	return m, nil
}

func (m appModel) View() string {
	switch m.screen {
	case screenMenu:
		return renderMainMenu(m.menuIdx, m.width, m.height)
	case screenListAll:
		return m.listAll.view()
	}
	return ""
}

func renderMainMenu(activeIdx, _, height int) string {
	var sb strings.Builder
	sb.WriteString(menuTitleStyle.Render("Backstage Catalog Browser"))
	sb.WriteString("\n\n")

	for i, entry := range mainMenuEntries {
		if i == activeIdx {
			sb.WriteString("  ")
			sb.WriteString(menuSelectedStyle.Render("▶ " + entry.title))
		} else {
			sb.WriteString("  ")
			sb.WriteString(menuNormalStyle.Render("  " + entry.title))
		}
		sb.WriteString("  ")
		sb.WriteString(menuDescStyle.Render(entry.desc))
		sb.WriteString("\n")
	}

	used := 4 + len(mainMenuEntries) + 2
	for i := used; i < height-1; i++ {
		sb.WriteString("\n")
	}
	sb.WriteString("  ")
	sb.WriteString(helpStyle.Render("↑/↓: navigate  enter: select  q: quit"))
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
		newAppModel(newBackstageClient(baseURL, token)),
		tea.WithAltScreen(),
	)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
