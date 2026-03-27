package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/trekmax/cloudguard-cli/internal/client"
	"github.com/trekmax/cloudguard-cli/internal/output"
)

func systemCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "system",
		Short: "Show server system information",
		RunE: func(cmd *cobra.Command, args []string) error {
			info, err := apiClient.GetSystem()
			if err != nil {
				return fmt.Errorf("get system info: %w", err)
			}

			output.Print(getFormat(), info, func() {
				printSystemTable(info)
			})
			return nil
		},
	}
}

func printSystemTable(info *client.SystemInfo) {
	uptime := formatUptime(info.Uptime)

	output.PrintTable(
		[]string{"Property", "Value"},
		[][]string{
			{"Hostname", info.Hostname},
			{"OS", info.OS},
			{"Platform", info.Platform},
			{"Architecture", info.Arch},
			{"Kernel", truncate(info.Kernel, 60)},
			{"CPU Cores", fmt.Sprintf("%d", info.CPUCores)},
			{"Uptime", uptime},
			{"Boot Time", time.Unix(info.BootTime, 0).Format("2006-01-02 15:04:05")},
			{"Agent Version", info.AgentVersion},
			{"Go Version", info.GoVersion},
		},
	)
}

func formatUptime(seconds int64) string {
	days := seconds / 86400
	hours := (seconds % 86400) / 3600
	mins := (seconds % 3600) / 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, mins)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, mins)
	}
	return fmt.Sprintf("%dm", mins)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
