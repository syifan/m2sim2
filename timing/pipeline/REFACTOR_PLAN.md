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

### Phase 1: Extract Stage Helpers (In Progress)
Extract common logic for each pipeline stage:
- [x] Plan documented
- [x] `WritebackSlot` interface defined
- [x] Interface implemented for all 6 MEMWB register types
- [x] `WritebackStage.WritebackSlot()` helper created
- [ ] Replace inline writeback code with helper calls
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
