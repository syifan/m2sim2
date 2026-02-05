# M2Sim Progress Report

**Last updated:** 2026-02-05 04:20 EST (Cycle 220)

## Current Status

| Metric | Value |
|--------|-------|
| Total PRs Merged | 54 |
| Open PRs | 0 |
| Open Issues | 12 |
| Pipeline Coverage | 76.2% |

## Cycle 220 Updates

- **Alice:** Updated task board, action count → 220
- **Eric:** Evaluated status, noted benchmark methodology issue
- **Bob:** Ran accuracy validation — found instruction count mismatch
- **Cathy:** Coverage analysis — timing/core at 60% (lowest)
- **Dana:** Updated PROGRESS.md

## Key Finding This Cycle

**Benchmark Instruction Count Mismatch Discovered!**

Bob ran accuracy validation with the new `branchTakenConditional()` benchmark and found the error INCREASED (51.3% → 62.5%). Investigation revealed:

| Benchmark | Instruction Count | CPI |
|-----------|------------------|-----|
| Native baseline | 5 (branch only) | 1.190 |
| branchTaken() (old) | 10 (5 B + 5 ADD) | 1.800 |
| branchTakenConditional() | 15 (5×(CMP+B.GE+ADD)) | 1.933 |

The CPI comparison is invalid because:
- Native measures CPI per *branch instruction*
- Simulator measures CPI for *entire program*

**Next step:** Redesign benchmark methodology to count same instructions.

## Accuracy Status (Microbenchmarks)

| Benchmark | Simulator CPI | M2 Real CPI | Error | Notes |
|-----------|---------------|-------------|-------|-------|
| arithmetic | 0.400 | 0.268 | 49.3% | 4-wide vs 6-wide issue |
| dependency | 1.200 | 1.009 | 18.9% | Closest to target |
| branch_taken | 1.800 | 1.190 | 51.3% | Instruction count mismatch |
| **Average** | — | — | 39.8% | |

**Target:** <20% average error (#141)

## Coverage Analysis

| Package | Coverage |
|---------|----------|
| timing/cache | 89.1% ✅ |
| timing/pipeline | 76.2% ✅ |
| timing/latency | 73.3% ✅ |
| timing/core | 60.0% ⚠️ |

Weak spots: `ExitCode()`, `Run()`, `RunCycles()`, `Reset()` at 0%.

## Active PRs

None — all merged!

## Active Investigations

- **#197** — Embench timing run request (waiting on human)
- **#132** — Research intermediate benchmarks (PolyBench, Embench)
- **Benchmark methodology** — Need to align instruction counting

## Calibration Milestones

| Milestone | Status | Description |
|-----------|--------|-------------|
| C1 | ✅ Complete | Benchmarks execute to completion |
| C2 | 🚧 Blocked | Accuracy calibration — methodology issue discovered |
| C3 | Pending | Intermediate benchmark timing |

## Stats

- 54 PRs merged total
- 205+ tests passing
- Zero-cycle branch elimination: working ✓
- Branch predictor: working ✓
- PSTATE flag updates: working ✓
- Conditional branch benchmark: added ✓
- Coverage: 76.2% (target: 70% ✓)
