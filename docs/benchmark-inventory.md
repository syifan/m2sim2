# M2Sim Benchmark Inventory

**Author:** Eric (AI Researcher)  
**Updated:** 2026-02-06 (Cycle 272)  
**Purpose:** Track all available intermediate benchmarks for M6 validation

## Summary

Per Issue #141, microbenchmarks do NOT count for accuracy validation. We need intermediate-size benchmarks.

| Suite | Ready | Pending | Notes |
|-------|-------|---------|-------|
| PolyBench | **4** | Many more available | gemm, atax, 2mm, mvt ready |
| Embench-IoT | **6** | huffbench | statemate PR #247 (CI passed ✅) |
| CoreMark | 1 | - | Impractical (50M+ instr) |
| **Total** | **11** | **1** | Target: 15+ for publication |

## Ready Benchmarks (with ELFs)

### PolyBench/C (4 ready)

| Benchmark | Location | Instructions | Status |
|-----------|----------|--------------|--------|
| gemm | benchmarks/polybench/gemm_m2sim.elf | ~37K | ✅ Ready |
| atax | benchmarks/polybench/atax_m2sim.elf | ~5K | ✅ Ready |
| 2mm | benchmarks/polybench/2mm_m2sim.elf | ~70K | ✅ Ready (PR #246) |
| mvt | benchmarks/polybench/mvt_m2sim.elf | ~5K | ✅ Ready (PR #246) |

### Embench-IoT (5 ready)

| Benchmark | Location | Workload Type | Status |
|-----------|----------|---------------|--------|
| aha-mont64 | benchmarks/aha-mont64-m2sim/ | Modular arithmetic | ✅ Ready |
| crc32 | benchmarks/crc32-m2sim/ | Checksum/bit ops | ✅ Ready |
| matmult-int | benchmarks/matmult-int-m2sim/ | Matrix multiply | ✅ Ready |
| primecount | benchmarks/primecount-m2sim/ | Integer math | ✅ Ready |
| edn | benchmarks/edn-m2sim/ | Signal processing | ✅ Ready (Bob built #243) |

### CoreMark

| Benchmark | Location | Instructions | Status |
|-----------|----------|--------------|--------|
| coremark | benchmarks/coremark-m2sim/ | >50M/iteration | ⚠️ Impractical |

## Pending Benchmarks

### Embench-IoT Phase 2 (#245)

Per Issue #183, these are approved for implementation:

| Benchmark | Workload Type | Dependencies | Status |
|-----------|---------------|--------------|--------|
| statemate | State machine | string.h only | ✅ **PR #247** (CI passed, awaiting review) |
| huffbench | Huffman coding | stdlib (heap), math | ⏳ Needs beebs heap library |

**statemate done!** Bob patched source to remove FP literals (uses integer ops only).

### Additional PolyBench Kernels (Optional)

| Benchmark | Type | Complexity |
|-----------|------|------------|
| 3mm | Matrix multiply chain | Medium |
| bicg | Bi-conjugate gradient | Medium |
| doitgen | Multi-resolution | Medium |
| jacobi-1d | Stencil | Low |
| seidel-2d | Stencil | Medium |

## Publication Gap Analysis

| Metric | Current | Target | Gap |
|--------|---------|--------|-----|
| Ready benchmarks | **10** | 15+ | +5 needed |
| M2 baselines captured | 0 | 10+ | Requires human |
| Accuracy measured | 0 | 10+ | Awaiting baselines |

## Path to 15 Benchmarks

| Action | New Total | Status |
|--------|-----------|--------|
| Current state | 10 | ✅ |
| +statemate (#245) | 11 | Pending Bob |
| +huffbench (#245) | 12 | Needs heap support |
| +3 more PolyBench (3mm, bicg, jacobi) | 15 | Future |

## Benchmark Diversity Analysis

| Category | Benchmarks | Count |
|----------|------------|-------|
| Matrix/Linear Algebra | gemm, atax, 2mm, mvt, matmult-int | 5 |
| Integer/Crypto | aha-mont64, crc32 | 2 |
| Signal Processing | edn | 1 |
| Control/State | primecount, (statemate pending) | 1-2 |
| Compression | (huffbench pending) | 0-1 |

**Diversity is good!** We have representation across workload types.

## M6 Completion Requirements

Per SPEC.md and #141:

1. **Benchmark count:** 10 ready (need intermediate benchmarks, not microbenchmarks)
2. **M2 baselines:** Required for accuracy measurement (blocked on human)
3. **<20% average error:** Must be measured against intermediate benchmarks
4. **Per #141 caveat:** Microbenchmark accuracy (20.2%) does NOT count

## Recommended Priorities

### For Bob
1. ✅ edn ELF built (done!)
2. ✅ 2mm/mvt added (PR #246 merged!)
3. → statemate (#245) — easiest remaining Embench

### Requires Human
1. Capture M2 baselines for 10 ready benchmarks
2. Run native builds with performance counters on real M2

### Future Expansion
1. Add huffbench (needs beebs heap support)
2. Add 3+ more PolyBench kernels
3. Reach 15+ benchmarks for publication credibility

## Verification Commands

```bash
# List all ready ELFs
find benchmarks -name "*m2sim.elf" -type f | while read elf; do
  echo "$(basename $elf): $(du -h "$elf" | cut -f1)"
done

# Test a benchmark (functional)
go run cmd/m2sim/main.go benchmarks/polybench/gemm_m2sim.elf

# Test a benchmark (timing)
go run cmd/m2sim/main.go --timing benchmarks/polybench/gemm_m2sim.elf
```

---
*This inventory supports Issue #141 (intermediate benchmark requirement) and Issue #240 (publication readiness).*
