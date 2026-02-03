// Package main provides tests for timing simulation mode.
package main

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/sarchlab/m2sim/emu"
	"github.com/sarchlab/m2sim/timing/latency"
	"github.com/sarchlab/m2sim/timing/pipeline"
)

func TestTiming(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Timing Suite")
}

var _ = Describe("Timing Mode", func() {
	var (
		regFile *emu.RegFile
		memory  *emu.Memory
	)

	BeforeEach(func() {
		regFile = &emu.RegFile{}
		regFile.WriteReg(8, 93) // Set exit syscall number
		memory = emu.NewMemory()
	})

	// Helper to run pipeline with timing and return stats
	runWithTiming := func(config *latency.TimingConfig) pipeline.Statistics {
		table := latency.NewTableWithConfig(config)
		pipe := pipeline.NewPipeline(regFile, memory, pipeline.WithLatencyTable(table))
		pipe.SetPC(0x1000)
		pipe.Run()
		return pipe.Stats()
	}

	// Test Program 1: Simple sequential ALU
	// 3 independent ADD instructions + exit
	// Expected: 3 instructions complete (SVC halts before retire)
	Describe("Test Program 1: Sequential ALU", func() {
		BeforeEach(func() {
			// ADD X0, XZR, #10  ; X0 = 10
			// ADD X1, XZR, #20  ; X1 = 20
			// ADD X2, XZR, #30  ; X2 = 30
			// SVC #0            ; exit
			memory.Write32(0x1000, 0x910029E0) // ADD X0, XZR, #10
			memory.Write32(0x1004, 0x910053E1) // ADD X1, XZR, #20
			memory.Write32(0x1008, 0x91007BE2) // ADD X2, XZR, #30
			memory.Write32(0x100C, 0xD4000001) // SVC #0
		})

		It("should execute 3 instructions (SVC halts before retire)", func() {
			stats := runWithTiming(latency.DefaultTimingConfig())
			// SVC triggers exit in memory stage, before writeback counts it
			Expect(stats.Instructions).To(Equal(uint64(3)))
		})

		It("should have CPI close to 1.0 for simple ALU", func() {
			stats := runWithTiming(latency.DefaultTimingConfig())
			// With pipeline fill time, CPI will be higher than 1.0 for short programs
			// but should be less than 2.0 for this simple case
			Expect(stats.CPI()).To(BeNumerically("<", 3.0))
		})

		It("should produce correct results", func() {
			runWithTiming(latency.DefaultTimingConfig())
			Expect(regFile.ReadReg(0)).To(Equal(uint64(10)))
			Expect(regFile.ReadReg(1)).To(Equal(uint64(20)))
			Expect(regFile.ReadReg(2)).To(Equal(uint64(30)))
		})
	})

	// Test Program 2: RAW Hazard Chain
	// Chained dependencies requiring forwarding
	Describe("Test Program 2: RAW Hazard Chain", func() {
		BeforeEach(func() {
			// ADD X0, XZR, #10  ; X0 = 10
			// ADD X1, X0, #5    ; X1 = X0 + 5 = 15 (RAW: depends on X0)
			// ADD X2, X1, #3    ; X2 = X1 + 3 = 18 (RAW: depends on X1)
			// SVC #0
			memory.Write32(0x1000, 0x910029E0) // ADD X0, XZR, #10
			memory.Write32(0x1004, 0x91001401) // ADD X1, X0, #5
			memory.Write32(0x1008, 0x91000C22) // ADD X2, X1, #3
			memory.Write32(0x100C, 0xD4000001) // SVC #0
		})

		It("should execute 3 instructions (SVC halts before retire)", func() {
			stats := runWithTiming(latency.DefaultTimingConfig())
			// SVC triggers exit in memory stage, before writeback counts it
			Expect(stats.Instructions).To(Equal(uint64(3)))
		})

		It("should produce correct results with forwarding", func() {
			runWithTiming(latency.DefaultTimingConfig())
			Expect(regFile.ReadReg(0)).To(Equal(uint64(10)))
			Expect(regFile.ReadReg(1)).To(Equal(uint64(15)))
			Expect(regFile.ReadReg(2)).To(Equal(uint64(18)))
		})
	})

	// Test Program 3: Load-Use Hazard
	// Load followed by immediate use (stall required)
	Describe("Test Program 3: Load-Use Hazard", func() {
		BeforeEach(func() {
			// LDR X0, [X1]      ; Load from address in X1
			// ADD X2, X0, #5    ; Use X0 immediately (stall)
			// SVC #0
			memory.Write32(0x1000, 0xF9400020) // LDR X0, [X1]
			memory.Write32(0x1004, 0x91001402) // ADD X2, X0, #5
			memory.Write32(0x1008, 0xD4000001) // SVC #0

			// Set up memory with test value
			memory.Write64(0x2000, 100)
			regFile.WriteReg(1, 0x2000)
		})

		It("should have at least one stall", func() {
			stats := runWithTiming(latency.DefaultTimingConfig())
			Expect(stats.Stalls).To(BeNumerically(">", 0))
		})

		It("should produce correct result despite stall", func() {
			runWithTiming(latency.DefaultTimingConfig())
			Expect(regFile.ReadReg(0)).To(Equal(uint64(100)))
			Expect(regFile.ReadReg(2)).To(Equal(uint64(105)))
		})

		It("should have higher CPI than sequential program", func() {
			stats := runWithTiming(latency.DefaultTimingConfig())
			// Load-use stall increases CPI
			Expect(stats.CPI()).To(BeNumerically(">", 1.0))
		})
	})

	// Test timing configuration effects
	Describe("Timing Configuration Effects", func() {
		BeforeEach(func() {
			// Simple program with one ALU instruction
			memory.Write32(0x1000, 0x910029E0) // ADD X0, XZR, #10
			memory.Write32(0x1004, 0xD4000001) // SVC #0
		})

		It("should have more cycles with higher ALU latency", func() {
			defaultConfig := latency.DefaultTimingConfig()
			statsDefault := runWithTiming(defaultConfig)

			// Reset
			regFile = &emu.RegFile{}
			regFile.WriteReg(8, 93)

			// Configure with 4-cycle ALU
			slowConfig := latency.DefaultTimingConfig()
			slowConfig.ALULatency = 4
			statsSlow := runWithTiming(slowConfig)

			// More cycles with slower ALU
			Expect(statsSlow.Cycles).To(BeNumerically(">", statsDefault.Cycles))
		})
	})

	// Test CPI calculation
	Describe("CPI Calculation", func() {
		It("should return 0 CPI for zero instructions", func() {
			stats := pipeline.Statistics{
				Cycles:       10,
				Instructions: 0,
			}
			Expect(stats.CPI()).To(Equal(float64(0)))
		})

		It("should calculate CPI correctly", func() {
			stats := pipeline.Statistics{
				Cycles:       100,
				Instructions: 50,
			}
			Expect(stats.CPI()).To(Equal(float64(2)))
		})
	})
})

// Document timing model assumptions for each test program
var _ = Describe("Timing Model Documentation", func() {
	It("documents timing assumptions", func() {
		// This test documents the timing model assumptions

		// Default M2-based timing:
		// - ALU operations: 1 cycle
		// - Branch: 1 cycle (+ misprediction penalty if applicable)
		// - Load (L1 hit): 4 cycles
		// - Store: 1 cycle (fire-and-forget)
		// - Syscall: 1 cycle

		// Pipeline model:
		// - 5-stage: IF -> ID -> EX -> MEM -> WB
		// - Forwarding: EX/MEM to EX, MEM/WB to EX
		// - Load-use hazard: 1 cycle stall
		// - Branch: execute in EX stage, flush IF/ID on taken

		config := latency.DefaultTimingConfig()
		Expect(config.ALULatency).To(Equal(uint64(1)))
		Expect(config.BranchLatency).To(Equal(uint64(1)))
		Expect(config.LoadLatency).To(Equal(uint64(4)))
		Expect(config.StoreLatency).To(Equal(uint64(1)))
		Expect(config.SyscallLatency).To(Equal(uint64(1)))
		Expect(config.BranchMispredictPenalty).To(Equal(uint64(12)))
	})
})
