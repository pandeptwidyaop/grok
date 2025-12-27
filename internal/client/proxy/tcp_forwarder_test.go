package proxy

import (
	"context"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	tunnelv1 "github.com/pandeptwidyaop/grok/gen/proto/tunnel/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewTCPForwarder tests TCP forwarder creation
func TestNewTCPForwarder(t *testing.T) {
	forwarder := NewTCPForwarder("localhost:22")

	require.NotNil(t, forwarder)
	assert.Equal(t, "localhost:22", forwarder.localAddr)
}

// TestTCPForwarder_Forward_NewConnection tests creating new connection
func TestTCPForwarder_Forward_NewConnection(t *testing.T) {
	// Create test TCP server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	serverAddr := listener.Addr().String()

	// Server goroutine - echo server
	var serverWg sync.WaitGroup
	serverWg.Add(1)
	go func() {
		defer serverWg.Done()
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		// Read data
		buf := make([]byte, 1024)
		n, _ := conn.Read(buf)
		// Echo back
		conn.Write(buf[:n])
	}()

	forwarder := NewTCPForwarder(serverAddr)

	data := &tunnelv1.TCPData{
		Data:     []byte("Hello TCP"),
		Sequence: 1,
	}

	startReadLoop, err := forwarder.Forward(context.Background(), "req-1", data, nil)

	require.NoError(t, err)
	assert.True(t, startReadLoop) // Should start read loop for new connection

	// Cleanup
	forwarder.Close()
	serverWg.Wait()
}

// TestTCPForwarder_Forward_ExistingConnection tests reusing existing connection
func TestTCPForwarder_Forward_ExistingConnection(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	serverAddr := listener.Addr().String()

	// Server goroutine
	var serverWg sync.WaitGroup
	serverWg.Add(1)
	go func() {
		defer serverWg.Done()
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		// Read multiple messages
		buf := make([]byte, 1024)
		for i := 0; i < 2; i++ {
			n, err := conn.Read(buf)
			if err != nil {
				return
			}
			conn.Write(buf[:n])
		}
	}()

	forwarder := NewTCPForwarder(serverAddr)

	// First message - creates connection
	data1 := &tunnelv1.TCPData{Data: []byte("First"), Sequence: 1}
	startReadLoop1, err := forwarder.Forward(context.Background(), "req-1", data1, nil)
	require.NoError(t, err)
	assert.True(t, startReadLoop1)

	// Second message - reuses connection
	data2 := &tunnelv1.TCPData{Data: []byte("Second"), Sequence: 2}
	startReadLoop2, err := forwarder.Forward(context.Background(), "req-1", data2, nil)
	require.NoError(t, err)
	assert.False(t, startReadLoop2) // Should NOT start read loop again

	// Cleanup
	forwarder.Close()
	serverWg.Wait()
}

// TestTCPForwarder_Forward_CloseSignal tests connection close with empty data
func TestTCPForwarder_Forward_CloseSignal(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	serverAddr := listener.Addr().String()

	var serverWg sync.WaitGroup
	serverWg.Add(1)
	go func() {
		defer serverWg.Done()
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		conn.Close()
	}()

	forwarder := NewTCPForwarder(serverAddr)

	// Create connection
	data := &tunnelv1.TCPData{Data: []byte("Hello"), Sequence: 1}
	_, err = forwarder.Forward(context.Background(), "req-1", data, nil)
	require.NoError(t, err)

	// Send close signal (empty data)
	closeSignal := &tunnelv1.TCPData{Data: []byte{}, Sequence: 0}
	startReadLoop, err := forwarder.Forward(context.Background(), "req-1", closeSignal, nil)

	require.NoError(t, err)
	assert.False(t, startReadLoop)

	// Connection should be removed
	time.Sleep(100 * time.Millisecond)
	_, ok := forwarder.connections.Load("req-1")
	assert.False(t, ok)

	serverWg.Wait()
}

// TestTCPForwarder_Forward_InvalidLocalAddr tests connection error
func TestTCPForwarder_Forward_InvalidLocalAddr(t *testing.T) {
	forwarder := NewTCPForwarder("localhost:99999")

	data := &tunnelv1.TCPData{
		Data:     []byte("Test"),
		Sequence: 1,
	}

	startReadLoop, err := forwarder.Forward(context.Background(), "req-1", data, nil)

	assert.Error(t, err)
	assert.False(t, startReadLoop)
	assert.Contains(t, err.Error(), "failed to get connection")
}

// TestTCPForwarder_Forward_WriteToClosed tests writing to closed connection
func TestTCPForwarder_Forward_WriteToClosed(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	serverAddr := listener.Addr().String()

	var serverWg sync.WaitGroup
	serverWg.Add(1)
	go func() {
		defer serverWg.Done()
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		// Close immediately
		conn.Close()
	}()

	forwarder := NewTCPForwarder(serverAddr)

	// Create connection
	data1 := &tunnelv1.TCPData{Data: []byte("Hello"), Sequence: 1}
	_, err = forwarder.Forward(context.Background(), "req-1", data1, nil)
	require.NoError(t, err)

	// Close the connection
	forwarder.closeConnection("req-1")

	// Try to write to closed connection
	data2 := &tunnelv1.TCPData{Data: []byte("After close"), Sequence: 2}
	_, err = forwarder.Forward(context.Background(), "req-1", data2, nil)

	// Should create new connection since old one was closed
	assert.NoError(t, err)

	serverWg.Wait()
}

// TestTCPForwarder_StartReadLoop tests TCP read loop
func TestTCPForwarder_StartReadLoop(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	serverAddr := listener.Addr().String()

	// Server sends data
	var serverWg sync.WaitGroup
	serverWg.Add(1)
	go func() {
		defer serverWg.Done()
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		// Send some data
		conn.Write([]byte("Hello from server"))

		// Wait a bit
		time.Sleep(200 * time.Millisecond)
	}()

	forwarder := NewTCPForwarder(serverAddr)

	// Create connection
	data := &tunnelv1.TCPData{Data: []byte("Client hello"), Sequence: 1}
	_, err = forwarder.Forward(context.Background(), "req-1", data, nil)
	require.NoError(t, err)

	// Collect responses
	var responses []*tunnelv1.TCPData
	var responseMu sync.Mutex

	sendResponse := func(data *tunnelv1.TCPData) error {
		responseMu.Lock()
		responses = append(responses, data)
		responseMu.Unlock()
		return nil
	}

	// Start read loop
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	go forwarder.StartReadLoop(ctx, "req-1", sendResponse)

	// Wait for response
	time.Sleep(500 * time.Millisecond)

	responseMu.Lock()
	receivedData := len(responses) > 0
	responseMu.Unlock()

	assert.True(t, receivedData, "Should receive data from server")

	if receivedData {
		responseMu.Lock()
		assert.Equal(t, "Hello from server", string(responses[0].Data))
		responseMu.Unlock()
	}

	forwarder.Close()
	serverWg.Wait()
}

// TestTCPForwarder_StartReadLoop_EOF tests read loop handling EOF
func TestTCPForwarder_StartReadLoop_EOF(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	serverAddr := listener.Addr().String()

	var serverWg sync.WaitGroup
	serverWg.Add(1)
	go func() {
		defer serverWg.Done()
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		// Close connection immediately (EOF)
		conn.Close()
	}()

	forwarder := NewTCPForwarder(serverAddr)

	// Create connection
	data := &tunnelv1.TCPData{Data: []byte("Hello"), Sequence: 1}
	_, err = forwarder.Forward(context.Background(), "req-1", data, nil)
	require.NoError(t, err)

	// Collect close signals
	var receivedClose bool
	var closeMu sync.Mutex

	sendResponse := func(data *tunnelv1.TCPData) error {
		closeMu.Lock()
		if len(data.Data) == 0 {
			receivedClose = true
		}
		closeMu.Unlock()
		return nil
	}

	// Start read loop
	ctx := context.Background()
	forwarder.StartReadLoop(ctx, "req-1", sendResponse)

	closeMu.Lock()
	gotClose := receivedClose
	closeMu.Unlock()

	assert.True(t, gotClose, "Should receive close signal on EOF")

	serverWg.Wait()
}

// TestTCPForwarder_StartReadLoop_SendError tests read loop handling send error
func TestTCPForwarder_StartReadLoop_SendError(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	serverAddr := listener.Addr().String()

	var serverWg sync.WaitGroup
	serverWg.Add(1)
	go func() {
		defer serverWg.Done()
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		// Send data
		conn.Write([]byte("Test data"))

		time.Sleep(200 * time.Millisecond)
	}()

	forwarder := NewTCPForwarder(serverAddr)

	// Create connection
	data := &tunnelv1.TCPData{Data: []byte("Hello"), Sequence: 1}
	_, err = forwarder.Forward(context.Background(), "req-1", data, nil)
	require.NoError(t, err)

	// Send callback that returns error
	sendResponse := func(data *tunnelv1.TCPData) error {
		return io.EOF // Simulate send error
	}

	// Start read loop
	ctx := context.Background()
	forwarder.StartReadLoop(ctx, "req-1", sendResponse)

	// Connection should be closed
	time.Sleep(500 * time.Millisecond)
	_, ok := forwarder.connections.Load("req-1")
	assert.False(t, ok)

	serverWg.Wait()
}

// TestTCPForwarder_StartReadLoop_ContextCancel tests context cancellation
func TestTCPForwarder_StartReadLoop_ContextCancel(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	serverAddr := listener.Addr().String()

	var serverWg sync.WaitGroup
	serverWg.Add(1)
	go func() {
		defer serverWg.Done()
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		// Keep connection alive
		time.Sleep(10 * time.Second)
	}()

	forwarder := NewTCPForwarder(serverAddr)

	// Create connection
	data := &tunnelv1.TCPData{Data: []byte("Hello"), Sequence: 1}
	_, err = forwarder.Forward(context.Background(), "req-1", data, nil)
	require.NoError(t, err)

	// Start read loop with cancelable context
	ctx, cancel := context.WithCancel(context.Background())

	go forwarder.StartReadLoop(ctx, "req-1", func(data *tunnelv1.TCPData) error {
		return nil
	})

	// Cancel context
	time.Sleep(100 * time.Millisecond)
	cancel()

	// Connection should be closed after context cancellation
	// Note: Read loop has 5-second timeout, so may take up to 5 seconds to detect cancellation
	time.Sleep(6 * time.Second)
	_, ok := forwarder.connections.Load("req-1")
	assert.False(t, ok)

	serverWg.Wait()
}

// TestTCPForwarder_StartReadLoop_NonexistentConnection tests read loop on missing connection
func TestTCPForwarder_StartReadLoop_NonexistentConnection(t *testing.T) {
	forwarder := NewTCPForwarder("localhost:12345")

	// Try to start read loop on connection that doesn't exist
	sendResponse := func(data *tunnelv1.TCPData) error {
		return nil
	}

	// Should return without error (just logs warning)
	forwarder.StartReadLoop(context.Background(), "nonexistent", sendResponse)

	// No crash = success
}

// TestTCPForwarder_Close tests closing all connections
func TestTCPForwarder_Close(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	serverAddr := listener.Addr().String()

	// Accept multiple connections
	var serverWg sync.WaitGroup
	for i := 0; i < 3; i++ {
		serverWg.Add(1)
		go func() {
			defer serverWg.Done()
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			defer conn.Close()

			// Keep alive
			buf := make([]byte, 1024)
			conn.Read(buf)
			time.Sleep(1 * time.Second)
		}()
	}

	forwarder := NewTCPForwarder(serverAddr)

	// Create multiple connections
	for i := 0; i < 3; i++ {
		data := &tunnelv1.TCPData{Data: []byte("Hello"), Sequence: 1}
		_, err := forwarder.Forward(context.Background(), "req-"+string(rune(i)), data, nil)
		require.NoError(t, err)
	}

	// Verify connections exist
	count := 0
	forwarder.connections.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	assert.Equal(t, 3, count)

	// Close all connections
	forwarder.Close()

	// Verify all connections closed
	count = 0
	forwarder.connections.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	assert.Equal(t, 0, count)

	serverWg.Wait()
}

// TestTCPForwarder_Close_EmptyConnections tests closing with no connections
func TestTCPForwarder_Close_EmptyConnections(t *testing.T) {
	forwarder := NewTCPForwarder("localhost:12345")

	// Should not panic
	forwarder.Close()
}

// TestTCPForwarder_MultipleConnections tests managing multiple concurrent connections
func TestTCPForwarder_MultipleConnections(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	serverAddr := listener.Addr().String()

	// Accept connections
	var serverWg sync.WaitGroup
	for i := 0; i < 5; i++ {
		serverWg.Add(1)
		go func() {
			defer serverWg.Done()
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			defer conn.Close()

			buf := make([]byte, 1024)
			n, _ := conn.Read(buf)
			conn.Write(buf[:n]) // Echo
		}()
	}

	forwarder := NewTCPForwarder(serverAddr)

	// Create connections concurrently
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			requestID := "req-" + string(rune('A'+id))
			data := &tunnelv1.TCPData{
				Data:     []byte("Hello from " + requestID),
				Sequence: 1,
			}

			_, err := forwarder.Forward(context.Background(), requestID, data, nil)
			assert.NoError(t, err)
		}(i)
	}

	wg.Wait()

	// Verify all connections created
	count := 0
	forwarder.connections.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	assert.Equal(t, 5, count)

	forwarder.Close()
	serverWg.Wait()
}

// BenchmarkTCPForwarder_Forward benchmarks TCP forwarding
func BenchmarkTCPForwarder_Forward(b *testing.B) {
	listener, _ := net.Listen("tcp", "127.0.0.1:0")
	defer listener.Close()

	serverAddr := listener.Addr().String()

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 1024)
				for {
					n, err := c.Read(buf)
					if err != nil {
						return
					}
					c.Write(buf[:n])
				}
			}(conn)
		}
	}()

	forwarder := NewTCPForwarder(serverAddr)
	defer forwarder.Close()

	data := &tunnelv1.TCPData{
		Data:     []byte("Benchmark data"),
		Sequence: 1,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		forwarder.Forward(context.Background(), "bench-req", data, nil)
	}
}
