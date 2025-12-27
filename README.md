# Grok - Self-Hosted Tunneling Solution

**Expose your localhost to the internet with secure tunnels**

Grok is a self-hosted alternative to ngrok that lets you share your local development server with anyone on the internet.

## ✨ Features

### Core Features
- 🌐 **HTTP/HTTPS Tunnels** - Expose local web servers with custom subdomains
- 🔌 **TCP Tunnels** - Expose any TCP service (SSH, databases, etc.)
- 📁 **Static File Server** - Serve and share local directories instantly
- 🎯 **Custom Subdomains** - Use your own subdomain names
- 🔐 **Secure Authentication** - Token-based access control
- 📊 **Real-time Dashboard** - Monitor tunnel traffic live
- 🔄 **Auto-Reconnect** - Tunnels automatically recover from disconnections

### Client Dashboard
- 📈 **Live Request Monitoring** - See incoming requests in real-time
- 🔍 **Request Inspector** - View headers, body, and response details
- 📊 **Performance Metrics** - Track latency, throughput, and error rates
- 💻 **Works with all tunnel types** - HTTP, TCP, and file server

### Server Dashboard
- 👥 **User Management** - Create and manage users
- 🏢 **Organization Support** - Multi-tenant with role-based access
- 🔑 **Token Management** - Generate and revoke auth tokens
- 🪝 **Webhook Broadcasting** - Send webhooks to multiple tunnels
- 🔐 **Two-Factor Auth (2FA)** - TOTP-based security
- 📊 **Analytics** - Track all tunnel activity and statistics

## 🚀 Quick Start

### Installation

1. Download the latest release from [GitHub Releases](https://github.com/pandeptwidyaop/grok/releases)
2. Or build from source:
   ```bash
   git clone https://github.com/pandeptwidyaop/grok.git
   cd grok
   make build-all
   ```

### Server Setup

1. **Start the server**
   ```bash
   ./bin/grok-server --config configs/server.example.yaml
   ```

2. **Open the dashboard**: `http://localhost:4040`

3. **Create a user and get auth token** from the dashboard

### Client Setup

1. **Configure your token**
   ```bash
   ./bin/grok config set-token grok_your_token_here
   ```

2. **You're ready!**

## 📖 Usage

### HTTP Tunnels

Expose a local web server:

```bash
# Basic tunnel (auto-generated subdomain)
grok http 3000

# Custom subdomain
grok http 3000 --subdomain myapp
# Access at: https://myapp.yourdomain.com

# With client dashboard for monitoring
grok http 3000 --dashboard
# Dashboard at: http://localhost:4041
```

**When to use:**
- Share your local dev server with teammates
- Test webhooks from GitHub, Stripe, etc.
- Demo your app before deployment
- Access localhost from mobile devices

### TCP Tunnels

Expose any TCP service:

```bash
# SSH access
grok tcp 22

# PostgreSQL database
grok tcp 5432

# MySQL database
grok tcp 3306

# With client dashboard
grok tcp 22 --dashboard
```

**When to use:**
- Remote SSH access to your machine
- Share database access with your team
- Expose game servers
- Any TCP-based service

### Static File Server

Share files and directories:

```bash
# Serve current directory
grok serve .

# Serve specific directory
grok serve ./dist

# With custom subdomain
grok serve ./website --name mysite

# With authentication
grok serve ./private --auth username:password

# With client dashboard
grok serve ./dist --dashboard

# Custom 404 page
grok serve . --404 custom404.html
```

**When to use:**
- Share static websites instantly
- Distribute files to team/clients
- Test static site deployments
- Quick file sharing without cloud storage

### Client Dashboard

Monitor your tunnel traffic in real-time:

```bash
# Any command with --dashboard flag
grok http 3000 --dashboard
grok tcp 22 --dashboard
grok serve . --dashboard
```

**Dashboard shows:**
- ✅ Real-time request logs (method, path, status, duration)
- ✅ Connection status and tunnel info
- ✅ Performance charts (request rate, latency, throughput)
- ✅ Request inspector (full headers and body)

**Access:** `http://localhost:4041` (or custom port with `--dashboard-port`)

### Configuration Commands

```bash
# Set authentication token
grok config set-token grok_abc123...

# Set server address
grok config set-server tunnel.mycompany.com:4443

# View current configuration
grok config show

# Check version
grok version

# Update to latest version
grok update
```

## 🎯 Common Use Cases

### 1. Local Web Development

```bash
# Frontend developer
grok http 5173 --subdomain myproject --dashboard

# Backend API
grok http 8000 --subdomain api --dashboard
```

Share your local dev server with:
- Frontend/backend running separately
- Designers for UI review
- QA team for testing
- Client for demos

### 2. Webhook Testing

```bash
# Start your local webhook handler
grok http 3000 --dashboard

# Use the generated URL in webhook settings:
# GitHub webhooks
# Stripe payment notifications
# Twilio SMS callbacks
# Any webhook service
```

Real-time dashboard shows all incoming webhooks instantly.

### 3. Mobile App Development

```bash
# Your API server
grok http 8000 --subdomain myapi

# Test from physical devices
# No need to deploy or configure ngrok
```

### 4. Database Access

```bash
# Expose PostgreSQL
grok tcp 5432 --dashboard

# Team connects with:
# psql -h yourdomain.com -p assigned_port -U user
```

Monitor all database connections in real-time.

### 5. Quick File Sharing

```bash
# Share build artifacts
grok serve ./dist --name builds --dashboard

# Share with password
grok serve ./confidential --auth team:secret123
```

## 🎨 Dashboard Features

### Server Dashboard (`http://localhost:4040`)

**For Administrators:**
- Create and manage users
- Generate auth tokens
- View all active tunnels
- Monitor system stats
- Manage organizations
- Configure webhooks
- Enable 2FA security

### Client Dashboard (`http://localhost:4041`)

**For Developers:**
- See requests as they happen (SSE real-time)
- Inspect request/response details
- Track performance metrics
- Filter by method, status, path
- Export request logs
- Monitor connection health
- View throughput graphs

**Example:** When testing webhooks, you can see the exact payload and headers instantly without checking logs.

## 🔒 Security

- 🔐 **Token-based authentication** - Secure access control
- 🔑 **Two-factor authentication** - TOTP support
- 🏢 **Organization isolation** - Multi-tenant security
- 🔒 **TLS encryption** - All tunnel traffic encrypted
- 👥 **Role-based access** - Admin, User, Super Admin roles

## ⚙️ Advanced Options

### HTTP Tunnels
```bash
grok http 3000 --subdomain myapp          # Custom subdomain
grok http 3000 --name persistent-tunnel   # Named tunnel
grok http 3000 --server custom.com:4443   # Custom server
grok http 3000 --dashboard-port 8080      # Custom dashboard port
```

### TCP Tunnels
```bash
grok tcp 22 --name ssh-server
grok tcp 5432 --dashboard
```

### File Server
```bash
grok serve ./dist --name mysite           # Named tunnel
grok serve . --auth user:pass             # Password protect
grok serve . --no-gzip                    # Disable compression
grok serve . --404 custom404.html         # Custom 404
grok serve . --dashboard                  # With monitoring
```

### Dashboard Options
```bash
--dashboard              # Enable client dashboard
--dashboard-port 8080    # Custom dashboard port
--no-dashboard           # Explicitly disable
```

## 📊 System Requirements

**Server:**
- Linux, macOS, or Windows
- 512MB RAM minimum
- PostgreSQL or SQLite

**Client:**
- Linux, macOS, or Windows
- Any machine that can run Go binaries

## 🆘 Troubleshooting

**Tunnel won't connect:**
```bash
# Check server is running
curl http://yourserver:4040/health

# Verify token
grok config show

# Test with verbose logging
grok http 3000 --debug
```

**Dashboard not showing:**
```bash
# Ensure --dashboard flag is used
grok http 3000 --dashboard

# Check if port 4041 is available
lsof -i :4041

# Try custom port
grok http 3000 --dashboard --dashboard-port 8080
```

**Can't access tunnel URL:**
- Check DNS wildcard record: `*.yourdomain.com → server-ip`
- Verify firewall allows ports 3080, 4040, 4443
- Ensure tunnel is showing as "connected" in dashboard

## 📝 License

MIT License - see [LICENSE](LICENSE) file

## 👨‍💻 Author

**Pandept Widya**
- GitHub: [@pandeptwidyaop](https://github.com/pandeptwidyaop)

## 🙏 Credits

Inspired by [ngrok](https://ngrok.com) - Built with Go, gRPC, and React

---

⭐ **Star this repo if you find it useful!**
