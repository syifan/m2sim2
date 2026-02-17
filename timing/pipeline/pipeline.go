package pipeline

import (
	"github.com/sarchlab/m2sim/emu"
	"github.com/sarchlab/m2sim/insts"
	"github.com/sarchlab/m2sim/timing/cache"
	"github.com/sarchlab/m2sim/timing/latency"
)

const (
	// minCacheLoadLatency is the minimum execute-stage latency for load
	// instructions when D-cache is enabled. The actual memory timing is
	// handled by the cache in the MEM stage, so we use 1 cycle here to
	// avoid double-counting latency.
	minCacheLoadLatency = 1

	// instrWindowSize is the capacity of the instruction window buffer.
	// A 192-entry window allows the issue logic to look across many loop
	// iterations, finding independent instructions for OoO-style dispatch.
	// Apple M2 has a 330+ entry ROB; 192 entries provides good overlap
	// for compute-heavy and memory-intensive PolyBench kernels.
	instrWindowSize = 192
)

// instrWindowEntry holds a pre-fetched instruction in the instruction window.
type instrWindowEntry struct {
	Valid           bool
	PC              uint64
	InstructionWord uint32
	PredictedTaken  bool
	PredictedTarget uint64
	EarlyResolved   bool
	AfterBranch     bool // instruction was fetched after a predicted-taken branch
}

// Statistics holds pipeline performance statistics.
type Statistics struct {
	// Cycles is the total number of cycles simulated.
	Cycles uint64
	// Instructions is the number of instructions completed (retired).
	Instructions uint64
	// Stalls is the number of stall cycles.
	Stalls uint64
	// Flushes is the number of pipeline flushes (due to branch mispredictions).
	Flushes uint64
	// ExecStalls is the number of stalls due to multi-cycle execution.
	ExecStalls uint64
	// MemStalls is the number of stalls due to memory latency.
	MemStalls uint64
	// DataHazards is the number of RAW data hazards detected.
	DataHazards uint64
	// BranchPredictions is the total number of branch predictions made.
	BranchPredictions uint64
	// BranchCorrect is the number of correct branch predictions.
	BranchCorrect uint64
	// BranchMispredictions is the number of branch mispredictions.
	BranchMispredictions uint64
	// EliminatedBranches is the count of unconditional branches (B, not BL)
	// that were eliminated at fetch time. These branches never enter the
	// pipeline and consume zero cycles, matching Apple M2's behavior where
	// unconditional B instructions never issue to execution units.
	EliminatedBranches uint64
	// FoldedBranches is the count of conditional branches that were folded
	// at fetch time due to high-confidence prediction. These branches are
	// eliminated from the pipeline (zero-cycle execution), similar to how
	// M2 handles predicted-taken branches with BTB hits.
	FoldedBranches uint64

	// --- Stall profiling counters ---

	// RAWHazardStalls counts cycles lost to load-use (RAW) data dependency
	// stalls, where a load result is needed by the immediately following
	// instruction and the pipeline must insert a bubble.
	RAWHazardStalls uint64
	// StructuralHazardStalls counts cycles where an instruction could not
	// co-issue because canIssueWith() rejected it due to port conflicts,
	// RAW hazards between co-issued instructions, or branch serialization.
	// Each count represents one instruction that failed to issue in a cycle
	// where the primary slot was active.
	StructuralHazardStalls uint64
	// BranchMispredictionStalls counts the total pipeline flush penalty
	// cycles attributed to branch mispredictions. Each misprediction
	// flushes IF and ID stages, costing 2 cycles of useful work.
	BranchMispredictionStalls uint64
}

// CPI returns the cycles per instruction.
func (s Statistics) CPI() float64 {
	if s.Instructions == 0 {
		return 0
	}
	return float64(s.Cycles) / float64(s.Instructions)
}

// PipelineOption is a functional option for configuring the Pipeline.
type PipelineOption func(*Pipeline)

// WithSyscallHandler sets a custom syscall handler.
func WithSyscallHandler(handler emu.SyscallHandler) PipelineOption {
	return func(p *Pipeline) {
		p.syscallHandler = handler
	}
}

// WithLatencyTable sets a custom latency table for instruction timing.
// When set, multi-cycle operations will stall the pipeline appropriately.
func WithLatencyTable(table *latency.Table) PipelineOption {
	return func(p *Pipeline) {
		p.latencyTable = table
	}
}

// WithICache enables L1 instruction cache with the given configuration.
func WithICache(config cache.Config) PipelineOption {
	return func(p *Pipeline) {
		backing := cache.NewMemoryBacking(p.memory)
		icache := cache.New(config, backing)
		p.cachedFetchStage = NewCachedFetchStage(icache, p.memory)
		p.useICache = true
	}
}

// WithDCache enables L1 data cache with the given configuration.
func WithDCache(config cache.Config) PipelineOption {
	return func(p *Pipeline) {
		backing := cache.NewMemoryBacking(p.memory)
		dcache := cache.New(config, backing)
		// Share one D-cache across all 5 memory ports (coherent).
		// Each CachedMemoryStage tracks its own pending/stall state.
		p.cachedMemoryStage = NewCachedMemoryStage(dcache, p.memory)
		p.cachedMemoryStage2 = NewCachedMemoryStage(dcache, p.memory)
		p.cachedMemoryStage3 = NewCachedMemoryStage(dcache, p.memory)
		p.cachedMemoryStage4 = NewCachedMemoryStage(dcache, p.memory)
		p.cachedMemoryStage5 = NewCachedMemoryStage(dcache, p.memory)
		p.useDCache = true
	}
}

// WithDefaultCaches enables L1 I-cache and D-cache with default Apple M2 configurations.
func WithDefaultCaches() PipelineOption {
	return func(p *Pipeline) {
		// Initialize I-cache
		backing := cache.NewMemoryBacking(p.memory)
		icache := cache.New(cache.DefaultL1IConfig(), backing)
		p.cachedFetchStage = NewCachedFetchStage(icache, p.memory)
		p.useICache = true

		// Initialize D-cache — single shared cache, 5 port stages (coherent)
		dcache := cache.New(cache.DefaultL1DConfig(), backing)
		p.cachedMemoryStage = NewCachedMemoryStage(dcache, p.memory)
		p.cachedMemoryStage2 = NewCachedMemoryStage(dcache, p.memory)
		p.cachedMemoryStage3 = NewCachedMemoryStage(dcache, p.memory)
		p.cachedMemoryStage4 = NewCachedMemoryStage(dcache, p.memory)
		p.cachedMemoryStage5 = NewCachedMemoryStage(dcache, p.memory)
		p.useDCache = true
	}
}

// WithBranchPredictorConfig sets a custom branch predictor configuration.
// This allows tuning BTB size, BHT size, global history length, etc.
func WithBranchPredictorConfig(config BranchPredictorConfig) PipelineOption {
	return func(p *Pipeline) {
		p.branchPredictor = NewBranchPredictor(config)
	}
}

// RegisterCheckpoint saves architectural register state before a branch
// executes, allowing rollback on misprediction. This enables speculative
// execution of non-store instructions after predicted-taken branches.
type RegisterCheckpoint struct {
	Valid  bool
	Regs   [31]uint64
	SP     uint64
	PSTATE emu.PSTATE
}

// Pipeline implements a 5-stage pipelined CPU model.
// Stages: Fetch (IF) -> Decode (ID) -> Execute (EX) -> Memory (MEM) -> Writeback (WB)
// Supports optional superscalar (dual-issue) execution for independent instructions.
type Pipeline struct {
	// Pipeline registers (primary slot)
	ifid  IFIDRegister
	idex  IDEXRegister
	exmem EXMEMRegister
	memwb MEMWBRegister

	// Pipeline registers (secondary slot for superscalar)
	ifid2  SecondaryIFIDRegister
	idex2  SecondaryIDEXRegister
	exmem2 SecondaryEXMEMRegister
	memwb2 SecondaryMEMWBRegister

	// Pipeline registers (tertiary slot for 4-wide superscalar)
	ifid3  TertiaryIFIDRegister
	idex3  TertiaryIDEXRegister
	exmem3 TertiaryEXMEMRegister
	memwb3 TertiaryMEMWBRegister

	// Pipeline registers (quaternary slot for 4-wide superscalar)
	ifid4  QuaternaryIFIDRegister
	idex4  QuaternaryIDEXRegister
	exmem4 QuaternaryEXMEMRegister
	memwb4 QuaternaryMEMWBRegister

	// Pipeline registers (quinary slot for 6-wide superscalar)
	ifid5  QuinaryIFIDRegister
	idex5  QuinaryIDEXRegister
	exmem5 QuinaryEXMEMRegister
	memwb5 QuinaryMEMWBRegister

	// Pipeline registers (senary slot for 6-wide superscalar)
	ifid6  SenaryIFIDRegister
	idex6  SenaryIDEXRegister
	exmem6 SenaryEXMEMRegister
	memwb6 SenaryMEMWBRegister

	// Pipeline registers (septenary slot for 8-wide superscalar)
	ifid7  SeptenaryIFIDRegister
	idex7  SeptenaryIDEXRegister
	exmem7 SeptenaryEXMEMRegister
	memwb7 SeptenaryMEMWBRegister

	// Pipeline registers (octonary slot for 8-wide superscalar)
	ifid8  OctonaryIFIDRegister
	idex8  OctonaryIDEXRegister
	exmem8 OctonaryEXMEMRegister
	memwb8 OctonaryMEMWBRegister

	// Pipeline stages
	fetchStage     *FetchStage
	decodeStage    *DecodeStage
	executeStage   *ExecuteStage
	memoryStage    *MemoryStage
	writebackStage *WritebackStage

	// Cached pipeline stages (optional)
	cachedFetchStage  *CachedFetchStage
	cachedMemoryStage *CachedMemoryStage
	useICache         bool
	useDCache         bool

	// Hazard detection
	hazardUnit *HazardUnit

	// Branch prediction
	branchPredictor *BranchPredictor

	// Instruction timing
	latencyTable *latency.Table
	exLatency    uint64 // Remaining cycles for execute stage
	exLatency2   uint64 // Remaining cycles for secondary execute slot
	exLatency3   uint64 // Remaining cycles for tertiary execute slot
	exLatency4   uint64 // Remaining cycles for quaternary execute slot
	exLatency5   uint64 // Remaining cycles for quinary execute slot
	exLatency6   uint64 // Remaining cycles for senary execute slot
	exLatency7   uint64 // Remaining cycles for septenary execute slot
	exLatency8   uint64 // Remaining cycles for octonary execute slot

	// Non-cached memory latency tracking (up to 5 memory ports)
	memPending    bool   // True if waiting for memory operation to complete
	memPendingPC  uint64 // PC of pending memory operation
	memPending2   bool
	memPendingPC2 uint64
	memPending3   bool
	memPendingPC3 uint64
	memPending4   bool
	memPendingPC4 uint64
	memPending5   bool
	memPendingPC5 uint64

	// Cached memory stages for secondary through quinary memory ports
	cachedMemoryStage2 *CachedMemoryStage
	cachedMemoryStage3 *CachedMemoryStage
	cachedMemoryStage4 *CachedMemoryStage
	cachedMemoryStage5 *CachedMemoryStage

	// Shared resources
	regFile *emu.RegFile
	memory  *emu.Memory

	// Syscall handling
	syscallHandler emu.SyscallHandler

	// Program counter
	pc uint64

	// Superscalar configuration
	superscalarConfig SuperscalarConfig

	// Pre-allocated scratch instruction for load-use hazard detection.
	// Avoids heap allocation per cycle for the transient decode result.
	hazardScratchInst insts.Instruction

	// Instruction window for OoO-style dispatch in octuple-issue mode.
	// Holds pre-fetched instructions that couldn't issue in previous cycles,
	// allowing the issue logic to see 16+ instructions and find independent
	// ones from different loop iterations.
	instrWindow    [instrWindowSize]instrWindowEntry
	instrWindowLen int

	// Register checkpoint for branch misprediction rollback
	branchCheckpoint RegisterCheckpoint

	// Statistics
	stats Statistics

	// Execution state
	halted   bool
	exitCode int64
}

// NewPipeline creates a new 5-stage pipeline.
func NewPipeline(regFile *emu.RegFile, memory *emu.Memory, opts ...PipelineOption) *Pipeline {
	p := &Pipeline{
		fetchStage:        NewFetchStage(memory),
		decodeStage:       NewDecodeStage(regFile),
		executeStage:      NewExecuteStage(regFile),
		memoryStage:       NewMemoryStage(memory),
		writebackStage:    NewWritebackStage(regFile),
		hazardUnit:        NewHazardUnit(),
		branchPredictor:   NewBranchPredictor(DefaultBranchPredictorConfig()),
		regFile:           regFile,
		memory:            memory,
		halted:            false,
		superscalarConfig: DefaultSuperscalarConfig(),
	}

	// Apply options
	for _, opt := range opts {
		opt(p)
	}

	// Set up default syscall handler if none provided
	if p.syscallHandler == nil {
		p.syscallHandler = emu.NewDefaultSyscallHandler(regFile, memory, nil, nil)
	}

	return p
}

// PC returns the current program counter.
func (p *Pipeline) PC() uint64 {
	return p.pc
}

// SetPC sets the program counter.
func (p *Pipeline) SetPC(pc uint64) {
	p.pc = pc
	p.regFile.PC = pc
}

// GetIFID returns the IF/ID pipeline register.
func (p *Pipeline) GetIFID() *IFIDRegister {
	return &p.ifid
}

// GetIDEX returns the ID/EX pipeline register.
func (p *Pipeline) GetIDEX() *IDEXRegister {
	return &p.idex
}

// GetEXMEM returns the EX/MEM pipeline register.
func (p *Pipeline) GetEXMEM() *EXMEMRegister {
	return &p.exmem
}

// GetMEMWB returns the MEM/WB pipeline register.
func (p *Pipeline) GetMEMWB() *MEMWBRegister {
	return &p.memwb
}

// Stats returns pipeline statistics.
func (p *Pipeline) Stats() Statistics {
	return p.stats
}

// Halted returns true if the pipeline has halted.
func (p *Pipeline) Halted() bool {
	return p.halted
}

// ExitCode returns the exit code if the pipeline has halted.
func (p *Pipeline) ExitCode() int64 {
	return p.exitCode
}

// Run executes the pipeline until it halts.
// Returns the exit code.
func (p *Pipeline) Run() int64 {
	for !p.halted {
		p.Tick()
	}
	return p.exitCode
}

// RunCycles executes the pipeline for the specified number of cycles.
// Returns true if still running, false if halted.
func (p *Pipeline) RunCycles(cycles uint64) bool {
	for i := uint64(0); i < cycles && !p.halted; i++ {
		p.Tick()
	}
	return !p.halted
}

// getExLatency returns the execute-stage latency for an instruction.
// Load instructions always use minCacheLoadLatency (1 cycle) for the address
// calculation in EX. The remaining load-to-use latency comes from the pipeline
// stages (MEM→WB) and the load-use hazard bubble, totaling 3 cycles — matching
// the Apple M2's L1 load-to-use latency. When D-cache is enabled, the actual
// memory access time is handled by the cache in the MEM stage.
func (p *Pipeline) getExLatency(inst *insts.Instruction) uint64 {
	if p.latencyTable == nil {
		return 1
	}
	if p.useDCache && p.latencyTable.IsLoadOp(inst) {
		return minCacheLoadLatency
	}
	return p.latencyTable.GetLatency(inst)
}

// Tick executes one pipeline cycle.
//
// The method models a classic 5-stage in-order pipeline (IF→ID→EX→MEM→WB)
// with optional superscalar support (dual-issue for independent instructions).
//
// Hazard handling:
//   - Data forwarding from EX/MEM and MEM/WB stages to resolve RAW hazards
//   - Load-use stalls when a load result is needed immediately
//   - Branch prediction with 2-bit saturating counters and BTB
//   - Branch misprediction flushes IF and ID stages (2-cycle penalty)
//
// Stages are evaluated in reverse order (WB→MEM→EX→ID→IF) to compute new
// values before latching them into pipeline registers at cycle end.
//
// Branch prediction reduces penalties for correctly predicted branches to zero.
// Unconditional branches are predicted correctly after first encounter (BTB hit).
func (p *Pipeline) Tick() {
	// Don't execute if halted
	if p.halted {
		return
	}

	p.stats.Cycles++

	// Use superscalar tick if multi-issue is enabled
	if p.superscalarConfig.IssueWidth >= 8 {
		p.tickOctupleIssue()
		return
	}
	if p.superscalarConfig.IssueWidth >= 6 {
		p.tickSextupleIssue()
		return
	}
	if p.superscalarConfig.IssueWidth >= 4 {
		p.tickQuadIssue()
		return
	}
	if p.superscalarConfig.IssueWidth >= 2 {
		p.tickSuperscalar()
		return
	}

	// Single-issue tick (original implementation)
	p.tickSingleIssue()
}

// Reset clears all pipeline state.
func (p *Pipeline) Reset() {
	p.ifid.Clear()
	p.idex.Clear()
	p.exmem.Clear()
	p.memwb.Clear()
	p.ifid2.Clear()
	p.idex2.Clear()
	p.exmem2.Clear()
	p.memwb2.Clear()
	p.ifid3.Clear()
	p.idex3.Clear()
	p.exmem3.Clear()
	p.memwb3.Clear()
	p.ifid4.Clear()
	p.idex4.Clear()
	p.exmem4.Clear()
	p.memwb4.Clear()
	p.ifid5.Clear()
	p.idex5.Clear()
	p.exmem5.Clear()
	p.memwb5.Clear()
	p.ifid6.Clear()
	p.idex6.Clear()
	p.exmem6.Clear()
	p.memwb6.Clear()
	p.ifid7.Clear()
	p.idex7.Clear()
	p.exmem7.Clear()
	p.memwb7.Clear()
	p.ifid8.Clear()
	p.idex8.Clear()
	p.exmem8.Clear()
	p.memwb8.Clear()
	p.pc = 0
	p.stats = Statistics{}
	p.halted = false
	p.exLatency = 0
	p.exLatency2 = 0
	p.exLatency3 = 0
	p.exLatency4 = 0
	p.exLatency5 = 0
	p.exLatency6 = 0
	p.exLatency7 = 0
	p.exLatency8 = 0
	p.memPending = false
	p.memPendingPC = 0
	p.memPending2 = false
	p.memPendingPC2 = 0
	p.memPending3 = false
	p.memPendingPC3 = 0
	p.memPending4 = false
	p.memPendingPC4 = 0
	p.memPending5 = false
	p.memPendingPC5 = 0
	if p.cachedMemoryStage2 != nil {
		p.cachedMemoryStage2.Reset()
	}
	if p.cachedMemoryStage3 != nil {
		p.cachedMemoryStage3.Reset()
	}
	if p.cachedMemoryStage4 != nil {
		p.cachedMemoryStage4.Reset()
	}
	if p.cachedMemoryStage5 != nil {
		p.cachedMemoryStage5.Reset()
	}
	if p.branchPredictor != nil {
		p.branchPredictor.Reset()
	}
}

// LatencyTable returns the current latency table, or nil if not set.
func (p *Pipeline) LatencyTable() *latency.Table {
	return p.latencyTable
}

// SetLatencyTable sets the latency table for instruction timing.
func (p *Pipeline) SetLatencyTable(table *latency.Table) {
	p.latencyTable = table
}

// ICacheStats returns I-cache statistics, or empty if I-cache not enabled.
func (p *Pipeline) ICacheStats() cache.Statistics {
	if p.cachedFetchStage != nil {
		return p.cachedFetchStage.CacheStats()
	}
	return cache.Statistics{}
}

// DCacheStats returns D-cache statistics, or empty if D-cache not enabled.
func (p *Pipeline) DCacheStats() cache.Statistics {
	if p.cachedMemoryStage != nil {
		return p.cachedMemoryStage.CacheStats()
	}
	return cache.Statistics{}
}

// UseICache returns true if I-cache is enabled.
func (p *Pipeline) UseICache() bool {
	return p.useICache
}

// UseDCache returns true if D-cache is enabled.
func (p *Pipeline) UseDCache() bool {
	return p.useDCache
}

// BranchPredictorStats returns branch predictor statistics.
func (p *Pipeline) BranchPredictorStats() BranchPredictorStats {
	if p.branchPredictor != nil {
		return p.branchPredictor.Stats()
	}
	return BranchPredictorStats{}
}
