// Package main provides a profiling wrapper for M2Sim to identify performance bottlenecks.
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime/pprof"
	"time"

	"github.com/sarchlab/m2sim/emu"
	"github.com/sarchlab/m2sim/loader"
	"github.com/sarchlab/m2sim/timing/latency"
	"github.com/sarchlab/m2sim/timing/pipeline"
)

var (
	timing      = flag.Bool("timing", false, "Enable timing simulation mode")
	fastTiming  = flag.Bool("fast-timing", false, "Enable fast timing simulation mode (optimized for calibration)")
	cpuProfile  = flag.String("cpuprofile", "", "write cpu profile to file")
	memProfile  = flag.String("memprofile", "", "write memory profile to file")
	duration    = flag.Duration("duration", 30*time.Second, "max duration to run (for profiling)")
	instruction = flag.Int("max-instr", 1000000, "max instructions to execute (0 = unlimited)")
)

func main() {
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "Usage: profile [options] <program.elf>\n")
		fmt.Fprintf(os.Stderr, "\nOptions:\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Start CPU profiling if requested
	if *cpuProfile != "" {
		f, err := os.Create(*cpuProfile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating CPU profile: %v\n", err)
			os.Exit(1)
		}
		defer func() { _ = f.Close() }()

		if err := pprof.StartCPUProfile(f); err != nil {
			fmt.Fprintf(os.Stderr, "Error starting CPU profile: %v\n", err)
			os.Exit(1)
		}
		defer pprof.StopCPUProfile()
	}

	programPath := flag.Arg(0)

	// Load the ELF program
	prog, err := loader.Load(programPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading program: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Loaded: %s\n", programPath)
	fmt.Printf("Entry point: 0x%X\n", prog.EntryPoint)

	start := time.Now()

	// Set timeout
	go func() {
		time.Sleep(*duration)
		fmt.Printf("\nTimeout reached after %v - stopping execution\n", *duration)
		os.Exit(2)
	}()

	var exitCode int64
	var instrCount uint64

	if *fastTiming {
		exitCode, instrCount = runFastTimingProfile(prog, programPath)
	} else if *timing {
		exitCode, instrCount = runTimingProfile(prog, programPath)
	} else {
		exitCode, instrCount = runEmulationProfile(prog, programPath)
	}

	elapsed := time.Since(start)

	// Write memory profile if requested
	if *memProfile != "" {
		f, err := os.Create(*memProfile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating memory profile: %v\n", err)
			os.Exit(1)
		}
		defer func() { _ = f.Close() }()

		if err := pprof.WriteHeapProfile(f); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing memory profile: %v\n", err)
		}
	}

	fmt.Printf("\nProfiling Results:\n")
	fmt.Printf("Exit code: %d\n", exitCode)
	fmt.Printf("Instructions executed: %d\n", instrCount)
	fmt.Printf("Elapsed time: %v\n", elapsed)
	if instrCount > 0 {
		fmt.Printf("Instructions/second: %.0f\n", float64(instrCount)/elapsed.Seconds())
	}
}

// loadSegments loads program segments into memory.
func loadSegments(memory *emu.Memory, prog *loader.Program) {
	for _, seg := range prog.Segments {
		for i, b := range seg.Data {
			memory.Write8(seg.VirtAddr+uint64(i), b)
		}
		// Zero-fill BSS (memsize > filesize)
		for i := uint64(len(seg.Data)); i < seg.MemSize; i++ {
			memory.Write8(seg.VirtAddr+i, 0)
		}
	}
}

// runEmulationProfile runs the program in functional emulation mode with profiling.
func runEmulationProfile(prog *loader.Program, programPath string) (int64, uint64) {
	memory := emu.NewMemory()

	// Load all segments into memory
	loadSegments(memory, prog)

	// Create emulator options
	opts := []emu.EmulatorOption{
		emu.WithStackPointer(prog.InitialSP),
	}

	// Add instruction limit if specified
	if *instruction > 0 {
		opts = append(opts, emu.WithMaxInstructions(uint64(*instruction)))
	}

	// Create emulator with options
	emulator := emu.NewEmulator(opts...)
	emulator.LoadProgram(prog.EntryPoint, memory)

	// Run
	exitCode := emulator.Run()
	instrCount := emulator.InstructionCount()

	return exitCode, instrCount
}

// runTimingProfile runs the program in timing simulation mode with profiling.
func runTimingProfile(prog *loader.Program, programPath string) (int64, uint64) {
	// Set up timing configuration
	timingConfig := latency.DefaultTimingConfig()
	latencyTable := latency.NewTableWithConfig(timingConfig)

	// Set up memory and register file
	memory := emu.NewMemory()
	regFile := &emu.RegFile{}
	regFile.SP = prog.InitialSP

	// Load all segments into memory
	loadSegments(memory, prog)

	// Create pipeline with timing
	syscallHandler := emu.NewDefaultSyscallHandler(regFile, memory, os.Stdout, os.Stderr)
	pipe := pipeline.NewPipeline(
		regFile,
		memory,
		pipeline.WithSyscallHandler(syscallHandler),
		pipeline.WithLatencyTable(latencyTable),
	)
	pipe.SetPC(prog.EntryPoint)

	// Note: Pipeline doesn't have instruction limits yet, will run with timeout

	// Run the pipeline
	exitCode := pipe.Run()

	// Get statistics
	stats := pipe.Stats()

	return exitCode, stats.Instructions
}

// runFastTimingProfile runs the program in fast timing simulation mode with profiling.
func runFastTimingProfile(prog *loader.Program, programPath string) (int64, uint64) {
	// Set up timing configuration
	timingConfig := latency.DefaultTimingConfig()
	latencyTable := latency.NewTableWithConfig(timingConfig)

	// Set up memory and register file
	memory := emu.NewMemory()
	regFile := &emu.RegFile{}
	regFile.SP = prog.InitialSP

	// Load all segments into memory
	loadSegments(memory, prog)

	// Create fast timing simulation
	syscallHandler := emu.NewDefaultSyscallHandler(regFile, memory, os.Stdout, os.Stderr)

	// Set up fast timing options
	var fastTimingOpts []pipeline.FastTimingOption
	if *instruction > 0 {
		fastTimingOpts = append(fastTimingOpts, pipeline.WithMaxInstructions(uint64(*instruction)))
	}

	fastTiming := pipeline.NewFastTiming(regFile, memory, latencyTable, syscallHandler, fastTimingOpts...)
	fastTiming.SetPC(prog.EntryPoint)

	// Run the fast timing simulation
	exitCode := fastTiming.Run()

	// Get statistics
	stats := fastTiming.Stats()

	return exitCode, stats.Instructions
}
