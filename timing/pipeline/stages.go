// Package pipeline provides a 5-stage pipeline model for cycle-accurate timing simulation.
package pipeline

import (
	"github.com/sarchlab/m2sim/emu"
	"github.com/sarchlab/m2sim/insts"
)

// FetchStage handles instruction fetch from memory.
type FetchStage struct {
	memory  *emu.Memory
	decoder *insts.Decoder
}

// NewFetchStage creates a new fetch stage.
func NewFetchStage(memory *emu.Memory) *FetchStage {
	return &FetchStage{
		memory:  memory,
		decoder: insts.NewDecoder(),
	}
}

// Fetch reads the instruction at the given PC.
func (s *FetchStage) Fetch(pc uint64) (uint32, bool) {
	word := s.memory.Read32(pc)
	return word, true
}

// DecodeStage handles instruction decode and register read.
type DecodeStage struct {
	regFile *emu.RegFile
	decoder *insts.Decoder
}

// NewDecodeStage creates a new decode stage.
func NewDecodeStage(regFile *emu.RegFile) *DecodeStage {
	return &DecodeStage{
		regFile: regFile,
		decoder: insts.NewDecoder(),
	}
}

// DecodeResult holds the result of the decode stage.
type DecodeResult struct {
	Inst    *insts.Instruction
	RnValue uint64
	RmValue uint64

	// Destination and source registers.
	Rd uint8
	Rn uint8
	Rm uint8

	// Control signals.
	MemRead   bool
	MemWrite  bool
	RegWrite  bool
	MemToReg  bool
	IsBranch  bool
	IsSyscall bool

	// For branches.
	BranchTarget uint64
	BranchTaken  bool
}

// Decode decodes the instruction and reads register values.
func (s *DecodeStage) Decode(word uint32, pc uint64) DecodeResult {
	inst := s.decoder.Decode(word)
	result := DecodeResult{
		Inst: inst,
		Rd:   inst.Rd,
		Rn:   inst.Rn,
		Rm:   inst.Rm,
	}

	// Read register values.
	result.RnValue = s.regFile.ReadReg(inst.Rn)
	result.RmValue = s.regFile.ReadReg(inst.Rm)

	// Set control signals based on instruction type.
	switch inst.Format {
	case insts.FormatDPImm, insts.FormatDPReg:
		result.RegWrite = inst.Rd != 31 // Don't write to XZR
	case insts.FormatLoadStore:
		if inst.Op == insts.OpLDR {
			result.MemRead = true
			result.MemToReg = true
			result.RegWrite = inst.Rd != 31
		} else if inst.Op == insts.OpSTR {
			result.MemWrite = true
			// For STR, Rd is actually the source register (Rt)
			result.RmValue = s.regFile.ReadReg(inst.Rd)
		}
	case insts.FormatBranch, insts.FormatBranchCond, insts.FormatBranchReg:
		result.IsBranch = true
		if inst.Op == insts.OpBL || inst.Op == insts.OpBLR {
			// BL/BLR write to X30
			result.RegWrite = true
			result.Rd = 30
		}
	case insts.FormatException:
		if inst.Op == insts.OpSVC {
			result.IsSyscall = true
		}
	}

	return result
}

// ExecuteStage handles ALU operations and address calculation.
type ExecuteStage struct {
	regFile *emu.RegFile
}

// NewExecuteStage creates a new execute stage.
func NewExecuteStage(regFile *emu.RegFile) *ExecuteStage {
	return &ExecuteStage{
		regFile: regFile,
	}
}

// ExecuteResult holds the result of the execute stage.
type ExecuteResult struct {
	ALUResult  uint64
	StoreValue uint64

	// Branch result.
	BranchTaken  bool
	BranchTarget uint64
}

// Execute performs ALU operations or address calculation.
func (s *ExecuteStage) Execute(idex *IDEXRegister, forwardedRn, forwardedRm uint64) ExecuteResult {
	result := ExecuteResult{}
	inst := idex.Inst

	if inst == nil {
		return result
	}

	rnVal := forwardedRn
	rmVal := forwardedRm

	switch inst.Format {
	case insts.FormatDPImm:
		// Data processing with immediate.
		imm := inst.Imm
		if inst.Shift > 0 {
			imm <<= inst.Shift
		}

		switch inst.Op {
		case insts.OpADD:
			if inst.Is64Bit {
				result.ALUResult = rnVal + imm
			} else {
				result.ALUResult = uint64(uint32(rnVal) + uint32(imm))
			}
		case insts.OpSUB:
			if inst.Is64Bit {
				result.ALUResult = rnVal - imm
			} else {
				result.ALUResult = uint64(uint32(rnVal) - uint32(imm))
			}
		}

	case insts.FormatDPReg:
		// Data processing with register.
		switch inst.Op {
		case insts.OpADD:
			if inst.Is64Bit {
				result.ALUResult = rnVal + rmVal
			} else {
				result.ALUResult = uint64(uint32(rnVal) + uint32(rmVal))
			}
		case insts.OpSUB:
			if inst.Is64Bit {
				result.ALUResult = rnVal - rmVal
			} else {
				result.ALUResult = uint64(uint32(rnVal) - uint32(rmVal))
			}
		case insts.OpAND:
			if inst.Is64Bit {
				result.ALUResult = rnVal & rmVal
			} else {
				result.ALUResult = uint64(uint32(rnVal) & uint32(rmVal))
			}
		case insts.OpORR:
			if inst.Is64Bit {
				result.ALUResult = rnVal | rmVal
			} else {
				result.ALUResult = uint64(uint32(rnVal) | uint32(rmVal))
			}
		case insts.OpEOR:
			if inst.Is64Bit {
				result.ALUResult = rnVal ^ rmVal
			} else {
				result.ALUResult = uint64(uint32(rnVal) ^ uint32(rmVal))
			}
		}

	case insts.FormatLoadStore:
		// Address calculation.
		baseAddr := rnVal
		if inst.Rn == 31 {
			// Use SP for base.
			baseAddr = s.regFile.SP
		}
		result.ALUResult = baseAddr + inst.Imm
		result.StoreValue = rmVal // For STR (Rm holds Rt value from decode)

	case insts.FormatBranch:
		switch inst.Op {
		case insts.OpB:
			result.BranchTaken = true
			result.BranchTarget = uint64(int64(idex.PC) + inst.BranchOffset)
			result.ALUResult = idex.PC + 4 // Not used
		case insts.OpBL:
			result.BranchTaken = true
			result.BranchTarget = uint64(int64(idex.PC) + inst.BranchOffset)
			result.ALUResult = idex.PC + 4 // Return address
		}

	case insts.FormatBranchCond:
		// Check condition.
		condMet := s.checkCondition(inst.Cond)
		if condMet {
			result.BranchTaken = true
			result.BranchTarget = uint64(int64(idex.PC) + inst.BranchOffset)
		}

	case insts.FormatBranchReg:
		switch inst.Op {
		case insts.OpBR:
			result.BranchTaken = true
			result.BranchTarget = rnVal
		case insts.OpBLR:
			result.BranchTaken = true
			result.BranchTarget = rnVal
			result.ALUResult = idex.PC + 4 // Return address
		case insts.OpRET:
			result.BranchTaken = true
			result.BranchTarget = rnVal // Usually X30
		}
	}

	return result
}

// checkCondition evaluates the branch condition against PSTATE flags.
func (s *ExecuteStage) checkCondition(cond insts.Cond) bool {
	pstate := s.regFile.PSTATE
	n := pstate.N
	z := pstate.Z
	c := pstate.C
	v := pstate.V

	switch cond {
	case insts.CondEQ:
		return z
	case insts.CondNE:
		return !z
	case insts.CondCS:
		return c
	case insts.CondCC:
		return !c
	case insts.CondMI:
		return n
	case insts.CondPL:
		return !n
	case insts.CondVS:
		return v
	case insts.CondVC:
		return !v
	case insts.CondHI:
		return c && !z
	case insts.CondLS:
		return !c || z
	case insts.CondGE:
		return n == v
	case insts.CondLT:
		return n != v
	case insts.CondGT:
		return !z && (n == v)
	case insts.CondLE:
		return z || (n != v)
	case insts.CondAL, insts.CondNV:
		return true
	default:
		return false
	}
}

// MemoryStage handles memory load/store operations.
type MemoryStage struct {
	memory *emu.Memory
}

// NewMemoryStage creates a new memory stage.
func NewMemoryStage(memory *emu.Memory) *MemoryStage {
	return &MemoryStage{
		memory: memory,
	}
}

// MemoryResult holds the result of the memory stage.
type MemoryResult struct {
	MemData uint64
}

// Access performs memory read or write.
func (s *MemoryStage) Access(exmem *EXMEMRegister) MemoryResult {
	result := MemoryResult{}

	if !exmem.Valid {
		return result
	}

	if exmem.MemRead {
		// Load instruction.
		if exmem.Inst != nil && exmem.Inst.Is64Bit {
			result.MemData = s.memory.Read64(exmem.ALUResult)
		} else {
			result.MemData = uint64(s.memory.Read32(exmem.ALUResult))
		}
	} else if exmem.MemWrite {
		// Store instruction.
		if exmem.Inst != nil && exmem.Inst.Is64Bit {
			s.memory.Write64(exmem.ALUResult, exmem.StoreValue)
		} else {
			s.memory.Write32(exmem.ALUResult, uint32(exmem.StoreValue))
		}
	}

	return result
}

// WritebackStage handles register file writeback.
type WritebackStage struct {
	regFile *emu.RegFile
}

// NewWritebackStage creates a new writeback stage.
func NewWritebackStage(regFile *emu.RegFile) *WritebackStage {
	return &WritebackStage{
		regFile: regFile,
	}
}

// Writeback writes the result to the register file.
func (s *WritebackStage) Writeback(memwb *MEMWBRegister) {
	if !memwb.Valid || !memwb.RegWrite {
		return
	}

	if memwb.Rd == 31 {
		return // Don't write to XZR
	}

	var value uint64
	if memwb.MemToReg {
		value = memwb.MemData
	} else {
		value = memwb.ALUResult
	}

	s.regFile.WriteReg(memwb.Rd, value)
}
