package proxy

import (
	"context"
	"fmt"
	"net"
	"time"

	tunnelv1 "github.com/pandeptwidyaop/grok/gen/proto/tunnel/v1"
	"github.com/pandeptwidyaop/grok/pkg/logger"
)

// TCPForwarder forwards TCP connections to local service
type TCPForwarder struct {
	localAddr string
}

// NewTCPForwarder creates a new TCP forwarder
func NewTCPForwarder(localAddr string) *TCPForwarder {
	return &TCPForwarder{
		localAddr: localAddr,
	}
}

// Forward forwards TCP data to local service
func (f *TCPForwarder) Forward(ctx context.Context, data *tunnelv1.TCPData) (*tunnelv1.TCPData, error) {
	// Connect to local service
	dialer := &net.Dialer{
		Timeout: 10 * time.Second,
	}

	conn, err := dialer.DialContext(ctx, "tcp", f.localAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to local service: %w", err)
	}
	defer conn.Close()

	logger.DebugEvent().
		Str("local_addr", f.localAddr).
		Int("data_size", len(data.Data)).
		Msg("Forwarding TCP data to local service")

	// Write data to local service
	_, err = conn.Write(data.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to write to local service: %w", err)
	}

	// Read response (with timeout)
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))

	// TODO: Implement proper TCP stream handling
	// This is a simplified version that reads once
	respBuf := make([]byte, 65536)
	n, err := conn.Read(respBuf)
	if err != nil {
		return nil, fmt.Errorf("failed to read from local service: %w", err)
	}

	logger.DebugEvent().
		Int("response_size", n).
		Msg("Received TCP response from local service")

	return &tunnelv1.TCPData{
		Data:     respBuf[:n],
		Sequence: data.Sequence,
	}, nil
}
