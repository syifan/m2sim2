package emu_test

import (
	"encoding/binary"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/sarchlab/m2sim/emu"
)

var _ = Describe("Bitfield Operations", func() {
	var e *emu.Emulator

	BeforeEach(func() {
		e = emu.NewEmulator()
	})

	Describe("UBFM (Unsigned Bitfield Move)", func() {
		Context("64-bit operations", func() {
			It("should perform LSR (logical shift right)", func() {
				// LSR X0, X1, #4 is UBFM X0, X1, #4, #63
				inst := encodeUBFM(0, 1, 4, 63, true)
				program := uint32ToBytesBitfield(inst)

				e.RegFile().WriteReg(1, 0xFF00)
				e.LoadProgram(0x1000, program)

				result := e.Step()

				Expect(result.Err).To(BeNil())
				Expect(e.RegFile().ReadReg(0)).To(Equal(uint64(0x0FF0)))
			})

			It("should perform UXTB (unsigned extend byte)", func() {
				// UXTB X0, X1 is UBFM X0, X1, #0, #7
				inst := encodeUBFM(0, 1, 0, 7, true)
				program := uint32ToBytesBitfield(inst)

				e.RegFile().WriteReg(1, 0xFFFFFFFFFFFFFF80) // -128 as byte
				e.LoadProgram(0x1000, program)

				result := e.Step()

				Expect(result.Err).To(BeNil())
				Expect(e.RegFile().ReadReg(0)).To(Equal(uint64(0x80))) // Zero-extended
			})

			It("should perform UXTH (unsigned extend halfword)", func() {
				// UXTH X0, X1 is UBFM X0, X1, #0, #15
				inst := encodeUBFM(0, 1, 0, 15, true)
				program := uint32ToBytesBitfield(inst)

				e.RegFile().WriteReg(1, 0xFFFFFFFF8000) // -32768 as halfword
				e.LoadProgram(0x1000, program)

				result := e.Step()

				Expect(result.Err).To(BeNil())
				Expect(e.RegFile().ReadReg(0)).To(Equal(uint64(0x8000))) // Zero-extended
			})

			It("should perform LSL (logical shift left)", func() {
				// LSL X0, X1, #4 is UBFM X0, X1, #60, #59 (for 64-bit)
				inst := encodeUBFM(0, 1, 60, 59, true)
				program := uint32ToBytesBitfield(inst)

				e.RegFile().WriteReg(1, 0x0F)
				e.LoadProgram(0x1000, program)

				result := e.Step()

				Expect(result.Err).To(BeNil())
				Expect(e.RegFile().ReadReg(0)).To(Equal(uint64(0xF0)))
			})
		})

		Context("32-bit operations", func() {
			It("should perform LSR (32-bit)", func() {
				// LSR W0, W1, #4 is UBFM W0, W1, #4, #31
				inst := encodeUBFM(0, 1, 4, 31, false)
				program := uint32ToBytesBitfield(inst)

				e.RegFile().WriteReg(1, 0xFF00)
				e.LoadProgram(0x1000, program)

				result := e.Step()

				Expect(result.Err).To(BeNil())
				Expect(e.RegFile().ReadReg(0)).To(Equal(uint64(0x0FF0)))
			})

			It("should perform UXTB (32-bit)", func() {
				// UXTB W0, W1 is UBFM W0, W1, #0, #7
				inst := encodeUBFM(0, 1, 0, 7, false)
				program := uint32ToBytesBitfield(inst)

				e.RegFile().WriteReg(1, 0xABCDEF80)
				e.LoadProgram(0x1000, program)

				result := e.Step()

				Expect(result.Err).To(BeNil())
				Expect(e.RegFile().ReadReg(0)).To(Equal(uint64(0x80)))
			})
		})
	})

	Describe("SBFM (Signed Bitfield Move)", func() {
		Context("64-bit operations", func() {
			It("should perform ASR (arithmetic shift right)", func() {
				// ASR X0, X1, #4 is SBFM X0, X1, #4, #63
				inst := encodeSBFM(0, 1, 4, 63, true)
				program := uint32ToBytesBitfield(inst)

				e.RegFile().WriteReg(1, 0x8000000000000000) // Negative
				e.LoadProgram(0x1000, program)

				result := e.Step()

				Expect(result.Err).To(BeNil())
				// After ASR by 4, upper bits should be sign-extended
				Expect(e.RegFile().ReadReg(0)).To(Equal(uint64(0xF800000000000000)))
			})

			It("should perform SXTB (signed extend byte)", func() {
				// SXTB X0, X1 is SBFM X0, X1, #0, #7
				inst := encodeSBFM(0, 1, 0, 7, true)
				program := uint32ToBytesBitfield(inst)

				e.RegFile().WriteReg(1, 0x80) // -128 as signed byte
				e.LoadProgram(0x1000, program)

				result := e.Step()

				Expect(result.Err).To(BeNil())
				Expect(e.RegFile().ReadReg(0)).To(Equal(uint64(0xFFFFFFFFFFFFFF80))) // Sign-extended
			})

			It("should sign extend positive byte", func() {
				// SXTB X0, X1 is SBFM X0, X1, #0, #7
				inst := encodeSBFM(0, 1, 0, 7, true)
				program := uint32ToBytesBitfield(inst)

				e.RegFile().WriteReg(1, 0x7F) // +127 as signed byte
				e.LoadProgram(0x1000, program)

				result := e.Step()

				Expect(result.Err).To(BeNil())
				Expect(e.RegFile().ReadReg(0)).To(Equal(uint64(0x7F))) // No sign extension needed
			})

			It("should perform SXTH (signed extend halfword)", func() {
				// SXTH X0, X1 is SBFM X0, X1, #0, #15
				inst := encodeSBFM(0, 1, 0, 15, true)
				program := uint32ToBytesBitfield(inst)

				e.RegFile().WriteReg(1, 0x8000) // -32768 as signed halfword
				e.LoadProgram(0x1000, program)

				result := e.Step()

				Expect(result.Err).To(BeNil())
				Expect(e.RegFile().ReadReg(0)).To(Equal(uint64(0xFFFFFFFFFFFF8000)))
			})

			It("should perform SXTW (signed extend word)", func() {
				// SXTW X0, X1 is SBFM X0, X1, #0, #31
				inst := encodeSBFM(0, 1, 0, 31, true)
				program := uint32ToBytesBitfield(inst)

				e.RegFile().WriteReg(1, 0x80000000) // -2147483648 as signed word
				e.LoadProgram(0x1000, program)

				result := e.Step()

				Expect(result.Err).To(BeNil())
				Expect(e.RegFile().ReadReg(0)).To(Equal(uint64(0xFFFFFFFF80000000)))
			})
		})

		Context("32-bit operations", func() {
			It("should perform ASR (32-bit)", func() {
				// ASR W0, W1, #4 is SBFM W0, W1, #4, #31
				inst := encodeSBFM(0, 1, 4, 31, false)
				program := uint32ToBytesBitfield(inst)

				e.RegFile().WriteReg(1, 0x80000000) // Negative 32-bit
				e.LoadProgram(0x1000, program)

				result := e.Step()

				Expect(result.Err).To(BeNil())
				// Result should be sign-extended 32-bit value
				Expect(e.RegFile().ReadReg(0)).To(Equal(uint64(0xF8000000)))
			})

			It("should perform SXTB (32-bit)", func() {
				// SXTB W0, W1 is SBFM W0, W1, #0, #7
				inst := encodeSBFM(0, 1, 0, 7, false)
				program := uint32ToBytesBitfield(inst)

				e.RegFile().WriteReg(1, 0x80) // -128 as signed byte
				e.LoadProgram(0x1000, program)

				result := e.Step()

				Expect(result.Err).To(BeNil())
				Expect(e.RegFile().ReadReg(0)).To(Equal(uint64(0xFFFFFF80)))
			})
		})
	})
})

var _ = Describe("Conditional Select Operations", func() {
	var e *emu.Emulator

	BeforeEach(func() {
		e = emu.NewEmulator()
	})

	Describe("CSEL (Conditional Select)", func() {
		It("should select Rn when condition is true", func() {
			// CSEL X0, X1, X2, EQ -> 0x9A820020
			program := uint32ToBytesBitfield(0x9A820020)

			e.RegFile().WriteReg(1, 100)
			e.RegFile().WriteReg(2, 200)
			e.RegFile().PSTATE.Z = true // EQ condition true
			e.LoadProgram(0x1000, program)

			result := e.Step()

			Expect(result.Err).To(BeNil())
			Expect(e.RegFile().ReadReg(0)).To(Equal(uint64(100)))
		})

		It("should select Rm when condition is false", func() {
			// CSEL X0, X1, X2, EQ -> 0x9A820020
			program := uint32ToBytesBitfield(0x9A820020)

			e.RegFile().WriteReg(1, 100)
			e.RegFile().WriteReg(2, 200)
			e.RegFile().PSTATE.Z = false // EQ condition false
			e.LoadProgram(0x1000, program)

			result := e.Step()

			Expect(result.Err).To(BeNil())
			Expect(e.RegFile().ReadReg(0)).To(Equal(uint64(200)))
		})
	})

	Describe("CSINC (Conditional Select Increment)", func() {
		It("should select Rn when condition is true", func() {
			// CSINC X3, X4, X5, NE -> 0x9A851483
			program := uint32ToBytesBitfield(0x9A851483)

			e.RegFile().WriteReg(4, 100)
			e.RegFile().WriteReg(5, 200)
			e.RegFile().PSTATE.Z = false // NE condition true
			e.LoadProgram(0x1000, program)

			result := e.Step()

			Expect(result.Err).To(BeNil())
			Expect(e.RegFile().ReadReg(3)).To(Equal(uint64(100)))
		})

		It("should select Rm+1 when condition is false", func() {
			// CSINC X3, X4, X5, NE -> 0x9A851483
			program := uint32ToBytesBitfield(0x9A851483)

			e.RegFile().WriteReg(4, 100)
			e.RegFile().WriteReg(5, 200)
			e.RegFile().PSTATE.Z = true // NE condition false
			e.LoadProgram(0x1000, program)

			result := e.Step()

			Expect(result.Err).To(BeNil())
			Expect(e.RegFile().ReadReg(3)).To(Equal(uint64(201))) // 200 + 1
		})
	})

	Describe("CSINV (Conditional Select Invert)", func() {
		It("should select Rn when condition is true", func() {
			// CSINV X6, X7, X8, GE -> 0xDA88A0E6
			program := uint32ToBytesBitfield(0xDA88A0E6)

			e.RegFile().WriteReg(7, 100)
			e.RegFile().WriteReg(8, 200)
			e.RegFile().PSTATE.N = false
			e.RegFile().PSTATE.V = false // GE condition true (N==V)
			e.LoadProgram(0x1000, program)

			result := e.Step()

			Expect(result.Err).To(BeNil())
			Expect(e.RegFile().ReadReg(6)).To(Equal(uint64(100)))
		})

		It("should select ~Rm when condition is false", func() {
			// CSINV X6, X7, X8, GE -> 0xDA88A0E6
			program := uint32ToBytesBitfield(0xDA88A0E6)

			e.RegFile().WriteReg(7, 100)
			e.RegFile().WriteReg(8, 0x0F)
			e.RegFile().PSTATE.N = true
			e.RegFile().PSTATE.V = false // GE condition false (N!=V)
			e.LoadProgram(0x1000, program)

			result := e.Step()

			Expect(result.Err).To(BeNil())
			Expect(e.RegFile().ReadReg(6)).To(Equal(^uint64(0x0F))) // ~0x0F
		})
	})

	Describe("CSNEG (Conditional Select Negate)", func() {
		It("should select Rn when condition is true", func() {
			// CSNEG X9, X10, X11, LT -> 0xDA8BB549
			program := uint32ToBytesBitfield(0xDA8BB549)

			e.RegFile().WriteReg(10, 100)
			e.RegFile().WriteReg(11, 200)
			e.RegFile().PSTATE.N = true
			e.RegFile().PSTATE.V = false // LT condition true (N!=V)
			e.LoadProgram(0x1000, program)

			result := e.Step()

			Expect(result.Err).To(BeNil())
			Expect(e.RegFile().ReadReg(9)).To(Equal(uint64(100)))
		})

		It("should select -Rm when condition is false", func() {
			// CSNEG X9, X10, X11, LT -> 0xDA8BB549
			program := uint32ToBytesBitfield(0xDA8BB549)

			e.RegFile().WriteReg(10, 100)
			e.RegFile().WriteReg(11, 5)
			e.RegFile().PSTATE.N = false
			e.RegFile().PSTATE.V = false // LT condition false (N==V)
			e.LoadProgram(0x1000, program)

			result := e.Step()

			Expect(result.Err).To(BeNil())
			Expect(e.RegFile().ReadReg(9)).To(Equal(uint64(0xFFFFFFFFFFFFFFFB))) // -5 as twos complement
		})
	})

	Describe("32-bit conditional select", func() {
		It("should mask result to 32 bits", func() {
			// CSEL W0, W1, W2, EQ -> 0x1A820020
			program := uint32ToBytesBitfield(0x1A820020)

			e.RegFile().WriteReg(1, 0xFFFFFFFF00000064) // Only lower 32 bits matter
			e.RegFile().WriteReg(2, 200)
			e.RegFile().PSTATE.Z = true // EQ condition true
			e.LoadProgram(0x1000, program)

			result := e.Step()

			Expect(result.Err).To(BeNil())
			Expect(e.RegFile().ReadReg(0)).To(Equal(uint64(0x64))) // Masked to 32 bits
		})
	})
})

// Helper functions to encode bitfield instructions

func uint32ToBytesBitfield(v uint32) []byte {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, v)
	return buf
}

// encodeUBFM encodes an UBFM instruction.
// Format: sf | opc(10) | 100110 | N | immr | imms | Rn | Rd
func encodeUBFM(rd, rn, immr, imms uint8, is64bit bool) uint32 {
	var inst uint32 = 0
	if is64bit {
		inst |= 1 << 31 // sf = 1
		inst |= 1 << 22 // N = 1 (must match sf for valid encoding)
	}
	inst |= 0b10 << 29     // opc = 10 for UBFM
	inst |= 0b100110 << 23 // opcode bits
	inst |= uint32(immr&0x3F) << 16
	inst |= uint32(imms&0x3F) << 10
	inst |= uint32(rn&0x1F) << 5
	inst |= uint32(rd & 0x1F)
	return inst
}

// encodeSBFM encodes an SBFM instruction.
// Format: sf | opc(00) | 100110 | N | immr | imms | Rn | Rd
func encodeSBFM(rd, rn, immr, imms uint8, is64bit bool) uint32 {
	var inst uint32 = 0
	if is64bit {
		inst |= 1 << 31 // sf = 1
		inst |= 1 << 22 // N = 1
	}
	inst |= 0b00 << 29 // opc = 00 for SBFM
	inst |= 0b100110 << 23
	inst |= uint32(immr&0x3F) << 16
	inst |= uint32(imms&0x3F) << 10
	inst |= uint32(rn&0x1F) << 5
	inst |= uint32(rd & 0x1F)
	return inst
}
