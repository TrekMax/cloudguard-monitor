package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/trekmax/cloudguard-cli/internal/tui"
)

func dashboardCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "dashboard",
		Short: "Real-time TUI dashboard",
		Long:  "Launch an interactive terminal dashboard with real-time server metrics.\nPress 'q' or Ctrl+C to exit.",
		RunE: func(cmd *cobra.Command, args []string) error {
			interval := cfg.Interval
			if interval <= 0 {
				interval = 5
			}
			if err := tui.Run(apiClient, interval); err != nil {
				return fmt.Errorf("dashboard: %w", err)
			}
			return nil
		},
	}
}
