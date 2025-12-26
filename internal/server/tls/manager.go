package tls

import (
	"crypto/tls"
	"fmt"
	"path/filepath"

	"golang.org/x/crypto/acme/autocert"
)

// Config holds TLS configuration.
type Config struct {
	AutoCert    bool
	CertDir     string
	Domain      string
	CertFile    string
	KeyFile     string
	Email       string // Email for Let's Encrypt registration
	DNSProvider string // DNS provider for DNS-01 challenge (cloudflare, route53, etc)
}

// Manager handles TLS certificate management.
type Manager struct {
	config      Config
	autocertMgr *autocert.Manager
	tlsConfig   *tls.Config
}

// NewManager creates a new TLS manager.
func NewManager(cfg Config) (*Manager, error) {
	m := &Manager{
		config: cfg,
	}

	if cfg.AutoCert {
		// Setup autocert for Let's Encrypt
		m.autocertMgr = &autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			HostPolicy: autocert.HostWhitelist(cfg.Domain),
			Cache:      autocert.DirCache(cfg.CertDir),
		}

		m.tlsConfig = &tls.Config{
			GetCertificate: m.autocertMgr.GetCertificate,
			MinVersion:     tls.VersionTLS12,
		}
	} else if cfg.CertFile != "" && cfg.KeyFile != "" {
		// Load manual certificates
		cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load certificates: %w", err)
		}

		m.tlsConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}
	}

	return m, nil
}

// GetTLSConfig returns the TLS configuration.
func (m *Manager) GetTLSConfig() *tls.Config {
	return m.tlsConfig
}

// Only needed for autocert.
func (m *Manager) GetHTTPHandler() *autocert.Manager {
	return m.autocertMgr
}

// IsEnabled returns whether TLS is enabled.
func (m *Manager) IsEnabled() bool {
	return m.tlsConfig != nil
}

// GetCertPath returns the certificate file path.
func (m *Manager) GetCertPath() string {
	if m.config.CertFile != "" {
		return m.config.CertFile
	}
	return filepath.Join(m.config.CertDir, m.config.Domain+".crt")
}

// GetKeyPath returns the key file path.
func (m *Manager) GetKeyPath() string {
	if m.config.KeyFile != "" {
		return m.config.KeyFile
	}
	return filepath.Join(m.config.CertDir, m.config.Domain+".key")
}
