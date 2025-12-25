package utils

import (
	"crypto/rand"
	"math/big"
	"regexp"
	"strings"
)

const (
	// SubdomainChars contains allowed characters for subdomain generation
	SubdomainChars = "abcdefghijklmnopqrstuvwxyz0123456789"
	// DefaultSubdomainLength is the default length for random subdomains
	DefaultSubdomainLength = 8
	// MinSubdomainLength is the minimum allowed subdomain length
	MinSubdomainLength = 4
	// MaxSubdomainLength is the maximum allowed subdomain length
	MaxSubdomainLength = 63
)

var (
	// subdomainRegex validates subdomain format
	subdomainRegex = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

	// Reserved subdomains that cannot be used
	reservedSubdomains = map[string]bool{
		"www":       true,
		"api":       true,
		"admin":     true,
		"dashboard": true,
		"app":       true,
		"web":       true,
		"mail":      true,
		"smtp":      true,
		"ftp":       true,
		"localhost": true,
		"test":      true,
		"dev":       true,
		"staging":   true,
		"prod":      true,
		"grpc":      true,
	}
)

// GenerateRandomSubdomain generates a random subdomain of specified length
func GenerateRandomSubdomain(length int) (string, error) {
	if length < MinSubdomainLength {
		length = DefaultSubdomainLength
	}

	result := make([]byte, length)
	charsLen := big.NewInt(int64(len(SubdomainChars)))

	for i := 0; i < length; i++ {
		num, err := rand.Int(rand.Reader, charsLen)
		if err != nil {
			return "", err
		}
		result[i] = SubdomainChars[num.Int64()]
	}

	return string(result), nil
}

// IsValidSubdomain checks if a subdomain is valid
func IsValidSubdomain(subdomain string) bool {
	// Convert to lowercase
	subdomain = strings.ToLower(subdomain)

	// Check length
	if len(subdomain) < MinSubdomainLength || len(subdomain) > MaxSubdomainLength {
		return false
	}

	// Check format (lowercase alphanumeric and hyphens, no leading/trailing hyphens)
	if !subdomainRegex.MatchString(subdomain) {
		return false
	}

	// Check if reserved
	if reservedSubdomains[subdomain] {
		return false
	}

	return true
}

// IsReservedSubdomain checks if a subdomain is reserved
func IsReservedSubdomain(subdomain string) bool {
	return reservedSubdomains[strings.ToLower(subdomain)]
}

// NormalizeSubdomain normalizes a subdomain (lowercase, trim)
func NormalizeSubdomain(subdomain string) string {
	return strings.ToLower(strings.TrimSpace(subdomain))
}
