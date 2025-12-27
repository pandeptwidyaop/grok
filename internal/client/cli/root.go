package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/pandeptwidyaop/grok/internal/client/config"
	"github.com/pandeptwidyaop/grok/pkg/logger"
)

var (
	cfgFile string
	cfg     *config.Config

	// Version info (set by main package).
	version   = "dev"
	buildTime = "unknown"
	gitCommit = "unknown"
)

// rootCmd represents the base command.
var rootCmd = &cobra.Command{
	Use:   "grok",
	Short: "Grok - Self-hosted tunneling solution",
	Long: `Grok is a self-hosted ngrok alternative that creates secure tunnels 
to localhost using gRPC for efficient data transfer.

Example usage:
  grok http 3000                    # Create HTTP tunnel to localhost:3000
  grok http 8080 --subdomain demo   # Create tunnel with custom subdomain
  grok tcp 22                       # Create TCP tunnel to localhost:22
  grok config set-token <token>     # Configure auth token`,
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		// Skip config loading for config and version commands
		// Check both the command name and its parent
		cmdName := cmd.Name()
		parentName := ""
		if cmd.Parent() != nil {
			parentName = cmd.Parent().Name()
		}

		if cmdName == "config" || cmdName == "version" || parentName == "config" {
			return nil
		}

		// Load configuration
		var err error
		cfg, err = config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Setup logger
		if err := logger.Setup(logger.Config{
			Level:  cfg.Logging.Level,
			Format: cfg.Logging.Format,
			Output: "stdout",
		}); err != nil {
			return fmt.Errorf("failed to setup logger: %w", err)
		}

		// Validate auth token (except for config command)
		if cfg.Auth.Token == "" {
			return fmt.Errorf("authentication token not configured. Use 'grok config set-token <token>' to configure")
		}

		return nil
	},
}

// Execute executes the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.grok/config.yaml)")
	rootCmd.PersistentFlags().String("server", "", "server address (overrides config)")
	rootCmd.PersistentFlags().String("token", "", "auth token (overrides config)")

	// Dashboard flags
	rootCmd.PersistentFlags().Bool("dashboard", true, "enable dashboard (default: true)")
	rootCmd.PersistentFlags().Int("dashboard-port", 4041, "dashboard port (default: 4041)")
	rootCmd.PersistentFlags().Bool("no-dashboard", false, "disable dashboard (shortcut for --dashboard=false)")
}

// GetConfig returns the loaded configuration.
func GetConfig() *config.Config {
	return cfg
}

// SetVersion sets the version information.
func SetVersion(v, bt, gc string) {
	version = v
	buildTime = bt
	gitCommit = gc
}
