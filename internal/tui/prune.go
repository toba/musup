package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/toba/musup/internal/state"
)

type pruneModel struct {
	db        *state.DB
	spinner   spinner.Model
	artists   []string // unfollowed artist names
	confirmed bool
	running   bool
	done      bool
	result    state.PruneResult
	err       error
}

type pruneDoneMsg struct {
	result state.PruneResult
	err    error
}

func newPruneModel(db *state.DB, artists []string) pruneModel {
	return pruneModel{
		db:      db,
		spinner: newSpinner(),
		artists: artists,
	}
}

func (m pruneModel) Update(msg tea.Msg) (pruneModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if m.done {
			return m, nil // handled by parent
		}
		if !m.running {
			switch msg.String() {
			case "y", "enter":
				m.confirmed = true
				m.running = true
				return m, tea.Batch(m.spinner.Tick, m.runPrune())
			case "n", "esc":
				return m, nil // handled by parent
			}
		}
	case pruneDoneMsg:
		m.done = true
		m.running = false
		m.result = msg.result
		m.err = msg.err
		return m, nil
	case spinner.TickMsg:
		if m.running {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m pruneModel) runPrune() tea.Cmd {
	return func() tea.Msg {
		result, err := m.db.PruneUnfollowed()
		if err != nil {
			return pruneDoneMsg{err: err}
		}
		err = m.db.Vacuum()
		return pruneDoneMsg{result: result, err: err}
	}
}

func (m pruneModel) View(width, height int, bg string) string {
	modal := modalStyle(50)

	var b strings.Builder
	b.WriteString(titleStyle.Render("Prune unfollowed artists") + "\n\n")

	if m.done {
		if m.err != nil {
			b.WriteString(errorStyle.Render("  Error: "+m.err.Error()) + "\n\n")
		} else {
			fmt.Fprintf(&b, "  Removed %d artists, %d albums, %d tracks\n",
				m.result.Artists, m.result.Albums, m.result.Tracks)
			b.WriteString("  Database vacuumed.\n\n")
		}
		b.WriteString(subtleStyle.Render("  Press any key to continue"))
	} else if m.running {
		b.WriteString("  " + m.spinner.View() + " Pruning and vacuuming...\n")
	} else {
		fmt.Fprintf(&b, "  This will remove album and track information\n  for %d unfollowed artists.\n\n", len(m.artists))
		b.WriteString(subtleStyle.Render("  y: confirm · n/esc: cancel"))
	}

	rendered := modal.Render(b.String())
	return placeOverlay(width, height, rendered, bg)
}
