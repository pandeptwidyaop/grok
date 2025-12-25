package tunnel

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	tunnelv1 "github.com/pandeptwidyaop/grok/gen/proto/tunnel/v1"
	"github.com/pandeptwidyaop/grok/internal/client/config"
	"github.com/pandeptwidyaop/grok/internal/client/proxy"
	"github.com/pandeptwidyaop/grok/pkg/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ClientConfig holds tunnel client configuration
type ClientConfig struct {
	ServerAddr   string
	TLS          bool
	AuthToken    string
	LocalAddr    string
	Subdomain    string
	Protocol     string
	ReconnectCfg config.ReconnectConfig
}

// Client represents a tunnel client
type Client struct {
	cfg         ClientConfig
	conn        *grpc.ClientConn
	tunnelSvc   tunnelv1.TunnelServiceClient
	tunnelID    string
	publicURL   string
	stream      tunnelv1.TunnelService_ProxyStreamClient
	forwarder   *proxy.HTTPForwarder
	mu          sync.RWMutex
	connected   bool
	stopCh      chan struct{}
	stopped     bool
}

// NewClient creates a new tunnel client
func NewClient(cfg ClientConfig) (*Client, error) {
	// Create forwarder based on protocol
	var forwarder *proxy.HTTPForwarder
	if cfg.Protocol == "http" || cfg.Protocol == "https" {
		forwarder = proxy.NewHTTPForwarder(cfg.LocalAddr)
	}

	return &Client{
		cfg:       cfg,
		forwarder: forwarder,
		stopCh:    make(chan struct{}),
	}, nil
}

// Start starts the tunnel client
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

// connect establishes connection to server and creates tunnel
func (c *Client) connect(ctx context.Context) error {
	// Create gRPC connection
	opts := []grpc.DialOption{
		grpc.WithBlock(),
		grpc.WithTimeout(10 * time.Second),
	}

	if c.cfg.TLS {
		// TODO: Add TLS credentials
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	logger.InfoEvent().
		Str("server", c.cfg.ServerAddr).
		Msg("Connecting to server")

	conn, err := grpc.DialContext(ctx, c.cfg.ServerAddr, opts...)
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

	// Start heartbeat
	go c.startHeartbeat(ctx)

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

	// Block until context is cancelled
	<-ctx.Done()

	c.mu.Lock()
	c.connected = false
	c.mu.Unlock()

	// Close connection
	if c.stream != nil {
		c.stream.CloseSend()
	}
	if c.conn != nil {
		c.conn.Close()
	}

	logger.InfoEvent().Msg("Tunnel closed")
	return nil
}

// createTunnel creates a tunnel on the server
func (c *Client) createTunnel(ctx context.Context) error {
	protocol := tunnelv1.TunnelProtocol_HTTP
	if c.cfg.Protocol == "https" {
		protocol = tunnelv1.TunnelProtocol_HTTPS
	} else if c.cfg.Protocol == "tcp" {
		protocol = tunnelv1.TunnelProtocol_TCP
	}

	req := &tunnelv1.CreateTunnelRequest{
		AuthToken:    c.cfg.AuthToken,
		Protocol:     protocol,
		LocalAddress: c.cfg.LocalAddr,
		Subdomain:    c.cfg.Subdomain,
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

// maintainConnection maintains connection with automatic reconnection
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
		jitter := time.Duration(rand.Float64() * float64(delay) * 0.2)
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

// IsConnected returns whether the client is connected
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// GetPublicURL returns the public URL of the tunnel
func (c *Client) GetPublicURL() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.publicURL
}
