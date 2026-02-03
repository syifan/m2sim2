// Package main provides the entry point for M2Sim.
// M2Sim is a cycle-accurate Apple M2 CPU simulator.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/sarchlab/m2sim/emu"
	"github.com/sarchlab/m2sim/loader"
	"github.com/sarchlab/m2sim/timing/latency"
	"github.com/sarchlab/m2sim/timing/pipeline"
)

var (
	timing     = flag.Bool("timing", false, "Enable timing simulation mode")
	configPath = flag.String("config", "", "Path to timing configuration JSON file")
	verbose    = flag.Bool("v", false, "Verbose output")
)

func main() {
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "Usage: m2sim [options] <program.elf>\n")
		fmt.Fprintf(os.Stderr, "\nOptions:\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	programPath := flag.Arg(0)

	// Load the ELF program
	prog, err := loader.Load(programPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading program: %v\n", err)
		os.Exit(1)
	}

	if *verbose {
		fmt.Printf("Loaded: %s\n", programPath)
		fmt.Printf("Entry point: 0x%X\n", prog.EntryPoint)
		fmt.Printf("Segments: %d\n", len(prog.Segments))
	}

	if *timing {
		exitCode := runTiming(prog, programPath)
		os.Exit(int(exitCode))
	} else {
		exitCode := runEmulation(prog, programPath)
		os.Exit(int(exitCode))
	}
}

// runEmulation runs the program in functional emulation mode.
func runEmulation(prog *loader.Program, programPath string) int64 {
	memory := emu.NewMemory()

	// Load all segments into memory
	for _, seg := range prog.Segments {
		for i, b := range seg.Data {
			memory.Write8(seg.VirtAddr+uint64(i), b)
		}
		// Zero-fill BSS (memsize > filesize)
		for i := uint64(len(seg.Data)); i < seg.MemSize; i++ {
			memory.Write8(seg.VirtAddr+i, 0)
		}
	}

	// Create emulator with loaded memory
	emulator := emu.NewEmulator(
		emu.WithStackPointer(prog.InitialSP),
	)
	emulator.LoadProgram(prog.EntryPoint, memory)

	// Run
	exitCode := emulator.Run()

	if *verbose {
		fmt.Printf("\nProgram: %s\n", programPath)
		fmt.Printf("Exit code: %d\n", exitCode)
		fmt.Printf("Instructions executed: %d\n", emulator.InstructionCount())
	}

	return exitCode
}

// runTiming runs the program in timing simulation mode.
func runTiming(prog *loader.Program, programPath string) int64 {
	// Set up timing configuration
	var timingConfig *latency.TimingConfig
	if *configPath != "" {
		var err error
		timingConfig, err = latency.LoadConfig(*configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading timing config: %v\n", err)
			os.Exit(1)
		}
	} else {
		timingConfig = latency.DefaultTimingConfig()
	}

	latencyTable := latency.NewTableWithConfig(timingConfig)

	// Set up memory and register file
	memory := emu.NewMemory()
	regFile := &emu.RegFile{}
	regFile.SP = prog.InitialSP

	// Load all segments into memory
	for _, seg := range prog.Segments {
		for i, b := range seg.Data {
			memory.Write8(seg.VirtAddr+uint64(i), b)
		}
		// Zero-fill BSS (memsize > filesize)
		for i := uint64(len(seg.Data)); i < seg.MemSize; i++ {
			memory.Write8(seg.VirtAddr+i, 0)
		}
	}

	// Create pipeline with timing
	syscallHandler := emu.NewDefaultSyscallHandler(regFile, memory, os.Stdout, os.Stderr)
	pipe := pipeline.NewPipeline(
		regFile,
		memory,
		pipeline.WithSyscallHandler(syscallHandler),
		pipeline.WithLatencyTable(latencyTable),
	)
	pipe.SetPC(prog.EntryPoint)

	// Run the pipeline
	exitCode := pipe.Run()

	// Get statistics
	stats := pipe.Stats()

	// Calculate breakdown percentages
	totalCycles := stats.Cycles
	if totalCycles == 0 {
		totalCycles = 1 // Avoid division by zero
	}

	// Calculate stall breakdown
	// Fetch stalls = Stalls (pipeline stalls)
	// Decode stalls = part of pipeline stalls (estimated)
	// Execute = Instructions (base execution)
	// Memory stalls = MemStalls

	// For a simple model:
	// - Execute cycles ~ Instructions (each instruction takes at least 1 cycle in execute)
	// - Stalls covers fetch/decode stalls from hazards
	// - ExecStalls covers multi-cycle execution
	// - MemStalls covers memory latency

	// Note: In a real pipeline, timing is complex. This is an approximation.
	fetchDecodeStalls := stats.Stalls + stats.Flushes // Stalls from hazards + branch flushes
	execCycles := stats.Instructions                  // Base execution
	execStalls := stats.ExecStalls                    // Multi-cycle instruction stalls
	memStalls := stats.MemStalls                      // Memory stalls

	// Print timing report
	fmt.Printf("\n")
	fmt.Printf("Program: %s\n", programPath)
	fmt.Printf("Exit code: %d\n", exitCode)
	fmt.Printf("Total Instructions: %d\n", stats.Instructions)
	fmt.Printf("Total Cycles: %d\n", stats.Cycles)
	fmt.Printf("CPI: %.2f\n", stats.CPI())
	fmt.Printf("\n")
	fmt.Printf("Breakdown:\n")
	fmt.Printf("  Fetch/Decode stalls: %4d cycles (%5.1f%%)\n",
		fetchDecodeStalls, 100.0*float64(fetchDecodeStalls)/float64(totalCycles))
	fmt.Printf("  Execute:             %4d cycles (%5.1f%%)\n",
		execCycles, 100.0*float64(execCycles)/float64(totalCycles))
	fmt.Printf("  Execute stalls:      %4d cycles (%5.1f%%)\n",
		execStalls, 100.0*float64(execStalls)/float64(totalCycles))
	fmt.Printf("  Memory stalls:       %4d cycles (%5.1f%%)\n",
		memStalls, 100.0*float64(memStalls)/float64(totalCycles))
	fmt.Printf("\n")
	fmt.Printf("Pipeline Events:\n")
	fmt.Printf("  Stalls:  %d\n", stats.Stalls)
	fmt.Printf("  Flushes: %d\n", stats.Flushes)

	return exitCode
}
