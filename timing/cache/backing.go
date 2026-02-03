// Package cache provides cache hierarchy modeling using Akita cache components.
package cache

import (
	"github.com/sarchlab/m2sim/emu"
)

// MemoryBacking wraps emu.Memory as a BackingStore.
type MemoryBacking struct {
	memory *emu.Memory
}

// NewMemoryBacking creates a new MemoryBacking adapter.
func NewMemoryBacking(memory *emu.Memory) *MemoryBacking {
	return &MemoryBacking{memory: memory}
}

// Read fetches data from the backing memory.
func (m *MemoryBacking) Read(addr uint64, size int) []byte {
	data := make([]byte, size)
	for i := 0; i < size; i++ {
		data[i] = m.memory.Read8(addr + uint64(i))
	}
	return data
}

// Write stores data to the backing memory.
func (m *MemoryBacking) Write(addr uint64, data []byte) {
	for i, b := range data {
		m.memory.Write8(addr+uint64(i), b)
	}
}
