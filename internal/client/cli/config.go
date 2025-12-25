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

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(setTokenCmd)
}
