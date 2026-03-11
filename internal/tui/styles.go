package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	colorAccent = lipgloss.Color("#E040FB")
	colorMuted  = lipgloss.Color("#9CA3AF")
	colorSubtle = lipgloss.Color("#555555")
	colorError  = lipgloss.Color("#FF5555")
	colorLocal   = lipgloss.Color("#50FA7B")
	colorWarning = lipgloss.Color("#F1FA8C")

	titleStyle = lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true)

	mutedStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	subtleStyle = lipgloss.NewStyle().
			Foreground(colorSubtle)

	cursorStyle = lipgloss.NewStyle().
			Foreground(colorAccent)

	errorStyle = lipgloss.NewStyle().
			Foreground(colorError)

	localStyle = lipgloss.NewStyle().
			Foreground(colorLocal)

	dimStyle = lipgloss.NewStyle().
			Foreground(colorSubtle)
)

// placeOverlay centers fg on top of bg, dimming the background.
func placeOverlay(width, height int, fg, bg string) string {
	bgLines := strings.Split(bg, "\n")
	for len(bgLines) < height {
		bgLines = append(bgLines, "")
	}
	if len(bgLines) > height {
		bgLines = bgLines[:height]
	}

	// Dim the background

	for i, line := range bgLines {
		bgLines[i] = dimStyle.Render(stripAnsi(line))
	}

	fgLines := strings.Split(fg, "\n")
	fgHeight := len(fgLines)
	fgWidth := lipgloss.Width(fg)

	startY := max(0, (height-fgHeight)/2)
	startX := max(0, (width-fgWidth)/2)

	for i, fgLine := range fgLines {
		bgY := startY + i
		if bgY >= len(bgLines) {
			break
		}
		bgLines[bgY] = overlayLine(bgLines[bgY], fgLine, startX, width)
	}

	return strings.Join(bgLines, "\n")
}

// overlayLine places a foreground line on top of a background line at position x.
func overlayLine(bgLine, fgLine string, startX, maxWidth int) string {
	bgRunes := []rune(stripAnsi(bgLine))
	for len(bgRunes) < maxWidth {
		bgRunes = append(bgRunes, ' ')
	}

	prefix := string(bgRunes[:startX])
	fgWidth := lipgloss.Width(fgLine)
	suffixStart := startX + fgWidth
	suffix := ""
	if suffixStart < len(bgRunes) {
		suffix = string(bgRunes[suffixStart:])
	}

	return dimStyle.Render(prefix) + fgLine + dimStyle.Render(suffix)
}

// stripAnsi removes ANSI escape codes from a string.
func stripAnsi(s string) string {
	var result strings.Builder
	inEscape := false
	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEscape = false
			}
			continue
		}
		result.WriteRune(r)
	}
	return result.String()
}
