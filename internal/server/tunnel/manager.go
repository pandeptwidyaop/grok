package tunnel

import (
	"context"
	"fmt"
	"sync"

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
	return &Manager{
		db:             db,
		baseDomain:     baseDomain,
		maxTunnelsPerUser: maxTunnelsPerUser,
	}
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

	// Update database
	err := m.db.WithContext(ctx).
		Model(&models.Tunnel{}).
		Where("id = ?", tunnelID).
		Updates(map[string]interface{}{
			"status":          "disconnected",
			"disconnected_at": "NOW()",
		}).Error

	if err != nil {
		return pkgerrors.Wrap(err, "failed to update tunnel status")
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
