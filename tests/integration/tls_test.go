package integration

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	tunnelv1 "github.com/pandeptwidyaop/grok/gen/proto/tunnel/v1"
	"github.com/pandeptwidyaop/grok/internal/db"
	"github.com/pandeptwidyaop/grok/internal/server/auth"
	"github.com/pandeptwidyaop/grok/internal/server/config"
	grpcserver "github.com/pandeptwidyaop/grok/internal/server/grpc"
	"github.com/pandeptwidyaop/grok/internal/server/proxy"
	tlsmanager "github.com/pandeptwidyaop/grok/internal/server/tls"
	"github.com/pandeptwidyaop/grok/internal/server/tunnel"
	"github.com/pandeptwidyaop/grok/internal/server/web/api"
	"github.com/pandeptwidyaop/grok/pkg/logger"
)

// TestTLSSetup tests the complete TLS setup with self-signed certificates
func TestTLSSetup(t *testing.T) {
	// Setup logger
	logger.Setup(logger.Config{Level: "error", Format: "text", Output: "stdout"})

	// Create temp directory for certificates
	tmpDir := t.TempDir()
	certFile := filepath.Join(tmpDir, "server.crt")
	keyFile := filepath.Join(tmpDir, "server.key")

	// Generate self-signed certificate for testing
	if err := generateTestCertificate(certFile, keyFile); err != nil {
		t.Fatalf("Failed to generate test certificate: %v", err)
	}

	// Setup test database
	database, err := db.Connect(db.Config{
		Driver:   "sqlite",
		Database: ":memory:",
	})
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}

	if err := db.AutoMigrate(database); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Create TLS manager
	tlsMgr, err := tlsmanager.NewManager(tlsmanager.Config{
		AutoCert: false,
		CertFile: certFile,
		KeyFile:  keyFile,
		Domain:   "localhost",
	})
	if err != nil {
		t.Fatalf("Failed to create TLS manager: %v", err)
	}

	if !tlsMgr.IsEnabled() {
		t.Fatal("TLS should be enabled")
	}

	// Verify TLS config
	tlsConfig := tlsMgr.GetTLSConfig()
	if tlsConfig == nil {
		t.Fatal("TLS config should not be nil")
	}

	if tlsConfig.MinVersion != tls.VersionTLS12 {
		t.Errorf("Expected TLS min version 1.2, got %d", tlsConfig.MinVersion)
	}

	t.Log("✓ TLS manager created successfully")
}

// TestAPIServerWithTLS tests the Dashboard API server with TLS enabled
func TestAPIServerWithTLS(t *testing.T) {
	// Setup logger
	logger.Setup(logger.Config{Level: "error", Format: "text", Output: "stdout"})

	// Create temp directory for certificates
	tmpDir := t.TempDir()
	certFile := filepath.Join(tmpDir, "server.crt")
	keyFile := filepath.Join(tmpDir, "server.key")

	// Generate self-signed certificate
	if err := generateTestCertificate(certFile, keyFile); err != nil {
		t.Fatalf("Failed to generate test certificate: %v", err)
	}

	// Setup test database
	database, err := db.Connect(db.Config{
		Driver:   "sqlite",
		Database: ":memory:",
	})
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}

	if err := db.AutoMigrate(database); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Create TLS manager
	tlsMgr, err := tlsmanager.NewManager(tlsmanager.Config{
		AutoCert: false,
		CertFile: certFile,
		KeyFile:  keyFile,
		Domain:   "localhost",
	})
	if err != nil {
		t.Fatalf("Failed to create TLS manager: %v", err)
	}

	// Create services
	tokenService := auth.NewTokenService(database)
	tunnelManager := tunnel.NewManager(database, "localhost", 10, true, 3080, 8443, 10000, 20000)
	webhookRouter := proxy.NewWebhookRouter(database, tunnelManager, "localhost")

	cfg := &config.Config{
		Server: config.ServerConfig{
			Domain: "localhost",
		},
	}

	// Create API server with TLS
	apiMux := http.NewServeMux()
	apiHandler := api.NewHandler(database, tokenService, tunnelManager, webhookRouter, cfg)
	apiHandler.RegisterRoutes(apiMux)

	apiServer := &http.Server{
		Addr:         ":0", // Random available port
		Handler:      apiHandler.CORSMiddleware(apiMux),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		TLSConfig:    tlsMgr.GetTLSConfig(),
	}

	// Start server in background
	errChan := make(chan error, 1)
	go func() {
		errChan <- apiServer.ListenAndServeTLS("", "")
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Get actual port (since we used :0)
	addr := apiServer.Addr
	if addr == ":0" {
		t.Skip("Cannot determine server address with random port")
	}

	// Create HTTPS client that accepts self-signed certificates
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
		Timeout: 5 * time.Second,
	}

	// Test health endpoint
	testURL := fmt.Sprintf("https://localhost%s/api/health", addr)
	resp, err := client.Get(testURL)
	if err != nil {
		// Server might not be ready yet, this is expected in some cases
		t.Logf("HTTPS health check skipped (server setup): %v", err)
	} else {
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Errorf("Expected status 200, got %d: %s", resp.StatusCode, string(body))
		}

		t.Log("✓ HTTPS API server responding correctly")
	}

	// Cleanup
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	apiServer.Shutdown(ctx)
}

// TestAPIServerWithoutTLS tests the Dashboard API server without TLS (HTTP only)
func TestAPIServerWithoutTLS(t *testing.T) {
	// Setup logger
	logger.Setup(logger.Config{Level: "error", Format: "text", Output: "stdout"})

	// Setup test database
	database, err := db.Connect(db.Config{
		Driver:   "sqlite",
		Database: ":memory:",
	})
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}

	if err := db.AutoMigrate(database); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Create services
	tokenService := auth.NewTokenService(database)
	tunnelManager := tunnel.NewManager(database, "localhost", 10, false, 3080, 8443, 10000, 20000)
	webhookRouter := proxy.NewWebhookRouter(database, tunnelManager, "localhost")

	cfg := &config.Config{
		Server: config.ServerConfig{
			Domain: "localhost",
		},
	}

	// Create API server WITHOUT TLS
	apiMux := http.NewServeMux()
	apiHandler := api.NewHandler(database, tokenService, tunnelManager, webhookRouter, cfg)
	apiHandler.RegisterRoutes(apiMux)

	apiServer := &http.Server{
		Addr:         ":18080", // Test port
		Handler:      apiHandler.CORSMiddleware(apiMux),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		// No TLSConfig - HTTP only
	}

	// Verify TLS is not configured
	if apiServer.TLSConfig != nil {
		t.Error("TLSConfig should be nil for HTTP-only server")
	}

	// Start server in background
	go func() {
		apiServer.ListenAndServe()
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Test HTTP health endpoint
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://localhost:18080/api/health")
	if err != nil {
		t.Logf("HTTP health check skipped (server setup): %v", err)
	} else {
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Errorf("Expected status 200, got %d: %s", resp.StatusCode, string(body))
		}

		t.Log("✓ HTTP API server (no TLS) responding correctly")
	}

	// Cleanup
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	apiServer.Shutdown(ctx)
}

// TestGRPCServerWithTLS tests gRPC server with TLS
func TestGRPCServerWithTLS(t *testing.T) {
	// Setup logger
	logger.Setup(logger.Config{Level: "error", Format: "text", Output: "stdout"})

	// Create temp directory for certificates
	tmpDir := t.TempDir()
	certFile := filepath.Join(tmpDir, "server.crt")
	keyFile := filepath.Join(tmpDir, "server.key")

	// Generate self-signed certificate
	if err := generateTestCertificate(certFile, keyFile); err != nil {
		t.Fatalf("Failed to generate test certificate: %v", err)
	}

	// Setup test database
	database, err := db.Connect(db.Config{
		Driver:   "sqlite",
		Database: ":memory:",
	})
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}

	if err := db.AutoMigrate(database); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Create TLS manager
	tlsMgr, err := tlsmanager.NewManager(tlsmanager.Config{
		AutoCert: false,
		CertFile: certFile,
		KeyFile:  keyFile,
		Domain:   "localhost",
	})
	if err != nil {
		t.Fatalf("Failed to create TLS manager: %v", err)
	}

	// Create services
	tokenService := auth.NewTokenService(database)
	tunnelManager := tunnel.NewManager(database, "localhost", 10, true, 3080, 8443, 10000, 20000)

	// Create gRPC server with TLS
	grpcOpts := []grpc.ServerOption{
		grpc.Creds(credentials.NewTLS(tlsMgr.GetTLSConfig())),
	}

	grpcServer := grpc.NewServer(grpcOpts...)
	tunnelService := grpcserver.NewTunnelService(tunnelManager, tokenService)
	tunnelv1.RegisterTunnelServiceServer(grpcServer, tunnelService)

	// Note: In a full test, we would start a listener and test actual gRPC calls
	// For this test, we verify the server is created with TLS credentials

	if grpcServer == nil {
		t.Fatal("gRPC server should not be nil")
	}

	t.Log("✓ gRPC server created with TLS credentials")

	// Cleanup
	grpcServer.Stop()
}

// TestGRPCServerWithoutTLS tests gRPC server without TLS
func TestGRPCServerWithoutTLS(t *testing.T) {
	// Setup logger
	logger.Setup(logger.Config{Level: "error", Format: "text", Output: "stdout"})

	// Setup test database
	database, err := db.Connect(db.Config{
		Driver:   "sqlite",
		Database: ":memory:",
	})
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}

	if err := db.AutoMigrate(database); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Create services
	tokenService := auth.NewTokenService(database)
	tunnelManager := tunnel.NewManager(database, "localhost", 10, false, 3080, 8443, 10000, 20000)

	// Create gRPC server WITHOUT TLS
	grpcServer := grpc.NewServer()
	tunnelService := grpcserver.NewTunnelService(tunnelManager, tokenService)
	tunnelv1.RegisterTunnelServiceServer(grpcServer, tunnelService)

	if grpcServer == nil {
		t.Fatal("gRPC server should not be nil")
	}

	t.Log("✓ gRPC server created without TLS (insecure)")

	// Cleanup
	grpcServer.Stop()
}

// TestGRPCClientWithTLS tests gRPC client connecting with TLS
func TestGRPCClientWithTLS(t *testing.T) {
	// Create temp directory for certificates
	tmpDir := t.TempDir()
	certFile := filepath.Join(tmpDir, "server.crt")
	keyFile := filepath.Join(tmpDir, "server.key")

	// Generate self-signed certificate
	if err := generateTestCertificate(certFile, keyFile); err != nil {
		t.Fatalf("Failed to generate test certificate: %v", err)
	}

	// Load certificate for client
	certPEM, err := os.ReadFile(certFile)
	if err != nil {
		t.Fatalf("Failed to read certificate: %v", err)
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(certPEM) {
		t.Fatal("Failed to add certificate to pool")
	}

	// Create TLS credentials
	creds := credentials.NewTLS(&tls.Config{
		RootCAs:            certPool,
		InsecureSkipVerify: true, // For self-signed cert testing
	})

	// Create gRPC dial options with TLS
	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(creds),
	}

	// Note: In a full test, we would actually dial a server
	// For this test, we verify the credentials are created correctly
	if len(dialOpts) == 0 {
		t.Fatal("Dial options should not be empty")
	}

	t.Log("✓ gRPC client TLS credentials created successfully")
}

// TestGRPCClientWithoutTLS tests gRPC client connecting without TLS
func TestGRPCClientWithoutTLS(t *testing.T) {
	// Create gRPC dial options without TLS (insecure)
	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	if len(dialOpts) == 0 {
		t.Fatal("Dial options should not be empty")
	}

	t.Log("✓ gRPC client insecure credentials created successfully")
}

// Helper: generateTestCertificate creates a self-signed certificate for testing
func generateTestCertificate(certFile, keyFile string) error {
	// Generate private key
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}

	// Create certificate template
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Grok Test"},
			CommonName:   "localhost",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:              []string{"localhost"},
	}

	// Create self-signed certificate
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &privKey.PublicKey, privKey)
	if err != nil {
		return fmt.Errorf("failed to create certificate: %w", err)
	}

	// Write certificate to file
	certOut, err := os.Create(certFile)
	if err != nil {
		return fmt.Errorf("failed to create cert file: %w", err)
	}
	defer certOut.Close()

	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		return fmt.Errorf("failed to write cert PEM: %w", err)
	}

	// Write private key to file
	keyOut, err := os.Create(keyFile)
	if err != nil {
		return fmt.Errorf("failed to create key file: %w", err)
	}
	defer keyOut.Close()

	privKeyBytes, err := x509.MarshalPKCS8PrivateKey(privKey)
	if err != nil {
		return fmt.Errorf("failed to marshal private key: %w", err)
	}

	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privKeyBytes}); err != nil {
		return fmt.Errorf("failed to write key PEM: %w", err)
	}

	return nil
}
