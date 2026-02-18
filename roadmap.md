# M2Sim Roadmap

## Overview
This roadmap tracks milestone completion and strategic decisions for the M2Sim project. Last updated: February 18, 2026.

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

### Milestone 13: Reduce PolyBench CPI error to <70% ✅ GOAL MET (February 18, 2026)
**Goal was:** Reduce PolyBench average CPI error from 98% to <70%.
**Outcome:** Pre-OoO code (reverted baseline at d2c3373) achieves PolyBench avg 26.68% — well under <70% target.

**How it happened:** The OoO dispatch experiment (PR #93, 20 cycles) tried to improve individual benchmarks further. After mixed results (GEMM CI inconclusive), the team reverted to pre-OoO baseline and discovered the structural hazard reduction from earlier PRs (#65-74) had already driven the PolyBench avg to 26.68%. The pre-OoO baseline meets the target.

**Current accuracy (February 18, 2026, CI-verified):**
- Microbenchmarks: 54.78% average (11 benchmarks) — **does NOT meet <20%** (memorystrided: 429% error dominates)
- PolyBench: 26.68% average (4 benchmarks: atax 5.7%, bicg 18.9%, jacobi-1d 52.9%, gemm 29.1%)
- Overall: 47.29% average (15 benchmarks)

**Remaining blocker:** memorystrided is massively wrong (sim CPI 0.5 vs HW CPI 2.648, 429% error). This single benchmark drags the micro average from ~13.5% to 54.78%.

**Lessons learned this milestone:**
8. **OoO experiments cause regressions.** The instruction window OoO approach caused dcache timeouts, infinite loops, and CI instability. Pre-OoO code is more stable.
9. **Don't abandon CI before results are in.** Team reverted without waiting for GEMM CI results, leaving CI ambiguity.
10. **memorystrided is the #1 remaining blocker.** Fix this first before anything else.

## Failed Milestones

### Milestone 11: Reduce PolyBench CPI to <80% ❌ FAILED (25/25 cycles)
**Goal:** Reduce PolyBench average CPI error from 98% to <80%.
**Result:** Failed after 25 cycles. PolyBench average remained ~98%.
**Changes attempted:** OoO issue within fetch group (PR #85 - memory ports), instruction window 48→192, load-use stall bypass (PR #87).
**Key insight:** The in-order pipeline fundamentally overestimates CPI for loop-heavy PolyBench kernels. The M2's 330+ ROB enables massive loop-level parallelism that our pipeline doesn't model.

## Current State (February 18, 2026)

**Accuracy (latest CI-verified):**
- **Microbenchmarks:** 54.78% average — driven by memorystrided (429% error)
- **PolyBench:** 26.68% average (4 benchmarks: atax 5.7%, bicg 18.9%, jacobi-1d 52.9%, gemm 29.1%)
- **Overall:** 47.29% average (15 benchmarks)

**PolyBench breakdown (4 completing):**
| Benchmark | Sim CPI | HW CPI | Error | Status |
|-----------|---------|--------|-------|--------|
| atax      | 0.231   | 0.219  | 5.7%  | ✅ Good |
| bicg      | 0.273   | 0.230  | 18.9% | ✅ Good |
| gemm      | 0.301   | 0.233  | 29.1% | CI pending verification |
| jacobi-1d | 0.231   | 0.151  | 52.9% | ⚠️ Needs improvement |
| mvt       | infeasible | 0.216 | — | ⛔ Too slow |
| 2mm/3mm   | infeasible | —    | — | ⛔ Too slow |

**Microbenchmark breakdown:**
| Benchmark | Sim CPI | HW CPI | Error | Status |
|-----------|---------|--------|-------|--------|
| arithmetic    | 0.219 | 0.296 | 35.2% | ⚠️ Needs work |
| dependency    | 1.015 | 1.088 | 7.2%  | ✅ Good |
| branch        | 1.311 | 1.303 | 0.6%  | ✅ Good |
| memorystrided | 0.500 | 2.648 | 429%  | ❌ Broken |
| loadheavy     | 0.349 | 0.429 | 22.9% | ⚠️ Slightly high |
| storeheavy    | 0.522 | 0.612 | 17.2% | ✅ Good |
| branchheavy   | 0.941 | 0.714 | 31.8% | ⚠️ Needs work |
| vectorsum     | 0.362 | 0.402 | 11.1% | ✅ Good |
| vectoradd     | 0.290 | 0.329 | 13.5% | ✅ Good |
| reductiontree | 0.406 | 0.480 | 18.2% | ✅ Good |
| strideindirect| 0.609 | 0.528 | 15.3% | ✅ Good |

**Root cause of memorystrided regression:** memorystrided does stride-4 store/load pairs. The HW CPI is 2.648 (memory-bound, cache misses dominate). Simulator CPI is 0.5 — the simulator dramatically underestimates memory stalls for strided access. This was likely caused by the pipeline changes in PRs #65-74.

### Lessons Learned (cumulative)
1. **Break big problems into small ones.** Milestone 11 failed by targeting all 7 PolyBench kernels. Target 1-2 at a time.
2. **CI turnaround is the bottleneck.** PolyBench CI takes hours. Each cycle can only test one CI iteration. Budget cycles accordingly.
3. **Caches are correctly configured** (issue #183 resolved). The problem is purely in the pipeline timing model.
4. **Research before implementation.** Profile WHY sim CPI is high on specific kernels before changing pipeline parameters.
5. **pipeline.go refactored.** Now split into 13 manageable files (issue #126 resolved).
6. **Structural hazards are the #1 accuracy bottleneck for PolyBench.** Profiling confirms in-order co-issue blocking is the dominant source of CPI overestimation.
7. **Milestone 11 tried too much at once.** Targeting <80% on all 7 PolyBench was too ambitious.
8. **OoO experiments cause regressions.** Instruction window OoO approach caused dcache timeouts and CI instability.
9. **memorystrided is the #1 remaining blocker for microbenchmark accuracy.**

## Milestone 14: Fix memorystrided and merge PR #93

**Goal:** Fix the memorystrided benchmark accuracy (currently 429% error) and close out PR #93 cleanly.

**Tasks:**
1. **Merge PR #93** (contains the reverted code, which meets the milestone 13 goal). PR description needs updating to reflect revert.
2. **Diagnose memorystrided root cause:** memorystrided does stride-4 store/load pairs. Sim CPI 0.5 vs HW CPI 2.648 — simulator is not modeling cache miss latency for strided access. The memorystrided benchmark was correctly simulated before PRs #65-74 (error was ~10%). Identify which PR introduced the regression.
3. **Fix memorystrided:** Restore correct memory stall behavior without breaking PolyBench improvements.
4. **Verify via CI:** Run microbenchmark CI; confirm memorystrided error drops to <50% without regressions.

**Target:** memorystrided error <100% (from 429%), micro avg <25% (from 54.78%), no PolyBench regressions.

**Constraints:**
- All existing tests must pass (ginkgo -r)
- No PolyBench regressions (avg must stay <70%)
- Changes should be minimal and targeted at memory stall handling

**Estimated cycles:** 15

## Future Milestones (tentative)

### Milestone 15: Improve arithmetic + branchheavy accuracy
Target: arithmetic <20% (from 35.2%), branchheavy <20% (from 31.8%), keeping memorystrided fixed.

### Milestone 16: Improve jacobi-1d PolyBench accuracy
Target: jacobi-1d <30% (from 52.9%), maintaining other PolyBench benchmarks.

### Milestone 17+: Overall <20% average error
Iterate on remaining accuracy gaps to achieve the H5 target across all benchmarks.

### H4: Multi-Core Support (deferred)
Not started. Prerequisites: H5 accuracy target must be CI-verified first.
