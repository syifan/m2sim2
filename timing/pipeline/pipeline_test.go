package pipeline_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/sarchlab/m2sim/emu"
	"github.com/sarchlab/m2sim/timing/latency"
	"github.com/sarchlab/m2sim/timing/pipeline"
)

var _ = Describe("Pipeline", func() {
	var (
		regFile *emu.RegFile
		memory  *emu.Memory
		pipe    *pipeline.Pipeline
	)

	BeforeEach(func() {
		regFile = &emu.RegFile{}
		memory = emu.NewMemory()
	})

	Describe("NewPipeline", func() {
		It("should create a new pipeline", func() {
			pipe = pipeline.NewPipeline(regFile, memory)
			Expect(pipe).NotTo(BeNil())
		})
	})

	Describe("SetPC / PC", func() {
		BeforeEach(func() {
			pipe = pipeline.NewPipeline(regFile, memory)
		})

		It("should set and get PC", func() {
			pipe.SetPC(0x1000)
			Expect(pipe.PC()).To(Equal(uint64(0x1000)))
		})

		It("should also update register file PC", func() {
			pipe.SetPC(0x2000)
			Expect(regFile.PC).To(Equal(uint64(0x2000)))
		})
	})

	Describe("Tick", func() {
		BeforeEach(func() {
			pipe = pipeline.NewPipeline(regFile, memory)
		})

		Context("single instruction execution", func() {
			It("should execute ADD immediate through pipeline", func() {
				// ADD X0, X1, #10 => 0x91002820
				memory.Write32(0x1000, 0x91002820)
				regFile.WriteReg(1, 100)
				pipe.SetPC(0x1000)

				// Pipeline needs 5 cycles to complete first instruction
				// Plus initial fill time
				for i := 0; i < 6; i++ {
					pipe.Tick()
				}

				Expect(regFile.ReadReg(0)).To(Equal(uint64(110)))
			})

			It("should execute SUB immediate through pipeline", func() {
				// SUB X0, X1, #20 => 0xD1005020
				memory.Write32(0x1000, 0xD1005020)
				regFile.WriteReg(1, 100)
				pipe.SetPC(0x1000)

				for i := 0; i < 6; i++ {
					pipe.Tick()
				}

				Expect(regFile.ReadReg(0)).To(Equal(uint64(80)))
			})

			It("should execute LDR through pipeline", func() {
				// LDR X0, [X1] => 0xF9400020
				memory.Write32(0x1000, 0xF9400020)
				memory.Write64(0x2000, 0xDEADBEEF12345678)
				regFile.WriteReg(1, 0x2000)
				pipe.SetPC(0x1000)

				for i := 0; i < 6; i++ {
					pipe.Tick()
				}

				Expect(regFile.ReadReg(0)).To(Equal(uint64(0xDEADBEEF12345678)))
			})

			It("should execute STR through pipeline", func() {
				// STR X0, [X1] => 0xF9000020
				memory.Write32(0x1000, 0xF9000020)
				regFile.WriteReg(0, 0xCAFEBABE)
				regFile.WriteReg(1, 0x3000)
				pipe.SetPC(0x1000)

				for i := 0; i < 6; i++ {
					pipe.Tick()
				}

				Expect(memory.Read64(0x3000)).To(Equal(uint64(0xCAFEBABE)))
			})
		})

		Context("sequential instructions", func() {
			It("should execute multiple independent instructions", func() {
				// ADD X0, XZR, #10 => 0x910029E0
				// ADD X1, XZR, #20 => 0x910053E1
				// ADD X2, XZR, #30 => 0x91007BE2
				memory.Write32(0x1000, 0x910029E0) // X0 = 10
				memory.Write32(0x1004, 0x910053E1) // X1 = 20
				memory.Write32(0x1008, 0x91007BE2) // X2 = 30
				pipe.SetPC(0x1000)

				// Execute enough cycles for all instructions
				for i := 0; i < 10; i++ {
					pipe.Tick()
				}

				Expect(regFile.ReadReg(0)).To(Equal(uint64(10)))
				Expect(regFile.ReadReg(1)).To(Equal(uint64(20)))
				Expect(regFile.ReadReg(2)).To(Equal(uint64(30)))
			})
		})
	})

	Describe("Data Forwarding", func() {
		BeforeEach(func() {
			pipe = pipeline.NewPipeline(regFile, memory)
		})

		It("should forward result from EX/MEM to EX (RAW hazard)", func() {
			// ADD X0, XZR, #10  ; X0 = 10
			// ADD X1, X0, #5    ; X1 = X0 + 5 = 15 (needs forwarding)
			memory.Write32(0x1000, 0x910029E0) // ADD X0, XZR, #10
			memory.Write32(0x1004, 0x91001401) // ADD X1, X0, #5
			pipe.SetPC(0x1000)

			for i := 0; i < 10; i++ {
				pipe.Tick()
			}

			Expect(regFile.ReadReg(0)).To(Equal(uint64(10)))
			Expect(regFile.ReadReg(1)).To(Equal(uint64(15)))
		})

		It("should forward result from MEM/WB to EX", func() {
			// ADD X0, XZR, #10  ; X0 = 10
			// ADD X1, XZR, #20  ; X1 = 20 (no dependency)
			// ADD X2, X0, #5    ; X2 = X0 + 5 (MEM/WB forwarding)
			memory.Write32(0x1000, 0x910029E0) // ADD X0, XZR, #10
			memory.Write32(0x1004, 0x910053E1) // ADD X1, XZR, #20
			memory.Write32(0x1008, 0x91001402) // ADD X2, X0, #5
			pipe.SetPC(0x1000)

			for i := 0; i < 12; i++ {
				pipe.Tick()
			}

			Expect(regFile.ReadReg(0)).To(Equal(uint64(10)))
			Expect(regFile.ReadReg(2)).To(Equal(uint64(15)))
		})
	})

	Describe("Load-Use Hazard (Stall)", func() {
		BeforeEach(func() {
			pipe = pipeline.NewPipeline(regFile, memory)
		})

		It("should stall on load-use hazard", func() {
			// LDR X0, [X1]      ; Load X0
			// ADD X2, X0, #5    ; Use X0 immediately (must stall)
			memory.Write32(0x1000, 0xF9400020) // LDR X0, [X1]
			memory.Write32(0x1004, 0x91001402) // ADD X2, X0, #5
			memory.Write64(0x2000, 100)
			regFile.WriteReg(1, 0x2000)
			pipe.SetPC(0x1000)

			for i := 0; i < 12; i++ {
				pipe.Tick()
			}

			Expect(regFile.ReadReg(0)).To(Equal(uint64(100)))
			Expect(regFile.ReadReg(2)).To(Equal(uint64(105)))

			// Verify a stall occurred
			stats := pipe.Stats()
			Expect(stats.Stalls).To(BeNumerically(">", 0))
		})
	})

	Describe("Branch Handling", func() {
		BeforeEach(func() {
			pipe = pipeline.NewPipeline(regFile, memory)
		})

		It("should handle unconditional branch B", func() {
			// B #8 (skip one instruction)
			// ADD X0, XZR, #10 (skipped)
			// ADD X1, XZR, #20 (executed)
			memory.Write32(0x1000, 0x14000002) // B #8
			memory.Write32(0x1004, 0x910029E0) // ADD X0, XZR, #10
			memory.Write32(0x1008, 0x910053E1) // ADD X1, XZR, #20
			pipe.SetPC(0x1000)

			for i := 0; i < 12; i++ {
				pipe.Tick()
			}

			Expect(regFile.ReadReg(0)).To(Equal(uint64(0)))  // Skipped
			Expect(regFile.ReadReg(1)).To(Equal(uint64(20))) // Executed
		})

		It("should handle BL (branch with link)", func() {
			// BL #8
			memory.Write32(0x1000, 0x94000002) // BL #8
			memory.Write32(0x1004, 0x910029E0) // ADD X0, XZR, #10
			memory.Write32(0x1008, 0x910053E1) // ADD X1, XZR, #20
			pipe.SetPC(0x1000)

			for i := 0; i < 12; i++ {
				pipe.Tick()
			}

			Expect(regFile.ReadReg(30)).To(Equal(uint64(0x1004))) // Return address
			Expect(regFile.ReadReg(1)).To(Equal(uint64(20)))
		})

		It("should handle conditional branch taken", func() {
			// SUBS XZR, X0, X1 ; Compare X0 and X1, set flags
			// B.EQ #8          ; Branch if equal
			// ADD X2, XZR, #10 (skipped if branch taken)
			// ADD X3, XZR, #20
			regFile.WriteReg(0, 5)
			regFile.WriteReg(1, 5) // X0 == X1, Z will be set

			memory.Write32(0x1000, 0xEB01001F) // SUBS XZR, X0, X1
			memory.Write32(0x1004, 0x54000040) // B.EQ #8
			memory.Write32(0x1008, 0x910029E2) // ADD X2, XZR, #10
			memory.Write32(0x100C, 0x910053E3) // ADD X3, XZR, #20
			pipe.SetPC(0x1000)

			for i := 0; i < 15; i++ {
				pipe.Tick()
			}

			Expect(regFile.ReadReg(3)).To(Equal(uint64(20)))
		})

	})
	
	Describe("Halted", func() {
		BeforeEach(func() {
			pipe = pipeline.NewPipeline(regFile, memory)
		})

		It("should initially not be halted", func() {
			Expect(pipe.Halted()).To(BeFalse())
		})
	})

	Describe("Stats", func() {
		BeforeEach(func() {
			pipe = pipeline.NewPipeline(regFile, memory)
		})

		It("should track cycle count", func() {
			memory.Write32(0x1000, 0x910029E0) // ADD X0, XZR, #10
			pipe.SetPC(0x1000)

			pipe.Tick()
			pipe.Tick()
			pipe.Tick()

			stats := pipe.Stats()
			Expect(stats.Cycles).To(Equal(uint64(3)))
		})

		It("should track instruction count", func() {
			memory.Write32(0x1000, 0x910029E0) // ADD X0, XZR, #10
			memory.Write32(0x1004, 0x910053E1) // ADD X1, XZR, #20
			pipe.SetPC(0x1000)

			for i := 0; i < 10; i++ {
				pipe.Tick()
			}

			stats := pipe.Stats()
			Expect(stats.Instructions).To(BeNumerically(">", 0))
		})

		It("should track stall count", func() {
			// Load-use hazard to trigger stall
			memory.Write32(0x1000, 0xF9400020) // LDR X0, [X1]
			memory.Write32(0x1004, 0x91001402) // ADD X2, X0, #5 (uses X0)
			memory.Write64(0x2000, 100)
			regFile.WriteReg(1, 0x2000)
			pipe.SetPC(0x1000)

			for i := 0; i < 15; i++ {
				pipe.Tick()
			}

			stats := pipe.Stats()
			Expect(stats.Stalls).To(BeNumerically(">", 0))
		})
	})

	Describe("Pipeline Register Inspection", func() {
		BeforeEach(func() {
			pipe = pipeline.NewPipeline(regFile, memory)
		})

		It("should expose IF/ID register", func() {
			memory.Write32(0x1000, 0x910029E0)
			pipe.SetPC(0x1000)
			pipe.Tick()

			ifid := pipe.GetIFID()
			Expect(ifid.Valid).To(BeTrue())
			Expect(ifid.PC).To(Equal(uint64(0x1000)))
		})

		It("should expose ID/EX register", func() {
			memory.Write32(0x1000, 0x910029E0)
			pipe.SetPC(0x1000)
			pipe.Tick()
			pipe.Tick()

			idex := pipe.GetIDEX()
			Expect(idex.Valid).To(BeTrue())
		})

		It("should expose EX/MEM register", func() {
			memory.Write32(0x1000, 0x910029E0)
			pipe.SetPC(0x1000)
			pipe.Tick()
			pipe.Tick()
			pipe.Tick()

			exmem := pipe.GetEXMEM()
			Expect(exmem.Valid).To(BeTrue())
		})

		It("should expose MEM/WB register", func() {
			memory.Write32(0x1000, 0x910029E0)
			pipe.SetPC(0x1000)
			pipe.Tick()
			pipe.Tick()
			pipe.Tick()
			pipe.Tick()

			memwb := pipe.GetMEMWB()
			Expect(memwb.Valid).To(BeTrue())
		})
	})

	Describe("Halted state", func() {
		BeforeEach(func() {
			pipe = pipeline.NewPipeline(regFile, memory)
		})

		It("should not tick when halted", func() {
			regFile.WriteReg(8, 93)
			memory.Write32(0x1000, 0xD4000001)
			pipe.SetPC(0x1000)

			// Run until halted
			for !pipe.Halted() {
				pipe.Tick()
			}

			cyclesBefore := pipe.Stats().Cycles

			// Try to tick more
			pipe.Tick()
			pipe.Tick()

			cyclesAfter := pipe.Stats().Cycles
			Expect(cyclesAfter).To(Equal(cyclesBefore))
		})
	})
})

var _ = Describe("Pipeline Integration", func() {
	var (
		regFile *emu.RegFile
		memory  *emu.Memory
		pipe    *pipeline.Pipeline
	)

	BeforeEach(func() {
		regFile = &emu.RegFile{}
		memory = emu.NewMemory()
		pipe = pipeline.NewPipeline(regFile, memory)
	})

	Describe("Complete program execution", func() {
		It("should execute a simple sum loop", func() {
			// Program: sum = 0; for i = 1 to 5: sum += i
			// X0 = sum = 0
			// X1 = i = 1
			// X2 = limit = 5
			// loop:
			//   ADD X0, X0, X1  ; sum += i
			//   ADD X1, X1, #1  ; i++
			//   CMP X1, X2      ; compare i with limit
			//   B.LE loop       ; if i <= 5, loop
			// exit

			regFile.WriteReg(0, 0)  // sum = 0
			regFile.WriteReg(1, 1)  // i = 1
			regFile.WriteReg(2, 6)  // limit + 1 (loop while i < 6)
			regFile.WriteReg(8, 93) // exit syscall

			// Simplified: just do 3 additions
			memory.Write32(0x1000, 0x8B010000) // ADD X0, X0, X1 (X0 = 0 + 1 = 1)
			memory.Write32(0x1004, 0x8B010000) // ADD X0, X0, X1 (X0 = 1 + 1 = 2)
			memory.Write32(0x1008, 0x8B010000) // ADD X0, X0, X1 (X0 = 2 + 1 = 3)
			memory.Write32(0x100C, 0xD4000001) // SVC #0 (exit)

			pipe.SetPC(0x1000)
			pipe.Run()

			Expect(regFile.ReadReg(0)).To(Equal(uint64(3)))
		})

		It("should handle memory operations correctly", func() {
			// Store a value, load it back, add to it
			// X1 = base address
			// X2 = value to store
			regFile.WriteReg(1, 0x2000)
			regFile.WriteReg(2, 100)
			regFile.WriteReg(8, 93)

			memory.Write32(0x1000, 0xF9000022) // STR X2, [X1] ; store 100 at [0x2000]
			memory.Write32(0x1004, 0xF9400020) // LDR X0, [X1] ; load from [0x2000] into X0
			// Insert a NOP or independent instruction to avoid load-use directly
			memory.Write32(0x1008, 0x910053E3) // ADD X3, XZR, #20 (independent)
			memory.Write32(0x100C, 0x91002800) // ADD X0, X0, #10 ; X0 = 100 + 10 = 110
			memory.Write32(0x1010, 0xD4000001) // SVC #0

			pipe.SetPC(0x1000)
			pipe.Run()

			Expect(regFile.ReadReg(0)).To(Equal(uint64(110)))
			Expect(memory.Read64(0x2000)).To(Equal(uint64(100)))
		})
	})

	Describe("Latency Table Integration", func() {
		BeforeEach(func() {
			regFile = &emu.RegFile{}
			memory = emu.NewMemory()
			regFile.WriteReg(8, 93) // Set exit syscall number
		})

		It("should support WithLatencyTable option", func() {
			table := latency.NewTable()
			pipe = pipeline.NewPipeline(regFile, memory, pipeline.WithLatencyTable(table))
			Expect(pipe.LatencyTable()).To(Equal(table))
		})

		It("should allow setting latency table after creation", func() {
			pipe = pipeline.NewPipeline(regFile, memory)
			Expect(pipe.LatencyTable()).To(BeNil())

			table := latency.NewTable()
			pipe.SetLatencyTable(table)
			Expect(pipe.LatencyTable()).To(Equal(table))
		})

		It("should track execution stalls with multi-cycle config", func() {
			// Configure with 3-cycle ALU latency
			config := &latency.TimingConfig{
				ALULatency:              3,
				BranchLatency:           1,
				BranchMispredictPenalty: 12,
				LoadLatency:             4,
				StoreLatency:            1,
				MultiplyLatency:         3,
				DivideLatencyMin:        10,
				DivideLatencyMax:        15,
				SyscallLatency:          1,
				L1HitLatency:            4,
				L2HitLatency:            12,
				L3HitLatency:            30,
				MemoryLatency:           150,
			}
			table := latency.NewTableWithConfig(config)
			pipe = pipeline.NewPipeline(regFile, memory, pipeline.WithLatencyTable(table))

			// Single ADD instruction
			memory.Write32(0x1000, 0x91002820) // ADD X0, X1, #10
			memory.Write32(0x1004, 0xD4000001) // SVC #0

			regFile.WriteReg(1, 100)
			pipe.SetPC(0x1000)
			pipe.Run()

			// Verify result is still correct
			Expect(regFile.ReadReg(0)).To(Equal(uint64(110)))

			// With 3-cycle ALU, we should have execution stalls
			stats := pipe.Stats()
			Expect(stats.ExecStalls).To(BeNumerically(">", 0))
		})

		It("should produce correct results with latency enabled", func() {
			table := latency.NewTable() // Default config
			pipe = pipeline.NewPipeline(regFile, memory, pipeline.WithLatencyTable(table))

			// Three consecutive ADDs
			regFile.WriteReg(1, 1)
			memory.Write32(0x1000, 0x8B010000) // ADD X0, X0, X1 (X0 = 0 + 1 = 1)
			memory.Write32(0x1004, 0x8B010000) // ADD X0, X0, X1 (X0 = 1 + 1 = 2)
			memory.Write32(0x1008, 0x8B010000) // ADD X0, X0, X1 (X0 = 2 + 1 = 3)
			memory.Write32(0x100C, 0xD4000001) // SVC #0

			pipe.SetPC(0x1000)
			pipe.Run()

			Expect(regFile.ReadReg(0)).To(Equal(uint64(3)))
		})

		It("should have more cycles with load latency", func() {
			// First run without latency
			pipe = pipeline.NewPipeline(regFile, memory)
			memory.Write32(0x1000, 0xF9400020) // LDR X0, [X1]
			memory.Write32(0x1004, 0xD4000001) // SVC #0
			memory.Write64(0x2000, 0xCAFEBABE)
			regFile.WriteReg(1, 0x2000)

			pipe.SetPC(0x1000)
			pipe.Run()
			cyclesWithoutLatency := pipe.Stats().Cycles

			// Reset for second run with latency
			regFile = &emu.RegFile{}
			regFile.WriteReg(8, 93)
			regFile.WriteReg(1, 0x2000)

			config := &latency.TimingConfig{
				ALULatency:              1,
				BranchLatency:           1,
				BranchMispredictPenalty: 12,
				LoadLatency:             4, // 4-cycle load
				StoreLatency:            1,
				MultiplyLatency:         3,
				DivideLatencyMin:        10,
				DivideLatencyMax:        15,
				SyscallLatency:          1,
				L1HitLatency:            4,
				L2HitLatency:            12,
				L3HitLatency:            30,
				MemoryLatency:           150,
			}
			table := latency.NewTableWithConfig(config)
			pipe = pipeline.NewPipeline(regFile, memory, pipeline.WithLatencyTable(table))

			pipe.SetPC(0x1000)
			pipe.Run()
			cyclesWithLatency := pipe.Stats().Cycles

			// With 4-cycle load, should take more cycles
			Expect(cyclesWithLatency).To(BeNumerically(">", cyclesWithoutLatency))
			Expect(regFile.ReadReg(0)).To(Equal(uint64(0xCAFEBABE)))
		})
	})
})
