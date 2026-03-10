package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/toba/musup/internal/state"
)

type statusModel struct {
	cursor  int
	current state.MonitorStatus
	artist  string
	db      *state.DB
}

func newStatusModel(db *state.DB, artist string, current state.MonitorStatus) statusModel {
	cursor := 0
	for i, s := range state.MonitorStatuses {
		if s == current {
			cursor = i
			break
		}
	}
	return statusModel{cursor: cursor, current: current, artist: artist, db: db}
}

func (m statusModel) Update(msg tea.Msg) (statusModel, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "j", "down":
			m.cursor = (m.cursor + 1) % len(state.MonitorStatuses)
		case "k", "up":
			m.cursor = (m.cursor - 1 + len(state.MonitorStatuses)) % len(state.MonitorStatuses)
		case "enter":
			chosen := state.MonitorStatuses[m.cursor]
			_ = m.db.SetMonitorStatus(m.artist, chosen)
			return m, func() tea.Msg { return statusChosenMsg{artist: m.artist, status: chosen} }
		case "esc":
			return m, func() tea.Msg { return statusCancelMsg{} }
		}
	}
	return m, nil
}

func (m statusModel) View(width, height int, bg string) string {
	modal := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorAccent).
		Padding(1, 2).
		Width(40)

	var s string
	s += titleStyle.Render("Monitor status") + "\n"
	s += mutedStyle.Render(m.artist) + "\n\n"
	var sb strings.Builder
	for i, status := range state.MonitorStatuses {
		cursor := "  "
		style := mutedStyle
		if i == m.cursor {
			cursor = cursorStyle.Render("> ")
			style = lipgloss.NewStyle().Foreground(colorAccent)
		}
		sb.WriteString(cursor + style.Render(state.MonitorLabels[status]) + "\n")
	}
	s += sb.String()

	rendered := modal.Render(s)
	return placeOverlay(width, height, rendered, bg)
}

type statusChosenMsg struct {
	artist string
	status state.MonitorStatus
}
type statusCancelMsg struct{}
