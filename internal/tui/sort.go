package tui

import (
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type sortMode int

const (
	sortByName sortMode = iota
	sortByNameArticle
	sortByNewest
	sortByCount
)

var sortLabels = [...]string{
	sortByName:        "Name",
	sortByNameArticle: "Name (including \"The\" / \"A\")",
	sortByNewest:      "Newest album",
	sortByCount:       "Album count",
}

type sortModel struct {
	cursor  int
	current sortMode
}

func newSortModel(current sortMode) sortModel {
	return sortModel{cursor: int(current), current: current}
}

func (m sortModel) Update(msg tea.Msg) (sortModel, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "j", "down":
			m.cursor = (m.cursor + 1) % len(sortLabels)
		case "k", "up":
			m.cursor = (m.cursor - 1 + len(sortLabels)) % len(sortLabels)
		case "enter":
			m.current = sortMode(m.cursor)
			return m, func() tea.Msg { return sortChosenMsg{mode: m.current} }
		case "esc":
			return m, func() tea.Msg { return sortCancelMsg{} }
		}
	}
	return m, nil
}

func (m sortModel) View(width, height int, bg string) string {
	modal := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorAccent).
		Padding(1, 2).
		Width(30)

	var s string
	s += titleStyle.Render("Sort by") + "\n\n"
	var sSb59 strings.Builder
	for i, label := range sortLabels {
		cursor := "  "
		style := mutedStyle
		if i == m.cursor {
			cursor = cursorStyle.Render("> ")
			style = lipgloss.NewStyle().Foreground(colorAccent)
		}
		sSb59.WriteString(cursor + style.Render(label) + "\n")
	}
	s += sSb59.String()

	rendered := modal.Render(s)
	return placeOverlay(width, height, rendered, bg)
}

type sortChosenMsg struct{ mode sortMode }
type sortCancelMsg struct{}

func sortArtists(items []artistItem, mode sortMode) {
	sort.SliceStable(items, func(i, j int) bool {
		switch mode {
		case sortByNameArticle:
			return strings.ToLower(items[i].name) < strings.ToLower(items[j].name)
		case sortByNewest:
			return items[i].newestAlbum < items[j].newestAlbum
		case sortByCount:
			return items[i].albumCount > items[j].albumCount
		default:
			return stripArticle(items[i].name) < stripArticle(items[j].name)
		}
	})
}

// stripArticle removes leading "The " or "A " for sort comparison.
func stripArticle(name string) string {
	lower := strings.ToLower(name)
	for _, prefix := range []string{"the ", "a "} {
		if strings.HasPrefix(lower, prefix) {
			return strings.ToLower(name[len(prefix):])
		}
	}
	return lower
}
