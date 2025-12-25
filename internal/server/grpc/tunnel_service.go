package grpc

import (
	"context"
	"io"
	"time"

	tunnelv1 "github.com/pandeptwidyaop/grok/gen/proto/tunnel/v1"
	"github.com/pandeptwidyaop/grok/internal/server/auth"
	"github.com/pandeptwidyaop/grok/internal/server/tunnel"
	pkgerrors "github.com/pandeptwidyaop/grok/pkg/errors"
	"github.com/pandeptwidyaop/grok/pkg/logger"
	"github.com/pandeptwidyaop/grok/pkg/utils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TunnelService implements the gRPC TunnelService
type TunnelService struct {
	tunnelv1.UnimplementedTunnelServiceServer
	tunnelManager *tunnel.Manager
	tokenService  *auth.TokenService
}

// NewTunnelService creates a new tunnel service
func NewTunnelService(
	tunnelManager *tunnel.Manager,
	tokenService *auth.TokenService,
) *TunnelService {
	return &TunnelService{
		tunnelManager: tunnelManager,
		tokenService:  tokenService,
	}
}

// CreateTunnel handles tunnel creation requests
func (s *TunnelService) CreateTunnel(
	ctx context.Context,
	req *tunnelv1.CreateTunnelRequest,
) (*tunnelv1.CreateTunnelResponse, error) {
	// Validate token
	authToken, err := s.tokenService.ValidateToken(ctx, req.AuthToken)
	if err != nil {
		logger.WarnEvent().
			Err(err).
			Msg("Invalid token in CreateTunnel")
		return nil, status.Error(codes.Unauthenticated, "invalid authentication token")
	}

	// Validate protocol
	if req.Protocol == tunnelv1.TunnelProtocol_TUNNEL_PROTOCOL_UNSPECIFIED {
		return nil, status.Error(codes.InvalidArgument, "protocol is required")
	}

	// Validate local address
	if req.LocalAddress == "" {
		return nil, status.Error(codes.InvalidArgument, "local_address is required")
	}

	// Allocate subdomain
	subdomain, err := s.tunnelManager.AllocateSubdomain(ctx, authToken.UserID, req.Subdomain)
	if err != nil {
		logger.ErrorEvent().
			Err(err).
			Str("requested_subdomain", req.Subdomain).
			Str("user_id", authToken.UserID.String()).
			Msg("Failed to allocate subdomain")

		if err == pkgerrors.ErrSubdomainTaken {
			return nil, status.Error(codes.AlreadyExists, "subdomain already taken")
		}
		if err == pkgerrors.ErrInvalidSubdomain {
			return nil, status.Error(codes.InvalidArgument, "invalid subdomain format")
		}
		return nil, status.Error(codes.Internal, "failed to allocate subdomain")
	}

	// Build public URL
	protocol := "https"
	if req.Protocol == tunnelv1.TunnelProtocol_TCP {
		protocol = "tcp"
	}
	publicURL := s.tunnelManager.BuildPublicURL(subdomain, protocol)

	logger.InfoEvent().
		Str("subdomain", subdomain).
		Str("public_url", publicURL).
		Str("user_id", authToken.UserID.String()).
		Msg("Tunnel created successfully")

	return &tunnelv1.CreateTunnelResponse{
		TunnelId:  "",                                // Will be set in ProxyStream
		PublicUrl: publicURL,
		Subdomain: subdomain,
		Status:    tunnelv1.TunnelStatus_ACTIVE,
	}, nil
}

// ProxyStream handles bidirectional streaming for tunnel proxying
func (s *TunnelService) ProxyStream(stream tunnelv1.TunnelService_ProxyStreamServer) error {
	ctx := stream.Context()

	logger.InfoEvent().Msg("ProxyStream connection established")

	// Wait for first control message from client with subdomain
	var currentTunnel *tunnel.Tunnel

	// Handle incoming messages from client (responses)
	for {
		select {
		case <-ctx.Done():
			logger.InfoEvent().Msg("ProxyStream context done")
			if currentTunnel != nil {
				s.tunnelManager.UnregisterTunnel(context.Background(), currentTunnel.ID)
			}
			return nil
		default:
		}

		msg, err := stream.Recv()
		if err == io.EOF {
			logger.InfoEvent().Msg("Client closed stream")
			if currentTunnel != nil {
				s.tunnelManager.UnregisterTunnel(context.Background(), currentTunnel.ID)
			}
			return nil
		}
		if err != nil {
			logger.ErrorEvent().
				Err(err).
				Msg("Error receiving message from client")
			if currentTunnel != nil {
				s.tunnelManager.UnregisterTunnel(context.Background(), currentTunnel.ID)
			}
			return status.Error(codes.Internal, "stream error")
		}

		// Handle different message types
		switch payload := msg.Message.(type) {
		case *tunnelv1.ProxyMessage_Control:
			// First control message should contain tunnel registration
			if currentTunnel == nil && payload.Control.Type == tunnelv1.ControlMessage_UNKNOWN {
				// Parse subdomain|token|localaddr|publicurl from TunnelId field
				parts := []string{}
				data := payload.Control.TunnelId
				lastIdx := 0
				for i, ch := range data {
					if ch == '|' {
						parts = append(parts, data[lastIdx:i])
						lastIdx = i + 1
					}
				}
				parts = append(parts, data[lastIdx:])

				if len(parts) != 4 {
					logger.WarnEvent().
						Int("parts_count", len(parts)).
						Msg("Invalid registration message format")
					return status.Error(codes.InvalidArgument, "invalid registration format")
				}

				subdomain := parts[0]
				authToken := parts[1]
				localAddr := parts[2]
				publicURL := parts[3]

				// Validate token
				token, err := s.tokenService.ValidateToken(ctx, authToken)
				if err != nil {
					logger.WarnEvent().Err(err).Msg("Invalid token in ProxyStream")
					return status.Error(codes.Unauthenticated, "invalid authentication token")
				}

				// Determine protocol from public URL
				protocol := tunnelv1.TunnelProtocol_HTTP
				if len(publicURL) > 0 {
					if publicURL[:8] == "https://" {
						protocol = tunnelv1.TunnelProtocol_HTTPS
					} else if publicURL[:6] == "tcp://" {
						protocol = tunnelv1.TunnelProtocol_TCP
					}
				}

				// Create tunnel using constructor
				tun := tunnel.NewTunnel(
					token.UserID,
					token.ID,
					subdomain,
					protocol,
					localAddr,
					publicURL,
					stream,
				)

				if err := s.tunnelManager.RegisterTunnel(ctx, tun); err != nil {
					logger.ErrorEvent().
						Err(err).
						Str("subdomain", subdomain).
						Msg("Failed to register tunnel")
					return status.Error(codes.Internal, "failed to register tunnel")
				}

				currentTunnel = tun
				logger.InfoEvent().
					Str("tunnel_id", tun.ID.String()).
					Str("subdomain", subdomain).
					Msg("Tunnel registered successfully")

				// Start processing requests for this tunnel
				go s.processRequests(ctx, currentTunnel)
				continue
			}

			// Handle other control messages
			logger.InfoEvent().
				Str("type", payload.Control.Type.String()).
				Msg("Received control message")

		case *tunnelv1.ProxyMessage_Response:
			// Handle response from client
			if currentTunnel == nil {
				logger.WarnEvent().Msg("Received response before tunnel registration")
				continue
			}

			response := payload.Response
			logger.DebugEvent().
				Str("request_id", response.RequestId).
				Str("tunnel_id", currentTunnel.ID.String()).
				Msg("Received response from client")

			// Route response back to waiting HTTP request
			if ch, ok := currentTunnel.ResponseMap.Load(response.RequestId); ok {
				respChan := ch.(chan *tunnelv1.ProxyResponse)
				select {
				case respChan <- response:
					// Response delivered
				default:
					logger.WarnEvent().
						Str("request_id", response.RequestId).
						Msg("Response channel full or closed")
				}
			} else {
				logger.WarnEvent().
					Str("request_id", response.RequestId).
					Msg("No waiting request found for response")
			}

		case *tunnelv1.ProxyMessage_Error:
			// Handle error from client
			logger.ErrorEvent().
				Str("request_id", payload.Error.RequestId).
				Str("error_code", payload.Error.Code.String()).
				Str("message", payload.Error.Message).
				Msg("Received error from client")

			// TODO: Send error response to waiting HTTP request

		default:
			logger.WarnEvent().Msg("Unknown message type received")
		}
	}
}

// Heartbeat handles heartbeat streaming
func (s *TunnelService) Heartbeat(stream tunnelv1.TunnelService_HeartbeatServer) error {
	ctx := stream.Context()

	logger.InfoEvent().Msg("Heartbeat stream established")

	for {
		select {
		case <-ctx.Done():
			logger.InfoEvent().Msg("Heartbeat stream context done")
			return nil
		default:
		}

		// Receive heartbeat from client
		req, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			logger.ErrorEvent().
				Err(err).
				Msg("Heartbeat error")
			return err
		}

		// Send heartbeat response
		resp := &tunnelv1.HeartbeatResponse{
			TunnelId:  req.TunnelId,
			Timestamp: time.Now().Unix(),
			Healthy:   true,
		}

		if err := stream.Send(resp); err != nil {
			logger.ErrorEvent().
				Err(err).
				Msg("Failed to send heartbeat response")
			return err
		}

		// Update tunnel activity
		// TODO: Update tunnel last activity in manager
	}
}

// processRequests processes pending requests from the tunnel queue
func (s *TunnelService) processRequests(ctx context.Context, tun *tunnel.Tunnel) {
	logger.DebugEvent().
		Str("tunnel_id", tun.ID.String()).
		Msg("Starting request processor")

	for {
		select {
		case <-ctx.Done():
			logger.DebugEvent().Msg("Request processor context done")
			return

		case pendingReq, ok := <-tun.RequestQueue:
			if !ok {
				logger.DebugEvent().Msg("Request queue closed")
				return
			}

			// Send request to client via gRPC stream
			proxyMsg := &tunnelv1.ProxyMessage{
				Message: &tunnelv1.ProxyMessage_Request{
					Request: pendingReq.Request,
				},
			}

			if err := tun.Stream.SendMsg(proxyMsg); err != nil {
				logger.ErrorEvent().
					Err(err).
					Str("request_id", pendingReq.RequestID).
					Msg("Failed to send request to client")

				// Send error back to waiting HTTP handler
				close(pendingReq.ResponseCh)
				continue
			}

			logger.DebugEvent().
				Str("request_id", pendingReq.RequestID).
				Msg("Sent request to client")
		}
	}
}

// generateTunnelID generates a unique tunnel ID
func generateTunnelID() string {
	id, _ := utils.GenerateRandomToken(16)
	return id
}
