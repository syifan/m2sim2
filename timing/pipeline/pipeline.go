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
		p.cachedMemoryStage = NewCachedMemoryStage(dcache, p.memory)
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

		// Initialize D-cache
		dcache := cache.New(cache.DefaultL1DConfig(), backing)
		p.cachedMemoryStage = NewCachedMemoryStage(dcache, p.memory)
		p.useDCache = true
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

	// Non-cached memory latency tracking
	memPending   bool   // True if waiting for memory operation to complete
	memPendingPC uint64 // PC of pending memory operation

	// Shared resources
	regFile *emu.RegFile
	memory  *emu.Memory

	// Syscall handling
	syscallHandler emu.SyscallHandler

	// Program counter
	pc uint64

	// Superscalar configuration
	superscalarConfig SuperscalarConfig

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
		nextInst := p.decodeStage.decoder.Decode(p.ifid.InstructionWord)
		if nextInst != nil && nextInst.Op != insts.OpUnknown {
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

	// Stage 5: Writeback
	savedMEMWB := p.memwb
	p.writebackStage.Writeback(&p.memwb)
	if p.memwb.Valid {
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

			execResult := p.executeStage.Execute(&p.idex, rnValue, rmValue)

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
	// Stage 5: Writeback (both slots)
	savedMEMWB := p.memwb
	p.writebackStage.Writeback(&p.memwb)
	if p.memwb.Valid {
		p.stats.Instructions++
	}
	// Writeback secondary slot
	if p.memwb2.Valid && p.memwb2.RegWrite && p.memwb2.Rd != 31 {
		var value uint64
		if p.memwb2.MemToReg {
			value = p.memwb2.MemData
		} else {
			value = p.memwb2.ALUResult
		}
		p.regFile.WriteReg(p.memwb2.Rd, value)
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
			}
		}
	}

	// Secondary slot memory (only ALU results, no memory access)
	if p.exmem2.Valid && !memStall {
		nextMEMWB2 = SecondaryMEMWBRegister{
			Valid:     true,
			PC:        p.exmem2.PC,
			Inst:      p.exmem2.Inst,
			ALUResult: p.exmem2.ALUResult,
			MemData:   0,
			Rd:        p.exmem2.Rd,
			RegWrite:  p.exmem2.RegWrite,
			MemToReg:  false,
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

			execResult := p.executeStage.Execute(&p.idex, rnValue, rmValue)

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
					} else {
						p.pc += 4
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
	// Stage 5: Writeback (all 4 slots)
	savedMEMWB := p.memwb
	p.writebackStage.Writeback(&p.memwb)
	if p.memwb.Valid {
		p.stats.Instructions++
	}

	// Writeback secondary slot
	if p.memwb2.Valid && p.memwb2.RegWrite && p.memwb2.Rd != 31 {
		var value uint64
		if p.memwb2.MemToReg {
			value = p.memwb2.MemData
		} else {
			value = p.memwb2.ALUResult
		}
		p.regFile.WriteReg(p.memwb2.Rd, value)
		p.stats.Instructions++
	}

	// Writeback tertiary slot
	if p.memwb3.Valid && p.memwb3.RegWrite && p.memwb3.Rd != 31 {
		var value uint64
		if p.memwb3.MemToReg {
			value = p.memwb3.MemData
		} else {
			value = p.memwb3.ALUResult
		}
		p.regFile.WriteReg(p.memwb3.Rd, value)
		p.stats.Instructions++
	}

	// Writeback quaternary slot
	if p.memwb4.Valid && p.memwb4.RegWrite && p.memwb4.Rd != 31 {
		var value uint64
		if p.memwb4.MemToReg {
			value = p.memwb4.MemData
		} else {
			value = p.memwb4.ALUResult
		}
		p.regFile.WriteReg(p.memwb4.Rd, value)
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
			}
		}
	}

	// Secondary slot memory (ALU results only, no memory access)
	if p.exmem2.Valid && !memStall {
		nextMEMWB2 = SecondaryMEMWBRegister{
			Valid:     true,
			PC:        p.exmem2.PC,
			Inst:      p.exmem2.Inst,
			ALUResult: p.exmem2.ALUResult,
			MemData:   0,
			Rd:        p.exmem2.Rd,
			RegWrite:  p.exmem2.RegWrite,
			MemToReg:  false,
		}
	}

	// Tertiary slot memory (ALU results only)
	if p.exmem3.Valid && !memStall {
		nextMEMWB3 = TertiaryMEMWBRegister{
			Valid:     true,
			PC:        p.exmem3.PC,
			Inst:      p.exmem3.Inst,
			ALUResult: p.exmem3.ALUResult,
			MemData:   0,
			Rd:        p.exmem3.Rd,
			RegWrite:  p.exmem3.RegWrite,
			MemToReg:  false,
		}
	}

	// Quaternary slot memory (ALU results only)
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

			execResult := p.executeStage.Execute(&p.idex, rnValue, rmValue)

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
			}

			// Check for branch in primary slot
			if execResult.BranchTaken {
				p.pc = execResult.BranchTarget
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
			Valid:    true,
			PC:       p.ifid.PC,
			Inst:     decResult.Inst,
			RnValue:  decResult.RnValue,
			RmValue:  decResult.RmValue,
			Rd:       decResult.Rd,
			Rn:       decResult.Rn,
			Rm:       decResult.Rm,
			MemRead:  decResult.MemRead,
			MemWrite: decResult.MemWrite,
			RegWrite: decResult.RegWrite,
			MemToReg: decResult.MemToReg,
			IsBranch: decResult.IsBranch,
		}

		// Try to issue instructions 2, 3, 4 if they can issue with earlier instructions
		issuedInsts := []*IDEXRegister{&nextIDEX}

		// Decode slot 2
		if p.ifid2.Valid {
			decResult2 := p.decodeStage.Decode(p.ifid2.InstructionWord, p.ifid2.PC)
			tempIDEX2 := IDEXRegister{
				Valid:    true,
				PC:       p.ifid2.PC,
				Inst:     decResult2.Inst,
				RnValue:  decResult2.RnValue,
				RmValue:  decResult2.RmValue,
				Rd:       decResult2.Rd,
				Rn:       decResult2.Rn,
				Rm:       decResult2.Rm,
				MemRead:  decResult2.MemRead,
				MemWrite: decResult2.MemWrite,
				RegWrite: decResult2.RegWrite,
				MemToReg: decResult2.MemToReg,
				IsBranch: decResult2.IsBranch,
			}

			if canIssueWith(&tempIDEX2, issuedInsts) {
				nextIDEX2.fromIDEX(&tempIDEX2)
				issuedInsts = append(issuedInsts, &tempIDEX2)
			}
		}

		// Decode slot 3
		if p.ifid3.Valid && nextIDEX2.Valid {
			decResult3 := p.decodeStage.Decode(p.ifid3.InstructionWord, p.ifid3.PC)
			tempIDEX3 := IDEXRegister{
				Valid:    true,
				PC:       p.ifid3.PC,
				Inst:     decResult3.Inst,
				RnValue:  decResult3.RnValue,
				RmValue:  decResult3.RmValue,
				Rd:       decResult3.Rd,
				Rn:       decResult3.Rn,
				Rm:       decResult3.Rm,
				MemRead:  decResult3.MemRead,
				MemWrite: decResult3.MemWrite,
				RegWrite: decResult3.RegWrite,
				MemToReg: decResult3.MemToReg,
				IsBranch: decResult3.IsBranch,
			}

			if canIssueWith(&tempIDEX3, issuedInsts) {
				nextIDEX3.fromIDEX(&tempIDEX3)
				issuedInsts = append(issuedInsts, &tempIDEX3)
			}
		}

		// Decode slot 4
		if p.ifid4.Valid && nextIDEX3.Valid {
			decResult4 := p.decodeStage.Decode(p.ifid4.InstructionWord, p.ifid4.PC)
			tempIDEX4 := IDEXRegister{
				Valid:    true,
				PC:       p.ifid4.PC,
				Inst:     decResult4.Inst,
				RnValue:  decResult4.RnValue,
				RmValue:  decResult4.RmValue,
				Rd:       decResult4.Rd,
				Rn:       decResult4.Rn,
				Rm:       decResult4.Rm,
				MemRead:  decResult4.MemRead,
				MemWrite: decResult4.MemWrite,
				RegWrite: decResult4.RegWrite,
				MemToReg: decResult4.MemToReg,
				IsBranch: decResult4.IsBranch,
			}

			if canIssueWith(&tempIDEX4, issuedInsts) {
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
		pendingInsts := p.collectPendingFetchInstructions(issueCount)

		// Fill slots with pending instructions first, then fetch new ones
		fetchPC := p.pc
		slotIdx := 0

		// Place pending instructions
		for _, pending := range pendingInsts {
			switch slotIdx {
			case 0:
				nextIFID = IFIDRegister{Valid: true, PC: pending.PC, InstructionWord: pending.Word}
			case 1:
				nextIFID2 = SecondaryIFIDRegister{Valid: true, PC: pending.PC, InstructionWord: pending.Word}
			case 2:
				nextIFID3 = TertiaryIFIDRegister{Valid: true, PC: pending.PC, InstructionWord: pending.Word}
			case 3:
				nextIFID4 = QuaternaryIFIDRegister{Valid: true, PC: pending.PC, InstructionWord: pending.Word}
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

			switch slotIdx {
			case 0:
				nextIFID = IFIDRegister{Valid: true, PC: fetchPC, InstructionWord: word}
			case 1:
				nextIFID2 = SecondaryIFIDRegister{Valid: true, PC: fetchPC, InstructionWord: word}
			case 2:
				nextIFID3 = TertiaryIFIDRegister{Valid: true, PC: fetchPC, InstructionWord: word}
			case 3:
				nextIFID4 = QuaternaryIFIDRegister{Valid: true, PC: fetchPC, InstructionWord: word}
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
//
//nolint:unused // Scaffolding for 4-wide implementation (PR #114)
func (p *Pipeline) collectPendingFetchInstructions(issueCount int) []pendingFetchInst {
	var pending []pendingFetchInst

	// All fetched instructions in order
	allFetched := []pendingFetchInst{}
	if p.ifid.Valid {
		allFetched = append(allFetched, pendingFetchInst{PC: p.ifid.PC, Word: p.ifid.InstructionWord})
	}
	if p.ifid2.Valid {
		allFetched = append(allFetched, pendingFetchInst{PC: p.ifid2.PC, Word: p.ifid2.InstructionWord})
	}
	if p.ifid3.Valid {
		allFetched = append(allFetched, pendingFetchInst{PC: p.ifid3.PC, Word: p.ifid3.InstructionWord})
	}
	if p.ifid4.Valid {
		allFetched = append(allFetched, pendingFetchInst{PC: p.ifid4.PC, Word: p.ifid4.InstructionWord})
	}

	// Skip the first issueCount instructions (they were issued)
	if issueCount < len(allFetched) {
		pending = allFetched[issueCount:]
	}

	return pending
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
}

// flushAllIDEX clears all ID/EX pipeline registers.
//
//nolint:unused // Scaffolding for 4-wide implementation (PR #114)
func (p *Pipeline) flushAllIDEX() {
	p.idex.Clear()
	p.idex2.Clear()
	p.idex3.Clear()
	p.idex4.Clear()
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
	p.pc = 0
	p.stats = Statistics{}
	p.halted = false
	p.exLatency = 0
	p.exLatency2 = 0
	p.exLatency3 = 0
	p.exLatency4 = 0
	p.memPending = false
	p.memPendingPC = 0
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
