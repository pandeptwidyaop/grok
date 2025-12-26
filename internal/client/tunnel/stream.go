package tunnel

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	tunnelv1 "github.com/pandeptwidyaop/grok/gen/proto/tunnel/v1"
	"github.com/pandeptwidyaop/grok/pkg/logger"
)

// startProxyStream starts the bidirectional proxy stream
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
	// Format: subdomain|token|localaddr|publicurl
	c.mu.RLock()
	regData := fmt.Sprintf("%s|%s|%s|%s",
		c.getSubdomain(),
		c.cfg.AuthToken,
		c.cfg.LocalAddr,
		c.publicURL,
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

// getSubdomain extracts subdomain from public URL
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

// receiveRequests receives and handles proxy requests from server
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

// handleProxyRequest handles a proxy request from the server
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

// handleHTTPRequest forwards HTTP request to local service
func (c *Client) handleHTTPRequest(ctx context.Context, requestID string, httpReq *tunnelv1.HTTPRequest) {
	start := time.Now()

	// Forward request to local service
	httpResp, err := c.forwarder.Forward(ctx, httpReq)
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
		RequestId:     requestID,
		TunnelId:      c.tunnelID,
		Payload:       &tunnelv1.ProxyResponse_Http{Http: httpResp},
		EndOfStream:   true,
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

// handleTCPRequest forwards TCP data to local service
func (c *Client) handleTCPRequest(ctx context.Context, requestID string, tcpData *tunnelv1.TCPData) {
	// Log TCP data received
	logger.InfoEvent().
		Str("request_id", requestID).
		Int("bytes", len(tcpData.Data)).
		Int64("sequence", tcpData.Sequence).
		Msg("TCP data received")

	// TODO: Implement TCP forwarding
	logger.WarnEvent().Msg("TCP forwarding not yet implemented")
}

// handleControlMessage handles control messages from server
func (c *Client) handleControlMessage(ctrl *tunnelv1.ControlMessage) {
	logger.InfoEvent().
		Str("type", ctrl.Type.String()).
		Str("tunnel_id", ctrl.TunnelId).
		Msg("Received control message")

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
			c.stream.CloseSend()
		}
	}
}

// sendError sends an error response to the server
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

// startHeartbeat starts sending heartbeat messages to server
func (c *Client) startHeartbeat(ctx context.Context, connLostCh chan struct{}) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Create heartbeat stream
	heartbeatStream, err := c.tunnelSvc.Heartbeat(ctx)
	if err != nil {
		logger.ErrorEvent().
			Err(err).
			Msg("Failed to create heartbeat stream")
		// Signal connection lost
		select {
		case connLostCh <- struct{}{}:
		default:
		}
		return
	}

	logger.DebugEvent().Msg("Heartbeat stream established")

	// Create channel to signal heartbeat receive errors
	heartbeatErrCh := make(chan error, 1)

	// Start receiving heartbeat responses
	go func() {
		for {
			_, err := heartbeatStream.Recv()
			if err == io.EOF {
				heartbeatErrCh <- io.EOF
				return
			}
			if err != nil {
				logger.WarnEvent().
					Err(err).
					Msg("Heartbeat receive error")
				heartbeatErrCh <- err
				return
			}
			// Heartbeat received, tunnel is healthy
		}
	}()

	// Send heartbeat periodically
	for {
		select {
		case <-ctx.Done():
			heartbeatStream.CloseSend()
			return

		case err := <-heartbeatErrCh:
			// Heartbeat receive failed, signal connection lost
			logger.WarnEvent().
				Err(err).
				Msg("Heartbeat failed, signaling connection lost")
			heartbeatStream.CloseSend()
			select {
			case connLostCh <- struct{}{}:
			default:
			}
			return

		case <-ticker.C:
			req := &tunnelv1.HeartbeatRequest{
				TunnelId:  c.tunnelID,
				Timestamp: time.Now().Unix(),
			}

			if err := heartbeatStream.Send(req); err != nil {
				logger.ErrorEvent().
					Err(err).
					Msg("Failed to send heartbeat")
				heartbeatStream.CloseSend()
				// Signal connection lost
				select {
				case connLostCh <- struct{}{}:
				default:
				}
				return
			}

			logger.DebugEvent().Msg("Heartbeat sent")
		}
	}
}
