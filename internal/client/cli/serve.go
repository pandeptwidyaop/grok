package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/pandeptwidyaop/grok/internal/client/dashboard"
	"github.com/pandeptwidyaop/grok/internal/client/fileserver"
	"github.com/pandeptwidyaop/grok/internal/client/tunnel"
	"github.com/pandeptwidyaop/grok/pkg/logger"
)

var (
	serveName     string
	serveAuth     string
	serveNoGzip   bool
	serveNoIndex  bool
	serve404      string
)

// serveCmd represents the serve command.
var serveCmd = &cobra.Command{
	Use:   "serve [directory]",
	Short: "Serve static files from a directory",
	Long: `Start a static file server and expose it through a tunnel.

Serves files from the specified directory (defaults to current directory).
Automatically falls back to index.html for directory requests.
Shows directory listing when index.html is not found.

Examples:
  grok serve                           # Serve current directory (auto-generated subdomain)
  grok serve .                         # Serve current directory
  grok serve ./dist --name myapp       # Serve dist/ with custom subdomain
  grok serve ~/website --auth user:pass # Serve with basic authentication
  grok serve . --no-gzip               # Disable gzip compression
  grok serve . --404 custom404.html    # Use custom 404 page

Features:
  - Automatic index.html fallback (SPA support)
  - Directory listing (when no index.html)
  - Gzip compression (enabled by default)
  - Basic authentication (optional)
  - Custom 404 page support`,
	Args: cobra.MaximumNArgs(1),
	RunE: runServe,
}

func init() {
	rootCmd.AddCommand(serveCmd)

	serveCmd.Flags().StringVarP(&serveName, "name", "n", "", "tunnel name for custom subdomain (min 3 chars)")
	serveCmd.Flags().StringVar(&serveAuth, "auth", "", "HTTP basic authentication (username:password)")
	serveCmd.Flags().BoolVar(&serveNoGzip, "no-gzip", false, "disable gzip compression")
	serveCmd.Flags().StringVar(&serve404, "404", "", "custom 404 page path")
	serveCmd.Flags().BoolVar(&serveNoIndex, "no-index", false, "disable automatic index.html fallback")
}

func runServe(cmd *cobra.Command, args []string) error {
	// Determine directory to serve
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}

	// Get absolute path
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("invalid directory path: %w", err)
	}

	// Verify directory exists
	info, err := os.Stat(absDir)
	if err != nil {
		return fmt.Errorf("cannot access directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", absDir)
	}

	// Parse basic auth credentials
	var authUser, authPass string
	if serveAuth != "" {
		parts := strings.SplitN(serveAuth, ":", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid auth format, expected username:password")
		}
		authUser, authPass = parts[0], parts[1]

		if authUser == "" || authPass == "" {
			return fmt.Errorf("username and password cannot be empty")
		}
	}

	// Find an available port for the local file server
	port, err := fileserver.FindAvailablePort()
	if err != nil {
		return fmt.Errorf("failed to find available port: %w", err)
	}

	// Create file server
	fs, err := fileserver.NewServer(fileserver.Config{
		Root:          absDir,
		EnableGzip:    !serveNoGzip,
		Custom404Path: serve404,
		BasicAuthUser: authUser,
		BasicAuthPass: authPass,
	})
	if err != nil {
		return fmt.Errorf("failed to create file server: %w", err)
	}

	// Start file server in background
	localAddr := fmt.Sprintf("localhost:%d", port)
	serverErrCh := make(chan error, 1)
	go func() {
		if err := fs.Start(localAddr); err != nil {
			serverErrCh <- fmt.Errorf("file server error: %w", err)
		}
	}()

	logger.InfoEvent().
		Str("directory", absDir).
		Str("local_addr", localAddr).
		Str("name", serveName).
		Bool("gzip", !serveNoGzip).
		Bool("auth", serveAuth != "").
		Msg("Starting file server")

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
		Subdomain:     serveName,
		SavedName:     serveName,
		Protocol:      "http",
		ReconnectCfg:  cfg.Reconnect,
		DashboardCfg:  dashboardCfg,
	})
	if err != nil {
		fs.Close()
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
		logger.InfoEvent().Msg("Received shutdown signal, closing tunnel and file server...")
		cancel()
		fs.Close()
	}()

	// Print helpful information
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Printf("  Serving: %s\n", absDir)
	fmt.Printf("  Local:   http://%s\n", localAddr)
	if serveAuth != "" {
		fmt.Printf("  Auth:    %s (HTTP Basic)\n", authUser)
	}
	if !serveNoGzip {
		fmt.Println("  Gzip:    enabled")
	}
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("\nWaiting for tunnel connection...")
	fmt.Println("Press Ctrl+C to stop")
	fmt.Println()

	// Start tunnel (this blocks until error or shutdown)
	tunnelErrCh := make(chan error, 1)
	go func() {
		if err := client.Start(ctx); err != nil {
			tunnelErrCh <- err
		}
	}()

	// Wait for either server error or tunnel error
	select {
	case err := <-serverErrCh:
		cancel()
		fs.Close()
		return err
	case err := <-tunnelErrCh:
		fs.Close()
		return fmt.Errorf("tunnel error: %w", err)
	case <-ctx.Done():
		fs.Close()
		return nil
	}
}
