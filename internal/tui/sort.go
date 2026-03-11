package tui

import (
	"cmp"
	"slices"
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
	modal := modalStyle(40)

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
	slices.SortStableFunc(items, func(a, b artistItem) int {
		switch mode {
		case sortByNameArticle:
			return cmp.Compare(strings.ToLower(a.name), strings.ToLower(b.name))
		case sortByNewest:
			return cmp.Compare(a.newestAlbum, b.newestAlbum)
		case sortByCount:
			return cmp.Compare(b.albumCount, a.albumCount) // descending
		default:
			return cmp.Compare(stripArticle(a.name), stripArticle(b.name))
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
