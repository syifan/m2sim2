// Package benchmarks provides simple debugging tests.
package benchmarks

import (
	"testing"

	"github.com/sarchlab/m2sim/emu"
	"github.com/sarchlab/m2sim/timing/pipeline"
)

func TestSimplePipeline(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	regFile := &emu.RegFile{}
	regFile.WriteReg(8, 93) // Set exit syscall number (pre-initialize)
	memory := emu.NewMemory()

	// Simple program: just exit (X8 already set to 93)
	memory.Write32(0x1000, 0xD4000001) // SVC #0

	pipe := pipeline.NewPipeline(regFile, memory)
	pipe.SetPC(0x1000)

	exitCode := pipe.Run()
	stats := pipe.Stats()

	t.Logf("Exit code: %d", exitCode)
	t.Logf("Cycles: %d, Instructions: %d, CPI: %.3f",
		stats.Cycles, stats.Instructions, stats.CPI())
}

func TestBenchmarkEncoding(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	// Test the encoding functions
	program := BuildProgram(
		EncodeADDImm(8, 31, 93, false), // X8 = 93
		EncodeSVC(0),
	)

	// Load and run
	regFile := &emu.RegFile{}
	memory := emu.NewMemory()

	for i, b := range program {
		memory.Write8(0x1000+uint64(i), b)
	}

	pipe := pipeline.NewPipeline(regFile, memory)
	pipe.SetPC(0x1000)

	exitCode := pipe.Run()
	stats := pipe.Stats()

	t.Logf("Exit code: %d", exitCode)
	t.Logf("Cycles: %d, Instructions: %d, CPI: %.3f",
		stats.Cycles, stats.Instructions, stats.CPI())

	if exitCode != 0 {
		t.Errorf("expected exit 0, got %d", exitCode)
	}
}

func TestCountdownLoop(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	// t.Skip("Skipped: timing pipeline doesn't update PSTATE flags, causing infinite loop")
	// Simple countdown: X0 = 5, loop decrement until 0
	program := BuildProgram(
		EncodeSUBImm(0, 0, 1, true), // SUBS X0, X0, #1
		EncodeBCond(-4, 1),          // B.NE -4 (back to SUBS)
		EncodeSVC(0),                // exit with X0 = 0
	)

	regFile := &emu.RegFile{}
	regFile.WriteReg(8, 93) // X8 = 93 (exit syscall)
	regFile.WriteReg(0, 5)  // X0 = 5 (counter)
	memory := emu.NewMemory()

	for i, b := range program {
		memory.Write8(0x1000+uint64(i), b)
	}

	pipe := pipeline.NewPipeline(regFile, memory)
	pipe.SetPC(0x1000)

	exitCode := pipe.Run()
	stats := pipe.Stats()

	t.Logf("Exit code: %d", exitCode)
	t.Logf("Cycles: %d, Instructions: %d, CPI: %.3f",
		stats.Cycles, stats.Instructions, stats.CPI())

	if exitCode != 0 {
		t.Errorf("expected exit 0, got %d", exitCode)
	}
}
