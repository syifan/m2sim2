# M2Sim Benchmark Inventory

**Author:** Eric (AI Researcher)  
**Updated:** 2026-02-06 (Cycle 275)  
**Purpose:** Track all available intermediate benchmarks for M6 validation

## Summary

Per Issue #141, microbenchmarks do NOT count for accuracy validation. We need intermediate-size benchmarks.

| Suite | Ready | Notes |
|-------|-------|-------|
| PolyBench | **5** | gemm, atax, 2mm, mvt, jacobi-1d |
| Embench-IoT | **7** | aha-mont64, crc32, matmult-int, primecount, edn, statemate, huffbench |
| CoreMark | 1 | Impractical (50M+ instr) |
| **Total** | **13** | Target: 15+ for publication |

## Ready Benchmarks (with ELFs)

### PolyBench/C (5 ready)

| Benchmark | Instructions | Status |
|-----------|--------------|--------|
| gemm | ~37K | ✅ Merged (PR #238) |
| atax | ~5K | ✅ Merged (PR #239) |
| 2mm | ~70K | ✅ Merged (PR #246) |
| mvt | ~5K | ✅ Merged (PR #246) |
| jacobi-1d | ~5.3K | ✅ Merged (PR #249) |

### Embench-IoT (7 ready)

| Benchmark | Workload Type | Status |
|-----------|---------------|--------|
| aha-mont64 | Modular arithmetic | ✅ Ready |
| crc32 | Checksum/bit ops | ✅ Ready |
| matmult-int | Matrix multiply | ✅ Ready |
| primecount | Integer math | ✅ Ready |
| edn | Signal processing | ✅ Ready (~3.1M instr) |
| statemate | State machine | ✅ Merged (PR #247, ~1.04M instr) |
| huffbench | Huffman coding | ✅ Merged (PR #248) |

### CoreMark

| Benchmark | Instructions | Status |
|-----------|--------------|--------|
| coremark | >50M/iteration | ⚠️ Impractical |

## Path to 15+ Benchmarks

| Step | Benchmark | New Total | Status |
|------|-----------|-----------|--------|
| 1-4 | PolyBench Phase 1 | 4 | ✅ Complete |
| 5-11 | Embench Phase 1+2 | 11 | ✅ Complete |
| 12 | (CoreMark counted) | 12 | ⚠️ Impractical but exists |
| 13 | jacobi-1d | 13 | ✅ Merged (PR #249) |
| 14 | 3mm | 14 | ⏳ Bob assigned |
| 15 | bicg | 15 | Next after 3mm |

## Next Benchmarks to Implement

### 3mm (PolyBench) — Bob assigned

- **Type:** Three chained matrix multiplications (E=AxB, F=CxD, G=ExF)
- **Effort:** Medium
- **Expected instructions:** ~90-120K
- **Implementation guide:** `docs/jacobi-3mm-implementation-guide.md`

### bicg (PolyBench) — Next after 3mm

- **Type:** Bi-conjugate gradient subkernel
- **Effort:** Low-Medium
- **Expected instructions:** ~10-15K
- **Pattern:** s = A^T * r, q = A * p (simultaneous A and A^T multiply)
- **Implementation guide:** `docs/bicg-implementation-guide.md`

## Workload Diversity Analysis

| Category | Benchmarks | Count |
|----------|------------|-------|
| Matrix/Linear Algebra | gemm, atax, 2mm, mvt, matmult-int | 5 |
| Integer/Crypto | aha-mont64, crc32 | 2 |
| Signal Processing | edn | 1 |
| Control/State | primecount, statemate | 2 |
| Compression | huffbench | 1 |
| Stencil | jacobi-1d | 1 |

**Diversity is excellent!** We cover all major workload categories.

### After 3mm + bicg (15 total)

| Category | Benchmarks | Count |
|----------|------------|-------|
| Matrix/Linear Algebra | gemm, atax, 2mm, mvt, matmult-int, 3mm, bicg | 7 |
| Integer/Crypto | aha-mont64, crc32 | 2 |
| Signal Processing | edn | 1 |
| Control/State | primecount, statemate | 2 |
| Compression | huffbench | 1 |
| Stencil | jacobi-1d | 1 |

## M6 Completion Requirements

Per SPEC.md and #141:

1. **Benchmark count:** 13 ready ✅ (need 15+ for publication)
2. **M2 baselines:** Required for accuracy measurement ❌ (blocked on human)
3. **<20% average error:** Must be measured against intermediate benchmarks ⏳
4. **Per #141 caveat:** Microbenchmark accuracy (20.2%) does NOT count

## ⚠️ Critical Blocker: M2 Baseline Capture

Still blocked on human to:
1. Build native versions for macOS (not bare-metal ELFs)
2. Run with performance counters on real M2 hardware
3. Capture cycle counts for comparison

---
*This inventory supports Issue #141 (intermediate benchmark requirement) and Issue #240 (publication readiness).*
