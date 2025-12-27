package tunnel

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	tunnelv1 "github.com/pandeptwidyaop/grok/gen/proto/tunnel/v1"
	"github.com/pandeptwidyaop/grok/internal/client/proxy"
	"github.com/pandeptwidyaop/grok/pkg/logger"
)

// startProxyStream starts the bidirectional proxy stream.
func (c *Client) startProxyStream(ctx context.Context) error {
	stream, err := c.tunnelSvc.ProxyStream(ctx)
	if err != nil {
		return fmt.Errorf("failed to create proxy stream: %w", err)
	}

	c.mu.Lock()
	c.stream = stream
	c.mu.Unlock()

	logger.InfoEvent().Msg("Proxy stream established")

	// Send registration control message with tunnel details
	// Format: subdomain|token|localaddr|publicurl|savedname(optional)
	c.mu.RLock()
	regData := fmt.Sprintf("%s|%s|%s|%s|%s",
		c.getSubdomain(),
		c.cfg.AuthToken,
		c.cfg.LocalAddr,
		c.publicURL,
		c.cfg.SavedName,
	)
	c.mu.RUnlock()

	regMsg := &tunnelv1.ProxyMessage{
		Message: &tunnelv1.ProxyMessage_Control{
			Control: &tunnelv1.ControlMessage{
				Type:     tunnelv1.ControlMessage_UNKNOWN, // Use UNKNOWN for registration
				TunnelId: regData,
			},
		},
	}

	if err := stream.Send(regMsg); err != nil {
		return fmt.Errorf("failed to send registration message: %w", err)
	}

	logger.DebugEvent().Msg("Sent tunnel registration message")

	// Start receiving requests from server
	go c.receiveRequests(ctx)

	return nil
}

// getSubdomain extracts subdomain from public URL.
func (c *Client) getSubdomain() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Extract subdomain from public URL (e.g., "https://abc123.localhost" → "abc123")
	if c.publicURL == "" {
		return ""
	}

	// Remove protocol prefix
	url := c.publicURL
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")
	url = strings.TrimPrefix(url, "tcp://")

	// Find the subdomain part (everything before the first dot)
	if dotIdx := strings.Index(url, "."); dotIdx != -1 {
		return url[:dotIdx]
	}

	// If no dot found, return the whole URL (shouldn't happen in normal cases)
	return url
}

// receiveRequests receives and handles proxy requests from server.
func (c *Client) receiveRequests(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		msg, err := c.stream.Recv()
		if err == io.EOF {
			logger.InfoEvent().Msg("Server closed stream")
			return
		}
		if err != nil {
			logger.ErrorEvent().
				Err(err).
				Msg("Error receiving from stream")
			return
		}

		// Handle different message types
		switch payload := msg.Message.(type) {
		case *tunnelv1.ProxyMessage_Request:
			// Handle request from server (public internet → local service)
			go c.handleProxyRequest(ctx, payload.Request)

		case *tunnelv1.ProxyMessage_Control:
			// Handle control messages
			c.handleControlMessage(payload.Control)

		case *tunnelv1.ProxyMessage_Error:
			// Handle error messages
			logger.ErrorEvent().
				Str("request_id", payload.Error.RequestId).
				Str("code", payload.Error.Code.String()).
				Str("message", payload.Error.Message).
				Msg("Received error from server")

		default:
			logger.WarnEvent().Msg("Received unknown message type")
		}
	}
}

// handleProxyRequest handles a proxy request from the server.
func (c *Client) handleProxyRequest(ctx context.Context, req *tunnelv1.ProxyRequest) {
	logger.DebugEvent().
		Str("request_id", req.RequestId).
		Msg("Handling proxy request")

	// Handle based on protocol
	switch payload := req.Payload.(type) {
	case *tunnelv1.ProxyRequest_Http:
		c.handleHTTPRequest(ctx, req.RequestId, payload.Http)

	case *tunnelv1.ProxyRequest_Tcp:
		c.handleTCPRequest(ctx, req.RequestId, payload.Tcp)

	default:
		logger.WarnEvent().
			Str("request_id", req.RequestId).
			Msg("Unknown request payload type")
	}
}

// handleHTTPRequest forwards HTTP request to local service.
func (c *Client) handleHTTPRequest(ctx context.Context, requestID string, httpReq *tunnelv1.HTTPRequest) {
	start := time.Now()

	// Check if this is a WebSocket upgrade request
	if proxy.IsWebSocketUpgrade(httpReq) {
		c.handleWebSocketUpgrade(ctx, requestID, httpReq)
		return
	}

	// Forward regular HTTP request to local service
	httpResp, err := c.httpForwarder.Forward(ctx, httpReq)
	if err != nil {
		logger.ErrorEvent().
			Err(err).
			Str("request_id", requestID).
			Str("method", httpReq.Method).
			Str("path", httpReq.Path).
			Str("remote_addr", httpReq.RemoteAddr).
			Msg("Failed to forward HTTP request")

		// Send error response
		c.sendError(requestID, tunnelv1.ErrorCode_LOCAL_SERVICE_UNREACHABLE, err.Error())
		return
	}

	// Send response back to server
	proxyResp := &tunnelv1.ProxyResponse{
		RequestId:   requestID,
		TunnelId:    c.tunnelID,
		Payload:     &tunnelv1.ProxyResponse_Http{Http: httpResp},
		EndOfStream: true,
	}

	msg := &tunnelv1.ProxyMessage{
		Message: &tunnelv1.ProxyMessage_Response{Response: proxyResp},
	}

	if err := c.stream.Send(msg); err != nil {
		logger.ErrorEvent().
			Err(err).
			Str("request_id", requestID).
			Msg("Failed to send response")
	}

	// Log access with details
	duration := time.Since(start)
	logger.InfoEvent().
		Str("method", httpReq.Method).
		Str("path", httpReq.Path).
		Str("remote_addr", httpReq.RemoteAddr).
		Int32("status", httpResp.StatusCode).
		Int("bytes_in", len(httpReq.Body)).
		Int("bytes_out", len(httpResp.Body)).
		Dur("duration", duration).
		Msg("HTTP request processed")
}

// handleTCPRequest forwards TCP data to local service.
func (c *Client) handleTCPRequest(ctx context.Context, requestID string, tcpData *tunnelv1.TCPData) {
	logger.DebugEvent().
		Str("request_id", requestID).
		Int("bytes", len(tcpData.Data)).
		Int64("sequence", tcpData.Sequence).
		Msg("TCP data received")

	// Check if this is WebSocket data (from server to client)
	c.mu.RLock()
	wsChan, isWebSocket := c.wsConnections[requestID]
	c.mu.RUnlock()

	if isWebSocket {
		// This is WebSocket data, route to WebSocket channel
		select {
		case wsChan <- tcpData.Data:
			logger.DebugEvent().
				Str("request_id", requestID).
				Int("bytes", len(tcpData.Data)).
				Msg("Routed WebSocket data to connection")
		case <-time.After(5 * time.Second):
			logger.WarnEvent().
				Str("request_id", requestID).
				Msg("Timeout routing WebSocket data")
		}
		return
	}

	// Create response sender function
	sendResponse := func(respData *tunnelv1.TCPData) error {
		response := &tunnelv1.ProxyResponse{
			RequestId: requestID,
			TunnelId:  c.tunnelID,
			Payload: &tunnelv1.ProxyResponse_Tcp{
				Tcp: respData,
			},
		}

		msg := &tunnelv1.ProxyMessage{
			Message: &tunnelv1.ProxyMessage_Response{
				Response: response,
			},
		}

		c.mu.RLock()
		stream := c.stream
		c.mu.RUnlock()

		if stream == nil {
			return fmt.Errorf("stream not available")
		}

		return stream.Send(msg)
	}

	// Forward to local service
	startReadLoop, err := c.tcpForwarder.Forward(ctx, requestID, tcpData, sendResponse)
	if err != nil {
		logger.ErrorEvent().
			Err(err).
			Str("request_id", requestID).
			Msg("Failed to forward TCP data")

		// Send error response (close signal)
		if err := sendResponse(&tunnelv1.TCPData{
			Data:     []byte{},
			Sequence: 0,
		}); err != nil {
			logger.WarnEvent().Err(err).Msg("Failed to send error response")
		}
		return
	}

	// Start read loop for new connections
	if startReadLoop {
		go c.tcpForwarder.StartReadLoop(ctx, requestID, sendResponse)
	}
}

// handleControlMessage handles control messages from server.
func (c *Client) handleControlMessage(ctrl *tunnelv1.ControlMessage) {
	logger.InfoEvent().
		Str("type", ctrl.Type.String()).
		Str("tunnel_id", ctrl.TunnelId).
		Msg("Received control message")

	// Check for public_url update in metadata
	if ctrl.Metadata != nil {
		if publicURL, ok := ctrl.Metadata["public_url"]; ok && publicURL != "" {
			c.mu.Lock()
			oldURL := c.publicURL
			c.publicURL = publicURL
			c.mu.Unlock()

			if oldURL != publicURL {
				logger.InfoEvent().
					Str("old_url", oldURL).
					Str("new_url", publicURL).
					Msg("Public URL updated")

				// Print updated URL to console
				fmt.Printf("\n✓ Public URL updated: %s\n", publicURL)
			}
		}
	}

	switch ctrl.Type {
	case tunnelv1.ControlMessage_TUNNEL_CLOSED:
		logger.WarnEvent().Msg("Server closed tunnel")
		// Connection will be reestablished by maintainConnection

	case tunnelv1.ControlMessage_RATE_LIMIT:
		logger.WarnEvent().Msg("Rate limit exceeded")

	case tunnelv1.ControlMessage_RECONNECT:
		logger.InfoEvent().Msg("Server requested reconnect")
		// Close current stream to trigger reconnect
		if c.stream != nil {
			if err := c.stream.CloseSend(); err != nil {
				logger.WarnEvent().Err(err).Msg("Failed to close send stream")
			}
		}

	case tunnelv1.ControlMessage_UNKNOWN:
		// Unknown type - likely a URL update (already handled above via metadata)
		logger.DebugEvent().Msg("Received control message with unknown type")
	}
}

// sendError sends an error response to the server.
func (c *Client) sendError(requestID string, code tunnelv1.ErrorCode, message string) {
	errorMsg := &tunnelv1.ProxyError{
		RequestId: requestID,
		Code:      code,
		Message:   message,
	}

	msg := &tunnelv1.ProxyMessage{
		Message: &tunnelv1.ProxyMessage_Error{Error: errorMsg},
	}

	if err := c.stream.Send(msg); err != nil {
		logger.ErrorEvent().
			Err(err).
			Str("request_id", requestID).
			Msg("Failed to send error message")
	}
}

// receiveHeartbeats receives heartbeat responses from server.
func receiveHeartbeats(stream tunnelv1.TunnelService_HeartbeatClient, heartbeatErrCh chan error) {
	for {
		_, err := stream.Recv()
		if err == io.EOF {
			heartbeatErrCh <- io.EOF
			return
		}
		if err != nil {
			logger.WarnEvent().Err(err).Msg("Heartbeat receive error")
			heartbeatErrCh <- err
			return
		}
	}
}

// signalConnectionLost signals that the connection was lost.
func signalConnectionLost(connLostCh chan struct{}) {
	select {
	case connLostCh <- struct{}{}:
	default:
	}
}

// startHeartbeat starts sending heartbeat messages to server.
func (c *Client) startHeartbeat(ctx context.Context, connLostCh chan struct{}) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	heartbeatStream, err := c.tunnelSvc.Heartbeat(ctx)
	if err != nil {
		logger.ErrorEvent().Err(err).Msg("Failed to create heartbeat stream")
		signalConnectionLost(connLostCh)
		return
	}

	logger.DebugEvent().Msg("Heartbeat stream established")

	heartbeatErrCh := make(chan error, 1)
	go receiveHeartbeats(heartbeatStream, heartbeatErrCh)

	for {
		select {
		case <-ctx.Done():
			if err := heartbeatStream.CloseSend(); err != nil {
				logger.WarnEvent().Err(err).Msg("Failed to close heartbeat stream")
			}
			return

		case err := <-heartbeatErrCh:
			logger.WarnEvent().Err(err).Msg("Heartbeat failed, signaling connection lost")
			if err := heartbeatStream.CloseSend(); err != nil {
				logger.WarnEvent().Err(err).Msg("Failed to close heartbeat stream")
			}
			signalConnectionLost(connLostCh)
			return

		case <-ticker.C:
			req := &tunnelv1.HeartbeatRequest{
				TunnelId:  c.tunnelID,
				Timestamp: time.Now().Unix(),
			}

			if err := heartbeatStream.Send(req); err != nil {
				logger.ErrorEvent().Err(err).Msg("Failed to send heartbeat")
				if err := heartbeatStream.CloseSend(); err != nil {
					logger.WarnEvent().Err(err).Msg("Failed to close heartbeat stream")
				}
				signalConnectionLost(connLostCh)
				return
			}

			logger.DebugEvent().Msg("Heartbeat sent")
		}
	}
}

// handleWebSocketUpgrade handles WebSocket upgrade and bidirectional streaming.
func (c *Client) handleWebSocketUpgrade(ctx context.Context, requestID string, httpReq *tunnelv1.HTTPRequest) {
	logger.InfoEvent().
		Str("request_id", requestID).
		Str("path", httpReq.Path).
		Msg("Handling WebSocket upgrade request")

	// Forward WebSocket upgrade to local service
	httpResp, wsConn, err := c.httpForwarder.ForwardWebSocketUpgrade(ctx, httpReq)
	if err != nil {
		logger.ErrorEvent().
			Err(err).
			Str("request_id", requestID).
			Msg("Failed to upgrade WebSocket connection")

		c.sendError(requestID, tunnelv1.ErrorCode_LOCAL_SERVICE_UNREACHABLE, err.Error())
		return
	}

	// Send upgrade response back to server
	proxyResp := &tunnelv1.ProxyResponse{
		RequestId:   requestID,
		TunnelId:    c.tunnelID,
		Payload:     &tunnelv1.ProxyResponse_Http{Http: httpResp},
		EndOfStream: false, // WebSocket connection stays open
	}

	msg := &tunnelv1.ProxyMessage{
		Message: &tunnelv1.ProxyMessage_Response{Response: proxyResp},
	}

	if err := c.stream.Send(msg); err != nil {
		logger.ErrorEvent().
			Err(err).
			Str("request_id", requestID).
			Msg("Failed to send WebSocket upgrade response")
		wsConn.Close()
		return
	}

	// If upgrade failed, close connection
	if httpResp.StatusCode != 101 {
		logger.WarnEvent().
			Int32("status_code", httpResp.StatusCode).
			Str("request_id", requestID).
			Msg("WebSocket upgrade failed")
		wsConn.Close()
		return
	}

	logger.InfoEvent().
		Str("request_id", requestID).
		Msg("WebSocket upgraded, starting bidirectional streaming")

	// Start bidirectional streaming
	c.streamWebSocketData(ctx, requestID, wsConn)
}

// streamWebSocketData handles bidirectional WebSocket data streaming.
func (c *Client) streamWebSocketData(ctx context.Context, requestID string, wsConn net.Conn) {
	defer wsConn.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	// Local service -> Server (read from WebSocket connection, send to gRPC stream)
	go func() {
		defer wg.Done()
		buffer := make([]byte, 32*1024) // 32KB buffer
		sequence := int64(0)

		for {
			n, err := wsConn.Read(buffer)
			if err != nil {
				if err != io.EOF {
					logger.DebugEvent().Err(err).Msg("WebSocket connection read error")
				}
				return
			}

			if n > 0 {
				// Send data to server via gRPC stream
				tcpData := &tunnelv1.TCPData{
					Data:     buffer[:n],
					Sequence: sequence,
				}
				sequence++

				proxyResp := &tunnelv1.ProxyResponse{
					RequestId:   requestID,
					TunnelId:    c.tunnelID,
					Payload:     &tunnelv1.ProxyResponse_Tcp{Tcp: tcpData},
					EndOfStream: false,
				}

				msg := &tunnelv1.ProxyMessage{
					Message: &tunnelv1.ProxyMessage_Response{Response: proxyResp},
				}

				c.mu.Lock()
				err = c.stream.Send(msg)
				c.mu.Unlock()

				if err != nil {
					logger.ErrorEvent().Err(err).Msg("Failed to send WebSocket data to server")
					return
				}
			}
		}
	}()

	// Server -> Local service (receive from gRPC stream, write to WebSocket connection)
	go func() {
		defer wg.Done()

		// Create a channel for receiving WebSocket data from the stream handler
		// The stream receiver will put data here when it receives TCP data for this requestID
		wsChan := make(chan []byte, 100)

		// Store channel in a map so receiveRequests can find it
		c.mu.Lock()
		if c.wsConnections == nil {
			c.wsConnections = make(map[string]chan []byte)
		}
		c.wsConnections[requestID] = wsChan
		c.mu.Unlock()

		defer func() {
			c.mu.Lock()
			delete(c.wsConnections, requestID)
			close(wsChan)
			c.mu.Unlock()
		}()

		for {
			select {
			case data, ok := <-wsChan:
				if !ok {
					return
				}
				if len(data) > 0 {
					if _, err := wsConn.Write(data); err != nil {
						logger.ErrorEvent().Err(err).Msg("Failed to write data to WebSocket")
						return
					}
				}
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Minute):
				// Idle timeout
				logger.InfoEvent().Str("request_id", requestID).Msg("WebSocket idle timeout")
				return
			}
		}
	}()

	wg.Wait()

	logger.InfoEvent().
		Str("request_id", requestID).
		Msg("WebSocket connection closed")
}
