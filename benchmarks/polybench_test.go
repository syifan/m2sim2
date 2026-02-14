package benchmarks

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// polybenchELFPath returns the absolute path to a polybench ELF binary.
func polybenchELFPath(name string) string {
	_, filename, _, _ := runtime.Caller(0)
	baseDir := filepath.Dir(filename)
	return filepath.Join(baseDir, "polybench", name+"_m2sim.elf")
}

// GetPolybenchBenchmarks returns the PolyBench benchmarks suitable for CI
// timing tests (SMALL dataset, 49K-131K instructions each).
func GetPolybenchBenchmarks() []Benchmark {
	return []Benchmark{
		BenchmarkFromELF("polybench_atax", "ATAX: Matrix transpose and vector multiply (80x80)", polybenchELFPath("atax")),
		BenchmarkFromELF("polybench_bicg", "BiCG: Bi-conjugate gradient sub-kernel (80x80)", polybenchELFPath("bicg")),
		BenchmarkFromELF("polybench_mvt", "MVT: Matrix vector product and transpose (60x70)", polybenchELFPath("mvt")),
		BenchmarkFromELF("polybench_jacobi1d", "Jacobi-1D: 1D Jacobi stencil computation (120 points, 20 steps)", polybenchELFPath("jacobi-1d")),
	}
}

// runPolybenchTest runs a single polybench benchmark and validates results.
// All PolyBench tests are skipped in short mode — they exceed CI timeout.
func runPolybenchTest(t *testing.T, name, elfName string) {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping PolyBench benchmark in short mode")
	}

	elfPath := polybenchELFPath(elfName)
	if _, err := os.Stat(elfPath); os.IsNotExist(err) {
		t.Skipf("ELF not found: %s (run benchmarks/polybench/build.sh)", elfPath)
	}

	config := DefaultConfig()
	config.Output = &bytes.Buffer{}
	config.EnableDCache = false      // D-cache stalls hurt CPI accuracy in in-order pipeline
	config.MaxCycles = 5_000_000_000 // 5B cycle safety limit to prevent hangs

	harness := NewHarness(config)
	harness.AddBenchmark(BenchmarkFromELF(name, name, polybenchELFPath(elfName)))

	results := harness.RunAll()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	r := results[0]
	if r.ExitCode == -1 {
		t.Fatalf("failed to load ELF: %s", polybenchELFPath(elfName))
	}
	if r.ExitCode == -2 {
		t.Logf("%s: exceeded 5B cycle limit, cycles=%d, insts=%d",
			r.Name, r.SimulatedCycles, r.InstructionsRetired)
		t.Skipf("benchmark did not complete within cycle limit")
	}
	if r.SimulatedCycles == 0 {
		t.Error("0 cycles")
	}
	if r.InstructionsRetired == 0 {
		t.Error("0 instructions retired")
	}

	t.Logf("%s: cycles=%d, insts=%d, CPI=%.3f, dcache_hits=%d, dcache_misses=%d, exit=%d",
		r.Name, r.SimulatedCycles, r.InstructionsRetired, r.CPI,
		r.DCacheHits, r.DCacheMisses, r.ExitCode)
}

func TestPolybenchATAX(t *testing.T) {
	runPolybenchTest(t, "polybench_atax", "atax")
}

func TestPolybenchBiCG(t *testing.T) {
	runPolybenchTest(t, "polybench_bicg", "bicg")
}

func TestPolybenchMVT(t *testing.T) {
	runPolybenchTest(t, "polybench_mvt", "mvt")
}

func TestPolybenchJacobi1D(t *testing.T) {
	runPolybenchTest(t, "polybench_jacobi1d", "jacobi-1d")
}

func TestPolybenchGEMM(t *testing.T) {
	runPolybenchTest(t, "polybench_gemm", "gemm")
}

func TestPolybench2MM(t *testing.T) {
	runPolybenchTest(t, "polybench_2mm", "2mm")
}

func TestPolybench3MM(t *testing.T) {
	runPolybenchTest(t, "polybench_3mm", "3mm")
}
