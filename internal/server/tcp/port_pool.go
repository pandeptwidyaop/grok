package tcp

import (
	"fmt"
	"sync"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/pandeptwidyaop/grok/internal/db/models"
	pkgerrors "github.com/pandeptwidyaop/grok/pkg/errors"
	"github.com/pandeptwidyaop/grok/pkg/logger"
)

// PortPool manages allocation of TCP ports for tunnels.
type PortPool struct {
	startPort      int               // Start of port range (e.g., 10000)
	endPort        int               // End of port range (e.g., 20000)
	allocatedPorts map[int]uuid.UUID // port → tunnel_id mapping
	availablePorts []int             // Queue of available ports
	mu             sync.RWMutex      // Protects allocatedPorts and availablePorts
	db             *gorm.DB          // Database connection for persistence
}

// NewPortPool creates a new port pool manager.
func NewPortPool(db *gorm.DB, startPort, endPort int) (*PortPool, error) {
	if startPort < 1024 {
		return nil, fmt.Errorf("start port must be >= 1024 (avoiding privileged ports)")
	}
	if endPort <= startPort {
		return nil, fmt.Errorf("end port must be greater than start port")
	}
	if endPort > 65535 {
		return nil, fmt.Errorf("end port must be <= 65535")
	}

	pp := &PortPool{
		startPort:      startPort,
		endPort:        endPort,
		allocatedPorts: make(map[int]uuid.UUID),
		availablePorts: make([]int, 0, endPort-startPort+1),
		db:             db,
	}

	// Initialize available ports queue
	for port := startPort; port <= endPort; port++ {
		pp.availablePorts = append(pp.availablePorts, port)
	}

	// Load existing port allocations from database
	if err := pp.loadAllocatedPorts(); err != nil {
		return nil, fmt.Errorf("failed to load allocated ports: %w", err)
	}

	logger.InfoEvent().
		Int("start_port", startPort).
		Int("end_port", endPort).
		Int("total_ports", endPort-startPort+1).
		Int("allocated_ports", len(pp.allocatedPorts)).
		Int("available_ports", len(pp.availablePorts)).
		Msg("Port pool initialized")

	return pp, nil
}

// loadAllocatedPorts loads port allocations from database on startup.
func (pp *PortPool) loadAllocatedPorts() error {
	var tunnels []models.Tunnel

	// Get all tunnels with allocated ports (both active and offline persistent tunnels)
	err := pp.db.Where("remote_port IS NOT NULL AND (status = ? OR (is_persistent = ? AND status = ?))",
		"active", true, "offline").
		Find(&tunnels).Error

	if err != nil {
		return err
	}

	// Mark ports as allocated and remove from available queue
	for _, tunnel := range tunnels {
		if tunnel.RemotePort != nil {
			port := *tunnel.RemotePort

			// Mark as allocated
			pp.allocatedPorts[port] = tunnel.ID

			// Remove from available ports
			pp.removeFromAvailable(port)

			logger.DebugEvent().
				Int("port", port).
				Str("tunnel_id", tunnel.ID.String()).
				Str("status", tunnel.Status).
				Bool("persistent", tunnel.IsPersistent).
				Msg("Restored port allocation from database")
		}
	}

	return nil
}

// removeFromAvailable removes a port from the available ports queue.
func (pp *PortPool) removeFromAvailable(port int) {
	for i, p := range pp.availablePorts {
		if p == port {
			// Remove by swapping with last element
			pp.availablePorts[i] = pp.availablePorts[len(pp.availablePorts)-1]
			pp.availablePorts = pp.availablePorts[:len(pp.availablePorts)-1]
			break
		}
	}
}

// AllocatePort allocates a port for the given tunnel.
func (pp *PortPool) AllocatePort(tunnelID uuid.UUID) (int, error) {
	pp.mu.Lock()
	defer pp.mu.Unlock()

	// Check if tunnel already has a port allocated (shouldn't happen, but safety check)
	for port, tid := range pp.allocatedPorts {
		if tid == tunnelID {
			logger.WarnEvent().
				Int("port", port).
				Str("tunnel_id", tunnelID.String()).
				Msg("Tunnel already has port allocated")
			return port, nil
		}
	}

	// Check if any ports available
	if len(pp.availablePorts) == 0 {
		return 0, pkgerrors.ErrNoAvailablePorts
	}

	// Allocate first available port
	port := pp.availablePorts[0]
	pp.availablePorts = pp.availablePorts[1:]

	// Mark as allocated
	pp.allocatedPorts[port] = tunnelID

	logger.InfoEvent().
		Int("port", port).
		Str("tunnel_id", tunnelID.String()).
		Int("remaining_ports", len(pp.availablePorts)).
		Msg("Port allocated")

	return port, nil
}

// If tunnel is persistent and status is offline, keep the port reserved.
func (pp *PortPool) ReleasePort(port int, isPersistent bool) error {
	pp.mu.Lock()
	defer pp.mu.Unlock()

	// Check if port is in our range
	if port < pp.startPort || port > pp.endPort {
		return fmt.Errorf("port %d is outside managed range [%d-%d]", port, pp.startPort, pp.endPort)
	}

	tunnelID, exists := pp.allocatedPorts[port]
	if !exists {
		logger.WarnEvent().
			Int("port", port).
			Msg("Attempted to release unallocated port")
		return nil // Not an error, just a no-op
	}

	// For persistent tunnels, keep port allocated but mark tunnel as offline
	// The port will be reused when the same tunnel reconnects
	if isPersistent {
		logger.InfoEvent().
			Int("port", port).
			Str("tunnel_id", tunnelID.String()).
			Msg("Port kept reserved for persistent tunnel")
		return nil
	}

	// For non-persistent tunnels, fully release the port
	delete(pp.allocatedPorts, port)

	// Add back to available ports
	pp.availablePorts = append(pp.availablePorts, port)

	logger.InfoEvent().
		Int("port", port).
		Str("tunnel_id", tunnelID.String()).
		Int("available_ports", len(pp.availablePorts)).
		Msg("Port released")

	return nil
}

// GetAllocatedPort returns the port allocated to a tunnel, if any.
func (pp *PortPool) GetAllocatedPort(tunnelID uuid.UUID) (int, bool) {
	pp.mu.RLock()
	defer pp.mu.RUnlock()

	for port, tid := range pp.allocatedPorts {
		if tid == tunnelID {
			return port, true
		}
	}

	return 0, false
}

// GetTunnelByPort returns the tunnel ID allocated to a port, if any.
func (pp *PortPool) GetTunnelByPort(port int) (uuid.UUID, bool) {
	pp.mu.RLock()
	defer pp.mu.RUnlock()

	tunnelID, exists := pp.allocatedPorts[port]
	return tunnelID, exists
}

// IsPortAvailable checks if a port is available for allocation.
func (pp *PortPool) IsPortAvailable(port int) bool {
	pp.mu.RLock()
	defer pp.mu.RUnlock()

	// Check if port is in range
	if port < pp.startPort || port > pp.endPort {
		return false
	}

	// Check if not allocated
	_, allocated := pp.allocatedPorts[port]
	return !allocated
}

// GetStats returns statistics about the port pool.
func (pp *PortPool) GetStats() map[string]interface{} {
	pp.mu.RLock()
	defer pp.mu.RUnlock()

	totalPorts := pp.endPort - pp.startPort + 1
	allocatedCount := len(pp.allocatedPorts)
	availableCount := len(pp.availablePorts)

	return map[string]interface{}{
		"start_port":      pp.startPort,
		"end_port":        pp.endPort,
		"total_ports":     totalPorts,
		"allocated_ports": allocatedCount,
		"available_ports": availableCount,
		"utilization":     float64(allocatedCount) / float64(totalPorts) * 100,
	}
}

// ReallocatePortForTunnel reallocates the same port for a persistent tunnel on reconnection.
func (pp *PortPool) ReallocatePortForTunnel(tunnelID uuid.UUID, previousPort int) (int, error) {
	pp.mu.Lock()
	defer pp.mu.Unlock()

	// Check if the previous port is still allocated to this tunnel
	if allocatedTunnelID, exists := pp.allocatedPorts[previousPort]; exists {
		if allocatedTunnelID == tunnelID {
			logger.InfoEvent().
				Int("port", previousPort).
				Str("tunnel_id", tunnelID.String()).
				Msg("Reusing existing port allocation for persistent tunnel")
			return previousPort, nil
		}

		// Port allocated to different tunnel - this shouldn't happen
		logger.ErrorEvent().
			Int("port", previousPort).
			Str("requested_tunnel_id", tunnelID.String()).
			Str("allocated_tunnel_id", allocatedTunnelID.String()).
			Msg("Port already allocated to different tunnel")
		return 0, fmt.Errorf("port %d already allocated to different tunnel", previousPort)
	}

	// Port not currently allocated, allocate it
	pp.allocatedPorts[previousPort] = tunnelID

	// Remove from available ports if present
	pp.removeFromAvailable(previousPort)

	logger.InfoEvent().
		Int("port", previousPort).
		Str("tunnel_id", tunnelID.String()).
		Msg("Reallocated port for persistent tunnel")

	return previousPort, nil
}
