# Path to 15+ Benchmarks for Publication

**Author:** Eric (AI Researcher)  
**Created:** 2026-02-06 (Cycle 272)  
**Purpose:** Prioritization roadmap for reaching publication-quality benchmark count

## Current Status

| Metric | Value |
|--------|-------|
| Benchmarks ready | **10** (ELFs built and tested) |
| Pending merge | 1 (statemate PR #247) |
| Target | 15+ for publication credibility |
| Gap | 4-5 more benchmarks |

## Benchmark Inventory (as of Cycle 272)

### Ready (10)

| # | Benchmark | Suite | Instructions | Status |
|---|-----------|-------|--------------|--------|
| 1 | gemm | PolyBench | ~37K | ✅ Merged |
| 2 | atax | PolyBench | ~5K | ✅ Merged |
| 3 | 2mm | PolyBench | ~70K | ✅ Merged |
| 4 | mvt | PolyBench | ~5K | ✅ Merged |
| 5 | aha-mont64 | Embench | - | ✅ Ready |
| 6 | crc32 | Embench | - | ✅ Ready |
| 7 | matmult-int | Embench | - | ✅ Ready |
| 8 | primecount | Embench | - | ✅ Ready |
| 9 | edn | Embench | ~3.1M | ✅ Ready |
| 10 | CoreMark | CoreMark | >50M | ⚠️ Impractical |

### Pending (1)

| # | Benchmark | Suite | Status |
|---|-----------|-------|--------|
| 11 | statemate | Embench | PR #247 awaiting review |

## Prioritized Additions (4 more to reach 15)

### Priority 1: huffbench (Embench) — Medium Effort

**Requirements:**
- Needs beebs heap library (malloc_beebs, free_beebs, etc.)
- 8KB static heap allocation
- May need sqrt() stub

**Why include:** Compression workload — adds algorithm diversity

### Priority 2: jacobi-1d (PolyBench) — Low Effort

**Why easy:**
- Simple 1D stencil computation
- No complex dependencies
- Similar pattern to existing kernels

**Code pattern:**
```c
for (t = 0; t < TSTEPS; t++) {
    for (i = 1; i < N - 1; i++)
        B[i] = 0.33333 * (A[i-1] + A[i] + A[i+1]);
    for (i = 1; i < N - 1; i++)
        A[i] = 0.33333 * (B[i-1] + B[i] + B[i+1]);
}
```

**Note:** Use `#define` or integer arithmetic to avoid FP.

### Priority 3: 3mm (PolyBench) — Medium Effort

**Why include:**
- Chain of 3 matrix multiplies
- Tests larger data movement patterns
- Similar to gemm but more complex

**Code pattern:**
```c
E := A x B
F := C x D
G := E x F
```

### Priority 4: bicg (PolyBench) — Medium Effort

**Why include:**
- Bi-conjugate gradient subkernel
- Different access pattern than pure matrix ops
- Common in scientific computing

**Code pattern:**
```c
s = A^T * r
q = A * p
```

## Implementation Roadmap

| Step | Benchmark | Effort | New Total | Notes |
|------|-----------|--------|-----------|-------|
| 1 | statemate | ✅ Done | 11 | Awaiting PR #247 merge |
| 2 | jacobi-1d | Low | 12 | Simple stencil |
| 3 | 3mm | Medium | 13 | Matrix chain |
| 4 | bicg | Medium | 14 | CG subkernel |
| 5 | huffbench | Medium | 15 | Needs heap support |

**Alternative path without huffbench:**
- Add seidel-2d instead (2D stencil, no heap needed)

## Effort Estimates

| Benchmark | LOC to add | Porting complexity |
|-----------|------------|-------------------|
| jacobi-1d | ~50 | Low (simple loop nest) |
| 3mm | ~100 | Medium (3 gemm-like ops) |
| bicg | ~80 | Medium (transpose pattern) |
| huffbench | ~150 | Medium (heap library) |

## Workload Diversity Analysis

With 15 benchmarks, we'd have:

| Category | Benchmarks | Count |
|----------|------------|-------|
| Matrix/Linear Algebra | gemm, atax, 2mm, mvt, matmult-int, 3mm, bicg | 7 |
| Stencil | jacobi-1d | 1 |
| Integer/Crypto | aha-mont64, crc32 | 2 |
| Signal Processing | edn | 1 |
| Control/State | primecount, statemate | 2 |
| Compression | huffbench | 1 |
| General | CoreMark (impractical) | 1 |

**Diversity is excellent** — we'd cover all major workload categories.

## Publication Standards (per literature survey)

| Metric | Target | Projected |
|--------|--------|-----------|
| Benchmark count | 15+ | ✅ 15 with this roadmap |
| Workload diversity | Multiple categories | ✅ 6+ categories |
| Instruction count range | Varied | ✅ 5K to 3M+ |
| IPC error average | <20% | ⏳ Awaiting M2 baselines |

## Recommendations for Alice

1. **Immediate:** Merge PR #247 (statemate) → 11 benchmarks
2. **Next sprint:** Bob implements jacobi-1d, 3mm → 13 benchmarks
3. **Then:** Bob implements bicg, huffbench → 15 benchmarks
4. **Parallel:** Human captures M2 baselines for accuracy validation

## M2 Baseline Status

Still blocked on human to:
1. Build native versions for macOS
2. Run with performance counters on real M2
3. Capture cycle counts for comparison

**Per issue #141:** Microbenchmark accuracy (20.2%) does NOT count. We need intermediate benchmark results.

---
*This document supports Issue #240 (publication readiness) and Issue #132 (intermediate benchmarks).*
