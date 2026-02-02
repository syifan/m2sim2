// Package latency provides instruction timing models for cycle-accurate simulation.
//
// The latency values are based on Apple M2 microarchitecture estimates and
// can be configured via TimingConfig.
package latency

import (
	"github.com/sarchlab/m2sim/insts"
)

// Table provides instruction latency lookups.
type Table struct {
	config *TimingConfig
}

// NewTable creates a new latency table with default M2 timing values.
func NewTable() *Table {
	return &Table{
		config: DefaultTimingConfig(),
	}
}

// NewTableWithConfig creates a new latency table with custom timing configuration.
func NewTableWithConfig(config *TimingConfig) *Table {
	return &Table{
		config: config,
	}
}

// GetLatency returns the execution latency in cycles for the given instruction.
// For variable-latency operations, returns the typical/expected latency.
func (t *Table) GetLatency(inst *insts.Instruction) uint64 {
	if inst == nil {
		return 1
	}

	switch inst.Op {
	case insts.OpADD, insts.OpSUB, insts.OpAND, insts.OpORR, insts.OpEOR:
		return t.config.ALULatency

	case insts.OpB, insts.OpBL, insts.OpBCond, insts.OpBR, insts.OpBLR, insts.OpRET:
		return t.config.BranchLatency

	case insts.OpLDR:
		return t.config.LoadLatency

	case insts.OpSTR:
		return t.config.StoreLatency

	case insts.OpSVC:
		return t.config.SyscallLatency

	default:
		return 1
	}
}

// GetMinLatency returns the minimum execution latency for variable-latency operations.
func (t *Table) GetMinLatency(inst *insts.Instruction) uint64 {
	if inst == nil {
		return 1
	}

	// Currently all implemented instructions have fixed latency.
	// This method is for future multiply/divide support.
	return t.GetLatency(inst)
}

// GetMaxLatency returns the maximum execution latency for variable-latency operations.
func (t *Table) GetMaxLatency(inst *insts.Instruction) uint64 {
	if inst == nil {
		return 1
	}

	// Currently all implemented instructions have fixed latency.
	return t.GetLatency(inst)
}

// IsMemoryOp returns true if the instruction accesses memory.
func (t *Table) IsMemoryOp(inst *insts.Instruction) bool {
	if inst == nil {
		return false
	}
	return inst.Op == insts.OpLDR || inst.Op == insts.OpSTR
}

// IsLoadOp returns true if the instruction is a load operation.
func (t *Table) IsLoadOp(inst *insts.Instruction) bool {
	if inst == nil {
		return false
	}
	return inst.Op == insts.OpLDR
}

// IsStoreOp returns true if the instruction is a store operation.
func (t *Table) IsStoreOp(inst *insts.Instruction) bool {
	if inst == nil {
		return false
	}
	return inst.Op == insts.OpSTR
}

// IsBranchOp returns true if the instruction is a branch operation.
func (t *Table) IsBranchOp(inst *insts.Instruction) bool {
	if inst == nil {
		return false
	}
	switch inst.Op {
	case insts.OpB, insts.OpBL, insts.OpBCond, insts.OpBR, insts.OpBLR, insts.OpRET:
		return true
	default:
		return false
	}
}

// Config returns the current timing configuration.
func (t *Table) Config() *TimingConfig {
	return t.config
}
