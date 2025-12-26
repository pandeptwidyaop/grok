# Grok - Self-Hosted Tunneling Solution

<div align="center">

![Grok Banner](https://img.shields.io/badge/Grok-Tunneling%20Solution-667eea?style=for-the-badge)

**Production-ready ngrok alternative built with Go, gRPC, and React**

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat-square&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg?style=flat-square)](LICENSE)
[![GitHub Issues](https://img.shields.io/github/issues/pandeptwidyaop/grok?style=flat-square)](https://github.com/pandeptwidyaop/grok/issues)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg?style=flat-square)](CONTRIBUTING.md)

[Features](#-features) • [Quick Start](#-quick-start) • [Documentation](#-documentation) • [Contributing](#-contributing)

</div>

---

## 📖 Overview

**Grok** is a self-hosted tunneling solution that securely exposes your local services to the internet through encrypted tunnels. Perfect for:

- 🚀 **Local Development** - Share your localhost with teammates
- 🔗 **Webhook Testing** - Test webhooks from external services
- 🔒 **Secure Access** - Access services behind firewalls/NAT
- 🏢 **Multi-Tenancy** - Organization-based access control
- 📊 **Real-time Monitoring** - Beautiful web dashboard

## ✨ Features

### Core Features
- ⚡ **gRPC Bidirectional Streaming** - High-performance tunnel communication
- 🌐 **Multi-Protocol Support** - HTTP/HTTPS, TCP with TLS termination
- 🎯 **Custom & Random Subdomains** - Flexible domain allocation
- 🔐 **JWT Authentication** - Secure token-based access control
- 🗄️ **Database Support** - SQLite (development) & PostgreSQL (production)
- 🔄 **Auto-Reconnection** - Resilient connections with exponential backoff
- 📈 **Real-time Updates** - Server-Sent Events (SSE) for live dashboard

### Advanced Features
- 🏢 **Organization Management** - Multi-tenant architecture with role-based access
- 👥 **User Roles** - Super Admin, Org Admin, Org User permissions
- 🔑 **Two-Factor Authentication (2FA)** - TOTP-based security via Google Authenticator
- 🪝 **Webhook Routing** - Broadcast webhooks to multiple tunnels simultaneously
- 📊 **Analytics Dashboard** - Track requests, bandwidth, tunnel statistics
- 🔔 **Version Checker** - Auto-detect updates from GitHub releases
- 🎨 **Modern UI** - React + Material-UI with professional design

### Developer Experience
- 🛠️ **Hot Reload** - Fast development with Air (backend) and Vite (frontend)
- 📦 **Semantic Versioning** - Automated releases with GitHub Actions
- 🧪 **Comprehensive Testing** - Unit, integration, and E2E tests
- 📝 **Type-Safe** - Protocol Buffers for API contracts
- 🐳 **Docker Ready** - Easy deployment with containers

## 🏗️ Architecture

```
┌──────────────┐     ┌─────────────────┐     ┌──────────────┐     ┌──────────────┐
│ Internet     │────▶│ Server          │────▶│ Tunnel       │────▶│ Client       │
│ Client       │     │ (HTTP Proxy +   │     │ Manager      │     │ (gRPC)       │
│              │◀────│  gRPC Server)   │◀────│ (sync.Map)   │◀────│              │
└──────────────┘     └─────────────────┘     └──────────────┘     └──────────────┘
                              │                                            │
                              │                                            │
                              ▼                                            ▼
                     ┌─────────────────┐                          ┌──────────────┐
                     │ PostgreSQL/     │                          │ Local        │
                     │ SQLite DB       │                          │ Service      │
                     └─────────────────┘                          │ (localhost)  │
                                                                   └──────────────┘
```

## 📁 Project Structure

```
grok/
├── cmd/
│   ├── grok-server/         # Server entry point
│   └── grok/                # Client CLI
├── internal/
│   ├── server/              # Server implementation
│   │   ├── auth/           # JWT + TOTP authentication
│   │   ├── grpc/           # gRPC tunnel service
│   │   ├── proxy/          # HTTP/TCP reverse proxy
│   │   ├── tunnel/         # Tunnel manager
│   │   ├── web/            # REST API + embedded dashboard
│   │   └── tcp/            # TCP port allocation
│   ├── client/              # Client implementation
│   │   ├── cli/            # Cobra CLI commands
│   │   ├── tunnel/         # gRPC client
│   │   └── proxy/          # Local forwarding
│   ├── db/                  # Database models & migrations
│   └── version/             # Version management
├── proto/                   # Protocol Buffers definitions
├── web/                     # React dashboard
│   ├── src/
│   │   ├── components/     # React components
│   │   ├── contexts/       # Auth & app contexts
│   │   └── lib/            # API client
├── configs/                 # Configuration examples
├── scripts/                 # Build & deployment scripts
└── tests/                   # Integration tests
```

## 🚀 Quick Start

### Prerequisites

- **Go 1.25+** - [Install Go](https://go.dev/dl/)
- **Node.js 18+** - [Install Node](https://nodejs.org/) (for building web dashboard)
- **Protocol Buffers** - `brew install protobuf` or [Install protoc](https://grpc.io/docs/protoc-installation/)
- **Database**: PostgreSQL 15+ or SQLite 3+ (SQLite included)

### Installation

1. **Clone the repository**
   ```bash
   git clone https://github.com/pandeptwidyaop/grok.git
   cd grok
   ```

2. **Install dependencies**
   ```bash
   go mod download
   make install-tools  # Install protoc-gen-go and protoc-gen-go-grpc
   ```

3. **Build everything**
   ```bash
   make build-all  # Builds server, client, and web dashboard
   ```

### Quick Test (5 Minutes)

1. **Start the server** (Terminal 1)
   ```bash
   ./bin/grok-server --config configs/server.example.yaml
   ```
   Server starts on:
   - gRPC: `localhost:4443`
   - HTTP Proxy: `localhost:3080`
   - Dashboard: `http://localhost:4040`

2. **Create a demo user** (Terminal 2)
   ```bash
   # Open the database
   sqlite3 grok.db

   # Run this SQL to create a super admin
   INSERT INTO users (id, email, password, name, role, is_active, created_at, updated_at)
   VALUES (
     lower(hex(randomblob(16))),
     'admin@grok.local',
     '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', -- password: "admin"
     'Admin User',
     'super_admin',
     1,
     datetime('now'),
     datetime('now')
   );
   ```

3. **Open the dashboard**
   - Navigate to `http://localhost:4040`
   - Login with: `admin@grok.local` / `admin`
   - Go to **Auth Tokens** → **Create Token**
   - Copy the generated token (starts with `grok_`)

4. **Configure the client** (Terminal 2)
   ```bash
   ./bin/grok config set-token grok_your_copied_token
   ```

5. **Start a test HTTP server** (Terminal 3)
   ```bash
   python3 -m http.server 8000
   ```

6. **Create a tunnel** (Terminal 2)
   ```bash
   ./bin/grok http 8000
   ```

7. **Test it!** (Terminal 4)
   ```bash
   # Replace abc123 with your actual subdomain
   curl http://abc123.localhost:3080
   ```

🎉 **Success!** You should see your local server's response.

## 📚 Documentation

### Server Configuration

Create `configs/server.yaml` from the example:

```yaml
server:
  grpc_port: 4443
  http_port: 3080
  api_port: 4040
  domain: "localhost"  # Change to your domain in production

database:
  driver: "sqlite"     # or "postgres"
  database: "grok.db"  # or PostgreSQL connection string

auth:
  jwt_secret: "your-secret-key-min-32-chars"

tls:
  auto_cert: false     # Enable for production with Let's Encrypt
  cert_dir: "/var/lib/grok/certs"

logging:
  level: "info"
  format: "json"
```

### Client Usage

#### HTTP Tunnels

```bash
# Basic tunnel with auto-generated subdomain
./bin/grok http 3000

# Custom subdomain
./bin/grok http 3000 --subdomain myapp
# → https://myapp.yourdomain.com

# Named tunnel (persistent identifier)
./bin/grok http 3000 --name my-api --subdomain api

# Override server
./bin/grok http 3000 --server tunnel.company.com:4443
```

#### TCP Tunnels

```bash
# SSH tunnel
./bin/grok tcp 22

# MySQL tunnel
./bin/grok tcp 3306

# PostgreSQL tunnel
./bin/grok tcp 5432
```

#### Webhook Broadcasting

```bash
# Create webhook app in dashboard first, then:
./bin/grok webhook {webhook_app_id} localhost:3000
```

### Configuration Commands

```bash
# Set auth token
./bin/grok config set-token grok_your_token

# Set server address
./bin/grok config set-server tunnel.company.com:4443

# View current config
./bin/grok config show

# Check version
./bin/grok version
```

## 🎯 Use Cases

### 1. Local Development Sharing

```bash
# Frontend dev server
./bin/grok http 5173 --subdomain my-project

# Share with team: https://my-project.yourdomain.com
```

### 2. Webhook Testing

```bash
# Test Stripe, GitHub, or any webhook locally
./bin/grok http 8000

# Use generated URL in webhook settings
```

### 3. Database Access

```bash
# Expose PostgreSQL for remote team
./bin/grok tcp 5432

# Team connects: psql -h yourdomain.com -p {assigned_port}
```

### 4. Multi-Service Development

```bash
# Terminal 1: Frontend
./bin/grok http 3000 --subdomain frontend

# Terminal 2: Backend API
./bin/grok http 8000 --subdomain api

# Terminal 3: Database
./bin/grok tcp 5432
```

### 5. Organization Collaboration

1. Super Admin creates organization: "ACME Corp"
2. Adds team members as Org Admin or Org User
3. Team members create tunnels under organization subdomain
4. All tunnels visible in organization dashboard

## 🔐 Security Features

### Two-Factor Authentication (2FA)

1. Navigate to **Settings → Security** in dashboard
2. Click **Enable 2FA**
3. Scan QR code with Google Authenticator
4. Enter 6-digit code to verify
5. Login now requires both password and OTP code

Format in authenticator: `Grok {domain}: {username}`

### Role-Based Access Control (RBAC)

- **Super Admin**: Full system access, manage all organizations
- **Org Admin**: Manage organization users, view all org tunnels
- **Org User**: Create personal tunnels, view own tunnels

### Token Security

- Tokens hashed with SHA256 before storage
- JWT-based session management
- Token scopes for granular permissions
- Token expiration and revocation

## 🧪 Development

### Setup Development Environment

```bash
# Install air for hot reload
go install github.com/cosmtrek/air@latest

# Terminal 1: Backend (with hot reload)
air

# Terminal 2: Frontend (with HMR)
cd web && npm run dev
```

### Build Commands

```bash
make help              # Show all commands
make proto             # Generate gRPC code
make build-all         # Build server + client + dashboard
make build-server      # Build server only
make build-client      # Build client only
make test              # Run tests
make clean             # Clean build artifacts
```

### Database Migrations

```bash
# PostgreSQL
make migrate-up        # Apply migrations
make migrate-down      # Rollback

# SQLite (auto-migrates on startup)
./bin/grok-server      # Creates grok.db automatically
```

### Testing

```bash
# Unit tests
go test ./...

# Integration tests
go test -v ./tests/integration/

# With coverage
go test -cover ./...
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## 📦 Deployment

### Docker (Recommended)

```dockerfile
# Coming soon
docker-compose up -d
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

### Production Checklist

- ✅ Use PostgreSQL for production
- ✅ Enable TLS with Let's Encrypt (`auto_cert: true`)
- ✅ Set strong JWT secret (min 32 characters)
- ✅ Configure proper domain DNS (wildcard A record)
- ✅ Set up firewall rules
- ✅ Enable 2FA for admin accounts
- ✅ Configure rate limiting
- ✅ Set up monitoring and logging
- ✅ Regular database backups

## 🛠️ Tech Stack

| Layer | Technology |
|-------|-----------|
| **Language** | Go 1.25+ |
| **RPC** | gRPC, Protocol Buffers |
| **Database** | PostgreSQL, SQLite, GORM |
| **Authentication** | JWT (golang-jwt/jwt), TOTP (pquerna/otp) |
| **CLI** | Cobra, Viper |
| **Logging** | zerolog |
| **Frontend** | React 18, TypeScript, Vite |
| **UI Library** | Material-UI (MUI) |
| **State Management** | TanStack Query, React Context |
| **Real-time** | Server-Sent Events (SSE) |
| **Build** | Make, GitHub Actions |
| **Deployment** | Systemd, Docker (coming soon) |

## 🗺️ Roadmap

### ✅ Completed
- [x] Core tunneling (HTTP/TCP)
- [x] Web dashboard with real-time updates
- [x] Organization management
- [x] Two-factor authentication
- [x] Webhook broadcasting
- [x] Version checker
- [x] Custom subdomains
- [x] Token management

### 🚧 In Progress
- [ ] Docker & Kubernetes deployment
- [ ] Metrics & monitoring (Prometheus)
- [ ] Rate limiting & DDoS protection
- [ ] Custom SSL certificates per tunnel

### 📅 Planned
- [ ] WebSocket support
- [ ] Traffic replay & inspection
- [ ] Tunnel templates
- [ ] API documentation (OpenAPI)
- [ ] Mobile client apps
- [ ] Terraform provider

## 🤝 Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for details.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 👨‍💻 Author

**Pandept Widya**

- GitHub: [@pandeptwidyaop](https://github.com/pandeptwidyaop)
- Website: [pandeptwidya.com](https://pandeptwidya.com)

## 🙏 Acknowledgments

- Inspired by [ngrok](https://ngrok.com)
- Built with amazing open-source projects
- Thanks to all contributors!

## 📞 Support

- 📫 **Issues**: [GitHub Issues](https://github.com/pandeptwidyaop/grok/issues)
- 💬 **Discussions**: [GitHub Discussions](https://github.com/pandeptwidyaop/grok/discussions)
- 📖 **Documentation**: [Wiki](https://github.com/pandeptwidyaop/grok/wiki)

---

<div align="center">

**Made with ❤️ by [Pandept Widya](https://github.com/pandeptwidyaop)**

⭐ Star us on GitHub — it motivates us a lot!

</div>
