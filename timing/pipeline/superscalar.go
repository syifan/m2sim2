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

	// Store-to-load forwarding: M2 has a 56-entry store buffer that
	// handles forwarding transparently. For our benchmark workloads
	// (PolyBench kernels use separate arrays for input/output),
	// store-to-load conflicts are essentially nonexistent.

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
		// Only check Rm for register-format instructions and register-offset loads/stores.
		// Immediate-format instructions (like ADD Xd, Xn, #imm) don't use Rm,
		// but Rm defaults to 0, causing false RAW hazards when first.Rd == 0.
		if second.Inst != nil && second.Rm == first.Rd && first.Rd != 31 {
			if second.Inst.Format == insts.FormatDPReg ||
				(second.Inst.Format == insts.FormatLoadStore && second.Inst.IndexMode == insts.IndexRegBase) {
				hasRAW = true
			}
		}
		// For stores, the value register (Inst.Rd) is read through a
		// separate path that does NOT support same-cycle forwarding.
		// Always block co-issue for this dependency.
		if second.MemWrite && second.Inst != nil && second.Inst.Rd == first.Rd {
			return false
		}

		if hasRAW {
			return false
		}
	}

	// WAW hazard relaxed: M2 has register renaming so pure WAW (both write
	// same Rd) is not a real hazard. The in-order writeback ensures the
	// later instruction's result wins.

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
	AfterBranch     bool
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
}

// Clear resets the secondary ID/EX register.
func (r *SecondaryIDEXRegister) Clear() {
	r.Valid = false
	r.Inst = nil
}

// Clear resets the secondary EX/MEM register.
func (r *SecondaryEXMEMRegister) Clear() {
	r.Valid = false
	r.Inst = nil
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
	r.Inst = nil
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
	AfterBranch     bool
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
}

// Clear resets the tertiary ID/EX register.
func (r *TertiaryIDEXRegister) Clear() {
	r.Valid = false
	r.Inst = nil
}

// Clear resets the tertiary EX/MEM register.
func (r *TertiaryEXMEMRegister) Clear() {
	r.Valid = false
	r.Inst = nil
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
	r.Inst = nil
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
	AfterBranch     bool
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
}

// Clear resets the quaternary ID/EX register.
func (r *QuaternaryIDEXRegister) Clear() {
	r.Valid = false
	r.Inst = nil
}

// Clear resets the quaternary EX/MEM register.
func (r *QuaternaryEXMEMRegister) Clear() {
	r.Valid = false
	r.Inst = nil
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
	r.Inst = nil
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
	AfterBranch     bool
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
}

// Clear resets the quinary ID/EX register.
func (r *QuinaryIDEXRegister) Clear() {
	r.Valid = false
	r.Inst = nil
}

// Clear resets the quinary EX/MEM register.
func (r *QuinaryEXMEMRegister) Clear() {
	r.Valid = false
	r.Inst = nil
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
	r.Inst = nil
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
	AfterBranch     bool
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
}

// Clear resets the senary ID/EX register.
func (r *SenaryIDEXRegister) Clear() {
	r.Valid = false
	r.Inst = nil
}

// Clear resets the senary EX/MEM register.
func (r *SenaryEXMEMRegister) Clear() {
	r.Valid = false
	r.Inst = nil
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
	r.Inst = nil
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
// M2 Avalanche has 3 load + 2 store = 5 total AGU ports.
// The MEM stage has 5 hardware memory ports (slots 1-5).
const maxMemPorts = 5

// maxWritePorts is the maximum number of register file write-back ports per
// cycle. This limits how many register-writing instructions can be issued in
// the same cycle due to register-file write-back bandwidth, independent of
// execution unit count. M2 has 6 ALU units + 3 load ports that write back.
const maxWritePorts = 8

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
// Uses a fixed-size array to avoid heap allocation per tick cycle.
// The forwarded parameter (optional, may be nil) tracks which earlier
// instructions were issued via same-cycle ALU forwarding. When non-nil,
// ALU-to-ALU RAW dependencies are allowed (same-cycle forwarding) as long
// as the producer was not itself forwarded (max 1-hop forwarding depth).
// Returns (canIssue, usesForwarding).
func canIssueWith(newInst *IDEXRegister, earlier *[8]*IDEXRegister, earlierCount int, issued *[8]bool) bool {
	ok, _ := canIssueWithFwd(newInst, earlier, earlierCount, issued, nil)
	return ok
}

// canIssueWithFwd is the full-featured version of canIssueWith that supports
// ALU-to-ALU same-cycle forwarding tracking.
func canIssueWithFwd(newInst *IDEXRegister, earlier *[8]*IDEXRegister, earlierCount int, issued *[8]bool, forwarded *[8]bool) (bool, bool) {
	if newInst == nil || !newInst.Valid {
		return false, false
	}

	// Cannot issue branches in superscalar mode (only in slot 0)
	if newInst.IsBranch {
		return false, false
	}

	// Cannot issue syscalls in secondary slots
	if newInst.Inst != nil && newInst.Inst.Op == insts.OpSVC {
		return false, false
	}

	// Count actually-issued instructions for slot position check.
	actualIssuedCount := 0
	for i := 0; i < earlierCount; i++ {
		if issued != nil && issued[i] {
			actualIssuedCount++
		}
	}

	// Memory operations can only execute in slots with memory ports (first maxMemPorts slots).
	// Use actualIssuedCount (not earlierCount) since non-issued instructions don't occupy ports.
	newAccessesMem := newInst.MemRead || newInst.MemWrite
	if newAccessesMem && actualIssuedCount >= maxMemPorts {
		return false, false
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

	usesForwarding := false

	for i := 0; i < earlierCount; i++ {
		prev := earlier[i]
		if prev == nil || !prev.Valid {
			continue
		}

		// With register checkpointing, we can allow non-store instructions
		// after predicted-taken branches. On misprediction, the checkpoint
		// restores all register state. Stores are still blocked because
		// memory writes cannot be rolled back.
		if prev.IsBranch {
			if !prev.PredictedTaken {
				return false, false
			}
			if newInst.MemWrite {
				return false, false
			}
		}

		// Only count port usage for actually-issued instructions.
		isIssued := issued != nil && issued[i]

		// Store-to-load ordering: when a load is in the same issue
		// group as an earlier store targeting the same address
		// (same base register AND same immediate offset), block
		// the load regardless of whether the store issued.
		// The store must complete first so the cache's store-forward
		// path can apply the appropriate latency penalty.
		// Skip SP (reg 31) since stack spill/reload patterns are
		// usually separated by many instructions and don't need
		// dispatch-level serialization.
		if newInst.MemRead && prev.MemWrite &&
			newInst.Rn == prev.Rn && newInst.Rn != 31 &&
			newInst.Inst != nil && prev.Inst != nil &&
			newInst.Inst.Imm == prev.Inst.Imm {
			return false, false
		}
		if isIssued {
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
			// Only check Rm for register-format instructions and register-offset loads/stores
			if newInst.Inst != nil && newInst.Rm == prev.Rd && prev.Rd != 31 {
				if newInst.Inst.Format == insts.FormatDPReg ||
					(newInst.Inst.Format == insts.FormatLoadStore && newInst.Inst.IndexMode == insts.IndexRegBase) {
					hasRAW = true
				}
			}
			// For stores, the value register (Inst.Rd) is read through a
			// separate path that does NOT support same-cycle forwarding.
			// Always block co-issue for this dependency.
			if newInst.MemWrite && newInst.Inst != nil && newInst.Inst.Rd == prev.Rd {
				return false, false
			}

			if hasRAW {
				// Same-cycle ALU forwarding: if the producer is a non-memory
				// ALU op that was issued (not just decoded), its result is
				// available via nextEXMEM forwarding in the EX stage.
				producerIsALU := isIssued && !prev.MemRead && !prev.MemWrite && !prev.IsBranch

				// ALU→Load address forwarding: always allow when the
				// consumer is a load instruction reading a register that
				// an issued ALU op writes. The AGU can receive the
				// forwarded ALU result for address computation. This
				// cannot chain (load results aren't available until MEM),
				// so no depth tracking is needed.
				consumerIsLoad := newInst.MemRead && !newInst.MemWrite
				if producerIsALU && consumerIsLoad {
					usesForwarding = true
				} else if forwarded != nil && producerIsALU {
					// General ALU→ALU forwarding with 1-hop depth limit:
					// the producer must not itself be a forwarding consumer
					// (to prevent unrealistic deep chaining like A→B→C in
					// one cycle).
					producerNotForwarded := !forwarded[i]
					if producerNotForwarded {
						usesForwarding = true
					} else {
						return false, false
					}
				} else {
					return false, false
				}
			}
		}

		// Pre/post-indexed load/store instructions write to Rn (base register)
		// in addition to any normal Rd write. This hidden write has no
		// forwarding path, so block co-issue with any instruction that
		// reads the same register.
		if prev.Inst != nil && prev.Rn != 31 &&
			(prev.MemRead || prev.MemWrite) &&
			(prev.Inst.IndexMode == insts.IndexPre || prev.Inst.IndexMode == insts.IndexPost) {
			if newInst.Rn == prev.Rn || newInst.Rm == prev.Rn {
				return false, false
			}
			// Store value register may also read the base register
			if newInst.MemWrite && newInst.Inst != nil && newInst.Inst.Rd == prev.Rn {
				return false, false
			}
		}

		// WAW hazard relaxed: M2 has register renaming so pure WAW (both
		// write same Rd) is not a real hazard. The in-order writeback
		// ensures the later instruction's result wins.

		// WAW hazard for pre/post-indexed Rn writes
		if prev.Inst != nil && prev.Rn != 31 &&
			(prev.MemRead || prev.MemWrite) &&
			(prev.Inst.IndexMode == insts.IndexPre || prev.Inst.IndexMode == insts.IndexPost) {
			if newInst.Inst != nil {
				// New instruction also writes to the same Rn via pre/post-index
				if (newInst.MemRead || newInst.MemWrite) &&
					(newInst.Inst.IndexMode == insts.IndexPre || newInst.Inst.IndexMode == insts.IndexPost) &&
					newInst.Rn == prev.Rn {
					return false, false
				}
			}
			// New instruction writes to Rn via normal Rd
			if newInst.RegWrite && newInst.Rd == prev.Rn {
				return false, false
			}
		}

		// WAW hazard: prev writes Rd, new writes same register via pre/post-index Rn
		if prev.RegWrite && prev.Rd != 31 && newInst.Inst != nil &&
			(newInst.MemRead || newInst.MemWrite) &&
			(newInst.Inst.IndexMode == insts.IndexPre || newInst.Inst.IndexMode == insts.IndexPost) &&
			newInst.Rn == prev.Rd {
			return false, false
		}
	}

	// Limit total memory operations to AGU bandwidth
	if loadCount+storeCount > maxMemPorts {
		return false, false
	}

	// Limit loads to available load ports
	if loadCount > maxLoadPorts {
		return false, false
	}

	// Limit stores to available store ports
	if storeCount > maxStorePorts {
		return false, false
	}

	// Limit ALU operations to available execution ports
	if aluOpCount > maxALUPorts {
		return false, false
	}

	// Limit register write-back ports
	if writePortCount > maxWritePorts {
		return false, false
	}

	return true, usesForwarding
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
	AfterBranch     bool
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
}

// Clear resets the septenary ID/EX register.
func (r *SeptenaryIDEXRegister) Clear() {
	r.Valid = false
	r.Inst = nil
}

// Clear resets the septenary EX/MEM register.
func (r *SeptenaryEXMEMRegister) Clear() {
	r.Valid = false
	r.Inst = nil
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
	r.Inst = nil
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
	AfterBranch     bool
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
}

// Clear resets the octonary ID/EX register.
func (r *OctonaryIDEXRegister) Clear() {
	r.Valid = false
	r.Inst = nil
}

// Clear resets the octonary EX/MEM register.
func (r *OctonaryEXMEMRegister) Clear() {
	r.Valid = false
	r.Inst = nil
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
	r.Inst = nil
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
