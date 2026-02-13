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
	// Pool of pre-allocated instructions to avoid heap allocations during decode.
	// Must be large enough that no pool slot is reused while a previous instruction
	// decoded into that slot is still referenced by pipeline registers (IDEX/EXMEM).
	// With up to 9 decodes per cycle (8-wide + fusion) and 2-cycle reference lifetime,
	// 32 slots provide sufficient margin.
	instPool  [32]insts.Instruction
	poolIndex int
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
	// Get next available pre-allocated instruction from pool
	inst := &s.instPool[s.poolIndex]
	s.poolIndex = (s.poolIndex + 1) % len(s.instPool)

	// Use DecodeInto with pre-allocated instruction to eliminate heap allocation
	s.decoder.DecodeInto(word, inst)

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
		insts.OpLDRH, insts.OpLDRSH, insts.OpLDRLit, insts.OpLDRQ,
		insts.OpLDRSW:
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
	case insts.OpADD, insts.OpSUB, insts.OpAND, insts.OpORR, insts.OpEOR,
		insts.OpBIC, insts.OpORN, insts.OpEON:
		return true
	case insts.OpLDR, insts.OpLDP, insts.OpLDRB, insts.OpLDRSB,
		insts.OpLDRH, insts.OpLDRSH, insts.OpLDRLit, insts.OpLDRQ,
		insts.OpLDRSW:
		return true
	case insts.OpBL, insts.OpBLR:
		return true // BL/BLR write to X30
	case insts.OpMOVZ, insts.OpMOVN, insts.OpMOVK:
		return true
	case insts.OpMADD, insts.OpMSUB:
		return true
	case insts.OpADRP, insts.OpADR:
		return true
	case insts.OpUBFM, insts.OpSBFM, insts.OpEXTR, insts.OpBFM:
		return true
	case insts.OpCSEL, insts.OpCSINC, insts.OpCSINV, insts.OpCSNEG:
		return true
	case insts.OpUDIV, insts.OpSDIV:
		return true
	case insts.OpLSLV, insts.OpLSRV, insts.OpASRV, insts.OpRORV:
		return true
	default:
		return false
	}
}

// isBranchInst determines if the instruction is a branch.
func (s *DecodeStage) isBranchInst(inst *insts.Instruction) bool {
	switch inst.Op {
	case insts.OpB, insts.OpBL, insts.OpBCond, insts.OpBR, insts.OpBLR, insts.OpRET,
		insts.OpCBZ, insts.OpCBNZ, insts.OpTBZ, insts.OpTBNZ:
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

	// Flag output for flag-setting instructions (CMP, SUBS, ADDS).
	// These are stored in EXMEM for forwarding to dependent B.cond instructions.
	SetsFlags bool
	FlagN     bool
	FlagZ     bool
	FlagC     bool
	FlagV     bool
}

// Execute performs the ALU operation for the instruction.
// rnValue and rmValue are the (possibly forwarded) operand values.
func (s *ExecuteStage) Execute(idex *IDEXRegister, rnValue, rmValue uint64) ExecuteResult {
	// Call ExecuteWithFlags with no forwarded flags (reads from PSTATE)
	return s.ExecuteWithFlags(idex, rnValue, rmValue, false, false, false, false, false)
}

// ExecuteWithFlags performs the ALU operation with optional flag forwarding.
// When forwardFlags is true, the provided n, z, c, v flags are used instead of reading PSTATE.
// This fixes the pipeline timing hazard where CMP sets PSTATE at cycle END but B.cond reads
// at cycle START, causing stale flag reads.
func (s *ExecuteStage) ExecuteWithFlags(idex *IDEXRegister, rnValue, rmValue uint64,
	forwardFlags bool, fwdN, fwdZ, fwdC, fwdV bool) ExecuteResult {
	result := ExecuteResult{}

	if !idex.Valid || idex.Inst == nil {
		return result
	}

	inst := idex.Inst

	// Apply shift to Rm for data-processing register instructions.
	// This mirrors the emulator's applyShift64/applyShift32 in executeDPReg.
	if inst.Format == insts.FormatDPReg && inst.ShiftAmount > 0 {
		if inst.Is64Bit {
			switch inst.ShiftType {
			case insts.ShiftLSL:
				rmValue = rmValue << inst.ShiftAmount
			case insts.ShiftLSR:
				rmValue = rmValue >> inst.ShiftAmount
			case insts.ShiftASR:
				rmValue = uint64(int64(rmValue) >> inst.ShiftAmount)
			case insts.ShiftROR:
				rmValue = (rmValue >> inst.ShiftAmount) | (rmValue << (64 - inst.ShiftAmount))
			}
		} else {
			rm32 := uint32(rmValue)
			switch inst.ShiftType {
			case insts.ShiftLSL:
				rm32 = rm32 << inst.ShiftAmount
			case insts.ShiftLSR:
				rm32 = rm32 >> inst.ShiftAmount
			case insts.ShiftASR:
				rm32 = uint32(int32(rm32) >> inst.ShiftAmount)
			case insts.ShiftROR:
				rm32 = (rm32 >> inst.ShiftAmount) | (rm32 << (32 - inst.ShiftAmount))
			}
			rmValue = uint64(rm32)
		}
	}

	switch inst.Op {
	case insts.OpADD:
		result.ALUResult = s.executeADD(inst, rnValue, rmValue)
		if inst.SetFlags {
			n, z, c, v := s.computeAddFlags(inst, rnValue, rmValue, result.ALUResult)
			s.regFile.PSTATE.N = n
			s.regFile.PSTATE.Z = z
			s.regFile.PSTATE.C = c
			s.regFile.PSTATE.V = v
			result.SetsFlags = true
			result.FlagN = n
			result.FlagZ = z
			result.FlagC = c
			result.FlagV = v
		}
	case insts.OpSUB:
		result.ALUResult = s.executeSUB(inst, rnValue, rmValue)
		if inst.SetFlags {
			n, z, c, v := s.computeSubFlags(inst, rnValue, rmValue, result.ALUResult)
			s.regFile.PSTATE.N = n
			s.regFile.PSTATE.Z = z
			s.regFile.PSTATE.C = c
			s.regFile.PSTATE.V = v
			result.SetsFlags = true
			result.FlagN = n
			result.FlagZ = z
			result.FlagC = c
			result.FlagV = v
		}
	case insts.OpAND:
		result.ALUResult = s.executeAND(inst, rnValue, rmValue)
	case insts.OpORR:
		result.ALUResult = s.executeORR(inst, rnValue, rmValue)
	case insts.OpEOR:
		result.ALUResult = s.executeEOR(inst, rnValue, rmValue)
	case insts.OpBIC:
		result.ALUResult = s.executeBIC(inst, rnValue, rmValue)
	case insts.OpORN:
		result.ALUResult = s.executeORN(inst, rnValue, rmValue)
	case insts.OpEON:
		result.ALUResult = s.executeEON(inst, rnValue, rmValue)
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
			// Pre-index: address = base + signed offset, writeback base
			newAddr := uint64(int64(baseAddr) + inst.SignedImm)
			result.ALUResult = newAddr
			if inst.Rn == 31 {
				s.regFile.SP = newAddr
			} else {
				s.regFile.WriteReg(inst.Rn, newAddr)
			}
		case insts.IndexPost:
			// Post-index: address = base, writeback base + offset
			result.ALUResult = baseAddr
			newAddr := uint64(int64(baseAddr) + inst.SignedImm)
			if inst.Rn == 31 {
				s.regFile.SP = newAddr
			} else {
				s.regFile.WriteReg(inst.Rn, newAddr)
			}
		case insts.IndexRegBase:
			// Register offset: base + (extended Rm << shift)
			var offset uint64
			switch inst.ShiftType {
			case 0b010: // UXTW
				offset = uint64(uint32(rmValue))
			case 0b011: // LSL/UXTX
				offset = rmValue
			case 0b110: // SXTW
				offset = uint64(int64(int32(rmValue)))
			case 0b111: // SXTX
				offset = rmValue
			default:
				offset = rmValue
			}
			offset <<= inst.ShiftAmount
			result.ALUResult = baseAddr + offset
		default:
			// Unsigned offset or signed offset for LDP/STP
			if inst.Format == insts.FormatLoadStorePair {
				result.ALUResult = uint64(int64(baseAddr) + inst.SignedImm)
			} else {
				result.ALUResult = baseAddr + inst.Imm
			}
		}
		if inst.IndexMode == insts.IndexRegBase {
			result.StoreValue = s.regFile.ReadReg(inst.Rd)
		} else {
			result.StoreValue = rmValue // For STR, the value to store
		}
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
		} else if forwardFlags {
			// Non-fused with flag forwarding: use forwarded flags from previous
			// flag-setting instruction (e.g., CMP in EXMEM stage).
			// This fixes the pipeline timing hazard where CMP sets PSTATE at cycle
			// END but B.cond reads at cycle START.
			conditionMet = EvaluateConditionWithFlags(inst.Cond, fwdN, fwdZ, fwdC, fwdV)
		} else {
			// Non-fused without forwarding: read condition from PSTATE
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

	case insts.OpMOVZ:
		shift := uint64(inst.Shift)
		result.ALUResult = inst.Imm << shift

	case insts.OpMOVN:
		shift := uint64(inst.Shift)
		result.ALUResult = ^(inst.Imm << shift)
		if !inst.Is64Bit {
			result.ALUResult &= 0xFFFFFFFF
		}

	case insts.OpMOVK:
		shift := uint64(inst.Shift)
		mask := ^(uint64(0xFFFF) << shift)
		current := s.regFile.ReadReg(inst.Rd)
		result.ALUResult = (current & mask) | (inst.Imm << shift)

	case insts.OpMADD:
		raValue := s.regFile.ReadReg(inst.Rt2)
		if inst.Is64Bit {
			result.ALUResult = raValue + rnValue*rmValue
		} else {
			result.ALUResult = uint64(uint32(raValue) + uint32(rnValue)*uint32(rmValue))
		}

	case insts.OpMSUB:
		raValue := s.regFile.ReadReg(inst.Rt2)
		if inst.Is64Bit {
			result.ALUResult = raValue - rnValue*rmValue
		} else {
			result.ALUResult = uint64(uint32(raValue) - uint32(rnValue)*uint32(rmValue))
		}

	case insts.OpADRP:
		pcPage := idex.PC &^ 0xFFF
		pageOffset := int64(inst.Imm) << 12
		result.ALUResult = uint64(int64(pcPage) + pageOffset)

	case insts.OpADR:
		result.ALUResult = uint64(int64(idex.PC) + int64(inst.Imm))

	case insts.OpUBFM:
		immr := inst.Imm
		imms := inst.Imm2
		if inst.Is64Bit {
			if imms >= immr {
				width := imms - immr + 1
				mask := (uint64(1) << width) - 1
				result.ALUResult = (rnValue >> immr) & mask
			} else {
				shift := uint64(64) - immr
				width := imms + 1
				mask := (uint64(1) << width) - 1
				result.ALUResult = (rnValue & mask) << shift
			}
		} else {
			rn32 := uint32(rnValue)
			immr32 := uint32(immr)
			imms32 := uint32(imms)
			if imms32 >= immr32 {
				width := imms32 - immr32 + 1
				mask := (uint32(1) << width) - 1
				result.ALUResult = uint64((rn32 >> immr32) & mask)
			} else {
				shift := uint32(32) - immr32
				width := imms32 + 1
				mask := (uint32(1) << width) - 1
				result.ALUResult = uint64((rn32 & mask) << shift)
			}
		}

	case insts.OpSBFM:
		immr := inst.Imm
		imms := inst.Imm2
		if inst.Is64Bit {
			if imms >= immr {
				width := imms - immr + 1
				mask := (uint64(1) << width) - 1
				extracted := (rnValue >> immr) & mask
				signBit := uint64(1) << (width - 1)
				if extracted&signBit != 0 {
					extracted |= ^mask
				}
				result.ALUResult = extracted
			} else {
				shift := uint64(64) - immr
				width := imms + 1
				mask := (uint64(1) << width) - 1
				extracted := rnValue & mask
				signBit := uint64(1) << (width - 1)
				if extracted&signBit != 0 {
					extracted |= ^mask
				}
				result.ALUResult = extracted << shift
			}
		} else {
			rn32 := uint32(rnValue)
			immr32 := uint32(immr)
			imms32 := uint32(imms)
			if imms32 >= immr32 {
				width := imms32 - immr32 + 1
				mask := (uint32(1) << width) - 1
				extracted := (rn32 >> immr32) & mask
				signBit := uint32(1) << (width - 1)
				if extracted&signBit != 0 {
					extracted |= ^mask
				}
				result.ALUResult = uint64(extracted)
			} else {
				shift := uint32(32) - immr32
				width := imms32 + 1
				mask := (uint32(1) << width) - 1
				extracted := rn32 & mask
				signBit := uint32(1) << (width - 1)
				if extracted&signBit != 0 {
					extracted |= ^mask
				}
				result.ALUResult = uint64(extracted << shift)
			}
		}

	case insts.OpBFM:
		rdVal := s.regFile.ReadReg(inst.Rd)
		immr := uint32(inst.Imm)
		imms := uint32(inst.Imm2)
		if inst.Is64Bit {
			if imms >= immr {
				width := imms - immr + 1
				srcMask := (uint64(1) << width) - 1
				bits := (rnValue >> immr) & srcMask
				result.ALUResult = (rdVal &^ srcMask) | bits
			} else {
				shift := 64 - immr
				width := imms + 1
				srcMask := (uint64(1) << width) - 1
				bits := (rnValue & srcMask) << shift
				dstMask := srcMask << shift
				result.ALUResult = (rdVal &^ dstMask) | bits
			}
		} else {
			rd32 := uint32(rdVal)
			rn32 := uint32(rnValue)
			if imms >= immr {
				width := imms - immr + 1
				srcMask := (uint32(1) << width) - 1
				bits := (rn32 >> immr) & srcMask
				result.ALUResult = uint64((rd32 &^ srcMask) | bits)
			} else {
				shift := 32 - immr
				width := imms + 1
				srcMask := (uint32(1) << width) - 1
				bits := (rn32 & srcMask) << shift
				dstMask := srcMask << shift
				result.ALUResult = uint64((rd32 &^ dstMask) | bits)
			}
		}

	case insts.OpEXTR:
		lsb := uint32(inst.Imm)
		if inst.Is64Bit {
			if lsb == 0 {
				result.ALUResult = rnValue
			} else if lsb == 64 {
				result.ALUResult = rmValue
			} else {
				result.ALUResult = (rnValue >> lsb) | (rmValue << (64 - lsb))
			}
		} else {
			rn32 := uint32(rnValue)
			rm32 := uint32(rmValue)
			if lsb == 0 {
				result.ALUResult = uint64(rn32)
			} else if lsb == 32 {
				result.ALUResult = uint64(rm32)
			} else {
				result.ALUResult = uint64((rn32 >> lsb) | (rm32 << (32 - lsb)))
			}
		}

	case insts.OpCSEL:
		if s.checkCondition(inst.Cond) {
			result.ALUResult = rnValue
		} else {
			result.ALUResult = rmValue
		}

	case insts.OpCSINC:
		if s.checkCondition(inst.Cond) {
			result.ALUResult = rnValue
		} else {
			result.ALUResult = rmValue + 1
		}

	case insts.OpCSINV:
		if s.checkCondition(inst.Cond) {
			result.ALUResult = rnValue
		} else {
			result.ALUResult = ^rmValue
		}

	case insts.OpCSNEG:
		if s.checkCondition(inst.Cond) {
			result.ALUResult = rnValue
		} else {
			result.ALUResult = -rmValue
		}

	case insts.OpUDIV:
		if inst.Is64Bit {
			if rmValue == 0 {
				result.ALUResult = 0
			} else {
				result.ALUResult = rnValue / rmValue
			}
		} else {
			rn32 := uint32(rnValue)
			rm32 := uint32(rmValue)
			if rm32 == 0 {
				result.ALUResult = 0
			} else {
				result.ALUResult = uint64(rn32 / rm32)
			}
		}

	case insts.OpSDIV:
		if inst.Is64Bit {
			if rmValue == 0 {
				result.ALUResult = 0
			} else {
				result.ALUResult = uint64(int64(rnValue) / int64(rmValue))
			}
		} else {
			rn32 := int32(rnValue)
			rm32 := int32(rmValue)
			if rm32 == 0 {
				result.ALUResult = 0
			} else {
				result.ALUResult = uint64(uint32(rn32 / rm32))
			}
		}

	case insts.OpLSLV:
		if inst.Is64Bit {
			shift := rmValue & 0x3F
			result.ALUResult = rnValue << shift
		} else {
			shift := uint32(rmValue) & 0x1F
			result.ALUResult = uint64(uint32(rnValue) << shift)
		}

	case insts.OpLSRV:
		if inst.Is64Bit {
			shift := rmValue & 0x3F
			result.ALUResult = rnValue >> shift
		} else {
			shift := uint32(rmValue) & 0x1F
			result.ALUResult = uint64(uint32(rnValue) >> shift)
		}

	case insts.OpASRV:
		if inst.Is64Bit {
			shift := rmValue & 0x3F
			result.ALUResult = uint64(int64(rnValue) >> shift)
		} else {
			shift := uint32(rmValue) & 0x1F
			result.ALUResult = uint64(uint32(int32(rnValue) >> shift))
		}

	case insts.OpRORV:
		if inst.Is64Bit {
			shift := rmValue & 0x3F
			result.ALUResult = (rnValue >> shift) | (rnValue << (64 - shift))
		} else {
			rn32 := uint32(rnValue)
			shift := uint32(rmValue) & 0x1F
			result.ALUResult = uint64((rn32 >> shift) | (rn32 << (32 - shift)))
		}

	case insts.OpCBZ:
		// Compare and branch if zero - Rd holds the register to test
		val := s.regFile.ReadReg(inst.Rd)
		if !inst.Is64Bit {
			val &= 0xFFFFFFFF
		}
		if val == 0 {
			result.BranchTaken = true
			result.BranchTarget = uint64(int64(idex.PC) + inst.BranchOffset)
		}

	case insts.OpCBNZ:
		// Compare and branch if not zero
		val := s.regFile.ReadReg(inst.Rd)
		if !inst.Is64Bit {
			val &= 0xFFFFFFFF
		}
		if val != 0 {
			result.BranchTaken = true
			result.BranchTarget = uint64(int64(idex.PC) + inst.BranchOffset)
		}

	case insts.OpTBZ:
		// Test bit and branch if zero
		val := s.regFile.ReadReg(inst.Rd)
		bitNum := inst.Imm
		bit := (val >> bitNum) & 1
		if bit == 0 {
			result.BranchTaken = true
			result.BranchTarget = uint64(int64(idex.PC) + inst.BranchOffset)
		}

	case insts.OpTBNZ:
		// Test bit and branch if not zero
		val := s.regFile.ReadReg(inst.Rd)
		bitNum := inst.Imm
		bit := (val >> bitNum) & 1
		if bit != 0 {
			result.BranchTaken = true
			result.BranchTarget = uint64(int64(idex.PC) + inst.BranchOffset)
		}

	case insts.OpCCMP:
		if s.checkCondition(inst.Cond) {
			var op2 uint64
			if inst.Rm == 0xFF {
				op2 = inst.Imm2
			} else {
				op2 = rmValue
			}
			subResult := rnValue - op2
			if inst.Is64Bit {
				s.regFile.PSTATE.N = (subResult >> 63) == 1
				s.regFile.PSTATE.Z = subResult == 0
				s.regFile.PSTATE.C = rnValue >= op2
				s.regFile.PSTATE.V = ((rnValue^op2)&(rnValue^subResult))>>63 == 1
			} else {
				rn32 := uint32(rnValue)
				op32 := uint32(op2)
				r32 := rn32 - op32
				s.regFile.PSTATE.N = (r32 >> 31) == 1
				s.regFile.PSTATE.Z = r32 == 0
				s.regFile.PSTATE.C = rn32 >= op32
				s.regFile.PSTATE.V = ((rn32^op32)&(rn32^r32))>>31 == 1
			}
		} else {
			nzcv := inst.Imm & 0xF
			s.regFile.PSTATE.N = (nzcv>>3)&1 == 1
			s.regFile.PSTATE.Z = (nzcv>>2)&1 == 1
			s.regFile.PSTATE.C = (nzcv>>1)&1 == 1
			s.regFile.PSTATE.V = nzcv&1 == 1
		}

	case insts.OpLDRSW:
		// Address calculation for LDRSW (same as LDR)
		baseAddr := rnValue
		if inst.Rn == 31 {
			baseAddr = s.regFile.SP
		}
		switch inst.IndexMode {
		case insts.IndexPre:
			result.ALUResult = uint64(int64(baseAddr) + inst.SignedImm)
		case insts.IndexPost:
			result.ALUResult = baseAddr
		case insts.IndexRegBase:
			var offset uint64
			switch inst.ShiftType {
			case 0b010: // UXTW
				offset = uint64(uint32(rmValue))
			case 0b011: // LSL/UXTX
				offset = rmValue
			case 0b110: // SXTW
				offset = uint64(int64(int32(rmValue)))
			case 0b111: // SXTX
				offset = rmValue
			default:
				offset = rmValue
			}
			offset <<= inst.ShiftAmount
			result.ALUResult = baseAddr + offset
		default:
			result.ALUResult = baseAddr + inst.Imm
		}

	case insts.OpLDRLit:
		// PC-relative literal load
		result.ALUResult = uint64(int64(idex.PC) + inst.BranchOffset)
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

func (s *ExecuteStage) executeBIC(inst *insts.Instruction, rnValue, rmValue uint64) uint64 {
	if inst.Is64Bit {
		return rnValue & ^rmValue
	}
	return uint64(uint32(rnValue) & ^uint32(rmValue))
}

func (s *ExecuteStage) executeORN(inst *insts.Instruction, rnValue, rmValue uint64) uint64 {
	if inst.Is64Bit {
		return rnValue | ^rmValue
	}
	return uint64(uint32(rnValue) | ^uint32(rmValue))
}

func (s *ExecuteStage) executeEON(inst *insts.Instruction, rnValue, rmValue uint64) uint64 {
	if inst.Is64Bit {
		return rnValue ^ ^rmValue
	}
	return uint64(uint32(rnValue) ^ ^uint32(rmValue))
}

// computeAddFlags computes PSTATE flags for an ADD/ADDS operation without setting them.
// Returns n, z, c, v flag values.
func (s *ExecuteStage) computeAddFlags(inst *insts.Instruction, op1, op2, result uint64) (n, z, c, v bool) {
	if inst.Is64Bit {
		// 64-bit flags
		n = (result >> 63) == 1
		z = result == 0
		c = result < op1 // unsigned overflow (carry out)
		// V: signed overflow - adding same signs gives different sign
		op1Sign := op1 >> 63
		op2Sign := op2 >> 63
		resultSign := result >> 63
		v = (op1Sign == op2Sign) && (op1Sign != resultSign)
	} else {
		// 32-bit flags
		r32 := uint32(result)
		o1 := uint32(op1)
		o2 := uint32(op2)
		n = (r32 >> 31) == 1
		z = r32 == 0
		c = r32 < o1
		op1Sign := o1 >> 31
		op2Sign := o2 >> 31
		resultSign := r32 >> 31
		v = (op1Sign == op2Sign) && (op1Sign != resultSign)
	}
	return
}

// computeSubFlags computes PSTATE flags for a SUB/SUBS/CMP operation without setting them.
// Returns n, z, c, v flag values.
func (s *ExecuteStage) computeSubFlags(inst *insts.Instruction, op1, op2, result uint64) (n, z, c, v bool) {
	if inst.Is64Bit {
		// 64-bit flags
		n = (result >> 63) == 1
		z = result == 0
		c = op1 >= op2 // no borrow
		// V: signed overflow - subtracting different signs gives wrong sign
		op1Sign := op1 >> 63
		op2Sign := op2 >> 63
		resultSign := result >> 63
		v = (op1Sign != op2Sign) && (op2Sign == resultSign)
	} else {
		// 32-bit flags
		r32 := uint32(result)
		o1 := uint32(op1)
		o2 := uint32(op2)
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
		if exmem.Inst != nil && exmem.Inst.Op == insts.OpLDRSW {
			// LDRSW: read 32-bit and sign-extend to 64-bit
			result.MemData = uint64(int64(int32(s.memory.Read32(addr))))
		} else if exmem.Inst != nil && exmem.Inst.Is64Bit {
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
	GetPC() uint64
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
		if inst != nil && inst.Op == insts.OpLDRSW {
			// LDRSW: read 32-bit and sign-extend to 64-bit
			result.MemData = uint64(int64(int32(s.memory.Read32(addr))))
		} else if inst != nil && inst.Is64Bit {
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

// WritebackSlots performs batched writeback for multiple MEMWB slots.
// Returns the total number of instructions retired.
// This optimization reduces function call overhead in tickOctupleIssue.
func (s *WritebackStage) WritebackSlots(slots []WritebackSlot) uint64 {
	retired := uint64(0)

	// Batch process all slots to reduce function call overhead
	for _, slot := range slots {
		if !slot.IsValid() {
			continue
		}

		retired++

		// Skip register write operations
		if !slot.GetRegWrite() || slot.GetRd() == 31 {
			continue
		}

		// Select value source
		var value uint64
		if slot.GetMemToReg() {
			value = slot.GetMemData()
		} else {
			value = slot.GetALUResult()
		}

		// Write to register file
		s.regFile.WriteReg(slot.GetRd(), value)
	}

	return retired
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
