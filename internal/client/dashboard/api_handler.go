package dashboard

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/pandeptwidyaop/grok/pkg/logger"
)

// respondJSON writes a JSON response.
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		logger.ErrorEvent().Err(err).Msg("Failed to encode JSON response")
	}
}

// respondError writes an error response.
func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{
		"error": message,
	})
}

// HandleStatus returns the current tunnel connection status.
// GET /api/status.
func (s *Server) HandleStatus(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	status := s.tunnelStatus
	s.mu.RUnlock()

	// Calculate uptime if connected
	if status.Connected && !status.ConnectedAt.IsZero() {
		status.Uptime = int64(time.Since(status.ConnectedAt).Seconds())
	}

	respondJSON(w, http.StatusOK, status)
}

// HandleRequests returns recent HTTP/TCP requests.
// GET /api/requests?limit=100&offset=0.
func (s *Server) HandleRequests(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	limitStr := r.URL.Query().Get("limit")
	limit := 100 // default

	if limitStr != "" {
		if val, err := strconv.Atoi(limitStr); err == nil && val > 0 {
			limit = val
		}
	}

	// Get recent requests (pointers, not copies)
	requests := s.requestStore.GetRecent(limit)

	// Use thread-safe JSON marshaling with locks held
	requestsJSON, err := s.requestStore.MarshalRecordsJSON(requests)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to marshal requests")
		return
	}

	// Manually construct response JSON
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"requests":%s,"total":%d}`, requestsJSON, s.requestStore.Size())
}

// HandleRequestDetail returns detailed information about a specific request.
// GET /api/requests/{id}.
func (s *Server) HandleRequestDetail(w http.ResponseWriter, r *http.Request) {
	// Extract ID from path
	path := r.URL.Path
	parts := strings.Split(strings.TrimPrefix(path, "/api/requests/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		respondError(w, http.StatusBadRequest, "Missing request ID")
		return
	}

	requestID := parts[0]

	// Get request from store
	request := s.requestStore.GetByID(requestID)
	if request == nil {
		respondError(w, http.StatusNotFound, "Request not found")
		return
	}

	respondJSON(w, http.StatusOK, request)
}

// HandleMetrics returns current performance metrics.
// GET /api/metrics.
func (s *Server) HandleMetrics(w http.ResponseWriter, _ *http.Request) {
	snapshot := s.metricsAgg.GetSnapshot()

	respondJSON(w, http.StatusOK, snapshot)
}

// HandleHealth returns health check status.
// GET /health.
func (s *Server) HandleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "OK")
}

// HandleClear clears all stored requests (useful for testing).
// POST /api/clear.
func (s *Server) HandleClear(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	s.requestStore.Clear()
	s.metricsAgg.Reset()

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Dashboard cleared successfully",
	})
}

// HandleStats returns statistics about the dashboard server.
// GET /api/stats.
func (s *Server) HandleStats(w http.ResponseWriter, _ *http.Request) {
	storeStats := s.requestStore.GetStats()

	stats := map[string]interface{}{
		"request_store":  storeStats,
		"sse_clients":    s.sseBroker.ClientCount(),
		"event_queue":    s.eventCollector.Size(),
		"uptime_seconds": int64(s.getUptime().Seconds()),
	}

	respondJSON(w, http.StatusOK, stats)
}

// corsMiddleware adds CORS headers for localhost access.
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow localhost only
		origin := r.Header.Get("Origin")
		if strings.Contains(origin, "localhost") || strings.Contains(origin, "127.0.0.1") {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		// Handle preflight
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
