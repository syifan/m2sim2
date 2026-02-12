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
)

// isUnconditionalBranch checks if an instruction word is an unconditional branch (B or BL).
// Returns true and the target PC if it is, false otherwise.
func isUnconditionalBranch(word uint32, pc uint64) (bool, uint64) {
	// B instruction: bits [31:26] = 000101
	// BL instruction: bits [31:26] = 100101
	opcode := (word >> 26) & 0x3F
	if opcode == 0b000101 || opcode == 0b100101 {
		// Extract signed 26-bit immediate (offset in words)
		imm26 := int64(word & 0x3FFFFFF)
		// Sign extend the 26-bit immediate
		if (imm26 & 0x2000000) != 0 {
			// Negative offset: sign extend from bit 25
			imm26 |= ^int64(0x3FFFFFF)
		}
		// Multiply by 4 to get byte offset
		target := uint64(int64(pc) + imm26*4)
		return true, target
	}
	return false, 0
}

// isEliminableBranch checks if an instruction word is an unconditional B (not BL).
// Returns true if the branch can be eliminated (doesn't write to a register).
// According to Dougall Johnson's Firestorm documentation, unconditional B
// instructions never issue to execution units on Apple M2.
func isEliminableBranch(word uint32) bool {
	// B instruction: bits [31:26] = 000101 (bit 31 = 0)
	// BL instruction: bits [31:26] = 100101 (bit 31 = 1)
	// Only pure B can be eliminated; BL writes to X30 (link register)
	opcode := (word >> 26) & 0x3F
	return opcode == 0b000101 // Only pure B, not BL
}

// isConditionalBranch checks if an instruction word is a conditional branch (B.cond).
// Returns true and the target PC if it is, false otherwise.
func isConditionalBranch(word uint32, pc uint64) (bool, uint64) {
	// B.cond instruction: bits [31:25] = 0101010, bit 24 = 0, bits [4] = 0
	// Encoding: 01010100 imm19 0 cond
	if (word >> 24) != 0x54 {
		return false, 0
	}
	// Check bit 4 must be 0
	if (word & 0x10) != 0 {
		return false, 0
	}
	// Extract signed 19-bit immediate (offset in words)
	imm19 := int64((word >> 5) & 0x7FFFF)
	// Sign extend the 19-bit immediate
	if (imm19 & 0x40000) != 0 {
		imm19 |= ^int64(0x7FFFF)
	}
	// Multiply by 4 to get byte offset
	target := uint64(int64(pc) + imm19*4)
	return true, target
}

// isCompareAndBranch checks if an instruction word is CBZ or CBNZ.
// Returns true and the target PC if it is, false otherwise.
func isCompareAndBranch(word uint32, pc uint64) (bool, uint64) {
	// CBZ: sf 011010 0 imm19 Rt
	// CBNZ: sf 011010 1 imm19 Rt
	// bits [30:25] = 011010
	if ((word >> 25) & 0x3F) != 0b011010 {
		return false, 0
	}
	// Extract signed 19-bit immediate
	imm19 := int64((word >> 5) & 0x7FFFF)
	// Sign extend
	if (imm19 & 0x40000) != 0 {
		imm19 |= ^int64(0x7FFFF)
	}
	target := uint64(int64(pc) + imm19*4)
	return true, target
}

// isTestAndBranch checks if an instruction word is TBZ or TBNZ.
// Returns true and the target PC if it is, false otherwise.
func isTestAndBranch(word uint32, pc uint64) (bool, uint64) {
	// TBZ: b5 011011 0 b40 imm14 Rt
	// TBNZ: b5 011011 1 b40 imm14 Rt
	// bits [30:25] = 011011
	if ((word >> 25) & 0x3F) != 0b011011 {
		return false, 0
	}
	// Extract signed 14-bit immediate
	imm14 := int64((word >> 5) & 0x3FFF)
	// Sign extend
	if (imm14 & 0x2000) != 0 {
		imm14 |= ^int64(0x3FFF)
	}
	target := uint64(int64(pc) + imm14*4)
	return true, target
}

// isFoldableConditionalBranch checks if an instruction can be folded (eliminated)
// at fetch time. Returns true with the target if the branch can be folded.
// A branch is foldable if it's a conditional branch type (B.cond, CBZ, CBNZ, TBZ, TBNZ).
// The actual folding decision also requires BTB hit and high-confidence prediction.
func isFoldableConditionalBranch(word uint32, pc uint64) (bool, uint64) {
	// Check B.cond
	if isCond, target := isConditionalBranch(word, pc); isCond {
		return true, target
	}
	// Check CBZ/CBNZ
	if isCB, target := isCompareAndBranch(word, pc); isCB {
		return true, target
	}
	// Check TBZ/TBNZ
	if isTB, target := isTestAndBranch(word, pc); isTB {
		return true, target
	}
	return false, 0
}

// enrichPredictionWithEncodedTarget fills in the branch target for conditional
// branches when the predictor says "taken" but BTB doesn't have the target.
// Real hardware (including M2) can extract the target from the instruction
// encoding during fetch, avoiding a full misprediction penalty for taken
// conditional branches with BTB misses.
func enrichPredictionWithEncodedTarget(pred *Prediction, word uint32, pc uint64) {
	if pred.Taken && !pred.TargetKnown {
		if isCond, target := isFoldableConditionalBranch(word, pc); isCond {
			pred.Target = target
			pred.TargetKnown = true
		}
	}
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
		// Share one D-cache across all 3 memory ports (coherent).
		// Each CachedMemoryStage tracks its own pending/stall state.
		p.cachedMemoryStage = NewCachedMemoryStage(dcache, p.memory)
		p.cachedMemoryStage2 = NewCachedMemoryStage(dcache, p.memory)
		p.cachedMemoryStage3 = NewCachedMemoryStage(dcache, p.memory)
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

		// Initialize D-cache — single shared cache, 3 port stages (coherent)
		dcache := cache.New(cache.DefaultL1DConfig(), backing)
		p.cachedMemoryStage = NewCachedMemoryStage(dcache, p.memory)
		p.cachedMemoryStage2 = NewCachedMemoryStage(dcache, p.memory)
		p.cachedMemoryStage3 = NewCachedMemoryStage(dcache, p.memory)
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

	// Non-cached memory latency tracking (up to 3 memory ports)
	memPending    bool   // True if waiting for memory operation to complete
	memPendingPC  uint64 // PC of pending memory operation
	memPending2   bool
	memPendingPC2 uint64
	memPending3   bool
	memPendingPC3 uint64

	// Cached memory stages for secondary/tertiary memory ports
	cachedMemoryStage2 *CachedMemoryStage
	cachedMemoryStage3 *CachedMemoryStage

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

// accessSecondaryMem processes a memory operation for secondary slot (slot 2).
// Returns the memory result and whether a stall occurred.
func (p *Pipeline) accessSecondaryMem(slot MemorySlot) (MemoryResult, bool) {
	if !slot.IsValid() || (!slot.GetMemRead() && !slot.GetMemWrite()) {
		p.memPending2 = false
		return MemoryResult{}, false
	}
	if p.useDCache && p.cachedMemoryStage2 != nil {
		result, stall := p.cachedMemoryStage2.AccessSlot(slot)
		return result, stall
	}
	// Non-cached path: immediate access (no stall).
	// Without cache simulation, memory is a direct array lookup.
	// Pipeline issue rules already enforce ordering constraints.
	p.memPending2 = false
	return p.memoryStage.MemorySlot(slot), false
}

// accessTertiaryMem processes a memory operation for tertiary slot (slot 3).
func (p *Pipeline) accessTertiaryMem(slot MemorySlot) (MemoryResult, bool) {
	if !slot.IsValid() || (!slot.GetMemRead() && !slot.GetMemWrite()) {
		p.memPending3 = false
		return MemoryResult{}, false
	}
	if p.useDCache && p.cachedMemoryStage3 != nil {
		result, stall := p.cachedMemoryStage3.AccessSlot(slot)
		return result, stall
	}
	// Non-cached path: immediate access (no stall).
	// Without cache simulation, memory is a direct array lookup.
	// Pipeline issue rules already enforce ordering constraints.
	p.memPending3 = false
	return p.memoryStage.MemorySlot(slot), false
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

// tickSingleIssue is the original single-issue pipeline tick.
func (p *Pipeline) tickSingleIssue() {
	// Detect hazards before executing stages
	forwarding := p.hazardUnit.DetectForwarding(&p.idex, &p.exmem, &p.memwb)

	// Track data hazards (RAW hazards resolved by forwarding)
	if forwarding.ForwardRn != ForwardNone || forwarding.ForwardRm != ForwardNone || forwarding.ForwardRd != ForwardNone {
		p.stats.DataHazards++
	}

	// Detect load-use hazards between EX stage (ID/EX) and ID stage (IF/ID)
	// Load-use hazards require a stall because the loaded value isn't available
	// until after the MEM stage, so it can't be forwarded in time for EX.
	// ALU-to-ALU dependencies are handled by forwarding (no stall needed).
	loadUseHazard := false
	if p.idex.Valid && p.idex.MemRead && p.idex.Rd != 31 && p.ifid.Valid {
		// Peek at the next instruction to check for load-use hazard
		p.decodeStage.decoder.DecodeInto(p.ifid.InstructionWord, &p.hazardScratchInst)
		nextInst := &p.hazardScratchInst
		if nextInst.Op != insts.OpUnknown {
			usesRn := true                                 // Most instructions use Rn
			usesRm := nextInst.Format == insts.FormatDPReg // Only register format uses Rm

			// For store instructions, the store data comes from Rd (Rt in AArch64),
			// which can be the destination of a preceding load. Treat Rd as a
			// source register for load-use hazard detection.
			sourceRm := nextInst.Rm
			switch nextInst.Op {
			case insts.OpSTR, insts.OpSTRQ:
				usesRm = true
				sourceRm = nextInst.Rd
			}

			loadUseHazard = p.hazardUnit.DetectLoadUseHazardDecoded(
				p.idex.Rd,
				nextInst.Rn,
				sourceRm,
				usesRn,
				usesRm,
			)
			// Note: stall cycles for load-use hazards are counted in the fetch
			// stage when the pipeline is actually stalled (see StallIF handling),
			// so we do not increment p.stats.Stalls here to avoid double-counting.
		}
	}

	// Branch prediction tracking
	branchMispredicted := false
	var branchTarget uint64

	// Stage 5: Writeback (using WritebackSlot helper)
	savedMEMWB := p.memwb
	if p.writebackStage.WritebackSlot(&p.memwb) {
		p.stats.Instructions++
	}

	// Stage 4: Memory
	var nextMEMWB MEMWBRegister
	memStall := false
	if p.exmem.Valid {
		if p.exmem.Inst != nil && p.exmem.Inst.Op == insts.OpSVC {
			if p.syscallHandler != nil {
				result := p.syscallHandler.Handle()
				if result.Exited {
					p.halted = true
					p.exitCode = result.ExitCode
				}
			}
		}

		var memResult MemoryResult
		if p.useDCache && p.cachedMemoryStage != nil {
			memResult, memStall = p.cachedMemoryStage.Access(&p.exmem)
			if memStall {
				p.stats.MemStalls++
			}
		} else {
			if p.exmem.MemRead || p.exmem.MemWrite {
				if p.memPending && p.memPendingPC != p.exmem.PC {
					p.memPending = false
				}
				if !p.memPending {
					p.memPending = true
					p.memPendingPC = p.exmem.PC
					memStall = true
					p.stats.MemStalls++
				} else {
					p.memPending = false
					memResult = p.memoryStage.Access(&p.exmem)
				}
			} else {
				p.memPending = false
			}
		}

		if !memStall {
			nextMEMWB = MEMWBRegister{
				Valid:     true,
				PC:        p.exmem.PC,
				Inst:      p.exmem.Inst,
				ALUResult: p.exmem.ALUResult,
				MemData:   memResult.MemData,
				Rd:        p.exmem.Rd,
				RegWrite:  p.exmem.RegWrite,
				MemToReg:  p.exmem.MemToReg,
				IsFused:   p.exmem.IsFused,
			}
		}
	}

	// Stage 3: Execute
	var nextEXMEM EXMEMRegister
	execStall := false
	if p.idex.Valid && !memStall {
		if p.latencyTable != nil && p.exLatency == 0 {
			if p.useDCache && p.latencyTable.IsLoadOp(p.idex.Inst) {
				p.exLatency = minCacheLoadLatency
			} else {
				p.exLatency = p.latencyTable.GetLatency(p.idex.Inst)
			}
		}

		if p.exLatency > 0 {
			p.exLatency--
		}

		if p.exLatency > 0 {
			execStall = true
			p.stats.ExecStalls++
		} else {
			rnValue := p.hazardUnit.GetForwardedValue(
				forwarding.ForwardRn, p.idex.RnValue, &p.exmem, &savedMEMWB)
			rmValue := p.hazardUnit.GetForwardedValue(
				forwarding.ForwardRm, p.idex.RmValue, &p.exmem, &savedMEMWB)

			// Check for PSTATE flag forwarding from EXMEM stage.
			// This fixes the timing hazard where CMP sets PSTATE at cycle END
			// but B.cond reads at cycle START, causing stale flag reads.
			forwardFlags := false
			var fwdN, fwdZ, fwdC, fwdV bool
			if p.idex.Inst != nil && p.idex.Inst.Op == insts.OpBCond && !p.idex.IsFused {
				// Check if previous instruction (now in EXMEM) sets flags
				if p.exmem.Valid && p.exmem.SetsFlags {
					forwardFlags = true
					fwdN = p.exmem.FlagN
					fwdZ = p.exmem.FlagZ
					fwdC = p.exmem.FlagC
					fwdV = p.exmem.FlagV
				}
			}

			execResult := p.executeStage.ExecuteWithFlags(&p.idex, rnValue, rmValue,
				forwardFlags, fwdN, fwdZ, fwdC, fwdV)

			// Handle branch prediction verification
			if p.idex.IsBranch {
				actualTaken := execResult.BranchTaken
				actualTarget := execResult.BranchTarget

				p.stats.BranchPredictions++

				// Use the prediction info that was captured at fetch time (stored in IDEX).
				// This correctly reflects what PC was used for the next fetch.
				predictedTaken := p.idex.PredictedTaken
				predictedTarget := p.idex.PredictedTarget
				earlyResolved := p.idex.EarlyResolved

				// Determine if misprediction occurred
				wasMispredicted := false
				if actualTaken {
					if !predictedTaken {
						// Predicted not taken, but was taken
						wasMispredicted = true
					} else if predictedTarget != actualTarget {
						// Predicted taken but to wrong target
						wasMispredicted = true
					}
					// Note: If earlyResolved is true and we reach here, the prediction
					// was correct (unconditional branch correctly resolved at fetch).
				} else {
					if predictedTaken {
						// Predicted taken, but was not taken
						wasMispredicted = true
					}
				}

				// For early-resolved unconditional branches, we should always be correct
				// (they are always taken and we computed the exact target at fetch).
				if earlyResolved && actualTaken {
					wasMispredicted = false // Double-check: early resolution is always correct
				}

				// Update predictor with actual outcome (for BTB training)
				p.branchPredictor.Update(p.idex.PC, actualTaken, actualTarget)

				if wasMispredicted {
					p.stats.BranchMispredictions++
					branchMispredicted = true
					branchTarget = actualTarget
					if !actualTaken {
						branchTarget = p.idex.PC + 4 // Continue to next instruction
					}
				} else {
					p.stats.BranchCorrect++
					// Correct prediction - no flush needed!
				}
			}

			storeValue := execResult.StoreValue
			if p.idex.MemWrite {
				rdValue := p.regFile.ReadReg(p.idex.Rd)
				storeValue = p.hazardUnit.GetForwardedValue(
					forwarding.ForwardRd, rdValue, &p.exmem, &savedMEMWB)
			}

			nextEXMEM = EXMEMRegister{
				Valid:      true,
				PC:         p.idex.PC,
				Inst:       p.idex.Inst,
				ALUResult:  execResult.ALUResult,
				StoreValue: storeValue,
				Rd:         p.idex.Rd,
				MemRead:    p.idex.MemRead,
				MemWrite:   p.idex.MemWrite,
				RegWrite:   p.idex.RegWrite,
				MemToReg:   p.idex.MemToReg,
				// Store computed flags for forwarding to dependent B.cond
				SetsFlags: execResult.SetsFlags,
				FlagN:     execResult.FlagN,
				FlagZ:     execResult.FlagZ,
				FlagC:     execResult.FlagC,
				FlagV:     execResult.FlagV,
			}
		}
	}

	// Compute stall signals
	// Memory stalls should also stall upstream stages
	// Note: Only load-use hazards require stalls. ALU-to-ALU dependencies
	// are resolved through forwarding without stalling the pipeline.
	// Branch mispredictions cause flushes, correct predictions don't.
	stallResult := p.hazardUnit.ComputeStalls(loadUseHazard || execStall || memStall, branchMispredicted)

	// Stage 1: Fetch (need to process fetch first to check for fetch stalls)
	var nextIFID IFIDRegister
	fetchStall := false
	if !stallResult.StallIF && !stallResult.FlushIF && !memStall {
		var word uint32
		var ok bool

		if p.useICache && p.cachedFetchStage != nil {
			word, ok, fetchStall = p.cachedFetchStage.Fetch(p.pc)
			if fetchStall {
				p.stats.Stalls++
			}
		} else {
			word, ok = p.fetchStage.Fetch(p.pc)
		}

		if ok && !fetchStall {
			// Branch elimination: unconditional B (not BL) instructions are
			// eliminated at fetch time. They never enter the pipeline, matching
			// Apple M2's behavior where B instructions never issue.
			if isEliminableBranch(word) {
				_, uncondTarget := isUnconditionalBranch(word, p.pc)
				p.pc = uncondTarget
				p.stats.EliminatedBranches++
				// Don't create IFID entry - branch is eliminated
				// nextIFID remains empty (Valid=false)
			} else {
				// Early branch resolution: detect unconditional branches (B, BL) and
				// resolve them immediately without waiting for BTB. This eliminates
				// misprediction penalties for unconditional branches.
				isUncondBranch, uncondTarget := isUnconditionalBranch(word, p.pc)

				// Use branch predictor for conditional branches
				pred := p.branchPredictor.Predict(p.pc)

				// For unconditional branches, override prediction with actual target
				earlyResolved := false
				if isUncondBranch {
					pred.Taken = true
					pred.Target = uncondTarget
					pred.TargetKnown = true
					earlyResolved = true
				}
				enrichPredictionWithEncodedTarget(&pred, word, p.pc)

				nextIFID = IFIDRegister{
					Valid:           true,
					PC:              p.pc,
					InstructionWord: word,
					PredictedTaken:  pred.Taken,
					PredictedTarget: pred.Target,
					EarlyResolved:   earlyResolved,
				}

				// Speculative fetch: redirect PC based on prediction/resolution
				if pred.Taken && pred.TargetKnown {
					p.pc = pred.Target
				} else {
					p.pc += 4 // Default: sequential fetch
				}
			}
		} else if fetchStall {
			nextIFID = p.ifid
		}
	} else if (stallResult.StallIF || memStall) && !stallResult.FlushIF {
		nextIFID = p.ifid
		p.stats.Stalls++
	}

	// Stage 2: Decode
	// Note: When fetch stalls, we must NOT decode because ifid is preserved
	// for next cycle. If we decode now, the instruction would be executed twice.
	var nextIDEX IDEXRegister
	if p.ifid.Valid && !stallResult.StallID && !stallResult.FlushID && !execStall && !fetchStall {
		decResult := p.decodeStage.Decode(p.ifid.InstructionWord, p.ifid.PC)
		nextIDEX = IDEXRegister{
			Valid:           true,
			PC:              p.ifid.PC,
			Inst:            decResult.Inst,
			RnValue:         decResult.RnValue,
			RmValue:         decResult.RmValue,
			Rd:              decResult.Rd,
			Rn:              decResult.Rn,
			Rm:              decResult.Rm,
			MemRead:         decResult.MemRead,
			MemWrite:        decResult.MemWrite,
			RegWrite:        decResult.RegWrite,
			MemToReg:        decResult.MemToReg,
			IsBranch:        decResult.IsBranch,
			PredictedTaken:  p.ifid.PredictedTaken,
			PredictedTarget: p.ifid.PredictedTarget,
			EarlyResolved:   p.ifid.EarlyResolved,
		}
	} else if (stallResult.StallID || execStall || memStall || fetchStall) && !stallResult.FlushID {
		nextIDEX = p.idex
	}

	// Handle branch misprediction: update PC and flush pipeline
	// Note: Only mispredictions cause flushes. Correct predictions don't need flushing.
	if branchMispredicted {
		p.pc = branchTarget
		nextIFID.Clear()
		nextIDEX.Clear()
		p.stats.Flushes++
	}

	if !memStall && !fetchStall {
		p.memwb = nextMEMWB
	} else {
		p.memwb.Clear()
	}
	if !execStall && !memStall && !fetchStall {
		p.exmem = nextEXMEM
	}
	if stallResult.InsertBubbleEX && !execStall && !memStall && !fetchStall {
		p.idex.Clear()
	} else if !memStall && !fetchStall {
		p.idex = nextIDEX
	}
	if !fetchStall {
		p.ifid = nextIFID
	}
}

// tickSuperscalar executes one cycle with dual-issue support.
// Independent instructions are executed in parallel when possible.
func (p *Pipeline) tickSuperscalar() {
	// Stage 5: Writeback (both slots using WritebackSlot helper)
	savedMEMWB := p.memwb
	if p.writebackStage.WritebackSlot(&p.memwb) {
		p.stats.Instructions++
	}
	// Writeback secondary slot
	if p.writebackStage.WritebackSlot(&p.memwb2) {
		p.stats.Instructions++
	}

	// Stage 4: Memory (primary slot only - single memory port)
	var nextMEMWB MEMWBRegister
	var nextMEMWB2 SecondaryMEMWBRegister
	memStall := false

	if p.exmem.Valid {
		if p.exmem.Inst != nil && p.exmem.Inst.Op == insts.OpSVC {
			if p.syscallHandler != nil {
				result := p.syscallHandler.Handle()
				if result.Exited {
					p.halted = true
					p.exitCode = result.ExitCode
				}
			}
		}

		var memResult MemoryResult
		if p.useDCache && p.cachedMemoryStage != nil {
			memResult, memStall = p.cachedMemoryStage.Access(&p.exmem)
			if memStall {
				p.stats.MemStalls++
			}
		} else {
			if p.exmem.MemRead || p.exmem.MemWrite {
				if p.memPending && p.memPendingPC != p.exmem.PC {
					p.memPending = false
				}
				if !p.memPending {
					p.memPending = true
					p.memPendingPC = p.exmem.PC
					memStall = true
					p.stats.MemStalls++
				} else {
					p.memPending = false
					memResult = p.memoryStage.Access(&p.exmem)
				}
			} else {
				p.memPending = false
			}
		}

		if !memStall {
			nextMEMWB = MEMWBRegister{
				Valid:     true,
				PC:        p.exmem.PC,
				Inst:      p.exmem.Inst,
				ALUResult: p.exmem.ALUResult,
				MemData:   memResult.MemData,
				Rd:        p.exmem.Rd,
				RegWrite:  p.exmem.RegWrite,
				MemToReg:  p.exmem.MemToReg,
				IsFused:   p.exmem.IsFused,
			}
		}
	}

	// Secondary slot memory (memory port 2) — tick in parallel with port 1
	var memStall2 bool
	var memResult2 MemoryResult
	if p.exmem2.Valid {
		if p.exmem2.MemRead || p.exmem2.MemWrite {
			memResult2, memStall2 = p.accessSecondaryMem(&p.exmem2)
		}
	}
	// Track whether primary port already counted this stall cycle
	primaryStalled := memStall
	memStall = memStall || memStall2
	if memStall && !primaryStalled {
		p.stats.MemStalls++
	}

	if p.exmem2.Valid && !memStall {
		nextMEMWB2 = SecondaryMEMWBRegister{
			Valid:     true,
			PC:        p.exmem2.PC,
			Inst:      p.exmem2.Inst,
			ALUResult: p.exmem2.ALUResult,
			MemData:   memResult2.MemData,
			Rd:        p.exmem2.Rd,
			RegWrite:  p.exmem2.RegWrite,
			MemToReg:  p.exmem2.MemToReg,
		}
	}

	// Stage 3: Execute (both slots)
	var nextEXMEM EXMEMRegister
	var nextEXMEM2 SecondaryEXMEMRegister
	execStall := false

	// Detect forwarding for primary slot
	forwarding := p.hazardUnit.DetectForwarding(&p.idex, &p.exmem, &p.memwb)

	// Execute primary slot
	if p.idex.Valid && !memStall {
		if p.latencyTable != nil && p.exLatency == 0 {
			if p.useDCache && p.latencyTable.IsLoadOp(p.idex.Inst) {
				p.exLatency = minCacheLoadLatency
			} else {
				p.exLatency = p.latencyTable.GetLatency(p.idex.Inst)
			}
		}

		if p.exLatency > 0 {
			p.exLatency--
		}

		if p.exLatency > 0 {
			execStall = true
			p.stats.ExecStalls++
		} else {
			rnValue := p.hazardUnit.GetForwardedValue(
				forwarding.ForwardRn, p.idex.RnValue, &p.exmem, &savedMEMWB)
			rmValue := p.hazardUnit.GetForwardedValue(
				forwarding.ForwardRm, p.idex.RmValue, &p.exmem, &savedMEMWB)

			// Forward from secondary pipeline stages (exmem2, memwb2) to primary slot
			// When pairs dual-issue, primary slot may need values from previous secondary execution
			if p.memwb2.Valid && p.memwb2.RegWrite && p.memwb2.Rd != 31 {
				if p.idex.Rn == p.memwb2.Rd {
					rnValue = p.memwb2.ALUResult
				}
				if p.idex.Rm == p.memwb2.Rd {
					rmValue = p.memwb2.ALUResult
				}
			}
			if p.exmem2.Valid && p.exmem2.RegWrite && p.exmem2.Rd != 31 {
				if p.idex.Rn == p.exmem2.Rd {
					rnValue = p.exmem2.ALUResult
				}
				if p.idex.Rm == p.exmem2.Rd {
					rmValue = p.exmem2.ALUResult
				}
			}

			// Check for PSTATE flag forwarding from all EXMEM stages (dual-issue).
			forwardFlags := false
			var fwdN, fwdZ, fwdC, fwdV bool
			if p.idex.Inst != nil && p.idex.Inst.Op == insts.OpBCond && !p.idex.IsFused {
				if p.exmem.Valid && p.exmem.SetsFlags {
					forwardFlags = true
					fwdN = p.exmem.FlagN
					fwdZ = p.exmem.FlagZ
					fwdC = p.exmem.FlagC
					fwdV = p.exmem.FlagV
				} else if p.exmem2.Valid && p.exmem2.SetsFlags {
					forwardFlags = true
					fwdN = p.exmem2.FlagN
					fwdZ = p.exmem2.FlagZ
					fwdC = p.exmem2.FlagC
					fwdV = p.exmem2.FlagV
				}
			}

			execResult := p.executeStage.ExecuteWithFlags(&p.idex, rnValue, rmValue,
				forwardFlags, fwdN, fwdZ, fwdC, fwdV)

			storeValue := execResult.StoreValue
			if p.idex.MemWrite {
				rdValue := p.regFile.ReadReg(p.idex.Rd)
				storeValue = p.hazardUnit.GetForwardedValue(
					forwarding.ForwardRd, rdValue, &p.exmem, &savedMEMWB)
			}

			nextEXMEM = EXMEMRegister{
				Valid:      true,
				PC:         p.idex.PC,
				Inst:       p.idex.Inst,
				ALUResult:  execResult.ALUResult,
				StoreValue: storeValue,
				Rd:         p.idex.Rd,
				MemRead:    p.idex.MemRead,
				MemWrite:   p.idex.MemWrite,
				RegWrite:   p.idex.RegWrite,
				MemToReg:   p.idex.MemToReg,
				// Store computed flags for forwarding
				SetsFlags: execResult.SetsFlags,
				FlagN:     execResult.FlagN,
				FlagZ:     execResult.FlagZ,
				FlagC:     execResult.FlagC,
				FlagV:     execResult.FlagV,
			}

			// Branch prediction verification for primary slot (same logic as single-issue)
			if p.idex.IsBranch {
				actualTaken := execResult.BranchTaken
				actualTarget := execResult.BranchTarget

				p.stats.BranchPredictions++

				// Use prediction info captured at fetch time
				predictedTaken := p.idex.PredictedTaken
				predictedTarget := p.idex.PredictedTarget
				earlyResolved := p.idex.EarlyResolved

				// Determine if misprediction occurred
				wasMispredicted := false
				if actualTaken {
					if !predictedTaken {
						wasMispredicted = true
					} else if predictedTarget != actualTarget {
						wasMispredicted = true
					}
				} else {
					if predictedTaken {
						wasMispredicted = true
					}
				}

				// Early-resolved unconditional branches should always be correct
				if earlyResolved && actualTaken {
					wasMispredicted = false
				}

				// Update predictor
				p.branchPredictor.Update(p.idex.PC, actualTaken, actualTarget)

				if wasMispredicted {
					p.stats.BranchMispredictions++
					branchTarget := actualTarget
					if !actualTaken {
						branchTarget = p.idex.PC + 4
					}
					p.pc = branchTarget
					p.ifid.Clear()
					p.ifid2.Clear()
					p.idex.Clear()
					p.idex2.Clear()
					p.stats.Flushes++

					// Latch results and return early
					if !memStall {
						p.memwb = nextMEMWB
						p.memwb2 = nextMEMWB2
						p.exmem = nextEXMEM
						p.exmem2.Clear()
					}
					return
				}
				p.stats.BranchCorrect++
			}
		}
	}

	// Execute secondary slot (if not stalled and slot is valid)
	if p.idex2.Valid && !memStall && !execStall {
		// Convert to IDEXRegister for hazard detection
		idex2 := p.idex2.toIDEX()

		// Detect forwarding for secondary slot from primary pipeline stages
		forwarding2 := p.hazardUnit.DetectForwarding(&idex2, &p.exmem, &p.memwb)

		// Also check forwarding from primary execute result (same cycle)
		if nextEXMEM.Valid && nextEXMEM.RegWrite && nextEXMEM.Rd != 31 {
			if p.idex2.Rn == nextEXMEM.Rd {
				forwarding2.ForwardRn = ForwardFromEXMEM
			}
			if p.idex2.Rm == nextEXMEM.Rd {
				forwarding2.ForwardRm = ForwardFromEXMEM
			}
		}

		if p.latencyTable != nil && p.exLatency2 == 0 {
			p.exLatency2 = p.latencyTable.GetLatency(p.idex2.Inst)
		}

		if p.exLatency2 > 0 {
			p.exLatency2--
		}

		if p.exLatency2 == 0 {
			// Get operand values with forwarding
			// Priority (most recent first): nextEXMEM > exmem/exmem2 > memwb/memwb2 > register
			rnValue := p.idex2.RnValue
			rmValue := p.idex2.RmValue

			// Bug fix: Forward from secondary pipeline stages (exmem2, memwb2)
			// This is needed when consecutive secondary-slot instructions have dependencies.
			// Example: add x1, x1, #1 (→exmem2) followed by add x1, x1, #1 needs x1 from exmem2.

			// First check memwb2 (oldest secondary pipeline stage)
			if p.memwb2.Valid && p.memwb2.RegWrite && p.memwb2.Rd != 31 {
				if p.idex2.Rn == p.memwb2.Rd {
					rnValue = p.memwb2.ALUResult
				}
				if p.idex2.Rm == p.memwb2.Rd {
					rmValue = p.memwb2.ALUResult
				}
			}

			// Then check primary memwb (same age as memwb2, but different register)
			rnValue = p.hazardUnit.GetForwardedValue(
				forwarding2.ForwardRn, rnValue, &p.exmem, &savedMEMWB)
			rmValue = p.hazardUnit.GetForwardedValue(
				forwarding2.ForwardRm, rmValue, &p.exmem, &savedMEMWB)

			// Then check exmem2 (newer than memwb2, same priority as exmem)
			if p.exmem2.Valid && p.exmem2.RegWrite && p.exmem2.Rd != 31 {
				if p.idex2.Rn == p.exmem2.Rd {
					rnValue = p.exmem2.ALUResult
				}
				if p.idex2.Rm == p.exmem2.Rd {
					rmValue = p.exmem2.ALUResult
				}
			}

			// Finally check nextEXMEM (current cycle - highest priority)
			if nextEXMEM.Valid && nextEXMEM.RegWrite && nextEXMEM.Rd != 31 {
				if p.idex2.Rn == nextEXMEM.Rd {
					rnValue = nextEXMEM.ALUResult
				}
				if p.idex2.Rm == nextEXMEM.Rd {
					rmValue = nextEXMEM.ALUResult
				}
			}

			execResult := p.executeStage.Execute(&idex2, rnValue, rmValue)

			nextEXMEM2 = SecondaryEXMEMRegister{
				Valid:      true,
				PC:         p.idex2.PC,
				Inst:       p.idex2.Inst,
				ALUResult:  execResult.ALUResult,
				StoreValue: execResult.StoreValue,
				Rd:         p.idex2.Rd,
				MemRead:    p.idex2.MemRead,
				MemWrite:   p.idex2.MemWrite,
				RegWrite:   p.idex2.RegWrite,
				MemToReg:   p.idex2.MemToReg,
				SetsFlags:  execResult.SetsFlags,
				FlagN:      execResult.FlagN,
				FlagZ:      execResult.FlagZ,
				FlagC:      execResult.FlagC,
				FlagV:      execResult.FlagV,
			}
		}
	}

	// Detect load-use hazards for primary decode
	loadUseHazard := false
	if p.idex.Valid && p.idex.MemRead && p.idex.Rd != 31 && p.ifid.Valid {
		nextInst := p.decodeStage.decoder.Decode(p.ifid.InstructionWord)
		if nextInst != nil && nextInst.Op != insts.OpUnknown {
			usesRn := true
			usesRm := nextInst.Format == insts.FormatDPReg

			sourceRm := nextInst.Rm
			switch nextInst.Op {
			case insts.OpSTR, insts.OpSTRQ:
				usesRm = true
				sourceRm = nextInst.Rd
			}

			loadUseHazard = p.hazardUnit.DetectLoadUseHazardDecoded(
				p.idex.Rd, nextInst.Rn, sourceRm, usesRn, usesRm)
		}
	}

	stallResult := p.hazardUnit.ComputeStalls(loadUseHazard || execStall || memStall, false)

	// Stage 2: Decode (both slots)
	var nextIDEX IDEXRegister
	var nextIDEX2 SecondaryIDEXRegister

	if p.ifid.Valid && !stallResult.StallID && !stallResult.FlushID && !execStall && !memStall {
		decResult := p.decodeStage.Decode(p.ifid.InstructionWord, p.ifid.PC)
		nextIDEX = IDEXRegister{
			Valid:           true,
			PC:              p.ifid.PC,
			Inst:            decResult.Inst,
			RnValue:         decResult.RnValue,
			RmValue:         decResult.RmValue,
			Rd:              decResult.Rd,
			Rn:              decResult.Rn,
			Rm:              decResult.Rm,
			MemRead:         decResult.MemRead,
			MemWrite:        decResult.MemWrite,
			RegWrite:        decResult.RegWrite,
			MemToReg:        decResult.MemToReg,
			IsBranch:        decResult.IsBranch,
			PredictedTaken:  p.ifid.PredictedTaken,
			PredictedTarget: p.ifid.PredictedTarget,
			EarlyResolved:   p.ifid.EarlyResolved,
		}

		// Decode secondary slot if available
		if p.ifid2.Valid {
			decResult2 := p.decodeStage.Decode(p.ifid2.InstructionWord, p.ifid2.PC)
			tempIDEX2 := IDEXRegister{
				Valid:           true,
				PC:              p.ifid2.PC,
				Inst:            decResult2.Inst,
				RnValue:         decResult2.RnValue,
				RmValue:         decResult2.RmValue,
				Rd:              decResult2.Rd,
				Rn:              decResult2.Rn,
				Rm:              decResult2.Rm,
				MemRead:         decResult2.MemRead,
				MemWrite:        decResult2.MemWrite,
				RegWrite:        decResult2.RegWrite,
				MemToReg:        decResult2.MemToReg,
				IsBranch:        decResult2.IsBranch,
				PredictedTaken:  p.ifid2.PredictedTaken,
				PredictedTarget: p.ifid2.PredictedTarget,
				EarlyResolved:   p.ifid2.EarlyResolved,
			}

			// Check if we can dual-issue these two instructions
			if canDualIssue(&nextIDEX, &tempIDEX2) {
				nextIDEX2.fromIDEX(&tempIDEX2)
			}
			// If cannot dual-issue, secondary slot remains clear and the
			// instruction at ifid2 will naturally flow through in the next cycle
			// (ifid2 becomes ifid when we only advance by 4 bytes)
		}
	} else if (stallResult.StallID || execStall || memStall) && !stallResult.FlushID {
		nextIDEX = p.idex
		nextIDEX2 = p.idex2
	}

	// Stage 1: Fetch (both slots)
	var nextIFID IFIDRegister
	var nextIFID2 SecondaryIFIDRegister
	fetchStall := false

	// Check if we successfully dual-issued in decode stage this cycle
	// If the secondary decode slot wasn't used, the instruction at ifid2
	// needs to become the next ifid (we only consumed one instruction)
	dualIssued := nextIDEX2.Valid

	if !stallResult.StallIF && !stallResult.FlushIF && !memStall && !execStall {
		// If we didn't dual-issue last decode, the second instruction (ifid2)
		// becomes the first instruction for this cycle
		if p.ifid2.Valid && !dualIssued {
			// Carry over the second fetched instruction to the primary slot
			// (including its prediction info)
			nextIFID = IFIDRegister{
				Valid:           true,
				PC:              p.ifid2.PC,
				InstructionWord: p.ifid2.InstructionWord,
				PredictedTaken:  p.ifid2.PredictedTaken,
				PredictedTarget: p.ifid2.PredictedTarget,
				EarlyResolved:   p.ifid2.EarlyResolved,
			}
			// Fetch a new second instruction
			var word2 uint32
			var ok2 bool
			var stall2 bool
			if p.useICache && p.cachedFetchStage != nil {
				word2, ok2, stall2 = p.cachedFetchStage.Fetch(p.pc)
				if stall2 {
					fetchStall = true
					p.stats.Stalls++
				}
			} else {
				word2, ok2 = p.fetchStage.Fetch(p.pc)
			}
			if ok2 && !stall2 {
				// Branch elimination for secondary slot
				if isEliminableBranch(word2) {
					_, uncondTarget2 := isUnconditionalBranch(word2, p.pc)
					p.pc = uncondTarget2
					p.stats.EliminatedBranches++
					// Don't create IFID2 entry
				} else {
					// Apply branch prediction to secondary slot
					isUncondBranch2, uncondTarget2 := isUnconditionalBranch(word2, p.pc)
					pred2 := p.branchPredictor.Predict(p.pc)
					earlyResolved2 := false
					if isUncondBranch2 {
						pred2.Taken = true
						pred2.Target = uncondTarget2
						pred2.TargetKnown = true
						earlyResolved2 = true
					}

					nextIFID2 = SecondaryIFIDRegister{
						Valid:           true,
						PC:              p.pc,
						InstructionWord: word2,
						PredictedTaken:  pred2.Taken,
						PredictedTarget: pred2.Target,
						EarlyResolved:   earlyResolved2,
					}

					// Handle branch speculation for secondary slot
					if pred2.Taken && pred2.TargetKnown {
						p.pc = pred2.Target
					} else {
						p.pc += 4
					}
				}
			} else if stall2 {
				// When fetch stalls, preserve the entire pipeline state
				nextIFID = p.ifid
				nextIFID2 = p.ifid2
				nextIDEX = p.idex
				nextIDEX2 = p.idex2
				nextEXMEM = p.exmem
				nextEXMEM2 = p.exmem2
			}
		} else {
			// Normal dual-fetch: fetch two new instructions
			var word uint32
			var ok bool

			if p.useICache && p.cachedFetchStage != nil {
				word, ok, fetchStall = p.cachedFetchStage.Fetch(p.pc)
				if fetchStall {
					p.stats.Stalls++
				}
			} else {
				word, ok = p.fetchStage.Fetch(p.pc)
			}

			if ok && !fetchStall {
				// Branch elimination: unconditional B (not BL) instructions are
				// eliminated at fetch time. They never enter the pipeline.
				if isEliminableBranch(word) {
					_, uncondTarget := isUnconditionalBranch(word, p.pc)
					p.pc = uncondTarget
					p.stats.EliminatedBranches++
					// Don't create IFID entry - branch is eliminated
					// Continue fetching from target in next cycle
				} else {
					// Apply branch prediction to primary slot
					isUncondBranch, uncondTarget := isUnconditionalBranch(word, p.pc)
					pred := p.branchPredictor.Predict(p.pc)
					earlyResolved := false
					if isUncondBranch {
						pred.Taken = true
						pred.Target = uncondTarget
						pred.TargetKnown = true
						earlyResolved = true
					}
					enrichPredictionWithEncodedTarget(&pred, word, p.pc)

					nextIFID = IFIDRegister{
						Valid:           true,
						PC:              p.pc,
						InstructionWord: word,
						PredictedTaken:  pred.Taken,
						PredictedTarget: pred.Target,
						EarlyResolved:   earlyResolved,
					}

					// Handle branch speculation for primary slot
					if pred.Taken && pred.TargetKnown {
						// Branch predicted taken - redirect PC
						p.pc = pred.Target
						// Don't fetch second instruction when branching
						p.pc += 0 // PC already set to target
					} else {
						// No branch or not taken - fetch second instruction
						var word2 uint32
						var ok2 bool
						if p.useICache && p.cachedFetchStage != nil {
							word2, ok2, _ = p.cachedFetchStage.Fetch(p.pc + 4)
						} else {
							word2, ok2 = p.fetchStage.Fetch(p.pc + 4)
						}

						if ok2 {
							// Branch elimination for secondary slot
							if isEliminableBranch(word2) {
								_, uncondTarget2 := isUnconditionalBranch(word2, p.pc+4)
								p.pc = uncondTarget2
								p.stats.EliminatedBranches++
								// Don't create IFID2 entry
							} else {
								// Apply branch prediction to secondary slot
								isUncondBranch2, uncondTarget2 := isUnconditionalBranch(word2, p.pc+4)
								pred2 := p.branchPredictor.Predict(p.pc + 4)
								earlyResolved2 := false
								if isUncondBranch2 {
									pred2.Taken = true
									pred2.Target = uncondTarget2
									pred2.TargetKnown = true
									earlyResolved2 = true
								}

								nextIFID2 = SecondaryIFIDRegister{
									Valid:           true,
									PC:              p.pc + 4,
									InstructionWord: word2,
									PredictedTaken:  pred2.Taken,
									PredictedTarget: pred2.Target,
									EarlyResolved:   earlyResolved2,
								}

								// Handle branch speculation for secondary slot
								if pred2.Taken && pred2.TargetKnown {
									p.pc = pred2.Target
								} else {
									p.pc += 8 // Advance PC by 2 instructions
								}
							}
						} else {
							p.pc += 4
						}
					}
				}
			} else if fetchStall {
				nextIFID = p.ifid
				nextIFID2 = p.ifid2
				// When fetch stalls, we must stall the entire pipeline to prevent
				// instructions from being executed twice. If we decoded/executed,
				// the instructions would be executed again when the stall clears.
				nextIDEX = p.idex
				nextIDEX2 = p.idex2
				nextEXMEM = p.exmem
				nextEXMEM2 = p.exmem2
			}
		}
	} else if (stallResult.StallIF || memStall || execStall) && !stallResult.FlushIF {
		nextIFID = p.ifid
		nextIFID2 = p.ifid2
		p.stats.Stalls++
	}

	// Latch all pipeline registers
	if !memStall && !fetchStall {
		p.memwb = nextMEMWB
		p.memwb2 = nextMEMWB2
	} else {
		p.memwb.Clear()
		p.memwb2.Clear()
	}
	if !execStall && !memStall {
		p.exmem = nextEXMEM
		p.exmem2 = nextEXMEM2
	}
	if stallResult.InsertBubbleEX && !execStall && !memStall {
		p.idex.Clear()
		p.idex2.Clear()
	} else if !memStall {
		p.idex = nextIDEX
		p.idex2 = nextIDEX2
	}
	p.ifid = nextIFID
	p.ifid2 = nextIFID2
}

// tickQuadIssue executes one cycle with 4-wide superscalar support.
// This extends dual-issue to support up to 4 independent instructions per cycle.
//
//nolint:unused // Scaffolding for 4-wide implementation (PR #114)
func (p *Pipeline) tickQuadIssue() {
	// Stage 5: Writeback (all 4 slots using WritebackSlot helper)
	savedMEMWB := p.memwb
	if p.writebackStage.WritebackSlot(&p.memwb) {
		p.stats.Instructions++
	}

	// Writeback secondary slot
	if p.writebackStage.WritebackSlot(&p.memwb2) {
		p.stats.Instructions++
	}

	// Writeback tertiary slot (using WritebackSlot helper)
	if p.writebackStage.WritebackSlot(&p.memwb3) {
		p.stats.Instructions++
	}

	// Writeback quaternary slot (using WritebackSlot helper)
	if p.writebackStage.WritebackSlot(&p.memwb4) {
		p.stats.Instructions++
	}

	// Stage 4: Memory (primary slot only - single memory port)
	var nextMEMWB MEMWBRegister
	var nextMEMWB2 SecondaryMEMWBRegister
	var nextMEMWB3 TertiaryMEMWBRegister
	var nextMEMWB4 QuaternaryMEMWBRegister
	memStall := false

	if p.exmem.Valid {
		if p.exmem.Inst != nil && p.exmem.Inst.Op == insts.OpSVC {
			if p.syscallHandler != nil {
				result := p.syscallHandler.Handle()
				if result.Exited {
					p.halted = true
					p.exitCode = result.ExitCode
				}
			}
		}

		var memResult MemoryResult
		if p.useDCache && p.cachedMemoryStage != nil {
			memResult, memStall = p.cachedMemoryStage.Access(&p.exmem)
			if memStall {
				p.stats.MemStalls++
			}
		} else {
			if p.exmem.MemRead || p.exmem.MemWrite {
				if p.memPending && p.memPendingPC != p.exmem.PC {
					p.memPending = false
				}
				if !p.memPending {
					p.memPending = true
					p.memPendingPC = p.exmem.PC
					memStall = true
					p.stats.MemStalls++
				} else {
					p.memPending = false
					memResult = p.memoryStage.Access(&p.exmem)
				}
			} else {
				p.memPending = false
			}
		}

		if !memStall {
			nextMEMWB = MEMWBRegister{
				Valid:     true,
				PC:        p.exmem.PC,
				Inst:      p.exmem.Inst,
				ALUResult: p.exmem.ALUResult,
				MemData:   memResult.MemData,
				Rd:        p.exmem.Rd,
				RegWrite:  p.exmem.RegWrite,
				MemToReg:  p.exmem.MemToReg,
				IsFused:   p.exmem.IsFused,
			}
		}
	}

	// Secondary slot memory (memory port 2) — tick in parallel with port 1
	var memStall2 bool
	var memResult2 MemoryResult
	if p.exmem2.Valid {
		if p.exmem2.MemRead || p.exmem2.MemWrite {
			memResult2, memStall2 = p.accessSecondaryMem(&p.exmem2)
		}
	}

	// Tertiary slot memory (memory port 3) — tick in parallel with ports 1 & 2
	var memStall3 bool
	var memResult3 MemoryResult
	if p.exmem3.Valid {
		if p.exmem3.MemRead || p.exmem3.MemWrite {
			memResult3, memStall3 = p.accessTertiaryMem(&p.exmem3)
		}
	}

	// Combine stall signals: pipeline stalls if ANY memory port is stalling.
	// Track whether primary port already counted this stall cycle.
	primaryStalled := memStall
	memStall = memStall || memStall2 || memStall3
	if memStall && !primaryStalled {
		p.stats.MemStalls++
	}

	if p.exmem2.Valid && !memStall {
		nextMEMWB2 = SecondaryMEMWBRegister{
			Valid:     true,
			PC:        p.exmem2.PC,
			Inst:      p.exmem2.Inst,
			ALUResult: p.exmem2.ALUResult,
			MemData:   memResult2.MemData,
			Rd:        p.exmem2.Rd,
			RegWrite:  p.exmem2.RegWrite,
			MemToReg:  p.exmem2.MemToReg,
		}
	}

	if p.exmem3.Valid && !memStall {
		nextMEMWB3 = TertiaryMEMWBRegister{
			Valid:     true,
			PC:        p.exmem3.PC,
			Inst:      p.exmem3.Inst,
			ALUResult: p.exmem3.ALUResult,
			MemData:   memResult3.MemData,
			Rd:        p.exmem3.Rd,
			RegWrite:  p.exmem3.RegWrite,
			MemToReg:  p.exmem3.MemToReg,
		}
	}

	// Quaternary slot memory (ALU results only, no memory port)
	if p.exmem4.Valid && !memStall {
		nextMEMWB4 = QuaternaryMEMWBRegister{
			Valid:     true,
			PC:        p.exmem4.PC,
			Inst:      p.exmem4.Inst,
			ALUResult: p.exmem4.ALUResult,
			MemData:   0,
			Rd:        p.exmem4.Rd,
			RegWrite:  p.exmem4.RegWrite,
			MemToReg:  false,
		}
	}

	// Stage 3: Execute (all 4 slots)
	var nextEXMEM EXMEMRegister
	var nextEXMEM2 SecondaryEXMEMRegister
	var nextEXMEM3 TertiaryEXMEMRegister
	var nextEXMEM4 QuaternaryEXMEMRegister
	execStall := false

	// Detect forwarding for primary slot
	forwarding := p.hazardUnit.DetectForwarding(&p.idex, &p.exmem, &p.memwb)

	// Execute primary slot
	if p.idex.Valid && !memStall {
		if p.latencyTable != nil && p.exLatency == 0 {
			if p.useDCache && p.latencyTable.IsLoadOp(p.idex.Inst) {
				p.exLatency = minCacheLoadLatency
			} else {
				p.exLatency = p.latencyTable.GetLatency(p.idex.Inst)
			}
		}

		if p.exLatency > 0 {
			p.exLatency--
		}

		if p.exLatency > 0 {
			execStall = true
			p.stats.ExecStalls++
		} else {
			rnValue := p.hazardUnit.GetForwardedValue(
				forwarding.ForwardRn, p.idex.RnValue, &p.exmem, &savedMEMWB)
			rmValue := p.hazardUnit.GetForwardedValue(
				forwarding.ForwardRm, p.idex.RmValue, &p.exmem, &savedMEMWB)

			// Forward from all secondary pipeline stages to primary slot
			rnValue = p.forwardFromAllSlots(p.idex.Rn, rnValue)
			rmValue = p.forwardFromAllSlots(p.idex.Rm, rmValue)

			// Check for PSTATE flag forwarding from all EXMEM stages (quad-issue).
			forwardFlags := false
			var fwdN, fwdZ, fwdC, fwdV bool
			if p.idex.Inst != nil && p.idex.Inst.Op == insts.OpBCond && !p.idex.IsFused {
				if p.exmem.Valid && p.exmem.SetsFlags {
					forwardFlags = true
					fwdN = p.exmem.FlagN
					fwdZ = p.exmem.FlagZ
					fwdC = p.exmem.FlagC
					fwdV = p.exmem.FlagV
				} else if p.exmem2.Valid && p.exmem2.SetsFlags {
					forwardFlags = true
					fwdN = p.exmem2.FlagN
					fwdZ = p.exmem2.FlagZ
					fwdC = p.exmem2.FlagC
					fwdV = p.exmem2.FlagV
				} else if p.exmem3.Valid && p.exmem3.SetsFlags {
					forwardFlags = true
					fwdN = p.exmem3.FlagN
					fwdZ = p.exmem3.FlagZ
					fwdC = p.exmem3.FlagC
					fwdV = p.exmem3.FlagV
				} else if p.exmem4.Valid && p.exmem4.SetsFlags {
					forwardFlags = true
					fwdN = p.exmem4.FlagN
					fwdZ = p.exmem4.FlagZ
					fwdC = p.exmem4.FlagC
					fwdV = p.exmem4.FlagV
				}
			}

			execResult := p.executeStage.ExecuteWithFlags(&p.idex, rnValue, rmValue,
				forwardFlags, fwdN, fwdZ, fwdC, fwdV)

			storeValue := execResult.StoreValue
			if p.idex.MemWrite {
				rdValue := p.regFile.ReadReg(p.idex.Rd)
				storeValue = p.hazardUnit.GetForwardedValue(
					forwarding.ForwardRd, rdValue, &p.exmem, &savedMEMWB)
			}

			nextEXMEM = EXMEMRegister{
				Valid:      true,
				PC:         p.idex.PC,
				Inst:       p.idex.Inst,
				ALUResult:  execResult.ALUResult,
				StoreValue: storeValue,
				Rd:         p.idex.Rd,
				MemRead:    p.idex.MemRead,
				MemWrite:   p.idex.MemWrite,
				RegWrite:   p.idex.RegWrite,
				MemToReg:   p.idex.MemToReg,
				// Store computed flags for forwarding
				SetsFlags: execResult.SetsFlags,
				FlagN:     execResult.FlagN,
				FlagZ:     execResult.FlagZ,
				FlagC:     execResult.FlagC,
				FlagV:     execResult.FlagV,
			}

			// Branch prediction verification for primary slot
			if p.idex.IsBranch {
				actualTaken := execResult.BranchTaken
				actualTarget := execResult.BranchTarget

				p.stats.BranchPredictions++

				// Use prediction info captured at fetch time
				predictedTaken := p.idex.PredictedTaken
				predictedTarget := p.idex.PredictedTarget
				earlyResolved := p.idex.EarlyResolved

				// Determine if misprediction occurred
				wasMispredicted := false
				if actualTaken {
					if !predictedTaken {
						wasMispredicted = true
					} else if predictedTarget != actualTarget {
						wasMispredicted = true
					}
				} else {
					if predictedTaken {
						wasMispredicted = true
					}
				}

				// Early-resolved unconditional branches should always be correct
				if earlyResolved && actualTaken {
					wasMispredicted = false
				}

				// Update predictor with actual outcome
				p.branchPredictor.Update(p.idex.PC, actualTaken, actualTarget)

				if wasMispredicted {
					p.stats.BranchMispredictions++
					branchTarget := actualTarget
					if !actualTaken {
						branchTarget = p.idex.PC + 4
					}
					p.pc = branchTarget
					p.flushAllIFID()
					p.flushAllIDEX()
					p.stats.Flushes++

					// Latch results and return early
					if !memStall {
						p.memwb = nextMEMWB
						p.memwb2 = nextMEMWB2
						p.memwb3 = nextMEMWB3
						p.memwb4 = nextMEMWB4
						p.exmem = nextEXMEM
						p.exmem2.Clear()
						p.exmem3.Clear()
						p.exmem4.Clear()
					}
					return
				}
				p.stats.BranchCorrect++
			}
		}
	}

	// Execute secondary slot (if not stalled and slot is valid)
	if p.idex2.Valid && !memStall && !execStall {
		if p.latencyTable != nil && p.exLatency2 == 0 {
			p.exLatency2 = p.latencyTable.GetLatency(p.idex2.Inst)
		}

		if p.exLatency2 > 0 {
			p.exLatency2--
		}

		if p.exLatency2 == 0 {
			rnValue := p.idex2.RnValue
			rmValue := p.idex2.RmValue

			// Forward from all pipeline stages
			rnValue = p.forwardFromAllSlots(p.idex2.Rn, rnValue)
			rmValue = p.forwardFromAllSlots(p.idex2.Rm, rmValue)

			// Forward from current cycle's primary execution
			if nextEXMEM.Valid && nextEXMEM.RegWrite && nextEXMEM.Rd != 31 {
				if p.idex2.Rn == nextEXMEM.Rd {
					rnValue = nextEXMEM.ALUResult
				}
				if p.idex2.Rm == nextEXMEM.Rd {
					rmValue = nextEXMEM.ALUResult
				}
			}

			idex2 := p.idex2.toIDEX()
			execResult := p.executeStage.Execute(&idex2, rnValue, rmValue)

			nextEXMEM2 = SecondaryEXMEMRegister{
				Valid:      true,
				PC:         p.idex2.PC,
				Inst:       p.idex2.Inst,
				ALUResult:  execResult.ALUResult,
				StoreValue: execResult.StoreValue,
				Rd:         p.idex2.Rd,
				MemRead:    p.idex2.MemRead,
				MemWrite:   p.idex2.MemWrite,
				RegWrite:   p.idex2.RegWrite,
				MemToReg:   p.idex2.MemToReg,
				SetsFlags:  execResult.SetsFlags,
				FlagN:      execResult.FlagN,
				FlagZ:      execResult.FlagZ,
				FlagC:      execResult.FlagC,
				FlagV:      execResult.FlagV,
			}
		}
	}

	// Execute tertiary slot
	if p.idex3.Valid && !memStall && !execStall {
		if p.latencyTable != nil && p.exLatency3 == 0 {
			p.exLatency3 = p.latencyTable.GetLatency(p.idex3.Inst)
		}

		if p.exLatency3 > 0 {
			p.exLatency3--
		}

		if p.exLatency3 == 0 {
			rnValue := p.idex3.RnValue
			rmValue := p.idex3.RmValue

			// Forward from all pipeline stages
			rnValue = p.forwardFromAllSlots(p.idex3.Rn, rnValue)
			rmValue = p.forwardFromAllSlots(p.idex3.Rm, rmValue)

			// Forward from current cycle's earlier executions
			if nextEXMEM.Valid && nextEXMEM.RegWrite && nextEXMEM.Rd != 31 {
				if p.idex3.Rn == nextEXMEM.Rd {
					rnValue = nextEXMEM.ALUResult
				}
				if p.idex3.Rm == nextEXMEM.Rd {
					rmValue = nextEXMEM.ALUResult
				}
			}
			if nextEXMEM2.Valid && nextEXMEM2.RegWrite && nextEXMEM2.Rd != 31 {
				if p.idex3.Rn == nextEXMEM2.Rd {
					rnValue = nextEXMEM2.ALUResult
				}
				if p.idex3.Rm == nextEXMEM2.Rd {
					rmValue = nextEXMEM2.ALUResult
				}
			}

			idex3 := p.idex3.toIDEX()
			execResult := p.executeStage.Execute(&idex3, rnValue, rmValue)

			nextEXMEM3 = TertiaryEXMEMRegister{
				Valid:      true,
				PC:         p.idex3.PC,
				Inst:       p.idex3.Inst,
				ALUResult:  execResult.ALUResult,
				StoreValue: execResult.StoreValue,
				Rd:         p.idex3.Rd,
				MemRead:    p.idex3.MemRead,
				MemWrite:   p.idex3.MemWrite,
				RegWrite:   p.idex3.RegWrite,
				MemToReg:   p.idex3.MemToReg,
				SetsFlags:  execResult.SetsFlags,
				FlagN:      execResult.FlagN,
				FlagZ:      execResult.FlagZ,
				FlagC:      execResult.FlagC,
				FlagV:      execResult.FlagV,
			}
		}
	}

	// Execute quaternary slot
	if p.idex4.Valid && !memStall && !execStall {
		if p.latencyTable != nil && p.exLatency4 == 0 {
			p.exLatency4 = p.latencyTable.GetLatency(p.idex4.Inst)
		}

		if p.exLatency4 > 0 {
			p.exLatency4--
		}

		if p.exLatency4 == 0 {
			rnValue := p.idex4.RnValue
			rmValue := p.idex4.RmValue

			// Forward from all pipeline stages
			rnValue = p.forwardFromAllSlots(p.idex4.Rn, rnValue)
			rmValue = p.forwardFromAllSlots(p.idex4.Rm, rmValue)

			// Forward from current cycle's earlier executions
			if nextEXMEM.Valid && nextEXMEM.RegWrite && nextEXMEM.Rd != 31 {
				if p.idex4.Rn == nextEXMEM.Rd {
					rnValue = nextEXMEM.ALUResult
				}
				if p.idex4.Rm == nextEXMEM.Rd {
					rmValue = nextEXMEM.ALUResult
				}
			}
			if nextEXMEM2.Valid && nextEXMEM2.RegWrite && nextEXMEM2.Rd != 31 {
				if p.idex4.Rn == nextEXMEM2.Rd {
					rnValue = nextEXMEM2.ALUResult
				}
				if p.idex4.Rm == nextEXMEM2.Rd {
					rmValue = nextEXMEM2.ALUResult
				}
			}
			if nextEXMEM3.Valid && nextEXMEM3.RegWrite && nextEXMEM3.Rd != 31 {
				if p.idex4.Rn == nextEXMEM3.Rd {
					rnValue = nextEXMEM3.ALUResult
				}
				if p.idex4.Rm == nextEXMEM3.Rd {
					rmValue = nextEXMEM3.ALUResult
				}
			}

			idex4 := p.idex4.toIDEX()
			execResult := p.executeStage.Execute(&idex4, rnValue, rmValue)

			nextEXMEM4 = QuaternaryEXMEMRegister{
				Valid:      true,
				PC:         p.idex4.PC,
				Inst:       p.idex4.Inst,
				ALUResult:  execResult.ALUResult,
				StoreValue: execResult.StoreValue,
				Rd:         p.idex4.Rd,
				MemRead:    p.idex4.MemRead,
				MemWrite:   p.idex4.MemWrite,
				RegWrite:   p.idex4.RegWrite,
				MemToReg:   p.idex4.MemToReg,
				SetsFlags:  execResult.SetsFlags,
				FlagN:      execResult.FlagN,
				FlagZ:      execResult.FlagZ,
				FlagC:      execResult.FlagC,
				FlagV:      execResult.FlagV,
			}
		}
	}

	// Detect load-use hazards for primary decode
	loadUseHazard := false
	if p.idex.Valid && p.idex.MemRead && p.idex.Rd != 31 && p.ifid.Valid {
		nextInst := p.decodeStage.decoder.Decode(p.ifid.InstructionWord)
		if nextInst != nil && nextInst.Op != insts.OpUnknown {
			usesRn := true
			usesRm := nextInst.Format == insts.FormatDPReg

			sourceRm := nextInst.Rm
			switch nextInst.Op {
			case insts.OpSTR, insts.OpSTRQ:
				usesRm = true
				sourceRm = nextInst.Rd
			}

			loadUseHazard = p.hazardUnit.DetectLoadUseHazardDecoded(
				p.idex.Rd, nextInst.Rn, sourceRm, usesRn, usesRm)
		}
	}

	stallResult := p.hazardUnit.ComputeStalls(loadUseHazard || execStall || memStall, false)

	// Stage 2: Decode (all 4 slots)
	var nextIDEX IDEXRegister
	var nextIDEX2 SecondaryIDEXRegister
	var nextIDEX3 TertiaryIDEXRegister
	var nextIDEX4 QuaternaryIDEXRegister

	if p.ifid.Valid && !stallResult.StallID && !stallResult.FlushID && !execStall && !memStall {
		decResult := p.decodeStage.Decode(p.ifid.InstructionWord, p.ifid.PC)
		nextIDEX = IDEXRegister{
			Valid:           true,
			PC:              p.ifid.PC,
			Inst:            decResult.Inst,
			RnValue:         decResult.RnValue,
			RmValue:         decResult.RmValue,
			Rd:              decResult.Rd,
			Rn:              decResult.Rn,
			Rm:              decResult.Rm,
			MemRead:         decResult.MemRead,
			MemWrite:        decResult.MemWrite,
			RegWrite:        decResult.RegWrite,
			MemToReg:        decResult.MemToReg,
			IsBranch:        decResult.IsBranch,
			PredictedTaken:  p.ifid.PredictedTaken,
			PredictedTarget: p.ifid.PredictedTarget,
			EarlyResolved:   p.ifid.EarlyResolved,
		}

		// Try to issue instructions 2, 3, 4 if they can issue with earlier instructions.
		// Uses fixed-size array to avoid heap allocation per tick.
		var issuedInsts [8]*IDEXRegister
		issuedInsts[0] = &nextIDEX
		issuedCount := 1

		// Decode slot 2
		if p.ifid2.Valid {
			decResult2 := p.decodeStage.Decode(p.ifid2.InstructionWord, p.ifid2.PC)
			tempIDEX2 := IDEXRegister{
				Valid:           true,
				PC:              p.ifid2.PC,
				Inst:            decResult2.Inst,
				RnValue:         decResult2.RnValue,
				RmValue:         decResult2.RmValue,
				Rd:              decResult2.Rd,
				Rn:              decResult2.Rn,
				Rm:              decResult2.Rm,
				MemRead:         decResult2.MemRead,
				MemWrite:        decResult2.MemWrite,
				RegWrite:        decResult2.RegWrite,
				MemToReg:        decResult2.MemToReg,
				IsBranch:        decResult2.IsBranch,
				PredictedTaken:  p.ifid2.PredictedTaken,
				PredictedTarget: p.ifid2.PredictedTarget,
				EarlyResolved:   p.ifid2.EarlyResolved,
			}

			if canIssueWith(&tempIDEX2, &issuedInsts, issuedCount) {
				nextIDEX2.fromIDEX(&tempIDEX2)
				issuedInsts[issuedCount] = &tempIDEX2
				issuedCount++
			}
		}

		// Decode slot 3
		if p.ifid3.Valid && nextIDEX2.Valid {
			decResult3 := p.decodeStage.Decode(p.ifid3.InstructionWord, p.ifid3.PC)
			tempIDEX3 := IDEXRegister{
				Valid:           true,
				PC:              p.ifid3.PC,
				Inst:            decResult3.Inst,
				RnValue:         decResult3.RnValue,
				RmValue:         decResult3.RmValue,
				Rd:              decResult3.Rd,
				Rn:              decResult3.Rn,
				Rm:              decResult3.Rm,
				MemRead:         decResult3.MemRead,
				MemWrite:        decResult3.MemWrite,
				RegWrite:        decResult3.RegWrite,
				MemToReg:        decResult3.MemToReg,
				IsBranch:        decResult3.IsBranch,
				PredictedTaken:  p.ifid3.PredictedTaken,
				PredictedTarget: p.ifid3.PredictedTarget,
				EarlyResolved:   p.ifid3.EarlyResolved,
			}

			if canIssueWith(&tempIDEX3, &issuedInsts, issuedCount) {
				nextIDEX3.fromIDEX(&tempIDEX3)
				issuedInsts[issuedCount] = &tempIDEX3
				issuedCount++
			}
		}

		// Decode slot 4
		if p.ifid4.Valid && nextIDEX3.Valid {
			decResult4 := p.decodeStage.Decode(p.ifid4.InstructionWord, p.ifid4.PC)
			tempIDEX4 := IDEXRegister{
				Valid:           true,
				PC:              p.ifid4.PC,
				Inst:            decResult4.Inst,
				RnValue:         decResult4.RnValue,
				RmValue:         decResult4.RmValue,
				Rd:              decResult4.Rd,
				Rn:              decResult4.Rn,
				Rm:              decResult4.Rm,
				MemRead:         decResult4.MemRead,
				MemWrite:        decResult4.MemWrite,
				RegWrite:        decResult4.RegWrite,
				MemToReg:        decResult4.MemToReg,
				IsBranch:        decResult4.IsBranch,
				PredictedTaken:  p.ifid4.PredictedTaken,
				PredictedTarget: p.ifid4.PredictedTarget,
				EarlyResolved:   p.ifid4.EarlyResolved,
			}

			if canIssueWith(&tempIDEX4, &issuedInsts, issuedCount) {
				nextIDEX4.fromIDEX(&tempIDEX4)
			}
		}
	} else if (stallResult.StallID || execStall || memStall) && !stallResult.FlushID {
		nextIDEX = p.idex
		nextIDEX2 = p.idex2
		nextIDEX3 = p.idex3
		nextIDEX4 = p.idex4
	}

	// Count how many instructions were issued this cycle for fetch advancement
	issueCount := 0
	if nextIDEX.Valid {
		issueCount++
	}
	if nextIDEX2.Valid {
		issueCount++
	}
	if nextIDEX3.Valid {
		issueCount++
	}
	if nextIDEX4.Valid {
		issueCount++
	}

	// Stage 1: Fetch (all 4 slots)
	var nextIFID IFIDRegister
	var nextIFID2 SecondaryIFIDRegister
	var nextIFID3 TertiaryIFIDRegister
	var nextIFID4 QuaternaryIFIDRegister
	fetchStall := false

	if !stallResult.StallIF && !stallResult.FlushIF && !memStall && !execStall {
		// Shift unissued instructions forward
		pendingInsts, pendingCount := p.collectPendingFetchInstructions(issueCount)

		// Fill slots with pending instructions first, then fetch new ones
		fetchPC := p.pc
		slotIdx := 0

		// Place pending instructions
		branchPredictedTaken := false
		for pi := 0; pi < pendingCount; pi++ {
			pending := pendingInsts[pi]
			// If slot 0 had a predicted-taken branch, discard remaining pending instructions
			// (they're on the wrong path)
			if branchPredictedTaken {
				break
			}
			switch slotIdx {
			case 0:
				// Apply branch prediction when placing in primary slot
				isUncondBranch, uncondTarget := isUnconditionalBranch(pending.Word, pending.PC)
				pred := p.branchPredictor.Predict(pending.PC)
				earlyResolved := false
				if isUncondBranch {
					pred.Taken = true
					pred.Target = uncondTarget
					pred.TargetKnown = true
					earlyResolved = true
				}
				enrichPredictionWithEncodedTarget(&pred, pending.Word, pending.PC)
				nextIFID = IFIDRegister{
					Valid:           true,
					PC:              pending.PC,
					InstructionWord: pending.Word,
					PredictedTaken:  pred.Taken,
					PredictedTarget: pred.Target,
					EarlyResolved:   earlyResolved,
				}
				// If branch predicted taken in slot 0, redirect fetch and discard other pending
				if pred.Taken && pred.TargetKnown {
					fetchPC = pred.Target
					branchPredictedTaken = true
				}
			default:
				isUncondBranch, uncondTarget := isUnconditionalBranch(pending.Word, pending.PC)
				pred := p.branchPredictor.Predict(pending.PC)
				earlyResolved := false
				if isUncondBranch {
					pred.Taken = true
					pred.Target = uncondTarget
					pred.TargetKnown = true
					earlyResolved = true
				}
				enrichPredictionWithEncodedTarget(&pred, pending.Word, pending.PC)
				switch slotIdx {
				case 1:
					nextIFID2 = SecondaryIFIDRegister{Valid: true, PC: pending.PC, InstructionWord: pending.Word, PredictedTaken: pred.Taken, PredictedTarget: pred.Target, EarlyResolved: earlyResolved}
				case 2:
					nextIFID3 = TertiaryIFIDRegister{Valid: true, PC: pending.PC, InstructionWord: pending.Word, PredictedTaken: pred.Taken, PredictedTarget: pred.Target, EarlyResolved: earlyResolved}
				case 3:
					nextIFID4 = QuaternaryIFIDRegister{Valid: true, PC: pending.PC, InstructionWord: pending.Word, PredictedTaken: pred.Taken, PredictedTarget: pred.Target, EarlyResolved: earlyResolved}
				}
				if pred.Taken && pred.TargetKnown {
					fetchPC = pred.Target
					branchPredictedTaken = true
				}
			}
			slotIdx++
		}

		// Fetch new instructions to fill remaining slots
		for slotIdx < 4 {
			var word uint32
			var ok bool

			if p.useICache && p.cachedFetchStage != nil {
				word, ok, fetchStall = p.cachedFetchStage.Fetch(fetchPC)
				if fetchStall {
					p.stats.Stalls++
					break
				}
			} else {
				word, ok = p.fetchStage.Fetch(fetchPC)
			}

			if !ok {
				break
			}

			// Branch elimination: unconditional B (not BL) instructions are
			// eliminated at fetch time. They never enter the pipeline.
			if isEliminableBranch(word) {
				_, uncondTarget := isUnconditionalBranch(word, fetchPC)
				fetchPC = uncondTarget
				p.stats.EliminatedBranches++
				// Don't create IFID entry - branch is eliminated
				// Continue fetching from target without advancing slotIdx
				continue
			}

			// Apply branch prediction for slot 0 (branches can only execute from primary slot)
			if slotIdx == 0 {
				isUncondBranch, uncondTarget := isUnconditionalBranch(word, fetchPC)
				pred := p.branchPredictor.Predict(fetchPC)
				earlyResolved := false
				if isUncondBranch {
					pred.Taken = true
					pred.Target = uncondTarget
					pred.TargetKnown = true
					earlyResolved = true
				}
				enrichPredictionWithEncodedTarget(&pred, word, fetchPC)

				nextIFID = IFIDRegister{
					Valid:           true,
					PC:              fetchPC,
					InstructionWord: word,
					PredictedTaken:  pred.Taken,
					PredictedTarget: pred.Target,
					EarlyResolved:   earlyResolved,
				}

				// If branch predicted taken, redirect fetch to target
				if pred.Taken && pred.TargetKnown {
					fetchPC = pred.Target
					slotIdx++
					continue
				}
			} else {
				isUncondBranch, uncondTarget := isUnconditionalBranch(word, fetchPC)
				pred := p.branchPredictor.Predict(fetchPC)
				earlyResolved := false
				if isUncondBranch {
					pred.Taken = true
					pred.Target = uncondTarget
					pred.TargetKnown = true
					earlyResolved = true
				}
				enrichPredictionWithEncodedTarget(&pred, word, fetchPC)
				switch slotIdx {
				case 1:
					nextIFID2 = SecondaryIFIDRegister{Valid: true, PC: fetchPC, InstructionWord: word, PredictedTaken: pred.Taken, PredictedTarget: pred.Target, EarlyResolved: earlyResolved}
				case 2:
					nextIFID3 = TertiaryIFIDRegister{Valid: true, PC: fetchPC, InstructionWord: word, PredictedTaken: pred.Taken, PredictedTarget: pred.Target, EarlyResolved: earlyResolved}
				case 3:
					nextIFID4 = QuaternaryIFIDRegister{Valid: true, PC: fetchPC, InstructionWord: word, PredictedTaken: pred.Taken, PredictedTarget: pred.Target, EarlyResolved: earlyResolved}
				}
				if pred.Taken && pred.TargetKnown {
					fetchPC = pred.Target
					slotIdx++
					continue
				}
			}
			fetchPC += 4
			slotIdx++
		}
		p.pc = fetchPC

		if fetchStall {
			// Preserve all pipeline state on fetch stall
			nextIFID = p.ifid
			nextIFID2 = p.ifid2
			nextIFID3 = p.ifid3
			nextIFID4 = p.ifid4
			nextIDEX = p.idex
			nextIDEX2 = p.idex2
			nextIDEX3 = p.idex3
			nextIDEX4 = p.idex4
			nextEXMEM = p.exmem
			nextEXMEM2 = p.exmem2
			nextEXMEM3 = p.exmem3
			nextEXMEM4 = p.exmem4
		}
	} else if (stallResult.StallIF || memStall || execStall) && !stallResult.FlushIF {
		nextIFID = p.ifid
		nextIFID2 = p.ifid2
		nextIFID3 = p.ifid3
		nextIFID4 = p.ifid4
		p.stats.Stalls++
	}

	// Latch all pipeline registers
	if !memStall && !fetchStall {
		p.memwb = nextMEMWB
		p.memwb2 = nextMEMWB2
		p.memwb3 = nextMEMWB3
		p.memwb4 = nextMEMWB4
	} else {
		p.memwb.Clear()
		p.memwb2.Clear()
		p.memwb3.Clear()
		p.memwb4.Clear()
	}
	if !execStall && !memStall {
		p.exmem = nextEXMEM
		p.exmem2 = nextEXMEM2
		p.exmem3 = nextEXMEM3
		p.exmem4 = nextEXMEM4
	}
	if stallResult.InsertBubbleEX && !execStall && !memStall {
		p.idex.Clear()
		p.idex2.Clear()
		p.idex3.Clear()
		p.idex4.Clear()
	} else if !memStall {
		p.idex = nextIDEX
		p.idex2 = nextIDEX2
		p.idex3 = nextIDEX3
		p.idex4 = nextIDEX4
	}
	p.ifid = nextIFID
	p.ifid2 = nextIFID2
	p.ifid3 = nextIFID3
	p.ifid4 = nextIFID4
}

// pendingFetchInst represents an instruction waiting in fetch buffer.
//
//nolint:unused // Scaffolding for 4-wide implementation (PR #114)
type pendingFetchInst struct {
	PC   uint64
	Word uint32
}

// collectPendingFetchInstructions returns unissued instructions that need to remain in fetch.
// issueCount is how many instructions were issued from the current IF/ID registers.
// Uses a fixed-size array to avoid heap allocation per tick.
//
//nolint:unused // Scaffolding for 4-wide implementation (PR #114)
func (p *Pipeline) collectPendingFetchInstructions(issueCount int) ([8]pendingFetchInst, int) {
	var allFetched [8]pendingFetchInst
	count := 0

	if p.ifid.Valid {
		allFetched[count] = pendingFetchInst{PC: p.ifid.PC, Word: p.ifid.InstructionWord}
		count++
	}
	if p.ifid2.Valid {
		allFetched[count] = pendingFetchInst{PC: p.ifid2.PC, Word: p.ifid2.InstructionWord}
		count++
	}
	if p.ifid3.Valid {
		allFetched[count] = pendingFetchInst{PC: p.ifid3.PC, Word: p.ifid3.InstructionWord}
		count++
	}
	if p.ifid4.Valid {
		allFetched[count] = pendingFetchInst{PC: p.ifid4.PC, Word: p.ifid4.InstructionWord}
		count++
	}

	// Skip the first issueCount instructions (they were issued).
	// Shift remaining down to index 0.
	pendingCount := 0
	if issueCount < count {
		pendingCount = count - issueCount
		for i := 0; i < pendingCount; i++ {
			allFetched[i] = allFetched[i+issueCount]
		}
	}

	return allFetched, pendingCount
}

// forwardFromAllSlots checks all secondary pipeline stages for forwarding.
//
//nolint:unused // Scaffolding for 4-wide implementation (PR #114)
func (p *Pipeline) forwardFromAllSlots(reg uint8, currentValue uint64) uint64 {
	if reg == 31 {
		return currentValue
	}

	// Check memwb stages (oldest first, primary slot first)
	if p.memwb.Valid && p.memwb.RegWrite && p.memwb.Rd == reg {
		if p.memwb.MemToReg {
			currentValue = p.memwb.MemData
		} else {
			currentValue = p.memwb.ALUResult
		}
	}
	if p.memwb2.Valid && p.memwb2.RegWrite && p.memwb2.Rd == reg {
		currentValue = p.memwb2.ALUResult
	}
	if p.memwb3.Valid && p.memwb3.RegWrite && p.memwb3.Rd == reg {
		currentValue = p.memwb3.ALUResult
	}
	if p.memwb4.Valid && p.memwb4.RegWrite && p.memwb4.Rd == reg {
		currentValue = p.memwb4.ALUResult
	}
	if p.memwb5.Valid && p.memwb5.RegWrite && p.memwb5.Rd == reg {
		currentValue = p.memwb5.ALUResult
	}
	if p.memwb6.Valid && p.memwb6.RegWrite && p.memwb6.Rd == reg {
		currentValue = p.memwb6.ALUResult
	}
	if p.memwb7.Valid && p.memwb7.RegWrite && p.memwb7.Rd == reg {
		currentValue = p.memwb7.ALUResult
	}
	if p.memwb8.Valid && p.memwb8.RegWrite && p.memwb8.Rd == reg {
		currentValue = p.memwb8.ALUResult
	}

	// Check exmem stages (newer, higher priority, primary slot first)
	if p.exmem.Valid && p.exmem.RegWrite && p.exmem.Rd == reg {
		currentValue = p.exmem.ALUResult
	}
	if p.exmem2.Valid && p.exmem2.RegWrite && p.exmem2.Rd == reg {
		currentValue = p.exmem2.ALUResult
	}
	if p.exmem3.Valid && p.exmem3.RegWrite && p.exmem3.Rd == reg {
		currentValue = p.exmem3.ALUResult
	}
	if p.exmem4.Valid && p.exmem4.RegWrite && p.exmem4.Rd == reg {
		currentValue = p.exmem4.ALUResult
	}
	if p.exmem5.Valid && p.exmem5.RegWrite && p.exmem5.Rd == reg {
		currentValue = p.exmem5.ALUResult
	}
	if p.exmem6.Valid && p.exmem6.RegWrite && p.exmem6.Rd == reg {
		currentValue = p.exmem6.ALUResult
	}
	if p.exmem7.Valid && p.exmem7.RegWrite && p.exmem7.Rd == reg {
		currentValue = p.exmem7.ALUResult
	}
	if p.exmem8.Valid && p.exmem8.RegWrite && p.exmem8.Rd == reg {
		currentValue = p.exmem8.ALUResult
	}

	return currentValue
}

// flushAllIFID clears all IF/ID pipeline registers.
//
//nolint:unused // Scaffolding for 4-wide implementation (PR #114)
func (p *Pipeline) flushAllIFID() {
	p.ifid.Clear()
	p.ifid2.Clear()
	p.ifid3.Clear()
	p.ifid4.Clear()
	p.ifid5.Clear()
	p.ifid6.Clear()
	p.ifid7.Clear()
	p.ifid8.Clear()
}

// flushAllIDEX clears all ID/EX pipeline registers.
//
//nolint:unused // Scaffolding for 4-wide implementation (PR #114)
func (p *Pipeline) flushAllIDEX() {
	p.idex.Clear()
	p.idex2.Clear()
	p.idex3.Clear()
	p.idex4.Clear()
	p.idex5.Clear()
	p.idex6.Clear()
	p.idex7.Clear()
	p.idex8.Clear()
}

// tickSextupleIssue executes one cycle with 6-wide superscalar support.
// This extends 4-wide to match the Apple M2's 6 integer ALUs.
func (p *Pipeline) tickSextupleIssue() {
	// Stage 5: Writeback (all 6 slots using WritebackSlot helper)
	savedMEMWB := p.memwb
	if p.writebackStage.WritebackSlot(&p.memwb) {
		p.stats.Instructions++
		// Fused CMP+B.cond counts as 2 instructions
		if p.memwb.IsFused {
			p.stats.Instructions++
		}
	}

	// Writeback secondary slot
	if p.writebackStage.WritebackSlot(&p.memwb2) {
		p.stats.Instructions++
	}

	// Writeback tertiary slot (using WritebackSlot helper)
	if p.writebackStage.WritebackSlot(&p.memwb3) {
		p.stats.Instructions++
	}

	// Writeback quaternary slot (using WritebackSlot helper)
	if p.writebackStage.WritebackSlot(&p.memwb4) {
		p.stats.Instructions++
	}

	// Writeback quinary slot (using WritebackSlot helper)
	if p.writebackStage.WritebackSlot(&p.memwb5) {
		p.stats.Instructions++
	}

	// Writeback senary slot (using WritebackSlot helper)
	if p.writebackStage.WritebackSlot(&p.memwb6) {
		p.stats.Instructions++
	}

	// Stage 4: Memory (primary slot only - single memory port)
	var nextMEMWB MEMWBRegister
	var nextMEMWB2 SecondaryMEMWBRegister
	var nextMEMWB3 TertiaryMEMWBRegister
	var nextMEMWB4 QuaternaryMEMWBRegister
	var nextMEMWB5 QuinaryMEMWBRegister
	var nextMEMWB6 SenaryMEMWBRegister
	memStall := false

	if p.exmem.Valid {
		if p.exmem.Inst != nil && p.exmem.Inst.Op == insts.OpSVC {
			if p.syscallHandler != nil {
				result := p.syscallHandler.Handle()
				if result.Exited {
					p.halted = true
					p.exitCode = result.ExitCode
				}
			}
		}

		var memResult MemoryResult
		if p.useDCache && p.cachedMemoryStage != nil {
			memResult, memStall = p.cachedMemoryStage.Access(&p.exmem)
			if memStall {
				p.stats.MemStalls++
			}
		} else {
			if p.exmem.MemRead || p.exmem.MemWrite {
				if p.memPending && p.memPendingPC != p.exmem.PC {
					p.memPending = false
				}
				if !p.memPending {
					p.memPending = true
					p.memPendingPC = p.exmem.PC
					memStall = true
					p.stats.MemStalls++
				} else {
					p.memPending = false
					memResult = p.memoryStage.Access(&p.exmem)
				}
			} else {
				p.memPending = false
			}
		}

		if !memStall {
			nextMEMWB = MEMWBRegister{
				Valid:     true,
				PC:        p.exmem.PC,
				Inst:      p.exmem.Inst,
				ALUResult: p.exmem.ALUResult,
				MemData:   memResult.MemData,
				Rd:        p.exmem.Rd,
				RegWrite:  p.exmem.RegWrite,
				MemToReg:  p.exmem.MemToReg,
				IsFused:   p.exmem.IsFused,
			}
		}
	}

	// Secondary slot memory (memory port 2) — tick in parallel with port 1
	var memStall2 bool
	var memResult2 MemoryResult
	if p.exmem2.Valid {
		if p.exmem2.MemRead || p.exmem2.MemWrite {
			memResult2, memStall2 = p.accessSecondaryMem(&p.exmem2)
		}
	}

	// Tertiary slot memory (memory port 3) — tick in parallel with ports 1 & 2
	var memStall3 bool
	var memResult3 MemoryResult
	if p.exmem3.Valid {
		if p.exmem3.MemRead || p.exmem3.MemWrite {
			memResult3, memStall3 = p.accessTertiaryMem(&p.exmem3)
		}
	}

	// Combine stall signals: pipeline stalls if ANY memory port is stalling.
	// Track whether primary port already counted this stall cycle.
	primaryStalled := memStall
	memStall = memStall || memStall2 || memStall3
	if memStall && !primaryStalled {
		p.stats.MemStalls++
	}

	if p.exmem2.Valid && !memStall {
		nextMEMWB2 = SecondaryMEMWBRegister{
			Valid:     true,
			PC:        p.exmem2.PC,
			Inst:      p.exmem2.Inst,
			ALUResult: p.exmem2.ALUResult,
			MemData:   memResult2.MemData,
			Rd:        p.exmem2.Rd,
			RegWrite:  p.exmem2.RegWrite,
			MemToReg:  p.exmem2.MemToReg,
		}
	}

	if p.exmem3.Valid && !memStall {
		nextMEMWB3 = TertiaryMEMWBRegister{
			Valid:     true,
			PC:        p.exmem3.PC,
			Inst:      p.exmem3.Inst,
			ALUResult: p.exmem3.ALUResult,
			MemData:   memResult3.MemData,
			Rd:        p.exmem3.Rd,
			RegWrite:  p.exmem3.RegWrite,
			MemToReg:  p.exmem3.MemToReg,
		}
	}

	// Quaternary slot memory (ALU results only, no memory port)
	if p.exmem4.Valid && !memStall {
		nextMEMWB4 = QuaternaryMEMWBRegister{
			Valid:     true,
			PC:        p.exmem4.PC,
			Inst:      p.exmem4.Inst,
			ALUResult: p.exmem4.ALUResult,
			MemData:   0,
			Rd:        p.exmem4.Rd,
			RegWrite:  p.exmem4.RegWrite,
			MemToReg:  false,
		}
	}

	// Quinary slot memory (ALU results only, no memory port)
	if p.exmem5.Valid && !memStall {
		nextMEMWB5 = QuinaryMEMWBRegister{
			Valid:     true,
			PC:        p.exmem5.PC,
			Inst:      p.exmem5.Inst,
			ALUResult: p.exmem5.ALUResult,
			MemData:   0,
			Rd:        p.exmem5.Rd,
			RegWrite:  p.exmem5.RegWrite,
			MemToReg:  false,
		}
	}

	// Senary slot memory (ALU results only, no memory port)
	if p.exmem6.Valid && !memStall {
		nextMEMWB6 = SenaryMEMWBRegister{
			Valid:     true,
			PC:        p.exmem6.PC,
			Inst:      p.exmem6.Inst,
			ALUResult: p.exmem6.ALUResult,
			MemData:   0,
			Rd:        p.exmem6.Rd,
			RegWrite:  p.exmem6.RegWrite,
			MemToReg:  false,
		}
	}

	// Stage 3: Execute (all 6 slots)
	var nextEXMEM EXMEMRegister
	var nextEXMEM2 SecondaryEXMEMRegister
	var nextEXMEM3 TertiaryEXMEMRegister
	var nextEXMEM4 QuaternaryEXMEMRegister
	var nextEXMEM5 QuinaryEXMEMRegister
	var nextEXMEM6 SenaryEXMEMRegister
	execStall := false

	// Detect forwarding for primary slot
	forwarding := p.hazardUnit.DetectForwarding(&p.idex, &p.exmem, &p.memwb)

	// Execute primary slot
	if p.idex.Valid && !memStall {
		if p.latencyTable != nil && p.exLatency == 0 {
			if p.useDCache && p.latencyTable.IsLoadOp(p.idex.Inst) {
				p.exLatency = minCacheLoadLatency
			} else {
				p.exLatency = p.latencyTable.GetLatency(p.idex.Inst)
			}
		}

		if p.exLatency > 0 {
			p.exLatency--
		}

		if p.exLatency > 0 {
			execStall = true
			p.stats.ExecStalls++
		} else {
			rnValue := p.hazardUnit.GetForwardedValue(
				forwarding.ForwardRn, p.idex.RnValue, &p.exmem, &savedMEMWB)
			rmValue := p.hazardUnit.GetForwardedValue(
				forwarding.ForwardRm, p.idex.RmValue, &p.exmem, &savedMEMWB)

			// Forward from all secondary pipeline stages to primary slot
			rnValue = p.forwardFromAllSlots(p.idex.Rn, rnValue)
			rmValue = p.forwardFromAllSlots(p.idex.Rm, rmValue)

			// Check for PSTATE flag forwarding from all EXMEM stages (sextuple-issue).
			forwardFlags := false
			var fwdN, fwdZ, fwdC, fwdV bool
			if p.idex.Inst != nil && p.idex.Inst.Op == insts.OpBCond && !p.idex.IsFused {
				if p.exmem.Valid && p.exmem.SetsFlags {
					forwardFlags = true
					fwdN = p.exmem.FlagN
					fwdZ = p.exmem.FlagZ
					fwdC = p.exmem.FlagC
					fwdV = p.exmem.FlagV
				} else if p.exmem2.Valid && p.exmem2.SetsFlags {
					forwardFlags = true
					fwdN = p.exmem2.FlagN
					fwdZ = p.exmem2.FlagZ
					fwdC = p.exmem2.FlagC
					fwdV = p.exmem2.FlagV
				} else if p.exmem3.Valid && p.exmem3.SetsFlags {
					forwardFlags = true
					fwdN = p.exmem3.FlagN
					fwdZ = p.exmem3.FlagZ
					fwdC = p.exmem3.FlagC
					fwdV = p.exmem3.FlagV
				} else if p.exmem4.Valid && p.exmem4.SetsFlags {
					forwardFlags = true
					fwdN = p.exmem4.FlagN
					fwdZ = p.exmem4.FlagZ
					fwdC = p.exmem4.FlagC
					fwdV = p.exmem4.FlagV
				} else if p.exmem5.Valid && p.exmem5.SetsFlags {
					forwardFlags = true
					fwdN = p.exmem5.FlagN
					fwdZ = p.exmem5.FlagZ
					fwdC = p.exmem5.FlagC
					fwdV = p.exmem5.FlagV
				} else if p.exmem6.Valid && p.exmem6.SetsFlags {
					forwardFlags = true
					fwdN = p.exmem6.FlagN
					fwdZ = p.exmem6.FlagZ
					fwdC = p.exmem6.FlagC
					fwdV = p.exmem6.FlagV
				}
			}

			execResult := p.executeStage.ExecuteWithFlags(&p.idex, rnValue, rmValue,
				forwardFlags, fwdN, fwdZ, fwdC, fwdV)

			storeValue := execResult.StoreValue
			if p.idex.MemWrite {
				rdValue := p.regFile.ReadReg(p.idex.Rd)
				storeValue = p.hazardUnit.GetForwardedValue(
					forwarding.ForwardRd, rdValue, &p.exmem, &savedMEMWB)
			}

			nextEXMEM = EXMEMRegister{
				Valid:      true,
				PC:         p.idex.PC,
				Inst:       p.idex.Inst,
				ALUResult:  execResult.ALUResult,
				StoreValue: storeValue,
				Rd:         p.idex.Rd,
				MemRead:    p.idex.MemRead,
				MemWrite:   p.idex.MemWrite,
				RegWrite:   p.idex.RegWrite,
				MemToReg:   p.idex.MemToReg,
				IsFused:    p.idex.IsFused,
				// Store computed flags for forwarding
				SetsFlags: execResult.SetsFlags,
				FlagN:     execResult.FlagN,
				FlagZ:     execResult.FlagZ,
				FlagC:     execResult.FlagC,
				FlagV:     execResult.FlagV,
			}

			// Branch prediction verification for primary slot
			if p.idex.IsBranch {
				actualTaken := execResult.BranchTaken
				actualTarget := execResult.BranchTarget

				p.stats.BranchPredictions++

				predictedTaken := p.idex.PredictedTaken
				predictedTarget := p.idex.PredictedTarget
				earlyResolved := p.idex.EarlyResolved

				wasMispredicted := false
				if actualTaken {
					if !predictedTaken {
						wasMispredicted = true
					} else if predictedTarget != actualTarget {
						wasMispredicted = true
					}
				} else {
					if predictedTaken {
						wasMispredicted = true
					}
				}

				if earlyResolved && actualTaken {
					wasMispredicted = false
				}

				p.branchPredictor.Update(p.idex.PC, actualTaken, actualTarget)

				if wasMispredicted {
					p.stats.BranchMispredictions++
					branchTarget := actualTarget
					if !actualTaken {
						branchTarget = p.idex.PC + 4
					}
					p.pc = branchTarget
					p.flushAllIFID()
					p.flushAllIDEX()
					p.stats.Flushes++

					// Latch results and return early
					if !memStall {
						p.memwb = nextMEMWB
						p.memwb2 = nextMEMWB2
						p.memwb3 = nextMEMWB3
						p.memwb4 = nextMEMWB4
						p.memwb5 = nextMEMWB5
						p.memwb6 = nextMEMWB6
						p.exmem = nextEXMEM
						p.exmem2.Clear()
						p.exmem3.Clear()
						p.exmem4.Clear()
						p.exmem5.Clear()
						p.exmem6.Clear()
					}
					return
				}
				p.stats.BranchCorrect++
			}
		}
	}

	// Execute secondary slot
	if p.idex2.Valid && !memStall && !execStall {
		if p.latencyTable != nil && p.exLatency2 == 0 {
			p.exLatency2 = p.latencyTable.GetLatency(p.idex2.Inst)
		}
		if p.exLatency2 > 0 {
			p.exLatency2--
		}
		if p.exLatency2 == 0 {
			rnValue := p.forwardFromAllSlots(p.idex2.Rn, p.idex2.RnValue)
			rmValue := p.forwardFromAllSlots(p.idex2.Rm, p.idex2.RmValue)
			if nextEXMEM.Valid && nextEXMEM.RegWrite && nextEXMEM.Rd != 31 {
				if p.idex2.Rn == nextEXMEM.Rd {
					rnValue = nextEXMEM.ALUResult
				}
				if p.idex2.Rm == nextEXMEM.Rd {
					rmValue = nextEXMEM.ALUResult
				}
			}
			idex2 := p.idex2.toIDEX()
			execResult := p.executeStage.Execute(&idex2, rnValue, rmValue)
			nextEXMEM2 = SecondaryEXMEMRegister{
				Valid:      true,
				PC:         p.idex2.PC,
				Inst:       p.idex2.Inst,
				ALUResult:  execResult.ALUResult,
				StoreValue: execResult.StoreValue,
				Rd:         p.idex2.Rd,
				MemRead:    p.idex2.MemRead,
				MemWrite:   p.idex2.MemWrite,
				RegWrite:   p.idex2.RegWrite,
				MemToReg:   p.idex2.MemToReg,
				SetsFlags:  execResult.SetsFlags,
				FlagN:      execResult.FlagN,
				FlagZ:      execResult.FlagZ,
				FlagC:      execResult.FlagC,
				FlagV:      execResult.FlagV,
			}
		}
	}

	// Execute tertiary slot
	if p.idex3.Valid && !memStall && !execStall {
		if p.latencyTable != nil && p.exLatency3 == 0 {
			p.exLatency3 = p.latencyTable.GetLatency(p.idex3.Inst)
		}
		if p.exLatency3 > 0 {
			p.exLatency3--
		}
		if p.exLatency3 == 0 {
			rnValue := p.forwardFromAllSlots(p.idex3.Rn, p.idex3.RnValue)
			rmValue := p.forwardFromAllSlots(p.idex3.Rm, p.idex3.RmValue)
			if nextEXMEM.Valid && nextEXMEM.RegWrite && nextEXMEM.Rd != 31 {
				if p.idex3.Rn == nextEXMEM.Rd {
					rnValue = nextEXMEM.ALUResult
				}
				if p.idex3.Rm == nextEXMEM.Rd {
					rmValue = nextEXMEM.ALUResult
				}
			}
			if nextEXMEM2.Valid && nextEXMEM2.RegWrite && nextEXMEM2.Rd != 31 {
				if p.idex3.Rn == nextEXMEM2.Rd {
					rnValue = nextEXMEM2.ALUResult
				}
				if p.idex3.Rm == nextEXMEM2.Rd {
					rmValue = nextEXMEM2.ALUResult
				}
			}
			idex3 := p.idex3.toIDEX()
			execResult := p.executeStage.Execute(&idex3, rnValue, rmValue)
			nextEXMEM3 = TertiaryEXMEMRegister{
				Valid:      true,
				PC:         p.idex3.PC,
				Inst:       p.idex3.Inst,
				ALUResult:  execResult.ALUResult,
				StoreValue: execResult.StoreValue,
				Rd:         p.idex3.Rd,
				MemRead:    p.idex3.MemRead,
				MemWrite:   p.idex3.MemWrite,
				RegWrite:   p.idex3.RegWrite,
				MemToReg:   p.idex3.MemToReg,
				SetsFlags:  execResult.SetsFlags,
				FlagN:      execResult.FlagN,
				FlagZ:      execResult.FlagZ,
				FlagC:      execResult.FlagC,
				FlagV:      execResult.FlagV,
			}
		}
	}

	// Execute quaternary slot
	if p.idex4.Valid && !memStall && !execStall {
		if p.latencyTable != nil && p.exLatency4 == 0 {
			p.exLatency4 = p.latencyTable.GetLatency(p.idex4.Inst)
		}
		if p.exLatency4 > 0 {
			p.exLatency4--
		}
		if p.exLatency4 == 0 {
			rnValue := p.forwardFromAllSlots(p.idex4.Rn, p.idex4.RnValue)
			rmValue := p.forwardFromAllSlots(p.idex4.Rm, p.idex4.RmValue)
			if nextEXMEM.Valid && nextEXMEM.RegWrite && nextEXMEM.Rd != 31 {
				if p.idex4.Rn == nextEXMEM.Rd {
					rnValue = nextEXMEM.ALUResult
				}
				if p.idex4.Rm == nextEXMEM.Rd {
					rmValue = nextEXMEM.ALUResult
				}
			}
			if nextEXMEM2.Valid && nextEXMEM2.RegWrite && nextEXMEM2.Rd != 31 {
				if p.idex4.Rn == nextEXMEM2.Rd {
					rnValue = nextEXMEM2.ALUResult
				}
				if p.idex4.Rm == nextEXMEM2.Rd {
					rmValue = nextEXMEM2.ALUResult
				}
			}
			if nextEXMEM3.Valid && nextEXMEM3.RegWrite && nextEXMEM3.Rd != 31 {
				if p.idex4.Rn == nextEXMEM3.Rd {
					rnValue = nextEXMEM3.ALUResult
				}
				if p.idex4.Rm == nextEXMEM3.Rd {
					rmValue = nextEXMEM3.ALUResult
				}
			}
			idex4 := p.idex4.toIDEX()
			execResult := p.executeStage.Execute(&idex4, rnValue, rmValue)
			nextEXMEM4 = QuaternaryEXMEMRegister{
				Valid:      true,
				PC:         p.idex4.PC,
				Inst:       p.idex4.Inst,
				ALUResult:  execResult.ALUResult,
				StoreValue: execResult.StoreValue,
				Rd:         p.idex4.Rd,
				MemRead:    p.idex4.MemRead,
				MemWrite:   p.idex4.MemWrite,
				RegWrite:   p.idex4.RegWrite,
				MemToReg:   p.idex4.MemToReg,
				SetsFlags:  execResult.SetsFlags,
				FlagN:      execResult.FlagN,
				FlagZ:      execResult.FlagZ,
				FlagC:      execResult.FlagC,
				FlagV:      execResult.FlagV,
			}
		}
	}

	// Execute quinary slot
	if p.idex5.Valid && !memStall && !execStall {
		if p.latencyTable != nil && p.exLatency5 == 0 {
			p.exLatency5 = p.latencyTable.GetLatency(p.idex5.Inst)
		}
		if p.exLatency5 > 0 {
			p.exLatency5--
		}
		if p.exLatency5 == 0 {
			rnValue := p.forwardFromAllSlots(p.idex5.Rn, p.idex5.RnValue)
			rmValue := p.forwardFromAllSlots(p.idex5.Rm, p.idex5.RmValue)
			if nextEXMEM.Valid && nextEXMEM.RegWrite && nextEXMEM.Rd != 31 {
				if p.idex5.Rn == nextEXMEM.Rd {
					rnValue = nextEXMEM.ALUResult
				}
				if p.idex5.Rm == nextEXMEM.Rd {
					rmValue = nextEXMEM.ALUResult
				}
			}
			if nextEXMEM2.Valid && nextEXMEM2.RegWrite && nextEXMEM2.Rd != 31 {
				if p.idex5.Rn == nextEXMEM2.Rd {
					rnValue = nextEXMEM2.ALUResult
				}
				if p.idex5.Rm == nextEXMEM2.Rd {
					rmValue = nextEXMEM2.ALUResult
				}
			}
			if nextEXMEM3.Valid && nextEXMEM3.RegWrite && nextEXMEM3.Rd != 31 {
				if p.idex5.Rn == nextEXMEM3.Rd {
					rnValue = nextEXMEM3.ALUResult
				}
				if p.idex5.Rm == nextEXMEM3.Rd {
					rmValue = nextEXMEM3.ALUResult
				}
			}
			if nextEXMEM4.Valid && nextEXMEM4.RegWrite && nextEXMEM4.Rd != 31 {
				if p.idex5.Rn == nextEXMEM4.Rd {
					rnValue = nextEXMEM4.ALUResult
				}
				if p.idex5.Rm == nextEXMEM4.Rd {
					rmValue = nextEXMEM4.ALUResult
				}
			}
			idex5 := p.idex5.toIDEX()
			execResult := p.executeStage.Execute(&idex5, rnValue, rmValue)
			nextEXMEM5 = QuinaryEXMEMRegister{
				Valid:      true,
				PC:         p.idex5.PC,
				Inst:       p.idex5.Inst,
				ALUResult:  execResult.ALUResult,
				StoreValue: execResult.StoreValue,
				Rd:         p.idex5.Rd,
				MemRead:    p.idex5.MemRead,
				MemWrite:   p.idex5.MemWrite,
				RegWrite:   p.idex5.RegWrite,
				MemToReg:   p.idex5.MemToReg,
				SetsFlags:  execResult.SetsFlags,
				FlagN:      execResult.FlagN,
				FlagZ:      execResult.FlagZ,
				FlagC:      execResult.FlagC,
				FlagV:      execResult.FlagV,
			}
		}
	}

	// Execute senary slot
	if p.idex6.Valid && !memStall && !execStall {
		if p.latencyTable != nil && p.exLatency6 == 0 {
			p.exLatency6 = p.latencyTable.GetLatency(p.idex6.Inst)
		}
		if p.exLatency6 > 0 {
			p.exLatency6--
		}
		if p.exLatency6 == 0 {
			rnValue := p.forwardFromAllSlots(p.idex6.Rn, p.idex6.RnValue)
			rmValue := p.forwardFromAllSlots(p.idex6.Rm, p.idex6.RmValue)
			if nextEXMEM.Valid && nextEXMEM.RegWrite && nextEXMEM.Rd != 31 {
				if p.idex6.Rn == nextEXMEM.Rd {
					rnValue = nextEXMEM.ALUResult
				}
				if p.idex6.Rm == nextEXMEM.Rd {
					rmValue = nextEXMEM.ALUResult
				}
			}
			if nextEXMEM2.Valid && nextEXMEM2.RegWrite && nextEXMEM2.Rd != 31 {
				if p.idex6.Rn == nextEXMEM2.Rd {
					rnValue = nextEXMEM2.ALUResult
				}
				if p.idex6.Rm == nextEXMEM2.Rd {
					rmValue = nextEXMEM2.ALUResult
				}
			}
			if nextEXMEM3.Valid && nextEXMEM3.RegWrite && nextEXMEM3.Rd != 31 {
				if p.idex6.Rn == nextEXMEM3.Rd {
					rnValue = nextEXMEM3.ALUResult
				}
				if p.idex6.Rm == nextEXMEM3.Rd {
					rmValue = nextEXMEM3.ALUResult
				}
			}
			if nextEXMEM4.Valid && nextEXMEM4.RegWrite && nextEXMEM4.Rd != 31 {
				if p.idex6.Rn == nextEXMEM4.Rd {
					rnValue = nextEXMEM4.ALUResult
				}
				if p.idex6.Rm == nextEXMEM4.Rd {
					rmValue = nextEXMEM4.ALUResult
				}
			}
			if nextEXMEM5.Valid && nextEXMEM5.RegWrite && nextEXMEM5.Rd != 31 {
				if p.idex6.Rn == nextEXMEM5.Rd {
					rnValue = nextEXMEM5.ALUResult
				}
				if p.idex6.Rm == nextEXMEM5.Rd {
					rmValue = nextEXMEM5.ALUResult
				}
			}
			idex6 := p.idex6.toIDEX()
			execResult := p.executeStage.Execute(&idex6, rnValue, rmValue)
			nextEXMEM6 = SenaryEXMEMRegister{
				Valid:      true,
				PC:         p.idex6.PC,
				Inst:       p.idex6.Inst,
				ALUResult:  execResult.ALUResult,
				StoreValue: execResult.StoreValue,
				Rd:         p.idex6.Rd,
				MemRead:    p.idex6.MemRead,
				MemWrite:   p.idex6.MemWrite,
				RegWrite:   p.idex6.RegWrite,
				MemToReg:   p.idex6.MemToReg,
				SetsFlags:  execResult.SetsFlags,
				FlagN:      execResult.FlagN,
				FlagZ:      execResult.FlagZ,
				FlagC:      execResult.FlagC,
				FlagV:      execResult.FlagV,
			}
		}
	}

	// Detect load-use hazards for primary decode
	loadUseHazard := false
	if p.idex.Valid && p.idex.MemRead && p.idex.Rd != 31 && p.ifid.Valid {
		nextInst := p.decodeStage.decoder.Decode(p.ifid.InstructionWord)
		if nextInst != nil && nextInst.Op != insts.OpUnknown {
			usesRn := true
			usesRm := nextInst.Format == insts.FormatDPReg

			sourceRm := nextInst.Rm
			switch nextInst.Op {
			case insts.OpSTR, insts.OpSTRQ:
				usesRm = true
				sourceRm = nextInst.Rd
			}

			loadUseHazard = p.hazardUnit.DetectLoadUseHazardDecoded(
				p.idex.Rd, nextInst.Rn, sourceRm, usesRn, usesRm)
		}
	}

	stallResult := p.hazardUnit.ComputeStalls(loadUseHazard || execStall || memStall, false)

	// Stage 2: Decode (all 6 slots)
	var nextIDEX IDEXRegister
	var nextIDEX2 SecondaryIDEXRegister
	var nextIDEX3 TertiaryIDEXRegister
	var nextIDEX4 QuaternaryIDEXRegister
	var nextIDEX5 QuinaryIDEXRegister
	var nextIDEX6 SenaryIDEXRegister

	// Track CMP+B.cond fusion for issue count adjustment
	fusedCMPBcond := false

	if p.ifid.Valid && !stallResult.StallID && !stallResult.FlushID && !execStall && !memStall {
		decResult := p.decodeStage.Decode(p.ifid.InstructionWord, p.ifid.PC)

		// CMP+B.cond fusion detection: check if slot 0 is CMP and slot 1 is B.cond
		if IsCMP(decResult.Inst) && p.ifid2.Valid {
			decResult2 := p.decodeStage.Decode(p.ifid2.InstructionWord, p.ifid2.PC)
			if IsBCond(decResult2.Inst) {
				// Fuse CMP+B.cond: put B.cond in slot 0 with CMP operands
				fusedCMPBcond = true
				nextIDEX = IDEXRegister{
					Valid:           true,
					PC:              p.ifid2.PC,
					Inst:            decResult2.Inst,
					RnValue:         decResult2.RnValue,
					RmValue:         decResult2.RmValue,
					Rd:              decResult2.Rd,
					Rn:              decResult2.Rn,
					Rm:              decResult2.Rm,
					MemRead:         decResult2.MemRead,
					MemWrite:        decResult2.MemWrite,
					RegWrite:        decResult2.RegWrite,
					MemToReg:        decResult2.MemToReg,
					IsBranch:        decResult2.IsBranch,
					PredictedTaken:  p.ifid2.PredictedTaken,
					PredictedTarget: p.ifid2.PredictedTarget,
					EarlyResolved:   p.ifid2.EarlyResolved,
					// Fusion fields from CMP
					IsFused:    true,
					FusedRnVal: decResult.RnValue,
					FusedRmVal: decResult.RmValue,
					FusedIs64:  decResult.Inst.Is64Bit,
					FusedIsImm: decResult.Inst.Format == insts.FormatDPImm,
					FusedImmVal: func() uint64 {
						if decResult.Inst.Format == insts.FormatDPImm {
							imm := decResult.Inst.Imm
							if decResult.Inst.Shift > 0 {
								imm <<= decResult.Inst.Shift
							}
							return imm
						}
						return 0
					}(),
				}
				// Mark both instructions as consumed (CMP + B.cond count as 2 issued)
				// This will be reflected in the issueCount later
				// Note: IsFused flag is propagated through the pipeline.
				// When the fused instruction retires, it counts as 2 instructions.
			}
		}

		if !fusedCMPBcond {
			nextIDEX = IDEXRegister{
				Valid:           true,
				PC:              p.ifid.PC,
				Inst:            decResult.Inst,
				RnValue:         decResult.RnValue,
				RmValue:         decResult.RmValue,
				Rd:              decResult.Rd,
				Rn:              decResult.Rn,
				Rm:              decResult.Rm,
				MemRead:         decResult.MemRead,
				MemWrite:        decResult.MemWrite,
				RegWrite:        decResult.RegWrite,
				MemToReg:        decResult.MemToReg,
				IsBranch:        decResult.IsBranch,
				PredictedTaken:  p.ifid.PredictedTaken,
				PredictedTarget: p.ifid.PredictedTarget,
				EarlyResolved:   p.ifid.EarlyResolved,
			}
		}

		// Try to issue instructions 2-6 if they can issue with earlier instructions.
		// Uses fixed-size array to avoid heap allocation per tick.
		var issuedInsts [8]*IDEXRegister
		issuedInsts[0] = &nextIDEX
		issuedCount := 1

		// Track if IFID2 was consumed by fusion (skip its decode)
		ifid2ConsumedByFusion := fusedCMPBcond

		// Decode slot 2 (IFID2) - skip if consumed by fusion
		if p.ifid2.Valid && !ifid2ConsumedByFusion {
			decResult2 := p.decodeStage.Decode(p.ifid2.InstructionWord, p.ifid2.PC)
			tempIDEX2 := IDEXRegister{
				Valid:           true,
				PC:              p.ifid2.PC,
				Inst:            decResult2.Inst,
				RnValue:         decResult2.RnValue,
				RmValue:         decResult2.RmValue,
				Rd:              decResult2.Rd,
				Rn:              decResult2.Rn,
				Rm:              decResult2.Rm,
				MemRead:         decResult2.MemRead,
				MemWrite:        decResult2.MemWrite,
				RegWrite:        decResult2.RegWrite,
				MemToReg:        decResult2.MemToReg,
				IsBranch:        decResult2.IsBranch,
				PredictedTaken:  p.ifid2.PredictedTaken,
				PredictedTarget: p.ifid2.PredictedTarget,
				EarlyResolved:   p.ifid2.EarlyResolved,
			}
			if canIssueWith(&tempIDEX2, &issuedInsts, issuedCount) {
				nextIDEX2.fromIDEX(&tempIDEX2)
				issuedInsts[issuedCount] = &tempIDEX2
				issuedCount++
			}
		}

		// Decode slot 3
		if p.ifid3.Valid && nextIDEX2.Valid {
			decResult3 := p.decodeStage.Decode(p.ifid3.InstructionWord, p.ifid3.PC)
			tempIDEX3 := IDEXRegister{
				Valid:           true,
				PC:              p.ifid3.PC,
				Inst:            decResult3.Inst,
				RnValue:         decResult3.RnValue,
				RmValue:         decResult3.RmValue,
				Rd:              decResult3.Rd,
				Rn:              decResult3.Rn,
				Rm:              decResult3.Rm,
				MemRead:         decResult3.MemRead,
				MemWrite:        decResult3.MemWrite,
				RegWrite:        decResult3.RegWrite,
				MemToReg:        decResult3.MemToReg,
				IsBranch:        decResult3.IsBranch,
				PredictedTaken:  p.ifid3.PredictedTaken,
				PredictedTarget: p.ifid3.PredictedTarget,
				EarlyResolved:   p.ifid3.EarlyResolved,
			}
			if canIssueWith(&tempIDEX3, &issuedInsts, issuedCount) {
				nextIDEX3.fromIDEX(&tempIDEX3)
				issuedInsts[issuedCount] = &tempIDEX3
				issuedCount++
			}
		}

		// Decode slot 4
		if p.ifid4.Valid && nextIDEX3.Valid {
			decResult4 := p.decodeStage.Decode(p.ifid4.InstructionWord, p.ifid4.PC)
			tempIDEX4 := IDEXRegister{
				Valid:           true,
				PC:              p.ifid4.PC,
				Inst:            decResult4.Inst,
				RnValue:         decResult4.RnValue,
				RmValue:         decResult4.RmValue,
				Rd:              decResult4.Rd,
				Rn:              decResult4.Rn,
				Rm:              decResult4.Rm,
				MemRead:         decResult4.MemRead,
				MemWrite:        decResult4.MemWrite,
				RegWrite:        decResult4.RegWrite,
				MemToReg:        decResult4.MemToReg,
				IsBranch:        decResult4.IsBranch,
				PredictedTaken:  p.ifid4.PredictedTaken,
				PredictedTarget: p.ifid4.PredictedTarget,
				EarlyResolved:   p.ifid4.EarlyResolved,
			}
			if canIssueWith(&tempIDEX4, &issuedInsts, issuedCount) {
				nextIDEX4.fromIDEX(&tempIDEX4)
				issuedInsts[issuedCount] = &tempIDEX4
				issuedCount++
			}
		}

		// Decode slot 5
		if p.ifid5.Valid && nextIDEX4.Valid {
			decResult5 := p.decodeStage.Decode(p.ifid5.InstructionWord, p.ifid5.PC)
			tempIDEX5 := IDEXRegister{
				Valid:           true,
				PC:              p.ifid5.PC,
				Inst:            decResult5.Inst,
				RnValue:         decResult5.RnValue,
				RmValue:         decResult5.RmValue,
				Rd:              decResult5.Rd,
				Rn:              decResult5.Rn,
				Rm:              decResult5.Rm,
				MemRead:         decResult5.MemRead,
				MemWrite:        decResult5.MemWrite,
				RegWrite:        decResult5.RegWrite,
				MemToReg:        decResult5.MemToReg,
				IsBranch:        decResult5.IsBranch,
				PredictedTaken:  p.ifid5.PredictedTaken,
				PredictedTarget: p.ifid5.PredictedTarget,
				EarlyResolved:   p.ifid5.EarlyResolved,
			}
			if canIssueWith(&tempIDEX5, &issuedInsts, issuedCount) {
				nextIDEX5.fromIDEX(&tempIDEX5)
				issuedInsts[issuedCount] = &tempIDEX5
				issuedCount++
			}
		}

		// Decode slot 6
		if p.ifid6.Valid && nextIDEX5.Valid {
			decResult6 := p.decodeStage.Decode(p.ifid6.InstructionWord, p.ifid6.PC)
			tempIDEX6 := IDEXRegister{
				Valid:           true,
				PC:              p.ifid6.PC,
				Inst:            decResult6.Inst,
				RnValue:         decResult6.RnValue,
				RmValue:         decResult6.RmValue,
				Rd:              decResult6.Rd,
				Rn:              decResult6.Rn,
				Rm:              decResult6.Rm,
				MemRead:         decResult6.MemRead,
				MemWrite:        decResult6.MemWrite,
				RegWrite:        decResult6.RegWrite,
				MemToReg:        decResult6.MemToReg,
				IsBranch:        decResult6.IsBranch,
				PredictedTaken:  p.ifid6.PredictedTaken,
				PredictedTarget: p.ifid6.PredictedTarget,
				EarlyResolved:   p.ifid6.EarlyResolved,
			}
			if canIssueWith(&tempIDEX6, &issuedInsts, issuedCount) {
				nextIDEX6.fromIDEX(&tempIDEX6)
			}
		}
	} else if (stallResult.StallID || execStall || memStall) && !stallResult.FlushID {
		nextIDEX = p.idex
		nextIDEX2 = p.idex2
		nextIDEX3 = p.idex3
		nextIDEX4 = p.idex4
		nextIDEX5 = p.idex5
		nextIDEX6 = p.idex6
	}

	// Count how many instructions were issued this cycle for fetch advancement
	issueCount := 0
	if nextIDEX.Valid {
		issueCount++
	}
	if nextIDEX2.Valid {
		issueCount++
	}
	if nextIDEX3.Valid {
		issueCount++
	}
	if nextIDEX4.Valid {
		issueCount++
	}
	if nextIDEX5.Valid {
		issueCount++
	}
	if nextIDEX6.Valid {
		issueCount++
	}
	// CMP+B.cond fusion consumes 2 IFID slots but produces 1 IDEX,
	// so add 1 to issueCount to advance fetch properly
	if fusedCMPBcond {
		issueCount++
	}

	// Stage 1: Fetch (all 6 slots)
	var nextIFID IFIDRegister
	var nextIFID2 SecondaryIFIDRegister
	var nextIFID3 TertiaryIFIDRegister
	var nextIFID4 QuaternaryIFIDRegister
	var nextIFID5 QuinaryIFIDRegister
	var nextIFID6 SenaryIFIDRegister
	fetchStall := false

	if !stallResult.StallIF && !stallResult.FlushIF && !memStall && !execStall {
		// Shift unissued instructions forward
		pendingInsts, pendingCount := p.collectPendingFetchInstructions6(issueCount)

		// Fill slots with pending instructions first, then fetch new ones
		fetchPC := p.pc
		slotIdx := 0

		// Place pending instructions
		branchPredictedTaken := false
		for pi := 0; pi < pendingCount; pi++ {
			pending := pendingInsts[pi]
			if branchPredictedTaken {
				break
			}
			switch slotIdx {
			case 0:
				isUncondBranch, uncondTarget := isUnconditionalBranch(pending.Word, pending.PC)
				pred := p.branchPredictor.Predict(pending.PC)
				earlyResolved := false
				if isUncondBranch {
					pred.Taken = true
					pred.Target = uncondTarget
					pred.TargetKnown = true
					earlyResolved = true
				}
				enrichPredictionWithEncodedTarget(&pred, pending.Word, pending.PC)
				nextIFID = IFIDRegister{
					Valid:           true,
					PC:              pending.PC,
					InstructionWord: pending.Word,
					PredictedTaken:  pred.Taken,
					PredictedTarget: pred.Target,
					EarlyResolved:   earlyResolved,
				}
				if pred.Taken && pred.TargetKnown {
					fetchPC = pred.Target
					branchPredictedTaken = true
				}
			default:
				isUncondBranch, uncondTarget := isUnconditionalBranch(pending.Word, pending.PC)
				pred := p.branchPredictor.Predict(pending.PC)
				earlyResolved := false
				if isUncondBranch {
					pred.Taken = true
					pred.Target = uncondTarget
					pred.TargetKnown = true
					earlyResolved = true
				}
				enrichPredictionWithEncodedTarget(&pred, pending.Word, pending.PC)
				switch slotIdx {
				case 1:
					nextIFID2 = SecondaryIFIDRegister{Valid: true, PC: pending.PC, InstructionWord: pending.Word, PredictedTaken: pred.Taken, PredictedTarget: pred.Target, EarlyResolved: earlyResolved}
				case 2:
					nextIFID3 = TertiaryIFIDRegister{Valid: true, PC: pending.PC, InstructionWord: pending.Word, PredictedTaken: pred.Taken, PredictedTarget: pred.Target, EarlyResolved: earlyResolved}
				case 3:
					nextIFID4 = QuaternaryIFIDRegister{Valid: true, PC: pending.PC, InstructionWord: pending.Word, PredictedTaken: pred.Taken, PredictedTarget: pred.Target, EarlyResolved: earlyResolved}
				case 4:
					nextIFID5 = QuinaryIFIDRegister{Valid: true, PC: pending.PC, InstructionWord: pending.Word, PredictedTaken: pred.Taken, PredictedTarget: pred.Target, EarlyResolved: earlyResolved}
				case 5:
					nextIFID6 = SenaryIFIDRegister{Valid: true, PC: pending.PC, InstructionWord: pending.Word, PredictedTaken: pred.Taken, PredictedTarget: pred.Target, EarlyResolved: earlyResolved}
				}
				if pred.Taken && pred.TargetKnown {
					fetchPC = pred.Target
					branchPredictedTaken = true
				}
			}
			slotIdx++
		}

		// Fetch new instructions to fill remaining slots
		for slotIdx < 6 {
			var word uint32
			var ok bool

			if p.useICache && p.cachedFetchStage != nil {
				word, ok, fetchStall = p.cachedFetchStage.Fetch(fetchPC)
				if fetchStall {
					p.stats.Stalls++
					break
				}
			} else {
				word, ok = p.fetchStage.Fetch(fetchPC)
			}

			if !ok {
				break
			}

			// Branch elimination: unconditional B (not BL) instructions are
			// eliminated at fetch time. They never enter the pipeline.
			if isEliminableBranch(word) {
				_, uncondTarget := isUnconditionalBranch(word, fetchPC)
				fetchPC = uncondTarget
				p.stats.EliminatedBranches++
				// Don't create IFID entry - branch is eliminated
				// Continue fetching from target without advancing slotIdx
				continue
			}

			if slotIdx == 0 {
				isUncondBranch, uncondTarget := isUnconditionalBranch(word, fetchPC)
				pred := p.branchPredictor.Predict(fetchPC)
				earlyResolved := false
				if isUncondBranch {
					pred.Taken = true
					pred.Target = uncondTarget
					pred.TargetKnown = true
					earlyResolved = true
				}
				enrichPredictionWithEncodedTarget(&pred, word, fetchPC)
				nextIFID = IFIDRegister{
					Valid:           true,
					PC:              fetchPC,
					InstructionWord: word,
					PredictedTaken:  pred.Taken,
					PredictedTarget: pred.Target,
					EarlyResolved:   earlyResolved,
				}
				if pred.Taken && pred.TargetKnown {
					fetchPC = pred.Target
					slotIdx++
					continue
				}
			} else {
				isUncondBranch, uncondTarget := isUnconditionalBranch(word, fetchPC)
				pred := p.branchPredictor.Predict(fetchPC)
				earlyResolved := false
				if isUncondBranch {
					pred.Taken = true
					pred.Target = uncondTarget
					pred.TargetKnown = true
					earlyResolved = true
				}
				enrichPredictionWithEncodedTarget(&pred, word, fetchPC)
				switch slotIdx {
				case 1:
					nextIFID2 = SecondaryIFIDRegister{Valid: true, PC: fetchPC, InstructionWord: word, PredictedTaken: pred.Taken, PredictedTarget: pred.Target, EarlyResolved: earlyResolved}
				case 2:
					nextIFID3 = TertiaryIFIDRegister{Valid: true, PC: fetchPC, InstructionWord: word, PredictedTaken: pred.Taken, PredictedTarget: pred.Target, EarlyResolved: earlyResolved}
				case 3:
					nextIFID4 = QuaternaryIFIDRegister{Valid: true, PC: fetchPC, InstructionWord: word, PredictedTaken: pred.Taken, PredictedTarget: pred.Target, EarlyResolved: earlyResolved}
				case 4:
					nextIFID5 = QuinaryIFIDRegister{Valid: true, PC: fetchPC, InstructionWord: word, PredictedTaken: pred.Taken, PredictedTarget: pred.Target, EarlyResolved: earlyResolved}
				case 5:
					nextIFID6 = SenaryIFIDRegister{Valid: true, PC: fetchPC, InstructionWord: word, PredictedTaken: pred.Taken, PredictedTarget: pred.Target, EarlyResolved: earlyResolved}
				}
				if pred.Taken && pred.TargetKnown {
					fetchPC = pred.Target
					slotIdx++
					continue
				}
			}
			fetchPC += 4
			slotIdx++
		}
		p.pc = fetchPC

		if fetchStall {
			nextIFID = p.ifid
			nextIFID2 = p.ifid2
			nextIFID3 = p.ifid3
			nextIFID4 = p.ifid4
			nextIFID5 = p.ifid5
			nextIFID6 = p.ifid6
			nextIDEX = p.idex
			nextIDEX2 = p.idex2
			nextIDEX3 = p.idex3
			nextIDEX4 = p.idex4
			nextIDEX5 = p.idex5
			nextIDEX6 = p.idex6
			nextEXMEM = p.exmem
			nextEXMEM2 = p.exmem2
			nextEXMEM3 = p.exmem3
			nextEXMEM4 = p.exmem4
			nextEXMEM5 = p.exmem5
			nextEXMEM6 = p.exmem6
		}
	} else if (stallResult.StallIF || memStall || execStall) && !stallResult.FlushIF {
		nextIFID = p.ifid
		nextIFID2 = p.ifid2
		nextIFID3 = p.ifid3
		nextIFID4 = p.ifid4
		nextIFID5 = p.ifid5
		nextIFID6 = p.ifid6
		p.stats.Stalls++
	}

	// Latch all pipeline registers
	if !memStall && !fetchStall {
		p.memwb = nextMEMWB
		p.memwb2 = nextMEMWB2
		p.memwb3 = nextMEMWB3
		p.memwb4 = nextMEMWB4
		p.memwb5 = nextMEMWB5
		p.memwb6 = nextMEMWB6
	} else {
		p.memwb.Clear()
		p.memwb2.Clear()
		p.memwb3.Clear()
		p.memwb4.Clear()
		p.memwb5.Clear()
		p.memwb6.Clear()
	}
	if !execStall && !memStall {
		p.exmem = nextEXMEM
		p.exmem2 = nextEXMEM2
		p.exmem3 = nextEXMEM3
		p.exmem4 = nextEXMEM4
		p.exmem5 = nextEXMEM5
		p.exmem6 = nextEXMEM6
	}
	if stallResult.InsertBubbleEX && !execStall && !memStall {
		p.idex.Clear()
		p.idex2.Clear()
		p.idex3.Clear()
		p.idex4.Clear()
		p.idex5.Clear()
		p.idex6.Clear()
	} else if !memStall {
		p.idex = nextIDEX
		p.idex2 = nextIDEX2
		p.idex3 = nextIDEX3
		p.idex4 = nextIDEX4
		p.idex5 = nextIDEX5
		p.idex6 = nextIDEX6
	}
	p.ifid = nextIFID
	p.ifid2 = nextIFID2
	p.ifid3 = nextIFID3
	p.ifid4 = nextIFID4
	p.ifid5 = nextIFID5
	p.ifid6 = nextIFID6
}

// collectPendingFetchInstructions6 returns unissued instructions for 6-wide.
// Uses a fixed-size array to avoid heap allocation per tick.
func (p *Pipeline) collectPendingFetchInstructions6(issueCount int) ([8]pendingFetchInst, int) {
	var allFetched [8]pendingFetchInst
	count := 0

	if p.ifid.Valid {
		allFetched[count] = pendingFetchInst{PC: p.ifid.PC, Word: p.ifid.InstructionWord}
		count++
	}
	if p.ifid2.Valid {
		allFetched[count] = pendingFetchInst{PC: p.ifid2.PC, Word: p.ifid2.InstructionWord}
		count++
	}
	if p.ifid3.Valid {
		allFetched[count] = pendingFetchInst{PC: p.ifid3.PC, Word: p.ifid3.InstructionWord}
		count++
	}
	if p.ifid4.Valid {
		allFetched[count] = pendingFetchInst{PC: p.ifid4.PC, Word: p.ifid4.InstructionWord}
		count++
	}
	if p.ifid5.Valid {
		allFetched[count] = pendingFetchInst{PC: p.ifid5.PC, Word: p.ifid5.InstructionWord}
		count++
	}
	if p.ifid6.Valid {
		allFetched[count] = pendingFetchInst{PC: p.ifid6.PC, Word: p.ifid6.InstructionWord}
		count++
	}

	pendingCount := 0
	if issueCount < count {
		pendingCount = count - issueCount
		for i := 0; i < pendingCount; i++ {
			allFetched[i] = allFetched[i+issueCount]
		}
	}

	return allFetched, pendingCount
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
	if p.cachedMemoryStage2 != nil {
		p.cachedMemoryStage2.Reset()
	}
	if p.cachedMemoryStage3 != nil {
		p.cachedMemoryStage3.Reset()
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

// tickOctupleIssue executes one cycle with 8-wide superscalar support.
// This extends 6-wide to match the Apple M2's 8-wide decode bandwidth.
func (p *Pipeline) tickOctupleIssue() {
	// Stage 5: Writeback (all 8 slots using WritebackSlot helper)
	savedMEMWB := p.memwb
	if p.writebackStage.WritebackSlot(&p.memwb) {
		p.stats.Instructions++
		// Fused CMP+B.cond counts as 2 instructions
		if p.memwb.IsFused {
			p.stats.Instructions++
		}
	}

	// Writeback secondary slot
	if p.writebackStage.WritebackSlot(&p.memwb2) {
		p.stats.Instructions++
	}

	// Writeback tertiary slot (using WritebackSlot helper)
	if p.writebackStage.WritebackSlot(&p.memwb3) {
		p.stats.Instructions++
	}

	// Writeback quaternary slot (using WritebackSlot helper)
	if p.writebackStage.WritebackSlot(&p.memwb4) {
		p.stats.Instructions++
	}

	// Writeback quinary slot (using WritebackSlot helper)
	if p.writebackStage.WritebackSlot(&p.memwb5) {
		p.stats.Instructions++
	}

	// Writeback senary slot (using WritebackSlot helper)
	if p.writebackStage.WritebackSlot(&p.memwb6) {
		p.stats.Instructions++
	}

	// Writeback septenary slot (using WritebackSlot helper)
	if p.writebackStage.WritebackSlot(&p.memwb7) {
		p.stats.Instructions++
	}

	// Writeback octonary slot (using WritebackSlot helper)
	if p.writebackStage.WritebackSlot(&p.memwb8) {
		p.stats.Instructions++
	}

	// Stage 4: Memory (primary slot only - single memory port)
	var nextMEMWB MEMWBRegister
	var nextMEMWB2 SecondaryMEMWBRegister
	var nextMEMWB3 TertiaryMEMWBRegister
	var nextMEMWB4 QuaternaryMEMWBRegister
	var nextMEMWB5 QuinaryMEMWBRegister
	var nextMEMWB6 SenaryMEMWBRegister
	var nextMEMWB7 SeptenaryMEMWBRegister
	var nextMEMWB8 OctonaryMEMWBRegister
	memStall := false

	if p.exmem.Valid {
		if p.exmem.Inst != nil && p.exmem.Inst.Op == insts.OpSVC {
			if p.syscallHandler != nil {
				result := p.syscallHandler.Handle()
				if result.Exited {
					p.halted = true
					p.exitCode = result.ExitCode
				}
			}
		}

		var memResult MemoryResult
		if p.useDCache && p.cachedMemoryStage != nil {
			memResult, memStall = p.cachedMemoryStage.Access(&p.exmem)
			if memStall {
				p.stats.MemStalls++
			}
		} else {
			// Non-cached path: immediate access (no stall).
			// Without cache simulation, memory is a direct array lookup.
			// Pipeline issue rules already enforce ordering constraints.
			if p.exmem.MemRead || p.exmem.MemWrite {
				p.memPending = false
				memResult = p.memoryStage.Access(&p.exmem)
			} else {
				p.memPending = false
			}
		}

		if !memStall {
			nextMEMWB = MEMWBRegister{
				Valid:     true,
				PC:        p.exmem.PC,
				Inst:      p.exmem.Inst,
				ALUResult: p.exmem.ALUResult,
				MemData:   memResult.MemData,
				Rd:        p.exmem.Rd,
				RegWrite:  p.exmem.RegWrite,
				MemToReg:  p.exmem.MemToReg,
				IsFused:   p.exmem.IsFused,
			}
		}
	}

	// Secondary slot memory (memory port 2) — tick in parallel with port 1
	var memStall2 bool
	var memResult2 MemoryResult
	if p.exmem2.Valid {
		if p.exmem2.MemRead || p.exmem2.MemWrite {
			memResult2, memStall2 = p.accessSecondaryMem(&p.exmem2)
		}
	}

	// Tertiary slot memory (memory port 3) — tick in parallel with ports 1 & 2
	var memStall3 bool
	var memResult3 MemoryResult
	if p.exmem3.Valid {
		if p.exmem3.MemRead || p.exmem3.MemWrite {
			memResult3, memStall3 = p.accessTertiaryMem(&p.exmem3)
		}
	}

	// Combine stall signals: pipeline stalls if ANY memory port is stalling.
	// Track whether primary port already counted this stall cycle.
	primaryStalled := memStall
	memStall = memStall || memStall2 || memStall3
	if memStall && !primaryStalled {
		p.stats.MemStalls++
	}

	if p.exmem2.Valid && !memStall {
		nextMEMWB2 = SecondaryMEMWBRegister{
			Valid:     true,
			PC:        p.exmem2.PC,
			Inst:      p.exmem2.Inst,
			ALUResult: p.exmem2.ALUResult,
			MemData:   memResult2.MemData,
			Rd:        p.exmem2.Rd,
			RegWrite:  p.exmem2.RegWrite,
			MemToReg:  p.exmem2.MemToReg,
		}
	}

	if p.exmem3.Valid && !memStall {
		nextMEMWB3 = TertiaryMEMWBRegister{
			Valid:     true,
			PC:        p.exmem3.PC,
			Inst:      p.exmem3.Inst,
			ALUResult: p.exmem3.ALUResult,
			MemData:   memResult3.MemData,
			Rd:        p.exmem3.Rd,
			RegWrite:  p.exmem3.RegWrite,
			MemToReg:  p.exmem3.MemToReg,
		}
	}

	// Quaternary slot memory (ALU results only, no memory port)
	if p.exmem4.Valid && !memStall {
		nextMEMWB4 = QuaternaryMEMWBRegister{
			Valid:     true,
			PC:        p.exmem4.PC,
			Inst:      p.exmem4.Inst,
			ALUResult: p.exmem4.ALUResult,
			MemData:   0,
			Rd:        p.exmem4.Rd,
			RegWrite:  p.exmem4.RegWrite,
			MemToReg:  false,
		}
	}

	// Quinary slot memory (ALU results only, no memory port)
	if p.exmem5.Valid && !memStall {
		nextMEMWB5 = QuinaryMEMWBRegister{
			Valid:     true,
			PC:        p.exmem5.PC,
			Inst:      p.exmem5.Inst,
			ALUResult: p.exmem5.ALUResult,
			MemData:   0,
			Rd:        p.exmem5.Rd,
			RegWrite:  p.exmem5.RegWrite,
			MemToReg:  false,
		}
	}

	// Senary slot memory (ALU results only, no memory port)
	if p.exmem6.Valid && !memStall {
		nextMEMWB6 = SenaryMEMWBRegister{
			Valid:     true,
			PC:        p.exmem6.PC,
			Inst:      p.exmem6.Inst,
			ALUResult: p.exmem6.ALUResult,
			MemData:   0,
			Rd:        p.exmem6.Rd,
			RegWrite:  p.exmem6.RegWrite,
			MemToReg:  false,
		}
	}

	// Septenary slot memory (ALU results only, no memory port)
	if p.exmem7.Valid && !memStall {
		nextMEMWB7 = SeptenaryMEMWBRegister{
			Valid:     true,
			PC:        p.exmem7.PC,
			Inst:      p.exmem7.Inst,
			ALUResult: p.exmem7.ALUResult,
			MemData:   0,
			Rd:        p.exmem7.Rd,
			RegWrite:  p.exmem7.RegWrite,
			MemToReg:  false,
		}
	}

	// Octonary slot memory (ALU results only, no memory port)
	if p.exmem8.Valid && !memStall {
		nextMEMWB8 = OctonaryMEMWBRegister{
			Valid:     true,
			PC:        p.exmem8.PC,
			Inst:      p.exmem8.Inst,
			ALUResult: p.exmem8.ALUResult,
			MemData:   0,
			Rd:        p.exmem8.Rd,
			RegWrite:  p.exmem8.RegWrite,
			MemToReg:  false,
		}
	}

	// Stage 3: Execute (all 8 slots)
	var nextEXMEM EXMEMRegister
	var nextEXMEM2 SecondaryEXMEMRegister
	var nextEXMEM3 TertiaryEXMEMRegister
	var nextEXMEM4 QuaternaryEXMEMRegister
	var nextEXMEM5 QuinaryEXMEMRegister
	var nextEXMEM6 SenaryEXMEMRegister
	var nextEXMEM7 SeptenaryEXMEMRegister
	var nextEXMEM8 OctonaryEXMEMRegister
	execStall := false

	// Detect forwarding for primary slot
	forwarding := p.hazardUnit.DetectForwarding(&p.idex, &p.exmem, &p.memwb)

	// Execute primary slot
	if p.idex.Valid && !memStall {
		if p.latencyTable != nil && p.exLatency == 0 {
			if p.useDCache && p.latencyTable.IsLoadOp(p.idex.Inst) {
				p.exLatency = minCacheLoadLatency
			} else {
				p.exLatency = p.latencyTable.GetLatency(p.idex.Inst)
			}
		}

		if p.exLatency > 0 {
			p.exLatency--
		}

		if p.exLatency > 0 {
			execStall = true
			p.stats.ExecStalls++
		} else {
			rnValue := p.hazardUnit.GetForwardedValue(
				forwarding.ForwardRn, p.idex.RnValue, &p.exmem, &savedMEMWB)
			rmValue := p.hazardUnit.GetForwardedValue(
				forwarding.ForwardRm, p.idex.RmValue, &p.exmem, &savedMEMWB)

			// Forward from all secondary pipeline stages to primary slot
			rnValue = p.forwardFromAllSlots(p.idex.Rn, rnValue)
			rmValue = p.forwardFromAllSlots(p.idex.Rm, rmValue)

			// Check for PSTATE flag forwarding from all EXMEM stages (octuple-issue).
			// CMP can execute in any slot, and B.cond in slot 0 needs the flags.
			forwardFlags := false
			var fwdN, fwdZ, fwdC, fwdV bool
			if p.idex.Inst != nil && p.idex.Inst.Op == insts.OpBCond && !p.idex.IsFused {
				if p.exmem.Valid && p.exmem.SetsFlags {
					forwardFlags = true
					fwdN = p.exmem.FlagN
					fwdZ = p.exmem.FlagZ
					fwdC = p.exmem.FlagC
					fwdV = p.exmem.FlagV
				} else if p.exmem2.Valid && p.exmem2.SetsFlags {
					forwardFlags = true
					fwdN = p.exmem2.FlagN
					fwdZ = p.exmem2.FlagZ
					fwdC = p.exmem2.FlagC
					fwdV = p.exmem2.FlagV
				} else if p.exmem3.Valid && p.exmem3.SetsFlags {
					forwardFlags = true
					fwdN = p.exmem3.FlagN
					fwdZ = p.exmem3.FlagZ
					fwdC = p.exmem3.FlagC
					fwdV = p.exmem3.FlagV
				} else if p.exmem4.Valid && p.exmem4.SetsFlags {
					forwardFlags = true
					fwdN = p.exmem4.FlagN
					fwdZ = p.exmem4.FlagZ
					fwdC = p.exmem4.FlagC
					fwdV = p.exmem4.FlagV
				} else if p.exmem5.Valid && p.exmem5.SetsFlags {
					forwardFlags = true
					fwdN = p.exmem5.FlagN
					fwdZ = p.exmem5.FlagZ
					fwdC = p.exmem5.FlagC
					fwdV = p.exmem5.FlagV
				} else if p.exmem6.Valid && p.exmem6.SetsFlags {
					forwardFlags = true
					fwdN = p.exmem6.FlagN
					fwdZ = p.exmem6.FlagZ
					fwdC = p.exmem6.FlagC
					fwdV = p.exmem6.FlagV
				} else if p.exmem7.Valid && p.exmem7.SetsFlags {
					forwardFlags = true
					fwdN = p.exmem7.FlagN
					fwdZ = p.exmem7.FlagZ
					fwdC = p.exmem7.FlagC
					fwdV = p.exmem7.FlagV
				} else if p.exmem8.Valid && p.exmem8.SetsFlags {
					forwardFlags = true
					fwdN = p.exmem8.FlagN
					fwdZ = p.exmem8.FlagZ
					fwdC = p.exmem8.FlagC
					fwdV = p.exmem8.FlagV
				}
			}

			execResult := p.executeStage.ExecuteWithFlags(&p.idex, rnValue, rmValue,
				forwardFlags, fwdN, fwdZ, fwdC, fwdV)

			storeValue := execResult.StoreValue
			if p.idex.MemWrite {
				rdValue := p.regFile.ReadReg(p.idex.Rd)
				storeValue = p.hazardUnit.GetForwardedValue(
					forwarding.ForwardRd, rdValue, &p.exmem, &savedMEMWB)
			}

			nextEXMEM = EXMEMRegister{
				Valid:      true,
				PC:         p.idex.PC,
				Inst:       p.idex.Inst,
				ALUResult:  execResult.ALUResult,
				StoreValue: storeValue,
				Rd:         p.idex.Rd,
				MemRead:    p.idex.MemRead,
				MemWrite:   p.idex.MemWrite,
				RegWrite:   p.idex.RegWrite,
				MemToReg:   p.idex.MemToReg,
				IsFused:    p.idex.IsFused,
				// Store computed flags for forwarding to dependent B.cond
				SetsFlags: execResult.SetsFlags,
				FlagN:     execResult.FlagN,
				FlagZ:     execResult.FlagZ,
				FlagC:     execResult.FlagC,
				FlagV:     execResult.FlagV,
			}

			// Branch prediction verification for primary slot
			if p.idex.IsBranch {
				actualTaken := execResult.BranchTaken
				actualTarget := execResult.BranchTarget

				p.stats.BranchPredictions++

				predictedTaken := p.idex.PredictedTaken
				predictedTarget := p.idex.PredictedTarget
				earlyResolved := p.idex.EarlyResolved

				wasMispredicted := false
				if actualTaken {
					if !predictedTaken {
						wasMispredicted = true
					} else if predictedTarget != actualTarget {
						wasMispredicted = true
					}
				} else {
					if predictedTaken {
						wasMispredicted = true
					}
				}

				if earlyResolved && actualTaken {
					wasMispredicted = false
				}

				p.branchPredictor.Update(p.idex.PC, actualTaken, actualTarget)

				if wasMispredicted {
					p.stats.BranchMispredictions++
					branchTarget := actualTarget
					if !actualTaken {
						branchTarget = p.idex.PC + 4
					}
					p.pc = branchTarget
					p.flushAllIFID()
					p.flushAllIDEX()
					p.stats.Flushes++

					// Latch results and return early
					if !memStall {
						p.memwb = nextMEMWB
						p.memwb2 = nextMEMWB2
						p.memwb3 = nextMEMWB3
						p.memwb4 = nextMEMWB4
						p.memwb5 = nextMEMWB5
						p.memwb6 = nextMEMWB6
						p.memwb7 = nextMEMWB7
						p.memwb8 = nextMEMWB8
						p.exmem = nextEXMEM
						p.exmem2.Clear()
						p.exmem3.Clear()
						p.exmem4.Clear()
						p.exmem5.Clear()
						p.exmem6.Clear()
						p.exmem7.Clear()
						p.exmem8.Clear()
					}
					return
				}
				p.stats.BranchCorrect++
			}
		}
	}

	// Execute secondary slot
	if p.idex2.Valid && !memStall && !execStall {
		if p.latencyTable != nil && p.exLatency2 == 0 {
			p.exLatency2 = p.latencyTable.GetLatency(p.idex2.Inst)
		}
		if p.exLatency2 > 0 {
			p.exLatency2--
		}
		if p.exLatency2 == 0 {
			rnValue := p.forwardFromAllSlots(p.idex2.Rn, p.idex2.RnValue)
			rmValue := p.forwardFromAllSlots(p.idex2.Rm, p.idex2.RmValue)
			if nextEXMEM.Valid && nextEXMEM.RegWrite && nextEXMEM.Rd != 31 {
				if p.idex2.Rn == nextEXMEM.Rd {
					rnValue = nextEXMEM.ALUResult
				}
				if p.idex2.Rm == nextEXMEM.Rd {
					rmValue = nextEXMEM.ALUResult
				}
			}
			// Same-cycle PSTATE flag forwarding for B.cond in slot 2
			forwardFlags2 := false
			var fwdN2, fwdZ2, fwdC2, fwdV2 bool
			if p.idex2.Inst != nil && p.idex2.Inst.Op == insts.OpBCond {
				// Check same-cycle: slot 0 (nextEXMEM)
				if nextEXMEM.Valid && nextEXMEM.SetsFlags {
					forwardFlags2 = true
					fwdN2, fwdZ2, fwdC2, fwdV2 = nextEXMEM.FlagN, nextEXMEM.FlagZ, nextEXMEM.FlagC, nextEXMEM.FlagV
				}
				// Check previous cycle EXMEM registers
				if !forwardFlags2 {
					if p.exmem.Valid && p.exmem.SetsFlags {
						forwardFlags2 = true
						fwdN2, fwdZ2, fwdC2, fwdV2 = p.exmem.FlagN, p.exmem.FlagZ, p.exmem.FlagC, p.exmem.FlagV
					} else if p.exmem2.Valid && p.exmem2.SetsFlags {
						forwardFlags2 = true
						fwdN2, fwdZ2, fwdC2, fwdV2 = p.exmem2.FlagN, p.exmem2.FlagZ, p.exmem2.FlagC, p.exmem2.FlagV
					}
				}
			}

			idex2 := p.idex2.toIDEX()
			execResult := p.executeStage.ExecuteWithFlags(&idex2, rnValue, rmValue,
				forwardFlags2, fwdN2, fwdZ2, fwdC2, fwdV2)
			nextEXMEM2 = SecondaryEXMEMRegister{
				Valid:      true,
				PC:         p.idex2.PC,
				Inst:       p.idex2.Inst,
				ALUResult:  execResult.ALUResult,
				StoreValue: execResult.StoreValue,
				Rd:         p.idex2.Rd,
				MemRead:    p.idex2.MemRead,
				MemWrite:   p.idex2.MemWrite,
				RegWrite:   p.idex2.RegWrite,
				MemToReg:   p.idex2.MemToReg,
				SetsFlags:  execResult.SetsFlags,
				FlagN:      execResult.FlagN,
				FlagZ:      execResult.FlagZ,
				FlagC:      execResult.FlagC,
				FlagV:      execResult.FlagV,
			}

			// Branch prediction verification for secondary slot (idex2)
			if p.idex2.IsBranch {
				actualTaken := execResult.BranchTaken
				actualTarget := execResult.BranchTarget

				p.stats.BranchPredictions++

				predictedTaken := p.idex2.PredictedTaken
				predictedTarget := p.idex2.PredictedTarget
				earlyResolved := p.idex2.EarlyResolved

				wasMispredicted := false
				if actualTaken {
					if !predictedTaken {
						wasMispredicted = true
					} else if predictedTarget != actualTarget {
						wasMispredicted = true
					}
				} else {
					if predictedTaken {
						wasMispredicted = true
					}
				}

				if earlyResolved && actualTaken {
					wasMispredicted = false
				}

				p.branchPredictor.Update(p.idex2.PC, actualTaken, actualTarget)

				if wasMispredicted {
					p.stats.BranchMispredictions++
					branchTarget := actualTarget
					if !actualTaken {
						branchTarget = p.idex2.PC + 4
					}
					p.pc = branchTarget
					p.flushAllIFID()
					p.flushAllIDEX()
					p.stats.Flushes++

					if !memStall {
						p.memwb = nextMEMWB
						p.memwb2 = nextMEMWB2
						p.memwb3 = nextMEMWB3
						p.memwb4 = nextMEMWB4
						p.memwb5 = nextMEMWB5
						p.memwb6 = nextMEMWB6
						p.memwb7 = nextMEMWB7
						p.memwb8 = nextMEMWB8
						p.exmem = nextEXMEM
						p.exmem2.Clear()
						p.exmem3.Clear()
						p.exmem4.Clear()
						p.exmem5.Clear()
						p.exmem6.Clear()
						p.exmem7.Clear()
						p.exmem8.Clear()
					}
					return
				}
				p.stats.BranchCorrect++
			}
		}
	}

	// Execute tertiary slot
	if p.idex3.Valid && !memStall && !execStall {
		if p.latencyTable != nil && p.exLatency3 == 0 {
			p.exLatency3 = p.latencyTable.GetLatency(p.idex3.Inst)
		}
		if p.exLatency3 > 0 {
			p.exLatency3--
		}
		if p.exLatency3 == 0 {
			rnValue := p.forwardFromAllSlots(p.idex3.Rn, p.idex3.RnValue)
			rmValue := p.forwardFromAllSlots(p.idex3.Rm, p.idex3.RmValue)
			if nextEXMEM.Valid && nextEXMEM.RegWrite && nextEXMEM.Rd != 31 {
				if p.idex3.Rn == nextEXMEM.Rd {
					rnValue = nextEXMEM.ALUResult
				}
				if p.idex3.Rm == nextEXMEM.Rd {
					rmValue = nextEXMEM.ALUResult
				}
			}
			if nextEXMEM2.Valid && nextEXMEM2.RegWrite && nextEXMEM2.Rd != 31 {
				if p.idex3.Rn == nextEXMEM2.Rd {
					rnValue = nextEXMEM2.ALUResult
				}
				if p.idex3.Rm == nextEXMEM2.Rd {
					rmValue = nextEXMEM2.ALUResult
				}
			}
			// Same-cycle PSTATE flag forwarding for B.cond in slot 3
			forwardFlags3 := false
			var fwdN3, fwdZ3, fwdC3, fwdV3 bool
			if p.idex3.Inst != nil && p.idex3.Inst.Op == insts.OpBCond {
				// Check same-cycle: slots 0-1 (nextEXMEM, nextEXMEM2)
				if nextEXMEM2.Valid && nextEXMEM2.SetsFlags {
					forwardFlags3 = true
					fwdN3, fwdZ3, fwdC3, fwdV3 = nextEXMEM2.FlagN, nextEXMEM2.FlagZ, nextEXMEM2.FlagC, nextEXMEM2.FlagV
				} else if nextEXMEM.Valid && nextEXMEM.SetsFlags {
					forwardFlags3 = true
					fwdN3, fwdZ3, fwdC3, fwdV3 = nextEXMEM.FlagN, nextEXMEM.FlagZ, nextEXMEM.FlagC, nextEXMEM.FlagV
				}
				// Check previous cycle EXMEM registers
				if !forwardFlags3 {
					if p.exmem.Valid && p.exmem.SetsFlags {
						forwardFlags3 = true
						fwdN3, fwdZ3, fwdC3, fwdV3 = p.exmem.FlagN, p.exmem.FlagZ, p.exmem.FlagC, p.exmem.FlagV
					} else if p.exmem2.Valid && p.exmem2.SetsFlags {
						forwardFlags3 = true
						fwdN3, fwdZ3, fwdC3, fwdV3 = p.exmem2.FlagN, p.exmem2.FlagZ, p.exmem2.FlagC, p.exmem2.FlagV
					} else if p.exmem3.Valid && p.exmem3.SetsFlags {
						forwardFlags3 = true
						fwdN3, fwdZ3, fwdC3, fwdV3 = p.exmem3.FlagN, p.exmem3.FlagZ, p.exmem3.FlagC, p.exmem3.FlagV
					}
				}
			}

			idex3 := p.idex3.toIDEX()
			execResult := p.executeStage.ExecuteWithFlags(&idex3, rnValue, rmValue,
				forwardFlags3, fwdN3, fwdZ3, fwdC3, fwdV3)
			nextEXMEM3 = TertiaryEXMEMRegister{
				Valid:      true,
				PC:         p.idex3.PC,
				Inst:       p.idex3.Inst,
				ALUResult:  execResult.ALUResult,
				StoreValue: execResult.StoreValue,
				Rd:         p.idex3.Rd,
				MemRead:    p.idex3.MemRead,
				MemWrite:   p.idex3.MemWrite,
				RegWrite:   p.idex3.RegWrite,
				MemToReg:   p.idex3.MemToReg,
				SetsFlags:  execResult.SetsFlags,
				FlagN:      execResult.FlagN,
				FlagZ:      execResult.FlagZ,
				FlagC:      execResult.FlagC,
				FlagV:      execResult.FlagV,
			}

			// Branch prediction verification for tertiary slot (idex3)
			if p.idex3.IsBranch {
				actualTaken := execResult.BranchTaken
				actualTarget := execResult.BranchTarget

				p.stats.BranchPredictions++

				predictedTaken := p.idex3.PredictedTaken
				predictedTarget := p.idex3.PredictedTarget
				earlyResolved := p.idex3.EarlyResolved

				wasMispredicted := false
				if actualTaken {
					if !predictedTaken {
						wasMispredicted = true
					} else if predictedTarget != actualTarget {
						wasMispredicted = true
					}
				} else {
					if predictedTaken {
						wasMispredicted = true
					}
				}

				if earlyResolved && actualTaken {
					wasMispredicted = false
				}

				p.branchPredictor.Update(p.idex3.PC, actualTaken, actualTarget)

				if wasMispredicted {
					p.stats.BranchMispredictions++
					branchTarget := actualTarget
					if !actualTaken {
						branchTarget = p.idex3.PC + 4
					}
					p.pc = branchTarget
					p.flushAllIFID()
					p.flushAllIDEX()
					p.stats.Flushes++

					if !memStall {
						p.memwb = nextMEMWB
						p.memwb2 = nextMEMWB2
						p.memwb3 = nextMEMWB3
						p.memwb4 = nextMEMWB4
						p.memwb5 = nextMEMWB5
						p.memwb6 = nextMEMWB6
						p.memwb7 = nextMEMWB7
						p.memwb8 = nextMEMWB8
						p.exmem = nextEXMEM
						p.exmem2 = nextEXMEM2
						p.exmem3.Clear()
						p.exmem4.Clear()
						p.exmem5.Clear()
						p.exmem6.Clear()
						p.exmem7.Clear()
						p.exmem8.Clear()
					}
					return
				}
				p.stats.BranchCorrect++
			}
		}
	}

	// Execute quaternary slot
	if p.idex4.Valid && !memStall && !execStall {
		if p.latencyTable != nil && p.exLatency4 == 0 {
			p.exLatency4 = p.latencyTable.GetLatency(p.idex4.Inst)
		}
		if p.exLatency4 > 0 {
			p.exLatency4--
		}
		if p.exLatency4 == 0 {
			rnValue := p.forwardFromAllSlots(p.idex4.Rn, p.idex4.RnValue)
			rmValue := p.forwardFromAllSlots(p.idex4.Rm, p.idex4.RmValue)
			if nextEXMEM.Valid && nextEXMEM.RegWrite && nextEXMEM.Rd != 31 {
				if p.idex4.Rn == nextEXMEM.Rd {
					rnValue = nextEXMEM.ALUResult
				}
				if p.idex4.Rm == nextEXMEM.Rd {
					rmValue = nextEXMEM.ALUResult
				}
			}
			if nextEXMEM2.Valid && nextEXMEM2.RegWrite && nextEXMEM2.Rd != 31 {
				if p.idex4.Rn == nextEXMEM2.Rd {
					rnValue = nextEXMEM2.ALUResult
				}
				if p.idex4.Rm == nextEXMEM2.Rd {
					rmValue = nextEXMEM2.ALUResult
				}
			}
			if nextEXMEM3.Valid && nextEXMEM3.RegWrite && nextEXMEM3.Rd != 31 {
				if p.idex4.Rn == nextEXMEM3.Rd {
					rnValue = nextEXMEM3.ALUResult
				}
				if p.idex4.Rm == nextEXMEM3.Rd {
					rmValue = nextEXMEM3.ALUResult
				}
			}
			// Same-cycle PSTATE flag forwarding for B.cond in slot 4
			forwardFlags4 := false
			var fwdN4, fwdZ4, fwdC4, fwdV4 bool
			if p.idex4.Inst != nil && p.idex4.Inst.Op == insts.OpBCond {
				// Check same-cycle: slots 0-2
				if nextEXMEM3.Valid && nextEXMEM3.SetsFlags {
					forwardFlags4 = true
					fwdN4, fwdZ4, fwdC4, fwdV4 = nextEXMEM3.FlagN, nextEXMEM3.FlagZ, nextEXMEM3.FlagC, nextEXMEM3.FlagV
				} else if nextEXMEM2.Valid && nextEXMEM2.SetsFlags {
					forwardFlags4 = true
					fwdN4, fwdZ4, fwdC4, fwdV4 = nextEXMEM2.FlagN, nextEXMEM2.FlagZ, nextEXMEM2.FlagC, nextEXMEM2.FlagV
				} else if nextEXMEM.Valid && nextEXMEM.SetsFlags {
					forwardFlags4 = true
					fwdN4, fwdZ4, fwdC4, fwdV4 = nextEXMEM.FlagN, nextEXMEM.FlagZ, nextEXMEM.FlagC, nextEXMEM.FlagV
				}
				// Check previous cycle
				if !forwardFlags4 {
					if p.exmem.Valid && p.exmem.SetsFlags {
						forwardFlags4 = true
						fwdN4, fwdZ4, fwdC4, fwdV4 = p.exmem.FlagN, p.exmem.FlagZ, p.exmem.FlagC, p.exmem.FlagV
					}
				}
			}

			idex4 := p.idex4.toIDEX()
			execResult := p.executeStage.ExecuteWithFlags(&idex4, rnValue, rmValue,
				forwardFlags4, fwdN4, fwdZ4, fwdC4, fwdV4)
			nextEXMEM4 = QuaternaryEXMEMRegister{
				Valid:      true,
				PC:         p.idex4.PC,
				Inst:       p.idex4.Inst,
				ALUResult:  execResult.ALUResult,
				StoreValue: execResult.StoreValue,
				Rd:         p.idex4.Rd,
				MemRead:    p.idex4.MemRead,
				MemWrite:   p.idex4.MemWrite,
				RegWrite:   p.idex4.RegWrite,
				MemToReg:   p.idex4.MemToReg,
				SetsFlags:  execResult.SetsFlags,
				FlagN:      execResult.FlagN,
				FlagZ:      execResult.FlagZ,
				FlagC:      execResult.FlagC,
				FlagV:      execResult.FlagV,
			}

			// Branch prediction verification for quaternary slot (idex4)
			if p.idex4.IsBranch {
				actualTaken := execResult.BranchTaken
				actualTarget := execResult.BranchTarget

				p.stats.BranchPredictions++

				predictedTaken := p.idex4.PredictedTaken
				predictedTarget := p.idex4.PredictedTarget
				earlyResolved := p.idex4.EarlyResolved

				wasMispredicted := false
				if actualTaken {
					if !predictedTaken {
						wasMispredicted = true
					} else if predictedTarget != actualTarget {
						wasMispredicted = true
					}
				} else {
					if predictedTaken {
						wasMispredicted = true
					}
				}

				if earlyResolved && actualTaken {
					wasMispredicted = false
				}

				p.branchPredictor.Update(p.idex4.PC, actualTaken, actualTarget)

				if wasMispredicted {
					p.stats.BranchMispredictions++
					branchTarget := actualTarget
					if !actualTaken {
						branchTarget = p.idex4.PC + 4
					}
					p.pc = branchTarget
					p.flushAllIFID()
					p.flushAllIDEX()
					p.stats.Flushes++

					if !memStall {
						p.memwb = nextMEMWB
						p.memwb2 = nextMEMWB2
						p.memwb3 = nextMEMWB3
						p.memwb4 = nextMEMWB4
						p.memwb5 = nextMEMWB5
						p.memwb6 = nextMEMWB6
						p.memwb7 = nextMEMWB7
						p.memwb8 = nextMEMWB8
						p.exmem = nextEXMEM
						p.exmem2 = nextEXMEM2
						p.exmem3 = nextEXMEM3
						p.exmem4.Clear()
						p.exmem5.Clear()
						p.exmem6.Clear()
						p.exmem7.Clear()
						p.exmem8.Clear()
					}
					return
				}
				p.stats.BranchCorrect++
			}
		}
	}

	// Execute quinary slot
	if p.idex5.Valid && !memStall && !execStall {
		if p.latencyTable != nil && p.exLatency5 == 0 {
			p.exLatency5 = p.latencyTable.GetLatency(p.idex5.Inst)
		}
		if p.exLatency5 > 0 {
			p.exLatency5--
		}
		if p.exLatency5 == 0 {
			rnValue := p.forwardFromAllSlots(p.idex5.Rn, p.idex5.RnValue)
			rmValue := p.forwardFromAllSlots(p.idex5.Rm, p.idex5.RmValue)
			if nextEXMEM.Valid && nextEXMEM.RegWrite && nextEXMEM.Rd != 31 {
				if p.idex5.Rn == nextEXMEM.Rd {
					rnValue = nextEXMEM.ALUResult
				}
				if p.idex5.Rm == nextEXMEM.Rd {
					rmValue = nextEXMEM.ALUResult
				}
			}
			if nextEXMEM2.Valid && nextEXMEM2.RegWrite && nextEXMEM2.Rd != 31 {
				if p.idex5.Rn == nextEXMEM2.Rd {
					rnValue = nextEXMEM2.ALUResult
				}
				if p.idex5.Rm == nextEXMEM2.Rd {
					rmValue = nextEXMEM2.ALUResult
				}
			}
			if nextEXMEM3.Valid && nextEXMEM3.RegWrite && nextEXMEM3.Rd != 31 {
				if p.idex5.Rn == nextEXMEM3.Rd {
					rnValue = nextEXMEM3.ALUResult
				}
				if p.idex5.Rm == nextEXMEM3.Rd {
					rmValue = nextEXMEM3.ALUResult
				}
			}
			if nextEXMEM4.Valid && nextEXMEM4.RegWrite && nextEXMEM4.Rd != 31 {
				if p.idex5.Rn == nextEXMEM4.Rd {
					rnValue = nextEXMEM4.ALUResult
				}
				if p.idex5.Rm == nextEXMEM4.Rd {
					rmValue = nextEXMEM4.ALUResult
				}
			}
			// Same-cycle PSTATE flag forwarding for B.cond in slot 5
			forwardFlags5 := false
			var fwdN5, fwdZ5, fwdC5, fwdV5 bool
			if p.idex5.Inst != nil && p.idex5.Inst.Op == insts.OpBCond {
				if nextEXMEM4.Valid && nextEXMEM4.SetsFlags {
					forwardFlags5 = true
					fwdN5, fwdZ5, fwdC5, fwdV5 = nextEXMEM4.FlagN, nextEXMEM4.FlagZ, nextEXMEM4.FlagC, nextEXMEM4.FlagV
				} else if nextEXMEM3.Valid && nextEXMEM3.SetsFlags {
					forwardFlags5 = true
					fwdN5, fwdZ5, fwdC5, fwdV5 = nextEXMEM3.FlagN, nextEXMEM3.FlagZ, nextEXMEM3.FlagC, nextEXMEM3.FlagV
				} else if nextEXMEM2.Valid && nextEXMEM2.SetsFlags {
					forwardFlags5 = true
					fwdN5, fwdZ5, fwdC5, fwdV5 = nextEXMEM2.FlagN, nextEXMEM2.FlagZ, nextEXMEM2.FlagC, nextEXMEM2.FlagV
				} else if nextEXMEM.Valid && nextEXMEM.SetsFlags {
					forwardFlags5 = true
					fwdN5, fwdZ5, fwdC5, fwdV5 = nextEXMEM.FlagN, nextEXMEM.FlagZ, nextEXMEM.FlagC, nextEXMEM.FlagV
				}
				if !forwardFlags5 && p.exmem.Valid && p.exmem.SetsFlags {
					forwardFlags5 = true
					fwdN5, fwdZ5, fwdC5, fwdV5 = p.exmem.FlagN, p.exmem.FlagZ, p.exmem.FlagC, p.exmem.FlagV
				}
			}

			idex5 := p.idex5.toIDEX()
			execResult := p.executeStage.ExecuteWithFlags(&idex5, rnValue, rmValue,
				forwardFlags5, fwdN5, fwdZ5, fwdC5, fwdV5)
			nextEXMEM5 = QuinaryEXMEMRegister{
				Valid:      true,
				PC:         p.idex5.PC,
				Inst:       p.idex5.Inst,
				ALUResult:  execResult.ALUResult,
				StoreValue: execResult.StoreValue,
				Rd:         p.idex5.Rd,
				MemRead:    p.idex5.MemRead,
				MemWrite:   p.idex5.MemWrite,
				RegWrite:   p.idex5.RegWrite,
				MemToReg:   p.idex5.MemToReg,
				SetsFlags:  execResult.SetsFlags,
				FlagN:      execResult.FlagN,
				FlagZ:      execResult.FlagZ,
				FlagC:      execResult.FlagC,
				FlagV:      execResult.FlagV,
			}

			// Branch prediction verification for quinary slot (idex5)
			if p.idex5.IsBranch {
				actualTaken := execResult.BranchTaken
				actualTarget := execResult.BranchTarget

				p.stats.BranchPredictions++

				predictedTaken := p.idex5.PredictedTaken
				predictedTarget := p.idex5.PredictedTarget
				earlyResolved := p.idex5.EarlyResolved

				wasMispredicted := false
				if actualTaken {
					if !predictedTaken {
						wasMispredicted = true
					} else if predictedTarget != actualTarget {
						wasMispredicted = true
					}
				} else {
					if predictedTaken {
						wasMispredicted = true
					}
				}

				if earlyResolved && actualTaken {
					wasMispredicted = false
				}

				p.branchPredictor.Update(p.idex5.PC, actualTaken, actualTarget)

				if wasMispredicted {
					p.stats.BranchMispredictions++
					branchTarget := actualTarget
					if !actualTaken {
						branchTarget = p.idex5.PC + 4
					}
					p.pc = branchTarget
					p.flushAllIFID()
					p.flushAllIDEX()
					p.stats.Flushes++

					if !memStall {
						p.memwb = nextMEMWB
						p.memwb2 = nextMEMWB2
						p.memwb3 = nextMEMWB3
						p.memwb4 = nextMEMWB4
						p.memwb5 = nextMEMWB5
						p.memwb6 = nextMEMWB6
						p.memwb7 = nextMEMWB7
						p.memwb8 = nextMEMWB8
						p.exmem = nextEXMEM
						p.exmem2 = nextEXMEM2
						p.exmem3 = nextEXMEM3
						p.exmem4 = nextEXMEM4
						p.exmem5.Clear()
						p.exmem6.Clear()
						p.exmem7.Clear()
						p.exmem8.Clear()
					}
					return
				}
				p.stats.BranchCorrect++
			}
		}
	}

	// Execute senary slot
	if p.idex6.Valid && !memStall && !execStall {
		if p.latencyTable != nil && p.exLatency6 == 0 {
			p.exLatency6 = p.latencyTable.GetLatency(p.idex6.Inst)
		}
		if p.exLatency6 > 0 {
			p.exLatency6--
		}
		if p.exLatency6 == 0 {
			rnValue := p.forwardFromAllSlots(p.idex6.Rn, p.idex6.RnValue)
			rmValue := p.forwardFromAllSlots(p.idex6.Rm, p.idex6.RmValue)
			if nextEXMEM.Valid && nextEXMEM.RegWrite && nextEXMEM.Rd != 31 {
				if p.idex6.Rn == nextEXMEM.Rd {
					rnValue = nextEXMEM.ALUResult
				}
				if p.idex6.Rm == nextEXMEM.Rd {
					rmValue = nextEXMEM.ALUResult
				}
			}
			if nextEXMEM2.Valid && nextEXMEM2.RegWrite && nextEXMEM2.Rd != 31 {
				if p.idex6.Rn == nextEXMEM2.Rd {
					rnValue = nextEXMEM2.ALUResult
				}
				if p.idex6.Rm == nextEXMEM2.Rd {
					rmValue = nextEXMEM2.ALUResult
				}
			}
			if nextEXMEM3.Valid && nextEXMEM3.RegWrite && nextEXMEM3.Rd != 31 {
				if p.idex6.Rn == nextEXMEM3.Rd {
					rnValue = nextEXMEM3.ALUResult
				}
				if p.idex6.Rm == nextEXMEM3.Rd {
					rmValue = nextEXMEM3.ALUResult
				}
			}
			if nextEXMEM4.Valid && nextEXMEM4.RegWrite && nextEXMEM4.Rd != 31 {
				if p.idex6.Rn == nextEXMEM4.Rd {
					rnValue = nextEXMEM4.ALUResult
				}
				if p.idex6.Rm == nextEXMEM4.Rd {
					rmValue = nextEXMEM4.ALUResult
				}
			}
			if nextEXMEM5.Valid && nextEXMEM5.RegWrite && nextEXMEM5.Rd != 31 {
				if p.idex6.Rn == nextEXMEM5.Rd {
					rnValue = nextEXMEM5.ALUResult
				}
				if p.idex6.Rm == nextEXMEM5.Rd {
					rmValue = nextEXMEM5.ALUResult
				}
			}
			// Same-cycle PSTATE flag forwarding for B.cond in slot 6
			forwardFlags6 := false
			var fwdN6, fwdZ6, fwdC6, fwdV6 bool
			if p.idex6.Inst != nil && p.idex6.Inst.Op == insts.OpBCond {
				if nextEXMEM5.Valid && nextEXMEM5.SetsFlags {
					forwardFlags6 = true
					fwdN6, fwdZ6, fwdC6, fwdV6 = nextEXMEM5.FlagN, nextEXMEM5.FlagZ, nextEXMEM5.FlagC, nextEXMEM5.FlagV
				} else if nextEXMEM4.Valid && nextEXMEM4.SetsFlags {
					forwardFlags6 = true
					fwdN6, fwdZ6, fwdC6, fwdV6 = nextEXMEM4.FlagN, nextEXMEM4.FlagZ, nextEXMEM4.FlagC, nextEXMEM4.FlagV
				} else if nextEXMEM3.Valid && nextEXMEM3.SetsFlags {
					forwardFlags6 = true
					fwdN6, fwdZ6, fwdC6, fwdV6 = nextEXMEM3.FlagN, nextEXMEM3.FlagZ, nextEXMEM3.FlagC, nextEXMEM3.FlagV
				} else if nextEXMEM2.Valid && nextEXMEM2.SetsFlags {
					forwardFlags6 = true
					fwdN6, fwdZ6, fwdC6, fwdV6 = nextEXMEM2.FlagN, nextEXMEM2.FlagZ, nextEXMEM2.FlagC, nextEXMEM2.FlagV
				} else if nextEXMEM.Valid && nextEXMEM.SetsFlags {
					forwardFlags6 = true
					fwdN6, fwdZ6, fwdC6, fwdV6 = nextEXMEM.FlagN, nextEXMEM.FlagZ, nextEXMEM.FlagC, nextEXMEM.FlagV
				}
				if !forwardFlags6 && p.exmem.Valid && p.exmem.SetsFlags {
					forwardFlags6 = true
					fwdN6, fwdZ6, fwdC6, fwdV6 = p.exmem.FlagN, p.exmem.FlagZ, p.exmem.FlagC, p.exmem.FlagV
				}
			}

			idex6 := p.idex6.toIDEX()
			execResult := p.executeStage.ExecuteWithFlags(&idex6, rnValue, rmValue,
				forwardFlags6, fwdN6, fwdZ6, fwdC6, fwdV6)
			nextEXMEM6 = SenaryEXMEMRegister{
				Valid:      true,
				PC:         p.idex6.PC,
				Inst:       p.idex6.Inst,
				ALUResult:  execResult.ALUResult,
				StoreValue: execResult.StoreValue,
				Rd:         p.idex6.Rd,
				MemRead:    p.idex6.MemRead,
				MemWrite:   p.idex6.MemWrite,
				RegWrite:   p.idex6.RegWrite,
				MemToReg:   p.idex6.MemToReg,
				SetsFlags:  execResult.SetsFlags,
				FlagN:      execResult.FlagN,
				FlagZ:      execResult.FlagZ,
				FlagC:      execResult.FlagC,
				FlagV:      execResult.FlagV,
			}

			// Branch prediction verification for senary slot (idex6)
			if p.idex6.IsBranch {
				actualTaken := execResult.BranchTaken
				actualTarget := execResult.BranchTarget

				p.stats.BranchPredictions++

				predictedTaken := p.idex6.PredictedTaken
				predictedTarget := p.idex6.PredictedTarget
				earlyResolved := p.idex6.EarlyResolved

				wasMispredicted := false
				if actualTaken {
					if !predictedTaken {
						wasMispredicted = true
					} else if predictedTarget != actualTarget {
						wasMispredicted = true
					}
				} else {
					if predictedTaken {
						wasMispredicted = true
					}
				}

				if earlyResolved && actualTaken {
					wasMispredicted = false
				}

				p.branchPredictor.Update(p.idex6.PC, actualTaken, actualTarget)

				if wasMispredicted {
					p.stats.BranchMispredictions++
					branchTarget := actualTarget
					if !actualTaken {
						branchTarget = p.idex6.PC + 4
					}
					p.pc = branchTarget
					p.flushAllIFID()
					p.flushAllIDEX()
					p.stats.Flushes++

					if !memStall {
						p.memwb = nextMEMWB
						p.memwb2 = nextMEMWB2
						p.memwb3 = nextMEMWB3
						p.memwb4 = nextMEMWB4
						p.memwb5 = nextMEMWB5
						p.memwb6 = nextMEMWB6
						p.memwb7 = nextMEMWB7
						p.memwb8 = nextMEMWB8
						p.exmem = nextEXMEM
						p.exmem2 = nextEXMEM2
						p.exmem3 = nextEXMEM3
						p.exmem4 = nextEXMEM4
						p.exmem5 = nextEXMEM5
						p.exmem6.Clear()
						p.exmem7.Clear()
						p.exmem8.Clear()
					}
					return
				}
				p.stats.BranchCorrect++
			}
		}
	}

	// Execute septenary slot
	if p.idex7.Valid && !memStall && !execStall {
		if p.latencyTable != nil && p.exLatency7 == 0 {
			p.exLatency7 = p.latencyTable.GetLatency(p.idex7.Inst)
		}
		if p.exLatency7 > 0 {
			p.exLatency7--
		}
		if p.exLatency7 == 0 {
			rnValue := p.forwardFromAllSlots(p.idex7.Rn, p.idex7.RnValue)
			rmValue := p.forwardFromAllSlots(p.idex7.Rm, p.idex7.RmValue)
			if nextEXMEM.Valid && nextEXMEM.RegWrite && nextEXMEM.Rd != 31 {
				if p.idex7.Rn == nextEXMEM.Rd {
					rnValue = nextEXMEM.ALUResult
				}
				if p.idex7.Rm == nextEXMEM.Rd {
					rmValue = nextEXMEM.ALUResult
				}
			}
			if nextEXMEM2.Valid && nextEXMEM2.RegWrite && nextEXMEM2.Rd != 31 {
				if p.idex7.Rn == nextEXMEM2.Rd {
					rnValue = nextEXMEM2.ALUResult
				}
				if p.idex7.Rm == nextEXMEM2.Rd {
					rmValue = nextEXMEM2.ALUResult
				}
			}
			if nextEXMEM3.Valid && nextEXMEM3.RegWrite && nextEXMEM3.Rd != 31 {
				if p.idex7.Rn == nextEXMEM3.Rd {
					rnValue = nextEXMEM3.ALUResult
				}
				if p.idex7.Rm == nextEXMEM3.Rd {
					rmValue = nextEXMEM3.ALUResult
				}
			}
			if nextEXMEM4.Valid && nextEXMEM4.RegWrite && nextEXMEM4.Rd != 31 {
				if p.idex7.Rn == nextEXMEM4.Rd {
					rnValue = nextEXMEM4.ALUResult
				}
				if p.idex7.Rm == nextEXMEM4.Rd {
					rmValue = nextEXMEM4.ALUResult
				}
			}
			if nextEXMEM5.Valid && nextEXMEM5.RegWrite && nextEXMEM5.Rd != 31 {
				if p.idex7.Rn == nextEXMEM5.Rd {
					rnValue = nextEXMEM5.ALUResult
				}
				if p.idex7.Rm == nextEXMEM5.Rd {
					rmValue = nextEXMEM5.ALUResult
				}
			}
			if nextEXMEM6.Valid && nextEXMEM6.RegWrite && nextEXMEM6.Rd != 31 {
				if p.idex7.Rn == nextEXMEM6.Rd {
					rnValue = nextEXMEM6.ALUResult
				}
				if p.idex7.Rm == nextEXMEM6.Rd {
					rmValue = nextEXMEM6.ALUResult
				}
			}
			// Same-cycle PSTATE flag forwarding for B.cond in slot 7
			forwardFlags7 := false
			var fwdN7, fwdZ7, fwdC7, fwdV7 bool
			if p.idex7.Inst != nil && p.idex7.Inst.Op == insts.OpBCond {
				if nextEXMEM6.Valid && nextEXMEM6.SetsFlags {
					forwardFlags7 = true
					fwdN7, fwdZ7, fwdC7, fwdV7 = nextEXMEM6.FlagN, nextEXMEM6.FlagZ, nextEXMEM6.FlagC, nextEXMEM6.FlagV
				} else if nextEXMEM5.Valid && nextEXMEM5.SetsFlags {
					forwardFlags7 = true
					fwdN7, fwdZ7, fwdC7, fwdV7 = nextEXMEM5.FlagN, nextEXMEM5.FlagZ, nextEXMEM5.FlagC, nextEXMEM5.FlagV
				} else if nextEXMEM4.Valid && nextEXMEM4.SetsFlags {
					forwardFlags7 = true
					fwdN7, fwdZ7, fwdC7, fwdV7 = nextEXMEM4.FlagN, nextEXMEM4.FlagZ, nextEXMEM4.FlagC, nextEXMEM4.FlagV
				} else if nextEXMEM3.Valid && nextEXMEM3.SetsFlags {
					forwardFlags7 = true
					fwdN7, fwdZ7, fwdC7, fwdV7 = nextEXMEM3.FlagN, nextEXMEM3.FlagZ, nextEXMEM3.FlagC, nextEXMEM3.FlagV
				} else if nextEXMEM2.Valid && nextEXMEM2.SetsFlags {
					forwardFlags7 = true
					fwdN7, fwdZ7, fwdC7, fwdV7 = nextEXMEM2.FlagN, nextEXMEM2.FlagZ, nextEXMEM2.FlagC, nextEXMEM2.FlagV
				} else if nextEXMEM.Valid && nextEXMEM.SetsFlags {
					forwardFlags7 = true
					fwdN7, fwdZ7, fwdC7, fwdV7 = nextEXMEM.FlagN, nextEXMEM.FlagZ, nextEXMEM.FlagC, nextEXMEM.FlagV
				}
				if !forwardFlags7 && p.exmem.Valid && p.exmem.SetsFlags {
					forwardFlags7 = true
					fwdN7, fwdZ7, fwdC7, fwdV7 = p.exmem.FlagN, p.exmem.FlagZ, p.exmem.FlagC, p.exmem.FlagV
				}
			}

			idex7 := p.idex7.toIDEX()
			execResult := p.executeStage.ExecuteWithFlags(&idex7, rnValue, rmValue,
				forwardFlags7, fwdN7, fwdZ7, fwdC7, fwdV7)
			nextEXMEM7 = SeptenaryEXMEMRegister{
				Valid:      true,
				PC:         p.idex7.PC,
				Inst:       p.idex7.Inst,
				ALUResult:  execResult.ALUResult,
				StoreValue: execResult.StoreValue,
				Rd:         p.idex7.Rd,
				MemRead:    p.idex7.MemRead,
				MemWrite:   p.idex7.MemWrite,
				RegWrite:   p.idex7.RegWrite,
				MemToReg:   p.idex7.MemToReg,
				SetsFlags:  execResult.SetsFlags,
				FlagN:      execResult.FlagN,
				FlagZ:      execResult.FlagZ,
				FlagC:      execResult.FlagC,
				FlagV:      execResult.FlagV,
			}

			// Branch prediction verification for septenary slot (idex7)
			if p.idex7.IsBranch {
				actualTaken := execResult.BranchTaken
				actualTarget := execResult.BranchTarget

				p.stats.BranchPredictions++

				predictedTaken := p.idex7.PredictedTaken
				predictedTarget := p.idex7.PredictedTarget
				earlyResolved := p.idex7.EarlyResolved

				wasMispredicted := false
				if actualTaken {
					if !predictedTaken {
						wasMispredicted = true
					} else if predictedTarget != actualTarget {
						wasMispredicted = true
					}
				} else {
					if predictedTaken {
						wasMispredicted = true
					}
				}

				if earlyResolved && actualTaken {
					wasMispredicted = false
				}

				p.branchPredictor.Update(p.idex7.PC, actualTaken, actualTarget)

				if wasMispredicted {
					p.stats.BranchMispredictions++
					branchTarget := actualTarget
					if !actualTaken {
						branchTarget = p.idex7.PC + 4
					}
					p.pc = branchTarget
					p.flushAllIFID()
					p.flushAllIDEX()
					p.stats.Flushes++

					if !memStall {
						p.memwb = nextMEMWB
						p.memwb2 = nextMEMWB2
						p.memwb3 = nextMEMWB3
						p.memwb4 = nextMEMWB4
						p.memwb5 = nextMEMWB5
						p.memwb6 = nextMEMWB6
						p.memwb7 = nextMEMWB7
						p.memwb8 = nextMEMWB8
						p.exmem = nextEXMEM
						p.exmem2 = nextEXMEM2
						p.exmem3 = nextEXMEM3
						p.exmem4 = nextEXMEM4
						p.exmem5 = nextEXMEM5
						p.exmem6 = nextEXMEM6
						p.exmem7.Clear()
						p.exmem8.Clear()
					}
					return
				}
				p.stats.BranchCorrect++
			}
		}
	}

	// Execute octonary slot
	if p.idex8.Valid && !memStall && !execStall {
		if p.latencyTable != nil && p.exLatency8 == 0 {
			p.exLatency8 = p.latencyTable.GetLatency(p.idex8.Inst)
		}
		if p.exLatency8 > 0 {
			p.exLatency8--
		}
		if p.exLatency8 == 0 {
			rnValue := p.forwardFromAllSlots(p.idex8.Rn, p.idex8.RnValue)
			rmValue := p.forwardFromAllSlots(p.idex8.Rm, p.idex8.RmValue)
			if nextEXMEM.Valid && nextEXMEM.RegWrite && nextEXMEM.Rd != 31 {
				if p.idex8.Rn == nextEXMEM.Rd {
					rnValue = nextEXMEM.ALUResult
				}
				if p.idex8.Rm == nextEXMEM.Rd {
					rmValue = nextEXMEM.ALUResult
				}
			}
			if nextEXMEM2.Valid && nextEXMEM2.RegWrite && nextEXMEM2.Rd != 31 {
				if p.idex8.Rn == nextEXMEM2.Rd {
					rnValue = nextEXMEM2.ALUResult
				}
				if p.idex8.Rm == nextEXMEM2.Rd {
					rmValue = nextEXMEM2.ALUResult
				}
			}
			if nextEXMEM3.Valid && nextEXMEM3.RegWrite && nextEXMEM3.Rd != 31 {
				if p.idex8.Rn == nextEXMEM3.Rd {
					rnValue = nextEXMEM3.ALUResult
				}
				if p.idex8.Rm == nextEXMEM3.Rd {
					rmValue = nextEXMEM3.ALUResult
				}
			}
			if nextEXMEM4.Valid && nextEXMEM4.RegWrite && nextEXMEM4.Rd != 31 {
				if p.idex8.Rn == nextEXMEM4.Rd {
					rnValue = nextEXMEM4.ALUResult
				}
				if p.idex8.Rm == nextEXMEM4.Rd {
					rmValue = nextEXMEM4.ALUResult
				}
			}
			if nextEXMEM5.Valid && nextEXMEM5.RegWrite && nextEXMEM5.Rd != 31 {
				if p.idex8.Rn == nextEXMEM5.Rd {
					rnValue = nextEXMEM5.ALUResult
				}
				if p.idex8.Rm == nextEXMEM5.Rd {
					rmValue = nextEXMEM5.ALUResult
				}
			}
			if nextEXMEM6.Valid && nextEXMEM6.RegWrite && nextEXMEM6.Rd != 31 {
				if p.idex8.Rn == nextEXMEM6.Rd {
					rnValue = nextEXMEM6.ALUResult
				}
				if p.idex8.Rm == nextEXMEM6.Rd {
					rmValue = nextEXMEM6.ALUResult
				}
			}
			if nextEXMEM7.Valid && nextEXMEM7.RegWrite && nextEXMEM7.Rd != 31 {
				if p.idex8.Rn == nextEXMEM7.Rd {
					rnValue = nextEXMEM7.ALUResult
				}
				if p.idex8.Rm == nextEXMEM7.Rd {
					rmValue = nextEXMEM7.ALUResult
				}
			}
			// Same-cycle PSTATE flag forwarding for B.cond in slot 8
			forwardFlags8 := false
			var fwdN8, fwdZ8, fwdC8, fwdV8 bool
			if p.idex8.Inst != nil && p.idex8.Inst.Op == insts.OpBCond {
				if nextEXMEM7.Valid && nextEXMEM7.SetsFlags {
					forwardFlags8 = true
					fwdN8, fwdZ8, fwdC8, fwdV8 = nextEXMEM7.FlagN, nextEXMEM7.FlagZ, nextEXMEM7.FlagC, nextEXMEM7.FlagV
				} else if nextEXMEM6.Valid && nextEXMEM6.SetsFlags {
					forwardFlags8 = true
					fwdN8, fwdZ8, fwdC8, fwdV8 = nextEXMEM6.FlagN, nextEXMEM6.FlagZ, nextEXMEM6.FlagC, nextEXMEM6.FlagV
				} else if nextEXMEM5.Valid && nextEXMEM5.SetsFlags {
					forwardFlags8 = true
					fwdN8, fwdZ8, fwdC8, fwdV8 = nextEXMEM5.FlagN, nextEXMEM5.FlagZ, nextEXMEM5.FlagC, nextEXMEM5.FlagV
				} else if nextEXMEM4.Valid && nextEXMEM4.SetsFlags {
					forwardFlags8 = true
					fwdN8, fwdZ8, fwdC8, fwdV8 = nextEXMEM4.FlagN, nextEXMEM4.FlagZ, nextEXMEM4.FlagC, nextEXMEM4.FlagV
				} else if nextEXMEM3.Valid && nextEXMEM3.SetsFlags {
					forwardFlags8 = true
					fwdN8, fwdZ8, fwdC8, fwdV8 = nextEXMEM3.FlagN, nextEXMEM3.FlagZ, nextEXMEM3.FlagC, nextEXMEM3.FlagV
				} else if nextEXMEM2.Valid && nextEXMEM2.SetsFlags {
					forwardFlags8 = true
					fwdN8, fwdZ8, fwdC8, fwdV8 = nextEXMEM2.FlagN, nextEXMEM2.FlagZ, nextEXMEM2.FlagC, nextEXMEM2.FlagV
				} else if nextEXMEM.Valid && nextEXMEM.SetsFlags {
					forwardFlags8 = true
					fwdN8, fwdZ8, fwdC8, fwdV8 = nextEXMEM.FlagN, nextEXMEM.FlagZ, nextEXMEM.FlagC, nextEXMEM.FlagV
				}
				if !forwardFlags8 && p.exmem.Valid && p.exmem.SetsFlags {
					forwardFlags8 = true
					fwdN8, fwdZ8, fwdC8, fwdV8 = p.exmem.FlagN, p.exmem.FlagZ, p.exmem.FlagC, p.exmem.FlagV
				}
			}

			idex8 := p.idex8.toIDEX()
			execResult := p.executeStage.ExecuteWithFlags(&idex8, rnValue, rmValue,
				forwardFlags8, fwdN8, fwdZ8, fwdC8, fwdV8)
			nextEXMEM8 = OctonaryEXMEMRegister{
				Valid:      true,
				PC:         p.idex8.PC,
				Inst:       p.idex8.Inst,
				ALUResult:  execResult.ALUResult,
				StoreValue: execResult.StoreValue,
				Rd:         p.idex8.Rd,
				MemRead:    p.idex8.MemRead,
				MemWrite:   p.idex8.MemWrite,
				RegWrite:   p.idex8.RegWrite,
				MemToReg:   p.idex8.MemToReg,
				SetsFlags:  execResult.SetsFlags,
				FlagN:      execResult.FlagN,
				FlagZ:      execResult.FlagZ,
				FlagC:      execResult.FlagC,
				FlagV:      execResult.FlagV,
			}

			// Branch prediction verification for octonary slot (idex8)
			if p.idex8.IsBranch {
				actualTaken := execResult.BranchTaken
				actualTarget := execResult.BranchTarget

				p.stats.BranchPredictions++

				predictedTaken := p.idex8.PredictedTaken
				predictedTarget := p.idex8.PredictedTarget
				earlyResolved := p.idex8.EarlyResolved

				wasMispredicted := false
				if actualTaken {
					if !predictedTaken {
						wasMispredicted = true
					} else if predictedTarget != actualTarget {
						wasMispredicted = true
					}
				} else {
					if predictedTaken {
						wasMispredicted = true
					}
				}

				if earlyResolved && actualTaken {
					wasMispredicted = false
				}

				p.branchPredictor.Update(p.idex8.PC, actualTaken, actualTarget)

				if wasMispredicted {
					p.stats.BranchMispredictions++
					branchTarget := actualTarget
					if !actualTaken {
						branchTarget = p.idex8.PC + 4
					}
					p.pc = branchTarget
					p.flushAllIFID()
					p.flushAllIDEX()
					p.stats.Flushes++

					if !memStall {
						p.memwb = nextMEMWB
						p.memwb2 = nextMEMWB2
						p.memwb3 = nextMEMWB3
						p.memwb4 = nextMEMWB4
						p.memwb5 = nextMEMWB5
						p.memwb6 = nextMEMWB6
						p.memwb7 = nextMEMWB7
						p.memwb8 = nextMEMWB8
						p.exmem = nextEXMEM
						p.exmem2 = nextEXMEM2
						p.exmem3 = nextEXMEM3
						p.exmem4 = nextEXMEM4
						p.exmem5 = nextEXMEM5
						p.exmem6 = nextEXMEM6
						p.exmem7 = nextEXMEM7
						p.exmem8.Clear()
					}
					return
				}
				p.stats.BranchCorrect++
			}
		}
	}

	// Detect load-use hazards for primary decode
	loadUseHazard := false
	if p.idex.Valid && p.idex.MemRead && p.idex.Rd != 31 && p.ifid.Valid {
		nextInst := p.decodeStage.decoder.Decode(p.ifid.InstructionWord)
		if nextInst != nil && nextInst.Op != insts.OpUnknown {
			usesRn := true
			usesRm := nextInst.Format == insts.FormatDPReg

			sourceRm := nextInst.Rm
			switch nextInst.Op {
			case insts.OpSTR, insts.OpSTRQ:
				usesRm = true
				sourceRm = nextInst.Rd
			}

			loadUseHazard = p.hazardUnit.DetectLoadUseHazardDecoded(
				p.idex.Rd, nextInst.Rn, sourceRm, usesRn, usesRm)
		}
	}

	stallResult := p.hazardUnit.ComputeStalls(loadUseHazard || execStall || memStall, false)

	// Stage 2: Decode (all 8 slots)
	var nextIDEX IDEXRegister
	var nextIDEX2 SecondaryIDEXRegister
	var nextIDEX3 TertiaryIDEXRegister
	var nextIDEX4 QuaternaryIDEXRegister
	var nextIDEX5 QuinaryIDEXRegister
	var nextIDEX6 SenaryIDEXRegister
	var nextIDEX7 SeptenaryIDEXRegister
	var nextIDEX8 OctonaryIDEXRegister

	// Track CMP+B.cond fusion for issue count adjustment
	fusedCMPBcond := false

	if p.ifid.Valid && !stallResult.StallID && !stallResult.FlushID && !execStall && !memStall {
		decResult := p.decodeStage.Decode(p.ifid.InstructionWord, p.ifid.PC)

		// CMP+B.cond fusion detection: check if slot 0 is CMP and slot 1 is B.cond
		if IsCMP(decResult.Inst) && p.ifid2.Valid {
			decResult2 := p.decodeStage.Decode(p.ifid2.InstructionWord, p.ifid2.PC)
			if IsBCond(decResult2.Inst) {
				// Fuse CMP+B.cond: put B.cond in slot 0 with CMP operands
				fusedCMPBcond = true
				nextIDEX = IDEXRegister{
					Valid:           true,
					PC:              p.ifid2.PC,
					Inst:            decResult2.Inst,
					RnValue:         decResult2.RnValue,
					RmValue:         decResult2.RmValue,
					Rd:              decResult2.Rd,
					Rn:              decResult2.Rn,
					Rm:              decResult2.Rm,
					MemRead:         decResult2.MemRead,
					MemWrite:        decResult2.MemWrite,
					RegWrite:        decResult2.RegWrite,
					MemToReg:        decResult2.MemToReg,
					IsBranch:        decResult2.IsBranch,
					PredictedTaken:  p.ifid2.PredictedTaken,
					PredictedTarget: p.ifid2.PredictedTarget,
					EarlyResolved:   p.ifid2.EarlyResolved,
					// Fusion fields from CMP
					IsFused:    true,
					FusedRnVal: decResult.RnValue,
					FusedRmVal: decResult.RmValue,
					FusedIs64:  decResult.Inst.Is64Bit,
					FusedIsImm: decResult.Inst.Format == insts.FormatDPImm,
					FusedImmVal: func() uint64 {
						if decResult.Inst.Format == insts.FormatDPImm {
							imm := decResult.Inst.Imm
							if decResult.Inst.Shift > 0 {
								imm <<= decResult.Inst.Shift
							}
							return imm
						}
						return 0
					}(),
				}
			}
		}

		if !fusedCMPBcond {
			nextIDEX = IDEXRegister{
				Valid:           true,
				PC:              p.ifid.PC,
				Inst:            decResult.Inst,
				RnValue:         decResult.RnValue,
				RmValue:         decResult.RmValue,
				Rd:              decResult.Rd,
				Rn:              decResult.Rn,
				Rm:              decResult.Rm,
				MemRead:         decResult.MemRead,
				MemWrite:        decResult.MemWrite,
				RegWrite:        decResult.RegWrite,
				MemToReg:        decResult.MemToReg,
				IsBranch:        decResult.IsBranch,
				PredictedTaken:  p.ifid.PredictedTaken,
				PredictedTarget: p.ifid.PredictedTarget,
				EarlyResolved:   p.ifid.EarlyResolved,
			}
		}

		// Try to issue instructions 2-8 if they can issue with earlier instructions.
		// Uses fixed-size array to avoid heap allocation per tick.
		var issuedInsts [8]*IDEXRegister
		issuedInsts[0] = &nextIDEX
		issuedCount := 1

		// Track if IFID2 was consumed by fusion (skip its decode)
		ifid2ConsumedByFusion := fusedCMPBcond

		// Decode slot 2 (IFID2) - skip if consumed by fusion
		if p.ifid2.Valid && !ifid2ConsumedByFusion {
			decResult2 := p.decodeStage.Decode(p.ifid2.InstructionWord, p.ifid2.PC)
			tempIDEX2 := IDEXRegister{
				Valid:           true,
				PC:              p.ifid2.PC,
				Inst:            decResult2.Inst,
				RnValue:         decResult2.RnValue,
				RmValue:         decResult2.RmValue,
				Rd:              decResult2.Rd,
				Rn:              decResult2.Rn,
				Rm:              decResult2.Rm,
				MemRead:         decResult2.MemRead,
				MemWrite:        decResult2.MemWrite,
				RegWrite:        decResult2.RegWrite,
				MemToReg:        decResult2.MemToReg,
				IsBranch:        decResult2.IsBranch,
				PredictedTaken:  p.ifid2.PredictedTaken,
				PredictedTarget: p.ifid2.PredictedTarget,
				EarlyResolved:   p.ifid2.EarlyResolved,
			}
			if canIssueWith(&tempIDEX2, &issuedInsts, issuedCount) {
				nextIDEX2.fromIDEX(&tempIDEX2)
				issuedInsts[issuedCount] = &tempIDEX2
				issuedCount++
			}
		}

		// Decode slot 3
		if p.ifid3.Valid && nextIDEX2.Valid {
			decResult3 := p.decodeStage.Decode(p.ifid3.InstructionWord, p.ifid3.PC)
			tempIDEX3 := IDEXRegister{
				Valid:           true,
				PC:              p.ifid3.PC,
				Inst:            decResult3.Inst,
				RnValue:         decResult3.RnValue,
				RmValue:         decResult3.RmValue,
				Rd:              decResult3.Rd,
				Rn:              decResult3.Rn,
				Rm:              decResult3.Rm,
				MemRead:         decResult3.MemRead,
				MemWrite:        decResult3.MemWrite,
				RegWrite:        decResult3.RegWrite,
				MemToReg:        decResult3.MemToReg,
				IsBranch:        decResult3.IsBranch,
				PredictedTaken:  p.ifid3.PredictedTaken,
				PredictedTarget: p.ifid3.PredictedTarget,
				EarlyResolved:   p.ifid3.EarlyResolved,
			}
			if canIssueWith(&tempIDEX3, &issuedInsts, issuedCount) {
				nextIDEX3.fromIDEX(&tempIDEX3)
				issuedInsts[issuedCount] = &tempIDEX3
				issuedCount++
			}
		}

		// Decode slot 4
		if p.ifid4.Valid && nextIDEX3.Valid {
			decResult4 := p.decodeStage.Decode(p.ifid4.InstructionWord, p.ifid4.PC)
			tempIDEX4 := IDEXRegister{
				Valid:           true,
				PC:              p.ifid4.PC,
				Inst:            decResult4.Inst,
				RnValue:         decResult4.RnValue,
				RmValue:         decResult4.RmValue,
				Rd:              decResult4.Rd,
				Rn:              decResult4.Rn,
				Rm:              decResult4.Rm,
				MemRead:         decResult4.MemRead,
				MemWrite:        decResult4.MemWrite,
				RegWrite:        decResult4.RegWrite,
				MemToReg:        decResult4.MemToReg,
				IsBranch:        decResult4.IsBranch,
				PredictedTaken:  p.ifid4.PredictedTaken,
				PredictedTarget: p.ifid4.PredictedTarget,
				EarlyResolved:   p.ifid4.EarlyResolved,
			}
			if canIssueWith(&tempIDEX4, &issuedInsts, issuedCount) {
				nextIDEX4.fromIDEX(&tempIDEX4)
				issuedInsts[issuedCount] = &tempIDEX4
				issuedCount++
			}
		}

		// Decode slot 5
		if p.ifid5.Valid && nextIDEX4.Valid {
			decResult5 := p.decodeStage.Decode(p.ifid5.InstructionWord, p.ifid5.PC)
			tempIDEX5 := IDEXRegister{
				Valid:           true,
				PC:              p.ifid5.PC,
				Inst:            decResult5.Inst,
				RnValue:         decResult5.RnValue,
				RmValue:         decResult5.RmValue,
				Rd:              decResult5.Rd,
				Rn:              decResult5.Rn,
				Rm:              decResult5.Rm,
				MemRead:         decResult5.MemRead,
				MemWrite:        decResult5.MemWrite,
				RegWrite:        decResult5.RegWrite,
				MemToReg:        decResult5.MemToReg,
				IsBranch:        decResult5.IsBranch,
				PredictedTaken:  p.ifid5.PredictedTaken,
				PredictedTarget: p.ifid5.PredictedTarget,
				EarlyResolved:   p.ifid5.EarlyResolved,
			}
			if canIssueWith(&tempIDEX5, &issuedInsts, issuedCount) {
				nextIDEX5.fromIDEX(&tempIDEX5)
				issuedInsts[issuedCount] = &tempIDEX5
				issuedCount++
			}
		}

		// Decode slot 6
		if p.ifid6.Valid && nextIDEX5.Valid {
			decResult6 := p.decodeStage.Decode(p.ifid6.InstructionWord, p.ifid6.PC)
			tempIDEX6 := IDEXRegister{
				Valid:           true,
				PC:              p.ifid6.PC,
				Inst:            decResult6.Inst,
				RnValue:         decResult6.RnValue,
				RmValue:         decResult6.RmValue,
				Rd:              decResult6.Rd,
				Rn:              decResult6.Rn,
				Rm:              decResult6.Rm,
				MemRead:         decResult6.MemRead,
				MemWrite:        decResult6.MemWrite,
				RegWrite:        decResult6.RegWrite,
				MemToReg:        decResult6.MemToReg,
				IsBranch:        decResult6.IsBranch,
				PredictedTaken:  p.ifid6.PredictedTaken,
				PredictedTarget: p.ifid6.PredictedTarget,
				EarlyResolved:   p.ifid6.EarlyResolved,
			}
			if canIssueWith(&tempIDEX6, &issuedInsts, issuedCount) {
				nextIDEX6.fromIDEX(&tempIDEX6)
				issuedInsts[issuedCount] = &tempIDEX6
				issuedCount++
			}
		}

		// Decode slot 7
		if p.ifid7.Valid && nextIDEX6.Valid {
			decResult7 := p.decodeStage.Decode(p.ifid7.InstructionWord, p.ifid7.PC)
			tempIDEX7 := IDEXRegister{
				Valid:           true,
				PC:              p.ifid7.PC,
				Inst:            decResult7.Inst,
				RnValue:         decResult7.RnValue,
				RmValue:         decResult7.RmValue,
				Rd:              decResult7.Rd,
				Rn:              decResult7.Rn,
				Rm:              decResult7.Rm,
				MemRead:         decResult7.MemRead,
				MemWrite:        decResult7.MemWrite,
				RegWrite:        decResult7.RegWrite,
				MemToReg:        decResult7.MemToReg,
				IsBranch:        decResult7.IsBranch,
				PredictedTaken:  p.ifid7.PredictedTaken,
				PredictedTarget: p.ifid7.PredictedTarget,
				EarlyResolved:   p.ifid7.EarlyResolved,
			}
			if canIssueWith(&tempIDEX7, &issuedInsts, issuedCount) {
				nextIDEX7.fromIDEX(&tempIDEX7)
				issuedInsts[issuedCount] = &tempIDEX7
				issuedCount++
			}
		}

		// Decode slot 8
		if p.ifid8.Valid && nextIDEX7.Valid {
			decResult8 := p.decodeStage.Decode(p.ifid8.InstructionWord, p.ifid8.PC)
			tempIDEX8 := IDEXRegister{
				Valid:           true,
				PC:              p.ifid8.PC,
				Inst:            decResult8.Inst,
				RnValue:         decResult8.RnValue,
				RmValue:         decResult8.RmValue,
				Rd:              decResult8.Rd,
				Rn:              decResult8.Rn,
				Rm:              decResult8.Rm,
				MemRead:         decResult8.MemRead,
				MemWrite:        decResult8.MemWrite,
				RegWrite:        decResult8.RegWrite,
				MemToReg:        decResult8.MemToReg,
				IsBranch:        decResult8.IsBranch,
				PredictedTaken:  p.ifid8.PredictedTaken,
				PredictedTarget: p.ifid8.PredictedTarget,
				EarlyResolved:   p.ifid8.EarlyResolved,
			}
			if canIssueWith(&tempIDEX8, &issuedInsts, issuedCount) {
				nextIDEX8.fromIDEX(&tempIDEX8)
			}
		}
	} else if (stallResult.StallID || execStall || memStall) && !stallResult.FlushID {
		nextIDEX = p.idex
		nextIDEX2 = p.idex2
		nextIDEX3 = p.idex3
		nextIDEX4 = p.idex4
		nextIDEX5 = p.idex5
		nextIDEX6 = p.idex6
		nextIDEX7 = p.idex7
		nextIDEX8 = p.idex8
	}

	// Count how many instructions were issued this cycle for fetch advancement
	issueCount := 0
	if nextIDEX.Valid {
		issueCount++
	}
	if nextIDEX2.Valid {
		issueCount++
	}
	if nextIDEX3.Valid {
		issueCount++
	}
	if nextIDEX4.Valid {
		issueCount++
	}
	if nextIDEX5.Valid {
		issueCount++
	}
	if nextIDEX6.Valid {
		issueCount++
	}
	if nextIDEX7.Valid {
		issueCount++
	}
	if nextIDEX8.Valid {
		issueCount++
	}
	// CMP+B.cond fusion consumes 2 IFID slots but produces 1 IDEX,
	// so add 1 to issueCount to advance fetch properly
	if fusedCMPBcond {
		issueCount++
	}

	// Stage 1: Fetch (all 8 slots)
	var nextIFID IFIDRegister
	var nextIFID2 SecondaryIFIDRegister
	var nextIFID3 TertiaryIFIDRegister
	var nextIFID4 QuaternaryIFIDRegister
	var nextIFID5 QuinaryIFIDRegister
	var nextIFID6 SenaryIFIDRegister
	var nextIFID7 SeptenaryIFIDRegister
	var nextIFID8 OctonaryIFIDRegister
	fetchStall := false

	if !stallResult.StallIF && !stallResult.FlushIF && !memStall && !execStall {
		// Shift unissued instructions forward
		pendingInsts, pendingCount := p.collectPendingFetchInstructions8(issueCount)

		// Fill slots with pending instructions first, then fetch new ones
		fetchPC := p.pc
		slotIdx := 0

		// Place pending instructions
		branchPredictedTaken := false
		for pi := 0; pi < pendingCount; pi++ {
			pending := pendingInsts[pi]
			if branchPredictedTaken {
				break
			}
			switch slotIdx {
			case 0:
				isUncondBranch, uncondTarget := isUnconditionalBranch(pending.Word, pending.PC)
				pred := p.branchPredictor.Predict(pending.PC)
				earlyResolved := false
				if isUncondBranch {
					pred.Taken = true
					pred.Target = uncondTarget
					pred.TargetKnown = true
					earlyResolved = true
				}
				enrichPredictionWithEncodedTarget(&pred, pending.Word, pending.PC)
				nextIFID = IFIDRegister{
					Valid:           true,
					PC:              pending.PC,
					InstructionWord: pending.Word,
					PredictedTaken:  pred.Taken,
					PredictedTarget: pred.Target,
					EarlyResolved:   earlyResolved,
				}
				if pred.Taken && pred.TargetKnown {
					fetchPC = pred.Target
					branchPredictedTaken = true
				}
			default:
				isUncondBranch, uncondTarget := isUnconditionalBranch(pending.Word, pending.PC)
				pred := p.branchPredictor.Predict(pending.PC)
				earlyResolved := false
				if isUncondBranch {
					pred.Taken = true
					pred.Target = uncondTarget
					pred.TargetKnown = true
					earlyResolved = true
				}
				enrichPredictionWithEncodedTarget(&pred, pending.Word, pending.PC)
				switch slotIdx {
				case 1:
					nextIFID2 = SecondaryIFIDRegister{Valid: true, PC: pending.PC, InstructionWord: pending.Word, PredictedTaken: pred.Taken, PredictedTarget: pred.Target, EarlyResolved: earlyResolved}
				case 2:
					nextIFID3 = TertiaryIFIDRegister{Valid: true, PC: pending.PC, InstructionWord: pending.Word, PredictedTaken: pred.Taken, PredictedTarget: pred.Target, EarlyResolved: earlyResolved}
				case 3:
					nextIFID4 = QuaternaryIFIDRegister{Valid: true, PC: pending.PC, InstructionWord: pending.Word, PredictedTaken: pred.Taken, PredictedTarget: pred.Target, EarlyResolved: earlyResolved}
				case 4:
					nextIFID5 = QuinaryIFIDRegister{Valid: true, PC: pending.PC, InstructionWord: pending.Word, PredictedTaken: pred.Taken, PredictedTarget: pred.Target, EarlyResolved: earlyResolved}
				case 5:
					nextIFID6 = SenaryIFIDRegister{Valid: true, PC: pending.PC, InstructionWord: pending.Word, PredictedTaken: pred.Taken, PredictedTarget: pred.Target, EarlyResolved: earlyResolved}
				case 6:
					nextIFID7 = SeptenaryIFIDRegister{Valid: true, PC: pending.PC, InstructionWord: pending.Word, PredictedTaken: pred.Taken, PredictedTarget: pred.Target, EarlyResolved: earlyResolved}
				case 7:
					nextIFID8 = OctonaryIFIDRegister{Valid: true, PC: pending.PC, InstructionWord: pending.Word, PredictedTaken: pred.Taken, PredictedTarget: pred.Target, EarlyResolved: earlyResolved}
				}
				if pred.Taken && pred.TargetKnown {
					fetchPC = pred.Target
					branchPredictedTaken = true
				}
			}
			slotIdx++
		}

		// Fetch new instructions to fill remaining slots
		for slotIdx < 8 {
			var word uint32
			var ok bool

			if p.useICache && p.cachedFetchStage != nil {
				word, ok, fetchStall = p.cachedFetchStage.Fetch(fetchPC)
				if fetchStall {
					p.stats.Stalls++
					break
				}
			} else {
				word, ok = p.fetchStage.Fetch(fetchPC)
			}

			if !ok {
				break
			}

			// Branch elimination: unconditional B (not BL) instructions are
			// eliminated at fetch time. They never enter the pipeline.
			if isEliminableBranch(word) {
				_, uncondTarget := isUnconditionalBranch(word, fetchPC)
				fetchPC = uncondTarget
				p.stats.EliminatedBranches++
				// Don't create IFID entry - branch is eliminated
				// Continue fetching from target without advancing slotIdx
				continue
			}

			// Zero-cycle branch folding: DISABLED
			// Previous implementation eliminated high-confidence conditional branches at
			// fetch time without entering the pipeline. This is unsafe because:
			// 1. Folded branches never execute, so condition flags are never checked
			// 2. When prediction is wrong (e.g., loop exit), there's no recovery path
			// 3. The pipeline hangs indefinitely on loop exits
			//
			// The M2's zero-cycle folding likely works differently - perhaps branches
			// still enter the pipeline but complete in zero cycles when prediction is
			// correct. For now, all conditional branches must enter the pipeline for
			// proper misprediction detection and recovery.
			//
			// TODO: Implement proper zero-cycle folding with misprediction recovery
			if slotIdx == 0 {
				isUncondBranch, uncondTarget := isUnconditionalBranch(word, fetchPC)
				pred := p.branchPredictor.Predict(fetchPC)
				earlyResolved := false
				if isUncondBranch {
					pred.Taken = true
					pred.Target = uncondTarget
					pred.TargetKnown = true
					earlyResolved = true
				}
				enrichPredictionWithEncodedTarget(&pred, word, fetchPC)
				nextIFID = IFIDRegister{
					Valid:           true,
					PC:              fetchPC,
					InstructionWord: word,
					PredictedTaken:  pred.Taken,
					PredictedTarget: pred.Target,
					EarlyResolved:   earlyResolved,
				}
				if pred.Taken && pred.TargetKnown {
					fetchPC = pred.Target
					slotIdx++
					continue
				}
			} else {
				isUncondBranch, uncondTarget := isUnconditionalBranch(word, fetchPC)
				pred := p.branchPredictor.Predict(fetchPC)
				earlyResolved := false
				if isUncondBranch {
					pred.Taken = true
					pred.Target = uncondTarget
					pred.TargetKnown = true
					earlyResolved = true
				}
				enrichPredictionWithEncodedTarget(&pred, word, fetchPC)
				switch slotIdx {
				case 1:
					nextIFID2 = SecondaryIFIDRegister{Valid: true, PC: fetchPC, InstructionWord: word, PredictedTaken: pred.Taken, PredictedTarget: pred.Target, EarlyResolved: earlyResolved}
				case 2:
					nextIFID3 = TertiaryIFIDRegister{Valid: true, PC: fetchPC, InstructionWord: word, PredictedTaken: pred.Taken, PredictedTarget: pred.Target, EarlyResolved: earlyResolved}
				case 3:
					nextIFID4 = QuaternaryIFIDRegister{Valid: true, PC: fetchPC, InstructionWord: word, PredictedTaken: pred.Taken, PredictedTarget: pred.Target, EarlyResolved: earlyResolved}
				case 4:
					nextIFID5 = QuinaryIFIDRegister{Valid: true, PC: fetchPC, InstructionWord: word, PredictedTaken: pred.Taken, PredictedTarget: pred.Target, EarlyResolved: earlyResolved}
				case 5:
					nextIFID6 = SenaryIFIDRegister{Valid: true, PC: fetchPC, InstructionWord: word, PredictedTaken: pred.Taken, PredictedTarget: pred.Target, EarlyResolved: earlyResolved}
				case 6:
					nextIFID7 = SeptenaryIFIDRegister{Valid: true, PC: fetchPC, InstructionWord: word, PredictedTaken: pred.Taken, PredictedTarget: pred.Target, EarlyResolved: earlyResolved}
				case 7:
					nextIFID8 = OctonaryIFIDRegister{Valid: true, PC: fetchPC, InstructionWord: word, PredictedTaken: pred.Taken, PredictedTarget: pred.Target, EarlyResolved: earlyResolved}
				}
				if pred.Taken && pred.TargetKnown {
					fetchPC = pred.Target
					slotIdx++
					continue
				}
			}
			fetchPC += 4
			slotIdx++
		}
		p.pc = fetchPC

		if fetchStall {
			nextIFID = p.ifid
			nextIFID2 = p.ifid2
			nextIFID3 = p.ifid3
			nextIFID4 = p.ifid4
			nextIFID5 = p.ifid5
			nextIFID6 = p.ifid6
			nextIFID7 = p.ifid7
			nextIFID8 = p.ifid8
			nextIDEX = p.idex
			nextIDEX2 = p.idex2
			nextIDEX3 = p.idex3
			nextIDEX4 = p.idex4
			nextIDEX5 = p.idex5
			nextIDEX6 = p.idex6
			nextIDEX7 = p.idex7
			nextIDEX8 = p.idex8
			nextEXMEM = p.exmem
			nextEXMEM2 = p.exmem2
			nextEXMEM3 = p.exmem3
			nextEXMEM4 = p.exmem4
			nextEXMEM5 = p.exmem5
			nextEXMEM6 = p.exmem6
			nextEXMEM7 = p.exmem7
			nextEXMEM8 = p.exmem8
		}
	} else if (stallResult.StallIF || memStall || execStall) && !stallResult.FlushIF {
		nextIFID = p.ifid
		nextIFID2 = p.ifid2
		nextIFID3 = p.ifid3
		nextIFID4 = p.ifid4
		nextIFID5 = p.ifid5
		nextIFID6 = p.ifid6
		nextIFID7 = p.ifid7
		nextIFID8 = p.ifid8
		p.stats.Stalls++
	}

	// Latch all pipeline registers
	if !memStall && !fetchStall {
		p.memwb = nextMEMWB
		p.memwb2 = nextMEMWB2
		p.memwb3 = nextMEMWB3
		p.memwb4 = nextMEMWB4
		p.memwb5 = nextMEMWB5
		p.memwb6 = nextMEMWB6
		p.memwb7 = nextMEMWB7
		p.memwb8 = nextMEMWB8
	} else {
		p.memwb.Clear()
		p.memwb2.Clear()
		p.memwb3.Clear()
		p.memwb4.Clear()
		p.memwb5.Clear()
		p.memwb6.Clear()
		p.memwb7.Clear()
		p.memwb8.Clear()
	}
	if !execStall && !memStall {
		p.exmem = nextEXMEM
		p.exmem2 = nextEXMEM2
		p.exmem3 = nextEXMEM3
		p.exmem4 = nextEXMEM4
		p.exmem5 = nextEXMEM5
		p.exmem6 = nextEXMEM6
		p.exmem7 = nextEXMEM7
		p.exmem8 = nextEXMEM8
	}
	if stallResult.InsertBubbleEX && !execStall && !memStall {
		p.idex.Clear()
		p.idex2.Clear()
		p.idex3.Clear()
		p.idex4.Clear()
		p.idex5.Clear()
		p.idex6.Clear()
		p.idex7.Clear()
		p.idex8.Clear()
	} else if !memStall {
		p.idex = nextIDEX
		p.idex2 = nextIDEX2
		p.idex3 = nextIDEX3
		p.idex4 = nextIDEX4
		p.idex5 = nextIDEX5
		p.idex6 = nextIDEX6
		p.idex7 = nextIDEX7
		p.idex8 = nextIDEX8
	}
	p.ifid = nextIFID
	p.ifid2 = nextIFID2
	p.ifid3 = nextIFID3
	p.ifid4 = nextIFID4
	p.ifid5 = nextIFID5
	p.ifid6 = nextIFID6
	p.ifid7 = nextIFID7
	p.ifid8 = nextIFID8
}

// collectPendingFetchInstructions8 returns unissued instructions for 8-wide.
// Uses a fixed-size array to avoid heap allocation per tick.
func (p *Pipeline) collectPendingFetchInstructions8(issueCount int) ([8]pendingFetchInst, int) {
	var allFetched [8]pendingFetchInst
	count := 0

	if p.ifid.Valid {
		allFetched[count] = pendingFetchInst{PC: p.ifid.PC, Word: p.ifid.InstructionWord}
		count++
	}
	if p.ifid2.Valid {
		allFetched[count] = pendingFetchInst{PC: p.ifid2.PC, Word: p.ifid2.InstructionWord}
		count++
	}
	if p.ifid3.Valid {
		allFetched[count] = pendingFetchInst{PC: p.ifid3.PC, Word: p.ifid3.InstructionWord}
		count++
	}
	if p.ifid4.Valid {
		allFetched[count] = pendingFetchInst{PC: p.ifid4.PC, Word: p.ifid4.InstructionWord}
		count++
	}
	if p.ifid5.Valid {
		allFetched[count] = pendingFetchInst{PC: p.ifid5.PC, Word: p.ifid5.InstructionWord}
		count++
	}
	if p.ifid6.Valid {
		allFetched[count] = pendingFetchInst{PC: p.ifid6.PC, Word: p.ifid6.InstructionWord}
		count++
	}
	if p.ifid7.Valid {
		allFetched[count] = pendingFetchInst{PC: p.ifid7.PC, Word: p.ifid7.InstructionWord}
		count++
	}
	if p.ifid8.Valid {
		allFetched[count] = pendingFetchInst{PC: p.ifid8.PC, Word: p.ifid8.InstructionWord}
		count++
	}

	pendingCount := 0
	if issueCount < count {
		pendingCount = count - issueCount
		for i := 0; i < pendingCount; i++ {
			allFetched[i] = allFetched[i+issueCount]
		}
	}

	return allFetched, pendingCount
}
