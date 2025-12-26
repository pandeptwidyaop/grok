package proxy

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/google/uuid"

	tunnelv1 "github.com/pandeptwidyaop/grok/gen/proto/tunnel/v1"
	"github.com/pandeptwidyaop/grok/internal/server/tunnel"
	"github.com/pandeptwidyaop/grok/pkg/logger"
)

// TCPProxy manages TCP listeners for allocated ports.
type TCPProxy struct {
	tunnelManager *tunnel.Manager
	listeners     map[int]net.Listener // port → listener
	mu            sync.RWMutex
	done          chan struct{}
}

// NewTCPProxy creates a new TCP proxy manager.
func NewTCPProxy(tunnelManager *tunnel.Manager) *TCPProxy {
	return &TCPProxy{
		tunnelManager: tunnelManager,
		listeners:     make(map[int]net.Listener),
		done:          make(chan struct{}),
	}
}

// StartListener starts a TCP listener on the specified port for a tunnel.
func (tp *TCPProxy) StartListener(port int, tunnelID uuid.UUID) error {
	tp.mu.Lock()
	defer tp.mu.Unlock()

	// Check if listener already exists
	if _, exists := tp.listeners[port]; exists {
		return fmt.Errorf("listener already exists on port %d", port)
	}

	// Create TCP listener
	addr := fmt.Sprintf(":%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %w", port, err)
	}

	tp.listeners[port] = listener

	logger.InfoEvent().
		Int("port", port).
		Str("tunnel_id", tunnelID.String()).
		Msg("TCP listener started")

	// Accept connections in background
	go tp.acceptConnections(listener, port, tunnelID)

	return nil
}

// StopListener stops the TCP listener on the specified port.
func (tp *TCPProxy) StopListener(port int) error {
	tp.mu.Lock()
	defer tp.mu.Unlock()

	listener, exists := tp.listeners[port]
	if !exists {
		return fmt.Errorf("no listener found on port %d", port)
	}

	// Close the listener
	if err := listener.Close(); err != nil {
		logger.WarnEvent().
			Err(err).
			Int("port", port).
			Msg("Error closing TCP listener")
	}

	delete(tp.listeners, port)

	logger.InfoEvent().
		Int("port", port).
		Msg("TCP listener stopped")

	return nil
}

// acceptConnections accepts incoming TCP connections and forwards them.
func (tp *TCPProxy) acceptConnections(listener net.Listener, port int, tunnelID uuid.UUID) {
	for {
		select {
		case <-tp.done:
			return
		default:
		}

		// Set a deadline for Accept to allow checking for shutdown
		if tcpListener, ok := listener.(*net.TCPListener); ok {
			tcpListener.SetDeadline(time.Now().Add(1 * time.Second))
		}

		conn, err := listener.Accept()
		if err != nil {
			// Check if listener was closed
			if opErr, ok := err.(*net.OpError); ok && opErr.Err.Error() == "use of closed network connection" {
				return
			}

			// Ignore timeout errors (expected from SetDeadline)
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}

			logger.ErrorEvent().
				Err(err).
				Int("port", port).
				Msg("Error accepting TCP connection")
			continue
		}

		// Handle connection in background
		go tp.handleConnection(conn, port, tunnelID)
	}
}

// handleConnection handles a single TCP connection by forwarding it through the tunnel.
func (tp *TCPProxy) handleConnection(conn net.Conn, port int, tunnelID uuid.UUID) {
	defer conn.Close()

	// Get the tunnel from the manager
	tun, exists := tp.tunnelManager.GetTunnelByID(tunnelID)
	if !exists {
		logger.WarnEvent().
			Str("tunnel_id", tunnelID.String()).
			Int("port", port).
			Msg("Tunnel not found for incoming TCP connection")
		return
	}

	// Update tunnel activity
	tun.UpdateActivity()

	// Get remote address
	remoteAddr := conn.RemoteAddr().String()

	logger.InfoEvent().
		Str("tunnel_id", tunnelID.String()).
		Int("port", port).
		Str("remote_addr", remoteAddr).
		Msg("Accepted TCP connection")

	// Create a unique connection ID
	connID := uuid.New().String()

	// Create response channel for this connection
	responseCh := make(chan *tunnelv1.ProxyResponse, 100)
	defer close(responseCh)

	// Store response channel in tunnel
	tun.ResponseMap.Store(connID, responseCh)
	defer tun.ResponseMap.Delete(connID)

	// Create context with cancel for this connection
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start goroutine to read from tunnel stream and write to TCP connection
	go tp.streamToConnection(ctx, conn, responseCh, connID)

	// Read from TCP connection and send to tunnel stream
	tp.connectionToStream(ctx, conn, tun, connID, remoteAddr)

	logger.InfoEvent().
		Str("tunnel_id", tunnelID.String()).
		Int("port", port).
		Str("connection_id", connID).
		Msg("TCP connection closed")
}

// connectionToStream reads from TCP connection and sends data to tunnel stream.
func (tp *TCPProxy) connectionToStream(
	ctx context.Context,
	conn net.Conn,
	tun *tunnel.Tunnel,
	connID string,
	remoteAddr string,
) {
	buffer := make([]byte, 32*1024) // 32KB buffer

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Set read deadline to allow periodic context checks
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))

		n, err := conn.Read(buffer)
		if err != nil {
			if err == io.EOF {
				// Normal connection close
				logger.DebugEvent().
					Str("connection_id", connID).
					Msg("TCP connection closed by client")
			} else if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				// Timeout - continue reading
				continue
			} else {
				logger.ErrorEvent().
					Err(err).
					Str("connection_id", connID).
					Msg("Error reading from TCP connection")
			}

			// Send close signal to tunnel (empty TCP data)
			closeReq := &tunnelv1.ProxyRequest{
				RequestId: connID,
				TunnelId:  tun.ID.String(),
				Payload: &tunnelv1.ProxyRequest_Tcp{
					Tcp: &tunnelv1.TCPData{
						Data:     []byte{},
						Sequence: 0,
					},
				},
			}

			select {
			case tun.RequestQueue <- &tunnel.PendingRequest{
				RequestID:  connID,
				Request:    closeReq,
				ResponseCh: make(chan *tunnelv1.ProxyResponse, 1),
				Timeout:    5 * time.Second,
				CreatedAt:  time.Now(),
			}:
			case <-time.After(1 * time.Second):
				logger.WarnEvent().Msg("Timeout sending TCP close signal")
			}

			return
		}

		if n > 0 {
			// Create proxy request with TCP data
			proxyReq := &tunnelv1.ProxyRequest{
				RequestId: connID,
				TunnelId:  tun.ID.String(),
				Payload: &tunnelv1.ProxyRequest_Tcp{
					Tcp: &tunnelv1.TCPData{
						Data:     buffer[:n],
						Sequence: 0, // TODO: implement proper sequencing
					},
				},
			}

			// Create pending request
			pendingReq := &tunnel.PendingRequest{
				RequestID:  connID,
				Request:    proxyReq,
				ResponseCh: make(chan *tunnelv1.ProxyResponse, 1),
				Timeout:    30 * time.Second,
				CreatedAt:  time.Now(),
			}

			// Send to tunnel
			select {
			case tun.RequestQueue <- pendingReq:
				// Update stats
				tun.UpdateStats(int64(n), 0)
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
				logger.WarnEvent().
					Str("connection_id", connID).
					Msg("Timeout sending TCP data to tunnel")
				return
			}
		}
	}
}

// streamToConnection reads from tunnel stream and writes to TCP connection.
func (tp *TCPProxy) streamToConnection(
	ctx context.Context,
	conn net.Conn,
	responseCh chan *tunnelv1.ProxyResponse,
	connID string,
) {
	for {
		select {
		case <-ctx.Done():
			return
		case response, ok := <-responseCh:
			if !ok {
				return
			}

			// Check if response has TCP payload
			tcpData := response.GetTcp()
			if tcpData == nil {
				logger.WarnEvent().
					Str("connection_id", connID).
					Msg("Received response without TCP payload")
				continue
			}

			// Handle TCP close (empty data)
			if len(tcpData.Data) == 0 || response.EndOfStream {
				logger.DebugEvent().
					Str("connection_id", connID).
					Msg("Received TCP close signal from tunnel")
				conn.Close()
				return
			}

			// Write response data to TCP connection
			if len(tcpData.Data) > 0 {
				conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				n, err := conn.Write(tcpData.Data)
				if err != nil {
					logger.ErrorEvent().
						Err(err).
						Str("connection_id", connID).
						Msg("Error writing to TCP connection")
					return
				}

				// Update stats (bytes out)
				if response.TunnelId != "" {
					if tunnelID, err := uuid.Parse(response.TunnelId); err == nil {
						if tun, exists := tp.tunnelManager.GetTunnelByID(tunnelID); exists {
							tun.UpdateStats(0, int64(n))
						}
					}
				}

				logger.DebugEvent().
					Str("connection_id", connID).
					Int("bytes", n).
					Msg("Wrote data to TCP connection")
			}
		}
	}
}

// Shutdown stops all TCP listeners.
func (tp *TCPProxy) Shutdown() {
	close(tp.done)

	tp.mu.Lock()
	defer tp.mu.Unlock()

	for port, listener := range tp.listeners {
		listener.Close()
		logger.InfoEvent().
			Int("port", port).
			Msg("TCP listener closed during shutdown")
	}

	tp.listeners = make(map[int]net.Listener)
}

// GetActiveListeners returns a list of active listener ports.
func (tp *TCPProxy) GetActiveListeners() []int {
	tp.mu.RLock()
	defer tp.mu.RUnlock()

	ports := make([]int, 0, len(tp.listeners))
	for port := range tp.listeners {
		ports = append(ports, port)
	}

	return ports
}
