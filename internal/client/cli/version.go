package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Long:  "Display the version, build time, and git commit of the Grok client",
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Printf("Grok Client\n")
		fmt.Printf("  Version:    %s\n", version)
		fmt.Printf("  Build Time: %s\n", buildTime)
		fmt.Printf("  Git Commit: %s\n", gitCommit)
		fmt.Printf("  Go Version: %s\n", "go1.23+")
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
