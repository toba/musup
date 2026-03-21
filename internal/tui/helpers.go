package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/toba/musup/internal/db"
)

const (
	mbMinMatchScore            = 90
	mbSearchLimit              = 5
	mbMaxReleaseGroups         = 500
	mbMaxReleaseGroupsComposer = 100
	headerSepWidth             = 40
)

var headerSep = subtleStyle.Render(strings.Repeat("─", headerSepWidth))

func newSpinner() spinner.Model {
	s := spinner.New()
	s.Spinner = spinner.MiniDot
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

func listenForSync(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return msg
	}
}

func pluralize(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}

// cursorPrefix returns the cursor string and text style for a list row.
func cursorPrefix(selected bool) (string, lipgloss.Style) {
	if selected {
		return cursorStyle.Render("> "), accentStyle
	}
	return "  ", lipgloss.NewStyle()
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

	modal := modalStyle(44).Render(b.String())
	return placeOverlay(width, height, modal, bg)
}

func summariesToItems(summaries []db.ArtistSummariesRow) []artistItem {
	items := make([]artistItem, len(summaries))
	for i, s := range summaries {
		synced := s.Mbid != ""
		trackCount := int(s.TrackCnt)
		if synced && int(s.LocalTracks) > trackCount {
			trackCount = int(s.LocalTracks)
		}
		items[i] = artistItem{
			name:        s.Name,
			albumCount:  int(s.AlbumCnt),
			newestAlbum: s.Newest,
			trackCount:  trackCount,
			totalAlbums: int(s.TotalAlbums),
			totalTracks: int(s.TotalTracks),
			synced:      synced,
			followed:    s.Followed != 0,
			hasNew:      s.HasNew != 0,
		}
	}
	return items
}
