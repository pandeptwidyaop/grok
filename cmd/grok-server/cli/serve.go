package cli

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"gorm.io/gorm"

	tunnelv1 "github.com/pandeptwidyaop/grok/gen/proto/tunnel/v1"
	"github.com/pandeptwidyaop/grok/internal/db"
	"github.com/pandeptwidyaop/grok/internal/db/models"
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
	"github.com/pandeptwidyaop/grok/pkg/utils"
)

func init() {
	rootCmd.AddCommand(serveCmd)
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the Grok server",
	Long:  `Start the Grok tunnel server with gRPC, HTTP proxy, and web dashboard.`,
	RunE: func(_ *cobra.Command, _ []string) error {
		return runServer()
	},
}

// initAdminUser creates or updates the admin user from config.
func initAdminUser(database *gorm.DB, cfg *config.Config) error {
	var adminUser models.User

	// Check if admin user exists
	err := database.Where("email = ?", cfg.Auth.AdminUsername).First(&adminUser).Error
	if err == gorm.ErrRecordNotFound {
		// Create new admin user
		hashedPassword, err := utils.HashPassword(cfg.Auth.AdminPassword)
		if err != nil {
			return fmt.Errorf("failed to hash password: %w", err)
		}

		adminUser = models.User{
			Email:          cfg.Auth.AdminUsername,
			Password:       hashedPassword,
			Name:           "Administrator",
			IsActive:       true,
			Role:           models.RoleSuperAdmin,
			OrganizationID: nil, // Super admin has no organization
		}

		if err := database.Create(&adminUser).Error; err != nil {
			return fmt.Errorf("failed to create admin user: %w", err)
		}

		logger.InfoEvent().
			Str("email", adminUser.Email).
			Msg("Created admin user from config")
	} else if err != nil {
		return fmt.Errorf("failed to check admin user: %w", err)
	} else {
		// Admin user exists, update password if changed
		hashedPassword, err := utils.HashPassword(cfg.Auth.AdminPassword)
		if err != nil {
			return fmt.Errorf("failed to hash password: %w", err)
		}

		// Only update if password actually changed
		if !utils.ComparePassword(adminUser.Password, cfg.Auth.AdminPassword) {
			adminUser.Password = hashedPassword
			if err := database.Save(&adminUser).Error; err != nil {
				return fmt.Errorf("failed to update admin password: %w", err)
			}
			logger.InfoEvent().
				Str("email", adminUser.Email).
				Msg("Updated admin password from config")
		} else {
			logger.InfoEvent().
				Str("email", adminUser.Email).
				Msg("Admin user exists, password unchanged")
		}
	}

	return nil
}

// setupTLS initializes TLS manager if configured.
func setupTLS(cfg *config.Config) (*tlsmanager.Manager, error) {
	if !cfg.TLS.AutoCert && (cfg.TLS.CertFile == "" || cfg.TLS.KeyFile == "") {
		return nil, nil
	}

	tlsMgr, err := tlsmanager.NewManager(tlsmanager.Config{
		AutoCert: cfg.TLS.AutoCert,
		CertDir:  cfg.TLS.CertDir,
		Domain:   cfg.Server.Domain,
		CertFile: cfg.TLS.CertFile,
		KeyFile:  cfg.TLS.KeyFile,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to setup TLS: %w", err)
	}

	if tlsMgr.IsEnabled() {
		logger.InfoEvent().
			Bool("auto_cert", cfg.TLS.AutoCert).
			Str("domain", cfg.Server.Domain).
			Msg("TLS enabled")
	}

	return tlsMgr, nil
}

// createGRPCServer creates and configures gRPC server.
func createGRPCServer(tlsMgr *tlsmanager.Manager, tunnelManager *tunnel.Manager, tokenService *auth.TokenService) *grpc.Server {
	grpcOpts := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(interceptors.LoggingInterceptor()),
		grpc.ChainStreamInterceptor(interceptors.StreamLoggingInterceptor()),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             30 * time.Second,
			PermitWithoutStream: true,
		}),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:    60 * time.Second,
			Timeout: 20 * time.Second,
		}),
		grpc.MaxRecvMsgSize(64 << 20),
		grpc.MaxSendMsgSize(64 << 20),
	}

	if tlsMgr != nil && tlsMgr.IsEnabled() {
		grpcOpts = append(grpcOpts, grpc.Creds(credentials.NewTLS(tlsMgr.GetTLSConfig())))
	}

	grpcServer := grpc.NewServer(grpcOpts...)
	tunnelService := grpcserver.NewTunnelService(tunnelManager, tokenService)
	tunnelv1.RegisterTunnelServiceServer(grpcServer, tunnelService)

	return grpcServer
}

// createHTTPServers creates HTTP/HTTPS/API servers.
func createHTTPServers(
	cfg *config.Config,
	tlsMgr *tlsmanager.Manager,
	httpProxy http.Handler,
	database *gorm.DB,
	tokenService *auth.TokenService,
	tunnelManager *tunnel.Manager,
	webhookRouter *proxy.WebhookRouter,
) (*http.Server, *http.Server, *http.Server) {
	// Create HTTP handler
	httpHandler := httpProxy
	if tlsMgr != nil && tlsMgr.GetHTTPHandler() != nil {
		httpHandler = tlsMgr.GetHTTPHandler().HTTPHandler(httpProxy)
	}

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.HTTPPort),
		Handler:      httpHandler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

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
	apiHandler := api.NewHandler(database, tokenService, tunnelManager, webhookRouter, cfg)
	apiHandler.RegisterRoutes(apiMux)

	if dashboardFS, err := web.GetFileSystem(); err != nil {
		logger.ErrorEvent().Err(err).Msg("Failed to load dashboard files")
	} else {
		apiMux.Handle("/", http.FileServer(dashboardFS))
	}

	apiServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.APIPort),
		Handler:      apiHandler.CORSMiddleware(apiMux),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	return httpServer, httpsServer, apiServer
}

// startServers starts all HTTP/HTTPS/API servers in background goroutines.
func startServers(httpServer, httpsServer, apiServer *http.Server) {
	go func() {
		logger.InfoEvent().Str("addr", httpServer.Addr).Msg("HTTP proxy server listening")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal(fmt.Sprintf("HTTP server error: %v", err))
		}
	}()

	if httpsServer != nil {
		go func() {
			logger.InfoEvent().Str("addr", httpsServer.Addr).Msg("HTTPS proxy server listening")
			if err := httpsServer.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
				logger.Fatal(fmt.Sprintf("HTTPS server error: %v", err))
			}
		}()
	}

	go func() {
		logger.InfoEvent().Str("addr", apiServer.Addr).Msg("Dashboard API server listening")
		if err := apiServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal(fmt.Sprintf("API server error: %v", err))
		}
	}()
}

// setupGracefulShutdown configures graceful shutdown handler.
func setupGracefulShutdown(httpServer, httpsServer, apiServer *http.Server, tcpProxy *proxy.TCPProxy, grpcServer *grpc.Server) {
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		<-sigCh

		logger.InfoEvent().Msg("Shutting down servers...")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := httpServer.Shutdown(ctx); err != nil {
			logger.ErrorEvent().Err(err).Msg("HTTP server shutdown error")
		}

		if httpsServer != nil {
			if err := httpsServer.Shutdown(ctx); err != nil {
				logger.ErrorEvent().Err(err).Msg("HTTPS server shutdown error")
			}
		}

		if err := apiServer.Shutdown(ctx); err != nil {
			logger.ErrorEvent().Err(err).Msg("API server shutdown error")
		}

		tcpProxy.Shutdown()
		logger.InfoEvent().Msg("TCP proxy shut down")

		grpcServer.GracefulStop()
	}()
}

func runServer() error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := logger.Setup(logger.Config{
		Level:  cfg.Logging.Level,
		Format: cfg.Logging.Format,
		Output: cfg.Logging.Output,
		File:   cfg.Logging.File,
	}); err != nil {
		return fmt.Errorf("failed to setup logger: %w", err)
	}

	logger.InfoEvent().
		Str("version", version).
		Str("build_time", buildTime).
		Str("git_commit", gitCommit).
		Msg("Starting Grok server")

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

	if err := db.AutoMigrate(database); err != nil {
		logger.Fatal(fmt.Sprintf("Failed to run migrations: %v", err))
	}

	if err := initAdminUser(database, cfg); err != nil {
		logger.Fatal(fmt.Sprintf("Failed to initialize admin user: %v", err))
	}

	tlsMgr, err := setupTLS(cfg)
	if err != nil {
		logger.Fatal(fmt.Sprintf("Failed to setup TLS: %v", err))
	}

	tokenService := auth.NewTokenService(database)
	tlsEnabled := tlsMgr != nil && tlsMgr.IsEnabled()

	tunnelManager := tunnel.NewManager(
		database, cfg.Server.Domain, cfg.Tunnels.MaxPerUser,
		tlsEnabled, cfg.Server.HTTPPort, cfg.Server.HTTPSPort,
		cfg.Server.TCPPortStart, cfg.Server.TCPPortEnd,
	)

	tcpProxy := proxy.NewTCPProxy(tunnelManager)
	tunnelManager.SetTCPProxy(tcpProxy)

	grpcServer := createGRPCServer(tlsMgr, tunnelManager, tokenService)

	router := proxy.NewRouter(tunnelManager, cfg.Server.Domain)
	webhookRouter := proxy.NewWebhookRouter(database, tunnelManager, cfg.Server.Domain)
	httpProxy := proxy.NewHTTPProxy(router, webhookRouter, tunnelManager, database)

	httpServer, httpsServer, apiServer := createHTTPServers(cfg, tlsMgr, httpProxy, database, tokenService, tunnelManager, webhookRouter)

	grpcAddr := fmt.Sprintf(":%d", cfg.Server.GRPCPort)
	grpcListener, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		logger.Fatal(fmt.Sprintf("Failed to listen on %s: %v", grpcAddr, err))
	}

	logger.InfoEvent().Str("addr", grpcAddr).Msg("gRPC server listening")

	startServers(httpServer, httpsServer, apiServer)
	setupGracefulShutdown(httpServer, httpsServer, apiServer, tcpProxy, grpcServer)

	if err := grpcServer.Serve(grpcListener); err != nil {
		logger.Fatal(fmt.Sprintf("Failed to serve gRPC: %v", err))
	}

	return nil
}
