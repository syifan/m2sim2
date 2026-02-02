// Package pipeline provides a 5-stage pipeline model for cycle-accurate timing simulation.
package pipeline

import (
	"github.com/sarchlab/m2sim/insts"
)

// IFIDRegister holds state between Fetch and Decode stages.
type IFIDRegister struct {
	// Valid indicates this register contains valid data.
	Valid bool

	// PC of the fetched instruction.
	PC uint64

	// Instruction word fetched from memory.
	InstructionWord uint32
}

// IDEXRegister holds state between Decode and Execute stages.
type IDEXRegister struct {
	// Valid indicates this register contains valid data.
	Valid bool

	// PC of this instruction.
	PC uint64

	// Decoded instruction.
	Inst *insts.Instruction

	// Register values read during decode.
	RnValue uint64
	RmValue uint64

	// Destination register (for forwarding detection).
	Rd uint8

	// Source registers (for hazard detection).
	Rn uint8
	Rm uint8

	// Control signals.
	MemRead    bool // LDR
	MemWrite   bool // STR
	RegWrite   bool // Will write to Rd
	MemToReg   bool // Result comes from memory (LDR)
	IsBranch   bool // Branch instruction
	IsSyscall  bool // SVC instruction
}

// EXMEMRegister holds state between Execute and Memory stages.
type EXMEMRegister struct {
	// Valid indicates this register contains valid data.
	Valid bool

	// PC of this instruction.
	PC uint64

	// Instruction (for debugging/tracing).
	Inst *insts.Instruction

	// ALU result or computed address.
	ALUResult uint64

	// Value to store (for STR).
	StoreValue uint64

	// Destination register.
	Rd uint8

	// Control signals.
	MemRead  bool
	MemWrite bool
	RegWrite bool
	MemToReg bool
}

// MEMWBRegister holds state between Memory and Writeback stages.
type MEMWBRegister struct {
	// Valid indicates this register contains valid data.
	Valid bool

	// PC of this instruction.
	PC uint64

	// Instruction (for debugging/tracing).
	Inst *insts.Instruction

	// ALU result (for non-memory instructions).
	ALUResult uint64

	// Memory read result (for LDR).
	MemData uint64

	// Destination register.
	Rd uint8

	// Control signals.
	RegWrite bool
	MemToReg bool
}

// Clear resets the IFID register.
func (r *IFIDRegister) Clear() {
	r.Valid = false
	r.PC = 0
	r.InstructionWord = 0
}

// Clear resets the IDEX register.
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
	r.IsSyscall = false
}

// Clear resets the EXMEM register.
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

// Clear resets the MEMWB register.
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
