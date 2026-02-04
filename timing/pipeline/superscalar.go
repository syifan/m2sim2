// Package pipeline provides the 5-stage pipeline implementation for timing simulation.
package pipeline

import "github.com/sarchlab/m2sim/insts"

// SuperscalarConfig controls the superscalar execution width.
type SuperscalarConfig struct {
	// IssueWidth is the maximum number of instructions that can be issued per cycle.
	// Default is 1 (single-issue). Set to 2 for dual-issue.
	IssueWidth int
}

// DefaultSuperscalarConfig returns the default superscalar configuration (single-issue).
func DefaultSuperscalarConfig() SuperscalarConfig {
	return SuperscalarConfig{
		IssueWidth: 1,
	}
}

// DualIssueConfig returns a dual-issue superscalar configuration.
func DualIssueConfig() SuperscalarConfig {
	return SuperscalarConfig{
		IssueWidth: 2,
	}
}

// WithSuperscalar sets the superscalar configuration.
func WithSuperscalar(config SuperscalarConfig) PipelineOption {
	return func(p *Pipeline) {
		p.superscalarConfig = config
	}
}

// WithDualIssue enables dual-issue superscalar execution.
func WithDualIssue() PipelineOption {
	return func(p *Pipeline) {
		p.superscalarConfig = DualIssueConfig()
	}
}

// canDualIssue checks if two decoded instructions can be issued together.
// Returns true if the instructions have no data dependencies and can execute in parallel.
func canDualIssue(first, second *IDEXRegister) bool {
	if first == nil || second == nil || !first.Valid || !second.Valid {
		return false
	}

	// Cannot dual-issue if either is a branch
	if first.IsBranch || second.IsBranch {
		return false
	}

	// Cannot dual-issue syscalls
	if first.Inst != nil && first.Inst.Op == insts.OpSVC {
		return false
	}
	if second.Inst != nil && second.Inst.Op == insts.OpSVC {
		return false
	}

	// Cannot dual-issue if both access memory (single memory port)
	if (first.MemRead || first.MemWrite) && (second.MemRead || second.MemWrite) {
		return false
	}

	// Check for RAW hazard: second reads register that first writes
	if first.RegWrite && first.Rd != 31 {
		// Second instruction uses first's destination as source
		if second.Rn == first.Rd && first.Rd != 31 {
			return false
		}
		if second.Rm == first.Rd && first.Rd != 31 {
			return false
		}
		// For stores, the value register might also conflict
		if second.MemWrite && second.Inst != nil && second.Inst.Rd == first.Rd {
			return false
		}
	}

	// Check for WAW hazard: both write to same register
	if first.RegWrite && second.RegWrite && first.Rd == second.Rd && first.Rd != 31 {
		return false
	}

	return true
}

// SecondaryIFIDRegister holds the second fetched instruction for dual-issue.
type SecondaryIFIDRegister struct {
	Valid           bool
	PC              uint64
	InstructionWord uint32
}

// SecondaryIDEXRegister holds the second decoded instruction for dual-issue.
type SecondaryIDEXRegister struct {
	Valid    bool
	PC       uint64
	Inst     *insts.Instruction
	RnValue  uint64
	RmValue  uint64
	Rd       uint8
	Rn       uint8
	Rm       uint8
	MemRead  bool
	MemWrite bool
	RegWrite bool
	MemToReg bool
	IsBranch bool
}

// SecondaryEXMEMRegister holds the second execute result for dual-issue.
type SecondaryEXMEMRegister struct {
	Valid      bool
	PC         uint64
	Inst       *insts.Instruction
	ALUResult  uint64
	StoreValue uint64
	Rd         uint8
	MemRead    bool
	MemWrite   bool
	RegWrite   bool
	MemToReg   bool
}

// SecondaryMEMWBRegister holds the second memory result for dual-issue.
type SecondaryMEMWBRegister struct {
	Valid     bool
	PC        uint64
	Inst      *insts.Instruction
	ALUResult uint64
	MemData   uint64
	Rd        uint8
	RegWrite  bool
	MemToReg  bool
}

// Clear resets the secondary IF/ID register.
func (r *SecondaryIFIDRegister) Clear() {
	r.Valid = false
	r.PC = 0
	r.InstructionWord = 0
}

// Clear resets the secondary ID/EX register.
func (r *SecondaryIDEXRegister) Clear() {
	r.Valid = false
	r.PC = 0
	r.Inst = nil
	r.RnValue = 0
	r.RmValue = 0
	r.Rd = 0
	r.Rn = 0
	r.Rm = 0
	r.MemRead = false
	r.MemWrite = false
	r.RegWrite = false
	r.MemToReg = false
	r.IsBranch = false
}

// Clear resets the secondary EX/MEM register.
func (r *SecondaryEXMEMRegister) Clear() {
	r.Valid = false
	r.PC = 0
	r.Inst = nil
	r.ALUResult = 0
	r.StoreValue = 0
	r.Rd = 0
	r.MemRead = false
	r.MemWrite = false
	r.RegWrite = false
	r.MemToReg = false
}

// Clear resets the secondary MEM/WB register.
func (r *SecondaryMEMWBRegister) Clear() {
	r.Valid = false
	r.PC = 0
	r.Inst = nil
	r.ALUResult = 0
	r.MemData = 0
	r.Rd = 0
	r.RegWrite = false
	r.MemToReg = false
}

// toIDEX converts SecondaryIDEXRegister to IDEXRegister for use with existing hazard/execute logic.
func (r *SecondaryIDEXRegister) toIDEX() IDEXRegister {
	return IDEXRegister{
		Valid:    r.Valid,
		PC:       r.PC,
		Inst:     r.Inst,
		RnValue:  r.RnValue,
		RmValue:  r.RmValue,
		Rd:       r.Rd,
		Rn:       r.Rn,
		Rm:       r.Rm,
		MemRead:  r.MemRead,
		MemWrite: r.MemWrite,
		RegWrite: r.RegWrite,
		MemToReg: r.MemToReg,
		IsBranch: r.IsBranch,
	}
}

// fromIDEX populates SecondaryIDEXRegister from IDEXRegister.
func (r *SecondaryIDEXRegister) fromIDEX(idex *IDEXRegister) {
	r.Valid = idex.Valid
	r.PC = idex.PC
	r.Inst = idex.Inst
	r.RnValue = idex.RnValue
	r.RmValue = idex.RmValue
	r.Rd = idex.Rd
	r.Rn = idex.Rn
	r.Rm = idex.Rm
	r.MemRead = idex.MemRead
	r.MemWrite = idex.MemWrite
	r.RegWrite = idex.RegWrite
	r.MemToReg = idex.MemToReg
	r.IsBranch = idex.IsBranch
}
