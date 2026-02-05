// Package benchmarks contains validation and benchmark tests for M2Sim.
package benchmarks

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/sarchlab/m2sim/emu"
)

// TestValidationBaseline runs all validation test programs and verifies expected results.
func TestValidationBaseline(t *testing.T) {
	tests := []struct {
		name           string
		setup          func(*emu.Emulator)
		program        []byte
		expectedExit   int64
		expectedOutput string
	}{
		{
			name: "simple_exit",
			program: buildProgram(
				encodeADDImm(8, 31, 93, false), // X8 = 93 (exit syscall)
				encodeADDImm(0, 31, 42, false), // X0 = 42 (exit code)
				encodeSVC(0),                   // syscall
			),
			expectedExit: 42,
		},
		{
			name: "arithmetic",
			program: buildProgram(
				encodeADDImm(0, 31, 10, false), // X0 = 10
				encodeADDImm(1, 31, 5, false),  // X1 = 5
				encodeADDReg(0, 0, 1, false),   // X0 = X0 + X1
				encodeADDImm(8, 31, 93, false), // X8 = 93
				encodeSVC(0),                   // syscall
			),
			expectedExit: 15,
		},
		{
			name: "subtraction",
			program: buildProgram(
				encodeADDImm(0, 31, 100, false), // X0 = 100
				encodeSUBImm(0, 0, 58, false),   // X0 = X0 - 58
				encodeADDImm(8, 31, 93, false),  // X8 = 93
				encodeSVC(0),                    // syscall
			),
			expectedExit: 42,
		},
		{
			name: "loop",
			program: buildProgram(
				encodeADDImm(0, 31, 3, false),  // X0 = 3
				encodeSUBImm(0, 0, 1, true),    // SUBS X0, X0, #1
				encodeBCond(-4, 1),             // B.NE loop (CondNE = 1)
				encodeADDImm(8, 31, 93, false), // X8 = 93
				encodeSVC(0),                   // syscall
			),
			expectedExit: 0,
		},
		{
			name: "loop_sum",
			program: buildProgram(
				encodeADDImm(0, 31, 0, false),  // X0 = 0 (sum)
				encodeADDImm(1, 31, 5, false),  // X1 = 5 (counter)
				encodeADDReg(0, 0, 1, false),   // ADD X0, X0, X1
				encodeSUBImm(1, 1, 1, true),    // SUBS X1, X1, #1
				encodeBCond(-8, 1),             // B.NE loop
				encodeADDImm(8, 31, 93, false), // X8 = 93
				encodeSVC(0),                   // syscall
			),
			expectedExit: 15,
		},
		{
			name: "hello",
			setup: func(e *emu.Emulator) {
				// Set up "Hello\n" in memory
				msg := []byte("Hello\n")
				for i, b := range msg {
					e.Memory().Write8(0x3000+uint64(i), b)
				}
				e.RegFile().WriteReg(1, 0x3000) // Buffer address
			},
			program: buildProgram(
				encodeADDImm(8, 31, 64, false), // X8 = 64 (write syscall)
				encodeADDImm(0, 31, 1, false),  // X0 = 1 (stdout)
				encodeADDImm(2, 31, 6, false),  // X2 = 6 (length)
				encodeSVC(0),                   // write syscall
				encodeADDImm(8, 31, 93, false), // X8 = 93 (exit)
				encodeADDImm(0, 31, 0, false),  // X0 = 0
				encodeSVC(0),                   // exit syscall
			),
			expectedExit:   0,
			expectedOutput: "Hello\n",
		},
		{
			name: "function_call",
			program: buildProgram(
				encodeADDImm(0, 31, 10, false), // X0 = 10
				encodeBL(12),                   // BL add_five
				encodeADDImm(8, 31, 93, false), // X8 = 93
				encodeSVC(0),                   // syscall
				// add_five:
				encodeADDImm(0, 0, 5, false), // X0 += 5
				encodeRET(),                  // RET
			),
			expectedExit: 15,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdoutBuf := &bytes.Buffer{}
			e := emu.NewEmulator(
				emu.WithStdout(stdoutBuf),
				emu.WithMaxInstructions(10000),
			)

			if tt.setup != nil {
				tt.setup(e)
			}

			e.LoadProgram(0x1000, tt.program)
			exitCode := e.Run()

			if exitCode != tt.expectedExit {
				t.Errorf("expected exit code %d, got %d", tt.expectedExit, exitCode)
			}

			if tt.expectedOutput != "" && stdoutBuf.String() != tt.expectedOutput {
				t.Errorf("expected output %q, got %q", tt.expectedOutput, stdoutBuf.String())
			}

			t.Logf("✓ %s: exit_code=%d, instructions=%d", tt.name, exitCode, e.InstructionCount())
		})
	}
}

// Helper functions to encode ARM64 instructions

func buildProgram(instrs ...uint32) []byte {
	program := make([]byte, 0, len(instrs)*4)
	for _, inst := range instrs {
		program = append(program, uint32ToBytes(inst)...)
	}
	return program
}

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

func encodeBL(offset int32) uint32 {
	var inst uint32 = 0
	inst |= 0b100101 << 26
	imm26 := uint32(offset/4) & 0x3FFFFFF
	inst |= imm26
	return inst
}

func encodeBCond(offset int32, cond uint8) uint32 {
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

func encodeSVC(imm uint16) uint32 {
	var inst uint32 = 0
	inst |= 0b11010100 << 24
	inst |= 0b000 << 21
	inst |= uint32(imm) << 5
	inst |= 0b00001
	return inst
}

// encodeMOVZ encodes MOVZ (move wide with zero) for 64-bit.
// Format: 1 10 100101 hw imm16 Rd
// Sets Rd = imm16 << (hw * 16), zeros other bits
func encodeMOVZ(rd uint8, imm16 uint16, hw uint8) uint32 {
	var inst uint32 = 0
	inst |= 1 << 31        // sf = 1 (64-bit)
	inst |= 0b10 << 29     // opc = 10 (MOVZ)
	inst |= 0b100101 << 23 // fixed bits
	inst |= uint32(hw&3) << 21
	inst |= uint32(imm16) << 5
	inst |= uint32(rd & 0x1F)
	return inst
}

// encodeSTR64 encodes STR (64-bit) with unsigned immediate offset.
// Format: 11 111 0 01 00 imm12 Rn Rt
func encodeSTR64(rt, rn uint8, imm12 uint16) uint32 {
	var inst uint32 = 0
	inst |= 0b11 << 30  // size = 64-bit
	inst |= 0b111 << 27 // op1
	inst |= 0 << 26     // V = 0 (not SIMD)
	inst |= 0b01 << 24  // op2
	inst |= 0b00 << 22  // opc = STR
	inst |= uint32(imm12&0xFFF) << 10
	inst |= uint32(rn&0x1F) << 5
	inst |= uint32(rt & 0x1F)
	return inst
}

// encodeLDR64 encodes LDR (64-bit) with unsigned immediate offset.
// Format: 11 111 0 01 01 imm12 Rn Rt
func encodeLDR64(rt, rn uint8, imm12 uint16) uint32 {
	var inst uint32 = 0
	inst |= 0b11 << 30  // size = 64-bit
	inst |= 0b111 << 27 // op1
	inst |= 0 << 26     // V = 0 (not SIMD)
	inst |= 0b01 << 24  // op2
	inst |= 0b01 << 22  // opc = LDR
	inst |= uint32(imm12&0xFFF) << 10
	inst |= uint32(rn&0x1F) << 5
	inst |= uint32(rt & 0x1F)
	return inst
}

// TestEdgeCases tests boundary conditions and edge cases.
func TestEdgeCases(t *testing.T) {
	tests := []struct {
		name         string
		setup        func(*emu.Emulator)
		program      []byte
		expectedExit int64
	}{
		{
			name: "stack_operations",
			setup: func(e *emu.Emulator) {
				// Initialize SP to a valid memory region
				e.RegFile().SP = 0x8000
			},
			program: buildProgram(
				// Push X0 onto stack (simulate stack operation)
				encodeMOVZ(0, 42, 0),  // X0 = 42 (using MOVZ, not ADD from SP)
				encodeSTR64(0, 31, 0), // STR X0, [SP, #0] (uses SP as base)
				encodeMOVZ(0, 0, 0),   // X0 = 0 (clear X0)
				encodeLDR64(0, 31, 0), // LDR X0, [SP, #0] (restore from stack)
				encodeMOVZ(8, 93, 0),  // X8 = 93 (exit syscall)
				encodeSVC(0),          // exit with X0
			),
			expectedExit: 42,
		},
		{
			name: "large_loop_stress_test",
			program: buildProgram(
				encodeADDImm(0, 31, 0, false),   // X0 = 0 (counter)
				encodeADDImm(1, 31, 100, false), // X1 = 100 (limit)
				// loop:
				encodeADDImm(0, 0, 1, false),   // X0 = X0 + 1
				encodeSUBImm(2, 0, 100, true),  // SUBS X2, X0, #100 (compare)
				encodeBCond(-8, 11),            // B.LT loop (CondLT = 0b1011 = 11)
				encodeADDImm(8, 31, 93, false), // X8 = 93
				encodeSVC(0),                   // exit with X0 = 100
			),
			expectedExit: 100,
		},
		{
			name: "max_immediate_value",
			program: buildProgram(
				// Test max 12-bit immediate (4095)
				encodeADDImm(0, 31, 4095, false), // X0 = 4095
				encodeSUBImm(0, 0, 4053, false),  // X0 = 4095 - 4053 = 42
				encodeADDImm(8, 31, 93, false),   // X8 = 93
				encodeSVC(0),                     // exit
			),
			expectedExit: 42,
		},
		{
			name: "zero_register_as_source",
			program: buildProgram(
				// XZR reads as 0, writes are ignored
				encodeADDImm(0, 31, 42, false), // X0 = XZR + 42 = 42
				encodeADDReg(0, 0, 31, false),  // X0 = X0 + XZR = 42
				encodeADDImm(8, 31, 93, false), // X8 = 93
				encodeSVC(0),                   // exit
			),
			expectedExit: 42,
		},
		{
			name: "nested_function_calls",
			// Test two-level function calls: main -> outer -> inner
			// Using BL to call functions, with manual LR save/restore
			program: buildProgram(
				// offset 0: main
				encodeADDImm(0, 31, 1, false),  // 0: X0 = 1
				encodeBL(12),                   // 4: BL outer (offset 12 -> addr 16)
				encodeADDImm(8, 31, 93, false), // 8: X8 = 93
				encodeSVC(0),                   // 12: exit with X0
				// offset 16: outer
				encodeADDImm(0, 0, 1, false),   // 16: X0 += 1 (now 2)
				encodeADDReg(9, 30, 31, false), // 20: X9 = LR (save)
				encodeBL(12),                   // 24: BL inner (offset 12 -> addr 36)
				encodeADDReg(30, 9, 31, false), // 28: LR = X9 (restore)
				encodeRET(),                    // 32: return to main
				// offset 36: inner
				encodeADDImm(0, 0, 1, false), // 36: X0 += 1 (now 3)
				encodeRET(),                  // 40: return to outer
			),
			expectedExit: 3,
		},
		{
			name: "conditional_branch_all_conditions",
			program: buildProgram(
				// Test B.EQ (should not branch, Z=0)
				encodeADDImm(0, 31, 1, false), // X0 = 1
				encodeSUBImm(2, 0, 0, true),   // SUBS X2, X0, #0 (sets Z=0, N=0)
				encodeBCond(8, 0),             // B.EQ skip1 (CondEQ=0, should NOT branch)
				encodeADDImm(0, 0, 1, false),  // X0 += 1 (now 2)
				// skip1:
				// Test B.NE (should branch, Z=0)
				encodeBCond(8, 1),              // B.NE skip2 (CondNE=1, should branch)
				encodeADDImm(0, 0, 100, false), // X0 += 100 (should be skipped)
				// skip2:
				encodeADDImm(8, 31, 93, false), // X8 = 93
				encodeSVC(0),                   // exit with X0 = 2
			),
			expectedExit: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := emu.NewEmulator(
				emu.WithMaxInstructions(100000),
			)

			if tt.setup != nil {
				tt.setup(e)
			}

			e.LoadProgram(0x1000, tt.program)
			exitCode := e.Run()

			if exitCode != tt.expectedExit {
				t.Errorf("expected exit code %d, got %d", tt.expectedExit, exitCode)
			}

			t.Logf("✓ %s: exit_code=%d, instructions=%d", tt.name, exitCode, e.InstructionCount())
		})
	}
}

// TestNegativeCases tests error handling for invalid inputs.
func TestNegativeCases(t *testing.T) {
	t.Run("unknown_opcode", func(t *testing.T) {
		e := emu.NewEmulator(
			emu.WithMaxInstructions(100),
		)

		// Create a program with an invalid instruction
		// Use an encoding that doesn't match any known format
		invalidInstr := uint32(0xDEADBEEF)
		program := uint32ToBytes(invalidInstr)

		e.LoadProgram(0x1000, program)
		exitCode := e.Run()

		// Should return error exit code
		if exitCode != -1 {
			t.Errorf("expected exit code -1 for unknown opcode, got %d", exitCode)
		}

		t.Logf("✓ unknown_opcode: correctly returned error exit code")
	})

	t.Run("max_instruction_limit", func(t *testing.T) {
		e := emu.NewEmulator(
			emu.WithMaxInstructions(5),
		)

		// Program that would loop many times without limit
		program := buildProgram(
			encodeADDImm(0, 31, 0, false),  // X0 = 0
			encodeADDImm(0, 0, 1, false),   // X0 += 1
			encodeADDImm(0, 0, 1, false),   // X0 += 1
			encodeADDImm(0, 0, 1, false),   // X0 += 1
			encodeADDImm(0, 0, 1, false),   // X0 += 1
			encodeADDImm(0, 0, 1, false),   // X0 += 1 (6th - should not execute)
			encodeADDImm(8, 31, 93, false), // X8 = 93
			encodeSVC(0),                   // exit
		)

		e.LoadProgram(0x1000, program)
		exitCode := e.Run()

		// Should hit max instruction limit and return error
		if exitCode != -1 {
			t.Errorf("expected exit code -1 when hitting max instructions, got %d", exitCode)
		}

		if e.InstructionCount() != 5 {
			t.Errorf("expected 5 instructions executed, got %d", e.InstructionCount())
		}

		t.Logf("✓ max_instruction_limit: stopped after %d instructions", e.InstructionCount())
	})
}

// TestIntermediateStateVerification demonstrates mid-execution state checking.
func TestIntermediateStateVerification(t *testing.T) {
	t.Run("register_state_after_each_step", func(t *testing.T) {
		e := emu.NewEmulator(
			emu.WithMaxInstructions(1000),
		)

		program := buildProgram(
			encodeADDImm(0, 31, 10, false), // X0 = 10
			encodeADDImm(1, 31, 20, false), // X1 = 20
			encodeADDReg(2, 0, 1, false),   // X2 = X0 + X1
			encodeADDImm(8, 31, 93, false), // X8 = 93
			encodeSVC(0),                   // exit with X0
		)

		e.LoadProgram(0x1000, program)

		// Step 1: X0 = 10
		e.Step()
		if e.RegFile().ReadReg(0) != 10 {
			t.Errorf("after step 1: expected X0=10, got %d", e.RegFile().ReadReg(0))
		}

		// Step 2: X1 = 20
		e.Step()
		if e.RegFile().ReadReg(1) != 20 {
			t.Errorf("after step 2: expected X1=20, got %d", e.RegFile().ReadReg(1))
		}

		// Step 3: X2 = 30
		e.Step()
		if e.RegFile().ReadReg(2) != 30 {
			t.Errorf("after step 3: expected X2=30, got %d", e.RegFile().ReadReg(2))
		}

		// Verify PC advanced correctly
		expectedPC := uint64(0x1000 + 3*4)
		if e.RegFile().PC != expectedPC {
			t.Errorf("expected PC=0x%X, got 0x%X", expectedPC, e.RegFile().PC)
		}

		t.Logf("✓ register_state_after_each_step: all intermediate states verified")
	})

	t.Run("memory_state_verification", func(t *testing.T) {
		e := emu.NewEmulator(
			emu.WithMaxInstructions(1000),
		)

		// Set up initial memory
		e.RegFile().SP = 0x8000

		program := buildProgram(
			encodeMOVZ(0, 42, 0),  // X0 = 42 (using MOVZ)
			encodeSTR64(0, 31, 0), // STR X0, [SP]
			encodeMOVZ(8, 93, 0),  // X8 = 93
			encodeSVC(0),          // exit
		)

		e.LoadProgram(0x1000, program)

		// Execute first two instructions
		e.Step() // X0 = 42
		e.Step() // STR X0, [SP]

		// Verify memory was written
		storedValue := e.Memory().Read64(0x8000)
		if storedValue != 42 {
			t.Errorf("expected memory[0x8000]=42, got %d", storedValue)
		}

		t.Logf("✓ memory_state_verification: memory correctly written mid-execution")
	})

	t.Run("flag_state_verification", func(t *testing.T) {
		e := emu.NewEmulator(
			emu.WithMaxInstructions(1000),
		)

		program := buildProgram(
			encodeMOVZ(0, 5, 0),            // X0 = 5 (using MOVZ)
			encodeSUBImm(0, 0, 5, true),    // SUBS X0, X0, #5 (should set Z flag)
			encodeADDImm(8, 31, 93, false), // X8 = 93
			encodeSVC(0),                   // exit
		)

		e.LoadProgram(0x1000, program)

		e.Step() // X0 = 5
		e.Step() // SUBS X0, X0, #5

		// Check Z flag is set (result was 0)
		if !e.RegFile().PSTATE.Z {
			t.Errorf("expected Z flag set after SUBS with zero result")
		}
		if e.RegFile().PSTATE.N {
			t.Errorf("expected N flag clear after SUBS with zero result")
		}

		t.Logf("✓ flag_state_verification: flags correctly set mid-execution")
	})
}
