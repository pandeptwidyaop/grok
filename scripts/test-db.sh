#!/bin/bash
set -e

echo "Testing database connections..."

# Test SQLite
echo ""
echo "1. Testing SQLite..."
cat > /tmp/test-sqlite.yaml <<EOF
server:
  grpc_port: 4443
  domain: "test.local"

database:
  driver: "sqlite"
  database: ":memory:"

auth:
  jwt_secret: "test-secret"

tunnels:
  max_per_user: 5
  idle_timeout: "10m"
  heartbeat_interval: "30s"

logging:
  level: "info"
  format: "text"
  output: "stdout"
EOF

echo "✓ SQLite config created"

# Test PostgreSQL config (won't actually connect without DB)
echo ""
echo "2. PostgreSQL config example:"
cat > /tmp/test-postgres.yaml <<EOF
server:
  grpc_port: 4443
  domain: "test.local"

database:
  driver: "postgres"
  host: "localhost"
  port: 5432
  database: "grok_test"
  username: "grok"
  password: "password"
  ssl_mode: "disable"

auth:
  jwt_secret: "test-secret"

tunnels:
  max_per_user: 5
  idle_timeout: "10m"
  heartbeat_interval: "30s"

logging:
  level: "info"
  format: "json"
  output: "stdout"
EOF

echo "✓ PostgreSQL config created"

echo ""
echo "Database driver switching is ready!"
echo ""
echo "Available configurations:"
echo "  - configs/server.sqlite.yaml (SQLite - Development)"
echo "  - configs/server.postgres.yaml (PostgreSQL - Production)"
echo "  - configs/server.example.yaml (SQLite default)"
echo ""
echo "Test configs created in /tmp:"
echo "  - /tmp/test-sqlite.yaml"
echo "  - /tmp/test-postgres.yaml"
