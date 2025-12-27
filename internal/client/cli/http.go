package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/spf13/cobra"

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
  grok http 3000                       # Tunnel to localhost:3000 (auto-generated name)
  grok http 8080 --subdomain demo      # Custom subdomain demo.grok.io
  grok http 3000 --name my-api         # Named persistent tunnel
  grok http localhost:3000             # Explicit host and port`,
	Args: cobra.ExactArgs(1),
	RunE: runHTTPTunnel,
}

func init() {
	rootCmd.AddCommand(httpCmd)

	httpCmd.Flags().StringVarP(&httpSubdomain, "subdomain", "s", "", "request custom subdomain")
	httpCmd.Flags().StringVarP(&httpSavedName, "name", "n", "", "saved tunnel name (auto-generated if not provided)")
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
