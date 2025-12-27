package metrics

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/pandeptwidyaop/grok/internal/client/dashboard/events"
)

// Snapshot represents a point-in-time view of metrics.
type Snapshot struct {
	TotalRequests  int64     `json:"total_requests"`
	ActiveRequests int64     `json:"active_requests"`
	BytesIn        int64     `json:"bytes_in"`
	BytesOut       int64     `json:"bytes_out"`
	AvgLatencyMS   float64   `json:"avg_latency_ms"` // milliseconds
	P50LatencyMS   float64   `json:"p50_latency_ms"` // milliseconds
	P95LatencyMS   float64   `json:"p95_latency_ms"` // milliseconds
	P99LatencyMS   float64   `json:"p99_latency_ms"` // milliseconds
	MinLatencyMS   float64   `json:"min_latency_ms"` // milliseconds
	MaxLatencyMS   float64   `json:"max_latency_ms"` // milliseconds
	RequestRate    float64   `json:"request_rate"`   // requests per second
	ErrorCount     int64     `json:"error_count"`
	Timestamp      time.Time `json:"timestamp"`
}

// Aggregator aggregates performance metrics from events.
type Aggregator struct {
	// Atomic counters for thread-safe increments
	totalRequests  atomic.Int64
	activeRequests atomic.Int64
	bytesIn        atomic.Int64
	bytesOut       atomic.Int64
	errorCount     atomic.Int64

	// Latency histogram
	latencyHist *Histogram

	// Request rate calculation
	mu             sync.RWMutex
	windowStart    time.Time
	windowRequests int64
	windowDuration time.Duration
}

// NewAggregator creates a new metrics aggregator.
func NewAggregator() *Aggregator {
	return &Aggregator{
		latencyHist:    NewHistogram(),
		windowStart:    time.Now(),
		windowDuration: 1 * time.Minute, // 1-minute rolling window
	}
}

// RecordRequestStart records the start of a request.
func (a *Aggregator) RecordRequestStart() {
	a.activeRequests.Add(1)
}

// RecordRequestEnd records the completion of a request.
func (a *Aggregator) RecordRequestEnd(event events.Event) {
	data, ok := event.Data.(events.RequestCompletedEvent)
	if !ok {
		return
	}

	// Decrement active requests
	a.activeRequests.Add(-1)

	// Increment counters
	a.totalRequests.Add(1)
	a.bytesIn.Add(data.BytesIn)
	a.bytesOut.Add(data.BytesOut)

	if data.Error != "" {
		a.errorCount.Add(1)
	}

	// Record latency
	a.latencyHist.Record(data.Duration)

	// Update request rate window
	a.mu.Lock()
	a.windowRequests++
	a.mu.Unlock()
}

// GetSnapshot returns a snapshot of current metrics.
func (a *Aggregator) GetSnapshot() Snapshot {
	a.mu.RLock()
	elapsed := time.Since(a.windowStart)
	windowReqs := a.windowRequests
	a.mu.RUnlock()

	// Calculate request rate (requests per second)
	requestRate := 0.0
	if elapsed.Seconds() > 0 {
		requestRate = float64(windowReqs) / elapsed.Seconds()
	}

	return Snapshot{
		TotalRequests:  a.totalRequests.Load(),
		ActiveRequests: a.activeRequests.Load(),
		BytesIn:        a.bytesIn.Load(),
		BytesOut:       a.bytesOut.Load(),
		AvgLatencyMS:   float64(a.latencyHist.Mean().Microseconds()) / 1000.0,
		P50LatencyMS:   float64(a.latencyHist.Percentile(0.50).Microseconds()) / 1000.0,
		P95LatencyMS:   float64(a.latencyHist.Percentile(0.95).Microseconds()) / 1000.0,
		P99LatencyMS:   float64(a.latencyHist.Percentile(0.99).Microseconds()) / 1000.0,
		MinLatencyMS:   float64(a.latencyHist.Min().Microseconds()) / 1000.0,
		MaxLatencyMS:   float64(a.latencyHist.Max().Microseconds()) / 1000.0,
		RequestRate:    requestRate,
		ErrorCount:     a.errorCount.Load(),
		Timestamp:      time.Now(),
	}
}

// ResetWindow resets the request rate window.
// Should be called periodically (e.g., every minute).
func (a *Aggregator) ResetWindow() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.windowStart = time.Now()
	a.windowRequests = 0
}

// Reset clears all metrics.
func (a *Aggregator) Reset() {
	a.totalRequests.Store(0)
	a.activeRequests.Store(0)
	a.bytesIn.Store(0)
	a.bytesOut.Store(0)
	a.errorCount.Store(0)
	a.latencyHist.Clear()

	a.mu.Lock()
	a.windowStart = time.Now()
	a.windowRequests = 0
	a.mu.Unlock()
}
