# Grok - ngrok Clone

> Production-ready ngrok clone built with Go, gRPC, and React

Grok is a self-hosted tunneling solution that allows you to expose local services to the internet through secure tunnels.

## Features

- **gRPC Bidirectional Streaming** - Efficient tunnel communication
- **Multi-Protocol Support** - HTTP, HTTPS, TCP with TLS termination
- **Custom & Random Subdomains** - Flexible domain allocation
- **Web Dashboard** - React-based management interface with real-time updates
- **Token-based Authentication** - Secure access control
- **Database Support** - SQLite (default) and PostgreSQL
- **Auto-reconnection** - Resilient client connections
- **Semantic Versioning** - Automated releases with GitHub Actions

## Architecture

```
Internet Client → Server (Reverse Proxy) → Tunnel Manager → gRPC Stream → Client → Local Service
```

## Project Structure

```
grok/
├── cmd/
│   ├── grok-server/     # Server entry point
│   └── grok/            # Client CLI
├── proto/               # Protobuf definitions
├── internal/
│   ├── server/          # Server implementation
│   ├── client/          # Client implementation
│   └── db/              # Database models & migrations
├── pkg/                 # Shared packages
├── gen/                 # Generated gRPC code
├── web/                 # React dashboard
└── configs/             # Configuration examples
```

## Getting Started

### Prerequisites

- Go 1.25+
- Protocol Buffers compiler (`protoc`)
- Database: PostgreSQL 15+ **or** SQLite 3+ (SQLite is default for easier development)

### Quick Start (5 minutes)

1. **Clone and Build**
   ```bash
   git clone https://github.com/pandeptwidyaop/grok.git
   cd grok
   go mod download
   make install-tools
   make build-all
   ```

2. **Start Server (Terminal 1)**
   ```bash
   ./bin/grok-server --config configs/server.yaml
   ```

   The server will start on:
   - gRPC: `:4443`
   - HTTP Proxy: `:3080`
   - API (future): `:4040`

3. **Create Auth Token**

   Open SQLite database and create a token:
   ```bash
   sqlite3 grok.db
   ```

   Run these SQL commands:
   ```sql
   -- Create a user
   INSERT INTO users (id, email, password, name, is_active, created_at, updated_at)
   VALUES (
     lower(hex(randomblob(16))),
     'demo@example.com',
     'hashed_password',
     'Demo User',
     1,
     datetime('now'),
     datetime('now')
   );

   -- Get the user ID
   SELECT id FROM users WHERE email = 'demo@example.com';

   -- Create auth token (replace USER_ID with actual ID from above)
   INSERT INTO auth_tokens (id, user_id, token_hash, name, is_active, created_at)
   VALUES (
     lower(hex(randomblob(16))),
     'USER_ID_HERE',
     'token_hash_placeholder',
     'Demo Token',
     1,
     datetime('now')
   );

   -- Or use this simpler method (for testing only):
   -- Token: grok_demo_token_12345678
   INSERT INTO auth_tokens (id, user_id, token_hash, name, is_active, created_at)
   SELECT
     lower(hex(randomblob(16))),
     id,
     '9a3f094c0e3a5e5c8e5c3a2f1e9d8c7b6a5e4d3c2b1a0f9e8d7c6b5a4e3d2c1b',
     'Demo Token',
     1,
     datetime('now')
   FROM users LIMIT 1;
   ```

   For production, use the actual token hash:
   ```bash
   # Token format: grok_<32_random_hex_chars>
   # Hash it with SHA256 before storing
   echo -n "grok_your_actual_token" | sha256sum
   ```

4. **Configure Client (Terminal 2)**
   ```bash
   ./bin/grok config set-token grok_demo_token_12345678
   ```

5. **Start Test HTTP Server (Terminal 3)**
   ```bash
   # Simple Python HTTP server
   python3 -m http.server 8000
   ```

6. **Start Tunnel (Terminal 2)**
   ```bash
   ./bin/grok http 8000
   ```

   You'll see output like:
   ```
   ╔═════════════════════════════════════════════════════════╗
   ║                 Tunnel Active                           ║
   ╠═════════════════════════════════════════════════════════╣
   ║  Public URL:  https://abc123.localhost                  ║
   ║  Local Addr:  localhost:8000                            ║
   ║  Protocol:    http                                      ║
   ╚═════════════════════════════════════════════════════════╝
   ```

7. **Test Your Tunnel (Terminal 4)**
   ```bash
   curl http://abc123.localhost:3080
   ```

### Full Installation

1. **Clone the repository**
   ```bash
   git clone https://github.com/pandeptwidyaop/grok.git
   cd grok
   ```

2. **Install dependencies**
   ```bash
   go mod download
   make install-tools
   ```

3. **Setup database**

   **Option A: SQLite (Default - Easiest)**
   ```bash
   # No setup needed! Database file will be created automatically
   # Server will create grok.db on first run
   cp configs/server.yaml configs/server.yaml
   # Ensure: driver: "sqlite" and database: "grok.db"
   ```

   **Option B: PostgreSQL (Production)**
   ```bash
   # Create database
   createdb -U postgres grok

   # Run migrations
   psql -U postgres -d grok -f internal/db/migrations/001_init.sql

   # Configure server
   cp configs/server.postgres.yaml configs/server.yaml
   # Edit with your PostgreSQL credentials
   ```

4. **Build binaries**
   ```bash
   make build-all
   ```

## Development

### Generate gRPC Code
```bash
make proto
```

### Build
```bash
# Build all components
make build-all

# Build server only
make build-server

# Build client only
make build-client
```

### Run Server
```bash
make dev-server
# or
./bin/grok-server --config configs/server.yaml
```

### Run Client

**First, configure your auth token:**
```bash
./bin/grok config set-token grok_your_token_here
```

**HTTP Tunnels:**
```bash
# Expose local web server (port only)
./bin/grok http 3000

# Expose with full address
./bin/grok http localhost:8080

# Custom subdomain
./bin/grok http 3000 --subdomain myapp
# Accessible at: http://myapp.localhost:3080

# Override server address
./bin/grok http 3000 --server your-server.com:4443

# Override token for single session
./bin/grok http 3000 --token grok_temporary_token
```

**TCP Tunnels:**
```bash
# SSH tunnel
./bin/grok tcp 22

# MySQL tunnel
./bin/grok tcp 3306

# PostgreSQL tunnel
./bin/grok tcp 5432
```

**Configuration:**
```bash
# View help
./bin/grok --help
./bin/grok http --help

# Check version
./bin/grok version
```

## Configuration

### Server Configuration

See `configs/server.example.yaml` for all available options:

- **Server**: Ports, domain configuration
- **Database**: SQLite or PostgreSQL
  - **SQLite** (default): `driver: sqlite`, `database: grok.db` or `:memory:`
  - **PostgreSQL**: `driver: postgres` + connection settings
- **TLS**: Auto-cert with Let's Encrypt or manual certificates
- **Auth**: JWT secret for token validation
- **Tunnels**: Limits and timeouts
- **Logging**: Level, format, output

Example configs provided:
- `configs/server.sqlite.yaml` - SQLite configuration (development)
- `configs/server.postgres.yaml` - PostgreSQL configuration (production)

### Client Configuration

See `configs/client.example.yaml` for client options:

- **Server**: Connection address and TLS settings
- **Auth**: Authentication token
- **Logging**: Client-side logging configuration

## Database Schema

The project supports **SQLite** (default) or **PostgreSQL** with GORM for ORM. Main tables:

- `users` - User accounts
- `auth_tokens` - Authentication tokens
- `domains` - Custom subdomains
- `tunnels` - Active tunnel sessions
- `request_logs` - HTTP request logs for analytics

See `internal/db/migrations/001_init.sql` for complete schema.

## Usage Examples

### Example 1: Expose Local Development Server

```bash
# Terminal 1: Start your dev server
npm run dev  # Running on localhost:5173

# Terminal 2: Create tunnel
./bin/grok http 5173 --subdomain myproject

# Share this URL with your team:
# http://myproject.localhost:3080
```

### Example 2: Test Webhooks Locally

```bash
# Terminal 1: Start your webhook receiver
python webhook_server.py  # Port 8000

# Terminal 2: Create tunnel
./bin/grok http 8000

# Use the generated URL in webhook settings:
# http://xyz789.localhost:3080/webhook
```

### Example 3: SSH Access Through Firewall

```bash
# Create TCP tunnel for SSH
./bin/grok tcp 22

# Connect from remote machine:
# ssh user@your-server.com -p <assigned_port>
```

### Example 4: Database Access for Remote Team

```bash
# Expose PostgreSQL
./bin/grok tcp 5432

# Team members can connect:
# psql -h your-server.com -p <assigned_port> -U username
```

### Example 5: Multiple Tunnels

```bash
# Terminal 1: Frontend
./bin/grok http 3000 --subdomain frontend

# Terminal 2: Backend API
./bin/grok http 8000 --subdomain api

# Terminal 3: Database
./bin/grok tcp 5432
```

### Example 6: Access Web Dashboard

```bash
# Start server
./bin/grok-server --config configs/server.yaml

# Open browser and navigate to:
# http://localhost:4040

# Features available in dashboard:
# - Create and manage authentication tokens
# - Monitor active tunnels in real-time
# - View tunnel statistics (requests, bytes transferred)
# - Monitor tunnel status and connected time
```

## Development Roadmap

### Phase 1: Foundation ✅ (Completed)
- [x] Project structure setup
- [x] gRPC protocol definition
- [x] Database models and migrations (SQLite + PostgreSQL)
- [x] Logging infrastructure (zerolog)
- [x] Configuration management (Viper)

### Phase 2: Server Core ✅ (Completed)
- [x] gRPC TunnelService implementation
- [x] Tunnel Manager with sync.Map
- [x] Authentication service (JWT + SHA256)
- [x] Subdomain allocation (random + custom)
- [x] Token validation and user management

### Phase 3: Proxy Layer ✅ (Completed)
- [x] HTTP reverse proxy
- [x] Subdomain routing
- [x] Request/response correlation
- [x] Graceful shutdown handling

### Phase 4: Client ✅ (Completed)
- [x] CLI implementation (Cobra)
- [x] gRPC client with bidirectional streaming
- [x] Local HTTP forwarding
- [x] Local TCP forwarding (basic)
- [x] Auto-reconnection with exponential backoff
- [x] Heartbeat keep-alive mechanism

### Phase 5: Dashboard ✅ (Completed)
- [x] React + Vite + Shadcn UI setup
- [x] Token management interface
- [x] Tunnel monitoring dashboard
- [x] Real-time tunnel status
- [x] HTTP API endpoints (tokens, tunnels, stats)
- [x] Dashboard embedded in server binary

### Phase 6: Testing & Polish
- [ ] Unit tests
- [ ] Integration tests
- [ ] E2E tests
- [ ] Performance testing
- [ ] Documentation improvements
- [ ] Docker packaging
- [ ] Kubernetes deployment guides

## Troubleshooting

### Client Cannot Connect to Server

```bash
# Check if server is running
ps aux | grep grok-server

# Check server logs
tail -f server.log

# Test gRPC connection
grpcurl -plaintext localhost:4443 list

# Verify firewall/network
nc -zv localhost 4443
```

### Authentication Failed

```bash
# Verify token is configured
cat ~/.grok/config.yaml

# Check token hash in database
sqlite3 grok.db "SELECT token_hash FROM auth_tokens WHERE is_active=1;"

# Test with inline token
./bin/grok http 3000 --token grok_your_token
```

### Tunnel Not Receiving Requests

```bash
# Check if tunnel is registered
# Look for "Tunnel registered successfully" in server logs

# Verify subdomain
curl -v http://your-subdomain.localhost:3080

# Check HTTP proxy is running
netstat -an | grep 3080
```

### Database Issues

```bash
# SQLite: Check database file
ls -lh grok.db

# SQLite: Verify tables
sqlite3 grok.db ".tables"

# PostgreSQL: Check connection
psql -U grok -d grok -c "\dt"

# Reset database (CAUTION: destroys data)
rm grok.db
./bin/grok-server  # Will recreate
```

### Build Errors

```bash
# Update dependencies
go mod tidy
go mod download

# Regenerate gRPC code
make proto

# Clean and rebuild
make clean
make build-all
```

## Commands

```bash
# Development
make help              # Show all available commands
make proto             # Generate gRPC code
make dev-server        # Run server in dev mode
make dev-client        # Run client in dev mode

# Building
make build-all         # Build everything
make build-server      # Build server
make build-client      # Build client

# Database
make migrate-up        # Run migrations (PostgreSQL only)
make migrate-down      # Rollback migrations

# Testing
make test              # Run tests

# Cleanup
make clean             # Remove build artifacts
```

## Production Deployment

### Server Setup

1. **Use PostgreSQL for production**
   ```bash
   # configs/server.yaml
   database:
     driver: postgres
     host: your-db-host
     port: 5432
     database: grok_prod
     username: grok_user
     password: ${DB_PASSWORD}  # Use env var
   ```

2. **Enable TLS**
   ```bash
   server:
     domain: "tunnel.yourdomain.com"

   tls:
     auto_cert: true
     cert_dir: "/var/lib/grok/certs"
   ```

3. **Set strong JWT secret**
   ```bash
   auth:
     jwt_secret: "${JWT_SECRET}"  # Use env var, min 32 chars
   ```

4. **Configure limits**
   ```bash
   tunnels:
     max_per_user: 10
     idle_timeout: "10m"
     heartbeat_interval: "30s"
   ```

### Systemd Service

Create `/etc/systemd/system/grok-server.service`:

```ini
[Unit]
Description=Grok Tunnel Server
After=network.target postgresql.service

[Service]
Type=simple
User=grok
WorkingDirectory=/opt/grok
ExecStart=/opt/grok/bin/grok-server --config /opt/grok/configs/server.yaml
Restart=always
RestartSec=5
Environment="DB_PASSWORD=your_secure_password"
Environment="JWT_SECRET=your_jwt_secret_min_32_chars"

[Install]
WantedBy=multi-user.target
```

Enable and start:
```bash
sudo systemctl enable grok-server
sudo systemctl start grok-server
sudo systemctl status grok-server
```

### Nginx Reverse Proxy (Optional)

```nginx
# HTTP Proxy
server {
    listen 80;
    server_name *.tunnel.yourdomain.com;

    location / {
        proxy_pass http://localhost:3080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }
}

# gRPC
server {
    listen 4443 http2;
    server_name tunnel.yourdomain.com;

    location / {
        grpc_pass grpc://localhost:4443;
    }
}
```

## Tech Stack

- **Language**: Go 1.25+
- **gRPC**: google.golang.org/grpc
- **Database**: SQLite or PostgreSQL + GORM
- **CLI**: Cobra + Viper
- **Web**: Fiber v3
- **Logging**: zerolog
- **Auth**: JWT (golang-jwt/jwt)
- **Frontend**: React + Vite + Shadcn UI (coming soon)

## License

MIT

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Author

**Pandept Widya**
- GitHub: [@pandeptwidyaop](https://github.com/pandeptwidyaop)
