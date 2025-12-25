# Grok Testing Guide

This directory contains all test files for the Grok project. We use a combination of integration tests and unit tests to ensure code quality and prevent regressions.

## Test Structure

```
tests/
├── integration/           # End-to-end integration tests
│   └── tunnel_flow_test.go
└── README.md

internal/
├── server/
│   ├── auth/
│   │   └── token_service_test.go    # Auth service unit tests
│   └── tunnel/
│       └── manager_test.go          # Tunnel manager unit tests
```

## Running Tests

### Run All Tests
```bash
go test ./...
```

### Run Integration Tests Only
```bash
go test ./tests/integration/
```

### Run Unit Tests Only
```bash
go test ./internal/server/auth/
go test ./internal/server/tunnel/
```

### Run with Verbose Output
```bash
go test -v ./tests/integration/
```

### Run with Coverage
```bash
go test -cover ./...
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out  # View coverage in browser
```

## Test Coverage

### Integration Tests (`tests/integration/tunnel_flow_test.go`)

**TestCompleteTunnelFlow** - Complete tunnel lifecycle validation
- Creates tunnel via CreateTunnel RPC
- Establishes ProxyStream connection
- Sends tunnel registration message
- Verifies database persistence (subdomain, local addr, public URL, status)
- Verifies in-memory tunnel state
- Tests cleanup on disconnect
- Validates status update to "disconnected"

**TestSubdomainAllocation** - Subdomain validation and allocation
- Custom subdomain allocation
- Reserved subdomain rejection ("api", "admin", etc.)
- Duplicate subdomain rejection
- Random 8-character subdomain generation

**TestDomainCleanupOnDisconnect** - Critical bug fix validation
- Verifies domain reservation exists after registration
- Verifies domain is deleted after tunnel unregisters
- Confirms subdomain can be reused immediately after cleanup

### Unit Tests - Tunnel Manager (`internal/server/tunnel/manager_test.go`)

**TestAllocateSubdomain**
- Allocate custom subdomain
- Reject duplicate subdomain
- Reject reserved subdomain
- Generate random subdomain

**TestRegisterTunnel**
- Register tunnel successfully
- Retrieve tunnel by ID
- Unregister tunnel (with domain cleanup)

**TestGetUserTunnels**
- Get all tunnels for a user
- Verify user isolation

**TestCountActiveTunnels**
- Count active tunnels across all users

**TestBuildPublicURL**
- Build HTTP public URL
- Build HTTPS public URL
- Build TCP public URL

### Unit Tests - Auth Service (`internal/server/auth/token_service_test.go`)

**TestValidateToken**
- Valid token validation
- Invalid token rejection
- Empty token rejection

**TestValidateTokenWithInactiveUser**
- Reject tokens for inactive users

**TestValidateTokenWithInactiveToken**
- Reject inactive tokens

**TestValidateTokenWithExpiredToken**
- Reject expired tokens

**TestCreateToken**
- Create token with expiration
- Verify token can be validated

**TestRevokeToken**
- Revoke token
- Verify revoked token is invalid

**TestListTokens**
- List user tokens
- Verify user isolation

**TestUpdateTokenLastUsed**
- Update last used timestamp on validation

## Test Infrastructure

### In-Memory Database
All tests use in-memory SQLite for fast, isolated testing:

```go
database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
```

### gRPC Testing
Integration tests use `bufconn` for in-memory gRPC connections:

```go
lis := bufconn.Listen(bufSize)
conn, err := grpc.DialContext(ctx, "bufnet",
    grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
        return lis.Dial()
    }),
    grpc.WithTransportCredentials(insecure.NewCredentials()),
)
```

### Test Fixtures
Helper functions for creating test data:

- `setupTestServer()` - Creates gRPC server with in-memory DB
- `createTestUser()` - Creates test user
- `createTestToken()` - Creates authentication token

## Writing New Tests

### Integration Test Example
```go
func TestNewFeature(t *testing.T) {
    grpcServer, database, tunnelManager, _, lis := setupTestServer(t)
    defer grpcServer.Stop()

    ctx := context.Background()
    // ... test implementation
}
```

### Unit Test Example
```go
func TestNewFunction(t *testing.T) {
    database := setupTestDB(t)
    service := NewService(database)
    ctx := context.Background()

    // Test setup
    // ...

    // Assertions
    require.NoError(t, err)
    assert.Equal(t, expected, actual)
}
```

## Best Practices

1. **Isolation**: Each test should be independent and not rely on others
2. **Cleanup**: Use `defer` to clean up resources (close connections, stop servers)
3. **Assertions**: Use `require` for critical assertions, `assert` for additional checks
4. **Naming**: Use descriptive test names that explain what is being tested
5. **Sub-tests**: Use `t.Run()` for related test cases
6. **Context**: Always pass context to functions that accept it
7. **Error Checking**: Always check errors, even in tests

## CI/CD Integration

Tests are automatically run on:
- Every commit
- Every pull request
- Before deployment

### GitHub Actions
```yaml
- name: Run tests
  run: go test -v -race -coverprofile=coverage.out ./...
```

## Debugging Tests

### Run Single Test
```bash
go test -v -run TestCompleteTunnelFlow ./tests/integration/
```

### Enable Race Detection
```bash
go test -race ./...
```

### View Test Logs
Tests use the logger package which outputs JSON logs during test execution.

## Future Improvements

- [ ] Add frontend tests with Vitest/React Testing Library
- [ ] Add API endpoint tests with real HTTP requests
- [ ] Add performance/benchmark tests
- [ ] Add gRPC proxy flow tests
- [ ] Add WebSocket tests for real-time updates
- [ ] Increase code coverage to 90%+
- [ ] Add mutation testing
- [ ] Add contract tests for gRPC API

## Dependencies

- `github.com/stretchr/testify` - Assertion library
- `gorm.io/driver/sqlite` - In-memory database
- `google.golang.org/grpc/test/bufconn` - In-memory gRPC
