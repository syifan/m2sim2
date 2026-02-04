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
	OpLDR
	OpSTR
	OpSVC
	// PC-relative addressing
	OpADR    // ADR - PC-relative address
	OpADRP   // ADRP - PC-relative page address
	OpLDRLit // LDR (literal) - PC-relative load
	// Move wide instructions
	OpMOVZ // Move wide with zero
	OpMOVN // Move wide with NOT
	OpMOVK // Move wide with keep
	// Load/Store pair
	OpLDP // Load pair of registers
	OpSTP // Store pair of registers
	// Byte/halfword load/store
	OpLDRB  // Load register byte (zero-extend)
	OpSTRB  // Store register byte
	OpLDRSB // Load register signed byte
	OpLDRH  // Load register halfword (zero-extend)
	OpSTRH  // Store register halfword
	OpLDRSH // Load register signed halfword
	// SIMD opcodes
	OpVADD  // Vector ADD
	OpVSUB  // Vector SUB
	OpVMUL  // Vector MUL
	OpLDRQ  // Load Q register (128-bit)
	OpSTRQ  // Store Q register (128-bit)
	OpVMOV  // Vector MOV
	OpVFADD // Vector floating-point ADD
	OpVFSUB // Vector floating-point SUB
	OpVFMUL // Vector floating-point MUL
)

// Format represents an instruction encoding format.
type Format uint8

// Instruction formats.
const (
	FormatUnknown       Format = iota
	FormatDPImm                // Data Processing (Immediate)
	FormatDPReg                // Data Processing (Register)
	FormatBranch               // Unconditional Branch (Immediate)
	FormatBranchCond           // Conditional Branch
	FormatBranchReg            // Branch to Register
	FormatLoadStore            // Load/Store (Immediate)
	FormatLoadStoreLit         // Load/Store (PC-relative Literal)
	FormatLoadStorePair        // Load/Store Pair (LDP/STP)
	FormatPCRel                // PC-relative addressing (ADR, ADRP)
	FormatMoveWide             // Move wide (MOVZ, MOVN, MOVK)
	FormatException            // Exception Generation (SVC, HVC, SMC, BRK)
	FormatSIMDReg              // SIMD Data Processing (Register)
	FormatSIMDLoadStore        // SIMD Load/Store
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

// SIMDArrangement represents the SIMD vector arrangement specifier.
type SIMDArrangement uint8

// SIMD arrangement specifiers.
const (
	Arr8B  SIMDArrangement = iota // 8 bytes (64-bit, D register)
	Arr16B                        // 16 bytes (128-bit, Q register)
	Arr4H                         // 4 halfwords (64-bit)
	Arr8H                         // 8 halfwords (128-bit)
	Arr2S                         // 2 singles (64-bit)
	Arr4S                         // 4 singles (128-bit)
	Arr2D                         // 2 doubles (128-bit)
)

// IndexMode represents the addressing mode for indexed load/store.
type IndexMode uint8

const (
	IndexNone   IndexMode = iota // No indexing (unsigned offset)
	IndexPost                    // Post-index: [Rn], #imm
	IndexPre                     // Pre-index: [Rn, #imm]!
	IndexSigned                  // Signed offset (for load/store pair)
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

	// Load/Store indexed fields
	IndexMode IndexMode // Addressing mode (none, pre, post)
	SignedImm int64     // Signed immediate for indexed addressing
	Rt2       uint8     // Second register for load/store pair

	// SIMD fields
	IsSIMD      bool            // true if this is a SIMD instruction
	Arrangement SIMDArrangement // Vector arrangement (8B, 16B, 4H, etc.)
	IsFloat     bool            // true for floating-point SIMD ops
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
	case d.isSIMDLoadStore(word):
		d.decodeSIMDLoadStore(word, inst)
	case d.isSIMDThreeSame(word):
		d.decodeSIMDThreeSame(word, inst)
	case d.isLoadStorePair(word):
		d.decodeLoadStorePair(word, inst)
	case d.isLoadStoreLiteral(word):
		d.decodeLoadStoreLiteral(word, inst)
	case d.isLoadStoreRegIndexed(word):
		d.decodeLoadStoreRegIndexed(word, inst)
	case d.isLoadStoreImm(word):
		d.decodeLoadStoreImm(word, inst)
	case d.isPCRelAddressing(word):
		d.decodePCRelAddressing(word, inst)
	case d.isMoveWide(word):
		d.decodeMoveWide(word, inst)
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
	case d.isException(word):
		d.decodeException(word, inst)
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

	sf := (word >> 31) & 0x1      // bit 31: 1=64-bit, 0=32-bit
	op := (word >> 30) & 0x1      // bit 30: 0=ADD, 1=SUB
	s := (word >> 29) & 0x1       // bit 29: 1=set flags
	sh := (word >> 22) & 0x1      // bit 22: shift
	imm12 := (word >> 10) & 0xFFF // bits [21:10]
	rn := (word >> 5) & 0x1F      // bits [9:5]
	rd := word & 0x1F             // bits [4:0]

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

	sf := (word >> 31) & 0x1    // bit 31
	op := (word >> 24) & 0x1F   // bits [28:24]
	rd := word & 0x1F           // bits [4:0]
	rn := (word >> 5) & 0x1F    // bits [9:5]
	imm6 := (word >> 10) & 0x3F // bits [15:10]
	rm := (word >> 16) & 0x1F   // bits [20:16]
	shift := (word >> 22) & 0x3 // bits [23:22]

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

	op := (word >> 31) & 0x1  // bit 31: 0=B, 1=BL
	imm26 := word & 0x3FFFFFF // bits [25:0]

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
	cond := word & 0xF             // bits [3:0]

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

// isLoadStoreImm checks for Load/Store with unsigned immediate offset.
// LDR/STR (unsigned immediate): bits [31:30] = size, bits [29:27] = 111, bit 26 = 0,
// bits [25:24] = 01, bit 23:22 = opc
// 64-bit: size=11 (0xF9), 32-bit: size=10 (0xB9)
func (d *Decoder) isLoadStoreImm(word uint32) bool {
	// Check pattern: xx 111 0 01 xx
	// bits [29:27] == 111, bit 26 == 0, bits [25:24] == 01
	op1 := (word >> 27) & 0x7 // bits [29:27]
	op2 := (word >> 26) & 0x1 // bit 26
	op3 := (word >> 24) & 0x3 // bits [25:24]

	return op1 == 0b111 && op2 == 0 && op3 == 0b01
}

// decodeLoadStoreImm decodes LDR and STR with unsigned immediate offset.
// Format: size | 111 | V | 01 | opc | imm12 | Rn | Rt
// size: 11=64-bit, 10=32-bit
// V: 0 for integer registers
// opc: 00=STR, 01=LDR
func (d *Decoder) decodeLoadStoreImm(word uint32, inst *Instruction) {
	inst.Format = FormatLoadStore

	size := (word >> 30) & 0x3    // bits [31:30]
	opc := (word >> 22) & 0x3     // bits [23:22]
	imm12 := (word >> 10) & 0xFFF // bits [21:10]
	rn := (word >> 5) & 0x1F      // bits [9:5]
	rt := word & 0x1F             // bits [4:0]

	inst.Rn = uint8(rn)
	inst.Rd = uint8(rt) // Rt uses Rd field

	// Determine 64-bit vs 32-bit
	inst.Is64Bit = size == 0b11

	// Scale immediate by size
	// 64-bit (size=11): scale by 8
	// 32-bit (size=10): scale by 4
	var scale uint64
	if size == 0b11 {
		scale = 8
	} else {
		scale = 4
	}
	inst.Imm = uint64(imm12) * scale

	// Determine LDR vs STR from opc
	// opc[1:0]: 00=STR, 01=LDR (unsigned offset)
	if opc&0x1 == 1 {
		inst.Op = OpLDR
	} else {
		inst.Op = OpSTR
	}
}

// isException checks for exception generation instructions.
// SVC: bits [31:21] == 0b11010100000, bits [4:0] == 0b00001
func (d *Decoder) isException(word uint32) bool {
	hi := (word >> 21) & 0x7FF // bits [31:21]
	lo := word & 0x1F          // bits [4:0]
	return hi == 0b11010100000 && lo == 0b00001
}

// decodeException decodes SVC (supervisor call) instruction.
// Format: 11010100 000 | imm16 | 00001
// imm16 is typically 0 for Linux syscalls (SVC #0)
func (d *Decoder) decodeException(word uint32, inst *Instruction) {
	inst.Format = FormatException
	inst.Op = OpSVC

	// Extract imm16 (bits [20:5])
	imm16 := (word >> 5) & 0xFFFF
	inst.Imm = uint64(imm16)
}

// isSIMDLoadStore checks for SIMD/FP Load/Store with unsigned immediate offset.
// LDR/STR (SIMD&FP, unsigned immediate): bits [31:30] = size, bits [29:26] = 0b1111
// Q register (128-bit): size=00, opc=11 for LDR, opc=10 for STR
func (d *Decoder) isSIMDLoadStore(word uint32) bool {
	// Check pattern: xx 1111 xx (bits 29:26 = 0b1111)
	// This indicates SIMD&FP load/store with unsigned immediate
	op := (word >> 26) & 0xF // bits [29:26]
	return op == 0b1111
}

// decodeSIMDLoadStore decodes SIMD LDR and STR with unsigned immediate offset.
// Format: size | 111 | 1 | 01 | opc | imm12 | Rn | Rt
// For Q register (128-bit): size=00, opc[1]=1
func (d *Decoder) decodeSIMDLoadStore(word uint32, inst *Instruction) {
	inst.Format = FormatSIMDLoadStore
	inst.IsSIMD = true

	size := (word >> 30) & 0x3    // bits [31:30]
	opc := (word >> 22) & 0x3     // bits [23:22]
	imm12 := (word >> 10) & 0xFFF // bits [21:10]
	rn := (word >> 5) & 0x1F      // bits [9:5]
	rt := word & 0x1F             // bits [4:0]

	inst.Rn = uint8(rn)
	inst.Rd = uint8(rt) // Rt uses Rd field (this is the SIMD register index)

	// Determine size and scale
	// For Q register (128-bit): size=00, opc[1]=1 -> scale=16
	// For D register (64-bit): size=01, opc[1]=1 -> scale=8
	// For S register (32-bit): size=10, opc[1]=1 -> scale=4
	var scale uint64
	if size == 0b00 && (opc&0x2) != 0 {
		// Q register (128-bit)
		scale = 16
		inst.Is64Bit = true // Use this to indicate 128-bit
		inst.Arrangement = Arr16B
	} else if size == 0b01 {
		// D register (64-bit)
		scale = 8
		inst.Arrangement = Arr8B
	} else if size == 0b10 {
		// S register (32-bit)
		scale = 4
		inst.Arrangement = Arr2S
	} else {
		// Default to Q register
		scale = 16
		inst.Arrangement = Arr16B
	}

	inst.Imm = uint64(imm12) * scale

	// Determine LDR vs STR
	// opc[0]: 0=STR, 1=LDR
	if opc&0x1 == 1 {
		inst.Op = OpLDRQ
	} else {
		inst.Op = OpSTRQ
	}
}

// isSIMDThreeSame checks for SIMD Three Same instructions (ADD, SUB, MUL, etc.).
// Format: 0 | Q | U | 01110 | size | 1 | Rm | opcode | 1 | Rn | Rd
// bits [31] = 0, bits [28:24] = 0b01110
func (d *Decoder) isSIMDThreeSame(word uint32) bool {
	bit31 := (word >> 31) & 0x1
	op := (word >> 24) & 0x1F // bits [28:24]
	return bit31 == 0 && op == 0b01110
}

// decodeSIMDThreeSame decodes SIMD Three Same instructions.
// Format: 0 | Q | U | 01110 | size | 1 | Rm | opcode | 1 | Rn | Rd
func (d *Decoder) decodeSIMDThreeSame(word uint32, inst *Instruction) {
	inst.Format = FormatSIMDReg
	inst.IsSIMD = true

	q := (word >> 30) & 0x1       // bit 30: 0=64-bit (D), 1=128-bit (Q)
	u := (word >> 29) & 0x1       // bit 29: unsigned flag
	size := (word >> 22) & 0x3    // bits [23:22]
	rm := (word >> 16) & 0x1F     // bits [20:16]
	opcode := (word >> 11) & 0x1F // bits [15:11]
	rn := (word >> 5) & 0x1F      // bits [9:5]
	rd := word & 0x1F             // bits [4:0]

	inst.Rd = uint8(rd)
	inst.Rn = uint8(rn)
	inst.Rm = uint8(rm)
	inst.Is64Bit = q == 1 // For SIMD, this indicates 128-bit (Q) vs 64-bit (D)

	// Set arrangement based on Q and size
	inst.Arrangement = d.getSIMDArrangement(q == 1, size)

	// Decode opcode
	// Integer three-same: U=0, opcode determines operation
	// ADD: U=0, opcode=10000 (0x10)
	// SUB: U=1, opcode=10000 (0x10)
	// MUL: U=0, opcode=10011 (0x13)
	// Floating-point three-same: different encoding
	switch {
	case opcode == 0b10000: // ADD or SUB
		if u == 0 {
			inst.Op = OpVADD
		} else {
			inst.Op = OpVSUB
		}
	case opcode == 0b10011 && u == 0: // MUL (integer only)
		inst.Op = OpVMUL
	case opcode == 0b11010 && u == 0 && size >= 2: // FADD (floating-point)
		inst.Op = OpVFADD
		inst.IsFloat = true
	case opcode == 0b11010 && u == 1 && size >= 2: // FSUB (floating-point)
		inst.Op = OpVFSUB
		inst.IsFloat = true
	case opcode == 0b11011 && u == 1 && size >= 2: // FMUL (floating-point)
		inst.Op = OpVFMUL
		inst.IsFloat = true
	default:
		inst.Op = OpUnknown
	}
}

// isPCRelAddressing checks for PC-relative addressing instructions (ADR, ADRP).
// Format: op | immlo | 10000 | immhi | Rd
// ADR:  op=0 (bit 31)
// ADRP: op=1 (bit 31)
// bits [28:24] == 10000
func (d *Decoder) isPCRelAddressing(word uint32) bool {
	op := (word >> 24) & 0x1F // bits [28:24]
	return op == 0b10000
}

// decodePCRelAddressing decodes ADR and ADRP instructions.
// Format: op | immlo | 10000 | immhi | Rd
// ADR:  Rd = PC + sign_extend(immhi:immlo)
// ADRP: Rd = (PC & ~0xFFF) + sign_extend(immhi:immlo) << 12
func (d *Decoder) decodePCRelAddressing(word uint32, inst *Instruction) {
	inst.Format = FormatPCRel

	op := (word >> 31) & 0x1       // bit 31: 0=ADR, 1=ADRP
	immlo := (word >> 29) & 0x3    // bits [30:29]
	immhi := (word >> 5) & 0x7FFFF // bits [23:5]
	rd := word & 0x1F              // bits [4:0]

	inst.Rd = uint8(rd)
	inst.Is64Bit = true // ADR/ADRP always operate on 64-bit registers

	// Combine immhi:immlo (21-bit signed)
	imm21 := (immhi << 2) | immlo

	// Sign-extend 21-bit value
	offset := int64(imm21)
	if (imm21 >> 20) == 1 {
		offset |= ^int64(0x1FFFFF) // Sign extend
	}

	if op == 0 {
		inst.Op = OpADR
		// ADR: offset is in bytes
		inst.BranchOffset = offset
	} else {
		inst.Op = OpADRP
		// ADRP: offset is page-aligned (shifted left by 12)
		inst.BranchOffset = offset << 12
	}
}

// isLoadStoreLiteral checks for load literal (PC-relative) instructions.
// Format: opc | 011 | V | 00 | imm19 | Rt
// bits [29:27] == 011, bit 26 == V (0 for GPR), bits [25:24] == 00
func (d *Decoder) isLoadStoreLiteral(word uint32) bool {
	op1 := (word >> 27) & 0x7 // bits [29:27]
	op2 := (word >> 24) & 0x3 // bits [25:24]
	return op1 == 0b011 && op2 == 0b00
}

// decodeLoadStoreLiteral decodes LDR (literal) instructions.
// Format: opc | 011 | V | 00 | imm19 | Rt
// opc: 00=32-bit, 01=64-bit, 10=LDRSW, 11=PRFM
// V: 0=GPR, 1=SIMD/FP
func (d *Decoder) decodeLoadStoreLiteral(word uint32, inst *Instruction) {
	inst.Format = FormatLoadStoreLit
	inst.Op = OpLDRLit

	opc := (word >> 30) & 0x3      // bits [31:30]
	v := (word >> 26) & 0x1        // bit 26: 0=GPR, 1=SIMD
	imm19 := (word >> 5) & 0x7FFFF // bits [23:5]
	rt := word & 0x1F              // bits [4:0]

	inst.Rd = uint8(rt)
	inst.IsSIMD = v == 1

	// Determine size from opc
	// For GPR: 00=32-bit, 01=64-bit
	inst.Is64Bit = opc == 0b01

	// Sign-extend imm19 and multiply by 4 (word-aligned)
	offset := int64(imm19)
	if (imm19 >> 18) == 1 {
		offset |= ^int64(0x7FFFF) // Sign extend
	}
	offset *= 4

	inst.BranchOffset = offset
	if offset >= 0 {
		inst.Imm = uint64(offset)
	}
}

// isMoveWide checks for move wide immediate instructions (MOVZ, MOVN, MOVK).
// Format: sf | opc | 100101 | hw | imm16 | Rd
// bits [28:23] == 100101
func (d *Decoder) isMoveWide(word uint32) bool {
	op := (word >> 23) & 0x3F // bits [28:23]
	return op == 0b100101
}

// decodeMoveWide decodes MOVZ, MOVN, and MOVK instructions.
// Format: sf | opc | 100101 | hw | imm16 | Rd
// opc: 00=MOVN, 10=MOVZ, 11=MOVK
// hw: shift amount (0, 16, 32, or 48)
func (d *Decoder) decodeMoveWide(word uint32, inst *Instruction) {
	inst.Format = FormatMoveWide

	sf := (word >> 31) & 0x1      // bit 31: 0=32-bit, 1=64-bit
	opc := (word >> 29) & 0x3     // bits [30:29]
	hw := (word >> 21) & 0x3      // bits [22:21]: shift amount / 16
	imm16 := (word >> 5) & 0xFFFF // bits [20:5]
	rd := word & 0x1F             // bits [4:0]

	inst.Rd = uint8(rd)
	inst.Is64Bit = sf == 1
	inst.Imm = uint64(imm16)
	inst.Shift = uint8(hw * 16) // Shift amount in bits

	switch opc {
	case 0b00:
		inst.Op = OpMOVN
	case 0b10:
		inst.Op = OpMOVZ
	case 0b11:
		inst.Op = OpMOVK
	default:
		inst.Op = OpUnknown
	}
}

// getSIMDArrangement returns the SIMD arrangement based on Q bit and size field.
func (d *Decoder) getSIMDArrangement(isQ bool, size uint32) SIMDArrangement {
	if isQ {
		// 128-bit (Q register)
		switch size {
		case 0:
			return Arr16B
		case 1:
			return Arr8H
		case 2:
			return Arr4S
		case 3:
			return Arr2D
		}
	} else {
		// 64-bit (D register)
		switch size {
		case 0:
			return Arr8B
		case 1:
			return Arr4H
		case 2:
			return Arr2S
		}
	}
	return Arr16B // Default
}

// isLoadStorePair checks for load/store pair instructions (LDP/STP).
// Format: opc | 101 | V | mode | L | imm7 | Rt2 | Rn | Rt
// bits [29:27] == 101, and mode[25:23] must be 001, 010, or 011
func (d *Decoder) isLoadStorePair(word uint32) bool {
	op := (word >> 27) & 0x7   // bits [29:27]
	mode := (word >> 23) & 0x7 // bits [25:23]
	// mode must be 001 (post-index), 010 (signed offset), or 011 (pre-index)
	// This distinguishes from data processing register instructions
	return op == 0b101 && (mode == 0b001 || mode == 0b010 || mode == 0b011)
}

// decodeLoadStorePair decodes LDP and STP instructions.
// Format: opc | 101 | V | mode | L | imm7 | Rt2 | Rn | Rt
// opc[31:30]: 00=32-bit, 10=64-bit
// V[26]: 0=GPR, 1=SIMD
// mode[25:23]: 001=post-index, 010=signed offset, 011=pre-index
// L[22]: 0=STP, 1=LDP
// imm7[21:15]: signed offset, scaled by register size
func (d *Decoder) decodeLoadStorePair(word uint32, inst *Instruction) {
	inst.Format = FormatLoadStorePair

	opc := (word >> 30) & 0x3   // bits [31:30]
	v := (word >> 26) & 0x1     // bit 26: 0=GPR, 1=SIMD
	mode := (word >> 23) & 0x7  // bits [25:23]
	l := (word >> 22) & 0x1     // bit 22: 0=STP, 1=LDP
	imm7 := (word >> 15) & 0x7F // bits [21:15]
	rt2 := (word >> 10) & 0x1F  // bits [14:10]
	rn := (word >> 5) & 0x1F    // bits [9:5]
	rt := word & 0x1F           // bits [4:0]

	inst.Rn = uint8(rn)
	inst.Rd = uint8(rt)   // Rt uses Rd field
	inst.Rt2 = uint8(rt2) // Second register

	inst.IsSIMD = v == 1

	// Determine 64-bit vs 32-bit from opc
	// For GPR: opc=00 means 32-bit, opc=10 means 64-bit
	inst.Is64Bit = opc == 0b10

	// Determine addressing mode
	switch mode {
	case 0b001:
		inst.IndexMode = IndexPost
	case 0b010:
		inst.IndexMode = IndexSigned
	case 0b011:
		inst.IndexMode = IndexPre
	default:
		inst.IndexMode = IndexNone
	}

	// Sign-extend imm7 and scale by register size
	offset := int64(imm7)
	if (imm7 >> 6) == 1 {
		offset |= ^int64(0x7F) // Sign extend
	}
	// Scale: 4 for 32-bit, 8 for 64-bit
	if inst.Is64Bit {
		offset *= 8
	} else {
		offset *= 4
	}
	inst.SignedImm = offset

	// Determine LDP vs STP
	if l == 1 {
		inst.Op = OpLDP
	} else {
		inst.Op = OpSTP
	}
}

// isLoadStoreRegIndexed checks for load/store register with pre/post-indexed addressing.
// Format: size | 111 | V | 00 | opc | 0 | imm9 | mode | Rn | Rt
// bits [29:27] == 111, bit 26 == V, bits [25:24] == 00, bit 21 == 0
// mode[11:10]: 00=unscaled, 01=post-index, 10=unprivileged, 11=pre-index
func (d *Decoder) isLoadStoreRegIndexed(word uint32) bool {
	op1 := (word >> 27) & 0x7   // bits [29:27]
	op2 := (word >> 24) & 0x3   // bits [25:24]
	bit21 := (word >> 21) & 0x1 // bit 21
	return op1 == 0b111 && op2 == 0b00 && bit21 == 0
}

// decodeLoadStoreRegIndexed decodes LDR/STR with pre/post-indexed addressing.
// Format: size | 111 | V | 00 | opc | 0 | imm9 | mode | Rn | Rt
// size[31:30]: 00=byte, 01=halfword, 10=32-bit, 11=64-bit
// V[26]: 0=GPR
// opc[23:22]: varies by size
// imm9[20:12]: signed 9-bit immediate
// mode[11:10]: 01=post-index, 11=pre-index
func (d *Decoder) decodeLoadStoreRegIndexed(word uint32, inst *Instruction) {
	inst.Format = FormatLoadStore

	size := (word >> 30) & 0x3   // bits [31:30]
	v := (word >> 26) & 0x1      // bit 26: 0=GPR
	opc := (word >> 22) & 0x3    // bits [23:22]
	imm9 := (word >> 12) & 0x1FF // bits [20:12]
	mode := (word >> 10) & 0x3   // bits [11:10]
	rn := (word >> 5) & 0x1F     // bits [9:5]
	rt := word & 0x1F            // bits [4:0]

	inst.Rn = uint8(rn)
	inst.Rd = uint8(rt)
	inst.IsSIMD = v == 1

	// Determine addressing mode
	switch mode {
	case 0b01:
		inst.IndexMode = IndexPost
	case 0b11:
		inst.IndexMode = IndexPre
	default:
		inst.IndexMode = IndexNone // Unscaled or unprivileged
	}

	// Sign-extend imm9
	offset := int64(imm9)
	if (imm9 >> 8) == 1 {
		offset |= ^int64(0x1FF) // Sign extend
	}
	inst.SignedImm = offset

	// Determine operation based on size and opc
	// size=11 (64-bit), opc=00=STR, opc=01=LDR
	// size=10 (32-bit), opc=00=STR, opc=01=LDR
	// size=01 (16-bit), opc=00=STRH, opc=01=LDRH, opc=10=LDRSW(16), opc=11=LDRSH
	// size=00 (8-bit), opc=00=STRB, opc=01=LDRB, opc=10=LDRSB(64), opc=11=LDRSB(32)

	switch size {
	case 0b11: // 64-bit
		inst.Is64Bit = true
		if opc&0x1 == 1 {
			inst.Op = OpLDR
		} else {
			inst.Op = OpSTR
		}
	case 0b10: // 32-bit
		inst.Is64Bit = false
		if opc&0x1 == 1 {
			inst.Op = OpLDR
		} else {
			inst.Op = OpSTR
		}
	case 0b01: // 16-bit (halfword)
		inst.Is64Bit = false
		switch opc {
		case 0b00:
			inst.Op = OpSTRH
		case 0b01:
			inst.Op = OpLDRH
		case 0b10, 0b11:
			inst.Op = OpLDRSH
			inst.Is64Bit = opc == 0b10 // 10=extend to 64-bit
		}
	case 0b00: // 8-bit (byte)
		inst.Is64Bit = false
		switch opc {
		case 0b00:
			inst.Op = OpSTRB
		case 0b01:
			inst.Op = OpLDRB
		case 0b10, 0b11:
			inst.Op = OpLDRSB
			inst.Is64Bit = opc == 0b10 // 10=extend to 64-bit
		}
	}
}
