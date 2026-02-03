// Package main provides the entry point for M2Sim.
// M2Sim is a cycle-accurate Apple M2 CPU simulator built on Akita.
//
// For the full CLI, use: go run ./cmd/m2sim
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("M2Sim - Apple M2 CPU Simulator")
	fmt.Println("Built on Akita simulation framework")
	fmt.Println("")
	fmt.Println("Usage: m2sim [options] <program.elf>")
	fmt.Println("")
	fmt.Println("Options:")
	fmt.Println("  -timing    Enable timing simulation mode")
	fmt.Println("  -config    Path to timing configuration JSON file")
	fmt.Println("  -v         Verbose output")
	fmt.Println("")
	fmt.Println("Run 'go run ./cmd/m2sim' for the full CLI.")

	if len(os.Args) > 1 {
		fmt.Println("\nNote: You provided arguments. Use 'go run ./cmd/m2sim' instead.")
	}
}
