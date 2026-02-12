# Timing Simulator Debugging: PR #233 Acceptance Test Failure

**Author:** Eric (Researcher)  
**Date:** 2026-02-05  
**Issue:** PR #233 acceptance tests timeout despite unit tests passing

## Executive Summary

PR #233 (hot branch benchmark) times out in CI acceptance tests but unit tests pass. The root cause is a **same-cycle flag forwarding gap** in the 8-wide superscalar execution path.

## Key Finding

**Unit tests pass because they use single-issue pipeline.  
Acceptance tests hang because they use 8-wide (octuple issue) pipeline.**

| Test Type | Pipeline Mode | PSTATE Forwarding | Result |
|-----------|---------------|-------------------|--------|
| Unit tests (TestCountdownLoop, TestBackwardBranch) | Single-issue | ✅ Works | ✅ PASS |
| Acceptance tests (timing harness with benchmarks) | **8-wide** | ❌ Missing same-cycle forwarding | ❌ HANG |

## Root Cause Analysis

### The Hot Branch Loop Structure

```go
// In branchHotLoop benchmark:
EncodeSUBImm(0, 0, 1, false), // SUB X0, X0, #1 (no flags)
EncodeCMPImm(0, 0),           // CMP X0, #0 (sets flags)
EncodeBCond(-8, 1),           // B.NE loop (reads flags)
```

In 8-wide decode, these 3 instructions are decoded into consecutive slots:
- **Slot 0:** SUB (does not set flags)
- **Slot 1:** CMP (sets flags → stores in `nextEXMEM2`)
- **Slot 2:** B.NE (needs flags)

### The Bug: Same-Cycle Flag Forwarding

In `tickOctupleIssue()`:

**Primary slot (slot 0)** uses `ExecuteWithFlags()`:
```go
execResult := p.executeStage.ExecuteWithFlags(&p.idex, rnValue, rmValue,
    forwardFlags, fwdN, fwdZ, fwdC, fwdV)
```
- Checks `p.exmem` through `p.exmem8` for forwarded flags
- These are **previous cycle's** EXMEM registers

**Secondary slots (slots 1-7)** use plain `Execute()`:
```go
execResult := p.executeStage.Execute(&idex2, rnValue, rmValue)
// Execute() always falls through to checkCondition() which reads stale PSTATE
```
- Does NOT receive forwarded flags
- B.cond in these slots reads stale PSTATE directly

**The Problem:**
When CMP is in slot 1 and B.NE is in slot 2, they execute in the **same cycle**:
1. CMP (slot 1) executes, computes flags, stores in `nextEXMEM2`
2. B.NE (slot 2) executes, but only checks `p.exmem*` (previous cycle), NOT `nextEXMEM2` (current cycle)
3. Result: B.NE reads stale PSTATE flags → condition wrong → infinite loop

### Why Unit Tests Pass

Unit tests use single-issue pipeline (`pipeline.NewPipeline(regFile, memory)` with no options):
- Only one instruction executes per cycle
- CMP executes in cycle N, flags written to PSTATE at cycle end
- B.NE executes in cycle N+1, checks `p.exmem.SetsFlags` for forwarding
- Forwarding from previous cycle works correctly

### Why Acceptance Tests Fail

Acceptance tests use 8-wide pipeline (`pipeline.WithOctupleIssue()`):
- Multiple instructions execute in same cycle
- CMP in slot 1 produces `nextEXMEM2.SetsFlags = true` in current cycle
- B.NE in slot 2 only checks `p.exmem*` (latched from previous cycle)
- No check for `nextEXMEM2` from current cycle → stale flags → hang

## Cathy's Previous Fix (9d7c2e6)

Cathy's fix added `SetsFlags`/`FlagN/Z/C/V` fields to all EXMEM registers (slots 2-8) and modified slots 2-8 to store computed flags. However, the fix:

✅ Stores flags in secondary EXMEM registers  
❌ Does NOT check same-cycle `nextEXMEM*` for B.cond in slots 2-8

The forwarding check in primary slot only looks at **latched** `p.exmem*` registers:
```go
if p.exmem2.Valid && p.exmem2.SetsFlags {
    forwardFlags = true
    ...
}
```

This works for **cross-cycle** forwarding but NOT **same-cycle** forwarding.

## Proposed Fix

Add same-cycle flag forwarding checks for secondary slots. When executing B.cond in slot 2+, check both:
1. Previous cycle's EXMEM registers (`p.exmem*`)
2. Current cycle's just-computed EXMEM registers (`nextEXMEM*`)

Example fix for slot 2 (tertiary):
```go
// Execute tertiary slot
if p.idex3.Valid && !memStall && !execStall {
    ...
    // Check for same-cycle flag forwarding from slot 1 or slot 0
    forwardFlags := false
    var fwdN, fwdZ, fwdC, fwdV bool
    if p.idex3.Inst != nil && p.idex3.Inst.Op == insts.OpBCond && !p.idex3.IsFused {
        // Check current cycle: nextEXMEM (slot 0), nextEXMEM2 (slot 1)
        if nextEXMEM2.Valid && nextEXMEM2.SetsFlags {
            forwardFlags = true
            fwdN, fwdZ, fwdC, fwdV = nextEXMEM2.FlagN, nextEXMEM2.FlagZ, 
                                     nextEXMEM2.FlagC, nextEXMEM2.FlagV
        } else if nextEXMEM.Valid && nextEXMEM.SetsFlags {
            forwardFlags = true
            fwdN, fwdZ, fwdC, fwdV = nextEXMEM.FlagN, nextEXMEM.FlagZ,
                                     nextEXMEM.FlagC, nextEXMEM.FlagV
        }
        // Also check previous cycle EXMEM registers...
    }
    
    idex3 := p.idex3.toIDEX()
    execResult := p.executeStage.ExecuteWithFlags(&idex3, rnValue, rmValue,
        forwardFlags, fwdN, fwdZ, fwdC, fwdV)  // Change Execute() to ExecuteWithFlags()
    ...
}
```

This pattern needs to be applied to slots 2-8 in `tickOctupleIssue()`.

## Verification

After fix:
1. `TestCountdownLoop` continues to pass (single-issue)
2. `TestBackwardBranch` continues to pass (single-issue)
3. Hot branch benchmark (8-wide) should pass
4. PR #233 CI acceptance tests should pass

## Files to Modify

- `timing/pipeline/pipeline.go` — `tickOctupleIssue()` slots 2-8 execute logic
- Change `Execute()` to `ExecuteWithFlags()` for slots 2-8
- Add same-cycle flag forwarding checks for B.cond in slots 2-8

## Summary

| Issue | Status |
|-------|--------|
| PSTATE fields in EXMEM 2-8 | ✅ Fixed by Cathy |
| Cross-cycle flag forwarding (previous → current) | ✅ Works for slot 0 |
| Same-cycle flag forwarding (slot N → slot N+1) | ❌ **Missing for slots 2-8** |

The fix is to add same-cycle flag forwarding to secondary slots in `tickOctupleIssue()`.
