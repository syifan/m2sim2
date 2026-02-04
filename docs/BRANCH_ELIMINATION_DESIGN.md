# Branch Elimination Feature Design

**Author:** Cathy (QA)  
**Date:** 2026-02-04  
**Status:** Draft Design

## Overview

This document describes the design for implementing zero-cycle unconditional branch elimination in M2Sim, matching the behavior of Apple M2's Firestorm cores where unconditional branches (B, BL) never issue to execution units.

## Background

### Real M2 Behavior

According to [Dougall Johnson's Firestorm documentation](https://dougallj.github.io/applecpu/firestorm.html):

> **Instruction elimination (zero-cycle):**
> - `b` (unconditional branch) - **NEVER ISSUES**
> - `mov x0, #0` - handled by renaming
> - `mov x0, x1` - handled by renaming (64-bit only)
> - `nop` - never issues

This means unconditional branches are resolved during the rename/dispatch stage and do not consume any execution bandwidth. The branch target is computed speculatively at fetch time, and if prediction is correct, the branch has effectively zero cost.

### Current M2Sim Behavior

Currently, M2Sim has **partial** early branch resolution:

1. **Fetch stage:** `isUnconditionalBranch()` detects B/BL instructions and computes the target
2. **Prediction:** Target is stored in IFID register, PC is redirected speculatively
3. **Execute stage:** Branch is still "executed" and verified
4. **Instruction count:** Branch is still counted as a retired instruction

The problem: Even with correct prediction, the branch consumes a pipeline slot and counts toward instruction count. This inflates CPI for branch-heavy code.

## Proposed Design

### Phase 1: True Instruction Elimination

**Goal:** Unconditional branches (B) should not consume execution resources when predicted correctly.

**Key Insight:** Since we already resolve B/BL at fetch time via `isUnconditionalBranch()`, we can simply NOT issue these instructions to the decode stage at all. Instead:

1. Detect unconditional B instruction at fetch
2. Update PC immediately to target (already done)
3. **Skip** adding the instruction to IFID register
4. Fetch the next instruction from the target address instead

**Implementation Location:** `tickQuadIssue()` and `tickSuperscalar()` in `pipeline.go`

```go
// In fetch stage, after detecting unconditional branch:
isUncondBranch, uncondTarget := isUnconditionalBranch(word, p.pc)
if isUncondBranch {
    // Check if this is a non-linking branch (pure B, not BL)
    isNonLinkingBranch := (word >> 31) == 0  // B has bit 31 = 0, BL has bit 31 = 1
    
    if isNonLinkingBranch {
        // Eliminate the branch - don't issue it, just redirect PC
        p.pc = uncondTarget
        p.stats.EliminatedBranches++  // New stat
        // Continue to fetch from target (don't add to IFID)
        continue  // or equivalent control flow
    }
    // For BL: still needs to execute (writes to X30)
}
```

### Phase 2: BL Handling (Branch with Link)

BL is trickier because it writes to X30 (link register). Options:

**Option A: Eliminate + Inject X30 Write (Recommended)**
- Eliminate BL from normal pipeline
- Inject a synthetic "write X30 = PC+4" operation
- This is what real M2 likely does via renaming

**Option B: Keep BL in Pipeline**
- BL still flows through pipeline normally
- Only pure B is eliminated
- Simpler implementation, slightly less accurate

**Recommendation:** Start with Option B for simplicity, then enhance to Option A if needed.

### Phase 3: Instruction Count Tracking

Update `Statistics` struct:

```go
type Statistics struct {
    // ... existing fields ...
    
    // EliminatedBranches is the count of unconditional branches that were
    // eliminated (not issued to execute stage).
    EliminatedBranches uint64
}
```

**CPI Calculation:** The eliminated branches should NOT count toward the instruction count for CPI calculation (they don't consume cycles), but should be tracked separately for validation.

### Implementation Details

#### 1. New Helper Functions

```go
// isEliminableBranch checks if an instruction word is an unconditional B (not BL).
// Returns true if the branch can be eliminated (doesn't write to a register).
func isEliminableBranch(word uint32) bool {
    // B instruction: bits [31:26] = 000101 (bit 31 = 0)
    // BL instruction: bits [31:26] = 100101 (bit 31 = 1)
    opcode := (word >> 26) & 0x3F
    if opcode == 0b000101 {  // Only pure B, not BL
        return true
    }
    return false
}
```

#### 2. Modify Fetch Stage

In `tickQuadIssue()` fetch logic:

```go
if ok && !fetchStall {
    // Check for eliminable unconditional branch
    if isEliminableBranch(word) {
        _, uncondTarget := isUnconditionalBranch(word, p.pc)
        p.pc = uncondTarget
        p.stats.EliminatedBranches++
        // Don't create IFID entry - branch is eliminated
        // Optionally: fetch from target in same cycle (aggressive)
    } else {
        // Normal fetch path...
        nextIFID = IFIDRegister{...}
    }
}
```

#### 3. Superscalar Considerations

When fetching multiple instructions:
- If slot N contains an eliminable branch, redirect PC but still fill later slots from target
- In quad-issue: if ifid1 is eliminated branch, ifid2/3/4 should come from target address

```
Example: 
  PC=0x100:  B 0x200       <- Eliminate, redirect to 0x200
  PC=0x104:  ADD X1, X1, 1 <- Skip (unreachable)
  
After elimination:
  PC=0x200:  SUB X2, X2, 1 <- Fetch this instead
```

### Testing Strategy

1. **Unit Tests:**
   - Test `isEliminableBranch()` function
   - Test that B instructions are not counted in instruction count
   - Test that PC correctly redirects

2. **Integration Tests:**
   - Create benchmark with many unconditional branches
   - Verify CPI improves vs baseline
   - Verify functional correctness (program produces correct results)

3. **Benchmark Validation:**
   - Re-run arithmetic_sequential (should be unaffected, no branches)
   - Re-run branch_taken (should improve - has branches)
   - Re-run dependency_chain (should improve if has unconditional branches)

### Expected Impact

| Benchmark | Current CPI | Expected After | Notes |
|-----------|-------------|----------------|-------|
| arithmetic_sequential | 0.450 | 0.450 | No branches |
| branch_taken | 0.900 | ~0.85-0.90 | Has some unconditional branches |
| dependency_chain | 1.200 | ~1.15-1.20 | Minor improvement |

The main improvement will be in code with unconditional branches (function calls, loops with B instead of B.cond).

### Risks and Mitigations

1. **Risk:** Breaking functional correctness
   - **Mitigation:** Extensive testing, compare results to emulator

2. **Risk:** Complexity in superscalar fetch logic
   - **Mitigation:** Implement single-issue first, then extend to superscalar

3. **Risk:** Statistics confusion (eliminated vs retired)
   - **Mitigation:** Track separately, document clearly

### Future Enhancements

1. **MOV elimination:** `mov xN, #0` and `mov xN, xM` could also be eliminated
2. **NOP elimination:** `nop` should never issue
3. **Instruction fusion:** Fuse compare+branch into single uop

## Implementation Plan

1. **PR #1: Basic B Elimination**
   - Add `isEliminableBranch()` helper
   - Modify `tickSingleIssue()` to eliminate B
   - Add `EliminatedBranches` stat
   - Unit tests

2. **PR #2: Superscalar B Elimination**
   - Extend to `tickSuperscalar()` and `tickQuadIssue()`
   - Handle multi-slot fetch with elimination
   - Integration tests

3. **PR #3 (Optional): BL Elimination**
   - Implement X30 write injection
   - More aggressive optimization

## References

1. [Dougall Johnson - Firestorm Overview](https://dougallj.github.io/applecpu/firestorm.html)
2. [Apple M1 Instruction Tables](https://dougallj.github.io/applecpu/firestorm-int.html)
3. M2Sim ARITHMETIC_THROUGHPUT_ANALYSIS.md
