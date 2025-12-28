package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/pandeptwidyaop/grok/pkg/logger"
)

// SSEEvent represents a server-sent event
type SSEEvent struct {
	Type string      `json:"type"` // event type: tunnel_created, stats_update, etc
	Data interface{} `json:"data"` // event payload
}

// SSEClient represents a connected SSE client
type SSEClient struct {
	ID       string
	Channel  chan SSEEvent
	LastSeen time.Time
}

// SSEBroker manages SSE connections and broadcasts events
type SSEBroker struct {
	clients     map[string]*SSEClient
	clientsMu   sync.RWMutex
	register    chan *SSEClient
	unregister  chan *SSEClient
	broadcast   chan SSEEvent
	done        chan struct{}
	wg          sync.WaitGroup // Wait for run() goroutine to exit
	sseLogLevel string          // SSE connection log level: silent, warn, info
}

// NewSSEBroker creates a new SSE broker
func NewSSEBroker(sseLogLevel string) *SSEBroker {
	broker := &SSEBroker{
		clients:     make(map[string]*SSEClient),
		register:    make(chan *SSEClient),
		unregister:  make(chan *SSEClient),
		broadcast:   make(chan SSEEvent, 100), // Buffer for 100 events
		done:        make(chan struct{}),
		sseLogLevel: sseLogLevel,
	}

	broker.wg.Add(1)
	go broker.run()
	return broker
}

// run handles client registration and event broadcasting
func (b *SSEBroker) run() {
	defer b.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case client := <-b.register:
			b.clientsMu.Lock()
			b.clients[client.ID] = client
			b.clientsMu.Unlock()
			// Only log if level is info
			if b.sseLogLevel == "info" {
				logger.InfoEvent().
					Str("client_id", client.ID).
					Int("total_clients", len(b.clients)).
					Msg("SSE client connected")
			}

		case client := <-b.unregister:
			b.clientsMu.Lock()
			if _, ok := b.clients[client.ID]; ok {
				delete(b.clients, client.ID)
				close(client.Channel)
			}
			b.clientsMu.Unlock()
			// Log disconnects at warn level or higher
			if b.sseLogLevel == "warn" || b.sseLogLevel == "info" {
				logger.InfoEvent().
					Str("client_id", client.ID).
					Int("total_clients", len(b.clients)).
					Msg("SSE client disconnected")
			}

		case event := <-b.broadcast:
			b.clientsMu.RLock()
			for _, client := range b.clients {
				select {
				case client.Channel <- event:
					client.LastSeen = time.Now()
				default:
					// Client is slow or disconnected, skip
					logger.WarnEvent().
						Str("client_id", client.ID).
						Str("event_type", event.Type).
						Msg("SSE client slow, skipping event")
				}
			}
			b.clientsMu.RUnlock()

		case <-ticker.C:
			// Cleanup stale clients (no activity for 5 minutes)
			b.cleanupStaleClients()

		case <-b.done:
			return
		}
	}
}

// cleanupStaleClients removes clients that haven't received events in 5 minutes
func (b *SSEBroker) cleanupStaleClients() {
	b.clientsMu.Lock()
	defer b.clientsMu.Unlock()

	staleTimeout := 5 * time.Minute
	now := time.Now()

	for id, client := range b.clients {
		if now.Sub(client.LastSeen) > staleTimeout {
			logger.WarnEvent().
				Str("client_id", id).
				Dur("inactive_duration", now.Sub(client.LastSeen)).
				Msg("Removing stale SSE client")
			delete(b.clients, id)
			close(client.Channel)
		}
	}
}

// RegisterClient registers a new SSE client
func (b *SSEBroker) RegisterClient(id string) *SSEClient {
	client := &SSEClient{
		ID:       id,
		Channel:  make(chan SSEEvent, 10), // Buffer 10 events per client
		LastSeen: time.Now(),
	}
	b.register <- client
	return client
}

// UnregisterClient unregisters an SSE client
func (b *SSEBroker) UnregisterClient(client *SSEClient) {
	b.unregister <- client
}

// Broadcast sends an event to all connected clients
func (b *SSEBroker) Broadcast(event SSEEvent) {
	select {
	case b.broadcast <- event:
		clientCount := b.GetClientCount()
		logger.DebugEvent().
			Str("event_type", event.Type).
			Int("clients_count", clientCount).
			Msg("Broadcasting SSE event")
	default:
		logger.WarnEvent().
			Str("event_type", event.Type).
			Msg("SSE broadcast channel full, dropping event")
	}
}

// GetClientCount returns the number of connected clients
func (b *SSEBroker) GetClientCount() int {
	b.clientsMu.RLock()
	defer b.clientsMu.RUnlock()
	return len(b.clients)
}

// Close closes the SSE broker
func (b *SSEBroker) Close() {
	// Signal shutdown
	close(b.done)

	// Wait for run() goroutine to exit
	b.wg.Wait()

	// Now safe to cleanup resources
	b.clientsMu.Lock()
	for _, client := range b.clients {
		close(client.Channel)
	}
	b.clients = make(map[string]*SSEClient)
	b.clientsMu.Unlock()
}

// HandleSSE handles SSE connections from clients
func (h *Handler) HandleSSE(w http.ResponseWriter, r *http.Request) {
	// Check if response writer supports flushing
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported by server", http.StatusInternalServerError)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering

	// Prevent response compression which can interfere with SSE
	w.Header().Set("Content-Encoding", "identity")

	// CORS headers - allow specific origin for credentials
	origin := r.Header.Get("Origin")
	if origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}

	// Write header immediately to establish the connection
	w.WriteHeader(http.StatusOK)

	// Generate client ID
	clientID := fmt.Sprintf("client_%d", time.Now().UnixNano())

	// Register client
	client := h.sseBroker.RegisterClient(clientID)
	defer h.sseBroker.UnregisterClient(client)

	// Send initial connection event
	fmt.Fprintf(w, "data: %s\n\n", `{"type":"connected","data":{"client_id":"`+clientID+`"}}`)
	flusher.Flush()

	// Create context that's cancelled when client disconnects
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Send keepalive pings every 15 seconds
	keepaliveTicker := time.NewTicker(15 * time.Second)
	defer keepaliveTicker.Stop()

	// Stream events to client
	for {
		select {
		case event, ok := <-client.Channel:
			if !ok {
				// Channel closed, client removed
				return
			}

			// Marshal event to JSON
			data, err := json.Marshal(event)
			if err != nil {
				logger.ErrorEvent().
					Err(err).
					Str("event_type", event.Type).
					Msg("Failed to marshal SSE event")
				continue
			}

			// Send event
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()

		case <-keepaliveTicker.C:
			// Send keepalive ping
			fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()

		case <-ctx.Done():
			// Client disconnected
			return
		}
	}
}
