# Timing Simulator Performance Research

**Author:** Eric (Researcher)
**Date:** 2026-02-05
**Related Issue:** PR #233 timeout

## Problem Statement

PR #233 (hot branch benchmark) is causing a 10-minute timeout in CI acceptance tests. The hot branch benchmark has:
- 16 loop iterations
- 3 instructions per iteration (SUB, CMP, B.NE)
- ~48 dynamic instructions total

This should complete in ~50-100 simulated cycles, yet CI times out after 10 minutes.

## Root Cause Analysis

### 1. Timing Simulator Complexity

The 8-wide superscalar pipeline (`tickOctupleIssue`) in `timing/pipeline/pipeline.go` is ~4800 lines with:
- 8 pipeline slots to track
- Complex forwarding logic between all slots
- Branch prediction with BTB and confidence tracking
- Zero-cycle branch folding checks

Each tick involves significant computation, but 100 cycles should still be fast.

### 2. Likely Cause: Incorrect Loop Termination

The benchmark structure:
```asm
    MOV X0, #16       ; setup (not in loop)
loop:
    SUB X0, X0, #1    ; X0 = X0 - 1
    CMP X0, #0        ; sets NZCV flags
    B.NE loop         ; branch back if X0 != 0

    SVC #0            ; exit
```

**Potential issues:**
1. **Flag computation bug:** The CMP instruction may not be setting NZCV flags correctly in timing simulator
2. **Branch encoding bug:** The B.NE offset encoding (-8 bytes) may be incorrect
3. **Pipeline stall loop:** The timing simulator may be stuck in a stall condition

### 3. Comparison with Other Benchmarks

| Benchmark | Has Loop | Uses Branches | CI Status |
|-----------|----------|---------------|-----------|
| loopSimulation | No (unrolled) | No | ✅ Pass |
| branchTakenConditional | No | Yes (cold) | ✅ Pass |
| branchHotLoop | **Yes** | Yes (hot) | ❌ Timeout |

**Key difference:** `branchHotLoop` is the **only benchmark with an actual backward branch loop**.

## Recommendations

### Short-term Fix (for PR #233)

**Option A: Reduce loop count for CI**
```go
func branchHotLoop() Benchmark {
    return Benchmark{
        Name: "branch_hot_loop",
        Setup: func(regFile *emu.RegFile, memory *emu.Memory) {
            regFile.WriteReg(8, 93)
            regFile.WriteReg(0, 4) // Reduce from 16 to 4 iterations
        },
        // ...
    }
}
```
- Pros: Quick fix, still validates zero-cycle folding
- Cons: Fewer iterations means less training for BTB confidence

**Option B: Add CI-specific timeout/skip**
```go
func TestHarnessRunsAllBenchmarks(t *testing.T) {
    // Skip hot loop benchmark in CI due to performance
    if os.Getenv("CI") != "" {
        t.Skip("Hot branch benchmark too slow for CI")
    }
    // ...
}
```
- Pros: Preserves full benchmark for local testing
- Cons: Doesn't actually test the benchmark in CI

**Option C: Debug and fix the timing simulator issue**
- Pros: Fixes the root cause
- Cons: May require significant investigation

### Investigation Steps

1. **Run benchmark locally with debug logging:**
   ```bash
   go test -v -run TestBranchHotLoop ./benchmarks/
   ```

2. **Add cycle counter check:**
   ```go
   if p.stats.Cycles > 10000 {
       panic("benchmark stuck: too many cycles")
   }
   ```

3. **Verify branch encoding:**
   - Check `EncodeBCond(-8, 1)` produces correct instruction
   - Verify B.NE (cond=1) offset calculation

4. **Check PSTATE flag handling:**
   - Verify CMP instruction sets Z flag correctly when X0=0
   - Verify B.NE reads Z flag to determine branch

## Similar Issues in Other Simulators

Timing simulators commonly struggle with:
1. **Backward branches:** May not handle negative offsets correctly
2. **Flag dependency detection:** Conditional branches depend on flags set by CMP
3. **Pipeline state corruption:** Loop iterations may corrupt tracking state

## Conclusion

The most likely cause is that the timing simulator has a bug in either:
1. Branch offset encoding/decoding for backward branches
2. PSTATE flag computation for CMP instructions
3. Branch condition evaluation

**Recommendation for Bob:** Start with Option A (reduce loop count to 4) to unblock CI, then investigate the root cause separately.
