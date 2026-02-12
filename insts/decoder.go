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
	OpBRK // Breakpoint - software debug trap
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
	OpLDRSW // Load register signed word (32-bit sign-extended to 64-bit)
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
	OpDUP   // Duplicate scalar to vector
	// Conditional select opcodes
	OpCSEL  // Conditional select
	OpCSINC // Conditional select increment
	OpCSINV // Conditional select invert
	OpCSNEG // Conditional select negate
	// Division opcodes
	OpUDIV // Unsigned divide
	OpSDIV // Signed divide
	// Shift register opcodes
	OpLSLV // Logical shift left (variable/register)
	OpLSRV // Logical shift right (variable/register)
	OpASRV // Arithmetic shift right (variable/register)
	OpRORV // Rotate right (variable/register)
	// Bitfield opcodes
	OpSBFM // Signed bitfield move (ASR imm, SXTB, etc.)
	OpBFM  // Bitfield move (BFI, BFXIL)
	OpUBFM // Unsigned bitfield move (LSL imm, LSR imm, UXTB, etc.)
	// Multiply-add opcodes
	OpMADD // Multiply-add
	OpMSUB // Multiply-subtract
	// Conditional compare opcodes
	OpCCMN // Conditional compare negative
	OpCCMP // Conditional compare
	// Test and branch opcodes
	OpTBZ  // Test bit and branch if zero
	OpTBNZ // Test bit and branch if not zero
	// Compare and branch opcodes
	OpCBZ  // Compare and branch if zero
	OpCBNZ // Compare and branch if not zero
	OpNOP  // No operation (HINT #0)
	// Extract register opcode
	OpEXTR // Extract register (bitfield from register pair)
	// System register opcodes
	OpMRS // Move from system register
	// Logical NOT register opcodes (N-bit = 1)
	OpBIC // Bitwise bit clear (AND NOT): Rd = Rn & ~Rm
	OpORN // Bitwise OR NOT: Rd = Rn | ~Rm
	OpEON // Bitwise exclusive OR NOT: Rd = Rn ^ ~Rm
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
	FormatSIMDCopy             // SIMD Copy (DUP, MOV, etc.)
	FormatCondSelect           // Conditional Select (CSEL, CSINC, etc.)
	FormatDataProc2Src         // Data Processing (2 source) - UDIV, SDIV
	FormatDataProc3Src         // Data Processing (3 source) - MADD, MSUB
	FormatTestBranch           // Test and Branch (TBZ, TBNZ)
	FormatCompareBranch        // Compare and Branch (CBZ, CBNZ)
	FormatLogicalImm           // Logical Immediate (AND, ORR, EOR, ANDS)
	FormatBitfield             // Bitfield (SBFM, BFM, UBFM / ASR, LSL, LSR imm)
	FormatCondCmp              // Conditional compare (CCMP, CCMN)
	FormatExtract              // Extract register (EXTR)
	FormatSystemReg            // System register operations (MRS, MSR)
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
	IndexNone    IndexMode = iota // No indexing (unsigned offset)
	IndexRegBase                  // Register offset: [Xn, Xm{, extend}]
	IndexPost                     // Post-index: [Rn], #imm
	IndexPre                      // Pre-index: [Rn, #imm]!
	IndexSigned                   // Signed offset (for load/store pair)
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
	Imm2  uint64 // Second immediate (for bitfield imms)
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

	// System register fields
	SysReg uint16 // System register encoding for MRS/MSR
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
	d.decodeInto(word, inst)
	return inst
}

// DecodeInto decodes a 32-bit ARM64 instruction word into a pre-allocated
// Instruction, avoiding heap allocation. The caller is responsible for
// ensuring inst is zeroed or freshly initialized before calling.
func (d *Decoder) DecodeInto(word uint32, inst *Instruction) {
	*inst = Instruction{Op: OpUnknown, Format: FormatUnknown}
	d.decodeInto(word, inst)
}

func (d *Decoder) decodeInto(word uint32, inst *Instruction) {
	// Extract top-level opcode bits to determine instruction class
	// ARM64 uses bits [31:25] for primary classification

	op0 := (word >> 25) & 0xF // bits [28:25]

	switch {
	case d.isSIMDLoadStore(word):
		d.decodeSIMDLoadStore(word, inst)
	case d.isSIMDThreeSame(word):
		d.decodeSIMDThreeSame(word, inst)
	case d.isSIMDCopy(word):
		d.decodeSIMDCopy(word, inst)
	case d.isLoadStorePair(word):
		d.decodeLoadStorePair(word, inst)
	case d.isLoadStoreLiteral(word):
		d.decodeLoadStoreLiteral(word, inst)
	case d.isLoadStoreRegOffset(word):
		d.decodeLoadStoreRegOffset(word, inst)
	case d.isLoadStoreRegIndexed(word):
		d.decodeLoadStoreRegIndexed(word, inst)
	case d.isLoadStoreImm(word):
		d.decodeLoadStoreImm(word, inst)
	case d.isPCRelAddressing(word):
		d.decodePCRelAddressing(word, inst)
	case d.isMoveWide(word):
		d.decodeMoveWide(word, inst)
	case d.isCondCmp(word):
		d.decodeCondCmp(word, inst)
	case d.isConditionalSelect(word):
		d.decodeConditionalSelect(word, inst)
	case d.isDataProc2Src(word):
		d.decodeDataProc2Src(word, inst)
	case d.isDataProc3Src(word):
		d.decodeDataProc3Src(word, inst)
	case d.isLogicalImm(word):
		d.decodeLogicalImm(word, inst)
	case d.isExtract(word):
		d.decodeExtract(word, inst)
	case d.isBitfield(word):
		d.decodeBitfield(word, inst)
	case d.isDataProcessingImm(word):
		d.decodeDataProcessingImm(word, inst)
	case d.isDataProcessingReg(word):
		d.decodeDataProcessingReg(word, inst)
	case d.isTestBranch(word):
		d.decodeTestBranch(word, inst)
	case d.isCompareBranch(word):
		d.decodeCompareBranch(word, inst)
	case d.isBranchImm(word):
		d.decodeBranchImm(word, inst)
	case d.isBranchCond(word):
		d.decodeBranchCond(word, inst)
	case d.isBranchReg(word):
		d.decodeBranchReg(word, inst)
	case d.isNOP(word):
		d.decodeNOP(word, inst)
	case d.isException(word):
		d.decodeException(word, inst)
	case d.isSystemReg(word):
		d.decodeSystemReg(word, inst)
	default:
		// Unknown instruction
		_ = op0 // unused, but extracted for future expansion
	}
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
		opc := (word >> 29) & 0x3  // bits [30:29]
		nBit := (word >> 21) & 0x1 // bit 21: invert Rm

		switch opc {
		case 0b00:
			if nBit == 0 {
				inst.Op = OpAND
			} else {
				inst.Op = OpBIC
			}
			inst.SetFlags = false
		case 0b01:
			if nBit == 0 {
				inst.Op = OpORR
			} else {
				inst.Op = OpORN
			}
			inst.SetFlags = false
		case 0b10:
			if nBit == 0 {
				inst.Op = OpEOR
			} else {
				inst.Op = OpEON
			}
			inst.SetFlags = false
		case 0b11:
			if nBit == 0 {
				inst.Op = OpAND
			} else {
				inst.Op = OpBIC
			}
			inst.SetFlags = true // ANDS / BICS
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

// decodeLoadStoreImm decodes LDR, STR, and LDRSW with unsigned immediate offset.
// Format: size | 111 | V | 01 | opc | imm12 | Rn | Rt
// size: 11=64-bit, 10=32-bit, 01=16-bit, 00=8-bit
// V: 0 for integer registers
// For size=10: opc: 00=STR, 01=LDR, 10=LDRSW
// For size=11: opc: 00=STR, 01=LDR
func (d *Decoder) decodeLoadStoreImm(word uint32, inst *Instruction) {
	inst.Format = FormatLoadStore

	size := (word >> 30) & 0x3    // bits [31:30]
	opc := (word >> 22) & 0x3     // bits [23:22]
	imm12 := (word >> 10) & 0xFFF // bits [21:10]
	rn := (word >> 5) & 0x1F      // bits [9:5]
	rt := word & 0x1F             // bits [4:0]

	inst.Rn = uint8(rn)
	inst.Rd = uint8(rt) // Rt uses Rd field

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

	// Determine operation based on size and opc
	switch size {
	case 0b11: // 64-bit
		inst.Is64Bit = true
		if opc&0x1 == 1 {
			inst.Op = OpLDR
		} else {
			inst.Op = OpSTR
		}
	case 0b10: // 32-bit
		switch opc {
		case 0b00:
			inst.Op = OpSTR
			inst.Is64Bit = false
		case 0b01:
			inst.Op = OpLDR
			inst.Is64Bit = false
		case 0b10:
			inst.Op = OpLDRSW
			inst.Is64Bit = true // LDRSW sign-extends to 64-bit
		}
	default:
		// For size=01 and size=00, use default LDR/STR behavior
		inst.Is64Bit = size == 0b11
		if opc&0x1 == 1 {
			inst.Op = OpLDR
		} else {
			inst.Op = OpSTR
		}
	}
}

// isNOP checks for NOP instruction (HINT #0).
// NOP encoding: 0xd503201f = 1101 0101 0000 0011 0010 0000 0001 1111
func (d *Decoder) isNOP(word uint32) bool {
	return word == 0xd503201f
}

// decodeNOP decodes the NOP instruction.
func (d *Decoder) decodeNOP(word uint32, inst *Instruction) {
	inst.Op = OpNOP
	// NOP has no operands
}

// isException checks for exception generation instructions.
// SVC: bits [31:21] == 0b11010100000, bits [4:0] == 0b00001
// BRK: bits [31:21] == 0b11010100001, bits [4:0] == 0b00000
func (d *Decoder) isException(word uint32) bool {
	hi := (word >> 21) & 0x7FF // bits [31:21]
	lo := word & 0x1F          // bits [4:0]
	// SVC
	if hi == 0b11010100000 && lo == 0b00001 {
		return true
	}
	// BRK
	if hi == 0b11010100001 && lo == 0b00000 {
		return true
	}
	return false
}

// decodeException decodes exception generation instructions (SVC, BRK).
// SVC format: 11010100 000 | imm16 | 00001
// BRK format: 11010100 001 | imm16 | 00000
func (d *Decoder) decodeException(word uint32, inst *Instruction) {
	inst.Format = FormatException

	// Extract imm16 (bits [20:5])
	imm16 := (word >> 5) & 0xFFFF
	inst.Imm = uint64(imm16)

	// Determine which exception instruction
	hi := (word >> 21) & 0x7FF // bits [31:21]
	if hi == 0b11010100001 {
		inst.Op = OpBRK
	} else {
		inst.Op = OpSVC
	}
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
// bits [31] = 0, bits [28:24] = 0b01110, bit [21] = 1
func (d *Decoder) isSIMDThreeSame(word uint32) bool {
	bit31 := (word >> 31) & 0x1
	op := (word >> 24) & 0x1F   // bits [28:24]
	bit21 := (word >> 21) & 0x1 // bit 21 must be 1 for three-same
	return bit31 == 0 && op == 0b01110 && bit21 == 1
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

// isLoadStoreRegOffset checks for load/store with register offset addressing.
// Format: size | 111 | V | 00 | opc | 1 | Rm | option | S | 10 | Rn | Rt
// bits [29:27] == 111, bits [25:24] == 00, bit 21 == 1, bits [11:10] == 10
func (d *Decoder) isLoadStoreRegOffset(word uint32) bool {
	op1 := (word >> 27) & 0x7      // bits [29:27]
	op2 := (word >> 24) & 0x3      // bits [25:24]
	bit21 := (word >> 21) & 0x1    // bit 21
	bits1110 := (word >> 10) & 0x3 // bits [11:10]
	return op1 == 0b111 && op2 == 0b00 && bit21 == 1 && bits1110 == 0b10
}

// decodeLoadStoreRegOffset decodes LDR/STR with register offset addressing.
// Format: size | 111 | V | 00 | opc | 1 | Rm | option | S | 10 | Rn | Rt
// size[31:30]: 00=byte, 01=halfword, 10=32-bit, 11=64-bit
// V[26]: 0=GPR
// opc[23:22]: 00=STR, 01=LDR
// Rm[20:16]: offset register
// option[15:13]: extend type (010=UXTW, 011=LSL, 110=SXTW, 111=SXTX)
// S[12]: scale - if 1, shift by log2(size)
func (d *Decoder) decodeLoadStoreRegOffset(word uint32, inst *Instruction) {
	inst.Format = FormatLoadStore
	inst.IndexMode = IndexRegBase

	size := (word >> 30) & 0x3   // bits [31:30]
	v := (word >> 26) & 0x1      // bit 26: 0=GPR
	opc := (word >> 22) & 0x3    // bits [23:22]
	rm := (word >> 16) & 0x1F    // bits [20:16]
	option := (word >> 13) & 0x7 // bits [15:13]
	s := (word >> 12) & 0x1      // bit 12: scale
	rn := (word >> 5) & 0x1F     // bits [9:5]
	rt := word & 0x1F            // bits [4:0]

	inst.Rn = uint8(rn)
	inst.Rd = uint8(rt)
	inst.Rm = uint8(rm)
	inst.IsSIMD = v == 1

	// Store extend type in ShiftType (repurposing for this use)
	// Option: 010=UXTW, 011=LSL, 110=SXTW, 111=SXTX
	inst.ShiftType = ShiftType(option)

	// Calculate shift amount
	if s == 1 {
		// Scale by size: 0=byte(0), 1=halfword(1), 2=word(2), 3=dword(3)
		inst.ShiftAmount = uint8(size)
	} else {
		inst.ShiftAmount = 0
	}

	// Determine operation based on size and opc
	switch size {
	case 0b11: // 64-bit
		inst.Is64Bit = true
		if opc&0x1 == 1 {
			inst.Op = OpLDR
		} else {
			inst.Op = OpSTR
		}
	case 0b10: // 32-bit
		switch opc {
		case 0b00:
			inst.Op = OpSTR
			inst.Is64Bit = false
		case 0b01:
			inst.Op = OpLDR
			inst.Is64Bit = false
		case 0b10:
			inst.Op = OpLDRSW
			inst.Is64Bit = true // LDRSW sign-extends to 64-bit
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
			inst.Is64Bit = opc == 0b10
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
			inst.Is64Bit = opc == 0b10
		}
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
	// size=10 (32-bit), opc=00=STR, opc=01=LDR, opc=10=LDRSW
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
		switch opc {
		case 0b00:
			inst.Op = OpSTR
			inst.Is64Bit = false
		case 0b01:
			inst.Op = OpLDR
			inst.Is64Bit = false
		case 0b10:
			inst.Op = OpLDRSW
			inst.Is64Bit = true // LDRSW sign-extends to 64-bit
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

// isCondCmp checks for conditional compare instructions (CCMP, CCMN).
// Format: sf | op | 1 | 11010010 | Rm/imm5 | cond | 1/0 | o2 | Rn | o3 | nzcv
// bits [29:25] = x1101, bits [24:21] = 0010
func (d *Decoder) isCondCmp(word uint32) bool {
	// bits [29:25] = x1101 and bits [24:21] = 0010 for CCMP/CCMN
	bits2925 := (word >> 25) & 0x1F
	bits2421 := (word >> 21) & 0xF
	bit10 := (word >> 10) & 0x1
	bit4 := (word >> 4) & 0x1
	return bits2925&0xF == 0b1101 && bits2421 == 0b0010 && bit10 == 0 && bit4 == 0
}

// decodeCondCmp decodes conditional compare instructions (CCMP, CCMN).
// If condition is true: compare Rn with Rm/imm and set flags
// If condition is false: set flags to nzcv value
func (d *Decoder) decodeCondCmp(word uint32, inst *Instruction) {
	inst.Format = FormatCondCmp

	sf := (word >> 31) & 0x1   // bit 31: 1=64-bit, 0=32-bit
	op := (word >> 30) & 0x1   // bit 30: 0=CCMN, 1=CCMP
	imm := (word >> 11) & 0x1  // bit 11: 1=immediate, 0=register
	rm := (word >> 16) & 0x1F  // bits [20:16]: Rm or imm5
	cond := (word >> 12) & 0xF // bits [15:12]: condition
	rn := (word >> 5) & 0x1F   // bits [9:5]: Rn
	nzcv := word & 0xF         // bits [3:0]: flags when condition false

	inst.Is64Bit = sf == 1
	inst.Rn = uint8(rn)
	inst.Cond = Cond(cond)
	inst.Imm = uint64(nzcv) // Store nzcv for condition-false case

	if imm == 1 {
		// Immediate form
		inst.Imm2 = uint64(rm) // Store immediate value
		inst.Rm = 0xFF         // Mark as immediate (invalid reg)
	} else {
		// Register form
		inst.Rm = uint8(rm)
	}

	if op == 0 {
		inst.Op = OpCCMN
	} else {
		inst.Op = OpCCMP
	}
}

// isConditionalSelect checks for conditional select instructions (CSEL, CSINC, CSINV, CSNEG).
// Format: sf | op | S | 11010100 | Rm | cond | op2 | Rn | Rd
// bits [29:21] == 0b011010100 (S=0 at bit 29, 11010100 at bits 28:21)
func (d *Decoder) isConditionalSelect(word uint32) bool {
	op := (word >> 21) & 0x1FF // bits [29:21]
	return op == 0b011010100
}

// decodeConditionalSelect decodes CSEL, CSINC, CSINV, CSNEG instructions.
// Format: sf | op | S | 11010100 | Rm | cond | op2 | Rn | Rd
// op[30]: 0 for CSEL/CSINC, 1 for CSINV/CSNEG
// op2[10]: 0 for CSEL/CSINV, 1 for CSINC/CSNEG
func (d *Decoder) decodeConditionalSelect(word uint32, inst *Instruction) {
	inst.Format = FormatCondSelect

	sf := (word >> 31) & 0x1   // bit 31: 0=32-bit, 1=64-bit
	op := (word >> 30) & 0x1   // bit 30
	rm := (word >> 16) & 0x1F  // bits [20:16]
	cond := (word >> 12) & 0xF // bits [15:12]
	op2 := (word >> 10) & 0x1  // bit 10
	rn := (word >> 5) & 0x1F   // bits [9:5]
	rd := word & 0x1F          // bits [4:0]

	inst.Is64Bit = sf == 1
	inst.Rd = uint8(rd)
	inst.Rn = uint8(rn)
	inst.Rm = uint8(rm)
	inst.Cond = Cond(cond)

	// Decode operation based on op and op2
	// op=0, op2=0: CSEL
	// op=0, op2=1: CSINC
	// op=1, op2=0: CSINV
	// op=1, op2=1: CSNEG
	switch {
	case op == 0 && op2 == 0:
		inst.Op = OpCSEL
	case op == 0 && op2 == 1:
		inst.Op = OpCSINC
	case op == 1 && op2 == 0:
		inst.Op = OpCSINV
	case op == 1 && op2 == 1:
		inst.Op = OpCSNEG
	default:
		inst.Op = OpUnknown
	}
}

// isDataProc2Src checks for data processing (2 source) instructions (UDIV, SDIV).
// Format: sf | 0 | S | 11010110 | Rm | opcode | Rn | Rd
// bits [29:21] == 0b011010110 (S=0)
func (d *Decoder) isDataProc2Src(word uint32) bool {
	op := (word >> 21) & 0x1FF // bits [29:21]
	return op == 0b011010110
}

// decodeDataProc2Src decodes UDIV and SDIV instructions.
// Format: sf | 0 | S | 11010110 | Rm | opcode | Rn | Rd
// opcode[15:10]: 000010=UDIV, 000011=SDIV
func (d *Decoder) decodeDataProc2Src(word uint32, inst *Instruction) {
	inst.Format = FormatDataProc2Src

	sf := (word >> 31) & 0x1      // bit 31: 0=32-bit, 1=64-bit
	rm := (word >> 16) & 0x1F     // bits [20:16]
	opcode := (word >> 10) & 0x3F // bits [15:10]
	rn := (word >> 5) & 0x1F      // bits [9:5]
	rd := word & 0x1F             // bits [4:0]

	inst.Is64Bit = sf == 1
	inst.Rd = uint8(rd)
	inst.Rn = uint8(rn)
	inst.Rm = uint8(rm)

	// Decode operation based on opcode
	// 000010 = UDIV
	// 000011 = SDIV
	// 001000 = LSLV (logical shift left variable)
	// 001001 = LSRV (logical shift right variable)
	// 001010 = ASRV (arithmetic shift right variable)
	// 001011 = RORV (rotate right variable)
	switch opcode {
	case 0b000010:
		inst.Op = OpUDIV
	case 0b000011:
		inst.Op = OpSDIV
	case 0b001000:
		inst.Op = OpLSLV
	case 0b001001:
		inst.Op = OpLSRV
	case 0b001010:
		inst.Op = OpASRV
	case 0b001011:
		inst.Op = OpRORV
	default:
		inst.Op = OpUnknown
	}
}

// isDataProc3Src checks for data processing (3 source) instructions (MADD, MSUB).
// Format: sf | op54 | 11011 | op31 | Rm | o0 | Ra | Rn | Rd
// bits [28:24] == 0b11011
func (d *Decoder) isDataProc3Src(word uint32) bool {
	op := (word >> 24) & 0x1F // bits [28:24]
	return op == 0b11011
}

// decodeDataProc3Src decodes MADD and MSUB instructions.
// Format: sf | op54 | 11011 | op31 | Rm | o0 | Ra | Rn | Rd
// op54=00, op31=000 for MADD/MSUB
// o0[15]: 0=MADD, 1=MSUB
func (d *Decoder) decodeDataProc3Src(word uint32, inst *Instruction) {
	inst.Format = FormatDataProc3Src

	sf := (word >> 31) & 0x1  // bit 31: 0=32-bit, 1=64-bit
	rm := (word >> 16) & 0x1F // bits [20:16]
	o0 := (word >> 15) & 0x1  // bit 15
	ra := (word >> 10) & 0x1F // bits [14:10]
	rn := (word >> 5) & 0x1F  // bits [9:5]
	rd := word & 0x1F         // bits [4:0]

	inst.Is64Bit = sf == 1
	inst.Rd = uint8(rd)
	inst.Rn = uint8(rn)
	inst.Rm = uint8(rm)
	inst.Rt2 = uint8(ra) // Reuse Rt2 field for Ra

	// Decode operation based on o0
	// o0=0: MADD (Rd = Ra + Rn * Rm)
	// o0=1: MSUB (Rd = Ra - Rn * Rm)
	if o0 == 0 {
		inst.Op = OpMADD
	} else {
		inst.Op = OpMSUB
	}
}

// isLogicalImm checks for logical immediate instructions (AND, ORR, EOR, ANDS).
// Format: sf | opc | 100100 | N | immr | imms | Rn | Rd
// bits [28:23] == 0b100100
func (d *Decoder) isLogicalImm(word uint32) bool {
	op := (word >> 23) & 0x3F // bits [28:23]
	return op == 0b100100
}

// decodeLogicalImm decodes logical immediate instructions.
// Format: sf | opc | 100100 | N | immr | imms | Rn | Rd
// sf[31]: 0=32-bit, 1=64-bit
// opc[30:29]: 00=AND, 01=ORR, 10=EOR, 11=ANDS
// N[22]: part of bitmask encoding (must be 0 for 32-bit)
// immr[21:16]: rotation amount
// imms[15:10]: size and ones count
func (d *Decoder) decodeLogicalImm(word uint32, inst *Instruction) {
	inst.Format = FormatLogicalImm

	sf := (word >> 31) & 0x1    // bit 31
	opc := (word >> 29) & 0x3   // bits [30:29]
	n := (word >> 22) & 0x1     // bit 22
	immr := (word >> 16) & 0x3F // bits [21:16]
	imms := (word >> 10) & 0x3F // bits [15:10]
	rn := (word >> 5) & 0x1F    // bits [9:5]
	rd := word & 0x1F           // bits [4:0]

	inst.Is64Bit = sf == 1
	inst.Rd = uint8(rd)
	inst.Rn = uint8(rn)

	// Decode the bitmask immediate
	inst.Imm = DecodeBitmaskImmediate(uint8(n), uint8(immr), uint8(imms), sf == 1)

	// Decode operation
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

// DecodeBitmaskImmediate decodes the ARM64 bitmask immediate encoding.
// The encoding uses N, immr, imms to represent patterns of consecutive 1 bits
// that are replicated across the register width.
// Returns the decoded immediate value.
func DecodeBitmaskImmediate(n, immr, imms uint8, is64bit bool) uint64 {
	// Determine the element size (len) from N and highest bit of imms
	// For 64-bit: N=1 means 64-bit element, N=0 means smaller element
	// For 32-bit: N must be 0

	var len uint8
	if n == 1 {
		len = 6 // 64-bit element
	} else {
		// Find the highest bit that is 0 in imms (starting from bit 5)
		// len is (5 - position of highest 0 bit + 1)
		for i := uint8(5); i >= 1; i-- {
			if (imms & (1 << i)) == 0 {
				len = i
				break
			}
		}
		if len == 0 {
			len = 1 // Minimum element size is 2 bits
		}
	}

	// Element size in bits: 2^len
	esize := uint64(1) << len

	// Number of 1 bits: (imms AND (esize-1)) + 1
	levels := esize - 1
	s := uint64(imms) & levels
	r := uint64(immr) & levels

	// Create the basic pattern: (s+1) ones
	welem := (uint64(1) << (s + 1)) - 1

	// Rotate right by r within esize
	if r > 0 {
		welem = ((welem >> r) | (welem << (esize - r))) & ((1 << esize) - 1)
	}

	// Replicate the element across 64 bits
	var result uint64
	for i := uint64(0); i < 64; i += esize {
		result |= welem << i
	}

	// For 32-bit operations, mask to 32 bits
	if !is64bit {
		result &= 0xFFFFFFFF
	}

	return result
}

// isExtract checks for extract register instruction (EXTR).
// Format: sf | op21 | 100111 | N | o0 | Rm | imms | Rn | Rd
// bits [28:23] == 0b100111 and op21 == 00 and o0 == 0
func (d *Decoder) isExtract(word uint32) bool {
	op := (word >> 23) & 0x3F  // bits [28:23]
	op21 := (word >> 29) & 0x3 // bits [30:29]
	o0 := (word >> 21) & 0x1   // bit 21
	return op == 0b100111 && op21 == 0b00 && o0 == 0
}

// decodeExtract decodes the EXTR instruction.
// Format: sf | 00 | 100111 | N | 0 | Rm | imms | Rn | Rd
// EXTR Rd, Rn, Rm, #lsb - Extract register from pair
// Result = (Rm:Rn >> lsb)[datasize-1:0]
func (d *Decoder) decodeExtract(word uint32, inst *Instruction) {
	inst.Format = FormatExtract
	inst.Op = OpEXTR

	sf := (word >> 31) & 0x1    // bit 31
	rm := (word >> 16) & 0x1F   // bits [20:16]
	imms := (word >> 10) & 0x3F // bits [15:10] - lsb position
	rn := (word >> 5) & 0x1F    // bits [9:5]
	rd := word & 0x1F           // bits [4:0]

	inst.Is64Bit = sf == 1
	inst.Rd = uint8(rd)
	inst.Rn = uint8(rn)
	inst.Rm = uint8(rm)
	inst.Imm = uint64(imms) // lsb position
}

// isBitfield checks for bitfield instructions (SBFM, BFM, UBFM).
// Format: sf | opc | 100110 | N | immr | imms | Rn | Rd
// bits [28:23] == 0b100110
func (d *Decoder) isBitfield(word uint32) bool {
	op := (word >> 23) & 0x3F // bits [28:23]
	return op == 0b100110
}

// decodeBitfield decodes bitfield instructions.
// SBFM (opc=00), BFM (opc=01), UBFM (opc=10)
// Aliases: ASR imm, LSL imm, LSR imm, SXTB, SXTH, SXTW, UXTB, UXTH
func (d *Decoder) decodeBitfield(word uint32, inst *Instruction) {
	inst.Format = FormatBitfield

	sf := (word >> 31) & 0x1    // bit 31
	opc := (word >> 29) & 0x3   // bits [30:29]
	n := (word >> 22) & 0x1     // bit 22
	immr := (word >> 16) & 0x3F // bits [21:16]
	imms := (word >> 10) & 0x3F // bits [15:10]
	rn := (word >> 5) & 0x1F    // bits [9:5]
	rd := word & 0x1F           // bits [4:0]

	inst.Is64Bit = sf == 1
	inst.Rd = uint8(rd)
	inst.Rn = uint8(rn)
	// Store immr and imms for execution
	inst.Imm = uint64(immr)
	inst.Imm2 = uint64(imms)

	_ = n // N must match sf for valid encoding

	switch opc {
	case 0b00:
		inst.Op = OpSBFM
	case 0b01:
		inst.Op = OpBFM
	case 0b10:
		inst.Op = OpUBFM
	default:
		inst.Op = OpUnknown
	}
}

// isTestBranch checks for test and branch instructions (TBZ, TBNZ).
// Format: b5 | 011011 | op | b40 | imm14 | Rt
// bits [30:25] == 0b011011
func (d *Decoder) isTestBranch(word uint32) bool {
	op := (word >> 25) & 0x3F // bits [30:25]
	return op == 0b011011
}

// decodeTestBranch decodes TBZ and TBNZ instructions.
// Format: b5 | 011011 | op | b40 | imm14 | Rt
// b5[31]: high bit of bit number (for 64-bit registers)
// op[24]: 0=TBZ, 1=TBNZ
// b40[23:19]: low 5 bits of bit number
// imm14[18:5]: signed offset (scaled by 4)
func (d *Decoder) decodeTestBranch(word uint32, inst *Instruction) {
	inst.Format = FormatTestBranch

	b5 := (word >> 31) & 0x1      // bit 31
	op := (word >> 24) & 0x1      // bit 24
	b40 := (word >> 19) & 0x1F    // bits [23:19]
	imm14 := (word >> 5) & 0x3FFF // bits [18:5]
	rt := word & 0x1F             // bits [4:0]

	inst.Rd = uint8(rt)
	inst.Is64Bit = b5 == 1 // 64-bit register if b5=1

	// Combine b5:b40 to get the bit number (0-63)
	bitNum := (b5 << 5) | b40
	inst.Imm = uint64(bitNum)

	// Sign-extend imm14 and multiply by 4
	offset := int64(imm14)
	if (imm14 >> 13) == 1 {
		offset |= ^int64(0x3FFF) // Sign extend
	}
	offset *= 4
	inst.BranchOffset = offset

	// Decode operation
	if op == 0 {
		inst.Op = OpTBZ
	} else {
		inst.Op = OpTBNZ
	}
}

// isCompareBranch checks for compare and branch instructions (CBZ, CBNZ).
// Format: sf | 011010 | op | imm19 | Rt
// bits [30:25] == 0b011010
func (d *Decoder) isCompareBranch(word uint32) bool {
	op := (word >> 25) & 0x3F // bits [30:25]
	return op == 0b011010
}

// decodeCompareBranch decodes CBZ and CBNZ instructions.
// Format: sf | 011010 | op | imm19 | Rt
// sf[31]: 0=32-bit, 1=64-bit
// op[24]: 0=CBZ, 1=CBNZ
// imm19[23:5]: signed offset (scaled by 4)
func (d *Decoder) decodeCompareBranch(word uint32, inst *Instruction) {
	inst.Format = FormatCompareBranch

	sf := (word >> 31) & 0x1       // bit 31
	op := (word >> 24) & 0x1       // bit 24
	imm19 := (word >> 5) & 0x7FFFF // bits [23:5]
	rt := word & 0x1F              // bits [4:0]

	inst.Rd = uint8(rt)
	inst.Is64Bit = sf == 1

	// Sign-extend imm19 and multiply by 4
	offset := int64(imm19)
	if (imm19 >> 18) == 1 {
		offset |= ^int64(0x7FFFF) // Sign extend
	}
	offset *= 4
	inst.BranchOffset = offset

	// Decode operation
	if op == 0 {
		inst.Op = OpCBZ
	} else {
		inst.Op = OpCBNZ
	}
}

// isSIMDCopy checks for SIMD copy instructions (DUP).
// DUP (general register): 0 | Q | 001110 | 0 | 0 | 0 | imm5 | 000011 | Rn | Rd
// bits [31:24] == 0b01001110, bit 21 == 0, bits [15:10] == 0b000011
func (d *Decoder) isSIMDCopy(word uint32) bool {
	op := (word >> 24) & 0xFF   // bits [31:24]
	bit21 := (word >> 21) & 0x1 // bit 21
	op2 := (word >> 10) & 0x3F  // bits [15:10]
	return op == 0b01001110 && bit21 == 0 && op2 == 0b000011
}

// decodeSIMDCopy decodes SIMD copy instructions like DUP.
// Format: 0 | Q | 001110000 | imm5 | 000011 | Rn | Rd
// Q[30]: 0=64-bit (D), 1=128-bit (Q)
// imm5[20:16]: encodes element size and target arrangement
// Rn[9:5]: source general register
// Rd[4:0]: destination SIMD register
func (d *Decoder) decodeSIMDCopy(word uint32, inst *Instruction) {
	inst.Format = FormatSIMDCopy
	inst.IsSIMD = true
	inst.Op = OpDUP

	q := (word >> 30) & 0x1     // bit 30: 0=64-bit, 1=128-bit
	imm5 := (word >> 16) & 0x1F // bits [20:16]
	rn := (word >> 5) & 0x1F    // bits [9:5]
	rd := word & 0x1F           // bits [4:0]

	inst.Rd = uint8(rd)   // SIMD destination register
	inst.Rn = uint8(rn)   // General purpose source register
	inst.Is64Bit = q == 1 // 128-bit (Q) vs 64-bit (D)

	// Decode element size from imm5
	// imm5[0]: if 1, then 8-bit elements (byte)
	// imm5[1]: if 1 and imm5[0]==0, then 16-bit elements (halfword)
	// imm5[2]: if 1 and imm5[1:0]==00, then 32-bit elements (word)
	// imm5[3]: if 1 and imm5[2:0]==000, then 64-bit elements (doubleword)

	if imm5&0x1 != 0 {
		// 8-bit elements (byte)
		if q == 1 {
			inst.Arrangement = Arr16B // 16 bytes for Q register
		} else {
			inst.Arrangement = Arr8B // 8 bytes for D register
		}
	} else if imm5&0x2 != 0 {
		// 16-bit elements (halfword)
		if q == 1 {
			inst.Arrangement = Arr8H // 8 halfwords for Q register
		} else {
			inst.Arrangement = Arr4H // 4 halfwords for D register
		}
	} else if imm5&0x4 != 0 {
		// 32-bit elements (word)
		if q == 1 {
			inst.Arrangement = Arr4S // 4 singles for Q register
		} else {
			inst.Arrangement = Arr2S // 2 singles for D register
		}
	} else if imm5&0x8 != 0 {
		// 64-bit elements (doubleword) - only valid for Q register
		if q == 1 {
			inst.Arrangement = Arr2D // 2 doubles for Q register
		} else {
			// Invalid: 64-bit elements in D register
			inst.Op = OpUnknown
		}
	} else {
		// Invalid imm5 encoding
		inst.Op = OpUnknown
	}

	// Store element size info in Imm for execution
	inst.Imm = uint64(imm5)
}

// isSystemReg checks for system register instructions (MRS).
// MRS pattern: 1101010100 | L | 1 | o0:o1:o2:op1:CRn:CRm:op2 | Rt
// bits [31:21] == 0b11010101001 and L=1 for MRS (0xD53)
func (d *Decoder) isSystemReg(word uint32) bool {
	op := (word >> 20) & 0xFFF // bits [31:20]
	return op == 0xD53         // MRS has L=1, so 0xD53 instead of 0xD51
}

// decodeSystemReg decodes system register instructions (MRS).
// MRS format: 1101010100 | 1 | S:S:imm4:CRn:CRm:imm3 | Rt
func (d *Decoder) decodeSystemReg(word uint32, inst *Instruction) {
	inst.Format = FormatSystemReg
	inst.Op = OpMRS
	inst.Is64Bit = true // MRS always operates on 64-bit X registers

	// Extract fields
	rt := word & 0x1F              // bits [4:0] - destination register
	sysreg := (word >> 5) & 0x7FFF // bits [19:5] - system register encoding

	inst.Rd = uint8(rt)
	inst.SysReg = uint16(sysreg)
}
