# M2 Microarchitecture Research

## Summary

This document contains research findings on Apple M2's CPU microarchitecture to help improve M2Sim's accuracy.

## Core Architecture

The M2 uses **Avalanche** performance cores (P-cores) and **Blizzard** efficiency cores (E-cores), first introduced in the A15 Bionic.

### Avalanche (Performance Core) Specifications

| Feature | M2 (Avalanche) | M2Sim Current |
|---------|----------------|---------------|
| Decode Width | 8-wide | 6-wide |
| ROB Size | ~630 entries | N/A (in-order) |
| Integer ALUs | 7 | 6 (sextuple) |
| FP/Vector Units | 4 | - |
| Load/Store Units | 4 | 2 |
| L1 I-Cache | 192 KB | 32 KB |
| L1 D-Cache | 128 KB | 32 KB |
| Branch Mispred. Penalty | ~14 cycles | ~5 cycles |

### Key Findings

1. **Decode Width Mismatch**: M2 decodes 8 instructions/cycle, but M2Sim simulates 6-wide max. This partially explains the arithmetic throughput gap.

2. **Execution Units**: M2 has **7 integer ALUs** (vs our 6), **4 FPUs** (for SIMD operations), and **4 load/store units** (vs our ~2).

3. **Out-of-Order Execution**: M2 is deeply out-of-order with a ~630 entry reorder buffer. M2Sim uses an in-order pipeline, which fundamentally limits ILP exploitation.

4. **Cache Hierarchy**:
   - L1 I-Cache: 192 KB (we simulate 32 KB)
   - L1 D-Cache: 128 KB (we simulate 32 KB)
   - Larger caches mean fewer misses and better performance

5. **Branch Prediction**: M2 has sophisticated branch prediction with ~14 cycle misprediction penalty. Modern Apple CPUs use perceptron-based predictors, not just bimodal.

## Accuracy Gap Analysis

### Arithmetic Sequential (49.3% error)
- **Cause**: 0.400 CPI (sim) vs 0.268 CPI (real)
- **Root Issue**: With 8-wide decode and 7 ALUs, M2 can execute more independent ALU operations per cycle
- **Fix**: Increase to 8-wide issue and add more ALU slots

### Dependency Chain (18.9% error)
- **Cause**: 1.200 CPI (sim) vs 1.009 CPI (real)
- **Root Issue**: Out-of-order execution allows M2 to better hide latencies
- **Fix**: Would require OoO simulation (significant undertaking)

### Branch Taken (51.3% error)
- **Cause**: 1.800 CPI (sim) vs 1.190 CPI (real)
- **Root Issue**: M2's branch predictor is more sophisticated; also OoO execution hides some misprediction penalty
- **Fix**: Implement more advanced predictor (tournament, perceptron)

## Recommended Parameters

For better accuracy without implementing OoO:

```go
// Proposed pipeline configuration
IssueWidth: 8              // Match M2's decode width
IntegerALUs: 7             // Match M2's ALU count
LoadStoreUnits: 4          // Match M2's LS units
L1ICacheSize: 192 * 1024   // 192 KB
L1DCacheSize: 128 * 1024   // 128 KB
BranchMispredPenalty: 14   // cycles
```

## Limitations

Even with parameter tuning, M2Sim will have accuracy limitations because:

1. **No OoO Execution**: The reorder buffer and speculative execution allow M2 to exploit ILP that an in-order simulator cannot model.

2. **No Register Renaming**: M2 has many physical registers for renaming, enabling more parallelism.

3. **No Macro-op Fusion**: M2 can fuse certain instruction pairs (like CMP+branch) into single operations.

4. **No Zero-Cycle Moves**: Some register-to-register moves may be eliminated.

## SPEC Integration Notes

For SPEC benchmark integration:
1. SPEC CPU 2017 binaries need ARM64 compilation
2. Focus on SPECint (integer) benchmarks initially
3. Consider extracting hot loops rather than full benchmarks
4. Real M2 timing requires performance counters (CNTVCT_EL0, PMU)

## References

- [NamuWiki Apple Microarchitecture](https://en.namu.wiki/w/Apple/마이크로아키텍처)
- [AnandTech M2 Analysis](https://www.anandtech.com/show/17431/apple-announces-m2-soc-apple-silicon-updated-for-2022)
- MDPI: Reverse Engineering the BTB Organizations on Apple M2

---

*Research compiled by Cathy during M2Sim cycle #126*
