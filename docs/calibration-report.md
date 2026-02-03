# M2Sim Calibration Report

**Date:** 2026-02-03  
**Author:** Bob (Coder Agent)

## Executive Summary

This report compares M2Sim simulator predictions against native M2 hardware execution to identify accuracy gaps and prioritize future improvements.

## Test Results

### Simulator Results (M2Sim Timing Mode)

| Benchmark             | Cycles | Instructions | CPI   | Exit Code |
|-----------------------|--------|--------------|-------|-----------|
| arithmetic_sequential | 24     | 20           | 1.200 | 4         |
| dependency_chain      | 44     | 20           | 2.200 | 20        |
| memory_sequential     | 45     | 20           | 2.250 | 32832     |
| function_calls        | 39     | 15           | 2.600 | 5         |
| branch_taken          | 29     | 10           | 2.900 | 5         |
| mixed_operations      | 42     | 18           | 2.333 | 100       |

### Native M2 Hardware Results

**Note:** Native results are dominated by ~18ms process startup overhead. Absolute cycle counts are not meaningful; focus on relative patterns.

| Benchmark             | Avg (ns) | Est. Cycles | Instructions | Notes                    |
|-----------------------|----------|-------------|--------------|--------------------------|
| arithmetic_sequential | 18.58ms  | ~63.6M      | 24           | Process overhead dominant|
| dependency_chain      | 18.52ms  | ~63.4M      | 24           | Process overhead dominant|
| memory_sequential     | 18.30ms  | ~63.4M      | 25           | Process overhead dominant|
| function_calls        | 18.67ms  | ~63.5M      | 18           | Process overhead dominant|
| branch_taken          | 18.45ms  | ~63.4M      | 15           | Process overhead dominant|
| mixed_operations      | 18.36ms  | ~63.5M      | 45           | Process overhead dominant|

## Analysis

### Key Findings

1. **Process Overhead Dominates Native Measurements**
   - All native benchmarks show ~18ms execution time
   - This is process startup overhead, not actual benchmark time
   - The actual benchmark code executes in nanoseconds
   - For accurate native cycles, Apple Instruments with CPU Counters is required

2. **Simulator Shows Expected CPI Patterns**
   - arithmetic_sequential: CPI=1.2 (near ideal IPC, independent operations)
   - dependency_chain: CPI=2.2 (higher due to data dependencies)
   - memory_sequential: CPI=2.25 (memory latency impact)
   - branch_taken: CPI=2.9 (branch penalty visible)
   - function_calls: CPI=2.6 (BL/RET overhead)

3. **Relative Performance Relationships (Simulator)**
   - dependency_chain > arithmetic_sequential (dependencies add stalls) ✓
   - branch_taken > dependency_chain (branch penalties) ✓
   - memory_sequential > arithmetic_sequential (memory latency) ✓

### Calibration Gaps Identified

#### Cannot Validate (Due to Process Overhead)
- Absolute cycle counts
- Absolute CPI values
- Real cache miss rates

#### Recommendations for Better Calibration

1. **Use Apple Instruments** (xctrace with CPU Counters template)
   - Provides true hardware cycle counts
   - No process overhead interference
   - See benchmarks/native/README.md for instructions

2. **Longer Running Benchmarks**
   - Current benchmarks are too short (~20 instructions)
   - 18ms process overhead >> benchmark execution time
   - Need benchmarks with millions of iterations

3. **Kernel-Mode Measurements**
   - Consider syscall-based timing to avoid process overhead
   - Or use perf events on Linux

## Feature Prioritization

Based on simulator CPI patterns, the following M5 features may have impact:

| Feature            | Current Impact | Priority | Notes                          |
|--------------------|----------------|----------|--------------------------------|
| Branch Prediction  | CPI penalty visible (2.9 for branches) | Medium | Already modeled, may need tuning |
| Out-of-Order       | Not modeled    | High     | Would reduce dependency stalls   |
| Cache Hierarchy    | Basic L1 modeled| Medium   | L2/L3 needed for real workloads  |
| SIMD (NEON)        | Not modeled    | TBD      | Need SIMD benchmarks to assess   |

## Next Steps

1. [ ] Add longer-running benchmark variants (1M+ iterations)
2. [ ] Run benchmarks with Apple Instruments CPU Counters
3. [ ] Create calibration script using xctrace
4. [ ] Once real cycle data available, tune simulator parameters

## Conclusion

The current benchmark infrastructure successfully validates **functional correctness** (exit codes match) and shows **reasonable CPI relationships** in the simulator. However, **absolute accuracy calibration** requires hardware performance counters, as process overhead makes timing measurements unusable.

The simulator's current CPI values (1.2-2.9 range) are realistic for an in-order pipeline with memory stalls and branch penalties. True calibration requires xctrace measurements or significantly longer-running benchmarks.
