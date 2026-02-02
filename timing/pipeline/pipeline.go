// Package pipeline provides a 5-stage pipeline model for cycle-accurate timing simulation.
//
// The pipeline implements the classic 5-stage design:
//   - Fetch (IF): Read instruction from memory
//   - Decode (ID): Decode instruction, read registers
//   - Execute (EX): ALU operations, address calculation
//   - Memory (MEM): Load/Store memory access
//   - Writeback (WB): Write results to register file
//
// Features:
//   - Pipeline registers between stages (IF/ID, ID/EX, EX/MEM, MEM/WB)
//   - Hazard detection for RAW (Read-After-Write) dependencies
//   - Data forwarding from EX/MEM and MEM/WB stages
//   - Stalling for load-use hazards
//   - Pipeline flushing for branches
package pipeline

import (
	"github.com/sarchlab/m2sim/emu"
	"github.com/sarchlab/m2sim/insts"
)

// Pipeline represents a 5-stage instruction pipeline.
type Pipeline struct {
	// Pipeline stages.
	fetchStage     *FetchStage
	decodeStage    *DecodeStage
	executeStage   *ExecuteStage
	memoryStage    *MemoryStage
	writebackStage *WritebackStage

	// Pipeline registers.
	ifid  IFIDRegister
	idex  IDEXRegister
	exmem EXMEMRegister
	memwb MEMWBRegister

	// Next-cycle pipeline registers (for synchronous update).
	nextIfid  IFIDRegister
	nextIdex  IDEXRegister
	nextExmem EXMEMRegister
	nextMemwb MEMWBRegister

	// Hazard detection unit.
	hazardUnit *HazardUnit

	// Processor state.
	regFile *emu.RegFile
	memory  *emu.Memory
	pc      uint64

	// Statistics.
	cycleCount       uint64
	instructionCount uint64
	stallCount       uint64
	branchCount      uint64
	flushCount       uint64

	// Execution state.
	halted   bool
	exitCode int64

	// Syscall handler for SVC instructions.
	syscallHandler emu.SyscallHandler
}

// PipelineOption is a functional option for configuring the Pipeline.
type PipelineOption func(*Pipeline)

// WithSyscallHandler sets a custom syscall handler.
func WithSyscallHandler(handler emu.SyscallHandler) PipelineOption {
	return func(p *Pipeline) {
		p.syscallHandler = handler
	}
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
	}

	// Apply options.
	for _, opt := range opts {
		opt(p)
	}

	// Create default syscall handler if not provided.
	if p.syscallHandler == nil {
		p.syscallHandler = emu.NewDefaultSyscallHandler(regFile, memory, nil, nil)
	}

	return p
}

// SetPC sets the program counter (entry point).
func (p *Pipeline) SetPC(pc uint64) {
	p.pc = pc
	p.regFile.PC = pc
}

// PC returns the current program counter.
func (p *Pipeline) PC() uint64 {
	return p.pc
}

// Halted returns true if the pipeline has halted (program exited).
func (p *Pipeline) Halted() bool {
	return p.halted
}

// ExitCode returns the exit code if halted.
func (p *Pipeline) ExitCode() int64 {
	return p.exitCode
}

// Stats returns pipeline statistics.
type Stats struct {
	Cycles       uint64
	Instructions uint64
	Stalls       uint64
	Branches     uint64
	Flushes      uint64
	CPI          float64 // Cycles per instruction
}

// Stats returns pipeline performance statistics.
func (p *Pipeline) Stats() Stats {
	s := Stats{
		Cycles:       p.cycleCount,
		Instructions: p.instructionCount,
		Stalls:       p.stallCount,
		Branches:     p.branchCount,
		Flushes:      p.flushCount,
	}
	if s.Instructions > 0 {
		s.CPI = float64(s.Cycles) / float64(s.Instructions)
	}
	return s
}

// Tick advances the pipeline by one cycle.
// This is the main simulation entry point.
func (p *Pipeline) Tick() {
	if p.halted {
		return
	}

	p.cycleCount++

	// Execute all stages in reverse order (to properly update pipeline registers).
	// Each stage reads from current registers and writes to next registers.

	// 1. Writeback stage (WB).
	p.doWriteback()

	// 2. Memory stage (MEM).
	p.doMemory()

	// 3. Execute stage (EX).
	branchTaken, branchTarget := p.doExecute()

	// 4. Decode stage (ID).
	loadUseHazard := p.doDecode()

	// 5. Fetch stage (IF).
	p.doFetch()

	// Compute stalls and flushes.
	stallResult := p.hazardUnit.ComputeStalls(loadUseHazard, branchTaken)

	// Apply stall/flush logic.
	if stallResult.StallIF || stallResult.StallID {
		p.stallCount++
	}

	if stallResult.InsertBubbleEX {
		// Insert bubble into EX stage (clear ID/EX).
		p.nextIdex.Clear()
	}

	if branchTaken {
		p.branchCount++
		p.flushCount++
		// Flush IF/ID and update PC.
		p.nextIfid.Clear()
		p.nextIdex.Clear()
		p.pc = branchTarget
	}

	if stallResult.StallIF {
		// Don't update IF/ID - keep fetching the same instruction.
		p.nextIfid = p.ifid
		// Don't advance PC.
	}

	if stallResult.StallID {
		// Don't update ID/EX - keep same decoded instruction.
		p.nextIdex = p.idex
	}

	// Update pipeline registers synchronously.
	p.ifid = p.nextIfid
	p.idex = p.nextIdex
	p.exmem = p.nextExmem
	p.memwb = p.nextMemwb

	// Advance PC for next fetch (if not stalled or branching).
	if !stallResult.StallIF && !branchTaken {
		p.pc += 4
	}
}

// doFetch performs the fetch stage.
func (p *Pipeline) doFetch() {
	word, ok := p.fetchStage.Fetch(p.pc)
	if !ok {
		p.nextIfid.Clear()
		return
	}

	p.nextIfid.Valid = true
	p.nextIfid.PC = p.pc
	p.nextIfid.InstructionWord = word
}

// doDecode performs the decode stage.
// Returns true if a load-use hazard is detected.
func (p *Pipeline) doDecode() bool {
	if !p.ifid.Valid {
		p.nextIdex.Clear()
		return false
	}

	result := p.decodeStage.Decode(p.ifid.InstructionWord, p.ifid.PC)

	// Check for load-use hazard before populating ID/EX.
	loadUseHazard := false
	if p.idex.Valid && p.idex.MemRead && p.idex.Rd != 31 {
		// Check if the current decoded instruction uses the load destination.
		usesRn := result.Inst.Format == insts.FormatDPImm ||
			result.Inst.Format == insts.FormatDPReg ||
			result.Inst.Format == insts.FormatLoadStore ||
			result.Inst.Format == insts.FormatBranchReg

		usesRm := result.Inst.Format == insts.FormatDPReg

		loadUseHazard = p.hazardUnit.DetectLoadUseHazardDecoded(
			p.idex.Rd, result.Rn, result.Rm, usesRn, usesRm)
	}

	if loadUseHazard {
		// Don't update ID/EX - stall.
		return true
	}

	p.nextIdex.Valid = true
	p.nextIdex.PC = p.ifid.PC
	p.nextIdex.Inst = result.Inst
	p.nextIdex.RnValue = result.RnValue
	p.nextIdex.RmValue = result.RmValue
	p.nextIdex.Rd = result.Rd
	p.nextIdex.Rn = result.Rn
	p.nextIdex.Rm = result.Rm
	p.nextIdex.MemRead = result.MemRead
	p.nextIdex.MemWrite = result.MemWrite
	p.nextIdex.RegWrite = result.RegWrite
	p.nextIdex.MemToReg = result.MemToReg
	p.nextIdex.IsBranch = result.IsBranch
	p.nextIdex.IsSyscall = result.IsSyscall

	return false
}

// doExecute performs the execute stage.
// Returns whether a branch was taken and the target address.
func (p *Pipeline) doExecute() (branchTaken bool, branchTarget uint64) {
	if !p.idex.Valid {
		p.nextExmem.Clear()
		return false, 0
	}

	// Check for syscall.
	if p.idex.IsSyscall {
		// Handle syscall.
		syscallResult := p.syscallHandler.Handle()
		if syscallResult.Exited {
			p.halted = true
			p.exitCode = syscallResult.ExitCode
		}
		// Syscalls don't produce a normal result.
		p.nextExmem.Clear()
		p.instructionCount++
		return false, 0
	}

	// Get forwarding decisions.
	forwarding := p.hazardUnit.DetectForwarding(&p.idex, &p.exmem, &p.memwb)

	// Apply forwarding to get actual operand values.
	rnVal := p.hazardUnit.GetForwardedValue(forwarding.ForwardRn, p.idex.RnValue, &p.exmem, &p.memwb)
	rmVal := p.hazardUnit.GetForwardedValue(forwarding.ForwardRm, p.idex.RmValue, &p.exmem, &p.memwb)

	// Execute.
	result := p.executeStage.Execute(&p.idex, rnVal, rmVal)

	// Update EX/MEM register.
	p.nextExmem.Valid = true
	p.nextExmem.PC = p.idex.PC
	p.nextExmem.Inst = p.idex.Inst
	p.nextExmem.ALUResult = result.ALUResult
	p.nextExmem.StoreValue = result.StoreValue
	p.nextExmem.Rd = p.idex.Rd
	p.nextExmem.MemRead = p.idex.MemRead
	p.nextExmem.MemWrite = p.idex.MemWrite
	p.nextExmem.RegWrite = p.idex.RegWrite
	p.nextExmem.MemToReg = p.idex.MemToReg

	return result.BranchTaken, result.BranchTarget
}

// doMemory performs the memory stage.
func (p *Pipeline) doMemory() {
	if !p.exmem.Valid {
		p.nextMemwb.Clear()
		return
	}

	result := p.memoryStage.Access(&p.exmem)

	// Update MEM/WB register.
	p.nextMemwb.Valid = true
	p.nextMemwb.PC = p.exmem.PC
	p.nextMemwb.Inst = p.exmem.Inst
	p.nextMemwb.ALUResult = p.exmem.ALUResult
	p.nextMemwb.MemData = result.MemData
	p.nextMemwb.Rd = p.exmem.Rd
	p.nextMemwb.RegWrite = p.exmem.RegWrite
	p.nextMemwb.MemToReg = p.exmem.MemToReg
}

// doWriteback performs the writeback stage.
func (p *Pipeline) doWriteback() {
	if !p.memwb.Valid {
		return
	}

	p.writebackStage.Writeback(&p.memwb)
	p.instructionCount++
}

// Run executes the pipeline until the program halts.
// Returns the exit code.
func (p *Pipeline) Run() int64 {
	for !p.halted {
		p.Tick()
	}
	return p.exitCode
}

// RunCycles executes the pipeline for a specified number of cycles.
// Returns true if still running, false if halted.
func (p *Pipeline) RunCycles(n uint64) bool {
	for i := uint64(0); i < n && !p.halted; i++ {
		p.Tick()
	}
	return !p.halted
}

// GetIFID returns the current IF/ID register for inspection.
func (p *Pipeline) GetIFID() IFIDRegister {
	return p.ifid
}

// GetIDEX returns the current ID/EX register for inspection.
func (p *Pipeline) GetIDEX() IDEXRegister {
	return p.idex
}

// GetEXMEM returns the current EX/MEM register for inspection.
func (p *Pipeline) GetEXMEM() EXMEMRegister {
	return p.exmem
}

// GetMEMWB returns the current MEM/WB register for inspection.
func (p *Pipeline) GetMEMWB() MEMWBRegister {
	return p.memwb
}
