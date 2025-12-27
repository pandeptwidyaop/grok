package tunnel

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	tunnelv1 "github.com/pandeptwidyaop/grok/gen/proto/tunnel/v1"
	"github.com/pandeptwidyaop/grok/internal/client/config"
	"github.com/pandeptwidyaop/grok/internal/client/proxy"
	"github.com/pandeptwidyaop/grok/pkg/logger"
)

// cryptoRandFloat64 generates a cryptographically secure random float64 in range [0, 1).
func cryptoRandFloat64() float64 {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Fallback to 0.5 if crypto/rand fails (extremely unlikely)
		return 0.5
	}
	// Convert to uint64 and normalize to [0, 1)
	return float64(binary.BigEndian.Uint64(b[:])&0x1FFFFFFFFFFFFF) / float64(0x20000000000000)
}

// ClientConfig holds tunnel client configuration.
type ClientConfig struct {
	ServerAddr    string
	TLS           bool
	TLSCertFile   string
	TLSInsecure   bool
	TLSServerName string
	AuthToken     string
	LocalAddr     string
	Subdomain     string
	Protocol      string
	SavedName     string // Saved tunnel name (optional, for persistent tunnels)
	WebhookAppID  string // Webhook app ID (optional, for webhook tunnels)
	ReconnectCfg  config.ReconnectConfig
}

// Client represents a tunnel client.
type Client struct {
	cfg           ClientConfig
	conn          *grpc.ClientConn
	tunnelSvc     tunnelv1.TunnelServiceClient
	tunnelID      string
	publicURL     string
	stream        tunnelv1.TunnelService_ProxyStreamClient
	httpForwarder *proxy.HTTPForwarder
	tcpForwarder  *proxy.TCPForwarder
	wsConnections map[string]chan []byte // WebSocket connections by request ID
	mu            sync.RWMutex
	connected     bool
	stopCh        chan struct{}
}

// NewClient creates a new tunnel client.
func NewClient(cfg ClientConfig) (*Client, error) {
	// Create forwarder based on protocol
	var httpForwarder *proxy.HTTPForwarder
	var tcpForwarder *proxy.TCPForwarder

	switch cfg.Protocol {
	case "http", "https":
		httpForwarder = proxy.NewHTTPForwarder(cfg.LocalAddr)
	case "tcp":
		tcpForwarder = proxy.NewTCPForwarder(cfg.LocalAddr)
	}

	return &Client{
		cfg:           cfg,
		httpForwarder: httpForwarder,
		tcpForwarder:  tcpForwarder,
		stopCh:        make(chan struct{}),
	}, nil
}

// Start starts the tunnel client.
func (c *Client) Start(ctx context.Context) error {
	logger.InfoEvent().
		Str("server", c.cfg.ServerAddr).
		Str("protocol", c.cfg.Protocol).
		Msg("Starting tunnel client")

	// Start connection loop with reconnection
	if c.cfg.ReconnectCfg.Enabled {
		return c.maintainConnection(ctx)
	}

	// Single connection without reconnection
	return c.connect(ctx)
}

// tcpDialer creates a TCP connection with TCP_NODELAY enabled.
func tcpDialer(ctx context.Context, addr string) (net.Conn, error) {
	d := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}

	// Set TCP_NODELAY to disable Nagle's algorithm
	// This reduces latency for small packets (HTTP headers, etc.)
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		if err := tcpConn.SetNoDelay(true); err != nil {
			logger.WarnEvent().Err(err).Msg("Failed to set TCP_NODELAY")
		}
	}

	return conn, nil
}

// setupTLSConfig configures TLS for gRPC connection.
func (c *Client) setupTLSConfig() (*tls.Config, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: c.cfg.TLSInsecure, //nolint:gosec // User-configurable option
	}

	if c.cfg.TLSServerName != "" {
		tlsConfig.ServerName = c.cfg.TLSServerName
	}

	if c.cfg.TLSCertFile != "" && !c.cfg.TLSInsecure {
		certPool := x509.NewCertPool()
		caCert, err := os.ReadFile(c.cfg.TLSCertFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read TLS certificate file: %w", err)
		}
		if !certPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse TLS certificate")
		}
		tlsConfig.RootCAs = certPool
		logger.InfoEvent().Str("cert_file", c.cfg.TLSCertFile).Msg("Loaded custom CA certificate")
	} else if c.cfg.TLSInsecure {
		logger.WarnEvent().Msg("TLS certificate verification disabled (insecure mode)")
	}

	return tlsConfig, nil
}

// createGRPCDialOptions creates gRPC dial options for connection.
func (c *Client) createGRPCDialOptions() ([]grpc.DialOption, error) {
	opts := []grpc.DialOption{
		grpc.WithContextDialer(tcpDialer),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                30 * time.Second,
			Timeout:             10 * time.Second,
			PermitWithoutStream: true,
		}),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(64<<20),
			grpc.MaxCallSendMsgSize(64<<20),
			grpc.UseCompressor(""),
		),
	}

	if c.cfg.TLS {
		tlsConfig, err := c.setupTLSConfig()
		if err != nil {
			return nil, err
		}
		creds := credentials.NewTLS(tlsConfig)
		opts = append(opts, grpc.WithTransportCredentials(creds))
		logger.InfoEvent().
			Bool("tls_enabled", true).
			Bool("tls_insecure", c.cfg.TLSInsecure).
			Msg("TLS enabled for gRPC connection")
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
		logger.WarnEvent().Msg("Connecting without TLS (insecure)")
	}

	return opts, nil
}

// connect establishes connection to server and creates tunnel.
func (c *Client) connect(ctx context.Context) error {
	opts, err := c.createGRPCDialOptions()
	if err != nil {
		return err
	}

	logger.InfoEvent().
		Str("server", c.cfg.ServerAddr).
		Msg("Connecting to server")

	conn, err := grpc.NewClient(c.cfg.ServerAddr, opts...)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}

	c.mu.Lock()
	c.conn = conn
	c.tunnelSvc = tunnelv1.NewTunnelServiceClient(conn)
	c.mu.Unlock()

	// Create tunnel
	if err := c.createTunnel(ctx); err != nil {
		conn.Close()
		return fmt.Errorf("failed to create tunnel: %w", err)
	}

	// Start proxy stream
	if err := c.startProxyStream(ctx); err != nil {
		conn.Close()
		return fmt.Errorf("failed to start proxy stream: %w", err)
	}

	c.mu.Lock()
	c.connected = true
	c.mu.Unlock()

	logger.InfoEvent().
		Str("public_url", c.publicURL).
		Msg("Tunnel established")

	fmt.Printf("\n")
	fmt.Printf("╔═════════════════════════════════════════════════════════╗\n")
	fmt.Printf("║                 Tunnel Active                           ║\n")
	fmt.Printf("╠═════════════════════════════════════════════════════════╣\n")
	fmt.Printf("║  Public URL:  %-42s║\n", c.publicURL)
	fmt.Printf("║  Local Addr:  %-42s║\n", c.cfg.LocalAddr)
	fmt.Printf("║  Protocol:    %-42s║\n", c.cfg.Protocol)
	fmt.Printf("╚═════════════════════════════════════════════════════════╝\n")
	fmt.Printf("\n")

	// Create connection monitor channel
	connLostCh := make(chan struct{}, 1)

	// Start heartbeat with connection monitor
	go c.startHeartbeat(ctx, connLostCh)

	// Block until context is canceled or connection lost
	select {
	case <-ctx.Done():
		logger.InfoEvent().Msg("Context canceled, closing tunnel")
	case <-connLostCh:
		logger.WarnEvent().Msg("Connection lost, will reconnect")
	}

	c.mu.Lock()
	c.connected = false
	c.mu.Unlock()

	// Close connection
	if c.stream != nil {
		if err := c.stream.CloseSend(); err != nil {
			logger.WarnEvent().Err(err).Msg("Failed to close stream")
		}
	}
	if c.conn != nil {
		c.conn.Close()
	}

	// If connection was lost (not context canceled), return error to trigger reconnect
	select {
	case <-connLostCh:
		return fmt.Errorf("connection lost")
	default:
		logger.InfoEvent().Msg("Tunnel closed gracefully")
		return nil
	}
}

// createTunnel creates a tunnel on the server.
func (c *Client) createTunnel(ctx context.Context) error {
	protocol := tunnelv1.TunnelProtocol_HTTP
	if c.cfg.Protocol == "https" {
		protocol = tunnelv1.TunnelProtocol_HTTPS
	} else if c.cfg.Protocol == "tcp" {
		protocol = tunnelv1.TunnelProtocol_TCP
	}

	// Priority: --subdomain takes precedence over --name for subdomain allocation
	// --name is used for persistent tunnel naming (stored in SavedName field)
	requestedSubdomain := c.cfg.Subdomain
	if requestedSubdomain == "" && c.cfg.SavedName != "" {
		// If only --name is provided, use it as subdomain (for reconnection)
		requestedSubdomain = c.cfg.SavedName
	}

	req := &tunnelv1.CreateTunnelRequest{
		AuthToken:    c.cfg.AuthToken,
		Protocol:     protocol,
		LocalAddress: c.cfg.LocalAddr,
		Subdomain:    requestedSubdomain,
	}

	resp, err := c.tunnelSvc.CreateTunnel(ctx, req)
	if err != nil {
		return fmt.Errorf("CreateTunnel RPC failed: %w", err)
	}

	c.tunnelID = resp.TunnelId
	c.publicURL = resp.PublicUrl

	logger.InfoEvent().
		Str("tunnel_id", c.tunnelID).
		Str("public_url", c.publicURL).
		Msg("Tunnel created")

	return nil
}

// maintainConnection maintains connection with automatic reconnection.
func (c *Client) maintainConnection(ctx context.Context) error {
	delay := time.Duration(c.cfg.ReconnectCfg.InitialDelay) * time.Second
	maxDelay := time.Duration(c.cfg.ReconnectCfg.MaxDelay) * time.Second
	attempts := 0

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		// Check if we've exceeded max attempts
		if c.cfg.ReconnectCfg.MaxAttempts > 0 && attempts >= c.cfg.ReconnectCfg.MaxAttempts {
			return fmt.Errorf("max reconnection attempts (%d) exceeded", c.cfg.ReconnectCfg.MaxAttempts)
		}

		attempts++

		// Try to connect
		err := c.connect(ctx)
		if err == nil {
			// Connection successful, reset delay
			delay = time.Duration(c.cfg.ReconnectCfg.InitialDelay) * time.Second
			attempts = 0
			continue
		}

		// Log connection error
		logger.WarnEvent().
			Err(err).
			Int("attempt", attempts).
			Dur("retry_in", delay).
			Msg("Connection failed, retrying")

		// Wait before retrying with exponential backoff and jitter
		jitter := time.Duration(cryptoRandFloat64() * float64(delay) * 0.2)
		select {
		case <-time.After(delay + jitter):
			// Calculate next delay with exponential backoff
			delay = time.Duration(float64(delay) * float64(c.cfg.ReconnectCfg.BackoffFactor))
			if delay > maxDelay {
				delay = maxDelay
			}
		case <-ctx.Done():
			return nil
		}
	}
}

// IsConnected returns whether the client is connected.
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// GetPublicURL returns the public URL of the tunnel.
func (c *Client) GetPublicURL() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.publicURL
}
