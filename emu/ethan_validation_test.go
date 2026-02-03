// Package emu provides the Ethan validation suite for M2Sim emulator.
// This test suite establishes the regression baseline before M3 timing model integration.
// Run with: go test ./emu/... -run "Ethan" -v
package emu_test

import (
	"bytes"
	"encoding/binary"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/sarchlab/m2sim/emu"
	"github.com/sarchlab/m2sim/insts"
)

// ValidationResult captures the result of a validation test
type ValidationResult struct {
	Name             string
	ExitCode         int64
	ExpectedExitCode int64
	Output           string
	ExpectedOutput   string
	InstructionCount uint64
	Pass             bool
}

var validationResults []ValidationResult

var _ = AfterSuite(func() {
	// Print validation summary
	fmt.Println("\n========================================")
	fmt.Println("M2Sim Ethan Validation Baseline Summary")
	fmt.Println("========================================")

	allPassed := true
	for _, r := range validationResults {
		status := "✓"
		if !r.Pass {
			status = "✗"
			allPassed = false
		}
		fmt.Printf("%s %-20s: exit=%d (expected=%d)", status, r.Name, r.ExitCode, r.ExpectedExitCode)
		if r.ExpectedOutput != "" {
			fmt.Printf(", output=%q", r.Output)
		}
		fmt.Printf(", insts=%d\n", r.InstructionCount)
	}

	fmt.Println("========================================")
	if allPassed {
		fmt.Println("All Ethan validation tests PASSED!")
		fmt.Println("Baseline established for M3 timing work.")
	} else {
		fmt.Println("Some validation tests FAILED!")
		fmt.Println("Fix issues before proceeding with M3.")
	}
	fmt.Println("========================================")
})

var _ = Describe("Ethan Validation Suite", func() {
	var (
		e         *emu.Emulator
		stdoutBuf *bytes.Buffer
	)

	BeforeEach(func() {
		stdoutBuf = &bytes.Buffer{}
		e = emu.NewEmulator(
			emu.WithStdout(stdoutBuf),
			emu.WithStackPointer(0x7FFF0000),
		)
	})

	Describe("Baseline Validation Programs", func() {
		Context("simple_exit: Basic program termination", func() {
			It("should exit with code 42", func() {
				program := []byte{}
				program = append(program, ethanEncodeInstBytes(ethanEncodeADDImm(8, 31, 93, false))...)
				program = append(program, ethanEncodeInstBytes(ethanEncodeADDImm(0, 31, 42, false))...)
				program = append(program, ethanEncodeInstBytes(ethanEncodeSVC(0))...)

				e.LoadProgram(0x1000, program)
				exitCode := e.Run()

				result := ValidationResult{
					Name:             "simple_exit",
					ExitCode:         exitCode,
					ExpectedExitCode: 42,
					InstructionCount: e.InstructionCount(),
					Pass:             exitCode == 42,
				}
				validationResults = append(validationResults, result)

				Expect(exitCode).To(Equal(int64(42)))
				Expect(e.InstructionCount()).To(Equal(uint64(3)))
			})
		})

		Context("arithmetic: ADD/SUB operations", func() {
			It("should compute 10 + 5 = 15", func() {
				program := []byte{}
				program = append(program, ethanEncodeInstBytes(ethanEncodeADDImm(0, 31, 10, false))...)
				program = append(program, ethanEncodeInstBytes(ethanEncodeADDImm(1, 31, 5, false))...)
				program = append(program, ethanEncodeInstBytes(ethanEncodeADDReg(0, 0, 1, false))...)
				program = append(program, ethanEncodeInstBytes(ethanEncodeADDImm(8, 31, 93, false))...)
				program = append(program, ethanEncodeInstBytes(ethanEncodeSVC(0))...)

				e.LoadProgram(0x1000, program)
				exitCode := e.Run()

				result := ValidationResult{
					Name:             "arithmetic",
					ExitCode:         exitCode,
					ExpectedExitCode: 15,
					InstructionCount: e.InstructionCount(),
					Pass:             exitCode == 15,
				}
				validationResults = append(validationResults, result)

				Expect(exitCode).To(Equal(int64(15)))
				Expect(e.InstructionCount()).To(Equal(uint64(5)))
			})
		})

		Context("subtraction: SUB operation", func() {
			It("should compute 100 - 58 = 42", func() {
				program := []byte{}
				program = append(program, ethanEncodeInstBytes(ethanEncodeADDImm(0, 31, 100, false))...)
				program = append(program, ethanEncodeInstBytes(ethanEncodeSUBImm(0, 0, 58, false))...)
				program = append(program, ethanEncodeInstBytes(ethanEncodeADDImm(8, 31, 93, false))...)
				program = append(program, ethanEncodeInstBytes(ethanEncodeSVC(0))...)

				e.LoadProgram(0x1000, program)
				exitCode := e.Run()

				result := ValidationResult{
					Name:             "subtraction",
					ExitCode:         exitCode,
					ExpectedExitCode: 42,
					InstructionCount: e.InstructionCount(),
					Pass:             exitCode == 42,
				}
				validationResults = append(validationResults, result)

				Expect(exitCode).To(Equal(int64(42)))
				Expect(e.InstructionCount()).To(Equal(uint64(4)))
			})
		})

		Context("loop: Conditional branch loop", func() {
			It("should count down from 3 to 0", func() {
				program := []byte{}
				program = append(program, ethanEncodeInstBytes(ethanEncodeADDImm(0, 31, 3, false))...)
				program = append(program, ethanEncodeInstBytes(ethanEncodeSUBImm(0, 0, 1, true))...)
				program = append(program, ethanEncodeInstBytes(ethanEncodeBCond(-4, insts.CondNE))...)
				program = append(program, ethanEncodeInstBytes(ethanEncodeADDImm(8, 31, 93, false))...)
				program = append(program, ethanEncodeInstBytes(ethanEncodeSVC(0))...)

				e.LoadProgram(0x1000, program)
				exitCode := e.Run()

				result := ValidationResult{
					Name:             "loop",
					ExitCode:         exitCode,
					ExpectedExitCode: 0,
					InstructionCount: e.InstructionCount(),
					Pass:             exitCode == 0,
				}
				validationResults = append(validationResults, result)

				Expect(exitCode).To(Equal(int64(0)))
			})
		})

		Context("loop_sum: Sum 1+2+3+4+5 = 15", func() {
			It("should compute sum correctly", func() {
				program := []byte{}
				program = append(program, ethanEncodeInstBytes(ethanEncodeADDImm(0, 31, 5, false))...) // counter = 5
				program = append(program, ethanEncodeInstBytes(ethanEncodeADDImm(1, 31, 0, false))...) // sum = 0
				// loop:
				program = append(program, ethanEncodeInstBytes(ethanEncodeADDReg(1, 1, 0, false))...)  // sum += counter
				program = append(program, ethanEncodeInstBytes(ethanEncodeSUBImm(0, 0, 1, true))...)   // counter-- (set flags)
				program = append(program, ethanEncodeInstBytes(ethanEncodeBCond(-8, insts.CondNE))...) // if counter != 0, goto loop
				// done:
				program = append(program, ethanEncodeInstBytes(ethanEncodeADDReg(0, 31, 1, false))...)  // x0 = sum
				program = append(program, ethanEncodeInstBytes(ethanEncodeADDImm(8, 31, 93, false))...) // x8 = exit
				program = append(program, ethanEncodeInstBytes(ethanEncodeSVC(0))...)

				e.LoadProgram(0x1000, program)
				exitCode := e.Run()

				result := ValidationResult{
					Name:             "loop_sum",
					ExitCode:         exitCode,
					ExpectedExitCode: 15,
					InstructionCount: e.InstructionCount(),
					Pass:             exitCode == 15,
				}
				validationResults = append(validationResults, result)

				Expect(exitCode).To(Equal(int64(15)))
			})
		})

		Context("hello: Write syscall", func() {
			It("should output 'Hello\\n'", func() {
				msg := []byte("Hello\n")
				bufAddr := uint64(0x3000)
				for i, b := range msg {
					e.Memory().Write8(bufAddr+uint64(i), b)
				}

				program := []byte{}
				program = append(program, ethanEncodeInstBytes(ethanEncodeADDImm(8, 31, 64, false))...)
				program = append(program, ethanEncodeInstBytes(ethanEncodeADDImm(0, 31, 1, false))...)
				program = append(program, ethanEncodeInstBytes(ethanEncodeADDImmShift(1, 31, 3, 12))...)
				program = append(program, ethanEncodeInstBytes(ethanEncodeADDImm(2, 31, 6, false))...)
				program = append(program, ethanEncodeInstBytes(ethanEncodeSVC(0))...)
				program = append(program, ethanEncodeInstBytes(ethanEncodeADDImm(8, 31, 93, false))...)
				program = append(program, ethanEncodeInstBytes(ethanEncodeADDImm(0, 31, 0, false))...)
				program = append(program, ethanEncodeInstBytes(ethanEncodeSVC(0))...)

				e.LoadProgram(0x1000, program)
				exitCode := e.Run()

				result := ValidationResult{
					Name:             "hello",
					ExitCode:         exitCode,
					ExpectedExitCode: 0,
					Output:           stdoutBuf.String(),
					ExpectedOutput:   "Hello\n",
					InstructionCount: e.InstructionCount(),
					Pass:             exitCode == 0 && stdoutBuf.String() == "Hello\n",
				}
				validationResults = append(validationResults, result)

				Expect(exitCode).To(Equal(int64(0)))
				Expect(stdoutBuf.String()).To(Equal("Hello\n"))
			})
		})

		Context("function_call: BL/RET", func() {
			It("should call subroutine and return", func() {
				program := []byte{}
				program = append(program, ethanEncodeInstBytes(ethanEncodeADDImm(0, 31, 10, false))...)
				program = append(program, ethanEncodeInstBytes(ethanEncodeBL(12))...)
				program = append(program, ethanEncodeInstBytes(ethanEncodeADDImm(8, 31, 93, false))...)
				program = append(program, ethanEncodeInstBytes(ethanEncodeSVC(0))...)
				// add_five:
				program = append(program, ethanEncodeInstBytes(ethanEncodeADDImm(0, 0, 5, false))...)
				program = append(program, ethanEncodeInstBytes(ethanEncodeRET())...)

				e.LoadProgram(0x1000, program)
				exitCode := e.Run()

				result := ValidationResult{
					Name:             "function_call",
					ExitCode:         exitCode,
					ExpectedExitCode: 15,
					InstructionCount: e.InstructionCount(),
					Pass:             exitCode == 15,
				}
				validationResults = append(validationResults, result)

				Expect(exitCode).To(Equal(int64(15)))
			})
		})

		Context("nested_calls: Nested BL/RET using register save", func() {
			It("should handle nested function calls", func() {
				// main -> outer(10) -> inner(15) -> return 20 -> return 25 -> exit 35
				// Uses x19 to save LR instead of stack (callee-saved register)
				program := []byte{}

				// main: (0x1000)
				program = append(program, ethanEncodeInstBytes(ethanEncodeADDImm(0, 31, 10, false))...) // x0 = 10
				program = append(program, ethanEncodeInstBytes(ethanEncodeBL(12))...)                   // bl outer (+12 bytes)
				program = append(program, ethanEncodeInstBytes(ethanEncodeADDImm(8, 31, 93, false))...) // x8 = 93
				program = append(program, ethanEncodeInstBytes(ethanEncodeSVC(0))...)                   // exit(x0)

				// outer: (0x1010 = main + 16 bytes)
				program = append(program, ethanEncodeInstBytes(ethanEncodeADDImm(0, 0, 5, false))...)    // x0 += 5 (now 15)
				program = append(program, ethanEncodeInstBytes(ethanEncodeADDReg(19, 31, 30, false))...) // x19 = x30 (save LR)
				program = append(program, ethanEncodeInstBytes(ethanEncodeBL(16))...)                    // bl inner (+16 bytes)
				program = append(program, ethanEncodeInstBytes(ethanEncodeADDReg(30, 31, 19, false))...) // x30 = x19 (restore LR)
				program = append(program, ethanEncodeInstBytes(ethanEncodeADDImm(0, 0, 5, false))...)    // x0 += 5 (now 30)
				program = append(program, ethanEncodeInstBytes(ethanEncodeRET())...)                     // return

				// inner: (0x1028 = outer + 24 bytes)
				program = append(program, ethanEncodeInstBytes(ethanEncodeADDImm(0, 0, 5, false))...) // x0 += 5 (now 20)
				program = append(program, ethanEncodeInstBytes(ethanEncodeRET())...)                  // return

				e.LoadProgram(0x1000, program)
				exitCode := e.Run()

				// 10 + 5 (outer) + 5 (inner) + 5 (outer again) = 25
				result := ValidationResult{
					Name:             "nested_calls",
					ExitCode:         exitCode,
					ExpectedExitCode: 25,
					InstructionCount: e.InstructionCount(),
					Pass:             exitCode == 25,
				}
				validationResults = append(validationResults, result)

				Expect(exitCode).To(Equal(int64(25)))
			})
		})

		Context("logical_ops: AND/ORR/EOR", func() {
			It("should compute bitwise operations", func() {
				program := []byte{}
				program = append(program, ethanEncodeInstBytes(ethanEncodeADDImm(0, 31, 0xFF, false))...)
				program = append(program, ethanEncodeInstBytes(ethanEncodeADDImm(1, 31, 0xF0, false))...)
				program = append(program, ethanEncodeInstBytes(ethanEncodeANDReg(2, 0, 1))...)
				program = append(program, ethanEncodeInstBytes(ethanEncodeADDImm(3, 31, 0x0F, false))...)
				program = append(program, ethanEncodeInstBytes(ethanEncodeORRReg(0, 2, 3))...)
				program = append(program, ethanEncodeInstBytes(ethanEncodeADDImm(8, 31, 93, false))...)
				program = append(program, ethanEncodeInstBytes(ethanEncodeSVC(0))...)

				e.LoadProgram(0x1000, program)
				exitCode := e.Run()

				// 0xFF & 0xF0 = 0xF0, 0xF0 | 0x0F = 0xFF
				result := ValidationResult{
					Name:             "logical_ops",
					ExitCode:         exitCode,
					ExpectedExitCode: 255,
					InstructionCount: e.InstructionCount(),
					Pass:             exitCode == 255,
				}
				validationResults = append(validationResults, result)

				Expect(exitCode).To(Equal(int64(255)))
			})
		})

		Context("memory_ops: LDR/STR", func() {
			It("should store and load values", func() {
				program := []byte{}
				program = append(program, ethanEncodeInstBytes(ethanEncodeADDImmShift(2, 31, 4, 12))...)
				program = append(program, ethanEncodeInstBytes(ethanEncodeADDImm(0, 31, 77, false))...)
				program = append(program, ethanEncodeInstBytes(ethanEncodeSTR64(0, 2, 0))...)
				program = append(program, ethanEncodeInstBytes(ethanEncodeADDImm(0, 31, 0, false))...)
				program = append(program, ethanEncodeInstBytes(ethanEncodeLDR64(0, 2, 0))...)
				program = append(program, ethanEncodeInstBytes(ethanEncodeADDImm(8, 31, 93, false))...)
				program = append(program, ethanEncodeInstBytes(ethanEncodeSVC(0))...)

				e.LoadProgram(0x1000, program)
				exitCode := e.Run()

				result := ValidationResult{
					Name:             "memory_ops",
					ExitCode:         exitCode,
					ExpectedExitCode: 77,
					InstructionCount: e.InstructionCount(),
					Pass:             exitCode == 77,
				}
				validationResults = append(validationResults, result)

				Expect(exitCode).To(Equal(int64(77)))
			})
		})

		Context("conditional_branches: B.cond", func() {
			It("should take B.EQ when equal", func() {
				program := []byte{}
				program = append(program, ethanEncodeInstBytes(ethanEncodeADDImm(0, 31, 5, false))...)
				program = append(program, ethanEncodeInstBytes(ethanEncodeSUBImm(31, 0, 5, true))...)   // cmp x0, #5
				program = append(program, ethanEncodeInstBytes(ethanEncodeBCond(8, insts.CondEQ))...)   // b.eq skip
				program = append(program, ethanEncodeInstBytes(ethanEncodeADDImm(0, 31, 99, false))...) // x0 = 99
				// skip:
				program = append(program, ethanEncodeInstBytes(ethanEncodeADDImm(8, 31, 93, false))...)
				program = append(program, ethanEncodeInstBytes(ethanEncodeSVC(0))...)

				e.LoadProgram(0x1000, program)
				exitCode := e.Run()

				result := ValidationResult{
					Name:             "cond_branch_eq",
					ExitCode:         exitCode,
					ExpectedExitCode: 5,
					InstructionCount: e.InstructionCount(),
					Pass:             exitCode == 5,
				}
				validationResults = append(validationResults, result)

				Expect(exitCode).To(Equal(int64(5)))
			})

			It("should take B.GT when greater", func() {
				program := []byte{}
				program = append(program, ethanEncodeInstBytes(ethanEncodeADDImm(0, 31, 10, false))...)
				program = append(program, ethanEncodeInstBytes(ethanEncodeSUBImm(31, 0, 5, true))...)   // cmp x0, #5
				program = append(program, ethanEncodeInstBytes(ethanEncodeBCond(8, insts.CondGT))...)   // b.gt skip
				program = append(program, ethanEncodeInstBytes(ethanEncodeADDImm(0, 31, 99, false))...) // x0 = 99
				// skip:
				program = append(program, ethanEncodeInstBytes(ethanEncodeADDImm(8, 31, 93, false))...)
				program = append(program, ethanEncodeInstBytes(ethanEncodeSVC(0))...)

				e.LoadProgram(0x1000, program)
				exitCode := e.Run()

				result := ValidationResult{
					Name:             "cond_branch_gt",
					ExitCode:         exitCode,
					ExpectedExitCode: 10,
					InstructionCount: e.InstructionCount(),
					Pass:             exitCode == 10,
				}
				validationResults = append(validationResults, result)

				Expect(exitCode).To(Equal(int64(10)))
			})
		})
	})
})

// Instruction encoding helper functions for Ethan validation suite

func ethanEncodeInstBytes(v uint32) []byte {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, v)
	return buf
}

func ethanEncodeADDImm(rd, rn uint8, imm uint16, setFlags bool) uint32 {
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

func ethanEncodeADDImmShift(rd, rn uint8, imm uint16, shift uint8) uint32 {
	var inst uint32 = 0
	inst |= 1 << 31
	inst |= 0 << 30
	inst |= 0b100010 << 23
	inst |= uint32((shift/12)&0x1) << 22
	inst |= uint32(imm&0xFFF) << 10
	inst |= uint32(rn&0x1F) << 5
	inst |= uint32(rd & 0x1F)
	return inst
}

func ethanEncodeSUBImm(rd, rn uint8, imm uint16, setFlags bool) uint32 {
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

func ethanEncodeADDReg(rd, rn, rm uint8, setFlags bool) uint32 {
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

func ethanEncodeANDReg(rd, rn, rm uint8) uint32 {
	var inst uint32 = 0
	inst |= 1 << 31
	inst |= 0b00 << 29
	inst |= 0b01010 << 24
	inst |= 0 << 22
	inst |= 0 << 21
	inst |= uint32(rm&0x1F) << 16
	inst |= 0 << 10
	inst |= uint32(rn&0x1F) << 5
	inst |= uint32(rd & 0x1F)
	return inst
}

func ethanEncodeORRReg(rd, rn, rm uint8) uint32 {
	var inst uint32 = 0
	inst |= 1 << 31
	inst |= 0b01 << 29
	inst |= 0b01010 << 24
	inst |= 0 << 22
	inst |= 0 << 21
	inst |= uint32(rm&0x1F) << 16
	inst |= 0 << 10
	inst |= uint32(rn&0x1F) << 5
	inst |= uint32(rd & 0x1F)
	return inst
}

//nolint:unused // helper for future tests
func ethanEncodeEORReg(rd, rn, rm uint8) uint32 {
	var inst uint32 = 0
	inst |= 1 << 31
	inst |= 0b10 << 29
	inst |= 0b01010 << 24
	inst |= 0 << 22
	inst |= 0 << 21
	inst |= uint32(rm&0x1F) << 16
	inst |= 0 << 10
	inst |= uint32(rn&0x1F) << 5
	inst |= uint32(rd & 0x1F)
	return inst
}

func ethanEncodeLDR64(rd, rn uint8, offset uint16) uint32 {
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

func ethanEncodeSTR64(rd, rn uint8, offset uint16) uint32 {
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

// Pre-index/post-index variants for stack operations
//
//nolint:unused // helper for future tests
func ethanEncodeLDR64Offset(rt, rn uint8, offset int16) uint32 {
	// LDR (immediate) with pre-index mode
	var inst uint32 = 0
	inst |= 0b11 << 30
	inst |= 0b111 << 27
	inst |= 0 << 26
	inst |= 0b00 << 24
	inst |= 0b01 << 22
	inst |= 0 << 21
	imm9 := uint32(offset) & 0x1FF
	inst |= imm9 << 12
	inst |= 0b01 << 10 // pre-index
	inst |= uint32(rn&0x1F) << 5
	inst |= uint32(rt & 0x1F)
	return inst
}

//nolint:unused // helper for future tests
func ethanEncodeSTR64Offset(rt, rn uint8, offset int16) uint32 {
	// STR (immediate) with pre-index mode
	var inst uint32 = 0
	inst |= 0b11 << 30
	inst |= 0b111 << 27
	inst |= 0 << 26
	inst |= 0b00 << 24
	inst |= 0b00 << 22
	inst |= 0 << 21
	imm9 := uint32(offset) & 0x1FF
	inst |= imm9 << 12
	inst |= 0b01 << 10 // pre-index
	inst |= uint32(rn&0x1F) << 5
	inst |= uint32(rt & 0x1F)
	return inst
}

//nolint:unused // helper for future tests
func ethanEncodeB(offset int32) uint32 {
	var inst uint32 = 0
	inst |= 0b000101 << 26
	imm26 := uint32(offset/4) & 0x3FFFFFF
	inst |= imm26
	return inst
}

func ethanEncodeBL(offset int32) uint32 {
	var inst uint32 = 0
	inst |= 0b100101 << 26
	imm26 := uint32(offset/4) & 0x3FFFFFF
	inst |= imm26
	return inst
}

func ethanEncodeBCond(offset int32, cond insts.Cond) uint32 {
	var inst uint32 = 0
	inst |= 0b0101010 << 25
	inst |= 0 << 24
	imm19 := uint32(offset/4) & 0x7FFFF
	inst |= imm19 << 5
	inst |= 0 << 4
	inst |= uint32(cond & 0xF)
	return inst
}

func ethanEncodeRET() uint32 {
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

func ethanEncodeSVC(imm uint16) uint32 {
	var inst uint32 = 0
	inst |= 0b11010100 << 24
	inst |= 0b000 << 21
	inst |= uint32(imm) << 5
	inst |= 0b00001
	return inst
}
