package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/trekmax/cloudguard-cli/internal/client"
)

func connectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "connect <server-address>",
		Short: "Configure and test server connection",
		Long:  "Set the server address and token, then verify connectivity.\nExample: cloudguard connect http://1.2.3.4:8080 --token mytoken",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			addr := args[0]
			token, _ := cmd.Flags().GetString("token")

			// Test connection
			c := client.New(addr, token)
			fmt.Printf("Connecting to %s ...\n", addr)

			if err := c.Health(); err != nil {
				return fmt.Errorf("connection failed: %w", err)
			}

			// Save config
			cfg.Server = addr
			if token != "" {
				cfg.Token = token
			}
			if err := saveConfig(cfg); err != nil {
				return fmt.Errorf("save config: %w", err)
			}

			fmt.Println("Connected successfully!")
			fmt.Printf("Configuration saved to %s\n", configPath())
			return nil
		},
	}
}
