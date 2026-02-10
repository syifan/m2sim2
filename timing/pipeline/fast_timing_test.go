package pipeline_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/sarchlab/m2sim/emu"
	"github.com/sarchlab/m2sim/timing/latency"
	"github.com/sarchlab/m2sim/timing/pipeline"
)

// mockSyscallHandler implements emu.SyscallHandler for testing.
type mockSyscallHandler struct {
	regFile    *emu.RegFile
	callCount  int
	exitOnCall bool // If true, return Exited=true on the next Handle() call
	exitCode   int64
}

func (m *mockSyscallHandler) Handle() emu.SyscallResult {
	m.callCount++
	sysNum := m.regFile.ReadReg(8)

	if sysNum == 93 || m.exitOnCall {
		return emu.SyscallResult{
			Exited:   true,
			ExitCode: m.exitCode,
		}
	}

	return emu.SyscallResult{Exited: false}
}

var _ = Describe("FastTiming", func() {
	var (
		regFile        *emu.RegFile
		memory         *emu.Memory
		table          *latency.Table
		syscallHandler *mockSyscallHandler
	)

	BeforeEach(func() {
		regFile = &emu.RegFile{}
		memory = emu.NewMemory()
		table = latency.NewTable()
		syscallHandler = &mockSyscallHandler{
			regFile: regFile,
		}
	})

	Describe("NewFastTiming", func() {
		It("should create a new fast timing instance", func() {
			ft := pipeline.NewFastTiming(regFile, memory, table, syscallHandler)
			Expect(ft).NotTo(BeNil())
		})

		It("should apply WithMaxInstructions option", func() {
			ft := pipeline.NewFastTiming(regFile, memory, table, syscallHandler,
				pipeline.WithMaxInstructions(100))
			Expect(ft).NotTo(BeNil())
		})
	})

	Describe("SetPC", func() {
		It("should set the program counter", func() {
			ft := pipeline.NewFastTiming(regFile, memory, table, syscallHandler)
			ft.SetPC(0x1000)
			Expect(ft.PC).To(Equal(uint64(0x1000)))
		})
	})

	Describe("Tick", func() {
		It("should not execute when halted", func() {
			ft := pipeline.NewFastTiming(regFile, memory, table, syscallHandler)
			ft.SetPC(0x1000)

			// Set up exit syscall
			regFile.WriteReg(8, 93)
			memory.Write32(0x1000, 0xD4000001) // SVC #0

			ft.Run()
			statsBefore := ft.Stats()

			// Extra ticks should be no-ops
			ft.Tick()
			ft.Tick()

			statsAfter := ft.Stats()
			Expect(statsAfter.Cycles).To(Equal(statsBefore.Cycles))
		})

		It("should halt on unknown instruction", func() {
			ft := pipeline.NewFastTiming(regFile, memory, table, syscallHandler)
			ft.SetPC(0x1000)
			memory.Write32(0x1000, 0x00000000) // Likely decodes to unknown

			exitCode := ft.Run()
			Expect(exitCode).To(Equal(int64(-1)))
		})
	})

	Describe("Instruction Execution", func() {
		Context("ADD", func() {
			It("should execute ADD immediate", func() {
				ft := pipeline.NewFastTiming(regFile, memory, table, syscallHandler)
				ft.SetPC(0x1000)

				// ADD X0, X1, #10 => 0x91002820
				memory.Write32(0x1000, 0x91002820)
				// SVC #0 (exit)
				memory.Write32(0x1004, 0xD4000001)
				regFile.WriteReg(1, 100)
				regFile.WriteReg(8, 93)

				ft.Run()
				Expect(regFile.ReadReg(0)).To(Equal(uint64(110)))
			})

			It("should execute ADD register", func() {
				ft := pipeline.NewFastTiming(regFile, memory, table, syscallHandler)
				ft.SetPC(0x1000)

				// ADD X0, X1, X2 => 0x8B020020
				memory.Write32(0x1000, 0x8B020020)
				memory.Write32(0x1004, 0xD4000001)
				regFile.WriteReg(1, 30)
				regFile.WriteReg(2, 12)
				regFile.WriteReg(8, 93)

				ft.Run()
				Expect(regFile.ReadReg(0)).To(Equal(uint64(42)))
			})
		})

		Context("SUB", func() {
			It("should execute SUB immediate", func() {
				ft := pipeline.NewFastTiming(regFile, memory, table, syscallHandler)
				ft.SetPC(0x1000)

				// SUB X0, X1, #20 => 0xD1005020
				memory.Write32(0x1000, 0xD1005020)
				memory.Write32(0x1004, 0xD4000001)
				regFile.WriteReg(1, 100)
				regFile.WriteReg(8, 93)

				ft.Run()
				Expect(regFile.ReadReg(0)).To(Equal(uint64(80)))
			})
		})

		Context("LDR", func() {
			It("should execute LDR with unsigned offset", func() {
				ft := pipeline.NewFastTiming(regFile, memory, table, syscallHandler)
				ft.SetPC(0x1000)

				// LDR X0, [X1] => 0xF9400020
				memory.Write32(0x1000, 0xF9400020)
				memory.Write32(0x1004, 0xD4000001)
				memory.Write64(0x2000, 0xDEADBEEF12345678)
				regFile.WriteReg(1, 0x2000)
				regFile.WriteReg(8, 93)

				ft.Run()
				Expect(regFile.ReadReg(0)).To(Equal(uint64(0xDEADBEEF12345678)))
			})
		})

		Context("STR", func() {
			It("should execute STR with unsigned offset", func() {
				ft := pipeline.NewFastTiming(regFile, memory, table, syscallHandler)
				ft.SetPC(0x1000)

				// STR X0, [X1] => 0xF9000020
				memory.Write32(0x1000, 0xF9000020)
				memory.Write32(0x1004, 0xD4000001)
				regFile.WriteReg(0, 0xCAFEBABE)
				regFile.WriteReg(1, 0x3000)
				regFile.WriteReg(8, 93)

				ft.Run()
				Expect(memory.Read64(0x3000)).To(Equal(uint64(0xCAFEBABE)))
			})
		})

		Context("Branch Instructions", func() {
			It("should handle unconditional branch B", func() {
				ft := pipeline.NewFastTiming(regFile, memory, table, syscallHandler)
				ft.SetPC(0x1000)

				// B #8 (branch forward 2 instructions)
				memory.Write32(0x1000, 0x14000002)
				// ADD X0, XZR, #10 (skipped)
				memory.Write32(0x1004, 0x910029E0)
				// SVC #0 (exit, reached via branch)
				memory.Write32(0x1008, 0xD4000001)
				regFile.WriteReg(8, 93)

				ft.Run()
				Expect(regFile.ReadReg(0)).To(Equal(uint64(0))) // X0 should remain 0
			})

			It("should handle BL (branch with link)", func() {
				ft := pipeline.NewFastTiming(regFile, memory, table, syscallHandler)
				ft.SetPC(0x1000)

				// BL #8
				memory.Write32(0x1000, 0x94000002)
				// ADD X0, XZR, #10 (skipped)
				memory.Write32(0x1004, 0x910029E0)
				// SVC #0
				memory.Write32(0x1008, 0xD4000001)
				regFile.WriteReg(8, 93)

				ft.Run()
				Expect(regFile.ReadReg(30)).To(Equal(uint64(0x1004))) // Link register
			})

			It("should handle RET", func() {
				ft := pipeline.NewFastTiming(regFile, memory, table, syscallHandler)
				ft.SetPC(0x1000)

				// BL #8 => jump to 0x1008, set X30=0x1004
				memory.Write32(0x1000, 0x94000002)
				// SVC (at return address 0x1004)
				memory.Write32(0x1004, 0xD4000001)
				// RET => jump to X30 (0x1004)
				memory.Write32(0x1008, 0xD65F03C0)
				regFile.WriteReg(8, 93)

				ft.Run()
				Expect(regFile.ReadReg(30)).To(Equal(uint64(0x1004)))
			})
		})

		Context("MOVZ", func() {
			It("should execute MOVZ", func() {
				ft := pipeline.NewFastTiming(regFile, memory, table, syscallHandler)
				ft.SetPC(0x1000)

				// MOVZ X0, #42 => 0xD2800540
				memory.Write32(0x1000, 0xD2800540)
				memory.Write32(0x1004, 0xD4000001)
				regFile.WriteReg(8, 93)

				ft.Run()
				Expect(regFile.ReadReg(0)).To(Equal(uint64(42)))
			})
		})

		Context("MOVN", func() {
			It("should execute MOVN", func() {
				ft := pipeline.NewFastTiming(regFile, memory, table, syscallHandler)
				ft.SetPC(0x1000)

				// MOVN X0, #0 => 0x92800000 (result = ^0 = 0xFFFFFFFFFFFFFFFF)
				memory.Write32(0x1000, 0x92800000)
				memory.Write32(0x1004, 0xD4000001)
				regFile.WriteReg(8, 93)

				ft.Run()
				Expect(regFile.ReadReg(0)).To(Equal(uint64(0xFFFFFFFFFFFFFFFF)))
			})
		})

		Context("Logical Instructions", func() {
			It("should execute AND immediate", func() {
				ft := pipeline.NewFastTiming(regFile, memory, table, syscallHandler)
				ft.SetPC(0x1000)

				// AND X0, X1, X2 => 0x8A020020
				memory.Write32(0x1000, 0x8A020020)
				memory.Write32(0x1004, 0xD4000001)
				regFile.WriteReg(1, 0xFF)
				regFile.WriteReg(2, 0x0F)
				regFile.WriteReg(8, 93)

				ft.Run()
				Expect(regFile.ReadReg(0)).To(Equal(uint64(0x0F)))
			})

			It("should execute ORR register", func() {
				ft := pipeline.NewFastTiming(regFile, memory, table, syscallHandler)
				ft.SetPC(0x1000)

				// ORR X0, X1, X2 => 0xAA020020
				memory.Write32(0x1000, 0xAA020020)
				memory.Write32(0x1004, 0xD4000001)
				regFile.WriteReg(1, 0xF0)
				regFile.WriteReg(2, 0x0F)
				regFile.WriteReg(8, 93)

				ft.Run()
				Expect(regFile.ReadReg(0)).To(Equal(uint64(0xFF)))
			})

			It("should execute EOR register", func() {
				ft := pipeline.NewFastTiming(regFile, memory, table, syscallHandler)
				ft.SetPC(0x1000)

				// EOR X0, X1, X2 => 0xCA020020
				memory.Write32(0x1000, 0xCA020020)
				memory.Write32(0x1004, 0xD4000001)
				regFile.WriteReg(1, 0xFF)
				regFile.WriteReg(2, 0x0F)
				regFile.WriteReg(8, 93)

				ft.Run()
				Expect(regFile.ReadReg(0)).To(Equal(uint64(0xF0)))
			})
		})

		Context("STP and LDP", func() {
			It("should execute STP", func() {
				ft := pipeline.NewFastTiming(regFile, memory, table, syscallHandler)
				ft.SetPC(0x1000)

				// STP X0, X1, [X2] => 0xA9000440
				memory.Write32(0x1000, 0xA9000440)
				memory.Write32(0x1004, 0xD4000001)
				regFile.WriteReg(0, 0xAAAA)
				regFile.WriteReg(1, 0xBBBB)
				regFile.WriteReg(2, 0x3000)
				regFile.WriteReg(8, 93)

				ft.Run()
				Expect(memory.Read64(0x3000)).To(Equal(uint64(0xAAAA)))
				Expect(memory.Read64(0x3008)).To(Equal(uint64(0xBBBB)))
			})

			It("should execute LDP", func() {
				ft := pipeline.NewFastTiming(regFile, memory, table, syscallHandler)
				ft.SetPC(0x1000)

				// LDP X0, X1, [X2] => 0xA9400440
				memory.Write32(0x1000, 0xA9400440)
				memory.Write32(0x1004, 0xD4000001)
				memory.Write64(0x3000, 0xAAAA)
				memory.Write64(0x3008, 0xBBBB)
				regFile.WriteReg(2, 0x3000)
				regFile.WriteReg(8, 93)

				ft.Run()
				Expect(regFile.ReadReg(0)).To(Equal(uint64(0xAAAA)))
				Expect(regFile.ReadReg(1)).To(Equal(uint64(0xBBBB)))
			})
		})

		Context("CBZ and CBNZ", func() {
			It("should branch on CBZ when register is zero", func() {
				ft := pipeline.NewFastTiming(regFile, memory, table, syscallHandler)
				ft.SetPC(0x1000)

				// CBZ X0, #8 => 0xB4000040
				memory.Write32(0x1000, 0xB4000040) // CBZ X0, +8
				memory.Write32(0x1004, 0x910029E1) // ADD X1, XZR, #10 (skipped)
				memory.Write32(0x1008, 0xD4000001) // SVC #0
				regFile.WriteReg(0, 0)             // X0 = 0 → branch taken
				regFile.WriteReg(8, 93)

				ft.Run()
				Expect(regFile.ReadReg(1)).To(Equal(uint64(0)))
			})

			It("should not branch on CBZ when register is nonzero", func() {
				ft := pipeline.NewFastTiming(regFile, memory, table, syscallHandler)
				ft.SetPC(0x1000)

				// CBZ X0, #8
				memory.Write32(0x1000, 0xB4000040)
				memory.Write32(0x1004, 0x910029E1) // ADD X1, XZR, #10
				memory.Write32(0x1008, 0xD4000001) // SVC #0
				regFile.WriteReg(0, 5)             // X0 = 5 → branch not taken
				regFile.WriteReg(8, 93)

				ft.Run()
				Expect(regFile.ReadReg(1)).To(Equal(uint64(10)))
			})

			It("should branch on CBNZ when register is nonzero", func() {
				ft := pipeline.NewFastTiming(regFile, memory, table, syscallHandler)
				ft.SetPC(0x1000)

				// CBNZ X0, #8 => 0xB5000040
				memory.Write32(0x1000, 0xB5000040)
				memory.Write32(0x1004, 0x910029E1) // ADD X1, XZR, #10 (skipped)
				memory.Write32(0x1008, 0xD4000001) // SVC #0
				regFile.WriteReg(0, 5)             // X0 = 5 → branch taken
				regFile.WriteReg(8, 93)

				ft.Run()
				Expect(regFile.ReadReg(1)).To(Equal(uint64(0)))
			})
		})

		Context("MADD and MSUB", func() {
			It("should execute MADD (multiply-add)", func() {
				ft := pipeline.NewFastTiming(regFile, memory, table, syscallHandler)
				ft.SetPC(0x1000)

				// MADD X0, X1, X2, X3 => 0x9B020C20
				memory.Write32(0x1000, 0x9B020C20)
				memory.Write32(0x1004, 0xD4000001)
				regFile.WriteReg(1, 3) // Rn
				regFile.WriteReg(2, 4) // Rm
				regFile.WriteReg(3, 5) // Ra (stored in Rt2)
				regFile.WriteReg(8, 93)

				ft.Run()
				// MADD: Rd = Ra + Rn * Rm = 5 + 3*4 = 17
				Expect(regFile.ReadReg(0)).To(Equal(uint64(17)))
			})
		})

		Context("Byte and Half-word Memory", func() {
			It("should execute LDRB", func() {
				ft := pipeline.NewFastTiming(regFile, memory, table, syscallHandler)
				ft.SetPC(0x1000)

				// LDRB W0, [X1] => 0x39400020
				memory.Write32(0x1000, 0x39400020)
				memory.Write32(0x1004, 0xD4000001)
				memory.Write8(0x2000, 0xAB)
				regFile.WriteReg(1, 0x2000)
				regFile.WriteReg(8, 93)

				ft.Run()
				Expect(regFile.ReadReg(0)).To(Equal(uint64(0xAB)))
			})

			It("should execute STRB", func() {
				ft := pipeline.NewFastTiming(regFile, memory, table, syscallHandler)
				ft.SetPC(0x1000)

				// STRB W0, [X1] => 0x39000020
				memory.Write32(0x1000, 0x39000020)
				memory.Write32(0x1004, 0xD4000001)
				regFile.WriteReg(0, 0x42)
				regFile.WriteReg(1, 0x3000)
				regFile.WriteReg(8, 93)

				ft.Run()
				Expect(memory.Read8(0x3000)).To(Equal(uint8(0x42)))
			})

			It("should execute LDRH", func() {
				ft := pipeline.NewFastTiming(regFile, memory, table, syscallHandler)
				ft.SetPC(0x1000)

				// LDRH W0, [X1] => 0x79400020
				memory.Write32(0x1000, 0x79400020)
				memory.Write32(0x1004, 0xD4000001)
				memory.Write16(0x2000, 0xABCD)
				regFile.WriteReg(1, 0x2000)
				regFile.WriteReg(8, 93)

				ft.Run()
				Expect(regFile.ReadReg(0)).To(Equal(uint64(0xABCD)))
			})

			It("should execute STRH", func() {
				ft := pipeline.NewFastTiming(regFile, memory, table, syscallHandler)
				ft.SetPC(0x1000)

				// STRH W0, [X1] => 0x79000020
				memory.Write32(0x1000, 0x79000020)
				memory.Write32(0x1004, 0xD4000001)
				regFile.WriteReg(0, 0x1234)
				regFile.WriteReg(1, 0x3000)
				regFile.WriteReg(8, 93)

				ft.Run()
				Expect(memory.Read16(0x3000)).To(Equal(uint16(0x1234)))
			})
		})
	})

	Describe("MaxInstructions Limit", func() {
		It("should halt after maxInstructions", func() {
			ft := pipeline.NewFastTiming(regFile, memory, table, syscallHandler,
				pipeline.WithMaxInstructions(3))
			ft.SetPC(0x1000)

			// 5 ADD instructions
			memory.Write32(0x1000, 0x910029E0) // ADD X0, XZR, #10
			memory.Write32(0x1004, 0x910053E1) // ADD X1, XZR, #20
			memory.Write32(0x1008, 0x91007BE2) // ADD X2, XZR, #30
			memory.Write32(0x100C, 0x9100A3E3) // ADD X3, XZR, #40
			memory.Write32(0x1010, 0x9100CBE4) // ADD X4, XZR, #50

			exitCode := ft.Run()
			Expect(exitCode).To(Equal(int64(0)))

			stats := ft.Stats()
			Expect(stats.Instructions).To(Equal(uint64(3)))
			// X3 and X4 should not have been written
			Expect(regFile.ReadReg(3)).To(Equal(uint64(0)))
			Expect(regFile.ReadReg(4)).To(Equal(uint64(0)))
		})
	})

	Describe("Stats", func() {
		It("should track cycles and instructions", func() {
			ft := pipeline.NewFastTiming(regFile, memory, table, syscallHandler)
			ft.SetPC(0x1000)

			memory.Write32(0x1000, 0x910029E0) // ADD X0, XZR, #10
			memory.Write32(0x1004, 0x910053E1) // ADD X1, XZR, #20
			memory.Write32(0x1008, 0xD4000001) // SVC #0
			regFile.WriteReg(8, 93)

			ft.Run()
			stats := ft.Stats()
			Expect(stats.Instructions).To(BeNumerically(">", 0))
			Expect(stats.Cycles).To(BeNumerically(">", 0))
		})

		It("should return zero stalls and flushes (simplified model)", func() {
			ft := pipeline.NewFastTiming(regFile, memory, table, syscallHandler)
			ft.SetPC(0x1000)

			memory.Write32(0x1000, 0x910029E0)
			memory.Write32(0x1004, 0xD4000001)
			regFile.WriteReg(8, 93)

			ft.Run()
			stats := ft.Stats()
			Expect(stats.Stalls).To(Equal(uint64(0)))
			Expect(stats.Flushes).To(Equal(uint64(0)))
			Expect(stats.DataHazards).To(Equal(uint64(0)))
			Expect(stats.BranchPredictions).To(Equal(uint64(0)))
		})
	})

	Describe("UnhandledCount", func() {
		It("should initially be zero", func() {
			ft := pipeline.NewFastTiming(regFile, memory, table, syscallHandler)
			Expect(ft.UnhandledCount()).To(Equal(uint64(0)))
		})
	})

	Describe("Syscall handling", func() {
		It("should halt on exit syscall", func() {
			ft := pipeline.NewFastTiming(regFile, memory, table, syscallHandler)
			ft.SetPC(0x1000)

			memory.Write32(0x1000, 0xD4000001) // SVC #0
			regFile.WriteReg(8, 93)

			exitCode := ft.Run()
			Expect(exitCode).To(Equal(int64(0)))
		})

		It("should continue on non-exit syscall", func() {
			ft := pipeline.NewFastTiming(regFile, memory, table, syscallHandler)
			ft.SetPC(0x1000)

			// Non-exit syscall then exit syscall
			memory.Write32(0x1000, 0xD4000001) // SVC #0 (write syscall)
			regFile.WriteReg(8, 64)            // write syscall number

			// After the write syscall, modify X8 for exit
			// But our mock will return non-exited for non-93 syscalls
			// The fast timing will continue to PC+4
			memory.Write32(0x1004, 0xD2800BA8) // MOVZ X8, #93
			memory.Write32(0x1008, 0xD4000001) // SVC #0 (exit)

			exitCode := ft.Run()
			Expect(exitCode).To(Equal(int64(0)))
			Expect(syscallHandler.callCount).To(Equal(2))
		})

		It("should advance PC on nil syscall handler", func() {
			ft := pipeline.NewFastTiming(regFile, memory, table, nil,
				pipeline.WithMaxInstructions(3))
			ft.SetPC(0x1000)

			memory.Write32(0x1000, 0xD4000001) // SVC #0
			memory.Write32(0x1004, 0x910029E0) // ADD X0, XZR, #10
			memory.Write32(0x1008, 0x910053E1) // ADD X1, XZR, #20

			ft.Run()
			// Without a handler, SVC just advances PC
			Expect(regFile.ReadReg(0)).To(Equal(uint64(10)))
		})
	})

	Describe("Complete Program Execution", func() {
		It("should execute a simple sum loop", func() {
			ft := pipeline.NewFastTiming(regFile, memory, table, syscallHandler)
			ft.SetPC(0x1000)

			// X0 = accumulator (starts at 0)
			// X1 = 1 (constant increment for fast timing)
			regFile.WriteReg(0, 0)
			regFile.WriteReg(1, 1)
			regFile.WriteReg(8, 93)

			// sum = 0 + 1 + 1 + 1 = 3
			memory.Write32(0x1000, 0x8B010000) // ADD X0, X0, X1
			memory.Write32(0x1004, 0x8B010000) // ADD X0, X0, X1
			memory.Write32(0x1008, 0x8B010000) // ADD X0, X0, X1
			memory.Write32(0x100C, 0xD4000001) // SVC #0

			ft.Run()
			Expect(regFile.ReadReg(0)).To(Equal(uint64(3)))
		})

		It("should handle memory store-load sequence", func() {
			ft := pipeline.NewFastTiming(regFile, memory, table, syscallHandler)
			ft.SetPC(0x1000)

			regFile.WriteReg(1, 0x2000) // base addr
			regFile.WriteReg(2, 42)     // value to store
			regFile.WriteReg(8, 93)

			// STR X2, [X1] ; store 42 at 0x2000
			memory.Write32(0x1000, 0xF9000022)
			// LDR X0, [X1] ; load from 0x2000 into X0
			memory.Write32(0x1004, 0xF9400020)
			// SVC #0
			memory.Write32(0x1008, 0xD4000001)

			ft.Run()
			Expect(regFile.ReadReg(0)).To(Equal(uint64(42)))
			Expect(memory.Read64(0x2000)).To(Equal(uint64(42)))
		})
	})

	Describe("Latency accounting", func() {
		It("should account for multi-cycle latency in cycle count", func() {
			config := &latency.TimingConfig{
				ALULatency:              3, // 3-cycle ALU
				BranchLatency:           1,
				BranchMispredictPenalty: 14,
				LoadLatency:             4,
				StoreLatency:            1,
				MultiplyLatency:         3,
				DivideLatencyMin:        10,
				DivideLatencyMax:        15,
				SyscallLatency:          1,
			}
			table := latency.NewTableWithConfig(config)

			ft := pipeline.NewFastTiming(regFile, memory, table, syscallHandler)
			ft.SetPC(0x1000)

			// Single ADD with 3-cycle latency
			memory.Write32(0x1000, 0x91002820) // ADD X0, X1, #10
			memory.Write32(0x1004, 0xD4000001) // SVC #0
			regFile.WriteReg(1, 100)
			regFile.WriteReg(8, 93)

			ft.Run()
			stats := ft.Stats()

			// With 3-cycle ALU, cycles should be > instructions
			Expect(stats.Cycles).To(BeNumerically(">", stats.Instructions))
			Expect(regFile.ReadReg(0)).To(Equal(uint64(110)))
		})
	})

	Describe("Flag-setting instructions", func() {
		It("should execute ADDS and set zero flag", func() {
			ft := pipeline.NewFastTiming(regFile, memory, table, syscallHandler)
			ft.SetPC(0x1000)

			// ADDS X0, XZR, #0 (sets Z flag) => 0xB100001F
			// We need SUBS XZR, X1, X2 (CMP X1, X2) with X1=X2 to set Z
			// SUBS XZR, X0, X1 => 0xEB01001F
			memory.Write32(0x1000, 0xEB01001F) // SUBS XZR, X0, X1 (CMP X0, X1)
			memory.Write32(0x1004, 0x54000040) // B.EQ #8 (branch if equal)
			memory.Write32(0x1008, 0x910029E2) // ADD X2, XZR, #10 (skipped)
			memory.Write32(0x100C, 0xD4000001) // SVC #0
			regFile.WriteReg(0, 5)
			regFile.WriteReg(1, 5) // X0 == X1
			regFile.WriteReg(8, 93)

			ft.Run()
			// Branch should be taken since X0 == X1
			Expect(regFile.ReadReg(2)).To(Equal(uint64(0))) // X2 not written
		})
	})
})
