package tunnel

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"gorm.io/gorm"

	tunnelv1 "github.com/pandeptwidyaop/grok/gen/proto/tunnel/v1"
	"github.com/pandeptwidyaop/grok/internal/db/models"
	"github.com/pandeptwidyaop/grok/internal/server/tcp"
	pkgerrors "github.com/pandeptwidyaop/grok/pkg/errors"
	"github.com/pandeptwidyaop/grok/pkg/logger"
	"github.com/pandeptwidyaop/grok/pkg/utils"
)

// TunnelEventType represents the type of tunnel event.
type TunnelEventType string

const (
	EventTunnelConnected    TunnelEventType = "tunnel_connected"
	EventTunnelDisconnected TunnelEventType = "tunnel_disconnected"
	EventTunnelUpdated      TunnelEventType = "tunnel_updated"
	EventTunnelStatsUpdated TunnelEventType = "tunnel_stats_updated"
)

// TunnelEvent represents a tunnel state change event.
type TunnelEvent struct {
	Type     TunnelEventType
	TunnelID uuid.UUID
	Tunnel   *models.Tunnel
}

// TunnelEventHandler is a callback for tunnel events.
type TunnelEventHandler func(TunnelEvent)

// TCPProxy interface for TCP proxy operations.
type TCPProxy interface {
	StartListener(port int, tunnelID uuid.UUID) error
	StopListener(port int) error
}

// Manager manages active tunnels.
type Manager struct {
	db                *gorm.DB
	tunnels           sync.Map // subdomain → *Tunnel
	tunnelsByID       sync.Map // tunnel_id → *Tunnel
	maxTunnelsPerUser int
	baseDomain        string
	tlsEnabled        bool          // Whether TLS is enabled
	httpPort          int           // HTTP port (default 80)
	httpsPort         int           // HTTPS port (default 443)
	portPool          *tcp.PortPool // TCP port pool for TCP tunnels
	tcpProxy          TCPProxy      // TCP proxy for starting/stopping listeners
	mu                sync.RWMutex
	eventHandlers     []TunnelEventHandler
	eventMu           sync.RWMutex
}

// NewManager creates a new tunnel manager.
func NewManager(db *gorm.DB, baseDomain string, maxTunnelsPerUser int, tlsEnabled bool, httpPort, httpsPort, tcpPortStart, tcpPortEnd int) *Manager {
	m := &Manager{
		db:                db,
		baseDomain:        baseDomain,
		maxTunnelsPerUser: maxTunnelsPerUser,
		tlsEnabled:        tlsEnabled,
		httpPort:          httpPort,
		httpsPort:         httpsPort,
	}

	// Initialize TCP port pool for TCP tunnels
	portPool, err := tcp.NewPortPool(db, tcpPortStart, tcpPortEnd)
	if err != nil {
		logger.ErrorEvent().
			Err(err).
			Int("tcp_port_start", tcpPortStart).
			Int("tcp_port_end", tcpPortEnd).
			Msg("Failed to initialize TCP port pool")
		// Continue without TCP support
		portPool = nil
	}
	m.portPool = portPool

	// Cleanup stale tunnels from previous server runs
	if err := m.CleanupStaleTunnels(context.Background()); err != nil {
		logger.WarnEvent().
			Err(err).
			Msg("Failed to cleanup stale tunnels on startup")
	}

	// Start periodic stats updater (every 3 seconds)
	go m.startPeriodicStatsUpdater()

	return m
}

// SetTCPProxy sets the TCP proxy for starting/stopping TCP listeners.
func (m *Manager) SetTCPProxy(proxy TCPProxy) {
	m.tcpProxy = proxy
}

// All tunnels are persistent - they are marked offline but domain reservations are kept.
func (m *Manager) CleanupStaleTunnels(ctx context.Context) error {
	logger.InfoEvent().Msg("Cleaning up stale tunnels from previous sessions...")

	// Update all active/connected tunnels to offline (keeping domain reservations)
	result := m.db.WithContext(ctx).
		Model(&models.Tunnel{}).
		Where("status IN (?)", []string{"active", "connected"}).
		Updates(map[string]interface{}{
			"status":          "offline",
			"disconnected_at": time.Now(),
		})

	if result.Error != nil {
		return pkgerrors.Wrap(result.Error, "failed to cleanup stale tunnels")
	}

	if result.RowsAffected > 0 {
		logger.InfoEvent().
			Int64("count", result.RowsAffected).
			Msg("Marked stale tunnels as offline, domain reservations retained")
	}

	return nil
}

// OnTunnelEvent subscribes to tunnel events.
func (m *Manager) OnTunnelEvent(handler TunnelEventHandler) {
	m.eventMu.Lock()
	defer m.eventMu.Unlock()
	m.eventHandlers = append(m.eventHandlers, handler)
}

// emitEvent emits a tunnel event to all subscribers.
func (m *Manager) emitEvent(event TunnelEvent) {
	m.eventMu.RLock()
	defer m.eventMu.RUnlock()

	// Call all event handlers in goroutines to avoid blocking
	for _, handler := range m.eventHandlers {
		go handler(event)
	}
}

// fullSubdomain format: {custom}-{org} or just {custom} if no org.
func (m *Manager) AllocateSubdomain(ctx context.Context, userID uuid.UUID, orgID *uuid.UUID, requested string) (string, string, error) {
	var orgSubdomain string
	var customPart string

	// If user belongs to organization, fetch org subdomain
	if orgID != nil {
		var org models.Organization
		if err := m.db.WithContext(ctx).Where("id = ? AND is_active = ?", orgID, true).First(&org).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return "", "", fmt.Errorf("organization not found or inactive")
			}
			return "", "", pkgerrors.Wrap(err, "failed to fetch organization")
		}
		orgSubdomain = org.Subdomain
	}

	// Normalize and validate custom subdomain if provided
	if requested != "" {
		customPart = utils.NormalizeSubdomain(requested)

		// Validate custom subdomain
		if !utils.IsValidSubdomain(customPart) {
			return "", "", pkgerrors.ErrInvalidSubdomain
		}
	} else {
		// Generate random custom part
		for attempts := 0; attempts < 10; attempts++ {
			random, err := utils.GenerateRandomSubdomain(8)
			if err != nil {
				continue
			}
			customPart = random
			break
		}

		if customPart == "" {
			return "", "", pkgerrors.ErrSubdomainAllocationFailed
		}
	}

	// Build full subdomain: {custom}-{org} or just {custom}
	var fullSubdomain string
	if orgSubdomain != "" {
		fullSubdomain = fmt.Sprintf("%s-%s", customPart, orgSubdomain)
	} else {
		fullSubdomain = customPart
	}

	// Reserve the full subdomain
	if err := m.reserveSubdomain(ctx, userID, orgID, fullSubdomain); err != nil {
		return "", "", err
	}

	return fullSubdomain, customPart, nil
}

// reserveSubdomain atomically reserves a subdomain in the database with organization support.
func (m *Manager) reserveSubdomain(ctx context.Context, userID uuid.UUID, orgID *uuid.UUID, subdomain string) error {
	// Check if already taken in memory
	if _, exists := m.tunnels.Load(subdomain); exists {
		return pkgerrors.ErrSubdomainTaken
	}

	// Try to create domain record
	domain := &models.Domain{
		UserID:         userID,
		OrganizationID: orgID,
		Subdomain:      subdomain,
	}

	if err := m.db.WithContext(ctx).Create(domain).Error; err != nil {
		// Check if it's a unique constraint violation
		if isDuplicateError(err) {
			return pkgerrors.ErrSubdomainTaken
		}
		return pkgerrors.Wrap(err, "failed to reserve subdomain")
	}

	return nil
}

// RegisterTunnel registers a new active tunnel.
func (m *Manager) RegisterTunnel(ctx context.Context, tunnel *Tunnel) error {
	// Check max tunnels per user
	if err := m.checkUserTunnelLimit(ctx, tunnel.UserID); err != nil {
		return err
	}

	// For TCP tunnels, allocate a port
	var allocatedPort *int
	if tunnel.Protocol == tunnelv1.TunnelProtocol_TCP {
		if m.portPool == nil {
			return pkgerrors.NewAppError("TCP_NOT_SUPPORTED", "TCP tunnels not supported (port pool not initialized)", nil)
		}

		port, err := m.portPool.AllocatePort(tunnel.ID)
		if err != nil {
			logger.ErrorEvent().
				Err(err).
				Str("tunnel_id", tunnel.ID.String()).
				Msg("Failed to allocate TCP port")
			return err
		}

		allocatedPort = &port
		tunnel.RemotePort = &port

		// Update public URL with allocated port
		tunnel.PublicURL = m.BuildTCPPublicURL(port)

		logger.InfoEvent().
			Int("port", port).
			Str("tunnel_id", tunnel.ID.String()).
			Str("public_url", tunnel.PublicURL).
			Msg("Allocated TCP port for tunnel")

		// Start TCP listener if TCP proxy is available
		if m.tcpProxy != nil {
			if err := m.tcpProxy.StartListener(port, tunnel.ID); err != nil {
				// Release the port if listener fails to start
				if releaseErr := m.portPool.ReleasePort(port, false); releaseErr != nil {
					logger.WarnEvent().Err(releaseErr).Int("port", port).Msg("Failed to release port")
				}
				logger.ErrorEvent().
					Err(err).
					Int("port", port).
					Str("tunnel_id", tunnel.ID.String()).
					Msg("Failed to start TCP listener")
				return pkgerrors.Wrap(err, "failed to start TCP listener")
			}
		}
	}

	// Store in memory
	m.tunnels.Store(tunnel.Subdomain, tunnel)
	m.tunnelsByID.Store(tunnel.ID, tunnel)

	// Store in database
	dbTunnel := &models.Tunnel{
		ID:             tunnel.ID,
		UserID:         tunnel.UserID,
		TokenID:        tunnel.TokenID,
		OrganizationID: tunnel.OrganizationID,
		TunnelType:     tunnel.Protocol.String(),
		Subdomain:      tunnel.Subdomain,
		RemotePort:     allocatedPort, // Store allocated TCP port
		LocalAddr:      tunnel.LocalAddr,
		PublicURL:      tunnel.PublicURL,
		ClientID:       tunnel.ID.String(), // Use tunnel ID as unique client ID
		Status:         "active",
		SavedName:      tunnel.SavedName,
		IsPersistent:   tunnel.SavedName != nil, // If has saved name, it's persistent
	}

	if err := m.db.WithContext(ctx).Create(dbTunnel).Error; err != nil {
		// Cleanup memory if DB insert fails
		m.tunnels.Delete(tunnel.Subdomain)
		m.tunnelsByID.Delete(tunnel.ID)

		// Release allocated port if TCP
		if allocatedPort != nil {
			if releaseErr := m.portPool.ReleasePort(*allocatedPort, false); releaseErr != nil {
				logger.WarnEvent().Err(releaseErr).Int("port", *allocatedPort).Msg("Failed to release port")
			}
		}

		return pkgerrors.Wrap(err, "failed to register tunnel in database")
	}

	logger.InfoEvent().
		Str("tunnel_id", tunnel.ID.String()).
		Str("subdomain", tunnel.Subdomain).
		Str("user_id", tunnel.UserID.String()).
		Str("protocol", tunnel.Protocol.String()).
		Msg("Tunnel registered")

	// Emit tunnel connected event
	m.emitEvent(TunnelEvent{
		Type:     EventTunnelConnected,
		TunnelID: tunnel.ID,
		Tunnel:   dbTunnel,
	})

	return nil
}

// All tunnels are now persistent - they are marked offline but not deleted.
func (m *Manager) UnregisterTunnel(ctx context.Context, tunnelID uuid.UUID) error {
	// Load tunnel
	value, ok := m.tunnelsByID.Load(tunnelID)
	if !ok {
		return pkgerrors.ErrTunnelNotFound
	}

	tunnel, ok := value.(*Tunnel)
	if !ok {
		return fmt.Errorf("invalid tunnel type in storage")
	}

	// Close tunnel
	tunnel.Close()

	// Remove from memory
	m.tunnels.Delete(tunnel.Subdomain)
	m.tunnelsByID.Delete(tunnelID)

	// Fetch tunnel from database
	var dbTunnel models.Tunnel
	if err := m.db.WithContext(ctx).Where("id = ?", tunnelID).First(&dbTunnel).Error; err != nil {
		return pkgerrors.Wrap(err, "failed to fetch tunnel from database")
	}

	// Handle TCP port release/reservation
	if dbTunnel.RemotePort != nil && m.portPool != nil {
		isPersistent := dbTunnel.IsPersistent

		// Stop TCP listener
		if m.tcpProxy != nil {
			if err := m.tcpProxy.StopListener(*dbTunnel.RemotePort); err != nil {
				logger.WarnEvent().
					Err(err).
					Int("port", *dbTunnel.RemotePort).
					Str("tunnel_id", tunnelID.String()).
					Msg("Failed to stop TCP listener")
			}
		}

		// Release or keep port based on persistence
		if err := m.portPool.ReleasePort(*dbTunnel.RemotePort, isPersistent); err != nil {
			logger.WarnEvent().
				Err(err).
				Int("port", *dbTunnel.RemotePort).
				Str("tunnel_id", tunnelID.String()).
				Bool("persistent", isPersistent).
				Msg("Failed to release TCP port")
		}

		if isPersistent {
			logger.InfoEvent().
				Int("port", *dbTunnel.RemotePort).
				Str("tunnel_id", tunnelID.String()).
				Msg("TCP port kept reserved for persistent tunnel")
		} else {
			logger.InfoEvent().
				Int("port", *dbTunnel.RemotePort).
				Str("tunnel_id", tunnelID.String()).
				Msg("TCP port released")
		}
	}

	// Mark tunnel as offline, KEEP domain reservation
	err := m.db.WithContext(ctx).
		Model(&models.Tunnel{}).
		Where("id = ?", tunnelID).
		Updates(map[string]interface{}{
			"status":          "offline",
			"disconnected_at": "NOW()",
		}).Error

	if err != nil {
		return pkgerrors.Wrap(err, "failed to update tunnel status")
	}

	savedName := "unnamed"
	if dbTunnel.SavedName != nil {
		savedName = *dbTunnel.SavedName
	}

	logger.InfoEvent().
		Str("tunnel_id", tunnelID.String()).
		Str("subdomain", tunnel.Subdomain).
		Str("saved_name", savedName).
		Msg("Tunnel marked offline, domain reservation retained")

	// Emit tunnel disconnected event
	dbTunnel.Status = "offline"
	m.emitEvent(TunnelEvent{
		Type:     EventTunnelDisconnected,
		TunnelID: tunnelID,
		Tunnel:   &dbTunnel,
	})

	return nil
}

// GetTunnelBySubdomain retrieves a tunnel by subdomain.
func (m *Manager) GetTunnelBySubdomain(subdomain string) (*Tunnel, bool) {
	value, ok := m.tunnels.Load(subdomain)
	if !ok {
		return nil, false
	}
	tunnel, ok := value.(*Tunnel)
	if !ok {
		return nil, false
	}
	return tunnel, true
}

// GetTunnelByID retrieves a tunnel by ID.
func (m *Manager) GetTunnelByID(tunnelID uuid.UUID) (*Tunnel, bool) {
	value, ok := m.tunnelsByID.Load(tunnelID)
	if !ok {
		return nil, false
	}
	tunnel, ok := value.(*Tunnel)
	if !ok {
		return nil, false
	}
	return tunnel, true
}

// SaveTunnelStats saves tunnel statistics to database.
func (m *Manager) SaveTunnelStats(ctx context.Context, tunnelID uuid.UUID) error {
	tunnel, ok := m.GetTunnelByID(tunnelID)
	if !ok {
		return pkgerrors.ErrTunnelNotFound
	}

	// Get current stats
	bytesIn, bytesOut, requestsCount := tunnel.GetStats()

	// Update database
	err := m.db.WithContext(ctx).
		Model(&models.Tunnel{}).
		Where("id = ?", tunnelID).
		Updates(map[string]interface{}{
			"bytes_in":         bytesIn,
			"bytes_out":        bytesOut,
			"requests_count":   requestsCount,
			"last_activity_at": time.Now(),
		}).Error

	if err != nil {
		return pkgerrors.Wrap(err, "failed to save tunnel stats")
	}

	logger.DebugEvent().
		Str("tunnel_id", tunnelID.String()).
		Int64("bytes_in", bytesIn).
		Int64("bytes_out", bytesOut).
		Int64("requests_count", requestsCount).
		Msg("Tunnel stats saved")

	return nil
}

// GetUserTunnels returns all active tunnels for a user.
func (m *Manager) GetUserTunnels(userID uuid.UUID) []*Tunnel {
	var tunnels []*Tunnel

	m.tunnelsByID.Range(func(key, value interface{}) bool {
		tunnel, ok := value.(*Tunnel)
		if !ok {
			return true // Skip invalid entries
		}
		if tunnel.UserID == userID {
			tunnels = append(tunnels, tunnel)
		}
		return true
	})

	return tunnels
}

// CountActiveTunnels returns the total number of active tunnels.
func (m *Manager) CountActiveTunnels() int {
	count := 0
	m.tunnels.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	return count
}

// checkUserTunnelLimit checks if user has reached max tunnel limit.
func (m *Manager) checkUserTunnelLimit(ctx context.Context, userID uuid.UUID) error {
	var count int64
	err := m.db.WithContext(ctx).
		Model(&models.Tunnel{}).
		Where("user_id = ? AND status = ?", userID, "active").
		Count(&count).Error

	if err != nil {
		return pkgerrors.Wrap(err, "failed to count user tunnels")
	}

	if int(count) >= m.maxTunnelsPerUser {
		return pkgerrors.ErrMaxTunnelsReached
	}

	return nil
}

// BuildPublicURL builds the public URL for a tunnel.
func (m *Manager) BuildPublicURL(subdomain string, protocol string) string {
	if protocol == "tcp" {
		// TCP tunnels use port-based routing, not subdomain
		// Return placeholder - actual URL set after port allocation
		return "tcp://pending-allocation"
	}

	// Determine scheme and port based on TLS configuration
	var scheme string
	var port int
	var defaultPort int

	// When TLS is enabled, always use HTTPS for public URLs
	if m.tlsEnabled {
		scheme = "https"
		port = m.httpsPort
		defaultPort = 443
	} else {
		scheme = "http"
		port = m.httpPort
		defaultPort = 80
	}

	// Build URL with or without port
	host := fmt.Sprintf("%s.%s", subdomain, m.baseDomain)
	if port != defaultPort && port != 0 {
		return fmt.Sprintf("%s://%s:%d", scheme, host, port)
	}
	return fmt.Sprintf("%s://%s", scheme, host)
}

// BuildTCPPublicURL builds the public URL for a TCP tunnel with allocated port.
func (m *Manager) BuildTCPPublicURL(port int) string {
	return fmt.Sprintf("tcp://%s:%d", m.baseDomain, port)
}

// IsTLSEnabled returns whether TLS is enabled on the server.
func (m *Manager) IsTLSEnabled() bool {
	return m.tlsEnabled
}

// GetHTTPPort returns the HTTP port.
func (m *Manager) GetHTTPPort() int {
	return m.httpPort
}

// GetHTTPSPort returns the HTTPS port.
func (m *Manager) GetHTTPSPort() int {
	return m.httpsPort
}

// GetBaseDomain returns the base domain.
func (m *Manager) GetBaseDomain() string {
	return m.baseDomain
}

// isDuplicateError checks if the error is a duplicate key error.
func isDuplicateError(err error) bool {
	// PostgreSQL duplicate key error code: 23505
	return err != nil && (err == gorm.ErrDuplicatedKey ||
		// Check error string for duplicate key patterns
		containsAny(err.Error(), []string{
			"duplicate key",
			"already exists",
			"unique constraint",
			"UNIQUE constraint failed",
		}))
}

func containsAny(str string, substrs []string) bool {
	for _, substr := range substrs {
		if len(str) >= len(substr) {
			for i := 0; i <= len(str)-len(substr); i++ {
				if str[i:i+len(substr)] == substr {
					return true
				}
			}
		}
	}
	return false
}

// FindOfflineTunnelBySavedName finds an existing offline tunnel by saved name.
func (m *Manager) FindOfflineTunnelBySavedName(ctx context.Context, userID uuid.UUID, savedName string) (*models.Tunnel, error) {
	var tunnel models.Tunnel
	err := m.db.WithContext(ctx).
		Where("user_id = ? AND saved_name = ? AND is_persistent = ? AND status IN (?)",
			userID, savedName, true, []string{"offline", "disconnected"}).
		Preload("Domain").
		First(&tunnel).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // Not an error, just not found
		}
		return nil, pkgerrors.Wrap(err, "failed to find offline tunnel")
	}

	return &tunnel, nil
}

// ReactivateTunnel reactivates an offline persistent tunnel.
func (m *Manager) ReactivateTunnel(ctx context.Context, offlineTunnel *models.Tunnel, stream grpc.ServerStream, newLocalAddr string) (*Tunnel, error) {
	// Determine protocol from tunnel type
	protocol := "http"
	if offlineTunnel.TunnelType == "HTTPS" {
		protocol = "https"
	} else if offlineTunnel.TunnelType == "TCP" {
		protocol = "tcp"
	}

	// Handle TCP port reallocation for persistent tunnels
	var publicURL string
	var remotePort *int
	if protocol == "tcp" && offlineTunnel.RemotePort != nil {
		// Reallocate the same port for persistent TCP tunnel
		if m.portPool == nil {
			return nil, pkgerrors.NewAppError("TCP_NOT_SUPPORTED", "TCP tunnels not supported (port pool not initialized)", nil)
		}

		port, err := m.portPool.ReallocatePortForTunnel(offlineTunnel.ID, *offlineTunnel.RemotePort)
		if err != nil {
			logger.ErrorEvent().
				Err(err).
				Int("previous_port", *offlineTunnel.RemotePort).
				Str("tunnel_id", offlineTunnel.ID.String()).
				Msg("Failed to reallocate TCP port")
			return nil, pkgerrors.Wrap(err, "failed to reallocate TCP port")
		}

		remotePort = &port
		publicURL = m.BuildTCPPublicURL(port)

		logger.InfoEvent().
			Int("port", port).
			Str("tunnel_id", offlineTunnel.ID.String()).
			Str("public_url", publicURL).
			Msg("Reallocated TCP port for persistent tunnel")

		// Start TCP listener if TCP proxy is available
		if m.tcpProxy != nil {
			if err := m.tcpProxy.StartListener(port, offlineTunnel.ID); err != nil {
				// Release the port if listener fails to start
				if releaseErr := m.portPool.ReleasePort(port, false); releaseErr != nil {
					logger.WarnEvent().Err(releaseErr).Int("port", port).Msg("Failed to release port")
				}
				logger.ErrorEvent().
					Err(err).
					Int("port", port).
					Str("tunnel_id", offlineTunnel.ID.String()).
					Msg("Failed to start TCP listener for reactivated tunnel")
				return nil, pkgerrors.Wrap(err, "failed to start TCP listener")
			}
		}
	} else {
		// Regenerate public URL with current TLS and port configuration for HTTP/HTTPS
		publicURL = m.BuildPublicURL(offlineTunnel.Subdomain, protocol)
	}

	// Update database with new public URL, local address, and active status
	err := m.db.WithContext(ctx).
		Model(&models.Tunnel{}).
		Where("id = ?", offlineTunnel.ID).
		Updates(map[string]interface{}{
			"status":           "active",
			"public_url":       publicURL,    // Update with regenerated URL
			"local_addr":       newLocalAddr, // Update with new local address
			"connected_at":     time.Now(),
			"disconnected_at":  nil,
			"last_activity_at": time.Now(),
		}).Error

	if err != nil {
		return nil, pkgerrors.Wrap(err, "failed to reactivate tunnel")
	}

	// Create in-memory tunnel object with the same ID and subdomain
	// IMPORTANT: Initialize stats from database to preserve cumulative stats across reconnects
	tunnel := &Tunnel{
		ID:             offlineTunnel.ID,
		UserID:         offlineTunnel.UserID,
		TokenID:        offlineTunnel.TokenID,
		OrganizationID: offlineTunnel.OrganizationID,
		Subdomain:      offlineTunnel.Subdomain,
		Protocol:       tunnelv1.TunnelProtocol(tunnelv1.TunnelProtocol_value[offlineTunnel.TunnelType]),
		RemotePort:     remotePort,   // Store remote port for TCP tunnels
		LocalAddr:      newLocalAddr, // Use new local address
		PublicURL:      publicURL,    // Use regenerated URL
		SavedName:      offlineTunnel.SavedName,
		Stream:         stream,
		RequestQueue:   make(chan *PendingRequest, 100),
		ResponseMap:    sync.Map{},
		Status:         "active",
		ConnectedAt:    time.Now(),
		LastActivity:   time.Now(),
		// Preserve cumulative stats from database
		BytesIn:       offlineTunnel.BytesIn,
		BytesOut:      offlineTunnel.BytesOut,
		RequestsCount: offlineTunnel.RequestsCount,
	}

	// Store in memory maps
	m.tunnels.Store(tunnel.Subdomain, tunnel)
	m.tunnelsByID.Store(tunnel.ID, tunnel)

	logger.InfoEvent().
		Str("tunnel_id", tunnel.ID.String()).
		Str("subdomain", tunnel.Subdomain).
		Str("public_url", publicURL).
		Str("local_addr", newLocalAddr).
		Str("saved_name", *offlineTunnel.SavedName).
		Msg("Persistent tunnel reactivated")

	// Emit tunnel connected event with updated public URL and local address
	offlineTunnel.Status = "active"
	offlineTunnel.PublicURL = publicURL    // Update event data with new URL
	offlineTunnel.LocalAddr = newLocalAddr // Update event data with new local addr
	m.emitEvent(TunnelEvent{
		Type:     EventTunnelConnected,
		TunnelID: tunnel.ID,
		Tunnel:   offlineTunnel,
	})

	return tunnel, nil
}

// startPeriodicStatsUpdater runs in background and periodically broadcasts stats updates.
func (m *Manager) startPeriodicStatsUpdater() {
	ticker := time.NewTicker(3 * time.Second) // Update every 3 seconds
	defer ticker.Stop()

	for range ticker.C {
		// Get all active tunnels and broadcast their stats
		m.tunnelsByID.Range(func(key, value interface{}) bool {
			tunnel, ok := value.(*Tunnel)
			if !ok {
				return true // Skip invalid entries
			}

			// Get current stats from in-memory tunnel
			bytesIn, bytesOut, requestsCount := tunnel.GetStats()

			// Load tunnel from database to get full data
			var dbTunnel models.Tunnel
			if err := m.db.Where("id = ?", tunnel.ID).First(&dbTunnel).Error; err != nil {
				return true // Continue to next tunnel
			}

			// Update database with latest stats
			err := m.db.Model(&models.Tunnel{}).
				Where("id = ?", tunnel.ID).
				Updates(map[string]interface{}{
					"bytes_in":         bytesIn,
					"bytes_out":        bytesOut,
					"requests_count":   requestsCount,
					"last_activity_at": time.Now(),
				}).Error

			if err != nil {
				logger.WarnEvent().
					Err(err).
					Str("tunnel_id", tunnel.ID.String()).
					Msg("Failed to update tunnel stats")
				return true // Continue to next tunnel
			}

			// Update dbTunnel with new stats for event
			dbTunnel.BytesIn = bytesIn
			dbTunnel.BytesOut = bytesOut
			dbTunnel.RequestsCount = requestsCount
			dbTunnel.LastActivityAt = time.Now()

			// Emit stats update event
			m.emitEvent(TunnelEvent{
				Type:     EventTunnelStatsUpdated,
				TunnelID: tunnel.ID,
				Tunnel:   &dbTunnel,
			})

			return true // Continue to next tunnel
		})
	}
}
