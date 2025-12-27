package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/pandeptwidyaop/grok/internal/client/config"
)

// configCmd represents the config command.
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage grok configuration",
	Long:  `Manage grok client configuration including authentication token.`,
}

// setTokenCmd represents the set-token command.
var setTokenCmd = &cobra.Command{
	Use:   "set-token [token]",
	Short: "Set authentication token",
	Long: `Set the authentication token for connecting to the grok server.

Example:
  grok config set-token grok_abc123def456...`,
	Args: cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		token := args[0]

		if err := config.SaveToken(token); err != nil {
			return fmt.Errorf("failed to save token: %w", err)
		}

		fmt.Printf("✓ Authentication token saved to config file\n")
		fmt.Printf("You can now use 'grok http' and 'grok tcp' commands\n")

		return nil
	},
}

// setServerCmd represents the set-server command.
var setServerCmd = &cobra.Command{
	Use:   "set-server [address]",
	Short: "Set grok server address and optional TLS settings",
	Long: `Set the grok server address for tunnel connections with optional TLS configuration.

The address can be specified as:
  - hostname only (uses default port 4443): cloudtunnel.id
  - hostname with port: cloudtunnel.id:8080
  - IP address with port: 192.168.1.100:4443

TLS can be configured in one command using flags, or separately using dedicated commands.

Examples:
  # Basic server setup
  grok config set-server cloudtunnel.id

  # Quick TLS setup (one-liner)
  grok config set-server cloudtunnel.id --tls
  grok config set-server cloudtunnel.id --tls --tls-insecure
  grok config set-server cloudtunnel.id --tls --tls-cert certs/server.crt

  # Separate commands (same result)
  grok config set-server cloudtunnel.id
  grok config enable-tls
  grok config set-tls-insecure true`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		addr := args[0]

		// Add default port if not specified
		if !hasPort(addr) {
			addr = addr + ":4443"
		}

		// Save server address first
		if err := config.SaveServer(addr); err != nil {
			return fmt.Errorf("failed to save server address: %w", err)
		}

		fmt.Printf("✓ Server address saved: %s\n", addr)

		// Get flags
		enableTLS, _ := cmd.Flags().GetBool("tls")
		disableTLS, _ := cmd.Flags().GetBool("no-tls")
		tlsCert, _ := cmd.Flags().GetString("tls-cert")
		tlsInsecure, _ := cmd.Flags().GetBool("tls-insecure")

		// Validate conflicting flags
		if enableTLS && disableTLS {
			return fmt.Errorf("cannot use both --tls and --no-tls flags")
		}

		if tlsCert != "" && tlsInsecure {
			return fmt.Errorf("cannot use both --tls-cert and --tls-insecure flags")
		}

		// Process TLS flags
		if disableTLS {
			// Explicitly disable TLS
			if err := config.DisableTLS(); err != nil {
				return fmt.Errorf("failed to disable TLS: %w", err)
			}
			fmt.Printf("⚠️  TLS disabled - connection will be insecure\n")
		} else if tlsCert != "" {
			// Enable TLS with custom certificate
			if err := config.SetTLSCert(tlsCert); err != nil {
				return fmt.Errorf("failed to set TLS certificate: %w", err)
			}
			fmt.Printf("✓ TLS enabled with certificate: %s\n", tlsCert)
		} else if tlsInsecure {
			// Enable TLS in insecure mode
			if err := config.SetTLSInsecure(true); err != nil {
				return fmt.Errorf("failed to set TLS insecure mode: %w", err)
			}
			fmt.Printf("⚠️  TLS enabled in insecure mode (certificate verification disabled)\n")
			fmt.Printf("⚠️  This should ONLY be used for development/testing!\n")
		} else if enableTLS {
			// Enable TLS with system CA pool
			if err := config.EnableTLS(); err != nil {
				return fmt.Errorf("failed to enable TLS: %w", err)
			}
			fmt.Printf("✓ TLS enabled with system CA pool\n")
		}

		fmt.Printf("Config file: ~/.grok/config.yaml\n")

		return nil
	},
}

// hasPort checks if address already has a port.
func hasPort(addr string) bool {
	// Simple check for colon in address
	// Works for: "example.com:8080", "192.168.1.1:4443"
	// Doesn't match: "example.com", "192.168.1.1"
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return true
		}
		if addr[i] == '/' {
			return false
		}
	}
	return false
}

// setTLSCertCmd represents the set-tls-cert command.
var setTLSCertCmd = &cobra.Command{
	Use:   "set-tls-cert [path]",
	Short: "Set TLS certificate file and enable TLS",
	Long: `Set the TLS certificate file for server verification and enable TLS.

Use this for self-signed certificates or custom CA certificates.
For Let's Encrypt or other trusted CAs, use 'enable-tls' instead.

Examples:
  grok config set-tls-cert certs/server.crt
  grok config set-tls-cert /path/to/ca-bundle.crt`,
	Args: cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		certPath := args[0]

		if err := config.SetTLSCert(certPath); err != nil {
			return fmt.Errorf("failed to set TLS certificate: %w", err)
		}

		fmt.Printf("✓ TLS enabled with certificate: %s\n", certPath)
		fmt.Printf("Certificate will be used to verify server identity\n")

		return nil
	},
}

// setTLSInsecureCmd represents the set-tls-insecure command.
var setTLSInsecureCmd = &cobra.Command{
	Use:   "set-tls-insecure [true|false]",
	Short: "Enable/disable TLS insecure mode (skip verification)",
	Long: `Enable or disable TLS insecure mode (skip certificate verification).

⚠️  WARNING: Insecure mode disables certificate verification!
This should ONLY be used for development/testing, NEVER in production.

When enabled:
  - TLS encryption is still active
  - Certificate verification is disabled
  - Vulnerable to man-in-the-middle attacks

Examples:
  grok config set-tls-insecure true   # Enable insecure mode (dev only)
  grok config set-tls-insecure false  # Disable insecure mode`,
	Args: cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		insecureStr := args[0]
		insecure := insecureStr == "true"

		if err := config.SetTLSInsecure(insecure); err != nil {
			return fmt.Errorf("failed to set TLS insecure mode: %w", err)
		}

		if insecure {
			fmt.Printf("⚠️  TLS insecure mode enabled (certificate verification disabled)\n")
			fmt.Printf("⚠️  This should ONLY be used for development/testing!\n")
		} else {
			fmt.Printf("✓ TLS insecure mode disabled (certificate verification enabled)\n")
		}

		return nil
	},
}

// enableTLSCmd represents the enable-tls command.
var enableTLSCmd = &cobra.Command{
	Use:   "enable-tls",
	Short: "Enable TLS with system CA pool",
	Long: `Enable TLS using the system's certificate authority (CA) pool.

Use this for servers with certificates from trusted CAs like:
  - Let's Encrypt
  - DigiCert
  - Cloudflare
  - Any CA trusted by your operating system

No custom certificate file is needed - the system's built-in CA certificates
will be used to verify the server.

Example:
  grok config enable-tls`,
	RunE: func(_ *cobra.Command, _ []string) error {
		if err := config.EnableTLS(); err != nil {
			return fmt.Errorf("failed to enable TLS: %w", err)
		}

		fmt.Printf("✓ TLS enabled with system CA pool\n")
		fmt.Printf("Server certificate will be verified using system certificates\n")
		fmt.Printf("Works with Let's Encrypt, DigiCert, and other trusted CAs\n")

		return nil
	},
}

// disableTLSCmd represents the disable-tls command.
var disableTLSCmd = &cobra.Command{
	Use:   "disable-tls",
	Short: "Disable TLS (insecure connection)",
	Long: `Disable TLS and use an insecure connection to the server.

⚠️  WARNING: This sends all data in plaintext!
Only use this for local development when connecting to localhost.

Example:
  grok config disable-tls`,
	RunE: func(_ *cobra.Command, _ []string) error {
		if err := config.DisableTLS(); err != nil {
			return fmt.Errorf("failed to disable TLS: %w", err)
		}

		fmt.Printf("⚠️  TLS disabled - connection will be insecure\n")
		fmt.Printf("All data will be sent in plaintext\n")

		return nil
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(setTokenCmd)
	configCmd.AddCommand(setServerCmd)
	configCmd.AddCommand(setTLSCertCmd)
	configCmd.AddCommand(setTLSInsecureCmd)
	configCmd.AddCommand(enableTLSCmd)
	configCmd.AddCommand(disableTLSCmd)

	// Add TLS flags to set-server command for convenience
	setServerCmd.Flags().Bool("tls", false, "Enable TLS with system CA pool")
	setServerCmd.Flags().Bool("no-tls", false, "Explicitly disable TLS")
	setServerCmd.Flags().String("tls-cert", "", "Enable TLS with custom certificate file")
	setServerCmd.Flags().Bool("tls-insecure", false, "Enable TLS in insecure mode (skip verification, dev only)")
}
