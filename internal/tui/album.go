package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/toba/musup/internal/state"
)

type albumDetailModel struct {
	artist     string
	albumTitle string
	year       string
	tracks     []state.TrackRecord
	cursor     int
	height     int
	offset     int
}

func newAlbumDetailModel(db *state.DB, artist, albumTitle, year string) albumDetailModel {
	tracks, _ := db.Tracks(artist, albumTitle)
	return albumDetailModel{
		artist:     artist,
		albumTitle: albumTitle,
		year:       year,
		tracks:     tracks,
	}
}

func (m albumDetailModel) Update(msg tea.Msg) (albumDetailModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "backspace", "left":
			return m, func() tea.Msg { return backToDetailMsg{} }
		case "q", "ctrl+c":
			return m, tea.Quit
		case "j", "down":
			if m.cursor < len(m.tracks)-1 {
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

func (m *albumDetailModel) ensureVisible() {
	viewable := m.viewableLines()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+viewable {
		m.offset = m.cursor - viewable + 1
	}
}

func (m albumDetailModel) viewableLines() int {
	v := max(m.height-5, 1)
	return v
}

func (m albumDetailModel) View() string {
	var b strings.Builder

	// Header
	yearStr := ""
	if m.year != "" {
		yearStr = "  (" + m.year + ")"
	}
	noun := "tracks"
	if len(m.tracks) == 1 {
		noun = "track"
	}
	header := titleStyle.Render(m.albumTitle) + mutedStyle.Render(yearStr) +
		"  " + mutedStyle.Render(fmt.Sprintf("%d %s", len(m.tracks), noun))
	b.WriteString(header + "\n")
	b.WriteString(subtleStyle.Render(strings.Repeat("─", 40)) + "\n\n")

	if len(m.tracks) == 0 {
		b.WriteString(mutedStyle.Render("No tracks found."))
		b.WriteString("\n" + subtleStyle.Render("esc: back · q: quit"))
		return b.String()
	}

	// Compute max track name width
	maxNameWidth := 0
	viewable := m.viewableLines()
	end := min(m.offset+viewable, len(m.tracks))
	for i := m.offset; i < end; i++ {
		if len(m.tracks[i].Title) > maxNameWidth {
			maxNameWidth = len(m.tracks[i].Title)
		}
	}

	for i := m.offset; i < end; i++ {
		t := m.tracks[i]

		cursor := "  "
		style := lipgloss.NewStyle()
		if i == m.cursor {
			cursor = cursorStyle.Render("> ")
			style = style.Foreground(colorAccent)
		}

		num := fmt.Sprintf("%3d", t.Position)
		name := t.Title + strings.Repeat(" ", max(0, maxNameWidth-len(t.Title)))
		owned := " "
		if t.Local {
			owned = localStyle.Render("✓")
		}

		b.WriteString(cursor + mutedStyle.Render(num) + "  " + style.Render(name) + "  " + owned + "\n")
	}

	b.WriteString("\n" + subtleStyle.Render("esc: back · q: quit"))

	return b.String()
}

type backToDetailMsg struct{}
