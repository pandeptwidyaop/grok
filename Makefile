 sqlite .PHONY: help proto build-server build-client build-dashboard build-all dev-server dev-client test clean migrate-up migrate-down

help:
	@echo "Grok - ngrok Clone"
	@echo ""
	@echo "Available targets:"
	@echo "  proto           - Generate gRPC code from proto files"
	@echo "  build-all       - Build server, client, and dashboard"
	@echo "  build-server    - Build server binary"
	@echo "  build-client    - Build client binary"
	@echo "  build-dashboard - Build React dashboard"
	@echo "  dev-server      - Run server in development mode"
	@echo "  dev-client      - Run client in development mode"
	@echo "  test            - Run tests"
	@echo "  migrate-up      - Run database migrations"
	@echo "  migrate-down    - Rollback database migrations"
	@echo "  clean           - Clean build artifacts"

proto:
	@echo "Generating gRPC code..."
	@./scripts/generate-proto.sh

build-dashboard:
	@echo "Building dashboard..."
	@cd web && npm install && npm run build

build-server: proto
	@echo "Building server..."
	@go build -o bin/grok-server ./cmd/grok-server

build-client: proto
	@echo "Building client..."
	@go build -o bin/grok ./cmd/grok

build-all: build-dashboard build-server build-client
	@echo "Build complete!"

dev-server:
	@echo "Running server in dev mode..."
	@go run ./cmd/grok-server/main.go --config configs/server.example.yaml

dev-client:
	@echo "Running client in dev mode..."
	@go run ./cmd/grok/main.go http localhost:3000

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
	@rm -rf gen/
	@echo "Clean complete!"

install-tools:
	@echo "Installing development tools..."
	@go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	@go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@echo "Tools installed!"
