package latency_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/sarchlab/m2sim/insts"
	"github.com/sarchlab/m2sim/timing/latency"
)

var _ = Describe("Latency", func() {
	var (
		table   *latency.Table
		decoder *insts.Decoder
	)

	BeforeEach(func() {
		table = latency.NewTable()
		decoder = insts.NewDecoder()
	})

	Describe("Default Timing Values", func() {
		It("should have correct ALU latency", func() {
			config := table.Config()
			Expect(config.ALULatency).To(Equal(uint64(1)))
		})

		It("should have correct branch latency", func() {
			config := table.Config()
			Expect(config.BranchLatency).To(Equal(uint64(1)))
		})

		It("should have correct load latency", func() {
			config := table.Config()
			Expect(config.LoadLatency).To(Equal(uint64(4)))
		})

		It("should have correct store latency", func() {
			config := table.Config()
			Expect(config.StoreLatency).To(Equal(uint64(1)))
		})

		It("should have correct branch misprediction penalty", func() {
			config := table.Config()
			Expect(config.BranchMispredictPenalty).To(Equal(uint64(12)))
		})
	})

	Describe("ALU Instruction Latencies", func() {
		It("should return 1 cycle for ADD immediate", func() {
			// ADD X0, X1, #42 -> 0x91002820
			inst := decoder.Decode(0x91002820)
			Expect(table.GetLatency(inst)).To(Equal(uint64(1)))
		})

		It("should return 1 cycle for SUB immediate", func() {
			// SUB X0, X1, #10 -> 0xD1002820
			inst := decoder.Decode(0xD1002820)
			Expect(table.GetLatency(inst)).To(Equal(uint64(1)))
		})

		It("should return 1 cycle for ADD register", func() {
			// ADD X0, X1, X2 -> 0x8B020020
			inst := decoder.Decode(0x8B020020)
			Expect(table.GetLatency(inst)).To(Equal(uint64(1)))
		})

		It("should return 1 cycle for AND register", func() {
			// AND X0, X1, X2 -> 0x8A020020
			inst := decoder.Decode(0x8A020020)
			Expect(table.GetLatency(inst)).To(Equal(uint64(1)))
		})

		It("should return 1 cycle for ORR register", func() {
			// ORR X0, X1, X2 -> 0xAA020020
			inst := decoder.Decode(0xAA020020)
			Expect(table.GetLatency(inst)).To(Equal(uint64(1)))
		})

		It("should return 1 cycle for EOR register", func() {
			// EOR X0, X1, X2 -> 0xCA020020
			inst := decoder.Decode(0xCA020020)
			Expect(table.GetLatency(inst)).To(Equal(uint64(1)))
		})
	})

	Describe("Multiply Instruction Latencies", func() {
		It("should return MultiplyLatency for MADD", func() {
			// MADD X0, X1, X2, X3 -> 0x9B020C20
			inst := decoder.Decode(0x9B020C20)
			Expect(inst.Op).To(Equal(insts.OpMADD))
			Expect(table.GetLatency(inst)).To(Equal(uint64(3)))
		})

		It("should return MultiplyLatency for MSUB", func() {
			// MSUB X4, X5, X6, X7 -> 0x9B069CA4
			inst := decoder.Decode(0x9B069CA4)
			Expect(inst.Op).To(Equal(insts.OpMSUB))
			Expect(table.GetLatency(inst)).To(Equal(uint64(3)))
		})
	})

	Describe("Branch Instruction Latencies", func() {
		It("should return 1 cycle for B", func() {
			// B #100 -> 0x14000019
			inst := decoder.Decode(0x14000019)
			Expect(table.GetLatency(inst)).To(Equal(uint64(1)))
		})

		It("should return 1 cycle for BL", func() {
			// BL #100 -> 0x94000019
			inst := decoder.Decode(0x94000019)
			Expect(table.GetLatency(inst)).To(Equal(uint64(1)))
		})

		It("should return 1 cycle for B.EQ", func() {
			// B.EQ #100 -> 0x54000320
			inst := decoder.Decode(0x54000320)
			Expect(table.GetLatency(inst)).To(Equal(uint64(1)))
		})

		It("should return 1 cycle for BR", func() {
			// BR X1 -> 0xD61F0020
			inst := decoder.Decode(0xD61F0020)
			Expect(table.GetLatency(inst)).To(Equal(uint64(1)))
		})

		It("should return 1 cycle for RET", func() {
			// RET -> 0xD65F03C0
			inst := decoder.Decode(0xD65F03C0)
			Expect(table.GetLatency(inst)).To(Equal(uint64(1)))
		})
	})

	Describe("Memory Instruction Latencies", func() {
		It("should return 4 cycles for LDR (L1 hit)", func() {
			// LDR X0, [X1, #8] -> 0xF9400420
			inst := decoder.Decode(0xF9400420)
			Expect(table.GetLatency(inst)).To(Equal(uint64(4)))
		})

		It("should return 1 cycle for STR", func() {
			// STR X0, [X1, #8] -> 0xF9000420
			inst := decoder.Decode(0xF9000420)
			Expect(table.GetLatency(inst)).To(Equal(uint64(1)))
		})

		It("should return LoadLatency for LDRSW", func() {
			// LDRSW X0, [X1] -> 0xB9800020
			inst := decoder.Decode(0xB9800020)
			Expect(inst.Op).To(Equal(insts.OpLDRSW))
			Expect(table.GetLatency(inst)).To(Equal(uint64(4)))
		})
	})

	Describe("Instruction Type Detection", func() {
		It("should detect memory operations", func() {
			ldr := decoder.Decode(0xF9400420)
			str := decoder.Decode(0xF9000420)
			ldrsw := decoder.Decode(0xB9800020)
			add := decoder.Decode(0x91002820)

			Expect(table.IsMemoryOp(ldr)).To(BeTrue())
			Expect(table.IsMemoryOp(str)).To(BeTrue())
			Expect(table.IsMemoryOp(ldrsw)).To(BeTrue())
			Expect(table.IsMemoryOp(add)).To(BeFalse())
		})

		It("should detect load operations", func() {
			ldr := decoder.Decode(0xF9400420)
			str := decoder.Decode(0xF9000420)
			ldrsw := decoder.Decode(0xB9800020)

			Expect(table.IsLoadOp(ldr)).To(BeTrue())
			Expect(table.IsLoadOp(ldrsw)).To(BeTrue())
			Expect(table.IsLoadOp(str)).To(BeFalse())
		})

		It("should detect store operations", func() {
			ldr := decoder.Decode(0xF9400420)
			str := decoder.Decode(0xF9000420)

			Expect(table.IsStoreOp(str)).To(BeTrue())
			Expect(table.IsStoreOp(ldr)).To(BeFalse())
		})

		It("should detect branch operations", func() {
			b := decoder.Decode(0x14000019)
			bl := decoder.Decode(0x94000019)
			ret := decoder.Decode(0xD65F03C0)
			add := decoder.Decode(0x91002820)

			Expect(table.IsBranchOp(b)).To(BeTrue())
			Expect(table.IsBranchOp(bl)).To(BeTrue())
			Expect(table.IsBranchOp(ret)).To(BeTrue())
			Expect(table.IsBranchOp(add)).To(BeFalse())
		})
	})

	Describe("Nil Instruction Handling", func() {
		It("should return 1 for nil instruction", func() {
			Expect(table.GetLatency(nil)).To(Equal(uint64(1)))
		})

		It("should return false for nil instruction memory check", func() {
			Expect(table.IsMemoryOp(nil)).To(BeFalse())
			Expect(table.IsLoadOp(nil)).To(BeFalse())
			Expect(table.IsStoreOp(nil)).To(BeFalse())
			Expect(table.IsBranchOp(nil)).To(BeFalse())
		})
	})

	Describe("Custom Configuration", func() {
		It("should use custom config values", func() {
			config := &latency.TimingConfig{
				ALULatency:              2,
				BranchLatency:           3,
				BranchMispredictPenalty: 20,
				LoadLatency:             8,
				StoreLatency:            2,
				MultiplyLatency:         4,
				DivideLatencyMin:        12,
				DivideLatencyMax:        20,
				SyscallLatency:          1,
			}
			customTable := latency.NewTableWithConfig(config)

			add := decoder.Decode(0x91002820)
			ldr := decoder.Decode(0xF9400420)
			b := decoder.Decode(0x14000019)

			Expect(customTable.GetLatency(add)).To(Equal(uint64(2)))
			Expect(customTable.GetLatency(ldr)).To(Equal(uint64(8)))
			Expect(customTable.GetLatency(b)).To(Equal(uint64(3)))
		})
	})
})

var _ = Describe("TimingConfig", func() {
	Describe("Default Config", func() {
		It("should create valid default config", func() {
			config := latency.DefaultTimingConfig()
			Expect(config.Validate()).To(Succeed())
		})
	})

	Describe("Validation", func() {
		It("should reject zero ALU latency", func() {
			config := latency.DefaultTimingConfig()
			config.ALULatency = 0
			Expect(config.Validate()).To(HaveOccurred())
		})

		It("should reject zero branch latency", func() {
			config := latency.DefaultTimingConfig()
			config.BranchLatency = 0
			Expect(config.Validate()).To(HaveOccurred())
		})

		It("should reject zero load latency", func() {
			config := latency.DefaultTimingConfig()
			config.LoadLatency = 0
			Expect(config.Validate()).To(HaveOccurred())
		})

		It("should reject zero store latency", func() {
			config := latency.DefaultTimingConfig()
			config.StoreLatency = 0
			Expect(config.Validate()).To(HaveOccurred())
		})

		It("should reject inverted divide latency range", func() {
			config := latency.DefaultTimingConfig()
			config.DivideLatencyMin = 20
			config.DivideLatencyMax = 10
			Expect(config.Validate()).To(HaveOccurred())
		})
	})

	Describe("Clone", func() {
		It("should create independent copy", func() {
			original := latency.DefaultTimingConfig()
			clone := original.Clone()

			clone.ALULatency = 100

			Expect(original.ALULatency).To(Equal(uint64(1)))
			Expect(clone.ALULatency).To(Equal(uint64(100)))
		})
	})

	Describe("File Operations", func() {
		var tempDir string

		BeforeEach(func() {
			var err error
			tempDir, err = os.MkdirTemp("", "latency-test")
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			_ = os.RemoveAll(tempDir)
		})

		It("should save and load config", func() {
			original := latency.DefaultTimingConfig()
			original.ALULatency = 5
			original.LoadLatency = 10

			path := filepath.Join(tempDir, "timing.json")
			Expect(original.SaveConfig(path)).To(Succeed())

			loaded, err := latency.LoadConfig(path)
			Expect(err).NotTo(HaveOccurred())
			Expect(loaded.ALULatency).To(Equal(uint64(5)))
			Expect(loaded.LoadLatency).To(Equal(uint64(10)))
		})

		It("should return error for non-existent file", func() {
			_, err := latency.LoadConfig("/nonexistent/path/timing.json")
			Expect(err).To(HaveOccurred())
		})

		It("should return error for invalid JSON", func() {
			path := filepath.Join(tempDir, "invalid.json")
			err := os.WriteFile(path, []byte("not valid json"), 0644)
			Expect(err).NotTo(HaveOccurred())

			_, err = latency.LoadConfig(path)
			Expect(err).To(HaveOccurred())
		})
	})
})
