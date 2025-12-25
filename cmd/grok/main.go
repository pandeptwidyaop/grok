package main

import (
	"fmt"
	"os"

	"github.com/pandeptwidyaop/grok/internal/client/cli"
)

var (
	// Version info (set by ldflags during build)
	version   = "dev"
	buildTime = "unknown"
	gitCommit = "unknown"
)

func main() {
	// Pass version info to CLI
	cli.SetVersion(version, buildTime, gitCommit)

	if err := cli.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
