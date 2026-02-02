// Package pipeline provides a 5-stage pipeline model for cycle-accurate timing simulation.
package pipeline

// HazardUnit detects data hazards and controls forwarding/stalling.
type HazardUnit struct{}

// NewHazardUnit creates a new hazard detection unit.
func NewHazardUnit() *HazardUnit {
	return &HazardUnit{}
}

// ForwardingSource indicates where to forward data from.
type ForwardingSource uint8

const (
	// ForwardNone means no forwarding, use register file value.
	ForwardNone ForwardingSource = iota
	// ForwardFromEXMEM means forward from EX/MEM pipeline register.
	ForwardFromEXMEM
	// ForwardFromMEMWB means forward from MEM/WB pipeline register.
	ForwardFromMEMWB
)

// ForwardingResult contains forwarding decisions for both source operands.
type ForwardingResult struct {
	ForwardRn ForwardingSource
	ForwardRm ForwardingSource
}

// DetectForwarding determines if forwarding is needed for the instruction in ID/EX.
// This implements the forwarding logic to resolve RAW hazards when possible.
func (h *HazardUnit) DetectForwarding(idex *IDEXRegister, exmem *EXMEMRegister, memwb *MEMWBRegister) ForwardingResult {
	result := ForwardingResult{
		ForwardRn: ForwardNone,
		ForwardRm: ForwardNone,
	}

	if !idex.Valid {
		return result
	}

	// Check if Rn needs forwarding
	if idex.Rn != 31 { // XZR doesn't need forwarding
		// EX/MEM has priority (more recent result)
		if exmem.Valid && exmem.RegWrite && exmem.Rd == idex.Rn && exmem.Rd != 31 {
			result.ForwardRn = ForwardFromEXMEM
		} else if memwb.Valid && memwb.RegWrite && memwb.Rd == idex.Rn && memwb.Rd != 31 {
			result.ForwardRn = ForwardFromMEMWB
		}
	}

	// Check if Rm needs forwarding
	if idex.Rm != 31 { // XZR doesn't need forwarding
		// EX/MEM has priority (more recent result)
		if exmem.Valid && exmem.RegWrite && exmem.Rd == idex.Rm && exmem.Rd != 31 {
			result.ForwardRm = ForwardFromEXMEM
		} else if memwb.Valid && memwb.RegWrite && memwb.Rd == idex.Rm && memwb.Rd != 31 {
			result.ForwardRm = ForwardFromMEMWB
		}
	}

	return result
}

// DetectLoadUseHazard checks for load-use hazards that require stalling.
// A load-use hazard occurs when a LDR is followed immediately by an instruction
// that uses the loaded value. Forwarding alone cannot resolve this because the
// data isn't available until after the MEM stage.
func (h *HazardUnit) DetectLoadUseHazard(ifid *IFIDRegister, idex *IDEXRegister) bool {
	// If ID/EX contains a load (MemRead) and the instruction in IF/ID
	// uses the destination register, we must stall.
	if !idex.Valid || !idex.MemRead {
		return false
	}

	if !ifid.Valid {
		return false
	}

	// Check if the load destination matches either source of the next instruction.
	// We need to decode the instruction to know its source registers.
	// For simplicity, we'll check in the decode stage after decoding.
	// This function is called with already-decoded info.
	return false
}

// DetectLoadUseHazardDecoded checks for load-use hazard with decoded instruction info.
func (h *HazardUnit) DetectLoadUseHazardDecoded(loadRd uint8, nextRn uint8, nextRm uint8, nextUsesRn bool, nextUsesRm bool) bool {
	if loadRd == 31 {
		return false // XZR
	}

	if nextUsesRn && nextRn == loadRd {
		return true
	}

	if nextUsesRm && nextRm == loadRd {
		return true
	}

	return false
}

// GetForwardedValue returns the value to use based on forwarding source.
func (h *HazardUnit) GetForwardedValue(source ForwardingSource, originalValue uint64, exmem *EXMEMRegister, memwb *MEMWBRegister) uint64 {
	switch source {
	case ForwardFromEXMEM:
		return exmem.ALUResult
	case ForwardFromMEMWB:
		if memwb.MemToReg {
			return memwb.MemData
		}
		return memwb.ALUResult
	default:
		return originalValue
	}
}

// StallResult indicates what pipeline actions are needed.
type StallResult struct {
	// StallIF means the IF stage should not advance (refetch same instruction).
	StallIF bool
	// StallID means the ID stage should not advance.
	StallID bool
	// InsertBubbleEX means insert a NOP/bubble into the EX stage.
	InsertBubbleEX bool
	// FlushIF means flush the IF/ID register (for branches).
	FlushIF bool
	// FlushID means flush the ID/EX register (for branches).
	FlushID bool
}

// ComputeStalls determines stalling and flushing actions.
func (h *HazardUnit) ComputeStalls(loadUseHazard bool, branchTaken bool) StallResult {
	result := StallResult{}

	if loadUseHazard {
		result.StallIF = true
		result.StallID = true
		result.InsertBubbleEX = true
	}

	if branchTaken {
		// Flush the pipeline stages before the branch
		result.FlushIF = true
		result.FlushID = true
	}

	return result
}
