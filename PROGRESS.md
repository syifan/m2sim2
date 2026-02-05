# M2Sim Progress Report

**Last updated:** 2026-02-04 20:16 EST (Cycle 192)

## Current Status

| Metric | Value |
|--------|-------|
| Total PRs Merged | 41 |
| Open PRs | 1 |
| Open Issues | ~15 |
| Pipeline Coverage | 70.1% |

## Active Work

### PR #175 — ADD/SUB SP Handling Fix (Bob)
- **Status:** cathy-approved ✅, CI running
- **Impact:** CoreMark jumps from 2406 → 10M+ instructions
- **Fix this cycle:** Bob fixed test hang (Ethan validation tests expected SP=0)
- Awaiting Lint + Unit Tests to complete

## Recently Merged

### PR #178 — Pipeline Stats Tests (Cathy)
- **Merged this cycle!** ✅
- Coverage for CPI, ExitCode, BranchPredictorStats methods

## Recent Progress

### This Cycle (192)
- **Issue #177 (unit test hang) FIXED** — Bob found root cause: Ethan tests used SP=0x7FFF0000 but expected ADD X8,SP,#93 to return 93. Fixed by setting SP=0.
- **PR #178 merged** — Cathy's pipeline stats coverage tests
- Eric built crc32 + matmult-int Embench benchmarks (20 new files)
- All 3 Embench benchmarks now have build infrastructure

### Previous Cycles
- PR #174 merged — BRK instruction support
- PR #173 merged — Shift regs, bitfield, CCMP instructions
- PR #171 merged — Logical immediate instructions
- CoreMark execution: 2127 → 2406 instructions before SP fix

## Calibration Milestones

| Milestone | Status | Description |
|-----------|--------|-------------|
| C1 | 🚧 Active | Execution Completeness — full CoreMark execution |
| C2 | Pending | Microbenchmark Accuracy — <20% avg error |
| C3 | Pending | Intermediate Benchmark Accuracy |
| C4 | Pending | SPEC Benchmark Accuracy |

## Embench Benchmark Status

| Benchmark | Build Status | Test Status |
|-----------|--------------|-------------|
| aha-mont64 | ✅ Built (68KB) | Pending |
| crc32 | ✅ Built (69KB) | Pending |
| matmult-int | ✅ Built (71KB) | Pending |

## Blockers

- **PR #175 CI** — Lint + Unit Tests pending, merge blocked until complete

## Next Steps

1. Merge PR #175 once CI passes
2. Verify CoreMark completes successfully (expected 10M+ instructions)
3. Test Embench benchmarks in M2Sim execution
4. Continue CoreMark debugging (#172)
