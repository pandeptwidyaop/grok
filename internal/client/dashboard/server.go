package dashboard

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/pandeptwidyaop/grok/internal/client/dashboard/events"
	"github.com/pandeptwidyaop/grok/internal/client/dashboard/metrics"
	"github.com/pandeptwidyaop/grok/internal/client/dashboard/storage"
	"github.com/pandeptwidyaop/grok/pkg/logger"
)

// Config contains dashboard server configuration.
type Config struct {
	Port          int           // Dashboard port (default: 4041)
	EnableSSE     bool          // Enable SSE (default: true)
	MaxRequests   int           // Max requests to store (default: 1000)
	MaxBodySize   int64         // Max body size to capture (default: 100KB)
	RetentionTime time.Duration // How long to keep requests (default: 1 hour)
}

// TunnelStatus represents the current tunnel connection status.
type TunnelStatus struct {
	Connected   bool      `json:"connected"`
	TunnelID    string    `json:"tunnel_id,omitempty"`
	PublicURL   string    `json:"public_url,omitempty"`
	LocalAddr   string    `json:"local_addr,omitempty"`
	Protocol    string    `json:"protocol,omitempty"`
	Uptime      int64     `json:"uptime_seconds"`
	ConnectedAt time.Time `json:"connected_at,omitempty"`
}

// Server is the dashboard HTTP server.
type Server struct {
	cfg            Config
	httpServer     *http.Server
	eventCollector *events.EventCollector
	requestStore   *storage.RequestStore
	metricsAgg     *metrics.Aggregator
	sseBroker      *SSEBroker
	tunnelStatus   TunnelStatus
	startTime      time.Time
	mu             sync.RWMutex
	running        bool
}

// NewServer creates a new dashboard server.
func NewServer(cfg Config) *Server {
	// Set defaults
	if cfg.Port == 0 {
		cfg.Port = 4041
	}
	if cfg.MaxRequests == 0 {
		cfg.MaxRequests = 1000
	}
	if cfg.MaxBodySize == 0 {
		cfg.MaxBodySize = 100 * 1024 // 100KB
	}
	if cfg.RetentionTime == 0 {
		cfg.RetentionTime = 1 * time.Hour
	}

	s := &Server{
		cfg:            cfg,
		eventCollector: events.NewEventCollector(),
		requestStore:   storage.NewRequestStore(cfg.MaxRequests, cfg.MaxBodySize),
		metricsAgg:     metrics.NewAggregator(),
		sseBroker:      NewSSEBroker(),
		startTime:      time.Now(),
		tunnelStatus: TunnelStatus{
			Connected: false,
		},
	}

	// Subscribe to events and process them
	go s.processEvents()

	// Start periodic metrics snapshot broadcaster
	go s.broadcastMetrics()

	return s
}

// Start starts the dashboard HTTP server.
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	// API endpoints
	mux.HandleFunc("/api/status", s.HandleStatus)
	mux.HandleFunc("/api/requests", s.HandleRequests)
	mux.HandleFunc("/api/requests/", s.HandleRequestDetail)
	mux.HandleFunc("/api/metrics", s.HandleMetrics)
	mux.HandleFunc("/api/stats", s.HandleStats)
	mux.HandleFunc("/api/clear", s.HandleClear)

	// SSE endpoint
	if s.cfg.EnableSSE {
		mux.HandleFunc("/api/sse", s.HandleSSE)
	}

	// Health check
	mux.HandleFunc("/health", s.HandleHealth)

	// Serve embedded React dashboard
	mux.Handle("/", s.serveDashboard())

	addr := fmt.Sprintf("127.0.0.1:%d", s.cfg.Port)
	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      s.corsMiddleware(mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 0, // No write timeout for SSE
		IdleTimeout:  60 * time.Second,
	}

	s.mu.Lock()
	s.running = true
	s.mu.Unlock()

	logger.InfoEvent().
		Str("addr", addr).
		Int("max_requests", s.cfg.MaxRequests).
		Msg("Dashboard server starting")

	// Run server in goroutine
	errCh := make(chan error, 1)
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// Wait for context cancellation or error
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return s.Close()
	}
}

// serveDashboard returns an HTTP handler for serving the embedded React dashboard.
func (s *Server) serveDashboard() http.Handler {
	// Try to get embedded dashboard filesystem
	dashFS, err := GetDashboardFS()
	if err != nil {
		// Fallback to placeholder if dashboard is not embedded
		logger.WarnEvent().Err(err).Msg("Dashboard not embedded, serving placeholder")
		return http.HandlerFunc(s.servePlaceholder)
	}

	// Serve embedded static files
	return http.FileServer(http.FS(dashFS))
}

// servePlaceholder serves a simple dashboard placeholder when React app is not embedded.
func (s *Server) servePlaceholder(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	html := `<!DOCTYPE html>
<html>
<head>
    <title>Grok Client Dashboard</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; background: #f5f5f5; }
        .container { max-width: 800px; margin: 0 auto; background: white; padding: 40px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        h1 { color: #333; margin-top: 0; }
        .info { background: #e3f2fd; padding: 20px; border-radius: 5px; border-left: 4px solid #2196f3; }
        code { background: #e0e0e0; padding: 2px 6px; border-radius: 3px; font-family: monospace; }
        ul { line-height: 1.8; }
        .note { color: #666; font-size: 14px; margin-top: 20px; }
    </style>
</head>
<body>
    <div class="container">
        <h1>🚀 Grok Client Dashboard</h1>
        <div class="info">
            <p><strong>Dashboard server is running!</strong></p>
            <p>API Endpoints:</p>
            <ul>
                <li><code>GET /api/status</code> - Tunnel connection status</li>
                <li><code>GET /api/requests</code> - Recent requests</li>
                <li><code>GET /api/metrics</code> - Performance metrics</li>
                <li><code>GET /api/sse</code> - Real-time event stream</li>
                <li><code>GET /health</code> - Health check</li>
            </ul>
            <p class="note">
                <strong>Note:</strong> The React dashboard UI is not embedded in this build.
                To build with the dashboard UI, run: <code>make build-client-dashboard && make build-client</code>
            </p>
        </div>
    </div>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, html)
}

// processEvents subscribes to events and processes them.
func (s *Server) processEvents() {
	eventCh := s.eventCollector.Subscribe()

	for event := range eventCh {
		switch event.Type {
		case events.EventRequestStarted:
			s.requestStore.RecordStart(event)
			s.metricsAgg.RecordRequestStart()

			// Broadcast to SSE clients
			s.sseBroker.Broadcast(SSEEvent{
				Type: "request_started",
				Data: event.Data,
			})

		case events.EventRequestCompleted:
			s.requestStore.RecordCompletion(event)
			s.metricsAgg.RecordRequestEnd(event)

			// Broadcast to SSE clients
			s.sseBroker.Broadcast(SSEEvent{
				Type: "request_completed",
				Data: event.Data,
			})

		case events.EventConnectionEstablished:
			data, ok := event.Data.(events.ConnectionEvent)
			if ok {
				s.mu.Lock()
				s.tunnelStatus = TunnelStatus{
					Connected:   true,
					TunnelID:    data.TunnelID,
					PublicURL:   data.PublicURL,
					LocalAddr:   data.LocalAddr,
					Protocol:    data.Protocol,
					ConnectedAt: event.Timestamp,
				}
				s.mu.Unlock()

				s.sseBroker.Broadcast(SSEEvent{
					Type: "connection_established",
					Data: s.tunnelStatus,
				})
			}

		case events.EventConnectionLost:
			s.mu.Lock()
			s.tunnelStatus.Connected = false
			s.mu.Unlock()

			s.sseBroker.Broadcast(SSEEvent{
				Type: "connection_lost",
				Data: map[string]bool{
					"connected": false,
				},
			})
		}
	}
}

// broadcastMetrics periodically broadcasts metrics snapshots.
func (s *Server) broadcastMetrics() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.RLock()
		if !s.running {
			s.mu.RUnlock()
			return
		}
		s.mu.RUnlock()

		snapshot := s.metricsAgg.GetSnapshot()

		s.sseBroker.Broadcast(SSEEvent{
			Type: "metrics_update",
			Data: snapshot,
		})
	}
}

// GetEventCollector returns the event collector for publishing events.
func (s *Server) GetEventCollector() *events.EventCollector {
	return s.eventCollector
}

// Port returns the port the server is running on.
func (s *Server) Port() int {
	return s.cfg.Port
}

// getUptime returns the server uptime.
func (s *Server) getUptime() time.Duration {
	return time.Since(s.startTime)
}

// Close gracefully shuts down the dashboard server.
func (s *Server) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	s.running = false

	logger.InfoEvent().Msg("Shutting down dashboard server")

	// Close components
	s.eventCollector.Close()
	s.sseBroker.Close()

	// Shutdown HTTP server
	if s.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := s.httpServer.Shutdown(ctx); err != nil {
			return fmt.Errorf("failed to shutdown HTTP server: %w", err)
		}
	}

	logger.InfoEvent().Msg("Dashboard server shut down successfully")

	return nil
}
