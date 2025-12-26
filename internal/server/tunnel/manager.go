package tunnel

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pandeptwidyaop/grok/internal/db/models"
	pkgerrors "github.com/pandeptwidyaop/grok/pkg/errors"
	"github.com/pandeptwidyaop/grok/pkg/logger"
	"github.com/pandeptwidyaop/grok/pkg/utils"
	"gorm.io/gorm"
)

// Manager manages active tunnels
type Manager struct {
	db             *gorm.DB
	tunnels        sync.Map // subdomain → *Tunnel
	tunnelsByID    sync.Map // tunnel_id → *Tunnel
	maxTunnelsPerUser int
	baseDomain     string
	mu             sync.RWMutex
}

// NewManager creates a new tunnel manager
func NewManager(db *gorm.DB, baseDomain string, maxTunnelsPerUser int) *Manager {
	m := &Manager{
		db:             db,
		baseDomain:     baseDomain,
		maxTunnelsPerUser: maxTunnelsPerUser,
	}

	// Cleanup stale tunnels from previous server runs
	if err := m.CleanupStaleTunnels(context.Background()); err != nil {
		logger.WarnEvent().
			Err(err).
			Msg("Failed to cleanup stale tunnels on startup")
	}

	return m
}

// CleanupStaleTunnels cleans up tunnels that are marked as active but have no active connection
// This happens when server restarts and old tunnel records remain in database
func (m *Manager) CleanupStaleTunnels(ctx context.Context) error {
	logger.InfoEvent().Msg("Cleaning up stale tunnels from previous sessions...")

	// Update all active/connected tunnels to disconnected
	result := m.db.WithContext(ctx).
		Model(&models.Tunnel{}).
		Where("status IN (?)", []string{"active", "connected"}).
		Updates(map[string]interface{}{
			"status":          "disconnected",
			"disconnected_at": time.Now(),
		})

	if result.Error != nil {
		return pkgerrors.Wrap(result.Error, "failed to cleanup stale tunnels")
	}

	if result.RowsAffected > 0 {
		logger.InfoEvent().
			Int64("count", result.RowsAffected).
			Msg("Cleaned up stale tunnels")
	}

	// Also cleanup orphaned domain reservations
	// (domains without any active tunnel using them)
	subResult := m.db.WithContext(ctx).Exec(`
		DELETE FROM domains
		WHERE id NOT IN (
			SELECT domain_id FROM tunnels
			WHERE domain_id IS NOT NULL
			AND status = 'active'
		)
	`)

	if subResult.Error != nil {
		logger.WarnEvent().
			Err(subResult.Error).
			Msg("Failed to cleanup orphaned domains")
	} else if subResult.RowsAffected > 0 {
		logger.InfoEvent().
			Int64("count", subResult.RowsAffected).
			Msg("Cleaned up orphaned domain reservations")
	}

	return nil
}

// AllocateSubdomain allocates a subdomain (custom or random)
func (m *Manager) AllocateSubdomain(ctx context.Context, userID uuid.UUID, requested string) (string, error) {
	// Normalize subdomain if provided
	if requested != "" {
		requested = utils.NormalizeSubdomain(requested)

		// Validate custom subdomain
		if !utils.IsValidSubdomain(requested) {
			return "", pkgerrors.ErrInvalidSubdomain
		}

		// Check if subdomain is already taken
		if err := m.reserveSubdomain(ctx, userID, requested); err != nil {
			return "", err
		}

		return requested, nil
	}

	// Generate random subdomain
	for attempts := 0; attempts < 10; attempts++ {
		subdomain, err := utils.GenerateRandomSubdomain(8)
		if err != nil {
			continue
		}

		if err := m.reserveSubdomain(ctx, userID, subdomain); err == nil {
			return subdomain, nil
		}
	}

	return "", pkgerrors.ErrSubdomainAllocationFailed
}

// reserveSubdomain atomically reserves a subdomain in the database
func (m *Manager) reserveSubdomain(ctx context.Context, userID uuid.UUID, subdomain string) error {
	// Check if already taken in memory
	if _, exists := m.tunnels.Load(subdomain); exists {
		return pkgerrors.ErrSubdomainTaken
	}

	// Try to create domain record
	domain := &models.Domain{
		UserID:    userID,
		Subdomain: subdomain,
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

// RegisterTunnel registers a new active tunnel
func (m *Manager) RegisterTunnel(ctx context.Context, tunnel *Tunnel) error {
	// Check max tunnels per user
	if err := m.checkUserTunnelLimit(ctx, tunnel.UserID); err != nil {
		return err
	}

	// Store in memory
	m.tunnels.Store(tunnel.Subdomain, tunnel)
	m.tunnelsByID.Store(tunnel.ID, tunnel)

	// Store in database
	dbTunnel := &models.Tunnel{
		ID:         tunnel.ID,
		UserID:     tunnel.UserID,
		TokenID:    tunnel.TokenID,
		TunnelType: tunnel.Protocol.String(),
		Subdomain:  tunnel.Subdomain,
		LocalAddr:  tunnel.LocalAddr,
		PublicURL:  tunnel.PublicURL,
		ClientID:   tunnel.ID.String(), // Use tunnel ID as unique client ID
		Status:     "active",
	}

	if err := m.db.WithContext(ctx).Create(dbTunnel).Error; err != nil {
		// Cleanup memory if DB insert fails
		m.tunnels.Delete(tunnel.Subdomain)
		m.tunnelsByID.Delete(tunnel.ID)
		return pkgerrors.Wrap(err, "failed to register tunnel in database")
	}

	logger.InfoEvent().
		Str("tunnel_id", tunnel.ID.String()).
		Str("subdomain", tunnel.Subdomain).
		Str("user_id", tunnel.UserID.String()).
		Msg("Tunnel registered")

	return nil
}

// UnregisterTunnel unregisters an active tunnel
func (m *Manager) UnregisterTunnel(ctx context.Context, tunnelID uuid.UUID) error {
	// Load tunnel
	value, ok := m.tunnelsByID.Load(tunnelID)
	if !ok {
		return pkgerrors.ErrTunnelNotFound
	}

	tunnel := value.(*Tunnel)

	// Close tunnel
	tunnel.Close()

	// Remove from memory
	m.tunnels.Delete(tunnel.Subdomain)
	m.tunnelsByID.Delete(tunnelID)

	// Update database - clear domain_id to break FK reference
	err := m.db.WithContext(ctx).
		Model(&models.Tunnel{}).
		Where("id = ?", tunnelID).
		Updates(map[string]interface{}{
			"status":          "disconnected",
			"disconnected_at": "NOW()",
			"domain_id":       nil, // Clear domain reference before deleting domain
		}).Error

	if err != nil {
		return pkgerrors.Wrap(err, "failed to update tunnel status")
	}

	// Delete domain reservation to allow reuse
	logger.InfoEvent().
		Str("subdomain", tunnel.Subdomain).
		Msg("Attempting to delete domain reservation")

	result := m.db.WithContext(ctx).
		Where("subdomain = ?", tunnel.Subdomain).
		Delete(&models.Domain{})

	if result.Error != nil {
		logger.WarnEvent().
			Err(result.Error).
			Str("subdomain", tunnel.Subdomain).
			Msg("Failed to delete domain reservation")
	} else {
		logger.InfoEvent().
			Str("subdomain", tunnel.Subdomain).
			Int64("rows_affected", result.RowsAffected).
			Msg("Domain reservation deleted successfully")
	}

	logger.InfoEvent().
		Str("tunnel_id", tunnelID.String()).
		Str("subdomain", tunnel.Subdomain).
		Msg("Tunnel unregistered")

	return nil
}

// GetTunnelBySubdomain retrieves a tunnel by subdomain
func (m *Manager) GetTunnelBySubdomain(subdomain string) (*Tunnel, bool) {
	value, ok := m.tunnels.Load(subdomain)
	if !ok {
		return nil, false
	}
	return value.(*Tunnel), true
}

// GetTunnelByID retrieves a tunnel by ID
func (m *Manager) GetTunnelByID(tunnelID uuid.UUID) (*Tunnel, bool) {
	value, ok := m.tunnelsByID.Load(tunnelID)
	if !ok {
		return nil, false
	}
	return value.(*Tunnel), true
}

// SaveTunnelStats saves tunnel statistics to database
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

// GetUserTunnels returns all active tunnels for a user
func (m *Manager) GetUserTunnels(userID uuid.UUID) []*Tunnel {
	var tunnels []*Tunnel

	m.tunnelsByID.Range(func(key, value interface{}) bool {
		tunnel := value.(*Tunnel)
		if tunnel.UserID == userID {
			tunnels = append(tunnels, tunnel)
		}
		return true
	})

	return tunnels
}

// CountActiveTunnels returns the total number of active tunnels
func (m *Manager) CountActiveTunnels() int {
	count := 0
	m.tunnels.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	return count
}

// checkUserTunnelLimit checks if user has reached max tunnel limit
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

// BuildPublicURL builds the public URL for a tunnel
func (m *Manager) BuildPublicURL(subdomain string, protocol string) string {
	if protocol == "tcp" {
		return fmt.Sprintf("tcp://%s.%s", subdomain, m.baseDomain)
	}
	return fmt.Sprintf("https://%s.%s", subdomain, m.baseDomain)
}

// isDuplicateError checks if the error is a duplicate key error
func isDuplicateError(err error) bool {
	// PostgreSQL duplicate key error code: 23505
	return err != nil && (
		err == gorm.ErrDuplicatedKey ||
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
