package pipeline

import (
	"github.com/sarchlab/m2sim/emu"
	"github.com/sarchlab/m2sim/timing/cache"
)

// CachedFetchStage fetches instructions through L1 instruction cache.
type CachedFetchStage struct {
	cache     *cache.Cache
	memory    *emu.Memory
	pending   bool         // True if waiting for cache miss
	pendingPC uint64       // PC being waited on
	latency   uint64       // Remaining latency cycles
	result    *fetchResult // Cached result while waiting
}

type fetchResult struct {
	word uint32
	ok   bool
}

// NewCachedFetchStage creates a new cached fetch stage.
func NewCachedFetchStage(icache *cache.Cache, memory *emu.Memory) *CachedFetchStage {
	return &CachedFetchStage{
		cache:  icache,
		memory: memory,
	}
}

// Fetch fetches an instruction word through the I-cache.
// Returns the instruction word, whether fetch completed, and latency info.
func (s *CachedFetchStage) Fetch(pc uint64) (word uint32, ok bool, stall bool) {
	// If PC changed, cancel any pending request (e.g., branch taken)
	if s.pending && s.pendingPC != pc {
		s.pending = false
		s.latency = 0
		s.result = nil
	}

	// If still waiting for previous miss at same PC
	if s.pending {
		s.latency--
		if s.latency > 0 {
			return 0, false, true // Still stalling
		}
		// Miss serviced
		s.pending = false
		if s.result != nil {
			return s.result.word, s.result.ok, false
		}
		return 0, false, false
	}

	// Access I-cache
	result := s.cache.Read(pc, 4)

	if result.Hit {
		// Hit: return immediately
		return uint32(result.Data), true, false
	}

	// Miss: need to wait for miss latency
	s.pending = true
	s.pendingPC = pc
	s.latency = result.Latency - 1 // Already consumed 1 cycle
	s.result = &fetchResult{
		word: uint32(result.Data),
		ok:   true,
	}

	if s.latency > 0 {
		return 0, false, true // Stall
	}

	// Single-cycle miss (latency = 1)
	s.pending = false
	return uint32(result.Data), true, false
}

// Reset clears pending state.
func (s *CachedFetchStage) Reset() {
	s.pending = false
	s.latency = 0
	s.result = nil
}

// CachedMemoryStage handles memory reads and writes through L1 data cache.
type CachedMemoryStage struct {
	cache       *cache.Cache
	memory      *emu.Memory
	pending     bool       // True if waiting for cache miss
	pendingAddr uint64     // Address being waited on
	pendingPC   uint64     // PC of instruction being waited on
	latency     uint64     // Remaining latency cycles
	result      *memResult // Cached result while waiting
}

type memResult struct {
	data uint64
}

// NewCachedMemoryStage creates a new cached memory stage.
func NewCachedMemoryStage(dcache *cache.Cache, memory *emu.Memory) *CachedMemoryStage {
	return &CachedMemoryStage{
		cache:  dcache,
		memory: memory,
	}
}

// Access performs memory read or write through D-cache.
// Returns result and whether the operation is stalling.
func (s *CachedMemoryStage) Access(exmem *EXMEMRegister) (MemoryResult, bool) {
	result := MemoryResult{}

	if !exmem.Valid {
		// If the register is not valid, clear any pending state
		s.pending = false
		return result, false
	}

	// If not a memory operation, no stall
	if !exmem.MemRead && !exmem.MemWrite {
		s.pending = false
		return result, false
	}

	addr := exmem.ALUResult

	// If PC/addr changed, this is a different memory operation - cancel pending
	if s.pending && (s.pendingPC != exmem.PC || s.pendingAddr != addr) {
		s.pending = false
		s.latency = 0
		s.result = nil
	}

	// If still waiting for previous miss at same address
	if s.pending {
		s.latency--
		if s.latency > 0 {
			return result, true // Still stalling
		}
		// Miss serviced
		s.pending = false
		if s.result != nil && exmem.MemRead {
			result.MemData = s.result.data
		}
		return result, false
	}

	// Determine access size
	size := 8
	if exmem.Inst != nil && !exmem.Inst.Is64Bit {
		size = 4
	}

	if exmem.MemRead {
		// Load through D-cache
		cacheResult := s.cache.Read(addr, size)

		if cacheResult.Hit {
			result.MemData = cacheResult.Data
			return result, false
		}

		// Miss: need to wait
		s.pending = true
		s.pendingPC = exmem.PC
		s.pendingAddr = addr
		s.latency = cacheResult.Latency - 1
		s.result = &memResult{data: cacheResult.Data}

		if s.latency > 0 {
			return result, true // Stall
		}

		// Single-cycle miss
		s.pending = false
		result.MemData = cacheResult.Data
		return result, false
	}

	if exmem.MemWrite {
		// Store through D-cache
		cacheResult := s.cache.Write(addr, size, exmem.StoreValue)

		if cacheResult.Hit {
			return result, false
		}

		// Miss: need to wait
		s.pending = true
		s.pendingPC = exmem.PC
		s.pendingAddr = addr
		s.latency = cacheResult.Latency - 1
		s.result = nil

		if s.latency > 0 {
			return result, true // Stall
		}

		// Single-cycle miss
		s.pending = false
		return result, false
	}

	return result, false
}

// Reset clears pending state.
func (s *CachedMemoryStage) Reset() {
	s.pending = false
	s.latency = 0
	s.result = nil
}

// CacheStats returns the underlying cache statistics.
func (s *CachedMemoryStage) CacheStats() cache.Statistics {
	return s.cache.Stats()
}

// CacheStats returns the underlying cache statistics.
func (s *CachedFetchStage) CacheStats() cache.Statistics {
	return s.cache.Stats()
}
