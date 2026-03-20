package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/lipgloss/v2"
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

func helpView(width, height int, bg string, fromView viewState) string {
	type shortcut struct {
		key  string
		desc string
	}

	var shortcuts []shortcut
	switch fromView {
	case viewList:
		shortcuts = []shortcut{
			{"/", "Filter artists"},
			{"n", "Toggle new releases filter"},
			{"r", "Mark as reviewed"},
			{"f", "Follow / unfollow"},
			{"o", "Sort order"},
			{"p", "Prune unfollowed artists"},
			{"u", "Sync artist catalog"},
			{"U", "Sync all followed artists"},
			{"enter", "View artist detail"},
			{"j/↓", "Move down"},
			{"k/↑", "Move up"},
			{"q", "Quit"},
		}
	case viewDetail:
		shortcuts = []shortcut{
			{"enter", "View album tracks"},
			{"r", "Mark album reviewed"},
			{"f", "Follow / unfollow artist"},
			{"u", "Update artist catalog"},
			{"j/↓", "Move down"},
			{"k/↑", "Move up"},
			{"esc", "Back to list"},
			{"q", "Quit"},
		}
	case viewAlbumDetail:
		shortcuts = []shortcut{
			{"j/↓", "Move down"},
			{"k/↑", "Move up"},
			{"esc", "Back to artist"},
			{"q", "Quit"},
		}
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("Keyboard Shortcuts") + "\n\n")
	for _, s := range shortcuts {
		fmt.Fprintf(&b, "  %s  %s\n", mutedStyle.Render(fmt.Sprintf("%-7s", s.key)), s.desc)
	}
	b.WriteString("\n" + subtleStyle.Render("Press any key to dismiss"))

	modal := modalStyle(34).Render(b.String())
	return placeOverlay(width, height, modal, bg)
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
			followed:    s.Followed,
			hasNew:      s.HasNew,
		}
	}
	return items
}
