package auth

import (
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// TOTPService handles two-factor authentication using TOTP.
type TOTPService struct{}

// NewTOTPService creates a new TOTP service.
func NewTOTPService() *TOTPService {
	return &TOTPService{}
}

// GenerateSecret generates a new TOTP secret for a user.
func (s *TOTPService) GenerateSecret(domain, email string) (string, string, error) {
	// Generate a random secret
	secret := make([]byte, 20)
	_, err := rand.Read(secret)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate secret: %w", err)
	}

	// Encode the secret in base32
	secretStr := base32.StdEncoding.EncodeToString(secret)

	// Generate key with issuer format: "Grok {domain}: {username}"
	issuerName := fmt.Sprintf("Grok %s", domain)
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuerName,
		AccountName: email,
		Secret:      []byte(secretStr),
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to generate TOTP key: %w", err)
	}

	// Return secret and QR code URL
	return key.Secret(), key.URL(), nil
}

// ValidateCode validates a TOTP code against a secret.
func (s *TOTPService) ValidateCode(secret, code string) bool {
	// Validate the code with a 30-second window
	valid := totp.Validate(code, secret)
	return valid
}

// This allows codes from previous/next periods to be valid.
func (s *TOTPService) ValidateCodeWithWindow(secret, code string, window int) bool {
	now := time.Now()

	// Try current time
	if totp.Validate(code, secret) {
		return true
	}

	// Try previous and next windows
	for i := 1; i <= window; i++ {
		// Try past
		pastTime := now.Add(time.Duration(-i*30) * time.Second)
		if code == generateCodeAtTime(secret, pastTime) {
			return true
		}

		// Try future
		futureTime := now.Add(time.Duration(i*30) * time.Second)
		if code == generateCodeAtTime(secret, futureTime) {
			return true
		}
	}

	return false
}

// generateCodeAtTime generates a TOTP code at a specific time.
func generateCodeAtTime(secret string, t time.Time) string {
	code, err := totp.GenerateCodeCustom(secret, t, totp.ValidateOpts{
		Period:    30,
		Skew:      0,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
	if err != nil {
		return ""
	}
	return code
}
