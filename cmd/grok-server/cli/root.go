package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	configPath string
	version    = "dev"
	buildTime  = "unknown"
	gitCommit  = "unknown"
)

// SetVersion sets the version information
func SetVersion(v, b, g string) {
	version = v
	buildTime = b
	gitCommit = g
}

// rootCmd represents the base command
var rootCmd = &cobra.Command{
	Use:   "grok-server",
	Short: "Grok tunnel server",
	Long:  `Grok server provides tunnel services similar to ngrok.`,
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "configs/server.yaml", "path to config file")

	// Version command
	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Grok Server\n")
			fmt.Printf("  Version:    %s\n", version)
			fmt.Printf("  Build Time: %s\n", buildTime)
			fmt.Printf("  Git Commit: %s\n", gitCommit)
			fmt.Printf("  Go Version: %s\n", "go1.23+")
		},
	})
}
