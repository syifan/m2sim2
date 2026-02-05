// Package benchmarks provides timing benchmark infrastructure for M2Sim calibration.
package benchmarks

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/sarchlab/m2sim/emu"
	"github.com/sarchlab/m2sim/timing/pipeline"
)

// BenchmarkResult holds the timing results for a single benchmark run.
type BenchmarkResult struct {
	// Name identifies the benchmark
	Name string `json:"name"`

	// Description explains what the benchmark measures
	Description string `json:"description"`

	// SimulatedCycles is the total cycle count from the timing simulator
	SimulatedCycles uint64 `json:"simulated_cycles"`

	// InstructionsRetired is the number of completed instructions
	InstructionsRetired uint64 `json:"instructions_retired"`

	// CPI is cycles per instruction
	CPI float64 `json:"cpi"`

	// StallCycles is the number of stall cycles
	StallCycles uint64 `json:"stall_cycles"`

	// ExecStalls is stalls due to multi-cycle execution
	ExecStalls uint64 `json:"exec_stalls"`

	// MemStalls is stalls due to memory latency
	MemStalls uint64 `json:"mem_stalls"`

	// DataHazards is the number of RAW data hazards detected and resolved via forwarding
	DataHazards uint64 `json:"data_hazards"`

	// PipelineFlushes is the number of pipeline flushes
	PipelineFlushes uint64 `json:"pipeline_flushes"`

	// EliminatedBranches is the count of unconditional branches (B, not BL)
	// that were eliminated at fetch time and never entered the pipeline
	EliminatedBranches uint64 `json:"eliminated_branches"`

	// ICacheHits/Misses (if cache enabled)
	ICacheHits   uint64 `json:"icache_hits,omitempty"`
	ICacheMisses uint64 `json:"icache_misses,omitempty"`

	// DCacheHits/Misses (if cache enabled)
	DCacheHits   uint64 `json:"dcache_hits,omitempty"`
	DCacheMisses uint64 `json:"dcache_misses,omitempty"`

	// Branch predictor stats
	BranchPredictions     uint64  `json:"branch_predictions,omitempty"`
	BranchCorrect         uint64  `json:"branch_correct,omitempty"`
	BranchMispredictions  uint64  `json:"branch_mispredictions,omitempty"`
	BranchAccuracyPercent float64 `json:"branch_accuracy_percent,omitempty"`

	// ExitCode is the program's exit code
	ExitCode int64 `json:"exit_code"`

	// WallTime is the actual time taken to run the simulation
	WallTime time.Duration `json:"wall_time_ns"`
}

// Benchmark defines a single benchmark program.
type Benchmark struct {
	// Name identifies the benchmark
	Name string

	// Description explains what the benchmark measures
	Description string

	// Setup prepares the emulator state (e.g., initialize registers, memory)
	Setup func(regFile *emu.RegFile, memory *emu.Memory)

	// Program is the ARM64 machine code to execute
	Program []byte

	// ExpectedExit is the expected exit code (for validation)
	ExpectedExit int64
}

// HarnessConfig configures the benchmark harness.
type HarnessConfig struct {
	// EnableICache enables instruction cache simulation
	EnableICache bool

	// EnableDCache enables data cache simulation
	EnableDCache bool

	// EnableDualIssue enables superscalar dual-issue execution (2-wide)
	EnableDualIssue bool

	// EnableQuadIssue enables 4-wide superscalar execution (overrides EnableDualIssue)
	EnableQuadIssue bool

	// EnableSextupleIssue enables 6-wide superscalar execution (overrides EnableQuadIssue)
	// This matches the Apple M2's 6 integer ALUs
	EnableSextupleIssue bool

	// Output is where to write results (default: os.Stdout)
	Output io.Writer

	// Verbose enables detailed output
	Verbose bool
}

// DefaultConfig returns a default harness configuration.
func DefaultConfig() HarnessConfig {
	return HarnessConfig{
		EnableICache:        true,
		EnableDCache:        true,
		EnableDualIssue:     false, // Superseded by higher-width options
		EnableQuadIssue:     false, // Superseded by EnableSextupleIssue
		EnableSextupleIssue: true,  // Enable 6-wide superscalar by default (matches M2)
		Output:              os.Stdout,
		Verbose:             false,
	}
}

// Harness runs timing benchmarks and reports results.
type Harness struct {
	config     HarnessConfig
	benchmarks []Benchmark
}

// NewHarness creates a new benchmark harness.
func NewHarness(config HarnessConfig) *Harness {
	if config.Output == nil {
		config.Output = os.Stdout
	}
	return &Harness{
		config:     config,
		benchmarks: []Benchmark{},
	}
}

// AddBenchmark adds a benchmark to the harness.
func (h *Harness) AddBenchmark(b Benchmark) {
	h.benchmarks = append(h.benchmarks, b)
}

// AddBenchmarks adds multiple benchmarks to the harness.
func (h *Harness) AddBenchmarks(benchmarks []Benchmark) {
	h.benchmarks = append(h.benchmarks, benchmarks...)
}

// RunAll executes all benchmarks and returns results.
func (h *Harness) RunAll() []BenchmarkResult {
	results := make([]BenchmarkResult, 0, len(h.benchmarks))

	for _, bench := range h.benchmarks {
		result := h.runBenchmark(bench)
		results = append(results, result)
	}

	return results
}

// runBenchmark executes a single benchmark.
func (h *Harness) runBenchmark(bench Benchmark) BenchmarkResult {
	// Create fresh state
	regFile := &emu.RegFile{}
	memory := emu.NewMemory()

	// Initialize stack pointer to a valid location
	regFile.SP = 0x10000

	// Run setup if provided
	if bench.Setup != nil {
		bench.Setup(regFile, memory)
	}

	// Load program at 0x1000
	programAddr := uint64(0x1000)
	for i, b := range bench.Program {
		memory.Write8(programAddr+uint64(i), b)
	}

	// Create pipeline with options
	opts := []pipeline.PipelineOption{}
	if h.config.EnableICache || h.config.EnableDCache {
		opts = append(opts, pipeline.WithDefaultCaches())
	}
	if h.config.EnableSextupleIssue {
		opts = append(opts, pipeline.WithSextupleIssue())
	} else if h.config.EnableQuadIssue {
		opts = append(opts, pipeline.WithQuadIssue())
	} else if h.config.EnableDualIssue {
		opts = append(opts, pipeline.WithDualIssue())
	}

	pipe := pipeline.NewPipeline(regFile, memory, opts...)
	pipe.SetPC(programAddr)

	// Run simulation and measure time
	start := time.Now()
	exitCode := pipe.Run()
	wallTime := time.Since(start)

	// Collect statistics
	stats := pipe.Stats()
	result := BenchmarkResult{
		Name:                bench.Name,
		Description:         bench.Description,
		SimulatedCycles:     stats.Cycles,
		InstructionsRetired: stats.Instructions,
		CPI:                 stats.CPI(),
		StallCycles:         stats.Stalls,
		ExecStalls:          stats.ExecStalls,
		MemStalls:           stats.MemStalls,
		DataHazards:         stats.DataHazards,
		PipelineFlushes:     stats.Flushes,
		EliminatedBranches:  stats.EliminatedBranches,
		ExitCode:            exitCode,
		WallTime:            wallTime,
	}

	// Collect cache stats if enabled
	if pipe.UseICache() {
		icStats := pipe.ICacheStats()
		result.ICacheHits = icStats.Hits
		result.ICacheMisses = icStats.Misses
	}
	if pipe.UseDCache() {
		dcStats := pipe.DCacheStats()
		result.DCacheHits = dcStats.Hits
		result.DCacheMisses = dcStats.Misses
	}

	// Collect branch predictor stats
	bpStats := pipe.BranchPredictorStats()
	result.BranchPredictions = bpStats.Predictions
	result.BranchCorrect = bpStats.Correct
	result.BranchMispredictions = bpStats.Mispredictions
	result.BranchAccuracyPercent = bpStats.Accuracy()

	return result
}

// PrintResults outputs benchmark results in a human-readable format.
func (h *Harness) PrintResults(results []BenchmarkResult) {
	_, _ = fmt.Fprintln(h.config.Output, "=== M2Sim Timing Benchmark Results ===")
	_, _ = fmt.Fprintln(h.config.Output, "")

	for _, r := range results {
		_, _ = fmt.Fprintf(h.config.Output, "Benchmark: %s\n", r.Name)
		_, _ = fmt.Fprintf(h.config.Output, "  Description: %s\n", r.Description)
		_, _ = fmt.Fprintf(h.config.Output, "  Exit Code: %d\n", r.ExitCode)
		_, _ = fmt.Fprintln(h.config.Output, "  --- Timing ---")
		_, _ = fmt.Fprintf(h.config.Output, "  Simulated Cycles:     %d\n", r.SimulatedCycles)
		_, _ = fmt.Fprintf(h.config.Output, "  Instructions Retired: %d\n", r.InstructionsRetired)
		_, _ = fmt.Fprintf(h.config.Output, "  CPI:                  %.3f\n", r.CPI)
		_, _ = fmt.Fprintf(h.config.Output, "  Stall Cycles:         %d\n", r.StallCycles)
		_, _ = fmt.Fprintf(h.config.Output, "  Exec Stalls:          %d\n", r.ExecStalls)
		_, _ = fmt.Fprintf(h.config.Output, "  Mem Stalls:           %d\n", r.MemStalls)
		_, _ = fmt.Fprintf(h.config.Output, "  Data Hazards:         %d\n", r.DataHazards)
		_, _ = fmt.Fprintf(h.config.Output, "  Pipeline Flushes:     %d\n", r.PipelineFlushes)
		if r.EliminatedBranches > 0 {
			_, _ = fmt.Fprintf(h.config.Output, "  Eliminated Branches:  %d\n", r.EliminatedBranches)
		}

		if r.ICacheHits > 0 || r.ICacheMisses > 0 {
			_, _ = fmt.Fprintln(h.config.Output, "  --- I-Cache ---")
			_, _ = fmt.Fprintf(h.config.Output, "  Hits:   %d\n", r.ICacheHits)
			_, _ = fmt.Fprintf(h.config.Output, "  Misses: %d\n", r.ICacheMisses)
		}

		if r.DCacheHits > 0 || r.DCacheMisses > 0 {
			_, _ = fmt.Fprintln(h.config.Output, "  --- D-Cache ---")
			_, _ = fmt.Fprintf(h.config.Output, "  Hits:   %d\n", r.DCacheHits)
			_, _ = fmt.Fprintf(h.config.Output, "  Misses: %d\n", r.DCacheMisses)
		}

		if r.BranchPredictions > 0 {
			_, _ = fmt.Fprintln(h.config.Output, "  --- Branch Predictor ---")
			_, _ = fmt.Fprintf(h.config.Output, "  Predictions:     %d\n", r.BranchPredictions)
			_, _ = fmt.Fprintf(h.config.Output, "  Correct:         %d\n", r.BranchCorrect)
			_, _ = fmt.Fprintf(h.config.Output, "  Mispredictions:  %d\n", r.BranchMispredictions)
			_, _ = fmt.Fprintf(h.config.Output, "  Accuracy:        %.1f%%\n", r.BranchAccuracyPercent)
		}

		_, _ = fmt.Fprintf(h.config.Output, "  Wall Time: %v\n", r.WallTime)
		_, _ = fmt.Fprintln(h.config.Output, "")
	}
}

// PrintCSV outputs benchmark results in CSV format for easy comparison.
func (h *Harness) PrintCSV(results []BenchmarkResult) {
	_, _ = fmt.Fprintln(h.config.Output,
		"name,cycles,instructions,cpi,stalls,exec_stalls,mem_stalls,data_hazards,flushes,icache_hits,icache_misses,dcache_hits,dcache_misses,exit_code")

	for _, r := range results {
		_, _ = fmt.Fprintf(h.config.Output, "%s,%d,%d,%.3f,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d\n",
			r.Name,
			r.SimulatedCycles,
			r.InstructionsRetired,
			r.CPI,
			r.StallCycles,
			r.ExecStalls,
			r.MemStalls,
			r.DataHazards,
			r.PipelineFlushes,
			r.ICacheHits,
			r.ICacheMisses,
			r.DCacheHits,
			r.DCacheMisses,
			r.ExitCode,
		)
	}
}

// Helper functions for building ARM64 programs

// BuildProgram assembles instruction words into a byte slice.
func BuildProgram(instrs ...uint32) []byte {
	program := make([]byte, 0, len(instrs)*4)
	for _, inst := range instrs {
		buf := make([]byte, 4)
		binary.LittleEndian.PutUint32(buf, inst)
		program = append(program, buf...)
	}
	return program
}

// Instruction encoding helpers (64-bit only for simplicity)

// EncodeADDImm encodes ADD/ADDS immediate: Rd = Rn + imm12
func EncodeADDImm(rd, rn uint8, imm uint16, setFlags bool) uint32 {
	var inst uint32 = 0
	inst |= 1 << 31 // sf = 1 (64-bit)
	inst |= 0 << 30 // op = 0 (ADD)
	if setFlags {
		inst |= 1 << 29 // S = 1 (set flags)
	}
	inst |= 0b100010 << 23 // opc
	inst |= 0 << 22        // sh = 0
	inst |= uint32(imm&0xFFF) << 10
	inst |= uint32(rn&0x1F) << 5
	inst |= uint32(rd & 0x1F)
	return inst
}

// EncodeSUBImm encodes SUB/SUBS immediate: Rd = Rn - imm12
func EncodeSUBImm(rd, rn uint8, imm uint16, setFlags bool) uint32 {
	var inst uint32 = 0
	inst |= 1 << 31 // sf = 1 (64-bit)
	inst |= 1 << 30 // op = 1 (SUB)
	if setFlags {
		inst |= 1 << 29 // S = 1 (set flags)
	}
	inst |= 0b100010 << 23 // opc
	inst |= 0 << 22        // sh = 0
	inst |= uint32(imm&0xFFF) << 10
	inst |= uint32(rn&0x1F) << 5
	inst |= uint32(rd & 0x1F)
	return inst
}

// EncodeCMPImm encodes compare immediate: CMP Xn, #imm
// This is an alias for SUBS XZR, Xn, #imm (sets flags, discards result)
func EncodeCMPImm(rn uint8, imm uint16) uint32 {
	// CMP is SUBS with Rd = XZR (31) and setFlags = true
	return EncodeSUBImm(31, rn, imm, true)
}

// EncodeADDReg encodes ADD/ADDS register: Rd = Rn + Rm
func EncodeADDReg(rd, rn, rm uint8, setFlags bool) uint32 {
	var inst uint32 = 0
	inst |= 1 << 31 // sf = 1 (64-bit)
	inst |= 0 << 30 // op = 0 (ADD)
	if setFlags {
		inst |= 1 << 29 // S = 1 (set flags)
	}
	inst |= 0b01011 << 24
	inst |= 0 << 22 // shift type
	inst |= 0 << 21 // 0
	inst |= uint32(rm&0x1F) << 16
	inst |= 0 << 10 // imm6
	inst |= uint32(rn&0x1F) << 5
	inst |= uint32(rd & 0x1F)
	return inst
}

// EncodeSUBReg encodes SUB/SUBS register: Rd = Rn - Rm
func EncodeSUBReg(rd, rn, rm uint8, setFlags bool) uint32 {
	var inst uint32 = 0
	inst |= 1 << 31 // sf = 1 (64-bit)
	inst |= 1 << 30 // op = 1 (SUB)
	if setFlags {
		inst |= 1 << 29 // S = 1 (set flags)
	}
	inst |= 0b01011 << 24
	inst |= 0 << 22 // shift type
	inst |= 0 << 21 // 0
	inst |= uint32(rm&0x1F) << 16
	inst |= 0 << 10 // imm6
	inst |= uint32(rn&0x1F) << 5
	inst |= uint32(rd & 0x1F)
	return inst
}

// EncodeBCond encodes conditional branch: B.cond offset
func EncodeBCond(offset int32, cond uint8) uint32 {
	var inst uint32 = 0
	inst |= 0b0101010 << 25
	inst |= 0 << 24
	imm19 := uint32(offset/4) & 0x7FFFF
	inst |= imm19 << 5
	inst |= 0 << 4
	inst |= uint32(cond & 0xF)
	return inst
}

// EncodeBL encodes branch with link: BL offset
func EncodeBL(offset int32) uint32 {
	var inst uint32 = 0
	inst |= 0b100101 << 26
	imm26 := uint32(offset/4) & 0x3FFFFFF
	inst |= imm26
	return inst
}

// EncodeRET encodes return: RET (X30)
func EncodeRET() uint32 {
	var inst uint32 = 0
	inst |= 0b1101011 << 25
	inst |= 0 << 24
	inst |= 0 << 23
	inst |= 0b10 << 21
	inst |= 0b11111 << 16
	inst |= 0b0000 << 12
	inst |= 0 << 11
	inst |= 0 << 10
	inst |= uint32(30) << 5 // X30 (LR)
	inst |= 0b00000
	return inst
}

// EncodeSVC encodes syscall: SVC #imm
func EncodeSVC(imm uint16) uint32 {
	var inst uint32 = 0
	inst |= 0b11010100 << 24
	inst |= 0b000 << 21
	inst |= uint32(imm) << 5
	inst |= 0b00001
	return inst
}

// EncodeSTR64 encodes STR (64-bit) with unsigned immediate offset
func EncodeSTR64(rt, rn uint8, imm12 uint16) uint32 {
	var inst uint32 = 0
	inst |= 0b11 << 30  // size = 64-bit
	inst |= 0b111 << 27 // op1
	inst |= 0 << 26     // V = 0
	inst |= 0b01 << 24  // op2
	inst |= 0b00 << 22  // opc = STR
	inst |= uint32(imm12&0xFFF) << 10
	inst |= uint32(rn&0x1F) << 5
	inst |= uint32(rt & 0x1F)
	return inst
}

// EncodeLDR64 encodes LDR (64-bit) with unsigned immediate offset
func EncodeLDR64(rt, rn uint8, imm12 uint16) uint32 {
	var inst uint32 = 0
	inst |= 0b11 << 30  // size = 64-bit
	inst |= 0b111 << 27 // op1
	inst |= 0 << 26     // V = 0
	inst |= 0b01 << 24  // op2
	inst |= 0b01 << 22  // opc = LDR
	inst |= uint32(imm12&0xFFF) << 10
	inst |= uint32(rn&0x1F) << 5
	inst |= uint32(rt & 0x1F)
	return inst
}

// BenchmarkReport is the complete output format for benchmark results.
type BenchmarkReport struct {
	// Metadata about the benchmark run
	Metadata ReportMetadata `json:"metadata"`

	// Results is the list of individual benchmark results
	Results []BenchmarkResult `json:"results"`

	// Summary contains aggregate statistics
	Summary ReportSummary `json:"summary"`
}

// ReportMetadata contains information about the benchmark run.
type ReportMetadata struct {
	// Timestamp when the benchmark was run
	Timestamp string `json:"timestamp"`

	// Version of the simulator
	Version string `json:"version"`

	// Config describes the benchmark configuration
	Config BenchmarkConfig `json:"config"`
}

// BenchmarkConfig describes the harness configuration used.
type BenchmarkConfig struct {
	ICacheEnabled bool `json:"icache_enabled"`
	DCacheEnabled bool `json:"dcache_enabled"`
}

// ReportSummary contains aggregate statistics across all benchmarks.
type ReportSummary struct {
	// TotalBenchmarks is the number of benchmarks run
	TotalBenchmarks int `json:"total_benchmarks"`

	// TotalCycles is the sum of all simulated cycles
	TotalCycles uint64 `json:"total_cycles"`

	// TotalInstructions is the sum of all instructions retired
	TotalInstructions uint64 `json:"total_instructions"`

	// AverageCPI is the average cycles per instruction
	AverageCPI float64 `json:"average_cpi"`

	// TotalWallTime is the total wall clock time for all benchmarks
	TotalWallTime time.Duration `json:"total_wall_time_ns"`
}

// PrintJSON outputs benchmark results in JSON format for automated comparison.
func (h *Harness) PrintJSON(results []BenchmarkResult) error {
	// Calculate summary statistics
	var totalCycles, totalInstructions uint64
	var totalWallTime time.Duration
	for _, r := range results {
		totalCycles += r.SimulatedCycles
		totalInstructions += r.InstructionsRetired
		totalWallTime += r.WallTime
	}

	avgCPI := float64(0)
	if totalInstructions > 0 {
		avgCPI = float64(totalCycles) / float64(totalInstructions)
	}

	report := BenchmarkReport{
		Metadata: ReportMetadata{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Version:   "0.6.0", // M6 milestone
			Config: BenchmarkConfig{
				ICacheEnabled: h.config.EnableICache,
				DCacheEnabled: h.config.EnableDCache,
			},
		},
		Results: results,
		Summary: ReportSummary{
			TotalBenchmarks:   len(results),
			TotalCycles:       totalCycles,
			TotalInstructions: totalInstructions,
			AverageCPI:        avgCPI,
			TotalWallTime:     totalWallTime,
		},
	}

	encoder := json.NewEncoder(h.config.Output)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}
