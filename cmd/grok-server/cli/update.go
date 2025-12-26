package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/pandeptwidyaop/grok/pkg/logger"
	"github.com/pandeptwidyaop/grok/pkg/updater"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update grok server to the latest version",
	Long: `Check for updates and download the latest version of grok server.
This command will:
  1. Check if a new version is available on GitHub
  2. Download the latest release for your platform
  3. Replace the current binary with the new version

Note: The server must be stopped before running this command.
After update, restart the server to use the new version.`,
	RunE: runUpdate,
}

var (
	updateForce bool
	updateYes   bool
)

func init() {
	updateCmd.Flags().BoolVarP(&updateForce, "force", "f", false, "Force update even if already up-to-date")
	updateCmd.Flags().BoolVarP(&updateYes, "yes", "y", false, "Skip confirmation prompt")
	rootCmd.AddCommand(updateCmd)
}

func runUpdate(_ *cobra.Command, _ []string) error {
	logger.InfoEvent().Msg("Checking for updates...")

	// Create updater
	u, err := updater.NewUpdater("grok-server")
	if err != nil {
		return fmt.Errorf("failed to create updater: %w", err)
	}

	// Check for updates
	updateInfo, err := u.CheckForUpdates()
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	// Display current and latest version
	fmt.Printf("Current version: %s\n", updateInfo.CurrentVersion)
	fmt.Printf("Latest version:  %s\n", updateInfo.LatestVersion)

	if !updateInfo.UpdateAvailable && !updateForce {
		fmt.Println("\n✓ You are already running the latest version!")
		return nil
	}

	if !updateInfo.UpdateAvailable && updateForce {
		fmt.Println("\nNo update available, but forcing re-download...")
	}

	// Show release notes if available
	if updateInfo.ReleaseNotes != "" {
		fmt.Printf("\n📝 Release Notes:\n%s\n", updateInfo.ReleaseNotes)
	}

	// Prompt for confirmation
	if !updateYes {
		fmt.Printf("\nDo you want to update to version %s? [y/N]: ", updateInfo.LatestVersion)
		var response string
		_, _ = fmt.Scanln(&response) // Ignore error for CLI input

		if response != "y" && response != "Y" && response != "yes" && response != "Yes" {
			fmt.Println("Update canceled.")
			return nil
		}
	}

	// Perform update
	fmt.Println("\n🔄 Downloading and installing update...")

	if err := u.Update(updateInfo); err != nil {
		return fmt.Errorf("update failed: %w", err)
	}

	fmt.Printf("\n✓ Successfully updated to version %s!\n", updateInfo.LatestVersion)
	fmt.Println("\n⚠️  Please restart the grok server to use the new version.")
	fmt.Println("\nIf running as systemd service:")
	fmt.Println("  sudo systemctl restart grok-server")
	fmt.Println("\nYou can verify the update by running: grok-server --version")

	return nil
}
