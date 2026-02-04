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
		matrixMultiply2x2(),
		loopSimulation(),
	}
}

// GetCoreBenchmarks returns a minimal set of 3 core benchmarks for quick validation.
// These correspond to the acceptance criteria: loop, matrix multiply, branch-heavy code.
func GetCoreBenchmarks() []Benchmark {
	return []Benchmark{
		loopSimulation(),
		matrixMultiply2x2(),
		branchTaken(),
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
			// Note: Between pairs (e.g., LDR X0 then STR X0), there's a load-use
			// hazard that requires a stall to ensure correct behavior.
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
		// X0 starts at 42, and with proper load-use hazard handling for stores,
		// the value is preserved through all store-load pairs.
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
			EncodeBL(24), // BL add_one (offset = 6 instrs = 24 bytes)
			EncodeBL(20), // BL add_one
			EncodeBL(16), // BL add_one
			EncodeBL(12), // BL add_one
			EncodeBL(8),  // BL add_one
			EncodeSVC(0), // exit with X0

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
			EncodeB(8),                    // B +8 (skip next instr)
			EncodeADDImm(1, 1, 99, false), // skipped
			EncodeADDImm(0, 0, 1, false),  // X0 += 1

			EncodeB(8),                    // B +8
			EncodeADDImm(1, 1, 99, false), // skipped
			EncodeADDImm(0, 0, 1, false),  // X0 += 1

			EncodeB(8),                    // B +8
			EncodeADDImm(1, 1, 99, false), // skipped
			EncodeADDImm(0, 0, 1, false),  // X0 += 1

			EncodeB(8),                    // B +8
			EncodeADDImm(1, 1, 99, false), // skipped
			EncodeADDImm(0, 0, 1, false),  // X0 += 1

			EncodeB(8),                    // B +8
			EncodeADDImm(1, 1, 99, false), // skipped
			EncodeADDImm(0, 0, 1, false),  // X0 += 1

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
		// iter1: X0=0, X2=10, X3=10, X0=10, call +5 → X0=15
		// iter2: X0=15, X2=25, X3=25, X0=40, call +5 → X0=45
		// iter3: X0=45, X2=55, X3=55, X0=100
		ExpectedExit: 100,
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

// 7. Matrix Operations - Tests computation with memory access pattern
// Loads values from memory, performs computations, stores results
// Note: Uses ADD instead of MUL since scalar MUL isn't implemented yet
func matrixMultiply2x2() Benchmark {
	return Benchmark{
		Name:        "matrix_operations",
		Description: "Matrix-style load/compute/store pattern - tests memory access",
		Setup: func(regFile *emu.RegFile, memory *emu.Memory) {
			regFile.WriteReg(8, 93) // X8 = 93 (exit syscall)
			// Array A at 0x8000: [10, 20, 30, 40]
			regFile.WriteReg(1, 0x8000)
			memory.Write64(0x8000, 10)
			memory.Write64(0x8008, 20)
			memory.Write64(0x8010, 30)
			memory.Write64(0x8018, 40)

			// Array B at 0x8100: [1, 2, 3, 4]
			regFile.WriteReg(2, 0x8100)
			memory.Write64(0x8100, 1)
			memory.Write64(0x8108, 2)
			memory.Write64(0x8110, 3)
			memory.Write64(0x8118, 4)

			// Array C at 0x8200 (result)
			regFile.WriteReg(3, 0x8200)
		},
		// Compute C[i] = A[i] + B[i] for i = 0..3
		// C = [11, 22, 33, 44]
		// Return sum of C = 11 + 22 + 33 + 44 = 110
		Program: BuildProgram(
			// Load A array into X10-X13
			EncodeLDR64(10, 1, 0), // X10 = A[0] = 10
			EncodeLDR64(11, 1, 1), // X11 = A[1] = 20
			EncodeLDR64(12, 1, 2), // X12 = A[2] = 30
			EncodeLDR64(13, 1, 3), // X13 = A[3] = 40

			// Load B array into X14-X17
			EncodeLDR64(14, 2, 0), // X14 = B[0] = 1
			EncodeLDR64(15, 2, 1), // X15 = B[1] = 2
			EncodeLDR64(16, 2, 2), // X16 = B[2] = 3
			EncodeLDR64(17, 2, 3), // X17 = B[3] = 4

			// Compute C[i] = A[i] + B[i]
			EncodeADDReg(20, 10, 14, false), // X20 = 10 + 1 = 11
			EncodeADDReg(21, 11, 15, false), // X21 = 20 + 2 = 22
			EncodeADDReg(22, 12, 16, false), // X22 = 30 + 3 = 33
			EncodeADDReg(23, 13, 17, false), // X23 = 40 + 4 = 44

			// Store C array
			EncodeSTR64(20, 3, 0), // C[0] = 11
			EncodeSTR64(21, 3, 1), // C[1] = 22
			EncodeSTR64(22, 3, 2), // C[2] = 33
			EncodeSTR64(23, 3, 3), // C[3] = 44

			// Sum all C elements for exit code: 11 + 22 + 33 + 44 = 110
			EncodeADDReg(0, 20, 21, false), // X0 = 11 + 22 = 33
			EncodeADDReg(0, 0, 22, false),  // X0 = 33 + 33 = 66
			EncodeADDReg(0, 0, 23, false),  // X0 = 66 + 44 = 110

			EncodeSVC(0),
		),
		ExpectedExit: 110,
	}
}

// 8. Loop Simulation - Simulates a counted loop (unrolled)
// This is what a "for i := 0; i < 10; i++" loop would look like
func loopSimulation() Benchmark {
	return Benchmark{
		Name:        "loop_simulation",
		Description: "Simulated 10-iteration loop (unrolled) - tests loop-like patterns",
		Setup: func(regFile *emu.RegFile, memory *emu.Memory) {
			regFile.WriteReg(8, 93) // X8 = 93 (exit syscall)
			regFile.WriteReg(0, 0)  // X0 = sum = 0
			regFile.WriteReg(1, 0)  // X1 = i = 0
		},
		// Simulate: for i := 0; i < 10; i++ { sum += i }
		// Result: 0 + 1 + 2 + 3 + 4 + 5 + 6 + 7 + 8 + 9 = 45
		Program: BuildProgram(
			// Iteration 0: sum += 0, i++
			EncodeADDReg(0, 0, 1, false), // sum += i
			EncodeADDImm(1, 1, 1, false), // i++

			// Iteration 1: sum += 1, i++
			EncodeADDReg(0, 0, 1, false),
			EncodeADDImm(1, 1, 1, false),

			// Iteration 2
			EncodeADDReg(0, 0, 1, false),
			EncodeADDImm(1, 1, 1, false),

			// Iteration 3
			EncodeADDReg(0, 0, 1, false),
			EncodeADDImm(1, 1, 1, false),

			// Iteration 4
			EncodeADDReg(0, 0, 1, false),
			EncodeADDImm(1, 1, 1, false),

			// Iteration 5
			EncodeADDReg(0, 0, 1, false),
			EncodeADDImm(1, 1, 1, false),

			// Iteration 6
			EncodeADDReg(0, 0, 1, false),
			EncodeADDImm(1, 1, 1, false),

			// Iteration 7
			EncodeADDReg(0, 0, 1, false),
			EncodeADDImm(1, 1, 1, false),

			// Iteration 8
			EncodeADDReg(0, 0, 1, false),
			EncodeADDImm(1, 1, 1, false),

			// Iteration 9
			EncodeADDReg(0, 0, 1, false),
			EncodeADDImm(1, 1, 1, false),

			EncodeSVC(0),
		),
		ExpectedExit: 45,
	}
}

// Note: encodeMUL removed - scalar MUL/MADD not yet implemented in simulator
