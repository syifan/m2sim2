package core_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/sarchlab/m2sim/emu"
	"github.com/sarchlab/m2sim/timing/core"
)

var _ = Describe("Core", func() {
	var (
		regFile *emu.RegFile
		memory  *emu.Memory
		c       *core.Core
	)

	BeforeEach(func() {
		regFile = &emu.RegFile{}
		memory = emu.NewMemory()
		c = core.NewCore(regFile, memory)
	})

	It("should create a core with pipeline", func() {
		Expect(c).NotTo(BeNil())
		Expect(c.Pipeline).NotTo(BeNil())
	})

	It("should set and get PC", func() {
		c.SetPC(0x1000)
		Expect(c.Pipeline.PC()).To(Equal(uint64(0x1000)))
	})

	It("should not be halted initially", func() {
		Expect(c.Halted()).To(BeFalse())
	})

	It("should execute instructions through tick", func() {
		// ADD X1, XZR, #42
		memory.Write32(0x1000, 0x9100A821)
		// NOP instructions to flush pipeline.
		memory.Write32(0x1004, 0xD503201F)
		memory.Write32(0x1008, 0xD503201F)
		memory.Write32(0x100C, 0xD503201F)
		memory.Write32(0x1010, 0xD503201F)

		c.SetPC(0x1000)

		for i := 0; i < 10; i++ {
			c.Tick()
		}

		Expect(regFile.X[1]).To(Equal(uint64(42)))
	})

	It("should return stats", func() {
		memory.Write32(0x1000, 0x9100A821) // ADD X1, XZR, #42
		memory.Write32(0x1004, 0xD503201F) // NOP

		c.SetPC(0x1000)
		c.Tick()
		c.Tick()

		stats := c.Stats()
		Expect(stats.Cycles).To(Equal(uint64(2)))
	})

	It("should run until halt and return exit code", func() {
		// Setup exit syscall: X8 = 93 (Linux exit), X0 = exit code
		regFile.WriteReg(8, 93) // syscall number in X8
		// MOV X0, #42 = ADD X0, XZR, #42
		memory.Write32(0x1000, 0x91002800) // ADD X0, X0, #10 -> actually use simpler
		// Let's use the pattern from pipeline tests
		memory.Write32(0x1000, 0x910029E0) // ADD X0, XZR, #10 (exit code = 10)
		memory.Write32(0x1004, 0xD4000001) // SVC #0

		c.SetPC(0x1000)
		exitCode := c.Run()

		Expect(c.Halted()).To(BeTrue())
		Expect(exitCode).To(Equal(int64(10)))
	})

	It("should return exit code correctly", func() {
		regFile.WriteReg(8, 93)            // syscall number
		memory.Write32(0x1000, 0x910001E0) // ADD X0, XZR, #0 (exit code 0)
		memory.Write32(0x1004, 0xD4000001) // SVC #0

		c.SetPC(0x1000)
		c.Run()

		Expect(c.ExitCode()).To(Equal(int64(0)))
	})

	It("should run for specified cycles and return running status", func() {
		// ADD X1, XZR, #1 repeated - fill with NOPs to keep running
		memory.Write32(0x1000, 0x91000421) // ADD X1, X1, #1
		memory.Write32(0x1004, 0xD503201F) // NOP
		memory.Write32(0x1008, 0xD503201F) // NOP
		memory.Write32(0x100C, 0xD503201F) // NOP
		memory.Write32(0x1010, 0xD503201F) // NOP
		memory.Write32(0x1014, 0xD503201F) // NOP
		memory.Write32(0x1018, 0xD503201F) // NOP
		memory.Write32(0x101C, 0xD503201F) // NOP
		memory.Write32(0x1020, 0xD503201F) // NOP
		memory.Write32(0x1024, 0xD503201F) // NOP

		c.SetPC(0x1000)
		running := c.RunCycles(5)

		Expect(running).To(BeTrue())
		Expect(c.Halted()).To(BeFalse())

		stats := c.Stats()
		Expect(stats.Cycles).To(Equal(uint64(5)))
	})

	It("should stop running cycles when halted", func() {
		regFile.WriteReg(8, 93)            // syscall number
		memory.Write32(0x1000, 0xD2800000) // MOV X0, #0
		memory.Write32(0x1004, 0xD4000001) // SVC #0

		c.SetPC(0x1000)
		running := c.RunCycles(100)

		Expect(running).To(BeFalse())
		Expect(c.Halted()).To(BeTrue())
	})

	It("should reset core state", func() {
		// Execute some instructions first
		memory.Write32(0x1000, 0x91000421) // ADD X1, XZR, #1
		memory.Write32(0x1004, 0xD503201F) // NOP
		memory.Write32(0x1008, 0xD503201F)
		memory.Write32(0x100C, 0xD503201F)
		memory.Write32(0x1010, 0xD503201F)

		c.SetPC(0x1000)
		for i := 0; i < 10; i++ {
			c.Tick()
		}

		stats := c.Stats()
		Expect(stats.Cycles).To(BeNumerically(">", 0))

		// Reset
		c.Reset()

		// After reset, stats should be zeroed
		statsAfterReset := c.Stats()
		Expect(statsAfterReset.Cycles).To(Equal(uint64(0)))
		Expect(statsAfterReset.Instructions).To(Equal(uint64(0)))
		Expect(c.Halted()).To(BeFalse())
	})
})
