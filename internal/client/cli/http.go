package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/pandeptwidyaop/grok/internal/client"
	"github.com/pandeptwidyaop/grok/internal/client/dashboard"
	"github.com/pandeptwidyaop/grok/internal/client/tunnel"
	"github.com/pandeptwidyaop/grok/pkg/logger"
)

var (
	httpSubdomain string
	httpSavedName string
)

// httpCmd represents the http command.
var httpCmd = &cobra.Command{
	Use:   "http [port]",
	Short: "Start HTTP tunnel",
	Long: `Create an HTTP tunnel to expose a local HTTP server to the internet.

Examples:
  grok http 3000                       # Tunnel to localhost:3000 (auto-generated subdomain)
  grok http 8080 --name api            # Named tunnel (recommended, min 3 chars)
  grok http 3000 --name my-service     # Persistent tunnel with custom name
  grok http 3000 --subdomain demo      # Custom subdomain (alternative to --name)
  grok http localhost:3000             # Explicit host and port`,
	Args: cobra.ExactArgs(1),
	RunE: runHTTPTunnel,
}

func init() {
	rootCmd.AddCommand(httpCmd)

	httpCmd.Flags().StringVarP(&httpSavedName, "name", "n", "", "tunnel name for persistent tunnels (min 3 chars, recommended)")
	httpCmd.Flags().StringVarP(&httpSubdomain, "subdomain", "s", "", "custom subdomain (alternative to --name)")
}

func runHTTPTunnel(cmd *cobra.Command, args []string) error {
	// Parse local address
	localAddr := parseLocalAddr(args[0], "http")

	logger.InfoEvent().
		Str("local_addr", localAddr).
		Str("subdomain", httpSubdomain).
		Str("saved_name", httpSavedName).
		Msg("Starting HTTP tunnel")

	// Get config with overrides from flags
	cfg := GetConfig()
	if serverFlag, _ := cmd.Flags().GetString("server"); serverFlag != "" {
		cfg.Server.Addr = serverFlag
	}
	if tokenFlag, _ := cmd.Flags().GetString("token"); tokenFlag != "" {
		cfg.Auth.Token = tokenFlag
	}

	// Get dashboard flags
	dashboardEnabled := cfg.Dashboard.Enabled
	dashboardPort := cfg.Dashboard.Port

	if noDashboard, _ := cmd.Flags().GetBool("no-dashboard"); noDashboard {
		dashboardEnabled = false
	} else if dashboardFlag, _ := cmd.Flags().GetBool("dashboard"); !dashboardFlag {
		dashboardEnabled = false
	}

	if portFlag, _ := cmd.Flags().GetInt("dashboard-port"); portFlag != 4041 {
		dashboardPort = portFlag
	}

	// Build dashboard config
	dashboardCfg := dashboard.Config{}
	if dashboardEnabled {
		dashboardCfg.Port = dashboardPort
		dashboardCfg.MaxRequests = cfg.Dashboard.MaxRequests
		dashboardCfg.MaxBodySize = cfg.Dashboard.MaxBodySize
		dashboardCfg.EnableSSE = true
	}

	// Check version compatibility with server (non-blocking, non-fatal)
	checkServerVersion(cfg.Server.Addr)

	// Create tunnel client
	client, err := tunnel.NewClient(tunnel.ClientConfig{
		ServerAddr:    cfg.Server.Addr,
		TLS:           cfg.Server.TLS,
		TLSCertFile:   cfg.Server.TLSCertFile,
		TLSInsecure:   cfg.Server.TLSInsecure,
		TLSServerName: cfg.Server.TLSServerName,
		AuthToken:     cfg.Auth.Token,
		LocalAddr:     localAddr,
		Subdomain:     httpSubdomain,
		SavedName:     httpSavedName,
		Protocol:      "http",
		ReconnectCfg:  cfg.Reconnect,
		DashboardCfg:  dashboardCfg,
	})
	if err != nil {
		return fmt.Errorf("failed to create tunnel client: %w", err)
	}

	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		logger.InfoEvent().Msg("Received shutdown signal, closing tunnel...")
		cancel()
	}()

	// Start tunnel
	if err := client.Start(ctx); err != nil {
		return fmt.Errorf("tunnel error: %w", err)
	}

	return nil
}

// parseLocalAddr converts port or host:port to full address.
func parseLocalAddr(addr string, _ string) string {
	// If it's just a number, treat as port
	if port, err := strconv.Atoi(addr); err == nil {
		return fmt.Sprintf("localhost:%d", port)
	}

	// If it already has host:port format, use as-is
	return addr
}

// checkServerVersion checks if client and server versions match.
// This is a non-blocking, non-fatal check that displays a warning if versions mismatch.
func checkServerVersion(serverAddr string) {
	checker := client.NewVersionChecker(serverAddr)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	mismatch, err := checker.CheckVersion(ctx)
	if err != nil {
		// Log warning but continue (non-fatal)
		logger.WarnEvent().Err(err).Msg("Failed to check server version")
		return
	}

	// Display warning banner if versions mismatch
	if mismatch != nil && mismatch.Mismatch {
		mismatch.DisplayWarning()
	}
}
