package pipeline

import "fmt"

// isUnconditionalBranch checks if an instruction word is an unconditional branch (B or BL).
// Returns true and the target PC if it is, false otherwise.
func isUnconditionalBranch(word uint32, pc uint64) (bool, uint64) {
	// B instruction: bits [31:26] = 000101
	// BL instruction: bits [31:26] = 100101
	opcode := (word >> 26) & 0x3F
	if opcode == 0b000101 || opcode == 0b100101 {
		// Extract signed 26-bit immediate (offset in words)
		imm26 := int64(word & 0x3FFFFFF)
		// Sign extend the 26-bit immediate
		if (imm26 & 0x2000000) != 0 {
			// Negative offset: sign extend from bit 25
			imm26 |= ^int64(0x3FFFFFF)
		}
		// Multiply by 4 to get byte offset
		target := uint64(int64(pc) + imm26*4)
		return true, target
	}
	return false, 0
}

// isEliminableBranch checks if an instruction word is an unconditional B (not BL).
// Returns true if the branch can be eliminated (doesn't write to a register).
// According to Dougall Johnson's Firestorm documentation, unconditional B
// instructions never issue to execution units on Apple M2.
func isEliminableBranch(word uint32) bool {
	// B instruction: bits [31:26] = 000101 (bit 31 = 0)
	// BL instruction: bits [31:26] = 100101 (bit 31 = 1)
	// Only pure B can be eliminated; BL writes to X30 (link register)
	opcode := (word >> 26) & 0x3F
	return opcode == 0b000101 // Only pure B, not BL
}

// isConditionalBranch checks if an instruction word is a conditional branch (B.cond).
// Returns true and the target PC if it is, false otherwise.
func isConditionalBranch(word uint32, pc uint64) (bool, uint64) {
	// B.cond instruction: bits [31:25] = 0101010, bit 24 = 0, bits [4] = 0
	// Encoding: 01010100 imm19 0 cond
	if (word >> 24) != 0x54 {
		return false, 0
	}
	// Check bit 4 must be 0
	if (word & 0x10) != 0 {
		return false, 0
	}
	// Extract signed 19-bit immediate (offset in words)
	imm19 := int64((word >> 5) & 0x7FFFF)
	// Sign extend the 19-bit immediate
	if (imm19 & 0x40000) != 0 {
		imm19 |= ^int64(0x7FFFF)
	}
	// Multiply by 4 to get byte offset
	target := uint64(int64(pc) + imm19*4)
	return true, target
}

// isCompareAndBranch checks if an instruction word is CBZ or CBNZ.
// Returns true and the target PC if it is, false otherwise.
func isCompareAndBranch(word uint32, pc uint64) (bool, uint64) {
	// CBZ: sf 011010 0 imm19 Rt
	// CBNZ: sf 011010 1 imm19 Rt
	// bits [30:25] = 011010
	if ((word >> 25) & 0x3F) != 0b011010 {
		return false, 0
	}
	// Extract signed 19-bit immediate
	imm19 := int64((word >> 5) & 0x7FFFF)
	// Sign extend
	if (imm19 & 0x40000) != 0 {
		imm19 |= ^int64(0x7FFFF)
	}
	target := uint64(int64(pc) + imm19*4)
	return true, target
}

// isTestAndBranch checks if an instruction word is TBZ or TBNZ.
// Returns true and the target PC if it is, false otherwise.
func isTestAndBranch(word uint32, pc uint64) (bool, uint64) {
	// TBZ: b5 011011 0 b40 imm14 Rt
	// TBNZ: b5 011011 1 b40 imm14 Rt
	// bits [30:25] = 011011
	if ((word >> 25) & 0x3F) != 0b011011 {
		return false, 0
	}
	// Extract signed 14-bit immediate
	imm14 := int64((word >> 5) & 0x3FFF)
	// Sign extend
	if (imm14 & 0x2000) != 0 {
		imm14 |= ^int64(0x3FFF)
	}
	target := uint64(int64(pc) + imm14*4)
	return true, target
}

// isFoldableConditionalBranch checks if an instruction can be folded (eliminated)
// at fetch time. Returns true with the target if the branch can be folded.
// A branch is foldable if it's a conditional branch type (B.cond, CBZ, CBNZ, TBZ, TBNZ).
// The actual folding decision also requires BTB hit and high-confidence prediction.
func isFoldableConditionalBranch(word uint32, pc uint64) (bool, uint64) {
	// Check B.cond
	if isCond, target := isConditionalBranch(word, pc); isCond {
		return true, target
	}
	// Check CBZ/CBNZ
	if isCB, target := isCompareAndBranch(word, pc); isCB {
		return true, target
	}
	// Check TBZ/TBNZ
	if isTB, target := isTestAndBranch(word, pc); isTB {
		return true, target
	}
	return false, 0
}

// enrichPredictionWithEncodedTarget fills in the branch target for conditional
// branches when the predictor says "taken" but BTB doesn't have the target.
// Real hardware (including M2) can extract the target from the instruction
// encoding during fetch, avoiding a full misprediction penalty for taken
// conditional branches with BTB misses.
func enrichPredictionWithEncodedTarget(pred *Prediction, word uint32, pc uint64) {
	if pred.Taken && !pred.TargetKnown {
		if isCond, target := isFoldableConditionalBranch(word, pc); isCond {
			pred.Target = target
			pred.TargetKnown = true
		}
	}
}

// accessSecondaryMem processes a memory operation for secondary slot (slot 2).
// Returns the memory result and whether a stall occurred.
func (p *Pipeline) accessSecondaryMem(slot MemorySlot) (MemoryResult, bool) {
	if !slot.IsValid() || (!slot.GetMemRead() && !slot.GetMemWrite()) {
		p.memPending2 = false
		return MemoryResult{}, false
	}
	if p.useDCache && p.cachedMemoryStage2 != nil {
		result, stall := p.cachedMemoryStage2.AccessSlot(slot)
		return result, stall
	}
	// Non-cached path: immediate access (no stall).
	// Without cache simulation, memory is a direct array lookup.
	// Pipeline issue rules already enforce ordering constraints.
	p.memPending2 = false
	return p.memoryStage.MemorySlot(slot), false
}

// accessTertiaryMem processes a memory operation for tertiary slot (slot 3).
func (p *Pipeline) accessTertiaryMem(slot MemorySlot) (MemoryResult, bool) {
	if !slot.IsValid() || (!slot.GetMemRead() && !slot.GetMemWrite()) {
		p.memPending3 = false
		return MemoryResult{}, false
	}
	if p.useDCache && p.cachedMemoryStage3 != nil {
		result, stall := p.cachedMemoryStage3.AccessSlot(slot)
		return result, stall
	}
	// Non-cached path: immediate access (no stall).
	// Without cache simulation, memory is a direct array lookup.
	// Pipeline issue rules already enforce ordering constraints.
	p.memPending3 = false
	return p.memoryStage.MemorySlot(slot), false
}

// accessQuaternaryMem processes a memory operation for quaternary slot (slot 4).
func (p *Pipeline) accessQuaternaryMem(slot MemorySlot) (MemoryResult, bool) {
	if !slot.IsValid() || (!slot.GetMemRead() && !slot.GetMemWrite()) {
		p.memPending4 = false
		return MemoryResult{}, false
	}
	if p.useDCache && p.cachedMemoryStage4 != nil {
		result, stall := p.cachedMemoryStage4.AccessSlot(slot)
		return result, stall
	}
	// Non-cached path: immediate access (no stall).
	// Without cache simulation, memory is a direct array lookup.
	// Pipeline issue rules already enforce ordering constraints.
	p.memPending4 = false
	return p.memoryStage.MemorySlot(slot), false
}

// accessQuinaryMem processes a memory operation for quinary slot (slot 5).
func (p *Pipeline) accessQuinaryMem(slot MemorySlot) (MemoryResult, bool) {
	if !slot.IsValid() || (!slot.GetMemRead() && !slot.GetMemWrite()) {
		p.memPending5 = false
		return MemoryResult{}, false
	}
	if p.useDCache && p.cachedMemoryStage5 != nil {
		result, stall := p.cachedMemoryStage5.AccessSlot(slot)
		return result, stall
	}
	// Non-cached path: immediate access (no stall).
	// Without cache simulation, memory is a direct array lookup.
	// Pipeline issue rules already enforce ordering constraints.
	p.memPending5 = false
	return p.memoryStage.MemorySlot(slot), false
}

// pendingFetchInst represents an instruction waiting in fetch buffer.
//
//nolint:unused // Scaffolding for 4-wide implementation (PR #114)
type pendingFetchInst struct {
	PC   uint64
	Word uint32
}

// collectPendingFetchInstructions returns unissued instructions that need to remain in fetch.
// issueCount is how many instructions were issued from the current IF/ID registers.
// Uses a fixed-size array to avoid heap allocation per tick.
//
//nolint:unused // Scaffolding for 4-wide implementation (PR #114)
func (p *Pipeline) collectPendingFetchInstructions(issueCount int) ([8]pendingFetchInst, int) {
	var allFetched [8]pendingFetchInst
	count := 0

	if p.ifid.Valid {
		allFetched[count] = pendingFetchInst{PC: p.ifid.PC, Word: p.ifid.InstructionWord}
		count++
	}
	if p.ifid2.Valid {
		allFetched[count] = pendingFetchInst{PC: p.ifid2.PC, Word: p.ifid2.InstructionWord}
		count++
	}
	if p.ifid3.Valid {
		allFetched[count] = pendingFetchInst{PC: p.ifid3.PC, Word: p.ifid3.InstructionWord}
		count++
	}
	if p.ifid4.Valid {
		allFetched[count] = pendingFetchInst{PC: p.ifid4.PC, Word: p.ifid4.InstructionWord}
		count++
	}

	// Skip the first issueCount instructions (they were issued).
	// Shift remaining down to index 0.
	pendingCount := 0
	if issueCount < count {
		pendingCount = count - issueCount
		for i := 0; i < pendingCount; i++ {
			allFetched[i] = allFetched[i+issueCount]
		}
	}

	return allFetched, pendingCount
}

// sameCycleForward applies same-cycle register forwarding from an earlier
// execute result. If the source produced a value for register rn or rm,
// the forwarded value is returned.
func sameCycleForward(
	valid, regWrite bool, rd uint8, aluResult uint64,
	rn, rm uint8, rnValue, rmValue uint64,
) (uint64, uint64) {
	if valid && regWrite && rd != 31 {
		if rn == rd {
			rnValue = aluResult
		}
		if rm == rd {
			rmValue = aluResult
		}
	}
	return rnValue, rmValue
}

// forwardPSTATEFromPrevCycleEXMEM checks all 8 previous-cycle EXMEM stages
// for PSTATE flag forwarding to a B.cond instruction.
func (p *Pipeline) forwardPSTATEFromPrevCycleEXMEM() (bool, bool, bool, bool, bool) {
	type flagSource struct {
		valid      bool
		setsFlags  bool
		n, z, c, v bool
	}
	sources := [8]flagSource{
		{p.exmem.Valid, p.exmem.SetsFlags, p.exmem.FlagN, p.exmem.FlagZ, p.exmem.FlagC, p.exmem.FlagV},
		{p.exmem2.Valid, p.exmem2.SetsFlags, p.exmem2.FlagN, p.exmem2.FlagZ, p.exmem2.FlagC, p.exmem2.FlagV},
		{p.exmem3.Valid, p.exmem3.SetsFlags, p.exmem3.FlagN, p.exmem3.FlagZ, p.exmem3.FlagC, p.exmem3.FlagV},
		{p.exmem4.Valid, p.exmem4.SetsFlags, p.exmem4.FlagN, p.exmem4.FlagZ, p.exmem4.FlagC, p.exmem4.FlagV},
		{p.exmem5.Valid, p.exmem5.SetsFlags, p.exmem5.FlagN, p.exmem5.FlagZ, p.exmem5.FlagC, p.exmem5.FlagV},
		{p.exmem6.Valid, p.exmem6.SetsFlags, p.exmem6.FlagN, p.exmem6.FlagZ, p.exmem6.FlagC, p.exmem6.FlagV},
		{p.exmem7.Valid, p.exmem7.SetsFlags, p.exmem7.FlagN, p.exmem7.FlagZ, p.exmem7.FlagC, p.exmem7.FlagV},
		{p.exmem8.Valid, p.exmem8.SetsFlags, p.exmem8.FlagN, p.exmem8.FlagZ, p.exmem8.FlagC, p.exmem8.FlagV},
	}
	for _, s := range sources {
		if s.valid && s.setsFlags {
			return true, s.n, s.z, s.c, s.v
		}
	}
	return false, false, false, false, false
}

// forwardFromAllSlots checks all secondary pipeline stages for forwarding.
//
//nolint:unused // Scaffolding for 4-wide implementation (PR #114)
func (p *Pipeline) forwardFromAllSlots(reg uint8, currentValue uint64) uint64 {
	if reg == 31 {
		return currentValue
	}

	// Check memwb stages (oldest first, primary slot first)
	if p.memwb.Valid && p.memwb.RegWrite && p.memwb.Rd == reg {
		if p.memwb.MemToReg {
			currentValue = p.memwb.MemData
		} else {
			currentValue = p.memwb.ALUResult
		}
	}
	if p.memwb2.Valid && p.memwb2.RegWrite && p.memwb2.Rd == reg {
		currentValue = p.memwb2.ALUResult
	}
	if p.memwb3.Valid && p.memwb3.RegWrite && p.memwb3.Rd == reg {
		currentValue = p.memwb3.ALUResult
	}
	if p.memwb4.Valid && p.memwb4.RegWrite && p.memwb4.Rd == reg {
		currentValue = p.memwb4.ALUResult
	}
	if p.memwb5.Valid && p.memwb5.RegWrite && p.memwb5.Rd == reg {
		currentValue = p.memwb5.ALUResult
	}
	if p.memwb6.Valid && p.memwb6.RegWrite && p.memwb6.Rd == reg {
		currentValue = p.memwb6.ALUResult
	}
	if p.memwb7.Valid && p.memwb7.RegWrite && p.memwb7.Rd == reg {
		currentValue = p.memwb7.ALUResult
	}
	if p.memwb8.Valid && p.memwb8.RegWrite && p.memwb8.Rd == reg {
		currentValue = p.memwb8.ALUResult
	}

	// Check exmem stages (newer, higher priority, primary slot first)
	if p.exmem.Valid && p.exmem.RegWrite && p.exmem.Rd == reg {
		currentValue = p.exmem.ALUResult
	}
	if p.exmem2.Valid && p.exmem2.RegWrite && p.exmem2.Rd == reg {
		currentValue = p.exmem2.ALUResult
	}
	if p.exmem3.Valid && p.exmem3.RegWrite && p.exmem3.Rd == reg {
		currentValue = p.exmem3.ALUResult
	}
	if p.exmem4.Valid && p.exmem4.RegWrite && p.exmem4.Rd == reg {
		currentValue = p.exmem4.ALUResult
	}
	if p.exmem5.Valid && p.exmem5.RegWrite && p.exmem5.Rd == reg {
		currentValue = p.exmem5.ALUResult
	}
	if p.exmem6.Valid && p.exmem6.RegWrite && p.exmem6.Rd == reg {
		currentValue = p.exmem6.ALUResult
	}
	if p.exmem7.Valid && p.exmem7.RegWrite && p.exmem7.Rd == reg {
		currentValue = p.exmem7.ALUResult
	}
	if p.exmem8.Valid && p.exmem8.RegWrite && p.exmem8.Rd == reg {
		currentValue = p.exmem8.ALUResult
	}

	return currentValue
}

// flushAllIFID clears all IF/ID pipeline registers.
//
//nolint:unused // Scaffolding for 4-wide implementation (PR #114)
func (p *Pipeline) flushAllIFID() {
	p.ifid.Clear()
	p.ifid2.Clear()
	p.ifid3.Clear()
	p.ifid4.Clear()
	p.ifid5.Clear()
	p.ifid6.Clear()
	p.ifid7.Clear()
	p.ifid8.Clear()
	p.instrWindowLen = 0 // flush instruction window on misprediction
}

// flushAllIDEX clears all ID/EX pipeline registers.
//
//nolint:unused // Scaffolding for 4-wide implementation (PR #114)
func (p *Pipeline) flushAllIDEX() {
	p.idex.Clear()
	p.idex2.Clear()
	p.idex3.Clear()
	p.idex4.Clear()
	p.idex5.Clear()
	p.idex6.Clear()
	p.idex7.Clear()
	p.idex8.Clear()
}

// collectPendingFetchInstructionsSelective returns unissued IFID instructions,
// using a per-slot consumed flag. Supports OoO-style dispatch where
// non-contiguous slots may be consumed. The consumed slice length determines
// how many IFID slots to consider (4, 6, or 8).
func (p *Pipeline) collectPendingFetchInstructionsSelective(consumed []bool) ([8]pendingFetchInst, int) {
	var result [8]pendingFetchInst
	count := 0

	type ifidSlot struct {
		valid bool
		pc    uint64
		word  uint32
	}
	slots := [8]ifidSlot{
		{p.ifid.Valid, p.ifid.PC, p.ifid.InstructionWord},
		{p.ifid2.Valid, p.ifid2.PC, p.ifid2.InstructionWord},
		{p.ifid3.Valid, p.ifid3.PC, p.ifid3.InstructionWord},
		{p.ifid4.Valid, p.ifid4.PC, p.ifid4.InstructionWord},
		{p.ifid5.Valid, p.ifid5.PC, p.ifid5.InstructionWord},
		{p.ifid6.Valid, p.ifid6.PC, p.ifid6.InstructionWord},
		{p.ifid7.Valid, p.ifid7.PC, p.ifid7.InstructionWord},
		{p.ifid8.Valid, p.ifid8.PC, p.ifid8.InstructionWord},
	}

	n := len(consumed)
	for i := 0; i < n; i++ {
		if slots[i].valid && !consumed[i] {
			result[count] = pendingFetchInst{PC: slots[i].pc, Word: slots[i].word}
			count++
		}
	}

	return result, count
}

// clearAndRemarkAfterBranch clears AfterBranch flags from IFID registers and
// the instruction window when a branch resolves correctly. This allows stores
// that were blocked behind the resolved branch to issue.
//
// All flags are cleared unconditionally because every instruction in the
// pipeline after a correctly-resolved branch is on the correct execution
// path. If a subsequent branch mispredicts, the pipeline flush discards all
// younger instructions and the register checkpoint restores processor state.
// Re-fetched instructions get fresh AfterBranch flags from the fetch stage.
// Memory writes from correct-path stores between two correctly-predicted
// branches are valid and don't need rollback.
func (p *Pipeline) clearAndRemarkAfterBranch() {
	for i := 0; i < p.instrWindowLen; i++ {
		p.instrWindow[i].AfterBranch = false
	}
	p.ifid.AfterBranch = false
	p.ifid2.AfterBranch = false
	p.ifid3.AfterBranch = false
	p.ifid4.AfterBranch = false
	p.ifid5.AfterBranch = false
	p.ifid6.AfterBranch = false
	p.ifid7.AfterBranch = false
	p.ifid8.AfterBranch = false
}

// pushUnconsumedToWindow pushes un-consumed IFID instructions into the
// instruction window buffer. Instructions that couldn't issue this cycle
// are preserved for potential issue in future cycles.
func (p *Pipeline) pushUnconsumedToWindow(consumed []bool) {
	type ifidSlot struct {
		valid           bool
		pc              uint64
		word            uint32
		predictedTaken  bool
		predictedTarget uint64
		earlyResolved   bool
		afterBranch     bool
	}
	slots := [8]ifidSlot{
		{p.ifid.Valid, p.ifid.PC, p.ifid.InstructionWord, p.ifid.PredictedTaken, p.ifid.PredictedTarget, p.ifid.EarlyResolved, p.ifid.AfterBranch},
		{p.ifid2.Valid, p.ifid2.PC, p.ifid2.InstructionWord, p.ifid2.PredictedTaken, p.ifid2.PredictedTarget, p.ifid2.EarlyResolved, p.ifid2.AfterBranch},
		{p.ifid3.Valid, p.ifid3.PC, p.ifid3.InstructionWord, p.ifid3.PredictedTaken, p.ifid3.PredictedTarget, p.ifid3.EarlyResolved, p.ifid3.AfterBranch},
		{p.ifid4.Valid, p.ifid4.PC, p.ifid4.InstructionWord, p.ifid4.PredictedTaken, p.ifid4.PredictedTarget, p.ifid4.EarlyResolved, p.ifid4.AfterBranch},
		{p.ifid5.Valid, p.ifid5.PC, p.ifid5.InstructionWord, p.ifid5.PredictedTaken, p.ifid5.PredictedTarget, p.ifid5.EarlyResolved, p.ifid5.AfterBranch},
		{p.ifid6.Valid, p.ifid6.PC, p.ifid6.InstructionWord, p.ifid6.PredictedTaken, p.ifid6.PredictedTarget, p.ifid6.EarlyResolved, p.ifid6.AfterBranch},
		{p.ifid7.Valid, p.ifid7.PC, p.ifid7.InstructionWord, p.ifid7.PredictedTaken, p.ifid7.PredictedTarget, p.ifid7.EarlyResolved, p.ifid7.AfterBranch},
		{p.ifid8.Valid, p.ifid8.PC, p.ifid8.InstructionWord, p.ifid8.PredictedTaken, p.ifid8.PredictedTarget, p.ifid8.EarlyResolved, p.ifid8.AfterBranch},
	}

	// Collect un-consumed entries
	var pending [8]instrWindowEntry
	pendingCount := 0
	for i := 0; i < len(consumed); i++ {
		if slots[i].valid && !consumed[i] {
			pending[pendingCount] = instrWindowEntry{
				Valid:           true,
				PC:              slots[i].pc,
				InstructionWord: slots[i].word,
				PredictedTaken:  slots[i].predictedTaken,
				PredictedTarget: slots[i].predictedTarget,
				EarlyResolved:   slots[i].earlyResolved,
				AfterBranch:     slots[i].afterBranch,
			}
			pendingCount++
		}
	}

	// Shift existing window entries down to make room at the front
	if pendingCount > 0 && p.instrWindowLen > 0 {
		// Move existing entries to after the pending ones
		newLen := p.instrWindowLen + pendingCount
		if newLen > instrWindowSize {
			newLen = instrWindowSize
		}
		// Shift existing entries right
		copyCount := newLen - pendingCount
		if copyCount > p.instrWindowLen {
			copyCount = p.instrWindowLen
		}
		for i := copyCount - 1; i >= 0; i-- {
			p.instrWindow[i+pendingCount] = p.instrWindow[i]
		}
		// Place pending entries at the front (they have priority as oldest)
		for i := 0; i < pendingCount; i++ {
			p.instrWindow[i] = pending[i]
		}
		p.instrWindowLen = newLen
	} else if pendingCount > 0 {
		for i := 0; i < pendingCount; i++ {
			p.instrWindow[i] = pending[i]
		}
		p.instrWindowLen = pendingCount
	}
}

// popWindowToIFID pops the first 8 entries from the instruction window
// into the IFID pipeline registers for decode/issue next cycle.
func (p *Pipeline) popWindowToIFID(
	ifid1 *IFIDRegister,
	ifid2 *SecondaryIFIDRegister,
	ifid3 *TertiaryIFIDRegister,
	ifid4 *QuaternaryIFIDRegister,
	ifid5 *QuinaryIFIDRegister,
	ifid6 *SenaryIFIDRegister,
	ifid7 *SeptenaryIFIDRegister,
	ifid8 *OctonaryIFIDRegister,
) {
	popCount := p.instrWindowLen
	if popCount > 8 {
		popCount = 8
	}

	for i := 0; i < popCount; i++ {
		e := p.instrWindow[i]
		switch i {
		case 0:
			*ifid1 = IFIDRegister{
				Valid: true, PC: e.PC, InstructionWord: e.InstructionWord,
				PredictedTaken: e.PredictedTaken, PredictedTarget: e.PredictedTarget,
				EarlyResolved: e.EarlyResolved, AfterBranch: e.AfterBranch,
			}
		case 1:
			*ifid2 = SecondaryIFIDRegister{
				Valid: true, PC: e.PC, InstructionWord: e.InstructionWord,
				PredictedTaken: e.PredictedTaken, PredictedTarget: e.PredictedTarget,
				EarlyResolved: e.EarlyResolved, AfterBranch: e.AfterBranch,
			}
		case 2:
			*ifid3 = TertiaryIFIDRegister{
				Valid: true, PC: e.PC, InstructionWord: e.InstructionWord,
				PredictedTaken: e.PredictedTaken, PredictedTarget: e.PredictedTarget,
				EarlyResolved: e.EarlyResolved, AfterBranch: e.AfterBranch,
			}
		case 3:
			*ifid4 = QuaternaryIFIDRegister{
				Valid: true, PC: e.PC, InstructionWord: e.InstructionWord,
				PredictedTaken: e.PredictedTaken, PredictedTarget: e.PredictedTarget,
				EarlyResolved: e.EarlyResolved, AfterBranch: e.AfterBranch,
			}
		case 4:
			*ifid5 = QuinaryIFIDRegister{
				Valid: true, PC: e.PC, InstructionWord: e.InstructionWord,
				PredictedTaken: e.PredictedTaken, PredictedTarget: e.PredictedTarget,
				EarlyResolved: e.EarlyResolved, AfterBranch: e.AfterBranch,
			}
		case 5:
			*ifid6 = SenaryIFIDRegister{
				Valid: true, PC: e.PC, InstructionWord: e.InstructionWord,
				PredictedTaken: e.PredictedTaken, PredictedTarget: e.PredictedTarget,
				EarlyResolved: e.EarlyResolved, AfterBranch: e.AfterBranch,
			}
		case 6:
			*ifid7 = SeptenaryIFIDRegister{
				Valid: true, PC: e.PC, InstructionWord: e.InstructionWord,
				PredictedTaken: e.PredictedTaken, PredictedTarget: e.PredictedTarget,
				EarlyResolved: e.EarlyResolved, AfterBranch: e.AfterBranch,
			}
		case 7:
			*ifid8 = OctonaryIFIDRegister{
				Valid: true, PC: e.PC, InstructionWord: e.InstructionWord,
				PredictedTaken: e.PredictedTaken, PredictedTarget: e.PredictedTarget,
				EarlyResolved: e.EarlyResolved, AfterBranch: e.AfterBranch,
			}
		}
	}

	// Remove popped entries from the window by shifting
	remaining := p.instrWindowLen - popCount
	for i := 0; i < remaining; i++ {
		p.instrWindow[i] = p.instrWindow[i+popCount]
	}
	// Clear vacated slots
	for i := remaining; i < p.instrWindowLen; i++ {
		p.instrWindow[i] = instrWindowEntry{}
	}
	p.instrWindowLen = remaining
}

// StallProfile returns a formatted string summarizing stall sources.
func (p *Pipeline) StallProfile() string {
	s := p.stats
	return fmt.Sprintf(
		"Stall Profile:\n"+
			"  Cycles:                    %d\n"+
			"  Instructions:              %d\n"+
			"  CPI:                       %.3f\n"+
			"  RAW Hazard Stalls:         %d\n"+
			"  Structural Hazard Stalls:  %d\n"+
			"  Exec Stalls:               %d\n"+
			"  Mem Stalls:                %d\n"+
			"  Branch Mispred Stalls:     %d\n"+
			"  Pipeline Flushes:          %d\n"+
			"  Branch Mispredictions:     %d\n"+
			"  Fetch/Other Stalls:        %d\n",
		s.Cycles,
		s.Instructions,
		s.CPI(),
		s.RAWHazardStalls,
		s.StructuralHazardStalls,
		s.ExecStalls,
		s.MemStalls,
		s.BranchMispredictionStalls,
		s.Flushes,
		s.BranchMispredictions,
		s.Stalls,
	)
}
