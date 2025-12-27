package proxy

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	tunnelv1 "github.com/pandeptwidyaop/grok/gen/proto/tunnel/v1"
	"github.com/pandeptwidyaop/grok/pkg/logger"
)

// tcpBufferPool pools 32KB buffers for TCP read operations to reduce GC pressure.
var tcpBufferPool = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, 32*1024) // 32KB
		return &buf
	},
}

// TCPConnection represents a persistent TCP connection to local service.
type TCPConnection struct {
	conn            net.Conn
	requestID       string
	mu              sync.Mutex
	closed          bool
	readLoopStarted bool
}

// TCPForwarder manages TCP connections and forwards data to local service.
type TCPForwarder struct {
	localAddr   string
	connections sync.Map // requestID → *TCPConnection
}

// NewTCPForwarder creates a new TCP forwarder.
func NewTCPForwarder(localAddr string) *TCPForwarder {
	return &TCPForwarder{
		localAddr: localAddr,
	}
}

// Forward forwards TCP data to local service and returns whether to start read loop.
func (f *TCPForwarder) Forward(ctx context.Context, requestID string, data *tunnelv1.TCPData, _ func(*tunnelv1.TCPData) error) (startReadLoop bool, err error) {
	// Handle connection close signal (empty data)
	if len(data.Data) == 0 {
		logger.DebugEvent().
			Str("request_id", requestID).
			Msg("Received TCP close signal")
		f.closeConnection(requestID)
		return false, nil
	}

	// Get or create connection
	tcpConn, isNew, err := f.getOrCreateConnection(ctx, requestID)
	if err != nil {
		return false, fmt.Errorf("failed to get connection: %w", err)
	}

	// Write data to local service
	tcpConn.mu.Lock()
	if tcpConn.closed {
		tcpConn.mu.Unlock()
		return false, fmt.Errorf("connection already closed")
	}

	_, err = tcpConn.conn.Write(data.Data)

	// Check if we should start read loop (only for new connections)
	shouldStartReadLoop := isNew && !tcpConn.readLoopStarted
	if shouldStartReadLoop {
		tcpConn.readLoopStarted = true
	}
	tcpConn.mu.Unlock()

	if err != nil {
		logger.ErrorEvent().
			Err(err).
			Str("request_id", requestID).
			Msg("Failed to write to local service")
		f.closeConnection(requestID)
		return false, fmt.Errorf("failed to write to local service: %w", err)
	}

	logger.DebugEvent().
		Str("request_id", requestID).
		Int("bytes", len(data.Data)).
		Msg("Forwarded TCP data to local service")

	return shouldStartReadLoop, nil
}

// getOrCreateConnection gets existing connection or creates new one, returns (conn, isNew, error).
func (f *TCPForwarder) getOrCreateConnection(ctx context.Context, requestID string) (*TCPConnection, bool, error) {
	// Check if connection exists
	if conn, ok := f.connections.Load(requestID); ok {
		tcpConn, ok := conn.(*TCPConnection)
		if !ok {
			return nil, false, fmt.Errorf("invalid connection type")
		}
		return tcpConn, false, nil
	}

	// Create new connection
	dialer := &net.Dialer{
		Timeout: 10 * time.Second,
	}

	conn, err := dialer.DialContext(ctx, "tcp", f.localAddr)
	if err != nil {
		return nil, false, fmt.Errorf("failed to connect to %s: %w", f.localAddr, err)
	}

	tcpConn := &TCPConnection{
		conn:            conn,
		requestID:       requestID,
		closed:          false,
		readLoopStarted: false,
	}

	f.connections.Store(requestID, tcpConn)

	logger.InfoEvent().
		Str("request_id", requestID).
		Str("local_addr", f.localAddr).
		Msg("Established new TCP connection to local service")

	return tcpConn, true, nil
}

// StartReadLoop starts reading from a TCP connection and sends responses back.
func (f *TCPForwarder) StartReadLoop(ctx context.Context, requestID string, sendResponse func(*tunnelv1.TCPData) error) {
	conn, ok := f.connections.Load(requestID)
	if !ok {
		logger.WarnEvent().
			Str("request_id", requestID).
			Msg("Connection not found for read loop")
		return
	}

	tcpConn, ok := conn.(*TCPConnection)
	if !ok {
		logger.ErrorEvent().
			Str("request_id", requestID).
			Msg("Invalid connection type in read loop")
		return
	}

	// Get buffer from pool
	bufPtr := tcpBufferPool.Get().(*[]byte) //nolint:errcheck // sync.Pool.Get() doesn't return error
	buffer := *bufPtr
	defer tcpBufferPool.Put(bufPtr)

	for {
		select {
		case <-ctx.Done():
			f.closeConnection(requestID)
			return
		default:
		}

		// Set read deadline to allow periodic context checks
		if err := tcpConn.conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
			logger.WarnEvent().Err(err).Str("request_id", requestID).Msg("Failed to set read deadline")
		}

		n, err := tcpConn.conn.Read(buffer)
		if err != nil {
			if err == io.EOF {
				// Normal connection close
				logger.DebugEvent().
					Str("request_id", requestID).
					Msg("Local service closed connection")
			} else if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				// Timeout - continue reading
				continue
			} else {
				logger.ErrorEvent().
					Err(err).
					Str("request_id", requestID).
					Msg("Error reading from local service")
			}

			// Send close signal (empty data)
			closeData := &tunnelv1.TCPData{
				Data:     []byte{},
				Sequence: 0,
			}
			if err := sendResponse(closeData); err != nil {
				logger.WarnEvent().Err(err).Str("request_id", requestID).Msg("Failed to send close signal")
			}
			f.closeConnection(requestID)
			return
		}

		if n > 0 {
			// Send response data
			respData := &tunnelv1.TCPData{
				Data:     append([]byte(nil), buffer[:n]...), // Copy buffer
				Sequence: 0,                                  // TODO: implement proper sequencing
			}

			if err := sendResponse(respData); err != nil {
				logger.ErrorEvent().
					Err(err).
					Str("request_id", requestID).
					Msg("Failed to send TCP response")
				f.closeConnection(requestID)
				return
			}

			logger.DebugEvent().
				Str("request_id", requestID).
				Int("bytes", n).
				Msg("Sent TCP response from local service")
		}
	}
}

// closeConnection closes and removes a connection.
func (f *TCPForwarder) closeConnection(requestID string) {
	if conn, ok := f.connections.LoadAndDelete(requestID); ok {
		tcpConn, ok := conn.(*TCPConnection)
		if !ok {
			logger.ErrorEvent().Str("request_id", requestID).Msg("Invalid connection type in close")
			return
		}
		tcpConn.mu.Lock()
		if !tcpConn.closed {
			tcpConn.conn.Close()
			tcpConn.closed = true
			logger.InfoEvent().
				Str("request_id", requestID).
				Msg("Closed TCP connection to local service")
		}
		tcpConn.mu.Unlock()
	}
}

// Close closes all connections.
func (f *TCPForwarder) Close() {
	f.connections.Range(func(key, _ interface{}) bool {
		requestID, ok := key.(string)
		if !ok {
			return true // Skip invalid entries
		}
		f.closeConnection(requestID)
		return true
	})
}
