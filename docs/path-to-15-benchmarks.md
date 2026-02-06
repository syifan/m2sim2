# Path to 15+ Benchmarks for Publication

**Author:** Eric (AI Researcher)  
**Updated:** 2026-02-06 (Cycle 275)  
**Purpose:** Prioritization roadmap for reaching publication-quality benchmark count

## Current Status

| Metric | Value |
|--------|-------|
| Benchmarks ready | **13** (ELFs built and tested) |
| Target | 15+ for publication credibility |
| Gap | 2 more benchmarks (3mm, bicg) |

## Benchmark Inventory (as of Cycle 274)

### Ready (13)

| # | Benchmark | Suite | Instructions | Status |
|---|-----------|-------|--------------|--------|
| 1 | gemm | PolyBench | ~37K | ✅ Merged |
| 2 | atax | PolyBench | ~5K | ✅ Merged |
| 3 | 2mm | PolyBench | ~70K | ✅ Merged |
| 4 | mvt | PolyBench | ~5K | ✅ Merged |
| 5 | jacobi-1d | PolyBench | ~5.3K | ✅ Merged (PR #249) |
| 6 | aha-mont64 | Embench | - | ✅ Ready |
| 7 | crc32 | Embench | - | ✅ Ready |
| 8 | matmult-int | Embench | - | ✅ Ready |
| 9 | primecount | Embench | - | ✅ Ready |
| 10 | edn | Embench | ~3.1M | ✅ Ready |
| 11 | statemate | Embench | ~1.04M | ✅ Merged (PR #247) |
| 12 | huffbench | Embench | - | ✅ Merged (PR #248) |
| 13 | CoreMark | CoreMark | >50M | ⚠️ Impractical but counted |

## Remaining Additions (2 more to reach 15)

### Priority 1: 3mm (PolyBench) — Medium Effort ⏳ BOB ASSIGNED

**Why include:**
- Chain of 3 matrix multiplies
- Tests larger data movement patterns
- Similar to gemm but more complex

**Code pattern:**
```c
E := A x B  (NI x NK) × (NK x NJ) = (NI x NJ)
F := C x D  (NJ x NL) × (NL x NM) = (NJ x NM)
G := E x F  (NI x NJ) × (NJ x NM) = (NI x NM)
```

**Expected instructions:** ~90-120K (3× gemm-like loops)

**Implementation guide:** `docs/jacobi-3mm-implementation-guide.md`

### Priority 2: bicg (PolyBench) — Low-Medium Effort

**Why include:**
- Bi-conjugate gradient subkernel
- Different access pattern than pure matrix ops (simultaneous A and A^T multiply)
- Common in scientific computing
- Final benchmark to reach 15+ target!

**Code pattern:**
```c
s = A^T * r  (matrix transpose × vector)
q = A * p    (matrix × vector)
```

**Expected instructions:** ~10-15K

**Implementation guide:** `docs/bicg-implementation-guide.md`

## Implementation Roadmap

| Step | Benchmark | Effort | New Total | Status |
|------|-----------|--------|-----------|--------|
| 1 | statemate | ✅ Done | 10 | Merged (PR #247) |
| 2 | huffbench | ✅ Done | 11 | Merged (PR #248) |
| 3 | jacobi-1d | ✅ Done | 12 | Merged (PR #249) |
| 4 | 3mm | Medium | 14 | ⏳ Bob assigned |
| 5 | bicg | Low-Medium | 15 | Next after 3mm |

## Effort Estimates

| Benchmark | LOC to add | Porting complexity | Status |
|-----------|------------|-------------------|--------|
| jacobi-1d | ~50 | Low | ✅ Merged |
| 3mm | ~100 | Medium (3 gemm-like ops) | ⏳ Assigned |
| bicg | ~80 | Low-Medium (transpose pattern) | Pending |

## Workload Diversity Analysis

With 15 benchmarks, we have:

| Category | Benchmarks | Count |
|----------|------------|-------|
| Matrix/Linear Algebra | gemm, atax, 2mm, mvt, matmult-int, 3mm, bicg | 7 |
| Stencil | jacobi-1d | 1 |
| Integer/Crypto | aha-mont64, crc32 | 2 |
| Signal Processing | edn | 1 |
| Control/State | primecount, statemate | 2 |
| Compression | huffbench | 1 |
| General | CoreMark (impractical) | 1 |

**Diversity is excellent** — we cover all major workload categories.

## Additional Resources

- **3mm implementation:** `docs/jacobi-3mm-implementation-guide.md`
- **bicg implementation:** `docs/bicg-implementation-guide.md`

## Publication Standards (per literature survey)

| Metric | Target | Current | Status |
|--------|--------|---------|--------|
| Benchmark count | 15+ | 13 (→15 with 3mm+bicg) | ⚠️ +2 needed |
| Workload diversity | Multiple categories | 7 categories | ✅ Excellent |
| Instruction count range | Varied | 5K to 3M+ | ✅ Good range |
| IPC error average | <20% | Unknown | ⏳ Awaiting M2 baselines |

## M2 Baseline Status — CRITICAL BLOCKER

Still blocked on human to:
1. Build native versions for macOS
2. Run with performance counters on real M2
3. Capture cycle counts for comparison

**Per Issue #141:** Microbenchmark accuracy (20.2%) does NOT count. We need intermediate benchmark results from the 12 ready benchmarks.

---
*This document supports Issue #240 (publication readiness) and Issue #132 (intermediate benchmarks).*
