package dashboard

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/pandeptwidyaop/grok/pkg/logger"
)

// SSEEvent represents a server-sent event.
type SSEEvent struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// SSEClient represents a connected SSE client.
type SSEClient struct {
	ID       string
	Channel  chan SSEEvent
	LastSeen time.Time
}

// SSEBroker manages SSE connections and broadcasts events.
type SSEBroker struct {
	clients    map[string]*SSEClient
	clientsMu  sync.RWMutex
	register   chan *SSEClient
	unregister chan *SSEClient
	broadcast  chan SSEEvent
	done       chan struct{}
}

// NewSSEBroker creates a new SSE broker.
func NewSSEBroker() *SSEBroker {
	broker := &SSEBroker{
		clients:    make(map[string]*SSEClient),
		register:   make(chan *SSEClient),
		unregister: make(chan *SSEClient),
		broadcast:  make(chan SSEEvent, 100), // Buffered channel
		done:       make(chan struct{}),
	}

	// Start background goroutine
	go broker.run()

	// Start cleanup goroutine
	go broker.cleanupStaleClients()

	return broker
}

// run processes registration, unregistration, and broadcasts.
func (b *SSEBroker) run() {
	for {
		select {
		case client := <-b.register:
			b.clientsMu.Lock()
			b.clients[client.ID] = client
			b.clientsMu.Unlock()

			logger.DebugEvent().
				Str("client_id", client.ID).
				Msg("SSE client registered")

		case client := <-b.unregister:
			b.clientsMu.Lock()
			if _, ok := b.clients[client.ID]; ok {
				delete(b.clients, client.ID)
				close(client.Channel)

				logger.DebugEvent().
					Str("client_id", client.ID).
					Msg("SSE client unregistered")
			}
			b.clientsMu.Unlock()

		case event := <-b.broadcast:
			b.clientsMu.RLock()
			for _, client := range b.clients {
				select {
				case client.Channel <- event:
					client.LastSeen = time.Now()
				default:
					// Client channel full, skip
					logger.WarnEvent().
						Str("client_id", client.ID).
						Msg("SSE client channel full, skipping event")
				}
			}
			b.clientsMu.RUnlock()

		case <-b.done:
			return
		}
	}
}

// cleanupStaleClients removes clients that haven't been active for 5 minutes.
func (b *SSEBroker) cleanupStaleClients() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			now := time.Now()
			b.clientsMu.Lock()

			for id, client := range b.clients {
				if now.Sub(client.LastSeen) > 5*time.Minute {
					delete(b.clients, id)
					close(client.Channel)

					logger.InfoEvent().
						Str("client_id", id).
						Msg("SSE client removed (stale)")
				}
			}

			b.clientsMu.Unlock()

		case <-b.done:
			return
		}
	}
}

// Broadcast sends an event to all connected clients.
func (b *SSEBroker) Broadcast(event SSEEvent) {
	select {
	case b.broadcast <- event:
		// Event queued successfully
	default:
		// Broadcast channel full, log warning
		logger.WarnEvent().
			Str("event_type", event.Type).
			Msg("SSE broadcast channel full, dropping event")
	}
}

// ClientCount returns the number of connected clients.
func (b *SSEBroker) ClientCount() int {
	b.clientsMu.RLock()
	defer b.clientsMu.RUnlock()
	return len(b.clients)
}

// Close stops the broker and closes all client connections.
func (b *SSEBroker) Close() {
	close(b.done)

	b.clientsMu.Lock()
	defer b.clientsMu.Unlock()

	for _, client := range b.clients {
		close(client.Channel)
	}

	b.clients = make(map[string]*SSEClient)
}

// HandleSSE handles SSE connections from clients.
func (s *Server) HandleSSE(w http.ResponseWriter, r *http.Request) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Create client
	clientID := fmt.Sprintf("client-%d", time.Now().UnixNano())
	client := &SSEClient{
		ID:       clientID,
		Channel:  make(chan SSEEvent, 10),
		LastSeen: time.Now(),
	}

	// Register client
	s.sseBroker.register <- client

	// Ensure unregistration on disconnect
	defer func() {
		s.sseBroker.unregister <- client
	}()

	// Create flusher for streaming
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	logger.InfoEvent().
		Str("client_id", clientID).
		Str("remote_addr", r.RemoteAddr).
		Msg("SSE client connected")

	// Send initial connection event
	if err := s.sendSSEEvent(w, flusher, SSEEvent{
		Type: "connected",
		Data: map[string]string{
			"message": "Connected to dashboard",
		},
	}); err != nil {
		logger.WarnEvent().Err(err).Msg("Failed to send initial connection event")
		return
	}

	// Keep-alive ticker
	keepAliveTicker := time.NewTicker(15 * time.Second)
	defer keepAliveTicker.Stop()

	// Stream events
	for {
		select {
		case event := <-client.Channel:
			if err := s.sendSSEEvent(w, flusher, event); err != nil {
				logger.WarnEvent().
					Err(err).
					Str("client_id", clientID).
					Msg("Failed to send SSE event, client disconnected")
				return
			}

		case <-keepAliveTicker.C:
			// Send keep-alive comment
			fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()

		case <-r.Context().Done():
			logger.InfoEvent().
				Str("client_id", clientID).
				Msg("SSE client disconnected")
			return
		}
	}
}

// sendSSEEvent writes an SSE event to the response writer.
func (s *Server) sendSSEEvent(w http.ResponseWriter, flusher http.Flusher, event SSEEvent) error {
	// Marshal data to JSON
	jsonData, err := json.Marshal(event.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	// Write SSE format: event: type\ndata: json\n\n
	fmt.Fprintf(w, "event: %s\n", event.Type)
	fmt.Fprintf(w, "data: %s\n\n", jsonData)

	flusher.Flush()

	return nil
}
