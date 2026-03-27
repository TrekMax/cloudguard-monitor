package cmd

import (
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"github.com/trekmax/cloudguard-cli/internal/client"
	"github.com/trekmax/cloudguard-cli/internal/output"
)

func alertsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "alerts",
		Short: "Manage alerts and alert rules",
	}

	cmd.AddCommand(alertsListCmd())
	cmd.AddCommand(alertsAckCmd())
	cmd.AddCommand(alertsRulesCmd())

	return cmd
}

func alertsListCmd() *cobra.Command {
	var status string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List alert events",
		RunE: func(cmd *cobra.Command, args []string) error {
			params := make(map[string]string)
			if status != "" {
				params["status"] = status
			}

			events, err := apiClient.GetAlerts(params)
			if err != nil {
				return fmt.Errorf("get alerts: %w", err)
			}

			if len(events) == 0 {
				fmt.Println("No alerts found.")
				return nil
			}

			output.Print(getFormat(), events, func() {
				printAlertsTable(events)
			})
			return nil
		},
	}

	cmd.Flags().StringVar(&status, "status", "", "filter by status: firing, resolved, acknowledged")
	return cmd
}

func alertsAckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ack <alert-id>",
		Short: "Acknowledge an alert",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid alert ID: %w", err)
			}
			if err := apiClient.AckAlert(id); err != nil {
				return fmt.Errorf("ack alert: %w", err)
			}
			fmt.Printf("Alert %d acknowledged.\n", id)
			return nil
		},
	}
}

func alertsRulesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rules",
		Short: "Manage alert rules",
	}

	cmd.AddCommand(alertsRulesListCmd())
	cmd.AddCommand(alertsRulesAddCmd())
	cmd.AddCommand(alertsRulesDeleteCmd())

	return cmd
}

func alertsRulesListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List alert rules",
		RunE: func(cmd *cobra.Command, args []string) error {
			rules, err := apiClient.GetAlertRules()
			if err != nil {
				return fmt.Errorf("get rules: %w", err)
			}

			if len(rules) == 0 {
				fmt.Println("No alert rules configured.")
				return nil
			}

			output.Print(getFormat(), rules, func() {
				printRulesTable(rules)
			})
			return nil
		},
	}
}

func alertsRulesAddCmd() *cobra.Command {
	var (
		name      string
		category  string
		metric    string
		operator  string
		threshold float64
		duration  int
	)

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Create a new alert rule",
		Long:  "Example: cloudguard alerts rules add --name 'CPU High' --category cpu --metric usage --operator gt --threshold 90 --duration 60",
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" || category == "" || metric == "" || operator == "" {
				return fmt.Errorf("name, category, metric, and operator are required")
			}

			rule := &client.AlertRule{
				Name:      name,
				Category:  category,
				Metric:    metric,
				Operator:  operator,
				Threshold: threshold,
				Duration:  duration,
				Enabled:   true,
			}

			if err := apiClient.CreateAlertRule(rule); err != nil {
				return fmt.Errorf("create rule: %w", err)
			}

			fmt.Printf("Alert rule '%s' created.\n", name)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "rule name")
	cmd.Flags().StringVar(&category, "category", "", "metric category (cpu, memory, disk, network)")
	cmd.Flags().StringVar(&metric, "metric", "", "metric name (e.g., usage, usage_percent)")
	cmd.Flags().StringVar(&operator, "operator", "gt", "operator: gt, lt, eq, gte, lte")
	cmd.Flags().Float64Var(&threshold, "threshold", 0, "threshold value")
	cmd.Flags().IntVar(&duration, "duration", 0, "duration in seconds before firing")

	return cmd
}

func alertsRulesDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <rule-id>",
		Short: "Delete an alert rule",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid rule ID: %w", err)
			}
			if err := apiClient.DeleteAlertRule(id); err != nil {
				return fmt.Errorf("delete rule: %w", err)
			}
			fmt.Printf("Alert rule %d deleted.\n", id)
			return nil
		},
	}
}

func printAlertsTable(events []client.AlertEvent) {
	headers := []string{"ID", "Rule", "Status", "Value", "Message", "Fired At"}
	var rows [][]string
	for _, e := range events {
		rows = append(rows, []string{
			strconv.FormatInt(e.ID, 10),
			e.RuleName,
			statusIcon(e.Status),
			fmt.Sprintf("%.2f", e.Value),
			truncate(e.Message, 50),
			time.Unix(e.FiredAt, 0).Format("01-02 15:04:05"),
		})
	}
	output.PrintTable(headers, rows)
}

func printRulesTable(rules []client.AlertRule) {
	headers := []string{"ID", "Name", "Category", "Metric", "Condition", "Duration", "Enabled"}
	var rows [][]string
	for _, r := range rules {
		enabled := "yes"
		if !r.Enabled {
			enabled = "no"
		}
		rows = append(rows, []string{
			strconv.FormatInt(r.ID, 10),
			r.Name,
			r.Category,
			r.Metric,
			fmt.Sprintf("%s %.1f", r.Operator, r.Threshold),
			fmt.Sprintf("%ds", r.Duration),
			enabled,
		})
	}
	output.PrintTable(headers, rows)
}

func statusIcon(status string) string {
	switch status {
	case "firing":
		return "FIRING"
	case "resolved":
		return "RESOLVED"
	case "acknowledged":
		return "ACKED"
	default:
		return status
	}
}
