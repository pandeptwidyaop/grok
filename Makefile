 sqlite .PHONY: help proto build-server build-client build-dashboard build-client-dashboard build-all dev-server dev-client dev-client-dashboard test clean migrate-up migrate-down

help:
	@echo "Grok - ngrok Clone"
	@echo ""
	@echo "Available targets:"
	@echo "  proto                  - Generate gRPC code from proto files"
	@echo "  build-all              - Build server, client, and dashboards"
	@echo "  build-server           - Build server binary"
	@echo "  build-client           - Build client binary (with embedded dashboard)"
	@echo "  build-dashboard        - Build React server dashboard"
	@echo "  build-client-dashboard - Build React client dashboard"
	@echo "  dev-server             - Run server in development mode"
	@echo "  dev-client             - Run client in development mode"
	@echo "  dev-client-dashboard   - Run client dashboard dev server (Vite)"
	@echo "  test                   - Run tests"
	@echo "  migrate-up             - Run database migrations"
	@echo "  migrate-down           - Rollback database migrations"
	@echo "  clean                  - Clean build artifacts"

proto:
	@echo "Generating gRPC code..."
	@./scripts/generate-proto.sh

build-dashboard:
	@echo "Building server dashboard..."
	@cd web && npm install && npm run build

build-client-dashboard:
	@echo "Building client dashboard..."
	@cd client-dashboard && npm install && npm run build

build-server: proto
	@echo "Building server..."
	@TAG_VERSION=$$(git describe --tags --abbrev=0 2>/dev/null || echo "dev"); \
	if ! git diff --quiet 2>/dev/null || ! git diff --cached --quiet 2>/dev/null; then \
		VERSION="$$TAG_VERSION-dirty"; \
	else \
		VERSION="$$TAG_VERSION"; \
	fi; \
	GIT_COMMIT=$$(git rev-parse --short HEAD 2>/dev/null || echo "unknown"); \
	if ! git diff --quiet 2>/dev/null || ! git diff --cached --quiet 2>/dev/null; then \
		GIT_COMMIT="$$GIT_COMMIT-dirty"; \
	fi; \
	BUILD_DATE=$$(date -u +%Y-%m-%dT%H:%M:%SZ); \
	go build -ldflags="-s -w \
		-X main.version=$$VERSION \
		-X main.gitCommit=$$GIT_COMMIT \
		-X main.buildTime=$$BUILD_DATE" \
		-o bin/grok-server ./cmd/grok-server

build-client: proto build-client-dashboard
	@echo "Building client..."
	@TAG_VERSION=$$(git describe --tags --abbrev=0 2>/dev/null || echo "dev"); \
	if ! git diff --quiet 2>/dev/null || ! git diff --cached --quiet 2>/dev/null; then \
		VERSION="$$TAG_VERSION-dirty"; \
	else \
		VERSION="$$TAG_VERSION"; \
	fi; \
	GIT_COMMIT=$$(git rev-parse --short HEAD 2>/dev/null || echo "unknown"); \
	if ! git diff --quiet 2>/dev/null || ! git diff --cached --quiet 2>/dev/null; then \
		GIT_COMMIT="$$GIT_COMMIT-dirty"; \
	fi; \
	BUILD_DATE=$$(date -u +%Y-%m-%dT%H:%M:%SZ); \
	go build -ldflags="-s -w \
		-X main.version=$$VERSION \
		-X main.gitCommit=$$GIT_COMMIT \
		-X main.buildTime=$$BUILD_DATE" \
		-o bin/grok ./cmd/grok

build-all: build-dashboard build-client-dashboard build-server build-client
	@echo "Build complete!"

dev-server:
	@echo "Running server in dev mode..."
	@go run ./cmd/grok-server/main.go --config configs/server.example.yaml

dev-client:
	@echo "Running client in dev mode..."
	@go run ./cmd/grok/main.go http localhost:3000

dev-client-dashboard:
	@echo "Running client dashboard dev server..."
	@echo "Dashboard will be available at http://localhost:5174"
	@echo "Make sure the client is running with: ./bin/grok http 3000"
	@cd client-dashboard && npm run dev

test:
	@echo "Running tests..."
	@go test -v -race -cover ./...

migrate-up:
	@echo "Running migrations..."
	@psql -U grok -d grok -f internal/db/migrations/001_init.sql

migrate-down:
	@echo "Rolling back migrations..."
	@echo "Manual rollback required - drop tables in reverse order"

clean:
	@echo "Cleaning build artifacts..."
	@rm -rf bin/
	@rm -rf web/dist/
	@rm -rf internal/server/web/dist/
	@rm -rf internal/client/dashboard/web/dist/
	@rm -rf client-dashboard/node_modules/.vite
	@rm -rf gen/
	@echo "Clean complete!"

install-tools:
	@echo "Installing development tools..."
	@go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	@go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@echo "Tools installed!"
