// Package pipeline provides the 5-stage pipeline implementation for timing simulation.
package pipeline

import "github.com/sarchlab/m2sim/insts"

// IFIDRegister holds state between Fetch and Decode stages.
type IFIDRegister struct {
	// Valid indicates if this pipeline register contains valid data.
	Valid bool

	// PC is the program counter of the fetched instruction.
	PC uint64

	// InstructionWord is the raw 32-bit instruction word.
	InstructionWord uint32

	// PredictedTaken indicates if the branch predictor predicted taken.
	PredictedTaken bool

	// PredictedTarget is the predicted branch target (from BTB or early resolution).
	PredictedTarget uint64

	// EarlyResolved indicates if this was an unconditional branch resolved at fetch time.
	EarlyResolved bool
}

// Clear resets the IF/ID register to empty state.
func (r *IFIDRegister) Clear() {
	r.Valid = false
	r.PC = 0
	r.InstructionWord = 0
	r.PredictedTaken = false
	r.PredictedTarget = 0
	r.EarlyResolved = false
}

// IDEXRegister holds state between Decode and Execute stages.
type IDEXRegister struct {
	// Valid indicates if this pipeline register contains valid data.
	Valid bool

	// PC is the program counter of the instruction.
	PC uint64

	// Inst is the decoded instruction.
	Inst *insts.Instruction

	// Register values read from the register file.
	RnValue uint64
	RmValue uint64

	// Register numbers for hazard detection.
	Rd uint8
	Rn uint8
	Rm uint8

	// Control signals.
	MemRead  bool // True for load instructions
	MemWrite bool // True for store instructions
	RegWrite bool // True if instruction writes to register
	MemToReg bool // True if result comes from memory (load)
	IsBranch bool // True for branch instructions

	// Branch prediction info (propagated from IF/ID).
	PredictedTaken  bool   // Whether predicted taken
	PredictedTarget uint64 // Predicted target address
	EarlyResolved   bool   // Whether resolved at fetch time (unconditional branch)

	// CMP+B.cond fusion fields.
	// When a CMP is immediately followed by B.cond, they are fused into a single
	// operation. The B.cond instruction carries the CMP operands and evaluates
	// the condition directly without reading PSTATE, eliminating the flag dependency.
	IsFused     bool   // True if this B.cond is fused with preceding CMP
	FusedRnVal  uint64 // CMP's Rn operand value
	FusedRmVal  uint64 // CMP's Rm operand value (or immediate)
	FusedIs64   bool   // CMP was 64-bit operation
	FusedIsImm  bool   // CMP used immediate operand
	FusedImmVal uint64 // CMP's immediate value (if FusedIsImm)
}

// Clear resets the ID/EX register to empty state.
func (r *IDEXRegister) Clear() {
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
	r.PredictedTaken = false
	r.PredictedTarget = 0
	r.EarlyResolved = false
	r.IsFused = false
	r.FusedRnVal = 0
	r.FusedRmVal = 0
	r.FusedIs64 = false
	r.FusedIsImm = false
	r.FusedImmVal = 0
}

// EXMEMRegister holds state between Execute and Memory stages.
type EXMEMRegister struct {
	// Valid indicates if this pipeline register contains valid data.
	Valid bool

	// PC is the program counter of the instruction.
	PC uint64

	// Inst is the decoded instruction.
	Inst *insts.Instruction

	// ALU result (address for load/store, result for ALU ops).
	ALUResult uint64

	// Value to store for store instructions.
	StoreValue uint64

	// Destination register number.
	Rd uint8

	// Control signals (propagated from ID/EX).
	MemRead  bool
	MemWrite bool
	RegWrite bool
	MemToReg bool

	// IsFused indicates this is a fused CMP+B.cond operation.
	// When this instruction retires, it counts as 2 instructions.
	IsFused bool
}

// Clear resets the EX/MEM register to empty state.
func (r *EXMEMRegister) Clear() {
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
	r.IsFused = false
}

// MemorySlot interface implementation for EXMEMRegister

// IsValid returns true if the register contains valid data.
func (r *EXMEMRegister) IsValid() bool { return r.Valid }

// GetMemRead returns true if this is a load instruction.
func (r *EXMEMRegister) GetMemRead() bool { return r.MemRead }

// GetMemWrite returns true if this is a store instruction.
func (r *EXMEMRegister) GetMemWrite() bool { return r.MemWrite }

// GetInst returns the instruction.
func (r *EXMEMRegister) GetInst() *insts.Instruction { return r.Inst }

// GetALUResult returns the computed address/result.
func (r *EXMEMRegister) GetALUResult() uint64 { return r.ALUResult }

// GetStoreValue returns the value to store.
func (r *EXMEMRegister) GetStoreValue() uint64 { return r.StoreValue }

// MEMWBRegister holds state between Memory and Writeback stages.
type MEMWBRegister struct {
	// Valid indicates if this pipeline register contains valid data.
	Valid bool

	// PC is the program counter of the instruction.
	PC uint64

	// Inst is the decoded instruction.
	Inst *insts.Instruction

	// ALU result (for ALU instructions).
	ALUResult uint64

	// Data read from memory (for load instructions).
	MemData uint64

	// Destination register number.
	Rd uint8

	// Control signals.
	RegWrite bool
	MemToReg bool // True if result comes from memory

	// IsFused indicates this is a fused CMP+B.cond operation.
	// When this instruction retires, it counts as 2 instructions.
	IsFused bool
}

// Clear resets the MEM/WB register to empty state.
func (r *MEMWBRegister) Clear() {
	r.Valid = false
	r.PC = 0
	r.Inst = nil
	r.ALUResult = 0
	r.MemData = 0
	r.Rd = 0
	r.RegWrite = false
	r.MemToReg = false
	r.IsFused = false
}

// WritebackSlot interface implementation for MEMWBRegister

// IsValid returns true if the register contains valid data.
func (r *MEMWBRegister) IsValid() bool { return r.Valid }

// GetRegWrite returns true if this instruction writes to a register.
func (r *MEMWBRegister) GetRegWrite() bool { return r.RegWrite }

// GetRd returns the destination register.
func (r *MEMWBRegister) GetRd() uint8 { return r.Rd }

// GetMemToReg returns true if the value comes from memory.
func (r *MEMWBRegister) GetMemToReg() bool { return r.MemToReg }

// GetALUResult returns the ALU computation result.
func (r *MEMWBRegister) GetALUResult() uint64 { return r.ALUResult }

// GetMemData returns the data loaded from memory.
func (r *MEMWBRegister) GetMemData() uint64 { return r.MemData }

// GetIsFused returns true if this is a fused macro-op (e.g., CMP+B.cond).
func (r *MEMWBRegister) GetIsFused() bool { return r.IsFused }
