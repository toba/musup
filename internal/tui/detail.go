package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/toba/musup/internal/state"
)

type detailModel struct {
	db     *state.DB
	artist string
	albums []string
	cursor int
	height int
	offset int
	err    error
}

func newDetailModel(db *state.DB, artist string) detailModel {
	m := detailModel{db: db, artist: artist}
	albums, err := db.LocalAlbums(artist)
	m.albums = albums
	m.err = err
	return m
}

func (m detailModel) Update(msg tea.Msg) (detailModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "backspace", "left":
			return m, func() tea.Msg { return backToListMsg{} }
		case "q", "ctrl+c":
			return m, tea.Quit
		case "j", "down":
			if m.cursor < len(m.albums)-1 {
				m.cursor++
				m.ensureVisible()
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
				m.ensureVisible()
			}
		}
	}
	return m, nil
}

func (m *detailModel) ensureVisible() {
	viewable := m.viewableLines()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+viewable {
		m.offset = m.cursor - viewable + 1
	}
}

func (m detailModel) viewableLines() int {
	// header takes 4 lines, leave 1 for footer
	v := m.height - 5
	if v < 1 {
		v = 1
	}
	return v
}

func (m detailModel) View() string {
	var b strings.Builder

	header := titleStyle.Render(m.artist)
	noun := "albums"
	if len(m.albums) == 1 {
		noun = "album"
	}
	albumCount := mutedStyle.Render(fmt.Sprintf("%d %s", len(m.albums), noun))
	b.WriteString(header + "  " + albumCount + "\n")
	b.WriteString(subtleStyle.Render(strings.Repeat("─", 40)) + "\n\n")

	if m.err != nil {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5555")).Render(m.err.Error()))
		return b.String()
	}

	if len(m.albums) == 0 {
		b.WriteString(mutedStyle.Render("No albums found."))
		return b.String()
	}

	viewable := m.viewableLines()
	end := m.offset + viewable
	if end > len(m.albums) {
		end = len(m.albums)
	}

	for i := m.offset; i < end; i++ {
		cursor := "  "
		style := lipgloss.NewStyle()
		if i == m.cursor {
			cursor = cursorStyle.Render("> ")
			style = style.Foreground(colorAccent)
		}
		b.WriteString(cursor + style.Render(m.albums[i]) + "\n")
	}

	b.WriteString("\n" + subtleStyle.Render("esc: back · q: quit"))

	return b.String()
}

type backToListMsg struct{}
