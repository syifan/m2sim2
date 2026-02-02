// Package emu provides functional ARM64 emulation.
package emu

// Cond represents an ARM64 condition code.
type Cond uint8

// ARM64 condition codes.
const (
	CondEQ Cond = 0b0000 // Equal (Z == 1)
	CondNE Cond = 0b0001 // Not Equal (Z == 0)
	CondCS Cond = 0b0010 // Carry Set / Unsigned higher or same (C == 1)
	CondCC Cond = 0b0011 // Carry Clear / Unsigned lower (C == 0)
	CondMI Cond = 0b0100 // Minus / Negative (N == 1)
	CondPL Cond = 0b0101 // Plus / Positive or zero (N == 0)
	CondVS Cond = 0b0110 // Overflow (V == 1)
	CondVC Cond = 0b0111 // No overflow (V == 0)
	CondHI Cond = 0b1000 // Unsigned higher (C == 1 && Z == 0)
	CondLS Cond = 0b1001 // Unsigned lower or same (C == 0 || Z == 1)
	CondGE Cond = 0b1010 // Signed greater than or equal (N == V)
	CondLT Cond = 0b1011 // Signed less than (N != V)
	CondGT Cond = 0b1100 // Signed greater than (Z == 0 && N == V)
	CondLE Cond = 0b1101 // Signed less than or equal (Z == 1 || N != V)
	CondAL Cond = 0b1110 // Always (unconditional)
	CondNV Cond = 0b1111 // Always (unconditional, reserved)
)

// BranchUnit implements ARM64 branch operations.
type BranchUnit struct {
	regFile *RegFile
}

// NewBranchUnit creates a new BranchUnit connected to the given register file.
func NewBranchUnit(regFile *RegFile) *BranchUnit {
	return &BranchUnit{regFile: regFile}
}

// B performs an unconditional branch (PC-relative).
// The offset is in bytes and is added to the current PC.
func (b *BranchUnit) B(offset int64) {
	b.regFile.PC = uint64(int64(b.regFile.PC) + offset)
}

// BL performs a branch with link (for function calls).
// Saves the return address (PC + 4) to X30 (link register),
// then branches to PC + offset.
func (b *BranchUnit) BL(offset int64) {
	// Save return address to X30 (link register)
	b.regFile.WriteReg(30, b.regFile.PC+4)

	// Branch to target
	b.regFile.PC = uint64(int64(b.regFile.PC) + offset)
}

// BR performs a branch to the address in the specified register.
func (b *BranchUnit) BR(rn uint8) {
	b.regFile.PC = b.regFile.ReadReg(rn)
}

// BLR performs a branch with link to the address in the specified register.
// Saves the return address (PC + 4) to X30, then branches to the address in Rn.
func (b *BranchUnit) BLR(rn uint8) {
	// Read target address first (in case rn == 30)
	target := b.regFile.ReadReg(rn)

	// Save return address to X30 (link register)
	b.regFile.WriteReg(30, b.regFile.PC+4)

	// Branch to target
	b.regFile.PC = target
}

// RET returns from a subroutine by branching to the address in the specified register.
// By default, RET uses X30 (link register), but can use any register.
func (b *BranchUnit) RET(rn uint8) {
	b.regFile.PC = b.regFile.ReadReg(rn)
}

// BCond performs a conditional branch based on the PSTATE flags.
// If the condition is met, branches to PC + offset; otherwise, PC is unchanged.
func (b *BranchUnit) BCond(offset int64, cond Cond) {
	if b.CheckCondition(cond) {
		b.regFile.PC = uint64(int64(b.regFile.PC) + offset)
	}
}

// CheckCondition evaluates an ARM64 condition code against the current PSTATE flags.
func (b *BranchUnit) CheckCondition(cond Cond) bool {
	pstate := &b.regFile.PSTATE

	switch cond {
	case CondEQ:
		// Equal: Z == 1
		return pstate.Z
	case CondNE:
		// Not Equal: Z == 0
		return !pstate.Z
	case CondCS:
		// Carry Set / Unsigned higher or same: C == 1
		return pstate.C
	case CondCC:
		// Carry Clear / Unsigned lower: C == 0
		return !pstate.C
	case CondMI:
		// Minus / Negative: N == 1
		return pstate.N
	case CondPL:
		// Plus / Positive or zero: N == 0
		return !pstate.N
	case CondVS:
		// Overflow: V == 1
		return pstate.V
	case CondVC:
		// No overflow: V == 0
		return !pstate.V
	case CondHI:
		// Unsigned higher: C == 1 && Z == 0
		return pstate.C && !pstate.Z
	case CondLS:
		// Unsigned lower or same: C == 0 || Z == 1
		return !pstate.C || pstate.Z
	case CondGE:
		// Signed greater than or equal: N == V
		return pstate.N == pstate.V
	case CondLT:
		// Signed less than: N != V
		return pstate.N != pstate.V
	case CondGT:
		// Signed greater than: Z == 0 && N == V
		return !pstate.Z && (pstate.N == pstate.V)
	case CondLE:
		// Signed less than or equal: Z == 1 || N != V
		return pstate.Z || (pstate.N != pstate.V)
	case CondAL, CondNV:
		// Always (unconditional)
		return true
	default:
		return false
	}
}
