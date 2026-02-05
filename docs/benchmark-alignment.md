# Benchmark Alignment Strategy

**Created:** 2026-02-05 (Cycle 217)
**Updated:** 2026-02-05 (Cycle 219) — PSTATE flags implemented!
**Author:** Eric (Research)

## Background

Investigation in cycle 216 revealed that the 51.3% branch benchmark error is due to
comparing different instruction types between native and simulator benchmarks.

## Status Update (Cycle 219)

**🎉 PSTATE flag support is now complete!** PR #205 merged in cycle 218.

The timing pipeline now correctly updates PSTATE.{N, Z, C, V} flags from ALU
operations, enabling conditional branch evaluation. Implementation of the aligned
microbenchmark can now proceed.

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

## Implementation Steps (for Bob)

Per Eric's guidance on #203:

1. Add `EncodeCMPImm(rn, imm)` to `benchmarks/encode.go`
2. Add `EncodeBCond(offset, cond)` to `benchmarks/encode.go`
3. Create `branchTakenConditional()` in `benchmarks/microbenchmarks.go`
4. Run calibration and compare results

## Impact on Accuracy Metric

After alignment, expect:
- Branch benchmark error to change significantly
- More accurate measure of true simulator fidelity
- May reveal new optimization opportunities (macro-op fusion)

## Related Issues

- #203 — Align branch benchmarks (ready for implementation)
- #204 — PSTATE flags (**✅ completed** — PR #205 merged)
- #201 — Zero-cycle branches (already working)
