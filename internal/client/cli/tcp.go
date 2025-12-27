package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

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
  grok tcp 22                       # Tunnel SSH on port 22 (auto-generated name)
  grok tcp 3306 --name my-db        # Named persistent tunnel
  grok tcp localhost:5432           # Tunnel PostgreSQL with explicit host`,
	Args: cobra.ExactArgs(1),
	RunE: runTCPTunnel,
}

func init() {
	rootCmd.AddCommand(tcpCmd)
	tcpCmd.Flags().StringVarP(&tcpSavedName, "name", "n", "", "saved tunnel name (auto-generated if not provided)")
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
