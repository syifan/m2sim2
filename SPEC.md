# SPEC.md - M2Sim Project Specification

## Project Goal

Build a cycle-accurate Apple M2 CPU simulator using the Akita simulation framework that can execute ARM64 user-space programs and predict execution time with high accuracy.

## Success Criteria

- [x] Execute ARM64 user-space programs correctly (functional emulation)
- [ ] Predict execution time with <20% average error across benchmarks
- [ ] Modular design: functional and timing simulation are separate
- [ ] Support benchmarks in μs to ms range

## Design Philosophy

### Independence from MGPUSim

While M2Sim uses Akita (like MGPUSim) and draws inspiration from MGPUSim's architecture, **M2Sim is not bound to follow MGPUSim's structure**. Make design decisions that best fit an ARM64 CPU simulator.

**Guidelines:**

1. **Choose meaningful names**: If a different name is more appropriate, use it
2. **Adapt to CPU semantics**: GPU and CPU have different abstractions (no wavefronts, warps, or GPU-specific concepts)
3. **Keep it simple**: M2Sim targets single-core initially
4. **Diverge when it makes sense**: Document why you're doing it differently

**What to Keep from MGPUSim:**
- Akita component/port patterns (they work well)
- Separation of concerns (functional vs timing)
- Testing practices (Ginkgo/Gomega)

**When in Doubt:** Ask "What would make this clearest for a CPU simulator?" — not "What does MGPUSim do?"

## Milestones

### M1: Foundation (MVP) ✅ COMPLETE
Basic ARM64 execution capability.

**Completion criteria:**
- [x] Project scaffolding with Akita dependency
- [x] ARM64 instruction decoder (basic formats)
- [x] Register file (X0-X30, SP, PC)
- [x] Basic ALU instructions (ADD, SUB, AND, OR, XOR)
- [x] Load/Store instructions (LDR, STR)
- [x] Branch instructions (B, BL, BR, RET, B.cond)

### M2: Memory & Control Flow ✅ COMPLETE
Complete functional emulation.

**Completion criteria:**
- [x] Syscall emulation (exit, write)
- [x] Simple memory model (flat address space)
- [x] Run simple C programs end-to-end

### M3: Timing Model ✅ COMPLETE
Cycle-accurate timing.

**Completion criteria:**
- [x] Pipeline stages (Fetch, Decode, Execute, Memory, Writeback)
- [x] Instruction timing
- [x] Basic timing predictions

### M4: Cache Hierarchy ✅ COMPLETE
Memory system timing.

**Completion criteria:**
- [x] L1 instruction cache (CachedFetchStage)
- [x] L1 data cache (CachedMemoryStage)
- [x] L2 unified cache (CacheBacking hierarchy)
- [x] Cache timing model (integrated via WithICache/WithDCache/WithDefaultCaches)

### M5: Advanced Features ✅ COMPLETE
Accuracy improvements.

**Completion criteria:**
- [x] Branch prediction
- [x] Superscalar execution (8-wide)
- [x] CMP+B.cond macro-op fusion
- [x] SIMD basics

### M6: Validation 🚧 IN PROGRESS
Final accuracy validation.

**Completion criteria:**
- [x] Run microbenchmark suite (arithmetic, branch, dependency)
- [ ] Compare with real M2 timing data
- [ ] Achieve <20% average error

## Calibration Milestones

Accuracy improvement tracking against M2 hardware baseline.

### C1: Baseline Established ✅ COMPLETE
Initial accuracy measurement.

**Status:**
- [x] Microbenchmarks created (arithmetic, branch, dependency)
- [x] M2 baseline data collected
- [x] Initial error: 39.8% average

### C2: Accuracy Optimization 🚧 IN PROGRESS
Reduce error through pipeline improvements.

**Current status (cycle 230):**
| Benchmark | Sim CPI | M2 CPI | Error |
|-----------|---------|--------|-------|
| arithmetic_sequential | 0.400 | 0.268 | 49.3% |
| dependency_chain | 1.200 | 1.009 | 18.9% |
| branch_taken_conditional | 1.600 | 1.190 | 34.5% |
| **Average** | — | — | **34.2%** |

**Optimizations applied:**
- [x] CMP+B.cond macro-op fusion (62.5% → 34.5% branch error)
- [x] 8-wide decode infrastructure (merged cycle 230)
- [ ] Full 8-wide execution (expected: 49.3% → ~28% arithmetic)

### C3: Target Achievement
Achieve <20% average error.

**Requirements:**
- [ ] All benchmarks individually <30% error
- [ ] Average error <20%
- [ ] Results reproducible

## Scope

### In Scope
- ARM64 user-space instructions
- CPU core simulation (single-core MVP, multi-core later)
- Cache hierarchy
- Timing prediction

### Out of Scope
- GPU / Neural Engine
- Kernel-space execution
- Full OS simulation
- I/O devices beyond basic syscalls

## Technical Constraints

- Use Akita v4 simulation framework
- Follow MGPUSim architecture patterns
- Go programming language
- Tests use Ginkgo/Gomega

## References

- Akita: https://github.com/sarchlab/akita
- MGPUSim: https://github.com/sarchlab/mgpusim
- ARM Architecture Reference Manual
- See `docs/calibration.md` for timing parameter reference
