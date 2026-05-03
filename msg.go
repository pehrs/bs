package main

// ── Messages ──────────────────────────────────────────────────────────────────

type pageLoadedMsg struct {
	entities   []Entity
	totalItems int
	nextCursor string
	kind       string // used to discard stale results if kind changed
}

type errMsg struct{ err error }

// backToMenuMsg is returned by a sub-screen when the user wants to go up to the main menu.
type backToMenuMsg struct{}

type searchPageMsg struct {
	entities   []Entity
	totalItems int
	nextCursor string
	term       string // used to discard stale results if the term changed
}

// viewTechDocsMsg is sent by a detail sub-screen to launch the TechDocs viewer.
type viewTechDocsMsg struct{ entity Entity }

// backToPrevMsg is sent by the TechDocs viewer to return to the previous screen.
type backToPrevMsg struct{}
