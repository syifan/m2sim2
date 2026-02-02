// Package core provides the cycle-accurate CPU core model.
// It wraps the pipeline implementation to provide a high-level interface.
package core

import (
	"github.com/sarchlab/m2sim/emu"
	"github.com/sarchlab/m2sim/timing/pipeline"
)

// Stats holds performance statistics for the core.
type Stats struct {
	// Cycles is the total number of cycles simulated.
	Cycles uint64
	// Instructions is the number of instructions retired.
	Instructions uint64
	// Stalls is the number of stall cycles.
	Stalls uint64
	// Flushes is the number of pipeline flushes.
	Flushes uint64
}

// Core represents a cycle-accurate CPU core model.
// It wraps a 5-stage pipeline and provides a simple interface for simulation.
type Core struct {
	// Pipeline is the underlying 5-stage pipeline.
	Pipeline *pipeline.Pipeline

	// Shared resources
	regFile *emu.RegFile
	memory  *emu.Memory
}

// NewCore creates a new Core with the given register file and memory.
func NewCore(regFile *emu.RegFile, memory *emu.Memory) *Core {
	return &Core{
		Pipeline: pipeline.NewPipeline(regFile, memory),
		regFile:  regFile,
		memory:   memory,
	}
}

// SetPC sets the program counter.
func (c *Core) SetPC(pc uint64) {
	c.Pipeline.SetPC(pc)
}

// Tick executes one pipeline cycle.
func (c *Core) Tick() {
	c.Pipeline.Tick()
}

// Halted returns true if the core has halted (e.g., due to exit syscall).
func (c *Core) Halted() bool {
	return c.Pipeline.Halted()
}

// ExitCode returns the exit code if the core has halted.
func (c *Core) ExitCode() int64 {
	return c.Pipeline.ExitCode()
}

// Stats returns performance statistics for the core.
func (c *Core) Stats() Stats {
	pipeStats := c.Pipeline.Stats()
	return Stats{
		Cycles:       pipeStats.Cycles,
		Instructions: pipeStats.Instructions,
		Stalls:       pipeStats.Stalls,
		Flushes:      pipeStats.Flushes,
	}
}

// Run executes the core until it halts.
// Returns the exit code.
func (c *Core) Run() int64 {
	return c.Pipeline.Run()
}

// RunCycles executes the core for the specified number of cycles.
// Returns true if still running, false if halted.
func (c *Core) RunCycles(cycles uint64) bool {
	return c.Pipeline.RunCycles(cycles)
}

// Reset clears all core state.
func (c *Core) Reset() {
	c.Pipeline.Reset()
}
