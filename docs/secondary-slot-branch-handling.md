# Missing Branch Handling for Secondary Slots (2-8) in 8-Wide Pipeline

**Author:** Eric (Researcher)  
**Date:** 2026-02-05  
**Issue:** PR #233 acceptance tests timeout despite PSTATE forwarding fix

## Executive Summary

**The PSTATE forwarding fix (48851e7) is correct but incomplete.** The acceptance tests still hang because **branches in secondary slots (2-8) are not handled at all**.

The branch prediction verification and misprediction handling code only exists for the **primary slot (slot 0, `p.idex`)**. When B.NE lands in slot 2 (tertiary), the branch executes and the condition is correctly evaluated (thanks to Cathy's fix), but:
- The branch decision is **never acted upon**
- The PC is **never redirected**
- The pipeline is **never flushed**

## Evidence

Searching for `IsBranch` handling in `pipeline.go`:

```
Line 636:  if p.idex.IsBranch {   // single-issue primary slot
Line 1029: if p.idex.IsBranch {   // dual-issue primary slot
Line 1711: if p.idex.IsBranch {   // quad-issue primary slot
Line 2699: if p.idex.IsBranch {   // sextuple-issue primary slot
Line 3981: if p.idex.IsBranch {   // octuple-issue primary slot
```

**No `p.idex2.IsBranch`, `p.idex3.IsBranch`, etc.** - secondary slots have NO branch handling.

## The Bug Flow

Hot branch loop structure:
```
Slot 0: SUB X0, X0, #1   (p.idex)  - not a branch
Slot 1: CMP X0, #0       (p.idex2) - sets flags
Slot 2: B.NE loop        (p.idex3) - IS a branch, but...
```

When B.NE executes in slot 2:
1. ✅ PSTATE flag forwarding works (Cathy's fix)
2. ✅ `ExecuteWithFlags()` correctly evaluates `BranchTaken = true`
3. ✅ Result stored in `nextEXMEM3`
4. ❌ **No code checks `p.idex3.IsBranch`**
5. ❌ **No branch misprediction verification**
6. ❌ **No pipeline flush**
7. ❌ **PC never redirected back to loop start**

Result: The loop counter decrements but the branch never happens → program runs off into invalid memory or loops forever.

## Why Unit Tests Pass

Unit tests run in **single-issue mode** (default). In single-issue:
- Only one instruction executes per cycle
- B.NE always lands in slot 0 (primary slot)
- Primary slot HAS branch handling
- ✅ Branch correctly redirects execution

## Solution

Add branch prediction verification and misprediction handling for **all secondary slots (2-8)** in `tickOctupleIssue()`.

For each slot, after executing a B.cond instruction:

```go
// Branch prediction verification for tertiary slot
if p.idex3.IsBranch {
    actualTaken := execResult.BranchTaken
    actualTarget := execResult.BranchTarget
    
    p.stats.BranchPredictions++
    
    predictedTaken := p.idex3.PredictedTaken
    predictedTarget := p.idex3.PredictedTarget
    
    wasMispredicted := false
    if actualTaken != predictedTaken {
        wasMispredicted = true
    } else if actualTaken && predictedTarget != actualTarget {
        wasMispredicted = true
    }
    
    p.branchPredictor.Update(p.idex3.PC, actualTaken, actualTarget)
    
    if wasMispredicted {
        p.stats.BranchMispredictions++
        branchTarget := actualTarget
        if !actualTaken {
            branchTarget = p.idex3.PC + 4
        }
        p.pc = branchTarget
        p.flushAllIFID()
        p.flushAllIDEX()
        p.stats.Flushes++
        
        // Clear all EXMEM registers after this slot
        nextEXMEM4.Clear()
        nextEXMEM5.Clear()
        // ... etc
        
        // Latch results and return early
        if !memStall {
            p.memwb = nextMEMWB
            // ... latch other registers
            p.exmem = nextEXMEM
            p.exmem2 = nextEXMEM2
            p.exmem3 = nextEXMEM3
            // Clear slots 4-8
        }
        return
    }
    p.stats.BranchCorrect++
}
```

This pattern must be added after each secondary slot execution (idex2, idex3, ..., idex8).

## Priority

**CRITICAL** - This is the actual root cause. The PSTATE fix was necessary but not sufficient.

## Files to Modify

- `timing/pipeline/pipeline.go` — Add branch handling for slots 2-8 in `tickOctupleIssue()`

## Verification

After fix:
1. Hot branch benchmark should complete (not hang)
2. FoldedBranches stat should show zero-cycle folding working
3. PR #233 CI acceptance tests should pass
