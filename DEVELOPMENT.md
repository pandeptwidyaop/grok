# Development Guide

This guide explains how to run Grok in development mode with hot reload for both backend and frontend.

## Prerequisites

- Go 1.24+
- Node.js 20+
- Air (Go hot reload tool)

### Install Air (if not installed)

```bash
go install github.com/air-verse/air@latest
```

## Development Workflow

### Option 1: Separate Terminal Windows (Recommended)

**Terminal 1 - Backend (Go server with hot reload):**
```bash
# Start server with hot reload
air

# Or with custom config
air -c .air.toml
```

The server will:
- Run on `http://localhost:4040`
- Auto-reload on Go file changes
- Serve API endpoints at `/api/*`
- Serve production dashboard at `/` (if built)

**Terminal 2 - Frontend (React dev server with HMR):**
```bash
cd web
npm run dev
```

The dev server will:
- Run on `http://localhost:3000`
- Proxy `/api/*` requests to backend at `:4040`
- Hot Module Replacement (instant updates)
- No need to rebuild on frontend changes

**Access Points:**
- Frontend Dev: `http://localhost:3000` (use this for development!)
- Backend API: `http://localhost:4040/api`
- Production Build: `http://localhost:4040`

### Option 2: Build Once, Use Air Only

If you don't need to change frontend:

```bash
# Build dashboard once
cd web && npm run build && cd ..

# Run server with air
air
```

Access at `http://localhost:4040`

## Development Features

### Backend Hot Reload (Air)

Air watches for changes in:
- `**/*.go` files
- `**/*.yaml` config files
- `**/*.html` templates

Excluded from watch:
- `*_test.go` files
- `web/` directory
- `tmp/` directory
- `vendor/` directory

Configuration: `.air.toml`

### Frontend Hot Module Replacement (Vite)

Vite automatically reloads on changes in:
- `src/**/*.tsx` React components
- `src/**/*.ts` TypeScript files
- `src/**/*.css` Stylesheets
- `public/*` Static assets

Configuration: `web/vite.config.ts`

### API Proxy Configuration

When running `npm run dev`, all `/api/*` requests are automatically proxied to the backend:

```typescript
// web/vite.config.ts
server: {
  port: 3000,
  proxy: {
    '/api': {
      target: 'http://localhost:4040',
      changeOrigin: true,
    },
  },
}
```

This means you can develop frontend and backend independently without CORS issues.

## Common Development Tasks

### 1. Login to Dashboard (Development)

```bash
# Access: http://localhost:3000/login
# Default credentials:
# Username: admin
# Password: admin123
```

### 2. Create Tunnel Client

```bash
# Build client
go build -o bin/grok ./cmd/grok

# Run tunnel
./bin/grok http 3000 --subdomain myapp
```

### 3. Run Tests

```bash
# All tests
go test ./...

# Unit tests only
go test ./internal/server/auth ./internal/server/tunnel

# Integration tests
go test ./tests/integration

# With coverage
go test -cover ./...
```

### 4. Format Code

```bash
# Go
go fmt ./...
gofmt -s -w .

# Frontend
cd web
npm run lint
```

### 5. Build for Production

```bash
# Build dashboard
cd web && npm run build && cd ..

# Build server
go build -o bin/grok-server ./cmd/grok-server

# Build client
go build -o bin/grok ./cmd/grok
```

## Debugging

### Backend Debugging

Add debug logs in your code:

```go
import "github.com/pandeptwidyaop/grok/pkg/logger"

logger.DebugEvent().
    Str("key", "value").
    Msg("Debug message")
```

Logs appear in terminal running `air`.

### Frontend Debugging

Use browser DevTools:

```typescript
console.log('Debug:', data);
```

Or use React DevTools extension.

### API Debugging

Use `curl` or tools like Postman:

```bash
# Login
TOKEN=$(curl -s -X POST http://localhost:4040/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | jq -r '.token')

# List tunnels
curl -s http://localhost:4040/api/tunnels \
  -H "Authorization: Bearer $TOKEN" | jq
```

## Troubleshooting

### Port Already in Use

```bash
# Find process using port 4040
lsof -i :4040

# Kill process
kill -9 <PID>
```

### Frontend Not Connecting to Backend

1. Check backend is running: `curl http://localhost:4040/api/stats`
2. Check proxy config in `web/vite.config.ts`
3. Clear browser cache and reload

### Hot Reload Not Working

**Backend (Air):**
1. Check `.air.toml` configuration
2. Ensure files are not in excluded directories
3. Try restarting air

**Frontend (Vite):**
1. Check for syntax errors in console
2. Try `rm -rf node_modules && npm install`
3. Restart dev server

### Login Redirects After Refresh

This should be fixed now with:
- Loading state in AuthContext
- Lazy localStorage initialization
- ProtectedRoute loading screen

If still happening:
1. Check browser console for errors
2. Verify localStorage is enabled
3. Check axios interceptor isn't clearing token

## Environment Variables

### Backend (optional)

Create `.env` file in project root:

```env
# Database
DB_HOST=localhost
DB_PORT=5432
DB_USER=grok
DB_PASSWORD=secret
DB_NAME=grok

# Server
SERVER_PORT=4040
GRPC_PORT=4443
BASE_DOMAIN=localhost
```

### Frontend (optional)

Create `web/.env.local`:

```env
VITE_API_URL=/api
```

## Git Workflow

```bash
# Create feature branch
git checkout -b feature/my-feature

# Make changes...
# Run tests
go test ./...

# Commit
git add .
git commit -m "feat: description"

# Push
git push origin feature/my-feature
```

## Performance Tips

1. **Use Air for Backend**: Faster than manual restarts
2. **Keep npm run dev Running**: HMR is instant
3. **Use Test Driven Development**: Run tests frequently
4. **Profile Before Optimizing**: Use `go test -bench`
5. **Monitor Memory**: Use `pprof` for profiling

## Editor Setup

### VS Code

Recommended extensions:
- Go (golang.go)
- ESLint (dbaeumer.vscode-eslint)
- Prettier (esbenp.prettier-vscode)
- Tailwind CSS IntelliSense (bradlc.vscode-tailwindcss)

Settings (`.vscode/settings.json`):

```json
{
  "go.useLanguageServer": true,
  "go.lintTool": "golangci-lint",
  "editor.formatOnSave": true,
  "[go]": {
    "editor.defaultFormatter": "golang.go"
  },
  "[typescript]": {
    "editor.defaultFormatter": "esbenp.prettier-vscode"
  },
  "[typescriptreact]": {
    "editor.defaultFormatter": "esbenp.prettier-vscode"
  }
}
```

## Next Steps

- Read [TESTING_SUMMARY.md](./TESTING_SUMMARY.md) for testing guide
- Check [tests/README.md](./tests/README.md) for test documentation
- See [README.md](./README.md) for production deployment
