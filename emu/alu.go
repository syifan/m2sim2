// Package emu provides functional ARM64 emulation.
package emu

// ALU implements ARM64 arithmetic and logic operations.
type ALU struct {
	regFile *RegFile
}

// NewALU creates a new ALU connected to the given register file.
func NewALU(regFile *RegFile) *ALU {
	return &ALU{regFile: regFile}
}

// ADD64 performs 64-bit addition: Xd = Xn + Xm
func (a *ALU) ADD64(rd, rn, rm uint8, setFlags bool) {
	op1 := a.regFile.ReadReg(rn)
	op2 := a.regFile.ReadReg(rm)
	result := op1 + op2

	a.regFile.WriteReg(rd, result)

	if setFlags {
		a.setAddFlags64(op1, op2, result)
	}
}

// ADD64Imm performs 64-bit addition with immediate: Xd = Xn + imm
func (a *ALU) ADD64Imm(rd, rn uint8, imm uint64, setFlags bool) {
	op1 := a.regFile.ReadReg(rn)
	result := op1 + imm

	a.regFile.WriteReg(rd, result)

	if setFlags {
		a.setAddFlags64(op1, imm, result)
	}
}

// ADD64ImmShifted performs 64-bit addition with shifted immediate: Xd = Xn + (imm << shift)
func (a *ALU) ADD64ImmShifted(rd, rn uint8, imm uint64, shift uint8, setFlags bool) {
	shiftedImm := imm << shift
	a.ADD64Imm(rd, rn, shiftedImm, setFlags)
}

// ADD32 performs 32-bit addition: Wd = Wn + Wm (zero-extended)
func (a *ALU) ADD32(rd, rn, rm uint8, setFlags bool) {
	op1 := uint32(a.regFile.ReadReg(rn))
	op2 := uint32(a.regFile.ReadReg(rm))
	result := op1 + op2

	a.regFile.WriteReg(rd, uint64(result))

	if setFlags {
		a.setAddFlags32(op1, op2, result)
	}
}

// ADD32Imm performs 32-bit addition with immediate: Wd = Wn + imm (zero-extended)
func (a *ALU) ADD32Imm(rd, rn uint8, imm uint32, setFlags bool) {
	op1 := uint32(a.regFile.ReadReg(rn))
	result := op1 + imm

	a.regFile.WriteReg(rd, uint64(result))

	if setFlags {
		a.setAddFlags32(op1, imm, result)
	}
}

// SUB64 performs 64-bit subtraction: Xd = Xn - Xm
func (a *ALU) SUB64(rd, rn, rm uint8, setFlags bool) {
	op1 := a.regFile.ReadReg(rn)
	op2 := a.regFile.ReadReg(rm)
	result := op1 - op2

	a.regFile.WriteReg(rd, result)

	if setFlags {
		a.setSubFlags64(op1, op2, result)
	}
}

// SUB64Imm performs 64-bit subtraction with immediate: Xd = Xn - imm
func (a *ALU) SUB64Imm(rd, rn uint8, imm uint64, setFlags bool) {
	op1 := a.regFile.ReadReg(rn)
	result := op1 - imm

	a.regFile.WriteReg(rd, result)

	if setFlags {
		a.setSubFlags64(op1, imm, result)
	}
}

// SUB32Imm performs 32-bit subtraction with immediate: Wd = Wn - imm (zero-extended)
func (a *ALU) SUB32Imm(rd, rn uint8, imm uint32, setFlags bool) {
	op1 := uint32(a.regFile.ReadReg(rn))
	result := op1 - imm

	a.regFile.WriteReg(rd, uint64(result))

	if setFlags {
		a.setSubFlags32(op1, imm, result)
	}
}

// SUB32 performs 32-bit subtraction: Wd = Wn - Wm (zero-extended)
func (a *ALU) SUB32(rd, rn, rm uint8, setFlags bool) {
	op1 := uint32(a.regFile.ReadReg(rn))
	op2 := uint32(a.regFile.ReadReg(rm))
	result := op1 - op2

	a.regFile.WriteReg(rd, uint64(result))

	if setFlags {
		a.setSubFlags32(op1, op2, result)
	}
}

// AND64 performs 64-bit bitwise AND: Xd = Xn & Xm
func (a *ALU) AND64(rd, rn, rm uint8, setFlags bool) {
	op1 := a.regFile.ReadReg(rn)
	op2 := a.regFile.ReadReg(rm)
	result := op1 & op2

	a.regFile.WriteReg(rd, result)

	if setFlags {
		a.setLogicFlags64(result)
	}
}

// AND32 performs 32-bit bitwise AND: Wd = Wn & Wm (zero-extended)
func (a *ALU) AND32(rd, rn, rm uint8, setFlags bool) {
	op1 := uint32(a.regFile.ReadReg(rn))
	op2 := uint32(a.regFile.ReadReg(rm))
	result := op1 & op2

	a.regFile.WriteReg(rd, uint64(result))

	if setFlags {
		a.setLogicFlags32(result)
	}
}

// ORR64 performs 64-bit bitwise OR: Xd = Xn | Xm
func (a *ALU) ORR64(rd, rn, rm uint8) {
	op1 := a.regFile.ReadReg(rn)
	op2 := a.regFile.ReadReg(rm)
	result := op1 | op2

	a.regFile.WriteReg(rd, result)
}

// ORR32 performs 32-bit bitwise OR: Wd = Wn | Wm (zero-extended)
func (a *ALU) ORR32(rd, rn, rm uint8) {
	op1 := uint32(a.regFile.ReadReg(rn))
	op2 := uint32(a.regFile.ReadReg(rm))
	result := op1 | op2

	a.regFile.WriteReg(rd, uint64(result))
}

// EOR64 performs 64-bit bitwise XOR: Xd = Xn ^ Xm
func (a *ALU) EOR64(rd, rn, rm uint8) {
	op1 := a.regFile.ReadReg(rn)
	op2 := a.regFile.ReadReg(rm)
	result := op1 ^ op2

	a.regFile.WriteReg(rd, result)
}

// EOR32 performs 32-bit bitwise XOR: Wd = Wn ^ Wm (zero-extended)
func (a *ALU) EOR32(rd, rn, rm uint8) {
	op1 := uint32(a.regFile.ReadReg(rn))
	op2 := uint32(a.regFile.ReadReg(rm))
	result := op1 ^ op2

	a.regFile.WriteReg(rd, uint64(result))
}

// setAddFlags64 sets NZCV flags for 64-bit addition.
func (a *ALU) setAddFlags64(op1, op2, result uint64) {
	// N: Set if result is negative (MSB is 1)
	a.regFile.PSTATE.N = (result >> 63) == 1

	// Z: Set if result is zero
	a.regFile.PSTATE.Z = result == 0

	// C: Set if unsigned overflow (carry out)
	a.regFile.PSTATE.C = result < op1

	// V: Set if signed overflow
	// Overflow occurs when adding two positives gives negative,
	// or adding two negatives gives positive
	op1Sign := op1 >> 63
	op2Sign := op2 >> 63
	resultSign := result >> 63
	a.regFile.PSTATE.V = (op1Sign == op2Sign) && (op1Sign != resultSign)
}

// setAddFlags32 sets NZCV flags for 32-bit addition.
func (a *ALU) setAddFlags32(op1, op2, result uint32) {
	a.regFile.PSTATE.N = (result >> 31) == 1
	a.regFile.PSTATE.Z = result == 0
	a.regFile.PSTATE.C = result < op1
	op1Sign := op1 >> 31
	op2Sign := op2 >> 31
	resultSign := result >> 31
	a.regFile.PSTATE.V = (op1Sign == op2Sign) && (op1Sign != resultSign)
}

// setSubFlags64 sets NZCV flags for 64-bit subtraction.
func (a *ALU) setSubFlags64(op1, op2, result uint64) {
	// N: Set if result is negative
	a.regFile.PSTATE.N = (result >> 63) == 1

	// Z: Set if result is zero
	a.regFile.PSTATE.Z = result == 0

	// C: Set if NO borrow occurred (op1 >= op2)
	a.regFile.PSTATE.C = op1 >= op2

	// V: Set if signed overflow
	// Overflow occurs when subtracting negative from positive gives negative,
	// or subtracting positive from negative gives positive
	op1Sign := op1 >> 63
	op2Sign := op2 >> 63
	resultSign := result >> 63
	a.regFile.PSTATE.V = (op1Sign != op2Sign) && (op2Sign == resultSign)
}

// setSubFlags32 sets NZCV flags for 32-bit subtraction.
func (a *ALU) setSubFlags32(op1, op2, result uint32) {
	a.regFile.PSTATE.N = (result >> 31) == 1
	a.regFile.PSTATE.Z = result == 0
	a.regFile.PSTATE.C = op1 >= op2
	op1Sign := op1 >> 31
	op2Sign := op2 >> 31
	resultSign := result >> 31
	a.regFile.PSTATE.V = (op1Sign != op2Sign) && (op2Sign == resultSign)
}

// setLogicFlags64 sets NZ flags for 64-bit logic operations (C and V are cleared).
func (a *ALU) setLogicFlags64(result uint64) {
	a.regFile.PSTATE.N = (result >> 63) == 1
	a.regFile.PSTATE.Z = result == 0
	a.regFile.PSTATE.C = false
	a.regFile.PSTATE.V = false
}

// setLogicFlags32 sets NZ flags for 32-bit logic operations (C and V are cleared).
func (a *ALU) setLogicFlags32(result uint32) {
	a.regFile.PSTATE.N = (result >> 31) == 1
	a.regFile.PSTATE.Z = result == 0
	a.regFile.PSTATE.C = false
	a.regFile.PSTATE.V = false
}

// AND64Imm performs 64-bit bitwise AND with immediate: Xd = Xn & imm
func (a *ALU) AND64Imm(rd, rn uint8, imm uint64, setFlags bool) {
	op1 := a.regFile.ReadReg(rn)
	result := op1 & imm

	a.regFile.WriteReg(rd, result)

	if setFlags {
		a.setLogicFlags64(result)
	}
}

// AND32Imm performs 32-bit bitwise AND with immediate: Wd = Wn & imm (zero-extended)
func (a *ALU) AND32Imm(rd, rn uint8, imm uint64, setFlags bool) {
	op1 := uint32(a.regFile.ReadReg(rn))
	result := op1 & uint32(imm)

	a.regFile.WriteReg(rd, uint64(result))

	if setFlags {
		a.setLogicFlags32(result)
	}
}

// ORR64Imm performs 64-bit bitwise OR with immediate: Xd = Xn | imm
func (a *ALU) ORR64Imm(rd, rn uint8, imm uint64) {
	op1 := a.regFile.ReadReg(rn)
	result := op1 | imm

	a.regFile.WriteReg(rd, result)
}

// ORR32Imm performs 32-bit bitwise OR with immediate: Wd = Wn | imm (zero-extended)
func (a *ALU) ORR32Imm(rd, rn uint8, imm uint64) {
	op1 := uint32(a.regFile.ReadReg(rn))
	result := op1 | uint32(imm)

	a.regFile.WriteReg(rd, uint64(result))
}

// EOR64Imm performs 64-bit bitwise XOR with immediate: Xd = Xn ^ imm
func (a *ALU) EOR64Imm(rd, rn uint8, imm uint64) {
	op1 := a.regFile.ReadReg(rn)
	result := op1 ^ imm

	a.regFile.WriteReg(rd, result)
}

// EOR32Imm performs 32-bit bitwise XOR with immediate: Wd = Wn ^ imm (zero-extended)
func (a *ALU) EOR32Imm(rd, rn uint8, imm uint64) {
	op1 := uint32(a.regFile.ReadReg(rn))
	result := op1 ^ uint32(imm)

	a.regFile.WriteReg(rd, uint64(result))
}
