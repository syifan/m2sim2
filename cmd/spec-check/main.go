// Package main provides a CLI tool to check SPEC benchmark availability.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sarchlab/m2sim/benchmarks"
)

func main() {
	// Find repository root
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting working directory: %v\n", err)
		os.Exit(1)
	}

	// Walk up to find go.mod
	repoRoot := cwd
	for {
		if _, err := os.Stat(filepath.Join(repoRoot, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(repoRoot)
		if parent == repoRoot {
			fmt.Fprintf(os.Stderr, "Could not find repository root (go.mod)\n")
			os.Exit(1)
		}
		repoRoot = parent
	}

	runner, err := benchmarks.NewSPECRunner(repoRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "SPEC not available: %v\n", err)
		fmt.Println("0")
		os.Exit(0)
	}

	if err := runner.ValidateSetup(); err != nil {
		fmt.Fprintf(os.Stderr, "SPEC setup invalid: %v\n", err)
		fmt.Println("0")
		os.Exit(0)
	}

	available := runner.ListAvailableBenchmarks()
	missing := runner.ListMissingBenchmarks()

	fmt.Printf("%d\n", len(available))

	if len(available) > 0 {
		fmt.Fprintf(os.Stderr, "\nAvailable benchmarks (%d):\n", len(available))
		for _, b := range available {
			fmt.Fprintf(os.Stderr, "  ✅ %s - %s\n", b.Name, b.Description)
		}
	}

	if len(missing) > 0 {
		fmt.Fprintf(os.Stderr, "\nMissing benchmarks (%d):\n", len(missing))
		for _, b := range missing {
			fmt.Fprintf(os.Stderr, "  ❌ %s - run 'scripts/spec-setup.sh build'\n", b.Name)
		}
	}
}
