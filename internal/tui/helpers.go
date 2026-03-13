package tui

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/toba/musup/internal/state"
)

const (
	mbMinMatchScore            = 90
	mbSearchLimit              = 5
	mbMaxReleaseGroups         = 500
	mbMaxReleaseGroupsComposer = 100
	spinnerFPS                 = 80 * time.Millisecond
	headerSepWidth             = 40
)

var headerSep = subtleStyle.Render(strings.Repeat("─", headerSepWidth))

func newSpinner() spinner.Model {
	s := spinner.New()
	s.Spinner = spinner.Spinner{
		Frames: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		FPS:    spinnerFPS,
	}
	s.Style = lipgloss.NewStyle().Foreground(colorAccent)
	return s
}

func modalStyle(width int) lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorAccent).
		Padding(1, 2).
		Width(width)
}

func viewableLines(height int) int {
	return max(height-5, 1)
}

func ensureVisible(cursor, offset, height int) int {
	viewable := viewableLines(height)
	if cursor < offset {
		offset = cursor
	}
	if cursor >= offset+viewable {
		offset = cursor - viewable + 1
	}
	return offset
}

func summariesToItems(summaries []state.ArtistSummary) []artistItem {
	items := make([]artistItem, len(summaries))
	for i, s := range summaries {
		items[i] = artistItem{
			name:        s.Name,
			albumCount:  s.AlbumCount,
			newestAlbum: s.NewestAlbum,
			trackCount:  s.TrackCount,
			totalAlbums: s.TotalAlbums,
			totalTracks: s.TotalTracks,
			synced:      s.Synced,
			monitor:     s.Monitor,
		}
	}
	return items
}
