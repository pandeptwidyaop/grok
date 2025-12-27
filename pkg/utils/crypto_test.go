package utils

import (
	"strings"
	"testing"
)

// TestGenerateRandomToken tests random token generation.
func TestGenerateRandomToken(t *testing.T) {
	token1, err := GenerateRandomToken(32)
	if err != nil {
		t.Fatalf("GenerateRandomToken() unexpected error: %v", err)
	}
	token2, err := GenerateRandomToken(32)
	if err != nil {
		t.Fatalf("GenerateRandomToken() unexpected error: %v", err)
	}

	// Tokens should be unique
	if token1 == token2 {
		t.Errorf("GenerateRandomToken() should generate unique tokens, got same: %s", token1)
	}

	// Tokens should be hex encoded (32 bytes * 2 = 64 hex chars)
	if len(token1) != 64 {
		t.Errorf("GenerateRandomToken() length = %d; want 64", len(token1))
	}

	if len(token2) != 64 {
		t.Errorf("GenerateRandomToken() length = %d; want 64", len(token2))
	}

	// Should only contain hex characters (0-9, a-f)
	for _, r := range token1 {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			t.Errorf("GenerateRandomToken() contains non-hex character: %c", r)
		}
	}

	// Test with different length
	token3, err := GenerateRandomToken(16)
	if err != nil {
		t.Fatalf("GenerateRandomToken(16) unexpected error: %v", err)
	}
	if len(token3) != 32 {
		t.Errorf("GenerateRandomToken(16) should produce 32 hex chars, got %d", len(token3))
	}
}

// TestGenerateRequestID tests request ID generation.
func TestGenerateRequestID(t *testing.T) {
	id1 := GenerateRequestID()
	id2 := GenerateRequestID()

	// IDs should be unique
	if id1 == id2 {
		t.Errorf("GenerateRequestID() should generate unique IDs, got same: %s", id1)
	}

	// Length should be 16 bytes * 2 = 32 hex chars
	if len(id1) != 32 {
		t.Errorf("GenerateRequestID() length = %d; want 32", len(id1))
	}

	// Should only contain hex characters
	for _, r := range id1 {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			t.Errorf("GenerateRequestID() contains non-hex character: %c", r)
		}
	}
}

// TestHashToken tests token hashing.
func TestHashToken(t *testing.T) {
	token := "grok_test_token_12345"
	hash1 := HashToken(token)
	hash2 := HashToken(token)

	// Same input should produce same hash
	if hash1 != hash2 {
		t.Errorf("HashToken() should be deterministic, got different hashes: %s vs %s", hash1, hash2)
	}

	// Hash should be SHA256 (64 hex chars)
	if len(hash1) != 64 {
		t.Errorf("HashToken() length = %d; want 64 (SHA256)", len(hash1))
	}

	// Different inputs should produce different hashes
	hash3 := HashToken("different_token")
	if hash1 == hash3 {
		t.Errorf("HashToken() should produce different hashes for different inputs")
	}

	// Should only contain hex characters
	for _, r := range hash1 {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			t.Errorf("HashToken() contains non-hex character: %c", r)
		}
	}
}

// TestGenerateAuthToken tests auth token generation.
func TestGenerateAuthToken(t *testing.T) {
	token, hash, err := GenerateAuthToken()
	if err != nil {
		t.Fatalf("GenerateAuthToken() unexpected error: %v", err)
	}

	// Token should start with "grok_"
	if !strings.HasPrefix(token, "grok_") {
		t.Errorf("GenerateAuthToken() token should start with 'grok_', got: %s", token)
	}

	// Token length should be "grok_" (5 chars) + 32 hex chars = 37
	// (16 bytes from GenerateRandomToken(16) = 32 hex chars)
	if len(token) != 37 {
		t.Errorf("GenerateAuthToken() token length = %d; want 37", len(token))
	}

	// Hash should be SHA256 of token
	expectedHash := HashToken(token)
	if hash != expectedHash {
		t.Errorf("GenerateAuthToken() hash mismatch: got %s, want %s", hash, expectedHash)
	}

	// Generate multiple tokens to verify uniqueness
	token2, hash2, err := GenerateAuthToken()
	if err != nil {
		t.Fatalf("GenerateAuthToken() unexpected error: %v", err)
	}

	if token == token2 {
		t.Errorf("GenerateAuthToken() should generate unique tokens")
	}

	if hash == hash2 {
		t.Errorf("GenerateAuthToken() should generate unique hashes")
	}
}

// TestHashPassword tests password hashing.
func TestHashPassword(t *testing.T) {
	password := "TestPassword123!"

	hash1, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() unexpected error: %v", err)
	}

	// Hash should not be empty
	if hash1 == "" {
		t.Error("HashPassword() returned empty hash")
	}

	// Hash should be different from password
	if hash1 == password {
		t.Error("HashPassword() hash should be different from password")
	}

	// Generate another hash with same password - should be different (bcrypt uses random salt)
	hash2, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() unexpected error: %v", err)
	}

	if hash1 == hash2 {
		t.Error("HashPassword() should generate different hashes due to random salt")
	}

	// Hash should start with bcrypt prefix
	if !strings.HasPrefix(hash1, "$2a$") && !strings.HasPrefix(hash1, "$2b$") {
		t.Errorf("HashPassword() should produce bcrypt hash, got: %s", hash1[:4])
	}
}

// TestComparePassword tests password comparison.
func TestComparePassword(t *testing.T) {
	password := "CorrectPassword123!"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() unexpected error: %v", err)
	}

	tests := []struct {
		name     string
		hash     string
		password string
		expected bool
	}{
		{
			name:     "correct password",
			hash:     hash,
			password: password,
			expected: true,
		},
		{
			name:     "wrong password",
			hash:     hash,
			password: "WrongPassword",
			expected: false,
		},
		{
			name:     "empty password",
			hash:     hash,
			password: "",
			expected: false,
		},
		{
			name:     "case sensitive",
			hash:     hash,
			password: strings.ToLower(password),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ComparePassword(tt.hash, tt.password)
			if result != tt.expected {
				t.Errorf("ComparePassword() = %v; want %v", result, tt.expected)
			}
		})
	}
}

// TestSecureCompareStrings tests constant-time string comparison.
func TestSecureCompareStrings(t *testing.T) {
	tests := []struct {
		name     string
		a        string
		b        string
		expected bool
	}{
		{
			name:     "identical strings",
			a:        "secret123",
			b:        "secret123",
			expected: true,
		},
		{
			name:     "different strings",
			a:        "secret123",
			b:        "secret456",
			expected: false,
		},
		{
			name:     "different lengths",
			a:        "short",
			b:        "longer string",
			expected: false,
		},
		{
			name:     "empty strings",
			a:        "",
			b:        "",
			expected: true,
		},
		{
			name:     "one empty",
			a:        "test",
			b:        "",
			expected: false,
		},
		{
			name:     "case sensitive",
			a:        "Secret",
			b:        "secret",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SecureCompareStrings(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("SecureCompareStrings(%q, %q) = %v; want %v", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

// TestSanitizeLikePattern tests SQL LIKE pattern sanitization.
func TestSanitizeLikePattern(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no special chars",
			input:    "simple",
			expected: "simple",
		},
		{
			name:     "with percent",
			input:    "test%pattern",
			expected: "test\\%pattern",
		},
		{
			name:     "with underscore",
			input:    "test_pattern",
			expected: "test\\_pattern",
		},
		{
			name:     "with backslash",
			input:    "path\\to\\file",
			expected: "path\\\\to\\\\file",
		},
		{
			name:     "all special chars",
			input:    "test%_\\pattern",
			expected: "test\\%\\_\\\\pattern",
		},
		{
			name:     "multiple percents",
			input:    "%%test%%",
			expected: "\\%\\%test\\%\\%",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeLikePattern(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeLikePattern(%q) = %q; want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// Benchmark tests.
func BenchmarkGenerateRandomToken(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := GenerateRandomToken(32)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGenerateRequestID(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GenerateRequestID()
	}
}

func BenchmarkHashToken(b *testing.B) {
	token := "grok_test_token_benchmark"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		HashToken(token)
	}
}

func BenchmarkGenerateAuthToken(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GenerateAuthToken()
	}
}

func BenchmarkHashPassword(b *testing.B) {
	password := "TestPassword123!"
	for i := 0; i < b.N; i++ {
		HashPassword(password)
	}
}

func BenchmarkComparePassword(b *testing.B) {
	password := "TestPassword123!"
	hash, _ := HashPassword(password)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ComparePassword(hash, password)
	}
}

func BenchmarkSecureCompareStrings(b *testing.B) {
	a := "secret_token_123"
	bStr := "secret_token_123"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SecureCompareStrings(a, bStr)
	}
}

func BenchmarkSanitizeLikePattern(b *testing.B) {
	input := "test%_\\pattern"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SanitizeLikePattern(input)
	}
}
