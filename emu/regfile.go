// Package emu provides functional ARM64 emulation.
package emu

// RegFile represents the ARM64 register file.
// It contains 31 general-purpose registers (X0-X30),
// the stack pointer (SP), and the program counter (PC).
type RegFile struct {
	// X holds general-purpose registers X0-X30.
	// X[31] is the zero register (XZR) which always reads as 0.
	X [32]uint64

	// SP is the stack pointer.
	SP uint64

	// PC is the program counter.
	PC uint64

	// PSTATE holds the processor state flags.
	PSTATE PSTATE
}

// PSTATE represents the processor state flags.
type PSTATE struct {
	// N is the negative flag.
	N bool
	// Z is the zero flag.
	Z bool
	// C is the carry flag.
	C bool
	// V is the overflow flag.
	V bool
}

// ReadReg reads a register value. Register 31 returns 0 (XZR).
// Registers >= 32 (e.g., 0xFF sentinel for immediate-mode operands) return 0.
func (r *RegFile) ReadReg(reg uint8) uint64 {
	if reg >= 31 {
		return 0 // XZR or invalid/sentinel register
	}
	return r.X[reg]
}

// ReadRegOrSP reads a register value, treating register 31 as SP (not XZR).
// This is used by instructions like ADD/SUB immediate where Rn=31 means SP.
func (r *RegFile) ReadRegOrSP(reg uint8) uint64 {
	if reg == 31 {
		return r.SP
	}
	return r.X[reg]
}

// WriteRegOrSP writes a register value, treating register 31 as SP (not XZR).
// This is used by instructions like ADD/SUB immediate where Rd=31 means SP.
func (r *RegFile) WriteRegOrSP(reg uint8, value uint64) {
	if reg == 31 {
		r.SP = value
		return
	}
	r.X[reg] = value
}

// WriteReg writes a value to a register. Writes to register 31+ are ignored.
func (r *RegFile) WriteReg(reg uint8, value uint64) {
	if reg >= 31 {
		return // XZR or invalid/sentinel register
	}
	r.X[reg] = value
}

// ReadReg32 reads the lower 32 bits of a register.
func (r *RegFile) ReadReg32(reg uint8) uint32 {
	return uint32(r.ReadReg(reg))
}

// WriteReg32 writes to the lower 32 bits and zero-extends.
func (r *RegFile) WriteReg32(reg uint8, value uint32) {
	r.WriteReg(reg, uint64(value))
}
