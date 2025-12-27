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

var webhookCmd = &cobra.Command{
	Use:   "webhook [app_id] [local_address]",
	Short: "Start webhook tunnel",
	Long: `Create a webhook tunnel that connects to a webhook app.

A webhook tunnel broadcasts incoming HTTP requests to all connected tunnels
for the same webhook app. This allows multiple services to process the same
webhook event.

Examples:
  # Connect to webhook app with local service
  grok webhook abc-123-def localhost:3000

  # Connect with path
  grok webhook abc-123-def localhost:3000/webhooks

  # Connect using port shorthand
  grok webhook abc-123-def 8080`,
	Args: cobra.ExactArgs(2),
	RunE: runWebhookTunnel,
}

func init() {
	rootCmd.AddCommand(webhookCmd)
}

func runWebhookTunnel(cmd *cobra.Command, args []string) error {
	appID := args[0]
	localAddr := parseLocalAddr(args[1], "http")

	logger.InfoEvent().
		Str("app_id", appID).
		Str("local_addr", localAddr).
		Msg("Starting webhook tunnel")

	// Get configuration
	cfg := GetConfig()

	// Check for server override from flags
	if serverFlag, _ := cmd.Flags().GetString("server"); serverFlag != "" {
		cfg.Server.Addr = serverFlag
	}

	// Validate auth token
	if cfg.Auth.Token == "" {
		return fmt.Errorf("authentication token not configured. Run 'grok config set-token <token>' first")
	}

	// Create tunnel client with webhook configuration
	client, err := tunnel.NewClient(tunnel.ClientConfig{
		ServerAddr:   cfg.Server.Addr,
		TLS:          cfg.Server.TLS,
		AuthToken:    cfg.Auth.Token,
		LocalAddr:    localAddr,
		Protocol:     "http",
		WebhookAppID: appID, // Webhook-specific field
		ReconnectCfg: cfg.Reconnect,
	})
	if err != nil {
		return fmt.Errorf("failed to create webhook tunnel client: %w", err)
	}

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	// Handle shutdown signal
	go func() {
		sig := <-sigCh
		logger.InfoEvent().
			Str("signal", sig.String()).
			Msg("Received shutdown signal, closing tunnel...")
		cancel()
	}()

	// Start tunnel connection
	logger.InfoEvent().
		Str("server", cfg.Server.Addr).
		Str("local_addr", localAddr).
		Str("app_id", appID).
		Msg("Connecting webhook tunnel to server...")

	if err := client.Start(ctx); err != nil {
		if err == context.Canceled {
			logger.InfoEvent().Msg("Webhook tunnel closed gracefully")
			return nil
		}
		return fmt.Errorf("webhook tunnel error: %w", err)
	}

	return nil
}
