# M2Sim Benchmark Inventory

**Author:** Eric (AI Researcher)  
**Updated:** 2026-02-06 (Cycle 276)  
**Purpose:** Track all available intermediate benchmarks for M6 validation

## Summary

Per Issue #141, microbenchmarks do NOT count for accuracy validation. We need intermediate-size benchmarks.

| Suite | Ready | Notes |
|-------|-------|-------|
| PolyBench | **6** | gemm, atax, 2mm, mvt, jacobi-1d, 3mm |
| Embench-IoT | **7** | aha-mont64, crc32, matmult-int, primecount, edn, statemate, huffbench |
| CoreMark | 1 | Impractical (50M+ instr) |
| **Total** | **14** | Target: 15+ for publication |

## Ready Benchmarks (with ELFs)

### PolyBench/C (6 ready)

| Benchmark | Instructions | Status |
|-----------|--------------|--------|
| gemm | ~37K | ✅ Merged (PR #238) |
| atax | ~5K | ✅ Merged (PR #239) |
| 2mm | ~70K | ✅ Merged (PR #246) |
| mvt | ~5K | ✅ Merged (PR #246) |
| jacobi-1d | ~5.3K | ✅ Merged (PR #249) |
| 3mm | ~105K | ✅ Merged (PR #250) |

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
| 14 | 3mm | 14 | ✅ Merged (PR #250) |
| 15 | bicg | 15 | ⏳ Bob assigned |

## Next Benchmark to Implement

### bicg (PolyBench) — FINAL STRETCH TO 15!

- **Type:** Bi-conjugate gradient subkernel
- **Effort:** Low-Medium
- **Expected instructions:** ~10-15K
- **Pattern:** s = A^T * r, q = A * p (simultaneous A and A^T multiply)
- **Implementation guide:** `docs/bicg-implementation-guide.md`

## Workload Diversity Analysis

| Category | Benchmarks | Count |
|----------|------------|-------|
| Matrix/Linear Algebra | gemm, atax, 2mm, mvt, 3mm, matmult-int | 6 |
| Stencil Computation | jacobi-1d | 1 |
| Signal Processing | edn | 1 |
| Compression | huffbench | 1 |
| State Machine | statemate | 1 |
| Cryptographic | aha-mont64 | 1 |
| Checksum/CRC | crc32 | 1 |
| General Integer | primecount | 1 |
| Industry Standard | CoreMark (impractical) | 1 |

**Excellent diversity** — 9 different workload categories covered!

## Post-15 Benchmark Candidates

After reaching 15+ target, consider these for further expansion:

### PolyBench Candidates (Easy to add)

| Benchmark | Type | Estimated Effort |
|-----------|------|------------------|
| seidel-2d | 2D stencil | Low |
| doitgen | MADWF | Medium |
| trisolv | Triangular solver | Low |
| gesummv | Scalar/vector/matrix | Low |
| symm | Symmetric matrix ops | Medium |

### Embench Candidates (Some require FP support)

| Benchmark | Type | Notes |
|-----------|------|-------|
| md5sum | Cryptographic hash | May need additional headers |
| nbody | N-body simulation | Needs FP support |
| nettle-aes | Encryption | Complex dependencies |

## ⚠️ Critical Blocker: M2 Baseline Capture

Per issue #141, microbenchmark accuracy (20.2%) does NOT count for M6 validation!

**Blocked on human to:**
1. Build native gemm/atax for macOS
2. Run on real M2 with performance counters
3. Capture cycle baselines for intermediate benchmark validation
