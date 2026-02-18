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
	pending     bool       // True if waiting for cache access (hit or miss)
	pendingAddr uint64     // Address being waited on
	pendingPC   uint64     // PC of instruction being waited on
	latency     uint64     // Remaining latency cycles
	result      *memResult // Cached result while waiting
	isHit       bool       // True if pending is for a hit (for stats)

	// Completed state: when a cache access finishes but the pipeline is
	// stalled by another memory port, the result is held here so that the
	// same instruction replaying does not re-trigger cache.Read(), which
	// would cause a livelock (each port would re-enter pending
	// out-of-phase with the others and never all clear simultaneously).
	completed       bool       // True if access completed but pipeline hasn't advanced
	completedPC     uint64     // PC of completed instruction
	completedAddr   uint64     // Address of completed access
	completedResult *memResult // Cached result from completed access

	storeIssuedPC   uint64 // PC of last fire-and-forget store issued
	storeIssuedAddr uint64 // Address of last fire-and-forget store issued
	storeIssued     bool   // True if store already written to cache for current (PC, addr)
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
// Both cache hits and misses cause pipeline stalls based on their latencies.
func (s *CachedMemoryStage) Access(exmem *EXMEMRegister) (MemoryResult, bool) {
	result := MemoryResult{}

	if !exmem.Valid {
		// If the register is not valid, clear any pending/completed state
		s.pending = false
		s.completed = false
		return result, false
	}

	// If not a memory operation, no stall
	if !exmem.MemRead && !exmem.MemWrite {
		s.pending = false
		s.completed = false
		return result, false
	}

	addr := exmem.ALUResult

	// If PC/addr changed, this is a different memory operation - cancel pending/completed
	if s.pending && (s.pendingPC != exmem.PC || s.pendingAddr != addr) {
		s.pending = false
		s.latency = 0
		s.result = nil
	}
	if s.completed && (s.completedPC != exmem.PC || s.completedAddr != addr) {
		s.completed = false
		s.completedResult = nil
	}

	// If access already completed but pipeline hasn't advanced (another port
	// is still stalling), return the cached result without re-triggering
	// cache.Read(). This breaks the multi-port livelock.
	if s.completed {
		if s.completedResult != nil && exmem.MemRead {
			result.MemData = s.completedResult.data
		}
		return result, false
	}

	// If still waiting for previous access (hit or miss) at same address
	if s.pending {
		s.latency--
		if s.latency > 0 {
			return result, true // Still stalling
		}
		// Access complete — transition to completed state
		s.pending = false
		s.completed = true
		s.completedPC = exmem.PC
		s.completedAddr = addr
		s.completedResult = s.result
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

		// Both hits and misses have latency - set up pending state
		s.pending = true
		s.pendingPC = exmem.PC
		s.pendingAddr = addr
		s.latency = cacheResult.Latency - 1 // -1 because this cycle counts
		s.result = &memResult{data: cacheResult.Data}
		s.isHit = cacheResult.Hit

		if s.latency > 0 {
			return result, true // Stall for remaining latency
		}

		// Single-cycle latency (latency=1) — go directly to completed
		s.pending = false
		s.completed = true
		s.completedPC = exmem.PC
		s.completedAddr = addr
		s.completedResult = &memResult{data: cacheResult.Data}
		result.MemData = cacheResult.Data
		return result, false
	}

	if exmem.MemWrite {
		// Store through D-cache — fire-and-forget to the store buffer.
		// The cache is updated immediately (write-allocate) by this call,
		// and the pipeline does not stall. In real M2 hardware, a deep
		// store queue absorbs store latency to lower memory levels, so
		// the architectural effect of the store appears asynchronous.
		//
		// Idempotency: when another port's stall replays this cycle,
		// skip the duplicate cache.Write to avoid inflating stats.
		if !s.storeIssued || s.storeIssuedPC != exmem.PC || s.storeIssuedAddr != addr {
			s.cache.Write(addr, size, exmem.StoreValue)
			s.storeIssued = true
			s.storeIssuedPC = exmem.PC
			s.storeIssuedAddr = addr
		}
		s.pending = false
		return result, false
	}

	return result, false
}

// AccessSlot performs memory read or write through D-cache for any pipeline slot.
func (s *CachedMemoryStage) AccessSlot(slot MemorySlot) (MemoryResult, bool) {
	result := MemoryResult{}

	if !slot.IsValid() {
		s.pending = false
		s.completed = false
		return result, false
	}

	if !slot.GetMemRead() && !slot.GetMemWrite() {
		s.pending = false
		s.completed = false
		return result, false
	}

	addr := slot.GetALUResult()
	pc := slot.GetPC()

	if s.pending && (s.pendingPC != pc || s.pendingAddr != addr) {
		s.pending = false
		s.latency = 0
		s.result = nil
	}
	if s.completed && (s.completedPC != pc || s.completedAddr != addr) {
		s.completed = false
		s.completedResult = nil
	}

	// If access already completed but pipeline hasn't advanced (another port
	// is still stalling), return the cached result without re-triggering
	// cache.Read(). This breaks the multi-port livelock.
	if s.completed {
		if s.completedResult != nil && slot.GetMemRead() {
			result.MemData = s.completedResult.data
		}
		return result, false
	}

	if s.pending {
		s.latency--
		if s.latency > 0 {
			return result, true
		}
		// Access complete — transition to completed state
		s.pending = false
		s.completed = true
		s.completedPC = pc
		s.completedAddr = addr
		s.completedResult = s.result
		if s.result != nil && slot.GetMemRead() {
			result.MemData = s.result.data
		}
		return result, false
	}

	inst := slot.GetInst()
	size := 8
	if inst != nil && !inst.Is64Bit {
		size = 4
	}

	if slot.GetMemRead() {
		cacheResult := s.cache.Read(addr, size)
		s.pending = true
		s.pendingPC = pc
		s.pendingAddr = addr
		s.latency = cacheResult.Latency - 1
		s.result = &memResult{data: cacheResult.Data}
		s.isHit = cacheResult.Hit

		if s.latency > 0 {
			return result, true
		}
		// Single-cycle latency — go directly to completed
		s.pending = false
		s.completed = true
		s.completedPC = pc
		s.completedAddr = addr
		s.completedResult = &memResult{data: cacheResult.Data}
		result.MemData = cacheResult.Data
		return result, false
	}

	if slot.GetMemWrite() {
		// Store through D-cache — fire-and-forget to store buffer.
		// Idempotency guard: skip duplicate writes on stall replays.
		if !s.storeIssued || s.storeIssuedPC != pc || s.storeIssuedAddr != addr {
			s.cache.Write(addr, size, slot.GetStoreValue())
			s.storeIssued = true
			s.storeIssuedPC = pc
			s.storeIssuedAddr = addr
		}
		s.pending = false
		return result, false
	}

	return result, false
}

// Reset clears pending and completed state.
func (s *CachedMemoryStage) Reset() {
	s.pending = false
	s.latency = 0
	s.result = nil
	s.isHit = false
	s.completed = false
	s.completedResult = nil
	s.storeIssued = false
}

// CacheStats returns the underlying cache statistics.
func (s *CachedMemoryStage) CacheStats() cache.Statistics {
	return s.cache.Stats()
}

// CacheStats returns the underlying cache statistics.
func (s *CachedFetchStage) CacheStats() cache.Statistics {
	return s.cache.Stats()
}
