// Package benchmarks provides timing benchmark infrastructure for M2Sim calibration.
package benchmarks

import (
	"bytes"
	"strings"
	"testing"
)

func TestHarnessRunsAllBenchmarks(t *testing.T) {
	config := DefaultConfig()
	config.Output = &bytes.Buffer{}
	config.Verbose = false
	// Disable caches for faster testing
	config.EnableICache = false
	config.EnableDCache = false

	harness := NewHarness(config)
	harness.AddBenchmarks(GetMicrobenchmarks())

	results := harness.RunAll()

	if len(results) != 6 {
		t.Errorf("expected 6 benchmark results, got %d", len(results))
	}

	// Verify each benchmark completed
	for _, r := range results {
		if r.SimulatedCycles == 0 {
			t.Errorf("benchmark %s has 0 cycles", r.Name)
		}
		if r.InstructionsRetired == 0 {
			t.Errorf("benchmark %s has 0 instructions", r.Name)
		}
		t.Logf("✓ %s: cycles=%d, insts=%d, CPI=%.3f, exit=%d",
			r.Name, r.SimulatedCycles, r.InstructionsRetired, r.CPI, r.ExitCode)
	}
}

func TestArithmeticSequential(t *testing.T) {
	config := DefaultConfig()
	config.Output = &bytes.Buffer{}
	config.EnableICache = false
	config.EnableDCache = false

	harness := NewHarness(config)
	harness.AddBenchmark(arithmeticSequential())

	results := harness.RunAll()

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	r := results[0]
	if r.ExitCode != 4 {
		t.Errorf("expected exit code 4, got %d", r.ExitCode)
	}

	t.Logf("arithmetic_sequential: cycles=%d, insts=%d, CPI=%.3f",
		r.SimulatedCycles, r.InstructionsRetired, r.CPI)
}

func TestDependencyChain(t *testing.T) {
	config := DefaultConfig()
	config.Output = &bytes.Buffer{}
	config.EnableICache = false
	config.EnableDCache = false

	harness := NewHarness(config)
	harness.AddBenchmark(dependencyChain())

	results := harness.RunAll()

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	r := results[0]
	if r.ExitCode != 20 {
		t.Errorf("expected exit code 20, got %d", r.ExitCode)
	}

	t.Logf("dependency_chain: cycles=%d, insts=%d, CPI=%.3f",
		r.SimulatedCycles, r.InstructionsRetired, r.CPI)
}

func TestMemorySequential(t *testing.T) {
	config := DefaultConfig()
	config.Output = &bytes.Buffer{}
	config.EnableICache = false
	config.EnableDCache = false

	harness := NewHarness(config)
	harness.AddBenchmark(memorySequential())

	results := harness.RunAll()

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	r := results[0]
	if r.ExitCode != 42 {
		t.Errorf("expected exit code 42, got %d", r.ExitCode)
	}

	t.Logf("memory_sequential: cycles=%d, insts=%d, CPI=%.3f",
		r.SimulatedCycles, r.InstructionsRetired, r.CPI)
}

func TestFunctionCalls(t *testing.T) {
	config := DefaultConfig()
	config.Output = &bytes.Buffer{}
	config.EnableICache = false
	config.EnableDCache = false

	harness := NewHarness(config)
	harness.AddBenchmark(functionCalls())

	results := harness.RunAll()

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	r := results[0]
	if r.ExitCode != 5 {
		t.Errorf("expected exit code 5, got %d", r.ExitCode)
	}

	t.Logf("function_calls: cycles=%d, insts=%d, CPI=%.3f, flushes=%d",
		r.SimulatedCycles, r.InstructionsRetired, r.CPI, r.PipelineFlushes)
}

func TestBranchTaken(t *testing.T) {
	config := DefaultConfig()
	config.Output = &bytes.Buffer{}
	config.EnableICache = false
	config.EnableDCache = false

	harness := NewHarness(config)
	harness.AddBenchmark(branchTaken())

	results := harness.RunAll()

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	r := results[0]
	if r.ExitCode != 5 {
		t.Errorf("expected exit code 5, got %d", r.ExitCode)
	}

	t.Logf("branch_taken: cycles=%d, insts=%d, CPI=%.3f, flushes=%d",
		r.SimulatedCycles, r.InstructionsRetired, r.CPI, r.PipelineFlushes)
}

func TestMixedOperations(t *testing.T) {
	config := DefaultConfig()
	config.Output = &bytes.Buffer{}
	config.EnableICache = false
	config.EnableDCache = false

	harness := NewHarness(config)
	harness.AddBenchmark(mixedOperations())

	results := harness.RunAll()

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	r := results[0]
	if r.ExitCode != 95 {
		t.Errorf("expected exit code 95, got %d", r.ExitCode)
	}

	t.Logf("mixed_operations: cycles=%d, insts=%d, CPI=%.3f",
		r.SimulatedCycles, r.InstructionsRetired, r.CPI)
}

func TestPrintResults(t *testing.T) {
	buf := &bytes.Buffer{}
	config := DefaultConfig()
	config.Output = buf
	config.EnableICache = false
	config.EnableDCache = false

	harness := NewHarness(config)
	harness.AddBenchmark(arithmeticSequential())

	results := harness.RunAll()
	harness.PrintResults(results)

	output := buf.String()
	if !strings.Contains(output, "arithmetic_sequential") {
		t.Error("output should contain benchmark name")
	}
	if !strings.Contains(output, "Simulated Cycles") {
		t.Error("output should contain cycle count header")
	}
}

func TestPrintCSV(t *testing.T) {
	buf := &bytes.Buffer{}
	config := DefaultConfig()
	config.Output = buf
	config.EnableICache = false
	config.EnableDCache = false

	harness := NewHarness(config)
	harness.AddBenchmark(arithmeticSequential())

	results := harness.RunAll()
	harness.PrintCSV(results)

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	if len(lines) != 2 {
		t.Errorf("expected 2 lines (header + data), got %d", len(lines))
	}

	if !strings.Contains(lines[0], "name,cycles,instructions") {
		t.Error("CSV header should contain expected columns")
	}

	if !strings.Contains(lines[1], "arithmetic_sequential") {
		t.Error("CSV data should contain benchmark name")
	}
}

func TestWithoutCaches(t *testing.T) {
	config := DefaultConfig()
	config.Output = &bytes.Buffer{}
	config.EnableICache = false
	config.EnableDCache = false

	harness := NewHarness(config)
	harness.AddBenchmark(arithmeticSequential())

	results := harness.RunAll()

	r := results[0]
	if r.SimulatedCycles == 0 {
		t.Error("should still have cycles without caches")
	}

	// Cache stats should be zero
	if r.ICacheHits != 0 || r.ICacheMisses != 0 {
		t.Error("I-cache stats should be zero when disabled")
	}

	t.Logf("arithmetic_sequential (no cache): cycles=%d, insts=%d, CPI=%.3f",
		r.SimulatedCycles, r.InstructionsRetired, r.CPI)
}
