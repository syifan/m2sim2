package pipeline

import (
	"fmt"

	"github.com/sarchlab/m2sim/emu"
	"github.com/sarchlab/m2sim/insts"
	"github.com/sarchlab/m2sim/timing/latency"
)

// FastTiming provides a simplified timing simulation optimized for calibration.
// It eliminates detailed pipeline simulation overhead while preserving basic
// timing relationships between instructions.
//
// Note on CPI accuracy: The CPI reported by fast timing reflects
// latency-weighted instruction mix, not pipeline-modeled CPI. There is no
// hazard, stall, or branch prediction modeling.
type FastTiming struct {
	regFile        *emu.RegFile
	memory         *emu.Memory
	decoder        *insts.Decoder
	latencyTable   *latency.Table
	syscallHandler emu.SyscallHandler

	// Simplified state
	PC              uint64
	halted          bool
	exitCode        int64
	cycleCount      uint64
	instrCount      uint64
	maxInstructions uint64 // 0 means no limit

	// Unhandled instruction tracking
	unhandledCount uint64
}

// FastTimingOption configures fast timing simulation.
type FastTimingOption func(*FastTiming)

// WithMaxInstructions sets the maximum number of instructions to execute.
// A value of 0 means no limit.
func WithMaxInstructions(max uint64) FastTimingOption {
	return func(ft *FastTiming) {
		ft.maxInstructions = max
	}
}

// NewFastTiming creates a new fast timing simulation.
func NewFastTiming(regFile *emu.RegFile, memory *emu.Memory, latencyTable *latency.Table, syscallHandler emu.SyscallHandler, opts ...FastTimingOption) *FastTiming {
	ft := &FastTiming{
		regFile:         regFile,
		memory:          memory,
		decoder:         insts.NewDecoder(),
		latencyTable:    latencyTable,
		syscallHandler:  syscallHandler,
		maxInstructions: 0, // Default: no limit
	}

	// Apply options
	for _, opt := range opts {
		opt(ft)
	}

	return ft
}

// SetPC sets the program counter.
func (ft *FastTiming) SetPC(pc uint64) {
	ft.PC = pc
}

// Run executes the fast timing simulation until halt.
func (ft *FastTiming) Run() int64 {
	for !ft.halted {
		ft.Tick()
	}
	if ft.unhandledCount > 0 {
		fmt.Printf("fast_timing: %d instructions executed as 1-cycle NOP (unhandled opcode)\n", ft.unhandledCount)
	}
	return ft.exitCode
}

// Tick executes one fast timing cycle.
func (ft *FastTiming) Tick() {
	if ft.halted {
		return
	}

	// Check instruction limit before executing
	if ft.maxInstructions > 0 && ft.instrCount >= ft.maxInstructions {
		ft.halted = true
		ft.exitCode = 0 // Exit normally when instruction limit reached
		return
	}

	ft.cycleCount++

	// Fetch and execute instruction
	word := ft.memory.Read32(ft.PC)
	inst := ft.decoder.Decode(word)

	if inst == nil || inst.Op == insts.OpUnknown {
		// Unknown instruction - halt
		ft.halted = true
		ft.exitCode = -1
		return
	}

	// Execute instruction with simplified timing
	ft.executeInstruction(inst, ft.PC)
	ft.instrCount++
}

// executeInstruction executes an instruction with simplified timing.
//
//nolint:gocyclo // Large switch over instruction opcodes is inherent to instruction dispatch.
func (ft *FastTiming) executeInstruction(inst *insts.Instruction, pc uint64) {
	writeReg := uint8(31) // XZR (no write)
	var writeValue uint64
	var instLatency uint64

	// Read operands
	rnValue := ft.regFile.ReadReg(inst.Rn)
	rmValue := ft.regFile.ReadReg(inst.Rm)

	switch inst.Op {
	case insts.OpADD:
		instLatency = ft.latencyTable.GetLatency(inst)
		writeReg = inst.Rd
		writeValue = ft.executeADD(inst, rnValue, rmValue)
		if inst.SetFlags {
			ft.updateFlagsAdd(rnValue, writeValue-rnValue, writeValue, inst.Is64Bit)
		}

	case insts.OpSUB:
		instLatency = ft.latencyTable.GetLatency(inst)
		writeReg = inst.Rd
		writeValue = ft.executeSUB(inst, rnValue, rmValue)
		var operand uint64
		switch inst.Format {
		case insts.FormatDPReg:
			operand = rmValue
		default:
			operand = uint64(int64(inst.Imm))
		}
		if inst.SetFlags {
			ft.updateFlagsSub(rnValue, operand, writeValue, inst.Is64Bit)
		}

	case insts.OpLDR:
		instLatency = ft.latencyTable.GetLatency(inst)
		writeReg = inst.Rd
		var addr uint64

		// Handle different addressing modes
		switch inst.IndexMode {
		case insts.IndexPost:
			addr = rnValue
			writeValue = ft.memory.Read64(addr)
			newAddr := rnValue + uint64(inst.SignedImm)
			ft.regFile.WriteReg(inst.Rn, newAddr)
		case insts.IndexPre:
			addr = rnValue + uint64(inst.SignedImm)
			writeValue = ft.memory.Read64(addr)
			ft.regFile.WriteReg(inst.Rn, addr)
		default:
			addr = rnValue + uint64(int64(inst.Imm))
			writeValue = ft.memory.Read64(addr)
		}

	case insts.OpSTR:
		instLatency = ft.latencyTable.GetLatency(inst)
		storeValue := ft.regFile.ReadReg(inst.Rd)
		switch inst.IndexMode {
		case insts.IndexPost:
			ft.memory.Write64(rnValue, storeValue)
			ft.regFile.WriteReg(inst.Rn, rnValue+uint64(inst.SignedImm))
		case insts.IndexPre:
			addr := rnValue + uint64(inst.SignedImm)
			ft.memory.Write64(addr, storeValue)
			ft.regFile.WriteReg(inst.Rn, addr)
		default:
			addr := rnValue + uint64(int64(inst.Imm))
			ft.memory.Write64(addr, storeValue)
		}

	case insts.OpB:
		ft.handleBranch(inst, pc)
		return

	case insts.OpBCond:
		ft.handleConditionalBranch(inst, pc)
		return

	case insts.OpSVC:
		ft.handleSyscall()
		return

	case insts.OpADRP:
		writeReg = inst.Rd
		pcPage := pc &^ 0xFFF
		pageOffset := int64(inst.Imm) << 12
		writeValue = uint64(int64(pcPage) + pageOffset)

	case insts.OpMOVZ:
		writeReg = inst.Rd
		shift := uint64(inst.Shift)
		writeValue = inst.Imm << shift

	case insts.OpMOVK:
		writeReg = inst.Rd
		shift := uint64(inst.Shift)
		mask := ^(uint64(0xFFFF) << shift)
		writeValue = (ft.regFile.ReadReg(inst.Rd) & mask) | (inst.Imm << shift)

	case insts.OpMOVN:
		writeReg = inst.Rd
		shift := uint64(inst.Shift)
		writeValue = ^(inst.Imm << shift)

	case insts.OpSTP:
		instLatency = ft.latencyTable.GetLatency(inst)
		addr := rnValue + uint64(inst.SignedImm)
		value1 := ft.regFile.ReadReg(inst.Rd)
		value2 := ft.regFile.ReadReg(inst.Rt2)
		ft.memory.Write64(addr, value1)
		ft.memory.Write64(addr+8, value2)
		if inst.IndexMode != insts.IndexNone && inst.IndexMode != insts.IndexSigned {
			ft.regFile.WriteReg(inst.Rn, addr)
		}

	case insts.OpLDP:
		instLatency = ft.latencyTable.GetLatency(inst)
		addr := rnValue + uint64(inst.SignedImm)
		value1 := ft.memory.Read64(addr)
		value2 := ft.memory.Read64(addr + 8)
		if inst.Rd != 31 {
			ft.regFile.WriteReg(inst.Rd, value1)
		}
		if inst.Rt2 != 31 {
			ft.regFile.WriteReg(inst.Rt2, value2)
		}
		if inst.IndexMode != insts.IndexNone && inst.IndexMode != insts.IndexSigned {
			ft.regFile.WriteReg(inst.Rn, addr)
		}

	case insts.OpBL:
		ft.regFile.WriteReg(30, pc+4)
		ft.PC = uint64(int64(pc) + inst.BranchOffset)
		return

	case insts.OpAND:
		instLatency = ft.latencyTable.GetLatency(inst)
		writeReg = inst.Rd
		switch inst.Format {
		case insts.FormatDPImm, insts.FormatLogicalImm:
			writeValue = rnValue & inst.Imm
		case insts.FormatDPReg:
			writeValue = rnValue & rmValue
		default:
			writeValue = rnValue & inst.Imm
		}
		if inst.SetFlags {
			ft.updateFlagsLogical(writeValue, inst.Is64Bit)
		}

	case insts.OpORR:
		instLatency = ft.latencyTable.GetLatency(inst)
		writeReg = inst.Rd
		switch inst.Format {
		case insts.FormatDPImm, insts.FormatLogicalImm:
			writeValue = rnValue | inst.Imm
		case insts.FormatDPReg:
			writeValue = rnValue | rmValue
		default:
			writeValue = rnValue | inst.Imm
		}

	case insts.OpEOR:
		instLatency = ft.latencyTable.GetLatency(inst)
		writeReg = inst.Rd
		switch inst.Format {
		case insts.FormatDPImm, insts.FormatLogicalImm:
			writeValue = rnValue ^ inst.Imm
		case insts.FormatDPReg:
			writeValue = rnValue ^ rmValue
		default:
			writeValue = rnValue ^ inst.Imm
		}

	case insts.OpBIC:
		instLatency = ft.latencyTable.GetLatency(inst)
		writeReg = inst.Rd
		writeValue = rnValue & ^rmValue
		if inst.SetFlags {
			ft.updateFlagsLogical(writeValue, inst.Is64Bit)
		}

	case insts.OpORN:
		instLatency = ft.latencyTable.GetLatency(inst)
		writeReg = inst.Rd
		writeValue = rnValue | ^rmValue

	case insts.OpEON:
		instLatency = ft.latencyTable.GetLatency(inst)
		writeReg = inst.Rd
		writeValue = rnValue ^ ^rmValue

	case insts.OpRET:
		targetAddr := ft.regFile.ReadReg(30)
		ft.PC = targetAddr
		return

	case insts.OpMADD:
		instLatency = ft.latencyTable.GetLatency(inst)
		raValue := ft.regFile.ReadReg(inst.Rt2) // Ra stored in Rt2
		writeReg = inst.Rd
		writeValue = raValue + rnValue*rmValue

	case insts.OpMSUB:
		instLatency = ft.latencyTable.GetLatency(inst)
		raValue := ft.regFile.ReadReg(inst.Rt2) // Ra stored in Rt2
		writeReg = inst.Rd
		writeValue = raValue - rnValue*rmValue

	case insts.OpCSEL:
		writeReg = inst.Rd
		if ft.evaluateCondition(uint8(inst.Cond)) {
			writeValue = rnValue
		} else {
			writeValue = rmValue
		}

	case insts.OpCSINC:
		writeReg = inst.Rd
		if ft.evaluateCondition(uint8(inst.Cond)) {
			writeValue = rnValue
		} else {
			writeValue = rmValue + 1
		}

	case insts.OpUBFM:
		writeReg = inst.Rd
		immr := inst.Imm  // immr stored in Imm
		imms := inst.Imm2 // imms stored in Imm2
		if imms == 63 && inst.Is64Bit {
			writeValue = rnValue >> immr
		} else if imms == 31 && !inst.Is64Bit {
			writeValue = (rnValue & 0xFFFFFFFF) >> immr
		} else if imms+1 == immr {
			if inst.Is64Bit {
				writeValue = rnValue << (64 - immr)
			} else {
				writeValue = (rnValue << (32 - immr)) & 0xFFFFFFFF
			}
		} else if immr == 0 {
			mask := (uint64(1) << (imms + 1)) - 1
			writeValue = rnValue & mask
		} else {
			width := imms - immr + 1
			mask := (uint64(1) << width) - 1
			writeValue = (rnValue >> immr) & mask
		}

	case insts.OpSBFM:
		writeReg = inst.Rd
		immr := inst.Imm  // immr stored in Imm
		imms := inst.Imm2 // imms stored in Imm2
		if imms == 63 && inst.Is64Bit {
			writeValue = uint64(int64(rnValue) >> immr)
		} else if imms == 31 && !inst.Is64Bit {
			val32 := int32(rnValue)
			writeValue = uint64(uint32(val32 >> immr))
		} else if immr == 0 && imms == 7 {
			writeValue = uint64(int64(int8(rnValue)))
		} else if immr == 0 && imms == 15 {
			writeValue = uint64(int64(int16(rnValue)))
		} else if immr == 0 && imms == 31 {
			writeValue = uint64(int64(int32(rnValue)))
		} else {
			width := imms - immr + 1
			mask := (uint64(1) << width) - 1
			extracted := (rnValue >> immr) & mask
			if extracted>>(width-1) != 0 {
				writeValue = extracted | ^mask
			} else {
				writeValue = extracted
			}
		}

	case insts.OpLSLV:
		writeReg = inst.Rd
		shift := rmValue & 63
		writeValue = rnValue << shift

	case insts.OpLSRV:
		writeReg = inst.Rd
		shift := rmValue & 63
		writeValue = rnValue >> shift

	case insts.OpASRV:
		writeReg = inst.Rd
		shift := rmValue & 63
		writeValue = uint64(int64(rnValue) >> shift)

	case insts.OpCBZ:
		val := ft.regFile.ReadReg(inst.Rd)
		if val == 0 {
			ft.PC = uint64(int64(pc) + inst.BranchOffset)
		} else {
			ft.PC += 4
		}
		return

	case insts.OpCBNZ:
		val := ft.regFile.ReadReg(inst.Rd)
		if val != 0 {
			ft.PC = uint64(int64(pc) + inst.BranchOffset)
		} else {
			ft.PC += 4
		}
		return

	case insts.OpTBZ:
		val := ft.regFile.ReadReg(inst.Rd)
		bitNum := inst.Imm // Bit number stored in Imm
		bit := (val >> bitNum) & 1
		if bit == 0 {
			ft.PC = uint64(int64(pc) + inst.BranchOffset)
		} else {
			ft.PC += 4
		}
		return

	case insts.OpTBNZ:
		val := ft.regFile.ReadReg(inst.Rd)
		bitNum := inst.Imm // Bit number stored in Imm
		bit := (val >> bitNum) & 1
		if bit != 0 {
			ft.PC = uint64(int64(pc) + inst.BranchOffset)
		} else {
			ft.PC += 4
		}
		return

	case insts.OpCCMP:
		if ft.evaluateCondition(uint8(inst.Cond)) {
			var op2 uint64
			if inst.Rm == 0xFF {
				op2 = inst.Imm2 // Immediate form
			} else {
				op2 = rmValue // Register form
			}
			result := rnValue - op2
			ft.updateFlagsSub(rnValue, op2, result, inst.Is64Bit)
		} else {
			nzcv := inst.Imm & 0xF
			ft.regFile.PSTATE.N = (nzcv>>3)&1 == 1
			ft.regFile.PSTATE.Z = (nzcv>>2)&1 == 1
			ft.regFile.PSTATE.C = (nzcv>>1)&1 == 1
			ft.regFile.PSTATE.V = nzcv&1 == 1
		}

	case insts.OpNOP, insts.OpMRS, insts.OpDUP:
		// NOP: nothing to do
		// MRS/DUP: simplified - treat as 1-cycle

	case insts.OpADR:
		writeReg = inst.Rd
		writeValue = uint64(int64(pc) + int64(inst.Imm))

	case insts.OpLDRB:
		instLatency = ft.latencyTable.GetLatency(inst)
		writeReg = inst.Rd
		addr := rnValue + uint64(int64(inst.Imm))
		writeValue = uint64(ft.memory.Read8(addr))

	case insts.OpSTRB:
		instLatency = ft.latencyTable.GetLatency(inst)
		addr := rnValue + uint64(int64(inst.Imm))
		storeValue := ft.regFile.ReadReg(inst.Rd)
		ft.memory.Write8(addr, uint8(storeValue))

	case insts.OpLDRH:
		instLatency = ft.latencyTable.GetLatency(inst)
		writeReg = inst.Rd
		addr := rnValue + uint64(int64(inst.Imm))
		writeValue = uint64(ft.memory.Read16(addr))

	case insts.OpSTRH:
		instLatency = ft.latencyTable.GetLatency(inst)
		addr := rnValue + uint64(int64(inst.Imm))
		storeValue := ft.regFile.ReadReg(inst.Rd)
		ft.memory.Write16(addr, uint16(storeValue))

	case insts.OpLDRSW:
		instLatency = ft.latencyTable.GetLatency(inst)
		writeReg = inst.Rd
		addr := rnValue + uint64(int64(inst.Imm))
		writeValue = uint64(int64(int32(ft.memory.Read32(addr))))

	default:
		// Unhandled opcode — treat as 1-cycle NOP but count it
		ft.unhandledCount++
	}

	// Handle instruction completion — write results immediately for
	// correctness (fast timing has no stall/forwarding model), but account
	// for multi-cycle latency in the cycle count.
	if writeReg != 31 {
		ft.regFile.WriteReg(writeReg, writeValue)
	}
	if instLatency > 1 {
		ft.cycleCount += instLatency - 1 // -1 because Tick already counted 1
	}

	// Advance PC
	ft.PC += 4
}

// executeADD performs ADD instruction calculation.
func (ft *FastTiming) executeADD(inst *insts.Instruction, rnValue, rmValue uint64) uint64 {
	switch inst.Format {
	case insts.FormatDPImm:
		return rnValue + uint64(int64(inst.Imm))
	case insts.FormatDPReg:
		return rnValue + rmValue
	default:
		return rnValue + uint64(int64(inst.Imm))
	}
}

// executeSUB performs SUB instruction calculation.
func (ft *FastTiming) executeSUB(inst *insts.Instruction, rnValue, rmValue uint64) uint64 {
	switch inst.Format {
	case insts.FormatDPImm:
		return rnValue - uint64(int64(inst.Imm))
	case insts.FormatDPReg:
		return rnValue - rmValue
	default:
		return rnValue - uint64(int64(inst.Imm))
	}
}

// updateFlagsAdd updates PSTATE for ADD/ADDS.
func (ft *FastTiming) updateFlagsAdd(op1, op2, result uint64, is64 bool) {
	if is64 {
		ft.regFile.PSTATE.N = (result >> 63) == 1
		ft.regFile.PSTATE.Z = result == 0
		ft.regFile.PSTATE.C = result < op1
		ft.regFile.PSTATE.V = ((^(op1 ^ op2)) & (op1 ^ result) >> 63) == 1
	} else {
		r32 := uint32(result)
		ft.regFile.PSTATE.N = (r32 >> 31) == 1
		ft.regFile.PSTATE.Z = r32 == 0
		ft.regFile.PSTATE.C = r32 < uint32(op1)
		ft.regFile.PSTATE.V = ((^(uint32(op1) ^ uint32(op2))) & (uint32(op1) ^ r32) >> 31) == 1
	}
}

// updateFlagsSub updates PSTATE for SUB/SUBS/CMP.
func (ft *FastTiming) updateFlagsSub(op1, op2, result uint64, is64 bool) {
	if is64 {
		ft.regFile.PSTATE.N = (result >> 63) == 1
		ft.regFile.PSTATE.Z = result == 0
		ft.regFile.PSTATE.C = op1 >= op2
		ft.regFile.PSTATE.V = ((op1 ^ op2) & (op1 ^ result) >> 63) == 1
	} else {
		r32 := uint32(result)
		ft.regFile.PSTATE.N = (r32 >> 31) == 1
		ft.regFile.PSTATE.Z = r32 == 0
		ft.regFile.PSTATE.C = uint32(op1) >= uint32(op2)
		ft.regFile.PSTATE.V = ((uint32(op1) ^ uint32(op2)) & (uint32(op1) ^ r32) >> 31) == 1
	}
}

// updateFlagsLogical updates PSTATE for ANDS (logical with flag set).
func (ft *FastTiming) updateFlagsLogical(result uint64, is64 bool) {
	if is64 {
		ft.regFile.PSTATE.N = (result >> 63) == 1
		ft.regFile.PSTATE.Z = result == 0
	} else {
		ft.regFile.PSTATE.N = (uint32(result) >> 31) == 1
		ft.regFile.PSTATE.Z = uint32(result) == 0
	}
	ft.regFile.PSTATE.C = false
	ft.regFile.PSTATE.V = false
}

// handleBranch processes unconditional branch instructions.
func (ft *FastTiming) handleBranch(inst *insts.Instruction, pc uint64) {
	ft.PC = uint64(int64(pc) + inst.BranchOffset)
}

// handleConditionalBranch processes conditional branch instructions.
func (ft *FastTiming) handleConditionalBranch(inst *insts.Instruction, pc uint64) {
	taken := ft.evaluateCondition(uint8(inst.Cond))

	if taken {
		ft.PC = uint64(int64(pc) + inst.BranchOffset)
	} else {
		ft.PC += 4
	}
}

// evaluateCondition evaluates a condition code.
func (ft *FastTiming) evaluateCondition(cond uint8) bool {
	pstate := &ft.regFile.PSTATE

	switch cond & 0xE {
	case 0x0: // EQ/NE
		return pstate.Z == (cond&1 == 0)
	case 0x2: // CS/CC
		return pstate.C == (cond&1 == 0)
	case 0x4: // MI/PL
		return pstate.N == (cond&1 == 0)
	case 0x6: // VS/VC
		return pstate.V == (cond&1 == 0)
	case 0x8: // HI/LS
		if cond&1 == 0 {
			return pstate.C && !pstate.Z
		}
		return !pstate.C || pstate.Z
	case 0xA: // GE/LT
		if cond&1 == 0 {
			return pstate.N == pstate.V
		}
		return pstate.N != pstate.V
	case 0xC: // GT/LE
		if cond&1 == 0 {
			return !pstate.Z && (pstate.N == pstate.V)
		}
		return pstate.Z || (pstate.N != pstate.V)
	case 0xE: // AL (always)
		return true
	default:
		return false
	}
}

// handleSyscall delegates syscall handling to the configured handler.
func (ft *FastTiming) handleSyscall() {
	if ft.syscallHandler != nil {
		result := ft.syscallHandler.Handle()
		if result.Exited {
			ft.halted = true
			ft.exitCode = result.ExitCode
			return
		}
	}

	ft.PC += 4
}

// Stats returns simulation statistics.
func (ft *FastTiming) Stats() Statistics {
	return Statistics{
		Cycles:       ft.cycleCount,
		Instructions: ft.instrCount,
		// No detailed hazard tracking in fast mode.
		// CPI = Cycles/Instructions reflects latency-weighted instruction mix only.
		Stalls:               0,
		Flushes:              0,
		ExecStalls:           0,
		MemStalls:            0,
		DataHazards:          0,
		BranchPredictions:    0,
		BranchCorrect:        0,
		BranchMispredictions: 0,
	}
}

// UnhandledCount returns the number of instructions that fell through
// to the default NOP path because they had no explicit handler.
func (ft *FastTiming) UnhandledCount() uint64 {
	return ft.unhandledCount
}
