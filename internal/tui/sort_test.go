package tui

import "testing"

func TestSortArtists_stripArticles(t *testing.T) {
	items := []artistItem{
		{name: "Radiohead"},
		{name: "A Perfect Circle"},
		{name: "Beck"},
		{name: "The Beatles"},
	}

	sortArtists(items, sortByName)

	want := []string{"The Beatles", "Beck", "A Perfect Circle", "Radiohead"}
	for i, w := range want {
		if items[i].name != w {
			t.Errorf("index %d: got %q, want %q", i, items[i].name, w)
		}
	}
}

func TestSortArtists_withArticle(t *testing.T) {
	items := []artistItem{
		{name: "Zebra"},
		{name: "The Cranberries"},
		{name: "A Fine Frenzy"},
		{name: "ABBA"},
	}

	sortArtists(items, sortByName)

	// "ABBA" → abba, "The Cranberries" → cranberries, "A Fine Frenzy" → fine frenzy
	want := []string{"ABBA", "The Cranberries", "A Fine Frenzy", "Zebra"}
	for i, w := range want {
		if items[i].name != w {
			t.Errorf("index %d: got %q, want %q", i, items[i].name, w)
		}
	}
}

func TestSortArtists_nameArticleMode(t *testing.T) {
	items := []artistItem{
		{name: "Zebra"},
		{name: "The Cranberries"},
		{name: "A Fine Frenzy"},
		{name: "ABBA"},
	}

	sortArtists(items, sortByNameArticle)

	// Literal alphabetical: "A Fine Frenzy", "ABBA", "The Cranberries", "Zebra"
	want := []string{"A Fine Frenzy", "ABBA", "The Cranberries", "Zebra"}
	for i, w := range want {
		if items[i].name != w {
			t.Errorf("index %d: got %q, want %q", i, items[i].name, w)
		}
	}
}
