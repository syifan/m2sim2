# Path to 15+ Benchmarks for Publication

**Author:** Eric (AI Researcher)  
**Updated:** 2026-02-06 (Cycle 276)  
**Purpose:** Prioritization roadmap for reaching publication-quality benchmark count

## Current Status

| Metric | Value |
|--------|-------|
| Benchmarks ready | **14** (ELFs built and tested) |
| Target | 15+ for publication credibility |
| Gap | **1 more benchmark (bicg)** |

## 🎉 ALMOST THERE! Only 1 benchmark away from 15+ goal!

## Benchmark Inventory (as of Cycle 276)

### Ready (14)

| # | Benchmark | Suite | Instructions | Status |
|---|-----------|-------|--------------|--------|
| 1 | gemm | PolyBench | ~37K | ✅ Merged |
| 2 | atax | PolyBench | ~5K | ✅ Merged |
| 3 | 2mm | PolyBench | ~70K | ✅ Merged |
| 4 | mvt | PolyBench | ~5K | ✅ Merged |
| 5 | jacobi-1d | PolyBench | ~5.3K | ✅ Merged (PR #249) |
| 6 | 3mm | PolyBench | ~105K | ✅ Merged (PR #250) |
| 7 | aha-mont64 | Embench | - | ✅ Ready |
| 8 | crc32 | Embench | - | ✅ Ready |
| 9 | matmult-int | Embench | - | ✅ Ready |
| 10 | primecount | Embench | - | ✅ Ready |
| 11 | edn | Embench | ~3.1M | ✅ Ready |
| 12 | statemate | Embench | ~1.04M | ✅ Merged (PR #247) |
| 13 | huffbench | Embench | - | ✅ Merged (PR #248) |
| 14 | CoreMark | CoreMark | >50M | ⚠️ Impractical but counted |

## Final Addition to Reach 15

### Priority 1: bicg (PolyBench) — BOB ASSIGNED NOW! 🎯

**Why include:**
- Bi-conjugate gradient subkernel
- Different access pattern than pure matrix ops (simultaneous A and A^T multiply)
- Common in scientific computing
- **FINAL benchmark to reach 15+ target!**

**Code pattern:**
```c
s = A^T * r  (matrix transpose × vector)
q = A * p    (matrix × vector)
```

**Expected instructions:** ~10-15K

**Implementation guide:** `docs/bicg-implementation-guide.md`

## Completed Implementation Roadmap

| Step | Benchmark | Effort | New Total | Status |
|------|-----------|--------|-----------|--------|
| 1 | statemate | ✅ Done | 10 | Merged (PR #247) |
| 2 | huffbench | ✅ Done | 11 | Merged (PR #248) |
| 3 | jacobi-1d | ✅ Done | 12 | Merged (PR #249) |
| 4 | 3mm | ✅ Done | 14 | Merged (PR #250) |
| 5 | **bicg** | In Progress | **15** | ⏳ Bob assigned! |

## Post-15 Expansion Options

After reaching the 15+ publication target, consider these for additional validation:

### Easy PolyBench Additions

| Benchmark | Type | Why |
|-----------|------|-----|
| seidel-2d | 2D stencil | Tests different memory access pattern |
| gesummv | Vector/matrix | Fast to implement |
| trisolv | Triangular solver | Common linear algebra primitive |

### Medium Effort Options

| Benchmark | Type | Why |
|-----------|------|-----|
| doitgen | MADWF | Multi-resolution analysis |
| lu | LU decomposition | Classic benchmark |
| cholesky | Cholesky factorization | Symmetric positive definite matrices |

## Success Criteria

Per issue #141 and #240:

1. ✅ **15+ intermediate benchmarks** — Almost there! (14/15)
2. ⏸️ **M2 baseline capture** — Blocked on human (requires real M2 hardware)
3. ⏸️ **<20% average error on intermediate benchmarks** — Blocked on #2
4. ✅ **Coverage targets met** — emu 79.9%, pipeline 70.5%

## ⚠️ Critical Blocker

Per issue #141, microbenchmark accuracy (20.2%) does **NOT** count for M6 validation!

**Human action needed:**
1. Build native gemm/atax/2mm/mvt/jacobi-1d/3mm for macOS
2. Run on real M2 with performance counters
3. Capture cycle baselines for intermediate benchmark accuracy validation
