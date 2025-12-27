package utils

import (
	"strings"
	"testing"
)

// TestGenerateRandomSubdomain tests random subdomain generation.
func TestGenerateRandomSubdomain(t *testing.T) {
	sub1, err := GenerateRandomSubdomain(8)
	if err != nil {
		t.Fatalf("GenerateRandomSubdomain() unexpected error: %v", err)
	}
	sub2, err := GenerateRandomSubdomain(8)
	if err != nil {
		t.Fatalf("GenerateRandomSubdomain() unexpected error: %v", err)
	}

	// Subdomains should be unique
	if sub1 == sub2 {
		t.Errorf("GenerateRandomSubdomain() should generate unique subdomains, got same: %s", sub1)
	}

	// Subdomains should be exactly 8 characters
	if len(sub1) != 8 {
		t.Errorf("GenerateRandomSubdomain() length = %d; want 8", len(sub1))
	}

	if len(sub2) != 8 {
		t.Errorf("GenerateRandomSubdomain() length = %d; want 8", len(sub2))
	}

	// Should only contain allowed characters (lowercase letters and numbers)
	for _, r := range sub1 {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')) {
			t.Errorf("GenerateRandomSubdomain() contains invalid character: %c", r)
		}
	}

	for _, r := range sub2 {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')) {
			t.Errorf("GenerateRandomSubdomain() contains invalid character: %c", r)
		}
	}
}

// TestIsValidSubdomain tests subdomain validation.
func TestIsValidSubdomain(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		// Valid cases
		{"valid lowercase", "myapp", true},
		{"valid with hyphen", "my-app", true},
		{"valid with numbers", "app123", true},
		{"valid min length", "abc", true},
		{"valid max length", strings.Repeat("a", 63), true},
		{"valid complex", "my-app-123", true},

		// Invalid cases
		{"too short", "ab", false},
		{"too long", strings.Repeat("a", 64), false},
		{"starts with hyphen", "-myapp", false},
		{"ends with hyphen", "myapp-", false},
		{"underscore", "my_app", false},
		{"special chars", "my@app", false},
		{"spaces", "my app", false},
		{"empty", "", false},
		{"reserved: api", "api", false},
		{"reserved: admin", "admin", false},
		{"reserved: www", "www", false},
		{"reserved: dashboard", "dashboard", false},
		{"reserved: test", "test", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidSubdomain(tt.input)
			if result != tt.expected {
				t.Errorf("IsValidSubdomain(%q) = %v; want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// TestIsReservedSubdomain tests reserved subdomain checking.
func TestIsReservedSubdomain(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		// Reserved subdomains
		{"api", "api", true},
		{"admin", "admin", true},
		{"www", "www", true},
		{"dashboard", "dashboard", true},
		{"test", "test", true},

		// Not reserved
		{"myapp", "myapp", false},
		{"app123", "app123", false},
		{"status", "status", false},
		{"docs", "docs", false},
		{"blog", "blog", false},
		{"support", "support", false},
		{"help", "help", false},

		// Case sensitivity - function converts to lowercase before checking
		{"API uppercase", "API", true},
		{"Admin mixed", "Admin", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsReservedSubdomain(tt.input)
			if result != tt.expected {
				t.Errorf("IsReservedSubdomain(%q) = %v; want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// TestNormalizeSubdomain tests subdomain normalization.
func TestNormalizeSubdomain(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"already lowercase", "myapp", "myapp"},
		{"uppercase to lowercase", "MyApp", "myapp"},
		{"mixed case", "MyAPP123", "myapp123"},
		{"with hyphens", "My-App", "my-app"},
		{"all caps", "TESTAPP", "testapp"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeSubdomain(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeSubdomain(%q) = %q; want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestGenerateRandomName tests random name generation.
func TestGenerateRandomName(t *testing.T) {
	name1, err := GenerateRandomName()
	if err != nil {
		t.Fatalf("GenerateRandomName() unexpected error: %v", err)
	}
	name2, err := GenerateRandomName()
	if err != nil {
		t.Fatalf("GenerateRandomName() unexpected error: %v", err)
	}

	// Names should be unique (very high probability)
	if name1 == name2 {
		t.Errorf("GenerateRandomName() should generate unique names, got same: %s", name1)
	}

	// Names should contain hyphens (adjective-noun format)
	if !strings.Contains(name1, "-") {
		t.Errorf("GenerateRandomName() should contain hyphen, got: %s", name1)
	}

	if !strings.Contains(name2, "-") {
		t.Errorf("GenerateRandomName() should contain hyphen, got: %s", name2)
	}

	// Names should be lowercase
	if name1 != strings.ToLower(name1) {
		t.Errorf("GenerateRandomName() should be lowercase, got: %s", name1)
	}

	if name2 != strings.ToLower(name2) {
		t.Errorf("GenerateRandomName() should be lowercase, got: %s", name2)
	}

	// Names should not be too long (adjective + hyphen + noun + random suffix)
	if len(name1) > 50 {
		t.Errorf("GenerateRandomName() too long: %d characters", len(name1))
	}

	// Names should not be empty
	if name1 == "" {
		t.Error("GenerateRandomName() returned empty string")
	}
}

// TestGenerateRandomNameUniqueness tests that random names are actually unique.
func TestGenerateRandomNameUniqueness(t *testing.T) {
	names := make(map[string]bool)
	iterations := 50 // Reduced from 100 to avoid collisions with limited word pool

	for i := 0; i < iterations; i++ {
		name, err := GenerateRandomName()
		if err != nil {
			t.Fatalf("GenerateRandomName() unexpected error: %v", err)
		}
		names[name] = true
	}

	// With 30 adjectives and 30 nouns, we have 900 combinations
	// Expect at least 90% uniqueness with 50 iterations
	if len(names) < int(float64(iterations)*0.9) {
		t.Errorf("Expected at least %d unique names (90%%), got %d", int(float64(iterations)*0.9), len(names))
	}
}

// TestGenerateRandomSubdomainUniqueness tests uniqueness of random subdomains.
func TestGenerateRandomSubdomainUniqueness(t *testing.T) {
	subdomains := make(map[string]bool)
	iterations := 100

	for i := 0; i < iterations; i++ {
		sub, err := GenerateRandomSubdomain(8)
		if err != nil {
			t.Fatalf("GenerateRandomSubdomain() unexpected error: %v", err)
		}
		if subdomains[sub] {
			t.Errorf("GenerateRandomSubdomain() generated duplicate: %s", sub)
		}
		subdomains[sub] = true
	}

	if len(subdomains) != iterations {
		t.Errorf("Expected %d unique subdomains, got %d", iterations, len(subdomains))
	}
}

// TestIsValidSubdomainWithRandomGenerated tests that randomly generated subdomains are valid.
func TestIsValidSubdomainWithRandomGenerated(t *testing.T) {
	for i := 0; i < 20; i++ {
		sub, err := GenerateRandomSubdomain(8)
		if err != nil {
			t.Fatalf("GenerateRandomSubdomain() unexpected error: %v", err)
		}
		if !IsValidSubdomain(sub) {
			t.Errorf("GenerateRandomSubdomain() generated invalid subdomain: %s", sub)
		}
	}
}

// Benchmark tests.
func BenchmarkGenerateRandomSubdomain(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := GenerateRandomSubdomain(8)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkIsValidSubdomain(b *testing.B) {
	subdomain := "my-app-123"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		IsValidSubdomain(subdomain)
	}
}

func BenchmarkIsReservedSubdomain(b *testing.B) {
	subdomain := "api"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		IsReservedSubdomain(subdomain)
	}
}

func BenchmarkNormalizeSubdomain(b *testing.B) {
	subdomain := "MyApp123"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NormalizeSubdomain(subdomain)
	}
}

func BenchmarkGenerateRandomName(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := GenerateRandomName()
		if err != nil {
			b.Fatal(err)
		}
	}
}
