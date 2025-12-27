package dashboard

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pandeptwidyaop/grok/internal/client/dashboard/events"
)

// TestDashboardIntegration tests the complete event flow from publishing to API endpoints.
func TestDashboardIntegration(t *testing.T) {
	// Create dashboard server
	cfg := Config{
		Port:        0, // Use random port
		EnableSSE:   true,
		MaxRequests: 100,
		MaxBodySize: 1024 * 10, // 10KB
	}

	server := NewServer(cfg)

	// Wait for server to initialize (goroutines started in NewServer)
	time.Sleep(50 * time.Millisecond)

	t.Run("Publish and retrieve request events", func(t *testing.T) {
		// Clear previous requests
		server.requestStore.Clear()
		server.metricsAgg.Reset()

		// Verify cleared
		if size := server.requestStore.Size(); size != 0 {
			t.Fatalf("Expected 0 requests after clear, got %d", size)
		}

		// Publish request started event
		server.eventCollector.Publish(events.Event{
			Type:      events.EventRequestStarted,
			Timestamp: time.Now(),
			Data: events.RequestStartedEvent{
				RequestID:  "req-123",
				Method:     "GET",
				Path:       "/api/test",
				RemoteAddr: "127.0.0.1:1234",
				Protocol:   "http",
			},
		})

		// Wait for request started to be processed
		time.Sleep(30 * time.Millisecond)

		// Verify request was added
		if size := server.requestStore.Size(); size != 1 {
			t.Logf("After RequestStarted: expected 1 request, got %d", size)
		}

		// Publish request completed event
		server.eventCollector.Publish(events.Event{
			Type:      events.EventRequestCompleted,
			Timestamp: time.Now(),
			Data: events.RequestCompletedEvent{
				RequestID:    "req-123",
				StatusCode:   200,
				BytesIn:      100,
				BytesOut:     500,
				Duration:     50 * time.Millisecond,
				ResponseBody: []byte(`{"result": "ok"}`),
			},
		})

		// Wait for events to be processed
		time.Sleep(50 * time.Millisecond)

		// Check that request was stored (should still be 1, not 2)
		if server.requestStore.Size() != 1 {
			requests := server.requestStore.GetAll()
			t.Logf("Requests in store:")
			for i, req := range requests {
				t.Logf("  [%d] ID=%s, Method=%s, Path=%s, Completed=%v", i, req.ID, req.Method, req.Path, req.Completed)
			}
			t.Errorf("Expected 1 request in store, got %d", server.requestStore.Size())
		}

		// Retrieve request via API
		req := httptest.NewRequest("GET", "/api/requests?limit=10", nil)
		w := httptest.NewRecorder()
		server.HandleRequests(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		requests, ok := response["requests"].([]interface{})
		if !ok || len(requests) == 0 {
			t.Error("Expected requests in response")
		}
	})

	t.Run("Metrics aggregation", func(t *testing.T) {
		// Reset metrics
		server.metricsAgg.Reset()

		// Publish multiple request events
		for i := 0; i < 5; i++ {
			server.eventCollector.Publish(events.Event{
				Type:      events.EventRequestStarted,
				Timestamp: time.Now(),
				Data: events.RequestStartedEvent{
					RequestID: "req-" + string(rune(i)),
					Method:    "GET",
					Path:      "/test",
					Protocol:  "http",
				},
			})

			server.eventCollector.Publish(events.Event{
				Type:      events.EventRequestCompleted,
				Timestamp: time.Now(),
				Data: events.RequestCompletedEvent{
					RequestID:  "req-" + string(rune(i)),
					StatusCode: 200,
					BytesIn:    100,
					BytesOut:   200,
					Duration:   time.Duration(i+1) * 10 * time.Millisecond,
				},
			})
		}

		// Wait for processing
		time.Sleep(100 * time.Millisecond)

		// Get metrics via API
		req := httptest.NewRequest("GET", "/api/metrics", nil)
		w := httptest.NewRecorder()
		server.HandleMetrics(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var metrics map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&metrics); err != nil {
			t.Fatalf("Failed to decode metrics: %v", err)
		}

		// Verify metrics
		totalRequests, ok := metrics["total_requests"].(float64)
		if !ok || totalRequests < 5 {
			t.Errorf("Expected at least 5 total requests, got %v", totalRequests)
		}
	})

	t.Run("Connection status updates", func(t *testing.T) {
		// Publish connection established event
		server.eventCollector.Publish(events.Event{
			Type:      events.EventConnectionEstablished,
			Timestamp: time.Now(),
			Data: events.ConnectionEvent{
				TunnelID:  "tunnel-123",
				PublicURL: "https://test.example.com",
				LocalAddr: "localhost:3000",
				Protocol:  "http",
			},
		})

		// Wait for processing
		time.Sleep(50 * time.Millisecond)

		// Get status via API
		req := httptest.NewRequest("GET", "/api/status", nil)
		w := httptest.NewRecorder()
		server.HandleStatus(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var status TunnelStatus
		if err := json.NewDecoder(w.Body).Decode(&status); err != nil {
			t.Fatalf("Failed to decode status: %v", err)
		}

		if !status.Connected {
			t.Error("Expected status to be connected")
		}

		if status.TunnelID != "tunnel-123" {
			t.Errorf("Expected tunnel ID 'tunnel-123', got '%s'", status.TunnelID)
		}

		if status.PublicURL != "https://test.example.com" {
			t.Errorf("Expected public URL 'https://test.example.com', got '%s'", status.PublicURL)
		}
	})

	t.Run("Clear functionality", func(t *testing.T) {
		// Clear dashboard
		req := httptest.NewRequest("POST", "/api/clear", nil)
		w := httptest.NewRecorder()
		server.HandleClear(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		// Verify requests are cleared
		if server.requestStore.Size() != 0 {
			t.Errorf("Expected 0 requests after clear, got %d", server.requestStore.Size())
		}

		// Verify metrics are reset
		snapshot := server.metricsAgg.GetSnapshot()
		if snapshot.TotalRequests != 0 {
			t.Errorf("Expected 0 total requests after reset, got %d", snapshot.TotalRequests)
		}
	})

	t.Run("Request detail retrieval", func(t *testing.T) {
		// Add a request with full details
		server.eventCollector.Publish(events.Event{
			Type:      events.EventRequestStarted,
			Timestamp: time.Now(),
			Data: events.RequestStartedEvent{
				RequestID:  "req-detail",
				Method:     "POST",
				Path:       "/api/data",
				RemoteAddr: "192.168.1.1:5678",
				Protocol:   "http",
				Headers: map[string][]string{
					"Content-Type": {"application/json"},
					"User-Agent":   {"TestClient/1.0"},
				},
			},
		})

		server.eventCollector.Publish(events.Event{
			Type:      events.EventRequestCompleted,
			Timestamp: time.Now(),
			Data: events.RequestCompletedEvent{
				RequestID:    "req-detail",
				StatusCode:   201,
				BytesIn:      256,
				BytesOut:     128,
				Duration:     100 * time.Millisecond,
				ResponseBody: []byte(`{"id":"abc","status":"created"}`),
			},
		})

		// Wait for processing
		time.Sleep(50 * time.Millisecond)

		// Get request detail
		req := httptest.NewRequest("GET", "/api/requests/req-detail", nil)
		w := httptest.NewRecorder()
		server.HandleRequestDetail(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var request map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&request); err != nil {
			t.Fatalf("Failed to decode request detail: %v", err)
		}

		if request["method"] != "POST" {
			t.Errorf("Expected method POST, got %v", request["method"])
		}

		if request["status_code"].(float64) != 201 {
			t.Errorf("Expected status 201, got %v", request["status_code"])
		}
	})

	t.Run("Stats endpoint", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/stats", nil)
		w := httptest.NewRecorder()
		server.HandleStats(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var stats map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&stats); err != nil {
			t.Fatalf("Failed to decode stats: %v", err)
		}

		// Verify stats structure
		if _, ok := stats["request_store"]; !ok {
			t.Error("Expected request_store in stats")
		}

		if _, ok := stats["sse_clients"]; !ok {
			t.Error("Expected sse_clients in stats")
		}

		if _, ok := stats["event_queue"]; !ok {
			t.Error("Expected event_queue in stats")
		}
	})

	// Cleanup
	server.Close()
}

// TestSSEBroker tests the SSE broker functionality.
func TestSSEBroker(t *testing.T) {
	broker := NewSSEBroker()
	defer broker.Close()

	t.Run("Client registration and broadcast", func(t *testing.T) {
		// Create test client
		client := &SSEClient{
			ID:       "test-client",
			Channel:  make(chan SSEEvent, 10),
			LastSeen: time.Now(),
		}

		// Register client
		broker.register <- client

		// Wait for registration
		time.Sleep(10 * time.Millisecond)

		// Broadcast event
		testEvent := SSEEvent{
			Type: "test_event",
			Data: map[string]string{"message": "hello"},
		}

		broker.Broadcast(testEvent)

		// Wait for broadcast
		select {
		case received := <-client.Channel:
			if received.Type != "test_event" {
				t.Errorf("Expected event type 'test_event', got '%s'", received.Type)
			}
		case <-time.After(1 * time.Second):
			t.Error("Timeout waiting for broadcast event")
		}

		// Unregister client
		broker.unregister <- client
		time.Sleep(10 * time.Millisecond)

		// Verify client count
		if broker.ClientCount() != 0 {
			t.Errorf("Expected 0 clients after unregister, got %d", broker.ClientCount())
		}
	})

	t.Run("Multiple clients", func(t *testing.T) {
		clients := make([]*SSEClient, 3)
		for i := range clients {
			clients[i] = &SSEClient{
				ID:       "client-" + string(rune(i)),
				Channel:  make(chan SSEEvent, 10),
				LastSeen: time.Now(),
			}
			broker.register <- clients[i]
		}

		// Wait for registration
		time.Sleep(50 * time.Millisecond)

		if broker.ClientCount() != 3 {
			t.Errorf("Expected 3 clients, got %d", broker.ClientCount())
		}

		// Broadcast to all
		testEvent := SSEEvent{
			Type: "broadcast_test",
			Data: map[string]int{"count": 123},
		}

		broker.Broadcast(testEvent)

		// Verify all clients received event
		for i, client := range clients {
			select {
			case received := <-client.Channel:
				if received.Type != "broadcast_test" {
					t.Errorf("Client %d: Expected event type 'broadcast_test', got '%s'", i, received.Type)
				}
			case <-time.After(1 * time.Second):
				t.Errorf("Client %d: Timeout waiting for event", i)
			}
		}

		// Cleanup
		for _, client := range clients {
			broker.unregister <- client
		}
	})
}

// TestEventCollectorConcurrency tests concurrent event publishing.
func TestEventCollectorConcurrency(t *testing.T) {
	collector := events.NewEventCollector()
	defer collector.Close()

	// Subscribe to events
	eventCh := collector.Subscribe()

	receivedEvents := make(chan events.Event, 100)
	go func() {
		for event := range eventCh {
			receivedEvents <- event
		}
	}()

	// Publish events concurrently
	const numGoroutines = 10
	const eventsPerGoroutine = 10

	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < eventsPerGoroutine; j++ {
				collector.Publish(events.Event{
					Type:      events.EventRequestCompleted,
					Timestamp: time.Now(),
					Data: events.RequestCompletedEvent{
						RequestID:  "concurrent-" + string(rune(id)) + "-" + string(rune(j)),
						StatusCode: 200,
						Duration:   10 * time.Millisecond,
					},
				})
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Wait for events to be processed
	time.Sleep(100 * time.Millisecond)

	// Count received events
	receivedCount := len(receivedEvents)
	expectedCount := numGoroutines * eventsPerGoroutine

	if receivedCount != expectedCount {
		t.Errorf("Expected %d events, received %d", expectedCount, receivedCount)
	}
}
