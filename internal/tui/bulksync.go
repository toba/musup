package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/toba/musup/internal/integration/musicbrainz"
	"github.com/toba/musup/internal/state"
)

type startBulkSyncMsg struct {
	artists []string
}

type bulkResult struct {
	artist string
	ok     bool
	errMsg string
}

type bulkSyncModel struct {
	spinner   spinner.Model
	db        *state.DB
	mb        *musicbrainz.Client
	artists   []string
	index     int
	ch        <-chan tea.Msg
	cancel    context.CancelFunc
	phase     string
	album     string
	current   int
	total     int
	results   []bulkResult
	done      bool
	cancelled bool
}

func newBulkSyncModel(db *state.DB, mb *musicbrainz.Client, artists []string) bulkSyncModel {
	m := bulkSyncModel{
		spinner: newSpinner(),
		db:      db,
		mb:      mb,
		artists: artists,
	}
	m.startNextArtist()
	return m
}

func (m *bulkSyncModel) startNextArtist() {
	ctx, cancel := context.WithCancel(context.Background()) //nolint:gosec // cancel stored in m.cancel, called on ESC
	m.cancel = cancel
	ch := make(chan tea.Msg, 1)
	m.ch = ch
	m.phase = "Connecting to MusicBrainz..."
	m.album = ""
	m.current = 0
	m.total = 0
	go runSync(ctx, ch, m.mb, m.db, m.artists[m.index])
}

func (m bulkSyncModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, listenForSync(m.ch))
}

func (m bulkSyncModel) Update(msg tea.Msg) (bulkSyncModel, tea.Cmd) {
	switch msg := msg.(type) {
	case syncProgressMsg:
		m.phase = msg.phase
		m.album = msg.album
		m.current = msg.current
		m.total = msg.total
		return m, listenForSync(m.ch)

	case syncDoneMsg:
		if m.cancelled {
			m.done = true
			return m, nil
		}

		result := bulkResult{artist: m.artists[m.index]}
		if msg.err != nil {
			result.ok = false
			result.errMsg = msg.err.Error()
		} else {
			result.ok = true
		}
		m.results = append(m.results, result)

		m.index++
		if m.index >= len(m.artists) {
			m.done = true
			return m, nil
		}

		m.startNextArtist()
		return m, listenForSync(m.ch)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m bulkSyncModel) View(width, height int, bg string) string {
	modal := modalStyle(50)

	var b strings.Builder

	b.WriteString(titleStyle.Render(fmt.Sprintf("Bulk Sync (%d/%d)", m.index+1, len(m.artists))) + "\n\n")

	// Show last few completed results (cap at 5)
	start := 0
	if len(m.results) > 5 {
		start = len(m.results) - 5
	}
	for _, r := range m.results[start:] {
		if r.ok {
			b.WriteString(localStyle.Render("  ✓ ") + r.artist + "\n")
		} else {
			b.WriteString(errorStyle.Render("  ✗ ") + r.artist + mutedStyle.Render(" — "+r.errMsg) + "\n")
		}
	}

	// Current artist
	if !m.done && m.index < len(m.artists) {
		counter := ""
		if m.total > 0 {
			counter = mutedStyle.Render(fmt.Sprintf(" (%d/%d)", m.current, m.total))
		}
		b.WriteString("  " + m.spinner.View() + " " + m.artists[m.index] + mutedStyle.Render(" — "+m.phase) + counter + "\n")
		if m.album != "" {
			b.WriteString("    " + subtleStyle.Render(m.album) + "\n")
		}
	}

	// Remaining count
	remaining := len(m.artists) - m.index - 1
	if !m.done && remaining > 0 {
		b.WriteString(mutedStyle.Render(fmt.Sprintf("\n  %d remaining", remaining)) + "\n")
	}

	b.WriteString("\n" + subtleStyle.Render("  esc: cancel"))

	rendered := modal.Render(b.String())
	return placeOverlay(width, height, rendered, bg)
}
