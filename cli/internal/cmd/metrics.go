package cmd

import (
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"github.com/trekmax/cloudguard-cli/internal/client"
	"github.com/trekmax/cloudguard-cli/internal/output"
)

func metricsCmd() *cobra.Command {
	var (
		category string
		name     string
		rangeStr string
		limit    int
	)

	cmd := &cobra.Command{
		Use:   "metrics",
		Short: "Query historical metrics",
		Long:  "Query historical metrics with optional filters.\nExample: cloudguard metrics --category cpu --range 24h --limit 20",
		RunE: func(cmd *cobra.Command, args []string) error {
			params := make(map[string]string)

			if category != "" {
				params["category"] = category
			}
			if name != "" {
				params["name"] = name
			}
			if limit > 0 {
				params["limit"] = strconv.Itoa(limit)
			}

			if rangeStr != "" {
				duration, err := parseDuration(rangeStr)
				if err != nil {
					return fmt.Errorf("invalid range: %w", err)
				}
				start := time.Now().Add(-duration).Unix()
				params["start"] = strconv.FormatInt(start, 10)
			}

			records, err := apiClient.GetMetrics(params)
			if err != nil {
				return fmt.Errorf("get metrics: %w", err)
			}

			if len(records) == 0 {
				fmt.Println("No metrics found.")
				return nil
			}

			output.Print(getFormat(), records, func() {
				printMetricsTable(records)
			})
			return nil
		},
	}

	cmd.Flags().StringVarP(&category, "category", "c", "", "filter by category (cpu, memory, disk, network)")
	cmd.Flags().StringVarP(&name, "name", "n", "", "filter by metric name")
	cmd.Flags().StringVarP(&rangeStr, "range", "r", "", "time range (e.g., 1h, 24h, 7d)")
	cmd.Flags().IntVarP(&limit, "limit", "l", 50, "max number of records")

	return cmd
}

func printMetricsTable(records []client.MetricRecord) {
	headers := []string{"Time", "Category", "Name", "Value"}
	var rows [][]string

	for _, r := range records {
		timeStr := time.Unix(r.Timestamp, 0).Format("01-02 15:04:05")
		rows = append(rows, []string{
			timeStr,
			r.Category,
			r.Name,
			fmt.Sprintf("%.2f", r.Value),
		})
	}

	output.PrintTable(headers, rows)
}

func parseDuration(s string) (time.Duration, error) {
	if len(s) == 0 {
		return 0, fmt.Errorf("empty duration")
	}

	// Handle "Nd" format (days) — not supported by time.ParseDuration
	last := s[len(s)-1]
	if last == 'd' {
		numStr := s[:len(s)-1]
		n, err := strconv.Atoi(numStr)
		if err != nil {
			return 0, fmt.Errorf("invalid day duration: %w", err)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}

	// Delegate everything else to standard library
	return time.ParseDuration(s)
}
