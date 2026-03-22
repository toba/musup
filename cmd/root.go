package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"
	"github.com/toba/musup/internal/check"
	"github.com/toba/musup/internal/db"
	"github.com/toba/musup/internal/integration/musicbrainz"
	"github.com/toba/musup/internal/tui"
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
		if len(args) == 0 {
			return runTUI()
		}
		return runCheck(cmd, args)
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

func runTUI() error {
	d, dp, err := openDB()
	if err != nil {
		return err
	}
	defer func() { _ = d.Close() }()

	p := tea.NewProgram(tui.New(d, filepath.Dir(dp)))
	_, err = p.Run()
	return err
}

func runCheck(cmd *cobra.Command, args []string) error {
	n, err := strconv.Atoi(args[0])
	if err != nil || n < 1 {
		return fmt.Errorf("years must be a positive integer, got %q", args[0])
	}

	d, _, err := openDB()
	if err != nil {
		return err
	}
	defer func() { _ = d.Close() }()

	mb := musicbrainz.New("musup", ver, "https://github.com/toba/musup")

	fmt.Fprintf(os.Stderr, "Checking MusicBrainz for new releases...\n")
	err = check.SyncAll(cmd.Context(), d, mb, 7*24*time.Hour, func(p check.Progress) {
		fmt.Fprintf(os.Stderr, "\r  [%d/%d] %s", p.Current, p.Total, p.Artist)
	})
	if err != nil {
		return fmt.Errorf("sync: %w", err)
	}
	fmt.Fprintf(os.Stderr, "\r%s\n", "                                                  ")

	cutoff := time.Now().AddDate(-n, 0, 0).Format("2006-01-02")
	releases, err := d.Q.NewerReleases(cmd.Context(), cutoff)
	if err != nil {
		return fmt.Errorf("query releases: %w", err)
	}

	if len(releases) == 0 {
		fmt.Println("No new releases found.")
		return nil
	}

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
}

func openDB() (*db.DB, string, error) {
	dp := dbPath
	if dp == "" {
		root, err := os.Getwd()
		if err != nil {
			return nil, "", fmt.Errorf("get working directory: %w", err)
		}
		dp = filepath.Join(root, ".musup.db")
	}
	d, err := db.Open(dp)
	if err != nil {
		return nil, "", fmt.Errorf("open db: %w", err)
	}
	return d, dp, nil
}
