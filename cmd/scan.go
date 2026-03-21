package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/toba/musup/internal/scan"
)

var scanCmd = &cobra.Command{
	Use:   "scan [path]",
	Short: "Scan music files into the database",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root := "."
		if len(args) > 0 {
			root = args[0]
		}

		d, err := openDB()
		if err != nil {
			return err
		}
		defer func() { _ = d.Close() }()

		fmt.Fprintf(os.Stderr, "Scanning %s...\n", root)
		start := time.Now()

		if err := scan.Scan(cmd.Context(), d, root); err != nil {
			return fmt.Errorf("scan: %w", err)
		}

		fmt.Fprintf(os.Stderr, "Done in %s.\n", time.Since(start).Round(time.Millisecond))
		return nil
	},
}
