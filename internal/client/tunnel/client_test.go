package tunnel

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pandeptwidyaop/grok/internal/client/config"
)

// TestNewClient tests client creation.
func TestNewClient(t *testing.T) {
	tests := []struct {
		name      string
		cfg       ClientConfig
		checkFunc func(*testing.T, *Client)
	}{
		{
			name: "HTTP client",
			cfg: ClientConfig{
				ServerAddr: "localhost:50051",
				Protocol:   "http",
				LocalAddr:  "localhost:3000",
				AuthToken:  "grok_test123",
			},
			checkFunc: func(t *testing.T, c *Client) {
				assert.NotNil(t, c.httpForwarder)
				assert.Nil(t, c.tcpForwarder)
				assert.Equal(t, "http", c.cfg.Protocol)
			},
		},
		{
			name: "HTTPS client",
			cfg: ClientConfig{
				ServerAddr: "localhost:50051",
				Protocol:   "https",
				LocalAddr:  "localhost:3000",
				AuthToken:  "grok_test123",
			},
			checkFunc: func(t *testing.T, c *Client) {
				assert.NotNil(t, c.httpForwarder)
				assert.Nil(t, c.tcpForwarder)
				assert.Equal(t, "https", c.cfg.Protocol)
			},
		},
		{
			name: "TCP client",
			cfg: ClientConfig{
				ServerAddr: "localhost:50051",
				Protocol:   "tcp",
				LocalAddr:  "localhost:22",
				AuthToken:  "grok_test123",
			},
			checkFunc: func(t *testing.T, c *Client) {
				assert.Nil(t, c.httpForwarder)
				assert.NotNil(t, c.tcpForwarder)
				assert.Equal(t, "tcp", c.cfg.Protocol)
			},
		},
		{
			name: "client with custom subdomain",
			cfg: ClientConfig{
				ServerAddr: "localhost:50051",
				Protocol:   "http",
				LocalAddr:  "localhost:3000",
				AuthToken:  "grok_test123",
				Subdomain:  "myapp",
			},
			checkFunc: func(t *testing.T, c *Client) {
				assert.Equal(t, "myapp", c.cfg.Subdomain)
			},
		},
		{
			name: "client with saved name",
			cfg: ClientConfig{
				ServerAddr: "localhost:50051",
				Protocol:   "http",
				LocalAddr:  "localhost:3000",
				AuthToken:  "grok_test123",
				SavedName:  "persistent-tunnel",
			},
			checkFunc: func(t *testing.T, c *Client) {
				assert.Equal(t, "persistent-tunnel", c.cfg.SavedName)
			},
		},
		{
			name: "client with TLS",
			cfg: ClientConfig{
				ServerAddr:    "localhost:50051",
				Protocol:      "http",
				LocalAddr:     "localhost:3000",
				AuthToken:     "grok_test123",
				TLS:           true,
				TLSInsecure:   false,
				TLSServerName: "tunnel.example.com",
			},
			checkFunc: func(t *testing.T, c *Client) {
				assert.True(t, c.cfg.TLS)
				assert.False(t, c.cfg.TLSInsecure)
				assert.Equal(t, "tunnel.example.com", c.cfg.TLSServerName)
			},
		},
		{
			name: "client with reconnect config",
			cfg: ClientConfig{
				ServerAddr: "localhost:50051",
				Protocol:   "http",
				LocalAddr:  "localhost:3000",
				AuthToken:  "grok_test123",
				ReconnectCfg: config.ReconnectConfig{
					Enabled:       true,
					InitialDelay:  1,
					MaxDelay:      30,
					BackoffFactor: 2.0,
					MaxAttempts:   10,
				},
			},
			checkFunc: func(t *testing.T, c *Client) {
				assert.True(t, c.cfg.ReconnectCfg.Enabled)
				assert.Equal(t, 1, c.cfg.ReconnectCfg.InitialDelay)
				assert.Equal(t, 30, c.cfg.ReconnectCfg.MaxDelay)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.cfg)
			require.NoError(t, err)
			require.NotNil(t, client)

			assert.NotNil(t, client.stopCh)
			assert.False(t, client.connected)

			if tt.checkFunc != nil {
				tt.checkFunc(t, client)
			}
		})
	}
}

// TestIsConnected tests connection status.
func TestIsConnected(t *testing.T) {
	client, err := NewClient(ClientConfig{
		ServerAddr: "localhost:50051",
		Protocol:   "http",
		LocalAddr:  "localhost:3000",
		AuthToken:  "grok_test123",
	})
	require.NoError(t, err)

	// Initially not connected
	assert.False(t, client.IsConnected())

	// Simulate connection
	client.mu.Lock()
	client.connected = true
	client.mu.Unlock()

	assert.True(t, client.IsConnected())

	// Simulate disconnection
	client.mu.Lock()
	client.connected = false
	client.mu.Unlock()

	assert.False(t, client.IsConnected())
}

// TestGetPublicURL tests public URL getter.
func TestGetPublicURL(t *testing.T) {
	client, err := NewClient(ClientConfig{
		ServerAddr: "localhost:50051",
		Protocol:   "http",
		LocalAddr:  "localhost:3000",
		AuthToken:  "grok_test123",
	})
	require.NoError(t, err)

	// Initially empty
	assert.Empty(t, client.GetPublicURL())

	// Set public URL
	client.mu.Lock()
	client.publicURL = "http://abc123.grok.io"
	client.mu.Unlock()

	assert.Equal(t, "http://abc123.grok.io", client.GetPublicURL())
}

// TestGetSubdomain tests subdomain extraction.
func TestGetSubdomain(t *testing.T) {
	tests := []struct {
		name      string
		publicURL string
		expected  string
	}{
		{
			name:      "HTTP URL",
			publicURL: "http://abc123.grok.io",
			expected:  "abc123",
		},
		{
			name:      "HTTPS URL",
			publicURL: "https://myapp.grok.io",
			expected:  "myapp",
		},
		{
			name:      "TCP URL",
			publicURL: "tcp://tunnel1.grok.io:10000",
			expected:  "tunnel1",
		},
		{
			name:      "URL without subdomain",
			publicURL: "http://localhost",
			expected:  "localhost",
		},
		{
			name:      "empty URL",
			publicURL: "",
			expected:  "",
		},
		{
			name:      "URL with port",
			publicURL: "http://test.grok.io:8080",
			expected:  "test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(ClientConfig{
				ServerAddr: "localhost:50051",
				Protocol:   "http",
				LocalAddr:  "localhost:3000",
				AuthToken:  "grok_test123",
			})
			require.NoError(t, err)

			client.mu.Lock()
			client.publicURL = tt.publicURL
			client.mu.Unlock()

			result := client.getSubdomain()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestCryptoRandFloat64 tests random number generation.
func TestCryptoRandFloat64(t *testing.T) {
	// Generate multiple random numbers
	values := make([]float64, 100)
	for i := 0; i < 100; i++ {
		values[i] = cryptoRandFloat64()
	}

	// Check all values are in range [0, 1)
	for i, val := range values {
		assert.GreaterOrEqual(t, val, 0.0, "value %d should be >= 0", i)
		assert.Less(t, val, 1.0, "value %d should be < 1", i)
	}

	// Check that values are not all the same (very unlikely with crypto random)
	allSame := true
	for i := 1; i < len(values); i++ {
		if values[i] != values[0] {
			allSame = false
			break
		}
	}
	assert.False(t, allSame, "random values should not all be the same")
}

// TestSetupTLSConfig tests TLS configuration.
func TestSetupTLSConfig(t *testing.T) {
	tests := []struct {
		name        string
		cfg         ClientConfig
		expectError bool
		checkFunc   func(*testing.T, *Client)
	}{
		{
			name: "insecure TLS",
			cfg: ClientConfig{
				ServerAddr:  "localhost:50051",
				Protocol:    "http",
				LocalAddr:   "localhost:3000",
				AuthToken:   "grok_test123",
				TLS:         true,
				TLSInsecure: true,
			},
			expectError: false,
			checkFunc: func(t *testing.T, c *Client) {
				tlsConfig, err := c.setupTLSConfig()
				require.NoError(t, err)
				assert.True(t, tlsConfig.InsecureSkipVerify)
			},
		},
		{
			name: "TLS with server name",
			cfg: ClientConfig{
				ServerAddr:    "localhost:50051",
				Protocol:      "http",
				LocalAddr:     "localhost:3000",
				AuthToken:     "grok_test123",
				TLS:           true,
				TLSInsecure:   true,
				TLSServerName: "tunnel.example.com",
			},
			expectError: false,
			checkFunc: func(t *testing.T, c *Client) {
				tlsConfig, err := c.setupTLSConfig()
				require.NoError(t, err)
				assert.Equal(t, "tunnel.example.com", tlsConfig.ServerName)
			},
		},
		{
			name: "invalid cert file",
			cfg: ClientConfig{
				ServerAddr:  "localhost:50051",
				Protocol:    "http",
				LocalAddr:   "localhost:3000",
				AuthToken:   "grok_test123",
				TLS:         true,
				TLSCertFile: "/nonexistent/cert.pem",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.cfg)
			require.NoError(t, err)

			tlsConfig, err := client.setupTLSConfig()
			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, tlsConfig)

				if tt.checkFunc != nil {
					tt.checkFunc(t, client)
				}
			}
		})
	}
}

// TestCreateGRPCDialOptions tests gRPC dial options creation.
func TestCreateGRPCDialOptions(t *testing.T) {
	tests := []struct {
		name        string
		cfg         ClientConfig
		expectError bool
		checkCount  int // expected minimum number of dial options
	}{
		{
			name: "without TLS",
			cfg: ClientConfig{
				ServerAddr: "localhost:50051",
				Protocol:   "http",
				LocalAddr:  "localhost:3000",
				AuthToken:  "grok_test123",
				TLS:        false,
			},
			expectError: false,
			checkCount:  4, // Context dialer, keepalive, call options, insecure creds
		},
		{
			name: "with TLS insecure",
			cfg: ClientConfig{
				ServerAddr:  "localhost:50051",
				Protocol:    "http",
				LocalAddr:   "localhost:3000",
				AuthToken:   "grok_test123",
				TLS:         true,
				TLSInsecure: true,
			},
			expectError: false,
			checkCount:  4, // Context dialer, keepalive, call options, TLS creds
		},
		{
			name: "with invalid cert file",
			cfg: ClientConfig{
				ServerAddr:  "localhost:50051",
				Protocol:    "http",
				LocalAddr:   "localhost:3000",
				AuthToken:   "grok_test123",
				TLS:         true,
				TLSCertFile: "/nonexistent/cert.pem",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.cfg)
			require.NoError(t, err)

			opts, err := client.createGRPCDialOptions()
			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.GreaterOrEqual(t, len(opts), tt.checkCount)
			}
		})
	}
}

// TestTCPDialer tests TCP dialer function.
func TestTCPDialer(t *testing.T) {
	// Test connection timeout with invalid address
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Try to connect to non-routable address (will timeout)
	conn, err := tcpDialer(ctx, "192.0.2.1:9999") // TEST-NET-1, non-routable
	if err != nil {
		// Expected timeout
		assert.Error(t, err)
	} else {
		// Unlikely but possible if routing is configured
		conn.Close()
	}
}

// TestSignalConnectionLost tests connection lost signaling.
func TestSignalConnectionLost(t *testing.T) {
	connLostCh := make(chan struct{}, 1)

	// First signal should succeed
	signalConnectionLost(connLostCh)

	select {
	case <-connLostCh:
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected signal on channel")
	}

	// Second signal should not block (channel is buffered)
	signalConnectionLost(connLostCh)

	// Third signal should not block even with full buffer (select default case)
	signalConnectionLost(connLostCh)
}

// BenchmarkCryptoRandFloat64 benchmarks random number generation.
func BenchmarkCryptoRandFloat64(b *testing.B) {
	for i := 0; i < b.N; i++ {
		cryptoRandFloat64()
	}
}

// BenchmarkGetSubdomain benchmarks subdomain extraction.
func BenchmarkGetSubdomain(b *testing.B) {
	client, _ := NewClient(ClientConfig{
		ServerAddr: "localhost:50051",
		Protocol:   "http",
		LocalAddr:  "localhost:3000",
		AuthToken:  "grok_test123",
	})

	client.mu.Lock()
	client.publicURL = "http://abc123.grok.io"
	client.mu.Unlock()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.getSubdomain()
	}
}
