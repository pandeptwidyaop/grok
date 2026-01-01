package proxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tunnelv1 "github.com/pandeptwidyaop/grok/gen/proto/tunnel/v1"
	"github.com/pandeptwidyaop/grok/internal/client/config"
)

// TestForwardWebSocketUpgrade_BufferedData reproduces the critical bufio.Reader leak.
// It sends WebSocket frames IMMEDIATELY after upgrade headers.
// Without the fix, these frames will be lost in the bufio buffer.
func TestForwardWebSocketUpgrade_BufferedData(t *testing.T) {
	// Create mock server that sends upgrade + immediate message
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		conn, buf, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Errorf("Hijack failed: %v", err)
			return
		}
		defer conn.Close()

		// Send upgrade response
		buf.WriteString("HTTP/1.1 101 Switching Protocols\r\n")
		buf.WriteString("Upgrade: websocket\r\n")
		buf.WriteString("Connection: Upgrade\r\n")
		buf.WriteString("\r\n")

		// Flush headers to ensure they are sent
		buf.Flush()

		// ✅ Immediately send WebSocket frame (tests the bug!)
		// 0x81 = final text frame
		// 0x05 = payload length 5
		// "Hello"
		conn.Write([]byte{0x81, 0x05, 'H', 'e', 'l', 'l', 'o'})
	}))
	defer server.Close()

	// Parse server URL
	serverAddr := strings.TrimPrefix(server.URL, "http://")

	// Setup forwarder
	cfg := config.PerformanceConfig{}
	cfg.ConnectionPool.Enabled = false // Disable pool for simple test
	forwarder := NewHTTPForwarder(serverAddr, cfg)
	defer forwarder.Close()

	// Create upgrade request
	req := &tunnelv1.HTTPRequest{
		Method: "GET",
		Path:   "/",
		Headers: map[string]*tunnelv1.HeaderValues{
			"Connection": {Values: []string{"Upgrade"}},
			"Upgrade":    {Values: []string{"websocket"}},
			"Host":       {Values: []string{"localhost"}},
		},
	}

	// Forward upgrade
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, conn, err := forwarder.ForwardWebSocketUpgrade(ctx, req)
	require.NoError(t, err)
	defer conn.Close()

	assert.Equal(t, int32(101), resp.StatusCode)

	// Attempt to read the frame that was sent immediately
	// If the bug exists, this might hang or return EOF/timeout because the data was lost in bufio
	readBuf := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := conn.Read(readBuf)

	// In the buggy implementation, this Assert will fail (timeout or no data)
	require.NoError(t, err, "Should be able to read immediate WebSocket frame")
	assert.Greater(t, n, 0, "Should have read some bytes")

	// Check if we got the correct frame
	expectedFrame := []byte{0x81, 0x05, 'H', 'e', 'l', 'l', 'o'}
	if n >= len(expectedFrame) {
		assert.Equal(t, expectedFrame, readBuf[:len(expectedFrame)])
	} else {
		assert.Fail(t, "Read fewer bytes than expected frame path")
	}
}
