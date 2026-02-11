// Package pipeline provides the 5-stage pipeline implementation for timing simulation.
package pipeline

import "github.com/sarchlab/m2sim/insts"

// SuperscalarConfig controls the superscalar execution width.
type SuperscalarConfig struct {
	// IssueWidth is the maximum number of instructions that can be issued per cycle.
	// Default is 1 (single-issue). Set to 2 for dual-issue.
	IssueWidth int
}

// DefaultSuperscalarConfig returns the default superscalar configuration (single-issue).
func DefaultSuperscalarConfig() SuperscalarConfig {
	return SuperscalarConfig{
		IssueWidth: 1,
	}
}

// DualIssueConfig returns a dual-issue superscalar configuration.
func DualIssueConfig() SuperscalarConfig {
	return SuperscalarConfig{
		IssueWidth: 2,
	}
}

// QuadIssueConfig returns a 4-wide superscalar configuration.
func QuadIssueConfig() SuperscalarConfig {
	return SuperscalarConfig{
		IssueWidth: 4,
	}
}

// SextupleIssueConfig returns a 6-wide superscalar configuration.
// This matches the Apple M2's 6 integer ALUs.
func SextupleIssueConfig() SuperscalarConfig {
	return SuperscalarConfig{
		IssueWidth: 6,
	}
}

// OctupleIssueConfig returns an 8-wide superscalar configuration.
// This matches the Apple M2's 8-wide decode bandwidth.
func OctupleIssueConfig() SuperscalarConfig {
	return SuperscalarConfig{
		IssueWidth: 8,
	}
}

// WithSuperscalar sets the superscalar configuration.
func WithSuperscalar(config SuperscalarConfig) PipelineOption {
	return func(p *Pipeline) {
		p.superscalarConfig = config
	}
}

// WithDualIssue enables dual-issue superscalar execution.
func WithDualIssue() PipelineOption {
	return func(p *Pipeline) {
		p.superscalarConfig = DualIssueConfig()
	}
}

// WithQuadIssue enables 4-wide superscalar execution.
func WithQuadIssue() PipelineOption {
	return func(p *Pipeline) {
		p.superscalarConfig = QuadIssueConfig()
	}
}

// WithSextupleIssue enables 6-wide superscalar execution.
// This matches the Apple M2's 6 integer ALUs.
func WithSextupleIssue() PipelineOption {
	return func(p *Pipeline) {
		p.superscalarConfig = SextupleIssueConfig()
	}
}

// WithOctupleIssue enables 8-wide superscalar execution.
// This matches the Apple M2's 8-wide decode bandwidth.
func WithOctupleIssue() PipelineOption {
	return func(p *Pipeline) {
		p.superscalarConfig = OctupleIssueConfig()
	}
}

// canDualIssue checks if two decoded instructions can be issued together.
// Returns true if the instructions have no data dependencies and can execute in parallel.
func canDualIssue(first, second *IDEXRegister) bool {
	if first == nil || second == nil || !first.Valid || !second.Valid {
		return false
	}

	// Cannot dual-issue if either is a branch
	if first.IsBranch || second.IsBranch {
		return false
	}

	// Cannot dual-issue syscalls
	if first.Inst != nil && first.Inst.Op == insts.OpSVC {
		return false
	}
	if second.Inst != nil && second.Inst.Op == insts.OpSVC {
		return false
	}

	// Cannot co-issue a load after a store (no store-to-load forwarding)
	if first.MemWrite && second.MemRead {
		return false
	}

	// Count loads and stores separately for port limiting
	loadOps := 0
	storeOps := 0
	if first.MemRead {
		loadOps++
	}
	if first.MemWrite {
		storeOps++
	}
	if second.MemRead {
		loadOps++
	}
	if second.MemWrite {
		storeOps++
	}
	if loadOps+storeOps > maxMemPorts {
		return false
	}
	if storeOps > maxStorePorts {
		return false
	}

	// Check for RAW hazard: second reads register that first writes.
	// Allow co-issue when the producer is a non-memory ALU op AND the
	// dependency is on Rn/Rm (which have forwarding paths). Block if:
	// - Producer is a load (result not available until MEM stage)
	// - Consumer is a store whose value register depends on producer
	//   (store value path doesn't support same-cycle forwarding)
	if first.RegWrite && first.Rd != 31 {
		hasRAW := false
		// Second instruction uses first's destination as source (Rn/Rm)
		if second.Rn == first.Rd && first.Rd != 31 {
			hasRAW = true
		}
		// Only check Rm for register-format instructions.
		// Immediate-format instructions (like ADD Xd, Xn, #imm) don't use Rm,
		// but Rm defaults to 0, causing false RAW hazards when first.Rd == 0.
		if second.Inst != nil && second.Inst.Format == insts.FormatDPReg {
			if second.Rm == first.Rd && first.Rd != 31 {
				hasRAW = true
			}
		}
		// For stores, the value register (Inst.Rd) is read through a
		// separate path that does NOT support same-cycle forwarding.
		// Always block co-issue for this dependency.
		if second.MemWrite && second.Inst != nil && second.Inst.Rd == first.Rd {
			return false
		}

		if hasRAW && first.MemRead {
			return false
		}
	}

	// Check for WAW hazard: both write to same register
	if first.RegWrite && second.RegWrite && first.Rd == second.Rd && first.Rd != 31 {
		return false
	}

	return true
}

// SecondaryIFIDRegister holds the second fetched instruction for dual-issue.
type SecondaryIFIDRegister struct {
	Valid           bool
	PC              uint64
	InstructionWord uint32
	PredictedTaken  bool
	PredictedTarget uint64
	EarlyResolved   bool
}

// SecondaryIDEXRegister holds the second decoded instruction for dual-issue.
type SecondaryIDEXRegister struct {
	Valid           bool
	PC              uint64
	Inst            *insts.Instruction
	RnValue         uint64
	RmValue         uint64
	Rd              uint8
	Rn              uint8
	Rm              uint8
	MemRead         bool
	MemWrite        bool
	RegWrite        bool
	MemToReg        bool
	IsBranch        bool
	PredictedTaken  bool
	PredictedTarget uint64
	EarlyResolved   bool
}

// SecondaryEXMEMRegister holds the second execute result for dual-issue.
type SecondaryEXMEMRegister struct {
	Valid      bool
	PC         uint64
	Inst       *insts.Instruction
	ALUResult  uint64
	StoreValue uint64
	Rd         uint8
	MemRead    bool
	MemWrite   bool
	RegWrite   bool
	MemToReg   bool

	// PSTATE flag forwarding fields.
	SetsFlags bool
	FlagN     bool
	FlagZ     bool
	FlagC     bool
	FlagV     bool
}

// SecondaryMEMWBRegister holds the second memory result for dual-issue.
type SecondaryMEMWBRegister struct {
	Valid     bool
	PC        uint64
	Inst      *insts.Instruction
	ALUResult uint64
	MemData   uint64
	Rd        uint8
	RegWrite  bool
	MemToReg  bool
}

// Clear resets the secondary IF/ID register.
func (r *SecondaryIFIDRegister) Clear() {
	r.Valid = false
	r.PC = 0
	r.InstructionWord = 0
	r.PredictedTaken = false
	r.PredictedTarget = 0
	r.EarlyResolved = false
}

// Clear resets the secondary ID/EX register.
func (r *SecondaryIDEXRegister) Clear() {
	r.Valid = false
	r.PC = 0
	r.Inst = nil
	r.RnValue = 0
	r.RmValue = 0
	r.Rd = 0
	r.Rn = 0
	r.Rm = 0
	r.MemRead = false
	r.MemWrite = false
	r.RegWrite = false
	r.MemToReg = false
	r.IsBranch = false
	r.PredictedTaken = false
	r.PredictedTarget = 0
	r.EarlyResolved = false
}

// Clear resets the secondary EX/MEM register.
func (r *SecondaryEXMEMRegister) Clear() {
	r.Valid = false
	r.PC = 0
	r.Inst = nil
	r.ALUResult = 0
	r.StoreValue = 0
	r.Rd = 0
	r.MemRead = false
	r.MemWrite = false
	r.RegWrite = false
	r.MemToReg = false
	r.SetsFlags = false
	r.FlagN = false
	r.FlagZ = false
	r.FlagC = false
	r.FlagV = false
}

// MemorySlot interface implementation for SecondaryEXMEMRegister

// IsValid returns true if the register contains valid data.
func (r *SecondaryEXMEMRegister) IsValid() bool { return r.Valid }

// GetPC returns the program counter.
func (r *SecondaryEXMEMRegister) GetPC() uint64 { return r.PC }

// GetMemRead returns true if this is a load instruction.
func (r *SecondaryEXMEMRegister) GetMemRead() bool { return r.MemRead }

// GetMemWrite returns true if this is a store instruction.
func (r *SecondaryEXMEMRegister) GetMemWrite() bool { return r.MemWrite }

// GetInst returns the instruction.
func (r *SecondaryEXMEMRegister) GetInst() *insts.Instruction { return r.Inst }

// GetALUResult returns the computed address/result.
func (r *SecondaryEXMEMRegister) GetALUResult() uint64 { return r.ALUResult }

// GetStoreValue returns the value to store.
func (r *SecondaryEXMEMRegister) GetStoreValue() uint64 { return r.StoreValue }

// Clear resets the secondary MEM/WB register.
func (r *SecondaryMEMWBRegister) Clear() {
	r.Valid = false
	r.PC = 0
	r.Inst = nil
	r.ALUResult = 0
	r.MemData = 0
	r.Rd = 0
	r.RegWrite = false
	r.MemToReg = false
}

// toIDEX converts SecondaryIDEXRegister to IDEXRegister for use with existing hazard/execute logic.
func (r *SecondaryIDEXRegister) toIDEX() IDEXRegister {
	return IDEXRegister{
		Valid:           r.Valid,
		PC:              r.PC,
		Inst:            r.Inst,
		RnValue:         r.RnValue,
		RmValue:         r.RmValue,
		Rd:              r.Rd,
		Rn:              r.Rn,
		Rm:              r.Rm,
		MemRead:         r.MemRead,
		MemWrite:        r.MemWrite,
		RegWrite:        r.RegWrite,
		MemToReg:        r.MemToReg,
		IsBranch:        r.IsBranch,
		PredictedTaken:  r.PredictedTaken,
		PredictedTarget: r.PredictedTarget,
		EarlyResolved:   r.EarlyResolved,
	}
}

// fromIDEX populates SecondaryIDEXRegister from IDEXRegister.
func (r *SecondaryIDEXRegister) fromIDEX(idex *IDEXRegister) {
	r.Valid = idex.Valid
	r.PC = idex.PC
	r.Inst = idex.Inst
	r.RnValue = idex.RnValue
	r.RmValue = idex.RmValue
	r.Rd = idex.Rd
	r.Rn = idex.Rn
	r.Rm = idex.Rm
	r.MemRead = idex.MemRead
	r.MemWrite = idex.MemWrite
	r.RegWrite = idex.RegWrite
	r.MemToReg = idex.MemToReg
	r.IsBranch = idex.IsBranch
	r.PredictedTaken = idex.PredictedTaken
	r.PredictedTarget = idex.PredictedTarget
	r.EarlyResolved = idex.EarlyResolved
}

// TertiaryIFIDRegister holds the third fetched instruction for 4-wide issue.
type TertiaryIFIDRegister struct {
	Valid           bool
	PC              uint64
	InstructionWord uint32
	PredictedTaken  bool
	PredictedTarget uint64
	EarlyResolved   bool
}

// TertiaryIDEXRegister holds the third decoded instruction for 4-wide issue.
type TertiaryIDEXRegister struct {
	Valid           bool
	PC              uint64
	Inst            *insts.Instruction
	RnValue         uint64
	RmValue         uint64
	Rd              uint8
	Rn              uint8
	Rm              uint8
	MemRead         bool
	MemWrite        bool
	RegWrite        bool
	MemToReg        bool
	IsBranch        bool
	PredictedTaken  bool
	PredictedTarget uint64
	EarlyResolved   bool
}

// TertiaryEXMEMRegister holds the third execute result for 4-wide issue.
type TertiaryEXMEMRegister struct {
	Valid      bool
	PC         uint64
	Inst       *insts.Instruction
	ALUResult  uint64
	StoreValue uint64
	Rd         uint8
	MemRead    bool
	MemWrite   bool
	RegWrite   bool
	MemToReg   bool

	// PSTATE flag forwarding fields.
	SetsFlags bool
	FlagN     bool
	FlagZ     bool
	FlagC     bool
	FlagV     bool
}

// TertiaryMEMWBRegister holds the third memory result for 4-wide issue.
type TertiaryMEMWBRegister struct {
	Valid     bool
	PC        uint64
	Inst      *insts.Instruction
	ALUResult uint64
	MemData   uint64
	Rd        uint8
	RegWrite  bool
	MemToReg  bool
}

// Clear resets the tertiary IF/ID register.
func (r *TertiaryIFIDRegister) Clear() {
	r.Valid = false
	r.PC = 0
	r.InstructionWord = 0
	r.PredictedTaken = false
	r.PredictedTarget = 0
	r.EarlyResolved = false
}

// Clear resets the tertiary ID/EX register.
func (r *TertiaryIDEXRegister) Clear() {
	r.Valid = false
	r.PC = 0
	r.Inst = nil
	r.RnValue = 0
	r.RmValue = 0
	r.Rd = 0
	r.Rn = 0
	r.Rm = 0
	r.MemRead = false
	r.MemWrite = false
	r.RegWrite = false
	r.MemToReg = false
	r.IsBranch = false
	r.PredictedTaken = false
	r.PredictedTarget = 0
	r.EarlyResolved = false
}

// Clear resets the tertiary EX/MEM register.
func (r *TertiaryEXMEMRegister) Clear() {
	r.Valid = false
	r.PC = 0
	r.Inst = nil
	r.ALUResult = 0
	r.StoreValue = 0
	r.Rd = 0
	r.MemRead = false
	r.MemWrite = false
	r.RegWrite = false
	r.MemToReg = false
	r.SetsFlags = false
	r.FlagN = false
	r.FlagZ = false
	r.FlagC = false
	r.FlagV = false
}

// MemorySlot interface implementation for TertiaryEXMEMRegister

// IsValid returns true if the register contains valid data.
func (r *TertiaryEXMEMRegister) IsValid() bool { return r.Valid }

// GetPC returns the program counter.
func (r *TertiaryEXMEMRegister) GetPC() uint64 { return r.PC }

// GetMemRead returns true if this is a load instruction.
func (r *TertiaryEXMEMRegister) GetMemRead() bool { return r.MemRead }

// GetMemWrite returns true if this is a store instruction.
func (r *TertiaryEXMEMRegister) GetMemWrite() bool { return r.MemWrite }

// GetInst returns the instruction.
func (r *TertiaryEXMEMRegister) GetInst() *insts.Instruction { return r.Inst }

// GetALUResult returns the computed address/result.
func (r *TertiaryEXMEMRegister) GetALUResult() uint64 { return r.ALUResult }

// GetStoreValue returns the value to store.
func (r *TertiaryEXMEMRegister) GetStoreValue() uint64 { return r.StoreValue }

// Clear resets the tertiary MEM/WB register.
func (r *TertiaryMEMWBRegister) Clear() {
	r.Valid = false
	r.PC = 0
	r.Inst = nil
	r.ALUResult = 0
	r.MemData = 0
	r.Rd = 0
	r.RegWrite = false
	r.MemToReg = false
}

// toIDEX converts TertiaryIDEXRegister to IDEXRegister.
//

func (r *TertiaryIDEXRegister) toIDEX() IDEXRegister {
	return IDEXRegister{
		Valid:           r.Valid,
		PC:              r.PC,
		Inst:            r.Inst,
		RnValue:         r.RnValue,
		RmValue:         r.RmValue,
		Rd:              r.Rd,
		Rn:              r.Rn,
		Rm:              r.Rm,
		MemRead:         r.MemRead,
		MemWrite:        r.MemWrite,
		RegWrite:        r.RegWrite,
		MemToReg:        r.MemToReg,
		IsBranch:        r.IsBranch,
		PredictedTaken:  r.PredictedTaken,
		PredictedTarget: r.PredictedTarget,
		EarlyResolved:   r.EarlyResolved,
	}
}

// fromIDEX populates TertiaryIDEXRegister from IDEXRegister.
func (r *TertiaryIDEXRegister) fromIDEX(idex *IDEXRegister) {
	r.Valid = idex.Valid
	r.PC = idex.PC
	r.Inst = idex.Inst
	r.RnValue = idex.RnValue
	r.RmValue = idex.RmValue
	r.Rd = idex.Rd
	r.Rn = idex.Rn
	r.Rm = idex.Rm
	r.MemRead = idex.MemRead
	r.MemWrite = idex.MemWrite
	r.RegWrite = idex.RegWrite
	r.MemToReg = idex.MemToReg
	r.IsBranch = idex.IsBranch
	r.PredictedTaken = idex.PredictedTaken
	r.PredictedTarget = idex.PredictedTarget
	r.EarlyResolved = idex.EarlyResolved
}

// QuaternaryIFIDRegister holds the fourth fetched instruction for 4-wide issue.
type QuaternaryIFIDRegister struct {
	Valid           bool
	PC              uint64
	InstructionWord uint32
	PredictedTaken  bool
	PredictedTarget uint64
	EarlyResolved   bool
}

// QuaternaryIDEXRegister holds the fourth decoded instruction for 4-wide issue.
type QuaternaryIDEXRegister struct {
	Valid           bool
	PC              uint64
	Inst            *insts.Instruction
	RnValue         uint64
	RmValue         uint64
	Rd              uint8
	Rn              uint8
	Rm              uint8
	MemRead         bool
	MemWrite        bool
	RegWrite        bool
	MemToReg        bool
	IsBranch        bool
	PredictedTaken  bool
	PredictedTarget uint64
	EarlyResolved   bool
}

// QuaternaryEXMEMRegister holds the fourth execute result for 4-wide issue.
type QuaternaryEXMEMRegister struct {
	Valid      bool
	PC         uint64
	Inst       *insts.Instruction
	ALUResult  uint64
	StoreValue uint64
	Rd         uint8
	MemRead    bool
	MemWrite   bool
	RegWrite   bool
	MemToReg   bool

	// PSTATE flag forwarding fields.
	SetsFlags bool
	FlagN     bool
	FlagZ     bool
	FlagC     bool
	FlagV     bool
}

// QuaternaryMEMWBRegister holds the fourth memory result for 4-wide issue.
type QuaternaryMEMWBRegister struct {
	Valid     bool
	PC        uint64
	Inst      *insts.Instruction
	ALUResult uint64
	MemData   uint64
	Rd        uint8
	RegWrite  bool
	MemToReg  bool
}

// Clear resets the quaternary IF/ID register.
func (r *QuaternaryIFIDRegister) Clear() {
	r.Valid = false
	r.PC = 0
	r.InstructionWord = 0
	r.PredictedTaken = false
	r.PredictedTarget = 0
	r.EarlyResolved = false
}

// Clear resets the quaternary ID/EX register.
func (r *QuaternaryIDEXRegister) Clear() {
	r.Valid = false
	r.PC = 0
	r.Inst = nil
	r.RnValue = 0
	r.RmValue = 0
	r.Rd = 0
	r.Rn = 0
	r.Rm = 0
	r.MemRead = false
	r.MemWrite = false
	r.RegWrite = false
	r.MemToReg = false
	r.IsBranch = false
	r.PredictedTaken = false
	r.PredictedTarget = 0
	r.EarlyResolved = false
}

// Clear resets the quaternary EX/MEM register.
func (r *QuaternaryEXMEMRegister) Clear() {
	r.Valid = false
	r.PC = 0
	r.Inst = nil
	r.ALUResult = 0
	r.StoreValue = 0
	r.Rd = 0
	r.MemRead = false
	r.MemWrite = false
	r.RegWrite = false
	r.MemToReg = false
	r.SetsFlags = false
	r.FlagN = false
	r.FlagZ = false
	r.FlagC = false
	r.FlagV = false
}

// MemorySlot interface implementation for QuaternaryEXMEMRegister

// IsValid returns true if the register contains valid data.
func (r *QuaternaryEXMEMRegister) IsValid() bool { return r.Valid }

// GetPC returns the program counter.
func (r *QuaternaryEXMEMRegister) GetPC() uint64 { return r.PC }

// GetMemRead returns true if this is a load instruction.
func (r *QuaternaryEXMEMRegister) GetMemRead() bool { return r.MemRead }

// GetMemWrite returns true if this is a store instruction.
func (r *QuaternaryEXMEMRegister) GetMemWrite() bool { return r.MemWrite }

// GetInst returns the instruction.
func (r *QuaternaryEXMEMRegister) GetInst() *insts.Instruction { return r.Inst }

// GetALUResult returns the computed address/result.
func (r *QuaternaryEXMEMRegister) GetALUResult() uint64 { return r.ALUResult }

// GetStoreValue returns the value to store.
func (r *QuaternaryEXMEMRegister) GetStoreValue() uint64 { return r.StoreValue }

// Clear resets the quaternary MEM/WB register.
func (r *QuaternaryMEMWBRegister) Clear() {
	r.Valid = false
	r.PC = 0
	r.Inst = nil
	r.ALUResult = 0
	r.MemData = 0
	r.Rd = 0
	r.RegWrite = false
	r.MemToReg = false
}

// toIDEX converts QuaternaryIDEXRegister to IDEXRegister.
//

func (r *QuaternaryIDEXRegister) toIDEX() IDEXRegister {
	return IDEXRegister{
		Valid:           r.Valid,
		PC:              r.PC,
		Inst:            r.Inst,
		RnValue:         r.RnValue,
		RmValue:         r.RmValue,
		Rd:              r.Rd,
		Rn:              r.Rn,
		Rm:              r.Rm,
		MemRead:         r.MemRead,
		MemWrite:        r.MemWrite,
		RegWrite:        r.RegWrite,
		MemToReg:        r.MemToReg,
		IsBranch:        r.IsBranch,
		PredictedTaken:  r.PredictedTaken,
		PredictedTarget: r.PredictedTarget,
		EarlyResolved:   r.EarlyResolved,
	}
}

// fromIDEX populates QuaternaryIDEXRegister from IDEXRegister.
func (r *QuaternaryIDEXRegister) fromIDEX(idex *IDEXRegister) {
	r.Valid = idex.Valid
	r.PC = idex.PC
	r.Inst = idex.Inst
	r.RnValue = idex.RnValue
	r.RmValue = idex.RmValue
	r.Rd = idex.Rd
	r.Rn = idex.Rn
	r.Rm = idex.Rm
	r.MemRead = idex.MemRead
	r.MemWrite = idex.MemWrite
	r.RegWrite = idex.RegWrite
	r.MemToReg = idex.MemToReg
	r.IsBranch = idex.IsBranch
	r.PredictedTaken = idex.PredictedTaken
	r.PredictedTarget = idex.PredictedTarget
	r.EarlyResolved = idex.EarlyResolved
}

// QuinaryIFIDRegister holds the fifth fetched instruction for 6-wide issue.
type QuinaryIFIDRegister struct {
	Valid           bool
	PC              uint64
	InstructionWord uint32
	PredictedTaken  bool
	PredictedTarget uint64
	EarlyResolved   bool
}

// QuinaryIDEXRegister holds the decoded instruction for wide issue.
type QuinaryIDEXRegister struct {
	Valid           bool
	PC              uint64
	Inst            *insts.Instruction
	RnValue         uint64
	RmValue         uint64
	Rd              uint8
	Rn              uint8
	Rm              uint8
	MemRead         bool
	MemWrite        bool
	RegWrite        bool
	MemToReg        bool
	IsBranch        bool
	PredictedTaken  bool
	PredictedTarget uint64
	EarlyResolved   bool
}

// QuinaryEXMEMRegister holds the fifth execute result for 6-wide issue.
type QuinaryEXMEMRegister struct {
	Valid      bool
	PC         uint64
	Inst       *insts.Instruction
	ALUResult  uint64
	StoreValue uint64
	Rd         uint8
	MemRead    bool
	MemWrite   bool
	RegWrite   bool
	MemToReg   bool

	// PSTATE flag forwarding fields.
	SetsFlags bool
	FlagN     bool
	FlagZ     bool
	FlagC     bool
	FlagV     bool
}

// QuinaryMEMWBRegister holds the fifth memory result for 6-wide issue.
type QuinaryMEMWBRegister struct {
	Valid     bool
	PC        uint64
	Inst      *insts.Instruction
	ALUResult uint64
	MemData   uint64
	Rd        uint8
	RegWrite  bool
	MemToReg  bool
}

// Clear resets the quinary IF/ID register.
func (r *QuinaryIFIDRegister) Clear() {
	r.Valid = false
	r.PC = 0
	r.InstructionWord = 0
	r.PredictedTaken = false
	r.PredictedTarget = 0
	r.EarlyResolved = false
}

// Clear resets the quinary ID/EX register.
func (r *QuinaryIDEXRegister) Clear() {
	r.Valid = false
	r.PC = 0
	r.Inst = nil
	r.RnValue = 0
	r.RmValue = 0
	r.Rd = 0
	r.Rn = 0
	r.Rm = 0
	r.MemRead = false
	r.MemWrite = false
	r.RegWrite = false
	r.MemToReg = false
	r.IsBranch = false
	r.PredictedTaken = false
	r.PredictedTarget = 0
	r.EarlyResolved = false
}

// Clear resets the quinary EX/MEM register.
func (r *QuinaryEXMEMRegister) Clear() {
	r.Valid = false
	r.PC = 0
	r.Inst = nil
	r.ALUResult = 0
	r.StoreValue = 0
	r.Rd = 0
	r.MemRead = false
	r.MemWrite = false
	r.RegWrite = false
	r.MemToReg = false
	r.SetsFlags = false
	r.FlagN = false
	r.FlagZ = false
	r.FlagC = false
	r.FlagV = false
}

// MemorySlot interface implementation for QuinaryEXMEMRegister

// IsValid returns true if the register contains valid data.
func (r *QuinaryEXMEMRegister) IsValid() bool { return r.Valid }

// GetPC returns the program counter.
func (r *QuinaryEXMEMRegister) GetPC() uint64 { return r.PC }

// GetMemRead returns true if this is a load instruction.
func (r *QuinaryEXMEMRegister) GetMemRead() bool { return r.MemRead }

// GetMemWrite returns true if this is a store instruction.
func (r *QuinaryEXMEMRegister) GetMemWrite() bool { return r.MemWrite }

// GetInst returns the instruction.
func (r *QuinaryEXMEMRegister) GetInst() *insts.Instruction { return r.Inst }

// GetALUResult returns the computed address/result.
func (r *QuinaryEXMEMRegister) GetALUResult() uint64 { return r.ALUResult }

// GetStoreValue returns the value to store.
func (r *QuinaryEXMEMRegister) GetStoreValue() uint64 { return r.StoreValue }

// Clear resets the quinary MEM/WB register.
func (r *QuinaryMEMWBRegister) Clear() {
	r.Valid = false
	r.PC = 0
	r.Inst = nil
	r.ALUResult = 0
	r.MemData = 0
	r.Rd = 0
	r.RegWrite = false
	r.MemToReg = false
}

// toIDEX converts QuinaryIDEXRegister to IDEXRegister.
func (r *QuinaryIDEXRegister) toIDEX() IDEXRegister {
	return IDEXRegister{
		Valid:           r.Valid,
		PC:              r.PC,
		Inst:            r.Inst,
		RnValue:         r.RnValue,
		RmValue:         r.RmValue,
		Rd:              r.Rd,
		Rn:              r.Rn,
		Rm:              r.Rm,
		MemRead:         r.MemRead,
		MemWrite:        r.MemWrite,
		RegWrite:        r.RegWrite,
		MemToReg:        r.MemToReg,
		IsBranch:        r.IsBranch,
		PredictedTaken:  r.PredictedTaken,
		PredictedTarget: r.PredictedTarget,
		EarlyResolved:   r.EarlyResolved,
	}
}

// fromIDEX populates QuinaryIDEXRegister from IDEXRegister.
func (r *QuinaryIDEXRegister) fromIDEX(idex *IDEXRegister) {
	r.Valid = idex.Valid
	r.PC = idex.PC
	r.Inst = idex.Inst
	r.RnValue = idex.RnValue
	r.RmValue = idex.RmValue
	r.Rd = idex.Rd
	r.Rn = idex.Rn
	r.Rm = idex.Rm
	r.MemRead = idex.MemRead
	r.MemWrite = idex.MemWrite
	r.RegWrite = idex.RegWrite
	r.MemToReg = idex.MemToReg
	r.IsBranch = idex.IsBranch
	r.PredictedTaken = idex.PredictedTaken
	r.PredictedTarget = idex.PredictedTarget
	r.EarlyResolved = idex.EarlyResolved
}

// SenaryIFIDRegister holds the sixth fetched instruction for 6-wide issue.
type SenaryIFIDRegister struct {
	Valid           bool
	PC              uint64
	InstructionWord uint32
	PredictedTaken  bool
	PredictedTarget uint64
	EarlyResolved   bool
}

// SenaryIDEXRegister holds the decoded instruction for wide issue.
type SenaryIDEXRegister struct {
	Valid           bool
	PC              uint64
	Inst            *insts.Instruction
	RnValue         uint64
	RmValue         uint64
	Rd              uint8
	Rn              uint8
	Rm              uint8
	MemRead         bool
	MemWrite        bool
	RegWrite        bool
	MemToReg        bool
	IsBranch        bool
	PredictedTaken  bool
	PredictedTarget uint64
	EarlyResolved   bool
}

// SenaryEXMEMRegister holds the sixth execute result for 6-wide issue.
type SenaryEXMEMRegister struct {
	Valid      bool
	PC         uint64
	Inst       *insts.Instruction
	ALUResult  uint64
	StoreValue uint64
	Rd         uint8
	MemRead    bool
	MemWrite   bool
	RegWrite   bool
	MemToReg   bool

	// PSTATE flag forwarding fields.
	SetsFlags bool
	FlagN     bool
	FlagZ     bool
	FlagC     bool
	FlagV     bool
}

// SenaryMEMWBRegister holds the sixth memory result for 6-wide issue.
type SenaryMEMWBRegister struct {
	Valid     bool
	PC        uint64
	Inst      *insts.Instruction
	ALUResult uint64
	MemData   uint64
	Rd        uint8
	RegWrite  bool
	MemToReg  bool
}

// Clear resets the senary IF/ID register.
func (r *SenaryIFIDRegister) Clear() {
	r.Valid = false
	r.PC = 0
	r.InstructionWord = 0
	r.PredictedTaken = false
	r.PredictedTarget = 0
	r.EarlyResolved = false
}

// Clear resets the senary ID/EX register.
func (r *SenaryIDEXRegister) Clear() {
	r.Valid = false
	r.PC = 0
	r.Inst = nil
	r.RnValue = 0
	r.RmValue = 0
	r.Rd = 0
	r.Rn = 0
	r.Rm = 0
	r.MemRead = false
	r.MemWrite = false
	r.RegWrite = false
	r.MemToReg = false
	r.IsBranch = false
	r.PredictedTaken = false
	r.PredictedTarget = 0
	r.EarlyResolved = false
}

// Clear resets the senary EX/MEM register.
func (r *SenaryEXMEMRegister) Clear() {
	r.Valid = false
	r.PC = 0
	r.Inst = nil
	r.ALUResult = 0
	r.StoreValue = 0
	r.Rd = 0
	r.MemRead = false
	r.MemWrite = false
	r.RegWrite = false
	r.MemToReg = false
	r.SetsFlags = false
	r.FlagN = false
	r.FlagZ = false
	r.FlagC = false
	r.FlagV = false
}

// MemorySlot interface implementation for SenaryEXMEMRegister

// IsValid returns true if the register contains valid data.
func (r *SenaryEXMEMRegister) IsValid() bool { return r.Valid }

// GetPC returns the program counter.
func (r *SenaryEXMEMRegister) GetPC() uint64 { return r.PC }

// GetMemRead returns true if this is a load instruction.
func (r *SenaryEXMEMRegister) GetMemRead() bool { return r.MemRead }

// GetMemWrite returns true if this is a store instruction.
func (r *SenaryEXMEMRegister) GetMemWrite() bool { return r.MemWrite }

// GetInst returns the instruction.
func (r *SenaryEXMEMRegister) GetInst() *insts.Instruction { return r.Inst }

// GetALUResult returns the computed address/result.
func (r *SenaryEXMEMRegister) GetALUResult() uint64 { return r.ALUResult }

// GetStoreValue returns the value to store.
func (r *SenaryEXMEMRegister) GetStoreValue() uint64 { return r.StoreValue }

// Clear resets the senary MEM/WB register.
func (r *SenaryMEMWBRegister) Clear() {
	r.Valid = false
	r.PC = 0
	r.Inst = nil
	r.ALUResult = 0
	r.MemData = 0
	r.Rd = 0
	r.RegWrite = false
	r.MemToReg = false
}

// toIDEX converts SenaryIDEXRegister to IDEXRegister.
func (r *SenaryIDEXRegister) toIDEX() IDEXRegister {
	return IDEXRegister{
		Valid:           r.Valid,
		PC:              r.PC,
		Inst:            r.Inst,
		RnValue:         r.RnValue,
		RmValue:         r.RmValue,
		Rd:              r.Rd,
		Rn:              r.Rn,
		Rm:              r.Rm,
		MemRead:         r.MemRead,
		MemWrite:        r.MemWrite,
		RegWrite:        r.RegWrite,
		MemToReg:        r.MemToReg,
		IsBranch:        r.IsBranch,
		PredictedTaken:  r.PredictedTaken,
		PredictedTarget: r.PredictedTarget,
		EarlyResolved:   r.EarlyResolved,
	}
}

// fromIDEX populates SenaryIDEXRegister from IDEXRegister.
func (r *SenaryIDEXRegister) fromIDEX(idex *IDEXRegister) {
	r.Valid = idex.Valid
	r.PC = idex.PC
	r.Inst = idex.Inst
	r.RnValue = idex.RnValue
	r.RmValue = idex.RmValue
	r.Rd = idex.Rd
	r.Rn = idex.Rn
	r.Rm = idex.Rm
	r.MemRead = idex.MemRead
	r.MemWrite = idex.MemWrite
	r.RegWrite = idex.RegWrite
	r.MemToReg = idex.MemToReg
	r.IsBranch = idex.IsBranch
	r.PredictedTaken = idex.PredictedTaken
	r.PredictedTarget = idex.PredictedTarget
	r.EarlyResolved = idex.EarlyResolved
}

// maxALUPorts is the maximum number of integer ALU operations that can execute
// per cycle. Apple M2 Avalanche has 6 integer ALU execution units.
const maxALUPorts = 6

// maxLoadPorts is the maximum number of load operations that can execute per
// cycle. Apple M2 Avalanche has 3 load ports (2 dedicated LD + 1 dual LD/ST).
const maxLoadPorts = 3

// maxStorePorts is the maximum number of store operations that can execute per
// cycle. Apple M2 Avalanche has 2 store ports (1 dedicated ST + 1 dual LD/ST).
const maxStorePorts = 2

// maxMemPorts is the total AGU bandwidth (load + store combined).
const maxMemPorts = 3

// maxWritePorts is the maximum number of register file write-back ports per
// cycle. This limits how many register-writing instructions can be issued in
// the same cycle due to register-file write-back bandwidth, independent of
// execution unit count. Calibrated against M2 hardware: arithmetic benchmark
// (5-register ALU cycling) shows CPI=0.296, consistent with 4 write ports
// limiting sustained throughput.
const maxWritePorts = 4

// isALUOp returns true if the instruction uses an integer ALU execution port.
func isALUOp(inst *IDEXRegister) bool {
	if inst == nil || !inst.Valid {
		return false
	}
	// Memory operations use load/store units, not ALU ports
	if inst.MemRead || inst.MemWrite {
		return false
	}
	// Branches use a separate branch unit
	if inst.IsBranch {
		return false
	}
	// SVC is handled separately
	if inst.Inst != nil && inst.Inst.Op == insts.OpSVC {
		return false
	}
	return true
}

// canIssueWith checks if a new instruction can be issued with a set of previously issued instructions.
// This is a generalized version that checks dependencies against all earlier instructions in the batch.
func canIssueWith(newInst *IDEXRegister, earlier []*IDEXRegister) bool {
	if newInst == nil || !newInst.Valid {
		return false
	}

	// Cannot issue branches in superscalar mode (only in slot 0)
	if newInst.IsBranch {
		return false
	}

	// Cannot issue syscalls in secondary slots
	if newInst.Inst != nil && newInst.Inst.Op == insts.OpSVC {
		return false
	}

	// Memory operations can only execute in slots with memory ports (first maxMemPorts slots).
	// The new instruction would go into slot len(earlier), so reject memory ops in slots >= maxMemPorts.
	newAccessesMem := newInst.MemRead || newInst.MemWrite
	if newAccessesMem && len(earlier) >= maxMemPorts {
		return false
	}

	// Count loads and stores separately for port limiting
	loadCount := 0
	storeCount := 0
	if newInst.MemRead {
		loadCount = 1
	}
	if newInst.MemWrite {
		storeCount = 1
	}

	// Count ALU operations for port limiting
	aluOpCount := 0
	if isALUOp(newInst) {
		aluOpCount = 1
	}

	// Count register write-back ports
	writePortCount := 0
	if newInst.RegWrite {
		writePortCount = 1
	}

	for _, prev := range earlier {
		if prev == nil || !prev.Valid {
			continue
		}

		// Cannot co-issue a load after a store (no store-to-load forwarding)
		if prev.MemWrite && newInst.MemRead {
			return false
		}

		if prev.MemRead {
			loadCount++
		}
		if prev.MemWrite {
			storeCount++
		}

		if isALUOp(prev) {
			aluOpCount++
		}

		if prev.RegWrite {
			writePortCount++
		}

		// Check for RAW hazard: new reads register that prev writes.
		// Allow co-issue when the producer is a non-memory ALU op AND the
		// dependency is on Rn/Rm (which have forwarding paths). Block if:
		// - Producer is a load (result not available until MEM stage)
		// - Consumer is a store whose value register depends on producer
		//   (store value path doesn't support same-cycle forwarding)
		if prev.RegWrite && prev.Rd != 31 {
			hasRAW := false
			if newInst.Rn == prev.Rd {
				hasRAW = true
			}
			// Only check Rm for register-format instructions
			if newInst.Inst != nil && newInst.Inst.Format == insts.FormatDPReg {
				if newInst.Rm == prev.Rd {
					hasRAW = true
				}
			}
			// For stores, the value register (Inst.Rd) is read through a
			// separate path that does NOT support same-cycle forwarding.
			// Always block co-issue for this dependency.
			if newInst.MemWrite && newInst.Inst != nil && newInst.Inst.Rd == prev.Rd {
				return false
			}

			if hasRAW && prev.MemRead {
				return false
			}
		}

		// Check for WAW hazard: both write to same register
		if prev.RegWrite && newInst.RegWrite && prev.Rd == newInst.Rd && prev.Rd != 31 {
			return false
		}
	}

	// Limit total memory operations to AGU bandwidth
	if loadCount+storeCount > maxMemPorts {
		return false
	}

	// Limit loads to available load ports
	if loadCount > maxLoadPorts {
		return false
	}

	// Limit stores to available store ports
	if storeCount > maxStorePorts {
		return false
	}

	// Limit ALU operations to available execution ports
	if aluOpCount > maxALUPorts {
		return false
	}

	// Limit register write-back ports
	if writePortCount > maxWritePorts {
		return false
	}

	return true
}

// WritebackSlot interface implementation for SecondaryMEMWBRegister

// IsValid returns true if the register contains valid data.
func (r *SecondaryMEMWBRegister) IsValid() bool { return r.Valid }

// GetRegWrite returns true if this instruction writes to a register.
func (r *SecondaryMEMWBRegister) GetRegWrite() bool { return r.RegWrite }

// GetRd returns the destination register.
func (r *SecondaryMEMWBRegister) GetRd() uint8 { return r.Rd }

// GetMemToReg returns true if the value comes from memory.
func (r *SecondaryMEMWBRegister) GetMemToReg() bool { return r.MemToReg }

// GetALUResult returns the ALU computation result.
func (r *SecondaryMEMWBRegister) GetALUResult() uint64 { return r.ALUResult }

// GetMemData returns the data loaded from memory.
func (r *SecondaryMEMWBRegister) GetMemData() uint64 { return r.MemData }

// GetIsFused returns false as fusion only occurs in slot 0.
func (r *SecondaryMEMWBRegister) GetIsFused() bool { return false }

// WritebackSlot interface implementation for TertiaryMEMWBRegister

// IsValid returns true if the register contains valid data.
func (r *TertiaryMEMWBRegister) IsValid() bool { return r.Valid }

// GetRegWrite returns true if this instruction writes to a register.
func (r *TertiaryMEMWBRegister) GetRegWrite() bool { return r.RegWrite }

// GetRd returns the destination register.
func (r *TertiaryMEMWBRegister) GetRd() uint8 { return r.Rd }

// GetMemToReg returns true if the value comes from memory.
func (r *TertiaryMEMWBRegister) GetMemToReg() bool { return r.MemToReg }

// GetALUResult returns the ALU computation result.
func (r *TertiaryMEMWBRegister) GetALUResult() uint64 { return r.ALUResult }

// GetMemData returns the data loaded from memory.
func (r *TertiaryMEMWBRegister) GetMemData() uint64 { return r.MemData }

// GetIsFused returns false as fusion only occurs in slot 0.
func (r *TertiaryMEMWBRegister) GetIsFused() bool { return false }

// WritebackSlot interface implementation for QuaternaryMEMWBRegister

// IsValid returns true if the register contains valid data.
func (r *QuaternaryMEMWBRegister) IsValid() bool { return r.Valid }

// GetRegWrite returns true if this instruction writes to a register.
func (r *QuaternaryMEMWBRegister) GetRegWrite() bool { return r.RegWrite }

// GetRd returns the destination register.
func (r *QuaternaryMEMWBRegister) GetRd() uint8 { return r.Rd }

// GetMemToReg returns true if the value comes from memory.
func (r *QuaternaryMEMWBRegister) GetMemToReg() bool { return r.MemToReg }

// GetALUResult returns the ALU computation result.
func (r *QuaternaryMEMWBRegister) GetALUResult() uint64 { return r.ALUResult }

// GetMemData returns the data loaded from memory.
func (r *QuaternaryMEMWBRegister) GetMemData() uint64 { return r.MemData }

// GetIsFused returns false as fusion only occurs in slot 0.
func (r *QuaternaryMEMWBRegister) GetIsFused() bool { return false }

// WritebackSlot interface implementation for QuinaryMEMWBRegister

// IsValid returns true if the register contains valid data.
func (r *QuinaryMEMWBRegister) IsValid() bool { return r.Valid }

// GetRegWrite returns true if this instruction writes to a register.
func (r *QuinaryMEMWBRegister) GetRegWrite() bool { return r.RegWrite }

// GetRd returns the destination register.
func (r *QuinaryMEMWBRegister) GetRd() uint8 { return r.Rd }

// GetMemToReg returns true if the value comes from memory.
func (r *QuinaryMEMWBRegister) GetMemToReg() bool { return r.MemToReg }

// GetALUResult returns the ALU computation result.
func (r *QuinaryMEMWBRegister) GetALUResult() uint64 { return r.ALUResult }

// GetMemData returns the data loaded from memory.
func (r *QuinaryMEMWBRegister) GetMemData() uint64 { return r.MemData }

// GetIsFused returns false as fusion only occurs in slot 0.
func (r *QuinaryMEMWBRegister) GetIsFused() bool { return false }

// WritebackSlot interface implementation for SenaryMEMWBRegister

// IsValid returns true if the register contains valid data.
func (r *SenaryMEMWBRegister) IsValid() bool { return r.Valid }

// GetRegWrite returns true if this instruction writes to a register.
func (r *SenaryMEMWBRegister) GetRegWrite() bool { return r.RegWrite }

// GetRd returns the destination register.
func (r *SenaryMEMWBRegister) GetRd() uint8 { return r.Rd }

// GetMemToReg returns true if the value comes from memory.
func (r *SenaryMEMWBRegister) GetMemToReg() bool { return r.MemToReg }

// GetALUResult returns the ALU computation result.
func (r *SenaryMEMWBRegister) GetALUResult() uint64 { return r.ALUResult }

// GetMemData returns the data loaded from memory.
func (r *SenaryMEMWBRegister) GetMemData() uint64 { return r.MemData }

// GetIsFused returns false as fusion only occurs in slot 0.
func (r *SenaryMEMWBRegister) GetIsFused() bool { return false }

// SeptenaryIFIDRegister holds the seventh fetched instruction for 8-wide issue.
type SeptenaryIFIDRegister struct {
	Valid           bool
	PC              uint64
	InstructionWord uint32
	PredictedTaken  bool
	PredictedTarget uint64
	EarlyResolved   bool
}

// SeptenaryIDEXRegister holds the decoded instruction for wide issue.
type SeptenaryIDEXRegister struct {
	Valid           bool
	PC              uint64
	Inst            *insts.Instruction
	RnValue         uint64
	RmValue         uint64
	Rd              uint8
	Rn              uint8
	Rm              uint8
	MemRead         bool
	MemWrite        bool
	RegWrite        bool
	MemToReg        bool
	IsBranch        bool
	PredictedTaken  bool
	PredictedTarget uint64
	EarlyResolved   bool
}

// SeptenaryEXMEMRegister holds the seventh execute result for 8-wide issue.
type SeptenaryEXMEMRegister struct {
	Valid      bool
	PC         uint64
	Inst       *insts.Instruction
	ALUResult  uint64
	StoreValue uint64
	Rd         uint8
	MemRead    bool
	MemWrite   bool
	RegWrite   bool
	MemToReg   bool

	// PSTATE flag forwarding fields.
	SetsFlags bool
	FlagN     bool
	FlagZ     bool
	FlagC     bool
	FlagV     bool
}

// SeptenaryMEMWBRegister holds the seventh memory result for 8-wide issue.
type SeptenaryMEMWBRegister struct {
	Valid     bool
	PC        uint64
	Inst      *insts.Instruction
	ALUResult uint64
	MemData   uint64
	Rd        uint8
	RegWrite  bool
	MemToReg  bool
}

// Clear resets the septenary IF/ID register.
func (r *SeptenaryIFIDRegister) Clear() {
	r.Valid = false
	r.PC = 0
	r.InstructionWord = 0
	r.PredictedTaken = false
	r.PredictedTarget = 0
	r.EarlyResolved = false
}

// Clear resets the septenary ID/EX register.
func (r *SeptenaryIDEXRegister) Clear() {
	r.Valid = false
	r.PC = 0
	r.Inst = nil
	r.RnValue = 0
	r.RmValue = 0
	r.Rd = 0
	r.Rn = 0
	r.Rm = 0
	r.MemRead = false
	r.MemWrite = false
	r.RegWrite = false
	r.MemToReg = false
	r.IsBranch = false
	r.PredictedTaken = false
	r.PredictedTarget = 0
	r.EarlyResolved = false
}

// Clear resets the septenary EX/MEM register.
func (r *SeptenaryEXMEMRegister) Clear() {
	r.Valid = false
	r.PC = 0
	r.Inst = nil
	r.ALUResult = 0
	r.StoreValue = 0
	r.Rd = 0
	r.MemRead = false
	r.MemWrite = false
	r.RegWrite = false
	r.MemToReg = false
	r.SetsFlags = false
	r.FlagN = false
	r.FlagZ = false
	r.FlagC = false
	r.FlagV = false
}

// MemorySlot interface implementation for SeptenaryEXMEMRegister

// IsValid returns true if the register contains valid data.
func (r *SeptenaryEXMEMRegister) IsValid() bool { return r.Valid }

// GetPC returns the program counter.
func (r *SeptenaryEXMEMRegister) GetPC() uint64 { return r.PC }

// GetMemRead returns true if this is a load instruction.
func (r *SeptenaryEXMEMRegister) GetMemRead() bool { return r.MemRead }

// GetMemWrite returns true if this is a store instruction.
func (r *SeptenaryEXMEMRegister) GetMemWrite() bool { return r.MemWrite }

// GetInst returns the instruction.
func (r *SeptenaryEXMEMRegister) GetInst() *insts.Instruction { return r.Inst }

// GetALUResult returns the computed address/result.
func (r *SeptenaryEXMEMRegister) GetALUResult() uint64 { return r.ALUResult }

// GetStoreValue returns the value to store.
func (r *SeptenaryEXMEMRegister) GetStoreValue() uint64 { return r.StoreValue }

// Clear resets the septenary MEM/WB register.
func (r *SeptenaryMEMWBRegister) Clear() {
	r.Valid = false
	r.PC = 0
	r.Inst = nil
	r.ALUResult = 0
	r.MemData = 0
	r.Rd = 0
	r.RegWrite = false
	r.MemToReg = false
}

// toIDEX converts SeptenaryIDEXRegister to IDEXRegister.
func (r *SeptenaryIDEXRegister) toIDEX() IDEXRegister {
	return IDEXRegister{
		Valid:           r.Valid,
		PC:              r.PC,
		Inst:            r.Inst,
		RnValue:         r.RnValue,
		RmValue:         r.RmValue,
		Rd:              r.Rd,
		Rn:              r.Rn,
		Rm:              r.Rm,
		MemRead:         r.MemRead,
		MemWrite:        r.MemWrite,
		RegWrite:        r.RegWrite,
		MemToReg:        r.MemToReg,
		IsBranch:        r.IsBranch,
		PredictedTaken:  r.PredictedTaken,
		PredictedTarget: r.PredictedTarget,
		EarlyResolved:   r.EarlyResolved,
	}
}

// fromIDEX populates SeptenaryIDEXRegister from IDEXRegister.
func (r *SeptenaryIDEXRegister) fromIDEX(idex *IDEXRegister) {
	r.Valid = idex.Valid
	r.PC = idex.PC
	r.Inst = idex.Inst
	r.RnValue = idex.RnValue
	r.RmValue = idex.RmValue
	r.Rd = idex.Rd
	r.Rn = idex.Rn
	r.Rm = idex.Rm
	r.MemRead = idex.MemRead
	r.MemWrite = idex.MemWrite
	r.RegWrite = idex.RegWrite
	r.MemToReg = idex.MemToReg
	r.IsBranch = idex.IsBranch
	r.PredictedTaken = idex.PredictedTaken
	r.PredictedTarget = idex.PredictedTarget
	r.EarlyResolved = idex.EarlyResolved
}

// WritebackSlot interface implementation for SeptenaryMEMWBRegister

// IsValid returns true if the register contains valid data.
func (r *SeptenaryMEMWBRegister) IsValid() bool { return r.Valid }

// GetRegWrite returns true if this instruction writes to a register.
func (r *SeptenaryMEMWBRegister) GetRegWrite() bool { return r.RegWrite }

// GetRd returns the destination register.
func (r *SeptenaryMEMWBRegister) GetRd() uint8 { return r.Rd }

// GetMemToReg returns true if the value comes from memory.
func (r *SeptenaryMEMWBRegister) GetMemToReg() bool { return r.MemToReg }

// GetALUResult returns the ALU computation result.
func (r *SeptenaryMEMWBRegister) GetALUResult() uint64 { return r.ALUResult }

// GetMemData returns the data loaded from memory.
func (r *SeptenaryMEMWBRegister) GetMemData() uint64 { return r.MemData }

// GetIsFused returns false as fusion only occurs in slot 0.
func (r *SeptenaryMEMWBRegister) GetIsFused() bool { return false }

// OctonaryIFIDRegister holds the eighth fetched instruction for 8-wide issue.
type OctonaryIFIDRegister struct {
	Valid           bool
	PC              uint64
	InstructionWord uint32
	PredictedTaken  bool
	PredictedTarget uint64
	EarlyResolved   bool
}

// OctonaryIDEXRegister holds the decoded instruction for wide issue.
type OctonaryIDEXRegister struct {
	Valid           bool
	PC              uint64
	Inst            *insts.Instruction
	RnValue         uint64
	RmValue         uint64
	Rd              uint8
	Rn              uint8
	Rm              uint8
	MemRead         bool
	MemWrite        bool
	RegWrite        bool
	MemToReg        bool
	IsBranch        bool
	PredictedTaken  bool
	PredictedTarget uint64
	EarlyResolved   bool
}

// OctonaryEXMEMRegister holds the eighth execute result for 8-wide issue.
type OctonaryEXMEMRegister struct {
	Valid      bool
	PC         uint64
	Inst       *insts.Instruction
	ALUResult  uint64
	StoreValue uint64
	Rd         uint8
	MemRead    bool
	MemWrite   bool
	RegWrite   bool
	MemToReg   bool

	// PSTATE flag forwarding fields.
	SetsFlags bool
	FlagN     bool
	FlagZ     bool
	FlagC     bool
	FlagV     bool
}

// OctonaryMEMWBRegister holds the eighth memory result for 8-wide issue.
type OctonaryMEMWBRegister struct {
	Valid     bool
	PC        uint64
	Inst      *insts.Instruction
	ALUResult uint64
	MemData   uint64
	Rd        uint8
	RegWrite  bool
	MemToReg  bool
}

// Clear resets the octonary IF/ID register.
func (r *OctonaryIFIDRegister) Clear() {
	r.Valid = false
	r.PC = 0
	r.InstructionWord = 0
	r.PredictedTaken = false
	r.PredictedTarget = 0
	r.EarlyResolved = false
}

// Clear resets the octonary ID/EX register.
func (r *OctonaryIDEXRegister) Clear() {
	r.Valid = false
	r.PC = 0
	r.Inst = nil
	r.RnValue = 0
	r.RmValue = 0
	r.Rd = 0
	r.Rn = 0
	r.Rm = 0
	r.MemRead = false
	r.MemWrite = false
	r.RegWrite = false
	r.MemToReg = false
	r.IsBranch = false
	r.PredictedTaken = false
	r.PredictedTarget = 0
	r.EarlyResolved = false
}

// Clear resets the octonary EX/MEM register.
func (r *OctonaryEXMEMRegister) Clear() {
	r.Valid = false
	r.PC = 0
	r.Inst = nil
	r.ALUResult = 0
	r.StoreValue = 0
	r.Rd = 0
	r.MemRead = false
	r.MemWrite = false
	r.RegWrite = false
	r.MemToReg = false
	r.SetsFlags = false
	r.FlagN = false
	r.FlagZ = false
	r.FlagC = false
	r.FlagV = false
}

// MemorySlot interface implementation for OctonaryEXMEMRegister

// IsValid returns true if the register contains valid data.
func (r *OctonaryEXMEMRegister) IsValid() bool { return r.Valid }

// GetPC returns the program counter.
func (r *OctonaryEXMEMRegister) GetPC() uint64 { return r.PC }

// GetMemRead returns true if this is a load instruction.
func (r *OctonaryEXMEMRegister) GetMemRead() bool { return r.MemRead }

// GetMemWrite returns true if this is a store instruction.
func (r *OctonaryEXMEMRegister) GetMemWrite() bool { return r.MemWrite }

// GetInst returns the instruction.
func (r *OctonaryEXMEMRegister) GetInst() *insts.Instruction { return r.Inst }

// GetALUResult returns the computed address/result.
func (r *OctonaryEXMEMRegister) GetALUResult() uint64 { return r.ALUResult }

// GetStoreValue returns the value to store.
func (r *OctonaryEXMEMRegister) GetStoreValue() uint64 { return r.StoreValue }

// Clear resets the octonary MEM/WB register.
func (r *OctonaryMEMWBRegister) Clear() {
	r.Valid = false
	r.PC = 0
	r.Inst = nil
	r.ALUResult = 0
	r.MemData = 0
	r.Rd = 0
	r.RegWrite = false
	r.MemToReg = false
}

// toIDEX converts OctonaryIDEXRegister to IDEXRegister.
func (r *OctonaryIDEXRegister) toIDEX() IDEXRegister {
	return IDEXRegister{
		Valid:           r.Valid,
		PC:              r.PC,
		Inst:            r.Inst,
		RnValue:         r.RnValue,
		RmValue:         r.RmValue,
		Rd:              r.Rd,
		Rn:              r.Rn,
		Rm:              r.Rm,
		MemRead:         r.MemRead,
		MemWrite:        r.MemWrite,
		RegWrite:        r.RegWrite,
		MemToReg:        r.MemToReg,
		IsBranch:        r.IsBranch,
		PredictedTaken:  r.PredictedTaken,
		PredictedTarget: r.PredictedTarget,
		EarlyResolved:   r.EarlyResolved,
	}
}

// fromIDEX populates OctonaryIDEXRegister from IDEXRegister.
func (r *OctonaryIDEXRegister) fromIDEX(idex *IDEXRegister) {
	r.Valid = idex.Valid
	r.PC = idex.PC
	r.Inst = idex.Inst
	r.RnValue = idex.RnValue
	r.RmValue = idex.RmValue
	r.Rd = idex.Rd
	r.Rn = idex.Rn
	r.Rm = idex.Rm
	r.MemRead = idex.MemRead
	r.MemWrite = idex.MemWrite
	r.RegWrite = idex.RegWrite
	r.MemToReg = idex.MemToReg
	r.IsBranch = idex.IsBranch
	r.PredictedTaken = idex.PredictedTaken
	r.PredictedTarget = idex.PredictedTarget
	r.EarlyResolved = idex.EarlyResolved
}

// WritebackSlot interface implementation for OctonaryMEMWBRegister

// IsValid returns true if the register contains valid data.
func (r *OctonaryMEMWBRegister) IsValid() bool { return r.Valid }

// GetRegWrite returns true if this instruction writes to a register.
func (r *OctonaryMEMWBRegister) GetRegWrite() bool { return r.RegWrite }

// GetRd returns the destination register.
func (r *OctonaryMEMWBRegister) GetRd() uint8 { return r.Rd }

// GetMemToReg returns true if the value comes from memory.
func (r *OctonaryMEMWBRegister) GetMemToReg() bool { return r.MemToReg }

// GetALUResult returns the ALU computation result.
func (r *OctonaryMEMWBRegister) GetALUResult() uint64 { return r.ALUResult }

// GetMemData returns the data loaded from memory.
func (r *OctonaryMEMWBRegister) GetMemData() uint64 { return r.MemData }

// GetIsFused returns false as fusion only occurs in slot 0.
func (r *OctonaryMEMWBRegister) GetIsFused() bool { return false }
