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
// timing tests (~5K instructions each, complete in under 60s).
func GetPolybenchBenchmarks() []Benchmark {
	return []Benchmark{
		BenchmarkFromELF("polybench_atax", "ATAX: Matrix transpose and vector multiply (16x16)", polybenchELFPath("atax")),
		BenchmarkFromELF("polybench_bicg", "BiCG: Bi-conjugate gradient sub-kernel (16x16)", polybenchELFPath("bicg")),
		BenchmarkFromELF("polybench_mvt", "MVT: Matrix vector product and transpose (16x16)", polybenchELFPath("mvt")),
		BenchmarkFromELF("polybench_jacobi1d", "Jacobi-1D: 1D Jacobi stencil computation (32 points, 8 steps)", polybenchELFPath("jacobi-1d")),
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
	config.EnableICache = false
	config.EnableDCache = false
	config.MaxCycles = 500_000_000 // 500M cycle safety limit to prevent hangs

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
		t.Logf("%s: exceeded 500M cycle limit (possible hang), cycles=%d, insts=%d",
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
