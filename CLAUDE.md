# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Grok is a production-ready ngrok clone built with Go, gRPC, and React. It's a self-hosted tunneling solution that allows exposing local services to the internet through secure tunnels using bidirectional gRPC streaming.

**Key Technologies:**
- Backend: Go 1.25+, gRPC, GORM
- Database: SQLite (development) or PostgreSQL (production)
- Frontend: React + Vite + Shadcn UI + TanStack Query
- CLI: Cobra + Viper

## Common Commands

### Development Setup
```bash
# Install development tools (protoc plugins)
make install-tools

# Generate gRPC code from proto files
make proto

# Build everything (dashboard, server, client)
make build-all

# Build individual components
make build-server    # Builds bin/grok-server
make build-client    # Builds bin/grok
make build-dashboard # Builds web/dist/
```

### Running the Application

**IMPORTANT: For development, ALWAYS use `air` for hot reload on the backend and `npm run dev` for the frontend.**

```bash
# PREFERRED: Run server with hot reload using air
air
# This watches for file changes and automatically rebuilds/restarts the server

# Install air if not already installed:
go install github.com/cosmtrek/air@latest

# Alternative: Run without hot reload (not recommended for development)
make dev-server
go run ./cmd/grok-server/main.go --config configs/server.example.yaml
./bin/grok-server --config configs/server.yaml

# Run client
./bin/grok http 3000                    # Expose port 3000
./bin/grok http 3000 --subdomain myapp  # Custom subdomain
./bin/grok tcp 22                       # TCP tunnel for SSH
```

### Testing
```bash
# Run all tests
go test ./...
make test

# Run with coverage
go test -cover ./...
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run specific test suites
go test -v ./tests/integration/              # Integration tests
go test -v ./internal/server/auth/           # Auth unit tests
go test -v ./internal/server/tunnel/         # Tunnel manager unit tests

# Run single test
go test -v -run TestCompleteTunnelFlow ./tests/integration/

# Run with race detection
go test -race ./...
```

### Web Dashboard Development

**IMPORTANT: ALWAYS use `npm run dev` for development with hot reload.**

```bash
cd web
npm install

# PREFERRED: Run with hot reload (Vite dev server)
npm run dev      # Starts dev server at http://localhost:5173 with hot reload

# Production build (only when needed)
npm run build    # Builds to web/dist/

# Linting
npm run lint     # ESLint
```

### Database Management
```bash
# SQLite (default for development)
sqlite3 grok.db
sqlite3 grok.db ".tables"
sqlite3 grok.db "SELECT * FROM users;"

# PostgreSQL migrations
make migrate-up
psql -U grok -d grok -f internal/db/migrations/001_init.sql
```

### Cleanup
```bash
make clean  # Removes bin/, web/dist/, internal/server/web/dist/, gen/
```

## Architecture Overview

### Request Flow
```
Internet Client → Server (HTTP Proxy) → Tunnel Manager → gRPC Stream → Client → Local Service
                                                ↓
                                          Database (SQLite/Postgres)
```

### Key Components

#### Server Architecture (`internal/server/`)
- **grpc/tunnel_service.go**: gRPC service implementation handling CreateTunnel, ProxyStream, and Heartbeat RPCs
- **tunnel/manager.go**: Core tunnel management with `sync.Map` for in-memory state
  - Manages subdomain allocation (custom and random 8-char generation)
  - Handles tunnel registration/unregistration
  - Critical: Cleans up domain reservations on disconnect to allow subdomain reuse
- **proxy/http_proxy.go**: HTTP reverse proxy that routes requests based on subdomain
- **auth/token_service.go**: JWT-based authentication with SHA256 token hashing
- **web/api.go**: REST API endpoints for dashboard (tokens, tunnels, stats)

#### Client Architecture (`internal/client/`)
- **cli/**: Cobra commands for CLI interface (http, tcp, config)
- **tunnel/client.go**: gRPC client with bidirectional streaming
- **proxy/**: HTTP and TCP forwarding to local services
  - Auto-reconnection with exponential backoff
  - Heartbeat keep-alive mechanism

#### Database Models (`internal/db/models/`)
- **User**: User accounts
- **AuthToken**: Authentication tokens (SHA256 hashed)
- **Domain**: Subdomain reservations
- **Tunnel**: Active tunnel sessions with metrics (bytes, requests)
- **RequestLog**: HTTP request analytics

### gRPC Protocol (`proto/tunnel/v1/tunnel.proto`)

The tunnel protocol uses bidirectional streaming for efficient request/response proxying:

1. **CreateTunnel RPC**: Client authenticates and requests tunnel, server allocates subdomain
2. **ProxyStream RPC**: Bidirectional stream for tunneling
   - Server sends `ProxyRequest` when public HTTP request arrives
   - Client forwards to local service and sends `ProxyResponse`
   - Supports both HTTP and TCP protocols
3. **Heartbeat RPC**: Maintains connection health

**Critical Implementation Detail**: Tunnel registration uses pipe-delimited data format:
```
subdomain|token|localaddr|publicurl
```
This ensures all tunnel metadata is properly persisted and displayed in the dashboard.

### Web Dashboard (`web/`)

- **React + Vite** with TypeScript
- **TanStack Query** for data fetching and caching
- **TanStack Table** for tunnel/token management
- **Shadcn UI** components
- **React Router** for navigation

Embedded in server binary at build time (served from `internal/server/web/dist/`).

## Important Development Notes

### Development Workflow (CRITICAL)

**ALWAYS use hot reload during development:**

1. **Backend Development**: Use `air` for automatic rebuild/restart
   ```bash
   air  # Watches Go files and auto-reloads server
   ```

2. **Frontend Development**: Use `npm run dev` for Vite hot reload
   ```bash
   cd web && npm run dev  # Vite dev server with HMR
   ```

3. **Full Stack Development**: Run both simultaneously in separate terminals
   ```bash
   # Terminal 1: Backend with air
   air

   # Terminal 2: Frontend with Vite
   cd web && npm run dev

   # Terminal 3: Client for testing
   ./bin/grok http 3000
   ```

**Never use `go run` or `make dev-server` during active development** - they don't support hot reload and will slow down the development cycle.

### Token Format and Hashing
- Token format: `grok_<32_random_hex_chars>`
- Stored as SHA256 hash in database
- Always validate token before hashing: `strings.HasPrefix(token, "grok_")`

### Subdomain Allocation
- Reserved subdomains: api, admin, www, status, dashboard, docs, blog, support, help
- Custom subdomains must be validated with `utils.IsValidSubdomain()`
- Random subdomains are 8 characters using `utils.GenerateRandomSubdomain()`
- **Critical**: Domain reservations MUST be deleted when tunnel disconnects (see `tunnel.Manager.UnregisterTunnel`)

### Database Migrations
- SQLite: Auto-migration with GORM (`db.AutoMigrate(&models.User{}, ...)`)
- PostgreSQL: Manual migrations in `internal/db/migrations/001_init.sql`
- Use `:memory:` for tests (fast, isolated)

### Testing Infrastructure
- Integration tests: Use `bufconn` for in-memory gRPC connections
- All tests use in-memory SQLite for isolation
- Test fixtures: `setupTestServer()`, `setupTestDB()`, `createTestUser()`, `createTestToken()`
- Always defer cleanup: `defer grpcServer.Stop()`

### Configuration Management
- Server config: `configs/server.example.yaml` (Viper)
- Client config: `~/.grok/config.yaml` (Viper)
- Use environment variables for secrets: `${DB_PASSWORD}`, `${JWT_SECRET}`

### Logging
- Uses `zerolog` for structured JSON logging
- Import from `pkg/logger`
- Methods: `.InfoEvent()`, `.ErrorEvent()`, `.WarnEvent()`, `.DebugEvent()`

## Project-Specific Patterns

### Error Handling
- Custom errors defined in `pkg/errors/`
- Use `pkgerrors.ErrInvalidSubdomain`, `pkgerrors.ErrUnauthorized`, etc.
- Always wrap errors with context: `fmt.Errorf("failed to create tunnel: %w", err)`

### Tunnel Manager State
- Uses `sync.Map` for thread-safe concurrent access
- Two maps: `tunnels` (subdomain → *Tunnel) and `tunnelsByID` (tunnel_id → *Tunnel)
- Always lock with `m.mu.RLock()` / `m.mu.Lock()` for compound operations

### gRPC Stream Management
The `ProxyStream` implementation is the core of the tunneling system:
- Client sends registration message on stream start
- Server stores stream reference in tunnel manager
- Server writes `ProxyRequest` to stream when public request arrives
- Client reads from stream, forwards to local service, writes `ProxyResponse`
- Stream errors trigger automatic cleanup and reconnection

### Database Constraints
- `Tunnel.ClientID` must be unique (use `tunnel.ID.String()`)
- Domain reservations prevent subdomain conflicts
- Soft deletes not used; tunnels are hard deleted on disconnect

## Known Issues and Fixes

### Fixed Bugs (validated by tests)
1. **Domain Cleanup**: Domain reservations were not deleted on disconnect → Fixed in `UnregisterTunnel()`
2. **Unknown Data Display**: Dashboard showed "Unknown" for tunnel fields → Fixed by sending complete registration data
3. **ClientID Constraint**: UNIQUE constraint failed → Fixed by using `tunnel.ID.String()`

## CI/CD

GitHub Actions workflows in `.github/workflows/`:
- **test.yml**: Runs tests on push/PR, generates coverage
- **pr-checks.yml**: Linting, build verification
- **release.yml**: Semantic versioning with automated releases
