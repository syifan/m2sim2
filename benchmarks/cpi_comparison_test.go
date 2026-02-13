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

// CPIComparison holds CPI data for a single benchmark across modes.
type CPIComparison struct {
	Name            string  `json:"name"`
	FullPipelineCPI float64 `json:"full_pipeline_cpi"`
	FastTimingCPI   float64 `json:"fast_timing_cpi"`
	Divergence      float64 `json:"divergence_pct"`
}

// runFastTimingBenchmark runs a single benchmark through the fast timing engine.
func runFastTimingBenchmark(bench Benchmark) (cycles uint64, instructions uint64) {
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
	syscallHandler := emu.NewDefaultSyscallHandler(regFile, memory, &bytes.Buffer{}, &bytes.Buffer{})

	ft := pipeline.NewFastTiming(regFile, memory, latencyTable, syscallHandler,
		pipeline.WithMaxInstructions(100000))
	ft.SetPC(programAddr)
	ft.Run()

	stats := ft.Stats()
	return stats.Cycles, stats.Instructions
}

// TestCPIComparison_FastVsFullPipeline compares CPI between fast timing and
// full pipeline modes for all microbenchmarks. This is the primary validation
// that fast timing CPI approximations are reasonable relative to the detailed
// pipeline model.
func TestCPIComparison_FastVsFullPipeline(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	config := DefaultConfig()
	config.Output = &bytes.Buffer{}
	config.EnableICache = false
	config.EnableDCache = false

	benchmarks := GetMicrobenchmarks()

	// Run full pipeline
	harness := NewHarness(config)
	harness.AddBenchmarks(benchmarks)
	fullResults := harness.RunAll()

	comparisons := make([]CPIComparison, 0, len(benchmarks))

	t.Logf("%-30s %12s %12s %12s", "Benchmark", "Full CPI", "Fast CPI", "Divergence")
	t.Logf("%-30s %12s %12s %12s", "---", "---", "---", "---")

	for i, bench := range benchmarks {
		ftCycles, ftInstrs := runFastTimingBenchmark(bench)

		var ftCPI float64
		if ftInstrs > 0 {
			ftCPI = float64(ftCycles) / float64(ftInstrs)
		}

		fullCPI := fullResults[i].CPI

		var divergence float64
		if fullCPI > 0 {
			divergence = (ftCPI - fullCPI) / fullCPI * 100
		}

		t.Logf("%-30s %12.3f %12.3f %11.1f%%", bench.Name, fullCPI, ftCPI, divergence)

		comparisons = append(comparisons, CPIComparison{
			Name:            bench.Name,
			FullPipelineCPI: fullCPI,
			FastTimingCPI:   ftCPI,
			Divergence:      divergence,
		})
	}

	// Write JSON results
	jsonData, err := json.MarshalIndent(comparisons, "", "  ")
	if err == nil {
		outPath := "cpi_comparison_results.json"
		if writeErr := os.WriteFile(outPath, jsonData, 0644); writeErr == nil {
			t.Logf("\nResults written to %s", outPath)
		}
	}

	// Summary statistics
	var totalDivAbs float64
	var maxDiv float64
	for _, c := range comparisons {
		abs := c.Divergence
		if abs < 0 {
			abs = -abs
		}
		totalDivAbs += abs
		if abs > maxDiv {
			maxDiv = abs
		}
	}
	avgDiv := totalDivAbs / float64(len(comparisons))
	t.Logf("\nSummary: avg |divergence| = %.1f%%, max |divergence| = %.1f%%", avgDiv, maxDiv)

	// Print the 3 core calibration benchmarks separately
	coreNames := map[string]bool{
		"arithmetic_sequential":    true,
		"dependency_chain":         true,
		"branch_taken_conditional": true,
	}
	t.Log("\nCore calibration benchmarks (mapped to M2 hardware baselines):")
	for _, c := range comparisons {
		if coreNames[c.Name] {
			t.Logf("  %-30s full=%.3f fast=%.3f divergence=%.1f%%",
				c.Name, c.FullPipelineCPI, c.FastTimingCPI, c.Divergence)
		}
	}
}

// TestCPIComparison_ThreeWay extends the comparison to include M2 hardware
// baselines, providing a three-way view: hardware vs full pipeline vs fast timing.
func TestCPIComparison_ThreeWay(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	// M2 hardware CPI baselines (from calibration_results.json, at 3.5 GHz)
	// CPI = latency_ns * frequency_GHz
	m2Baselines := map[string]float64{
		"arithmetic": 0.0845121752522325 * 3.5,  // 0.296
		"dependency": 0.31083896341552336 * 3.5, // 1.088
		"branch":     0.37242962810330615 * 3.5, // 1.304
	}

	// Map benchmark names to calibration names
	nameMapping := map[string]string{
		"arithmetic_sequential":    "arithmetic",
		"dependency_chain":         "dependency",
		"branch_taken_conditional": "branch",
	}

	config := DefaultConfig()
	config.Output = &bytes.Buffer{}
	config.EnableICache = false
	config.EnableDCache = false

	benchmarks := []Benchmark{
		arithmeticSequential(),
		dependencyChain(),
		branchTakenConditional(),
	}

	harness := NewHarness(config)
	harness.AddBenchmarks(benchmarks)
	fullResults := harness.RunAll()

	t.Log("Three-way CPI comparison: M2 Hardware vs Full Pipeline vs Fast Timing")
	t.Logf("%-15s %10s %10s %10s %12s %12s",
		"Benchmark", "M2 CPI", "Full CPI", "Fast CPI", "Full Err%", "Fast Err%")
	t.Logf("%-15s %10s %10s %10s %12s %12s",
		"---", "---", "---", "---", "---", "---")

	type ThreeWayResult struct {
		Name         string  `json:"name"`
		M2CPI        float64 `json:"m2_cpi"`
		FullCPI      float64 `json:"full_pipeline_cpi"`
		FastCPI      float64 `json:"fast_timing_cpi"`
		FullErrorPct float64 `json:"full_error_pct"`
		FastErrorPct float64 `json:"fast_error_pct"`
	}

	var results []ThreeWayResult

	for i, bench := range benchmarks {
		calibName := nameMapping[bench.Name]
		m2CPI := m2Baselines[calibName]
		fullCPI := fullResults[i].CPI

		ftCycles, ftInstrs := runFastTimingBenchmark(bench)
		var ftCPI float64
		if ftInstrs > 0 {
			ftCPI = float64(ftCycles) / float64(ftInstrs)
		}

		fullErr := calculateError(fullCPI, m2CPI)
		fastErr := calculateError(ftCPI, m2CPI)

		t.Logf("%-15s %10.3f %10.3f %10.3f %11.1f%% %11.1f%%",
			calibName, m2CPI, fullCPI, ftCPI, fullErr, fastErr)

		results = append(results, ThreeWayResult{
			Name:         calibName,
			M2CPI:        m2CPI,
			FullCPI:      fullCPI,
			FastCPI:      ftCPI,
			FullErrorPct: fullErr,
			FastErrorPct: fastErr,
		})
	}

	jsonData, err := json.MarshalIndent(results, "", "  ")
	if err == nil {
		outPath := "cpi_three_way_results.json"
		if writeErr := os.WriteFile(outPath, jsonData, 0644); writeErr == nil {
			t.Logf("\nResults written to %s", outPath)
		}
	}
}
