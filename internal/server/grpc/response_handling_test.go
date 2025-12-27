package grpc

import (
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tunnelv1 "github.com/pandeptwidyaop/grok/gen/proto/tunnel/v1"
	"github.com/pandeptwidyaop/grok/internal/server/tunnel"
)

// TestHandleProxyResponse_BlockingSend tests that blocking send mechanism works correctly.
func TestHandleProxyResponse_BlockingSend(t *testing.T) {
	// Create tunnel with response channel
	tun := &tunnel.Tunnel{
		ID:          uuid.New(),
		ResponseMap: sync.Map{},
	}

	requestID := "test-request-1"
	responseCh := make(chan *tunnelv1.ProxyResponse, 5) // Small buffer to test blocking
	tun.ResponseMap.Store(requestID, responseCh)

	service := &TunnelService{
		tunnelManager: nil, // Not needed for this test
		tokenService:  nil, // Not needed for this test
	}

	// Create multiple chunks (more than buffer size)
	numChunks := 10
	chunks := make([]*tunnelv1.ProxyResponse, numChunks)
	for i := 0; i < numChunks; i++ {
		chunks[i] = &tunnelv1.ProxyResponse{
			RequestId: requestID,
			TunnelId:  tun.ID.String(),
			Payload: &tunnelv1.ProxyResponse_Http{
				Http: &tunnelv1.HTTPResponse{
					StatusCode: 200,
					Body:       make([]byte, 1024), // 1KB chunk
				},
			},
			EndOfStream: i == numChunks-1,
		}
	}

	// Producer goroutine: Send all chunks
	var producerDone sync.WaitGroup
	producerDone.Add(1)
	go func() {
		defer producerDone.Done()
		for _, chunk := range chunks {
			// This should BLOCK when buffer is full, not drop chunks
			service.handleProxyResponse(tun, chunk)
		}
	}()

	// Consumer goroutine: Slowly consume chunks
	receivedChunks := make([]*tunnelv1.ProxyResponse, 0, numChunks)
	var consumerDone sync.WaitGroup
	consumerDone.Add(1)
	go func() {
		defer consumerDone.Done()
		for i := 0; i < numChunks; i++ {
			// Simulate slow consumer
			time.Sleep(10 * time.Millisecond)
			chunk := <-responseCh
			receivedChunks = append(receivedChunks, chunk)
		}
	}()

	// Wait for both to complete
	producerDone.Wait()
	consumerDone.Wait()

	// Verify all chunks were delivered (no drops)
	assert.Equal(t, numChunks, len(receivedChunks), "All chunks should be delivered")
	for i, chunk := range receivedChunks {
		assert.Equal(t, requestID, chunk.RequestId)
		assert.Equal(t, i == numChunks-1, chunk.EndOfStream)
	}
}

// TestHandleProxyResponse_LargeFileSimulation simulates large file download (100+ chunks).
func TestHandleProxyResponse_LargeFileSimulation(t *testing.T) {
	// Create tunnel with response channel
	tun := &tunnel.Tunnel{
		ID:          uuid.New(),
		ResponseMap: sync.Map{},
	}

	requestID := "large-file-request"
	responseCh := make(chan *tunnelv1.ProxyResponse, 50) // Production buffer size
	tun.ResponseMap.Store(requestID, responseCh)

	service := &TunnelService{
		tunnelManager: nil,
		tokenService:  nil,
	}

	// Simulate 1GB file: 1024MB / 4MB per chunk = 256 chunks
	numChunks := 256
	chunkSize := 4 * 1024 * 1024 // 4MB

	// Producer goroutine: Send all chunks
	var producerDone sync.WaitGroup
	producerDone.Add(1)
	producerStart := time.Now()
	go func() {
		defer producerDone.Done()
		for i := 0; i < numChunks; i++ {
			chunk := &tunnelv1.ProxyResponse{
				RequestId: requestID,
				TunnelId:  tun.ID.String(),
				Payload: &tunnelv1.ProxyResponse_Http{
					Http: &tunnelv1.HTTPResponse{
						StatusCode: 200,
						Body:       make([]byte, chunkSize),
					},
				},
				EndOfStream: i == numChunks-1,
			}
			service.handleProxyResponse(tun, chunk)
		}
	}()

	// Consumer goroutine: Consume chunks
	receivedCount := 0
	var consumerDone sync.WaitGroup
	consumerDone.Add(1)
	go func() {
		defer consumerDone.Done()
		for i := 0; i < numChunks; i++ {
			chunk := <-responseCh
			require.NotNil(t, chunk)
			receivedCount++
		}
	}()

	// Wait for completion
	producerDone.Wait()
	consumerDone.Wait()
	duration := time.Since(producerStart)

	// Verify all chunks delivered
	assert.Equal(t, numChunks, receivedCount, "All 256 chunks (1GB) should be delivered")
	t.Logf("✅ Successfully streamed %d chunks (1GB) in %v", numChunks, duration)
}

// TestHandleProxyResponse_SlowConsumer tests behavior when consumer is very slow.
func TestHandleProxyResponse_SlowConsumer(t *testing.T) {
	// Create tunnel with response channel
	tun := &tunnel.Tunnel{
		ID:          uuid.New(),
		ResponseMap: sync.Map{},
	}

	requestID := "slow-consumer-test"
	responseCh := make(chan *tunnelv1.ProxyResponse, 10)
	tun.ResponseMap.Store(requestID, responseCh)

	service := &TunnelService{
		tunnelManager: nil,
		tokenService:  nil,
	}

	// Producer: Send chunks quickly
	numChunks := 20
	var producerDone sync.WaitGroup
	producerDone.Add(1)
	go func() {
		defer producerDone.Done()
		for i := 0; i < numChunks; i++ {
			chunk := &tunnelv1.ProxyResponse{
				RequestId: requestID,
				TunnelId:  tun.ID.String(),
				Payload: &tunnelv1.ProxyResponse_Http{
					Http: &tunnelv1.HTTPResponse{
						StatusCode: 200,
						Body:       []byte("chunk"),
					},
				},
				EndOfStream: i == numChunks-1,
			}
			service.handleProxyResponse(tun, chunk)
		}
	}()

	// Consumer: Very slow (100ms per chunk)
	receivedCount := 0
	var consumerDone sync.WaitGroup
	consumerDone.Add(1)
	go func() {
		defer consumerDone.Done()
		for i := 0; i < numChunks; i++ {
			time.Sleep(100 * time.Millisecond) // Slow consumer
			<-responseCh
			receivedCount++
		}
	}()

	// Wait with timeout
	done := make(chan struct{})
	go func() {
		producerDone.Wait()
		consumerDone.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success: All chunks delivered despite slow consumer
		assert.Equal(t, numChunks, receivedCount)
		t.Logf("✅ Backpressure mechanism works: slow consumer handled correctly")
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout: Producer or consumer stuck (should not happen with blocking send)")
	}
}

// TestHandleProxyResponse_ChannelNotFound tests error handling when request ID not found.
func TestHandleProxyResponse_ChannelNotFound(_ *testing.T) {
	tun := &tunnel.Tunnel{
		ID:          uuid.New(),
		ResponseMap: sync.Map{},
	}

	service := &TunnelService{
		tunnelManager: nil,
		tokenService:  nil,
	}

	// Send response for non-existent request
	response := &tunnelv1.ProxyResponse{
		RequestId: "non-existent",
		TunnelId:  tun.ID.String(),
		Payload: &tunnelv1.ProxyResponse_Http{
			Http: &tunnelv1.HTTPResponse{
				StatusCode: 200,
			},
		},
	}

	// Should not panic, just log warning
	service.handleProxyResponse(tun, response)
	// Test passes if no panic occurs
}

// TestHandleProxyResponse_InvalidChannelType tests error handling for invalid channel type.
func TestHandleProxyResponse_InvalidChannelType(_ *testing.T) {
	tun := &tunnel.Tunnel{
		ID:          uuid.New(),
		ResponseMap: sync.Map{},
	}

	requestID := "invalid-type-test"
	// Store wrong type (string instead of channel)
	tun.ResponseMap.Store(requestID, "invalid-type")

	service := &TunnelService{
		tunnelManager: nil,
		tokenService:  nil,
	}

	response := &tunnelv1.ProxyResponse{
		RequestId: requestID,
		TunnelId:  tun.ID.String(),
		Payload: &tunnelv1.ProxyResponse_Http{
			Http: &tunnelv1.HTTPResponse{
				StatusCode: 200,
			},
		},
	}

	// Should not panic, just log error
	service.handleProxyResponse(tun, response)
	// Test passes if no panic occurs
}

// TestHandleProxyResponse_ConcurrentWrites tests concurrent writes to response channel.
func TestHandleProxyResponse_ConcurrentWrites(t *testing.T) {
	tun := &tunnel.Tunnel{
		ID:          uuid.New(),
		ResponseMap: sync.Map{},
	}

	requestID := "concurrent-test"
	responseCh := make(chan *tunnelv1.ProxyResponse, 50)
	tun.ResponseMap.Store(requestID, responseCh)

	service := &TunnelService{
		tunnelManager: nil,
		tokenService:  nil,
	}

	// Multiple producers writing concurrently
	numProducers := 5
	chunksPerProducer := 10
	totalChunks := numProducers * chunksPerProducer

	var wg sync.WaitGroup
	wg.Add(numProducers)

	for p := 0; p < numProducers; p++ {
		go func(_ int) {
			defer wg.Done()
			for i := 0; i < chunksPerProducer; i++ {
				chunk := &tunnelv1.ProxyResponse{
					RequestId: requestID,
					TunnelId:  tun.ID.String(),
					Payload: &tunnelv1.ProxyResponse_Http{
						Http: &tunnelv1.HTTPResponse{
							StatusCode: 200,
							Body:       []byte("data"),
						},
					},
				}
				service.handleProxyResponse(tun, chunk)
			}
		}(p)
	}

	// Consumer
	receivedCount := 0
	consumerDone := make(chan struct{})
	go func() {
		for i := 0; i < totalChunks; i++ {
			<-responseCh
			receivedCount++
		}
		close(consumerDone)
	}()

	// Wait for all producers
	wg.Wait()

	// Wait for consumer
	select {
	case <-consumerDone:
		assert.Equal(t, totalChunks, receivedCount, "All chunks from concurrent producers should be delivered")
		t.Logf("✅ Concurrent writes handled correctly: %d chunks from %d producers", totalChunks, numProducers)
	case <-time.After(5 * time.Second):
		t.Fatalf("Timeout waiting for consumer (received %d/%d)", receivedCount, totalChunks)
	}
}
