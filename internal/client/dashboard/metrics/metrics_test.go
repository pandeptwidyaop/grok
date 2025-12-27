package metrics

import (
	"testing"
	"time"

	"github.com/pandeptwidyaop/grok/internal/client/dashboard/events"
)

func TestHistogram_Record(t *testing.T) {
	h := NewHistogram()

	h.Record(100 * time.Millisecond)
	h.Record(200 * time.Millisecond)
	h.Record(150 * time.Millisecond)

	if h.Count() != 3 {
		t.Errorf("expected 3 samples, got %d", h.Count())
	}
}

func TestHistogram_Percentile(t *testing.T) {
	h := NewHistogram()

	// Add 100 samples: 1ms, 2ms, 3ms, ..., 100ms
	for i := 1; i <= 100; i++ {
		h.Record(time.Duration(i) * time.Millisecond)
	}

	// P50 should be around 50ms
	p50 := h.Percentile(0.50)
	if p50 < 49*time.Millisecond || p50 > 51*time.Millisecond {
		t.Errorf("expected P50 around 50ms, got %v", p50)
	}

	// P95 should be around 95ms
	p95 := h.Percentile(0.95)
	if p95 < 94*time.Millisecond || p95 > 96*time.Millisecond {
		t.Errorf("expected P95 around 95ms, got %v", p95)
	}

	// P99 should be around 99ms
	p99 := h.Percentile(0.99)
	if p99 < 98*time.Millisecond || p99 > 100*time.Millisecond {
		t.Errorf("expected P99 around 99ms, got %v", p99)
	}
}

func TestHistogram_Mean(t *testing.T) {
	h := NewHistogram()

	h.Record(100 * time.Millisecond)
	h.Record(200 * time.Millisecond)
	h.Record(300 * time.Millisecond)

	mean := h.Mean()
	expected := 200 * time.Millisecond

	if mean != expected {
		t.Errorf("expected mean %v, got %v", expected, mean)
	}
}

func TestHistogram_MinMax(t *testing.T) {
	h := NewHistogram()

	h.Record(100 * time.Millisecond)
	h.Record(500 * time.Millisecond)
	h.Record(300 * time.Millisecond)

	if h.Min() != 100*time.Millisecond {
		t.Errorf("expected min 100ms, got %v", h.Min())
	}

	if h.Max() != 500*time.Millisecond {
		t.Errorf("expected max 500ms, got %v", h.Max())
	}
}

func TestHistogram_Eviction(t *testing.T) {
	h := NewHistogram()

	// Fill beyond capacity
	for i := 0; i < 12000; i++ {
		h.Record(time.Duration(i) * time.Millisecond)
	}

	// Should have evicted oldest 20% when reaching 10000
	if h.Count() > 10000 {
		t.Errorf("expected count <= 10000, got %d", h.Count())
	}
}

func TestAggregator_RecordRequestEnd(t *testing.T) {
	agg := NewAggregator()

	// Record request start
	agg.RecordRequestStart()

	if agg.activeRequests.Load() != 1 {
		t.Errorf("expected 1 active request, got %d", agg.activeRequests.Load())
	}

	// Record completion
	event := events.Event{
		Type:      events.EventRequestCompleted,
		Timestamp: time.Now(),
		Data: events.RequestCompletedEvent{
			RequestID:  "req-1",
			StatusCode: 200,
			BytesIn:    1024,
			BytesOut:   2048,
			Duration:   100 * time.Millisecond,
		},
	}

	agg.RecordRequestEnd(event)

	// Verify metrics
	if agg.activeRequests.Load() != 0 {
		t.Errorf("expected 0 active requests, got %d", agg.activeRequests.Load())
	}

	if agg.totalRequests.Load() != 1 {
		t.Errorf("expected 1 total request, got %d", agg.totalRequests.Load())
	}

	if agg.bytesIn.Load() != 1024 {
		t.Errorf("expected 1024 bytes in, got %d", agg.bytesIn.Load())
	}

	if agg.bytesOut.Load() != 2048 {
		t.Errorf("expected 2048 bytes out, got %d", agg.bytesOut.Load())
	}
}

func TestAggregator_GetSnapshot(t *testing.T) {
	agg := NewAggregator()

	// Record some requests
	for i := 0; i < 10; i++ {
		agg.RecordRequestStart()

		event := events.Event{
			Type:      events.EventRequestCompleted,
			Timestamp: time.Now(),
			Data: events.RequestCompletedEvent{
				RequestID:  "req",
				StatusCode: 200,
				BytesIn:    100,
				BytesOut:   200,
				Duration:   50 * time.Millisecond,
			},
		}

		agg.RecordRequestEnd(event)
	}

	snapshot := agg.GetSnapshot()

	if snapshot.TotalRequests != 10 {
		t.Errorf("expected 10 total requests, got %d", snapshot.TotalRequests)
	}

	if snapshot.BytesIn != 1000 {
		t.Errorf("expected 1000 bytes in, got %d", snapshot.BytesIn)
	}

	if snapshot.BytesOut != 2000 {
		t.Errorf("expected 2000 bytes out, got %d", snapshot.BytesOut)
	}

	if snapshot.AvgLatencyMS != 50.0 {
		t.Errorf("expected avg latency 50ms, got %v", snapshot.AvgLatencyMS)
	}
}

func TestAggregator_ErrorCount(t *testing.T) {
	agg := NewAggregator()

	// Record successful request
	event1 := events.Event{
		Type:      events.EventRequestCompleted,
		Timestamp: time.Now(),
		Data: events.RequestCompletedEvent{
			RequestID:  "req-1",
			StatusCode: 200,
			Duration:   50 * time.Millisecond,
		},
	}

	agg.RecordRequestEnd(event1)

	// Record failed request
	event2 := events.Event{
		Type:      events.EventRequestCompleted,
		Timestamp: time.Now(),
		Data: events.RequestCompletedEvent{
			RequestID:  "req-2",
			StatusCode: 500,
			Duration:   100 * time.Millisecond,
			Error:      "internal server error",
		},
	}

	agg.RecordRequestEnd(event2)

	snapshot := agg.GetSnapshot()

	if snapshot.TotalRequests != 2 {
		t.Errorf("expected 2 total requests, got %d", snapshot.TotalRequests)
	}

	if snapshot.ErrorCount != 1 {
		t.Errorf("expected 1 error, got %d", snapshot.ErrorCount)
	}
}

func TestAggregator_RequestRate(t *testing.T) {
	agg := NewAggregator()

	// Simulate 10 requests
	for i := 0; i < 10; i++ {
		event := events.Event{
			Type:      events.EventRequestCompleted,
			Timestamp: time.Now(),
			Data: events.RequestCompletedEvent{
				RequestID: "req",
				Duration:  10 * time.Millisecond,
			},
		}

		agg.RecordRequestEnd(event)
	}

	// Wait a bit
	time.Sleep(100 * time.Millisecond)

	snapshot := agg.GetSnapshot()

	// Request rate should be roughly 10 / 0.1s = 100 req/s
	// Allow for some variance
	if snapshot.RequestRate < 50 || snapshot.RequestRate > 150 {
		t.Errorf("expected request rate around 100 req/s, got %.2f", snapshot.RequestRate)
	}
}

func BenchmarkHistogram_Record(b *testing.B) {
	h := NewHistogram()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h.Record(time.Duration(i) * time.Millisecond)
	}
}

func BenchmarkAggregator_RecordRequestEnd(b *testing.B) {
	agg := NewAggregator()

	event := events.Event{
		Type:      events.EventRequestCompleted,
		Timestamp: time.Now(),
		Data: events.RequestCompletedEvent{
			RequestID:  "bench",
			StatusCode: 200,
			BytesIn:    1024,
			BytesOut:   2048,
			Duration:   50 * time.Millisecond,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		agg.RecordRequestEnd(event)
	}
}
