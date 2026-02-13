// Package benchmarks provides SPEC benchmark execution tests.
// These tests validate that SPEC CPU 2017 binaries can execute
// correctly in M2Sim's emulator and timing pipeline.
package benchmarks

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/sarchlab/m2sim/emu"
	"github.com/sarchlab/m2sim/loader"
)

// specRoot returns the path to the SPEC installation, or empty if not available.
func specRoot() string {
	// Look for SPEC at benchmarks/spec relative to repo root.
	// Walk up from the test binary's working directory.
	candidates := []string{
		"benchmarks/spec",
		"../benchmarks/spec",
		filepath.Join(os.Getenv("SPEC_ROOT"), ""),
	}

	for _, c := range candidates {
		if c == "" {
			continue
		}
		info, err := os.Stat(c)
		if err == nil && info.IsDir() {
			abs, _ := filepath.Abs(c)
			return abs
		}
		// Follow symlink
		real, err := filepath.EvalSymlinks(c)
		if err == nil {
			info, err = os.Stat(real)
			if err == nil && info.IsDir() {
				abs, _ := filepath.Abs(real)
				return abs
			}
		}
	}

	return ""
}

// loadAndRunSPEC loads a SPEC benchmark binary and runs it in emulation mode.
// Returns (exitCode, instructionCount, error).
func loadAndRunSPEC(
	binaryPath string,
	workDir string,
	maxInstructions uint64,
) (int64, uint64, error) {
	prog, err := loader.Load(binaryPath)
	if err != nil {
		return -1, 0, fmt.Errorf("failed to load ELF: %w", err)
	}

	memory := emu.NewMemory()

	// Load segments into memory
	for _, seg := range prog.Segments {
		for i, b := range seg.Data {
			memory.Write8(seg.VirtAddr+uint64(i), b)
		}
		for i := uint64(len(seg.Data)); i < seg.MemSize; i++ {
			memory.Write8(seg.VirtAddr+i, 0)
		}
	}

	// Change to working directory so file I/O finds input files
	if workDir != "" {
		origDir, _ := os.Getwd()
		if err := os.Chdir(workDir); err != nil {
			return -1, 0, fmt.Errorf("failed to chdir to %s: %w", workDir, err)
		}
		defer func() { _ = os.Chdir(origDir) }()
	}

	stdoutBuf := &bytes.Buffer{}
	stderrBuf := &bytes.Buffer{}

	emulator := emu.NewEmulator(
		emu.WithStackPointer(prog.InitialSP),
		emu.WithStdout(stdoutBuf),
		emu.WithStderr(stderrBuf),
		emu.WithMaxInstructions(maxInstructions),
	)
	emulator.LoadProgram(prog.EntryPoint, memory)

	exitCode := emulator.Run()
	count := emulator.InstructionCount()

	return exitCode, count, nil
}

// TestSPECExchange2Emulation validates that 548.exchange2_r can execute
// in M2Sim's emulator. Skips if SPEC is not installed.
func TestSPECExchange2Emulation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	root := specRoot()
	if root == "" {
		t.Skip("SPEC CPU 2017 not available (set SPEC_ROOT or symlink benchmarks/spec)")
	}

	bench := SPECBenchmark{
		Name:           "548.exchange2_r",
		Binary:         "benchspec/CPU/548.exchange2_r/exe/exchange2_r_base.arm64",
		TestArgs:       []string{"0"},
		TestInputFiles: []string{"puzzles.txt"},
		WorkingDir:     "benchspec/CPU/548.exchange2_r/run/run_base_test_arm64.0000",
	}

	binaryPath := filepath.Join(root, bench.Binary)
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Skipf("exchange2_r binary not built at %s — run benchmarks/build_spec.sh first", binaryPath)
	}

	workDir := filepath.Join(root, bench.WorkingDir)
	if _, err := os.Stat(workDir); os.IsNotExist(err) {
		t.Skipf("exchange2_r run directory not set up at %s", workDir)
	}

	// Verify input files exist
	for _, f := range bench.TestInputFiles {
		inputPath := filepath.Join(workDir, f)
		if _, err := os.Stat(inputPath); os.IsNotExist(err) {
			t.Skipf("input file %s not found at %s", f, inputPath)
		}
	}

	t.Logf("Binary: %s", binaryPath)
	t.Logf("Working dir: %s", workDir)

	// First pass: try with a small instruction limit to check for
	// immediate failures (unsupported instructions).
	const probeLimit = 100_000
	exitCode, count, err := loadAndRunSPEC(binaryPath, workDir, probeLimit)
	if err != nil {
		t.Fatalf("Failed to load/run exchange2_r: %v", err)
	}

	t.Logf("Probe run (%d instruction limit): exit=%d, instructions=%d",
		probeLimit, exitCode, count)

	if exitCode == -1 && count < probeLimit {
		t.Errorf("exchange2_r hit an unsupported instruction after %d instructions (exit=-1)", count)
		t.Logf("This likely means an ARM64 instruction used by the Fortran runtime or exchange2_r is not yet implemented in M2Sim")
		return
	}

	if count >= probeLimit {
		t.Logf("exchange2_r executed %d instructions without error — instruction coverage looks sufficient", count)
		t.Logf("Full execution would require more instructions (and should run via CI)")
	}

	if exitCode == 0 {
		t.Logf("exchange2_r completed successfully with exit code 0")
	}
}

// TestSPECAllBenchmarksProbe attempts to load and probe-run all configured
// SPEC benchmarks. Reports which ones are available and whether they hit
// unsupported instructions early.
func TestSPECAllBenchmarksProbe(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	root := specRoot()
	if root == "" {
		t.Skip("SPEC CPU 2017 not available (set SPEC_ROOT or symlink benchmarks/spec)")
	}

	const probeLimit = 50_000

	for _, bench := range GetSPECBenchmarks() {
		t.Run(bench.Name, func(t *testing.T) {
			binaryPath := filepath.Join(root, bench.Binary)
			if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
				t.Skipf("binary not built: %s", binaryPath)
			}

			workDir := filepath.Join(root, bench.WorkingDir)
			if _, err := os.Stat(workDir); os.IsNotExist(err) {
				t.Skipf("run directory not set up: %s", workDir)
			}

			exitCode, count, err := loadAndRunSPEC(binaryPath, workDir, probeLimit)
			if err != nil {
				t.Fatalf("Failed to load: %v", err)
			}

			t.Logf("%s: exit=%d, instructions=%d", bench.Name, exitCode, count)

			if exitCode == -1 && count < probeLimit {
				t.Errorf("%s hit unsupported instruction after %d instructions",
					bench.Name, count)
			} else if count >= probeLimit {
				t.Logf("%s: passed probe (%d instructions without error)", bench.Name, count)
			} else {
				t.Logf("%s: completed with exit code %d after %d instructions",
					bench.Name, exitCode, count)
			}
		})
	}
}
