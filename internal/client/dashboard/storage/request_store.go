package storage

import (
	"sync"
	"time"

	"github.com/pandeptwidyaop/grok/internal/client/dashboard/events"
)

// RequestRecord represents a stored HTTP/TCP request with response data.
type RequestRecord struct {
	ID             string            `json:"id"`
	Method         string            `json:"method"`
	Path           string            `json:"path"`
	RemoteAddr     string            `json:"remote_addr"`
	Protocol       string            `json:"protocol"` // "http" or "tcp"
	StatusCode     int32             `json:"status_code"`
	BytesIn        int64             `json:"bytes_in"`
	BytesOut       int64             `json:"bytes_out"`
	Duration       time.Duration     `json:"-"`           // Internal use only
	DurationMS     float64           `json:"duration_ms"` // milliseconds for JSON
	StartTime      time.Time         `json:"start_time"`
	EndTime        time.Time         `json:"end_time"`
	Error          string            `json:"error,omitempty"`
	Completed      bool              `json:"completed"`
	RequestHeaders map[string]string `json:"request_headers,omitempty"`
	RequestBody    []byte            `json:"request_body,omitempty"`
	ResponseBody   []byte            `json:"response_body,omitempty"`
}

// RequestStore stores HTTP/TCP requests in memory with bounded size.
type RequestStore struct {
	requests    *CircularBuffer[*RequestRecord]
	byID        sync.Map // requestID -> *RequestRecord
	maxBodySize int64
}

// NewRequestStore creates a new request store with specified limits.
func NewRequestStore(maxRequests int, maxBodySize int64) *RequestStore {
	return &RequestStore{
		requests:    NewCircularBuffer[*RequestRecord](maxRequests),
		maxBodySize: maxBodySize,
	}
}

// RecordStart records the start of a new request.
func (rs *RequestStore) RecordStart(event events.Event) {
	data, ok := event.Data.(events.RequestStartedEvent)
	if !ok {
		return
	}

	record := &RequestRecord{
		ID:         data.RequestID,
		Method:     data.Method,
		Path:       data.Path,
		RemoteAddr: data.RemoteAddr,
		Protocol:   data.Protocol,
		StartTime:  event.Timestamp,
		Completed:  false,
	}

	// Store headers if present
	if len(data.Headers) > 0 {
		record.RequestHeaders = data.Headers
	}

	rs.requests.Add(record)
	rs.byID.Store(data.RequestID, record)
}

// RecordCompletion updates a request with completion data.
func (rs *RequestStore) RecordCompletion(event events.Event) {
	data, ok := event.Data.(events.RequestCompletedEvent)
	if !ok {
		return
	}

	val, found := rs.byID.Load(data.RequestID)
	if !found {
		return
	}

	record, ok := val.(*RequestRecord)
	if !ok {
		return
	}
	record.StatusCode = data.StatusCode
	record.BytesIn = data.BytesIn
	record.BytesOut = data.BytesOut
	record.Duration = data.Duration
	record.DurationMS = float64(data.Duration.Microseconds()) / 1000.0
	record.EndTime = event.Timestamp
	record.Error = data.Error
	record.Completed = true

	// Store response body if present and within size limit
	if len(data.ResponseBody) > 0 && int64(len(data.ResponseBody)) <= rs.maxBodySize {
		record.ResponseBody = data.ResponseBody
	}
}

// GetRecent returns the most recent N requests.
func (rs *RequestStore) GetRecent(limit int) []*RequestRecord {
	return rs.requests.GetRecent(limit)
}

// GetAll returns all stored requests.
func (rs *RequestStore) GetAll() []*RequestRecord {
	return rs.requests.GetAll()
}

// GetByID retrieves a specific request by ID.
func (rs *RequestStore) GetByID(id string) *RequestRecord {
	val, ok := rs.byID.Load(id)
	if !ok {
		return nil
	}
	record, ok := val.(*RequestRecord)
	if !ok {
		return nil
	}
	return record
}

// Size returns the number of requests currently stored.
func (rs *RequestStore) Size() int {
	return rs.requests.Size()
}

// Clear removes all stored requests.
func (rs *RequestStore) Clear() {
	rs.requests.Clear()

	// Clear the ID map
	rs.byID.Range(func(key, _ interface{}) bool {
		rs.byID.Delete(key)
		return true
	})
}

// GetStats returns statistics about the request store.
func (rs *RequestStore) GetStats() map[string]interface{} {
	requests := rs.requests.GetAll()

	var totalDuration time.Duration
	var completedCount int
	var errorCount int
	var totalBytesIn, totalBytesOut int64

	for _, req := range requests {
		if req.Completed {
			completedCount++
			totalDuration += req.Duration
		}
		if req.Error != "" {
			errorCount++
		}
		totalBytesIn += req.BytesIn
		totalBytesOut += req.BytesOut
	}

	avgDuration := time.Duration(0)
	if completedCount > 0 {
		avgDuration = totalDuration / time.Duration(completedCount)
	}

	return map[string]interface{}{
		"total_requests":  rs.requests.Size(),
		"completed_count": completedCount,
		"error_count":     errorCount,
		"total_bytes_in":  totalBytesIn,
		"total_bytes_out": totalBytesOut,
		"avg_duration_ms": avgDuration.Milliseconds(),
		"capacity":        rs.requests.Capacity(),
		"is_full":         rs.requests.IsFull(),
	}
}
