# M2Sim Progress Report

**Last updated:** 2026-02-05 07:31 EST (Cycle 231)

## Current Status

| Metric | Value |
|--------|-------|
| Total PRs Merged | 61 |
| Open PRs | 2 (#218, #220) |
| Open Issues | 14 |
| Pipeline Coverage | 77.0% |

## Cycle 231 Updates

- **PR #217** (Cathy load/store byte/half tests) — **MERGED ✅**
- **reports/ folder** — deleted (was duplicate of docs/)
- **Emu coverage** — 47.4% → 50.2% (+2.8pp)
- **PR #218** (Cathy bitfield/cond select tests) — CI running
- **PR #220** (Bob 8-wide benchmark enable) — CI running

## Key Progress This Cycle

**PR #217 — Load/store byte/halfword tests (MERGED ✅)**
- 11 functions now at 100%: LDRB, STRB, LDRSB64/32, LDRH, STRH, LDRSH64/32, LDRSW, LDR32SP, STR32SP
- Tests verify zero/sign extension, edge cases
- Emu coverage: 47.4% → 50.2%

**Issue #216 — Housekeeping (COMPLETE ✅)**
- [x] reports/ folder deleted (duplicates of docs/)
- [x] Calibration milestones documented in SPEC.md
- [x] Archive in docs/ kept for historical reference

## Accuracy Status (Microbenchmarks)

| Benchmark | Simulator CPI | M2 Real CPI | Error | Notes |
|-----------|---------------|-------------|-------|-------|
| arithmetic | 0.400 | 0.268 | 49.3% | Awaiting 8-wide validation |
| dependency | 1.200 | 1.009 | 18.9% | ✅ Near target |
| branch_taken_conditional | 1.600 | 1.190 | 34.5% | ↓ from 62.5% |
| **Average** | — | — | 34.2% | Target: <20% |

**Key finding (Eric):** Benchmarks still running 6-wide! PR #220 enables 8-wide in harness.

## Coverage Analysis

| Package | Coverage | Status |
|---------|----------|--------|
| timing/cache | 89.1% | ✅ |
| timing/pipeline | 77.0% | ✅ |
| timing/latency | 73.3% | ✅ |
| timing/core | 100% | ✅ |
| emu | 50.2% | Target: 70%+ |

## Active Work

- PR #218: Bitfield/conditional select tests (Cathy)
- PR #220: Enable 8-wide in benchmarks (Bob) — **critical for accuracy validation**
- Issue #219: Update benchmark harness to use 8-wide (addressed by PR #220)

## Potential Accuracy Improvements

Per Eric's analysis:
1. ~~CMP + B.cond fusion~~ — **DONE** (PR #212)
2. ~~8-wide decode~~ — **DONE** (PR #215)
3. 8-wide benchmark enable — **PR #220** (in progress)
4. Branch predictor tuning (see docs/branch-predictor-tuning.md)
5. Pipeline stall reduction

## Calibration Milestones

| Milestone | Status | Description |
|-----------|--------|-------------|
| C1 | ✅ Complete | Benchmarks execute to completion |
| C2 | 🚧 In Progress | Accuracy calibration — 34.2% avg, target <20% |
| C3 | Pending | Intermediate benchmark timing (PolyBench) |

## Stats

- 61 PRs merged total
- 205+ tests passing
- timing/core coverage: 100% ✓
- emu coverage: 50.2% (target 70%+)
