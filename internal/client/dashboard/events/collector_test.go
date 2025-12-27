package events

import (
	"testing"
	"time"
)

func TestEventCollector_Publish(t *testing.T) {
	ec := NewEventCollector()
	defer ec.Close()

	// Subscribe to events
	ch := ec.Subscribe()

	// Publish event
	event := Event{
		Type:      EventRequestStarted,
		Timestamp: time.Now(),
		Data: RequestStartedEvent{
			RequestID:  "test-123",
			Method:     "GET",
			Path:       "/test",
			RemoteAddr: "192.168.1.1",
			Protocol:   "http",
		},
	}

	ec.Publish(event)

	// Verify event received
	select {
	case received := <-ch:
		if received.Type != EventRequestStarted {
			t.Errorf("expected event type %s, got %s", EventRequestStarted, received.Type)
		}

		data, ok := received.Data.(RequestStartedEvent)
		if !ok {
			t.Fatal("failed to cast event data to RequestStartedEvent")
		}

		if data.RequestID != "test-123" {
			t.Errorf("expected request ID test-123, got %s", data.RequestID)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestEventCollector_NonBlocking(t *testing.T) {
	ec := NewEventCollector()
	defer ec.Close()

	// Fill the event channel
	for i := 0; i < 1001; i++ {
		ec.Publish(Event{
			Type:      EventRequestStarted,
			Timestamp: time.Now(),
			Data:      RequestStartedEvent{RequestID: "test"},
		})
	}

	// Publishing should not block even if channel is full
	start := time.Now()
	ec.Publish(Event{Type: EventRequestStarted, Timestamp: time.Now()})

	if time.Since(start) > 10*time.Millisecond {
		t.Fatal("Publish blocked unexpectedly")
	}
}

func TestEventCollector_MultipleSubscribers(t *testing.T) {
	ec := NewEventCollector()
	defer ec.Close()

	// Create multiple subscribers
	ch1 := ec.Subscribe()
	ch2 := ec.Subscribe()
	ch3 := ec.Subscribe()

	if ec.SubscriberCount() != 3 {
		t.Errorf("expected 3 subscribers, got %d", ec.SubscriberCount())
	}

	// Publish event
	event := Event{
		Type:      EventConnectionEstablished,
		Timestamp: time.Now(),
		Data: ConnectionEvent{
			TunnelID:  "tunnel-123",
			PublicURL: "https://abc123.example.com",
			LocalAddr: "localhost:3000",
			Protocol:  "http",
		},
	}

	ec.Publish(event)

	// All subscribers should receive the event
	timeout := time.After(1 * time.Second)

	select {
	case <-ch1:
		// Received
	case <-timeout:
		t.Fatal("ch1: timeout waiting for event")
	}

	select {
	case <-ch2:
		// Received
	case <-timeout:
		t.Fatal("ch2: timeout waiting for event")
	}

	select {
	case <-ch3:
		// Received
	case <-timeout:
		t.Fatal("ch3: timeout waiting for event")
	}
}

func TestEventCollector_Unsubscribe(t *testing.T) {
	ec := NewEventCollector()
	defer ec.Close()

	ch1 := ec.Subscribe()
	ch2 := ec.Subscribe()

	if ec.SubscriberCount() != 2 {
		t.Errorf("expected 2 subscribers, got %d", ec.SubscriberCount())
	}

	// Unsubscribe ch1
	ec.Unsubscribe(ch1)

	if ec.SubscriberCount() != 1 {
		t.Errorf("expected 1 subscriber after unsubscribe, got %d", ec.SubscriberCount())
	}

	// Publish event
	event := Event{Type: EventRequestCompleted, Timestamp: time.Now()}
	ec.Publish(event)

	// ch2 should receive, ch1 should not (closed)
	select {
	case <-ch2:
		// Received as expected
	case <-time.After(1 * time.Second):
		t.Fatal("ch2: timeout waiting for event")
	}

	// ch1 should be closed
	select {
	case _, ok := <-ch1:
		if ok {
			t.Fatal("ch1 should be closed")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("ch1 should be closed immediately")
	}
}

func TestEventCollector_Close(t *testing.T) {
	ec := NewEventCollector()

	ch := ec.Subscribe()

	// Close the collector
	ec.Close()

	// Subscriber channel should be closed
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("subscriber channel should be closed")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("subscriber channel should be closed immediately")
	}

	// Publishing after close should not panic
	ec.Publish(Event{Type: EventRequestStarted, Timestamp: time.Now()})
}

func BenchmarkEventCollector_Publish(b *testing.B) {
	ec := NewEventCollector()
	defer ec.Close()

	event := Event{
		Type:      EventRequestCompleted,
		Timestamp: time.Now(),
		Data: RequestCompletedEvent{
			RequestID:  "bench-123",
			StatusCode: 200,
			BytesIn:    1024,
			BytesOut:   2048,
			Duration:   100 * time.Millisecond,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ec.Publish(event)
	}
}
