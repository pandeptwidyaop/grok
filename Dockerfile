# Multi-stage build for Grok server
# Stage 1: Build the application
FROM golang:alpine AS builder

# Set Go toolchain to auto-download required version
ENV GOTOOLCHAIN=auto

# Install build dependencies (including CGO dependencies for SQLite)
RUN apk add --no-cache git make protobuf-dev gcc musl-dev sqlite-dev

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Install protoc plugins
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Generate proto files
RUN make proto

# Build arguments for version info
ARG VERSION=dev
ARG BUILD_TIME
ARG GIT_COMMIT

# Build the server binary (CGO enabled for SQLite support)
RUN CGO_ENABLED=1 GOOS=linux go build \
    -ldflags="-s -w -X main.version=${VERSION} -X main.buildTime=${BUILD_TIME} -X main.gitCommit=${GIT_COMMIT}" \
    -o grok-server \
    ./cmd/grok-server

# Stage 2: Create minimal runtime image
FROM alpine:latest

# Install runtime dependencies (including SQLite libraries)
RUN apk add --no-cache ca-certificates tzdata sqlite-libs

# Create non-root user
RUN addgroup -g 1000 grok && \
    adduser -D -u 1000 -G grok grok

# Create necessary directories
RUN mkdir -p /var/lib/grok/certs /var/log/grok /etc/grok && \
    chown -R grok:grok /var/lib/grok /var/log/grok /etc/grok

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/grok-server /usr/local/bin/grok-server

# Copy default configuration
COPY --from=builder /build/configs/server.example.yaml /etc/grok/server.yaml

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
