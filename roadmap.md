# M2Sim Roadmap

## Overview
Strategic plan for achieving H5: <20% average CPI error across 15+ benchmarks.
Last updated: February 19, 2026.

## Active Milestone

**M17: Fix jacobi-1d and bicg over-stalling — IN PROGRESS**

## Completed High-Level Milestones

- **H1: Core Simulator** — ARM64 decode, pipeline, caches, branch prediction, 8-wide superscalar
- **H2: SPEC Benchmark Enablement** — Syscalls, cross-compilation, medium benchmarks
- **H3: Microbenchmark Calibration** — Achieved 14.1% avg error on 3 microbenchmarks

## Completed Implementation Milestones (M10–M16)

| Milestone | Result | Key Outcome |
|-----------|--------|-------------|
| M10: Stability Recovery | Done | 18 benchmarks with error data |
| M11: Cache Verification | Done | Caches correctly configured |
| M12: Refactor pipeline.go + Profile | Done | Split to 13 files; stall profiling added |
| M13: Reduce PolyBench CPI <70% | Done | Pre-OoO baseline achieves 26.68% PolyBench avg |
| M14: Fix memorystrided livelock | Done | Livelock fixed, memorystrided 429%→253% |
| M15: Verify CI + Prepare Next Target | Missed | Data partially collected; PR#99 merged |
| M16: Collect PR#99 CI + Merge PRs | Done | PR#96, PR#101 merged; 14 benchmarks verified |

## Current State (February 19, 2026)

**Latest CI-verified accuracy (from h5_accuracy_results.json, post-PR#106):**
- **15 benchmarks with error data** (11 micro + 4 PolyBench with HW CPI)
- **Overall average error: 29.46%** — does NOT meet <20% target
- **Key update:** PR#106 (Leo) fixed bicg regression by gating store-to-load ordering on D-cache
- **PR#106 did NOT regress memorystrided** — memorystrided runs with EnableDCache=true, so the store-to-load ordering check remains active. CI run 22180241267 confirms memorystrided CPI=2.125 (24.61% error), unchanged from pre-PR#106.

**Error breakdown (sorted by error, all CI-verified):**

| Benchmark | Category | Sim CPI | HW CPI | Error |
|-----------|----------|---------|--------|-------|
| jacobi-1d | polybench | 0.349 | 0.151 | 131.13% |
| bicg | polybench | 0.391 | 0.230 | 70.37% |
| arithmetic | micro | 0.219 | 0.296 | 35.16% |
| branchheavy | micro | 0.941 | 0.714 | 31.79% |
| mvt | polybench | 0.277 | 0.216 | 28.48% |
| memorystrided | micro | 2.125 | 2.648 | 24.61% |
| loadheavy | micro | 0.357 | 0.429 | 20.17% |
| atax | polybench | 0.183 | 0.219 | 19.40% |
| reductiontree | micro | 0.406 | 0.480 | 18.23% |
| storeheavy | micro | 0.522 | 0.612 | 17.24% |
| strideindirect | micro | 0.609 | 0.528 | 15.34% |
| vectoradd | micro | 0.296 | 0.329 | 11.15% |
| vectorsum | micro | 0.362 | 0.402 | 11.05% |
| dependency | micro | 1.015 | 1.088 | 7.19% |
| branch | micro | 1.311 | 1.303 | 0.61% |

**Infeasible:** gemm, 2mm, 3mm (polybench); crc32, edn, statemate, primecount, huffbench, matmult-int (embench)

## Path to H5: <20% Average Error Across 15+ Benchmarks

**Math:** Current sum of errors = ~442%. For 15 benchmarks at <20% avg, need sum < 300%. Must reduce by ~142 percentage points.

**The 2-benchmark roadblock:** The top 2 errors account for 201 percentage points:
1. **jacobi-1d** (131.13% → target <20%): saves ~111 points — CRITICAL
2. **bicg** (70.37% → target <20%): saves ~50 points — CRITICAL

If we fix both to <20%, remaining sum ≈ 261%, avg ≈ 17.4% → **H5 achieved**.

**Secondary targets** (above 20%):
3. **arithmetic** (35.16%): saves ~15 points
4. **branchheavy** (31.79%): saves ~12 points
5. **mvt** (28.48%): saves ~8 points
6. **memorystrided** (24.61%): saves ~5 points

**Root cause analysis:**
- **jacobi-1d** (sim too SLOW: 0.349 vs 0.151): Sim is 2.3x over-stalling for 1D stencil computation. Likely WAW/RAW hazard over-stalling in the pipeline.
- **bicg** (sim too SLOW: 0.391 vs 0.230): Sim is 70% over-stalling for dot products. PR#106 partially fixed this but more improvement needed.
- **memorystrided** (sim too SLOW: 2.125 vs 2.648): 24.61% error, above target but not critical. Sim slightly under-counts cache miss stall cycles for strided access patterns.

## Milestone Plan (M17–M18)

### M17: Fix jacobi-1d and bicg over-stalling (NEXT)
**Budget:** 12 cycles
**Goal:** jacobi-1d from 131% → <50%. bicg from 70% → <40%.
Both have sim CPI >> HW CPI (over-stalling). Profile stall sources in both benchmarks and reduce excessive WAW/structural hazard stalls for these compute patterns.
**Success:** jacobi-1d < 70%, bicg < 50%. No regressions on other benchmarks.

### M18: Final calibration — achieve H5 target
**Budget:** 10 cycles
**Goal:** Achieve <20% average error across all 15 benchmarks. Address remaining outliers (arithmetic 35%, branchheavy 32%, mvt 28%, memorystrided 25%). Verify final CI results.
**Success:** Average error < 20% across 15 benchmarks, all CI-verified.

**Total estimated budget:** ~22 cycles

### H4: Multi-Core Support (deferred until H5 complete)

## Lessons Learned (from milestones 10–17)

1. **Break big problems into small ones.** Target 1–2 benchmarks per milestone, not all at once.
2. **CI turnaround is the bottleneck.** Each cycle can only test one CI iteration. Budget accordingly.
3. **Caches are correctly configured** (M11 confirmed). Problems are purely pipeline timing.
4. **Research before implementation.** Profile WHY sim CPI is wrong before changing parameters.
5. **OoO experiments cause regressions.** Stick to in-order pipeline improvements.
6. **Don't merge without CI verification.** Update accuracy data ONLY from CI-verified runs.
7. **"Wait for CI" should be its own task.** Never combine CI wait + implementation in one milestone.
8. **Structural hazards are the #1 pipeline accuracy bottleneck** for most benchmarks.
9. **memorystrided is a distinct problem** — sim is too fast (not too slow), needs cache miss stall cycles.
10. **The Marin runner group** provides Apple M2 hardware for accuracy benchmarks.
11. **Verify regressions with code analysis, not assumptions.** PR#106 was wrongly assumed to regress memorystrided — code analysis confirmed it didn't (D-cache gating only affects non-D-cache benchmarks).
12. **The top 2 errors are the main roadblock.** Fix jacobi-1d + bicg → H5 likely achieved (avg drops to ~17.4%).
