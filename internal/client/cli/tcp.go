package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/pandeptwidyaop/grok/internal/client/dashboard"
	"github.com/pandeptwidyaop/grok/internal/client/tunnel"
	"github.com/pandeptwidyaop/grok/pkg/logger"
)

var (
	tcpSavedName string
)

// tcpCmd represents the tcp command.
var tcpCmd = &cobra.Command{
	Use:   "tcp [port]",
	Short: "Start TCP tunnel",
	Long: `Create a TCP tunnel to expose a local TCP service to the internet.

Examples:
  grok tcp 22                       # Tunnel SSH on port 22 (auto-generated subdomain)
  grok tcp 3306 --name db           # Named tunnel (recommended, min 3 chars)
  grok tcp 5432 --name postgres     # Persistent tunnel with custom name
  grok tcp localhost:27017          # Explicit host and port`,
	Args: cobra.ExactArgs(1),
	RunE: runTCPTunnel,
}

func init() {
	rootCmd.AddCommand(tcpCmd)
	tcpCmd.Flags().StringVarP(&tcpSavedName, "name", "n", "", "tunnel name for persistent tunnels (min 3 chars, recommended)")
}

func runTCPTunnel(cmd *cobra.Command, args []string) error {
	// Parse local address
	localAddr := parseLocalAddr(args[0], "tcp")

	logger.InfoEvent().
		Str("local_addr", localAddr).
		Str("saved_name", tcpSavedName).
		Msg("Starting TCP tunnel")

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

	// Create tunnel client
	client, err := tunnel.NewClient(tunnel.ClientConfig{
		ServerAddr:    cfg.Server.Addr,
		TLS:           cfg.Server.TLS,
		TLSCertFile:   cfg.Server.TLSCertFile,
		TLSInsecure:   cfg.Server.TLSInsecure,
		TLSServerName: cfg.Server.TLSServerName,
		AuthToken:     cfg.Auth.Token,
		LocalAddr:     localAddr,
		SavedName:     tcpSavedName,
		Protocol:      "tcp",
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
