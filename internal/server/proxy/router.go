package proxy

import (
	"strings"

	"github.com/pandeptwidyaop/grok/internal/server/tunnel"
	pkgerrors "github.com/pandeptwidyaop/grok/pkg/errors"
)

// Router routes incoming requests to appropriate tunnels.
type Router struct {
	tunnelManager *tunnel.Manager
	baseDomain    string
}

// NewRouter creates a new proxy router.
func NewRouter(tunnelManager *tunnel.Manager, baseDomain string) *Router {
	return &Router{
		tunnelManager: tunnelManager,
		baseDomain:    baseDomain,
	}
}

// Example: "myapp.grok.io" -> "myapp".
func (r *Router) ExtractSubdomain(host string) (string, error) {
	// Remove port if present
	if idx := strings.Index(host, ":"); idx != -1 {
		host = host[:idx]
	}

	// Check if host ends with base domain
	suffix := "." + r.baseDomain
	if !strings.HasSuffix(host, suffix) {
		// Not a subdomain request
		return "", pkgerrors.NewAppError("INVALID_HOST", "invalid host", nil)
	}

	// Extract subdomain
	subdomain := strings.TrimSuffix(host, suffix)
	if subdomain == "" {
		return "", pkgerrors.NewAppError("NO_SUBDOMAIN", "no subdomain in host", nil)
	}

	return subdomain, nil
}

// RouteToTunnel finds the tunnel for a given host.
func (r *Router) RouteToTunnel(host string) (*tunnel.Tunnel, error) {
	// Extract subdomain
	subdomain, err := r.ExtractSubdomain(host)
	if err != nil {
		return nil, err
	}

	// Find tunnel
	tun, ok := r.tunnelManager.GetTunnelBySubdomain(subdomain)
	if !ok {
		return nil, pkgerrors.ErrTunnelNotFound
	}

	return tun, nil
}
