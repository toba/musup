package db

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

// openLargeDB opens the real-world database snapshot in testdata/large.db.
// The file is gitignored; benchmarks that need it are skipped if absent.
// To populate it:
//
//	cp "/Users/jason/Library/Mobile Documents/com~apple~CloudDocs/Music/.musup.db" internal/db/testdata/large.db
func openLargeDB(tb testing.TB) *DB {
	tb.Helper()
	src := filepath.Join("testdata", "large.db")

	if _, err := os.Stat(src); err != nil {
		tb.Skipf("testdata/large.db not available: %v", err)
	}

	// Copy to a temp dir so we never mutate the snapshot.
	tmp := filepath.Join(tb.TempDir(), "large.db")
	if err := copyFile(src, tmp); err != nil {
		tb.Fatalf("copy large.db: %v", err)
	}

	db, err := Open(tmp)
	if err != nil {
		tb.Fatalf("open large db: %v", err)
	}
	tb.Cleanup(func() { db.Close() })
	return db
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func BenchmarkNewerReleases(b *testing.B) {
	db := openLargeDB(b)

	// Warm up.
	if _, err := db.Q.NewerReleases(bg, "2020-01-01"); err != nil {
		b.Fatalf("NewerReleases: %v", err)
	}

	b.ResetTimer()
	for b.Loop() {
		if _, err := db.Q.NewerReleases(bg, "2020-01-01"); err != nil {
			b.Fatalf("NewerReleases: %v", err)
		}
	}
}

func BenchmarkAllFileMeta(b *testing.B) {
	db := openLargeDB(b)

	b.ResetTimer()
	for b.Loop() {
		if _, err := db.Q.AllFileMeta(bg); err != nil {
			b.Fatalf("AllFileMeta: %v", err)
		}
	}
}
