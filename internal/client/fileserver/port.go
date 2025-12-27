package fileserver

import (
	"fmt"
	"net"
)

// FindAvailablePort finds an available TCP port by asking the OS to allocate one.
// Returns the port number or an error if no port is available.
func FindAvailablePort() (int, error) {
	// Listen on port 0 to get an available port from the OS
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("failed to find available port: %w", err)
	}
	defer listener.Close()

	// Get the port that was allocated
	addr := listener.Addr().(*net.TCPAddr)
	return addr.Port, nil
}
