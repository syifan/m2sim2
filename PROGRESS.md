# M2Sim Progress Report

**Last updated:** 2026-02-05 03:15 EST (Cycle 216)

## Current Status

| Metric | Value |
|--------|-------|
| Total PRs Merged | 51 |
| Open PRs | 1 (PR #202 awaiting review) |
| Open Issues | 13 |
| Pipeline Coverage | 76.5% |

## Cycle 216 Updates

- **Alice:** Updated task board, assigned zero-cycle branch investigation
- **Eric:** Created issue #201 (zero-cycle branches), updated docs
- **Bob:** Investigated branch handling — found benchmark mismatch!
- **Cathy:** Created PR #202 (dead code removal)
- **Dana:** Cleanup, updated PROGRESS.md

## Key Finding This Cycle

**Zero-cycle branch elimination IS working correctly!**

Bob's investigation revealed:
- Unconditional `B` instructions ARE being eliminated at fetch time
- `TestBranchTaken`: 9 cycles, 5 instructions (branches not counted)
- The CPI of 1.8 is CORRECT for data-dependent ADD chain in 5-stage pipeline

**The 51.3% "error" is due to BENCHMARK MISMATCH:**
- Native baseline (m2_baseline.json): uses `b.ge` (conditional branches)
- Simulator microbenchmark (branchTaken): uses `B` (unconditional branches)

We are comparing different instruction types!

## Accuracy Status (Microbenchmarks)

| Benchmark | Simulator CPI | M2 Real CPI | Error | Notes |
|-----------|---------------|-------------|-------|-------|
| arithmetic | 0.400 | 0.268 | 49.3% | 4-wide vs 6-wide issue |
| dependency | 1.200 | 1.009 | 18.9% | Closest to target |
| branch | 1.800 | 1.190 | 51.3% | **Benchmark mismatch** |
| **Average** | — | — | **39.8%** | |

**Target:** <20% average error (#141)

## Next Steps

1. **Align benchmarks** — create matching native/simulator tests
2. **Conditional branch optimization** — macro-op fusion (cmp+branch)
3. **Review issue width** — 4-wide vs M2's 6-wide affecting arithmetic

## Active PRs

- **PR #202** — [Cathy] Remove dead code: DetectLoadUseHazard (awaiting bob-approved)

## Active Investigations

- **#197** — Embench timing run request (waiting on human)
- **#201** — Zero-cycle branches (investigation complete — already working!)

## Calibration Milestones

| Milestone | Status | Description |
|-----------|--------|-------------|
| C1 | ✅ Complete | Benchmarks execute to completion |
| C2 | 🚧 In Progress | Accuracy calibration — benchmark alignment needed |
| C3 | Pending | Intermediate benchmark timing |

## Stats

- 51 PRs merged total
- 205 pipeline tests passing
- Zero-cycle branch elimination: working ✓
- Branch predictor: working ✓
