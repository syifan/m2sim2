// Package benchmarks provides debugging tests for backward branches.
package benchmarks

import (
	"testing"

	"github.com/sarchlab/m2sim/emu"
	"github.com/sarchlab/m2sim/timing/pipeline"
)

// TestSVCHalt verifies that SVC halts the pipeline correctly
func TestSVCHalt(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	regFile := &emu.RegFile{}
	regFile.WriteReg(8, 93) // X8 = 93 (exit syscall)
	regFile.WriteReg(0, 42) // X0 = 42 (exit code)
	memory := emu.NewMemory()

	// Just SVC #0
	memory.Write32(0x1000, 0xD4000001) // SVC #0

	pipe := pipeline.NewPipeline(regFile, memory)
	pipe.SetPC(0x1000)

	exitCode := pipe.Run()
	stats := pipe.Stats()

	t.Logf("Exit code: %d, cycles: %d, insts: %d", exitCode, stats.Cycles, stats.Instructions)

	if exitCode != 42 {
		t.Errorf("expected exit code 42, got %d", exitCode)
	}
	if !pipe.Halted() {
		t.Error("pipeline should be halted")
	}
}

// TestSUBSFlags verifies that SUBS sets flags correctly
func TestSUBSFlags(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	regFile := &emu.RegFile{}
	regFile.WriteReg(8, 93)
	regFile.WriteReg(0, 1) // X0 = 1
	memory := emu.NewMemory()

	// SUBS X0, X0, #1 -> X0 = 0, Z flag set
	// SVC #0
	memory.Write32(0x1000, 0xF1000400) // SUBS X0, X0, #1
	memory.Write32(0x1004, 0xD4000001) // SVC #0

	pipe := pipeline.NewPipeline(regFile, memory)
	pipe.SetPC(0x1000)

	exitCode := pipe.Run()

	t.Logf("Exit code: %d, X0=%d, Z=%v", exitCode, regFile.ReadReg(0), regFile.PSTATE.Z)

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}
}

func TestBackwardBranch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	// t.Skip("Skipped: timing pipeline doesn't update PSTATE flags, causing infinite loop")
	regFile := &emu.RegFile{}
	regFile.WriteReg(8, 93) // X8 = 93 (exit syscall)
	regFile.WriteReg(0, 2)  // X0 = 2 (small counter)
	memory := emu.NewMemory()

	// Simple backward branch with just 2 iterations
	// 0x1000: SUBS X0, X0, #1  (decrement counter)
	// 0x1004: B.NE -4          (branch back if not zero)
	// 0x1008: SVC #0           (exit)

	// Encode SUBS X0, X0, #1 (64-bit, set flags)
	// sf=1, op=1, S=1, 100010, sh=0, imm12=1, Rn=0, Rd=0
	subs := uint32(0xF1000400) // Pre-computed from ARM ARM
	memory.Write32(0x1000, subs)

	// Encode B.NE -4
	// 0101010 o1 imm19 o0 cond
	// imm19 = -1 (for -4 bytes / 4 = -1)
	bne := uint32(0x54FFFFE1) // B.NE -4
	memory.Write32(0x1004, bne)

	// SVC #0
	svc := uint32(0xD4000001)
	memory.Write32(0x1008, svc)

	pipe := pipeline.NewPipeline(regFile, memory)
	pipe.SetPC(0x1000)

	// Run with limited cycles to prevent infinite loop
	maxCycles := uint64(50)
	for i := uint64(0); i < maxCycles && !pipe.Halted(); i++ {
		pipe.Tick()
		// Log every cycle for debugging
		t.Logf("Cycle %d: PC=0x%x, X0=%d, X8=%d, halted=%v",
			i, pipe.PC(), regFile.ReadReg(0), regFile.ReadReg(8), pipe.Halted())
	}

	stats := pipe.Stats()
	t.Logf("Final: cycles=%d, insts=%d, exit=%d, halted=%v",
		stats.Cycles, stats.Instructions, pipe.ExitCode(), pipe.Halted())

	if !pipe.Halted() {
		t.Error("pipeline did not halt within 100 cycles")
	}
	if pipe.ExitCode() != 0 {
		t.Errorf("expected exit code 0, got %d", pipe.ExitCode())
	}
}
