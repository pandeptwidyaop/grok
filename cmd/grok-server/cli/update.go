package cli

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	versionpkg "github.com/pandeptwidyaop/grok/internal/version"
	"github.com/pandeptwidyaop/grok/pkg/logger"
	"github.com/pandeptwidyaop/grok/pkg/updater"
)

const (
	githubOwner = "pandeptwidyaop"
	githubRepo  = "grok"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update grok server to a specific version",
	Long: `Update grok server to a specific version with interactive selection.

This command will:
  1. Fetch recent releases from GitHub
  2. Show interactive selection menu
  3. Download the selected release for your platform
  4. Replace the current binary with the new version

For automated scripts, use the --latest flag to restore
the old behavior of auto-updating to the latest version.

Examples:
  # Interactive selection (default)
  grok-server update

  # Auto-update to latest (old behavior)
  grok-server update --latest

  # Update to specific version
  grok-server update --version v1.1.0

  # Include pre-releases in selection
  grok-server update --include-prerelease`,
	RunE: runUpdate,
}

var (
	updateForce             bool
	updateYes               bool
	updateLatest            bool   // Auto-update to latest (backward compatibility)
	updateIncludePrerelease bool   // Include pre-releases
	updateVersion           string // Specific version
)

func init() {
	updateCmd.Flags().BoolVarP(&updateForce, "force", "f", false,
		"Force update even if already up-to-date")
	updateCmd.Flags().BoolVarP(&updateYes, "yes", "y", false,
		"Skip confirmation prompt")
	updateCmd.Flags().BoolVar(&updateLatest, "latest", false,
		"Update to latest version without interactive selection")
	updateCmd.Flags().BoolVar(&updateIncludePrerelease, "include-prerelease", false,
		"Include pre-release versions in selection")
	updateCmd.Flags().StringVar(&updateVersion, "version", "",
		"Update to specific version (e.g., v1.1.0)")

	rootCmd.AddCommand(updateCmd)
}

var errUserCanceled = errors.New("user canceled")

func runUpdate(_ *cobra.Command, _ []string) error {
	logger.InfoEvent().Msg("Checking for updates...")

	u, err := updater.NewUpdater("grok-server")
	if err != nil {
		return fmt.Errorf("failed to create updater: %w", err)
	}

	// Case 1: Specific version requested
	if updateVersion != "" {
		return updateToVersion(u, updateVersion)
	}

	// Case 2: Latest flag (backward compatibility)
	if updateLatest {
		return updateToLatest(u)
	}

	// Case 3: Interactive selection (new default)
	return updateInteractive(u)
}

func updateToVersion(u *updater.Updater, targetVersion string) error {
	// Validate version format
	if !strings.HasPrefix(targetVersion, "v") {
		targetVersion = "v" + targetVersion
	}

	logger.InfoEvent().Str("version", targetVersion).Msg("Fetching specific release")

	// Fetch release info for specific version
	release, err := versionpkg.FetchReleaseByTag(githubOwner, githubRepo, targetVersion)
	if err != nil {
		return fmt.Errorf("failed to fetch release %s: %w", targetVersion, err)
	}

	// Validate assets
	if err := versionpkg.ValidateReleaseAssets(release, "grok-server"); err != nil {
		return fmt.Errorf("release %s missing binary for %s/%s: %w",
			targetVersion, runtime.GOOS, runtime.GOARCH, err)
	}

	currentVersion := versionpkg.Version
	fmt.Printf("Current version: %s\n", currentVersion)
	fmt.Printf("Target version:  %s\n", targetVersion)

	// Show release notes preview
	if release.Body != "" {
		fmt.Printf("\n📝 Release Notes:\n%s\n",
			versionpkg.TruncateReleaseNotes(release.Body, 10))
	}

	// Confirm (unless --yes)
	if !updateYes {
		if !confirmUpdate(targetVersion) {
			fmt.Println("Update canceled.")
			return nil
		}
	}

	// Perform update
	return performUpdate(u, release)
}

func updateToLatest(u *updater.Updater) error {
	// Original behavior - fetch latest and update
	updateInfo, err := u.CheckForUpdates()
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	fmt.Printf("Current version: %s\n", updateInfo.CurrentVersion)
	fmt.Printf("Latest version:  %s\n", updateInfo.LatestVersion)

	if !updateInfo.UpdateAvailable && !updateForce {
		fmt.Println("\n✓ You are already running the latest version!")
		return nil
	}

	if updateInfo.ReleaseNotes != "" {
		fmt.Printf("\n📝 Release Notes:\n%s\n", updateInfo.ReleaseNotes)
	}

	if !updateYes {
		if !confirmUpdate(updateInfo.LatestVersion) {
			fmt.Println("Update canceled.")
			return nil
		}
	}

	return u.Update(updateInfo)
}

func updateInteractive(u *updater.Updater) error {
	// Fetch releases
	opts := versionpkg.ReleaseListOptions{
		IncludePrerelease: updateIncludePrerelease,
		Limit:             10,
	}

	releases, err := versionpkg.FetchReleases(githubOwner, githubRepo, opts)
	if err != nil {
		return fmt.Errorf("failed to fetch releases: %w", err)
	}

	if len(releases) == 0 {
		return fmt.Errorf("no releases found")
	}

	// Display current version
	currentVersion := versionpkg.Version
	fmt.Printf("Current version: %s\n\n", currentVersion)

	// Check if terminal supports interactive mode
	if !isTerminalInteractive() {
		logger.WarnEvent().Msg("Non-interactive terminal detected, falling back to latest version")
		return updateToLatest(u)
	}

	// Show interactive selection
	selectedRelease, err := showReleaseSelector(releases, currentVersion)
	if err != nil {
		if errors.Is(err, errUserCanceled) {
			fmt.Println("Update canceled.")
			return nil
		}
		return fmt.Errorf("selection failed: %w", err)
	}

	// Validate platform binary exists
	if err := versionpkg.ValidateReleaseAssets(selectedRelease, "grok-server"); err != nil {
		return fmt.Errorf("selected version missing binary for %s/%s: %w",
			runtime.GOOS, runtime.GOARCH, err)
	}

	// Show release notes for selected version
	if selectedRelease.Body != "" {
		fmt.Printf("\n📝 Release Notes for %s:\n%s\n",
			selectedRelease.TagName,
			versionpkg.TruncateReleaseNotes(selectedRelease.Body, 10))
	}

	// Confirm (unless --yes)
	if !updateYes {
		if !confirmUpdate(selectedRelease.TagName) {
			fmt.Println("Update canceled.")
			return nil
		}
	}

	// Perform update
	return performUpdate(u, selectedRelease)
}

func showReleaseSelector(releases []versionpkg.Release, currentVersion string) (versionpkg.Release, error) {
	// Build list items
	items := make([]list.Item, len(releases))
	for i, r := range releases {
		items[i] = releaseItem{
			release: r,
			current: r.TagName == currentVersion,
		}
	}

	// Create list component
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(lipgloss.Color("#00FF00"))

	l := list.New(items, delegate, 80, 20)
	l.Title = "Select a version to install"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(false)

	// Run Bubble Tea program
	m := releaseModel{list: l, releases: releases}
	p := tea.NewProgram(m)

	finalModel, err := p.Run()
	if err != nil {
		return versionpkg.Release{}, fmt.Errorf("UI error: %w", err)
	}

	result, ok := finalModel.(releaseModel)
	if !ok {
		return versionpkg.Release{}, fmt.Errorf("unexpected model type")
	}

	if result.canceled {
		return versionpkg.Release{}, errUserCanceled
	}

	return result.selected, nil
}

// releaseItem implements list.Item interface.
type releaseItem struct {
	release versionpkg.Release
	current bool
}

func (r releaseItem) FilterValue() string { return r.release.TagName }

func (r releaseItem) Title() string {
	title := r.release.TagName
	if r.current {
		title += " (current)"
	}
	if r.release.Prerelease {
		title += " [pre-release]"
	}
	return title
}

func (r releaseItem) Description() string {
	age := time.Since(r.release.PublishedAt)
	return fmt.Sprintf("Published %s ago", formatDuration(age))
}

type releaseModel struct {
	list     list.Model
	releases []versionpkg.Release
	selected versionpkg.Release
	canceled bool
}

func (m releaseModel) Init() tea.Cmd { return nil }

func (m releaseModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			idx := m.list.Index()
			m.selected = m.releases[idx]
			return m, tea.Quit
		case "q", "ctrl+c", "esc":
			m.canceled = true
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		m.list.SetHeight(msg.Height - 4)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m releaseModel) View() string {
	return "\n" + m.list.View() + "\n\n" +
		"↑/↓: Navigate • Enter: Select • q/Esc: Cancel\n"
}

func isTerminalInteractive() bool {
	// Check if stdin is a terminal
	fileInfo, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}

func confirmUpdate(versionStr string) bool {
	fmt.Printf("\nDo you want to update to version %s? [y/N]: ", versionStr)
	var response string
	_, _ = fmt.Scanln(&response)
	return response == "y" || response == "Y" || response == "yes" || response == "Yes"
}

func performUpdate(u *updater.Updater, release versionpkg.Release) error {
	fmt.Println("\n🔄 Downloading and installing update...")

	// Create UpdateInfo from Release
	updateInfo := &versionpkg.UpdateInfo{
		CurrentVersion:  versionpkg.Version,
		LatestVersion:   release.TagName,
		UpdateAvailable: true,
		ReleaseURL:      release.HTMLURL,
		ReleaseNotes:    release.Body,
	}

	if err := u.Update(updateInfo); err != nil {
		return fmt.Errorf("update failed: %w", err)
	}

	fmt.Printf("\n✓ Successfully updated to version %s!\n", release.TagName)
	fmt.Println("\n⚠️  Please restart the grok server to use the new version.")
	fmt.Println("\nIf running as systemd service:")
	fmt.Println("  sudo systemctl restart grok-server")
	fmt.Println("\nYou can verify the update by running: grok-server --version")

	return nil
}

func formatDuration(d time.Duration) string {
	days := int(d.Hours() / 24)
	if days > 365 {
		years := days / 365
		return fmt.Sprintf("%d year%s", years, pluralize(years))
	}
	if days > 30 {
		months := days / 30
		return fmt.Sprintf("%d month%s", months, pluralize(months))
	}
	if days > 0 {
		return fmt.Sprintf("%d day%s", days, pluralize(days))
	}
	hours := int(d.Hours())
	if hours > 0 {
		return fmt.Sprintf("%d hour%s", hours, pluralize(hours))
	}
	return "< 1 hour"
}

func pluralize(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
