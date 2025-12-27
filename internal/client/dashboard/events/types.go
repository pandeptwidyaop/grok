package events

import "time"

// EventType represents the type of event.
type EventType string

const (
	// EventRequestStarted is emitted when a new request begins processing.
	EventRequestStarted EventType = "request_started"

	// EventRequestCompleted is emitted when request processing finishes.
	EventRequestCompleted EventType = "request_completed"

	// EventConnectionEstablished is emitted when tunnel connection is established.
	EventConnectionEstablished EventType = "connection_established"

	// EventConnectionLost is emitted when tunnel connection is lost.
	EventConnectionLost EventType = "connection_lost"

	// EventMetricsSnapshot is emitted periodically with performance metrics.
	EventMetricsSnapshot EventType = "metrics_snapshot"
)

// Event is the base event structure with polymorphic payload.
type Event struct {
	Type      EventType   `json:"type"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data"`
}

// RequestStartedEvent contains data for request start events.
type RequestStartedEvent struct {
	RequestID  string            `json:"request_id"`
	Method     string            `json:"method"`
	Path       string            `json:"path"`
	RemoteAddr string            `json:"remote_addr"`
	Protocol   string            `json:"protocol"` // "http" or "tcp"
	Headers    map[string]string `json:"headers,omitempty"`
}

// RequestCompletedEvent contains data for request completion events.
type RequestCompletedEvent struct {
	RequestID       string            `json:"request_id"`
	StatusCode      int32             `json:"status_code"` // HTTP only, 0 for TCP
	BytesIn         int64             `json:"bytes_in"`
	BytesOut        int64             `json:"bytes_out"`
	Duration        time.Duration     `json:"duration"`
	Error           string            `json:"error,omitempty"` // Empty if successful
	ResponseHeaders map[string]string `json:"response_headers,omitempty"`
	ResponseBody    []byte            `json:"response_body,omitempty"`
}

// ConnectionEvent contains data for connection state changes.
type ConnectionEvent struct {
	TunnelID  string `json:"tunnel_id"`
	PublicURL string `json:"public_url"`
	LocalAddr string `json:"local_addr"`
	Protocol  string `json:"protocol"`
}

// MetricsSnapshotEvent contains periodic performance metrics.
type MetricsSnapshotEvent struct {
	TotalRequests  int64         `json:"total_requests"`
	ActiveRequests int64         `json:"active_requests"`
	BytesIn        int64         `json:"bytes_in"`
	BytesOut       int64         `json:"bytes_out"`
	AvgLatency     time.Duration `json:"avg_latency"`
	P50Latency     time.Duration `json:"p50_latency"`
	P95Latency     time.Duration `json:"p95_latency"`
	P99Latency     time.Duration `json:"p99_latency"`
	RequestRate    float64       `json:"request_rate"` // requests per second
	ErrorCount     int64         `json:"error_count"`
	Timestamp      time.Time     `json:"timestamp"`
}
