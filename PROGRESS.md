# M2Sim Progress Report

**Last updated:** 2026-02-04 19:55 EST (Cycle 191)

## Current Status

| Metric | Value |
|--------|-------|
| Total PRs Merged | 40 |
| Open PRs | 2 |
| Open Issues | ~15 |
| Pipeline Coverage | 70.1% |

## Active Work

### PR #175 — ADD/SUB SP Handling Fix (Bob)
- **Status:** cathy-approved ✅, CI in progress
- **Impact:** CoreMark jumps from 2406 → 10M+ instructions
- **Root cause:** Register 31 was always treated as XZR, but ARM64 uses SP for ADD/SUB immediate
- Awaiting Unit Tests to complete

### PR #178 — Pipeline Stats Tests (Cathy)
- **Status:** Ready for review
- **Impact:** Coverage 69.7% → 70.1%
- Tests for CPI, ExitCode, BranchPredictorStats methods

## Recent Progress

### This Cycle (191)
- Grace updated guidance per Human #176: Cathy/Eric/Dana should produce code, not wait for Bob
- Eric built aha-mont64 Embench benchmark (committed to main)
- Cathy created PR #178 with pipeline coverage tests
- Bob's PR #175 CI progressing

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

## Blockers

- **PR #175 CI** — Unit Tests running, merge blocked until complete

## Next Steps

1. Merge PR #175 once CI passes
2. Verify CoreMark completes successfully (expected 10M+ instructions)
3. Review and merge PR #178 (Cathy's coverage tests)
4. Continue Embench benchmark integration (#163-165)
