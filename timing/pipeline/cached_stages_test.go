package pipeline_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/sarchlab/m2sim/emu"
	"github.com/sarchlab/m2sim/insts"
	"github.com/sarchlab/m2sim/timing/cache"
	"github.com/sarchlab/m2sim/timing/pipeline"
)

var _ = Describe("CachedFetchStage", func() {
	var (
		fetchStage *pipeline.CachedFetchStage
		icache     *cache.Cache
		memory     *emu.Memory
	)

	BeforeEach(func() {
		memory = emu.NewMemory()
		backing := cache.NewMemoryBacking(memory)
		// Small cache for testing: 4KB, 4-way, 64B lines
		config := cache.Config{
			Size:          4 * 1024,
			Associativity: 4,
			BlockSize:     64,
			HitLatency:    1,
			MissLatency:   10,
		}
		icache = cache.New(config, backing)
		fetchStage = pipeline.NewCachedFetchStage(icache, memory)
	})

	Describe("Cache miss behavior", func() {
		It("should stall on cold cache miss", func() {
			// Write instruction to memory
			memory.Write32(0x1000, 0x91002820)

			// First fetch - should stall
			word, ok, stall := fetchStage.Fetch(0x1000)
			Expect(stall).To(BeTrue())
			Expect(ok).To(BeFalse())
			Expect(word).To(Equal(uint32(0)))
		})

		It("should complete fetch after miss latency cycles", func() {
			memory.Write32(0x1000, 0x91002820)

			// First fetch - stall
			fetchStage.Fetch(0x1000)

			// Wait out the miss latency (10-1=9 more cycles)
			for i := 0; i < 8; i++ {
				_, ok, stall := fetchStage.Fetch(0x1000)
				Expect(stall).To(BeTrue(), "Should still be stalling at cycle %d", i)
				Expect(ok).To(BeFalse())
			}

			// Final cycle should complete
			word, ok, stall := fetchStage.Fetch(0x1000)
			Expect(stall).To(BeFalse())
			Expect(ok).To(BeTrue())
			Expect(word).To(Equal(uint32(0x91002820)))
		})
	})

	Describe("Cache hit behavior", func() {
		It("should hit after data is cached", func() {
			memory.Write32(0x1000, 0x91002820)

			// First fetch - miss, wait for completion
			for i := 0; i < 10; i++ {
				fetchStage.Fetch(0x1000)
			}

			// Second fetch - should hit immediately
			word, ok, stall := fetchStage.Fetch(0x1000)
			Expect(stall).To(BeFalse())
			Expect(ok).To(BeTrue())
			Expect(word).To(Equal(uint32(0x91002820)))
		})

		It("should hit on nearby address in same cache line", func() {
			memory.Write32(0x1000, 0x11111111)
			memory.Write32(0x1004, 0x22222222)

			// Fetch first address - miss, wait for completion
			for i := 0; i < 10; i++ {
				fetchStage.Fetch(0x1000)
			}

			// Fetch nearby address - should hit (same cache line)
			word, ok, stall := fetchStage.Fetch(0x1004)
			Expect(stall).To(BeFalse())
			Expect(ok).To(BeTrue())
			Expect(word).To(Equal(uint32(0x22222222)))
		})
	})

	Describe("PC change behavior", func() {
		It("should cancel pending request when PC changes", func() {
			memory.Write32(0x1000, 0x11111111)
			memory.Write32(0x2000, 0x22222222)

			// Start fetch at 0x1000 - stall
			fetchStage.Fetch(0x1000)

			// PC changes (e.g., branch taken) - fetch at 0x2000
			_, ok, stall := fetchStage.Fetch(0x2000)
			Expect(stall).To(BeTrue()) // New miss at 0x2000
			Expect(ok).To(BeFalse())
		})
	})

	Describe("Reset", func() {
		It("should clear pending state on reset", func() {
			memory.Write32(0x1000, 0x91111111)
			memory.Write32(0x2000, 0x22222222)

			// Start a fetch at 0x1000 - stall (starts miss)
			fetchStage.Fetch(0x1000)

			// Reset clears pending state
			fetchStage.Reset()

			// Note: The cache line at 0x1000 was loaded during the first access
			// (cache loads data immediately, stage just models latency)
			// So accessing 0x1000 again would hit.

			// To verify Reset works, access a NEW uncached address
			_, ok, stall := fetchStage.Fetch(0x2000)
			Expect(stall).To(BeTrue(), "Should miss on new address")
			Expect(ok).To(BeFalse())
		})

		It("should allow new miss to start after reset during pending", func() {
			memory.Write32(0x1000, 0x91002820)

			// Start a fetch - stall
			fetchStage.Fetch(0x1000)

			// Wait partway through
			fetchStage.Fetch(0x1000)
			fetchStage.Fetch(0x1000)

			// Reset during pending
			fetchStage.Reset()

			// Same address access after reset - data is in cache, should hit
			_, ok, stall := fetchStage.Fetch(0x1000)
			Expect(stall).To(BeFalse(), "Cache line was loaded, should hit")
			Expect(ok).To(BeTrue())
		})
	})

	Describe("Statistics", func() {
		It("should track cache statistics", func() {
			memory.Write32(0x1000, 0x91002820)

			// Miss
			for i := 0; i < 10; i++ {
				fetchStage.Fetch(0x1000)
			}

			// Hit
			fetchStage.Fetch(0x1000)

			stats := fetchStage.CacheStats()
			Expect(stats.Reads).To(BeNumerically(">", 0))
			Expect(stats.Misses).To(BeNumerically(">", 0))
			Expect(stats.Hits).To(BeNumerically(">", 0))
		})
	})
})

var _ = Describe("CachedMemoryStage", func() {
	var (
		memStage *pipeline.CachedMemoryStage
		dcache   *cache.Cache
		memory   *emu.Memory
	)

	BeforeEach(func() {
		memory = emu.NewMemory()
		backing := cache.NewMemoryBacking(memory)
		// Small cache for testing: 4KB, 4-way, 64B lines
		config := cache.Config{
			Size:          4 * 1024,
			Associativity: 4,
			BlockSize:     64,
			HitLatency:    1,
			MissLatency:   10,
		}
		dcache = cache.New(config, backing)
		memStage = pipeline.NewCachedMemoryStage(dcache, memory)
	})

	Describe("Load operations", func() {
		Context("Cache miss", func() {
			It("should stall on cold cache miss for 64-bit load", func() {
				memory.Write64(0x2000, 0x123456789ABCDEF0)

				exmem := &pipeline.EXMEMRegister{
					Valid:     true,
					PC:        0x1000,
					ALUResult: 0x2000,
					MemRead:   true,
					Inst:      &insts.Instruction{Is64Bit: true},
				}

				result, stall := memStage.Access(exmem)
				Expect(stall).To(BeTrue())
				Expect(result.MemData).To(Equal(uint64(0)))
			})

			It("should complete load after miss latency cycles", func() {
				memory.Write64(0x2000, 0xDEADBEEFCAFEBABE)

				exmem := &pipeline.EXMEMRegister{
					Valid:     true,
					PC:        0x1000,
					ALUResult: 0x2000,
					MemRead:   true,
					Inst:      &insts.Instruction{Is64Bit: true},
				}

				// First access - stall
				memStage.Access(exmem)

				// Wait out the miss latency (10-1=9 more cycles)
				for i := 0; i < 8; i++ {
					_, stall := memStage.Access(exmem)
					Expect(stall).To(BeTrue(), "Should still be stalling at cycle %d", i)
				}

				// Final cycle should complete
				result, stall := memStage.Access(exmem)
				Expect(stall).To(BeFalse())
				Expect(result.MemData).To(Equal(uint64(0xDEADBEEFCAFEBABE)))
			})

			It("should stall on cold cache miss for 32-bit load", func() {
				memory.Write32(0x2000, 0xDEADBEEF)

				exmem := &pipeline.EXMEMRegister{
					Valid:     true,
					PC:        0x1000,
					ALUResult: 0x2000,
					MemRead:   true,
					Inst:      &insts.Instruction{Is64Bit: false},
				}

				_, stall := memStage.Access(exmem)
				Expect(stall).To(BeTrue())
			})
		})

		Context("Cache hit", func() {
			It("should hit after data is cached", func() {
				memory.Write64(0x2000, 0x123456789ABCDEF0)

				exmem := &pipeline.EXMEMRegister{
					Valid:     true,
					PC:        0x1000,
					ALUResult: 0x2000,
					MemRead:   true,
					Inst:      &insts.Instruction{Is64Bit: true},
				}

				// First access - miss, wait for completion
				for i := 0; i < 10; i++ {
					memStage.Access(exmem)
				}

				// Reset pending state by changing something
				memStage.Reset()

				// Second access - should hit immediately
				result, stall := memStage.Access(exmem)
				Expect(stall).To(BeFalse())
				Expect(result.MemData).To(Equal(uint64(0x123456789ABCDEF0)))
			})

			It("should hit on nearby address in same cache line", func() {
				memory.Write64(0x2000, 0x1111111111111111)
				memory.Write64(0x2008, 0x2222222222222222)

				exmem := &pipeline.EXMEMRegister{
					Valid:     true,
					PC:        0x1000,
					ALUResult: 0x2000,
					MemRead:   true,
					Inst:      &insts.Instruction{Is64Bit: true},
				}

				// Load first address - miss
				for i := 0; i < 10; i++ {
					memStage.Access(exmem)
				}

				memStage.Reset()

				// Load nearby address - should hit
				exmem2 := &pipeline.EXMEMRegister{
					Valid:     true,
					PC:        0x1004,
					ALUResult: 0x2008,
					MemRead:   true,
					Inst:      &insts.Instruction{Is64Bit: true},
				}

				result, stall := memStage.Access(exmem2)
				Expect(stall).To(BeFalse())
				Expect(result.MemData).To(Equal(uint64(0x2222222222222222)))
			})
		})
	})

	Describe("Store operations", func() {
		Context("Store buffer model (fire-and-forget)", func() {
			It("should not stall on store miss (store buffer absorbs latency)", func() {
				exmem := &pipeline.EXMEMRegister{
					Valid:      true,
					PC:         0x1000,
					ALUResult:  0x3000,
					StoreValue: 0xCAFEBABE12345678,
					MemWrite:   true,
					Inst:       &insts.Instruction{Is64Bit: true},
				}

				_, stall := memStage.Access(exmem)
				Expect(stall).To(BeFalse(),
					"Stores should be fire-and-forget (store buffer)")
			})

			It("should update cache immediately so subsequent read hits", func() {
				exmem := &pipeline.EXMEMRegister{
					Valid:      true,
					PC:         0x1000,
					ALUResult:  0x3000,
					StoreValue: 0xCAFEBABE12345678,
					MemWrite:   true,
					Inst:       &insts.Instruction{Is64Bit: true},
				}

				// Store completes in 1 cycle (no stall)
				_, stall := memStage.Access(exmem)
				Expect(stall).To(BeFalse())

				// Verify data was written to cache (subsequent read should hit)
				memStage.Reset()
				readExmem := &pipeline.EXMEMRegister{
					Valid:     true,
					PC:        0x1004,
					ALUResult: 0x3000,
					MemRead:   true,
					Inst:      &insts.Instruction{Is64Bit: true},
				}
				// Drain store-to-load forwarding stall cycles
				stallCycles := 0
				for {
					result, stall := memStage.Access(readExmem)
					if !stall {
						Expect(result.MemData).To(Equal(uint64(0xCAFEBABE12345678)))
						break
					}
					stallCycles++
				}
				Expect(uint64(stallCycles)).To(Equal(cache.StoreForwardLatency),
					"Should stall for exactly StoreForwardLatency cycles")
			})
		})

		Context("Cache hit", func() {
			It("should hit on store to cached address", func() {
				// First, load the address to get it into cache
				memory.Write64(0x3000, 0x0)

				loadExmem := &pipeline.EXMEMRegister{
					Valid:     true,
					PC:        0x1000,
					ALUResult: 0x3000,
					MemRead:   true,
					Inst:      &insts.Instruction{Is64Bit: true},
				}
				for i := 0; i < 10; i++ {
					memStage.Access(loadExmem)
				}
				memStage.Reset()

				// Now store should hit
				storeExmem := &pipeline.EXMEMRegister{
					Valid:      true,
					PC:         0x1004,
					ALUResult:  0x3000,
					StoreValue: 0xDEADBEEF,
					MemWrite:   true,
					Inst:       &insts.Instruction{Is64Bit: true},
				}

				_, stall := memStage.Access(storeExmem)
				Expect(stall).To(BeFalse())
			})
		})
	})

	Describe("Address change behavior", func() {
		It("should cancel pending request when address changes", func() {
			memory.Write64(0x2000, 0x1111111111111111)
			memory.Write64(0x4000, 0x2222222222222222)

			exmem := &pipeline.EXMEMRegister{
				Valid:     true,
				PC:        0x1000,
				ALUResult: 0x2000,
				MemRead:   true,
				Inst:      &insts.Instruction{Is64Bit: true},
			}

			// Start load - stall
			memStage.Access(exmem)

			// Address changes (different instruction)
			exmem2 := &pipeline.EXMEMRegister{
				Valid:     true,
				PC:        0x1004,
				ALUResult: 0x4000,
				MemRead:   true,
				Inst:      &insts.Instruction{Is64Bit: true},
			}

			_, stall := memStage.Access(exmem2)
			Expect(stall).To(BeTrue()) // New miss at 0x4000
		})

		It("should cancel pending request when PC changes", func() {
			memory.Write64(0x2000, 0x1111111111111111)
			memory.Write64(0x4000, 0x3333333333333333)

			exmem := &pipeline.EXMEMRegister{
				Valid:     true,
				PC:        0x1000,
				ALUResult: 0x2000,
				MemRead:   true,
				Inst:      &insts.Instruction{Is64Bit: true},
			}

			// Start load - stall
			memStage.Access(exmem)

			// PC changes to access a DIFFERENT uncached address
			// (same address would hit since cache line was loaded)
			exmem2 := &pipeline.EXMEMRegister{
				Valid:     true,
				PC:        0x1008, // Different PC
				ALUResult: 0x4000, // Different address
				MemRead:   true,
				Inst:      &insts.Instruction{Is64Bit: true},
			}

			_, stall := memStage.Access(exmem2)
			Expect(stall).To(BeTrue()) // Restart miss at new address
		})

		It("should hit after PC change if cache line already loaded", func() {
			memory.Write64(0x2000, 0x1111111111111111)

			exmem := &pipeline.EXMEMRegister{
				Valid:     true,
				PC:        0x1000,
				ALUResult: 0x2000,
				MemRead:   true,
				Inst:      &insts.Instruction{Is64Bit: true},
			}

			// Start load - this loads the cache line
			memStage.Access(exmem)

			// PC changes but same address - should hit since data is cached
			exmem2 := &pipeline.EXMEMRegister{
				Valid:     true,
				PC:        0x1008, // Different PC
				ALUResult: 0x2000, // Same address
				MemRead:   true,
				Inst:      &insts.Instruction{Is64Bit: true},
			}

			result, stall := memStage.Access(exmem2)
			Expect(stall).To(BeFalse(), "Cache line was already loaded")
			Expect(result.MemData).To(Equal(uint64(0x1111111111111111)))
		})
	})

	Describe("Non-memory operations", func() {
		It("should not stall for ALU instructions", func() {
			exmem := &pipeline.EXMEMRegister{
				Valid:     true,
				PC:        0x1000,
				ALUResult: 150,
				MemRead:   false,
				MemWrite:  false,
			}

			result, stall := memStage.Access(exmem)
			Expect(stall).To(BeFalse())
			Expect(result.MemData).To(Equal(uint64(0)))
		})

		It("should not stall for invalid register", func() {
			exmem := &pipeline.EXMEMRegister{
				Valid: false,
			}

			_, stall := memStage.Access(exmem)
			Expect(stall).To(BeFalse())
		})
	})

	Describe("Reset", func() {
		It("should clear pending state on reset", func() {
			memory.Write64(0x2000, 0x123456789ABCDEF0)
			memory.Write64(0x4000, 0xAAAAAAAAAAAAAAAA)

			exmem := &pipeline.EXMEMRegister{
				Valid:     true,
				PC:        0x1000,
				ALUResult: 0x2000,
				MemRead:   true,
				Inst:      &insts.Instruction{Is64Bit: true},
			}

			// Start a load - stall (this loads cache line at 0x2000)
			memStage.Access(exmem)

			// Reset clears pending state
			memStage.Reset()

			// Access a new uncached address to verify Reset allows new operations
			exmem2 := &pipeline.EXMEMRegister{
				Valid:     true,
				PC:        0x1004,
				ALUResult: 0x4000, // New address
				MemRead:   true,
				Inst:      &insts.Instruction{Is64Bit: true},
			}
			_, stall := memStage.Access(exmem2)
			Expect(stall).To(BeTrue(), "Should miss on new uncached address")
		})

		It("should hit after reset if cache line already loaded", func() {
			memory.Write64(0x2000, 0x123456789ABCDEF0)

			exmem := &pipeline.EXMEMRegister{
				Valid:     true,
				PC:        0x1000,
				ALUResult: 0x2000,
				MemRead:   true,
				Inst:      &insts.Instruction{Is64Bit: true},
			}

			// Start a load - this loads the cache line
			memStage.Access(exmem)

			// Reset
			memStage.Reset()

			// Same address should hit (cache line was loaded)
			result, stall := memStage.Access(exmem)
			Expect(stall).To(BeFalse(), "Cache line was already loaded")
			Expect(result.MemData).To(Equal(uint64(0x123456789ABCDEF0)))
		})
	})

	Describe("Statistics", func() {
		It("should track cache statistics for loads", func() {
			memory.Write64(0x2000, 0x123456789ABCDEF0)

			exmem := &pipeline.EXMEMRegister{
				Valid:     true,
				PC:        0x1000,
				ALUResult: 0x2000,
				MemRead:   true,
				Inst:      &insts.Instruction{Is64Bit: true},
			}

			// Miss
			for i := 0; i < 10; i++ {
				memStage.Access(exmem)
			}

			memStage.Reset()

			// Hit
			memStage.Access(exmem)

			stats := memStage.CacheStats()
			Expect(stats.Reads).To(BeNumerically(">", 0))
			Expect(stats.Misses).To(BeNumerically(">", 0))
			Expect(stats.Hits).To(BeNumerically(">", 0))
		})

		It("should track cache statistics for stores", func() {
			exmem := &pipeline.EXMEMRegister{
				Valid:      true,
				PC:         0x1000,
				ALUResult:  0x3000,
				StoreValue: 0xCAFEBABE,
				MemWrite:   true,
				Inst:       &insts.Instruction{Is64Bit: true},
			}

			// First store (miss)
			memStage.Access(exmem)
			memStage.Reset()

			// Second store to same address with different PC (hit)
			exmem.PC = 0x1004
			memStage.Access(exmem)

			stats := memStage.CacheStats()
			Expect(stats.Writes).To(BeNumerically(">", 0))
		})

		It("should issue store write only once on repeated calls (idempotency)", func() {
			exmem := &pipeline.EXMEMRegister{
				Valid:      true,
				PC:         0x1000,
				ALUResult:  0x3000,
				StoreValue: 0xCAFEBABE,
				MemWrite:   true,
				Inst:       &insts.Instruction{Is64Bit: true},
			}

			// Call Access 5 times with identical store (simulating stall replay)
			for i := 0; i < 5; i++ {
				_, stall := memStage.Access(exmem)
				Expect(stall).To(BeFalse())
			}

			// Only 1 write should have been issued to the cache
			stats := memStage.CacheStats()
			Expect(stats.Writes).To(Equal(uint64(1)),
				"Repeated store with same PC/addr should only write once")
		})
	})
})

var _ = Describe("CachedMemoryStage Integration", func() {
	var (
		memStage *pipeline.CachedMemoryStage
		dcache   *cache.Cache
		memory   *emu.Memory
	)

	BeforeEach(func() {
		memory = emu.NewMemory()
		backing := cache.NewMemoryBacking(memory)
		// Use L1D-like configuration
		config := cache.Config{
			Size:          128 * 1024,
			Associativity: 8,
			BlockSize:     64,
			HitLatency:    1,
			MissLatency:   10,
		}
		dcache = cache.New(config, backing)
		memStage = pipeline.NewCachedMemoryStage(dcache, memory)
	})

	Describe("Cache coherence", func() {
		It("should read back stored data", func() {
			// Store data — fire-and-forget, completes in 1 cycle
			storeExmem := &pipeline.EXMEMRegister{
				Valid:      true,
				PC:         0x1000,
				ALUResult:  0x5000,
				StoreValue: 0xABCDEF0123456789,
				MemWrite:   true,
				Inst:       &insts.Instruction{Is64Bit: true},
			}

			_, stall := memStage.Access(storeExmem)
			Expect(stall).To(BeFalse())
			memStage.Reset()

			// Read it back
			loadExmem := &pipeline.EXMEMRegister{
				Valid:     true,
				PC:        0x1004,
				ALUResult: 0x5000,
				MemRead:   true,
				Inst:      &insts.Instruction{Is64Bit: true},
			}

			// Drain store-to-load forwarding stall cycles
			stallCycles := 0
			for {
				result, stallAgain := memStage.Access(loadExmem)
				if !stallAgain {
					Expect(result.MemData).To(Equal(uint64(0xABCDEF0123456789)))
					break
				}
				stallCycles++
			}
			Expect(uint64(stallCycles)).To(Equal(cache.StoreForwardLatency),
				"Should stall for exactly StoreForwardLatency cycles")
		})
	})

	Describe("Timing verification", func() {
		It("should have correct stall cycles for miss", func() {
			memory.Write64(0x2000, 0xDEADBEEF)

			exmem := &pipeline.EXMEMRegister{
				Valid:     true,
				PC:        0x1000,
				ALUResult: 0x2000,
				MemRead:   true,
				Inst:      &insts.Instruction{Is64Bit: true},
			}

			stallCycles := 0
			for {
				_, stall := memStage.Access(exmem)
				if !stall {
					break
				}
				stallCycles++
				if stallCycles > 100 {
					Fail("Too many stall cycles")
				}
			}

			// Miss latency is 10, first cycle initiates, so 9 stall cycles
			Expect(stallCycles).To(Equal(9))
		})

		It("should have zero stall cycles for hit", func() {
			memory.Write64(0x2000, 0xCAFEBABE)

			exmem := &pipeline.EXMEMRegister{
				Valid:     true,
				PC:        0x1000,
				ALUResult: 0x2000,
				MemRead:   true,
				Inst:      &insts.Instruction{Is64Bit: true},
			}

			// First: miss
			for i := 0; i < 10; i++ {
				memStage.Access(exmem)
			}
			memStage.Reset()

			// Second: should hit with no stall
			exmem.PC = 0x1004
			_, stall := memStage.Access(exmem)
			Expect(stall).To(BeFalse())
		})
	})
})
