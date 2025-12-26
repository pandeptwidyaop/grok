# TCP Tunnel Implementation - Design Document

## Overview

Implementasi TCP tunnel dengan sistem alokasi port otomatis di sisi server. Berbeda dengan HTTP/HTTPS tunnel yang menggunakan subdomain routing, TCP tunnel membutuhkan alokasi port unik untuk setiap tunnel.

## Arsitektur

### HTTP/HTTPS vs TCP Tunnel

**HTTP/HTTPS Tunnel:**
```
Client Request → subdomain.domain.com:80/443 → Proxy routing by subdomain → Tunnel → Local Service
```

**TCP Tunnel:**
```
Client Connection → domain.com:12345 (allocated port) → TCP Proxy → Tunnel → Local Service
```

## Komponen yang Diperlukan

### 1. Port Pool Manager

**File**: `internal/server/tcp/port_pool.go`

Mengelola pool port yang tersedia untuk TCP tunnels:

```go
type PortPool struct {
    startPort    int           // e.g., 10000
    endPort      int           // e.g., 20000
    allocatedPorts map[int]uuid.UUID  // port → tunnel_id
    availablePorts []int        // Queue of available ports
    mu           sync.RWMutex
    db           *gorm.DB
}

// Methods:
- AllocatePort(tunnelID uuid.UUID) (int, error)
- ReleasePort(port int) error
- IsPortAvailable(port int) bool
- GetAllocatedPorts() map[int]uuid.UUID
- LoadAllocatedPorts() error  // Restore from DB on startup
```

**Fitur**:
- Thread-safe allocation/deallocation
- Persist port allocation to database
- Auto-recovery on server restart
- Port reuse prevention

### 2. TCP Proxy Server

**File**: `internal/server/tcp/tcp_proxy.go`

Dynamic TCP listener yang membuka port sesuai alokasi:

```go
type TCPProxy struct {
    tunnelManager *tunnel.Manager
    listeners     map[int]*net.Listener  // port → listener
    mu            sync.RWMutex
    done          chan struct{}
}

// Methods:
- StartListener(port int, tunnelID uuid.UUID) error
- StopListener(port int) error
- HandleConnection(conn net.Conn, tunnelID uuid.UUID)
- ForwardToTunnel(conn net.Conn, tunnel *tunnel.Tunnel)
```

**Alur Koneksi**:
1. Client connect ke `domain.com:12345`
2. TCP Proxy terima koneksi
3. Lookup tunnel ID by port
4. Forward raw bytes via gRPC stream ke client
5. Client forward ke local service
6. Response kembali via stream
7. TCP Proxy kirim response ke client

### 3. Database Schema Update

**Field sudah ada di model**: `RemotePort *int` ✅

Cukup pastikan field ini diisi saat TCP tunnel dibuat.

### 4. Tunnel Manager Update

**File**: `internal/server/tunnel/manager.go`

Tambahkan integration dengan PortPool:

```go
type Manager struct {
    // ... existing fields
    portPool  *tcp.PortPool  // NEW
    tcpProxy  *tcp.TCPProxy  // NEW
}

// Update methods:
func (m *Manager) RegisterTunnel(ctx context.Context, t *Tunnel) error {
    if t.Protocol == tunnelv1.TunnelProtocol_TCP {
        // Allocate port
        port, err := m.portPool.AllocatePort(t.ID)
        if err != nil {
            return err
        }

        // Start TCP listener
        if err := m.tcpProxy.StartListener(port, t.ID); err != nil {
            m.portPool.ReleasePort(port)
            return err
        }

        // Update tunnel with allocated port
        t.RemotePort = &port

        // Update database
        m.db.Model(&models.Tunnel{}).Where("id = ?", t.ID).Update("remote_port", port)
    }

    // ... rest of registration
}

func (m *Manager) UnregisterTunnel(ctx context.Context, tunnelID uuid.UUID) error {
    tunnel := // ... get tunnel

    if tunnel.RemotePort != nil {
        // Stop TCP listener
        m.tcpProxy.StopListener(*tunnel.RemotePort)

        // Release port for persistent tunnels (mark as available but don't deallocate)
        // For non-persistent, fully deallocate
        if !tunnel.IsPersistent {
            m.portPool.ReleasePort(*tunnel.RemotePort)
        }
    }

    // ... rest of cleanup
}
```

### 5. BuildPublicURL Update

```go
func (m *Manager) BuildPublicURL(subdomain string, protocol string) string {
    if protocol == "tcp" {
        // TCP tunnels don't use subdomain, return placeholder
        // Actual URL will be set after port allocation
        return "tcp://pending-allocation"
    }

    // ... HTTP/HTTPS logic
}

// New method for TCP
func (m *Manager) BuildTCPPublicURL(port int) string {
    return fmt.Sprintf("tcp://%s:%d", m.baseDomain, port)
}
```

### 6. Configuration Update

**File**: `configs/server.yaml`

Tambah konfigurasi TCP port range:

```yaml
server:
  grpc_port: 4443
  http_port: 3080
  https_port: 8443
  api_port: 4040
  domain: "localhost"

  # TCP tunnel port allocation
  tcp_port_start: 10000
  tcp_port_end: 20000
```

### 7. gRPC Service Update

**File**: `internal/server/grpc/tunnel_service.go`

Update `CreateTunnel` untuk TCP:

```go
func (s *TunnelService) CreateTunnel(...) (*tunnelv1.CreateTunnelResponse, error) {
    // ... existing validation

    var publicURL string
    var allocatedPort int32

    if req.Protocol == tunnelv1.TunnelProtocol_TCP {
        // Allocate port first
        port, err := s.tunnelManager.AllocatePortForUser(ctx, authToken.UserID)
        if err != nil {
            return nil, status.Error(codes.ResourceExhausted, "no available ports")
        }

        allocatedPort = int32(port)
        publicURL = s.tunnelManager.BuildTCPPublicURL(port)
    } else {
        // ... existing HTTP/HTTPS logic
    }

    return &tunnelv1.CreateTunnelResponse{
        TunnelId:  "",
        PublicUrl: publicURL,
        Subdomain: fullSubdomain,  // Empty for TCP
        Status:    tunnelv1.TunnelStatus_ACTIVE,
        RemotePort: allocatedPort,  // NEW field in proto
    }, nil
}
```

### 8. Proto Update (Optional)

**File**: `proto/tunnel/v1/tunnel.proto`

Add `remote_port` to response if not already there:

```protobuf
message CreateTunnelResponse {
    string tunnel_id = 1;
    string public_url = 2;
    string subdomain = 3;
    TunnelStatus status = 4;
    int32 remote_port = 5;  // NEW - for TCP tunnels
}
```

### 9. CLI Update

**File**: `internal/client/cli/tcp.go`

Sudah ada command `tcp`, pastikan berfungsi dengan baik:

```bash
./grok tcp 22            # Expose local SSH
./grok tcp 3306          # Expose local MySQL
./grok tcp 5432 --name db-tunnel  # Persistent TCP tunnel
```

Output:
```
Tunnel established!
Public URL: tcp://localhost:12345
Forwarding to: localhost:22
```

## Alur Implementasi

### Phase 1: Port Pool Manager (Hari 1)
- [x] Buat `PortPool` struct
- [x] Implement allocation/deallocation
- [x] Database persistence
- [x] Unit tests

### Phase 2: TCP Proxy (Hari 2)
- [ ] Buat `TCPProxy` struct
- [ ] Dynamic listener management
- [ ] Connection forwarding via gRPC
- [ ] Integration dengan tunnel manager

### Phase 3: Integration (Hari 3)
- [ ] Update tunnel manager
- [ ] Update gRPC service
- [ ] Update configuration
- [ ] Update proto jika perlu

### Phase 4: Testing (Hari 4)
- [ ] Unit tests
- [ ] Integration tests
- [ ] End-to-end TCP tunnel flow
- [ ] Load testing (multiple concurrent TCP tunnels)

### Phase 5: Dashboard (Hari 5)
- [ ] Show allocated port in tunnel list
- [ ] Real-time TCP tunnel stats
- [ ] TCP-specific connection metrics

## Contoh Penggunaan

### 1. Expose SSH Server

```bash
# Server side
./bin/grok-server --config configs/server.yaml

# Client side
./bin/grok tcp 22 --name ssh-tunnel

# Output:
# Tunnel established!
# Public URL: tcp://localhost:12345
# Forwarding to: localhost:22
```

Connect dari luar:
```bash
ssh -p 12345 user@yourdomain.com
```

### 2. Expose MySQL Database

```bash
./bin/grok tcp 3306 --name mysql-tunnel

# Output:
# Tunnel established!
# Public URL: tcp://localhost:15678
# Forwarding to: localhost:3306
```

Connect:
```bash
mysql -h yourdomain.com -P 15678 -u user -p
```

### 3. Persistent TCP Tunnel

```bash
# First connection
./bin/grok tcp 5432 --name postgres-prod

# Output:
# Tunnel established!
# Public URL: tcp://localhost:18901
# Forwarding to: localhost:5432
# Saved name: postgres-prod (port 18901 reserved)
```

Disconnect and reconnect:
```bash
# Reconnect with same name → same port!
./bin/grok tcp 5433 --name postgres-prod

# Output:
# Tunnel re-established!
# Public URL: tcp://localhost:18901  ← Same port!
# Forwarding to: localhost:5433      ← New local port
```

## Persistent TCP Tunnels

### Port Preservation
- Saat TCP tunnel disconnect, port **TIDAK** di-deallocate jika tunnel persistent
- Port reserved untuk tunnel tersebut
- Saat reconnect dengan `saved_name` yang sama, gunakan port yang sama
- User bisa reliably share `tcp://domain:12345` karena port tidak berubah

### Database Schema
```sql
-- Tunnel dengan RemotePort
INSERT INTO tunnels (
    id, user_id, tunnel_type, remote_port, saved_name, is_persistent, status
) VALUES (
    'uuid', 'user-uuid', 'tcp', 12345, 'ssh-tunnel', true, 'offline'
);

-- Port allocation tracking
-- Option 1: Use tunnels.remote_port directly
-- Option 2: Separate port_allocations table (if need history)
```

## Performance Considerations

### 1. Port Pool Size
- Default: 10000 ports (10000-20000)
- Support up to 10,000 concurrent TCP tunnels
- Configurable via config

### 2. Memory Usage
- PortPool: ~200KB for 10,000 ports (map overhead)
- Each TCP listener: ~10KB
- Total overhead: ~500KB - 1MB

### 3. Connection Limits
- Operating system limits: `ulimit -n` (file descriptors)
- Recommend: Set to 65535 for production
- Each TCP connection = 1 file descriptor

### 4. Port Reuse
- Use `SO_REUSEADDR` untuk fast restart
- Prevent "address already in use" errors
- Graceful shutdown to close all listeners

## Security Considerations

### 1. Port Range Isolation
- Use high ports (10000+) to avoid conflicts with system services
- Prevent allocation of privileged ports (< 1024)
- Firewall rules untuk restrict access

### 2. Rate Limiting
- Per-user TCP tunnel limits (default: 10)
- Connection rate limiting per tunnel
- Bandwidth throttling (future)

### 3. Authentication
- TCP connections tidak bisa di-auth di layer TCP
- Auth dilakukan di gRPC layer (tunnel creation)
- Monitor for abuse (connection flooding)

## Testing Strategy

### Unit Tests
```go
// Test port allocation
func TestPortPool_AllocatePort(t *testing.T)
func TestPortPool_ReleasePort(t *testing.T)
func TestPortPool_Concurrent(t *testing.T)

// Test TCP proxy
func TestTCPProxy_StartListener(t *testing.T)
func TestTCPProxy_HandleConnection(t *testing.T)
func TestTCPProxy_ForwardData(t *testing.T)
```

### Integration Tests
```go
// Test full TCP tunnel flow
func TestTCPTunnel_EndToEnd(t *testing.T) {
    // 1. Create TCP tunnel
    // 2. Verify port allocated
    // 3. Connect to public port
    // 4. Verify data forwarded to local service
    // 5. Verify response received
    // 6. Disconnect
    // 7. Verify port released (if not persistent)
}
```

### Load Tests
```bash
# Test concurrent TCP connections
ab -n 1000 -c 100 tcp://localhost:12345

# Test port allocation under load
for i in {1..100}; do
    ./grok tcp 300$i --name tcp-$i &
done
```

## Migration Strategy

### Database Migration
```sql
-- Field already exists, just ensure it's nullable
ALTER TABLE tunnels MODIFY remote_port INT NULL;

-- Create index for fast port lookup
CREATE INDEX idx_tunnels_remote_port ON tunnels(remote_port);
```

### Backward Compatibility
- Existing HTTP/HTTPS tunnels tidak terpengaruh
- `remote_port` nullable → backward compatible
- Old clients tanpa TCP support tetap bisa connect
- Feature flag untuk enable/disable TCP tunnels

## Monitoring & Metrics

### Key Metrics
- Available ports count
- Allocated ports count
- TCP connections per tunnel
- Bytes transferred per TCP tunnel
- Connection errors / timeouts

### Logging
```
[INFO] TCP Tunnel created: port=12345, tunnel_id=xxx, user_id=yyy
[INFO] TCP Connection established: port=12345, client_ip=1.2.3.4
[DEBUG] TCP Data forwarded: port=12345, bytes_in=1024, bytes_out=512
[INFO] TCP Tunnel disconnected: port=12345, duration=5m, total_bytes=10MB
[WARN] Port allocation failed: no available ports (pool exhausted)
```

## Future Enhancements

1. **Dynamic Port Range Expansion** - Auto-expand port range when near capacity
2. **Port Reservation** - User request specific port (admin only)
3. **TCP Load Balancing** - Multiple tunnels on same port (round-robin)
4. **UDP Support** - Similar architecture for UDP tunnels
5. **Connection Pooling** - Reuse connections for performance
6. **Bandwidth Limits** - Per-tunnel bandwidth caps
7. **Connection Filtering** - IP whitelist/blacklist
8. **TLS over TCP** - Secure TCP tunnels with TLS wrapping

---

**Status**: Design Complete - Ready for Implementation
**Estimated Time**: 5 days
**Complexity**: Medium-High
**Priority**: High (core feature)
