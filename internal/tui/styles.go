package tui

import "github.com/charmbracelet/lipgloss"

var (
	colorAccent = lipgloss.Color("#E040FB")
	colorMuted  = lipgloss.Color("#9CA3AF")
	colorSubtle = lipgloss.Color("#555555")

	titleStyle = lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true)

	mutedStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	subtleStyle = lipgloss.NewStyle().
			Foreground(colorSubtle)

	cursorStyle = lipgloss.NewStyle().
			Foreground(colorAccent)
)
