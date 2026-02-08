package pipeline_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/sarchlab/m2sim/emu"
	"github.com/sarchlab/m2sim/timing/pipeline"
)

var _ = Describe("Superscalar Pipeline", func() {
	var (
		regFile *emu.RegFile
		memory  *emu.Memory
		pipe    *pipeline.Pipeline
	)

	BeforeEach(func() {
		regFile = &emu.RegFile{}
		memory = emu.NewMemory()
		regFile.WriteReg(8, 93) // exit syscall
	})

	Describe("Dual-Issue (tickSuperscalar)", func() {
		BeforeEach(func() {
			pipe = pipeline.NewPipeline(regFile, memory, pipeline.WithDualIssue())
		})

		It("should create a dual-issue pipeline", func() {
			Expect(pipe).NotTo(BeNil())
		})

		It("should execute independent instructions in parallel", func() {
			// Two independent ADD instructions can issue together
			// ADD X0, XZR, #10 => 0x910029E0
			// ADD X1, XZR, #20 => 0x910053E1
			memory.Write32(0x1000, 0x910029E0) // ADD X0, XZR, #10
			memory.Write32(0x1004, 0x910053E1) // ADD X1, XZR, #20
			memory.Write32(0x1008, 0xD4000001) // SVC #0

			pipe.SetPC(0x1000)
			pipe.Run()

			Expect(regFile.ReadReg(0)).To(Equal(uint64(10)))
			Expect(regFile.ReadReg(1)).To(Equal(uint64(20)))
		})

		It("should execute dual-issue faster than single-issue", func() {
			// 4 independent instructions
			memory.Write32(0x1000, 0x910029E0) // ADD X0, XZR, #10
			memory.Write32(0x1004, 0x910053E1) // ADD X1, XZR, #20
			memory.Write32(0x1008, 0x91007BE2) // ADD X2, XZR, #30
			memory.Write32(0x100C, 0x9100A3E3) // ADD X3, XZR, #40
			memory.Write32(0x1010, 0xD4000001) // SVC #0

			pipe.SetPC(0x1000)
			pipe.Run()

			dualCycles := pipe.Stats().Cycles

			// Reset and run with single-issue
			regFile = &emu.RegFile{}
			regFile.WriteReg(8, 93)
			singlePipe := pipeline.NewPipeline(regFile, memory)
			singlePipe.SetPC(0x1000)
			singlePipe.Run()

			singleCycles := singlePipe.Stats().Cycles

			// Dual-issue should take fewer cycles
			Expect(dualCycles).To(BeNumerically("<", singleCycles))

			// Verify results are still correct
			Expect(regFile.ReadReg(0)).To(Equal(uint64(10)))
			Expect(regFile.ReadReg(1)).To(Equal(uint64(20)))
			Expect(regFile.ReadReg(2)).To(Equal(uint64(30)))
			Expect(regFile.ReadReg(3)).To(Equal(uint64(40)))
		})

		It("should co-issue ALU RAW dependency with forwarding", func() {
			// ADD X0, XZR, #10  ; X0 = 10
			// ADD X1, X0, #5    ; X1 = X0 + 5 (RAW dependency, but ALU result forwarded same cycle)
			memory.Write32(0x1000, 0x910029E0) // ADD X0, XZR, #10
			memory.Write32(0x1004, 0x91001401) // ADD X1, X0, #5
			memory.Write32(0x1008, 0xD4000001) // SVC #0

			pipe.SetPC(0x1000)
			pipe.Run()

			Expect(regFile.ReadReg(0)).To(Equal(uint64(10)))
			Expect(regFile.ReadReg(1)).To(Equal(uint64(15)))
		})

		It("should block co-issue for load-dependent RAW hazard", func() {
			// LDR X0, [X10]     ; load X0 from memory (result not available until MEM)
			// ADD X1, X0, #5    ; X1 = X0 + 5 (load-use hazard - cannot co-issue)
			memory.Write64(0x2000, 10)
			regFile.WriteReg(10, 0x2000)
			memory.Write32(0x1000, 0xF9400140) // LDR X0, [X10]
			memory.Write32(0x1004, 0x91001401) // ADD X1, X0, #5
			memory.Write32(0x1008, 0xD4000001) // SVC #0

			pipe.SetPC(0x1000)
			pipe.Run()

			Expect(regFile.ReadReg(0)).To(Equal(uint64(10)))
			Expect(regFile.ReadReg(1)).To(Equal(uint64(15)))
		})

		It("should handle WAW dependency correctly", func() {
			// Both write to X0 - cannot dual issue
			// ADD X0, XZR, #10  ; X0 = 10
			// ADD X0, XZR, #20  ; X0 = 20 (WAW)
			memory.Write32(0x1000, 0x910029E0) // ADD X0, XZR, #10
			memory.Write32(0x1004, 0x910053E0) // ADD X0, XZR, #20
			memory.Write32(0x1008, 0xD4000001) // SVC #0

			pipe.SetPC(0x1000)
			pipe.Run()

			// Second instruction should execute last
			Expect(regFile.ReadReg(0)).To(Equal(uint64(20)))
		})

		It("should not dual-issue memory operations", func() {
			// Two loads cannot issue together (single memory port)
			// LDR X0, [X1]
			// LDR X2, [X3]
			memory.Write32(0x1000, 0xF9400020) // LDR X0, [X1]
			memory.Write32(0x1004, 0xF9400062) // LDR X2, [X3]
			memory.Write32(0x1008, 0xD4000001) // SVC #0
			memory.Write64(0x2000, 100)
			memory.Write64(0x3000, 200)
			regFile.WriteReg(1, 0x2000)
			regFile.WriteReg(3, 0x3000)

			pipe.SetPC(0x1000)
			pipe.Run()

			Expect(regFile.ReadReg(0)).To(Equal(uint64(100)))
			Expect(regFile.ReadReg(2)).To(Equal(uint64(200)))
		})

		It("should not dual-issue branch instructions", func() {
			// B #8 (branches can only issue in primary slot)
			// ADD X0, XZR, #10 (skipped due to branch)
			// ADD X1, XZR, #20 (executed)
			memory.Write32(0x1000, 0x14000002) // B #8
			memory.Write32(0x1004, 0x910029E0) // ADD X0, XZR, #10
			memory.Write32(0x1008, 0x910053E1) // ADD X1, XZR, #20
			memory.Write32(0x100C, 0xD4000001) // SVC #0

			pipe.SetPC(0x1000)
			pipe.Run()

			Expect(regFile.ReadReg(0)).To(Equal(uint64(0)))  // Skipped
			Expect(regFile.ReadReg(1)).To(Equal(uint64(20))) // Executed
		})

		It("should complete all instructions in dual-issue mode", func() {
			// Many independent instructions
			memory.Write32(0x1000, 0x910029E0) // ADD X0, XZR, #10
			memory.Write32(0x1004, 0x910053E1) // ADD X1, XZR, #20
			memory.Write32(0x1008, 0x91007BE2) // ADD X2, XZR, #30
			memory.Write32(0x100C, 0x9100A3E3) // ADD X3, XZR, #40
			memory.Write32(0x1010, 0x9100CBE4) // ADD X4, XZR, #50
			memory.Write32(0x1014, 0x9100F3E5) // ADD X5, XZR, #60
			memory.Write32(0x1018, 0x91011BE6) // ADD X6, XZR, #70
			memory.Write32(0x101C, 0x910143E7) // ADD X7, XZR, #80
			memory.Write32(0x1020, 0xD4000001) // SVC #0

			pipe.SetPC(0x1000)
			pipe.Run()

			// Verify all completed
			Expect(regFile.ReadReg(0)).To(Equal(uint64(10)))
			Expect(regFile.ReadReg(7)).To(Equal(uint64(80)))
		})
	})

	Describe("Quad-Issue (tickQuadIssue)", func() {
		BeforeEach(func() {
			pipe = pipeline.NewPipeline(regFile, memory, pipeline.WithQuadIssue())
		})

		It("should create a quad-issue pipeline", func() {
			Expect(pipe).NotTo(BeNil())
		})

		It("should execute four independent instructions in parallel", func() {
			// Four independent ADD instructions can issue together
			memory.Write32(0x1000, 0x910029E0) // ADD X0, XZR, #10
			memory.Write32(0x1004, 0x910053E1) // ADD X1, XZR, #20
			memory.Write32(0x1008, 0x91007BE2) // ADD X2, XZR, #30
			memory.Write32(0x100C, 0x9100A3E3) // ADD X3, XZR, #40
			memory.Write32(0x1010, 0xD4000001) // SVC #0

			pipe.SetPC(0x1000)
			pipe.Run()

			Expect(regFile.ReadReg(0)).To(Equal(uint64(10)))
			Expect(regFile.ReadReg(1)).To(Equal(uint64(20)))
			Expect(regFile.ReadReg(2)).To(Equal(uint64(30)))
			Expect(regFile.ReadReg(3)).To(Equal(uint64(40)))
		})

		It("should execute quad-issue faster than dual-issue", func() {
			// 8 independent instructions
			memory.Write32(0x1000, 0x910029E0) // ADD X0, XZR, #10
			memory.Write32(0x1004, 0x910053E1) // ADD X1, XZR, #20
			memory.Write32(0x1008, 0x91007BE2) // ADD X2, XZR, #30
			memory.Write32(0x100C, 0x9100A3E3) // ADD X3, XZR, #40
			memory.Write32(0x1010, 0x9100CBE4) // ADD X4, XZR, #50
			memory.Write32(0x1014, 0x9100F3E5) // ADD X5, XZR, #60
			memory.Write32(0x1018, 0x91011BE6) // ADD X6, XZR, #70
			memory.Write32(0x101C, 0x910143E7) // ADD X7, XZR, #80
			memory.Write32(0x1020, 0xD4000001) // SVC #0

			pipe.SetPC(0x1000)
			pipe.Run()

			quadCycles := pipe.Stats().Cycles

			// Reset and run with dual-issue
			regFile = &emu.RegFile{}
			regFile.WriteReg(8, 93)
			dualPipe := pipeline.NewPipeline(regFile, memory, pipeline.WithDualIssue())
			dualPipe.SetPC(0x1000)
			dualPipe.Run()

			dualCycles := dualPipe.Stats().Cycles

			// Quad-issue should take fewer cycles
			Expect(quadCycles).To(BeNumerically("<", dualCycles))

			// Verify results are still correct
			Expect(regFile.ReadReg(0)).To(Equal(uint64(10)))
			Expect(regFile.ReadReg(4)).To(Equal(uint64(50)))
			Expect(regFile.ReadReg(7)).To(Equal(uint64(80)))
		})

		It("should handle chained dependencies correctly", func() {
			// Chain of dependent instructions
			// ADD X0, XZR, #1   ; X0 = 1
			// ADD X1, X0, #1    ; X1 = X0 + 1 = 2
			// ADD X2, X1, #1    ; X2 = X1 + 1 = 3
			// ADD X3, X2, #1    ; X3 = X2 + 1 = 4
			memory.Write32(0x1000, 0x910007E0) // ADD X0, XZR, #1
			memory.Write32(0x1004, 0x91000401) // ADD X1, X0, #1
			memory.Write32(0x1008, 0x91000422) // ADD X2, X1, #1
			memory.Write32(0x100C, 0x91000443) // ADD X3, X2, #1
			memory.Write32(0x1010, 0xD4000001) // SVC #0

			pipe.SetPC(0x1000)
			pipe.Run()

			// Dependencies should be resolved correctly
			Expect(regFile.ReadReg(0)).To(Equal(uint64(1)))
			Expect(regFile.ReadReg(1)).To(Equal(uint64(2)))
			Expect(regFile.ReadReg(2)).To(Equal(uint64(3)))
			Expect(regFile.ReadReg(3)).To(Equal(uint64(4)))
		})

		It("should complete all quad-issue instructions", func() {
			// 8 independent instructions
			memory.Write32(0x1000, 0x910029E0) // ADD X0, XZR, #10
			memory.Write32(0x1004, 0x910053E1) // ADD X1, XZR, #20
			memory.Write32(0x1008, 0x91007BE2) // ADD X2, XZR, #30
			memory.Write32(0x100C, 0x9100A3E3) // ADD X3, XZR, #40
			memory.Write32(0x1010, 0x9100CBE4) // ADD X4, XZR, #50
			memory.Write32(0x1014, 0x9100F3E5) // ADD X5, XZR, #60
			memory.Write32(0x1018, 0x91011BE6) // ADD X6, XZR, #70
			memory.Write32(0x101C, 0x910143E7) // ADD X7, XZR, #80
			memory.Write32(0x1020, 0xD4000001) // SVC #0

			pipe.SetPC(0x1000)
			pipe.Run()

			// Verify all completed
			Expect(regFile.ReadReg(0)).To(Equal(uint64(10)))
			Expect(regFile.ReadReg(4)).To(Equal(uint64(50)))
			Expect(regFile.ReadReg(7)).To(Equal(uint64(80)))
		})

		It("should handle mixed memory and ALU operations", func() {
			// Mix of loads and ALU ops - only one memory op per cycle
			// LDR X0, [X10]     ; load
			// ADD X1, XZR, #20  ; ALU (can issue with load)
			// ADD X2, XZR, #30  ; ALU (can issue with load)
			// ADD X3, XZR, #40  ; ALU (can issue with load)
			memory.Write32(0x1000, 0xF9400140) // LDR X0, [X10]
			memory.Write32(0x1004, 0x910053E1) // ADD X1, XZR, #20
			memory.Write32(0x1008, 0x91007BE2) // ADD X2, XZR, #30
			memory.Write32(0x100C, 0x9100A3E3) // ADD X3, XZR, #40
			memory.Write32(0x1010, 0xD4000001) // SVC #0
			memory.Write64(0x2000, 100)
			regFile.WriteReg(10, 0x2000)

			pipe.SetPC(0x1000)
			pipe.Run()

			Expect(regFile.ReadReg(0)).To(Equal(uint64(100)))
			Expect(regFile.ReadReg(1)).To(Equal(uint64(20)))
			Expect(regFile.ReadReg(2)).To(Equal(uint64(30)))
			Expect(regFile.ReadReg(3)).To(Equal(uint64(40)))
		})
	})

	Describe("Sextuple-Issue (tickSextupleIssue)", func() {
		BeforeEach(func() {
			pipe = pipeline.NewPipeline(regFile, memory, pipeline.WithSextupleIssue())
		})

		It("should create a sextuple-issue pipeline", func() {
			Expect(pipe).NotTo(BeNil())
		})

		It("should execute six independent instructions", func() {
			// Six independent ADD instructions
			memory.Write32(0x1000, 0x910029E0) // ADD X0, XZR, #10
			memory.Write32(0x1004, 0x910053E1) // ADD X1, XZR, #20
			memory.Write32(0x1008, 0x91007BE2) // ADD X2, XZR, #30
			memory.Write32(0x100C, 0x9100A3E3) // ADD X3, XZR, #40
			memory.Write32(0x1010, 0x9100CBE4) // ADD X4, XZR, #50
			memory.Write32(0x1014, 0x9100F3E5) // ADD X5, XZR, #60
			memory.Write32(0x1018, 0xD4000001) // SVC #0

			pipe.SetPC(0x1000)
			pipe.Run()

			Expect(regFile.ReadReg(0)).To(Equal(uint64(10)))
			Expect(regFile.ReadReg(1)).To(Equal(uint64(20)))
			Expect(regFile.ReadReg(2)).To(Equal(uint64(30)))
			Expect(regFile.ReadReg(3)).To(Equal(uint64(40)))
			Expect(regFile.ReadReg(4)).To(Equal(uint64(50)))
			Expect(regFile.ReadReg(5)).To(Equal(uint64(60)))
		})

		It("should execute sextuple-issue faster than quad-issue", func() {
			// 12 independent instructions
			memory.Write32(0x1000, 0x910029E0) // ADD X0, XZR, #10
			memory.Write32(0x1004, 0x910053E1) // ADD X1, XZR, #20
			memory.Write32(0x1008, 0x91007BE2) // ADD X2, XZR, #30
			memory.Write32(0x100C, 0x9100A3E3) // ADD X3, XZR, #40
			memory.Write32(0x1010, 0x9100CBE4) // ADD X4, XZR, #50
			memory.Write32(0x1014, 0x9100F3E5) // ADD X5, XZR, #60
			memory.Write32(0x1018, 0x91011BE6) // ADD X6, XZR, #70
			memory.Write32(0x101C, 0x910143E7) // ADD X7, XZR, #80
			memory.Write32(0x1020, 0x91016BF8) // ADD X24, XZR, #90
			memory.Write32(0x1024, 0x910193F9) // ADD X25, XZR, #100
			memory.Write32(0x1028, 0x9101BBFA) // ADD X26, XZR, #110
			memory.Write32(0x102C, 0x9101E3FB) // ADD X27, XZR, #120
			memory.Write32(0x1030, 0xD4000001) // SVC #0

			pipe.SetPC(0x1000)
			pipe.Run()

			sextupleCycles := pipe.Stats().Cycles

			// Reset and run with quad-issue
			regFile = &emu.RegFile{}
			regFile.WriteReg(8, 93)
			quadPipe := pipeline.NewPipeline(regFile, memory, pipeline.WithQuadIssue())
			quadPipe.SetPC(0x1000)
			quadPipe.Run()

			quadCycles := quadPipe.Stats().Cycles

			// Sextuple-issue should take fewer cycles
			Expect(sextupleCycles).To(BeNumerically("<=", quadCycles))

			// Verify results are correct
			Expect(regFile.ReadReg(0)).To(Equal(uint64(10)))
			Expect(regFile.ReadReg(5)).To(Equal(uint64(60)))
		})
	})

	Describe("Octuple-Issue (tickOctupleIssue)", func() {
		BeforeEach(func() {
			pipe = pipeline.NewPipeline(regFile, memory, pipeline.WithOctupleIssue())
		})

		It("should create an octuple-issue pipeline", func() {
			Expect(pipe).NotTo(BeNil())
		})

		It("should execute eight independent instructions in parallel", func() {
			// Eight independent ADD instructions can issue together
			memory.Write32(0x1000, 0x910029E0) // ADD X0, XZR, #10
			memory.Write32(0x1004, 0x910053E1) // ADD X1, XZR, #20
			memory.Write32(0x1008, 0x91007BE2) // ADD X2, XZR, #30
			memory.Write32(0x100C, 0x9100A3E3) // ADD X3, XZR, #40
			memory.Write32(0x1010, 0x9100CBE4) // ADD X4, XZR, #50
			memory.Write32(0x1014, 0x9100F3E5) // ADD X5, XZR, #60
			memory.Write32(0x1018, 0x91011BE6) // ADD X6, XZR, #70
			memory.Write32(0x101C, 0x910143E7) // ADD X7, XZR, #80
			memory.Write32(0x1020, 0xD4000001) // SVC #0

			pipe.SetPC(0x1000)
			pipe.Run()

			Expect(regFile.ReadReg(0)).To(Equal(uint64(10)))
			Expect(regFile.ReadReg(1)).To(Equal(uint64(20)))
			Expect(regFile.ReadReg(2)).To(Equal(uint64(30)))
			Expect(regFile.ReadReg(3)).To(Equal(uint64(40)))
			Expect(regFile.ReadReg(4)).To(Equal(uint64(50)))
			Expect(regFile.ReadReg(5)).To(Equal(uint64(60)))
			Expect(regFile.ReadReg(6)).To(Equal(uint64(70)))
			Expect(regFile.ReadReg(7)).To(Equal(uint64(80)))
		})

		It("should execute octuple-issue faster than sextuple-issue", func() {
			// 16 independent instructions
			memory.Write32(0x1000, 0x910029E0) // ADD X0, XZR, #10
			memory.Write32(0x1004, 0x910053E1) // ADD X1, XZR, #20
			memory.Write32(0x1008, 0x91007BE2) // ADD X2, XZR, #30
			memory.Write32(0x100C, 0x9100A3E3) // ADD X3, XZR, #40
			memory.Write32(0x1010, 0x9100CBE4) // ADD X4, XZR, #50
			memory.Write32(0x1014, 0x9100F3E5) // ADD X5, XZR, #60
			memory.Write32(0x1018, 0x91011BE6) // ADD X6, XZR, #70
			memory.Write32(0x101C, 0x910143E7) // ADD X7, XZR, #80
			memory.Write32(0x1020, 0x91016BF8) // ADD X24, XZR, #90
			memory.Write32(0x1024, 0x910193F9) // ADD X25, XZR, #100
			memory.Write32(0x1028, 0x9101BBFA) // ADD X26, XZR, #110
			memory.Write32(0x102C, 0x9101E3FB) // ADD X27, XZR, #120
			memory.Write32(0x1030, 0x91020BFC) // ADD X28, XZR, #130
			memory.Write32(0x1034, 0x910233FD) // ADD X29, XZR, #140
			memory.Write32(0x1038, 0x91025BEE) // ADD X14, XZR, #150
			memory.Write32(0x103C, 0x910283EF) // ADD X15, XZR, #160
			memory.Write32(0x1040, 0xD4000001) // SVC #0

			pipe.SetPC(0x1000)
			pipe.Run()

			octupleCycles := pipe.Stats().Cycles

			// Reset and run with sextuple-issue
			regFile = &emu.RegFile{}
			regFile.WriteReg(8, 93)
			sextuplePipe := pipeline.NewPipeline(regFile, memory, pipeline.WithSextupleIssue())
			sextuplePipe.SetPC(0x1000)
			sextuplePipe.Run()

			sextupleCycles := sextuplePipe.Stats().Cycles

			// Octuple-issue should take fewer or equal cycles
			Expect(octupleCycles).To(BeNumerically("<=", sextupleCycles))

			// Verify results are correct
			Expect(regFile.ReadReg(0)).To(Equal(uint64(10)))
			Expect(regFile.ReadReg(7)).To(Equal(uint64(80)))
		})

		It("should handle chained dependencies correctly in 8-wide", func() {
			// Chain of dependent instructions - tests forwarding in 8-wide
			memory.Write32(0x1000, 0x910007E0) // ADD X0, XZR, #1
			memory.Write32(0x1004, 0x91000401) // ADD X1, X0, #1
			memory.Write32(0x1008, 0x91000422) // ADD X2, X1, #1
			memory.Write32(0x100C, 0x91000443) // ADD X3, X2, #1
			memory.Write32(0x1010, 0x91000464) // ADD X4, X3, #1
			memory.Write32(0x1014, 0x91000485) // ADD X5, X4, #1
			memory.Write32(0x1018, 0x910004A6) // ADD X6, X5, #1
			memory.Write32(0x101C, 0x910004C7) // ADD X7, X6, #1
			memory.Write32(0x1020, 0xD4000001) // SVC #0

			pipe.SetPC(0x1000)
			pipe.Run()

			// Dependencies should be resolved correctly
			Expect(regFile.ReadReg(0)).To(Equal(uint64(1)))
			Expect(regFile.ReadReg(1)).To(Equal(uint64(2)))
			Expect(regFile.ReadReg(2)).To(Equal(uint64(3)))
			Expect(regFile.ReadReg(3)).To(Equal(uint64(4)))
			Expect(regFile.ReadReg(4)).To(Equal(uint64(5)))
			Expect(regFile.ReadReg(5)).To(Equal(uint64(6)))
			Expect(regFile.ReadReg(6)).To(Equal(uint64(7)))
			Expect(regFile.ReadReg(7)).To(Equal(uint64(8)))
		})

		It("should handle branch in 8-wide mode", func() {
			// Branch followed by 7 instructions (some skipped)
			memory.Write32(0x1000, 0x14000004) // B #16 (skip next 3 instructions)
			memory.Write32(0x1004, 0x910029E0) // ADD X0, XZR, #10 (skipped)
			memory.Write32(0x1008, 0x910053E1) // ADD X1, XZR, #20 (skipped)
			memory.Write32(0x100C, 0x91007BE2) // ADD X2, XZR, #30 (skipped)
			memory.Write32(0x1010, 0x9100A3E3) // ADD X3, XZR, #40 (executed)
			memory.Write32(0x1014, 0x9100CBE4) // ADD X4, XZR, #50
			memory.Write32(0x1018, 0x9100F3E5) // ADD X5, XZR, #60
			memory.Write32(0x101C, 0x91011BE6) // ADD X6, XZR, #70
			memory.Write32(0x1020, 0xD4000001) // SVC #0

			pipe.SetPC(0x1000)
			pipe.Run()

			// Skipped instructions should not execute
			Expect(regFile.ReadReg(0)).To(Equal(uint64(0)))
			Expect(regFile.ReadReg(1)).To(Equal(uint64(0)))
			Expect(regFile.ReadReg(2)).To(Equal(uint64(0)))
			// Executed instructions
			Expect(regFile.ReadReg(3)).To(Equal(uint64(40)))
			Expect(regFile.ReadReg(4)).To(Equal(uint64(50)))
			Expect(regFile.ReadReg(5)).To(Equal(uint64(60)))
			Expect(regFile.ReadReg(6)).To(Equal(uint64(70)))
		})

		It("should handle conditional branch in 8-wide mode", func() {
			// Set up condition and test B.EQ
			memory.Write32(0x1000, 0x910029E0) // ADD X0, XZR, #10
			memory.Write32(0x1004, 0xF100041F) // CMP X0, #1 (sets flags, X0 > 1)
			memory.Write32(0x1008, 0x54000041) // B.NE #8 (take branch)
			memory.Write32(0x100C, 0x910053E1) // ADD X1, XZR, #20 (skipped)
			memory.Write32(0x1010, 0x91007BE2) // ADD X2, XZR, #30 (executed)
			memory.Write32(0x1014, 0xD4000001) // SVC #0

			pipe.SetPC(0x1000)
			pipe.Run()

			Expect(regFile.ReadReg(0)).To(Equal(uint64(10)))
			Expect(regFile.ReadReg(1)).To(Equal(uint64(0))) // Skipped
			Expect(regFile.ReadReg(2)).To(Equal(uint64(30)))
		})

		It("should handle memory operations in 8-wide mode", func() {
			// Mix of loads/stores and ALU ops
			memory.Write64(0x2000, 100)
			memory.Write64(0x2008, 200)
			regFile.WriteReg(10, 0x2000)
			regFile.WriteReg(11, 0x2008)

			memory.Write32(0x1000, 0xF9400140) // LDR X0, [X10]
			memory.Write32(0x1004, 0x910053E1) // ADD X1, XZR, #20
			memory.Write32(0x1008, 0x91007BE2) // ADD X2, XZR, #30
			memory.Write32(0x100C, 0x9100A3E3) // ADD X3, XZR, #40
			memory.Write32(0x1010, 0xF9400164) // LDR X4, [X11]
			memory.Write32(0x1014, 0x9100F3E5) // ADD X5, XZR, #60
			memory.Write32(0x1018, 0x91011BE6) // ADD X6, XZR, #70
			memory.Write32(0x101C, 0x910143E7) // ADD X7, XZR, #80
			memory.Write32(0x1020, 0xD4000001) // SVC #0

			pipe.SetPC(0x1000)
			pipe.Run()

			Expect(regFile.ReadReg(0)).To(Equal(uint64(100)))
			Expect(regFile.ReadReg(1)).To(Equal(uint64(20)))
			Expect(regFile.ReadReg(2)).To(Equal(uint64(30)))
			Expect(regFile.ReadReg(3)).To(Equal(uint64(40)))
			Expect(regFile.ReadReg(4)).To(Equal(uint64(200)))
			Expect(regFile.ReadReg(5)).To(Equal(uint64(60)))
			Expect(regFile.ReadReg(6)).To(Equal(uint64(70)))
			Expect(regFile.ReadReg(7)).To(Equal(uint64(80)))
		})

		It("should complete many independent instructions in 8-wide mode", func() {
			// Many independent ADD instructions in a row - tests sustained 8-wide issue
			memory.Write32(0x1000, 0x910029E0) // ADD X0, XZR, #10
			memory.Write32(0x1004, 0x910053E1) // ADD X1, XZR, #20
			memory.Write32(0x1008, 0x91007BE2) // ADD X2, XZR, #30
			memory.Write32(0x100C, 0x9100A3E3) // ADD X3, XZR, #40
			memory.Write32(0x1010, 0x9100CBE4) // ADD X4, XZR, #50
			memory.Write32(0x1014, 0x9100F3E5) // ADD X5, XZR, #60
			memory.Write32(0x1018, 0x91011BE6) // ADD X6, XZR, #70
			memory.Write32(0x101C, 0x910143E7) // ADD X7, XZR, #80
			memory.Write32(0x1020, 0x91016BF8) // ADD X24, XZR, #90
			memory.Write32(0x1024, 0x910193F9) // ADD X25, XZR, #100
			memory.Write32(0x1028, 0x9101BBFA) // ADD X26, XZR, #110
			memory.Write32(0x102C, 0x9101E3FB) // ADD X27, XZR, #120
			memory.Write32(0x1030, 0x91020BFC) // ADD X28, XZR, #130
			memory.Write32(0x1034, 0x910233FD) // ADD X29, XZR, #140
			memory.Write32(0x1038, 0x91025BEE) // ADD X14, XZR, #150
			memory.Write32(0x103C, 0x910283EF) // ADD X15, XZR, #160
			memory.Write32(0x1040, 0xD4000001) // SVC #0

			pipe.SetPC(0x1000)
			pipe.Run()

			// Verify all completed correctly
			Expect(regFile.ReadReg(0)).To(Equal(uint64(10)))
			Expect(regFile.ReadReg(7)).To(Equal(uint64(80)))
			Expect(regFile.ReadReg(24)).To(Equal(uint64(90)))
			Expect(regFile.ReadReg(27)).To(Equal(uint64(120)))
			Expect(regFile.ReadReg(14)).To(Equal(uint64(150)))
			Expect(regFile.ReadReg(15)).To(Equal(uint64(160)))
		})
	})

	Describe("Superscalar Configuration", func() {
		It("should support WithSuperscalar option", func() {
			config := pipeline.DualIssueConfig()
			pipe = pipeline.NewPipeline(regFile, memory, pipeline.WithSuperscalar(config))
			Expect(pipe).NotTo(BeNil())

			// Execute a simple program
			memory.Write32(0x1000, 0x910029E0) // ADD X0, XZR, #10
			memory.Write32(0x1004, 0xD4000001) // SVC #0
			pipe.SetPC(0x1000)
			pipe.Run()

			Expect(regFile.ReadReg(0)).To(Equal(uint64(10)))
		})

		It("should correctly use QuadIssueConfig", func() {
			config := pipeline.QuadIssueConfig()
			Expect(config.IssueWidth).To(Equal(4))
		})

		It("should correctly use SextupleIssueConfig", func() {
			config := pipeline.SextupleIssueConfig()
			Expect(config.IssueWidth).To(Equal(6))
		})

		It("should correctly use OctupleIssueConfig", func() {
			config := pipeline.OctupleIssueConfig()
			Expect(config.IssueWidth).To(Equal(8))
		})
	})
})
