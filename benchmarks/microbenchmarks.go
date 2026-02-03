// Package benchmarks provides timing benchmark infrastructure for M2Sim calibration.
package benchmarks

import "github.com/sarchlab/m2sim/emu"

// GetMicrobenchmarks returns the standard set of microbenchmarks for M2 calibration.
// Each benchmark targets a specific CPU characteristic.
//
// NOTE: These benchmarks use unrolled code (no loops) because the timing pipeline
// currently doesn't update PSTATE flags, which breaks conditional branch evaluation.
// See issue tracking: need to update pipeline to set flags on SUBS/ADDS.
func GetMicrobenchmarks() []Benchmark {
	return []Benchmark{
		arithmeticSequential(),
		dependencyChain(),
		memorySequential(),
		functionCalls(),
		branchTaken(),
		mixedOperations(),
	}
}

// 1. Arithmetic Sequential - Tests ALU throughput with independent operations
func arithmeticSequential() Benchmark {
	return Benchmark{
		Name:        "arithmetic_sequential",
		Description: "20 independent ADD operations - measures ALU throughput",
		Setup: func(regFile *emu.RegFile, memory *emu.Memory) {
			regFile.WriteReg(8, 93) // X8 = 93 (exit syscall)
		},
		Program: BuildProgram(
			// 20 independent ADDs to different registers
			EncodeADDImm(0, 0, 1, false),
			EncodeADDImm(1, 1, 1, false),
			EncodeADDImm(2, 2, 1, false),
			EncodeADDImm(3, 3, 1, false),
			EncodeADDImm(4, 4, 1, false),
			EncodeADDImm(0, 0, 1, false),
			EncodeADDImm(1, 1, 1, false),
			EncodeADDImm(2, 2, 1, false),
			EncodeADDImm(3, 3, 1, false),
			EncodeADDImm(4, 4, 1, false),
			EncodeADDImm(0, 0, 1, false),
			EncodeADDImm(1, 1, 1, false),
			EncodeADDImm(2, 2, 1, false),
			EncodeADDImm(3, 3, 1, false),
			EncodeADDImm(4, 4, 1, false),
			EncodeADDImm(0, 0, 1, false),
			EncodeADDImm(1, 1, 1, false),
			EncodeADDImm(2, 2, 1, false),
			EncodeADDImm(3, 3, 1, false),
			EncodeADDImm(4, 4, 1, false),
			EncodeSVC(0),
		),
		ExpectedExit: 4, // X0 = 0 + 4*1 = 4
	}
}

// 2. Dependency Chain - Tests instruction latency with RAW hazards
func dependencyChain() Benchmark {
	return Benchmark{
		Name:        "dependency_chain",
		Description: "20 dependent ADDs (X0 = X0 + 1) - measures forwarding latency",
		Setup: func(regFile *emu.RegFile, memory *emu.Memory) {
			regFile.WriteReg(8, 93) // X8 = 93 (exit syscall)
			regFile.WriteReg(0, 0)  // X0 = 0 (start value)
		},
		Program:      buildDependencyChain(20),
		ExpectedExit: 20, // X0 = 0 + 20*1 = 20
	}
}

func buildDependencyChain(n int) []byte {
	instrs := make([]uint32, 0, n+1)
	for i := 0; i < n; i++ {
		instrs = append(instrs, EncodeADDImm(0, 0, 1, false))
	}
	instrs = append(instrs, EncodeSVC(0))
	return BuildProgram(instrs...)
}

// 3. Memory Sequential - Tests cache/memory performance
func memorySequential() Benchmark {
	return Benchmark{
		Name:        "memory_sequential",
		Description: "10 store/load pairs to sequential addresses - measures memory latency",
		Setup: func(regFile *emu.RegFile, memory *emu.Memory) {
			regFile.WriteReg(8, 93)     // X8 = 93 (exit syscall)
			regFile.WriteReg(1, 0x8000) // X1 = base address
			regFile.WriteReg(0, 42)     // X0 = value to store/load
		},
		Program: BuildProgram(
			// Store X0 to [X1], load back, repeat at different offsets
			EncodeSTR64(0, 1, 0), EncodeLDR64(0, 1, 0),
			EncodeSTR64(0, 1, 1), EncodeLDR64(0, 1, 1), // offset = 8 bytes
			EncodeSTR64(0, 1, 2), EncodeLDR64(0, 1, 2),
			EncodeSTR64(0, 1, 3), EncodeLDR64(0, 1, 3),
			EncodeSTR64(0, 1, 4), EncodeLDR64(0, 1, 4),
			EncodeSTR64(0, 1, 5), EncodeLDR64(0, 1, 5),
			EncodeSTR64(0, 1, 6), EncodeLDR64(0, 1, 6),
			EncodeSTR64(0, 1, 7), EncodeLDR64(0, 1, 7),
			EncodeSTR64(0, 1, 8), EncodeLDR64(0, 1, 8),
			EncodeSTR64(0, 1, 9), EncodeLDR64(0, 1, 9),
			EncodeSVC(0),
		),
		ExpectedExit: 42,
	}
}

// 4. Function Calls - Tests BL/RET overhead
func functionCalls() Benchmark {
	return Benchmark{
		Name:        "function_calls",
		Description: "5 function calls (BL + RET pairs) - measures call overhead",
		Setup: func(regFile *emu.RegFile, memory *emu.Memory) {
			regFile.WriteReg(8, 93) // X8 = 93 (exit syscall)
			regFile.WriteReg(0, 0)  // X0 = 0 (result)
			regFile.SP = 0x10000    // Stack pointer
		},
		Program: BuildProgram(
			// main: call add_one 5 times
			EncodeBL(24),                 // BL add_one (offset = 6 instrs = 24 bytes)
			EncodeBL(20),                 // BL add_one
			EncodeBL(16),                 // BL add_one
			EncodeBL(12),                 // BL add_one
			EncodeBL(8),                  // BL add_one
			EncodeSVC(0),                 // exit with X0

			// add_one function (at offset 24)
			EncodeADDImm(0, 0, 1, false), // X0 += 1
			EncodeRET(),                  // return
		),
		ExpectedExit: 5, // 5 calls * 1 add = 5
	}
}

// 5. Branch Taken - Tests unconditional branch overhead
func branchTaken() Benchmark {
	return Benchmark{
		Name:        "branch_taken",
		Description: "5 unconditional branches (B forward) - measures branch overhead",
		Setup: func(regFile *emu.RegFile, memory *emu.Memory) {
			regFile.WriteReg(8, 93) // X8 = 93 (exit syscall)
			regFile.WriteReg(0, 0)  // X0 = 0 (result)
		},
		Program: BuildProgram(
			// Jump over NOP-like instructions
			EncodeB(8),                   // B +8 (skip next instr)
			EncodeADDImm(1, 1, 99, false), // skipped
			EncodeADDImm(0, 0, 1, false), // X0 += 1

			EncodeB(8),                   // B +8
			EncodeADDImm(1, 1, 99, false), // skipped
			EncodeADDImm(0, 0, 1, false), // X0 += 1

			EncodeB(8),                   // B +8
			EncodeADDImm(1, 1, 99, false), // skipped
			EncodeADDImm(0, 0, 1, false), // X0 += 1

			EncodeB(8),                   // B +8
			EncodeADDImm(1, 1, 99, false), // skipped
			EncodeADDImm(0, 0, 1, false), // X0 += 1

			EncodeB(8),                   // B +8
			EncodeADDImm(1, 1, 99, false), // skipped
			EncodeADDImm(0, 0, 1, false), // X0 += 1

			EncodeSVC(0), // exit with X0 = 5
		),
		ExpectedExit: 5,
	}
}

// 6. Mixed Operations - Combination of ALU, memory, and branches
func mixedOperations() Benchmark {
	return Benchmark{
		Name:        "mixed_operations",
		Description: "Mix of ADD, STR/LDR, and BL - realistic workload characteristics",
		Setup: func(regFile *emu.RegFile, memory *emu.Memory) {
			regFile.WriteReg(8, 93)     // X8 = 93 (exit syscall)
			regFile.WriteReg(0, 0)      // X0 = sum
			regFile.WriteReg(1, 0x8000) // X1 = buffer address
			regFile.SP = 0x10000
		},
		Program: BuildProgram(
			// Iteration 1: compute, store, load, call
			EncodeADDImm(2, 0, 10, false), // X2 = X0 + 10 = 10
			EncodeSTR64(2, 1, 0),          // [X1] = X2
			EncodeLDR64(3, 1, 0),          // X3 = [X1]
			EncodeADDReg(0, 0, 3, false),  // X0 += X3
			EncodeBL(44),                  // BL add_five

			// Iteration 2
			EncodeADDImm(2, 0, 10, false),
			EncodeSTR64(2, 1, 1),
			EncodeLDR64(3, 1, 1),
			EncodeADDReg(0, 0, 3, false),
			EncodeBL(24),

			// Iteration 3
			EncodeADDImm(2, 0, 10, false),
			EncodeSTR64(2, 1, 2),
			EncodeLDR64(3, 1, 2),
			EncodeADDReg(0, 0, 3, false),

			EncodeSVC(0), // exit with X0

			// add_five function
			EncodeADDImm(0, 0, 5, false),
			EncodeRET(),
		),
		ExpectedExit: 95, // Computed: (10+5) + (25+5) + 50 = 95
	}
}

// EncodeB encodes unconditional branch: B offset
func EncodeB(offset int32) uint32 {
	var inst uint32 = 0
	inst |= 0b000101 << 26 // B opcode
	imm26 := uint32(offset/4) & 0x3FFFFFF
	inst |= imm26
	return inst
}
