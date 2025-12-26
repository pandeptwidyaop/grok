package cli

import (
	"fmt"

	"github.com/pandeptwidyaop/grok/internal/client/config"
	"github.com/spf13/cobra"
)

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage grok configuration",
	Long:  `Manage grok client configuration including authentication token.`,
}

// setTokenCmd represents the set-token command
var setTokenCmd = &cobra.Command{
	Use:   "set-token [token]",
	Short: "Set authentication token",
	Long: `Set the authentication token for connecting to the grok server.

Example:
  grok config set-token grok_abc123def456...`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		token := args[0]

		if err := config.SaveToken(token); err != nil {
			return fmt.Errorf("failed to save token: %w", err)
		}

		fmt.Printf("✓ Authentication token saved to config file\n")
		fmt.Printf("You can now use 'grok http' and 'grok tcp' commands\n")

		return nil
	},
}

// setServerCmd represents the set-server command
var setServerCmd = &cobra.Command{
	Use:   "set-server [address]",
	Short: "Set grok server address",
	Long: `Set the grok server address for tunnel connections.

The address can be specified as:
  - hostname only (uses default port 4443): cloudtunnel.id
  - hostname with port: cloudtunnel.id:8080
  - IP address with port: 192.168.1.100:4443

Examples:
  grok config set-server cloudtunnel.id
  grok config set-server cloudtunnel.id:8080
  grok config set-server localhost:4443`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		addr := args[0]

		// Add default port if not specified
		if !hasPort(addr) {
			addr = addr + ":4443"
		}

		if err := config.SaveServer(addr); err != nil {
			return fmt.Errorf("failed to save server address: %w", err)
		}

		fmt.Printf("✓ Server address saved: %s\n", addr)
		fmt.Printf("Config file: ~/.grok/config.yaml\n")

		return nil
	},
}

// hasPort checks if address already has a port
func hasPort(addr string) bool {
	// Simple check for colon in address
	// Works for: "example.com:8080", "192.168.1.1:4443"
	// Doesn't match: "example.com", "192.168.1.1"
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return true
		}
		if addr[i] == '/' {
			return false
		}
	}
	return false
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(setTokenCmd)
	configCmd.AddCommand(setServerCmd)
}
