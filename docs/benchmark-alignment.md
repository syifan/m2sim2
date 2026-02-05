# Benchmark Alignment Strategy

**Created:** 2026-02-05 (Cycle 217)
**Author:** Eric (Research)

## Background

Investigation in cycle 216 revealed that the 51.3% branch benchmark error is due to
comparing different instruction types between native and simulator benchmarks.

## Current Mismatch

| Metric | Native Benchmark | Simulator Micro |
|--------|-----------------|-----------------|
| File | branch_taken_long.s | branchTaken() |
| Branch Type | Conditional | Unconditional |
| Instructions | `cmp` + `b.ge` | `B` |
| Expected CPI | Higher (compare + branch) | Lower (branch only) |

## Why This Matters

Unconditional branches (`B`) can be:
- Fully eliminated at fetch time (zero-cycle)
- No prediction needed — always taken

Conditional branches (`b.ge`) require:
- Compare instruction execution
- Branch prediction
- Possible misprediction penalty

Comparing these is meaningless — they're fundamentally different workloads.

## Alignment Strategy

### Recommended: Align to Conditional

Modify the simulator microbenchmark to use conditional branches:

```assembly
// Old (unconditional)
loop:
    ADD X0, X0, #1
    B loop

// New (conditional)
loop:
    ADD X0, X0, #1
    CMP X0, X10      // compare against limit
    B.LT loop        // conditional branch
```

**Benefits:**
- More realistic workload
- Matches typical program patterns
- Exposes actual conditional branch overhead

### Alternative: Align to Unconditional

Modify native benchmark to use unconditional branches.

**Drawbacks:**
- Less realistic
- Unconditional loops are rare in real code

## Impact on Accuracy Metric

After alignment, expect:
- Branch benchmark error to change significantly
- More accurate measure of true simulator fidelity
- May reveal new optimization opportunities (macro-op fusion)

## Implementation

See issue #203 for tracking.

1. Create new microbenchmark with conditional branches
2. Re-run calibration
3. Update baseline metrics
4. Compare results

## Related Issues

- #203 — Align branch benchmarks
- #201 — Zero-cycle branches (already working)
- #199 — Branch predictor investigation (closed)
