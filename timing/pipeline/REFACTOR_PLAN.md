# Pipeline Refactor Plan

**Issue:** #122  
**Author:** [Cathy]  
**Started:** 2026-02-04

## Goal

Reduce code duplication in `pipeline.go` (3320 lines → ~1500 lines).

## Current State

Four near-identical tick functions:
- `tickSingleIssue()` (~340 lines)
- `tickSuperscalar()` (~620 lines)
- `tickQuadIssue()` (~880 lines)
- `tickSextupleIssue()` (~1100 lines)

Each repeats the same 5-stage logic (WB, MEM, EX, ID, IF) with minor variations for slot count.

## Approach: Phased Refactor

### Phase 1: Extract Stage Helpers (Complete)
Extract common logic for each pipeline stage:
- [x] Plan documented
- [x] `WritebackSlot` interface defined
- [x] Interface implemented for all 6 MEMWB register types
- [x] `WritebackStage.WritebackSlot()` helper created
- [x] Replace inline writeback code with helper calls (all slots now use WritebackSlot)
- [ ] `memorySlot()` - process single EXMEM slot  
- [ ] `executeSlot()` - process single IDEX slot
- [ ] `decodeSlot()` - process single IFID slot
- [ ] `fetchSlots()` - fetch N instructions

### Phase 2: Convert to Slices
Convert individual slot fields to slices:
```go
// Before
memwb, memwb2, memwb3, memwb4, memwb5, memwb6 MEMWBRegister

// After
memwbSlots [6]MEMWBRegister
```

### Phase 3: Unified Tick
Single `tick()` function parameterized by issue width.

## Testing Strategy

Each phase must maintain 75%+ test coverage. Run tests after each change:
```bash
go test ./timing/pipeline/... -cover
```

## Progress Log

**2026-02-04:** Plan created. Starting Phase 1.

**2026-02-04 21:50:** Added WritebackSlot interface and implementations.
  - Interface in stages.go
  - MEMWBRegister implementation in registers.go
  - Secondary/Tertiary/Quaternary/Quinary/Senary implementations in superscalar.go
  - Coverage: 75.9% (maintained)

**2026-02-04 23:45:** Phase 2 analysis (Cathy)
  - Identified 14 inline writeback patterns across tick functions:
    - tickSuperscalar: memwb2 (lines 721-730)
    - tickQuadIssue: memwb2, memwb3, memwb4 (lines 1340-1376)
    - tickSextupleIssue: memwb2-6 (lines 2221-2280)
  - **Finding:** Secondary slots only handle ALU ops (memory goes through primary slot only)
  - **Issue found:** Secondary slots don't count XZR-writes as retired, but primary slot does
    - Primary: `if p.memwb.Valid { p.stats.Instructions++ }`
    - Secondary: `if p.memwb2.Valid && p.memwb2.RegWrite && p.memwb2.Rd != 31 { ... }`
  - WritebackSlot returns true for all valid instructions (including rd=31)
  - **Next step:** Carefully replace inline code with WritebackSlot, verify instruction counts match

## Replacement Pattern

Current inline code:
```go
// Writeback secondary slot
if p.memwb2.Valid && p.memwb2.RegWrite && p.memwb2.Rd != 31 {
    var value uint64
    if p.memwb2.MemToReg {
        value = p.memwb2.MemData
    } else {
        value = p.memwb2.ALUResult
    }
    p.regFile.WriteReg(p.memwb2.Rd, value)
    p.stats.Instructions++
}
```

Target replacement:
```go
// Writeback secondary slot
if p.writebackStage.WritebackSlot(&p.memwb2) {
    p.stats.Instructions++
}
```

**Note:** This also fixes the XZR counting bug — instructions writing to XZR will now be counted.

**2026-02-05 00:17:** Phase 3 primary slot refactor (Cathy)
  - Replaced primary slot writeback with WritebackSlot helper (4 locations)
  - All tick functions now use WritebackSlot for both primary and secondary slots
  - Unified writeback pattern across all issue widths
  - Coverage: 77.3% → 77.6%
