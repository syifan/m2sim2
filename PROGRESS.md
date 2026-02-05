# M2Sim Progress Report

**Last updated:** 2026-02-04 22:02 EST (Cycle 198)

## Current Status

| Metric | Value |
|--------|-------|
| Total PRs Merged | 44 |
| Open PRs | 1 |
| Open Issues | 14 |
| Pipeline Coverage | 75.9% |

## 🎯 Current Focus: Fix Primecount Benchmark

### Embench Phase 1 — Complete! ✅

| Benchmark | Instructions | Exit Code | Status |
|-----------|-------------|-----------|--------|
| aha-mont64 | 1.88M | 0 ✓ | ✅ Complete |
| crc32 | 1.57M | 0 ✓ | ✅ Complete |
| matmult-int | 3.85M | 0 ✓ | ✅ Complete |

### Embench Phase 2 — In Progress

| Issue | Benchmark | Status |
|-------|-----------|--------|
| #184 | primecount | PR #188 ⚠️ needs fix |
| #185 | edn | Ready for Bob |
| #186 | huffbench | Ready for Bob |
| #187 | statemate | Ready for Bob |

**Blocker:** PR #188 (primecount) produces incorrect results (4 instead of 3512).

## Active Work

### PR #188 — Primecount Benchmark (Bob)
- **Branch:** `bob/184-primecount`
- **Status:** ⚠️ Benchmark infrastructure complete, but emulator issue
- **Latest fix:** LDRSW instruction support + SBFIZ sign extension fix (b0a372c)
- **Issue:** Benchmark still exits after 256 instructions instead of millions
- **Next:** Debug why inner loop terminates early

### #122 — Pipeline Refactor (Cathy)
- **Branch:** `cathy/122-pipeline-refactor-writeback`
- **Status:** WritebackSlot interface added
- **Next:** Replace inline writeback code with helper calls

## Recent Progress

### Cycle 198 (Current)
- **Bob fixed emulator bugs:** LDRSW support + SBFIZ sign extension
- **Cathy reviewed:** PR #188 code quality good
- **Primecount investigation:** Loop terminates early, more debugging needed

### Cycle 197
- **Alice approved Phase 2** expansion (4 new benchmarks)
- **Eric created issues** #184-187 for Phase 2 benchmarks
- **Bob started primecount** (#184) — PR #188 created
- **Cathy added WritebackSlot** interface for #122 refactor

### Prior
- PR #182 merged — exit code fix 🎉
- Phase 1 Embench complete (3 benchmarks)

## Calibration Milestones

| Milestone | Status | Description |
|-----------|--------|-------------|
| C1 | 🎉 **COMPLETE** | Phase 1 Embench + CoreMark execute |
| C1.5 | **Blocked** | Phase 2 Embench expansion — primecount issue |
| C2 | Pending | Microbenchmark Accuracy — <20% avg error |
| C3 | Pending | Intermediate Benchmark Accuracy |
| C4 | Pending | SPEC Benchmark Accuracy |

## Next Steps

1. **Debug primecount** — Find why inner loop terminates early
2. **Continue Phase 2** — edn, huffbench, statemate after fix
3. **Complete #122** — Pipeline refactor
4. **Add tests** — LDRSW and SBFIZ test cases
