# M2Sim Roadmap

## Overview
Strategic plan for achieving H5: <20% average CPI error across 15+ benchmarks.
Last updated: February 18, 2026.

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

## Current State (February 18, 2026)

**Accuracy (from h5_accuracy_results.json):**
- **14 benchmarks with error data** (11 micro + 3 PolyBench with HW CPI)
- **Overall average error: 39.29%** — does NOT meet <20% target
- **Micro average: 39.25%** (excl. memorystrided: 22.91%)
- **PolyBench average: 39.42%** (atax 19.4%, bicg 70.4%, mvt 28.5%)
- 6 microbenchmarks have stale data (ci_verified=false, pending post-PR#101 CI)

**Error breakdown (sorted by error, descending):**

| Benchmark | Category | Sim CPI | HW CPI | Error | Verified? |
|-----------|----------|---------|--------|-------|-----------|
| memorystrided | micro | 0.875 | 2.648 | 202.6% | Yes |
| bicg | polybench | 0.391 | 0.230 | 70.4% | Yes |
| storeheavy | micro | 0.957 | 0.612 | 56.4% | No (stale) |
| loadheavy | micro | 0.600 | 0.429 | 39.9% | No (stale) |
| arithmetic | micro | 0.219 | 0.296 | 35.2% | Yes |
| branchheavy | micro | 0.941 | 0.714 | 31.8% | Yes |
| mvt | polybench | 0.277 | 0.216 | 28.5% | Yes |
| reductiontree | micro | 0.594 | 0.480 | 23.8% | No (stale) |
| atax | polybench | 0.183 | 0.219 | 19.4% | Yes |
| vectoradd | micro | 0.385 | 0.329 | 17.0% | No (stale) |
| strideindirect | micro | 0.609 | 0.528 | 15.3% | No (stale) |
| dependency | micro | 1.015 | 1.088 | 7.2% | Yes |
| vectorsum | micro | 0.394 | 0.402 | 2.0% | No (stale) |
| branch | micro | 1.311 | 1.303 | 0.6% | Yes |

**Additional data points (no HW CPI for error calculation):**
- jacobi-1d: sim CPI 0.349, HW CPI unknown
- aha_mont64: sim CPI 0.347, no HW CPI workflow

**Infeasible:** gemm, 2mm, 3mm (polybench); crc32, edn, statemate, primecount, huffbench, matmult-int (embench)

## Path to H5: <20% Average Error Across 15+ Benchmarks

**Math:** Current sum of errors across 14 benchmarks = 5.50. For 15 benchmarks at <20% average, sum must be < 3.0. Need to reduce total error by >2.5.

**High-impact fixes needed (in priority order):**
1. **memorystrided** (error 2.026 → target 0.20): saves ~1.83 from total — THE #1 priority
2. **bicg** (error 0.704 → target 0.20): saves ~0.50
3. **storeheavy** (error 0.564 → target 0.20): saves ~0.36
4. **loadheavy** (error 0.399 → target 0.20): saves ~0.20
5. **arithmetic** (error 0.352 → target 0.20): saves ~0.15
6. **branchheavy** (error 0.318 → target 0.20): saves ~0.12

Fixing #1–#4 + adding 1 new benchmark at ~15% error → 15 benchmarks, sum ≈ 2.60, avg ≈ 17.3%. This achieves the target.

## Milestone Plan (M17–M21)

### M17: Verify CI Baseline + Diagnose memorystrided (NEXT)
**Budget:** 12 cycles
**Goal:** Get verified CI data for all benchmarks on current main. Profile memorystrided to understand why sim CPI (0.875) is so far below HW CPI (2.648). Implement targeted fix.
**Success:** memorystrided error < 80%. Verified CI data for all 14 benchmarks.

### M18: Fix PolyBench Accuracy (bicg focus)
**Budget:** 12 cycles
**Goal:** Reduce bicg error from 70% to <30%. Improve mvt if possible.

### M19: Fix Memory-Heavy Microbenchmarks
**Budget:** 12 cycles
**Goal:** Reduce storeheavy (<30%) and loadheavy (<25%). These benchmarks share a root cause (memory pipeline CPI overestimation).

### M20: Reduce Remaining Microbenchmark Errors
**Budget:** 10 cycles
**Goal:** Reduce arithmetic (<25%), branchheavy (<25%). These are secondary blockers.

### M21: Final Push — 15+ Benchmarks at <20% Average
**Budget:** 10 cycles
**Goal:** Add jacobi-1d HW CPI (or other benchmarks), final calibration, achieve H5 target.

**Total estimated budget:** ~56 cycles

### H4: Multi-Core Support (deferred until H5 complete)

## Lessons Learned (from milestones 10–16)

1. **Break big problems into small ones.** Target 1–2 benchmarks per milestone, not all at once.
2. **CI turnaround is the bottleneck.** Each cycle can only test one CI iteration. Budget accordingly.
3. **Caches are correctly configured** (M11 confirmed). Problems are purely pipeline timing.
4. **Research before implementation.** Profile WHY sim CPI is wrong before changing parameters.
5. **OoO experiments cause regressions.** Stick to in-order pipeline improvements.
6. **Don't merge without CI verification.** Update accuracy data ONLY from CI-verified runs.
7. **"Wait for CI" should be its own task.** Never combine CI wait + implementation in one milestone.
8. **Structural hazards are the #1 pipeline accuracy bottleneck** for most benchmarks.
9. **memorystrided is a distinct problem** — sim is too fast (not too slow like others), suggesting missing memory penalties.
10. **The Marin runner group** provides Apple M2 hardware for accuracy benchmarks.
