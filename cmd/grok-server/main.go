package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	tunnelv1 "github.com/pandeptwidyaop/grok/gen/proto/tunnel/v1"
	"github.com/pandeptwidyaop/grok/internal/db"
	"github.com/pandeptwidyaop/grok/internal/server/auth"
	"github.com/pandeptwidyaop/grok/internal/server/config"
	grpcserver "github.com/pandeptwidyaop/grok/internal/server/grpc"
	"github.com/pandeptwidyaop/grok/internal/server/grpc/interceptors"
	"github.com/pandeptwidyaop/grok/internal/server/proxy"
	tlsmanager "github.com/pandeptwidyaop/grok/internal/server/tls"
	"github.com/pandeptwidyaop/grok/internal/server/tunnel"
	"github.com/pandeptwidyaop/grok/internal/server/web"
	"github.com/pandeptwidyaop/grok/internal/server/web/api"
	"github.com/pandeptwidyaop/grok/pkg/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	// Version info (set by ldflags during build)
	version   = "dev"
	buildTime = "unknown"
	gitCommit = "unknown"

	// Flags
	configPath  = flag.String("config", "configs/server.yaml", "path to config file")
	showVersion = flag.Bool("version", false, "show version information")
)

func main() {
	flag.Parse()

	// Show version and exit
	if *showVersion {
		fmt.Printf("Grok Server\n")
		fmt.Printf("  Version:    %s\n", version)
		fmt.Printf("  Build Time: %s\n", buildTime)
		fmt.Printf("  Git Commit: %s\n", gitCommit)
		fmt.Printf("  Go Version: %s\n", "go1.23+")
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Setup logger
	if err := logger.Setup(logger.Config{
		Level:  cfg.Logging.Level,
		Format: cfg.Logging.Format,
		Output: cfg.Logging.Output,
		File:   cfg.Logging.File,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to setup logger: %v\n", err)
		os.Exit(1)
	}

	logger.InfoEvent().
		Str("version", version).
		Str("build_time", buildTime).
		Str("git_commit", gitCommit).
		Msg("Starting Grok server")

	// Connect to database
	logger.InfoEvent().
		Str("driver", cfg.Database.Driver).
		Str("database", cfg.Database.Database).
		Msg("Connecting to database")

	database, err := db.Connect(db.Config{
		Driver:   cfg.Database.Driver,
		Host:     cfg.Database.Host,
		Port:     cfg.Database.Port,
		Database: cfg.Database.Database,
		Username: cfg.Database.Username,
		Password: cfg.Database.Password,
		SSLMode:  cfg.Database.SSLMode,
	})
	if err != nil {
		logger.Fatal(fmt.Sprintf("Failed to connect to database: %v", err))
	}

	logger.InfoEvent().Msg("Connected to database")

	// Run auto migrations
	if err := db.AutoMigrate(database); err != nil {
		logger.Fatal(fmt.Sprintf("Failed to run migrations: %v", err))
	}

	logger.InfoEvent().Msg("Database migrations completed")

	// Setup TLS if enabled
	var tlsMgr *tlsmanager.Manager
	if cfg.TLS.AutoCert || (cfg.TLS.CertFile != "" && cfg.TLS.KeyFile != "") {
		tlsMgr, err = tlsmanager.NewManager(tlsmanager.Config{
			AutoCert: cfg.TLS.AutoCert,
			CertDir:  cfg.TLS.CertDir,
			Domain:   cfg.Server.Domain,
			CertFile: cfg.TLS.CertFile,
			KeyFile:  cfg.TLS.KeyFile,
		})
		if err != nil {
			logger.Fatal(fmt.Sprintf("Failed to setup TLS: %v", err))
		}

		if tlsMgr.IsEnabled() {
			logger.InfoEvent().
				Bool("auto_cert", cfg.TLS.AutoCert).
				Str("domain", cfg.Server.Domain).
				Msg("TLS enabled")
		}
	}

	// Initialize services
	tokenService := auth.NewTokenService(database)
	tunnelManager := tunnel.NewManager(
		database,
		cfg.Server.Domain,
		cfg.Tunnels.MaxPerUser,
	)

	// Create gRPC server with interceptors
	var grpcOpts []grpc.ServerOption
	grpcOpts = append(grpcOpts,
		grpc.ChainUnaryInterceptor(
			interceptors.LoggingInterceptor(),
			// Note: CreateTunnel doesn't use auth interceptor since it has auth in request
		),
		grpc.ChainStreamInterceptor(
			interceptors.StreamLoggingInterceptor(),
			// Note: ProxyStream will handle auth differently
		),
	)

	// Add TLS credentials for gRPC if enabled
	if tlsMgr != nil && tlsMgr.IsEnabled() {
		grpcOpts = append(grpcOpts, grpc.Creds(credentials.NewTLS(tlsMgr.GetTLSConfig())))
	}

	grpcServer := grpc.NewServer(grpcOpts...)

	// Register services
	tunnelService := grpcserver.NewTunnelService(tunnelManager, tokenService)
	tunnelv1.RegisterTunnelServiceServer(grpcServer, tunnelService)

	// Setup HTTP reverse proxy
	router := proxy.NewRouter(tunnelManager, cfg.Server.Domain)
	httpProxy := proxy.NewHTTPProxy(router)

	// Create HTTP handler (with autocert support if enabled)
	var httpHandler http.Handler = httpProxy
	if tlsMgr != nil && tlsMgr.GetHTTPHandler() != nil {
		// Wrap with autocert HTTP-01 challenge handler
		httpHandler = tlsMgr.GetHTTPHandler().HTTPHandler(httpProxy)
	}

	// Create HTTP server
	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.HTTPPort),
		Handler:      httpHandler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// Create HTTPS server if TLS is enabled
	var httpsServer *http.Server
	if tlsMgr != nil && tlsMgr.IsEnabled() {
		httpsServer = &http.Server{
			Addr:         fmt.Sprintf(":%d", cfg.Server.HTTPSPort),
			Handler:      httpProxy,
			TLSConfig:    tlsMgr.GetTLSConfig(),
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
		}
	}

	// Setup Dashboard API server
	apiMux := http.NewServeMux()

	// Register API handlers
	apiHandler := api.NewHandler(database, tokenService, tunnelManager)
	apiHandler.RegisterRoutes(apiMux)

	// Serve embedded dashboard
	dashboardFS, err := web.GetFileSystem()
	if err != nil {
		logger.ErrorEvent().Err(err).Msg("Failed to load dashboard files")
	} else {
		apiMux.Handle("/", http.FileServer(dashboardFS))
	}

	apiServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.APIPort),
		Handler:      apiMux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// Start gRPC server
	grpcAddr := fmt.Sprintf(":%d", cfg.Server.GRPCPort)
	grpcListener, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		logger.Fatal(fmt.Sprintf("Failed to listen on %s: %v", grpcAddr, err))
	}

	logger.InfoEvent().
		Str("addr", grpcAddr).
		Msg("gRPC server listening")

	// Start HTTP proxy server in goroutine
	go func() {
		logger.InfoEvent().
			Str("addr", httpServer.Addr).
			Msg("HTTP proxy server listening")

		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal(fmt.Sprintf("HTTP server error: %v", err))
		}
	}()

	// Start HTTPS proxy server if TLS is enabled
	if httpsServer != nil {
		go func() {
			logger.InfoEvent().
				Str("addr", httpsServer.Addr).
				Msg("HTTPS proxy server listening")

			if err := httpsServer.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
				logger.Fatal(fmt.Sprintf("HTTPS server error: %v", err))
			}
		}()
	}

	// Start Dashboard API server
	go func() {
		logger.InfoEvent().
			Str("addr", apiServer.Addr).
			Msg("Dashboard API server listening")

		if err := apiServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal(fmt.Sprintf("API server error: %v", err))
		}
	}()

	// Handle graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		<-sigCh

		logger.InfoEvent().Msg("Shutting down servers...")

		// Shutdown HTTP server
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(ctx); err != nil {
			logger.ErrorEvent().Err(err).Msg("HTTP server shutdown error")
		}

		// Shutdown HTTPS server if running
		if httpsServer != nil {
			if err := httpsServer.Shutdown(ctx); err != nil {
				logger.ErrorEvent().Err(err).Msg("HTTPS server shutdown error")
			}
		}

		// Shutdown API server
		if err := apiServer.Shutdown(ctx); err != nil {
			logger.ErrorEvent().Err(err).Msg("API server shutdown error")
		}

		// Stop gRPC server
		grpcServer.GracefulStop()
	}()

	// Start serving gRPC
	if err := grpcServer.Serve(grpcListener); err != nil {
		logger.Fatal(fmt.Sprintf("Failed to serve gRPC: %v", err))
	}
}
