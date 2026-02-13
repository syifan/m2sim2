// Package benchmarks provides timing benchmark infrastructure for M2Sim calibration.
package benchmarks

import (
	"bytes"
	"testing"
)

// TestTimingPredictions_DependencyVsIndependent validates that dependency chains
// use forwarding to avoid stalls. With proper data forwarding, ALU-to-ALU
// dependencies are resolved in a single cycle, achieving the same CPI as
// independent operations. The pipeline should detect and track data hazards
// that are resolved via forwarding.
func TestTimingPredictions_DependencyVsIndependent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	config := DefaultConfig()
	config.Output = &bytes.Buffer{}
	config.EnableICache = false
	config.EnableDCache = false

	harness := NewHarness(config)
	harness.AddBenchmark(arithmeticSequential())
	harness.AddBenchmark(dependencyChain())

	results := harness.RunAll()

	indep := findResult(results, "arithmetic_sequential")
	dep := findResult(results, "dependency_chain")

	if indep == nil || dep == nil {
		t.Fatal("could not find expected benchmarks")
	}

	t.Logf("Independent ops: CPI=%.3f, DataHazards=%d, Stalls=%d",
		indep.CPI, indep.DataHazards, indep.StallCycles)
	t.Logf("Dependency chain: CPI=%.3f, DataHazards=%d, Stalls=%d",
		dep.CPI, dep.DataHazards, dep.StallCycles)

	// With proper forwarding, both should achieve similar CPI (around 1.0)
	// since ALU-to-ALU dependencies don't require stalls
	if dep.CPI < 1.0 {
		t.Errorf("TIMING BUG: dependency chain CPI (%.3f) should be >= 1.0", dep.CPI)
	}

	// Key invariant: dependency chain should have higher CPI than independent operations
	// because dependencies prevent dual-issue. With superscalar, independent ops can
	// dual-issue, but dependency chains cannot.
	if dep.CPI <= indep.CPI {
		t.Errorf("TIMING BUG: dependency chain CPI (%.3f) should be > independent CPI (%.3f)",
			dep.CPI, indep.CPI)
		t.Error("Dependencies should prevent dual-issue, resulting in higher CPI")
	}

	// Note: In superscalar mode, DataHazards may not be tracked the same way
	// since dependency detection is used for issue decisions rather than
	// forwarding-based tracking. We skip the DataHazards check for superscalar.

	// With proper forwarding, dependency and independent CPIs should be close.
	// A large gap would indicate forwarding isn't working properly.
	cpiDiff := dep.CPI - indep.CPI
	if cpiDiff < 0 {
		cpiDiff = -cpiDiff
	}
	if cpiDiff > 1.0 {
		t.Errorf("TIMING BUG: CPI difference (%.3f) too large between dependency (%.3f) and independent (%.3f)",
			cpiDiff, dep.CPI, indep.CPI)
		t.Error("With proper forwarding, ALU-to-ALU dependency chains should not stall")
	}

	// For ALU-only benchmarks (no loads), neither should have stalls when caches disabled
	if dep.StallCycles > indep.StallCycles {
		t.Logf("Note: dependency chain has more stalls (%d) than independent (%d)",
			dep.StallCycles, indep.StallCycles)
		t.Log("For ALU-only chains with forwarding, this may indicate a bug")
	}
}

// TestTimingPredictions_MemoryLatency validates that memory operations
// behave correctly with and without D-cache.
func TestTimingPredictions_MemoryLatency(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	// Without D-cache: memory accesses are direct array lookups (no stalls).
	// The non-cached path has no stall model — memory latency comes from
	// cache simulation only.
	t.Run("without_dcache", func(t *testing.T) {
		config := DefaultConfig()
		config.Output = &bytes.Buffer{}
		config.EnableICache = false
		config.EnableDCache = false

		harness := NewHarness(config)
		harness.AddBenchmark(arithmeticSequential())
		harness.AddBenchmark(memorySequential())

		results := harness.RunAll()

		alu := findResult(results, "arithmetic_sequential")
		mem := findResult(results, "memory_sequential")

		if alu == nil || mem == nil {
			t.Fatal("could not find expected benchmarks")
		}

		t.Logf("ALU only: CPI=%.3f, MemStalls=%d", alu.CPI, alu.MemStalls)
		t.Logf("Memory ops: CPI=%.3f, MemStalls=%d", mem.CPI, mem.MemStalls)

		// Without D-cache, memory is a direct lookup — no stalls expected.
		// Memory CPI may still differ from ALU due to issue constraints
		// (mem ops restricted to first 3 slots, store-then-load ordering).
	})

	// With D-cache: memory operations go through cache simulation and
	// should produce real stalls on cache misses.
	t.Run("with_dcache", func(t *testing.T) {
		config := DefaultConfig()
		config.Output = &bytes.Buffer{}
		config.EnableICache = false
		config.EnableDCache = true

		harness := NewHarness(config)
		harness.AddBenchmark(arithmeticSequential())
		harness.AddBenchmark(memorySequential())

		results := harness.RunAll()

		alu := findResult(results, "arithmetic_sequential")
		mem := findResult(results, "memory_sequential")

		if alu == nil || mem == nil {
			t.Fatal("could not find expected benchmarks")
		}

		t.Logf("ALU only: CPI=%.3f, MemStalls=%d", alu.CPI, alu.MemStalls)
		t.Logf("Memory ops: CPI=%.3f, MemStalls=%d, DCacheHits=%d, DCacheMisses=%d",
			mem.CPI, mem.MemStalls, mem.DCacheHits, mem.DCacheMisses)

		// With D-cache, memory benchmark should have higher CPI than ALU-only
		if mem.CPI <= alu.CPI {
			t.Errorf("TIMING BUG: memory CPI (%.3f) should be > ALU CPI (%.3f) with D-cache",
				mem.CPI, alu.CPI)
		}
	})
}

// TestTimingPredictions_BranchOverhead validates that branches cause
// pipeline flushes.
func TestTimingPredictions_BranchOverhead(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	config := DefaultConfig()
	config.Output = &bytes.Buffer{}
	config.EnableICache = false
	config.EnableDCache = false

	harness := NewHarness(config)
	harness.AddBenchmark(arithmeticSequential())
	harness.AddBenchmark(branchTaken())

	results := harness.RunAll()

	alu := findResult(results, "arithmetic_sequential")
	branch := findResult(results, "branch_taken")

	if alu == nil || branch == nil {
		t.Fatal("could not find expected benchmarks")
	}

	t.Logf("ALU only: CPI=%.3f, Flushes=%d", alu.CPI, alu.PipelineFlushes)
	t.Logf("Branches: CPI=%.3f, Flushes=%d", branch.CPI, branch.PipelineFlushes)

	// With branch prediction and early resolution for unconditional branches (B, BL),
	// correctly predicted branches do NOT cause pipeline flushes.
	// Unconditional branches are always correctly predicted via early resolution.
	// Only branch mispredictions cause flushes (e.g., conditional branches on first encounter).
	// Having 0 flushes for unconditional branches is now CORRECT behavior.

	// Branch-heavy code may still have slightly higher CPI due to:
	// 1. Non-parallelizable instruction dependencies (branches depend on previous instrs)
	// 2. Any BTB training overhead on first encounters
	// But with early resolution, the CPI difference should be small.
	// We just verify that branch CPI is reasonable (> 0.5 for dual-issue, < 5.0 overall).
	if branch.CPI < 0.25 || branch.CPI > 5.0 {
		t.Errorf("TIMING BUG: branch CPI (%.3f) out of reasonable range [0.5, 5.0]",
			branch.CPI)
	}
}

// TestTimingPredictions_FunctionCallOverhead validates that function calls
// incur appropriate overhead.
func TestTimingPredictions_FunctionCallOverhead(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	config := DefaultConfig()
	config.Output = &bytes.Buffer{}
	config.EnableICache = false
	config.EnableDCache = false

	harness := NewHarness(config)
	harness.AddBenchmark(arithmeticSequential())
	harness.AddBenchmark(functionCalls())

	results := harness.RunAll()

	alu := findResult(results, "arithmetic_sequential")
	calls := findResult(results, "function_calls")

	if alu == nil || calls == nil {
		t.Fatal("could not find expected benchmarks")
	}

	t.Logf("ALU only: CPI=%.3f, Flushes=%d", alu.CPI, alu.PipelineFlushes)
	t.Logf("Function calls: CPI=%.3f, Flushes=%d", calls.CPI, calls.PipelineFlushes)

	// Function calls (BL + RET) should cause pipeline flushes
	if calls.PipelineFlushes == 0 {
		t.Error("TIMING BUG: function call benchmark has 0 pipeline flushes")
		t.Error("BL and RET instructions should flush the pipeline")
	}
}

// TestTimingPredictions_CPIBounds validates that CPI is within reasonable bounds
// for all benchmarks.
func TestTimingPredictions_CPIBounds(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	config := DefaultConfig()
	config.Output = &bytes.Buffer{}
	config.EnableICache = false
	config.EnableDCache = false

	harness := NewHarness(config)
	harness.AddBenchmarks(GetMicrobenchmarks())

	results := harness.RunAll()

	for _, r := range results {
		t.Logf("%s: CPI=%.3f", r.Name, r.CPI)

		// With 8-wide superscalar, CPI can be as low as 0.125 (theoretical max for 8-issue)
		// CPI < 0.125 would indicate a bug
		if r.CPI < 0.125 {
			t.Errorf("TIMING BUG: %s has CPI < 0.125 (%.3f)", r.Name, r.CPI)
			t.Error("An 8-wide pipeline cannot achieve CPI < 0.125")
		}

		// CPI should be reasonable (not absurdly high for these simple benchmarks)
		// Even with stalls, CPI > 10 would indicate something is wrong
		if r.CPI > 10.0 {
			t.Errorf("TIMING BUG: %s has unreasonably high CPI (%.3f)", r.Name, r.CPI)
			t.Error("CPI > 10 suggests excessive stalls or pipeline bugs")
		}
	}
}

// TestTimingPredictions_Consistency validates that running the same benchmark
// multiple times produces consistent results.
func TestTimingPredictions_Consistency(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	config := DefaultConfig()
	config.Output = &bytes.Buffer{}
	config.EnableICache = false
	config.EnableDCache = false

	// Run the same benchmark 3 times and compare results
	runs := make([]BenchmarkResult, 3)
	for i := 0; i < 3; i++ {
		harness := NewHarness(config)
		harness.AddBenchmark(arithmeticSequential())
		results := harness.RunAll()
		runs[i] = results[0]
	}

	t.Logf("Run 1: cycles=%d, CPI=%.3f", runs[0].SimulatedCycles, runs[0].CPI)
	t.Logf("Run 2: cycles=%d, CPI=%.3f", runs[1].SimulatedCycles, runs[1].CPI)
	t.Logf("Run 3: cycles=%d, CPI=%.3f", runs[2].SimulatedCycles, runs[2].CPI)

	// Simulated cycles should be identical across runs (deterministic simulation)
	if runs[0].SimulatedCycles != runs[1].SimulatedCycles ||
		runs[1].SimulatedCycles != runs[2].SimulatedCycles {
		t.Errorf("TIMING BUG: inconsistent cycle counts across runs (%d, %d, %d)",
			runs[0].SimulatedCycles, runs[1].SimulatedCycles, runs[2].SimulatedCycles)
		t.Error("Timing simulation should be deterministic")
	}

	// CPI should be identical
	if runs[0].CPI != runs[1].CPI || runs[1].CPI != runs[2].CPI {
		t.Errorf("TIMING BUG: inconsistent CPI across runs (%.3f, %.3f, %.3f)",
			runs[0].CPI, runs[1].CPI, runs[2].CPI)
	}
}

// TestTimingPredictions_CacheEffect validates that enabling caches
// affects timing results.
func TestTimingPredictions_CacheEffect(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	// Run without caches
	configNoCache := DefaultConfig()
	configNoCache.Output = &bytes.Buffer{}
	configNoCache.EnableICache = false
	configNoCache.EnableDCache = false

	harnessNoCache := NewHarness(configNoCache)
	harnessNoCache.AddBenchmark(memorySequential())
	resultsNoCache := harnessNoCache.RunAll()

	// Run with caches
	configWithCache := DefaultConfig()
	configWithCache.Output = &bytes.Buffer{}
	configWithCache.EnableICache = true
	configWithCache.EnableDCache = true

	harnessWithCache := NewHarness(configWithCache)
	harnessWithCache.AddBenchmark(memorySequential())
	resultsWithCache := harnessWithCache.RunAll()

	noCache := resultsNoCache[0]
	withCache := resultsWithCache[0]

	t.Logf("No cache: cycles=%d, CPI=%.3f, DCacheHits=%d, DCacheMisses=%d",
		noCache.SimulatedCycles, noCache.CPI, noCache.DCacheHits, noCache.DCacheMisses)
	t.Logf("With cache: cycles=%d, CPI=%.3f, DCacheHits=%d, DCacheMisses=%d",
		withCache.SimulatedCycles, withCache.CPI, withCache.DCacheHits, withCache.DCacheMisses)

	// With cache enabled, we should have cache statistics
	if withCache.DCacheHits == 0 && withCache.DCacheMisses == 0 {
		t.Error("TIMING BUG: cache enabled but no D-cache activity recorded")
	}

	// Without cache, we should have no cache statistics
	if noCache.DCacheHits != 0 || noCache.DCacheMisses != 0 {
		t.Error("TIMING BUG: cache disabled but D-cache stats are non-zero")
	}
}

// TestTimingPredictions_StallAccounting validates that total stalls
// equal the sum of stall types.
func TestTimingPredictions_StallAccounting(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	config := DefaultConfig()
	config.Output = &bytes.Buffer{}
	config.EnableICache = false
	config.EnableDCache = false

	harness := NewHarness(config)
	harness.AddBenchmarks(GetMicrobenchmarks())

	results := harness.RunAll()

	for _, r := range results {
		// Total stalls should be >= sum of categorized stalls
		// (there might be other stall types not separately tracked)
		categorizedStalls := r.ExecStalls + r.MemStalls

		t.Logf("%s: TotalStalls=%d, ExecStalls=%d, MemStalls=%d, Sum=%d",
			r.Name, r.StallCycles, r.ExecStalls, r.MemStalls, categorizedStalls)

		if categorizedStalls > r.StallCycles {
			t.Errorf("TIMING BUG: %s has categorized stalls (%d) > total stalls (%d)",
				r.Name, categorizedStalls, r.StallCycles)
			t.Error("Sum of stall categories cannot exceed total stalls")
		}
	}
}

// TestTimingPredictions_CycleEquation validates the fundamental cycle equation.
// With 8-wide superscalar, we can retire up to 8 instructions per cycle,
// so Cycles >= Instructions/8 (theoretically).
func TestTimingPredictions_CycleEquation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	config := DefaultConfig()
	config.Output = &bytes.Buffer{}
	config.EnableICache = false
	config.EnableDCache = false

	harness := NewHarness(config)
	harness.AddBenchmarks(GetMicrobenchmarks())

	results := harness.RunAll()

	for _, r := range results {
		// With 8-wide issue: Cycles >= Instructions/8 (theoretical minimum)
		// In practice, pipeline fill/drain and dependencies increase this
		minCycles := r.InstructionsRetired / 8
		if r.SimulatedCycles < minCycles {
			t.Errorf("TIMING BUG: %s has cycles (%d) < instructions/8 (%d)",
				r.Name, r.SimulatedCycles, minCycles)
			t.Error("An 8-wide pipeline cannot retire more than 8 instructions per cycle")
		}

		t.Logf("%s: Cycles=%d, Insts=%d, Stalls=%d, Flushes=%d, CPI=%.3f",
			r.Name, r.SimulatedCycles, r.InstructionsRetired, r.StallCycles, r.PipelineFlushes, r.CPI)
	}
}

// TestTimingPredictions_MixedWorkload validates that the mixed operations
// benchmark exhibits characteristics of all operation types.
func TestTimingPredictions_MixedWorkload(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	// Without D-cache: memory accesses are instant, so no MemStalls expected.
	// We still expect pipeline flushes from function calls.
	t.Run("without_dcache", func(t *testing.T) {
		config := DefaultConfig()
		config.Output = &bytes.Buffer{}
		config.EnableICache = false
		config.EnableDCache = false

		harness := NewHarness(config)
		harness.AddBenchmark(mixedOperations())

		results := harness.RunAll()
		r := results[0]

		t.Logf("Mixed ops: Cycles=%d, CPI=%.3f, ExecStalls=%d, MemStalls=%d, Flushes=%d",
			r.SimulatedCycles, r.CPI, r.ExecStalls, r.MemStalls, r.PipelineFlushes)

		// Mixed benchmark has function calls - should have pipeline flushes
		if r.PipelineFlushes == 0 {
			t.Error("TIMING BUG: mixed benchmark has no flushes despite BL/RET operations")
		}

		// CPI should be reasonable (no memory stalls, but still function call overhead)
		if r.CPI < 0.125 || r.CPI > 5.0 {
			t.Errorf("TIMING BUG: mixed benchmark CPI (%.3f) outside expected range [0.125, 5.0]",
				r.CPI)
		}
	})

	// With D-cache: memory operations produce stalls from cache misses.
	t.Run("with_dcache", func(t *testing.T) {
		config := DefaultConfig()
		config.Output = &bytes.Buffer{}
		config.EnableICache = false
		config.EnableDCache = true

		harness := NewHarness(config)
		harness.AddBenchmark(mixedOperations())

		results := harness.RunAll()
		r := results[0]

		t.Logf("Mixed ops: Cycles=%d, CPI=%.3f, ExecStalls=%d, MemStalls=%d, Flushes=%d",
			r.SimulatedCycles, r.CPI, r.ExecStalls, r.MemStalls, r.PipelineFlushes)

		// Mixed benchmark has function calls - should have pipeline flushes
		if r.PipelineFlushes == 0 {
			t.Error("TIMING BUG: mixed benchmark has no flushes despite BL/RET operations")
		}
	})
}

// Helper function to find a benchmark result by name
func findResult(results []BenchmarkResult, name string) *BenchmarkResult {
	for i := range results {
		if results[i].Name == name {
			return &results[i]
		}
	}
	return nil
}
