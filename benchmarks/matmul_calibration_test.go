package benchmarks

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/sarchlab/m2sim/emu"
	"github.com/sarchlab/m2sim/timing/latency"
	"github.com/sarchlab/m2sim/timing/pipeline"
)

// encodeMADD encodes MADD: Rd = Ra + Rn * Rm (64-bit)
func encodeMADD(rd, rn, rm, ra uint8) uint32 {
	var inst uint32
	inst |= 1 << 31          // sf = 1 (64-bit)
	inst |= 0b00 << 29       // op54 = 00
	inst |= 0b11011 << 24    // fixed
	inst |= 0b000 << 21      // op31 = 000
	inst |= uint32(rm) << 16 // Rm
	inst |= 0 << 15          // o0 = 0 (MADD)
	inst |= uint32(ra) << 10 // Ra
	inst |= uint32(rn) << 5  // Rn
	inst |= uint32(rd)       // Rd
	return inst
}

// encodeMUL encodes MUL: Rd = Rn * Rm (alias: MADD Rd, Rn, Rm, XZR)
func encodeMUL(rd, rn, rm uint8) uint32 {
	return encodeMADD(rd, rn, rm, 31) // Ra = XZR
}

// encodeLSLImm encodes LSL (logical shift left) immediate.
// LSL is an alias for UBFM: Rd = Rn << shift
// Encoded as: UBFM Rd, Rn, #(-shift mod 64), #(63 - shift)
func encodeLSLImm(rd, rn uint8, shift uint8) uint32 {
	immr := (64 - uint32(shift)) & 0x3F
	imms := (63 - uint32(shift)) & 0x3F
	var inst uint32
	inst |= 1 << 31         // sf = 1 (64-bit)
	inst |= 0b10 << 29      // opc = 10 (UBFM)
	inst |= 0b100110 << 23  // fixed
	inst |= 1 << 22         // N = 1 (64-bit)
	inst |= immr << 16      // immr
	inst |= imms << 10      // imms
	inst |= uint32(rn) << 5 // Rn
	inst |= uint32(rd)      // Rd
	return inst
}

// buildMatmul4x4 builds a 4x4 integer matrix multiply C = A * B using
// triple-nested loops with real branch instructions.
//
// Register allocation:
//
//	X0  = exit code (sum of C)
//	X1  = base address of A (0x8000)
//	X2  = base address of B (0x8100)
//	X3  = base address of C (0x8200)
//	X4  = i (outer loop counter)
//	X5  = j (middle loop counter)
//	X6  = k (inner loop counter)
//	X7  = N (matrix dimension = 4)
//	X8  = syscall number (93)
//	X9  = temp: A[i][k]
//	X10 = temp: B[k][j]
//	X11 = temp: C[i][j] accumulator
//	X12 = temp: address calculation
//	X13 = temp: address calculation
//	X14 = temp: row offset (i*N or k*N)
func buildMatmul4x4() Benchmark {
	instrs := []uint32{
		// === Initialization ===
		EncodeADDImm(4, 31, 0, false), // X4 = 0 (i)            [0]

		// === Outer loop (i_loop): offset 1 ===
		EncodeADDImm(5, 31, 0, false), // X5 = 0 (j)            [1]

		// === Middle loop (j_loop): offset 2 ===
		EncodeADDImm(11, 31, 0, false), // X11 = 0 (accum)      [2]
		EncodeADDImm(6, 31, 0, false),  // X6 = 0 (k)           [3]

		// === Inner loop (k_loop): offset 4 ===
		// Compute addr of A[i][k]: A + (i*4 + k) * 8
		encodeMUL(14, 4, 7),            // X14 = i * N           [4]
		EncodeADDReg(12, 14, 6, false), // X12 = i*N + k         [5]
		encodeLSLImm(12, 12, 3),        // X12 = (i*N+k) * 8     [6]
		EncodeADDReg(12, 1, 12, false), // X12 = &A[i][k]        [7]
		EncodeLDR64(9, 12, 0),          // X9 = A[i][k]           [8]

		// Compute addr of B[k][j]: B + (k*4 + j) * 8
		encodeMUL(14, 6, 7),            // X14 = k * N           [9]
		EncodeADDReg(13, 14, 5, false), // X13 = k*N + j         [10]
		encodeLSLImm(13, 13, 3),        // X13 = (k*N+j) * 8     [11]
		EncodeADDReg(13, 2, 13, false), // X13 = &B[k][j]        [12]
		EncodeLDR64(10, 13, 0),         // X10 = B[k][j]          [13]

		// C[i][j] += A[i][k] * B[k][j]
		encodeMADD(11, 9, 10, 11), // X11 += X9*X10         [14]

		// k++; if k < N goto k_loop
		EncodeADDImm(6, 6, 1, false), // k++                  [15]
		EncodeCMPReg(6, 7),           // CMP k, N             [16]
		// B.LT k_loop: target=4, branch=17
		// offset = (4 - 17) * 4 = -52
		EncodeBCond(-52, 11), // B.LT k_loop           [17]

		// Store C[i][j]: C + (i*4 + j) * 8
		encodeMUL(14, 4, 7),            // X14 = i * N           [18]
		EncodeADDReg(12, 14, 5, false), // X12 = i*N + j         [19]
		encodeLSLImm(12, 12, 3),        // X12 = (i*N+j) * 8     [20]
		EncodeADDReg(12, 3, 12, false), // X12 = &C[i][j]        [21]
		EncodeSTR64(11, 12, 0),         // C[i][j] = X11          [22]

		// j++; if j < N goto j_loop
		EncodeADDImm(5, 5, 1, false), // j++                  [23]
		EncodeCMPReg(5, 7),           // CMP j, N             [24]
		// B.LT j_loop: target=2, branch=25
		// offset = (2 - 25) * 4 = -92
		EncodeBCond(-92, 11), // B.LT j_loop           [25]

		// i++; if i < N goto i_loop
		EncodeADDImm(4, 4, 1, false), // i++                  [26]
		EncodeCMPReg(4, 7),           // CMP i, N             [27]
		// B.LT i_loop: target=1, branch=28
		// offset = (1 - 28) * 4 = -108
		EncodeBCond(-108, 11), // B.LT i_loop           [28]

		// Sum all C elements into X0 for exit code verification
		EncodeADDImm(0, 31, 0, false),   // X0 = 0               [29]
		EncodeADDImm(4, 31, 0, false),   // i = 0                [30]
		EncodeADDImm(15, 31, 16, false), // X15 = 16             [31]

		// sum_loop: offset 32
		encodeLSLImm(12, 4, 3),         // X12 = i * 8           [32]
		EncodeADDReg(12, 3, 12, false), // X12 = &C[i]           [33]
		EncodeLDR64(9, 12, 0),          // X9 = C[i]             [34]
		EncodeADDReg(0, 0, 9, false),   // X0 += C[i]            [35]
		EncodeADDImm(4, 4, 1, false),   // i++                   [36]
		EncodeCMPReg(4, 15),            // CMP i, 16             [37]
		// B.LT sum_loop: target=32, branch=38
		// offset = (32 - 38) * 4 = -24
		EncodeBCond(-24, 11), // B.LT sum_loop         [38]

		EncodeSVC(0), // exit with X0          [39]
	}

	return Benchmark{
		Name:        "matmul_4x4",
		Description: "4x4 integer matrix multiply with triple-nested loops - realistic workload",
		Setup: func(regFile *emu.RegFile, memory *emu.Memory) {
			regFile.WriteReg(8, 93) // X8 = 93 (exit syscall)
			regFile.WriteReg(7, 4)  // X7 = N = 4

			// Matrix A at 0x8000 (4x4, row-major, 64-bit elements)
			// A = [[1,2,3,4],[5,6,7,8],[9,10,11,12],[13,14,15,16]]
			regFile.WriteReg(1, 0x8000)
			for i := uint64(0); i < 16; i++ {
				memory.Write64(0x8000+i*8, i+1)
			}

			// Matrix B at 0x8100 (identity matrix)
			regFile.WriteReg(2, 0x8100)
			for i := uint64(0); i < 16; i++ {
				memory.Write64(0x8100+i*8, 0)
			}
			memory.Write64(0x8100+0*8, 1)  // B[0][0]
			memory.Write64(0x8100+5*8, 1)  // B[1][1]
			memory.Write64(0x8100+10*8, 1) // B[2][2]
			memory.Write64(0x8100+15*8, 1) // B[3][3]

			// Matrix C at 0x8200 (result buffer)
			regFile.WriteReg(3, 0x8200)
		},
		Program: BuildProgram(instrs...),
		// C = A * I = A, so sum(C) = sum(1..16) = 136
		ExpectedExit: 136,
	}
}

// MatmulCalibrationResult holds results for the matmul calibration.
type MatmulCalibrationResult struct {
	Benchmark    string  `json:"benchmark"`
	Mode         string  `json:"mode"`
	Instructions uint64  `json:"instructions"`
	Cycles       uint64  `json:"cycles"`
	CPI          float64 `json:"cpi"`
	ExitCode     int64   `json:"exit_code"`
}

// TestMatmulCalibration runs a 4x4 matrix multiply through the fast timing
// engine and reports CPI for calibration per issue #359.
func TestMatmulCalibration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	bench := buildMatmul4x4()

	// Run through fast timing
	regFile := &emu.RegFile{}
	memory := emu.NewMemory()
	regFile.SP = 0x10000

	if bench.Setup != nil {
		bench.Setup(regFile, memory)
	}

	programAddr := uint64(0x1000)
	for i, b := range bench.Program {
		memory.Write8(programAddr+uint64(i), b)
	}

	timingConfig := latency.DefaultTimingConfig()
	latencyTable := latency.NewTableWithConfig(timingConfig)
	syscallHandler := emu.NewDefaultSyscallHandler(
		regFile, memory, &bytes.Buffer{}, &bytes.Buffer{})

	ft := pipeline.NewFastTiming(regFile, memory, latencyTable, syscallHandler,
		pipeline.WithMaxInstructions(1000000))
	ft.SetPC(programAddr)
	exitCode := ft.Run()

	stats := ft.Stats()
	var cpi float64
	if stats.Instructions > 0 {
		cpi = float64(stats.Cycles) / float64(stats.Instructions)
	}

	// Report correctness — fail the test if exit code is wrong
	if exitCode != bench.ExpectedExit {
		t.Errorf("Exit code mismatch: expected %d, got %d (may indicate unhandled instruction or instruction limit hit)",
			bench.ExpectedExit, exitCode)
	}

	// Report in the format requested by issue #359
	t.Log("=== Matrix Multiply Fast Timing Calibration ===")
	t.Log("")
	t.Logf("Benchmark: %s", bench.Name)
	t.Logf("Mode: fast timing")
	t.Logf("Instructions: %d", stats.Instructions)
	t.Logf("Cycles: %d", stats.Cycles)
	t.Logf("CPI: %.3f", cpi)
	t.Logf("Exit code: %d (expected: %d)", exitCode, bench.ExpectedExit)
	t.Log("")
	t.Log("M2 hardware CPI: (not yet measured for matmul)")
	t.Log("Error: TBD (pending hardware baseline)")

	// Write JSON results
	result := MatmulCalibrationResult{
		Benchmark:    bench.Name,
		Mode:         "fast_timing",
		Instructions: stats.Instructions,
		Cycles:       stats.Cycles,
		CPI:          cpi,
		ExitCode:     exitCode,
	}

	outPath := "matmul_calibration_results.json"
	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Logf("Failed to marshal results to JSON: %v", err)
	} else if writeErr := os.WriteFile(outPath, jsonData, 0644); writeErr != nil {
		t.Logf("Failed to write results JSON to %s: %v", outPath, writeErr)
	} else {
		t.Logf("Results written to %s", outPath)
	}
}
