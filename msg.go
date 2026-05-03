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
