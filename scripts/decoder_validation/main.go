// Validate decoder optimization - measures allocation improvements in decoder
package main

import (
	"fmt"
	"runtime"
	"time"

	"github.com/sarchlab/m2sim/emu"
	"github.com/sarchlab/m2sim/timing/pipeline"
)

func main() {
	// Create basic memory and regfile for decoder testing
	memory := emu.NewMemory()
	regFile := &emu.RegFile{}

	// Initialize with some test data
	memory.Write32(0x1000, 0x91002820) // ADD X0, X1, #42
	memory.Write32(0x1004, 0xB1002862) // ADDS W2, W3, #42
	memory.Write32(0x1008, 0x8B020020) // ADD X0, X1, X2
	memory.Write32(0x100C, 0xF1001549) // SUBS X9, X10, #5
	memory.Write32(0x1010, 0x0B050083) // ADD W3, W4, W5

	decodeStage := pipeline.NewDecodeStage(regFile)

	// Warm up
	for i := 0; i < 1000; i++ {
		decodeStage.Decode(0x91002820, 0x1000)
	}

	// Measure allocations before optimization test
	runtime.GC()
	var m1, m2 runtime.MemStats
	runtime.ReadMemStats(&m1)

	start := time.Now()
	iterations := 100000

	// Test decode performance - simulate superscalar decode (multiple calls per cycle)
	for i := 0; i < iterations; i++ {
		// Simulate 4-wide superscalar decode (4 instructions per cycle)
		decodeStage.Decode(0x91002820, 0x1000) // ADD X0, X1, #42
		decodeStage.Decode(0xB1002862, 0x1004) // ADDS W2, W3, #42
		decodeStage.Decode(0x8B020020, 0x1008) // ADD X0, X1, X2
		decodeStage.Decode(0xF1001549, 0x100C) // SUBS X9, X10, #5
	}

	elapsed := time.Since(start)
	runtime.ReadMemStats(&m2)

	totalDecodes := iterations * 4
	allocations := m2.Mallocs - m1.Mallocs
	allocatedBytes := m2.TotalAlloc - m1.TotalAlloc

	fmt.Printf("Decoder Optimization Validation Results:\n")
	fmt.Printf("========================================\n")
	fmt.Printf("Total decode operations: %d\n", totalDecodes)
	fmt.Printf("Time elapsed: %v\n", elapsed)
	fmt.Printf("Decodes per second: %.0f\n", float64(totalDecodes)/elapsed.Seconds())
	fmt.Printf("Allocations: %d\n", allocations)
	fmt.Printf("Allocated bytes: %d\n", allocatedBytes)
	fmt.Printf("Allocations per decode: %.3f\n", float64(allocations)/float64(totalDecodes))
	fmt.Printf("Bytes per decode: %.1f\n", float64(allocatedBytes)/float64(totalDecodes))

	if allocations == 0 {
		fmt.Printf("\n✅ SUCCESS: Zero allocations detected! Optimization effective.\n")
	} else if float64(allocations)/float64(totalDecodes) < 0.1 {
		fmt.Printf("\n✅ GOOD: Low allocation rate (< 0.1 per decode)\n")
	} else {
		fmt.Printf("\n⚠️  WARNING: High allocation rate detected\n")
	}
}
