package utils

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"io"

	"golang.org/x/crypto/bcrypt"
)

// GenerateRandomToken generates a random token of specified length.
func GenerateRandomToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := io.ReadFull(rand.Reader, bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// GenerateRequestID generates a unique request ID.
func GenerateRequestID() string {
	token, _ := GenerateRandomToken(16)
	return token
}

// HashToken creates a SHA256 hash of a token.
func HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// Format: grok_<random_32_chars>.
func GenerateAuthToken() (string, string, error) {
	randomPart, err := GenerateRandomToken(16) // 16 bytes = 32 hex chars
	if err != nil {
		return "", "", err
	}

	token := fmt.Sprintf("grok_%s", randomPart)
	hash := HashToken(token)

	return token, hash, nil
}

// HashPassword hashes a password using bcrypt.
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// ComparePassword compares a hashed password with a plain password.
func ComparePassword(hashedPassword, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	return err == nil
}

// - Any other sensitive strings where timing attacks are a concern.
func SecureCompareStrings(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// SanitizeLikePattern escapes special characters in SQL LIKE patterns to prevent SQL injection.
// Escapes: % (matches any string), _ (matches any single character), \ (escape character)
// Use this when building LIKE queries with user input:
//
//	sanitized := utils.SanitizeLikePattern(userInput)
//	query.Where("column LIKE ?", "%"+sanitized+"%")
func SanitizeLikePattern(pattern string) string {
	// Escape backslash first to avoid double-escaping
	pattern = escapeChar(pattern, '\\')
	// Escape SQL LIKE wildcards
	pattern = escapeChar(pattern, '%')
	pattern = escapeChar(pattern, '_')
	return pattern
}

// escapeChar escapes a specific character by prefixing it with backslash.
func escapeChar(s string, char rune) string {
	result := make([]rune, 0, len(s)*2)
	for _, r := range s {
		if r == char {
			result = append(result, '\\')
		}
		result = append(result, r)
	}
	return string(result)
}
