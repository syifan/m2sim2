# Zero-Cycle Branch Research: Implementation Analysis

**Author:** Bob (Coder)
**Date:** 2026-02-05 (Cycle 243)
**Task:** Research zero-cycle predicted-taken branches

## Executive Summary

After analyzing the codebase and Eric's implementation guide, I've identified the key gaps between our current implementation and what M2 achieves. The simulator **already does speculative fetch redirection** based on branch prediction, but branches still consume pipeline slots. True zero-cycle branches require "folding" — allowing predicted-taken branches to skip the execute stage entirely.

## Current Implementation Analysis

### What We Already Have ✅

1. **Tournament Predictor** (bimodal + gshare + choice) — high accuracy
2. **BTB with 2048 entries** (PR #227 merged) — good capacity
3. **Unconditional B elimination** — B (not BL) never enters pipeline
4. **Early resolution for B/BL** — target computed at fetch, not execute
5. **Speculative fetch** — PC redirected based on prediction

### Current Flow for Conditional Branches

```
Cycle N:   Fetch branch @ PC, predict taken, redirect fetch to target
Cycle N+1: Decode branch (still in pipeline)
Cycle N+2: Execute branch, verify prediction
           - If correct: continue (branch consumed 3 slots)
           - If wrong: flush, 5-8 cycle penalty
```

Even with correct prediction, the branch **consumes IF/ID/EX slots**.

### M2's Likely Zero-Cycle Flow

```
Cycle N:   Fetch branch @ PC, BTB hit + predict taken
           → Mark as "folded", redirect fetch to target immediately
Cycle N+1: Fetch from target (branch verification runs in background)
           - If verify correct: no action needed
           - If verify wrong: flush + penalty
```

The key difference: **folded branches don't block the pipeline**. They're verified in parallel.

## Key Code Locations

| File | Function | Purpose |
|------|----------|---------|
| `pipeline.go` | `tickSingleIssue()` | Single-issue main loop |
| `pipeline.go` | `tickOctupleIssue()` | 8-wide main loop (primary target) |
| `branch_predictor.go` | `Predict()` | Returns prediction + BTB target |
| `stages.go` | Execute stage | Branch verification |

## Implementation Options

### Option A: Folded Branch Tracking (Recommended)

Add a `foldedBranches` map to Pipeline:

```go
type Pipeline struct {
    // ... existing fields ...
    
    // Zero-cycle branch tracking
    // Maps branch PC → (predicted target, prediction confidence)
    foldedBranches map[uint64]foldedBranchInfo
}

type foldedBranchInfo struct {
    target     uint64
    predTaken  bool
    confidence uint8  // From predictor state (0-3)
}
```

**In Fetch stage:**
```go
pred := p.branchPredictor.Predict(pc)
if pred.TargetKnown && pred.Taken && isHighConfidence(pred) {
    // Mark branch as folded — it will be verified later but doesn't 
    // block the pipeline
    p.foldedBranches[pc] = foldedBranchInfo{
        target:    pred.Target,
        predTaken: true,
    }
    // Redirect fetch immediately
    p.pc = pred.Target
    // Don't create IFID entry OR create a "folded" entry that skips decode/execute
    p.stats.FoldedBranches++
    continue // Fetch next instruction from target
}
```

**In Execute stage (verification):**
```go
if branchInfo, ok := p.foldedBranches[idex.PC]; ok {
    // This was a folded branch — verify prediction
    if actualTaken == branchInfo.predTaken && 
       actualTarget == branchInfo.target {
        // Correct — nothing to do, already fetching from target
        delete(p.foldedBranches, idex.PC)
        p.stats.BranchCorrect++
    } else {
        // MISPREDICT — flush everything after this branch
        delete(p.foldedBranches, idex.PC)
        p.flushPipeline()
        p.pc = actualTarget
        p.stats.BranchMispredictions++
    }
}
```

### Option B: Execute Stage Bypass (Simpler)

Don't track folded branches separately. Instead, when execute stage sees a predicted-taken branch with BTB hit:

```go
if idex.IsBranch && idex.PredictedTaken && idex.EarlyResolved {
    // For high-confidence predictions, skip full branch evaluation
    // Trust the prediction, count as 0-cycle
    // Still verify in background but don't block
}
```

This is simpler but less accurate to M2's actual behavior.

## Recommended Implementation Order

1. **Add statistics tracking** for folded vs non-folded branches
2. **Implement folded branch map** in Pipeline struct
3. **Modify fetch** to mark high-confidence predicted-taken as folded
4. **Modify execute** to verify folded branches without blocking
5. **Handle misprediction recovery** (flush + correct PC)
6. **Test extensively** with branch benchmarks

## Estimated Impact

| Metric | Current | After Zero-Cycle | Delta |
|--------|---------|------------------|-------|
| Branch CPI (tight loop) | 1.600 | ~1.2-1.3 | -25% |
| Average error | 34.2% | ~20-25% | -10pp |
| BTB hit branches | 1+ cycle | 0 cycles | -1 cycle |

## Edge Cases to Handle

1. **Back-to-back branches**: If instruction N and N+1 are both branches, folding both requires careful tracking
2. **BTB miss**: First iteration can't be folded (no target known)
3. **Indirect branches (BR, BLR)**: May require separate BTB for indirect targets
4. **Return stack**: RET instructions might benefit from return stack predictor (future work)

## Files to Modify

| File | Changes |
|------|---------|
| `timing/pipeline/pipeline.go` | Add foldedBranches field, modify fetch/execute |
| `timing/pipeline/superscalar.go` | Update 8-wide logic |
| `timing/pipeline/branch_predictor.go` | Add confidence info to Prediction |

## Testing Strategy

1. **Unit tests**: Verify folded branch map operations
2. **Branch benchmarks**: Run branchTakenConditional, compare CPI
3. **Correctness**: Run all benchmarks, verify same instruction count
4. **Edge cases**: Test back-to-back branches, BTB miss scenarios

## Next Steps for Implementation

1. Create `bob/zero-cycle-branches` branch
2. Add `foldedBranches` map and statistics
3. Implement fetch-stage folding logic
4. Implement execute-stage verification
5. Run tests, measure improvement
6. Open PR for review

## References

- Eric's implementation guide: `docs/zero-cycle-branch-implementation.md`
- Branch predictor research: `docs/branch-predictor-research.md`
- Dougall Johnson's Firestorm documentation (external)
