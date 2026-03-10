package musicbrainz

// ArtistSearchResult is the response from /artist?query=...
type ArtistSearchResult struct {
	Count   int      `json:"count"`
	Offset  int      `json:"offset"`
	Artists []Artist `json:"artists"`
}

// Artist is a MusicBrainz artist entity.
type Artist struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	SortName       string   `json:"sort-name"`
	Type           string   `json:"type"`
	Disambiguation string   `json:"disambiguation"`
	Country        string   `json:"country"`
	Score          int      `json:"score"`
	LifeSpan       LifeSpan `json:"life-span"`
	Aliases        []Alias  `json:"aliases"`
}

// LifeSpan represents the active period of an artist.
type LifeSpan struct {
	Begin string `json:"begin"`
	End   string `json:"end"`
	Ended bool   `json:"ended"`
}

// Alias is an alternative name for an artist.
type Alias struct {
	Name     string `json:"name"`
	SortName string `json:"sort-name"`
	Type     string `json:"type"`
	Locale   string `json:"locale"`
}

// ReleaseGroupBrowseResult is the response from /release-group?artist=...
type ReleaseGroupBrowseResult struct {
	Count         int            `json:"release-group-count"`
	Offset        int            `json:"release-group-offset"`
	ReleaseGroups []ReleaseGroup `json:"release-groups"`
}

// ReleaseGroup is a logical grouping of releases (album, single, EP, etc.).
type ReleaseGroup struct {
	ID               string         `json:"id"`
	Title            string         `json:"title"`
	PrimaryType      string         `json:"primary-type"`
	FirstReleaseDate string         `json:"first-release-date"`
	Disambiguation   string         `json:"disambiguation"`
	ArtistCredit     []ArtistCredit `json:"artist-credit"`
}

// ArtistCredit links an artist to a release or release group.
type ArtistCredit struct {
	Name       string `json:"name"`
	Artist     Artist `json:"artist"`
	JoinPhrase string `json:"joinphrase"`
}
