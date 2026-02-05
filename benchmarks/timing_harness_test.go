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

	if len(results) != 10 {
		t.Errorf("expected 10 benchmark results, got %d", len(results))
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
	// With proper load-use hazard detection for stores, the benchmark correctly
	// preserves X0's value (42) through all store-load pairs. The pipeline stalls
	// when a load result is needed by a subsequent store.
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
	// Correct calculation: (0+10)+10+5=25, (25+10)+35+5=75, (75+10)+85=170... wait
	// Actually: iter1: X0=0+10=10, call +5=15; iter2: X0=15+25=40, call +5=45;
	// iter3: X0=45+55=100. So 100 is correct.
	if r.ExitCode != 100 {
		t.Errorf("expected exit code 100, got %d", r.ExitCode)
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

func TestPrintJSON(t *testing.T) {
	buf := &bytes.Buffer{}
	config := DefaultConfig()
	config.Output = buf
	config.EnableICache = false
	config.EnableDCache = false

	harness := NewHarness(config)
	harness.AddBenchmarks(GetCoreBenchmarks())

	results := harness.RunAll()
	err := harness.PrintJSON(results)

	if err != nil {
		t.Fatalf("PrintJSON failed: %v", err)
	}

	output := buf.String()

	// Check JSON structure
	if !strings.Contains(output, `"metadata"`) {
		t.Error("JSON should contain metadata")
	}
	if !strings.Contains(output, `"results"`) {
		t.Error("JSON should contain results")
	}
	if !strings.Contains(output, `"summary"`) {
		t.Error("JSON should contain summary")
	}
	if !strings.Contains(output, `"simulated_cycles"`) {
		t.Error("JSON should contain simulated_cycles field")
	}
	if !strings.Contains(output, `"loop_simulation"`) {
		t.Error("JSON should contain loop_simulation benchmark")
	}
}

func TestGetCoreBenchmarks(t *testing.T) {
	benchmarks := GetCoreBenchmarks()

	if len(benchmarks) != 3 {
		t.Errorf("expected 3 core benchmarks, got %d", len(benchmarks))
	}

	// Verify we have loop, matrix, and branch benchmarks
	names := make(map[string]bool)
	for _, b := range benchmarks {
		names[b.Name] = true
	}

	if !names["loop_simulation"] {
		t.Error("core benchmarks should include loop_simulation")
	}
	if !names["matrix_operations"] {
		t.Error("core benchmarks should include matrix_operations")
	}
	if !names["branch_taken_conditional"] {
		t.Error("core benchmarks should include branch_taken_conditional")
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
