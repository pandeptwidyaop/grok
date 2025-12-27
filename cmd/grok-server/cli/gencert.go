package cli

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

var (
	certOutput    string
	keyOutput     string
	certHost      string
	certValidDays int
)

func init() {
	gencertCmd.Flags().StringVar(&certOutput, "cert", "certs/server.crt", "output path for certificate")
	gencertCmd.Flags().StringVarP(&keyOutput, "key", "k", "certs/server.key", "output path for private key")
	gencertCmd.Flags().StringVarP(&certHost, "host", "H", "localhost", "hostname/domain for certificate")
	gencertCmd.Flags().IntVarP(&certValidDays, "days", "d", 365, "certificate validity in days")

	rootCmd.AddCommand(gencertCmd)
}

var gencertCmd = &cobra.Command{
	Use:   "gencert",
	Short: "Generate self-signed TLS certificate and key",
	Long: `Generate a self-signed TLS certificate and private key for development/testing.

This creates a certificate that can be used for gRPC TLS connections. For production,
use proper certificates from Let's Encrypt or a trusted CA.

Examples:
  # Generate certificate for localhost (development)
  grok-server gencert

  # Generate certificate for custom domain
  grok-server gencert --host grok.io

  # Generate with custom output paths
  grok-server gencert --cert /etc/grok/server.crt --key /etc/grok/server.key

  # Generate with 2 year validity
  grok-server gencert --days 730
`,
	RunE: func(_ *cobra.Command, _ []string) error {
		return generateCertificate()
	},
}

func generateCertificate() error {
	fmt.Printf("Generating self-signed certificate...\n")
	fmt.Printf("  Host: %s\n", certHost)
	fmt.Printf("  Valid for: %d days\n", certValidDays)
	fmt.Printf("  Certificate: %s\n", certOutput)
	fmt.Printf("  Private Key: %s\n\n", keyOutput)

	// Generate RSA private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}

	// Create certificate template
	notBefore := time.Now()
	notAfter := notBefore.Add(time.Duration(certValidDays) * 24 * time.Hour)

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("failed to generate serial number: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Grok Tunnel"},
			CommonName:   certHost,
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{certHost},
	}

	// Support wildcard subdomains if domain provided
	if certHost != "localhost" && certHost != "127.0.0.1" {
		template.DNSNames = append(template.DNSNames, "*."+certHost)
	}

	// Create self-signed certificate
	certBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return fmt.Errorf("failed to create certificate: %w", err)
	}

	// Create output directory if not exists
	certDir := filepath.Dir(certOutput)
	if err := os.MkdirAll(certDir, 0755); err != nil {
		return fmt.Errorf("failed to create cert directory: %w", err)
	}

	keyDir := filepath.Dir(keyOutput)
	if err := os.MkdirAll(keyDir, 0755); err != nil {
		return fmt.Errorf("failed to create key directory: %w", err)
	}

	// Write certificate to file
	certFile, err := os.Create(certOutput)
	if err != nil {
		return fmt.Errorf("failed to create cert file: %w", err)
	}
	defer certFile.Close()

	if err := pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: certBytes}); err != nil {
		return fmt.Errorf("failed to write certificate: %w", err)
	}

	// Write private key to file
	keyFile, err := os.OpenFile(keyOutput, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create key file: %w", err)
	}
	defer keyFile.Close()

	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	if err := pem.Encode(keyFile, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: privateKeyBytes}); err != nil {
		return fmt.Errorf("failed to write private key: %w", err)
	}

	fmt.Printf("✓ Certificate generated successfully!\n\n")
	fmt.Printf("Server configuration:\n")
	fmt.Printf("  tls:\n")
	fmt.Printf("    cert_file: %s\n", certOutput)
	fmt.Printf("    key_file: %s\n\n", keyOutput)

	fmt.Printf("Client configuration:\n")
	fmt.Printf("  server:\n")
	fmt.Printf("    tls: true\n")
	fmt.Printf("    tls_cert_file: %s\n", certOutput)
	fmt.Printf("    # OR for development only:\n")
	fmt.Printf("    # tls_insecure: true\n\n")

	fmt.Printf("⚠️  This is a self-signed certificate for development/testing only.\n")
	fmt.Printf("⚠️  For production, use certificates from Let's Encrypt or a trusted CA.\n")

	return nil
}
