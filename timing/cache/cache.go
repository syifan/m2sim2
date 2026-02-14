// Package cache provides cache hierarchy modeling using Akita cache components.
package cache

import (
	akitacache "github.com/sarchlab/akita/v4/mem/cache"
)

// Config holds cache configuration parameters.
type Config struct {
	// Size in bytes
	Size int
	// Associativity (number of ways)
	Associativity int
	// BlockSize in bytes (cache line size)
	BlockSize int
	// HitLatency in cycles
	HitLatency uint64
	// MissLatency in cycles (includes memory access time)
	MissLatency uint64
}

// DefaultL1IConfig returns default configuration for L1 instruction cache.
// Based on Apple M2 specifications:
// - 192KB per performance core (6-way, 64B line)
// - 128KB per efficiency core (4-way, 64B line)
func DefaultL1IConfig() Config {
	return Config{
		Size:          192 * 1024, // 192KB
		Associativity: 6,          // 6-way
		BlockSize:     64,         // 64B cache line
		HitLatency:    1,          // 1 cycle
		MissLatency:   12,         // ~12 cycles to L2
	}
}

// DefaultL1DConfig returns default configuration for L1 data cache.
// Based on Apple M2 specifications:
// - 128KB per performance core (8-way, 64B line)
// - 64KB per efficiency core (4-way, 64B line)
// - 4-cycle load-to-use latency
func DefaultL1DConfig() Config {
	return Config{
		Size:          128 * 1024, // 128KB
		Associativity: 8,          // 8-way
		BlockSize:     64,         // 64B cache line
		HitLatency:    3,          // 3-cycle load-to-use latency (M2)
		MissLatency:   12,         // ~12 cycles to L2
	}
}

// DefaultL2Config returns default configuration for unified L2 cache.
// Based on Apple M2 specifications:
// - 24MB shared L2 (entire chip)
// - 16-way set associative
// - 128B cache line
// - ~12-14 cycle latency
// - Unified memory architecture has lower latency than typical DRAM
func DefaultL2Config() Config {
	return Config{
		Size:          24 * 1024 * 1024, // 24MB (M2 spec)
		Associativity: 16,               // 16-way
		BlockSize:     128,              // 128B cache line
		HitLatency:    12,               // ~12 cycles
		MissLatency:   150,              // ~150 cycles (unified memory is faster than typical DRAM)
	}
}

// DefaultL2PerCoreConfig returns L2 configuration for per-core L2 setups.
// Useful for simulating systems with private L2 per core.
func DefaultL2PerCoreConfig() Config {
	return Config{
		Size:          512 * 1024, // 512KB per core
		Associativity: 8,          // 8-way
		BlockSize:     128,        // 128B cache line
		HitLatency:    12,         // ~12 cycles
		MissLatency:   150,        // ~150 cycles (unified memory)
	}
}

// AccessResult contains the result of a cache access.
type AccessResult struct {
	// Hit indicates whether the access was a cache hit.
	Hit bool
	// Latency is the number of cycles this access takes.
	Latency uint64
	// Data is the data read (for load operations).
	Data uint64
	// Evicted is true if a dirty block was evicted.
	Evicted bool
	// EvictedAddr is the address of the evicted block (if Evicted is true).
	EvictedAddr uint64
}

// StoreForwardLatency is the extra latency (in cycles) when a load must
// forward data from a recent store to the same cache line. On Apple M2,
// the store-to-load forwarding path through the store queue adds latency
// compared to a normal L1 hit because the data must be checked against
// pending stores in the store buffer.
const StoreForwardLatency uint64 = 1

// Cache represents an L1 cache using Akita cache components.
type Cache struct {
	// Configuration
	config Config

	// Akita cache directory for tag/state management
	directory *akitacache.DirectoryImpl

	// Data storage - indexed by (setID * associativity + wayID)
	dataStore [][]byte

	// Statistics
	stats Statistics

	// Backing store interface (for fetching on miss and writeback)
	backing BackingStore

	// Store buffer tracking for store-to-load forwarding detection.
	// When a store writes to an address, we record it. A subsequent load
	// to the same address incurs extra forwarding latency.
	recentStoreAddr  uint64
	recentStoreValid bool
}

// Statistics holds cache performance statistics.
type Statistics struct {
	Reads      uint64
	Writes     uint64
	Hits       uint64
	Misses     uint64
	Evictions  uint64
	Writebacks uint64
}

// BackingStore interface for the next level in the memory hierarchy.
type BackingStore interface {
	// Read fetches data from the backing store.
	Read(addr uint64, size int) []byte
	// Write stores data to the backing store.
	Write(addr uint64, data []byte)
}

// New creates a new cache with the given configuration.
func New(config Config, backing BackingStore) *Cache {
	numSets := config.Size / (config.Associativity * config.BlockSize)
	totalBlocks := numSets * config.Associativity

	// Initialize data storage
	dataStore := make([][]byte, totalBlocks)
	for i := range dataStore {
		dataStore[i] = make([]byte, config.BlockSize)
	}

	return &Cache{
		config: config,
		directory: akitacache.NewDirectory(
			numSets,
			config.Associativity,
			config.BlockSize,
			akitacache.NewLRUVictimFinder(),
		),
		dataStore: dataStore,
		backing:   backing,
	}
}

// Config returns the cache configuration.
func (c *Cache) Config() Config {
	return c.config
}

// Stats returns cache statistics.
func (c *Cache) Stats() Statistics {
	return c.stats
}

// ResetStats clears cache statistics.
func (c *Cache) ResetStats() {
	c.stats = Statistics{}
}

// blockIndex computes the index into dataStore for a block.
func (c *Cache) blockIndex(block *akitacache.Block) int {
	return block.SetID*c.config.Associativity + block.WayID
}

// Read performs a cache read operation.
// Returns the access result including hit/miss and latency.
func (c *Cache) Read(addr uint64, size int) AccessResult {
	c.stats.Reads++

	// Compute block-aligned address for lookup
	blockAddr := (addr / uint64(c.config.BlockSize)) * uint64(c.config.BlockSize)

	// Look up in directory using block-aligned address
	block := c.directory.Lookup(0, blockAddr) // PID=0 for now

	if block != nil && block.IsValid {
		// Cache hit
		c.stats.Hits++
		c.directory.Visit(block) // Update LRU

		// Extract data from the block
		offset := addr % uint64(c.config.BlockSize)
		blockData := c.dataStore[c.blockIndex(block)]
		data := extractData(blockData, offset, size)

		latency := c.config.HitLatency
		// Store-to-load forwarding: when a load reads from an address
		// that was recently stored, the data must be forwarded from the
		// store buffer. This adds extra latency over a normal cache hit.
		if c.recentStoreValid && c.recentStoreAddr == addr {
			latency += StoreForwardLatency
			c.recentStoreValid = false // Consume the forwarding event
		}

		return AccessResult{
			Hit:     true,
			Latency: latency,
			Data:    data,
		}
	}

	// Cache miss
	c.stats.Misses++
	return c.handleMiss(addr, size, false, 0)
}

// Write performs a cache write operation.
// Uses write-allocate policy: on miss, fetch the block first, then write.
func (c *Cache) Write(addr uint64, size int, data uint64) AccessResult {
	c.stats.Writes++

	// Track this store address for store-to-load forwarding detection
	c.recentStoreAddr = addr
	c.recentStoreValid = true

	// Compute block-aligned address for lookup
	blockAddr := (addr / uint64(c.config.BlockSize)) * uint64(c.config.BlockSize)

	// Look up in directory using block-aligned address
	block := c.directory.Lookup(0, blockAddr)

	if block != nil && block.IsValid {
		// Cache hit
		c.stats.Hits++
		c.directory.Visit(block) // Update LRU

		// Write data to the block
		offset := addr % uint64(c.config.BlockSize)
		blockData := c.dataStore[c.blockIndex(block)]
		storeData(blockData, offset, size, data)
		block.IsDirty = true

		return AccessResult{
			Hit:     true,
			Latency: c.config.HitLatency,
		}
	}

	// Cache miss - write-allocate: fetch block, then write
	c.stats.Misses++
	return c.handleMiss(addr, size, true, data)
}

// handleMiss handles a cache miss by fetching from backing store.
func (c *Cache) handleMiss(addr uint64, size int, isWrite bool, writeData uint64) AccessResult {
	result := AccessResult{
		Hit:     false,
		Latency: c.config.MissLatency,
	}

	// Compute block-aligned address
	blockAddr := (addr / uint64(c.config.BlockSize)) * uint64(c.config.BlockSize)

	// Find victim block
	victim := c.directory.FindVictim(blockAddr)
	if victim == nil {
		// This shouldn't happen with proper directory setup
		return result
	}

	victimData := c.dataStore[c.blockIndex(victim)]

	// Check if we need to evict
	if victim.IsValid {
		c.stats.Evictions++
		result.Evicted = true
		result.EvictedAddr = victim.Tag // Tag stores block-aligned address

		// Writeback if dirty
		if victim.IsDirty && c.backing != nil {
			c.stats.Writebacks++
			c.backing.Write(victim.Tag, victimData)
		}
	}

	// Fetch from backing store
	if c.backing != nil {
		newData := c.backing.Read(blockAddr, c.config.BlockSize)
		copy(victimData, newData)
	} else {
		// Initialize to zeros if no backing store
		for i := range victimData {
			victimData[i] = 0
		}
	}

	// Update block metadata - store block-aligned address as tag
	victim.Tag = blockAddr
	victim.IsValid = true
	victim.IsDirty = false

	if isWrite {
		// Write data to the newly fetched block
		offset := addr % uint64(c.config.BlockSize)
		storeData(victimData, offset, size, writeData)
		victim.IsDirty = true
	} else {
		// Extract read data
		offset := addr % uint64(c.config.BlockSize)
		result.Data = extractData(victimData, offset, size)
	}

	c.directory.Visit(victim) // Update LRU

	return result
}

// Invalidate marks a cache line as invalid.
func (c *Cache) Invalidate(addr uint64) {
	blockAddr := (addr / uint64(c.config.BlockSize)) * uint64(c.config.BlockSize)
	block := c.directory.Lookup(0, blockAddr)
	if block != nil && block.IsValid {
		block.IsValid = false
		block.IsDirty = false
	}
}

// Flush writes back all dirty blocks and invalidates them.
func (c *Cache) Flush() {
	sets := c.directory.GetSets()
	for _, set := range sets {
		for _, block := range set.Blocks {
			if block.IsValid && block.IsDirty && c.backing != nil {
				// Tag stores block-aligned address directly
				blockData := c.dataStore[c.blockIndex(block)]
				c.backing.Write(block.Tag, blockData)
				c.stats.Writebacks++
			}
			block.IsValid = false
			block.IsDirty = false
		}
	}
}

// Reset invalidates all cache lines without writeback.
func (c *Cache) Reset() {
	c.directory.Reset()
	c.stats = Statistics{}
	c.recentStoreValid = false
	c.recentStoreAddr = 0
}

// extractData extracts a value of the given size from a byte slice.
func extractData(data []byte, offset uint64, size int) uint64 {
	if data == nil || int(offset)+size > len(data) {
		return 0
	}

	var result uint64
	for i := 0; i < size; i++ {
		result |= uint64(data[int(offset)+i]) << (i * 8)
	}
	return result
}

// storeData stores a value of the given size into a byte slice.
func storeData(data []byte, offset uint64, size int, value uint64) {
	if data == nil || int(offset)+size > len(data) {
		return
	}

	for i := 0; i < size; i++ {
		data[int(offset)+i] = byte(value >> (i * 8))
	}
}
