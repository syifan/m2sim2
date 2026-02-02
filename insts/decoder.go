// Package insts provides ARM64 instruction definitions and decoding.
package insts

// Op represents an ARM64 opcode.
type Op uint16

// ARM64 opcodes.
const (
	OpUnknown Op = iota
	OpADD
	OpSUB
	OpAND
	OpORR
	OpEOR
	OpB
	OpBL
	OpBCond
	OpBR
	OpBLR
	OpRET
)

// Format represents an instruction encoding format.
type Format uint8

// Instruction formats.
const (
	FormatUnknown Format = iota
	FormatDPImm       // Data Processing (Immediate)
	FormatDPReg       // Data Processing (Register)
	FormatBranch      // Unconditional Branch (Immediate)
	FormatBranchCond  // Conditional Branch
	FormatBranchReg   // Branch to Register
)

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

// ShiftType represents a shift type for register operands.
type ShiftType uint8

// Shift types.
const (
	ShiftLSL ShiftType = 0b00 // Logical shift left
	ShiftLSR ShiftType = 0b01 // Logical shift right
	ShiftASR ShiftType = 0b10 // Arithmetic shift right
	ShiftROR ShiftType = 0b11 // Rotate right
)

// Instruction represents a decoded ARM64 instruction.
type Instruction struct {
	Op     Op     // Operation code
	Format Format // Encoding format

	// Common fields
	Is64Bit  bool  // true for 64-bit (X registers), false for 32-bit (W registers)
	SetFlags bool  // true if instruction sets condition flags (S suffix)
	Rd       uint8 // Destination register
	Rn       uint8 // First source register
	Rm       uint8 // Second source register (for register format)

	// Immediate operand
	Imm   uint64 // Immediate value
	Shift uint8  // Shift amount for immediate

	// Branch fields
	BranchOffset int64 // Signed branch offset in bytes
	Cond         Cond  // Condition code for conditional branches

	// Shift for register operand
	ShiftType   ShiftType // Type of shift applied to Rm
	ShiftAmount uint8     // Shift amount for Rm
}

// Decoder decodes ARM64 machine code into instructions.
type Decoder struct{}

// NewDecoder creates a new ARM64 instruction decoder.
func NewDecoder() *Decoder {
	return &Decoder{}
}

// Decode decodes a 32-bit ARM64 instruction word.
func (d *Decoder) Decode(word uint32) *Instruction {
	inst := &Instruction{Op: OpUnknown, Format: FormatUnknown}

	// Extract top-level opcode bits to determine instruction class
	// ARM64 uses bits [31:25] for primary classification

	op0 := (word >> 25) & 0xF // bits [28:25]

	switch {
	case d.isDataProcessingImm(word):
		d.decodeDataProcessingImm(word, inst)
	case d.isDataProcessingReg(word):
		d.decodeDataProcessingReg(word, inst)
	case d.isBranchImm(word):
		d.decodeBranchImm(word, inst)
	case d.isBranchCond(word):
		d.decodeBranchCond(word, inst)
	case d.isBranchReg(word):
		d.decodeBranchReg(word, inst)
	default:
		// Unknown instruction
		_ = op0 // unused, but extracted for future expansion
	}

	return inst
}

// isDataProcessingImm checks if instruction is Data Processing (Immediate).
// Add/Sub immediate: bits [28:23] == 0b100010
func (d *Decoder) isDataProcessingImm(word uint32) bool {
	op := (word >> 23) & 0x3F // bits [28:23]
	return op == 0b100010
}

// decodeDataProcessingImm decodes Add/Sub immediate instructions.
// Format: sf | op | S | 100010 | sh | imm12 | Rn | Rd
func (d *Decoder) decodeDataProcessingImm(word uint32, inst *Instruction) {
	inst.Format = FormatDPImm

	sf := (word >> 31) & 0x1       // bit 31: 1=64-bit, 0=32-bit
	op := (word >> 30) & 0x1       // bit 30: 0=ADD, 1=SUB
	s := (word >> 29) & 0x1        // bit 29: 1=set flags
	sh := (word >> 22) & 0x1       // bit 22: shift
	imm12 := (word >> 10) & 0xFFF  // bits [21:10]
	rn := (word >> 5) & 0x1F       // bits [9:5]
	rd := word & 0x1F              // bits [4:0]

	inst.Is64Bit = sf == 1
	inst.SetFlags = s == 1
	inst.Rd = uint8(rd)
	inst.Rn = uint8(rn)
	inst.Imm = uint64(imm12)

	if sh == 1 {
		inst.Shift = 12
	}

	if op == 0 {
		inst.Op = OpADD
	} else {
		inst.Op = OpSUB
	}
}

// isDataProcessingReg checks if instruction is Data Processing (Register).
// Add/Sub register: bits [28:24] == 0b01011
// Logical register: bits [28:24] == 0b01010
func (d *Decoder) isDataProcessingReg(word uint32) bool {
	op := (word >> 24) & 0x1F // bits [28:24]
	return op == 0b01011 || op == 0b01010
}

// decodeDataProcessingReg decodes Add/Sub/Logical register instructions.
// Add/Sub format: sf | op | S | 01011 | shift | 0 | Rm | imm6 | Rn | Rd
// Logical format: sf | opc | 01010 | shift | N | Rm | imm6 | Rn | Rd
func (d *Decoder) decodeDataProcessingReg(word uint32, inst *Instruction) {
	inst.Format = FormatDPReg

	sf := (word >> 31) & 0x1     // bit 31
	op := (word >> 24) & 0x1F    // bits [28:24]
	rd := word & 0x1F            // bits [4:0]
	rn := (word >> 5) & 0x1F     // bits [9:5]
	imm6 := (word >> 10) & 0x3F  // bits [15:10]
	rm := (word >> 16) & 0x1F    // bits [20:16]
	shift := (word >> 22) & 0x3  // bits [23:22]

	inst.Is64Bit = sf == 1
	inst.Rd = uint8(rd)
	inst.Rn = uint8(rn)
	inst.Rm = uint8(rm)
	inst.ShiftType = ShiftType(shift)
	inst.ShiftAmount = uint8(imm6)

	if op == 0b01011 {
		// Add/Sub register
		opBit := (word >> 30) & 0x1 // bit 30: 0=ADD, 1=SUB
		sBit := (word >> 29) & 0x1  // bit 29: set flags

		inst.SetFlags = sBit == 1

		if opBit == 0 {
			inst.Op = OpADD
		} else {
			inst.Op = OpSUB
		}
	} else {
		// Logical register (op == 0b01010)
		opc := (word >> 29) & 0x3 // bits [30:29]

		switch opc {
		case 0b00:
			inst.Op = OpAND
			inst.SetFlags = false
		case 0b01:
			inst.Op = OpORR
			inst.SetFlags = false
		case 0b10:
			inst.Op = OpEOR
			inst.SetFlags = false
		case 0b11:
			inst.Op = OpAND
			inst.SetFlags = true // ANDS
		}
	}
}

// isBranchImm checks for unconditional branch immediate.
// B:  bits [31:26] == 0b000101
// BL: bits [31:26] == 0b100101
func (d *Decoder) isBranchImm(word uint32) bool {
	op := (word >> 26) & 0x3F
	return op == 0b000101 || op == 0b100101
}

// decodeBranchImm decodes B and BL instructions.
// Format: op | imm26
func (d *Decoder) decodeBranchImm(word uint32, inst *Instruction) {
	inst.Format = FormatBranch

	op := (word >> 31) & 0x1    // bit 31: 0=B, 1=BL
	imm26 := word & 0x3FFFFFF   // bits [25:0]

	// Sign-extend imm26 to int64 and multiply by 4
	offset := int64(imm26)
	if (imm26 >> 25) == 1 {
		// Sign extend
		offset |= ^int64(0x3FFFFFF)
	}
	offset *= 4

	inst.BranchOffset = offset

	// For positive offsets, also store as unsigned immediate
	if offset >= 0 {
		inst.Imm = uint64(offset)
	}

	if op == 0 {
		inst.Op = OpB
	} else {
		inst.Op = OpBL
	}
}

// isBranchCond checks for conditional branch.
// B.cond: bits [31:25] == 0b0101010, bit 4 == 0
func (d *Decoder) isBranchCond(word uint32) bool {
	op := (word >> 25) & 0x7F
	bit4 := (word >> 4) & 0x1
	return op == 0b0101010 && bit4 == 0
}

// decodeBranchCond decodes conditional branch instructions.
// Format: 0101010 0 | imm19 | 0 | cond
func (d *Decoder) decodeBranchCond(word uint32, inst *Instruction) {
	inst.Format = FormatBranchCond
	inst.Op = OpBCond

	imm19 := (word >> 5) & 0x7FFFF // bits [23:5]
	cond := word & 0xF              // bits [3:0]

	// Sign-extend imm19 and multiply by 4
	offset := int64(imm19)
	if (imm19 >> 18) == 1 {
		offset |= ^int64(0x7FFFF)
	}
	offset *= 4

	inst.BranchOffset = offset
	if offset >= 0 {
		inst.Imm = uint64(offset)
	}
	inst.Cond = Cond(cond)
}

// isBranchReg checks for branch to register.
// Format: 1101011 0 0 op[1:0] 11111 0000 0 0 Rn 00000
func (d *Decoder) isBranchReg(word uint32) bool {
	// Check bits [31:25] == 0b1101011 and bits [15:10] == 0b000000 and bits [4:0] == 0b00000
	hi := (word >> 25) & 0x7F
	mid := (word >> 10) & 0x3F
	lo := word & 0x1F

	return hi == 0b1101011 && mid == 0b000000 && lo == 0b00000
}

// decodeBranchReg decodes BR, BLR, and RET instructions.
// Format: 1101011 0 0 op[1:0] 11111 0000 0 0 Rn 00000
func (d *Decoder) decodeBranchReg(word uint32, inst *Instruction) {
	inst.Format = FormatBranchReg

	op := (word >> 21) & 0x3 // bits [22:21]
	rn := (word >> 5) & 0x1F // bits [9:5]

	inst.Rn = uint8(rn)

	switch op {
	case 0b00:
		inst.Op = OpBR
	case 0b01:
		inst.Op = OpBLR
	case 0b10:
		inst.Op = OpRET
	default:
		inst.Op = OpUnknown
	}
}
