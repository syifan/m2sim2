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
