package pipeline

// ForwardSource indicates where a forwarded value should come from.
type ForwardSource int

// ForwardingSource is an alias for ForwardSource for API compatibility.
type ForwardingSource = ForwardSource

const (
	// ForwardNone means no forwarding needed - use register file value.
	ForwardNone ForwardSource = iota
	// ForwardFromEXMEM means forward from EX/MEM pipeline register.
	ForwardFromEXMEM
	// ForwardFromMEMWB means forward from MEM/WB pipeline register.
	ForwardFromMEMWB
)

// ForwardingResult contains forwarding decisions for both source operands.
type ForwardingResult struct {
	// ForwardRn specifies the forwarding source for the Rn operand.
	ForwardRn ForwardSource
	// ForwardRm specifies the forwarding source for the Rm operand.
	ForwardRm ForwardSource
	// ForwardRd specifies the forwarding source for the Rd operand (store data).
	ForwardRd ForwardSource
}

// StallResult contains stall and flush control signals.
type StallResult struct {
	// StallIF indicates the IF stage should stall (hold current instruction).
	StallIF bool
	// StallID indicates the ID stage should stall.
	StallID bool
	// InsertBubbleEX indicates a bubble (NOP) should be inserted in EX stage.
	InsertBubbleEX bool
	// FlushIF indicates the IF stage should be flushed (for branch).
	FlushIF bool
	// FlushID indicates the ID stage should be flushed (for branch).
	FlushID bool
}

// HazardUnit detects data hazards and determines forwarding/stall signals.
type HazardUnit struct{}

// NewHazardUnit creates a new hazard detection unit.
func NewHazardUnit() *HazardUnit {
	return &HazardUnit{}
}

// DetectForwarding determines if forwarding is needed for the ID/EX stage.
// It checks if the source registers (Rn, Rm) match the destination register
// of instructions in later pipeline stages. For store instructions, it also
// checks if the store data register (Rd) needs forwarding.
func (h *HazardUnit) DetectForwarding(
	idex *IDEXRegister,
	exmem *EXMEMRegister,
	memwb *MEMWBRegister,
) ForwardingResult {
	result := ForwardingResult{
		ForwardRn: ForwardNone,
		ForwardRm: ForwardNone,
		ForwardRd: ForwardNone,
	}

	if !idex.Valid {
		return result
	}

	// Check forwarding for Rn operand
	result.ForwardRn = h.detectForwardForReg(idex.Rn, exmem, memwb)

	// Check forwarding for Rm operand
	result.ForwardRm = h.detectForwardForReg(idex.Rm, exmem, memwb)

	// Check forwarding for Rd operand (store data)
	// For store instructions, Rd contains the register to store
	if idex.MemWrite {
		result.ForwardRd = h.detectForwardForReg(idex.Rd, exmem, memwb)
	}

	return result
}

// detectForwardForReg checks if a specific register needs forwarding.
func (h *HazardUnit) detectForwardForReg(
	reg uint8,
	exmem *EXMEMRegister,
	memwb *MEMWBRegister,
) ForwardSource {
	// XZR (register 31) always reads as 0, no need to forward
	if reg == 31 {
		return ForwardNone
	}

	// Priority: EX/MEM has precedence over MEM/WB (more recent value)
	// Check EX/MEM forwarding
	if exmem.Valid && exmem.RegWrite && exmem.Rd == reg {
		return ForwardFromEXMEM
	}

	// Check MEM/WB forwarding
	if memwb.Valid && memwb.RegWrite && memwb.Rd == reg {
		return ForwardFromMEMWB
	}

	return ForwardNone
}

// DetectLoadUseHazard detects load-use hazards where a load instruction
// is immediately followed by an instruction using the loaded value.
// This requires a stall because the value isn't available until MEM stage.
func (h *HazardUnit) DetectLoadUseHazard(idex *IDEXRegister, nextRn, nextRm uint8) bool {
	// Only load instructions cause load-use hazards
	if !idex.Valid || !idex.MemRead {
		return false
	}

	// XZR doesn't cause hazards
	if idex.Rd == 31 {
		return false
	}

	// Check if next instruction uses the load destination
	return idex.Rd == nextRn || idex.Rd == nextRm
}

// DetectLoadUseHazardDecoded detects load-use hazard using decoded register info.
// loadRd is the destination of the load instruction in ID/EX.
// nextRn, nextRm are the source registers of the next instruction.
// usesRn, usesRm indicate if the instruction actually uses these operands.
func (h *HazardUnit) DetectLoadUseHazardDecoded(
	loadRd uint8,
	nextRn, nextRm uint8,
	usesRn, usesRm bool,
) bool {
	// XZR doesn't cause hazards
	if loadRd == 31 {
		return false
	}

	// Check if next instruction uses the load destination as a source
	if usesRn && loadRd == nextRn {
		return true
	}
	if usesRm && loadRd == nextRm {
		return true
	}

	return false
}

// ComputeStalls computes stall and flush signals based on hazard conditions.
func (h *HazardUnit) ComputeStalls(loadUseHazard bool, branchTaken bool) StallResult {
	result := StallResult{}

	// Load-use hazard: stall IF and ID, insert bubble in EX
	if loadUseHazard {
		result.StallIF = true
		result.StallID = true
		result.InsertBubbleEX = true
	}

	// Branch taken: flush IF and ID (kill fetched/decoded instructions)
	if branchTaken {
		result.FlushIF = true
		result.FlushID = true
	}

	return result
}

// GetForwardedValue returns the value to use based on forwarding decision.
func (h *HazardUnit) GetForwardedValue(
	forward ForwardSource,
	originalValue uint64,
	exmem *EXMEMRegister,
	memwb *MEMWBRegister,
) uint64 {
	switch forward {
	case ForwardFromEXMEM:
		return exmem.ALUResult
	case ForwardFromMEMWB:
		// For load instructions, use memory data; otherwise use ALU result
		if memwb.MemToReg {
			return memwb.MemData
		}
		return memwb.ALUResult
	default:
		return originalValue
	}
}
