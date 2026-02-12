# PSTATE Flag Forwarding Research

**Author:** Eric (Researcher)
**Date:** 2026-02-05
**Related:** PR #233 timeout, Issue #232, docs/timing-sim-backward-branch-debugging.md

## Executive Summary

This document researches how other cycle-level simulators handle condition flag forwarding for pipeline hazard avoidance. The goal is to support Bob in implementing a proper PSTATE forwarding fix for M2Sim.

## The Problem in M2Sim

Our hot branch benchmark (PR #233) exposes a flag dependency hazard:
- CMP sets PSTATE at cycle END
- B.NE reads PSTATE at cycle START (same cycle)
- Result: B.NE sees stale flags → infinite loop

## How Other Simulators Handle Flag Forwarding

### 1. gem5 (Industry Standard)

gem5's O3 CPU model handles flag dependencies through:

**Rename & Register File Approach:**
- Treats condition flags (NZCV) as part of the architectural register file
- Uses a unified physical register file that includes flag registers
- Flag-producing instructions write to physical flag registers
- Flag-consuming instructions have dependencies tracked via scoreboard

**Key Implementation:**
```cpp
// In src/cpu/o3/regfile.cc
class PhysRegFile {
    // Integer, FP, Vector, and CC (Condition Code) registers
    RegClassInfo ccRegClass;  // Separate class for condition codes
};
```

**Forwarding Network:**
- gem5 forwards CC values through the same bypass network as data registers
- Flag producers write to bypass registers
- Flag consumers check bypass registers before reading from committed state

### 2. MARSS (Microarchitectural and System Simulator)

MARSS uses a simpler approach:

**Scoreboard-Based Hazard Detection:**
- Tracks which instructions set flags
- Stalls flag-consuming instructions until producer commits
- No explicit forwarding — relies on stalling

**Simpler but Higher Latency:**
- Every non-fused conditional branch waits for flag producer to commit
- Works correctly but adds 2-3 cycles per dependent branch

### 3. SimpleScalar

SimpleScalar handles flags as implicit dependencies:

**Implicit Operand Tracking:**
- Flag-setting instructions are tagged with "sets CC"
- Flag-consuming instructions (branches) wait for CC-setters
- Forwarding happens via the standard operand forwarding network

**Key Insight:** SimpleScalar treats CC as "operand 3" for branches, enabling reuse of existing forwarding infrastructure.

### 4. Sniper (Multi-core Simulator)

Sniper uses a hybrid approach:

**Branch Prediction + Resolution:**
- Predicts branch outcome speculatively
- If flag producer is still in flight, uses prediction
- Validates prediction when flags are available
- Flushes on misprediction

**Advantage:** No stalls for correctly predicted branches
**Disadvantage:** Misprediction penalty on wrong guesses

## Recommended Approach for M2Sim

Based on the research, I recommend **Option A: Explicit Flag Forwarding** with elements from gem5 and SimpleScalar.

### Implementation Plan

#### Step 1: Add Flag Fields to Pipeline Registers

```go
// timing/pipeline/registers.go
type EXMEMRegister struct {
    // ... existing fields ...
    
    // PSTATE flag forwarding
    SetsFlags bool    // True if this instruction sets PSTATE
    N, Z, C, V bool   // Flag values computed in execute stage
}
```

#### Step 2: Modify Execute Stage to Set Forward Flags

```go
// In execute stage for CMP, SUBS, ADDS, etc.
func (s *Stages) executeCMP(inst *insts.Instruction, idex *IDEXRegister) *EXMEMRegister {
    n, z, c, v := ComputeSubFlags(rn, op2, is64)
    
    exmem := &EXMEMRegister{
        Valid:     true,
        SetsFlags: true,
        N: n, Z: z, C: c, V: v,
        // ... other fields ...
    }
    return exmem
}
```

#### Step 3: Forward Flags to Conditional Branch Execute

```go
// In execute stage for B.cond, CBZ, CBNZ, TBZ, TBNZ
func (s *Stages) executeBCond(inst *insts.Instruction, idex *IDEXRegister) *EXMEMRegister {
    var n, z, c, v bool
    
    if idex.IsFused {
        // Fused: use embedded CMP operands
        n, z, c, v = ComputeSubFlags(idex.FusedRnVal, op2, idex.FusedIs64)
    } else if s.shouldForwardFlags() {
        // Forward: use flags from previous EXMEM
        n, z, c, v = s.getForwardedFlags()
    } else {
        // Committed: read from PSTATE register file
        n, z, c, v = s.regFile.PSTATE.N, s.regFile.PSTATE.Z, 
                     s.regFile.PSTATE.C, s.regFile.PSTATE.V
    }
    
    conditionMet := EvaluateConditionWithFlags(inst.Cond, n, z, c, v)
    // ...
}
```

#### Step 4: Add Forwarding Detection

```go
// Check if flags should be forwarded from EXMEM stage
func (s *Stages) shouldForwardFlags() bool {
    // Check all EXMEM registers (8-wide pipeline)
    for _, exmem := range s.exmemRegisters {
        if exmem.Valid && exmem.SetsFlags {
            return true
        }
    }
    return false
}

func (s *Stages) getForwardedFlags() (n, z, c, v bool) {
    // Find most recent flag producer (latest slot with SetsFlags)
    for i := len(s.exmemRegisters) - 1; i >= 0; i-- {
        exmem := s.exmemRegisters[i]
        if exmem.Valid && exmem.SetsFlags {
            return exmem.N, exmem.Z, exmem.C, exmem.V
        }
    }
    // Fallback to committed PSTATE
    return s.regFile.PSTATE.N, s.regFile.PSTATE.Z, 
           s.regFile.PSTATE.C, s.regFile.PSTATE.V
}
```

#### Step 5: Commit Flags in Writeback Stage

```go
// In writeback stage
func (s *Stages) writeback(memwb *MEMWBRegister) {
    // ... existing writeback logic ...
    
    // Commit flags to PSTATE
    if memwb.SetsFlags {
        s.regFile.PSTATE.N = memwb.N
        s.regFile.PSTATE.Z = memwb.Z
        s.regFile.PSTATE.C = memwb.C
        s.regFile.PSTATE.V = memwb.V
    }
}
```

## Testing Strategy

1. **Unit Tests:**
   - Test flag forwarding for CMP → B.NE (1-cycle dependency)
   - Test flag forwarding across multiple instructions
   - Test that non-flag-setting instructions don't forward

2. **Integration Tests:**
   - Hot branch benchmark (PR #233) should pass
   - Enable skipped tests: TestCountdownLoop, backward branch tests

3. **Accuracy Validation:**
   - Verify CPI doesn't regress for existing benchmarks
   - Measure hot branch CPI improvement

## Files to Modify

| File | Changes |
|------|---------|
| `timing/pipeline/registers.go` | Add SetsFlags, N, Z, C, V to EXMEMRegister |
| `timing/pipeline/stages.go` | Modify execute stage to set forward flags |
| `timing/pipeline/pipeline.go` | Add forwarding logic to tick functions |
| `timing/pipeline/superscalar.go` | Handle forwarding in 8-wide execution |

## Expected Impact

- **Hot branch benchmark:** Should pass CI (no timeout)
- **Branch accuracy:** May improve slightly due to correct flag handling
- **Skipped tests:** Can be re-enabled
- **Zero-cycle folding:** Can be properly validated

## References

- [gem5 O3 CPU Model](https://www.gem5.org/documentation/general_docs/cpu_models/O3CPU)
- [MARSS Simulator](http://marss86.org/)
- [SimpleScalar Documentation](http://www.simplescalar.com/)
- ARM Architecture Reference Manual - Condition Flags
