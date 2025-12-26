# Contributing to Grok

First off, thank you for considering contributing to Grok! It's people like you that make Grok such a great tool.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [How Can I Contribute?](#how-can-i-contribute)
  - [Reporting Bugs](#reporting-bugs)
  - [Suggesting Enhancements](#suggesting-enhancements)
  - [Pull Requests](#pull-requests)
- [Development Setup](#development-setup)
- [Coding Guidelines](#coding-guidelines)
- [Commit Messages](#commit-messages)
- [Testing](#testing)

## Code of Conduct

This project and everyone participating in it is governed by a Code of Conduct. By participating, you are expected to uphold this code. Please report unacceptable behavior to the project maintainers.

## How Can I Contribute?

### Reporting Bugs

Before creating bug reports, please check the existing issues to avoid duplicates. When you create a bug report, include as many details as possible:

- **Use a clear and descriptive title**
- **Describe the exact steps to reproduce the problem**
- **Provide specific examples**
- **Describe the behavior you observed and what you expected**
- **Include logs, screenshots, or error messages**
- **Mention your environment** (OS, Go version, etc.)

**Bug Report Template:**

```markdown
## Description
[Clear description of the bug]

## Steps to Reproduce
1. [First Step]
2. [Second Step]
3. [And so on...]

## Expected Behavior
[What you expected to happen]

## Actual Behavior
[What actually happened]

## Environment
- OS: [e.g., Ubuntu 22.04, macOS 14.0]
- Go Version: [e.g., 1.25.1]
- Grok Version: [e.g., v1.0.0]

## Logs/Screenshots
[Paste relevant logs or add screenshots]
```

### Suggesting Enhancements

Enhancement suggestions are tracked as GitHub issues. When creating an enhancement suggestion:

- **Use a clear and descriptive title**
- **Provide a detailed description of the proposed feature**
- **Explain why this enhancement would be useful**
- **Provide examples of how it would work**

**Enhancement Template:**

```markdown
## Feature Description
[Clear description of the feature]

## Use Case
[Explain the problem this solves]

## Proposed Solution
[How you envision this working]

## Alternatives Considered
[Other solutions you've thought about]

## Additional Context
[Any other relevant information]
```

### Pull Requests

1. **Fork the repository**
   ```bash
   git clone https://github.com/your-username/grok.git
   cd grok
   ```

2. **Create a branch**
   ```bash
   git checkout -b feature/your-feature-name
   # or
   git checkout -b fix/your-bug-fix
   ```

3. **Make your changes**
   - Follow the [coding guidelines](#coding-guidelines)
   - Write or update tests as needed
   - Update documentation if needed

4. **Test your changes**
   ```bash
   make test
   go test ./...
   ```

5. **Commit your changes**
   - Follow the [commit message guidelines](#commit-messages)
   ```bash
   git add .
   git commit -m "feat: add amazing feature"
   ```

6. **Push to your fork**
   ```bash
   git push origin feature/your-feature-name
   ```

7. **Open a Pull Request**
   - Fill in the PR template
   - Link related issues
   - Request review from maintainers

## Development Setup

### Prerequisites

- Go 1.25 or higher
- Node.js 18+ (for web dashboard)
- Protocol Buffers compiler
- SQLite or PostgreSQL
- Air (for hot reload during development)

### Setup Instructions

1. **Clone and install dependencies**
   ```bash
   git clone https://github.com/pandeptwidyaop/grok.git
   cd grok
   go mod download
   make install-tools
   ```

2. **Generate gRPC code**
   ```bash
   make proto
   ```

3. **Build everything**
   ```bash
   make build-all
   ```

4. **Run tests**
   ```bash
   make test
   ```

5. **Start development servers**
   ```bash
   # Terminal 1: Backend with hot reload
   air

   # Terminal 2: Frontend with hot reload
   cd web && npm run dev
   ```

## Coding Guidelines

### Go Code Style

- Follow the official [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- Use `gofmt` for formatting
- Use `golint` and `go vet` for linting
- Keep functions small and focused
- Write descriptive variable names
- Add comments for exported functions and complex logic

**Example:**

```go
// GenerateSubdomain creates a random 8-character subdomain
// using lowercase alphanumeric characters.
func GenerateSubdomain() string {
    const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
    b := make([]byte, 8)
    for i := range b {
        b[i] = charset[rand.Intn(len(charset))]
    }
    return string(b)
}
```

### TypeScript/React Code Style

- Use TypeScript for type safety
- Follow React best practices and hooks guidelines
- Use functional components
- Use meaningful component and variable names
- Keep components small and reusable

**Example:**

```typescript
interface TunnelCardProps {
  tunnel: Tunnel;
  onDelete: (id: string) => void;
}

export function TunnelCard({ tunnel, onDelete }: TunnelCardProps) {
  return (
    <Card>
      <CardContent>
        <Typography variant="h6">{tunnel.subdomain}</Typography>
        <Button onClick={() => onDelete(tunnel.id)}>Delete</Button>
      </CardContent>
    </Card>
  );
}
```

### Project Structure

- Place server code in `internal/server/`
- Place client code in `internal/client/`
- Place shared code in `pkg/`
- Place React components in `web/src/components/`
- Write tests in `*_test.go` files next to the code

## Commit Messages

We follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <subject>

<body>

<footer>
```

### Types

- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, no logic change)
- `refactor`: Code refactoring
- `perf`: Performance improvements
- `test`: Adding or updating tests
- `chore`: Maintenance tasks
- `ci`: CI/CD changes

### Examples

```
feat(server): add webhook broadcasting support

Implement webhook routing that broadcasts incoming webhook
requests to multiple active tunnels simultaneously.

Closes #123
```

```
fix(client): resolve auto-reconnection race condition

Fix race condition in tunnel reconnection logic that could
cause duplicate tunnel registrations.

Fixes #456
```

```
docs(readme): update installation instructions

Add detailed steps for PostgreSQL setup and clarify
environment variable configuration.
```

## Testing

### Writing Tests

- Write unit tests for new functions
- Write integration tests for new features
- Aim for high code coverage (>80%)
- Use table-driven tests when appropriate

**Example Unit Test:**

```go
func TestGenerateSubdomain(t *testing.T) {
    subdomain := GenerateSubdomain()

    if len(subdomain) != 8 {
        t.Errorf("expected length 8, got %d", len(subdomain))
    }

    // Test that subdomain only contains valid characters
    for _, char := range subdomain {
        if !isValidSubdomainChar(char) {
            t.Errorf("invalid character in subdomain: %c", char)
        }
    }
}
```

**Example Integration Test:**

```go
func TestCompleteTunnelFlow(t *testing.T) {
    // Setup test server
    server := setupTestServer(t)
    defer server.Stop()

    // Create client and connect
    client := createTestClient(t)
    err := client.Connect()
    if err != nil {
        t.Fatalf("failed to connect: %v", err)
    }

    // Test tunnel registration
    tunnel, err := client.RegisterTunnel("test-subdomain")
    if err != nil {
        t.Fatalf("failed to register tunnel: %v", err)
    }

    // Verify tunnel is active
    if tunnel.Status != "active" {
        t.Errorf("expected active status, got %s", tunnel.Status)
    }
}
```

### Running Tests

```bash
# Run all tests
make test

# Run specific package tests
go test ./internal/server/tunnel/

# Run with coverage
go test -cover ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## Pull Request Checklist

Before submitting a pull request, ensure:

- [ ] Code follows the project's coding guidelines
- [ ] All tests pass (`make test`)
- [ ] New code has appropriate test coverage
- [ ] Documentation is updated if needed
- [ ] Commit messages follow conventional commits
- [ ] PR description clearly describes the changes
- [ ] Related issues are linked
- [ ] Code is formatted (`gofmt`, `prettier`)
- [ ] No linting errors (`golint`, `eslint`)

## Review Process

1. **Automated Checks**: CI/CD will run tests and linters
2. **Code Review**: Maintainers will review your code
3. **Discussion**: Address feedback and questions
4. **Approval**: Once approved, your PR will be merged
5. **Release**: Changes will be included in the next release

## Getting Help

- **Questions**: Open a [GitHub Discussion](https://github.com/pandeptwidyaop/grok/discussions)
- **Chat**: Join our community (coming soon)
- **Issues**: Browse existing [GitHub Issues](https://github.com/pandeptwidyaop/grok/issues)

## Recognition

Contributors will be recognized in:
- The project README
- Release notes
- GitHub contributors page

Thank you for contributing to Grok! 🎉
