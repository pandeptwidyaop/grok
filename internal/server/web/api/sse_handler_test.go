package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewSSEBroker tests SSE broker initialization
func TestNewSSEBroker(t *testing.T) {
	tests := []struct {
		name     string
		logLevel string
	}{
		{"silent log level", "silent"},
		{"warn log level", "warn"},
		{"info log level", "info"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			broker := NewSSEBroker(tt.logLevel)
			assert.NotNil(t, broker)
			assert.Equal(t, tt.logLevel, broker.sseLogLevel)
			assert.NotNil(t, broker.clients)
			assert.NotNil(t, broker.register)
			assert.NotNil(t, broker.unregister)
			assert.NotNil(t, broker.broadcast)
			assert.NotNil(t, broker.done)

			// Cleanup
			broker.Close()
		})
	}
}

// TestRegisterClient tests client registration
func TestRegisterClient(t *testing.T) {
	broker := NewSSEBroker("silent")
	defer broker.Close()

	clientID := "test-client-1"
	client := broker.RegisterClient(clientID)

	assert.NotNil(t, client)
	assert.Equal(t, clientID, client.ID)
	assert.NotNil(t, client.Channel)
	assert.False(t, client.LastSeen.IsZero())

	// Give broker time to process registration
	time.Sleep(50 * time.Millisecond)

	count := broker.GetClientCount()
	assert.Equal(t, 1, count)
}

// TestUnregisterClient tests client unregistration
func TestUnregisterClient(t *testing.T) {
	broker := NewSSEBroker("silent")
	defer broker.Close()

	client := broker.RegisterClient("test-client-1")
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 1, broker.GetClientCount())

	broker.UnregisterClient(client)
	time.Sleep(50 * time.Millisecond)

	assert.Equal(t, 0, broker.GetClientCount())
}

// TestBroadcastToSingleClient tests broadcasting to one client
func TestBroadcastToSingleClient(t *testing.T) {
	broker := NewSSEBroker("silent")
	defer broker.Close()

	client := broker.RegisterClient("test-client-1")
	time.Sleep(50 * time.Millisecond)

	// Broadcast event
	event := SSEEvent{
		Type: "test_event",
		Data: map[string]string{"message": "hello"},
	}
	broker.Broadcast(event)

	// Receive event
	select {
	case received := <-client.Channel:
		assert.Equal(t, "test_event", received.Type)
		data, ok := received.Data.(map[string]string)
		require.True(t, ok)
		assert.Equal(t, "hello", data["message"])
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for event")
	}
}

// TestBroadcastToMultipleClients tests broadcasting to multiple clients
func TestBroadcastToMultipleClients(t *testing.T) {
	broker := NewSSEBroker("silent")
	defer broker.Close()

	// Register 3 clients
	client1 := broker.RegisterClient("client-1")
	client2 := broker.RegisterClient("client-2")
	client3 := broker.RegisterClient("client-3")
	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, 3, broker.GetClientCount())

	// Broadcast event
	event := SSEEvent{
		Type: "broadcast_test",
		Data: "test data",
	}
	broker.Broadcast(event)

	// All clients should receive the event
	clients := []*SSEClient{client1, client2, client3}
	for i, client := range clients {
		select {
		case received := <-client.Channel:
			assert.Equal(t, "broadcast_test", received.Type, "client %d", i+1)
			assert.Equal(t, "test data", received.Data, "client %d", i+1)
		case <-time.After(1 * time.Second):
			t.Fatalf("client %d timeout waiting for event", i+1)
		}
	}
}

// TestConcurrentClientRegistration tests concurrent client registration
func TestConcurrentClientRegistration(t *testing.T) {
	broker := NewSSEBroker("silent")
	defer broker.Close()

	numClients := 50
	var wg sync.WaitGroup
	wg.Add(numClients)

	// Register clients concurrently
	for i := 0; i < numClients; i++ {
		go func(id int) {
			defer wg.Done()
			clientID := string(rune(65 + id)) // A, B, C, ...
			broker.RegisterClient(clientID)
		}(i)
	}

	wg.Wait()
	time.Sleep(100 * time.Millisecond)

	count := broker.GetClientCount()
	assert.Equal(t, numClients, count)
}

// TestBroadcastChannelBufferOverflow tests broadcast buffer overflow
func TestBroadcastChannelBufferOverflow(t *testing.T) {
	// Create a broker with a smaller buffer for easier testing
	// Don't start run() goroutine so events accumulate
	broker := &SSEBroker{
		clients:     make(map[string]*SSEClient),
		register:    make(chan *SSEClient),
		unregister:  make(chan *SSEClient),
		broadcast:   make(chan SSEEvent, 5), // Smaller buffer for testing
		done:        make(chan struct{}),
		sseLogLevel: "silent",
	}

	// Fill the broadcast buffer (5 events)
	for i := 0; i < 5; i++ {
		broker.broadcast <- SSEEvent{
			Type: "test",
			Data: i,
		}
	}

	// Next broadcast should be dropped (channel full)
	select {
	case broker.broadcast <- SSEEvent{Type: "overflow", Data: "should be dropped"}:
		t.Fatal("Expected broadcast to block on full channel")
	default:
		// Expected: channel is full, broadcast would block
	}

	// Buffer should be full
	assert.Equal(t, 5, len(broker.broadcast))
}

// TestClientChannelBufferOverflow tests client channel buffer overflow
func TestClientChannelBufferOverflow(t *testing.T) {
	broker := NewSSEBroker("silent")
	defer broker.Close()

	client := broker.RegisterClient("slow-client")
	time.Sleep(50 * time.Millisecond)

	// Fill client buffer (10 events) without reading
	for i := 0; i < 15; i++ {
		broker.Broadcast(SSEEvent{
			Type: "test",
			Data: i,
		})
		time.Sleep(10 * time.Millisecond)
	}

	// Client should have received some events but not all (slow client handling)
	// Channel buffer is 10, so at most 10 events should be in buffer
	assert.LessOrEqual(t, len(client.Channel), 10)
}

// TestCleanupStaleClients tests stale client cleanup
func TestCleanupStaleClients(t *testing.T) {
	broker := NewSSEBroker("silent")
	defer broker.Close()

	// Register client
	client := broker.RegisterClient("stale-client")
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 1, broker.GetClientCount())

	// Manually set LastSeen to 6 minutes ago
	broker.clientsMu.Lock()
	client.LastSeen = time.Now().Add(-6 * time.Minute)
	broker.clientsMu.Unlock()

	// Trigger cleanup
	broker.cleanupStaleClients()
	time.Sleep(50 * time.Millisecond)

	// Client should be removed
	assert.Equal(t, 0, broker.GetClientCount())
}

// TestBrokerClose tests closing the broker
func TestBrokerClose(t *testing.T) {
	broker := NewSSEBroker("silent")

	// Register some clients
	client1 := broker.RegisterClient("client-1")
	client2 := broker.RegisterClient("client-2")
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 2, broker.GetClientCount())

	// Close broker
	broker.Close()
	time.Sleep(50 * time.Millisecond)

	// All clients should be removed
	assert.Equal(t, 0, broker.GetClientCount())

	// Client channels should be closed
	_, ok1 := <-client1.Channel
	_, ok2 := <-client2.Channel
	assert.False(t, ok1, "client1 channel should be closed")
	assert.False(t, ok2, "client2 channel should be closed")
}

// TestHandleSSE_Basic tests basic SSE connection
func TestHandleSSE_Basic(t *testing.T) {
	broker := NewSSEBroker("silent")
	defer broker.Close()

	handler := &Handler{
		sseBroker: broker,
	}

	// Create context with cancel
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest("GET", "/events", nil)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	// Run HandleSSE in goroutine
	done := make(chan bool)
	go func() {
		handler.HandleSSE(rec, req)
		done <- true
	}()

	// Give it time to establish connection
	time.Sleep(100 * time.Millisecond)

	// Check response headers
	assert.Equal(t, "text/event-stream", rec.Header().Get("Content-Type"))
	assert.Equal(t, "no-cache", rec.Header().Get("Cache-Control"))
	assert.Equal(t, "keep-alive", rec.Header().Get("Connection"))

	// Check initial connection event
	body := rec.Body.String()
	assert.Contains(t, body, `"type":"connected"`)
	assert.Contains(t, body, `"client_id"`)

	// Client should be registered
	assert.Equal(t, 1, broker.GetClientCount())

	// Cancel context to close connection
	cancel()

	select {
	case <-done:
		// Connection closed
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for connection to close")
	}
}

// TestHandleSSE_ReceiveEvents tests receiving events through SSE
func TestHandleSSE_ReceiveEvents(t *testing.T) {
	broker := NewSSEBroker("silent")
	defer broker.Close()

	handler := &Handler{
		sseBroker: broker,
	}

	req := httptest.NewRequest("GET", "/events", nil)
	rec := httptest.NewRecorder()

	// Create context with cancel
	ctx, cancel := context.WithCancel(req.Context())
	req = req.WithContext(ctx)

	// Run HandleSSE in goroutine
	go handler.HandleSSE(rec, req)

	// Give it time to establish connection
	time.Sleep(100 * time.Millisecond)

	// Broadcast event
	broker.Broadcast(SSEEvent{
		Type: "tunnel_created",
		Data: map[string]string{"id": "tunnel-123"},
	})

	// Give it time to send event
	time.Sleep(100 * time.Millisecond)

	// Check that event was sent
	body := rec.Body.String()
	assert.Contains(t, body, `"type":"tunnel_created"`)
	assert.Contains(t, body, `"id":"tunnel-123"`)

	// Close connection
	cancel()
	time.Sleep(100 * time.Millisecond)
}

// TestHandleSSE_Keepalive tests keepalive pings
func TestHandleSSE_Keepalive(t *testing.T) {
	broker := NewSSEBroker("silent")
	defer broker.Close()

	handler := &Handler{
		sseBroker: broker,
	}

	req := httptest.NewRequest("GET", "/events", nil)
	rec := httptest.NewRecorder()

	ctx, cancel := context.WithCancel(req.Context())
	defer cancel()
	req = req.WithContext(ctx)

	// Run HandleSSE in goroutine
	go handler.HandleSSE(rec, req)

	// Wait for potential keepalive (15 seconds is too long for test,
	// but we can check the mechanism is in place)
	time.Sleep(100 * time.Millisecond)

	// Just verify connection established successfully
	body := rec.Body.String()
	assert.Contains(t, body, "data:")
}

// TestHandleSSE_CORS tests CORS headers
func TestHandleSSE_CORS(t *testing.T) {
	broker := NewSSEBroker("silent")
	defer broker.Close()

	handler := &Handler{
		sseBroker: broker,
	}

	req := httptest.NewRequest("GET", "/events", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()

	ctx, cancel := context.WithCancel(req.Context())
	defer cancel()
	req = req.WithContext(ctx)

	go handler.HandleSSE(rec, req)
	time.Sleep(100 * time.Millisecond)

	// Check CORS headers
	assert.Equal(t, "http://localhost:3000", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", rec.Header().Get("Access-Control-Allow-Credentials"))
}

// TestHandleSSE_InvalidResponseWriter tests handling when ResponseWriter doesn't support flushing
func TestHandleSSE_InvalidResponseWriter(t *testing.T) {
	broker := NewSSEBroker("silent")
	defer broker.Close()

	handler := &Handler{
		sseBroker: broker,
	}

	// Use a custom ResponseWriter that doesn't implement http.Flusher
	type nonFlusher struct {
		http.ResponseWriter
	}

	req := httptest.NewRequest("GET", "/events", nil)
	rec := &nonFlusher{ResponseWriter: httptest.NewRecorder()}

	handler.HandleSSE(rec, req)

	// Should return error since flusher is not supported
	// (Can't easily test this with httptest.ResponseRecorder as it implements Flusher)
}

// TestSSEEvent_JSONMarshaling tests JSON marshaling of SSE events
func TestSSEEvent_JSONMarshaling(t *testing.T) {
	event := SSEEvent{
		Type: "test_event",
		Data: map[string]interface{}{
			"key1": "value1",
			"key2": 123,
			"key3": true,
		},
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)

	var unmarshaled SSEEvent
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, event.Type, unmarshaled.Type)
	assert.NotNil(t, unmarshaled.Data)
}

// TestLogLevels tests different log levels
func TestLogLevels(t *testing.T) {
	tests := []struct {
		name     string
		logLevel string
	}{
		{"silent level", "silent"},
		{"warn level", "warn"},
		{"info level", "info"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			broker := NewSSEBroker(tt.logLevel)
			defer broker.Close()

			// Register and unregister client (should not panic with any log level)
			client := broker.RegisterClient("test-client")
			time.Sleep(50 * time.Millisecond)
			broker.UnregisterClient(client)
			time.Sleep(50 * time.Millisecond)

			// Should work without errors
			assert.Equal(t, 0, broker.GetClientCount())
		})
	}
}

// BenchmarkBroadcast benchmarks event broadcasting
func BenchmarkBroadcast(b *testing.B) {
	broker := NewSSEBroker("silent")
	defer broker.Close()

	// Register 10 clients
	for i := 0; i < 10; i++ {
		client := broker.RegisterClient(string(rune(65 + i)))
		// Drain events in background
		go func(c *SSEClient) {
			for range c.Channel {
				// Consume events
			}
		}(client)
	}
	time.Sleep(100 * time.Millisecond)

	event := SSEEvent{
		Type: "benchmark",
		Data: "test data",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		broker.Broadcast(event)
	}
}

// BenchmarkRegisterUnregister benchmarks client registration/unregistration
func BenchmarkRegisterUnregister(b *testing.B) {
	broker := NewSSEBroker("silent")
	defer broker.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client := broker.RegisterClient("bench-client")
		broker.UnregisterClient(client)
	}
}
