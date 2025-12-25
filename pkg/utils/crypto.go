package utils

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
)

// GenerateRandomToken generates a random token of specified length
func GenerateRandomToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := io.ReadFull(rand.Reader, bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// GenerateRequestID generates a unique request ID
func GenerateRequestID() string {
	token, _ := GenerateRandomToken(16)
	return token
}

// HashToken creates a SHA256 hash of a token
func HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// GenerateAuthToken generates a formatted auth token
// Format: grok_<random_32_chars>
func GenerateAuthToken() (string, string, error) {
	randomPart, err := GenerateRandomToken(16) // 16 bytes = 32 hex chars
	if err != nil {
		return "", "", err
	}

	token := fmt.Sprintf("grok_%s", randomPart)
	hash := HashToken(token)

	return token, hash, nil
}
