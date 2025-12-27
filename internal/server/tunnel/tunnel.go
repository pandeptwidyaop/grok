package tunnel

import (
	"sync"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"

	tunnelv1 "github.com/pandeptwidyaop/grok/gen/proto/tunnel/v1"
)

// Tunnel represents an active tunnel connection.
type Tunnel struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	TokenID        uuid.UUID
	OrganizationID *uuid.UUID // Organization ID (nullable)
	Subdomain      string
	Protocol       tunnelv1.TunnelProtocol
	RemotePort     *int // Allocated port for TCP tunnels
	LocalAddr      string
	PublicURL      string
	Stream         grpc.ServerStream
	RequestQueue   chan *PendingRequest
	ResponseMap    sync.Map // request_id → response channel
	Status         string
	ConnectedAt    time.Time
	LastActivity   time.Time

	// Webhook-specific fields
	WebhookAppID *uuid.UUID // Webhook app ID if this is a webhook tunnel
	IsWebhook    bool       // True if this tunnel is registered for webhook routing

	// Persistent tunnel fields
	SavedName *string // Optional saved name for persistent tunnels

	// Statistics (in-memory counters)
	BytesIn       int64
	BytesOut      int64
	RequestsCount int64

	mu sync.RWMutex
}

// PendingRequest represents a request waiting for response.
type PendingRequest struct {
	RequestID  string
	Request    *tunnelv1.ProxyRequest
	ResponseCh chan *tunnelv1.ProxyResponse
	Timeout    time.Duration
	CreatedAt  time.Time
}

// NewTunnel creates a new tunnel instance.
func NewTunnel(
	userID, tokenID uuid.UUID,
	organizationID *uuid.UUID,
	subdomain string,
	protocol tunnelv1.TunnelProtocol,
	localAddr string,
	publicURL string,
	stream grpc.ServerStream,
) *Tunnel {
	return &Tunnel{
		ID:             uuid.New(),
		UserID:         userID,
		TokenID:        tokenID,
		OrganizationID: organizationID,
		Subdomain:      subdomain,
		Protocol:       protocol,
		LocalAddr:      localAddr,
		PublicURL:      publicURL,
		Stream:         stream,
		RequestQueue:   make(chan *PendingRequest, 100),
		Status:         "active",
		ConnectedAt:    time.Now(),
		LastActivity:   time.Now(),
	}
}

// UpdateActivity updates the last activity timestamp.
func (t *Tunnel) UpdateActivity() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.LastActivity = time.Now()
}

// GetStatus returns the current tunnel status.
func (t *Tunnel) GetStatus() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.Status
}

// SetStatus updates the tunnel status.
func (t *Tunnel) SetStatus(status string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Status = status
}

// UpdateStats updates tunnel statistics (thread-safe).
func (t *Tunnel) UpdateStats(bytesIn, bytesOut int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.BytesIn += bytesIn
	t.BytesOut += bytesOut
	t.RequestsCount++
	t.LastActivity = time.Now()
}

// GetStats returns current statistics (thread-safe).
func (t *Tunnel) GetStats() (bytesIn, bytesOut, requestsCount int64) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.BytesIn, t.BytesOut, t.RequestsCount
}

// Close closes the tunnel and cleans up resources.
func (t *Tunnel) Close() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.Status = "closed"
	close(t.RequestQueue)

	// Clean up pending requests
	t.ResponseMap.Range(func(key, value interface{}) bool {
		if ch, ok := value.(chan *tunnelv1.ProxyResponse); ok {
			close(ch)
		}
		t.ResponseMap.Delete(key)
		return true
	})
}
