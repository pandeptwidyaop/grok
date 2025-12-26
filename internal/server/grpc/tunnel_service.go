package grpc

import (
	"context"
	"io"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	tunnelv1 "github.com/pandeptwidyaop/grok/gen/proto/tunnel/v1"
	"github.com/pandeptwidyaop/grok/internal/server/auth"
	"github.com/pandeptwidyaop/grok/internal/server/tunnel"
	pkgerrors "github.com/pandeptwidyaop/grok/pkg/errors"
	"github.com/pandeptwidyaop/grok/pkg/logger"
	"github.com/pandeptwidyaop/grok/pkg/utils"
)

// TunnelService implements the gRPC TunnelService.
type TunnelService struct {
	tunnelv1.UnimplementedTunnelServiceServer
	tunnelManager *tunnel.Manager
	tokenService  *auth.TokenService
}

// NewTunnelService creates a new tunnel service.
func NewTunnelService(
	tunnelManager *tunnel.Manager,
	tokenService *auth.TokenService,
) *TunnelService {
	return &TunnelService{
		tunnelManager: tunnelManager,
		tokenService:  tokenService,
	}
}

// CreateTunnel handles tunnel creation requests.
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

	// Load user with organization to get org subdomain
	user, err := s.tokenService.GetUserByID(ctx, authToken.UserID)
	if err != nil {
		logger.ErrorEvent().
			Err(err).
			Str("user_id", authToken.UserID.String()).
			Msg("Failed to load user")
		return nil, status.Error(codes.Internal, "failed to load user")
	}

	// Validate protocol
	if req.Protocol == tunnelv1.TunnelProtocol_TUNNEL_PROTOCOL_UNSPECIFIED {
		return nil, status.Error(codes.InvalidArgument, "protocol is required")
	}

	// Validate local address
	if req.LocalAddress == "" {
		return nil, status.Error(codes.InvalidArgument, "local_address is required")
	}

	var fullSubdomain string
	var customPart string

	// Check if req.Subdomain is a savedName (check for existing offline tunnel)
	// This allows reconnecting with the same name and reusing the old subdomain
	if req.Subdomain != "" {
		offlineTunnel, err := s.tunnelManager.FindOfflineTunnelBySavedName(ctx, authToken.UserID, req.Subdomain)
		if err != nil {
			logger.ErrorEvent().Err(err).Msg("Failed to check for offline tunnel")
			// Continue with normal allocation
		}

		if offlineTunnel != nil {
			// Found offline tunnel - reuse its subdomain!
			fullSubdomain = offlineTunnel.Subdomain
			customPart = req.Subdomain
			logger.InfoEvent().
				Str("saved_name", req.Subdomain).
				Str("subdomain", fullSubdomain).
				Str("tunnel_id", offlineTunnel.ID.String()).
				Msg("Found offline tunnel, will reuse subdomain")
		}
	}

	// If no offline tunnel found, allocate new subdomain
	if fullSubdomain == "" {
		var err error
		fullSubdomain, customPart, err = s.tunnelManager.AllocateSubdomain(
			ctx,
			authToken.UserID,
			user.OrganizationID,
			req.Subdomain,
		)
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
	}

	// Build public URL with full subdomain
	var protocol string
	switch req.Protocol {
	case tunnelv1.TunnelProtocol_HTTP:
		protocol = "http"
	case tunnelv1.TunnelProtocol_HTTPS:
		protocol = "https"
	case tunnelv1.TunnelProtocol_TCP:
		protocol = "tcp"
	default:
		// Default to https if TLS enabled, http otherwise
		if s.tunnelManager.IsTLSEnabled() {
			protocol = "https"
		} else {
			protocol = "http"
		}
	}
	publicURL := s.tunnelManager.BuildPublicURL(fullSubdomain, protocol)

	logger.InfoEvent().
		Str("subdomain", fullSubdomain).
		Str("custom_part", customPart).
		Str("public_url", publicURL).
		Str("user_id", authToken.UserID.String()).
		Str("org_id", func() string {
			if user.OrganizationID != nil {
				return user.OrganizationID.String()
			}
			return "none"
		}()).
		Msg("Tunnel created successfully")

	return &tunnelv1.CreateTunnelResponse{
		TunnelId:  "", // Will be set in ProxyStream
		PublicUrl: publicURL,
		Subdomain: fullSubdomain,
		Status:    tunnelv1.TunnelStatus_ACTIVE,
	}, nil
}

// ProxyStream handles bidirectional streaming for tunnel proxying.
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
				if err := s.tunnelManager.UnregisterTunnel(context.Background(), currentTunnel.ID); err != nil {
					logger.WarnEvent().Err(err).Msg("Failed to unregister tunnel on context done")
				}
			}
			return nil
		default:
		}

		msg, err := stream.Recv()
		if err == io.EOF {
			logger.InfoEvent().Msg("Client closed stream")
			if currentTunnel != nil {
				if err := s.tunnelManager.UnregisterTunnel(context.Background(), currentTunnel.ID); err != nil {
					logger.WarnEvent().Err(err).Msg("Failed to unregister tunnel on stream close")
				}
			}
			return nil
		}
		if err != nil {
			logger.ErrorEvent().
				Err(err).
				Msg("Error receiving message from client")
			if currentTunnel != nil {
				if err := s.tunnelManager.UnregisterTunnel(context.Background(), currentTunnel.ID); err != nil {
					logger.WarnEvent().Err(err).Msg("Failed to unregister tunnel on stream error")
				}
			}
			return status.Error(codes.Internal, "stream error")
		}

		// Handle different message types
		switch payload := msg.Message.(type) {
		case *tunnelv1.ProxyMessage_Control:
			// First control message should contain tunnel registration
			if currentTunnel == nil && payload.Control.Type == tunnelv1.ControlMessage_UNKNOWN {
				// Parse subdomain|token|localaddr|publicurl|savedname(optional) from TunnelId field
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

				if len(parts) < 4 || len(parts) > 5 {
					logger.WarnEvent().
						Int("parts_count", len(parts)).
						Msg("Invalid registration message format")
					return status.Error(codes.InvalidArgument, "invalid registration format")
				}

				subdomain := parts[0]
				authToken := parts[1]
				localAddr := parts[2]
				publicURL := parts[3]
				var savedName string
				if len(parts) == 5 && parts[4] != "" {
					savedName = parts[4]
				}

				// Validate token
				token, err := s.tokenService.ValidateToken(ctx, authToken)
				if err != nil {
					logger.WarnEvent().Err(err).Msg("Invalid token in ProxyStream")
					return status.Error(codes.Unauthenticated, "invalid authentication token")
				}

				// Load user to get organization ID
				user, err := s.tokenService.GetUserByID(ctx, token.UserID)
				if err != nil {
					logger.ErrorEvent().Err(err).Msg("Failed to load user in ProxyStream")
					return status.Error(codes.Internal, "failed to load user")
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

				var tun *tunnel.Tunnel

				// Check if savedName is provided (or generate one)
				if savedName == "" {
					// Auto-generate Docker-style name
					generatedName, err := utils.GenerateRandomName()
					if err != nil {
						logger.ErrorEvent().Err(err).Msg("Failed to generate random name")
						return status.Error(codes.Internal, "failed to generate tunnel name")
					}
					savedName = generatedName
					logger.InfoEvent().
						Str("generated_name", savedName).
						Msg("Auto-generated tunnel name")
				}

				// Check for existing offline tunnel with this saved name
				offlineTunnel, err := s.tunnelManager.FindOfflineTunnelBySavedName(ctx, token.UserID, savedName)
				if err != nil {
					logger.ErrorEvent().Err(err).Msg("Failed to check for offline tunnel")
					return status.Error(codes.Internal, "failed to check existing tunnel")
				}

				if offlineTunnel != nil {
					// Reactivate existing tunnel with new local address
					logger.InfoEvent().
						Str("saved_name", savedName).
						Str("tunnel_id", offlineTunnel.ID.String()).
						Str("new_local_addr", localAddr).
						Msg("Reactivating existing tunnel")

					tun, err = s.tunnelManager.ReactivateTunnel(ctx, offlineTunnel, stream, localAddr)
					if err != nil {
						logger.ErrorEvent().Err(err).Msg("Failed to reactivate tunnel")
						return status.Error(codes.Internal, "failed to reactivate tunnel")
					}
				} else {
					// Create new persistent tunnel
					tun = tunnel.NewTunnel(
						token.UserID,
						token.ID,
						user.OrganizationID,
						subdomain,
						protocol,
						localAddr,
						publicURL,
						stream,
					)
					tun.SavedName = &savedName

					if err := s.tunnelManager.RegisterTunnel(ctx, tun); err != nil {
						logger.ErrorEvent().
							Err(err).
							Str("subdomain", subdomain).
							Msg("Failed to register tunnel")
						return status.Error(codes.Internal, "failed to register tunnel")
					}

					logger.InfoEvent().
						Str("tunnel_id", tun.ID.String()).
						Str("subdomain", subdomain).
						Str("saved_name", savedName).
						Msg("New persistent tunnel created")
				}

				currentTunnel = tun
				logger.InfoEvent().
					Str("tunnel_id", tun.ID.String()).
					Str("subdomain", subdomain).
					Str("public_url", tun.PublicURL).
					Msg("Tunnel registered successfully")

				// Send updated public URL to client (important for TCP tunnels with allocated ports)
				updateMsg := &tunnelv1.ProxyMessage{
					Message: &tunnelv1.ProxyMessage_Control{
						Control: &tunnelv1.ControlMessage{
							Type:     tunnelv1.ControlMessage_UNKNOWN, // Use UNKNOWN type for URL update
							TunnelId: tun.ID.String(),
							Metadata: map[string]string{
								"public_url": tun.PublicURL,
							},
						},
					},
				}
				if err := stream.Send(updateMsg); err != nil {
					logger.ErrorEvent().
						Err(err).
						Str("tunnel_id", tun.ID.String()).
						Msg("Failed to send public URL update to client")
				}

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
				respChan, ok := ch.(chan *tunnelv1.ProxyResponse)
				if !ok {
					logger.ErrorEvent().
						Str("request_id", response.RequestId).
						Msg("Invalid response channel type")
					continue
				}
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

// Heartbeat handles heartbeat streaming.
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

// processRequests processes pending requests from the tunnel queue.
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
