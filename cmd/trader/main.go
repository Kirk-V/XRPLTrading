// Package main is the initial executable entry point.
package main

import (
	"fmt"
	"os"

	"github.com/Kirk-V/XRPLTrading/internal/config"
)

func main() {
	cfg := config.Default()
	if err := cfg.Validate(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("XRPL Trading Tool: %s read-only foundation\n", cfg.Network)
}
