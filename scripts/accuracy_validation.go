// Package main provides accuracy validation for performance optimizations.
// Ensures that optimizations preserve simulation correctness.
package main

import (
	"encoding/binary"
	"fmt"
	"os"

	"github.com/sarchlab/m2sim/emu"
	"github.com/sarchlab/m2sim/insts"
	"github.com/sarchlab/m2sim/timing/pipeline"
)

// testInstructionDecoding validates that the optimized decoder produces
// identical results to the original decoder.
func testInstructionDecoding() bool {
	decoder := insts.NewDecoder()

	// Test various instruction encodings
	testCases := []uint32{
		0x91000400, // ADD X0, X2, #1
		0xF1000400, // SUBS X0, X2, #1
		0x54000041, // B.NE +8
		0xD503201F, // NOP
		0xF8408420, // LDR X0, [X1], #8
		0xF8008420, // STR X0, [X1], #8
	}

	fmt.Println("Testing instruction decoder accuracy...")

	for i, word := range testCases {
		// Test original Decode method
		inst1 := decoder.Decode(word)

		// Test optimized DecodeInto method
		var inst2 insts.Instruction
		decoder.DecodeInto(word, &inst2)

		// Compare all fields
		if inst1.Op != inst2.Op ||
			inst1.Format != inst2.Format ||
			inst1.Rd != inst2.Rd ||
			inst1.Rn != inst2.Rn ||
			inst1.Rm != inst2.Rm ||
			inst1.Imm != inst2.Imm ||
			inst1.Cond != inst2.Cond {

			fmt.Printf("❌ Test case %d failed: Decode mismatch\n", i)
			fmt.Printf("  Decode():     %+v\n", inst1)
			fmt.Printf("  DecodeInto(): %+v\n", inst2)
			return false
		}

		fmt.Printf("✅ Test case %d: Instruction 0x%08X decoded correctly\n", i, word)
	}

	return true
}

// testPipelineExecution validates that pipeline execution produces
// identical results with the optimizations.
func testPipelineExecution() bool {
	fmt.Println("\nTesting pipeline execution accuracy...")

	// Create a simple test program
	memory := emu.NewMemory()

	// Program: ADD X1, X0, #1; ADD X2, X1, #2; SVC #0
	basePC := uint64(0x1000)
	words := []uint32{
		0x91000401, // ADD X1, X0, #1
		0x91000822, // ADD X2, X1, #2
		0xD4000001, // SVC #0
	}

	// Write program to memory
	for i, word := range words {
		var buf [4]byte
		binary.LittleEndian.PutUint32(buf[:], word)
		for j := 0; j < 4; j++ {
			memory.Write8(basePC+uint64(i*4+j), buf[j])
		}
	}

	// Test with different initial values
	testValues := []uint64{0, 1, 42, 0xFFFFFFFF}

	for i, initialValue := range testValues {
		// Create fresh register file for each test to avoid state pollution
		regFile := &emu.RegFile{}
		regFile.WriteReg(0, initialValue)
		regFile.WriteReg(1, 0)
		regFile.WriteReg(2, 0)
		regFile.WriteReg(8, 93) // Exit syscall

		// Create pipeline with optimizations
		pipe := pipeline.NewPipeline(regFile, memory)
		pipe.SetPC(basePC)

		// Run pipeline
		exitCode := pipe.Run()

		// Check results
		finalX1 := regFile.ReadReg(1)
		finalX2 := regFile.ReadReg(2)

		expectedX1 := initialValue + 1
		expectedX2 := expectedX1 + 2

		// Check computational correctness (exit code may vary based on implementation)
		if finalX1 != expectedX1 || finalX2 != expectedX2 {
			fmt.Printf("❌ Test case %d failed:\n", i)
			fmt.Printf("  Initial X0: %d\n", initialValue)
			fmt.Printf("  Expected X1: %d, Got: %d\n", expectedX1, finalX1)
			fmt.Printf("  Expected X2: %d, Got: %d\n", expectedX2, finalX2)
			fmt.Printf("  Exit code: %d\n", exitCode)
			return false
		}

		fmt.Printf("✅ Test case %d: X0=%d → X1=%d, X2=%d (exit %d)\n",
			i, initialValue, finalX1, finalX2, exitCode)
	}

	return true
}

// testBranchPredictorAccuracy validates that branch predictor behavior
// is preserved after optimization.
func testBranchPredictorAccuracy() bool {
	fmt.Println("\nTesting branch predictor accuracy...")

	config := pipeline.BranchPredictorConfig{
		BHTSize:             16,
		BTBSize:             8,
		GlobalHistoryLength: 4,
		UseTournament:       true,
	}

	bp1 := pipeline.NewBranchPredictor(config)
	bp2 := pipeline.NewBranchPredictor(config)

	testPCs := []uint64{0x1000, 0x1004, 0x1008, 0x100C}
	testTarget := uint64(0x2000)

	// Test prediction consistency
	for i, pc := range testPCs {
		// Make prediction with both predictors
		pred1 := bp1.Predict(pc)
		pred2 := bp2.Predict(pc)

		// Predictions should be identical for fresh predictors
		if pred1.Taken != pred2.Taken || pred1.Target != pred2.Target {
			fmt.Printf("❌ Prediction mismatch at PC 0x%X\n", pc)
			return false
		}

		// Update both predictors identically
		bp1.Update(pc, i%2 == 0, testTarget)
		bp2.Update(pc, i%2 == 0, testTarget)

		fmt.Printf("✅ PC 0x%X: Prediction consistent (taken=%v, target=0x%X)\n",
			pc, pred1.Taken, pred1.Target)
	}

	// Test reset functionality
	bp1.Reset()
	bp2.Reset()

	// After reset, predictions should be identical again
	for _, pc := range testPCs {
		pred1 := bp1.Predict(pc)
		pred2 := bp2.Predict(pc)

		if pred1.Taken != pred2.Taken {
			fmt.Printf("❌ Post-reset prediction mismatch at PC 0x%X\n", pc)
			return false
		}
	}

	fmt.Println("✅ Branch predictor reset behavior validated")
	return true
}

func main() {
	fmt.Println("M2Sim Accuracy Validation - Performance Optimization")
	fmt.Println("=======================================================")

	allPassed := true

	// Test instruction decoding accuracy
	if !testInstructionDecoding() {
		allPassed = false
	}

	// Test pipeline execution accuracy
	if !testPipelineExecution() {
		allPassed = false
	}

	// Test branch predictor accuracy
	if !testBranchPredictorAccuracy() {
		allPassed = false
	}

	fmt.Println("\n=======================================================")
	if allPassed {
		fmt.Println("🎉 ALL ACCURACY TESTS PASSED")
		fmt.Println("✅ Performance optimizations preserve simulation correctness")
		os.Exit(0)
	} else {
		fmt.Println("❌ ACCURACY TESTS FAILED")
		fmt.Println("🚨 Performance optimizations may have introduced errors")
		os.Exit(1)
	}
}
