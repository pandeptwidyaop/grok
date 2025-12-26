package main

import (
	"github.com/pandeptwidyaop/grok/cmd/grok-server/cli"
)

var (
	// Version info (set by ldflags during build)
	version   = "dev"
	buildTime = "unknown"
	gitCommit = "unknown"
)

func main() {
	// Set version info for CLI
	cli.SetVersion(version, buildTime, gitCommit)

	// Execute CLI
	cli.Execute()
}
