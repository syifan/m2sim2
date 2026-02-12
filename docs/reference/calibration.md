# M2Sim Calibration Reference

This document catalogs all timing parameters in M2Sim's timing model. Use this as a reference for calibration against real Apple M2 hardware.

## Overview

M2Sim models a 5-stage pipeline with:
- Fetch (IF) → Decode (ID) → Execute (EX) → Memory (MEM) → Writeback (WB)
- L1 instruction and data caches
- Configurable instruction latencies
- Hazard detection with forwarding and stalls

---

## Instruction Latencies

**Source:** `timing/latency/config.go`

| Parameter | Default Value | Description | Tunable |
|-----------|---------------|-------------|---------|
| `ALULatency` | 1 cycle | Basic ALU ops (ADD, SUB, AND, OR, XOR) | ✅ Yes |
| `BranchLatency` | 1 cycle | Base branch execution (no misprediction) | ✅ Yes |
| `BranchMispredictPenalty` | 12 cycles | Additional penalty on misprediction | ✅ Yes |
| `LoadLatency` | 4 cycles | Load assuming L1 hit | ✅ Yes |
| `StoreLatency` | 1 cycle | Store to LSQ (fire-and-forget) | ✅ Yes |
| `MultiplyLatency` | 3 cycles | Integer multiply (future) | ✅ Yes |
| `DivideLatencyMin` | 10 cycles | Integer divide minimum (future) | ✅ Yes |
| `DivideLatencyMax` | 15 cycles | Integer divide maximum (future) | ✅ Yes |
| `SyscallLatency` | 1 cycle | System call instruction | ✅ Yes |

**How to configure:** Pass a `TimingConfig` to `latency.NewTableWithConfig()`, or load from JSON with `latency.LoadConfig(path)`.

---

## L1 Instruction Cache

**Source:** `timing/cache/cache.go` → `DefaultL1IConfig()`

| Parameter | Default Value | Apple M2 Reference | Tunable |
|-----------|---------------|-------------------|---------|
| `Size` | 192 KB | 192 KB (P-core), 128 KB (E-core) | ✅ Yes |
| `Associativity` | 6-way | 6-way (P-core), 4-way (E-core) | ✅ Yes |
| `BlockSize` | 64 bytes | 64 bytes | ✅ Yes |
| `HitLatency` | 1 cycle | ~1-2 cycles | ✅ Yes |
| `MissLatency` | 12 cycles | ~12 cycles (to L2) | ✅ Yes |

---

## L1 Data Cache

**Source:** `timing/cache/cache.go` → `DefaultL1DConfig()`

| Parameter | Default Value | Apple M2 Reference | Tunable |
|-----------|---------------|-------------------|---------|
| `Size` | 128 KB | 128 KB (P-core), 64 KB (E-core) | ✅ Yes |
| `Associativity` | 8-way | 8-way (P-core), 4-way (E-core) | ✅ Yes |
| `BlockSize` | 64 bytes | 64 bytes | ✅ Yes |
| `HitLatency` | 1 cycle | ~4 cycles | ⚠️ Review |
| `MissLatency` | 12 cycles | ~12 cycles (to L2) | ✅ Yes |

**Note:** L1D `HitLatency` is 1 cycle in cache config, but `LoadLatency` (4 cycles) in the latency table represents total load-to-use latency. These interact—need clarification on how they combine.

---

## L2 Cache (Unified)

**Source:** `timing/cache/cache.go` → `DefaultL2Config()`

| Parameter | Default Value | Apple M2 Reference | Tunable |
|-----------|---------------|-------------------|---------|
| `Size` | 16 MB | 16 MB (shared per cluster) | ✅ Yes |
| `Associativity` | 16-way | 16-way (estimated) | ✅ Yes |
| `BlockSize` | 128 bytes | 128 bytes | ✅ Yes |
| `HitLatency` | 12 cycles | ~12-14 cycles | ✅ Yes |
| `MissLatency` | 200 cycles | ~150-200 cycles (to DRAM) | ✅ Yes |

**Note:** L2 cache is implemented but not yet integrated into the default pipeline configuration.

---

## Memory Latencies

**Source:** `timing/cache/cache.go` (cache.Config)

Memory hierarchy latencies are configured in cache configurations, not in the instruction latency table.

| Parameter | Location | Default Value | Apple M2 Reference | Tunable |
|-----------|----------|---------------|-------------------|---------|
| L1D `HitLatency` | `cache.DefaultL1DConfig()` | 4 cycles | ~4 cycles | ✅ Yes |
| L1D `MissLatency` | `cache.DefaultL1DConfig()` | 12 cycles | ~12 cycles to L2 | ✅ Yes |
| L1I `HitLatency` | `cache.DefaultL1IConfig()` | 1 cycle | ~1 cycle | ✅ Yes |
| L1I `MissLatency` | `cache.DefaultL1IConfig()` | 12 cycles | ~12 cycles to L2 | ✅ Yes |
| L2 `HitLatency` | `cache.DefaultL2Config()` | 12 cycles | ~12-14 cycles | ✅ Yes |
| L2 `MissLatency` | `cache.DefaultL2Config()` | 200 cycles | ~150-200 cycles (DRAM) | ✅ Yes |

**Note:** The instruction latency table (`timing/latency/config.go`) provides execution latencies only. Memory hierarchy latencies were moved to cache configurations to avoid duplication and double-counting.

---

## Pipeline Structure

**Source:** `timing/pipeline/pipeline.go`

| Component | Description | Fixed/Tunable |
|-----------|-------------|---------------|
| 5-stage pipeline | IF → ID → EX → MEM → WB | 🔒 Fixed |
| Pipeline registers | IFID, IDEX, EXMEM, MEMWB | 🔒 Fixed |
| Hazard detection | Full forwarding + load-use stalls | 🔒 Fixed |
| Branch handling | Always-not-taken prediction | ⚠️ Needs work |

**Note:** Current branch predictor is trivial (always not-taken). Real M2 has sophisticated branch prediction. This is a significant accuracy gap.

---

## Hardcoded Values

These values are embedded in code and require source changes:

| Location | Value | Description |
|----------|-------|-------------|
| `pipeline.go` | 5 stages | Pipeline depth |
| `pipeline.go` | 1 cycle/stage | Stage latency (ideal) |
| Tests | 6 cycles | Expected instruction completion time |
| Tests | 10 cycles | Cache miss completion time |

---

## Configuration Methods

### 1. Programmatic (Go API)

```go
// Custom latency table
config := &latency.TimingConfig{
    ALULatency:              1,
    LoadLatency:             4,
    BranchMispredictPenalty: 12,
    // ...
}
table := latency.NewTableWithConfig(config)
pipe := pipeline.NewPipeline(regFile, mem, pipeline.WithLatencyTable(table))

// Custom cache configuration
icacheConfig := cache.Config{
    Size:          192 * 1024,
    Associativity: 6,
    BlockSize:     64,
    HitLatency:    1,
    MissLatency:   12,
}
pipe := pipeline.NewPipeline(regFile, mem, pipeline.WithICache(icacheConfig))
```

### 2. JSON Configuration File

```json
{
    "alu_latency": 1,
    "branch_latency": 1,
    "branch_mispredict_penalty": 12,
    "load_latency": 4,
    "store_latency": 1,
    "multiply_latency": 3,
    "divide_latency_min": 10,
    "divide_latency_max": 15,
    "syscall_latency": 1,
    "l1_hit_latency": 4,
    "l2_hit_latency": 12,
    "l3_hit_latency": 30,
    "memory_latency": 150
}
```

Load with: `config, err := latency.LoadConfig("timing.json")`

---

## Known Calibration Gaps

### High Priority
1. **Branch prediction** - Currently always-not-taken; M2 has advanced predictors
2. **L1D hit latency discrepancy** - Cache config says 1 cycle, latency table says 4 cycles
3. **Out-of-order execution** - M2 is OoO; we model in-order only

### Medium Priority
4. **L3/SLC naming** - Parameter exists but M2 doesn't have traditional L3
5. **Store buffer** - Not modeled; stores appear instant
6. **Memory disambiguation** - Not modeled

### Low Priority
7. **E-core vs P-core** - Currently P-core defaults only
8. **Multi-core** - Not yet implemented
9. **SMT** - Not applicable to M2

---

## Calibration Workflow

1. **Baseline measurement**: Run benchmarks on real M2, collect cycles/IPC
2. **Simulation**: Run same benchmarks in M2Sim
3. **Compare**: Identify largest discrepancies
4. **Tune**: Adjust parameters, prioritizing high-impact ones
5. **Iterate**: Re-run and refine

Recommended benchmarks:
- Simple loops (ALU-bound)
- Memory traversal (cache-bound)
- Branch-heavy code (predictor-bound)
- Mixed workloads

---

*Document generated by Frank for M2Sim calibration phase.*
*Last updated: Issue #74*
