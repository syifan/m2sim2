package cache_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/sarchlab/m2sim/emu"
	"github.com/sarchlab/m2sim/timing/cache"
)

var _ = Describe("Cache", func() {
	var (
		c       *cache.Cache
		memory  *emu.Memory
		backing *cache.MemoryBacking
	)

	BeforeEach(func() {
		memory = emu.NewMemory()
		backing = cache.NewMemoryBacking(memory)
		// Small cache for testing: 4KB, 4-way, 64B lines
		config := cache.Config{
			Size:          4 * 1024,
			Associativity: 4,
			BlockSize:     64,
			HitLatency:    1,
			MissLatency:   10,
		}
		c = cache.New(config, backing)
	})

	Describe("Read operations", func() {
		It("should miss on cold cache", func() {
			// Write data to memory first
			memory.Write64(0x1000, 0xDEADBEEF)

			result := c.Read(0x1000, 8)
			Expect(result.Hit).To(BeFalse())
			Expect(result.Latency).To(Equal(uint64(10)))
			Expect(result.Data).To(Equal(uint64(0xDEADBEEF)))

			stats := c.Stats()
			Expect(stats.Reads).To(Equal(uint64(1)))
			Expect(stats.Misses).To(Equal(uint64(1)))
			Expect(stats.Hits).To(Equal(uint64(0)))
		})

		It("should hit on cached data", func() {
			memory.Write64(0x1000, 0xCAFEBABE)

			// First read - miss
			c.Read(0x1000, 8)

			// Second read - should hit
			result := c.Read(0x1000, 8)
			Expect(result.Hit).To(BeTrue())
			Expect(result.Latency).To(Equal(uint64(1)))
			Expect(result.Data).To(Equal(uint64(0xCAFEBABE)))

			stats := c.Stats()
			Expect(stats.Reads).To(Equal(uint64(2)))
			Expect(stats.Misses).To(Equal(uint64(1)))
			Expect(stats.Hits).To(Equal(uint64(1)))
		})

		It("should hit on different addresses in same cache line", func() {
			memory.Write32(0x1000, 0x11111111)
			memory.Write32(0x1004, 0x22222222)

			// First read at 0x1000 - miss, loads entire cache line
			c.Read(0x1000, 4)

			// Read at 0x1004 - should hit (same cache line)
			result := c.Read(0x1004, 4)
			Expect(result.Hit).To(BeTrue())
			Expect(result.Data).To(Equal(uint64(0x22222222)))
		})
	})

	Describe("Write operations", func() {
		It("should write-allocate on miss", func() {
			result := c.Write(0x1000, 8, 0x12345678)
			Expect(result.Hit).To(BeFalse())
			Expect(result.Latency).To(Equal(uint64(10)))

			// Subsequent read should hit
			readResult := c.Read(0x1000, 8)
			Expect(readResult.Hit).To(BeTrue())
			Expect(readResult.Data).To(Equal(uint64(0x12345678)))
		})

		It("should hit on cached data", func() {
			// First write - miss
			c.Write(0x1000, 8, 0x11111111)

			// Second write - should hit
			result := c.Write(0x1000, 8, 0x22222222)
			Expect(result.Hit).To(BeTrue())
			Expect(result.Latency).To(Equal(uint64(1)))

			// Verify data
			readResult := c.Read(0x1000, 8)
			Expect(readResult.Data).To(Equal(uint64(0x22222222)))
		})
	})

	Describe("Eviction", func() {
		It("should evict when cache is full", func() {
			// 4KB cache, 64B lines, 4-way = 16 sets
			// Fill one set completely (4 ways), then access one more
			// Set 0 addresses: 0, 1024, 2048, 3072, 4096 (all map to set 0)
			// (assuming sets = 4KB / (4 * 64) = 16 sets)

			// Fill set 0 with 4 blocks
			c.Write(0x0000, 8, 0x11111111) // Set 0, way 0
			c.Write(0x0400, 8, 0x22222222) // Set 0, way 1
			c.Write(0x0800, 8, 0x33333333) // Set 0, way 2
			c.Write(0x0C00, 8, 0x44444444) // Set 0, way 3

			// All should hit now
			Expect(c.Read(0x0000, 8).Hit).To(BeTrue())
			Expect(c.Read(0x0400, 8).Hit).To(BeTrue())
			Expect(c.Read(0x0800, 8).Hit).To(BeTrue())
			Expect(c.Read(0x0C00, 8).Hit).To(BeTrue())

			// Access 5th address in same set - should evict LRU
			result := c.Write(0x1000, 8, 0x55555555)
			Expect(result.Hit).To(BeFalse())
			Expect(result.Evicted).To(BeTrue())

			stats := c.Stats()
			Expect(stats.Evictions).To(Equal(uint64(1)))
		})

		It("should writeback dirty evicted blocks", func() {
			// Fill set 0 completely
			c.Write(0x0000, 8, 0x11111111)
			c.Write(0x0400, 8, 0x22222222)
			c.Write(0x0800, 8, 0x33333333)
			c.Write(0x0C00, 8, 0x44444444)

			// Access the first three to make 0x0000 the LRU
			c.Read(0x0400, 8)
			c.Read(0x0800, 8)
			c.Read(0x0C00, 8)

			// Evict - should write back 0x0000
			c.Write(0x1000, 8, 0x55555555)

			// Check memory was written back
			Expect(memory.Read64(0x0000)).To(Equal(uint64(0x11111111)))

			stats := c.Stats()
			Expect(stats.Writebacks).To(Equal(uint64(1)))
		})
	})

	Describe("Flush", func() {
		It("should write back all dirty blocks", func() {
			c.Write(0x0000, 8, 0x11111111)
			c.Write(0x1000, 8, 0x22222222)

			// Data not yet in memory (only in cache)
			Expect(memory.Read64(0x0000)).To(Equal(uint64(0)))
			Expect(memory.Read64(0x1000)).To(Equal(uint64(0)))

			c.Flush()

			// After flush, data should be in memory
			Expect(memory.Read64(0x0000)).To(Equal(uint64(0x11111111)))
			Expect(memory.Read64(0x1000)).To(Equal(uint64(0x22222222)))

			stats := c.Stats()
			Expect(stats.Writebacks).To(Equal(uint64(2)))
		})
	})

	Describe("Default configurations", func() {
		It("should create L1I config", func() {
			config := cache.DefaultL1IConfig()
			Expect(config.Size).To(Equal(192 * 1024))
			Expect(config.Associativity).To(Equal(6))
			Expect(config.BlockSize).To(Equal(64))
		})

		It("should create L1D config", func() {
			config := cache.DefaultL1DConfig()
			Expect(config.Size).To(Equal(128 * 1024))
			Expect(config.Associativity).To(Equal(8))
			Expect(config.BlockSize).To(Equal(64))
		})
	})
})
