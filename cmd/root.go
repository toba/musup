package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/toba/musup/internal/state"
	"github.com/toba/musup/internal/tui"
)

var (
	dbPath string
	ver    = "dev"
	commit = "none"
	date   = "unknown"
)

var rootCmd = &cobra.Command{
	Use:     "musup",
	Short:   "Interactive artist browser for your music library",
	Long:    "Scan a folder of music files and browse artists in an interactive TUI.",
	Version: fmt.Sprintf("%s (%s) built %s", ver, commit, date),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}

		dp := dbPath
		if dp == "" {
			dp = filepath.Join(root, ".musup.db")
		}

		db, err := state.Open(dp)
		if err != nil {
			return fmt.Errorf("open db: %w", err)
		}
		defer func() { _ = db.Close() }()

		p := tea.NewProgram(tui.New(db, root, ver), tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			return err
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
}
