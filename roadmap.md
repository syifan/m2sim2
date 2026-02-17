# M2Sim Roadmap

## Overview
This roadmap tracks milestone completion and strategic decisions for the M2Sim project. Last updated: February 17, 2026.

## Completed Milestones

### H1: Core Simulator ✅ COMPLETE
Foundation simulator with ARM64 decode, pipeline timing, cache hierarchy, branch prediction, 8-wide superscalar, and macro-op fusion.

### H2: SPEC Benchmark Enablement ✅ COMPLETE
Complete syscall infrastructure and ARM64 cross-compilation setup.

### H3: Initial Accuracy Calibration ✅ COMPLETE
Achieved <20% average CPI error on microbenchmarks (14.1%).

### Milestone 10: Stability Recovery ✅ COMPLETE (February 17, 2026)
Restored simulator stability: memorystrided regression fixed, all PolyBench timeouts resolved, 18 benchmarks with error data.

### Milestone 11 (cache verification portion): ✅ COMPLETE
Cache verification tests written and passed (PR #88, issue #183 closed). Akita caches behave as configured — no misconfigurations found. PR #87 merged.

### Milestone 12: Refactor pipeline.go + Profile PolyBench Bottlenecks ✅ COMPLETE
- Pipeline.go split into 13 files, all under 2000 lines (PRs #90, #92)
- Stall profiling counters added (PR #91): RAWHazardStalls, StructuralHazardStalls, BranchMispredictionStalls
- Stall profile data collected for gemm, bicg, atax — structural hazards are the dominant overestimated source
- CI passing on main after gofmt fix
- **Actual cycles used:** ~8 (vs 15 budgeted)

## Failed Milestones

### Milestone 11: Reduce PolyBench CPI to <80% ❌ FAILED (25/25 cycles)
**Goal:** Reduce PolyBench average CPI error from 98% to <80%.
**Result:** Failed after 25 cycles. PolyBench average remained ~98%.
**Changes attempted:** OoO issue within fetch group (PR #85 - memory ports), instruction window 48→192, load-use stall bypass (PR #87).
**Key insight:** The in-order pipeline fundamentally overestimates CPI for loop-heavy PolyBench kernels. The M2's 330+ ROB enables massive loop-level parallelism that our pipeline doesn't model.

## Current State (February 17, 2026)

**Accuracy (CI-verified, post PR #92 merge):**
- **Microbenchmarks:** 17.00% average (11 benchmarks, meets <20% target)
- **PolyBench:** 98.15% average (7 benchmarks completing)
- **Overall:** 48.56% average (18 benchmarks with error data)

**PolyBench breakdown (sorted by error):**
| Benchmark | Sim CPI | HW CPI | Error |
|-----------|---------|--------|-------|
| gemm | 0.301 | 0.233 | 29.1% |
| bicg | 0.343 | 0.230 | 49.5% |
| mvt | 0.364 | 0.216 | 68.8% |
| atax | 0.396 | 0.219 | 81.2% |
| 3mm | 0.334 | 0.145 | 129.9% |
| 2mm | 0.328 | 0.144 | 128.6% |
| jacobi-1d | 0.453 | 0.151 | 200.0% |

**Stall profiling results (from Milestone 12):**
- Structural hazard stalls dominate all 3 profiled kernels (gemm: 22.5M, bicg: 16.7M, atax: 22.5M in 10M cycles)
- RAW hazard stalls: zero (compiler schedules loads ahead of consumers)
- Memory stalls: secondary contributor (2.5-3.3M per 10M cycles)
- Branch mispredictions: negligible (3-7 total per 10M cycles)
- **Root cause:** in-order issue logic blocks co-issue for dependent instructions. Real M2 (OoO, 330+ ROB) dynamically reorders and issues dependent instructions as operands become ready.

### Lessons Learned (cumulative)
1. **Break big problems into small ones.** Milestone 11 failed by targeting all 7 PolyBench kernels. Target 1-2 at a time.
2. **CI turnaround is the bottleneck.** PolyBench CI takes hours. Each cycle can only test one CI iteration. Budget cycles accordingly.
3. **Caches are correctly configured** (issue #183 resolved). The problem is purely in the pipeline timing model.
4. **Research before implementation.** Profile WHY sim CPI is high on specific kernels before changing pipeline parameters.
5. **pipeline.go refactored.** Now split into 13 manageable files (issue #126 resolved).
6. **Structural hazards are the #1 accuracy bottleneck.** Profiling confirms in-order co-issue blocking is the dominant source of CPI overestimation.
7. **Milestone 11 tried too much at once.** Targeting <80% on all 7 PolyBench was too ambitious. Milestone 13 should target only the closest kernels (gemm at 29.1%, bicg at 49.5%).

## Milestone 13: Reduce gemm + bicg + mvt + atax error via OoO dispatch improvements

**Goal:** Reduce PolyBench average CPI error from 98% to <70% by improving out-of-order dispatch to reduce structural hazard stalls.

**Approach (informed by stall profiling data):**
1. **Enable instruction window OoO dispatch for sextuple-issue mode** — currently the instruction window only feeds the octuple-issue path. Enabling it for 6-wide issue would allow finding independent instructions across loop iterations, directly reducing the dominant structural hazard stalls.
2. **Relax co-issue hazard checks** — for ALU→ALU dependencies where the producer completes in 1 cycle, same-cycle forwarding should allow the dependent instruction to issue. The `canIssueWith()` function is overly conservative for single-cycle producers.
3. **Target:** gemm <25%, bicg <40%, while keeping microbenchmark average <20%.

**Constraints:**
- All existing tests must pass (ginkgo -r)
- No microbenchmark regressions beyond 2% average
- Changes must be in the pipeline issue logic, not parameter tuning
- CI must verify accuracy changes

**Estimated cycles:** 20

## Future Milestones (tentative)

### Milestone 14: Reduce mvt + atax error to <50%
Target the next pair of PolyBench kernels after gemm/bicg are improved.

### Milestone 15: Reduce 2mm + 3mm + jacobi-1d error
Target the highest-error kernels. These have very low HW CPI (0.14-0.15) suggesting extreme ILP — may need more aggressive OoO modeling.

### Milestone 16+: Overall <20% average error
Iterate on remaining accuracy gaps to achieve the H5 target across all 18 benchmarks.

### H4: Multi-Core Support (deferred)
Not started. Prerequisites: H5 accuracy target must be CI-verified first.
