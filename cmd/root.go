package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"github.com/toba/musup/internal/check"
	"github.com/toba/musup/internal/db"
	"github.com/toba/musup/internal/integration/musicbrainz"
)

var (
	dbPath string
	ver    = "dev"
	commit = "none"
	date   = "unknown"
)

var rootCmd = &cobra.Command{
	Use:     "musup [years]",
	Short:   "Show artists with new album releases",
	Long:    "Show artists from your music library that have released new albums you don't have locally.",
	Version: fmt.Sprintf("%s (%s) built %s", ver, commit, date),
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		years := 1
		if len(args) > 0 {
			n, err := strconv.Atoi(args[0])
			if err != nil || n < 1 {
				return fmt.Errorf("years must be a positive integer, got %q", args[0])
			}
			years = n
		}

		d, err := openDB()
		if err != nil {
			return err
		}
		defer func() { _ = d.Close() }()

		mb := musicbrainz.New("musup", ver, "https://github.com/toba/musup")

		// Sync artists that haven't been checked in the last 7 days.
		fmt.Fprintf(os.Stderr, "Checking MusicBrainz for new releases...\n")
		err = check.SyncAll(cmd.Context(), d, mb, 7*24*time.Hour, func(p check.Progress) {
			fmt.Fprintf(os.Stderr, "\r  [%d/%d] %s", p.Current, p.Total, p.Artist)
		})
		if err != nil {
			return fmt.Errorf("sync: %w", err)
		}
		fmt.Fprintf(os.Stderr, "\r%s\n", "                                                  ")

		// Query for newer releases.
		cutoff := time.Now().AddDate(-years, 0, 0).Format("2006-01-02")
		releases, err := d.Q.NewerReleases(cmd.Context(), cutoff)
		if err != nil {
			return fmt.Errorf("query releases: %w", err)
		}

		if len(releases) == 0 {
			fmt.Println("No new releases found.")
			return nil
		}

		// Group by artist for display.
		var currentArtist string
		for _, r := range releases {
			if r.ArtistName != currentArtist {
				if currentArtist != "" {
					fmt.Println()
				}
				currentArtist = r.ArtistName
				fmt.Println(r.ArtistName)
			}
			suffix := ""
			if r.SecondaryTypes != "" {
				suffix = " [" + r.SecondaryTypes + "]"
			}
			fmt.Printf("  %s (%s)%s\n", r.AlbumTitle, r.ReleaseDate, suffix)
		}

		return nil
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringVar(&dbPath, "db", "", "path to state database (default: .musup.db in current dir)")
	rootCmd.AddCommand(scanCmd)
}

func openDB() (*db.DB, error) {
	dp := dbPath
	if dp == "" {
		root, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get working directory: %w", err)
		}
		dp = filepath.Join(root, ".musup.db")
	}
	d, err := db.Open(dp)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	return d, nil
}
