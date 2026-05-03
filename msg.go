package main

// ── Messages ──────────────────────────────────────────────────────────────────

type pageLoadedMsg struct {
	entities   []Entity
	totalItems int
	nextCursor string
	kind       string // used to discard stale results if kind changed
}

type errMsg struct{ err error }
