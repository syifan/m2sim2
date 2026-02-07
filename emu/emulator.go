// Package emu provides functional ARM64 emulation.
package emu

import (
	"fmt"
	"io"
	"os"

	"github.com/sarchlab/m2sim/insts"
)

// StepResult represents the result of executing a single instruction.
type StepResult struct {
	// Exited is true if the program terminated (via exit syscall).
	Exited bool

	// ExitCode is the exit status if Exited is true.
	ExitCode int64

	// Err is set if an error occurred during execution.
	Err error
}

// Emulator executes ARM64 instructions functionally.
type Emulator struct {
	regFile        *RegFile
	memory         *Memory
	decoder        *insts.Decoder
	syscallHandler SyscallHandler

	// Execution units
	alu        *ALU
	lsu        *LoadStoreUnit
	branchUnit *BranchUnit
	simdUnit   *SIMD

	// SIMD register file
	simdRegFile *SIMDRegFile

	// I/O
	stdout io.Writer
	stderr io.Writer

	// Execution state
	instructionCount uint64
	maxInstructions  uint64 // 0 means no limit
}

// EmulatorOption is a functional option for configuring the Emulator.
type EmulatorOption func(*Emulator)

// WithStdout sets a custom stdout writer.
func WithStdout(w io.Writer) EmulatorOption {
	return func(e *Emulator) {
		e.stdout = w
	}
}

// WithStderr sets a custom stderr writer.
func WithStderr(w io.Writer) EmulatorOption {
	return func(e *Emulator) {
		e.stderr = w
	}
}

// WithSyscallHandler sets a custom syscall handler.
func WithSyscallHandler(handler SyscallHandler) EmulatorOption {
	return func(e *Emulator) {
		e.syscallHandler = handler
	}
}

// WithStackPointer sets the initial stack pointer value.
func WithStackPointer(sp uint64) EmulatorOption {
	return func(e *Emulator) {
		e.regFile.SP = sp
	}
}

// WithMaxInstructions sets the maximum number of instructions to execute.
// A value of 0 means no limit.
func WithMaxInstructions(max uint64) EmulatorOption {
	return func(e *Emulator) {
		e.maxInstructions = max
	}
}

// NewEmulator creates a new ARM64 emulator.
func NewEmulator(opts ...EmulatorOption) *Emulator {
	regFile := &RegFile{}
	memory := NewMemory()

	e := &Emulator{
		regFile:          regFile,
		memory:           memory,
		decoder:          insts.NewDecoder(),
		stdout:           os.Stdout,
		stderr:           os.Stderr,
		instructionCount: 0,
		maxInstructions:  0,
	}

	// Apply options first (may set stdout/stderr)
	for _, opt := range opts {
		opt(e)
	}

	// Create execution units
	e.alu = NewALU(regFile)
	e.lsu = NewLoadStoreUnit(regFile, memory)
	e.branchUnit = NewBranchUnit(regFile)
	e.simdRegFile = NewSIMDRegFile()
	e.simdUnit = NewSIMD(e.simdRegFile, regFile, memory)

	// If no syscall handler was provided, create a default one
	if e.syscallHandler == nil {
		e.syscallHandler = NewDefaultSyscallHandler(regFile, memory, e.stdout, e.stderr)
	}

	return e
}

// RegFile returns the emulator's register file.
func (e *Emulator) RegFile() *RegFile {
	return e.regFile
}

// Memory returns the emulator's memory.
func (e *Emulator) Memory() *Memory {
	return e.memory
}

// SIMDRegFile returns the emulator's SIMD register file.
func (e *Emulator) SIMDRegFile() *SIMDRegFile {
	return e.simdRegFile
}

// InstructionCount returns the number of instructions executed.
func (e *Emulator) InstructionCount() uint64 {
	return e.instructionCount
}

// LoadProgram loads a program into memory and sets the entry point.
// The program can be either a []byte or a *Memory.
func (e *Emulator) LoadProgram(entry uint64, program interface{}) {
	switch p := program.(type) {
	case []byte:
		e.memory.LoadProgram(entry, p)
	case *Memory:
		// Use the provided memory directly
		e.memory = p
		// Update execution units to use new memory
		e.lsu = NewLoadStoreUnit(e.regFile, e.memory)
		e.simdUnit = NewSIMD(e.simdRegFile, e.regFile, e.memory)
		// Update syscall handler with new memory
		e.syscallHandler = NewDefaultSyscallHandler(e.regFile, e.memory, e.stdout, e.stderr)
	}
	e.regFile.PC = entry
}

// Reset resets the emulator to its initial state.
func (e *Emulator) Reset() {
	e.regFile = &RegFile{}
	e.memory = NewMemory()
	e.instructionCount = 0

	// Recreate execution units
	e.alu = NewALU(e.regFile)
	e.lsu = NewLoadStoreUnit(e.regFile, e.memory)
	e.branchUnit = NewBranchUnit(e.regFile)
	e.simdRegFile = NewSIMDRegFile()
	e.simdUnit = NewSIMD(e.simdRegFile, e.regFile, e.memory)

	// Recreate syscall handler
	e.syscallHandler = NewDefaultSyscallHandler(e.regFile, e.memory, e.stdout, e.stderr)
}

// Step executes a single instruction.
// Returns a StepResult indicating whether execution should continue.
func (e *Emulator) Step() StepResult {
	// Check instruction limit before executing
	if e.maxInstructions > 0 && e.instructionCount >= e.maxInstructions {
		return StepResult{
			Err: fmt.Errorf("max instructions reached"),
		}
	}

	// 1. Fetch: Read 4 bytes at PC
	word := e.memory.Read32(e.regFile.PC)

	// 2. Decode
	inst := e.decoder.Decode(word)

	// 3. Execute
	result := e.execute(inst)

	// Increment instruction count
	e.instructionCount++

	return result
}

// Run executes instructions until the program exits or an error occurs.
// Returns the exit code (-1 if error).
func (e *Emulator) Run() int64 {
	for {
		result := e.Step()
		if result.Exited {
			return result.ExitCode
		}
		if result.Err != nil {
			// Print error for debugging
			_, _ = fmt.Fprintf(e.stderr, "Emulation error: %v\n", result.Err)
			return -1
		}
	}
}

// execute dispatches and executes a decoded instruction.
func (e *Emulator) execute(inst *insts.Instruction) StepResult {
	// Check for unknown instruction
	if inst.Op == insts.OpUnknown {
		return StepResult{
			Err: fmt.Errorf("unknown instruction at PC=0x%X", e.regFile.PC),
		}
	}

	// Handle SVC (syscall) separately
	if inst.Op == insts.OpSVC {
		return e.executeSVC()
	}

	// Handle BRK (breakpoint/trap) - indicates an error condition or assertion
	if inst.Op == insts.OpBRK {
		return StepResult{
			Exited:   true,
			ExitCode: -1, // Trap exit code
			Err:      fmt.Errorf("BRK trap #0x%X at PC=0x%X", inst.Imm, e.regFile.PC),
		}
	}

	// Handle NOP - no operation, just advance PC
	if inst.Op == insts.OpNOP {
		e.regFile.PC += 4
		return StepResult{}
	}

	// Execute based on instruction type
	switch inst.Format {
	case insts.FormatDPImm:
		e.executeDPImm(inst)
	case insts.FormatDPReg:
		e.executeDPReg(inst)
	case insts.FormatLogicalImm:
		e.executeLogicalImm(inst)
	case insts.FormatBitfield:
		e.executeBitfield(inst)
	case insts.FormatExtract:
		e.executeExtract(inst)
	case insts.FormatBranch:
		e.executeBranch(inst)
		return StepResult{} // PC already updated by branch
	case insts.FormatBranchCond:
		e.executeBranchCond(inst)
		return StepResult{} // PC already updated
	case insts.FormatBranchReg:
		e.executeBranchReg(inst)
		return StepResult{} // PC already updated
	case insts.FormatLoadStore:
		e.executeLoadStore(inst)
	case insts.FormatLoadStorePair:
		e.executeLoadStorePair(inst)
	case insts.FormatPCRel:
		e.executePCRel(inst)
	case insts.FormatLoadStoreLit:
		e.executeLoadStoreLit(inst)
	case insts.FormatMoveWide:
		e.executeMoveWide(inst)
	case insts.FormatCondSelect:
		e.executeCondSelect(inst)
	case insts.FormatCondCmp:
		e.executeCondCmp(inst)
	case insts.FormatDataProc2Src:
		e.executeDataProc2Src(inst)
	case insts.FormatDataProc3Src:
		e.executeDataProc3Src(inst)
	case insts.FormatTestBranch:
		e.executeTestBranch(inst)
		return StepResult{} // PC already updated by branch
	case insts.FormatCompareBranch:
		e.executeCompareBranch(inst)
		return StepResult{} // PC already updated by branch
	case insts.FormatSIMDReg:
		e.executeSIMDReg(inst)
	case insts.FormatSIMDLoadStore:
		e.executeSIMDLoadStore(inst)
	default:
		return StepResult{
			Err: fmt.Errorf("unimplemented format %d at PC=0x%X", inst.Format, e.regFile.PC),
		}
	}

	// Advance PC by 4 (for non-branch instructions)
	e.regFile.PC += 4

	return StepResult{}
}

// executeSVC handles the SVC (supervisor call) instruction.
func (e *Emulator) executeSVC() StepResult {
	// Advance PC first (syscall return address is next instruction)
	e.regFile.PC += 4

	// Invoke syscall handler
	syscallResult := e.syscallHandler.Handle()

	return StepResult{
		Exited:   syscallResult.Exited,
		ExitCode: syscallResult.ExitCode,
	}
}

// executeDPImm executes Data Processing Immediate instructions.
func (e *Emulator) executeDPImm(inst *insts.Instruction) {
	imm := inst.Imm
	if inst.Shift > 0 {
		imm <<= inst.Shift
	}

	switch inst.Op {
	case insts.OpADD:
		if inst.Is64Bit {
			e.alu.ADD64Imm(inst.Rd, inst.Rn, imm, inst.SetFlags)
		} else {
			e.alu.ADD32Imm(inst.Rd, inst.Rn, uint32(imm), inst.SetFlags)
		}
	case insts.OpSUB:
		if inst.Is64Bit {
			e.alu.SUB64Imm(inst.Rd, inst.Rn, imm, inst.SetFlags)
		} else {
			e.alu.SUB32Imm(inst.Rd, inst.Rn, uint32(imm), inst.SetFlags)
		}
	}
}

// applyShift64 applies a shift operation to a 64-bit value.
func applyShift64(value uint64, shiftType insts.ShiftType, amount uint8) uint64 {
	if amount == 0 {
		return value
	}
	switch shiftType {
	case insts.ShiftLSL:
		return value << amount
	case insts.ShiftLSR:
		return value >> amount
	case insts.ShiftASR:
		return uint64(int64(value) >> amount)
	case insts.ShiftROR:
		return (value >> amount) | (value << (64 - amount))
	default:
		return value
	}
}

// applyShift32 applies a shift operation to a 32-bit value.
func applyShift32(value uint32, shiftType insts.ShiftType, amount uint8) uint32 {
	if amount == 0 {
		return value
	}
	switch shiftType {
	case insts.ShiftLSL:
		return value << amount
	case insts.ShiftLSR:
		return value >> amount
	case insts.ShiftASR:
		return uint32(int32(value) >> amount)
	case insts.ShiftROR:
		return (value >> amount) | (value << (32 - amount))
	default:
		return value
	}
}

// executeDPReg executes Data Processing Register instructions.
func (e *Emulator) executeDPReg(inst *insts.Instruction) {
	switch inst.Op {
	case insts.OpADD:
		if inst.Is64Bit {
			op1 := e.regFile.ReadReg(inst.Rn)
			op2 := applyShift64(e.regFile.ReadReg(inst.Rm), inst.ShiftType, inst.ShiftAmount)
			result := op1 + op2
			e.regFile.WriteReg(inst.Rd, result)
			if inst.SetFlags {
				e.alu.setAddFlags64(op1, op2, result)
			}
		} else {
			op1 := uint32(e.regFile.ReadReg(inst.Rn))
			op2 := applyShift32(uint32(e.regFile.ReadReg(inst.Rm)), inst.ShiftType, inst.ShiftAmount)
			result := op1 + op2
			e.regFile.WriteReg(inst.Rd, uint64(result))
			if inst.SetFlags {
				e.alu.setAddFlags32(op1, op2, result)
			}
		}
	case insts.OpSUB:
		if inst.Is64Bit {
			op1 := e.regFile.ReadReg(inst.Rn)
			op2 := applyShift64(e.regFile.ReadReg(inst.Rm), inst.ShiftType, inst.ShiftAmount)
			result := op1 - op2
			e.regFile.WriteReg(inst.Rd, result)
			if inst.SetFlags {
				e.alu.setSubFlags64(op1, op2, result)
			}
		} else {
			op1 := uint32(e.regFile.ReadReg(inst.Rn))
			op2 := applyShift32(uint32(e.regFile.ReadReg(inst.Rm)), inst.ShiftType, inst.ShiftAmount)
			result := op1 - op2
			e.regFile.WriteReg(inst.Rd, uint64(result))
			if inst.SetFlags {
				e.alu.setSubFlags32(op1, op2, result)
			}
		}
	case insts.OpAND:
		if inst.Is64Bit {
			op1 := e.regFile.ReadReg(inst.Rn)
			op2 := applyShift64(e.regFile.ReadReg(inst.Rm), inst.ShiftType, inst.ShiftAmount)
			result := op1 & op2
			e.regFile.WriteReg(inst.Rd, result)
			if inst.SetFlags {
				e.alu.setLogicFlags64(result)
			}
		} else {
			op1 := uint32(e.regFile.ReadReg(inst.Rn))
			op2 := applyShift32(uint32(e.regFile.ReadReg(inst.Rm)), inst.ShiftType, inst.ShiftAmount)
			result := op1 & op2
			e.regFile.WriteReg(inst.Rd, uint64(result))
			if inst.SetFlags {
				e.alu.setLogicFlags32(result)
			}
		}
	case insts.OpORR:
		if inst.Is64Bit {
			op1 := e.regFile.ReadReg(inst.Rn)
			op2 := applyShift64(e.regFile.ReadReg(inst.Rm), inst.ShiftType, inst.ShiftAmount)
			e.regFile.WriteReg(inst.Rd, op1|op2)
		} else {
			op1 := uint32(e.regFile.ReadReg(inst.Rn))
			op2 := applyShift32(uint32(e.regFile.ReadReg(inst.Rm)), inst.ShiftType, inst.ShiftAmount)
			e.regFile.WriteReg(inst.Rd, uint64(op1|op2))
		}
	case insts.OpEOR:
		if inst.Is64Bit {
			op1 := e.regFile.ReadReg(inst.Rn)
			op2 := applyShift64(e.regFile.ReadReg(inst.Rm), inst.ShiftType, inst.ShiftAmount)
			e.regFile.WriteReg(inst.Rd, op1^op2)
		} else {
			op1 := uint32(e.regFile.ReadReg(inst.Rn))
			op2 := applyShift32(uint32(e.regFile.ReadReg(inst.Rm)), inst.ShiftType, inst.ShiftAmount)
			e.regFile.WriteReg(inst.Rd, uint64(op1^op2))
		}
	}
}

// executeLogicalImm executes Logical Immediate instructions (AND, ORR, EOR, ANDS).
func (e *Emulator) executeLogicalImm(inst *insts.Instruction) {
	switch inst.Op {
	case insts.OpAND:
		if inst.Is64Bit {
			e.alu.AND64Imm(inst.Rd, inst.Rn, inst.Imm, inst.SetFlags)
		} else {
			e.alu.AND32Imm(inst.Rd, inst.Rn, inst.Imm, inst.SetFlags)
		}
	case insts.OpORR:
		if inst.Is64Bit {
			e.alu.ORR64Imm(inst.Rd, inst.Rn, inst.Imm)
		} else {
			e.alu.ORR32Imm(inst.Rd, inst.Rn, inst.Imm)
		}
	case insts.OpEOR:
		if inst.Is64Bit {
			e.alu.EOR64Imm(inst.Rd, inst.Rn, inst.Imm)
		} else {
			e.alu.EOR32Imm(inst.Rd, inst.Rn, inst.Imm)
		}
	}
}

// executeBitfield executes bitfield instructions (SBFM, BFM, UBFM).
// immr is in inst.Imm, imms is in inst.Imm2
func (e *Emulator) executeBitfield(inst *insts.Instruction) {
	rnVal := e.regFile.ReadReg(inst.Rn)
	immr := uint32(inst.Imm)
	imms := uint32(inst.Imm2)

	var result uint64

	switch inst.Op {
	case insts.OpUBFM:
		// UBFM: Unsigned bitfield move
		// If imms >= immr: extract bits (LSR with imms=regsize-1)
		// If imms < immr: insert bits (LSL)
		if inst.Is64Bit {
			if imms >= immr {
				// LSR, UXTB, UXTH style: extract bits [imms:immr]
				width := imms - immr + 1
				mask := (uint64(1) << width) - 1
				result = (rnVal >> immr) & mask
			} else {
				// LSL style: shift left
				shift := 64 - immr
				width := imms + 1
				mask := (uint64(1) << width) - 1
				result = (rnVal & mask) << shift
			}
		} else {
			rn32 := uint32(rnVal)
			if imms >= immr {
				width := imms - immr + 1
				mask := (uint32(1) << width) - 1
				result = uint64((rn32 >> immr) & mask)
			} else {
				shift := 32 - immr
				width := imms + 1
				mask := (uint32(1) << width) - 1
				result = uint64((rn32 & mask) << shift)
			}
		}
	case insts.OpSBFM:
		// SBFM: Signed bitfield move (ASR, SXTB, SXTH, SXTW, SBFIZ)
		if inst.Is64Bit {
			if imms >= immr {
				// ASR, SXTB, SXTH style: extract and sign-extend
				width := imms - immr + 1
				mask := (uint64(1) << width) - 1
				extracted := (rnVal >> immr) & mask
				// Sign-extend from bit (width-1)
				signBit := uint64(1) << (width - 1)
				if extracted&signBit != 0 {
					extracted |= ^mask // Sign extend
				}
				result = extracted
			} else {
				// SBFIZ: extract bits [imms:0], sign-extend, then shift left
				shift := 64 - immr
				width := imms + 1
				mask := (uint64(1) << width) - 1
				extracted := rnVal & mask
				// Sign-extend from bit (width-1)
				signBit := uint64(1) << (width - 1)
				if extracted&signBit != 0 {
					extracted |= ^mask // Sign extend
				}
				result = extracted << shift
			}
		} else {
			rn32 := uint32(rnVal)
			if imms >= immr {
				width := imms - immr + 1
				mask := (uint32(1) << width) - 1
				extracted := (rn32 >> immr) & mask
				signBit := uint32(1) << (width - 1)
				if extracted&signBit != 0 {
					extracted |= ^mask
				}
				result = uint64(extracted)
			} else {
				// SBFIZ: extract bits [imms:0], sign-extend, then shift left
				shift := 32 - immr
				width := imms + 1
				mask := (uint32(1) << width) - 1
				extracted := rn32 & mask
				// Sign-extend from bit (width-1)
				signBit := uint32(1) << (width - 1)
				if extracted&signBit != 0 {
					extracted |= ^mask
				}
				result = uint64(extracted << shift)
			}
		}
	case insts.OpBFM:
		// BFM: Bitfield move (insert bits into destination)
		rdVal := e.regFile.ReadReg(inst.Rd)
		if inst.Is64Bit {
			if imms >= immr {
				width := imms - immr + 1
				srcMask := (uint64(1) << width) - 1
				bits := (rnVal >> immr) & srcMask
				dstMask := srcMask
				result = (rdVal &^ dstMask) | bits
			} else {
				shift := 64 - immr
				width := imms + 1
				srcMask := (uint64(1) << width) - 1
				bits := (rnVal & srcMask) << shift
				dstMask := srcMask << shift
				result = (rdVal &^ dstMask) | bits
			}
		} else {
			rd32 := uint32(rdVal)
			rn32 := uint32(rnVal)
			if imms >= immr {
				width := imms - immr + 1
				srcMask := (uint32(1) << width) - 1
				bits := (rn32 >> immr) & srcMask
				dstMask := srcMask
				result = uint64((rd32 &^ dstMask) | bits)
			} else {
				shift := 32 - immr
				width := imms + 1
				srcMask := (uint32(1) << width) - 1
				bits := (rn32 & srcMask) << shift
				dstMask := srcMask << shift
				result = uint64((rd32 &^ dstMask) | bits)
			}
		}
	}

	e.regFile.WriteReg(inst.Rd, result)
}

// executeExtract executes the EXTR instruction.
// EXTR Rd, Rn, Rm, #lsb
// Result = (Rm:Rn >> lsb)[datasize-1:0]
// Conceptually concatenates Rm (high) and Rn (low), then extracts datasize bits starting at lsb.
func (e *Emulator) executeExtract(inst *insts.Instruction) {
	rnVal := e.regFile.ReadReg(inst.Rn)
	rmVal := e.regFile.ReadReg(inst.Rm)
	lsb := uint32(inst.Imm)

	var result uint64

	if inst.Is64Bit {
		// 64-bit: concatenate Rm:Rn (128 bits), extract 64 bits at position lsb
		if lsb == 0 {
			result = rnVal
		} else if lsb == 64 {
			result = rmVal
		} else {
			// result = (Rn >> lsb) | (Rm << (64 - lsb))
			result = (rnVal >> lsb) | (rmVal << (64 - lsb))
		}
	} else {
		// 32-bit: concatenate Rm:Rn (64 bits), extract 32 bits at position lsb
		rn32 := uint32(rnVal)
		rm32 := uint32(rmVal)
		if lsb == 0 {
			result = uint64(rn32)
		} else if lsb == 32 {
			result = uint64(rm32)
		} else {
			// result = (Rn >> lsb) | (Rm << (32 - lsb))
			result = uint64((rn32 >> lsb) | (rm32 << (32 - lsb)))
		}
	}

	e.regFile.WriteReg(inst.Rd, result)
}

// executeBranch executes unconditional branch instructions (B, BL).
func (e *Emulator) executeBranch(inst *insts.Instruction) {
	switch inst.Op {
	case insts.OpB:
		e.branchUnit.B(inst.BranchOffset)
	case insts.OpBL:
		e.branchUnit.BL(inst.BranchOffset)
	}
}

// executeBranchCond executes conditional branch instructions.
func (e *Emulator) executeBranchCond(inst *insts.Instruction) {
	// Convert insts.Cond to emu.Cond
	cond := Cond(inst.Cond)

	if e.branchUnit.CheckCondition(cond) {
		e.regFile.PC = uint64(int64(e.regFile.PC) + inst.BranchOffset)
	} else {
		// Condition not met, advance to next instruction
		e.regFile.PC += 4
	}
}

// executeBranchReg executes branch to register instructions (BR, BLR, RET).
func (e *Emulator) executeBranchReg(inst *insts.Instruction) {
	switch inst.Op {
	case insts.OpBR:
		e.branchUnit.BR(inst.Rn)
	case insts.OpBLR:
		e.branchUnit.BLR(inst.Rn)
	case insts.OpRET:
		e.branchUnit.RET(inst.Rn)
	}
}

// executeLoadStore executes load and store instructions.
func (e *Emulator) executeLoadStore(inst *insts.Instruction) {
	// Check if base register is SP (register 31 in load/store context means SP)
	useSP := inst.Rn == 31

	// Calculate address based on indexing mode
	var base uint64
	if useSP {
		base = e.regFile.SP
	} else {
		base = e.regFile.ReadReg(inst.Rn)
	}

	var addr uint64
	switch inst.IndexMode {
	case insts.IndexPre:
		// Pre-index: address = base + offset, then writeback
		addr = uint64(int64(base) + inst.SignedImm)
	case insts.IndexPost:
		// Post-index: address = base, then writeback base + offset
		addr = base
	case insts.IndexRegBase:
		// Register offset: base + (extended Rm << shift)
		rm := e.regFile.ReadReg(inst.Rm)
		var offset uint64
		// Handle extend type (stored in ShiftType)
		// 010=UXTW, 011=LSL/UXTX, 110=SXTW, 111=SXTX
		switch inst.ShiftType {
		case 0b010: // UXTW - zero extend 32-bit
			offset = uint64(uint32(rm))
		case 0b011: // LSL or UXTX - use 64-bit value as-is
			offset = rm
		case 0b110: // SXTW - sign extend 32-bit
			offset = uint64(int64(int32(rm)))
		case 0b111: // SXTX - use 64-bit value as-is (signed)
			offset = rm
		default:
			offset = rm // Fallback
		}
		// Apply scale shift
		offset <<= inst.ShiftAmount
		addr = base + offset
	default:
		// Unsigned offset (no writeback)
		addr = base + inst.Imm
	}

	// Execute the load/store operation
	switch inst.Op {
	case insts.OpLDR:
		if inst.Is64Bit {
			value := e.memory.Read64(addr)
			e.regFile.WriteReg(inst.Rd, value)
		} else {
			value := e.memory.Read32(addr)
			e.regFile.WriteReg(inst.Rd, uint64(value))
		}
	case insts.OpSTR:
		if inst.Is64Bit {
			value := e.regFile.ReadReg(inst.Rd)
			e.memory.Write64(addr, value)
		} else {
			value := uint32(e.regFile.ReadReg(inst.Rd))
			e.memory.Write32(addr, value)
		}
	case insts.OpLDRB:
		e.lsu.LDRB(inst.Rd, addr)
	case insts.OpSTRB:
		e.lsu.STRB(inst.Rd, addr)
	case insts.OpLDRSB:
		if inst.Is64Bit {
			e.lsu.LDRSB64(inst.Rd, addr)
		} else {
			e.lsu.LDRSB32(inst.Rd, addr)
		}
	case insts.OpLDRH:
		e.lsu.LDRH(inst.Rd, addr)
	case insts.OpSTRH:
		e.lsu.STRH(inst.Rd, addr)
	case insts.OpLDRSH:
		if inst.Is64Bit {
			e.lsu.LDRSH64(inst.Rd, addr)
		} else {
			e.lsu.LDRSH32(inst.Rd, addr)
		}
	case insts.OpLDRSW:
		// LDRSW: Load 32-bit word and sign-extend to 64-bit
		e.lsu.LDRSW(inst.Rd, addr)
	}

	// Handle writeback for pre/post-indexed modes
	if inst.IndexMode == insts.IndexPre || inst.IndexMode == insts.IndexPost {
		newBase := uint64(int64(base) + inst.SignedImm)
		if useSP {
			e.regFile.SP = newBase
		} else {
			e.regFile.WriteReg(inst.Rn, newBase)
		}
	}
}

// executeLoadStorePair executes LDP and STP instructions.
func (e *Emulator) executeLoadStorePair(inst *insts.Instruction) {
	// Check if base register is SP
	useSP := inst.Rn == 31

	var base uint64
	if useSP {
		base = e.regFile.SP
	} else {
		base = e.regFile.ReadReg(inst.Rn)
	}

	// Calculate address based on indexing mode
	var addr uint64
	switch inst.IndexMode {
	case insts.IndexPre:
		// Pre-index: address = base + offset, then writeback
		addr = uint64(int64(base) + inst.SignedImm)
	case insts.IndexPost:
		// Post-index: address = base, then writeback base + offset
		addr = base
	default:
		// Signed offset (no writeback)
		addr = uint64(int64(base) + inst.SignedImm)
	}

	// Determine element size
	var elemSize uint64 = 4 // 32-bit
	if inst.Is64Bit {
		elemSize = 8 // 64-bit
	}

	switch inst.Op {
	case insts.OpLDP:
		// Load pair
		if inst.Is64Bit {
			val1 := e.memory.Read64(addr)
			val2 := e.memory.Read64(addr + elemSize)
			e.regFile.WriteReg(inst.Rd, val1)
			e.regFile.WriteReg(inst.Rt2, val2)
		} else {
			val1 := e.memory.Read32(addr)
			val2 := e.memory.Read32(addr + elemSize)
			e.regFile.WriteReg(inst.Rd, uint64(val1))
			e.regFile.WriteReg(inst.Rt2, uint64(val2))
		}
	case insts.OpSTP:
		// Store pair
		if inst.Is64Bit {
			val1 := e.regFile.ReadReg(inst.Rd)
			val2 := e.regFile.ReadReg(inst.Rt2)
			e.memory.Write64(addr, val1)
			e.memory.Write64(addr+elemSize, val2)
		} else {
			val1 := uint32(e.regFile.ReadReg(inst.Rd))
			val2 := uint32(e.regFile.ReadReg(inst.Rt2))
			e.memory.Write32(addr, val1)
			e.memory.Write32(addr+elemSize, val2)
		}
	}

	// Handle writeback for pre/post-indexed modes
	if inst.IndexMode == insts.IndexPre || inst.IndexMode == insts.IndexPost {
		newBase := uint64(int64(base) + inst.SignedImm)
		if useSP {
			e.regFile.SP = newBase
		} else {
			e.regFile.WriteReg(inst.Rn, newBase)
		}
	}
}

// executePCRel executes PC-relative addressing instructions (ADR, ADRP).
func (e *Emulator) executePCRel(inst *insts.Instruction) {
	pc := e.regFile.PC

	switch inst.Op {
	case insts.OpADR:
		// ADR: Rd = PC + offset
		result := uint64(int64(pc) + inst.BranchOffset)
		e.regFile.WriteReg(inst.Rd, result)
	case insts.OpADRP:
		// ADRP: Rd = (PC & ~0xFFF) + (offset << 12)
		// Note: BranchOffset is already shifted by 12 in the decoder
		pageBase := pc & ^uint64(0xFFF)
		result := uint64(int64(pageBase) + inst.BranchOffset)
		e.regFile.WriteReg(inst.Rd, result)
	}
}

// executeLoadStoreLit executes PC-relative load literal instructions.
func (e *Emulator) executeLoadStoreLit(inst *insts.Instruction) {
	// Calculate target address: PC + offset
	addr := uint64(int64(e.regFile.PC) + inst.BranchOffset)

	switch inst.Op {
	case insts.OpLDRLit:
		if inst.Is64Bit {
			// Load 64-bit value
			val := e.memory.Read64(addr)
			e.regFile.WriteReg(inst.Rd, val)
		} else {
			// Load 32-bit value (zero-extended)
			val := uint64(e.memory.Read32(addr))
			e.regFile.WriteReg(inst.Rd, val)
		}
	}
}

// executeMoveWide executes move wide immediate instructions (MOVZ, MOVN, MOVK).
func (e *Emulator) executeMoveWide(inst *insts.Instruction) {
	imm := inst.Imm
	shift := uint64(inst.Shift)

	switch inst.Op {
	case insts.OpMOVZ:
		// MOVZ: Rd = imm16 << shift, zero other bits
		result := imm << shift
		e.regFile.WriteReg(inst.Rd, result)
	case insts.OpMOVN:
		// MOVN: Rd = NOT(imm16 << shift)
		result := ^(imm << shift)
		if !inst.Is64Bit {
			// Mask to 32 bits for W registers
			result &= 0xFFFFFFFF
		}
		e.regFile.WriteReg(inst.Rd, result)
	case insts.OpMOVK:
		// MOVK: Rd = (Rd & ~(0xFFFF << shift)) | (imm16 << shift)
		// Keep other bits, replace 16 bits at shift position
		current := e.regFile.ReadReg(inst.Rd)
		mask := ^(uint64(0xFFFF) << shift)
		result := (current & mask) | (imm << shift)
		e.regFile.WriteReg(inst.Rd, result)
	}
}

// executeCondSelect executes conditional select instructions (CSEL, CSINC, CSINV, CSNEG).
func (e *Emulator) executeCondSelect(inst *insts.Instruction) {
	// Convert insts.Cond to emu.Cond
	cond := Cond(inst.Cond)

	// Read source registers
	rnVal := e.regFile.ReadReg(inst.Rn)
	rmVal := e.regFile.ReadReg(inst.Rm)

	// Mask to 32 bits for W registers
	if !inst.Is64Bit {
		rnVal &= 0xFFFFFFFF
		rmVal &= 0xFFFFFFFF
	}

	var result uint64
	if e.branchUnit.CheckCondition(cond) {
		// Condition true: select Rn
		result = rnVal
	} else {
		// Condition false: apply operation to Rm
		switch inst.Op {
		case insts.OpCSEL:
			result = rmVal
		case insts.OpCSINC:
			result = rmVal + 1
		case insts.OpCSINV:
			result = ^rmVal
		case insts.OpCSNEG:
			result = -rmVal
		}
	}

	// Mask result for 32-bit operations
	if !inst.Is64Bit {
		result &= 0xFFFFFFFF
	}

	e.regFile.WriteReg(inst.Rd, result)
}

// executeCondCmp executes conditional compare instructions (CCMP, CCMN).
// If condition is true: compare Rn with Rm/imm and set flags
// If condition is false: set flags to nzcv value
func (e *Emulator) executeCondCmp(inst *insts.Instruction) {
	cond := Cond(inst.Cond)

	if e.branchUnit.CheckCondition(cond) {
		// Condition true: perform the comparison and set flags
		rnVal := e.regFile.ReadReg(inst.Rn)

		var operand uint64
		if inst.Rm == 0xFF {
			// Immediate form
			operand = inst.Imm2
		} else {
			// Register form
			operand = e.regFile.ReadReg(inst.Rm)
		}

		if inst.Is64Bit {
			if inst.Op == insts.OpCCMP {
				// CCMP: compare Rn - operand (sets flags like SUBS)
				result := rnVal - operand
				e.regFile.PSTATE.N = (result >> 63) == 1
				e.regFile.PSTATE.Z = result == 0
				e.regFile.PSTATE.C = rnVal >= operand
				e.regFile.PSTATE.V = ((rnVal^operand)&(rnVal^result))>>63 == 1
			} else {
				// CCMN: compare Rn + operand (sets flags like ADDS)
				result := rnVal + operand
				e.regFile.PSTATE.N = (result >> 63) == 1
				e.regFile.PSTATE.Z = result == 0
				e.regFile.PSTATE.C = result < rnVal // Overflow in unsigned add
				e.regFile.PSTATE.V = ((^(rnVal ^ operand)) & (rnVal ^ result) >> 63) == 1
			}
		} else {
			rn32 := uint32(rnVal)
			op32 := uint32(operand)
			if inst.Op == insts.OpCCMP {
				result := rn32 - op32
				e.regFile.PSTATE.N = (result >> 31) == 1
				e.regFile.PSTATE.Z = result == 0
				e.regFile.PSTATE.C = rn32 >= op32
				e.regFile.PSTATE.V = ((rn32^op32)&(rn32^result))>>31 == 1
			} else {
				result := rn32 + op32
				e.regFile.PSTATE.N = (result >> 31) == 1
				e.regFile.PSTATE.Z = result == 0
				e.regFile.PSTATE.C = result < rn32
				e.regFile.PSTATE.V = ((^(rn32 ^ op32)) & (rn32 ^ result) >> 31) == 1
			}
		}
	} else {
		// Condition false: set flags to nzcv value
		nzcv := inst.Imm
		e.regFile.PSTATE.N = (nzcv>>3)&1 == 1
		e.regFile.PSTATE.Z = (nzcv>>2)&1 == 1
		e.regFile.PSTATE.C = (nzcv>>1)&1 == 1
		e.regFile.PSTATE.V = nzcv&1 == 1
	}
}

// executeDataProc2Src executes two-source data processing instructions (UDIV, SDIV).
func (e *Emulator) executeDataProc2Src(inst *insts.Instruction) {
	rnVal := e.regFile.ReadReg(inst.Rn)
	rmVal := e.regFile.ReadReg(inst.Rm)

	var result uint64

	switch inst.Op {
	case insts.OpUDIV:
		if inst.Is64Bit {
			if rmVal == 0 {
				result = 0 // Division by zero returns 0
			} else {
				result = rnVal / rmVal
			}
		} else {
			rn32 := uint32(rnVal)
			rm32 := uint32(rmVal)
			if rm32 == 0 {
				result = 0
			} else {
				result = uint64(rn32 / rm32)
			}
		}
	case insts.OpSDIV:
		if inst.Is64Bit {
			if rmVal == 0 {
				result = 0
			} else {
				result = uint64(int64(rnVal) / int64(rmVal))
			}
		} else {
			rn32 := int32(rnVal)
			rm32 := int32(rmVal)
			if rm32 == 0 {
				result = 0
			} else {
				result = uint64(uint32(rn32 / rm32))
			}
		}
	case insts.OpLSLV:
		// Logical shift left by register
		if inst.Is64Bit {
			shift := rmVal & 0x3F // Shift amount mod 64
			result = rnVal << shift
		} else {
			shift := uint32(rmVal) & 0x1F // Shift amount mod 32
			result = uint64(uint32(rnVal) << shift)
		}
	case insts.OpLSRV:
		// Logical shift right by register
		if inst.Is64Bit {
			shift := rmVal & 0x3F // Shift amount mod 64
			result = rnVal >> shift
		} else {
			shift := uint32(rmVal) & 0x1F // Shift amount mod 32
			result = uint64(uint32(rnVal) >> shift)
		}
	case insts.OpASRV:
		// Arithmetic shift right by register
		if inst.Is64Bit {
			shift := rmVal & 0x3F // Shift amount mod 64
			result = uint64(int64(rnVal) >> shift)
		} else {
			shift := uint32(rmVal) & 0x1F // Shift amount mod 32
			result = uint64(uint32(int32(rnVal) >> shift))
		}
	case insts.OpRORV:
		// Rotate right by register
		if inst.Is64Bit {
			shift := rmVal & 0x3F // Shift amount mod 64
			result = (rnVal >> shift) | (rnVal << (64 - shift))
		} else {
			rn32 := uint32(rnVal)
			shift := uint32(rmVal) & 0x1F // Shift amount mod 32
			result = uint64((rn32 >> shift) | (rn32 << (32 - shift)))
		}
	}

	e.regFile.WriteReg(inst.Rd, result)
}

// executeDataProc3Src executes three-source data processing instructions (MADD, MSUB).
func (e *Emulator) executeDataProc3Src(inst *insts.Instruction) {
	rnVal := e.regFile.ReadReg(inst.Rn)
	rmVal := e.regFile.ReadReg(inst.Rm)
	raVal := e.regFile.ReadReg(inst.Rt2) // Ra is stored in Rt2 field

	var result uint64

	switch inst.Op {
	case insts.OpMADD:
		// MADD: Rd = Ra + (Rn * Rm)
		if inst.Is64Bit {
			result = raVal + rnVal*rmVal
		} else {
			result = uint64(uint32(raVal) + uint32(rnVal)*uint32(rmVal))
		}
	case insts.OpMSUB:
		// MSUB: Rd = Ra - (Rn * Rm)
		if inst.Is64Bit {
			result = raVal - rnVal*rmVal
		} else {
			result = uint64(uint32(raVal) - uint32(rnVal)*uint32(rmVal))
		}
	}

	e.regFile.WriteReg(inst.Rd, result)
}

// executeTestBranch executes test and branch instructions (TBZ, TBNZ).
func (e *Emulator) executeTestBranch(inst *insts.Instruction) {
	// Read the register to test
	rtVal := e.regFile.ReadReg(inst.Rd)

	// Get the bit number from Imm
	bitNum := uint(inst.Imm)

	// Test the specified bit
	bitValue := (rtVal >> bitNum) & 1

	var takeBranch bool
	switch inst.Op {
	case insts.OpTBZ:
		// TBZ: branch if bit is zero
		takeBranch = bitValue == 0
	case insts.OpTBNZ:
		// TBNZ: branch if bit is not zero
		takeBranch = bitValue != 0
	}

	if takeBranch {
		e.regFile.PC = uint64(int64(e.regFile.PC) + inst.BranchOffset)
	} else {
		e.regFile.PC += 4
	}
}

// executeCompareBranch executes compare and branch instructions (CBZ, CBNZ).
func (e *Emulator) executeCompareBranch(inst *insts.Instruction) {
	// Read the register to compare
	rtVal := e.regFile.ReadReg(inst.Rd)

	// Mask to 32 bits for W registers
	if !inst.Is64Bit {
		rtVal &= 0xFFFFFFFF
	}

	var takeBranch bool
	switch inst.Op {
	case insts.OpCBZ:
		// CBZ: branch if register is zero
		takeBranch = rtVal == 0
	case insts.OpCBNZ:
		// CBNZ: branch if register is not zero
		takeBranch = rtVal != 0
	}

	if takeBranch {
		e.regFile.PC = uint64(int64(e.regFile.PC) + inst.BranchOffset)
	} else {
		e.regFile.PC += 4
	}
}

// executeSIMDReg executes SIMD data processing (three-same) instructions.
func (e *Emulator) executeSIMDReg(inst *insts.Instruction) {
	arr := SIMDArrangement(inst.Arrangement)

	switch inst.Op {
	case insts.OpVADD:
		e.simdUnit.VADD(inst.Rd, inst.Rn, inst.Rm, arr)
	case insts.OpVSUB:
		e.simdUnit.VSUB(inst.Rd, inst.Rn, inst.Rm, arr)
	case insts.OpVMUL:
		e.simdUnit.VMUL(inst.Rd, inst.Rn, inst.Rm, arr)
	case insts.OpVFADD:
		e.simdUnit.VFADD(inst.Rd, inst.Rn, inst.Rm, arr)
	case insts.OpVFSUB:
		e.simdUnit.VFSUB(inst.Rd, inst.Rn, inst.Rm, arr)
	case insts.OpVFMUL:
		e.simdUnit.VFMUL(inst.Rd, inst.Rn, inst.Rm, arr)
	}
}

// executeSIMDLoadStore executes SIMD load/store instructions.
func (e *Emulator) executeSIMDLoadStore(inst *insts.Instruction) {
	// Calculate address: base + unsigned offset
	useSP := inst.Rn == 31
	var base uint64
	if useSP {
		base = e.regFile.SP
	} else {
		base = e.regFile.ReadReg(inst.Rn)
	}
	addr := base + inst.Imm

	switch inst.Op {
	case insts.OpLDRQ:
		e.simdUnit.LDR128(inst.Rd, addr)
	case insts.OpSTRQ:
		e.simdUnit.STR128(inst.Rd, addr)
	}
}
