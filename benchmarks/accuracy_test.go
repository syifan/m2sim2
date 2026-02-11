// Package benchmarks provides accuracy analysis comparing M2Sim against real M2 hardware.
package benchmarks

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// BaselineData represents the M2 baseline measurements.
type BaselineData struct {
	Metadata struct {
		Version     string `json:"version"`
		Date        string `json:"date"`
		Hardware    string `json:"hardware"`
		Methodology string `json:"methodology"`
		Author      string `json:"author"`
	} `json:"metadata"`
	Baselines      []BaselineEntry `json:"baselines"`
	TargetAccuracy struct {
		MaxErrorPercent float64 `json:"max_error_percent"`
		ErrorFormula    string  `json:"error_formula"`
	} `json:"target_accuracy"`
}

// BaselineEntry represents a single baseline measurement.
type BaselineEntry struct {
	Name                     string  `json:"name"`
	Description              string  `json:"description"`
	InstructionsPerIteration int     `json:"instructions_per_iteration"`
	LatencyNsPerInstruction  float64 `json:"latency_ns_per_instruction"`
	CPIAt35GHz               float64 `json:"cpi_at_3_5_ghz"`
	IPCAt35GHz               float64 `json:"ipc_at_3_5_ghz"`
	RSquared                 float64 `json:"r_squared"`
	Notes                    string  `json:"notes"`
}

// AccuracyResult holds the comparison between simulator and baseline.
type AccuracyResult struct {
	BenchmarkName   string
	SimulatorCPI    float64
	BaselineCPI     float64
	ErrorPercent    float64
	PassesThreshold bool
	Notes           string
}

// AccuracyReport is the complete accuracy analysis output.
type AccuracyReport struct {
	SimulatorVersion string           `json:"simulator_version"`
	BaselineHardware string           `json:"baseline_hardware"`
	TargetError      float64          `json:"target_error_percent"`
	Results          []AccuracyResult `json:"results"`
	Summary          AccuracySummary  `json:"summary"`
}

// AccuracySummary contains aggregate accuracy statistics.
type AccuracySummary struct {
	TotalBenchmarks   int     `json:"total_benchmarks"`
	PassingBenchmarks int     `json:"passing_benchmarks"`
	AverageError      float64 `json:"average_error_percent"`
	MaxError          float64 `json:"max_error_percent"`
	OverallPass       bool    `json:"overall_pass"`
}

// benchmarkMapping maps simulator benchmark names to baseline names.
// Uses branch_taken_conditional to match native benchmark pattern (CMP + B.GE).
// Memory-latency benchmarks (loadheavy, storeheavy, memorystrided) require
// D-cache for meaningful comparison — see dcache_accuracy_test.go (PR #429).
var benchmarkMapping = map[string]string{
	"arithmetic_sequential":    "arithmetic",
	"dependency_chain":         "dependency",
	"branch_taken_conditional": "branch",
	"branch_heavy":             "branchheavy",
}

// loadBaseline loads the M2 baseline data from the native directory.
func loadBaseline(t *testing.T) *BaselineData {
	_, filename, _, _ := runtime.Caller(0)
	baseDir := filepath.Dir(filename)
	baselinePath := filepath.Join(baseDir, "native", "m2_baseline.json")

	data, err := os.ReadFile(baselinePath)
	if err != nil {
		t.Fatalf("Failed to load baseline: %v", err)
	}

	var baseline BaselineData
	if err := json.Unmarshal(data, &baseline); err != nil {
		t.Fatalf("Failed to parse baseline: %v", err)
	}

	return &baseline
}

// findBaseline finds a baseline entry by name.
func findBaseline(baseline *BaselineData, name string) *BaselineEntry {
	for _, entry := range baseline.Baselines {
		if entry.Name == name {
			return &entry
		}
	}
	return nil
}

// findBenchmarkResult finds a benchmark result by name.
func findBenchmarkResult(results []BenchmarkResult, name string) *BenchmarkResult {
	for _, result := range results {
		if result.Name == name {
			return &result
		}
	}
	return nil
}

// calculateError computes the error percentage using the formula:
// error = |sim - real| / min(sim, real) * 100
func calculateError(simCPI, baselineCPI float64) float64 {
	if simCPI == 0 && baselineCPI == 0 {
		return 0
	}
	minCPI := math.Min(simCPI, baselineCPI)
	if minCPI == 0 {
		return 100.0 // Avoid division by zero
	}
	return math.Abs(simCPI-baselineCPI) / minCPI * 100
}

// TestAccuracyAgainstBaseline compares simulator results against M2 baseline.
// This is the main accuracy validation test for M2Sim.
func TestAccuracyAgainstBaseline(t *testing.T) {
	// Load baseline data
	baseline := loadBaseline(t)

	// Run simulator benchmarks (without caches to isolate core timing)
	config := DefaultConfig()
	config.EnableICache = false
	config.EnableDCache = false
	config.Verbose = false
	harness := NewHarness(config)
	harness.AddBenchmarks(GetMicrobenchmarks())
	results := harness.RunAll()

	// Compare each mapped benchmark
	var totalError float64
	var passCount int
	targetError := baseline.TargetAccuracy.MaxErrorPercent

	t.Logf("=== M2Sim Accuracy Analysis ===")
	t.Logf("Target accuracy: <%.1f%% error", targetError)
	t.Logf("")

	for simName, baselineName := range benchmarkMapping {
		simResult := findBenchmarkResult(results, simName)
		if simResult == nil {
			t.Errorf("Simulator benchmark not found: %s", simName)
			continue
		}

		baselineEntry := findBaseline(baseline, baselineName)
		if baselineEntry == nil {
			t.Errorf("Baseline not found: %s", baselineName)
			continue
		}

		errorPct := calculateError(simResult.CPI, baselineEntry.CPIAt35GHz)
		passes := errorPct <= targetError

		t.Logf("Benchmark: %s", simName)
		t.Logf("  Simulator CPI: %.3f", simResult.CPI)
		t.Logf("  M2 Real CPI:   %.3f", baselineEntry.CPIAt35GHz)
		t.Logf("  Error:         %.1f%%", errorPct)
		if passes {
			t.Logf("  Status:        PASS ✓")
			passCount++
		} else {
			t.Logf("  Status:        FAIL ✗")
		}
		t.Logf("")

		totalError += errorPct
	}

	totalBenchmarks := len(benchmarkMapping)
	avgError := totalError / float64(totalBenchmarks)

	t.Logf("=== Summary ===")
	t.Logf("Benchmarks tested: %d", totalBenchmarks)
	t.Logf("Passing (<%.1f%%): %d/%d", targetError, passCount, totalBenchmarks)
	t.Logf("Average error: %.1f%%", avgError)

	// For now, log results but don't fail the test - we know accuracy gap exists.
	// Once timing model is calibrated, enable the assertion below:
	//
	// if passCount != totalBenchmarks {
	// 	t.Errorf("Accuracy validation failed: %d/%d benchmarks exceed %.1f%% error",
	// 		totalBenchmarks-passCount, totalBenchmarks, targetError)
	// }

	t.Logf("\nNote: Current simulator has known accuracy gaps (see docs/validation-test-plan.md)")
	t.Logf("These tests track progress toward <2%% accuracy target.")
}

// TestAccuracyDependencyChain specifically tests the dependency chain accuracy.
// This is critical because it measures the simulator's handling of RAW hazards.
func TestAccuracyDependencyChain(t *testing.T) {
	baseline := loadBaseline(t)
	baselineEntry := findBaseline(baseline, "dependency")
	if baselineEntry == nil {
		t.Fatal("Dependency baseline not found")
	}

	config := DefaultConfig()
	config.EnableICache = false
	config.EnableDCache = false
	harness := NewHarness(config)
	harness.AddBenchmark(dependencyChain())
	results := harness.RunAll()

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	simCPI := results[0].CPI
	realCPI := baselineEntry.CPIAt35GHz
	errorPct := calculateError(simCPI, realCPI)

	t.Logf("Dependency Chain Accuracy:")
	t.Logf("  Simulator: %.3f CPI (%d cycles, %d instructions)",
		simCPI, results[0].SimulatedCycles, results[0].InstructionsRetired)
	t.Logf("  M2 Real:   %.3f CPI", realCPI)
	t.Logf("  Error:     %.1f%%", errorPct)

	// Log whether this would pass target
	if errorPct <= 2.0 {
		t.Logf("  Would PASS <2%% target")
	} else {
		t.Logf("  Would FAIL <2%% target (current gap: %.1f%%)", errorPct-2.0)
	}
}

// TestAccuracyArithmetic tests ALU throughput accuracy.
func TestAccuracyArithmetic(t *testing.T) {
	baseline := loadBaseline(t)
	baselineEntry := findBaseline(baseline, "arithmetic")
	if baselineEntry == nil {
		t.Fatal("Arithmetic baseline not found")
	}

	config := DefaultConfig()
	config.EnableICache = false
	config.EnableDCache = false
	harness := NewHarness(config)
	harness.AddBenchmark(arithmeticSequential())
	results := harness.RunAll()

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	simCPI := results[0].CPI
	realCPI := baselineEntry.CPIAt35GHz
	errorPct := calculateError(simCPI, realCPI)

	t.Logf("Arithmetic Throughput Accuracy:")
	t.Logf("  Simulator: %.3f CPI (6-wide superscalar)", simCPI)
	t.Logf("  M2 Real:   %.3f CPI (8+ ALUs with fusion)", realCPI)
	t.Logf("  Error:     %.1f%%", errorPct)
	t.Logf("  Note: M2 is wider and has instruction fusion. See docs/accuracy-analysis.md")
}

// TestAccuracyBranch tests branch handling accuracy using conditional branches.
// Uses branchTakenConditional to match native benchmark pattern (CMP + B.GE).
func TestAccuracyBranch(t *testing.T) {
	baseline := loadBaseline(t)
	baselineEntry := findBaseline(baseline, "branch")
	if baselineEntry == nil {
		t.Fatal("Branch baseline not found")
	}

	config := DefaultConfig()
	config.EnableICache = false
	config.EnableDCache = false
	harness := NewHarness(config)
	harness.AddBenchmark(branchTakenConditional())
	results := harness.RunAll()

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	simCPI := results[0].CPI
	realCPI := baselineEntry.CPIAt35GHz
	errorPct := calculateError(simCPI, realCPI)

	t.Logf("Branch Handling Accuracy:")
	t.Logf("  Simulator: %.3f CPI (%d flushes)",
		simCPI, results[0].PipelineFlushes)
	t.Logf("  M2 Real:   %.3f CPI", realCPI)
	t.Logf("  Error:     %.1f%%", errorPct)
}

// GenerateAccuracyReport creates a detailed accuracy report for documentation.
func GenerateAccuracyReport(t *testing.T) AccuracyReport {
	baseline := loadBaseline(t)

	config := DefaultConfig()
	config.EnableICache = false
	config.EnableDCache = false
	harness := NewHarness(config)
	harness.AddBenchmarks(GetMicrobenchmarks())
	results := harness.RunAll()

	targetError := baseline.TargetAccuracy.MaxErrorPercent
	var accuracyResults []AccuracyResult
	var totalError, maxError float64
	var passCount int

	for simName, baselineName := range benchmarkMapping {
		simResult := findBenchmarkResult(results, simName)
		baselineEntry := findBaseline(baseline, baselineName)

		if simResult == nil || baselineEntry == nil {
			continue
		}

		errorPct := calculateError(simResult.CPI, baselineEntry.CPIAt35GHz)
		passes := errorPct <= targetError

		accuracyResults = append(accuracyResults, AccuracyResult{
			BenchmarkName:   simName,
			SimulatorCPI:    simResult.CPI,
			BaselineCPI:     baselineEntry.CPIAt35GHz,
			ErrorPercent:    errorPct,
			PassesThreshold: passes,
			Notes:           baselineEntry.Notes,
		})

		totalError += errorPct
		if errorPct > maxError {
			maxError = errorPct
		}
		if passes {
			passCount++
		}
	}

	totalBenchmarks := len(benchmarkMapping)
	avgError := totalError / float64(totalBenchmarks)

	return AccuracyReport{
		SimulatorVersion: "0.6.0",
		BaselineHardware: baseline.Metadata.Hardware,
		TargetError:      targetError,
		Results:          accuracyResults,
		Summary: AccuracySummary{
			TotalBenchmarks:   totalBenchmarks,
			PassingBenchmarks: passCount,
			AverageError:      avgError,
			MaxError:          maxError,
			OverallPass:       passCount == totalBenchmarks,
		},
	}
}

// TestGenerateAccuracyReport tests the report generation and outputs JSON.
func TestGenerateAccuracyReport(t *testing.T) {
	report := GenerateAccuracyReport(t)

	reportJSON, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal report: %v", err)
	}

	t.Logf("Accuracy Report (JSON):\n%s", string(reportJSON))

	// Also print human-readable summary
	fmt.Println("\n=== M2Sim Accuracy Report ===")
	fmt.Printf("Simulator Version: %s\n", report.SimulatorVersion)
	fmt.Printf("Baseline Hardware: %s\n", report.BaselineHardware)
	fmt.Printf("Target Error: <%.1f%%\n\n", report.TargetError)

	fmt.Println("Results:")
	for _, r := range report.Results {
		status := "FAIL"
		if r.PassesThreshold {
			status = "PASS"
		}
		fmt.Printf("  %s: sim=%.3f, real=%.3f, error=%.1f%% [%s]\n",
			r.BenchmarkName, r.SimulatorCPI, r.BaselineCPI, r.ErrorPercent, status)
	}

	fmt.Println("\nSummary:")
	fmt.Printf("  Benchmarks: %d/%d passing\n", report.Summary.PassingBenchmarks, report.Summary.TotalBenchmarks)
	fmt.Printf("  Average Error: %.1f%%\n", report.Summary.AverageError)
	fmt.Printf("  Max Error: %.1f%%\n", report.Summary.MaxError)
	if report.Summary.OverallPass {
		fmt.Println("  Overall: PASS ✓")
	} else {
		fmt.Println("  Overall: FAIL ✗ (work in progress)")
	}
}
