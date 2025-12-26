package grpc

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
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

// allocateSubdomainForTunnel allocates subdomain for new tunnel, checking for offline tunnel reuse.
func (s *TunnelService) allocateSubdomainForTunnel(ctx context.Context, userID uuid.UUID, orgID *uuid.UUID, requestedSubdomain string) (fullSubdomain, customPart string, err error) {
	if requestedSubdomain != "" {
		offlineTunnel, err := s.tunnelManager.FindOfflineTunnelBySavedName(ctx, userID, requestedSubdomain)
		if err == nil && offlineTunnel != nil {
			logger.InfoEvent().
				Str("saved_name", requestedSubdomain).
				Str("subdomain", offlineTunnel.Subdomain).
				Str("tunnel_id", offlineTunnel.ID.String()).
				Msg("Found offline tunnel, will reuse subdomain")
			return offlineTunnel.Subdomain, requestedSubdomain, nil
		}
	}

	return s.tunnelManager.AllocateSubdomain(ctx, userID, orgID, requestedSubdomain)
}

// determineProtocol determines tunnel protocol from request.
func (s *TunnelService) determineProtocol(reqProtocol tunnelv1.TunnelProtocol) string {
	switch reqProtocol {
	case tunnelv1.TunnelProtocol_HTTP:
		return "http"
	case tunnelv1.TunnelProtocol_HTTPS:
		return "https"
	case tunnelv1.TunnelProtocol_TCP:
		return "tcp"
	default:
		if s.tunnelManager.IsTLSEnabled() {
			return "https"
		}
		return "http"
	}
}

// CreateTunnel handles tunnel creation requests.
func (s *TunnelService) CreateTunnel(
	ctx context.Context,
	req *tunnelv1.CreateTunnelRequest,
) (*tunnelv1.CreateTunnelResponse, error) {
	authToken, err := s.tokenService.ValidateToken(ctx, req.AuthToken)
	if err != nil {
		logger.WarnEvent().Err(err).Msg("Invalid token in CreateTunnel")
		return nil, status.Error(codes.Unauthenticated, "invalid authentication token")
	}

	user, err := s.tokenService.GetUserByID(ctx, authToken.UserID)
	if err != nil {
		logger.ErrorEvent().Err(err).Str("user_id", authToken.UserID.String()).Msg("Failed to load user")
		return nil, status.Error(codes.Internal, "failed to load user")
	}

	if req.Protocol == tunnelv1.TunnelProtocol_TUNNEL_PROTOCOL_UNSPECIFIED {
		return nil, status.Error(codes.InvalidArgument, "protocol is required")
	}

	if req.LocalAddress == "" {
		return nil, status.Error(codes.InvalidArgument, "local_address is required")
	}

	fullSubdomain, customPart, err := s.allocateSubdomainForTunnel(ctx, authToken.UserID, user.OrganizationID, req.Subdomain)
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

	protocol := s.determineProtocol(req.Protocol)
	publicURL := s.tunnelManager.BuildPublicURL(fullSubdomain, protocol)

	orgID := "none"
	if user.OrganizationID != nil {
		orgID = user.OrganizationID.String()
	}

	logger.InfoEvent().
		Str("subdomain", fullSubdomain).
		Str("custom_part", customPart).
		Str("public_url", publicURL).
		Str("user_id", authToken.UserID.String()).
		Str("org_id", orgID).
		Msg("Tunnel created successfully")

	return &tunnelv1.CreateTunnelResponse{
		TunnelId:  "",
		PublicUrl: publicURL,
		Subdomain: fullSubdomain,
		Status:    tunnelv1.TunnelStatus_ACTIVE,
	}, nil
}

// ProxyStream handles bidirectional streaming for tunnel proxying.
// registrationData holds parsed tunnel registration information.
type registrationData struct {
	subdomain string
	authToken string
	localAddr string
	publicURL string
	savedName string
}

// parseRegistrationData parses pipe-delimited registration data.
func parseRegistrationData(data string) (*registrationData, error) {
	parts := []string{}
	lastIdx := 0
	for i, ch := range data {
		if ch == '|' {
			parts = append(parts, data[lastIdx:i])
			lastIdx = i + 1
		}
	}
	parts = append(parts, data[lastIdx:])

	if len(parts) < 4 || len(parts) > 5 {
		return nil, fmt.Errorf("invalid registration format: expected 4-5 parts, got %d", len(parts))
	}

	reg := &registrationData{
		subdomain: parts[0],
		authToken: parts[1],
		localAddr: parts[2],
		publicURL: parts[3],
	}

	if len(parts) == 5 && parts[4] != "" {
		reg.savedName = parts[4]
	}

	return reg, nil
}

// determineProtocolFromURL determines the tunnel protocol from public URL.
func determineProtocolFromURL(publicURL string) tunnelv1.TunnelProtocol {
	if len(publicURL) > 0 {
		if len(publicURL) >= 8 && publicURL[:8] == "https://" {
			return tunnelv1.TunnelProtocol_HTTPS
		}
		if len(publicURL) >= 6 && publicURL[:6] == "tcp://" {
			return tunnelv1.TunnelProtocol_TCP
		}
	}
	return tunnelv1.TunnelProtocol_HTTP
}

// handleTunnelRegistration handles the registration of a new tunnel.
func (s *TunnelService) handleTunnelRegistration(
	ctx context.Context,
	stream tunnelv1.TunnelService_ProxyStreamServer,
	reg *registrationData,
) (*tunnel.Tunnel, error) {
	// Validate token
	token, err := s.tokenService.ValidateToken(ctx, reg.authToken)
	if err != nil {
		logger.WarnEvent().Err(err).Msg("Invalid token in ProxyStream")
		return nil, status.Error(codes.Unauthenticated, "invalid authentication token")
	}

	// Load user to get organization ID
	user, err := s.tokenService.GetUserByID(ctx, token.UserID)
	if err != nil {
		logger.ErrorEvent().Err(err).Msg("Failed to load user in ProxyStream")
		return nil, status.Error(codes.Internal, "failed to load user")
	}

	// Determine protocol from public URL
	protocol := determineProtocolFromURL(reg.publicURL)

	// Auto-generate tunnel name if not provided
	savedName := reg.savedName
	if savedName == "" {
		generatedName, err := utils.GenerateRandomName()
		if err != nil {
			logger.ErrorEvent().Err(err).Msg("Failed to generate random name")
			return nil, status.Error(codes.Internal, "failed to generate tunnel name")
		}
		savedName = generatedName
		logger.InfoEvent().Str("generated_name", savedName).Msg("Auto-generated tunnel name")
	}

	// Check for existing offline tunnel with this saved name
	offlineTunnel, err := s.tunnelManager.FindOfflineTunnelBySavedName(ctx, token.UserID, savedName)
	if err != nil {
		logger.ErrorEvent().Err(err).Msg("Failed to check for offline tunnel")
		return nil, status.Error(codes.Internal, "failed to check existing tunnel")
	}

	var tun *tunnel.Tunnel
	if offlineTunnel != nil {
		// Reactivate existing tunnel with new local address
		logger.InfoEvent().
			Str("saved_name", savedName).
			Str("tunnel_id", offlineTunnel.ID.String()).
			Str("new_local_addr", reg.localAddr).
			Msg("Reactivating existing tunnel")

		tun, err = s.tunnelManager.ReactivateTunnel(ctx, offlineTunnel, stream, reg.localAddr)
		if err != nil {
			logger.ErrorEvent().Err(err).Msg("Failed to reactivate tunnel")
			return nil, status.Error(codes.Internal, "failed to reactivate tunnel")
		}
	} else {
		// Create new persistent tunnel
		tun = tunnel.NewTunnel(
			token.UserID,
			token.ID,
			user.OrganizationID,
			reg.subdomain,
			protocol,
			reg.localAddr,
			reg.publicURL,
			stream,
		)
		tun.SavedName = &savedName

		if err := s.tunnelManager.RegisterTunnel(ctx, tun); err != nil {
			logger.ErrorEvent().
				Err(err).
				Str("subdomain", reg.subdomain).
				Msg("Failed to register tunnel")
			return nil, status.Error(codes.Internal, "failed to register tunnel")
		}

		logger.InfoEvent().
			Str("tunnel_id", tun.ID.String()).
			Str("subdomain", reg.subdomain).
			Str("saved_name", savedName).
			Msg("New persistent tunnel created")
	}

	logger.InfoEvent().
		Str("tunnel_id", tun.ID.String()).
		Str("subdomain", reg.subdomain).
		Str("public_url", tun.PublicURL).
		Msg("Tunnel registered successfully")

	// Send updated public URL to client
	updateMsg := &tunnelv1.ProxyMessage{
		Message: &tunnelv1.ProxyMessage_Control{
			Control: &tunnelv1.ControlMessage{
				Type:     tunnelv1.ControlMessage_UNKNOWN,
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

	return tun, nil
}

// cleanupTunnel unregisters a tunnel on stream close.
func (s *TunnelService) cleanupTunnel(tun *tunnel.Tunnel, reason string) {
	if tun != nil {
		if err := s.tunnelManager.UnregisterTunnel(context.Background(), tun.ID); err != nil {
			logger.WarnEvent().Err(err).Str("reason", reason).Msg("Failed to unregister tunnel")
		}
	}
}

// handleProxyResponse routes proxy response back to waiting HTTP request.
func (s *TunnelService) handleProxyResponse(currentTunnel *tunnel.Tunnel, response *tunnelv1.ProxyResponse) {
	logger.DebugEvent().
		Str("request_id", response.RequestId).
		Str("tunnel_id", currentTunnel.ID.String()).
		Msg("Received response from client")

	ch, ok := currentTunnel.ResponseMap.Load(response.RequestId)
	if !ok {
		logger.WarnEvent().
			Str("request_id", response.RequestId).
			Msg("No waiting request found for response")
		return
	}

	respChan, ok := ch.(chan *tunnelv1.ProxyResponse)
	if !ok {
		logger.ErrorEvent().
			Str("request_id", response.RequestId).
			Msg("Invalid response channel type")
		return
	}

	select {
	case respChan <- response:
		// Response delivered
	default:
		logger.WarnEvent().
			Str("request_id", response.RequestId).
			Msg("Response channel full or closed")
	}
}

func (s *TunnelService) ProxyStream(stream tunnelv1.TunnelService_ProxyStreamServer) error {
	ctx := stream.Context()

	logger.InfoEvent().Msg("ProxyStream connection established")

	var currentTunnel *tunnel.Tunnel

	for {
		select {
		case <-ctx.Done():
			logger.InfoEvent().Msg("ProxyStream context done")
			s.cleanupTunnel(currentTunnel, "context done")
			return nil
		default:
		}

		msg, err := stream.Recv()
		if err == io.EOF {
			logger.InfoEvent().Msg("Client closed stream")
			s.cleanupTunnel(currentTunnel, "stream close")
			return nil
		}
		if err != nil {
			logger.ErrorEvent().Err(err).Msg("Error receiving message from client")
			s.cleanupTunnel(currentTunnel, "stream error")
			return status.Error(codes.Internal, "stream error")
		}

		switch payload := msg.Message.(type) {
		case *tunnelv1.ProxyMessage_Control:
			if currentTunnel == nil && payload.Control.Type == tunnelv1.ControlMessage_UNKNOWN {
				reg, err := parseRegistrationData(payload.Control.TunnelId)
				if err != nil {
					logger.WarnEvent().Err(err).Msg("Invalid registration message format")
					return status.Error(codes.InvalidArgument, err.Error())
				}

				tun, err := s.handleTunnelRegistration(ctx, stream, reg)
				if err != nil {
					return err
				}

				currentTunnel = tun
				go s.processRequests(ctx, currentTunnel)
				continue
			}

			logger.InfoEvent().
				Str("type", payload.Control.Type.String()).
				Msg("Received control message")

		case *tunnelv1.ProxyMessage_Response:
			if currentTunnel == nil {
				logger.WarnEvent().Msg("Received response before tunnel registration")
				continue
			}

			s.handleProxyResponse(currentTunnel, payload.Response)

		case *tunnelv1.ProxyMessage_Error:
			logger.ErrorEvent().
				Str("request_id", payload.Error.RequestId).
				Str("error_code", payload.Error.Code.String()).
				Str("message", payload.Error.Message).
				Msg("Received error from client")

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
