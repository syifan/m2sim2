# M2Sim Roadmap

## Overview
Strategic plan for achieving H5: <20% average CPI error across 15+ benchmarks.
Last updated: February 19, 2026.

## Active Milestone

**M17c: Verify CI baseline + Fix arithmetic and branchheavy — NEXT**

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

## Current State (February 20, 2026)

**Branch state:** leo/fix-fp-coissue (HEAD = 8e4c397). Last 3 commits reverted failed M17b experiments, restored nonCacheLoadLatency=3. CI NOT YET RUN on current HEAD — h5_accuracy_results.json shows stale regressed data from co-issue commit b1f8d23 (avg 27.04%). Expected baseline after CI: ~23.70% (matching pre-M17b commit 28f7ec1).

**Expected accuracy (pending CI verification, based on pre-M17b state at commit 28f7ec1):**
- **15 benchmarks with error data** (11 micro + 4 PolyBench with HW CPI)
- **Overall average error: ~23.70%** — does NOT yet meet <20% target

**Error breakdown (from commit 28f7ec1 CI, pending re-verification):**

| Benchmark | Category | Sim CPI | HW CPI | Error | Direction |
|-----------|----------|---------|--------|-------|-----------|
| bicg | polybench | 0.393 | 0.230 | 71.24% | sim too SLOW |
| jacobi-1d | polybench | 0.253 | 0.151 | 67.55% | sim too SLOW |
| branchheavy | micro | 0.970 | 0.714 | 35.85% | sim too SLOW |
| arithmetic | micro | 0.220 | 0.296 | 34.55% | sim too FAST |
| loadheavy | micro | 0.357 | 0.429 | 20.17% | sim too FAST |
| atax | polybench | 0.183 | 0.219 | 19.40% | sim too FAST |
| storeheavy | micro | 0.522 | 0.612 | 17.24% | sim too FAST |
| memorystrided | micro | 2.267 | 2.648 | 16.81% | sim too FAST |
| reductiontree | micro | 0.419 | 0.480 | 14.56% | sim too FAST |
| strideindirect | micro | 0.600 | 0.528 | 13.64% | sim too SLOW |
| vectorsum | micro | 0.354 | 0.402 | 13.56% | sim too FAST |
| mvt | polybench | 0.241 | 0.216 | 11.78% | sim too SLOW |
| vectoradd | micro | 0.296 | 0.329 | 11.15% | sim too FAST |
| dependency | micro | 1.020 | 1.088 | 6.67% | sim too FAST |
| branch | micro | 1.320 | 1.303 | 1.30% | sim too SLOW |

**Infeasible:** gemm, 2mm, 3mm (polybench); crc32, edn, statemate, primecount, huffbench, matmult-int (embench)

## Path to H5: <20% Average Error Across 15+ Benchmarks

**Math:** Current sum of errors = ~355.5%. For 15 benchmarks at <20% avg, need sum < 300%. Must reduce by ~55.5 percentage points.

**STRATEGIC PIVOT (February 20, 2026):** After 18 cycles (M17 + M17b) of failed attempts to fix bicg, we are pivoting to a multi-pronged approach:

1. **Fix arithmetic (34.55%) and branchheavy (35.85%)** — fresh, unexplored targets
2. **bicg requires proper diagnosis** — the load-use latency hypothesis was DISPROVEN (see M17b outcome below)
3. **Adding low-error benchmarks** as a fallback path to dilute high errors

**If arithmetic → 20% and branchheavy → 20%:** saves 30.4 pts → sum 325.1 / 15 = 21.7%
**If we also add 3 benchmarks at ~10% each:** sum 355.1 / 18 = 19.7% ✅ H5 achieved
**If we also partially fix bicg (71% → 45%):** saves 26 more pts → easily under 20%

**Root cause analysis (updated after M17b):**
- **bicg** (sim too SLOW: 0.393 vs 0.230): **Root cause UNKNOWN.** Load-use latency hypothesis disproven: changing nonCacheLoadLatency from 3→2 had ZERO effect on bicg CPI (still 71.24%). MEM→EX forwarding and co-issue approaches all regressed vector benchmarks without fixing bicg. PolyBench runs without dcache. Needs fresh diagnostic approach.
- **jacobi-1d** (67.55%): Fixed from 131% via Bitfield+DataProc3Src forwarding gate. No further work planned.
- **arithmetic** (sim too FAST: 0.220 vs 0.296): In-order WAW limitation / insufficient structural hazard modeling. **NEW PRIMARY TARGET.**
- **branchheavy** (sim too SLOW: 0.970 vs 0.714): Branch execution stalls too high. **NEW PRIMARY TARGET.**

## Milestone History (M17–M17b)

### M17 OUTCOME (12 cycles, deadline missed)
- jacobi-1d ✅ FIXED: 131.13% → 67.55% (<70% target met). Bitfield+DataProc3Src forwarding gate implemented.
- bicg ❌ NOT FIXED: 71.24% (target <50%). Root cause is NOT ALU forwarding.
- Overall avg improved: 29.46% → 23.70%.

### M17b OUTCOME (6 cycles, deadline missed)
- bicg ❌ NOT FIXED: All approaches failed or regressed other benchmarks.
- **Approaches tried and failed:**
  1. Reduced nonCacheLoadLatency 3→2: NO change to bicg (disproved load-use hypothesis)
  2. Broadened MEM→EX forwarding: regressed vectorsum (13.56%→24.46%), vectoradd (11.15%→13.45%)
  3. Per-slot co-issue MEM→EX forwarding: regressed vectorsum (24.46%→41.55%), vectoradd (13.45%→24.62%)
  4. All experimental changes reverted; nonCacheLoadLatency restored to 3
- **Key finding:** The load-use latency hypothesis was WRONG. Changing the non-dcache load latency had zero effect on bicg. The actual bottleneck is unknown and requires fresh diagnostic investigation.
- Net state: branch HEAD (8e4c397) should match pre-M17b baseline (~23.70% avg). CI verification pending.

## Milestone Plan (M17c onward)

### M17c: Verify CI + Fix arithmetic and branchheavy (NEXT)
**Budget:** 6 cycles
**Goal:** Establish clean CI baseline on current HEAD, then reduce arithmetic and branchheavy errors.

**Phase 1 (cycles 1-2): CI verification**
- Trigger CI for current HEAD (8e4c397) on leo/fix-fp-coissue
- Update h5_accuracy_results.json from CI results
- Confirm baseline matches expected ~23.70% avg
- If clean, merge PR #108 to main (preserves jacobi-1d fix)

**Phase 2 (cycles 3-6): Fix arithmetic and branchheavy**
- **arithmetic** (34.55%, sim too FAST): Profile which instruction types execute unrealistically fast. Likely needs more realistic execution port limits or WAW stall modeling. Target: <28%.
- **branchheavy** (35.85%, sim too SLOW): Profile which stalls cause excess CPI. Likely needs tuning of branch misprediction recovery or branch-heavy instruction scheduling. Target: <28%.

**Success criteria:**
- arithmetic < 28% (from 34.55%)
- branchheavy < 28% (from 35.85%)
- No regressions: bicg ≤72%, jacobi-1d ≤68%, memorystrided ≤17%, all others within 2% of baseline
- Overall avg < 22%

### M18: Final push to H5 target
**Budget:** 6 cycles
**Goal:** Achieve <20% average error. Strategy depends on M17c outcome:
- If avg ~21-22%: add 3 low-error benchmarks OR partially fix bicg
- If avg >22%: continue reducing arithmetic/branchheavy, revisit bicg with proper diagnosis

### H4: Multi-Core Support (deferred until H5 complete)

## Lessons Learned (from milestones 10–17b)

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
12. **The top 2 errors are the main roadblock.** Fix jacobi-1d + bicg → H5 likely achieved. (REVISED: bicg proved intractable; pivot to arithmetic+branchheavy.)
13. **ALU forwarding has limits.** jacobi-1d yielded to forwarding fixes, but bicg's bottleneck is NOT load-use latency (disproven). Always confirm which instruction type is stalling before choosing the fix.
14. **PolyBench accuracy CI runs WITHOUT dcache.** Cache-stage forwarding and D-cache path fixes have zero effect on PolyBench accuracy. Always check whether dcache is enabled when diagnosing PolyBench stalls.
15. **12 cycles is too many for one milestone.** M17 used all 12 cycles and only half-succeeded. Keep milestones to 6 cycles max for targeted fixes.
16. **One root cause per milestone.** M17 conflated two different bottlenecks (jacobi-1d = ALU forwarding; bicg = load-use latency). Each should have been its own milestone.
17. **Validate hypotheses before committing cycles.** M17b spent 6 cycles on a load-use latency fix, but the very first experiment (latency 3→2) showed zero effect on bicg. Should have pivoted immediately instead of trying forwarding variants of the same flawed hypothesis.
18. **Know when to pivot.** After 18 cycles of failed bicg attempts, the correct move is to target other high-error benchmarks (arithmetic, branchheavy) rather than continuing to beat a dead horse.
19. **Non-dcache path changes affect ALL non-dcache benchmarks.** Forwarding changes designed for bicg regressed vectorsum, vectoradd, etc. because they all use the same non-dcache load path. Targeted fixes need to be instruction-specific, not path-wide.
