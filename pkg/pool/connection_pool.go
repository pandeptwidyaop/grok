package pool

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pandeptwidyaop/grok/pkg/logger"
)

var (
	// ErrPoolClosed is returned when attempting to use a closed pool.
	ErrPoolClosed = errors.New("connection pool is closed")
	// ErrPoolExhausted is returned when pool is at max capacity and no connections available.
	ErrPoolExhausted = errors.New("connection pool exhausted")
	// ErrInvalidConfig is returned when pool configuration is invalid.
	ErrInvalidConfig = errors.New("invalid pool configuration")
)

// ConnectionFactory creates new connections.
type ConnectionFactory func() (net.Conn, error)

// Config holds connection pool configuration.
type Config struct {
	// MinSize is the minimum number of connections to maintain in the pool.
	MinSize int
	// MaxSize is the maximum number of connections allowed in the pool.
	MaxSize int
	// IdleTimeout is the maximum time a connection can be idle before being closed.
	IdleTimeout time.Duration
	// HealthCheckInterval is the interval between health checks.
	HealthCheckInterval time.Duration
	// MaxWaitTime is the maximum time to wait for a connection when pool is exhausted.
	MaxWaitTime time.Duration
	// Factory creates new connections.
	Factory ConnectionFactory
}

// DefaultConfig returns default pool configuration.
func DefaultConfig(factory ConnectionFactory) Config {
	return Config{
		MinSize:             10,
		MaxSize:             100,
		IdleTimeout:         90 * time.Second,
		HealthCheckInterval: 30 * time.Second,
		MaxWaitTime:         5 * time.Second,
		Factory:             factory,
	}
}

// Validate validates pool configuration.
func (c Config) Validate() error {
	if c.MinSize < 0 {
		return fmt.Errorf("%w: MinSize must be >= 0", ErrInvalidConfig)
	}
	if c.MaxSize <= 0 {
		return fmt.Errorf("%w: MaxSize must be > 0", ErrInvalidConfig)
	}
	if c.MinSize > c.MaxSize {
		return fmt.Errorf("%w: MinSize must be <= MaxSize", ErrInvalidConfig)
	}
	if c.IdleTimeout <= 0 {
		return fmt.Errorf("%w: IdleTimeout must be > 0", ErrInvalidConfig)
	}
	if c.Factory == nil {
		return fmt.Errorf("%w: Factory must not be nil", ErrInvalidConfig)
	}
	return nil
}

// PoolMetrics tracks connection pool statistics.
// Metrics tracks connection pool statistics.
type Metrics struct {
	// TotalConnections is the total number of connections created.
	TotalConnections atomic.Int64
	// ActiveConnections is the current number of active connections.
	ActiveConnections atomic.Int64
	// IdleConnections is the current number of idle connections in pool.
	IdleConnections atomic.Int64
	// ReuseCount is the total number of connection reuses.
	ReuseCount atomic.Int64
	// WaitCount is the total number of times callers waited for a connection.
	WaitCount atomic.Int64
	// TotalWaitTime is the cumulative time spent waiting for connections.
	TotalWaitTime atomic.Int64 // nanoseconds
	// HealthCheckFailures is the total number of health check failures.
	HealthCheckFailures atomic.Int64
}

// ReuseRate returns the connection reuse rate (0.0 - 1.0).
func (m *Metrics) ReuseRate() float64 {
	total := m.TotalConnections.Load()
	if total == 0 {
		return 0
	}
	reused := m.ReuseCount.Load()
	return float64(reused) / float64(total+reused)
}

// AvgWaitTime returns the average wait time in milliseconds.
func (m *Metrics) AvgWaitTime() float64 {
	count := m.WaitCount.Load()
	if count == 0 {
		return 0
	}
	totalNs := m.TotalWaitTime.Load()
	return float64(totalNs) / float64(count) / 1e6 // convert to ms
}

// pooledConn wraps a connection with metadata.
type pooledConn struct {
	conn      net.Conn
	createdAt time.Time
	lastUsed  time.Time
	useCount  int64
	pool      *ConnectionPool
}

// Read implements net.Conn.
func (pc *pooledConn) Read(b []byte) (int, error) {
	return pc.conn.Read(b)
}

// Write implements net.Conn.
func (pc *pooledConn) Write(b []byte) (int, error) {
	return pc.conn.Write(b)
}

// Close returns the connection to the pool instead of closing it.
func (pc *pooledConn) Close() error {
	return pc.pool.Put(pc)
}

// LocalAddr implements net.Conn.
func (pc *pooledConn) LocalAddr() net.Addr {
	return pc.conn.LocalAddr()
}

// RemoteAddr implements net.Conn.
func (pc *pooledConn) RemoteAddr() net.Addr {
	return pc.conn.RemoteAddr()
}

// SetDeadline implements net.Conn.
func (pc *pooledConn) SetDeadline(t time.Time) error {
	return pc.conn.SetDeadline(t)
}

// SetReadDeadline implements net.Conn.
func (pc *pooledConn) SetReadDeadline(t time.Time) error {
	return pc.conn.SetReadDeadline(t)
}

// SetWriteDeadline implements net.Conn.
func (pc *pooledConn) SetWriteDeadline(t time.Time) error {
	return pc.conn.SetWriteDeadline(t)
}

// forceClose actually closes the underlying connection.
func (pc *pooledConn) forceClose() error {
	return pc.conn.Close()
}

// isHealthy checks if the connection is still healthy.
func (pc *pooledConn) isHealthy() bool {
	// Check if connection is too old
	if time.Since(pc.lastUsed) > pc.pool.config.IdleTimeout {
		return false
	}

	// Actually test the connection with a read
	// Set a very short deadline for the test
	if err := pc.conn.SetReadDeadline(time.Now().Add(1 * time.Millisecond)); err != nil {
		return false
	}

	// Try to read 1 byte (will timeout if connection is healthy but no data)
	// This actually tests if the connection is alive
	one := make([]byte, 1)
	_, err := pc.conn.Read(one)

	// Reset deadline
	_ = pc.conn.SetReadDeadline(time.Time{})

	// Connection is healthy if:
	// - Read timed out (no data, but connection alive) OR
	// - Got data (connection alive and has data)
	// Connection is dead if:
	// - EOF or other error
	if err == nil {
		return true // Got data, connection is alive
	}

	// Check if it's a timeout error (expected for healthy idle connection)
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return true // Timeout is expected for healthy idle connection
	}

	// Any other error means connection is dead
	return false
}

// ConnectionPool manages a pool of reusable connections.
type ConnectionPool struct {
	config    Config
	pool      chan *pooledConn
	metrics   Metrics
	mu        sync.RWMutex
	closed    bool
	stopCh    chan struct{}
	wg        sync.WaitGroup
	closeOnce sync.Once // Ensure Close() is only called once
}

// NewConnectionPool creates a new connection pool.
func NewConnectionPool(config Config) (*ConnectionPool, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	p := &ConnectionPool{
		config: config,
		pool:   make(chan *pooledConn, config.MaxSize),
		stopCh: make(chan struct{}),
	}

	// Pre-fill pool with minimum connections
	for i := 0; i < config.MinSize; i++ {
		conn, err := p.createConnection()
		if err != nil {
			logger.WarnEvent().
				Err(err).
				Int("attempt", i+1).
				Int("min_size", config.MinSize).
				Msg("Failed to create initial connection, will retry later")
			continue
		}

		select {
		case p.pool <- conn:
			p.metrics.IdleConnections.Add(1)
		default:
			// Pool full, close connection
			_ = conn.forceClose()
		}
	}

	// Start health check goroutine if interval is set
	if config.HealthCheckInterval > 0 {
		p.wg.Add(1)
		go p.healthCheckLoop()
	}

	logger.InfoEvent().
		Int("min_size", config.MinSize).
		Int("max_size", config.MaxSize).
		Dur("idle_timeout", config.IdleTimeout).
		Msg("Connection pool initialized")

	return p, nil
}

// Get retrieves a connection from the pool or creates a new one.
func (p *ConnectionPool) Get(ctx context.Context) (net.Conn, error) {
	p.mu.RLock()
	if p.closed {
		p.mu.RUnlock()
		return nil, ErrPoolClosed
	}
	p.mu.RUnlock()

	start := time.Now()

	// Try to get from pool first (non-blocking)
	select {
	case conn := <-p.pool:
		p.metrics.IdleConnections.Add(-1)
		p.metrics.ActiveConnections.Add(1)
		p.metrics.ReuseCount.Add(1)

		// Update connection metadata
		conn.lastUsed = time.Now()
		conn.useCount++

		logger.DebugEvent().
			Int64("use_count", conn.useCount).
			Dur("age", time.Since(conn.createdAt)).
			Msg("Reused connection from pool")

		return conn, nil

	default:
		// Pool empty, try to create new connection
		if p.metrics.ActiveConnections.Load()+p.metrics.IdleConnections.Load() < int64(p.config.MaxSize) {
			conn, err := p.createConnection()
			if err != nil {
				return nil, fmt.Errorf("failed to create connection: %w", err)
			}

			p.metrics.ActiveConnections.Add(1)
			return conn, nil
		}

		// Pool exhausted, wait for a connection with timeout
		p.metrics.WaitCount.Add(1)

		waitTimeout := p.config.MaxWaitTime
		if deadline, ok := ctx.Deadline(); ok {
			if remaining := time.Until(deadline); remaining < waitTimeout {
				waitTimeout = remaining
			}
		}

		select {
		case conn := <-p.pool:
			waitTime := time.Since(start)
			p.metrics.TotalWaitTime.Add(waitTime.Nanoseconds())
			p.metrics.IdleConnections.Add(-1)
			p.metrics.ActiveConnections.Add(1)
			p.metrics.ReuseCount.Add(1)

			conn.lastUsed = time.Now()
			atomic.AddInt64(&conn.useCount, 1)

			logger.DebugEvent().
				Dur("wait_time", waitTime).
				Int64("use_count", conn.useCount).
				Msg("Got connection after waiting")

			return conn, nil

		case <-time.After(waitTimeout):
			logger.WarnEvent().
				Dur("wait_time", waitTimeout).
				Int64("active", p.metrics.ActiveConnections.Load()).
				Int64("idle", p.metrics.IdleConnections.Load()).
				Msg("Connection pool exhausted, timeout waiting for connection")
			return nil, ErrPoolExhausted

		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// Put returns a connection to the pool.
func (p *ConnectionPool) Put(conn net.Conn) error {
	p.mu.RLock()
	closed := p.closed
	p.mu.RUnlock()

	pc, ok := conn.(*pooledConn)
	if !ok {
		return fmt.Errorf("invalid connection type")
	}

	p.metrics.ActiveConnections.Add(-1)

	// If pool is closed or connection is unhealthy, close it
	if closed || !pc.isHealthy() {
		p.metrics.IdleConnections.Add(-1)
		return pc.forceClose()
	}

	// Try to return to pool (non-blocking)
	select {
	case p.pool <- pc:
		p.metrics.IdleConnections.Add(1)
		logger.DebugEvent().
			Int64("use_count", atomic.LoadInt64(&pc.useCount)).
			Dur("age", time.Since(pc.createdAt)).
			Msg("Returned connection to pool")
		return nil

	default:
		// Pool full, close connection
		logger.DebugEvent().Msg("Pool full, closing connection")
		return pc.forceClose()
	}
}

// createConnection creates a new pooled connection.
func (p *ConnectionPool) createConnection() (*pooledConn, error) {
	conn, err := p.config.Factory()
	if err != nil {
		return nil, err
	}

	pc := &pooledConn{
		conn:      conn,
		createdAt: time.Now(),
		lastUsed:  time.Now(),
		useCount:  0,
		pool:      p,
	}

	p.metrics.TotalConnections.Add(1)

	logger.DebugEvent().
		Int64("total_created", p.metrics.TotalConnections.Load()).
		Msg("Created new connection")

	return pc, nil
}

// healthCheckLoop periodically checks and removes unhealthy connections.
func (p *ConnectionPool) healthCheckLoop() {
	defer p.wg.Done()

	ticker := time.NewTicker(p.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.performHealthCheck()

		case <-p.stopCh:
			return
		}
	}
}

// performHealthCheck checks all idle connections and removes unhealthy ones.
// Uses a snapshot approach to avoid race conditions with Get().
func (p *ConnectionPool) performHealthCheck() {
	p.mu.RLock()
	if p.closed {
		p.mu.RUnlock()
		return
	}
	p.mu.RUnlock()

	checked := 0
	removed := 0
	toCheck := make([]*pooledConn, 0, p.config.MaxSize)

	// Phase 1: Collect connections to check (with timeout to avoid blocking)
	collectTimeout := time.After(100 * time.Millisecond)
collectLoop:
	for {
		select {
		case conn := <-p.pool:
			p.metrics.IdleConnections.Add(-1)
			toCheck = append(toCheck, conn)
		case <-collectTimeout:
			break collectLoop
		default:
			// No more connections available
			break collectLoop
		}
	}

	// Phase 2: Check health and return to pool
	for _, conn := range toCheck {
		checked++
		if conn.isHealthy() {
			// Return healthy connection to pool
			select {
			case p.pool <- conn:
				p.metrics.IdleConnections.Add(1)
			default:
				// Pool full (shouldn't happen, but handle gracefully)
				_ = conn.forceClose()
				removed++
			}
		} else {
			// Close unhealthy connection
			_ = conn.forceClose()
			removed++
			p.metrics.HealthCheckFailures.Add(1)
		}
	}

	if removed > 0 {
		logger.InfoEvent().
			Int("checked", checked).
			Int("removed", removed).
			Int64("idle", p.metrics.IdleConnections.Load()).
			Msg("Health check completed")
	}
}

// Close gracefully shuts down the pool and closes all connections.
// Safe to call multiple times (idempotent).
func (p *ConnectionPool) Close() error {
	var err error
	p.closeOnce.Do(func() {
		p.mu.Lock()
		p.closed = true
		p.mu.Unlock()

		// Stop health check goroutine
		close(p.stopCh)
		p.wg.Wait()

		// Close all connections in pool
		close(p.pool)
		count := 0
		for conn := range p.pool {
			_ = conn.forceClose()
			count++
		}

		logger.InfoEvent().
			Int("closed_connections", count).
			Float64("reuse_rate", p.metrics.ReuseRate()).
			Float64("avg_wait_ms", p.metrics.AvgWaitTime()).
			Msg("Connection pool closed")
	})

	return err
}

// Metrics returns a snapshot of the current metrics.
func (p *ConnectionPool) Metrics() *Metrics {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return &p.metrics
}

// Stats returns pool statistics as a map.
func (p *ConnectionPool) Stats() map[string]interface{} {
	return map[string]interface{}{
		"total_connections":     p.metrics.TotalConnections.Load(),
		"active_connections":    p.metrics.ActiveConnections.Load(),
		"idle_connections":      p.metrics.IdleConnections.Load(),
		"reuse_count":           p.metrics.ReuseCount.Load(),
		"reuse_rate":            p.metrics.ReuseRate(),
		"wait_count":            p.metrics.WaitCount.Load(),
		"avg_wait_time_ms":      p.metrics.AvgWaitTime(),
		"health_check_failures": p.metrics.HealthCheckFailures.Load(),
	}
}
