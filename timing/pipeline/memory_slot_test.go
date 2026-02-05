package pipeline_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/sarchlab/m2sim/emu"
	"github.com/sarchlab/m2sim/insts"
	"github.com/sarchlab/m2sim/timing/pipeline"
)

var _ = Describe("MemorySlot Interface", func() {
	var (
		memory *emu.Memory
		stage  *pipeline.MemoryStage
	)

	BeforeEach(func() {
		memory = emu.NewMemory()
		stage = pipeline.NewMemoryStage(memory)
	})

	Describe("EXMEMRegister", func() {
		It("should return invalid when not valid", func() {
			reg := &pipeline.EXMEMRegister{Valid: false}
			Expect(reg.IsValid()).To(BeFalse())
		})

		It("should return valid when valid", func() {
			reg := &pipeline.EXMEMRegister{Valid: true}
			Expect(reg.IsValid()).To(BeTrue())
		})

		It("should return correct MemRead flag", func() {
			reg := &pipeline.EXMEMRegister{Valid: true, MemRead: true}
			Expect(reg.GetMemRead()).To(BeTrue())

			reg2 := &pipeline.EXMEMRegister{Valid: true, MemRead: false}
			Expect(reg2.GetMemRead()).To(BeFalse())
		})

		It("should return correct MemWrite flag", func() {
			reg := &pipeline.EXMEMRegister{Valid: true, MemWrite: true}
			Expect(reg.GetMemWrite()).To(BeTrue())

			reg2 := &pipeline.EXMEMRegister{Valid: true, MemWrite: false}
			Expect(reg2.GetMemWrite()).To(BeFalse())
		})

		It("should return ALU result", func() {
			reg := &pipeline.EXMEMRegister{Valid: true, ALUResult: 0x1000}
			Expect(reg.GetALUResult()).To(Equal(uint64(0x1000)))
		})

		It("should return store value", func() {
			reg := &pipeline.EXMEMRegister{Valid: true, StoreValue: 0xDEADBEEF}
			Expect(reg.GetStoreValue()).To(Equal(uint64(0xDEADBEEF)))
		})

		It("should return instruction", func() {
			inst := &insts.Instruction{Is64Bit: true}
			reg := &pipeline.EXMEMRegister{Valid: true, Inst: inst}
			Expect(reg.GetInst()).To(Equal(inst))
		})
	})

	Describe("MemoryStage.MemorySlot", func() {
		It("should return empty result for invalid slot", func() {
			reg := &pipeline.EXMEMRegister{Valid: false}
			result := stage.MemorySlot(reg)
			Expect(result.MemData).To(Equal(uint64(0)))
		})

		It("should perform 64-bit load", func() {
			memory.Write64(0x1000, 0x123456789ABCDEF0)
			inst := &insts.Instruction{Is64Bit: true}
			reg := &pipeline.EXMEMRegister{
				Valid:     true,
				MemRead:   true,
				ALUResult: 0x1000,
				Inst:      inst,
			}
			result := stage.MemorySlot(reg)
			Expect(result.MemData).To(Equal(uint64(0x123456789ABCDEF0)))
		})

		It("should perform 32-bit load", func() {
			memory.Write32(0x2000, 0xDEADBEEF)
			inst := &insts.Instruction{Is64Bit: false}
			reg := &pipeline.EXMEMRegister{
				Valid:     true,
				MemRead:   true,
				ALUResult: 0x2000,
				Inst:      inst,
			}
			result := stage.MemorySlot(reg)
			Expect(result.MemData).To(Equal(uint64(0xDEADBEEF)))
		})

		It("should perform 64-bit store", func() {
			inst := &insts.Instruction{Is64Bit: true}
			reg := &pipeline.EXMEMRegister{
				Valid:      true,
				MemWrite:   true,
				ALUResult:  0x3000,
				StoreValue: 0xCAFEBABECAFEBABE,
				Inst:       inst,
			}
			stage.MemorySlot(reg)
			Expect(memory.Read64(0x3000)).To(Equal(uint64(0xCAFEBABECAFEBABE)))
		})

		It("should perform 32-bit store", func() {
			inst := &insts.Instruction{Is64Bit: false}
			reg := &pipeline.EXMEMRegister{
				Valid:      true,
				MemWrite:   true,
				ALUResult:  0x4000,
				StoreValue: 0xBEEFCAFE,
				Inst:       inst,
			}
			stage.MemorySlot(reg)
			Expect(memory.Read32(0x4000)).To(Equal(uint32(0xBEEFCAFE)))
		})
	})
})
