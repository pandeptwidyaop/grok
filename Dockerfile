# Multi-stage build for Grok server
# Stage 1: Build web dashboard
FROM node:20-alpine AS web-builder

WORKDIR /build

# Copy web package files
COPY web/package*.json ./web/
RUN cd web && npm ci

# Copy web source
COPY web/ ./web/

# Build web dashboard (outputs to internal/server/web/dist/)
RUN cd web && npm run build

# Stage 2: Build Go application
FROM golang:alpine AS go-builder

# Set Go toolchain to auto-download required version
ENV GOTOOLCHAIN=auto

# Install build dependencies (pure Go build - no CGO required)
RUN apk add --no-cache git make protobuf-dev bash

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Copy built web dashboard from web-builder stage
COPY --from=web-builder /build/internal/server/web/dist/ ./internal/server/web/dist/

# Install protoc plugins
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Generate proto files (use bash explicitly)
RUN bash scripts/generate-proto.sh

# Build arguments for version info
ARG VERSION=dev
ARG BUILD_TIME
ARG GIT_COMMIT

# Build the server binary (pure Go - no CGO required)
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=${VERSION} -X main.buildTime=${BUILD_TIME} -X main.gitCommit=${GIT_COMMIT}" \
    -o grok-server \
    ./cmd/grok-server

# Stage 3: Create minimal runtime image
FROM alpine:latest

# Install runtime dependencies (pure Go - no SQLite libraries needed)
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 grok && \
    adduser -D -u 1000 -G grok grok

# Create necessary directories
RUN mkdir -p /var/lib/grok/certs /var/log/grok /etc/grok && \
    chown -R grok:grok /var/lib/grok /var/log/grok /etc/grok

# Set working directory
WORKDIR /app

# Copy binary from go-builder
COPY --from=go-builder /build/grok-server /usr/local/bin/grok-server

# Copy default configuration
COPY --from=go-builder /build/configs/server.example.yaml /etc/grok/server.yaml

# Switch to non-root user
USER grok

# Expose ports
EXPOSE 80 443 4040 4080

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:4080/health || exit 1

# Set environment variables
ENV GROK_CONFIG=/etc/grok/server.yaml

# Run the application
ENTRYPOINT ["/usr/local/bin/grok-server"]
CMD ["--config", "/etc/grok/server.yaml"]
