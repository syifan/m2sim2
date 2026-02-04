# Pipeline.go Refactoring Plan

**Issue:** #122 - Refactor pipeline.go to reduce code duplication

## Current State

| Function | Lines | Description |
|----------|-------|-------------|
| tickSingleIssue | ~340 | 1-wide pipeline tick |
| tickSuperscalar | ~613 | 2-wide pipeline tick |
| tickQuadIssue | ~773 | 4-wide pipeline tick |
| tickSextupleIssue | ~992 | 6-wide pipeline tick |
| **Total** | **~2718** | Nearly 82% of pipeline.go |

## Identified Duplication

### 1. Pipeline Register Types (superscalar.go)

Each width requires its own register types:
- `IFIDRegister`, `SecondaryIFIDRegister`, `TertiaryIFIDRegister`, etc.
- All have identical fields, just different names
- Each has its own `Clear()`, `toIDEX()`, `fromIDEX()` methods

**Refactor:** Use slice/array of generic pipeline registers instead of named types.

### 2. Writeback Stage Logic

Each tick function has nearly identical writeback code:
```go
// For each slot:
if memwb.Valid && memwb.RegWrite && memwb.Rd != 31 {
    var value uint64
    if memwb.MemToReg {
        value = memwb.MemData
    } else {
        value = memwb.ALUResult
    }
    regFile.WriteReg(memwb.Rd, value)
    stats.Instructions++
}
```

**Refactor:** Create `writebackSlot(memwb *MEMWBRegister)` helper.

### 3. Memory Stage Logic

Duplicated syscall handling and cache access:
```go
if exmem.Inst != nil && exmem.Inst.Op == insts.OpSVC {
    if syscallHandler != nil {
        result := syscallHandler.Handle()
        if result.Exited { halted = true; exitCode = result.ExitCode }
    }
}
// ... cache access logic ...
```

**Refactor:** Create `memoryAccessSlot(exmem *EXMEMRegister)` helper.

### 4. Execute Stage Logic

Forwarding and execution duplicated for each slot:
```go
rnValue := forwardFromAllSlots(idex.Rn, idex.RnValue)
rmValue := forwardFromAllSlots(idex.Rm, idex.RmValue)
// Forward from earlier slots in same cycle
if nextEXMEM.Valid && nextEXMEM.RegWrite && nextEXMEM.Rd != 31 {
    if idex.Rn == nextEXMEM.Rd { rnValue = nextEXMEM.ALUResult }
    if idex.Rm == nextEXMEM.Rd { rmValue = nextEXMEM.ALUResult }
}
execResult := executeStage.Execute(&idex, rnValue, rmValue)
```

**Refactor:** Create `executeSlot(idex *IDEXRegister, earlierResults []*EXMEMRegister)` helper.

### 5. Fetch Stage Logic

Branch elimination and prediction duplicated for each fetch slot:
```go
if isEliminableBranch(word) {
    _, uncondTarget := isUnconditionalBranch(word, fetchPC)
    fetchPC = uncondTarget
    stats.EliminatedBranches++
    continue
}
pred := branchPredictor.Predict(fetchPC)
if isUncondBranch {
    pred.Taken = true
    pred.Target = uncondTarget
    pred.TargetKnown = true
    earlyResolved = true
}
```

**Refactor:** Create `fetchSlot(pc uint64) (IFIDRegister, uint64, bool)` helper.

### 6. Decode Stage Logic

Same pattern - decode each slot and check canIssueWith:
```go
decResult := decodeStage.Decode(ifid.InstructionWord, ifid.PC)
nextIDEX := IDEXRegister{...decResult fields...}
if canIssueWith(&tempIDEX, issuedInsts) {
    nextIDEX2.fromIDEX(&tempIDEX)
    issuedInsts = append(issuedInsts, &tempIDEX)
}
```

**Refactor:** Create `decodeSlots(ifids []IFIDRegister) []IDEXRegister` helper.

## Proposed Architecture

### Option A: Slice-Based Registers

Replace named types with slices:
```go
type Pipeline struct {
    ifid  []IFIDRegister  // Length = issueWidth
    idex  []IDEXRegister
    exmem []EXMEMRegister
    memwb []MEMWBRegister
    // ...
}
```

**Pros:** Eliminates all register type duplication, single tick function
**Cons:** More complex indexing, potential performance impact

### Option B: Generic Tick Function

Create parameterized tick:
```go
func (p *Pipeline) tickN(width int) {
    // Single implementation that handles any width
}
```

**Pros:** Reduces duplication to near-zero
**Cons:** Still need named register types for type safety

### Option C: Stage Helpers (Recommended)

Keep existing structure but extract common logic:
```go
func (p *Pipeline) writebackAll()
func (p *Pipeline) memoryAll() bool  // returns memStall
func (p *Pipeline) executeAll() bool // returns execStall
func (p *Pipeline) decodeAll()
func (p *Pipeline) fetchAll()
```

**Pros:** Incremental change, maintains readability
**Cons:** Still has some duplication in orchestration

## Implementation Plan

### Phase 1: Extract Helpers (Low Risk)
1. Create helper methods for each stage
2. Use in tickSextupleIssue first (newest, most complex)
3. Verify tests pass
4. Backport to other tick functions

### Phase 2: Unify Register Types (Medium Risk)
1. Create generic `PipelineRegister` interface
2. Replace named types with interface implementations
3. Verify performance impact

### Phase 3: Single Tick Function (Higher Risk)
1. Consolidate all tick functions into one parameterized implementation
2. Remove width-specific tick functions
3. Extensive testing

## Estimated Savings

| Phase | Lines Removed | Risk |
|-------|---------------|------|
| Phase 1 | ~500 | Low |
| Phase 2 | ~600 | Medium |
| Phase 3 | ~800 | High |
| **Total** | **~1900** | - |

Final pipeline.go would be ~1400 lines (down from 3320).

## Next Steps

1. Create issue for each phase
2. Start with Phase 1 helpers
3. Review impact on test coverage
4. Consider adding benchmarks to detect performance regression

---
*Refactoring analysis by Cathy during M2Sim cycle #127*
