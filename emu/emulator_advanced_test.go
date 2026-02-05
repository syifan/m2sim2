package emu_test

import (
	"bytes"
	"encoding/binary"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/sarchlab/m2sim/emu"
	"github.com/sarchlab/m2sim/insts"
)

var _ = Describe("Emulator Advanced Operations", func() {
	var (
		e         *emu.Emulator
		stdoutBuf *bytes.Buffer
	)

	BeforeEach(func() {
		stdoutBuf = &bytes.Buffer{}
		e = emu.NewEmulator(
			emu.WithStdout(stdoutBuf),
		)
	})

	Describe("Load Store Pair", func() {
		Context("LDP - Load Pair", func() {
			It("should load pair of 64-bit values", func() {
				// LDP X0, X1, [X2] - 64-bit
				inst := encodeLDP64(0, 1, 2, 0, false)
				program := uint32ToLEBytes(inst)

				// Setup memory with values
				e.Memory().Write64(0x8000, 0xDEADBEEF12345678)
				e.Memory().Write64(0x8008, 0xCAFEBABE87654321)
				e.RegFile().WriteReg(2, 0x8000)
				e.LoadProgram(0x1000, program)

				result := e.Step()

				Expect(result.Err).To(BeNil())
				Expect(e.RegFile().ReadReg(0)).To(Equal(uint64(0xDEADBEEF12345678)))
				Expect(e.RegFile().ReadReg(1)).To(Equal(uint64(0xCAFEBABE87654321)))
			})

			It("should load pair of 32-bit values", func() {
				// LDP W0, W1, [X2] - 32-bit
				inst := encodeLDP32(0, 1, 2, 0, false)
				program := uint32ToLEBytes(inst)

				e.Memory().Write32(0x8000, 0x12345678)
				e.Memory().Write32(0x8004, 0x87654321)
				e.RegFile().WriteReg(2, 0x8000)
				e.LoadProgram(0x1000, program)

				result := e.Step()

				Expect(result.Err).To(BeNil())
				Expect(e.RegFile().ReadReg(0)).To(Equal(uint64(0x12345678)))
				Expect(e.RegFile().ReadReg(1)).To(Equal(uint64(0x87654321)))
			})

			It("should load pair with signed offset", func() {
				// LDP X0, X1, [X2, #16]
				inst := encodeLDP64(0, 1, 2, 16, false)
				program := uint32ToLEBytes(inst)

				e.Memory().Write64(0x8010, 0x1111111111111111)
				e.Memory().Write64(0x8018, 0x2222222222222222)
				e.RegFile().WriteReg(2, 0x8000)
				e.LoadProgram(0x1000, program)

				result := e.Step()

				Expect(result.Err).To(BeNil())
				Expect(e.RegFile().ReadReg(0)).To(Equal(uint64(0x1111111111111111)))
				Expect(e.RegFile().ReadReg(1)).To(Equal(uint64(0x2222222222222222)))
			})

			It("should load pair with pre-index writeback", func() {
				// LDP X0, X1, [X2, #16]!
				inst := encodeLDP64PreIndex(0, 1, 2, 16)
				program := uint32ToLEBytes(inst)

				e.Memory().Write64(0x8010, 0xAAAAAAAAAAAAAAAA)
				e.Memory().Write64(0x8018, 0xBBBBBBBBBBBBBBBB)
				e.RegFile().WriteReg(2, 0x8000)
				e.LoadProgram(0x1000, program)

				result := e.Step()

				Expect(result.Err).To(BeNil())
				Expect(e.RegFile().ReadReg(0)).To(Equal(uint64(0xAAAAAAAAAAAAAAAA)))
				Expect(e.RegFile().ReadReg(1)).To(Equal(uint64(0xBBBBBBBBBBBBBBBB)))
				Expect(e.RegFile().ReadReg(2)).To(Equal(uint64(0x8010))) // Base updated
			})

			It("should load pair with post-index writeback", func() {
				// LDP X0, X1, [X2], #16
				inst := encodeLDP64PostIndex(0, 1, 2, 16)
				program := uint32ToLEBytes(inst)

				e.Memory().Write64(0x8000, 0xCCCCCCCCCCCCCCCC)
				e.Memory().Write64(0x8008, 0xDDDDDDDDDDDDDDDD)
				e.RegFile().WriteReg(2, 0x8000)
				e.LoadProgram(0x1000, program)

				result := e.Step()

				Expect(result.Err).To(BeNil())
				Expect(e.RegFile().ReadReg(0)).To(Equal(uint64(0xCCCCCCCCCCCCCCCC)))
				Expect(e.RegFile().ReadReg(1)).To(Equal(uint64(0xDDDDDDDDDDDDDDDD)))
				Expect(e.RegFile().ReadReg(2)).To(Equal(uint64(0x8010))) // Base updated after load
			})
		})

		Context("STP - Store Pair", func() {
			It("should store pair of 64-bit values", func() {
				// STP X0, X1, [X2]
				inst := encodeSTP64(0, 1, 2, 0, false)
				program := uint32ToLEBytes(inst)

				e.RegFile().WriteReg(0, 0x1234567890ABCDEF)
				e.RegFile().WriteReg(1, 0xFEDCBA0987654321)
				e.RegFile().WriteReg(2, 0x8000)
				e.LoadProgram(0x1000, program)

				result := e.Step()

				Expect(result.Err).To(BeNil())
				Expect(e.Memory().Read64(0x8000)).To(Equal(uint64(0x1234567890ABCDEF)))
				Expect(e.Memory().Read64(0x8008)).To(Equal(uint64(0xFEDCBA0987654321)))
			})

			It("should store pair of 32-bit values", func() {
				// STP W0, W1, [X2]
				inst := encodeSTP32(0, 1, 2, 0, false)
				program := uint32ToLEBytes(inst)

				e.RegFile().WriteReg(0, 0xAABBCCDD)
				e.RegFile().WriteReg(1, 0x11223344)
				e.RegFile().WriteReg(2, 0x8000)
				e.LoadProgram(0x1000, program)

				result := e.Step()

				Expect(result.Err).To(BeNil())
				Expect(e.Memory().Read32(0x8000)).To(Equal(uint32(0xAABBCCDD)))
				Expect(e.Memory().Read32(0x8004)).To(Equal(uint32(0x11223344)))
			})

			It("should store pair with pre-index writeback", func() {
				// STP X0, X1, [X2, #-16]!
				inst := encodeSTP64PreIndex(0, 1, 2, -16)
				program := uint32ToLEBytes(inst)

				e.RegFile().WriteReg(0, 0x1111111111111111)
				e.RegFile().WriteReg(1, 0x2222222222222222)
				e.RegFile().WriteReg(2, 0x8010)
				e.LoadProgram(0x1000, program)

				result := e.Step()

				Expect(result.Err).To(BeNil())
				Expect(e.Memory().Read64(0x8000)).To(Equal(uint64(0x1111111111111111)))
				Expect(e.Memory().Read64(0x8008)).To(Equal(uint64(0x2222222222222222)))
				Expect(e.RegFile().ReadReg(2)).To(Equal(uint64(0x8000))) // Base decremented
			})
		})
	})

	Describe("PC-Relative Addressing", func() {
		Context("ADR", func() {
			It("should compute PC + offset", func() {
				// ADR X0, #0x100 (PC + 0x100)
				inst := encodeADR(0, 0x100)
				program := uint32ToLEBytes(inst)

				e.LoadProgram(0x1000, program)

				result := e.Step()

				Expect(result.Err).To(BeNil())
				Expect(e.RegFile().ReadReg(0)).To(Equal(uint64(0x1100))) // 0x1000 + 0x100
			})

			It("should handle negative offset", func() {
				// ADR X1, #-0x50
				inst := encodeADR(1, -0x50)
				program := uint32ToLEBytes(inst)

				e.LoadProgram(0x1000, program)

				result := e.Step()

				Expect(result.Err).To(BeNil())
				Expect(e.RegFile().ReadReg(1)).To(Equal(uint64(0x1000 - 0x50)))
			})
		})

		Context("ADRP", func() {
			It("should compute page-aligned PC + shifted offset", func() {
				// ADRP X0, #1 (page base + 1 page)
				inst := encodeADRP(0, 1)
				program := uint32ToLEBytes(inst)

				e.LoadProgram(0x1234, program) // Not page-aligned

				result := e.Step()

				Expect(result.Err).To(BeNil())
				// PC page = 0x1000, result = 0x1000 + 0x1000 = 0x2000
				Expect(e.RegFile().ReadReg(0)).To(Equal(uint64(0x2000)))
			})

			It("should handle page-aligned PC", func() {
				// ADRP X2, #2
				inst := encodeADRP(2, 2)
				program := uint32ToLEBytes(inst)

				e.LoadProgram(0x2000, program)

				result := e.Step()

				Expect(result.Err).To(BeNil())
				// PC page = 0x2000, result = 0x2000 + 0x2000 = 0x4000
				Expect(e.RegFile().ReadReg(2)).To(Equal(uint64(0x4000)))
			})
		})
	})

	Describe("Move Wide", func() {
		Context("MOVZ", func() {
			It("should move zero with immediate at LSL #0", func() {
				// MOVZ X0, #0x1234
				inst := encodeMOVZ64(0, 0x1234, 0)
				program := uint32ToLEBytes(inst)

				e.RegFile().WriteReg(0, 0xFFFFFFFFFFFFFFFF) // Pre-fill with 1s
				e.LoadProgram(0x1000, program)

				result := e.Step()

				Expect(result.Err).To(BeNil())
				Expect(e.RegFile().ReadReg(0)).To(Equal(uint64(0x1234)))
			})

			It("should move zero with immediate at LSL #16", func() {
				// MOVZ X0, #0xABCD, LSL #16
				inst := encodeMOVZ64(0, 0xABCD, 16)
				program := uint32ToLEBytes(inst)

				e.LoadProgram(0x1000, program)

				result := e.Step()

				Expect(result.Err).To(BeNil())
				Expect(e.RegFile().ReadReg(0)).To(Equal(uint64(0xABCD0000)))
			})

			It("should move zero with immediate at LSL #48", func() {
				// MOVZ X0, #0xFFFF, LSL #48
				inst := encodeMOVZ64(0, 0xFFFF, 48)
				program := uint32ToLEBytes(inst)

				e.LoadProgram(0x1000, program)

				result := e.Step()

				Expect(result.Err).To(BeNil())
				Expect(e.RegFile().ReadReg(0)).To(Equal(uint64(0xFFFF000000000000)))
			})
		})

		Context("MOVN", func() {
			It("should move NOT immediate at LSL #0", func() {
				// MOVN X0, #0x1234 -> ~0x1234 = 0xFFFF...EDCB
				inst := encodeMOVN64(0, 0x1234, 0)
				program := uint32ToLEBytes(inst)

				e.LoadProgram(0x1000, program)

				result := e.Step()

				Expect(result.Err).To(BeNil())
				Expect(e.RegFile().ReadReg(0)).To(Equal(^uint64(0x1234)))
			})

			It("should move NOT immediate at LSL #16", func() {
				// MOVN X0, #0, LSL #16 -> ~(0<<16) = 0xFFFFFFFFFFFFFFFF
				inst := encodeMOVN64(0, 0, 16)
				program := uint32ToLEBytes(inst)

				e.LoadProgram(0x1000, program)

				result := e.Step()

				Expect(result.Err).To(BeNil())
				Expect(e.RegFile().ReadReg(0)).To(Equal(^uint64(0)))
			})
		})

		Context("MOVK", func() {
			It("should keep other bits and insert immediate", func() {
				// MOVK X0, #0xABCD, LSL #16 (keeps other bits)
				inst := encodeMOVK64(0, 0xABCD, 16)
				program := uint32ToLEBytes(inst)

				e.RegFile().WriteReg(0, 0x1234567812345678)
				e.LoadProgram(0x1000, program)

				result := e.Step()

				Expect(result.Err).To(BeNil())
				// Keep bits except [31:16], insert 0xABCD there
				expected := uint64(0x12345678ABCD5678)
				Expect(e.RegFile().ReadReg(0)).To(Equal(expected))
			})

			It("should work at LSL #0", func() {
				// MOVK X0, #0xFFFF
				inst := encodeMOVK64(0, 0xFFFF, 0)
				program := uint32ToLEBytes(inst)

				e.RegFile().WriteReg(0, 0xAAAABBBBCCCCDDDD)
				e.LoadProgram(0x1000, program)

				result := e.Step()

				Expect(result.Err).To(BeNil())
				Expect(e.RegFile().ReadReg(0)).To(Equal(uint64(0xAAAABBBBCCCCFFFF)))
			})
		})
	})

	Describe("Emulator Options", func() {
		It("should accept WithStderr option", func() {
			stderrBuf := &bytes.Buffer{}
			emulator := emu.NewEmulator(
				emu.WithStdout(stdoutBuf),
				emu.WithStderr(stderrBuf),
			)
			Expect(emulator).NotTo(BeNil())
		})

		It("should accept WithSyscallHandler option", func() {
			// Test that WithSyscallHandler is a valid option (uses nil handler)
			emulator := emu.NewEmulator(
				emu.WithStdout(stdoutBuf),
				emu.WithSyscallHandler(nil),
			)
			Expect(emulator).NotTo(BeNil())
		})

		It("should reset emulator state", func() {
			e.RegFile().WriteReg(0, 0x12345678)
			e.RegFile().PC = 0x5000
			e.Memory().Write64(0x8000, 0xDEADBEEF)

			e.Reset()

			Expect(e.RegFile().ReadReg(0)).To(Equal(uint64(0)))
			Expect(e.RegFile().PC).To(Equal(uint64(0)))
			Expect(e.InstructionCount()).To(Equal(uint64(0)))
		})
	})
})

// Helper functions for encoding instructions

func uint32ToLEBytes(val uint32) []byte {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, val)
	return buf
}

// LDP 64-bit: 10 101 0 01 1 imm7 Rt2 Rn Rt
func encodeLDP64(rt, rt2, rn uint8, imm int16, preIndex bool) uint32 {
	var inst uint32 = 0
	inst |= 0b10 << 30  // opc = 10 (64-bit)
	inst |= 0b101 << 27 // Fixed bits
	inst |= 0b0 << 26   // V = 0 (general purpose)
	inst |= 0b010 << 23 // Signed offset
	inst |= 0b1 << 22   // L = 1 (Load)
	imm7 := uint32(imm/8) & 0x7F
	inst |= imm7 << 15
	inst |= uint32(rt2&0x1F) << 10
	inst |= uint32(rn&0x1F) << 5
	inst |= uint32(rt & 0x1F)
	return inst
}

// LDP 32-bit: 00 101 0 01 1 imm7 Rt2 Rn Rt
func encodeLDP32(rt, rt2, rn uint8, imm int16, preIndex bool) uint32 {
	var inst uint32 = 0
	inst |= 0b00 << 30  // opc = 00 (32-bit)
	inst |= 0b101 << 27 // Fixed bits
	inst |= 0b0 << 26   // V = 0 (general purpose)
	inst |= 0b010 << 23 // Signed offset
	inst |= 0b1 << 22   // L = 1 (Load)
	imm7 := uint32(imm/4) & 0x7F
	inst |= imm7 << 15
	inst |= uint32(rt2&0x1F) << 10
	inst |= uint32(rn&0x1F) << 5
	inst |= uint32(rt & 0x1F)
	return inst
}

// LDP 64-bit pre-index: 10 101 0 01 1 imm7 Rt2 Rn Rt (opc=11 for pre-index)
func encodeLDP64PreIndex(rt, rt2, rn uint8, imm int16) uint32 {
	var inst uint32 = 0
	inst |= 0b10 << 30  // opc = 10 (64-bit)
	inst |= 0b101 << 27 // Fixed bits
	inst |= 0b0 << 26   // V = 0 (general purpose)
	inst |= 0b011 << 23 // Pre-index
	inst |= 0b1 << 22   // L = 1 (Load)
	imm7 := uint32(imm/8) & 0x7F
	inst |= imm7 << 15
	inst |= uint32(rt2&0x1F) << 10
	inst |= uint32(rn&0x1F) << 5
	inst |= uint32(rt & 0x1F)
	return inst
}

// LDP 64-bit post-index: 10 101 0 00 1 imm7 Rt2 Rn Rt
func encodeLDP64PostIndex(rt, rt2, rn uint8, imm int16) uint32 {
	var inst uint32 = 0
	inst |= 0b10 << 30  // opc = 10 (64-bit)
	inst |= 0b101 << 27 // Fixed bits
	inst |= 0b0 << 26   // V = 0 (general purpose)
	inst |= 0b001 << 23 // Post-index
	inst |= 0b1 << 22   // L = 1 (Load)
	imm7 := uint32(imm/8) & 0x7F
	inst |= imm7 << 15
	inst |= uint32(rt2&0x1F) << 10
	inst |= uint32(rn&0x1F) << 5
	inst |= uint32(rt & 0x1F)
	return inst
}

// STP 64-bit: 10 101 0 01 0 imm7 Rt2 Rn Rt
func encodeSTP64(rt, rt2, rn uint8, imm int16, preIndex bool) uint32 {
	var inst uint32 = 0
	inst |= 0b10 << 30  // opc = 10 (64-bit)
	inst |= 0b101 << 27 // Fixed bits
	inst |= 0b0 << 26   // V = 0 (general purpose)
	inst |= 0b010 << 23 // Signed offset
	inst |= 0b0 << 22   // L = 0 (Store)
	imm7 := uint32(imm/8) & 0x7F
	inst |= imm7 << 15
	inst |= uint32(rt2&0x1F) << 10
	inst |= uint32(rn&0x1F) << 5
	inst |= uint32(rt & 0x1F)
	return inst
}

// STP 32-bit: 00 101 0 01 0 imm7 Rt2 Rn Rt
func encodeSTP32(rt, rt2, rn uint8, imm int16, preIndex bool) uint32 {
	var inst uint32 = 0
	inst |= 0b00 << 30  // opc = 00 (32-bit)
	inst |= 0b101 << 27 // Fixed bits
	inst |= 0b0 << 26   // V = 0 (general purpose)
	inst |= 0b010 << 23 // Signed offset
	inst |= 0b0 << 22   // L = 0 (Store)
	imm7 := uint32(imm/4) & 0x7F
	inst |= imm7 << 15
	inst |= uint32(rt2&0x1F) << 10
	inst |= uint32(rn&0x1F) << 5
	inst |= uint32(rt & 0x1F)
	return inst
}

// STP 64-bit pre-index
func encodeSTP64PreIndex(rt, rt2, rn uint8, imm int16) uint32 {
	var inst uint32 = 0
	inst |= 0b10 << 30  // opc = 10 (64-bit)
	inst |= 0b101 << 27 // Fixed bits
	inst |= 0b0 << 26   // V = 0 (general purpose)
	inst |= 0b011 << 23 // Pre-index
	inst |= 0b0 << 22   // L = 0 (Store)
	imm7 := uint32(imm/8) & 0x7F
	inst |= imm7 << 15
	inst |= uint32(rt2&0x1F) << 10
	inst |= uint32(rn&0x1F) << 5
	inst |= uint32(rt & 0x1F)
	return inst
}

// ADR: 0 immlo op(0) 10000 immhi Rd
func encodeADR(rd uint8, offset int64) uint32 {
	var inst uint32 = 0
	inst |= 0b0 << 31 // op = 0 (ADR)
	immlo := uint32(offset) & 0x3
	immhi := uint32(offset>>2) & 0x7FFFF
	inst |= immlo << 29
	inst |= 0b10000 << 24
	inst |= immhi << 5
	inst |= uint32(rd & 0x1F)
	return inst
}

// ADRP: 1 immlo op(0) 10000 immhi Rd (offset in pages)
func encodeADRP(rd uint8, pages int64) uint32 {
	var inst uint32 = 0
	inst |= 0b1 << 31 // op = 1 (ADRP)
	immlo := uint32(pages) & 0x3
	immhi := uint32(pages>>2) & 0x7FFFF
	inst |= immlo << 29
	inst |= 0b10000 << 24
	inst |= immhi << 5
	inst |= uint32(rd & 0x1F)
	return inst
}

// MOVZ 64-bit: 1 10 100101 hw imm16 Rd
func encodeMOVZ64(rd uint8, imm16 uint16, shift uint8) uint32 {
	var inst uint32 = 0
	inst |= 0b1 << 31      // sf = 1 (64-bit)
	inst |= 0b10 << 29     // opc = 10 (MOVZ)
	inst |= 0b100101 << 23 // Fixed bits
	hw := shift / 16
	inst |= uint32(hw) << 21
	inst |= uint32(imm16) << 5
	inst |= uint32(rd & 0x1F)
	return inst
}

// MOVN 64-bit: 1 00 100101 hw imm16 Rd
func encodeMOVN64(rd uint8, imm16 uint16, shift uint8) uint32 {
	var inst uint32 = 0
	inst |= 0b1 << 31      // sf = 1 (64-bit)
	inst |= 0b00 << 29     // opc = 00 (MOVN)
	inst |= 0b100101 << 23 // Fixed bits
	hw := shift / 16
	inst |= uint32(hw) << 21
	inst |= uint32(imm16) << 5
	inst |= uint32(rd & 0x1F)
	return inst
}

// MOVK 64-bit: 1 11 100101 hw imm16 Rd
func encodeMOVK64(rd uint8, imm16 uint16, shift uint8) uint32 {
	var inst uint32 = 0
	inst |= 0b1 << 31      // sf = 1 (64-bit)
	inst |= 0b11 << 29     // opc = 11 (MOVK)
	inst |= 0b100101 << 23 // Fixed bits
	hw := shift / 16
	inst |= uint32(hw) << 21
	inst |= uint32(imm16) << 5
	inst |= uint32(rd & 0x1F)
	return inst
}

// Helper to suppress unused variable warnings
var _ = insts.OpADR
