// Command benchmark runs the M2Sim timing benchmark harness.
//
// Usage:
//
//	go run ./cmd/benchmark [flags]
//
// Flags:
//
//	-csv        Output results in CSV format (default: human-readable)
//	-no-icache  Disable instruction cache simulation
//	-no-dcache  Disable data cache simulation
//
// Example:
//
//	# Run all benchmarks with human-readable output
//	go run ./cmd/benchmark
//
//	# Output CSV for spreadsheet comparison
//	go run ./cmd/benchmark -csv > results.csv
//
// The benchmark results can be compared against real M2 hardware measurements
// to calibrate the simulator's timing model.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/sarchlab/m2sim/benchmarks"
)

func main() {
	// Parse flags
	csvOutput := flag.Bool("csv", false, "Output results in CSV format")
	noICache := flag.Bool("no-icache", false, "Disable instruction cache simulation")
	noDCache := flag.Bool("no-dcache", false, "Disable data cache simulation")
	flag.Parse()

	// Configure harness
	config := benchmarks.DefaultConfig()
	config.EnableICache = !*noICache
	config.EnableDCache = !*noDCache
	config.Output = os.Stdout

	// Create harness and add benchmarks
	harness := benchmarks.NewHarness(config)
	harness.AddBenchmarks(benchmarks.GetMicrobenchmarks())

	// Print configuration
	if !*csvOutput {
		fmt.Println("M2Sim Timing Benchmark Harness")
		fmt.Println("==============================")
		fmt.Printf("I-Cache: %v\n", config.EnableICache)
		fmt.Printf("D-Cache: %v\n", config.EnableDCache)
		fmt.Println("")
	}

	// Run benchmarks
	results := harness.RunAll()

	// Output results
	if *csvOutput {
		harness.PrintCSV(results)
	} else {
		harness.PrintResults(results)

		// Print summary
		fmt.Println("=== Summary ===")
		fmt.Println("")
		fmt.Println("To compare with real M2 hardware:")
		fmt.Println("1. Compile equivalent ARM64 programs")
		fmt.Println("2. Run on real M2 with performance counters")
		fmt.Println("3. Compare cycle counts and CPI")
		fmt.Println("")
		fmt.Println("Expected characteristics:")
		fmt.Println("- arithmetic_loop: High branch prediction rate, low CPI")
		fmt.Println("- dependency_chain: Higher CPI due to RAW hazards")
		fmt.Println("- memory_sequential: Cache-friendly, good hit rate")
		fmt.Println("- branch_heavy: More pipeline flushes, higher CPI")
		fmt.Println("- function_calls: Call/return overhead visible")
		fmt.Println("- mixed_workload: Balanced characteristics")
	}
}
