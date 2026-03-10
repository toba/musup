package scan

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/toba/musup/internal/state"
)

func TestScan_EmptyDir(t *testing.T) {
	root := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "test.db")

	db, err := state.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err := Scan(context.Background(), db, root); err != nil {
		t.Fatalf("Scan: %v", err)
	}

	artists, err := db.UniqueArtists()
	if err != nil {
		t.Fatalf("UniqueArtists: %v", err)
	}
	if len(artists) != 0 {
		t.Fatalf("expected 0 artists, got %d", len(artists))
	}
}

func TestScan_SkipsUnsupportedExtensions(t *testing.T) {
	root := t.TempDir()
	// Create a .txt file that should be skipped
	if err := os.WriteFile(filepath.Join(root, "readme.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := state.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err := Scan(context.Background(), db, root); err != nil {
		t.Fatalf("Scan: %v", err)
	}

	artists, err := db.UniqueArtists()
	if err != nil {
		t.Fatalf("UniqueArtists: %v", err)
	}
	if len(artists) != 0 {
		t.Fatalf("expected 0 artists, got %d", len(artists))
	}
}

func TestSupportedExts(t *testing.T) {
	expected := []string{".flac", ".mp3", ".m4a", ".mp4", ".aac"}
	for _, ext := range expected {
		if _, ok := supportedExts[ext]; !ok {
			t.Errorf("expected %s to be in supportedExts", ext)
		}
	}
}
