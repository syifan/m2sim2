package pipeline

import (
	"github.com/sarchlab/m2sim/emu"
	"github.com/sarchlab/m2sim/insts"
)

// FetchStage reads instructions from memory.
type FetchStage struct {
	memory *emu.Memory
}

// NewFetchStage creates a new fetch stage.
func NewFetchStage(memory *emu.Memory) *FetchStage {
	return &FetchStage{memory: memory}
}

// Fetch fetches an instruction word from memory at the given PC.
// Returns the instruction word and whether the fetch was successful.
func (s *FetchStage) Fetch(pc uint64) (uint32, bool) {
	word := s.memory.Read32(pc)
	return word, true
}

// DecodeStage decodes instructions and reads register values.
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

// DecodeResult contains the output of the decode stage.
type DecodeResult struct {
	Inst      *insts.Instruction
	RnValue   uint64
	RmValue   uint64
	Rd        uint8
	Rn        uint8
	Rm        uint8
	MemRead   bool
	MemWrite  bool
	RegWrite  bool
	MemToReg  bool
	IsBranch  bool
	IsSyscall bool
}

// Decode decodes an instruction word and reads register values.
func (s *DecodeStage) Decode(word uint32, pc uint64) DecodeResult {
	inst := s.decoder.Decode(word)

	result := DecodeResult{
		Inst: inst,
		Rd:   inst.Rd,
		Rn:   inst.Rn,
		Rm:   inst.Rm,
	}

	// For BL/BLR, the destination is always X30 (link register)
	if inst.Op == insts.OpBL || inst.Op == insts.OpBLR {
		result.Rd = 30
	}

	// Read register values
	result.RnValue = s.regFile.ReadReg(inst.Rn)
	result.RmValue = s.regFile.ReadReg(inst.Rm)

	// Determine control signals based on instruction type
	result.RegWrite = s.isRegWriteInst(inst)
	result.MemRead = s.isLoadOp(inst.Op)
	result.MemWrite = s.isStoreOp(inst.Op)
	result.MemToReg = s.isLoadOp(inst.Op)
	result.IsBranch = s.isBranchInst(inst)
	result.IsSyscall = inst.Op == insts.OpSVC

	return result
}

// isLoadOp returns true if the opcode is a load operation.
func (s *DecodeStage) isLoadOp(op insts.Op) bool {
	switch op {
	case insts.OpLDR, insts.OpLDP, insts.OpLDRB, insts.OpLDRSB,
		insts.OpLDRH, insts.OpLDRSH, insts.OpLDRLit, insts.OpLDRQ:
		return true
	default:
		return false
	}
}

// isStoreOp returns true if the opcode is a store operation.
func (s *DecodeStage) isStoreOp(op insts.Op) bool {
	switch op {
	case insts.OpSTR, insts.OpSTP, insts.OpSTRB, insts.OpSTRH, insts.OpSTRQ:
		return true
	default:
		return false
	}
}

// isRegWriteInst determines if the instruction writes to a register.
func (s *DecodeStage) isRegWriteInst(inst *insts.Instruction) bool {
	// Don't write if destination is XZR (register 31)
	if inst.Rd == 31 && inst.Op != insts.OpBL && inst.Op != insts.OpBLR {
		return false
	}

	switch inst.Op {
	case insts.OpADD, insts.OpSUB, insts.OpAND, insts.OpORR, insts.OpEOR:
		return true
	case insts.OpLDR, insts.OpLDP, insts.OpLDRB, insts.OpLDRSB,
		insts.OpLDRH, insts.OpLDRSH, insts.OpLDRLit, insts.OpLDRQ:
		return true
	case insts.OpBL, insts.OpBLR:
		return true // BL/BLR write to X30
	default:
		return false
	}
}

// isBranchInst determines if the instruction is a branch.
func (s *DecodeStage) isBranchInst(inst *insts.Instruction) bool {
	switch inst.Op {
	case insts.OpB, insts.OpBL, insts.OpBCond, insts.OpBR, insts.OpBLR, insts.OpRET:
		return true
	default:
		return false
	}
}

// ExecuteStage performs ALU operations.
type ExecuteStage struct {
	regFile *emu.RegFile
}

// NewExecuteStage creates a new execute stage.
func NewExecuteStage(regFile *emu.RegFile) *ExecuteStage {
	return &ExecuteStage{regFile: regFile}
}

// ExecuteResult contains the output of the execute stage.
type ExecuteResult struct {
	ALUResult    uint64
	StoreValue   uint64
	BranchTaken  bool
	BranchTarget uint64
}

// Execute performs the ALU operation for the instruction.
// rnValue and rmValue are the (possibly forwarded) operand values.
func (s *ExecuteStage) Execute(idex *IDEXRegister, rnValue, rmValue uint64) ExecuteResult {
	result := ExecuteResult{}

	if !idex.Valid || idex.Inst == nil {
		return result
	}

	inst := idex.Inst

	switch inst.Op {
	case insts.OpADD:
		result.ALUResult = s.executeADD(inst, rnValue, rmValue)
		if inst.SetFlags {
			s.setAddFlags(inst, rnValue, rmValue, result.ALUResult)
		}
	case insts.OpSUB:
		result.ALUResult = s.executeSUB(inst, rnValue, rmValue)
		if inst.SetFlags {
			s.setSubFlags(inst, rnValue, rmValue, result.ALUResult)
		}
	case insts.OpAND:
		result.ALUResult = s.executeAND(inst, rnValue, rmValue)
	case insts.OpORR:
		result.ALUResult = s.executeORR(inst, rnValue, rmValue)
	case insts.OpEOR:
		result.ALUResult = s.executeEOR(inst, rnValue, rmValue)
	case insts.OpLDR, insts.OpSTR, insts.OpLDP, insts.OpSTP,
		insts.OpLDRB, insts.OpSTRB, insts.OpLDRSB,
		insts.OpLDRH, insts.OpSTRH, insts.OpLDRSH:
		// Address calculation: base + offset
		// If base register is 31, use SP instead
		baseAddr := rnValue
		if inst.Rn == 31 {
			baseAddr = s.regFile.SP
		}
		// Handle indexed addressing modes
		switch inst.IndexMode {
		case insts.IndexPre:
			// Pre-index: address = base + signed offset
			result.ALUResult = uint64(int64(baseAddr) + inst.SignedImm)
		case insts.IndexPost:
			// Post-index: address = base (writeback happens later)
			result.ALUResult = baseAddr
		default:
			// Unsigned offset or signed offset for LDP/STP
			if inst.Format == insts.FormatLoadStorePair {
				result.ALUResult = uint64(int64(baseAddr) + inst.SignedImm)
			} else {
				result.ALUResult = baseAddr + inst.Imm
			}
		}
		result.StoreValue = rmValue // For STR, the value to store
	case insts.OpB:
		// Unconditional branch
		result.BranchTaken = true
		result.BranchTarget = uint64(int64(idex.PC) + inst.BranchOffset)
	case insts.OpBL:
		// Branch with link
		result.BranchTaken = true
		result.BranchTarget = uint64(int64(idex.PC) + inst.BranchOffset)
		result.ALUResult = idex.PC + 4 // Return address
	case insts.OpBCond:
		// Conditional branch
		var conditionMet bool
		if idex.IsFused {
			// Fused CMP+B.cond: compute flags from fused operands
			var op2 uint64
			if idex.FusedIsImm {
				op2 = idex.FusedImmVal
			} else {
				op2 = idex.FusedRmVal
			}
			n, z, c, v := ComputeSubFlags(idex.FusedRnVal, op2, idex.FusedIs64)
			conditionMet = EvaluateConditionWithFlags(inst.Cond, n, z, c, v)
		} else {
			// Non-fused: read condition from PSTATE (set by previous CMP)
			conditionMet = s.checkCondition(inst.Cond)
		}
		if conditionMet {
			result.BranchTaken = true
			result.BranchTarget = uint64(int64(idex.PC) + inst.BranchOffset)
		} else {
			result.BranchTaken = false
		}
	case insts.OpBR:
		// Branch to register
		result.BranchTaken = true
		result.BranchTarget = rnValue
	case insts.OpBLR:
		// Branch with link to register
		result.BranchTaken = true
		result.BranchTarget = rnValue
		result.ALUResult = idex.PC + 4 // Return address
	case insts.OpRET:
		// Return (branch to Rn, typically X30)
		result.BranchTaken = true
		result.BranchTarget = rnValue
	}

	return result
}

// checkCondition evaluates a branch condition based on PSTATE flags.
func (s *ExecuteStage) checkCondition(cond insts.Cond) bool {
	pstate := s.regFile.PSTATE

	switch cond {
	case insts.CondEQ:
		return pstate.Z
	case insts.CondNE:
		return !pstate.Z
	case insts.CondCS:
		return pstate.C
	case insts.CondCC:
		return !pstate.C
	case insts.CondMI:
		return pstate.N
	case insts.CondPL:
		return !pstate.N
	case insts.CondVS:
		return pstate.V
	case insts.CondVC:
		return !pstate.V
	case insts.CondHI:
		return pstate.C && !pstate.Z
	case insts.CondLS:
		return !pstate.C || pstate.Z
	case insts.CondGE:
		return pstate.N == pstate.V
	case insts.CondLT:
		return pstate.N != pstate.V
	case insts.CondGT:
		return !pstate.Z && (pstate.N == pstate.V)
	case insts.CondLE:
		return pstate.Z || (pstate.N != pstate.V)
	case insts.CondAL, insts.CondNV:
		return true
	default:
		return false
	}
}

func (s *ExecuteStage) executeADD(inst *insts.Instruction, rnValue, rmValue uint64) uint64 {
	if inst.Format == insts.FormatDPImm {
		imm := inst.Imm
		if inst.Shift > 0 {
			imm <<= inst.Shift
		}
		if inst.Is64Bit {
			return rnValue + imm
		}
		return uint64(uint32(rnValue) + uint32(imm))
	}
	// Register format
	if inst.Is64Bit {
		return rnValue + rmValue
	}
	return uint64(uint32(rnValue) + uint32(rmValue))
}

func (s *ExecuteStage) executeSUB(inst *insts.Instruction, rnValue, rmValue uint64) uint64 {
	if inst.Format == insts.FormatDPImm {
		imm := inst.Imm
		if inst.Shift > 0 {
			imm <<= inst.Shift
		}
		if inst.Is64Bit {
			return rnValue - imm
		}
		return uint64(uint32(rnValue) - uint32(imm))
	}
	// Register format
	if inst.Is64Bit {
		return rnValue - rmValue
	}
	return uint64(uint32(rnValue) - uint32(rmValue))
}

func (s *ExecuteStage) executeAND(inst *insts.Instruction, rnValue, rmValue uint64) uint64 {
	if inst.Is64Bit {
		return rnValue & rmValue
	}
	return uint64(uint32(rnValue) & uint32(rmValue))
}

func (s *ExecuteStage) executeORR(inst *insts.Instruction, rnValue, rmValue uint64) uint64 {
	if inst.Is64Bit {
		return rnValue | rmValue
	}
	return uint64(uint32(rnValue) | uint32(rmValue))
}

func (s *ExecuteStage) executeEOR(inst *insts.Instruction, rnValue, rmValue uint64) uint64 {
	if inst.Is64Bit {
		return rnValue ^ rmValue
	}
	return uint64(uint32(rnValue) ^ uint32(rmValue))
}

// setAddFlags sets PSTATE flags after an ADD/ADDS operation.
func (s *ExecuteStage) setAddFlags(inst *insts.Instruction, op1, op2, result uint64) {
	if inst.Is64Bit {
		// 64-bit flags
		s.regFile.PSTATE.N = (result >> 63) == 1
		s.regFile.PSTATE.Z = result == 0
		s.regFile.PSTATE.C = result < op1 // unsigned overflow (carry out)
		// V: signed overflow - adding same signs gives different sign
		op1Sign := op1 >> 63
		op2Sign := op2 >> 63
		resultSign := result >> 63
		s.regFile.PSTATE.V = (op1Sign == op2Sign) && (op1Sign != resultSign)
	} else {
		// 32-bit flags
		r32 := uint32(result)
		o1 := uint32(op1)
		o2 := uint32(op2)
		s.regFile.PSTATE.N = (r32 >> 31) == 1
		s.regFile.PSTATE.Z = r32 == 0
		s.regFile.PSTATE.C = r32 < o1
		op1Sign := o1 >> 31
		op2Sign := o2 >> 31
		resultSign := r32 >> 31
		s.regFile.PSTATE.V = (op1Sign == op2Sign) && (op1Sign != resultSign)
	}
}

// setSubFlags sets PSTATE flags after a SUB/SUBS/CMP operation.
func (s *ExecuteStage) setSubFlags(inst *insts.Instruction, op1, op2, result uint64) {
	if inst.Is64Bit {
		// 64-bit flags
		s.regFile.PSTATE.N = (result >> 63) == 1
		s.regFile.PSTATE.Z = result == 0
		s.regFile.PSTATE.C = op1 >= op2 // no borrow
		// V: signed overflow - subtracting different signs gives wrong sign
		op1Sign := op1 >> 63
		op2Sign := op2 >> 63
		resultSign := result >> 63
		s.regFile.PSTATE.V = (op1Sign != op2Sign) && (op2Sign == resultSign)
	} else {
		// 32-bit flags
		r32 := uint32(result)
		o1 := uint32(op1)
		o2 := uint32(op2)
		s.regFile.PSTATE.N = (r32 >> 31) == 1
		s.regFile.PSTATE.Z = r32 == 0
		s.regFile.PSTATE.C = o1 >= o2
		op1Sign := o1 >> 31
		op2Sign := o2 >> 31
		resultSign := r32 >> 31
		s.regFile.PSTATE.V = (op1Sign != op2Sign) && (op2Sign == resultSign)
	}
}

// MemoryStage handles memory reads and writes.
type MemoryStage struct {
	memory *emu.Memory
}

// NewMemoryStage creates a new memory stage.
func NewMemoryStage(memory *emu.Memory) *MemoryStage {
	return &MemoryStage{memory: memory}
}

// MemoryResult contains the output of the memory stage.
type MemoryResult struct {
	MemData uint64
}

// Access performs memory read or write operations.
func (s *MemoryStage) Access(exmem *EXMEMRegister) MemoryResult {
	result := MemoryResult{}

	if !exmem.Valid {
		return result
	}

	addr := exmem.ALUResult

	if exmem.MemRead {
		// Load: read from memory
		if exmem.Inst != nil && exmem.Inst.Is64Bit {
			result.MemData = s.memory.Read64(addr)
		} else {
			result.MemData = uint64(s.memory.Read32(addr))
		}
	}

	if exmem.MemWrite {
		// Store: write to memory
		if exmem.Inst != nil && exmem.Inst.Is64Bit {
			s.memory.Write64(addr, exmem.StoreValue)
		} else {
			s.memory.Write32(addr, uint32(exmem.StoreValue))
		}
	}

	return result
}

// MemorySlot interface for memory stage processing.
// Implemented by all EXMEM register types.
type MemorySlot interface {
	IsValid() bool
	GetMemRead() bool
	GetMemWrite() bool
	GetInst() *insts.Instruction
	GetALUResult() uint64
	GetStoreValue() uint64
}

// MemorySlot performs memory access for any EXMEM slot.
// Returns the memory result.
func (s *MemoryStage) MemorySlot(slot MemorySlot) MemoryResult {
	result := MemoryResult{}

	if !slot.IsValid() {
		return result
	}

	addr := slot.GetALUResult()
	inst := slot.GetInst()

	if slot.GetMemRead() {
		// Load: read from memory
		if inst != nil && inst.Is64Bit {
			result.MemData = s.memory.Read64(addr)
		} else {
			result.MemData = uint64(s.memory.Read32(addr))
		}
	}

	if slot.GetMemWrite() {
		// Store: write to memory
		if inst != nil && inst.Is64Bit {
			s.memory.Write64(addr, slot.GetStoreValue())
		} else {
			s.memory.Write32(addr, uint32(slot.GetStoreValue()))
		}
	}

	return result
}

// WritebackStage writes results back to the register file.
type WritebackStage struct {
	regFile *emu.RegFile
}

// NewWritebackStage creates a new writeback stage.
func NewWritebackStage(regFile *emu.RegFile) *WritebackStage {
	return &WritebackStage{regFile: regFile}
}

// Writeback writes the result to the destination register.
func (s *WritebackStage) Writeback(memwb *MEMWBRegister) {
	if !memwb.Valid || !memwb.RegWrite {
		return
	}

	// Don't write to XZR
	if memwb.Rd == 31 {
		return
	}

	var value uint64
	if memwb.MemToReg {
		value = memwb.MemData
	} else {
		value = memwb.ALUResult
	}

	s.regFile.WriteReg(memwb.Rd, value)
}

// WritebackSlot interface for writeback stage processing.
// Implemented by all MEMWB register types.
type WritebackSlot interface {
	IsValid() bool
	GetRegWrite() bool
	GetRd() uint8
	GetMemToReg() bool
	GetALUResult() uint64
	GetMemData() uint64
	GetIsFused() bool
}

// writebackSlot performs writeback for any MEMWB slot.
// Returns true if an instruction was retired.
func (s *WritebackStage) WritebackSlot(slot WritebackSlot) bool {
	if !slot.IsValid() || !slot.GetRegWrite() {
		return slot.IsValid() // Valid but no regwrite still counts as retired
	}

	// Don't write to XZR
	if slot.GetRd() == 31 {
		return true // Instruction retired
	}

	var value uint64
	if slot.GetMemToReg() {
		value = slot.GetMemData()
	} else {
		value = slot.GetALUResult()
	}

	s.regFile.WriteReg(slot.GetRd(), value)
	return true
}

// IsCMP returns true if the instruction is a CMP (compare) operation.
// CMP is encoded as SUB/SUBS with Rd=31 (XZR) and SetFlags=true.
func IsCMP(inst *insts.Instruction) bool {
	if inst == nil {
		return false
	}
	return inst.Op == insts.OpSUB && inst.SetFlags && inst.Rd == 31
}

// IsBCond returns true if the instruction is a conditional branch (B.cond).
func IsBCond(inst *insts.Instruction) bool {
	if inst == nil {
		return false
	}
	return inst.Op == insts.OpBCond
}

// ComputeSubFlags computes PSTATE flags from a SUB/CMP operation.
// Returns N, Z, C, V flags.
func ComputeSubFlags(op1, op2 uint64, is64Bit bool) (n, z, c, v bool) {
	if is64Bit {
		result := op1 - op2
		n = (result >> 63) == 1
		z = result == 0
		c = op1 >= op2 // no borrow
		// V: signed overflow - subtracting different signs gives wrong sign
		op1Sign := op1 >> 63
		op2Sign := op2 >> 63
		resultSign := result >> 63
		v = (op1Sign != op2Sign) && (op2Sign == resultSign)
	} else {
		o1 := uint32(op1)
		o2 := uint32(op2)
		r32 := o1 - o2
		n = (r32 >> 31) == 1
		z = r32 == 0
		c = o1 >= o2
		op1Sign := o1 >> 31
		op2Sign := o2 >> 31
		resultSign := r32 >> 31
		v = (op1Sign != op2Sign) && (op2Sign == resultSign)
	}
	return
}

// EvaluateConditionWithFlags evaluates a branch condition with given PSTATE flags.
func EvaluateConditionWithFlags(cond insts.Cond, n, z, c, v bool) bool {
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
