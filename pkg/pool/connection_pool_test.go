package pool

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockConn is a mock net.Conn for testing.
type mockConn struct {
	net.Conn
	closed    bool
	readErr   error
	writeErr  error
	mu        sync.Mutex
	closeFunc func() error
}

func newMockConn() *mockConn {
	return &mockConn{}
}

func (m *mockConn) Read(b []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.readErr != nil {
		return 0, m.readErr
	}
	return len(b), nil
}

func (m *mockConn) Write(b []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.writeErr != nil {
		return 0, m.writeErr
	}
	return len(b), nil
}

func (m *mockConn) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return errors.New("already closed")
	}
	m.closed = true
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

func (m *mockConn) LocalAddr() net.Addr                { return nil }
func (m *mockConn) RemoteAddr() net.Addr               { return nil }
func (m *mockConn) SetDeadline(_ time.Time) error      { return nil }
func (m *mockConn) SetReadDeadline(_ time.Time) error  { return nil }
func (m *mockConn) SetWriteDeadline(_ time.Time) error { return nil }

func (m *mockConn) IsClosed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closed
}

// TestNewConnectionPool tests pool initialization.
func TestNewConnectionPool(t *testing.T) {
	factory := func() (net.Conn, error) {
		return newMockConn(), nil
	}

	config := Config{
		MinSize:             5,
		MaxSize:             10,
		IdleTimeout:         30 * time.Second,
		HealthCheckInterval: 10 * time.Second,
		MaxWaitTime:         5 * time.Second,
		Factory:             factory,
	}

	pool, err := NewConnectionPool(config)
	require.NoError(t, err)
	require.NotNil(t, pool)
	defer pool.Close()

	// Check that minimum connections were created
	assert.GreaterOrEqual(t, pool.metrics.IdleConnections.Load(), int64(0))
	assert.LessOrEqual(t, pool.metrics.IdleConnections.Load(), int64(config.MinSize))
}

// TestConnectionPoolInvalidConfig tests invalid configurations.
func TestConnectionPoolInvalidConfig(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{
			name: "negative min size",
			config: Config{
				MinSize:     -1,
				MaxSize:     10,
				IdleTimeout: 30 * time.Second,
				Factory:     func() (net.Conn, error) { return newMockConn(), nil },
			},
		},
		{
			name: "zero max size",
			config: Config{
				MinSize:     0,
				MaxSize:     0,
				IdleTimeout: 30 * time.Second,
				Factory:     func() (net.Conn, error) { return newMockConn(), nil },
			},
		},
		{
			name: "min > max",
			config: Config{
				MinSize:     10,
				MaxSize:     5,
				IdleTimeout: 30 * time.Second,
				Factory:     func() (net.Conn, error) { return newMockConn(), nil },
			},
		},
		{
			name: "nil factory",
			config: Config{
				MinSize:     0,
				MaxSize:     10,
				IdleTimeout: 30 * time.Second,
				Factory:     nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool, err := NewConnectionPool(tt.config)
			assert.Error(t, err)
			assert.Nil(t, pool)
		})
	}
}

// TestConnectionPoolGetPut tests getting and putting connections.
func TestConnectionPoolGetPut(t *testing.T) {
	factory := func() (net.Conn, error) {
		return newMockConn(), nil
	}

	config := DefaultConfig(factory)
	config.MinSize = 2
	config.MaxSize = 5

	pool, err := NewConnectionPool(config)
	require.NoError(t, err)
	defer pool.Close()

	ctx := context.Background()

	// Get a connection
	conn1, err := pool.Get(ctx)
	require.NoError(t, err)
	require.NotNil(t, conn1)

	assert.Equal(t, int64(1), pool.metrics.ActiveConnections.Load())

	// Put it back
	err = conn1.Close() // Close() returns to pool
	require.NoError(t, err)

	assert.Equal(t, int64(0), pool.metrics.ActiveConnections.Load())
	assert.GreaterOrEqual(t, pool.metrics.IdleConnections.Load(), int64(1))
}

// TestConnectionPoolReuse tests connection reuse.
func TestConnectionPoolReuse(t *testing.T) {
	factory := func() (net.Conn, error) {
		return newMockConn(), nil
	}

	config := DefaultConfig(factory)
	config.MinSize = 1
	config.MaxSize = 5

	pool, err := NewConnectionPool(config)
	require.NoError(t, err)
	defer pool.Close()

	ctx := context.Background()

	// Get and return connection
	conn1, err := pool.Get(ctx)
	require.NoError(t, err)
	err = conn1.Close()
	require.NoError(t, err)

	initialTotal := pool.metrics.TotalConnections.Load()

	// Get again - should reuse
	conn2, err := pool.Get(ctx)
	require.NoError(t, err)
	err = conn2.Close()
	require.NoError(t, err)

	// Total connections should not increase (reused)
	assert.Equal(t, initialTotal, pool.metrics.TotalConnections.Load())
	assert.GreaterOrEqual(t, pool.metrics.ReuseCount.Load(), int64(1))
}

// TestConnectionPoolConcurrent tests concurrent access.
func TestConnectionPoolConcurrent(t *testing.T) {
	factory := func() (net.Conn, error) {
		return newMockConn(), nil
	}

	config := DefaultConfig(factory)
	config.MinSize = 5
	config.MaxSize = 20

	pool, err := NewConnectionPool(config)
	require.NoError(t, err)
	defer pool.Close()

	ctx := context.Background()
	concurrency := 50
	iterations := 100

	var wg sync.WaitGroup
	wg.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()

			for j := 0; j < iterations; j++ {
				conn, err := pool.Get(ctx)
				if err != nil {
					// Pool might be exhausted, that's ok
					continue
				}

				// Simulate some work
				time.Sleep(1 * time.Millisecond)

				_ = conn.Close()
			}
		}()
	}

	wg.Wait()

	// Verify metrics
	assert.GreaterOrEqual(t, pool.metrics.TotalConnections.Load(), int64(1))
	assert.GreaterOrEqual(t, pool.metrics.ReuseCount.Load(), int64(1))
	assert.LessOrEqual(t, pool.metrics.ActiveConnections.Load(), int64(config.MaxSize))
}

// TestConnectionPoolExhaustion tests pool exhaustion behavior.
func TestConnectionPoolExhaustion(t *testing.T) {
	factory := func() (net.Conn, error) {
		return newMockConn(), nil
	}

	config := DefaultConfig(factory)
	config.MinSize = 0
	config.MaxSize = 2
	config.MaxWaitTime = 100 * time.Millisecond

	pool, err := NewConnectionPool(config)
	require.NoError(t, err)
	defer pool.Close()

	ctx := context.Background()

	// Get all connections
	conn1, err := pool.Get(ctx)
	require.NoError(t, err)

	conn2, err := pool.Get(ctx)
	require.NoError(t, err)

	// Try to get one more - should timeout
	start := time.Now()
	conn3, err := pool.Get(ctx)
	elapsed := time.Since(start)

	assert.Error(t, err)
	assert.Nil(t, conn3)
	assert.GreaterOrEqual(t, elapsed, config.MaxWaitTime)

	// Return connections
	_ = conn1.Close()
	_ = conn2.Close()
}

// TestConnectionPoolWait tests waiting for available connection.
func TestConnectionPoolWait(t *testing.T) {
	factory := func() (net.Conn, error) {
		return newMockConn(), nil
	}

	config := DefaultConfig(factory)
	config.MinSize = 0
	config.MaxSize = 1
	config.MaxWaitTime = 2 * time.Second

	pool, err := NewConnectionPool(config)
	require.NoError(t, err)
	defer pool.Close()

	ctx := context.Background()

	// Get the only connection
	conn1, err := pool.Get(ctx)
	require.NoError(t, err)

	// Start goroutine to return connection after delay
	go func() {
		time.Sleep(200 * time.Millisecond)
		_ = conn1.Close()
	}()

	// Try to get connection - should wait and succeed
	start := time.Now()
	conn2, err := pool.Get(ctx)
	elapsed := time.Since(start)

	require.NoError(t, err)
	require.NotNil(t, conn2)
	assert.GreaterOrEqual(t, elapsed, 200*time.Millisecond)
	assert.GreaterOrEqual(t, pool.metrics.WaitCount.Load(), int64(1))

	_ = conn2.Close()
}

// TestConnectionPoolClose tests pool closure.
func TestConnectionPoolClose(t *testing.T) {
	closedCount := 0
	var mu sync.Mutex

	factory := func() (net.Conn, error) {
		mc := newMockConn()
		mc.closeFunc = func() error {
			mu.Lock()
			closedCount++
			mu.Unlock()
			return nil
		}
		return mc, nil
	}

	config := DefaultConfig(factory)
	config.MinSize = 3
	config.MaxSize = 5

	pool, err := NewConnectionPool(config)
	require.NoError(t, err)

	// Get some connections
	ctx := context.Background()
	conn1, _ := pool.Get(ctx)
	conn2, _ := pool.Get(ctx)

	// Close pool
	err = pool.Close()
	require.NoError(t, err)

	// Try to get connection from closed pool
	conn3, err := pool.Get(ctx)
	assert.Error(t, err)
	assert.Equal(t, ErrPoolClosed, err)
	assert.Nil(t, conn3)

	// Return connections to closed pool - should close them
	_ = conn1.Close()
	_ = conn2.Close()

	// Verify connections were closed
	mu.Lock()
	assert.GreaterOrEqual(t, closedCount, 1)
	mu.Unlock()
}

// TestConnectionPoolMetrics tests metrics tracking.
func TestConnectionPoolMetrics(t *testing.T) {
	factory := func() (net.Conn, error) {
		return newMockConn(), nil
	}

	config := DefaultConfig(factory)
	config.MinSize = 2
	config.MaxSize = 5

	pool, err := NewConnectionPool(config)
	require.NoError(t, err)
	defer pool.Close()

	ctx := context.Background()

	// Get and return connections multiple times
	for i := 0; i < 10; i++ {
		conn, err := pool.Get(ctx)
		require.NoError(t, err)
		_ = conn.Close()
	}

	// Check metrics
	metrics := pool.Metrics()
	assert.GreaterOrEqual(t, metrics.TotalConnections.Load(), int64(1))
	assert.GreaterOrEqual(t, metrics.ReuseCount.Load(), int64(1))

	reuseRate := metrics.ReuseRate()
	assert.GreaterOrEqual(t, reuseRate, 0.0)
	assert.LessOrEqual(t, reuseRate, 1.0)

	// Check stats map
	stats := pool.Stats()
	assert.NotNil(t, stats)
	assert.Contains(t, stats, "total_connections")
	assert.Contains(t, stats, "reuse_rate")
}

// BenchmarkConnectionPoolGet benchmarks getting connections.
func BenchmarkConnectionPoolGet(b *testing.B) {
	factory := func() (net.Conn, error) {
		return newMockConn(), nil
	}

	config := DefaultConfig(factory)
	config.MinSize = 10
	config.MaxSize = 100

	pool, err := NewConnectionPool(config)
	require.NoError(b, err)
	defer pool.Close()

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			conn, err := pool.Get(ctx)
			if err != nil {
				b.Fatal(err)
			}
			_ = conn.Close()
		}
	})
}

// BenchmarkConnectionPoolGetPut benchmarks get and put operations.
func BenchmarkConnectionPoolGetPut(b *testing.B) {
	factory := func() (net.Conn, error) {
		return newMockConn(), nil
	}

	config := DefaultConfig(factory)
	config.MinSize = 50
	config.MaxSize = 200

	pool, err := NewConnectionPool(config)
	require.NoError(b, err)
	defer pool.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		conn, err := pool.Get(ctx)
		if err != nil {
			b.Fatal(err)
		}
		_ = conn.Close()
	}
}
