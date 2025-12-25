package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/pandeptwidyaop/grok/internal/client/tunnel"
	"github.com/pandeptwidyaop/grok/pkg/logger"
	"github.com/spf13/cobra"
)

var (
	httpSubdomain string
)

// httpCmd represents the http command
var httpCmd = &cobra.Command{
	Use:   "http [port]",
	Short: "Start HTTP tunnel",
	Long: `Create an HTTP tunnel to expose a local HTTP server to the internet.

Examples:
  grok http 3000                    # Tunnel to localhost:3000
  grok http 8080 --subdomain demo   # Custom subdomain demo.grok.io
  grok http localhost:3000          # Explicit host and port`,
	Args: cobra.ExactArgs(1),
	RunE: runHTTPTunnel,
}

func init() {
	rootCmd.AddCommand(httpCmd)

	httpCmd.Flags().StringVarP(&httpSubdomain, "subdomain", "s", "", "request custom subdomain")
}

func runHTTPTunnel(cmd *cobra.Command, args []string) error {
	// Parse local address
	localAddr := parseLocalAddr(args[0], "http")

	logger.InfoEvent().
		Str("local_addr", localAddr).
		Str("subdomain", httpSubdomain).
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
		AuthToken:     cfg.Auth.Token,
		LocalAddr:     localAddr,
		Subdomain:     httpSubdomain,
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

// parseLocalAddr converts port or host:port to full address
func parseLocalAddr(addr string, protocol string) string {
	// If it's just a number, treat as port
	if port, err := strconv.Atoi(addr); err == nil {
		return fmt.Sprintf("localhost:%d", port)
	}

	// If it already has host:port format, use as-is
	return addr
}
