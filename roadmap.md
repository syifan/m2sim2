# M2Sim Roadmap

## Overview
This roadmap tracks milestone completion and strategic decisions for the M2Sim project. Last updated: February 17, 2026.

## Completed Milestones

### H1: Core Simulator ✅ COMPLETE
Foundation simulator with ARM64 decode, pipeline timing, cache hierarchy, branch prediction, 8-wide superscalar, and macro-op fusion. Achieved 34.2% average CPI error on microbenchmarks.

### H2: SPEC Benchmark Enablement ✅ COMPLETE
Complete syscall infrastructure (file I/O, memory management, exit handling) and ARM64 cross-compilation setup. Matrix multiply and medium-sized benchmarks created.

### H3: Initial Accuracy Calibration ✅ COMPLETE
Achieved <20% average CPI error on microbenchmarks (14.1% target met) through parameter tuning and pipeline improvements.

### Milestone 10: Stability Recovery ✅ COMPLETE (February 17, 2026)
Restored simulator stability: memorystrided regression fixed (253.1% → 16.81%), all PolyBench timeouts resolved (jacobi-1d, gemm, 2mm completing successfully), 18 benchmarks with error data.

## Failed Milestones

### Milestone 11: Reduce PolyBench CPI to <80% ❌ FAILED (25/25 cycles)
**Goal:** Reduce PolyBench average CPI error from 98% to <80%.
**Result:** Failed after 25 cycles. PolyBench average remained ~98%.
**Changes attempted:** OoO issue within fetch group (PR #85 - memory ports), instruction window 48→192, load-use stall bypass (PR #87 - still in CI). These changes improved some individual benchmarks but didn't achieve the target.
**Key insight:** The in-order pipeline fundamentally overestimates CPI for loop-heavy PolyBench kernels. The M2's 330+ ROB enables massive loop-level parallelism that our pipeline doesn't model.

### Lessons Learned
1. **Too ambitious target.** Reducing from 98% to <80% required simultaneous improvement across 7 kernels with different bottlenecks. Should have targeted 1-2 specific kernels.
2. **CI turnaround time.** PolyBench CI takes hours per run. With 25 cycles, the team could only test ~5-6 iterations. Need faster feedback loops.
3. **Akita cache behavior (issue #183).** Human flagged that Akita cache/memory controllers may not behave as expected. This could be a fundamental accuracy issue hiding behind pipeline tuning.
4. **PR #87 still in flight.** The window increase + load-use bypass may yield improvement but we can't verify without CI results.

## Current State (February 17, 2026)

**Accuracy (CI-verified from main branch):**
- **Microbenchmarks:** 17.00% average (meets <20% target)
- **PolyBench:** 98.34% average (7 kernels completing)
- **Overall:** 48.63% average

**In-flight:** PR #87 (window 192 + load-use bypass) — CI still running

**Upcoming work priorities:**
1. Investigate Akita cache behavior (issue #183) — may be root cause of PolyBench overestimation
2. Merge PR #87 results and measure actual improvement
3. Incrementally reduce PolyBench error through targeted kernel-specific fixes

## Next Milestone Plan

### Milestone 12: Cache Verification + Merge Pending Work
**Strategy:** Before more pipeline tuning, verify the cache subsystem behaves correctly (issue #183). Also merge PR #87 and measure actual PolyBench improvement. These are prerequisite steps — if caches are misconfigured, all pipeline tuning is wasted effort.

### Future Milestones (tentative)
- **Milestone 13+:** Targeted PolyBench kernel improvements (one kernel at a time)
- **H4:** Multi-core support (deferred until accuracy is acceptable)
