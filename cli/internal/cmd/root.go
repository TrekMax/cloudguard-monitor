package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/trekmax/cloudguard-cli/internal/client"
	"github.com/trekmax/cloudguard-cli/internal/output"
)

var (
	cfg       *CLIConfig
	apiClient *client.Client
	formatFlag string
)

func rootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cloudguard",
		Short: "CloudGuard Monitor CLI — lightweight server monitoring tool",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			cfg = loadConfig()

			// Flag overrides
			if cmd.Flags().Changed("server") {
				s, _ := cmd.Flags().GetString("server")
				cfg.Server = s
			}
			if cmd.Flags().Changed("token") {
				t, _ := cmd.Flags().GetString("token")
				cfg.Token = t
			}
			if cmd.Flags().Changed("format") {
				cfg.Format = formatFlag
			}

			apiClient = client.New(cfg.Server, cfg.Token)
		},
	}

	cmd.PersistentFlags().StringP("server", "s", "", "server address (e.g. http://host:8080)")
	cmd.PersistentFlags().StringP("token", "t", "", "API token")
	cmd.PersistentFlags().StringVarP(&formatFlag, "format", "f", "table", "output format: table, json, yaml")

	cmd.AddCommand(connectCmd())
	cmd.AddCommand(statusCmd())
	cmd.AddCommand(metricsCmd())
	cmd.AddCommand(systemCmd())
	cmd.AddCommand(dashboardCmd())
	cmd.AddCommand(alertsCmd())

	return cmd
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func getFormat() output.Format {
	return output.ParseFormat(cfg.Format)
}
