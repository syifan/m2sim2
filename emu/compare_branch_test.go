package emu_test

import (
	"encoding/binary"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/sarchlab/m2sim/emu"
)

// Encoder helpers for compare-and-branch and test-bit-branch instructions.

// encodeCBZ encodes a CBZ instruction.
// Format: sf | 011010 | op | imm19 | Rt
// sf[31]: 0=32-bit, 1=64-bit
// op[24]: 0=CBZ, 1=CBNZ
// imm19[23:5]: signed offset / 4
func encodeCBZ(rt uint8, offset int32, is64Bit bool) uint32 {
	var sf uint32
	if is64Bit {
		sf = 1
	}
	imm19 := uint32((offset / 4) & 0x7FFFF)
	return (sf << 31) | (0b011010 << 25) | (0 << 24) | (imm19 << 5) | uint32(rt)
}

// encodeCBNZ encodes a CBNZ instruction.
func encodeCBNZ(rt uint8, offset int32, is64Bit bool) uint32 {
	var sf uint32
	if is64Bit {
		sf = 1
	}
	imm19 := uint32((offset / 4) & 0x7FFFF)
	return (sf << 31) | (0b011010 << 25) | (1 << 24) | (imm19 << 5) | uint32(rt)
}

// encodeTBZ encodes a TBZ instruction.
// Format: b5 | 011011 | op | b40 | imm14 | Rt
// b5[31]: bit number[5]
// op[24]: 0=TBZ, 1=TBNZ
// b40[23:19]: bit number[4:0]
// imm14[18:5]: signed offset / 4
func encodeTBZ(rt uint8, bitNum uint8, offset int32) uint32 {
	b5 := (bitNum >> 5) & 1
	b40 := bitNum & 0x1F
	imm14 := uint32((offset / 4) & 0x3FFF)
	return (uint32(b5) << 31) | (0b011011 << 25) | (0 << 24) | (uint32(b40) << 19) | (imm14 << 5) | uint32(rt)
}

// encodeTBNZ encodes a TBNZ instruction.
func encodeTBNZ(rt uint8, bitNum uint8, offset int32) uint32 {
	b5 := (bitNum >> 5) & 1
	b40 := bitNum & 0x1F
	imm14 := uint32((offset / 4) & 0x3FFF)
	return (uint32(b5) << 31) | (0b011011 << 25) | (1 << 24) | (uint32(b40) << 19) | (imm14 << 5) | uint32(rt)
}

func compareBranchProgram(inst uint32) []byte {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, inst)
	return buf
}

var _ = Describe("Compare and Branch Instructions", func() {
	var e *emu.Emulator

	BeforeEach(func() {
		e = emu.NewEmulator()
	})

	Describe("CBZ - Compare and Branch if Zero", func() {
		Context("64-bit register", func() {
			It("should branch when register is zero", func() {
				// CBZ X0, #16 (offset = 16 bytes)
				inst := encodeCBZ(0, 16, true)
				program := compareBranchProgram(inst)

				e.RegFile().WriteReg(0, 0) // X0 = 0
				e.LoadProgram(0x1000, program)

				result := e.Step()

				Expect(result.Err).To(BeNil())
				Expect(e.RegFile().PC).To(Equal(uint64(0x1000 + 16))) // Branched
			})

			It("should not branch when register is non-zero", func() {
				// CBZ X0, #16
				inst := encodeCBZ(0, 16, true)
				program := compareBranchProgram(inst)

				e.RegFile().WriteReg(0, 42) // X0 = 42 (non-zero)
				e.LoadProgram(0x1000, program)

				result := e.Step()

				Expect(result.Err).To(BeNil())
				Expect(e.RegFile().PC).To(Equal(uint64(0x1000 + 4))) // No branch, PC += 4
			})

			It("should handle negative offset", func() {
				// CBZ X0, #-8 (branch backward)
				inst := encodeCBZ(0, -8, true)
				program := compareBranchProgram(inst)

				e.RegFile().WriteReg(0, 0)
				e.LoadProgram(0x1000, program)

				result := e.Step()

				Expect(result.Err).To(BeNil())
				Expect(e.RegFile().PC).To(Equal(uint64(0x1000 - 8)))
			})
		})

		Context("32-bit register", func() {
			It("should branch when W register is zero", func() {
				// CBZ W0, #20
				inst := encodeCBZ(0, 20, false)
				program := compareBranchProgram(inst)

				e.RegFile().WriteReg(0, 0)
				e.LoadProgram(0x1000, program)

				result := e.Step()

				Expect(result.Err).To(BeNil())
				Expect(e.RegFile().PC).To(Equal(uint64(0x1000 + 20)))
			})

			It("should only check low 32 bits for W register", func() {
				// CBZ W0, #20
				inst := encodeCBZ(0, 20, false)
				program := compareBranchProgram(inst)

				// High 32 bits set, low 32 bits zero
				e.RegFile().WriteReg(0, 0xFFFFFFFF00000000)
				e.LoadProgram(0x1000, program)

				result := e.Step()

				Expect(result.Err).To(BeNil())
				Expect(e.RegFile().PC).To(Equal(uint64(0x1000 + 20))) // Should branch (W0 = 0)
			})

			It("should not branch when W register low bits are non-zero", func() {
				// CBZ W0, #20
				inst := encodeCBZ(0, 20, false)
				program := compareBranchProgram(inst)

				e.RegFile().WriteReg(0, 0x00000001) // W0 = 1
				e.LoadProgram(0x1000, program)

				result := e.Step()

				Expect(result.Err).To(BeNil())
				Expect(e.RegFile().PC).To(Equal(uint64(0x1000 + 4))) // No branch
			})
		})
	})

	Describe("CBNZ - Compare and Branch if Not Zero", func() {
		Context("64-bit register", func() {
			It("should branch when register is non-zero", func() {
				// CBNZ X1, #24
				inst := encodeCBNZ(1, 24, true)
				program := compareBranchProgram(inst)

				e.RegFile().WriteReg(1, 100) // X1 = 100 (non-zero)
				e.LoadProgram(0x1000, program)

				result := e.Step()

				Expect(result.Err).To(BeNil())
				Expect(e.RegFile().PC).To(Equal(uint64(0x1000 + 24)))
			})

			It("should not branch when register is zero", func() {
				// CBNZ X1, #24
				inst := encodeCBNZ(1, 24, true)
				program := compareBranchProgram(inst)

				e.RegFile().WriteReg(1, 0) // X1 = 0
				e.LoadProgram(0x1000, program)

				result := e.Step()

				Expect(result.Err).To(BeNil())
				Expect(e.RegFile().PC).To(Equal(uint64(0x1000 + 4)))
			})
		})

		Context("32-bit register", func() {
			It("should branch when W register is non-zero", func() {
				// CBNZ W2, #12
				inst := encodeCBNZ(2, 12, false)
				program := compareBranchProgram(inst)

				e.RegFile().WriteReg(2, 0x80000000) // W2 has bit 31 set
				e.LoadProgram(0x1000, program)

				result := e.Step()

				Expect(result.Err).To(BeNil())
				Expect(e.RegFile().PC).To(Equal(uint64(0x1000 + 12)))
			})

			It("should not branch when W register low bits are zero", func() {
				// CBNZ W2, #12
				inst := encodeCBNZ(2, 12, false)
				program := compareBranchProgram(inst)

				e.RegFile().WriteReg(2, 0xABCDEF0000000000) // W2 = 0 (low 32 bits)
				e.LoadProgram(0x1000, program)

				result := e.Step()

				Expect(result.Err).To(BeNil())
				Expect(e.RegFile().PC).To(Equal(uint64(0x1000 + 4)))
			})
		})
	})

	Describe("TBZ - Test Bit and Branch if Zero", func() {
		It("should branch when specified bit is zero", func() {
			// TBZ X0, #3, #32 (test bit 3, branch if zero)
			inst := encodeTBZ(0, 3, 32)
			program := compareBranchProgram(inst)

			e.RegFile().WriteReg(0, 0b11110111) // Bit 3 is 0
			e.LoadProgram(0x1000, program)

			result := e.Step()

			Expect(result.Err).To(BeNil())
			Expect(e.RegFile().PC).To(Equal(uint64(0x1000 + 32)))
		})

		It("should not branch when specified bit is one", func() {
			// TBZ X0, #3, #32
			inst := encodeTBZ(0, 3, 32)
			program := compareBranchProgram(inst)

			e.RegFile().WriteReg(0, 0b00001000) // Bit 3 is 1
			e.LoadProgram(0x1000, program)

			result := e.Step()

			Expect(result.Err).To(BeNil())
			Expect(e.RegFile().PC).To(Equal(uint64(0x1000 + 4)))
		})

		It("should test high bits (bit 32-63)", func() {
			// TBZ X0, #35, #16 (test bit 35)
			inst := encodeTBZ(0, 35, 16)
			program := compareBranchProgram(inst)

			e.RegFile().WriteReg(0, uint64(1)<<36) // Bit 36 is 1, bit 35 is 0
			e.LoadProgram(0x1000, program)

			result := e.Step()

			Expect(result.Err).To(BeNil())
			Expect(e.RegFile().PC).To(Equal(uint64(0x1000 + 16))) // Should branch
		})

		It("should handle backward branch", func() {
			// TBZ X0, #0, #-12
			inst := encodeTBZ(0, 0, -12)
			program := compareBranchProgram(inst)

			e.RegFile().WriteReg(0, 0b11111110) // Bit 0 is 0
			e.LoadProgram(0x1000, program)

			result := e.Step()

			Expect(result.Err).To(BeNil())
			Expect(e.RegFile().PC).To(Equal(uint64(0x1000 - 12)))
		})
	})

	Describe("TBNZ - Test Bit and Branch if Not Zero", func() {
		It("should branch when specified bit is one", func() {
			// TBNZ X0, #5, #28 (test bit 5, branch if not zero)
			inst := encodeTBNZ(0, 5, 28)
			program := compareBranchProgram(inst)

			e.RegFile().WriteReg(0, 0b00100000) // Bit 5 is 1
			e.LoadProgram(0x1000, program)

			result := e.Step()

			Expect(result.Err).To(BeNil())
			Expect(e.RegFile().PC).To(Equal(uint64(0x1000 + 28)))
		})

		It("should not branch when specified bit is zero", func() {
			// TBNZ X0, #5, #28
			inst := encodeTBNZ(0, 5, 28)
			program := compareBranchProgram(inst)

			e.RegFile().WriteReg(0, 0b11011111) // Bit 5 is 0
			e.LoadProgram(0x1000, program)

			result := e.Step()

			Expect(result.Err).To(BeNil())
			Expect(e.RegFile().PC).To(Equal(uint64(0x1000 + 4)))
		})

		It("should test bit 63 (highest bit)", func() {
			// TBNZ X0, #63, #8 (test sign bit)
			inst := encodeTBNZ(0, 63, 8)
			program := compareBranchProgram(inst)

			e.RegFile().WriteReg(0, 0x8000000000000000) // Bit 63 is 1
			e.LoadProgram(0x1000, program)

			result := e.Step()

			Expect(result.Err).To(BeNil())
			Expect(e.RegFile().PC).To(Equal(uint64(0x1000 + 8)))
		})

		It("should not branch when sign bit is zero", func() {
			// TBNZ X0, #63, #8
			inst := encodeTBNZ(0, 63, 8)
			program := compareBranchProgram(inst)

			e.RegFile().WriteReg(0, 0x7FFFFFFFFFFFFFFF) // Bit 63 is 0
			e.LoadProgram(0x1000, program)

			result := e.Step()

			Expect(result.Err).To(BeNil())
			Expect(e.RegFile().PC).To(Equal(uint64(0x1000 + 4)))
		})
	})
})
