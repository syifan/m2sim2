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

// Statistics holds pipeline performance statistics.
type Statistics struct {
	// Cycles is the total number of cycles simulated.
	Cycles uint64
	// Instructions is the number of instructions completed (retired).
	Instructions uint64
	// Stalls is the number of stall cycles.
	Stalls uint64
	// Flushes is the number of pipeline flushes (due to branches).
	Flushes uint64
	// ExecStalls is the number of stalls due to multi-cycle execution.
	ExecStalls uint64
	// MemStalls is the number of stalls due to memory latency.
	MemStalls uint64
	// DataHazards is the number of RAW data hazards detected.
	DataHazards uint64
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

	// Instruction timing
	latencyTable *latency.Table
	exLatency    uint64 // Remaining cycles for execute stage
	exLatency2   uint64 // Remaining cycles for secondary execute slot

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
//   - Branch flushes when a taken branch is detected in EX stage
//
// Stages are evaluated in reverse order (WB→MEM→EX→ID→IF) to compute new
// values before latching them into pipeline registers at cycle end.
func (p *Pipeline) Tick() {
	// Don't execute if halted
	if p.halted {
		return
	}

	p.stats.Cycles++

	// Use superscalar tick if dual-issue is enabled
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

	branchTaken := false
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

			if execResult.BranchTaken {
				branchTaken = true
				branchTarget = execResult.BranchTarget
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
	stallResult := p.hazardUnit.ComputeStalls(loadUseHazard || execStall || memStall, branchTaken)

	// Stage 2: Decode
	var nextIDEX IDEXRegister
	if p.ifid.Valid && !stallResult.StallID && !stallResult.FlushID && !execStall {
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
	} else if (stallResult.StallID || execStall || memStall) && !stallResult.FlushID {
		nextIDEX = p.idex
	}

	// Stage 1: Fetch
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
			nextIFID = IFIDRegister{
				Valid:           true,
				PC:              p.pc,
				InstructionWord: word,
			}
			p.pc += 4
		} else if fetchStall {
			nextIFID = p.ifid
		}
	} else if (stallResult.StallIF || memStall) && !stallResult.FlushIF {
		nextIFID = p.ifid
		p.stats.Stalls++
	}

	if branchTaken {
		p.pc = branchTarget
		nextIFID.Clear()
		nextIDEX.Clear()
		p.stats.Flushes++
	}

	if !memStall {
		p.memwb = nextMEMWB
	} else {
		p.memwb.Clear()
	}
	if !execStall && !memStall {
		p.exmem = nextEXMEM
	}
	if stallResult.InsertBubbleEX && !execStall && !memStall {
		p.idex.Clear()
	} else if !memStall {
		p.idex = nextIDEX
	}
	p.ifid = nextIFID
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
		}
	}

	// Execute secondary slot (if not stalled and slot is valid)
	if p.idex2.Valid && !memStall && !execStall {
		// Convert to IDEXRegister for hazard detection
		idex2 := p.idex2.toIDEX()

		// Detect forwarding for secondary slot
		forwarding2 := p.hazardUnit.DetectForwarding(&idex2, &p.exmem, &p.memwb)

		// Also check forwarding from primary execute result
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
			rnValue := p.idex2.RnValue
			rmValue := p.idex2.RmValue

			// Apply forwarding from primary execute stage if needed
			if nextEXMEM.Valid && nextEXMEM.RegWrite && nextEXMEM.Rd != 31 {
				if p.idex2.Rn == nextEXMEM.Rd {
					rnValue = nextEXMEM.ALUResult
				}
				if p.idex2.Rm == nextEXMEM.Rd {
					rmValue = nextEXMEM.ALUResult
				}
			} else {
				rnValue = p.hazardUnit.GetForwardedValue(
					forwarding2.ForwardRn, p.idex2.RnValue, &p.exmem, &savedMEMWB)
				rmValue = p.hazardUnit.GetForwardedValue(
					forwarding2.ForwardRm, p.idex2.RmValue, &p.exmem, &savedMEMWB)
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

		// Decode secondary slot if available
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
			nextIFID = IFIDRegister{
				Valid:           true,
				PC:              p.ifid2.PC,
				InstructionWord: p.ifid2.InstructionWord,
			}
			// Fetch a new second instruction
			var word2 uint32
			var ok2 bool
			if p.useICache && p.cachedFetchStage != nil {
				word2, ok2, _ = p.cachedFetchStage.Fetch(p.pc)
			} else {
				word2, ok2 = p.fetchStage.Fetch(p.pc)
			}
			if ok2 {
				nextIFID2 = SecondaryIFIDRegister{
					Valid:           true,
					PC:              p.pc,
					InstructionWord: word2,
				}
				p.pc += 4
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
				nextIFID = IFIDRegister{
					Valid:           true,
					PC:              p.pc,
					InstructionWord: word,
				}

				// Fetch second instruction for dual-issue
				var word2 uint32
				var ok2 bool
				if p.useICache && p.cachedFetchStage != nil {
					word2, ok2, _ = p.cachedFetchStage.Fetch(p.pc + 4)
				} else {
					word2, ok2 = p.fetchStage.Fetch(p.pc + 4)
				}

				if ok2 {
					nextIFID2 = SecondaryIFIDRegister{
						Valid:           true,
						PC:              p.pc + 4,
						InstructionWord: word2,
					}
					p.pc += 8 // Advance PC by 2 instructions
				} else {
					p.pc += 4
				}
			} else if fetchStall {
				nextIFID = p.ifid
				nextIFID2 = p.ifid2
			}
		}
	} else if (stallResult.StallIF || memStall || execStall) && !stallResult.FlushIF {
		nextIFID = p.ifid
		nextIFID2 = p.ifid2
		p.stats.Stalls++
	}

	// Latch all pipeline registers
	if !memStall {
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
	p.pc = 0
	p.stats = Statistics{}
	p.halted = false
	p.exLatency = 0
	p.exLatency2 = 0
	p.memPending = false
	p.memPendingPC = 0
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
