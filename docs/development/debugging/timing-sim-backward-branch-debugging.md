# Timing Simulator Backward Branch Debugging

**Author:** Eric (Researcher)
**Date:** 2026-02-05
**Related:** PR #233 timeout, Issue #232

## Problem Statement

PR #233 (hot branch benchmark) times out in CI even after reducing loop iterations from 16 to 4. The hot branch benchmark is the **only benchmark with actual backward branch loops**. All other benchmarks use unrolled loops or cold forward branches.

## Root Cause Analysis

### Key Finding: PSTATE Flag Hazard

The hot loop benchmark has this structure:
```asm
loop:
    SUB X0, X0, #1    ; X0 = X0 - 1 (does NOT set flags)
    CMP X0, #0        ; Sets PSTATE flags (Z=1 when X0==0)
    B.NE loop         ; Branch back if Z==0 (i.e., X0 != 0)
```

**The Critical Issue:** CMP+B.NE fusion does NOT happen for this benchmark.

### Why Fusion Fails

CMP+B.cond fusion requires:
1. CMP in decode slot 0 (IFID)
2. B.cond in decode slot 1 (IFID2)

For the hot loop, after the backward branch jumps back to `loop:`:
- IFID: SUB instruction (at loop target)
- IFID2: CMP instruction
- IFID3: B.NE instruction

CMP is in slot 1, not slot 0, so **fusion doesn't trigger**!

### Non-Fused Branch Execution Path

When fusion doesn't happen, the B.NE uses the non-fused path in stages.go:
```go
case insts.OpBCond:
    if idex.IsFused {
        // Fused: compute flags from CMP operands (bypasses PSTATE)
        n, z, c, v := ComputeSubFlags(idex.FusedRnVal, op2, idex.FusedIs64)
        conditionMet = EvaluateConditionWithFlags(inst.Cond, n, z, c, v)
    } else {
        // Non-fused: read condition from PSTATE
        conditionMet = s.checkCondition(inst.Cond)
    }
```

The non-fused path reads PSTATE from `s.regFile.PSTATE`.

### Pipeline Timing Hazard

The problem is a classic **flag dependency hazard**:

| Cycle | CMP (earlier) | B.NE (later) |
|-------|---------------|--------------|
| N     | Execute: computes flags | Decode: waiting |
| N     | **Sets PSTATE at cycle end** | — |
| N+1   | Memory stage | Execute: **reads PSTATE** |

In the current implementation:
1. CMP's execute stage calls `setSubFlags()` which updates `s.regFile.PSTATE`
2. This update happens at the END of the execute stage
3. B.NE's execute stage calls `checkCondition()` which reads `s.regFile.PSTATE`
4. But in pipelined execution, both happen in the SAME tick!

**Result:** B.NE may read stale PSTATE values from before CMP's update.

### Evidence from Existing Tests

Two tests are SKIPPED with this exact reason:
```go
// benchmarks/simple_test.go:61
t.Skip("Skipped: timing pipeline doesn't update PSTATE flags, causing infinite loop")

// benchmarks/branch_test.go:62  
t.Skip("Skipped: timing pipeline doesn't update PSTATE flags, causing infinite loop")
```

This confirms the issue is **known** but was worked around by:
1. Using CMP+B.cond fusion (works when CMP is in slot 0)
2. Using forward branches only (cold branches, each executed once)

## Why branchTakenConditional Works

The working benchmark uses this pattern:
```go
EncodeCMPImm(0, 0),    // CMP X0, #0
EncodeBCond(8, 10),    // B.GE +8 (forward, always taken)
```

This works because:
1. **CMP is in slot 0** → fusion happens
2. **Forward branch** → no repeated execution at same PC
3. **Always taken** → no need to check loop termination

## Proposed Solutions

### Option A: Fix PSTATE Forwarding (Recommended)

Add flag forwarding similar to register forwarding:

```go
// In hazard unit
func (h *HazardUnit) GetForwardedFlags(exmem *EXMEMRegister) (n, z, c, v bool) {
    if exmem.Valid && exmem.Inst.SetFlags {
        return exmem.N, exmem.Z, exmem.C, exmem.V
    }
    return h.regFile.PSTATE.N, h.regFile.PSTATE.Z, 
           h.regFile.PSTATE.C, h.regFile.PSTATE.V
}
```

**Changes needed:**
1. Add N, Z, C, V fields to EXMEMRegister
2. Set flags in execute stage to EXMEM register (not directly to PSTATE)
3. Forward flags to next B.cond execute
4. Write flags to PSTATE in memory/writeback stage

### Option B: Detect Flag Hazards and Stall

Add flag hazard detection:
```go
func (h *HazardUnit) HasFlagHazard(inst *insts.Instruction, idex *IDEXRegister) bool {
    // B.cond depends on flags
    if inst.Op == insts.OpBCond {
        // Check if previous instruction (in EXMEM) sets flags
        if h.exmem.Valid && h.exmem.Inst.SetFlags && !idex.IsFused {
            return true // Stall needed
        }
    }
    return false
}
```

**Pros:** Simple to implement
**Cons:** Adds stall cycles for all non-fused conditional branches

### Option C: Extend Fusion to Backward Branches

Modify fusion detection to handle cases where CMP is not in slot 0:

```go
// Check adjacent IFID slots for CMP+B.cond regardless of slot position
if IsBCond(decResult2.Inst) && p.ifid.Valid && IsCMP(previousDecResult) {
    // Fuse even if CMP is not in slot 0
}
```

**Pros:** Best performance
**Cons:** More complex, may have edge cases

### Option D: Skip Hot Branch in CI (Short-term Workaround)

```go
func GetMicrobenchmarks() []Benchmark {
    benchmarks := []Benchmark{...}
    if os.Getenv("CI") != "" {
        // Skip hot branch benchmark in CI
        benchmarks = filterOut(benchmarks, "branch_hot_loop")
    }
    return benchmarks
}
```

**Pros:** Unblocks CI immediately
**Cons:** Doesn't fix the underlying issue

## Recommendation

**Short-term:** Use Option D to unblock CI and PR #233
**Long-term:** Implement Option A (PSTATE forwarding) as the proper fix

The PSTATE forwarding fix will:
1. Enable hot branch benchmarks
2. Fix the skipped tests (TestCountdownLoop, etc.)
3. Improve timing accuracy for all non-fused conditional branches

## Related Files

- `timing/pipeline/stages.go` - Execute stage, setSubFlags, checkCondition
- `timing/pipeline/pipeline.go` - Pipeline tick functions, fusion detection
- `benchmarks/simple_test.go` - Skipped TestCountdownLoop
- `benchmarks/branch_test.go` - Skipped backward branch test

## References

- ARM Architecture Reference Manual - Condition flags
- PR #205 - Original PSTATE implementation
- PR #212 - CMP+B.cond fusion
- Issue #232 - Hot branch benchmark design
