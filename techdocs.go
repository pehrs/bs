package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"gopkg.in/yaml.v3"
)

// glamourStyle is detected once before bubbletea starts (see initGlamourStyle).
var glamourStyle = "dark"

// initGlamourStyle probes the terminal background colour and caches the
// appropriate glamour style name. It MUST be called before tea.NewProgram —
// termenv queries stdin/stdout directly, which conflicts with bubbletea's
// input loop and causes multi-hundred-millisecond timeouts if called later.
func initGlamourStyle() {
	if s := os.Getenv("GLAMOUR_STYLE"); s != "" {
		glamourStyle = s
		return
	}
	if termenv.NewOutput(os.Stdout).HasDarkBackground() {
		glamourStyle = "dark"
	} else {
		glamourStyle = "light"
	}
}

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

// ── History ───────────────────────────────────────────────────────────────────

type historyEntry struct {
	file     string
	rendered string // rendered markdown content (for restoring without re-fetch)
	links    []navEntry
}

// ── Model ─────────────────────────────────────────────────────────────────────

type techdocsModel struct {
	state           techdocsState
	entity          Entity
	baseURL         string // raw-content base URL with trailing slash
	docsDir         string // docs_dir from mkdocs.yml (default "docs")
	startFile       string // if set, open this page directly after loading nav
	nav             []navEntry
	navList         list.Model
	vp              viewport.Model
	spin            spinner.Model
	pageTitle       string
	currentFile     string       // file path of the currently displayed page
	renderedContent string       // rendered markdown stored for history
	pageLinks       []navEntry   // internal links extracted from the current page
	linkList        list.Model   // list view for in-page links
	showLinks       bool         // whether the link-selection panel is active
	pageHistory     []historyEntry
	width           int
	height          int
	err             error
}

// ── Messages ──────────────────────────────────────────────────────────────────

type techDocsMkdocsLoadedMsg struct {
	docsDir string
	nav     []navEntry
}

type techDocsPageLoadedMsg struct {
	file    string // relative to docsDir
	content string // raw markdown
}

// ── Constructor ───────────────────────────────────────────────────────────────

func newTechdocsModel(entity Entity, startFile string, width, height int) (techdocsModel, tea.Cmd) {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = spinnerStyle

	makeList := func() list.Model {
		delegate := list.NewDefaultDelegate()
		delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
			BorderLeftForeground(lipgloss.Color("#0079C6"))
		l := list.New([]list.Item{}, delegate, width, max(0, height-2))
		l.SetShowTitle(false)
		l.SetShowHelp(false)
		l.SetFilteringEnabled(false)
		return l
	}

	vp := viewport.New(width, max(0, height-2))

	m := techdocsModel{
		entity:    entity,
		docsDir:   "docs",
		startFile: startFile,
		navList:   makeList(),
		linkList:  makeList(),
		vp:        vp,
		spin:      sp,
		width:     width,
		height:    height,
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
		return techDocsPageLoadedMsg{file: file, content: content}
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

// ── Link extraction ───────────────────────────────────────────────────────────

var mdLinkRe = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)

// extractInternalLinks parses inline markdown links from raw markdown and
// returns navEntries for links that resolve to .md files within the same site.
// currentFile is relative to docsDir (e.g. "guide/setup.md").
func extractInternalLinks(markdown, currentFile string) []navEntry {
	dir := path.Dir(currentFile)
	seen := map[string]bool{}
	var out []navEntry

	for _, m := range mdLinkRe.FindAllStringSubmatch(markdown, -1) {
		text := m[1]
		href := m[2]

		// Skip external URLs and bare anchors.
		if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") ||
			strings.HasPrefix(href, "mailto:") || strings.HasPrefix(href, "#") {
			continue
		}

		// Strip anchor fragment.
		if idx := strings.Index(href, "#"); idx >= 0 {
			href = href[:idx]
		}
		if href == "" {
			continue
		}

		// Resolve relative to the current page's directory.
		resolved := path.Join(dir, href)

		// Normalise to a .md path.
		switch {
		case strings.HasSuffix(resolved, "/") || !strings.Contains(path.Base(resolved), "."):
			resolved = strings.TrimSuffix(resolved, "/") + "/index.md"
		case !strings.HasSuffix(resolved, ".md"):
			continue // skip non-markdown links (images, PDFs, …)
		}

		if seen[resolved] {
			continue
		}
		seen[resolved] = true
		out = append(out, navEntry{title: text, file: resolved})
	}
	return out
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
		case strings.HasPrefix(origin, "file:"): // Backstage may emit file:/ or file:// or file:///
			rawOrigin = origin
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

// httpGet fetches a URL (http/https or any file: variant) and returns the body.
// file: URL forms handled: file:///path  file://path  file:/path  file:path
func httpGet(rawURL string) (string, error) {
	if strings.HasPrefix(rawURL, "file:") {
		// Strip "file:" then strip at most two leading slashes so that
		// file:///abs, file://abs, file:/abs all yield /abs (absolute path)
		// and file:rel yields rel (relative path).
		p := rawURL[5:] // after "file:"
		if strings.HasPrefix(p, "//") {
			p = p[2:] // strip authority prefix; for file:///path this leaves /path
		}
		data, err := os.ReadFile(p)
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
		m.linkList.SetSize(msg.Width, max(0, msg.Height-2))
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
		setCmd := m.navList.SetItems(items)
		if m.startFile != "" {
			m.state = techdocsLoadingPage
			return m, tea.Batch(setCmd, m.spin.Tick,
				doFetchTechDocsPage(m.baseURL, m.docsDir, m.startFile))
		}
		m.state = techdocsNav
		return m, setCmd

	case techDocsPageLoadedMsg:
		rendered, err := renderMarkdown(msg.content, m.width)
		if err != nil {
			rendered = msg.content
		}
		links := extractInternalLinks(msg.content, msg.file)
		linkItems := make([]list.Item, len(links))
		for i, e := range links {
			linkItems[i] = e
		}

		m.currentFile = msg.file
		m.pageTitle = msg.file
		m.renderedContent = rendered
		m.pageLinks = links
		m.showLinks = false
		m.vp.SetContent(rendered)
		m.vp.GotoTop()
		cmd := m.linkList.SetItems(linkItems)
		m.state = techdocsPage
		return m, cmd

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
				if m.showLinks {
					m.showLinks = false
					return m, nil
				}
				if len(m.pageHistory) > 0 {
					return m.popHistory()
				}
				m.state = techdocsNav
				return m, nil
			case techdocsNav, techdocsError:
				return m, func() tea.Msg { return backToPrevMsg{} }
			}

		case "enter":
			if m.state == techdocsNav {
				if item, ok := m.navList.SelectedItem().(navEntry); ok {
					m.pageHistory = nil // fresh navigation from the nav clears history
					m.state = techdocsLoadingPage
					return m, tea.Batch(m.spin.Tick,
						doFetchTechDocsPage(m.baseURL, m.docsDir, item.file))
				}
			}
			if m.state == techdocsPage && m.showLinks {
				if item, ok := m.linkList.SelectedItem().(navEntry); ok {
					m.pushHistory()
					m.showLinks = false
					m.state = techdocsLoadingPage
					return m, tea.Batch(m.spin.Tick,
						doFetchTechDocsPage(m.baseURL, m.docsDir, item.file))
				}
			}

		case "l":
			if m.state == techdocsPage && !m.showLinks && len(m.pageLinks) > 0 {
				m.showLinks = true
				return m, nil
			}
		}
	}

	switch m.state {
	case techdocsNav:
		var cmd tea.Cmd
		m.navList, cmd = m.navList.Update(msg)
		return m, cmd
	case techdocsPage:
		if m.showLinks {
			var cmd tea.Cmd
			m.linkList, cmd = m.linkList.Update(msg)
			return m, cmd
		}
		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(msg)
		return m, cmd
	}

	return m, nil
}

// pushHistory saves the current page state onto the history stack.
func (m *techdocsModel) pushHistory() {
	m.pageHistory = append(m.pageHistory, historyEntry{
		file:     m.currentFile,
		rendered: m.renderedContent,
		links:    m.pageLinks,
	})
}

// popHistory restores the previous page from the history stack without re-fetching.
func (m techdocsModel) popHistory() (techdocsModel, tea.Cmd) {
	entry := m.pageHistory[len(m.pageHistory)-1]
	m.pageHistory = m.pageHistory[:len(m.pageHistory)-1]

	m.currentFile = entry.file
	m.pageTitle = entry.file
	m.renderedContent = entry.rendered
	m.pageLinks = entry.links
	m.showLinks = false

	items := make([]list.Item, len(entry.links))
	for i, e := range entry.links {
		items[i] = e
	}
	m.vp.SetContent(entry.rendered)
	m.vp.GotoTop()
	cmd := m.linkList.SetItems(items)
	m.state = techdocsPage
	return m, cmd
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
		if m.showLinks {
			help := helpStyle.Render("↑/↓: navigate  enter: follow link  esc: cancel  q: quit")
			return title + "\n" + m.linkList.View() + "\n" + help
		}
		var parts []string
		parts = append(parts, "↑/↓/pgup/pgdn: scroll")
		if len(m.pageLinks) > 0 {
			parts = append(parts, "l: links")
		}
		if len(m.pageHistory) > 0 {
			parts = append(parts, "esc: back")
		} else {
			parts = append(parts, "esc: nav")
		}
		parts = append(parts, "q: quit")
		help := helpStyle.Render(strings.Join(parts, "  "))
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
		glamour.WithStandardStyle(glamourStyle),
		glamour.WithWordWrap(ww),
	)
	if err != nil {
		return "", err
	}
	return r.Render(content)
}

