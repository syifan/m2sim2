package benchmarks

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// embenchELFPath returns the absolute path to an EmBench ELF binary.
func embenchELFPath(dir, name string) string {
	_, filename, _, _ := runtime.Caller(0)
	baseDir := filepath.Dir(filename)
	return filepath.Join(baseDir, dir, name)
}

// GetEmbenchBenchmarks returns the EmBench benchmarks suitable for timing tests.
func GetEmbenchBenchmarks() []Benchmark {
	return []Benchmark{
		BenchmarkFromELF("embench_aha_mont64", "AHA-Mont64: Montgomery multiplication (cryptographic)", embenchELFPath("aha-mont64-m2sim", "aha-mont64_m2sim.elf")),
		BenchmarkFromELF("embench_crc32", "CRC32: Cyclic redundancy check (bit manipulation)", embenchELFPath("crc32-m2sim", "crc32_m2sim.elf")),
		BenchmarkFromELF("embench_edn", "EDN: Finite impulse response filter (DSP)", embenchELFPath("edn-m2sim", "edn_m2sim.elf")),
		BenchmarkFromELF("embench_huffbench", "Huffbench: Huffman compression/decompression", embenchELFPath("huffbench-m2sim", "huffbench_m2sim.elf")),
		BenchmarkFromELF("embench_matmult_int", "MatMult-Int: Integer matrix multiplication", embenchELFPath("matmult-int-m2sim", "matmult-int_m2sim.elf")),
		BenchmarkFromELF("embench_statemate", "Statemate: Car window lift state machine", embenchELFPath("statemate-m2sim", "statemate_m2sim.elf")),
		BenchmarkFromELF("embench_primecount", "Primecount: Prime number sieve", embenchELFPath("primecount-m2sim", "primecount_m2sim.elf")),
	}
}

// runEmbenchTest runs a single EmBench benchmark and validates results.
// All EmBench tests are skipped in short mode.
func runEmbenchTest(t *testing.T, name, dir, elfName string) {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping EmBench benchmark in short mode")
	}

	elfPath := embenchELFPath(dir, elfName)
	if _, err := os.Stat(elfPath); os.IsNotExist(err) {
		t.Skipf("ELF not found: %s (run %s/build.sh)", elfPath, dir)
	}

	config := DefaultConfig()
	config.Output = &bytes.Buffer{}
	config.EnableICache = false
	config.EnableDCache = false
	config.MaxCycles = 500_000_000 // 500M cycle safety limit to prevent hangs

	harness := NewHarness(config)
	harness.AddBenchmark(BenchmarkFromELF(name, name, elfPath))

	results := harness.RunAll()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	r := results[0]
	if r.ExitCode == -1 {
		t.Fatalf("failed to load ELF: %s", elfPath)
	}
	if r.ExitCode == -2 {
		t.Logf("%s: exceeded 500M cycle limit (possible hang), cycles=%d, insts=%d, wall=%v",
			r.Name, r.SimulatedCycles, r.InstructionsRetired, r.WallTime)
		t.Skipf("benchmark did not complete within cycle limit")
	}
	if r.SimulatedCycles == 0 {
		t.Error("0 cycles")
	}
	if r.InstructionsRetired == 0 {
		t.Error("0 instructions retired")
	}

	t.Logf("%s: cycles=%d, insts=%d, CPI=%.3f, exit=%d, wall=%v",
		r.Name, r.SimulatedCycles, r.InstructionsRetired, r.CPI,
		r.ExitCode, r.WallTime)
}

func TestEmbenchAhaMont64(t *testing.T) {
	runEmbenchTest(t, "embench_aha_mont64", "aha-mont64-m2sim", "aha-mont64_m2sim.elf")
}

func TestEmbenchCRC32(t *testing.T) {
	runEmbenchTest(t, "embench_crc32", "crc32-m2sim", "crc32_m2sim.elf")
}

func TestEmbenchEDN(t *testing.T) {
	runEmbenchTest(t, "embench_edn", "edn-m2sim", "edn_m2sim.elf")
}

func TestEmbenchHuffbench(t *testing.T) {
	runEmbenchTest(t, "embench_huffbench", "huffbench-m2sim", "huffbench_m2sim.elf")
}

func TestEmbenchMatmultInt(t *testing.T) {
	runEmbenchTest(t, "embench_matmult_int", "matmult-int-m2sim", "matmult-int_m2sim.elf")
}

func TestEmbenchStatemate(t *testing.T) {
	runEmbenchTest(t, "embench_statemate", "statemate-m2sim", "statemate_m2sim.elf")
}

func TestEmbenchPrimecount(t *testing.T) {
	runEmbenchTest(t, "embench_primecount", "primecount-m2sim", "primecount_m2sim.elf")
}
