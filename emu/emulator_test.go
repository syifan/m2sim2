package emu_test

import (
	"bytes"
	"encoding/binary"
	"math"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/sarchlab/m2sim/emu"
	"github.com/sarchlab/m2sim/insts"
)

var _ = Describe("Emulator", func() {
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

	Describe("NewEmulator", func() {
		It("should create an emulator with initialized components", func() {
			Expect(e).NotTo(BeNil())
			Expect(e.RegFile()).NotTo(BeNil())
			Expect(e.Memory()).NotTo(BeNil())
		})
	})

	Describe("LoadProgram", func() {
		It("should set the PC to the entry point", func() {
			entryPoint := uint64(0x1000)
			program := []byte{0x00, 0x00, 0x00, 0x00}

			e.LoadProgram(entryPoint, program)

			Expect(e.RegFile().PC).To(Equal(entryPoint))
		})

		It("should load program bytes into memory", func() {
			entryPoint := uint64(0x2000)
			program := []byte{0xDE, 0xAD, 0xBE, 0xEF}

			e.LoadProgram(entryPoint, program)

			Expect(e.Memory().Read8(0x2000)).To(Equal(byte(0xDE)))
			Expect(e.Memory().Read8(0x2001)).To(Equal(byte(0xAD)))
			Expect(e.Memory().Read8(0x2002)).To(Equal(byte(0xBE)))
			Expect(e.Memory().Read8(0x2003)).To(Equal(byte(0xEF)))
		})
	})

	Describe("Step", func() {
		Context("ALU instructions", func() {
			It("should execute ADD immediate instruction", func() {
				inst := encodeADDImm(0, 1, 5, false)
				program := uint32ToBytes(inst)

				e.RegFile().WriteReg(1, 10)
				e.LoadProgram(0x1000, program)

				result := e.Step()

				Expect(result.Err).To(BeNil())
				Expect(result.Exited).To(BeFalse())
				Expect(e.RegFile().ReadReg(0)).To(Equal(uint64(15)))
				Expect(e.RegFile().PC).To(Equal(uint64(0x1004)))
			})

			It("should execute SUB immediate instruction", func() {
				inst := encodeSUBImm(0, 1, 3, false)
				program := uint32ToBytes(inst)

				e.RegFile().WriteReg(1, 10)
				e.LoadProgram(0x1000, program)

				result := e.Step()

				Expect(result.Err).To(BeNil())
				Expect(result.Exited).To(BeFalse())
				Expect(e.RegFile().ReadReg(0)).To(Equal(uint64(7)))
			})

			It("should execute ADD register instruction", func() {
				inst := encodeADDReg(0, 1, 2, false)
				program := uint32ToBytes(inst)

				e.RegFile().WriteReg(1, 10)
				e.RegFile().WriteReg(2, 5)
				e.LoadProgram(0x1000, program)

				result := e.Step()

				Expect(result.Err).To(BeNil())
				Expect(result.Exited).To(BeFalse())
				Expect(e.RegFile().ReadReg(0)).To(Equal(uint64(15)))
			})

			It("should execute ADDS and set flags", func() {
				inst := encodeADDImm(0, 1, 0, true)
				program := uint32ToBytes(inst)

				e.RegFile().WriteReg(1, 0)
				e.LoadProgram(0x1000, program)

				e.Step()

				Expect(e.RegFile().ReadReg(0)).To(Equal(uint64(0)))
				Expect(e.RegFile().PSTATE.Z).To(BeTrue())
			})
		})

		Context("Load/Store instructions", func() {
			It("should execute LDR (64-bit)", func() {
				inst := encodeLDR64(0, 1, 8)
				program := uint32ToBytes(inst)

				e.RegFile().WriteReg(1, 0x2000)
				e.Memory().Write64(0x2008, 0xDEADBEEFCAFEBABE)
				e.LoadProgram(0x1000, program)

				e.Step()

				Expect(e.RegFile().ReadReg(0)).To(Equal(uint64(0xDEADBEEFCAFEBABE)))
			})

			It("should execute STR (64-bit)", func() {
				inst := encodeSTR64(0, 1, 16)
				program := uint32ToBytes(inst)

				e.RegFile().WriteReg(0, 0x123456789ABCDEF0)
				e.RegFile().WriteReg(1, 0x3000)
				e.LoadProgram(0x1000, program)

				e.Step()

				Expect(e.Memory().Read64(0x3010)).To(Equal(uint64(0x123456789ABCDEF0)))
			})
		})

		Context("Branch instructions", func() {
			It("should execute B (unconditional branch)", func() {
				inst := encodeB(8)
				program := uint32ToBytes(inst)

				e.LoadProgram(0x1000, program)

				e.Step()

				Expect(e.RegFile().PC).To(Equal(uint64(0x1008)))
			})

			It("should execute BL (branch with link)", func() {
				inst := encodeBL(12)
				program := uint32ToBytes(inst)

				e.LoadProgram(0x1000, program)

				e.Step()

				Expect(e.RegFile().PC).To(Equal(uint64(0x100C)))
				Expect(e.RegFile().ReadReg(30)).To(Equal(uint64(0x1004)))
			})

			It("should execute B.EQ when Z flag is set", func() {
				inst := encodeBCond(8, insts.CondEQ)
				program := uint32ToBytes(inst)

				e.RegFile().PSTATE.Z = true
				e.LoadProgram(0x1000, program)

				e.Step()

				Expect(e.RegFile().PC).To(Equal(uint64(0x1008)))
			})

			It("should not branch B.EQ when Z flag is clear", func() {
				inst := encodeBCond(8, insts.CondEQ)
				program := uint32ToBytes(inst)

				e.RegFile().PSTATE.Z = false
				e.LoadProgram(0x1000, program)

				e.Step()

				Expect(e.RegFile().PC).To(Equal(uint64(0x1004)))
			})

			It("should execute RET", func() {
				inst := encodeRET()
				program := uint32ToBytes(inst)

				e.RegFile().WriteReg(30, 0x2000)
				e.LoadProgram(0x1000, program)

				e.Step()

				Expect(e.RegFile().PC).To(Equal(uint64(0x2000)))
			})
		})

		Context("SVC instruction", func() {
			It("should handle exit syscall", func() {
				inst := encodeSVC(0)
				program := uint32ToBytes(inst)

				e.RegFile().WriteReg(8, emu.SyscallExit)
				e.RegFile().WriteReg(0, 42)
				e.LoadProgram(0x1000, program)

				result := e.Step()

				Expect(result.Exited).To(BeTrue())
				Expect(result.ExitCode).To(Equal(int64(42)))
			})

			It("should handle write syscall", func() {
				msg := []byte("Hello")
				bufAddr := uint64(0x3000)
				for i, b := range msg {
					e.Memory().Write8(bufAddr+uint64(i), b)
				}

				inst := encodeSVC(0)
				program := uint32ToBytes(inst)

				e.RegFile().WriteReg(8, emu.SyscallWrite)
				e.RegFile().WriteReg(0, 1)
				e.RegFile().WriteReg(1, bufAddr)
				e.RegFile().WriteReg(2, uint64(len(msg)))
				e.LoadProgram(0x1000, program)

				result := e.Step()

				Expect(result.Exited).To(BeFalse())
				Expect(stdoutBuf.String()).To(Equal("Hello"))
			})
		})

		Context("Unknown instructions", func() {
			It("should return error for unknown instruction", func() {
				// Use an instruction pattern that doesn't match any decoder
				// 0x00000001 is not a valid ARM64 instruction
				program := uint32ToBytes(0x00000001)

				e.LoadProgram(0x1000, program)

				result := e.Step()

				Expect(result.Err).NotTo(BeNil())
				Expect(result.Err.Error()).To(ContainSubstring("unknown"))
			})
		})
	})

	Describe("Run", func() {
		It("should execute until exit syscall", func() {
			program := []byte{}
			program = append(program, uint32ToBytes(encodeADDImm(8, 31, 93, false))...)
			program = append(program, uint32ToBytes(encodeADDImm(0, 31, 42, false))...)
			program = append(program, uint32ToBytes(encodeSVC(0))...)

			e.LoadProgram(0x1000, program)

			exitCode := e.Run()

			Expect(exitCode).To(Equal(int64(42)))
		})

		It("should execute a simple computation before exit", func() {
			program := []byte{}
			program = append(program, uint32ToBytes(encodeADDImm(0, 31, 10, false))...)
			program = append(program, uint32ToBytes(encodeADDImm(1, 31, 5, false))...)
			program = append(program, uint32ToBytes(encodeADDReg(0, 0, 1, false))...)
			program = append(program, uint32ToBytes(encodeADDImm(8, 31, 93, false))...)
			program = append(program, uint32ToBytes(encodeSVC(0))...)

			e.LoadProgram(0x1000, program)

			exitCode := e.Run()

			Expect(exitCode).To(Equal(int64(15)))
		})

		It("should handle branches in a loop", func() {
			program := []byte{}
			program = append(program, uint32ToBytes(encodeADDImm(0, 31, 3, false))...)
			program = append(program, uint32ToBytes(encodeSUBImm(0, 0, 1, true))...)
			program = append(program, uint32ToBytes(encodeBCond(-4, insts.CondNE))...)
			program = append(program, uint32ToBytes(encodeADDImm(8, 31, 93, false))...)
			program = append(program, uint32ToBytes(encodeSVC(0))...)

			e.LoadProgram(0x1000, program)

			exitCode := e.Run()

			Expect(exitCode).To(Equal(int64(0)))
		})

		It("should write output during execution", func() {
			e.Memory().Write8(0x3000, 'H')
			e.Memory().Write8(0x3001, 'i')

			program := []byte{}
			program = append(program, uint32ToBytes(encodeADDImm(8, 31, 64, false))...)
			program = append(program, uint32ToBytes(encodeADDImm(0, 31, 1, false))...)

			e.RegFile().WriteReg(1, 0x3000)

			program = append(program, uint32ToBytes(encodeADDImm(2, 31, 2, false))...)
			program = append(program, uint32ToBytes(encodeSVC(0))...)
			program = append(program, uint32ToBytes(encodeADDImm(8, 31, 93, false))...)
			program = append(program, uint32ToBytes(encodeADDImm(0, 31, 0, false))...)
			program = append(program, uint32ToBytes(encodeSVC(0))...)

			e.LoadProgram(0x1000, program)

			exitCode := e.Run()

			Expect(exitCode).To(Equal(int64(0)))
			Expect(stdoutBuf.String()).To(Equal("Hi"))
		})
	})

	Describe("WithStackPointer option", func() {
		It("should set initial stack pointer", func() {
			spValue := uint64(0x7FFFFF00)
			e = emu.NewEmulator(
				emu.WithStackPointer(spValue),
			)

			Expect(e.RegFile().SP).To(Equal(spValue))
		})
	})
})

// Helper functions to encode ARM64 instructions

func uint32ToBytes(v uint32) []byte {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, v)
	return buf
}

func encodeADDImm(rd, rn uint8, imm uint16, setFlags bool) uint32 {
	var inst uint32 = 0
	inst |= 1 << 31
	inst |= 0 << 30
	if setFlags {
		inst |= 1 << 29
	}
	inst |= 0b100010 << 23
	inst |= 0 << 22
	inst |= uint32(imm&0xFFF) << 10
	inst |= uint32(rn&0x1F) << 5
	inst |= uint32(rd & 0x1F)
	return inst
}

func encodeSUBImm(rd, rn uint8, imm uint16, setFlags bool) uint32 {
	var inst uint32 = 0
	inst |= 1 << 31
	inst |= 1 << 30
	if setFlags {
		inst |= 1 << 29
	}
	inst |= 0b100010 << 23
	inst |= 0 << 22
	inst |= uint32(imm&0xFFF) << 10
	inst |= uint32(rn&0x1F) << 5
	inst |= uint32(rd & 0x1F)
	return inst
}

func encodeADDReg(rd, rn, rm uint8, setFlags bool) uint32 {
	var inst uint32 = 0
	inst |= 1 << 31
	inst |= 0 << 30
	if setFlags {
		inst |= 1 << 29
	}
	inst |= 0b01011 << 24
	inst |= 0 << 22
	inst |= 0 << 21
	inst |= uint32(rm&0x1F) << 16
	inst |= 0 << 10
	inst |= uint32(rn&0x1F) << 5
	inst |= uint32(rd & 0x1F)
	return inst
}

func encodeLDR64(rd, rn uint8, offset uint16) uint32 {
	var inst uint32 = 0
	inst |= 0b11 << 30
	inst |= 0b111 << 27
	inst |= 0 << 26
	inst |= 0b01 << 24
	inst |= 0b01 << 22
	scaledOffset := offset / 8
	inst |= uint32(scaledOffset&0xFFF) << 10
	inst |= uint32(rn&0x1F) << 5
	inst |= uint32(rd & 0x1F)
	return inst
}

func encodeSTR64(rd, rn uint8, offset uint16) uint32 {
	var inst uint32 = 0
	inst |= 0b11 << 30
	inst |= 0b111 << 27
	inst |= 0 << 26
	inst |= 0b01 << 24
	inst |= 0b00 << 22
	scaledOffset := offset / 8
	inst |= uint32(scaledOffset&0xFFF) << 10
	inst |= uint32(rn&0x1F) << 5
	inst |= uint32(rd & 0x1F)
	return inst
}

func encodeB(offset int32) uint32 {
	var inst uint32 = 0
	inst |= 0b000101 << 26
	imm26 := uint32(offset/4) & 0x3FFFFFF
	inst |= imm26
	return inst
}

func encodeBL(offset int32) uint32 {
	var inst uint32 = 0
	inst |= 0b100101 << 26
	imm26 := uint32(offset/4) & 0x3FFFFFF
	inst |= imm26
	return inst
}

func encodeBCond(offset int32, cond insts.Cond) uint32 {
	var inst uint32 = 0
	inst |= 0b0101010 << 25
	inst |= 0 << 24
	imm19 := uint32(offset/4) & 0x7FFFF
	inst |= imm19 << 5
	inst |= 0 << 4
	inst |= uint32(cond & 0xF)
	return inst
}

func encodeRET() uint32 {
	var inst uint32 = 0
	inst |= 0b1101011 << 25
	inst |= 0 << 24
	inst |= 0 << 23
	inst |= 0b10 << 21
	inst |= 0b11111 << 16
	inst |= 0b0000 << 12
	inst |= 0 << 11
	inst |= 0 << 10
	inst |= uint32(30) << 5
	inst |= 0b00000
	return inst
}

var _ = Describe("Emulator SIMD Dispatch", func() {
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

	It("should dispatch SIMD VADD through the emulator", func() {
		// Set up SIMD registers with known values
		e.SIMDRegFile().WriteLane32(1, 0, 10)
		e.SIMDRegFile().WriteLane32(1, 1, 20)
		e.SIMDRegFile().WriteLane32(2, 0, 3)
		e.SIMDRegFile().WriteLane32(2, 1, 7)

		// ADD V0.4S, V1.4S, V2.4S -> 0x4EA28420
		program := make([]byte, 8)
		binary.LittleEndian.PutUint32(program[0:4], 0x4EA28420)
		binary.LittleEndian.PutUint32(program[4:8], encodeSVC(0))

		e.RegFile().WriteReg(8, 93) // SyscallExit
		e.RegFile().WriteReg(0, 0)
		e.LoadProgram(0x1000, program)
		e.Step()

		Expect(e.SIMDRegFile().ReadLane32(0, 0)).To(Equal(uint32(13)))
		Expect(e.SIMDRegFile().ReadLane32(0, 1)).To(Equal(uint32(27)))
	})

	It("should dispatch SIMD LDR Q through the emulator", func() {
		e.Memory().Write64(0x2000, 0x0807060504030201)
		e.Memory().Write64(0x2008, 0x100F0E0D0C0B0A09)
		e.RegFile().WriteReg(1, 0x2000)

		// LDR Q0, [X1] -> 0x3DC00020
		program := make([]byte, 8)
		binary.LittleEndian.PutUint32(program[0:4], 0x3DC00020)
		binary.LittleEndian.PutUint32(program[4:8], encodeSVC(0))

		e.RegFile().WriteReg(8, 93)
		e.RegFile().WriteReg(0, 0)
		e.LoadProgram(0x1000, program)
		e.Step()

		low, high := e.SIMDRegFile().ReadQ(0)
		Expect(low).To(Equal(uint64(0x0807060504030201)))
		Expect(high).To(Equal(uint64(0x100F0E0D0C0B0A09)))
	})

	It("should dispatch VFADD through the emulator", func() {
		e.SIMDRegFile().WriteLane32(1, 0, math.Float32bits(1.5))
		e.SIMDRegFile().WriteLane32(1, 1, math.Float32bits(2.5))
		e.SIMDRegFile().WriteLane32(2, 0, math.Float32bits(3.0))
		e.SIMDRegFile().WriteLane32(2, 1, math.Float32bits(4.0))

		// FADD V0.4S, V1.4S, V2.4S -> 0x4EA2D420
		program := make([]byte, 8)
		binary.LittleEndian.PutUint32(program[0:4], 0x4EA2D420)
		binary.LittleEndian.PutUint32(program[4:8], encodeSVC(0))

		e.RegFile().WriteReg(8, 93)
		e.RegFile().WriteReg(0, 0)
		e.LoadProgram(0x1000, program)
		e.Step()

		result0 := math.Float32frombits(e.SIMDRegFile().ReadLane32(0, 0))
		result1 := math.Float32frombits(e.SIMDRegFile().ReadLane32(0, 1))
		Expect(result0).To(BeNumerically("~", 4.5, 0.001))
		Expect(result1).To(BeNumerically("~", 6.5, 0.001))
	})
})

func encodeSVC(imm uint16) uint32 {
	var inst uint32 = 0
	inst |= 0b11010100 << 24
	inst |= 0b000 << 21
	inst |= uint32(imm) << 5
	inst |= 0b00001
	return inst
}
