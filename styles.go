package main

import "github.com/charmbracelet/lipgloss"

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
