package pipeline

import (
	"github.com/sarchlab/m2sim/emu"
	"github.com/sarchlab/m2sim/insts"
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

// Pipeline implements a 5-stage pipelined CPU model.
// Stages: Fetch (IF) -> Decode (ID) -> Execute (EX) -> Memory (MEM) -> Writeback (WB)
type Pipeline struct {
	// Pipeline registers
	ifid  IFIDRegister
	idex  IDEXRegister
	exmem EXMEMRegister
	memwb MEMWBRegister

	// Pipeline stages
	fetchStage     *FetchStage
	decodeStage    *DecodeStage
	executeStage   *ExecuteStage
	memoryStage    *MemoryStage
	writebackStage *WritebackStage

	// Hazard detection
	hazardUnit *HazardUnit

	// Shared resources
	regFile *emu.RegFile
	memory  *emu.Memory

	// Syscall handling
	syscallHandler emu.SyscallHandler

	// Program counter
	pc uint64

	// Statistics
	stats Statistics

	// Execution state
	halted   bool
	exitCode int64
}

// NewPipeline creates a new 5-stage pipeline.
func NewPipeline(regFile *emu.RegFile, memory *emu.Memory, opts ...PipelineOption) *Pipeline {
	p := &Pipeline{
		fetchStage:     NewFetchStage(memory),
		decodeStage:    NewDecodeStage(regFile),
		executeStage:   NewExecuteStage(regFile),
		memoryStage:    NewMemoryStage(memory),
		writebackStage: NewWritebackStage(regFile),
		hazardUnit:     NewHazardUnit(),
		regFile:        regFile,
		memory:         memory,
		halted:         false,
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
// All stages execute in parallel, with results latched at the end.
func (p *Pipeline) Tick() {
	// Don't execute if halted
	if p.halted {
		return
	}

	p.stats.Cycles++

	// Detect hazards before executing stages
	forwarding := p.hazardUnit.DetectForwarding(&p.idex, &p.exmem, &p.memwb)

	// Detect load-use hazard (checking ID/EX against IF/ID's decoded instruction)
	loadUseHazard := false
	if p.idex.Valid && p.idex.MemRead && p.ifid.Valid {
		// Peek at the next instruction to check for hazard
		nextInst := p.decodeStage.decoder.Decode(p.ifid.InstructionWord)
		if nextInst != nil && nextInst.Op != insts.OpUnknown {
			// Check if the load destination conflicts with next instruction's sources
			loadUseHazard = p.hazardUnit.DetectLoadUseHazardDecoded(
				p.idex.Rd,
				nextInst.Rn,
				nextInst.Rm,
				true, // Most instructions use Rn
				nextInst.Format == insts.FormatDPReg, // Only register format uses Rm
			)
		}
	}

	// Track if branch is taken this cycle
	branchTaken := false
	var branchTarget uint64

	// Execute stages in reverse order (WB -> MEM -> EX -> ID -> IF)
	// This allows us to compute new values before latching

	// Stage 5: Writeback
	newMEMWB := p.memwb // Save for forwarding
	p.writebackStage.Writeback(&p.memwb)
	if p.memwb.Valid {
		p.stats.Instructions++
	}

	// Stage 4: Memory
	var nextMEMWB MEMWBRegister
	if p.exmem.Valid {
		// Check for syscall in memory stage
		if p.exmem.Inst != nil && p.exmem.Inst.Op == insts.OpSVC {
			if p.syscallHandler != nil {
				result := p.syscallHandler.Handle()
				if result.Exited {
					p.halted = true
					p.exitCode = result.ExitCode
				}
			}
		}

		memResult := p.memoryStage.Access(&p.exmem)
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

	// Stage 3: Execute
	var nextEXMEM EXMEMRegister
	if p.idex.Valid {
		// Apply forwarding to get correct operand values
		rnValue := p.hazardUnit.GetForwardedValue(
			forwarding.ForwardRn, p.idex.RnValue, &p.exmem, &newMEMWB)
		rmValue := p.hazardUnit.GetForwardedValue(
			forwarding.ForwardRm, p.idex.RmValue, &p.exmem, &newMEMWB)

		execResult := p.executeStage.Execute(&p.idex, rnValue, rmValue)

		// Check for branch taken
		if execResult.BranchTaken {
			branchTaken = true
			branchTarget = execResult.BranchTarget
		}

		// For store instructions, we need the value from Rd (which is actually Rt)
		storeValue := execResult.StoreValue
		if p.idex.MemWrite {
			// Store value comes from Rd register
			storeValue = p.regFile.ReadReg(p.idex.Rd)
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

	// Compute stall signals
	stallResult := p.hazardUnit.ComputeStalls(loadUseHazard, branchTaken)

	// Stage 2: Decode
	var nextIDEX IDEXRegister
	if p.ifid.Valid && !stallResult.StallID && !stallResult.FlushID {
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
	} else if stallResult.StallID && !stallResult.FlushID {
		// Keep the current ID/EX contents during stall
		nextIDEX = p.idex
	}

	// Stage 1: Fetch
	var nextIFID IFIDRegister
	if !stallResult.StallIF && !stallResult.FlushIF {
		word, ok := p.fetchStage.Fetch(p.pc)
		if ok {
			nextIFID = IFIDRegister{
				Valid:           true,
				PC:              p.pc,
				InstructionWord: word,
			}
			p.pc += 4 // Advance PC
		}
	} else if stallResult.StallIF && !stallResult.FlushIF {
		// Keep the current IF/ID contents during stall
		nextIFID = p.ifid
		p.stats.Stalls++
	}

	// Handle branch: update PC and flush pipeline
	if branchTaken {
		p.pc = branchTarget
		// Flush IF and ID stages (clear their pipeline registers)
		nextIFID.Clear()
		nextIDEX.Clear()
		p.stats.Flushes++
	}

	// Latch all pipeline registers at the end of the cycle
	p.memwb = nextMEMWB
	p.exmem = nextEXMEM
	if stallResult.InsertBubbleEX {
		p.idex.Clear() // Insert bubble
	} else {
		p.idex = nextIDEX
	}
	p.ifid = nextIFID
}

// Reset clears all pipeline state.
func (p *Pipeline) Reset() {
	p.ifid.Clear()
	p.idex.Clear()
	p.exmem.Clear()
	p.memwb.Clear()
	p.pc = 0
	p.stats = Statistics{}
	p.halted = false
}
