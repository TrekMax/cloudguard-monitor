package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/trekmax/cloudguard-cli/internal/output"
)

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show real-time server status",
		RunE: func(cmd *cobra.Command, args []string) error {
			raw, err := apiClient.GetStatus()
			if err != nil {
				return fmt.Errorf("get status: %w", err)
			}

			status := output.FromStatusData(raw.CPU, raw.Memory, raw.Network)

			output.Print(getFormat(), status, func() {
				printStatusTable(status)
			})
			return nil
		},
	}
}

func printStatusTable(s *output.StatusDisplay) {
	fmt.Println("=== CPU ===")
	output.PrintTable(
		[]string{"Metric", "Value"},
		[][]string{
			{"Usage", fmt.Sprintf("%.1f%%", s.CPU.Usage)},
			{"User", fmt.Sprintf("%.1f%%", s.CPU.User)},
			{"System", fmt.Sprintf("%.1f%%", s.CPU.System)},
			{"Idle", fmt.Sprintf("%.1f%%", s.CPU.Idle)},
			{"IOWait", fmt.Sprintf("%.1f%%", s.CPU.IOWait)},
			{"Load (1/5/15)", fmt.Sprintf("%.2f / %.2f / %.2f", s.CPU.Load1, s.CPU.Load5, s.CPU.Load15)},
			{"Cores", fmt.Sprintf("%.0f", s.CPU.Cores)},
		},
	)

	fmt.Println("\n=== Memory ===")
	output.PrintTable(
		[]string{"Metric", "Value"},
		[][]string{
			{"Usage", fmt.Sprintf("%.1f%%", s.Memory.UsagePercent)},
			{"Total", formatBytes(s.Memory.Total)},
			{"Used", formatBytes(s.Memory.Used)},
			{"Available", formatBytes(s.Memory.Available)},
			{"Swap", fmt.Sprintf("%.1f%% (%s / %s)", s.Memory.SwapPercent, formatBytes(s.Memory.SwapUsed), formatBytes(s.Memory.SwapTotal))},
		},
	)

	fmt.Println("\n=== Network ===")
	output.PrintTable(
		[]string{"Metric", "Value"},
		[][]string{
			{"RX Rate", formatBytesRate(s.Network.RxRate)},
			{"TX Rate", formatBytesRate(s.Network.TxRate)},
			{"Connections", fmt.Sprintf("%.0f", s.Network.Connections)},
		},
	)
}

func formatBytes(b float64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)
	switch {
	case b >= GB:
		return fmt.Sprintf("%.1f GB", b/GB)
	case b >= MB:
		return fmt.Sprintf("%.1f MB", b/MB)
	case b >= KB:
		return fmt.Sprintf("%.1f KB", b/KB)
	default:
		return fmt.Sprintf("%.0f B", b)
	}
}

func formatBytesRate(b float64) string {
	return formatBytes(b) + "/s"
}
