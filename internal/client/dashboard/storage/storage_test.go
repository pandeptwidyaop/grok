package storage

import (
	"testing"
	"time"

	"github.com/pandeptwidyaop/grok/internal/client/dashboard/events"
)

func TestCircularBuffer_Add(t *testing.T) {
	cb := NewCircularBuffer[int](5)

	// Add 3 items
	cb.Add(1)
	cb.Add(2)
	cb.Add(3)

	if cb.Size() != 3 {
		t.Errorf("expected size 3, got %d", cb.Size())
	}

	if cb.IsFull() {
		t.Error("buffer should not be full")
	}
}

func TestCircularBuffer_Overflow(t *testing.T) {
	cb := NewCircularBuffer[int](3)

	// Add more items than capacity
	cb.Add(1)
	cb.Add(2)
	cb.Add(3)
	cb.Add(4) // Overwrites 1
	cb.Add(5) // Overwrites 2

	if cb.Size() != 3 {
		t.Errorf("expected size 3, got %d", cb.Size())
	}

	if !cb.IsFull() {
		t.Error("buffer should be full")
	}

	// Should contain 5, 4, 3 (newest to oldest)
	recent := cb.GetRecent(3)
	expected := []int{5, 4, 3}

	for i, val := range recent {
		if val != expected[i] {
			t.Errorf("index %d: expected %d, got %d", i, expected[i], val)
		}
	}
}

func TestCircularBuffer_GetRecent(t *testing.T) {
	cb := NewCircularBuffer[string](10)

	cb.Add("a")
	cb.Add("b")
	cb.Add("c")
	cb.Add("d")
	cb.Add("e")

	// Get last 3
	recent := cb.GetRecent(3)
	expected := []string{"e", "d", "c"}

	if len(recent) != 3 {
		t.Fatalf("expected 3 items, got %d", len(recent))
	}

	for i, val := range recent {
		if val != expected[i] {
			t.Errorf("index %d: expected %s, got %s", i, expected[i], val)
		}
	}
}

func TestCircularBuffer_GetRecent_ExceedsSize(t *testing.T) {
	cb := NewCircularBuffer[int](10)

	cb.Add(1)
	cb.Add(2)
	cb.Add(3)

	// Request more than available
	recent := cb.GetRecent(10)

	if len(recent) != 3 {
		t.Errorf("expected 3 items, got %d", len(recent))
	}
}

func TestCircularBuffer_Clear(t *testing.T) {
	cb := NewCircularBuffer[int](5)

	cb.Add(1)
	cb.Add(2)
	cb.Add(3)

	cb.Clear()

	if cb.Size() != 0 {
		t.Errorf("expected size 0 after clear, got %d", cb.Size())
	}

	if !cb.IsEmpty() {
		t.Error("buffer should be empty after clear")
	}
}

func TestRequestStore_RecordStartAndCompletion(t *testing.T) {
	store := NewRequestStore(100, 100*1024)

	// Record request start
	startEvent := events.Event{
		Type:      events.EventRequestStarted,
		Timestamp: time.Now(),
		Data: events.RequestStartedEvent{
			RequestID:  "req-123",
			Method:     "GET",
			Path:       "/api/users",
			RemoteAddr: "192.168.1.1",
			Protocol:   "http",
			Headers: map[string]string{
				"User-Agent": "Mozilla/5.0",
			},
		},
	}

	store.RecordStart(startEvent)

	// Verify stored
	record := store.GetByID("req-123")
	if record == nil {
		t.Fatal("expected record to be stored")
	}

	if record.Method != "GET" {
		t.Errorf("expected method GET, got %s", record.Method)
	}

	if record.Path != "/api/users" {
		t.Errorf("expected path /api/users, got %s", record.Path)
	}

	if record.Completed {
		t.Error("request should not be marked as completed yet")
	}

	// Record completion
	completeEvent := events.Event{
		Type:      events.EventRequestCompleted,
		Timestamp: time.Now().Add(100 * time.Millisecond),
		Data: events.RequestCompletedEvent{
			RequestID:    "req-123",
			StatusCode:   200,
			BytesIn:      512,
			BytesOut:     1024,
			Duration:     100 * time.Millisecond,
			ResponseBody: []byte(`{"success":true}`),
		},
	}

	store.RecordCompletion(completeEvent)

	// Verify updated
	record = store.GetByID("req-123")
	if record == nil {
		t.Fatal("expected record to exist")
	}

	if !record.Completed {
		t.Error("request should be marked as completed")
	}

	if record.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", record.StatusCode)
	}

	if record.Duration != 100*time.Millisecond {
		t.Errorf("expected duration 100ms, got %v", record.Duration)
	}

	if string(record.ResponseBody) != `{"success":true}` {
		t.Errorf("expected response body to be stored, got %s", string(record.ResponseBody))
	}
}

func TestRequestStore_GetRecent(t *testing.T) {
	store := NewRequestStore(100, 100*1024)

	// Add multiple requests
	for i := 1; i <= 5; i++ {
		event := events.Event{
			Type:      events.EventRequestStarted,
			Timestamp: time.Now(),
			Data: events.RequestStartedEvent{
				RequestID:  string(rune('a' + i - 1)),
				Method:     "GET",
				Path:       "/test",
				RemoteAddr: "192.168.1.1",
				Protocol:   "http",
			},
		}
		store.RecordStart(event)
	}

	recent := store.GetRecent(3)

	if len(recent) != 3 {
		t.Fatalf("expected 3 recent requests, got %d", len(recent))
	}

	// Should be in reverse chronological order (newest first)
	// Last added was 'e', so recent should be [e, d, c]
	expected := []string{"e", "d", "c"}
	for i, record := range recent {
		if record.ID != expected[i] {
			t.Errorf("index %d: expected ID %s, got %s", i, expected[i], record.ID)
		}
	}
}

func TestRequestStore_BodySizeLimit(t *testing.T) {
	store := NewRequestStore(100, 10) // Max 10 bytes

	startEvent := events.Event{
		Type:      events.EventRequestStarted,
		Timestamp: time.Now(),
		Data: events.RequestStartedEvent{
			RequestID: "req-large",
			Method:    "POST",
			Path:      "/upload",
			Protocol:  "http",
		},
	}

	store.RecordStart(startEvent)

	// Try to store large response body (exceeds limit)
	completeEvent := events.Event{
		Type:      events.EventRequestCompleted,
		Timestamp: time.Now(),
		Data: events.RequestCompletedEvent{
			RequestID:    "req-large",
			StatusCode:   200,
			ResponseBody: []byte("This is a very long response body that exceeds the limit"),
		},
	}

	store.RecordCompletion(completeEvent)

	record := store.GetByID("req-large")
	if record == nil {
		t.Fatal("expected record to exist")
	}

	// Body should not be stored due to size limit
	if len(record.ResponseBody) > 0 {
		t.Error("response body should not be stored (exceeds limit)")
	}
}

func TestRequestStore_GetStats(t *testing.T) {
	store := NewRequestStore(100, 100*1024)

	// Add some requests
	for i := 1; i <= 3; i++ {
		startEvent := events.Event{
			Type:      events.EventRequestStarted,
			Timestamp: time.Now(),
			Data: events.RequestStartedEvent{
				RequestID: string(rune('a' + i - 1)),
				Method:    "GET",
				Path:      "/test",
				Protocol:  "http",
			},
		}
		store.RecordStart(startEvent)

		completeEvent := events.Event{
			Type:      events.EventRequestCompleted,
			Timestamp: time.Now(),
			Data: events.RequestCompletedEvent{
				RequestID:  string(rune('a' + i - 1)),
				StatusCode: 200,
				BytesIn:    100,
				BytesOut:   200,
				Duration:   50 * time.Millisecond,
			},
		}
		store.RecordCompletion(completeEvent)
	}

	stats := store.GetStats()

	if stats["total_requests"].(int) != 3 {
		t.Errorf("expected 3 total requests, got %v", stats["total_requests"])
	}

	if stats["completed_count"].(int) != 3 {
		t.Errorf("expected 3 completed requests, got %v", stats["completed_count"])
	}

	if stats["total_bytes_in"].(int64) != 300 {
		t.Errorf("expected 300 bytes in, got %v", stats["total_bytes_in"])
	}

	if stats["total_bytes_out"].(int64) != 600 {
		t.Errorf("expected 600 bytes out, got %v", stats["total_bytes_out"])
	}
}

func BenchmarkCircularBuffer_Add(b *testing.B) {
	cb := NewCircularBuffer[int](1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cb.Add(i)
	}
}

func BenchmarkRequestStore_RecordStart(b *testing.B) {
	store := NewRequestStore(10000, 100*1024)

	event := events.Event{
		Type:      events.EventRequestStarted,
		Timestamp: time.Now(),
		Data: events.RequestStartedEvent{
			RequestID:  "bench-123",
			Method:     "GET",
			Path:       "/test",
			RemoteAddr: "192.168.1.1",
			Protocol:   "http",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.RecordStart(event)
	}
}
