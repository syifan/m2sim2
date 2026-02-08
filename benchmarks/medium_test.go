// Package benchmarks contains tests for medium-sized benchmarks.
package benchmarks

import (
	"bytes"
	"testing"

	"github.com/sarchlab/m2sim/emu"
	"github.com/sarchlab/m2sim/loader"
)

// TestMatrixMultiplyBenchmark tests the matrix multiply benchmark execution.
// Now enabled after implementing DUP SIMD instruction and MRS system instruction support.
func TestMatrixMultiplyBenchmark(t *testing.T) {

	// Load the matrix multiply benchmark
	prog, err := loader.Load("medium/matmul")
	if err != nil {
		t.Fatalf("Failed to load matrix multiply benchmark: %v", err)
	}

	// Create memory and load program segments
	memory := emu.NewMemory()
	for _, seg := range prog.Segments {
		for i, b := range seg.Data {
			memory.Write8(seg.VirtAddr+uint64(i), b)
		}
		// Zero-fill BSS (memsize > filesize)
		for i := uint64(len(seg.Data)); i < seg.MemSize; i++ {
			memory.Write8(seg.VirtAddr+i, 0)
		}
	}

	// Capture stdout
	var stdout bytes.Buffer

	// Create emulator
	emulator := emu.NewEmulator(
		emu.WithStackPointer(prog.InitialSP),
		emu.WithStdout(&stdout),
	)

	// Load program into emulator
	emulator.LoadProgram(prog.EntryPoint, memory)

	// Run until completion
	exitCode := emulator.Run()

	// Check results
	t.Logf("Matrix multiply benchmark completed")
	t.Logf("Exit code: %d", exitCode)
	t.Logf("Output:\n%s", stdout.String())

	// Verify successful completion
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

	// Verify expected output structure
	output := stdout.String()
	if output == "" {
		t.Error("Expected output, got empty string")
	}

	// Check for key output markers
	expectedMarkers := []string{
		"Matrix Multiply Benchmark",
		"Initializing matrices",
		"Performing matrix multiplication",
		"Computing checksum",
		"Checksum:",
		"Verification: PASSED",
		"Benchmark completed successfully",
	}

	for _, marker := range expectedMarkers {
		if !bytes.Contains([]byte(output), []byte(marker)) {
			t.Errorf("Expected output to contain '%s', but it was missing", marker)
		}
	}

	t.Logf("Matrix multiply benchmark test completed successfully")
}
