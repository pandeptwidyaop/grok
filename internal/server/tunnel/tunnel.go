package tunnel

import (
	"sync"
	"time"

	"github.com/google/uuid"
	tunnelv1 "github.com/pandeptwidyaop/grok/gen/proto/tunnel/v1"
	"google.golang.org/grpc"
)

// Tunnel represents an active tunnel connection
type Tunnel struct {
	ID            uuid.UUID
	UserID        uuid.UUID
	TokenID       uuid.UUID
	Subdomain     string
	Protocol      tunnelv1.TunnelProtocol
	LocalAddr     string
	PublicURL     string
	Stream        grpc.ServerStream
	RequestQueue  chan *PendingRequest
	ResponseMap   sync.Map // request_id → response channel
	Status        string
	ConnectedAt   time.Time
	LastActivity  time.Time
	mu            sync.RWMutex
}

// PendingRequest represents a request waiting for response
type PendingRequest struct {
	RequestID  string
	Request    *tunnelv1.ProxyRequest
	ResponseCh chan *tunnelv1.ProxyResponse
	Timeout    time.Duration
	CreatedAt  time.Time
}

// NewTunnel creates a new tunnel instance
func NewTunnel(
	userID, tokenID uuid.UUID,
	subdomain string,
	protocol tunnelv1.TunnelProtocol,
	localAddr string,
	publicURL string,
	stream grpc.ServerStream,
) *Tunnel {
	return &Tunnel{
		ID:           uuid.New(),
		UserID:       userID,
		TokenID:      tokenID,
		Subdomain:    subdomain,
		Protocol:     protocol,
		LocalAddr:    localAddr,
		PublicURL:    publicURL,
		Stream:       stream,
		RequestQueue: make(chan *PendingRequest, 100),
		Status:       "active",
		ConnectedAt:  time.Now(),
		LastActivity: time.Now(),
	}
}

// UpdateActivity updates the last activity timestamp
func (t *Tunnel) UpdateActivity() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.LastActivity = time.Now()
}

// GetStatus returns the current tunnel status
func (t *Tunnel) GetStatus() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.Status
}

// SetStatus updates the tunnel status
func (t *Tunnel) SetStatus(status string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Status = status
}

// Close closes the tunnel and cleans up resources
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
