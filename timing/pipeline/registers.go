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
}

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
