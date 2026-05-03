package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"gopkg.in/yaml.v3"
)

// ── States ────────────────────────────────────────────────────────────────────

type techdocsState int

const (
	techdocsLoadingMeta techdocsState = iota // fetching mkdocs.yml
	techdocsNav                              // showing page navigation list
	techdocsLoadingPage                      // fetching a markdown file
	techdocsPage                             // showing rendered markdown
	techdocsError
)

// ── Nav entry (list.Item) ─────────────────────────────────────────────────────

type navEntry struct {
	title string
	file  string // path relative to docs_dir
}

func (n navEntry) Title() string       { return n.title }
func (n navEntry) Description() string { return n.file }
func (n navEntry) FilterValue() string { return n.title }

// ── Model ─────────────────────────────────────────────────────────────────────

type techdocsModel struct {
	state     techdocsState
	entity    Entity
	baseURL   string // raw-content base URL with trailing slash
	docsDir   string // docs_dir from mkdocs.yml (default "docs")
	nav       []navEntry
	navList   list.Model
	vp        viewport.Model
	spin      spinner.Model
	pageTitle string
	width     int
	height    int
	err       error
}

// ── Messages ──────────────────────────────────────────────────────────────────

type techDocsMkdocsLoadedMsg struct {
	docsDir string
	nav     []navEntry
}

type techDocsPageLoadedMsg struct {
	title   string
	content string
}

// ── Constructor ───────────────────────────────────────────────────────────────

func newTechdocsModel(entity Entity, width, height int) (techdocsModel, tea.Cmd) {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = spinnerStyle

	delegate := list.NewDefaultDelegate()
	l := list.New([]list.Item{}, delegate, width, max(0, height-2))
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(false)

	vp := viewport.New(width, max(0, height-2))

	m := techdocsModel{
		entity:  entity,
		docsDir: "docs",
		navList: l,
		vp:      vp,
		spin:    sp,
		width:   width,
		height:  height,
	}

	baseURL, err := deriveTechDocsBaseURL(entity)
	if err != nil {
		m.err = err
		m.state = techdocsError
		return m, nil
	}

	m.baseURL = baseURL
	m.state = techdocsLoadingMeta
	return m, tea.Batch(sp.Tick, doFetchMkdocsYAML(baseURL))
}

// ── Commands ──────────────────────────────────────────────────────────────────

func doFetchMkdocsYAML(baseURL string) tea.Cmd {
	return func() tea.Msg {
		docsDir, nav, err := fetchAndParseMkdocs(baseURL)
		if err != nil {
			return errMsg{err}
		}
		return techDocsMkdocsLoadedMsg{docsDir: docsDir, nav: nav}
	}
}

func doFetchTechDocsPage(baseURL, docsDir, file string) tea.Cmd {
	rawURL := baseURL + docsDir + "/" + file
	return func() tea.Msg {
		content, err := httpGet(rawURL)
		if err != nil {
			return errMsg{err}
		}
		return techDocsPageLoadedMsg{title: file, content: content}
	}
}

// ── mkdocs.yml fetching and parsing ──────────────────────────────────────────

func fetchAndParseMkdocs(baseURL string) (string, []navEntry, error) {
	content, err := httpGet(baseURL + "mkdocs.yml")
	if err != nil {
		content, err = httpGet(baseURL + "mkdocs.yaml")
	}
	if err != nil {
		// No mkdocs.yml accessible; fall back to a single index page.
		return "docs", []navEntry{{title: "Home", file: "index.md"}}, nil
	}
	return parseMkdocsYAML(content)
}

func parseMkdocsYAML(raw string) (string, []navEntry, error) {
	var cfg struct {
		DocsDir string      `yaml:"docs_dir"`
		Nav     interface{} `yaml:"nav"`
	}
	if err := yaml.Unmarshal([]byte(raw), &cfg); err != nil {
		return "docs", []navEntry{{title: "Home", file: "index.md"}}, nil
	}

	docsDir := cfg.DocsDir
	if docsDir == "" {
		docsDir = "docs"
	}

	entries := flattenNav(cfg.Nav, "")
	if len(entries) == 0 {
		entries = []navEntry{{title: "Home", file: "index.md"}}
	}
	return docsDir, entries, nil
}

// flattenNav recursively flattens mkdocs nav YAML into a list of navEntry.
// Each nav node is one of:
//
//	string            – untitled page (filename used as title)
//	map[string]iface  – {"Title": "file.md"} or {"Title": [children]}
//	[]interface{}     – list of the above
func flattenNav(node interface{}, prefix string) []navEntry {
	switch v := node.(type) {
	case []interface{}:
		var out []navEntry
		for _, item := range v {
			out = append(out, flattenNav(item, prefix)...)
		}
		return out

	case map[string]interface{}:
		var out []navEntry
		for title, val := range v {
			fullTitle := title
			if prefix != "" {
				fullTitle = prefix + " / " + title
			}
			switch vv := val.(type) {
			case string:
				out = append(out, navEntry{title: fullTitle, file: vv})
			default:
				out = append(out, flattenNav(vv, fullTitle)...)
			}
		}
		return out

	case string:
		// Untitled entry – use the filename (without extension) as title.
		title := strings.TrimSuffix(path.Base(v), ".md")
		if prefix != "" {
			title = prefix + " / " + title
		}
		return []navEntry{{title: title, file: v}}
	}

	return nil
}

// ── URL helpers ───────────────────────────────────────────────────────────────

// deriveTechDocsBaseURL resolves the raw-content base URL for an entity's
// TechDocs source by inspecting its backstage.io annotations.
func deriveTechDocsBaseURL(entity Entity) (string, error) {
	ann := entity.Metadata.Annotations
	ref := ann["backstage.io/techdocs-ref"]
	origin := ann["backstage.io/managed-by-origin-location"]

	switch {
	case strings.HasPrefix(ref, "url:"):
		base := strings.TrimPrefix(ref, "url:")
		if !strings.HasSuffix(base, "/") {
			base += "/"
		}
		return toRawGitHubURL(base), nil

	case strings.HasPrefix(ref, "dir:"):
		var rawOrigin string
		switch {
		case strings.HasPrefix(origin, "url:"):
			rawOrigin = toRawGitHubURL(strings.TrimPrefix(origin, "url:"))
		case strings.HasPrefix(origin, "file:/"): // The BS response does not have the double slashes.
			rawOrigin = origin // keep the file:/ prefix intact
		default:
			return "", fmt.Errorf("unsupported origin location type: %q", origin)
		}

		// Strip the filename to get the containing directory.
		slash := strings.LastIndex(rawOrigin, "/")
		if slash < 0 {
			return "", fmt.Errorf("cannot parse origin location: %q", origin)
		}
		baseDir := rawOrigin[:slash+1]

		relPath := strings.TrimPrefix(ref, "dir:")
		relPath = strings.TrimPrefix(relPath, "./")
		if relPath != "" && relPath != "." {
			baseDir = baseDir + relPath + "/"
		}
		return baseDir, nil
	}

	return "", fmt.Errorf("unsupported techdocs-ref format: %q", ref)
}

// toRawGitHubURL converts a GitHub HTML blob URL to a raw.githubusercontent.com URL.
// Non-GitHub URLs are returned unchanged.
func toRawGitHubURL(u string) string {
	if strings.HasPrefix(u, "https://github.com/") {
		u = strings.Replace(u, "https://github.com/", "https://raw.githubusercontent.com/", 1)
		u = strings.Replace(u, "/blob/", "/", 1)
	}
	return u
}

// httpGet fetches a URL (http/https or file://) and returns the body as a string.
func httpGet(rawURL string) (string, error) {
	if strings.HasPrefix(rawURL, "file://") {
		localPath := strings.TrimPrefix(rawURL, "file://")
		data, err := os.ReadFile(localPath)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}

	resp, err := http.Get(rawURL) //nolint:gosec
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d fetching %s", resp.StatusCode, rawURL)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// hasTechDocs reports whether the entity has the techdocs-ref annotation.
func hasTechDocs(e Entity) bool {
	return e.Metadata.Annotations != nil &&
		e.Metadata.Annotations["backstage.io/techdocs-ref"] != ""
}

// ── Update ────────────────────────────────────────────────────────────────────

func (m techdocsModel) update(msg tea.Msg) (techdocsModel, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.vp.Width = msg.Width
		m.vp.Height = max(0, msg.Height-2)
		m.navList.SetSize(msg.Width, max(0, msg.Height-2))
		return m, nil

	case spinner.TickMsg:
		if m.state == techdocsLoadingMeta || m.state == techdocsLoadingPage {
			var cmd tea.Cmd
			m.spin, cmd = m.spin.Update(msg)
			return m, cmd
		}
		return m, nil

	case techDocsMkdocsLoadedMsg:
		m.docsDir = msg.docsDir
		m.nav = msg.nav
		items := make([]list.Item, len(msg.nav))
		for i, e := range msg.nav {
			items[i] = e
		}
		cmd := m.navList.SetItems(items)
		m.state = techdocsNav
		return m, cmd

	case techDocsPageLoadedMsg:
		rendered, err := renderMarkdown(msg.content, m.width)
		if err != nil {
			rendered = msg.content
		}
		m.pageTitle = msg.title
		m.vp.SetContent(rendered)
		m.vp.GotoTop()
		m.state = techdocsPage
		return m, nil

	case errMsg:
		m.err = msg.err
		m.state = techdocsError
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q":
			return m, tea.Quit

		case "esc":
			switch m.state {
			case techdocsPage:
				m.state = techdocsNav
				return m, nil
			case techdocsNav, techdocsError:
				return m, func() tea.Msg { return backToPrevMsg{} }
			}

		case "enter":
			if m.state == techdocsNav {
				if item, ok := m.navList.SelectedItem().(navEntry); ok {
					m.state = techdocsLoadingPage
					return m, tea.Batch(m.spin.Tick,
						doFetchTechDocsPage(m.baseURL, m.docsDir, item.file))
				}
			}
		}
	}

	switch m.state {
	case techdocsNav:
		var cmd tea.Cmd
		m.navList, cmd = m.navList.Update(msg)
		return m, cmd
	case techdocsPage:
		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(msg)
		return m, cmd
	}

	return m, nil
}

// ── View ──────────────────────────────────────────────────────────────────────

func (m techdocsModel) view() string {
	name := m.entity.Metadata.Name
	switch m.state {
	case techdocsLoadingMeta:
		return "\n  " + m.spin.View() + "  Loading TechDocs for " + name + "…"
	case techdocsLoadingPage:
		return "\n  " + m.spin.View() + "  Loading page…"
	case techdocsError:
		help := helpStyle.Render("esc: back  q: quit")
		return "\n\n  " + errorStyle.Render("TechDocs error: "+m.err.Error()) + "\n\n  " + help
	case techdocsNav:
		title := headerStyle.Render("TechDocs: " + name)
		help := helpStyle.Render("↑/↓: navigate  enter: open  esc: back  q: quit")
		return title + "\n" + m.navList.View() + "\n" + help
	case techdocsPage:
		title := headerStyle.Render("TechDocs: " + name + "  " + m.pageTitle)
		help := helpStyle.Render("↑/↓/pgup/pgdn: scroll  esc: nav  q: quit")
		return title + "\n" + m.vp.View() + "\n" + help
	}
	return ""
}

// ── Markdown rendering ────────────────────────────────────────────────────────

func renderMarkdown(content string, width int) (string, error) {
	ww := width - 4
	if ww < 20 {
		ww = 20
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(ww),
	)
	if err != nil {
		return "", err
	}
	return r.Render(content)
}
