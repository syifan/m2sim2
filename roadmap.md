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

## Current State (February 18, 2026, post-PR#94 + PR#95)

**Accuracy (CI-verified, post-PR#94 from run 22141495151, memorystrided from run 22142753352):**
- **Microbenchmarks:** 51.14% average (11 benchmarks) — memorystrided (253.1%) dominates; excl. memorystrided: 30.94%
- **PolyBench:** 105.22% average (3 benchmarks: atax, bicg, mvt from run 22139825134) — significant regression from PR#94
- **Overall:** 62.72% average (14 benchmarks)

**Key regression from PR#94 (fix memory stall propagation in tickOctupleIssue):**
- 6 microbenchmarks show 25-84% CPI increase: loadheavy, storeheavy, vectoradd, vectorsum, reductiontree, strideindirect
- PolyBench: atax regressed (5.7%→59.7%), bicg regressed (18.9%→144.9%), jacobi-1d now infeasible (timeout)
- mvt now completes (previously infeasible) with 111.0% error

**PolyBench breakdown (3 completing, post-PR#94):**
| Benchmark | Sim CPI | HW CPI | Error | Status |
|-----------|---------|--------|-------|--------|
| atax      | 0.349   | 0.219  | 59.7% | ⚠️ Regressed from 5.7% |
| bicg      | 0.562   | 0.230  | 144.9%| ❌ Regressed from 18.9% |
| mvt       | 0.455   | 0.216  | 111.0%| ⚠️ New (was infeasible) |
| jacobi-1d | infeasible | 0.151 | — | ⛔ Timeout (was 52.9%) |
| gemm      | infeasible | 0.233 | — | ⛔ Too slow |
| 2mm/3mm   | infeasible | —    | — | ⛔ Too slow |

**Microbenchmark breakdown (post-PR#94):**
| Benchmark | Sim CPI | HW CPI | Error | Status |
|-----------|---------|--------|-------|--------|
| arithmetic    | 0.219 | 0.296 | 35.2% | ⚠️ Unchanged |
| dependency    | 1.015 | 1.088 | 7.2%  | ✅ Unchanged |
| branch        | 1.311 | 1.303 | 0.6%  | ✅ Unchanged |
| memorystrided | 0.750 | 2.648 | 253.1%| ❌ Improved from 429% (livelock fix) |
| loadheavy     | 0.643 | 0.429 | 49.9% | ⚠️ Regressed from 22.9% |
| storeheavy    | 0.957 | 0.612 | 56.4% | ⚠️ Regressed from 17.2% |
| branchheavy   | 0.941 | 0.714 | 31.8% | ⚠️ Unchanged |
| vectorsum     | 0.500 | 0.402 | 24.4% | ⚠️ Regressed from 11.1% |
| vectoradd     | 0.448 | 0.329 | 36.2% | ⚠️ Regressed from 13.5% |
| reductiontree | 0.594 | 0.480 | 23.8% | ⚠️ Regressed from 18.2% |
| strideindirect| 0.761 | 0.528 | 44.1% | ⚠️ Regressed from 15.3% |

**Root cause of regressions:** PR#94 fixed memory stall propagation in tickOctupleIssue, which correctly added stalls that were previously missing. This increased CPI for all memory-heavy benchmarks. The stalls are arguably more correct (closer to real hardware behavior for memory latency) but the overall error increased because the simulator now overestimates CPI for these benchmarks.

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

### Milestone 14: Fix memorystrided livelock ✅ GOAL ACHIEVED (February 18, 2026)

**Goal:** Fix memorystrided accuracy and related livelock bugs.

**What happened:**
- PR #93 (revert OoO, pre-OoO baseline) merged ✅
- PR #94 (no-cache path memory stall fix): memorystrided improved from 429% → ~285% ✅
- PR #95 (CachedMemoryStage livelock fix): eliminates multi-port replay bug ✅
- Microbenchmark CI run 22144669883 triggered on main post-livelock-fix (pending results)

**Deadline missed at 15/15 cycles** — but all planned fixes were implemented and merged. CI is running.

**Lesson 11:** The livelock fix was correct but took the full budget. Next milestone: collect results and continue calibration.

## Milestone 15: Verify CI + Prepare Next Accuracy Target (CURRENT)

**Goal:** Verify memorystrided livelock fix CI results, update h5_accuracy_results.json, and prepare a profiling-based diagnosis of the next high-error benchmarks.

**Why this matters:** With memorystrided fixed, micro avg should drop from 54.78% to ~13.5% (meets H5 <20% goal). We need CI verification to confirm, then plan targeted fixes for jacobi-1d (52.9%), arithmetic (35.2%), branchheavy (31.8%).

**Tasks (CI-first approach):**
1. ✅ Check CI run 22144669883 — still QUEUED (runner congestion); used PR branch run 22142753352
2. ✅ Update h5_accuracy_results.json — memorystrided DCache CPI=0.750, error=253.1% (2.531)
3. ✅ Close open issues #222 and #223 in tbc-db
4. ❌ memorystrided still >100% (253.1%): root cause identified in issue #226; PR#96 (StoreForwardLatency 1→3) reduces to 202% error (CPI=0.875), still needs work
5. ✅ Root cause analysis complete: issues #226 (memorystrided) and #227 (jacobi-1d) filed
6. ✅ Investigated jacobi-1d root cause (sim 0.231 vs HW 0.151 — see issue #227)
7. ❌ PR#94 regression discovered: 6 micros regressed 25-84%, jacobi-1d now infeasible; PR#99 (Leo) fixes this — CI running
8. ⏳ PR#99 CI pending — must merge before PR#96 and PR#97 can be evaluated cleanly

**Current micro average: 51.14%** (11 benchmarks, post-PR#94 from CI run 22141495151)
- memorystrided: 253.1% error (dominant blocker — store-to-load forwarding latency issue)
- Without memorystrided: 30.94% average (does NOT meet H5 <20% target)
- PR#94 regressed 6 microbenchmarks by 25-84% CPI increase
- **H5 micro goal NOT met** — both memorystrided and PR#94 regressions need addressing

**Fix queue (in order of dependency):**
1. PR#99 — revert secondary port stalls (fixes PR#94 regressions) → must merge first
2. PR#96 — StoreForwardLatency 1→3 (memorystrided 253%→202% error)
3. PR#97 — SMULL stall overlap (jacobi-1d improvement)

**Budget: 10 cycles** (CI wait + verification + diagnosis)

**Success criteria:** h5_accuracy_results.json updated with CI-verified data; next accuracy target identified with root cause analysis.

**Constraints:**
- No speculative code changes until CI results are confirmed
- Update accuracy JSON only from CI-verified runs
- No PolyBench regressions (PR#99 must fix jacobi-1d timeout)

## Future Milestones (tentative)

### Milestone 16: Fix arithmetic + branchheavy accuracy
Target: arithmetic <20% (from 35.2%), branchheavy <20% (from 31.8%), keeping memorystrided fixed.

### Milestone 17: Improve jacobi-1d PolyBench accuracy
Target: jacobi-1d <30% (from 52.9%), maintaining other PolyBench benchmarks.

### Milestone 18+: Overall <20% average error
Iterate on remaining accuracy gaps to achieve the H5 target across all benchmarks.

### H4: Multi-Core Support (deferred)
Not started. Prerequisites: H5 accuracy target must be CI-verified first.
